package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func setupAccountMixedChannelRouter(adminSvc *stubAdminService, scopeSvc service.ChannelAdminScopeService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	withAuthMiddleware(router)
	accountHandler := NewAccountHandler(adminSvc, scopeSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	router.POST("/api/v1/admin/accounts/check-mixed-channel", accountHandler.CheckMixedChannel)
	router.GET("/api/v1/admin/accounts", accountHandler.List)
	router.GET("/api/v1/admin/accounts/:id", accountHandler.GetByID)
	router.POST("/api/v1/admin/accounts", accountHandler.Create)
	router.PUT("/api/v1/admin/accounts/:id", accountHandler.Update)
	router.POST("/api/v1/admin/accounts/batch", accountHandler.BatchCreate)
	router.DELETE("/api/v1/admin/accounts/:id", accountHandler.Delete)
	router.POST("/api/v1/admin/accounts/bulk-update", accountHandler.BulkUpdate)
	return router
}

func withAdminAuth(req *http.Request) {
	req.Header.Set("X-Test-Auth", "admin")
}

func withChannelAdminAuth(req *http.Request) {
	req.Header.Set("X-Test-Auth", "channel-admin")
}

func withAuthMiddleware(router *gin.Engine) {
	router.Use(func(c *gin.Context) {
		switch c.GetHeader("X-Test-Auth") {
		case "channel-admin":
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 7})
			c.Set(string(middleware.ContextKeyUserRole), service.RoleChannelAdmin)
		case "admin":
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 9})
			c.Set(string(middleware.ContextKeyUserRole), service.RoleAdmin)
		case "user":
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 11})
			c.Set(string(middleware.ContextKeyUserRole), service.RoleUser)
		case "unknown":
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 12})
			c.Set(string(middleware.ContextKeyUserRole), "super_admin")
		}
		c.Next()
	})
}

func TestAccountHandlerCheckMixedChannelNoRisk(t *testing.T) {
	adminSvc := newStubAdminService()
	router := setupAccountMixedChannelRouter(adminSvc, nil)

	body, _ := json.Marshal(map[string]any{
		"platform":  "antigravity",
		"group_ids": []int64{27},
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/check-mixed-channel", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, float64(0), resp["code"])
	data, ok := resp["data"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, false, data["has_risk"])
	require.Equal(t, int64(0), adminSvc.lastMixedCheck.accountID)
	require.Equal(t, "antigravity", adminSvc.lastMixedCheck.platform)
	require.Equal(t, []int64{27}, adminSvc.lastMixedCheck.groupIDs)
}

func TestAccountHandlerCheckMixedChannelWithRisk(t *testing.T) {
	adminSvc := newStubAdminService()
	adminSvc.checkMixedErr = &service.MixedChannelError{
		GroupID:         27,
		GroupName:       "claude-max",
		CurrentPlatform: "Antigravity",
		OtherPlatform:   "Anthropic",
	}
	router := setupAccountMixedChannelRouter(adminSvc, nil)

	body, _ := json.Marshal(map[string]any{
		"platform":   "antigravity",
		"group_ids":  []int64{27},
		"account_id": 99,
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/check-mixed-channel", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, float64(0), resp["code"])
	data, ok := resp["data"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, true, data["has_risk"])
	require.Equal(t, "mixed_channel_warning", data["error"])
	details, ok := data["details"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, float64(27), details["group_id"])
	require.Equal(t, "claude-max", details["group_name"])
	require.Equal(t, "Antigravity", details["current_platform"])
	require.Equal(t, "Anthropic", details["other_platform"])
	require.Equal(t, int64(99), adminSvc.lastMixedCheck.accountID)
}

func TestAccountHandlerCreateMixedChannelConflictSimplifiedResponse(t *testing.T) {
	adminSvc := newStubAdminService()
	adminSvc.createAccountErr = &service.MixedChannelError{
		GroupID:         27,
		GroupName:       "claude-max",
		CurrentPlatform: "Antigravity",
		OtherPlatform:   "Anthropic",
	}
	router := setupAccountMixedChannelRouter(adminSvc, nil)

	body, _ := json.Marshal(map[string]any{
		"name":        "ag-oauth-1",
		"platform":    "antigravity",
		"type":        "oauth",
		"credentials": map[string]any{"refresh_token": "rt"},
		"group_ids":   []int64{27},
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	withAdminAuth(req)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusConflict, rec.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, "mixed_channel_warning", resp["error"])
	require.Contains(t, resp["message"], "mixed_channel_warning")
	_, hasDetails := resp["details"]
	_, hasRequireConfirmation := resp["require_confirmation"]
	require.False(t, hasDetails)
	require.False(t, hasRequireConfirmation)
}

func TestAccountHandlerUpdateMixedChannelConflictSimplifiedResponse(t *testing.T) {
	adminSvc := newStubAdminService()
	adminSvc.updateAccountErr = &service.MixedChannelError{
		GroupID:         27,
		GroupName:       "claude-max",
		CurrentPlatform: "Antigravity",
		OtherPlatform:   "Anthropic",
	}
	router := setupAccountMixedChannelRouter(adminSvc, nil)

	body, _ := json.Marshal(map[string]any{
		"group_ids": []int64{27},
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/accounts/3", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	withAdminAuth(req)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusConflict, rec.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, "mixed_channel_warning", resp["error"])
	require.Contains(t, resp["message"], "mixed_channel_warning")
	_, hasDetails := resp["details"]
	_, hasRequireConfirmation := resp["require_confirmation"]
	require.False(t, hasDetails)
	require.False(t, hasRequireConfirmation)
}

func TestAccountHandlerBulkUpdateMixedChannelConflict(t *testing.T) {
	adminSvc := newStubAdminService()
	adminSvc.bulkUpdateAccountErr = &service.MixedChannelError{
		GroupID:         27,
		GroupName:       "claude-max",
		CurrentPlatform: "Antigravity",
		OtherPlatform:   "Anthropic",
	}
	router := setupAccountMixedChannelRouter(adminSvc, nil)

	body, _ := json.Marshal(map[string]any{
		"account_ids": []int64{1, 2, 3},
		"group_ids":   []int64{27},
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/bulk-update", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	withAdminAuth(req)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusConflict, rec.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, "mixed_channel_warning", resp["error"])
	require.Contains(t, resp["message"], "claude-max")
}

func TestAccountHandlerBulkUpdateMixedChannelConfirmSkips(t *testing.T) {
	adminSvc := newStubAdminService()
	router := setupAccountMixedChannelRouter(adminSvc, nil)

	body, _ := json.Marshal(map[string]any{
		"account_ids":                []int64{1, 2},
		"group_ids":                  []int64{27},
		"confirm_mixed_channel_risk": true,
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/bulk-update", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	withAdminAuth(req)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, float64(0), resp["code"])
	data, ok := resp["data"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, float64(2), data["success"])
	require.Equal(t, float64(0), data["failed"])
}

func TestBulkUpdateAcceptsFilterTargetRequest(t *testing.T) {
	adminSvc := newStubAdminService()
	router := setupAccountMixedChannelRouter(adminSvc, nil)

	body, _ := json.Marshal(map[string]any{
		"filters": map[string]any{
			"platform":     "openai",
			"type":         "oauth",
			"status":       "active",
			"group":        "12",
			"privacy_mode": "blocked",
			"search":       "bulk-target",
		},
		"schedulable": true,
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/bulk-update", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	withAdminAuth(req)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, float64(0), resp["code"])
}

type channelAdminScopeServiceStub struct {
	authorizedGroupIDsFn func(ctx context.Context, userID int64) ([]int64, error)
	canManageGroupsFn    func(ctx context.Context, userID int64, groupIDs []int64) (bool, error)
	accountInScopeFn     func(ctx context.Context, userID, accountID int64) (bool, error)
}

func (s *channelAdminScopeServiceStub) AuthorizedGroupIDs(ctx context.Context, userID int64) ([]int64, error) {
	if s.authorizedGroupIDsFn != nil {
		return s.authorizedGroupIDsFn(ctx, userID)
	}
	return nil, nil
}

func (s *channelAdminScopeServiceStub) CanManageAccountGroups(ctx context.Context, userID int64, groupIDs []int64) (bool, error) {
	if s.canManageGroupsFn != nil {
		return s.canManageGroupsFn(ctx, userID, groupIDs)
	}
	return false, nil
}

func (s *channelAdminScopeServiceStub) AccountInScope(ctx context.Context, userID, accountID int64) (bool, error) {
	if s.accountInScopeFn != nil {
		return s.accountInScopeFn(ctx, userID, accountID)
	}
	return false, nil
}

func TestAccountHandlerListChannelAdminWithNoAuthorizedGroupsIncludesUnassigned(t *testing.T) {
	adminSvc := newStubAdminService()
	adminSvc.accounts = []service.Account{{ID: 1, Name: "unassigned-account"}}
	adminSvc.listAccountsFn = func(ctx context.Context, page, pageSize int, platform, accountType, status, search string, groupID int64, privacyMode string, sortBy, sortOrder string, scopedGroupIDs ...int64) ([]service.Account, int64, error) {
		require.Empty(t, scopedGroupIDs)
		return []service.Account{{ID: 1, Name: "unassigned-account"}}, 1, nil
	}
	scopeSvc := &channelAdminScopeServiceStub{
		authorizedGroupIDsFn: func(ctx context.Context, userID int64) ([]int64, error) {
			require.Equal(t, int64(7), userID)
			return []int64{}, nil
		},
	}
	router := setupAccountMixedChannelRouter(adminSvc, scopeSvc)
	withAuthMiddleware(router)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts", nil)
	withChannelAdminAuth(req)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Empty(t, adminSvc.lastListAccounts.scopedGroupIDs)

	var resp struct {
		Code int `json:"code"`
		Data struct {
			Items []json.RawMessage `json:"items"`
			Total int64             `json:"total"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.Len(t, resp.Data.Items, 1)
	require.Equal(t, int64(1), resp.Data.Total)
	require.Contains(t, string(resp.Data.Items[0]), "unassigned-account")
}

func TestAccountHandlerCreateChannelAdminRejectsOutOfScopeGroups(t *testing.T) {
	adminSvc := newStubAdminService()
	scopeSvc := &channelAdminScopeServiceStub{
		canManageGroupsFn: func(ctx context.Context, userID int64, groupIDs []int64) (bool, error) {
			require.Equal(t, int64(7), userID)
			require.Equal(t, []int64{99}, groupIDs)
			return false, nil
		},
	}
	router := setupAccountMixedChannelRouter(adminSvc, scopeSvc)
	withAuthMiddleware(router)

	body, _ := json.Marshal(map[string]any{
		"name":        "ag-oauth-1",
		"platform":    "antigravity",
		"type":        "oauth",
		"credentials": map[string]any{"refresh_token": "rt"},
		"group_ids":   []int64{99},
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	withChannelAdminAuth(req)
	router.ServeHTTP(rec, req)

	var resp response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Equal(t, "Forbidden", resp.Message)
	require.Equal(t, "FORBIDDEN", resp.Reason)
	require.Empty(t, adminSvc.createdAccounts)
}

func TestAccountHandlerCreateChannelAdminAllowsEmptyGroupIDs(t *testing.T) {
	adminSvc := newStubAdminService()
	adminSvc.groupsByPlatform = map[string][]service.Group{
		"antigravity": {
			{ID: 55, Name: "antigravity-default", Platform: service.PlatformAntigravity, Status: service.StatusActive},
		},
	}
	scopeSvc := &channelAdminScopeServiceStub{
		canManageGroupsFn: func(ctx context.Context, userID int64, groupIDs []int64) (bool, error) {
			t.Fatalf("CanManageAccountGroups should not be called for empty create group_ids")
			return false, nil
		},
	}
	router := setupAccountMixedChannelRouter(adminSvc, scopeSvc)
	withAuthMiddleware(router)

	body, _ := json.Marshal(map[string]any{
		"name":        "ag-oauth-empty-groups",
		"platform":    "antigravity",
		"type":        "oauth",
		"credentials": map[string]any{"refresh_token": "rt"},
		"group_ids":   []int64{},
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	withChannelAdminAuth(req)
	router.ServeHTTP(rec, req)

	var resp response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, 0, resp.Code)
	require.Len(t, adminSvc.createdAccounts, 1)
	require.Empty(t, adminSvc.createdAccounts[0].GroupIDs)
}


func TestAccountHandlerBatchCreateChannelAdminAllowsEmptyGroupIDsPerItem(t *testing.T) {
	adminSvc := newStubAdminService()
	calls := 0
	scopeSvc := &channelAdminScopeServiceStub{
		canManageGroupsFn: func(ctx context.Context, userID int64, groupIDs []int64) (bool, error) {
			calls++
			require.Equal(t, int64(7), userID)
			require.Equal(t, []int64{88}, groupIDs)
			return true, nil
		},
	}
	router := setupAccountMixedChannelRouter(adminSvc, scopeSvc)
	withAuthMiddleware(router)

	body, _ := json.Marshal(map[string]any{
		"accounts": []map[string]any{
			{
				"name":        "missing-groups",
				"platform":    "antigravity",
				"type":        "oauth",
				"credentials": map[string]any{"refresh_token": "rt-1"},
				"group_ids":   []int64{},
			},
			{
				"name":        "in-scope",
				"platform":    "antigravity",
				"type":        "oauth",
				"credentials": map[string]any{"refresh_token": "rt-2"},
				"group_ids":   []int64{88},
			},
		},
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/batch", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	withChannelAdminAuth(req)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, adminSvc.createdAccounts, 2)
	require.Empty(t, adminSvc.createdAccounts[0].GroupIDs)
	require.Equal(t, []int64{88}, adminSvc.createdAccounts[1].GroupIDs)
	require.Equal(t, 1, calls)

	var resp struct {
		Code int `json:"code"`
		Data struct {
			Success int `json:"success"`
			Failed  int `json:"failed"`
			Results []struct {
				Name    string `json:"name"`
				ID      int64  `json:"id"`
				Success bool   `json:"success"`
				Error   string `json:"error"`
			} `json:"results"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.Equal(t, 2, resp.Data.Success)
	require.Equal(t, 0, resp.Data.Failed)
	require.Len(t, resp.Data.Results, 2)
	require.Equal(t, "missing-groups", resp.Data.Results[0].Name)
	require.True(t, resp.Data.Results[0].Success)
	require.Equal(t, "in-scope", resp.Data.Results[1].Name)
	require.True(t, resp.Data.Results[1].Success)
}


func TestAccountHandlerBatchCreateChannelAdminRejectsOutOfScopeGroupIDsPerItem(t *testing.T) {
	adminSvc := newStubAdminService()
	scopeSvc := &channelAdminScopeServiceStub{
		canManageGroupsFn: func(ctx context.Context, userID int64, groupIDs []int64) (bool, error) {
			require.Equal(t, int64(7), userID)
			switch {
			case len(groupIDs) == 1 && groupIDs[0] == 99:
				return false, nil
			case len(groupIDs) == 1 && groupIDs[0] == 88:
				return true, nil
			default:
				t.Fatalf("unexpected groupIDs %v", groupIDs)
				return false, nil
			}
		},
	}
	router := setupAccountMixedChannelRouter(adminSvc, scopeSvc)
	withAuthMiddleware(router)

	body, _ := json.Marshal(map[string]any{
		"accounts": []map[string]any{
			{
				"name":        "out-of-scope",
				"platform":    "antigravity",
				"type":        "oauth",
				"credentials": map[string]any{"refresh_token": "rt-1"},
				"group_ids":   []int64{99},
			},
			{
				"name":        "in-scope",
				"platform":    "antigravity",
				"type":        "oauth",
				"credentials": map[string]any{"refresh_token": "rt-2"},
				"group_ids":   []int64{88},
			},
		},
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/batch", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	withChannelAdminAuth(req)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, adminSvc.createdAccounts, 1)
	require.Equal(t, "in-scope", adminSvc.createdAccounts[0].Name)
	require.Equal(t, []int64{88}, adminSvc.createdAccounts[0].GroupIDs)

	var resp struct {
		Code int `json:"code"`
		Data struct {
			Success int `json:"success"`
			Failed  int `json:"failed"`
			Results []struct {
				Name    string `json:"name"`
				Success bool   `json:"success"`
				Error   string `json:"error"`
			} `json:"results"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.Equal(t, 1, resp.Data.Success)
	require.Equal(t, 1, resp.Data.Failed)
	require.Len(t, resp.Data.Results, 2)
	require.Equal(t, "out-of-scope", resp.Data.Results[0].Name)
	require.False(t, resp.Data.Results[0].Success)
	require.Contains(t, resp.Data.Results[0].Error, "Forbidden")
	require.Equal(t, "in-scope", resp.Data.Results[1].Name)
	require.True(t, resp.Data.Results[1].Success)
}

func TestAccountHandlerGetByIDChannelAdminRejectsOutOfScopeAccount(t *testing.T) {
	adminSvc := newStubAdminService()
	scopeSvc := &channelAdminScopeServiceStub{
		accountInScopeFn: func(ctx context.Context, userID, accountID int64) (bool, error) {
			require.Equal(t, int64(7), userID)
			require.Equal(t, int64(42), accountID)
			return false, nil
		},
	}
	router := setupAccountMixedChannelRouter(adminSvc, scopeSvc)
	withAuthMiddleware(router)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts/42", nil)
	withChannelAdminAuth(req)
	router.ServeHTTP(rec, req)

	var resp response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Equal(t, "Forbidden", resp.Message)
	require.Equal(t, "FORBIDDEN", resp.Reason)
}

func TestAccountHandlerUpdateChannelAdminRejectsOutOfScopeAccount(t *testing.T) {
	adminSvc := newStubAdminService()
	scopeSvc := &channelAdminScopeServiceStub{
		accountInScopeFn: func(ctx context.Context, userID, accountID int64) (bool, error) {
			require.Equal(t, int64(7), userID)
			require.Equal(t, int64(42), accountID)
			return false, nil
		},
	}
	router := setupAccountMixedChannelRouter(adminSvc, scopeSvc)
	withAuthMiddleware(router)

	body, _ := json.Marshal(map[string]any{"name": "updated"})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/accounts/42", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	withChannelAdminAuth(req)
	router.ServeHTTP(rec, req)

	var resp response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Equal(t, "Forbidden", resp.Message)
	require.Equal(t, "FORBIDDEN", resp.Reason)
}

func TestAccountHandlerUpdateChannelAdminRejectsOutOfScopeGroupIDs(t *testing.T) {
	adminSvc := newStubAdminService()
	scopeSvc := &channelAdminScopeServiceStub{
		accountInScopeFn: func(ctx context.Context, userID, accountID int64) (bool, error) {
			require.Equal(t, int64(7), userID)
			require.Equal(t, int64(42), accountID)
			return true, nil
		},
		canManageGroupsFn: func(ctx context.Context, userID int64, groupIDs []int64) (bool, error) {
			require.Equal(t, int64(7), userID)
			require.Equal(t, []int64{77}, groupIDs)
			return false, nil
		},
	}
	router := setupAccountMixedChannelRouter(adminSvc, scopeSvc)
	withAuthMiddleware(router)

	body, _ := json.Marshal(map[string]any{"group_ids": []int64{77}})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/accounts/42", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	withChannelAdminAuth(req)
	router.ServeHTTP(rec, req)

	var resp response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Equal(t, "Forbidden", resp.Message)
	require.Equal(t, "FORBIDDEN", resp.Reason)
}

func TestAccountHandlerUpdateChannelAdminAllowsEmptyGroupIDs(t *testing.T) {
	adminSvc := newStubAdminService()
	scopeSvc := &channelAdminScopeServiceStub{
		accountInScopeFn: func(ctx context.Context, userID, accountID int64) (bool, error) {
			require.Equal(t, int64(7), userID)
			require.Equal(t, int64(42), accountID)
			return true, nil
		},
		canManageGroupsFn: func(ctx context.Context, userID int64, groupIDs []int64) (bool, error) {
			t.Fatalf("CanManageAccountGroups should not be called for empty update group_ids")
			return false, nil
		},
	}
	router := setupAccountMixedChannelRouter(adminSvc, scopeSvc)
	withAuthMiddleware(router)

	body, _ := json.Marshal(map[string]any{"group_ids": []int64{}})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/accounts/42", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	withChannelAdminAuth(req)
	router.ServeHTTP(rec, req)

	var resp response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, 0, resp.Code)
	require.Len(t, adminSvc.updatedAccounts, 1)
	require.Empty(t, adminSvc.updatedAccounts[0].GroupIDs)
}


func TestAccountHandlerBulkUpdateChannelAdminAllowsEmptyGroupIDs(t *testing.T) {
	adminSvc := newStubAdminService()
	scopeSvc := &channelAdminScopeServiceStub{
		canManageGroupsFn: func(ctx context.Context, userID int64, groupIDs []int64) (bool, error) {
			t.Fatalf("CanManageAccountGroups should not be called for empty bulk update group_ids")
			return false, nil
		},
	}
	router := setupAccountMixedChannelRouter(adminSvc, scopeSvc)
	withAuthMiddleware(router)

	body, _ := json.Marshal(map[string]any{
		"account_ids": []int64{1, 2},
		"group_ids":   []int64{},
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/bulk-update", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	withChannelAdminAuth(req)
	router.ServeHTTP(rec, req)

	var resp response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, 0, resp.Code)
	require.NotNil(t, adminSvc.lastBulkUpdateAccounts)
	require.Empty(t, adminSvc.lastBulkUpdateAccounts.GroupIDs)
}


func TestAccountHandlerBulkUpdateChannelAdminRejectsExplicitOutOfScopeAccountIDs(t *testing.T) {
	adminSvc := newStubAdminService()
	scopeSvc := &channelAdminScopeServiceStub{
		accountInScopeFn: func(ctx context.Context, userID, accountID int64) (bool, error) {
			require.Equal(t, int64(7), userID)
			switch accountID {
			case 1:
				return true, nil
			case 2:
				return false, nil
			default:
				t.Fatalf("unexpected accountID %d", accountID)
				return false, nil
			}
		},
	}
	router := setupAccountMixedChannelRouter(adminSvc, scopeSvc)
	withAuthMiddleware(router)

	body, _ := json.Marshal(map[string]any{
		"account_ids": []int64{1, 2},
		"status":      "inactive",
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/bulk-update", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	withChannelAdminAuth(req)
	router.ServeHTTP(rec, req)

	var resp response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Equal(t, "Forbidden", resp.Message)
	require.Equal(t, "FORBIDDEN", resp.Reason)
	require.Nil(t, adminSvc.lastBulkUpdateAccounts)
}

func TestAccountHandlerDeleteChannelAdminRejectsOutOfScopeAccount(t *testing.T) {
	adminSvc := newStubAdminService()
	scopeSvc := &channelAdminScopeServiceStub{
		accountInScopeFn: func(ctx context.Context, userID, accountID int64) (bool, error) {
			require.Equal(t, int64(7), userID)
			require.Equal(t, int64(42), accountID)
			return false, nil
		},
	}
	router := setupAccountMixedChannelRouter(adminSvc, scopeSvc)
	withAuthMiddleware(router)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/accounts/42", nil)
	withChannelAdminAuth(req)
	router.ServeHTTP(rec, req)

	var resp response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Equal(t, "Forbidden", resp.Message)
	require.Equal(t, "FORBIDDEN", resp.Reason)
}

func TestAccountHandlerBulkUpdateChannelAdminRejectsOutOfScopeGroupIDs(t *testing.T) {
	adminSvc := newStubAdminService()
	scopeSvc := &channelAdminScopeServiceStub{
		canManageGroupsFn: func(ctx context.Context, userID int64, groupIDs []int64) (bool, error) {
			require.Equal(t, int64(7), userID)
			require.Equal(t, []int64{77}, groupIDs)
			return false, nil
		},
	}
	router := setupAccountMixedChannelRouter(adminSvc, scopeSvc)
	withAuthMiddleware(router)

	body, _ := json.Marshal(map[string]any{
		"account_ids": []int64{1, 2},
		"group_ids":   []int64{77},
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/bulk-update", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	withChannelAdminAuth(req)
	router.ServeHTTP(rec, req)

	var resp response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Equal(t, "Forbidden", resp.Message)
	require.Equal(t, "FORBIDDEN", resp.Reason)
	require.Nil(t, adminSvc.lastBulkUpdateAccounts)
}

func TestAccountHandlerBulkUpdateChannelAdminScopesFilterTargets(t *testing.T) {
	adminSvc := newStubAdminService()
	scopeSvc := &channelAdminScopeServiceStub{
		authorizedGroupIDsFn: func(ctx context.Context, userID int64) ([]int64, error) {
			require.Equal(t, int64(7), userID)
			return []int64{3, 5}, nil
		},
	}
	router := setupAccountMixedChannelRouter(adminSvc, scopeSvc)
	withAuthMiddleware(router)

	body, _ := json.Marshal(map[string]any{
		"filters": map[string]any{
			"platform": "openai",
		},
		"schedulable": true,
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/bulk-update", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	withChannelAdminAuth(req)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, adminSvc.lastBulkUpdateAccounts)
	require.Equal(t, []int64{3, 5}, adminSvc.lastBulkUpdateAccounts.ScopedGroupIDs)
}

func TestAccountHandlerScopedRoutesRejectInvalidRoles(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		authHeader string
		method     string
		path       string
		body       string
		wantCode   int
		wantMsg    string
		wantReason string
	}{
		{name: "list missing context", method: http.MethodGet, path: "/api/v1/admin/accounts", wantCode: http.StatusForbidden, wantMsg: "Forbidden", wantReason: "FORBIDDEN"},
		{name: "list unknown role", authHeader: "unknown", method: http.MethodGet, path: "/api/v1/admin/accounts", wantCode: http.StatusForbidden, wantMsg: "Forbidden", wantReason: "FORBIDDEN"},
		{name: "list normal user role", authHeader: "user", method: http.MethodGet, path: "/api/v1/admin/accounts", wantCode: http.StatusForbidden, wantMsg: "Forbidden", wantReason: "FORBIDDEN"},
		{name: "get missing role", method: http.MethodGet, path: "/api/v1/admin/accounts/42", wantCode: http.StatusForbidden, wantMsg: "Forbidden", wantReason: "FORBIDDEN"},
		{name: "create normal user role", authHeader: "user", method: http.MethodPost, path: "/api/v1/admin/accounts", body: `{"name":"acc","platform":"openai","type":"oauth","credentials":{"token":"x"}}`, wantCode: http.StatusForbidden, wantMsg: "Forbidden", wantReason: "FORBIDDEN"},
		{name: "update unknown role", authHeader: "unknown", method: http.MethodPut, path: "/api/v1/admin/accounts/42", body: `{"name":"updated"}`, wantCode: http.StatusForbidden, wantMsg: "Forbidden", wantReason: "FORBIDDEN"},
		{name: "delete normal user role", authHeader: "user", method: http.MethodDelete, path: "/api/v1/admin/accounts/42", wantCode: http.StatusForbidden, wantMsg: "Forbidden", wantReason: "FORBIDDEN"},
		{name: "bulk update unknown role", authHeader: "unknown", method: http.MethodPost, path: "/api/v1/admin/accounts/bulk-update", body: `{"account_ids":[1],"status":"inactive"}`, wantCode: http.StatusForbidden, wantMsg: "Forbidden", wantReason: "FORBIDDEN"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adminSvc := newStubAdminService()
			router := setupAccountMixedChannelRouter(adminSvc, nil)
			withAuthMiddleware(router)

			var body *bytes.Reader
			if tt.body != "" {
				body = bytes.NewReader([]byte(tt.body))
			} else {
				body = bytes.NewReader(nil)
			}
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(tt.method, tt.path, body)
			if tt.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			if tt.authHeader != "" {
				req.Header.Set("X-Test-Auth", tt.authHeader)
			}
			router.ServeHTTP(rec, req)

			var resp response.Response
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
			require.Equal(t, tt.wantCode, rec.Code)
			require.Equal(t, tt.wantCode, resp.Code)
			require.Equal(t, tt.wantMsg, resp.Message)
			require.Equal(t, tt.wantReason, resp.Reason)
		})
	}
}
