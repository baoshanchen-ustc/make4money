package service

import (
	"context"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

type refundProviderStub struct {
	key string
}

func (s refundProviderStub) Name() string { return s.key }

func (s refundProviderStub) ProviderKey() string { return s.key }

func (s refundProviderStub) SupportedTypes() []payment.PaymentType { return nil }

func (s refundProviderStub) CreatePayment(context.Context, payment.CreatePaymentRequest) (*payment.CreatePaymentResponse, error) {
	return nil, nil
}

func (s refundProviderStub) QueryOrder(context.Context, string) (*payment.QueryOrderResponse, error) {
	return nil, nil
}

func (s refundProviderStub) VerifyNotification(context.Context, string, map[string]string) (*payment.PaymentNotification, error) {
	return nil, nil
}

func (s refundProviderStub) Refund(context.Context, payment.RefundRequest) (*payment.RefundResponse, error) {
	return nil, nil
}

type refundUserRepoStub struct {
	user *User
}

func (s *refundUserRepoStub) Create(context.Context, *User) error {
	panic("unexpected Create call")
}

func (s *refundUserRepoStub) GetByID(context.Context, int64) (*User, error) {
	if s.user == nil {
		return nil, ErrUserNotFound
	}
	return s.user, nil
}

func (s *refundUserRepoStub) GetByEmail(context.Context, string) (*User, error) {
	panic("unexpected GetByEmail call")
}

func (s *refundUserRepoStub) GetFirstAdmin(context.Context) (*User, error) {
	panic("unexpected GetFirstAdmin call")
}

func (s *refundUserRepoStub) Update(context.Context, *User) error {
	panic("unexpected Update call")
}

func (s *refundUserRepoStub) Delete(context.Context, int64) error {
	panic("unexpected Delete call")
}

func (s *refundUserRepoStub) List(context.Context, pagination.PaginationParams) ([]User, *pagination.PaginationResult, error) {
	panic("unexpected List call")
}

func (s *refundUserRepoStub) ListWithFilters(context.Context, pagination.PaginationParams, UserListFilters) ([]User, *pagination.PaginationResult, error) {
	panic("unexpected ListWithFilters call")
}

func (s *refundUserRepoStub) UpdateBalance(context.Context, int64, float64) error {
	panic("unexpected UpdateBalance call")
}

func (s *refundUserRepoStub) DeductBalance(context.Context, int64, float64) error {
	panic("unexpected DeductBalance call")
}

func (s *refundUserRepoStub) UpdateConcurrency(context.Context, int64, int) error {
	panic("unexpected UpdateConcurrency call")
}

func (s *refundUserRepoStub) ExistsByEmail(context.Context, string) (bool, error) {
	panic("unexpected ExistsByEmail call")
}

func (s *refundUserRepoStub) RemoveGroupFromAllowedGroups(context.Context, int64) (int64, error) {
	panic("unexpected RemoveGroupFromAllowedGroups call")
}

func (s *refundUserRepoStub) AddGroupToAllowedGroups(context.Context, int64, int64) error {
	panic("unexpected AddGroupToAllowedGroups call")
}

func (s *refundUserRepoStub) RemoveGroupFromUserAllowedGroups(context.Context, int64, int64) error {
	panic("unexpected RemoveGroupFromUserAllowedGroups call")
}

func (s *refundUserRepoStub) UpdateTotpSecret(context.Context, int64, *string) error {
	panic("unexpected UpdateTotpSecret call")
}

func (s *refundUserRepoStub) EnableTotp(context.Context, int64) error {
	panic("unexpected EnableTotp call")
}

func (s *refundUserRepoStub) DisableTotp(context.Context, int64) error {
	panic("unexpected DisableTotp call")
}

func TestResolveRefundTradeNo(t *testing.T) {
	t.Parallel()

	svc := &PaymentService{}
	order := &dbent.PaymentOrder{
		ID:         42,
		OutTradeNo: "sub2_test_order",
	}

	t.Run("returns stored payment trade number when present", func(t *testing.T) {
		t.Parallel()

		o := *order
		o.PaymentTradeNo = "pi_123"
		tradeNo, err := svc.resolveRefundTradeNo(context.Background(), refundProviderStub{key: payment.TypeStripe}, &o)
		if err != nil {
			t.Fatalf("resolveRefundTradeNo returned error: %v", err)
		}
		if tradeNo != "pi_123" {
			t.Fatalf("tradeNo = %q, want pi_123", tradeNo)
		}
	})

	t.Run("easypay allows refund by out_trade_no when trade number is missing", func(t *testing.T) {
		t.Parallel()

		tradeNo, err := svc.resolveRefundTradeNo(context.Background(), refundProviderStub{key: payment.TypeEasyPay}, order)
		if err != nil {
			t.Fatalf("resolveRefundTradeNo returned error: %v", err)
		}
		if tradeNo != "" {
			t.Fatalf("tradeNo = %q, want empty string", tradeNo)
		}
	})

	t.Run("stripe requires payment intent id when trade number is missing", func(t *testing.T) {
		t.Parallel()

		if _, err := svc.resolveRefundTradeNo(context.Background(), refundProviderStub{key: payment.TypeStripe}, order); err == nil {
			t.Fatal("expected missing stripe payment trade number to fail")
		}
	})
}

func TestPrepDeductAllowsPartialBalanceDeductionWithoutForce(t *testing.T) {
	t.Parallel()

	svc := &PaymentService{
		userRepo: &refundUserRepoStub{
			user: &User{ID: 1, Balance: 12},
		},
	}
	order := &dbent.PaymentOrder{
		UserID:    1,
		OrderType: payment.OrderTypeBalance,
	}
	plan := &RefundPlan{
		RefundAmount: 20,
	}

	result := svc.prepDeduct(context.Background(), order, plan, false)
	if result != nil {
		t.Fatalf("prepDeduct returned unexpected early result: %+v", result)
	}
	if plan.DeductionType != payment.DeductionTypeBalance {
		t.Fatalf("DeductionType = %q, want %q", plan.DeductionType, payment.DeductionTypeBalance)
	}
	if plan.BalanceToDeduct != 12 {
		t.Fatalf("BalanceToDeduct = %v, want 12", plan.BalanceToDeduct)
	}
}
