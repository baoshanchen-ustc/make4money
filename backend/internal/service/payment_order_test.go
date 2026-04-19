//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
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

func TestPsDailyLimitAmount_UsesPayAmountForBalanceOrders(t *testing.T) {
	t.Parallel()

	require.Equal(t, 102.5, psDailyLimitAmount(payment.OrderTypeBalance, 100, 102.5))
	require.Equal(t, 100.0, psDailyLimitAmount(payment.OrderTypeSubscription, 100, 102.5))
}
