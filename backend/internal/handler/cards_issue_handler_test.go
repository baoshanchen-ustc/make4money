//go:build unit

package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type cardsIssueSettingRepoStub struct {
	values map[string]string
}

func (s *cardsIssueSettingRepoStub) Get(ctx context.Context, key string) (*service.Setting, error) {
	panic("unexpected Get call")
}

func (s *cardsIssueSettingRepoStub) GetValue(ctx context.Context, key string) (string, error) {
	return s.values[key], nil
}

func (s *cardsIssueSettingRepoStub) Set(ctx context.Context, key, value string) error {
	if s.values == nil {
		s.values = map[string]string{}
	}
	s.values[key] = value
	return nil
}

func (s *cardsIssueSettingRepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := s.values[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}

func (s *cardsIssueSettingRepoStub) SetMultiple(ctx context.Context, settings map[string]string) error {
	if s.values == nil {
		s.values = map[string]string{}
	}
	for k, v := range settings {
		s.values[k] = v
	}
	return nil
}

func (s *cardsIssueSettingRepoStub) GetAll(ctx context.Context) (map[string]string, error) {
	out := make(map[string]string, len(s.values))
	for k, v := range s.values {
		out[k] = v
	}
	return out, nil
}

func (s *cardsIssueSettingRepoStub) Delete(ctx context.Context, key string) error {
	delete(s.values, key)
	return nil
}

type cardsIssueHandlerAdminStub struct {
	createCalls   int
	createErr     error
	updateBalance float64
	updateOp      string
	updateErr     error
	lastUser      *service.User
}

func (s *cardsIssueHandlerAdminStub) CreateUser(_ context.Context, input *service.CreateUserInput) (*service.User, error) {
	s.createCalls++
	if s.createErr != nil {
		return nil, s.createErr
	}
	u := &service.User{
		ID:           42,
		Email:        input.Email,
		Username:     input.Username,
		SignupSource: input.SignupSource,
	}
	s.lastUser = u
	return u, nil
}

func (s *cardsIssueHandlerAdminStub) UpdateUserBalance(_ context.Context, userID int64, balance float64, op, _ string) (*service.User, error) {
	s.updateBalance = balance
	s.updateOp = op
	if s.updateErr != nil {
		return nil, s.updateErr
	}
	if s.lastUser != nil && s.lastUser.ID == userID {
		clone := *s.lastUser
		clone.Balance = balance
		return &clone, nil
	}
	return &service.User{ID: userID, Balance: balance}, nil
}

type cardsIssueHandlerUserLookupStub struct{}

func (cardsIssueHandlerUserLookupStub) GetByEmail(_ context.Context, _ string) (*service.User, error) {
	return nil, service.ErrUserNotFound
}

type cardsIssueHandlerBindingStub struct {
	bindings map[string]int64
}

func (s *cardsIssueHandlerBindingStub) FindUserIDByBuyerID(_ context.Context, buyerID string) (int64, bool, error) {
	if id, ok := s.bindings[buyerID]; ok {
		return id, true, nil
	}
	return 0, false, nil
}

func (s *cardsIssueHandlerBindingStub) BindBuyerID(_ context.Context, userID int64, buyerID string) error {
	if s.bindings == nil {
		s.bindings = map[string]int64{}
	}
	s.bindings[buyerID] = userID
	return nil
}

func buildCardsIssueTestHandler(t *testing.T, settingValues map[string]string) (*CardsIssueHandler, *cardsIssueHandlerAdminStub, *cardsIssueHandlerBindingStub) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	service.SetDefaultIdempotencyCoordinator(nil)

	repo := &cardsIssueSettingRepoStub{values: settingValues}
	settingSvc := service.NewSettingService(repo, &config.Config{})
	adminSvc := &cardsIssueHandlerAdminStub{}
	bindingRepo := &cardsIssueHandlerBindingStub{}
	cardsSvc := service.NewCardsIssueService(adminSvc, cardsIssueHandlerUserLookupStub{}, bindingRepo)
	return NewCardsIssueHandler(cardsSvc, settingSvc), adminSvc, bindingRepo
}

func newCardsIssueTestContext(method, body, authHeader string) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	var reader *bytes.Reader
	if body == "" {
		reader = bytes.NewReader(nil)
	} else {
		reader = bytes.NewReader([]byte(body))
	}
	c.Request = httptest.NewRequest(method, "/api/custom/cards/issue", reader)
	c.Request.Header.Set("Content-Type", "application/json")
	if authHeader != "" {
		c.Request.Header.Set("Authorization", authHeader)
	}
	return c, recorder
}

func readCardsIssueResp(t *testing.T, recorder *httptest.ResponseRecorder) (int, string) {
	t.Helper()
	var resp struct {
		Code   int    `json:"code"`
		Reason string `json:"reason"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	return resp.Code, resp.Reason
}

func TestCardsIssueHandler_Issue_ReturnsForbiddenWhenDisabled(t *testing.T) {
	h, _, _ := buildCardsIssueTestHandler(t, map[string]string{
		service.SettingKeyCardsIssueEnabled:   "false",
		service.SettingKeyCardsIssueBearerKey: "cards-issue-secret",
	})

	c, recorder := newCardsIssueTestContext(http.MethodPost, `{}`, "Bearer cards-issue-secret")
	h.Issue(c)

	require.Equal(t, http.StatusForbidden, recorder.Code)
	_, reason := readCardsIssueResp(t, recorder)
	require.Equal(t, "CARDS_ISSUE_DISABLED", reason)
}

func TestCardsIssueHandler_Issue_ReturnsServiceUnavailableWhenKeyMissing(t *testing.T) {
	h, _, _ := buildCardsIssueTestHandler(t, map[string]string{
		service.SettingKeyCardsIssueEnabled: "true",
	})

	c, recorder := newCardsIssueTestContext(http.MethodPost, `{}`, "Bearer anything")
	h.Issue(c)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	_, reason := readCardsIssueResp(t, recorder)
	require.Equal(t, "CARDS_ISSUE_KEY_UNCONFIGURED", reason)
}

func TestCardsIssueHandler_Issue_RejectsMissingAuthorization(t *testing.T) {
	h, _, _ := buildCardsIssueTestHandler(t, map[string]string{
		service.SettingKeyCardsIssueEnabled:   "true",
		service.SettingKeyCardsIssueBearerKey: "cards-issue-secret",
	})

	c, recorder := newCardsIssueTestContext(http.MethodPost, `{}`, "")
	h.Issue(c)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	_, reason := readCardsIssueResp(t, recorder)
	require.Equal(t, "CARDS_ISSUE_UNAUTHORIZED", reason)
}

func TestCardsIssueHandler_Issue_RejectsNonBearerScheme(t *testing.T) {
	h, _, _ := buildCardsIssueTestHandler(t, map[string]string{
		service.SettingKeyCardsIssueEnabled:   "true",
		service.SettingKeyCardsIssueBearerKey: "cards-issue-secret",
	})

	c, recorder := newCardsIssueTestContext(http.MethodPost, `{}`, "Basic cards-issue-secret")
	h.Issue(c)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestCardsIssueHandler_Issue_RejectsWrongKey(t *testing.T) {
	h, _, _ := buildCardsIssueTestHandler(t, map[string]string{
		service.SettingKeyCardsIssueEnabled:   "true",
		service.SettingKeyCardsIssueBearerKey: "cards-issue-secret",
	})

	c, recorder := newCardsIssueTestContext(http.MethodPost, `{}`, "Bearer wrong-key")
	h.Issue(c)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	_, reason := readCardsIssueResp(t, recorder)
	require.Equal(t, "CARDS_ISSUE_UNAUTHORIZED", reason)
}

func TestCardsIssueHandler_Issue_RejectsEmptyBody(t *testing.T) {
	h, _, _ := buildCardsIssueTestHandler(t, map[string]string{
		service.SettingKeyCardsIssueEnabled:   "true",
		service.SettingKeyCardsIssueBearerKey: "cards-issue-secret",
	})

	c, recorder := newCardsIssueTestContext(http.MethodPost, "", "Bearer cards-issue-secret")
	h.Issue(c)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	_, reason := readCardsIssueResp(t, recorder)
	require.Equal(t, "CARDS_ISSUE_INVALID_REQUEST", reason)
}

func TestCardsIssueHandler_Issue_RejectsInvalidJSON(t *testing.T) {
	h, _, _ := buildCardsIssueTestHandler(t, map[string]string{
		service.SettingKeyCardsIssueEnabled:   "true",
		service.SettingKeyCardsIssueBearerKey: "cards-issue-secret",
	})

	c, recorder := newCardsIssueTestContext(http.MethodPost, `{not-json`, "Bearer cards-issue-secret")
	h.Issue(c)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	_, reason := readCardsIssueResp(t, recorder)
	require.Equal(t, "CARDS_ISSUE_INVALID_REQUEST", reason)
}

func TestCardsIssueHandler_Issue_RejectsAmountAboveCap(t *testing.T) {
	h, _, _ := buildCardsIssueTestHandler(t, map[string]string{
		service.SettingKeyCardsIssueEnabled:   "true",
		service.SettingKeyCardsIssueBearerKey: "cards-issue-secret",
	})

	payload := map[string]any{
		"buyer_id":       "buyer-1",
		"order_id":       "order-1",
		"order_amount":   service.CardsIssueMaxOrderAmount + 1,
		"order_quantity": 1,
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)

	c, recorder := newCardsIssueTestContext(http.MethodPost, string(raw), "Bearer cards-issue-secret")
	h.Issue(c)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	_, reason := readCardsIssueResp(t, recorder)
	require.Equal(t, "CARDS_ISSUE_ORDER_AMOUNT_TOO_LARGE", reason)
}

func TestCardsIssueHandler_Issue_SucceedsForNewBuyer(t *testing.T) {
	h, adminSvc, bindingRepo := buildCardsIssueTestHandler(t, map[string]string{
		service.SettingKeyCardsIssueEnabled:          "true",
		service.SettingKeyCardsIssueBearerKey:        "cards-issue-secret",
		service.SettingKeyCardsIssueResponseTemplate: "{buyer_id}|{order_id}|{login_email}|{password}",
	})

	payload := map[string]any{
		"buyer_id":       "buyer-ok",
		"buyer_name":     "Bob",
		"order_id":       "order-ok",
		"order_amount":   19.9,
		"order_quantity": 2,
		"item_id":        "item-1",
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)

	c, recorder := newCardsIssueTestContext(http.MethodPost, string(raw), "Bearer cards-issue-secret")
	h.Issue(c)

	require.Equal(t, http.StatusOK, recorder.Code, "body=%s", recorder.Body.String())
	var resp struct {
		Code int `json:"code"`
		Data struct {
			Created        bool    `json:"created"`
			UserID         int64   `json:"user_id"`
			LoginEmail     string  `json:"login_email"`
			Password       string  `json:"password"`
			Username       string  `json:"username"`
			RechargeAmount float64 `json:"recharge_amount"`
			OrderID        string  `json:"order_id"`
			Card           string  `json:"card"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.True(t, resp.Data.Created)
	require.NotEmpty(t, resp.Data.LoginEmail)
	require.NotEmpty(t, resp.Data.Password)
	require.Equal(t, "order-ok", resp.Data.OrderID)
	require.InDelta(t, 39.8, resp.Data.RechargeAmount, 1e-9)
	require.True(t, strings.HasPrefix(resp.Data.Card, "buyer-ok|order-ok|"))
	require.Equal(t, 1, adminSvc.createCalls)
	require.Equal(t, "add", adminSvc.updateOp)
	require.InDelta(t, 39.8, adminSvc.updateBalance, 1e-9)
	require.Equal(t, resp.Data.UserID, bindingRepo.bindings["buyer-ok"])
}

func TestCardsIssueHandler_Issue_AcceptsStringNumericFields(t *testing.T) {
	h, adminSvc, _ := buildCardsIssueTestHandler(t, map[string]string{
		service.SettingKeyCardsIssueEnabled:   "true",
		service.SettingKeyCardsIssueBearerKey: "cards-issue-secret",
	})

	// Upstream callers that stringify all "params" entries should still work.
	payload := `{"buyer_id":"TEST-BUYER","buyer_name":"","order_id":"TEST-ORDER","order_amount":"99.00","order_quantity":"1"}`
	c, recorder := newCardsIssueTestContext(http.MethodPost, payload, "Bearer cards-issue-secret")
	h.Issue(c)

	require.Equal(t, http.StatusOK, recorder.Code, "body=%s", recorder.Body.String())
	var resp struct {
		Code int `json:"code"`
		Data struct {
			RechargeAmount float64 `json:"recharge_amount"`
			BalanceAfter   float64 `json:"balance_after"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.InDelta(t, 99.0, resp.Data.RechargeAmount, 1e-9)
	require.InDelta(t, 99.0, adminSvc.updateBalance, 1e-9)
}

func TestCardsIssueHandler_Issue_PropagatesServiceBadRequest(t *testing.T) {
	h, _, _ := buildCardsIssueTestHandler(t, map[string]string{
		service.SettingKeyCardsIssueEnabled:   "true",
		service.SettingKeyCardsIssueBearerKey: "cards-issue-secret",
	})

	// Missing order_id triggers service-layer validation (proves handler forwards
	// application errors through response.ErrorFrom with the right status).
	payload := map[string]any{
		"buyer_id":       "buyer-1",
		"order_amount":   1.0,
		"order_quantity": 1,
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)

	c, recorder := newCardsIssueTestContext(http.MethodPost, string(raw), "Bearer cards-issue-secret")
	h.Issue(c)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	_, reason := readCardsIssueResp(t, recorder)
	require.Equal(t, "CARDS_ISSUE_ORDER_ID_REQUIRED", reason)

	// Sanity: infraerrors.Reason round-trips from the service error so our response
	// payload stays aligned with what the service layer emits.
	require.Equal(t, "CARDS_ISSUE_ORDER_ID_REQUIRED", infraerrors.Reason(infraerrors.BadRequest("CARDS_ISSUE_ORDER_ID_REQUIRED", "noop")))
}
