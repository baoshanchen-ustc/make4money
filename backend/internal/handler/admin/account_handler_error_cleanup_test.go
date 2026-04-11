package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type stubAccountTestRunner struct {
	results map[int64]*service.ScheduledTestResult
	errs    map[int64]error
	calls   []int64
}

func (s *stubAccountTestRunner) TestAccountConnection(c *gin.Context, accountID int64, modelID string, prompt string) error {
	s.calls = append(s.calls, accountID)
	if err, ok := s.errs[accountID]; ok {
		return err
	}
	return nil
}

func (s *stubAccountTestRunner) RunTestBackground(ctx context.Context, accountID int64, modelID string) (*service.ScheduledTestResult, error) {
	s.calls = append(s.calls, accountID)
	if err, ok := s.errs[accountID]; ok {
		return nil, err
	}
	if result, ok := s.results[accountID]; ok {
		return result, nil
	}
	return &service.ScheduledTestResult{Status: "failed", ErrorMessage: "test failed"}, nil
}

type stubAccountRecoveryService struct {
	recoveredIDs []int64
	recoverErrs  map[int64]error
}

func (s *stubAccountRecoveryService) RecoverAccountAfterSuccessfulTest(ctx context.Context, accountID int64) (*service.SuccessfulTestRecoveryResult, error) {
	if err, ok := s.recoverErrs[accountID]; ok {
		return nil, err
	}
	s.recoveredIDs = append(s.recoveredIDs, accountID)
	return &service.SuccessfulTestRecoveryResult{ClearedError: true}, nil
}

func (s *stubAccountRecoveryService) RecoverAccountState(ctx context.Context, accountID int64, options service.AccountRecoveryOptions) (*service.SuccessfulTestRecoveryResult, error) {
	return s.RecoverAccountAfterSuccessfulTest(ctx, accountID)
}

func (s *stubAccountRecoveryService) ClearRateLimit(ctx context.Context, accountID int64) error {
	return nil
}

func (s *stubAccountRecoveryService) GetTempUnschedStatus(ctx context.Context, accountID int64) (*service.TempUnschedState, error) {
	return nil, nil
}

func (s *stubAccountRecoveryService) ClearTempUnschedulable(ctx context.Context, accountID int64) error {
	return nil
}

func setupErrorCleanupRouter(handler *AccountHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/v1/admin/accounts/error-cleanup/preview", handler.PreviewErrorCleanup)
	router.POST("/api/v1/admin/accounts/error-cleanup/execute", handler.ExecuteErrorCleanup)
	return router
}

func TestAccountHandlerPreviewErrorCleanup(t *testing.T) {
	adminSvc := newStubAdminService()
	adminSvc.accounts = []service.Account{
		{ID: 101, Name: "error-ok", Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth, Status: service.StatusError},
		{ID: 102, Name: "error-still-bad", Platform: service.PlatformAnthropic, Type: service.AccountTypeOAuth, Status: service.StatusError},
	}
	accountTester := &stubAccountTestRunner{
		results: map[int64]*service.ScheduledTestResult{
			101: {Status: "success"},
			102: {Status: "failed", ErrorMessage: "401 invalid token"},
		},
	}
	recovery := &stubAccountRecoveryService{}
	handler := &AccountHandler{
		adminService:       adminSvc,
		accountTestService: accountTester,
		rateLimitService:   recovery,
	}
	router := setupErrorCleanupRouter(handler)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/error-cleanup/preview", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, float64(0), resp["code"])

	data := resp["data"].(map[string]any)
	require.Equal(t, float64(2), data["total_error"])
	require.Equal(t, float64(2), data["tested"])
	require.Equal(t, float64(1), data["recovered"])
	require.Equal(t, float64(1), data["delete_candidates"])
	require.Equal(t, []int64{101}, recovery.recoveredIDs)

	candidateIDsAny := data["candidate_ids"].([]any)
	require.Len(t, candidateIDsAny, 1)
	require.Equal(t, float64(102), candidateIDsAny[0])

	results := data["results"].([]any)
	require.Len(t, results, 2)
	first := results[0].(map[string]any)
	second := results[1].(map[string]any)
	require.Equal(t, float64(101), first["account_id"])
	require.Equal(t, true, first["recovered"])
	require.Equal(t, "success", first["test_status"])
	require.Equal(t, float64(102), second["account_id"])
	require.Equal(t, true, second["delete_candidate"])
	require.Equal(t, "401 invalid token", second["error"])
}

func TestAccountHandlerExecuteErrorCleanup(t *testing.T) {
	adminSvc := newStubAdminService()
	adminSvc.accounts = []service.Account{
		{ID: 201, Name: "delete-me", Status: service.StatusError},
		{ID: 202, Name: "already-recovered", Status: service.StatusActive},
		{ID: 203, Name: "delete-fails", Status: service.StatusError},
	}
	adminSvc.deleteAccountErrByID = map[int64]error{
		203: errors.New("delete failed"),
	}
	handler := &AccountHandler{adminService: adminSvc}
	router := setupErrorCleanupRouter(handler)

	body, _ := json.Marshal(map[string]any{
		"account_ids": []int64{201, 202, 203, 204},
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/error-cleanup/execute", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, float64(0), resp["code"])

	data := resp["data"].(map[string]any)
	require.Equal(t, float64(4), data["total"])
	require.Equal(t, float64(1), data["success"])
	require.Equal(t, float64(2), data["failed"])

	warnings := data["warnings"].([]any)
	require.Len(t, warnings, 1)
	require.Equal(t, float64(202), warnings[0].(map[string]any)["account_id"])

	errorsList := data["errors"].([]any)
	require.Len(t, errorsList, 2)
	require.ElementsMatch(t, []int64{201, 203}, adminSvc.deletedAccountIDs)
}
