package handler

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	_ "modernc.org/sqlite"
)

func TestVerifyOrderPublicReturnsFeeRate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	client := newPaymentHandlerEntClient(t)
	ctx := context.Background()
	now := time.Now().UTC()
	user, err := client.User.Create().
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
	_, err = client.PaymentOrder.Create().
		SetUserID(int64(user.ID)).
		SetUserEmail("user@example.com").
		SetUserName("user").
		SetAmount(100).
		SetPayAmount(102.5).
		SetFeeRate(2.5).
		SetRechargeCode("PAY-42").
		SetOutTradeNo("trade-42").
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("provider-trade-42").
		SetOrderType(payment.OrderTypeBalance).
		SetStatus(service.OrderStatusPaid).
		SetExpiresAt(now.Add(time.Hour)).
		SetPaidAt(now).
		SetClientIP("127.0.0.1").
		SetSrcHost("example.com").
		Save(ctx)
	require.NoError(t, err)

	paymentSvc := service.NewPaymentService(client, nil, nil, nil, nil, nil, paymentHandlerUserRepoStub{}, nil)
	paymentHandler := NewPaymentHandler(paymentSvc, nil, nil)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/payment/public/orders/verify", bytes.NewBufferString(`{"out_trade_no":"trade-42"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	paymentHandler.VerifyOrderPublic(c)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp struct {
		Code int `json:"code"`
		Data struct {
			OutTradeNo string  `json:"out_trade_no"`
			FeeRate    float64 `json:"fee_rate"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.Equal(t, "trade-42", resp.Data.OutTradeNo)
	require.Equal(t, 2.5, resp.Data.FeeRate)
}

func newPaymentHandlerEntClient(t *testing.T) *dbent.Client {
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
