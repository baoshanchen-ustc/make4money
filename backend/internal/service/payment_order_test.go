//go:build unit

package service

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"

	_ "modernc.org/sqlite"
)

func TestValidateOrderInput_RejectsDisabledPaymentType(t *testing.T) {
	t.Parallel()

	svc := &PaymentService{}
	cfg := &PaymentConfig{
		EnabledTypes: []string{payment.TypeWxpay},
	}

	_, err := svc.validateOrderInput(context.Background(), CreateOrderRequest{
		Amount:      10,
		PaymentType: payment.TypeAlipay,
	}, cfg)

	require.Error(t, err)
	require.Equal(t, "PAYMENT_TYPE_DISABLED", infraerrors.Reason(err))
	require.Equal(t, "payment method is disabled", infraerrors.Message(err))
}

func TestValidateOrderInput_AllowsEnabledPaymentTypeAfterNormalization(t *testing.T) {
	t.Parallel()

	svc := &PaymentService{}
	cfg := &PaymentConfig{
		EnabledTypes: []string{payment.TypeAlipay},
	}

	_, err := svc.validateOrderInput(context.Background(), CreateOrderRequest{
		Amount:      10,
		PaymentType: payment.TypeAlipayDirect,
	}, cfg)

	require.NoError(t, err)
}

func TestValidateOrderInput_AllowsVisiblePaymentTypeWhenEnabledTypesEmpty(t *testing.T) {
	t.Parallel()

	svc := &PaymentService{}
	cfg := &PaymentConfig{}

	_, err := svc.validateOrderInput(context.Background(), CreateOrderRequest{
		Amount:      10,
		PaymentType: payment.TypeAlipay,
	}, cfg)

	require.NoError(t, err)
}

func TestPsDailyLimitAmount_UsesPayAmountForBalanceOrders(t *testing.T) {
	t.Parallel()

	require.Equal(t, 102.5, psDailyLimitAmount(payment.OrderTypeBalance, 100, 102.5))
	require.Equal(t, 100.0, psDailyLimitAmount(payment.OrderTypeSubscription, 100, 102.5))
}

func TestCheckDailyLimit_CountsPendingOrders(t *testing.T) {
	t.Parallel()

	client := newPaymentOrderEntClient(t)
	ctx := context.Background()
	tx, err := client.Tx(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })

	now := time.Now().UTC()
	user, err := tx.User.Create().
		SetCreatedAt(now).
		SetUpdatedAt(now).
		SetEmail("user@example.com").
		SetPasswordHash("hashed-password").
		SetSignupSource("email").
		SetRole("user").
		SetBalance(0).
		SetConcurrency(1).
		SetStatus("active").
		SetUsername("user").
		SetNotes("").
		SetTotpEnabled(false).
		SetBalanceNotifyEnabled(false).
		SetBalanceNotifyThresholdType("").
		SetBalanceNotifyExtraEmails("[]").
		SetTotalRecharged(0).
		Save(ctx)
	require.NoError(t, err)

	_, err = tx.PaymentOrder.Create().
		SetUserID(int64(user.ID)).
		SetUserEmail("user@example.com").
		SetUserName("user").
		SetAmount(50).
		SetPayAmount(50).
		SetFeeRate(0).
		SetRechargeCode("PAY-1").
		SetOutTradeNo("trade-1").
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("").
		SetOrderType(payment.OrderTypeBalance).
		SetStatus(OrderStatusPending).
		SetExpiresAt(now.Add(time.Hour)).
		SetClientIP("127.0.0.1").
		SetSrcHost("example.com").
		SetCreatedAt(now).
		Save(ctx)
	require.NoError(t, err)

	svc := &PaymentService{}
	err = svc.checkDailyLimit(ctx, tx, int64(user.ID), 60, 100)
	require.Error(t, err)
	require.Equal(t, "DAILY_LIMIT_EXCEEDED", infraerrors.Reason(err))
}

func TestCreateOrderInTx_ReservesSelectedProviderInstance(t *testing.T) {
	t.Parallel()

	client := newPaymentOrderEntClient(t)
	ctx := context.Background()
	now := time.Now().UTC()
	user := createPaymentOrderUserForTest(t, client, now, "reserve@example.com")
	inst := createPaymentProviderInstanceForTest(t, client, payment.TypeAlipay, payment.TypeAlipay, `{"alipay":{"dailyLimit":500}}`)

	svc := &PaymentService{entClient: client}
	order, err := svc.createOrderInTx(ctx, CreateOrderRequest{
		UserID:      int64(user.ID),
		PaymentType: payment.TypeAlipay,
		OrderType:   payment.OrderTypeBalance,
		ClientIP:    "127.0.0.1",
		SrcHost:     "example.com",
	}, nil, &PaymentConfig{OrderTimeoutMin: 30}, 100, 100, 0, 100, &payment.InstanceSelection{
		InstanceID: strconv.FormatInt(int64(inst.ID), 10),
	})
	require.NoError(t, err)
	require.NotNil(t, order.ProviderInstanceID)
	require.Equal(t, strconv.FormatInt(int64(inst.ID), 10), *order.ProviderInstanceID)
}

func TestRevalidateSelectedInstance_RejectsDailyLimitOverflow(t *testing.T) {
	t.Parallel()

	client := newPaymentOrderEntClient(t)
	ctx := context.Background()
	now := time.Now().UTC()
	user := createPaymentOrderUserForTest(t, client, now, "limit@example.com")
	inst := createPaymentProviderInstanceForTest(t, client, payment.TypeAlipay, payment.TypeAlipay, `{"alipay":{"dailyLimit":100}}`)

	_, err := client.PaymentOrder.Create().
		SetUserID(int64(user.ID)).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(90).
		SetPayAmount(90).
		SetFeeRate(0).
		SetRechargeCode("PAY-LIMIT-1").
		SetOutTradeNo("trade-limit-1").
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("").
		SetOrderType(payment.OrderTypeBalance).
		SetStatus(OrderStatusPending).
		SetProviderInstanceID(strconv.FormatInt(int64(inst.ID), 10)).
		SetExpiresAt(now.Add(time.Hour)).
		SetClientIP("127.0.0.1").
		SetSrcHost("example.com").
		SetCreatedAt(now).
		Save(ctx)
	require.NoError(t, err)

	tx, err := client.Tx(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })

	svc := &PaymentService{}
	err = svc.revalidateSelectedInstance(ctx, tx, payment.TypeAlipay, 20, &payment.InstanceSelection{
		InstanceID: strconv.FormatInt(int64(inst.ID), 10),
	})
	require.Error(t, err)
	require.Equal(t, "NO_AVAILABLE_INSTANCE", infraerrors.Reason(err))
}

func TestRevalidateSelectedInstance_CountsPendingOrdersAcrossDays(t *testing.T) {
	t.Parallel()

	client := newPaymentOrderEntClient(t)
	ctx := context.Background()
	now := time.Now().UTC()
	yesterday := now.Add(-24 * time.Hour)
	user := createPaymentOrderUserForTest(t, client, now, "cross-day@example.com")
	inst := createPaymentProviderInstanceForTest(t, client, payment.TypeAlipay, payment.TypeAlipay, `{"alipay":{"dailyLimit":100}}`)

	_, err := client.PaymentOrder.Create().
		SetUserID(int64(user.ID)).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(90).
		SetPayAmount(90).
		SetFeeRate(0).
		SetRechargeCode("PAY-CROSS-1").
		SetOutTradeNo("trade-cross-1").
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("").
		SetOrderType(payment.OrderTypeBalance).
		SetStatus(OrderStatusPending).
		SetProviderInstanceID(strconv.FormatInt(int64(inst.ID), 10)).
		SetExpiresAt(now.Add(time.Hour)).
		SetClientIP("127.0.0.1").
		SetSrcHost("example.com").
		SetCreatedAt(yesterday).
		Save(ctx)
	require.NoError(t, err)

	tx, err := client.Tx(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })

	svc := &PaymentService{}
	err = svc.revalidateSelectedInstance(ctx, tx, payment.TypeAlipay, 20, &payment.InstanceSelection{
		InstanceID: strconv.FormatInt(int64(inst.ID), 10),
	})
	require.Error(t, err)
	require.Equal(t, "NO_AVAILABLE_INSTANCE", infraerrors.Reason(err))
}

func TestCreateOrderWithSelectionRetry_FallsBackToNextInstance(t *testing.T) {
	t.Parallel()

	req := CreateOrderRequest{PaymentType: payment.TypeAlipay}
	cfg := &PaymentConfig{LoadBalanceStrategy: string(payment.StrategyRoundRobin)}

	selectCalls := 0
	var excludedSnapshots []string

	selectionFnCalls := 0
	originalSelector := func(ctx context.Context, req CreateOrderRequest, cfg *PaymentConfig, payAmount float64, excluded []string) (*payment.InstanceSelection, error) {
		selectionFnCalls++
		excludedSnapshots = append(excludedSnapshots, strings.Join(excluded, ","))
		if len(excluded) == 0 {
			return &payment.InstanceSelection{InstanceID: "101"}, nil
		}
		return &payment.InstanceSelection{InstanceID: "202"}, nil
	}
	createCalls := 0
	originalCreate := func(sel *payment.InstanceSelection) (*dbent.PaymentOrder, error) {
		createCalls++
		if sel.InstanceID == "101" {
			return nil, infraerrors.TooManyRequests("NO_AVAILABLE_INSTANCE", "selected payment instance is no longer available")
		}
		return &dbent.PaymentOrder{ID: 1}, nil
	}

	selection, order, oauthResp, err := psCreateOrderWithSelectionRetry(
		func(excluded []string) (*payment.InstanceSelection, *CreateOrderResponse, error) {
			selectCalls++
			sel, selectErr := originalSelector(context.Background(), req, cfg, 10, excluded)
			return sel, nil, selectErr
		},
		originalCreate,
	)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.Equal(t, "202", selection.InstanceID)
	require.NotNil(t, order)
	require.Nil(t, oauthResp)
	require.Equal(t, 2, selectionFnCalls)
	require.Equal(t, 2, createCalls)
	require.Equal(t, []string{"", "101"}, excludedSnapshots)
	require.Equal(t, 2, selectCalls)
}

func TestCreateOrderWithSelectionRetry_StopsOnOAuthPreparation(t *testing.T) {
	t.Parallel()

	selection, order, oauthResp, err := psCreateOrderWithSelectionRetry(
		func(_ []string) (*payment.InstanceSelection, *CreateOrderResponse, error) {
			return &payment.InstanceSelection{InstanceID: "301"}, &CreateOrderResponse{ResultType: payment.CreatePaymentResultOAuthRequired}, nil
		},
		func(_ *payment.InstanceSelection) (*dbent.PaymentOrder, error) {
			t.Fatal("create should not be called when oauth response is returned")
			return nil, nil
		},
	)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.Nil(t, order)
	require.NotNil(t, oauthResp)
	require.Equal(t, payment.CreatePaymentResultOAuthRequired, oauthResp.ResultType)
}

func newPaymentOrderEntClient(t *testing.T) *dbent.Client {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared&_fk=1", t.Name())
	db, err := sql.Open("sqlite", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func createPaymentOrderUserForTest(t *testing.T, client *dbent.Client, now time.Time, email string) *dbent.User {
	t.Helper()

	user, err := client.User.Create().
		SetCreatedAt(now).
		SetUpdatedAt(now).
		SetEmail(email).
		SetPasswordHash("hashed-password").
		SetSignupSource("email").
		SetRole("user").
		SetBalance(0).
		SetConcurrency(1).
		SetStatus("active").
		SetUsername("user").
		SetNotes("").
		SetTotpEnabled(false).
		SetBalanceNotifyEnabled(false).
		SetBalanceNotifyThresholdType("").
		SetBalanceNotifyExtraEmails("[]").
		SetTotalRecharged(0).
		Save(context.Background())
	require.NoError(t, err)
	return user
}

func createPaymentProviderInstanceForTest(t *testing.T, client *dbent.Client, providerKey, supportedTypes, limits string) *dbent.PaymentProviderInstance {
	t.Helper()

	inst, err := client.PaymentProviderInstance.Create().
		SetProviderKey(providerKey).
		SetName(providerKey + "-instance").
		SetConfig("{}").
		SetSupportedTypes(supportedTypes).
		SetEnabled(true).
		SetLimits(limits).
		Save(context.Background())
	require.NoError(t, err)
	return inst
}
