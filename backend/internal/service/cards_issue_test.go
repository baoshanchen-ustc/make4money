//go:build unit

package service

import (
	"context"
	"math"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type cardsIssueAdminServiceStub struct {
	createInput      *CreateUserInput
	createUser       *User
	createErr        error
	updateUser       *User
	updateErr        error
	updatedUserID    int64
	updatedAmount    float64
	updatedOperation string
	updatedNotes     string
}

func (s *cardsIssueAdminServiceStub) CreateUser(ctx context.Context, input *CreateUserInput) (*User, error) {
	if input != nil {
		clone := *input
		s.createInput = &clone
	}
	if s.createErr != nil {
		return nil, s.createErr
	}
	if s.createUser != nil {
		clone := *s.createUser
		return &clone, nil
	}
	return &User{}, nil
}

func (s *cardsIssueAdminServiceStub) UpdateUserBalance(ctx context.Context, userID int64, balance float64, operation string, notes string) (*User, error) {
	s.updatedUserID = userID
	s.updatedAmount = balance
	s.updatedOperation = operation
	s.updatedNotes = notes
	if s.updateErr != nil {
		return nil, s.updateErr
	}
	if s.updateUser != nil {
		clone := *s.updateUser
		clone.ID = userID
		clone.Balance = balance + clone.Balance - balance // keep configured value if caller set it
		return &clone, nil
	}
	return &User{ID: userID, Balance: balance}, nil
}

type cardsIssueBindingRepoStub struct {
	userID       int64
	found        bool
	findErr      error
	bindErr      error
	boundUserID  int64
	boundBuyerID string
}

func (s *cardsIssueBindingRepoStub) FindUserIDByBuyerID(ctx context.Context, buyerID string) (int64, bool, error) {
	return s.userID, s.found, s.findErr
}

func (s *cardsIssueBindingRepoStub) BindBuyerID(ctx context.Context, userID int64, buyerID string) error {
	s.boundUserID = userID
	s.boundBuyerID = buyerID
	return s.bindErr
}

func TestCardsIssueService_IssueOrder_CreatesUserAndRecharges(t *testing.T) {
	adminSvc := &cardsIssueAdminServiceStub{
		createUser: &User{ID: 101, Email: buildCardsIssueLoginEmail("buyer-1"), Username: "Alice", Status: StatusActive},
		updateUser: &User{ID: 101, Email: buildCardsIssueLoginEmail("buyer-1"), Username: "Alice", Balance: 39.8, Status: StatusActive},
	}
	bindingRepo := &cardsIssueBindingRepoStub{}
	userLookup := &userRepoStub{}
	svc := NewCardsIssueService(adminSvc, userLookup, bindingRepo)
	svc.randomPassword = func() (string, error) { return "Pwd#123456", nil }

	result, err := svc.IssueOrder(context.Background(), CardsIssueRequest{
		BuyerID:       "buyer-1",
		BuyerName:     "Alice",
		OrderID:       "order-1",
		OrderAmount:   19.9,
		OrderQuantity: 2,
		Extra:         map[string]any{"item_id": "item-9"},
	}, CardsIssueRuntimeConfig{ResponseTemplate: "{buyer_id}|{login_email}|{password}|{recharge_amount}|{user_status}"})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.Created)
	require.Equal(t, int64(101), result.UserID)
	require.Equal(t, buildCardsIssueLoginEmail("buyer-1"), result.LoginEmail)
	require.Equal(t, "Pwd#123456", result.Password)
	require.Equal(t, 39.8, result.RechargeAmount)
	require.Equal(t, "buyer-1|"+buildCardsIssueLoginEmail("buyer-1")+"|Pwd#123456|39.8|new", result.Card)
	require.NotNil(t, adminSvc.createInput)
	require.Equal(t, CardsIssueSignupSource, adminSvc.createInput.SignupSource)
	require.True(t, adminSvc.createInput.SkipDefaultSubscriptions)
	require.Equal(t, "add", adminSvc.updatedOperation)
	require.Equal(t, int64(101), bindingRepo.boundUserID)
	require.Equal(t, "buyer-1", bindingRepo.boundBuyerID)
}

func TestCardsIssueService_IssueOrder_RechargesExistingUser(t *testing.T) {
	adminSvc := &cardsIssueAdminServiceStub{
		updateUser: &User{ID: 42, Email: "existing@example.com", Username: "Existing", Balance: 15, Status: StatusActive},
	}
	bindingRepo := &cardsIssueBindingRepoStub{userID: 42, found: true}
	userLookup := &userRepoStub{}
	svc := NewCardsIssueService(adminSvc, userLookup, bindingRepo)

	result, err := svc.IssueOrder(context.Background(), CardsIssueRequest{
		BuyerID:       "buyer-42",
		BuyerName:     "Existing",
		OrderID:       "order-42",
		OrderAmount:   5,
		OrderQuantity: 3,
	}, CardsIssueRuntimeConfig{ResponseTemplate: "{buyer_id}|{password}|{user_status}"})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.Created)
	require.Empty(t, result.LoginEmail)
	require.Empty(t, result.Password)
	require.Equal(t, 15.0, result.RechargeAmount)
	require.Equal(t, int64(42), adminSvc.updatedUserID)
	require.Equal(t, "buyer-42||existing", result.Card)
	require.Nil(t, adminSvc.createInput)
}

func TestCardsIssueService_IssueOrder_RejectsInvalidAmounts(t *testing.T) {
	cases := []struct {
		name   string
		req    CardsIssueRequest
		reason string
	}{
		{
			name:   "zero amount",
			req:    CardsIssueRequest{BuyerID: "b", OrderID: "o", OrderAmount: 0, OrderQuantity: 1},
			reason: "CARDS_ISSUE_ORDER_AMOUNT_INVALID",
		},
		{
			name:   "negative amount",
			req:    CardsIssueRequest{BuyerID: "b", OrderID: "o", OrderAmount: -1, OrderQuantity: 1},
			reason: "CARDS_ISSUE_ORDER_AMOUNT_INVALID",
		},
		{
			name:   "nan amount",
			req:    CardsIssueRequest{BuyerID: "b", OrderID: "o", OrderAmount: math.NaN(), OrderQuantity: 1},
			reason: "CARDS_ISSUE_ORDER_AMOUNT_INVALID",
		},
		{
			name:   "inf amount",
			req:    CardsIssueRequest{BuyerID: "b", OrderID: "o", OrderAmount: math.Inf(1), OrderQuantity: 1},
			reason: "CARDS_ISSUE_ORDER_AMOUNT_INVALID",
		},
		{
			name:   "amount above per-unit cap",
			req:    CardsIssueRequest{BuyerID: "b", OrderID: "o", OrderAmount: CardsIssueMaxOrderAmount + 1, OrderQuantity: 1},
			reason: "CARDS_ISSUE_ORDER_AMOUNT_TOO_LARGE",
		},
		{
			name:   "quantity above cap",
			req:    CardsIssueRequest{BuyerID: "b", OrderID: "o", OrderAmount: 1, OrderQuantity: CardsIssueMaxOrderQuantity + 1},
			reason: "CARDS_ISSUE_ORDER_QUANTITY_TOO_LARGE",
		},
		{
			name:   "product above recharge cap",
			req:    CardsIssueRequest{BuyerID: "b", OrderID: "o", OrderAmount: CardsIssueMaxOrderAmount, OrderQuantity: 2},
			reason: "CARDS_ISSUE_RECHARGE_AMOUNT_TOO_LARGE",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := NewCardsIssueService(&cardsIssueAdminServiceStub{}, &userRepoStub{}, &cardsIssueBindingRepoStub{})
			_, err := svc.IssueOrder(context.Background(), tc.req, CardsIssueRuntimeConfig{})
			require.Error(t, err)
			require.Equal(t, tc.reason, infraerrors.Reason(err), "unexpected reason for case %s", tc.name)
		})
	}
}

// cardsIssueRaceBindingStub resolves the binding on the second FindUserIDByBuyerID
// call, simulating a concurrent winner inserting the buyer_id mapping between the
// first lookup (miss) and the post-ErrEmailExists re-check.
type cardsIssueRaceBindingStub struct {
	winnerUserID int64
	lookupCount  int
}

func (s *cardsIssueRaceBindingStub) FindUserIDByBuyerID(_ context.Context, _ string) (int64, bool, error) {
	s.lookupCount++
	if s.lookupCount == 1 {
		return 0, false, nil
	}
	return s.winnerUserID, true, nil
}
func (s *cardsIssueRaceBindingStub) BindBuyerID(_ context.Context, _ int64, _ string) error {
	return nil
}

// cardsIssueNilUserLookupStub returns (nil, nil) from GetByEmail to exercise the
// defensive branch in resolveUserAfterEmailConflict.
type cardsIssueNilUserLookupStub struct{ called int }

func (s *cardsIssueNilUserLookupStub) GetByEmail(_ context.Context, _ string) (*User, error) {
	s.called++
	return nil, nil
}

func TestCardsIssueService_IssueOrder_EmailConflictReusesExistingBinding(t *testing.T) {
	adminSvc := &cardsIssueAdminServiceStub{
		createErr:  ErrEmailExists,
		updateUser: &User{ID: 77, Username: "Existing", Balance: 20, Status: StatusActive},
	}
	bindingRepo := &cardsIssueRaceBindingStub{winnerUserID: 77}
	userLookup := &cardsIssueNilUserLookupStub{}
	svc := NewCardsIssueService(adminSvc, userLookup, bindingRepo)
	svc.randomPassword = func() (string, error) { return "Pwd#99999", nil }

	result, err := svc.IssueOrder(context.Background(), CardsIssueRequest{
		BuyerID:       "buyer-race-2",
		OrderID:       "order-race-2",
		OrderAmount:   5,
		OrderQuantity: 2,
	}, CardsIssueRuntimeConfig{ResponseTemplate: "{user_id}|{password}|{user_status}"})
	require.NoError(t, err)
	require.False(t, result.Created)
	require.Empty(t, result.Password)
	require.Equal(t, int64(77), result.UserID)
	require.Equal(t, "77||existing", result.Card)
	require.Equal(t, 2, bindingRepo.lookupCount, "binding lookup must be retried after email conflict")
	require.Zero(t, userLookup.called, "email lookup must be skipped when binding already resolved")
}

func TestCardsIssueService_IssueOrder_EmailConflictReturnsErrorWhenNoUser(t *testing.T) {
	adminSvc := &cardsIssueAdminServiceStub{createErr: ErrEmailExists}
	bindingRepo := &cardsIssueBindingRepoStub{}
	userLookup := &cardsIssueNilUserLookupStub{}
	svc := NewCardsIssueService(adminSvc, userLookup, bindingRepo)
	svc.randomPassword = func() (string, error) { return "Pwd#aaaaa", nil }

	_, err := svc.IssueOrder(context.Background(), CardsIssueRequest{
		BuyerID:       "buyer-race-3",
		OrderID:       "order-race-3",
		OrderAmount:   1,
		OrderQuantity: 1,
	}, CardsIssueRuntimeConfig{})
	require.Error(t, err)
	require.Equal(t, "CARDS_ISSUE_USER_LOOKUP_FAILED", infraerrors.Reason(err))
}

func TestCardsIssueService_IssueOrder_EmailConflictFallsBackToExistingUser(t *testing.T) {
	loginEmail := buildCardsIssueLoginEmail("buyer-race")
	adminSvc := &cardsIssueAdminServiceStub{
		createErr:  ErrEmailExists,
		updateUser: &User{ID: 55, Email: loginEmail, Username: "Race User", Balance: 10, Status: StatusActive},
	}
	bindingRepo := &cardsIssueBindingRepoStub{}
	userLookup := &userRepoStub{usersByEmail: map[string]*User{
		loginEmail: {ID: 55, Email: loginEmail, Username: "Race User", Status: StatusActive},
	}}
	svc := NewCardsIssueService(adminSvc, userLookup, bindingRepo)
	svc.randomPassword = func() (string, error) { return "Pwd#123456", nil }

	result, err := svc.IssueOrder(context.Background(), CardsIssueRequest{
		BuyerID:       "buyer-race",
		OrderID:       "order-race",
		OrderAmount:   10,
		OrderQuantity: 1,
	}, CardsIssueRuntimeConfig{ResponseTemplate: "{created}|{password}|{user_status}"})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.Created)
	require.Empty(t, result.Password)
	require.Equal(t, "false||existing", result.Card)
	require.Equal(t, int64(55), bindingRepo.boundUserID)
}
