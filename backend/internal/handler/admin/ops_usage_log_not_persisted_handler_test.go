package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type opsUsageLogRepoStub struct {
	service.OpsRepository
	logs []*service.OpsSystemLog
}

func (s *opsUsageLogRepoStub) ListSystemLogs(ctx context.Context, filter *service.OpsSystemLogFilter) (*service.OpsSystemLogList, error) {
	match := func(log *service.OpsSystemLog) bool {
		if log == nil {
			return false
		}
		if filter.Component != "" && log.Component != filter.Component {
			return false
		}
		if filter.ResolvedRequestID != "" {
			requestID := strings.TrimSpace(log.RequestID)
			if requestID == "" {
				requestID = strings.TrimSpace(usageLogStringFromExtra(log.Extra, "request_id"))
			}
			if requestID != filter.ResolvedRequestID {
				return false
			}
		}
		if filter.ExtraAPIKeyID != nil {
			value, ok := usageLogInt64FromExtra(log.Extra, "api_key_id")
			if !ok || value != *filter.ExtraAPIKeyID {
				return false
			}
		}
		if filter.ExtraGroupID != nil {
			value, ok := usageLogInt64FromExtra(log.Extra, "group_id")
			if !ok || value != *filter.ExtraGroupID {
				return false
			}
		}
		return true
	}

	filtered := make([]*service.OpsSystemLog, 0, len(s.logs))
	for _, item := range s.logs {
		if match(item) {
			filtered = append(filtered, item)
		}
	}

	page := filter.Page
	if page <= 0 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize <= 0 {
		pageSize = 50
	}
	start := (page - 1) * pageSize
	if start > len(filtered) {
		start = len(filtered)
	}
	end := start + pageSize
	if end > len(filtered) {
		end = len(filtered)
	}

	return &service.OpsSystemLogList{
		Logs:     filtered[start:end],
		Total:    len(filtered),
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func usageLogStringFromExtra(extra map[string]any, key string) string {
	if extra == nil || key == "" {
		return ""
	}
	if value, ok := extra[key]; ok {
		if str, ok := value.(string); ok {
			return str
		}
	}
	if lastRaw, ok := extra["last"]; ok {
		if last, ok := lastRaw.(map[string]any); ok {
			if value, ok := last[key]; ok {
				if str, ok := value.(string); ok {
					return str
				}
			}
		}
	}
	return ""
}

func usageLogInt64FromExtra(extra map[string]any, key string) (int64, bool) {
	if extra == nil || key == "" {
		return 0, false
	}
	if value, ok := extra[key]; ok {
		if id, ok := coerceInt64(value); ok {
			return id, true
		}
	}
	if lastRaw, ok := extra["last"]; ok {
		if last, ok := lastRaw.(map[string]any); ok {
			if value, ok := last[key]; ok {
				return coerceInt64(value)
			}
		}
	}
	return 0, false
}

func TestOpsUsageLogNotPersistedHandler_ListAndFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now().UTC()
	repo := &opsUsageLogRepoStub{
		logs: []*service.OpsSystemLog{
			{
				ID:        1,
				Component: service.OpsRuntimeUsageLogComponent,
				Message:   "persist usage failed",
				CreatedAt: now,
				RequestID: "req-usage",
				Extra: map[string]any{
					"request_id": "req-usage",
					"api_key_id": int64(11),
					"group_id":   int64(22),
					"error":      "usage error",
					"last": map[string]any{
						"request_id": "req-usage",
						"account_id": int64(33),
						"error":      "usage error",
					},
				},
			},
			{
				ID:        2,
				Component: service.OpsRuntimeUsageLogComponent,
				Message:   "another",
				CreatedAt: now.Add(-time.Minute),
				RequestID: "req-other",
			},
		},
	}
	opsSvc := service.NewOpsService(repo, nil, &config.Config{Ops: config.OpsConfig{Enabled: true}}, nil, nil, nil, nil, nil, nil, nil, nil)
	handler := NewOpsHandler(opsSvc)
	router := gin.New()
	router.GET("/admin/ops/usage-log-not-persisted", handler.ListUsageLogNotPersisted)

	req := httptest.NewRequest(http.MethodGet, "/admin/ops/usage-log-not-persisted?request_id=req-usage&api_key_id=11&group_id=22", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var envelope response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))

	raw, err := json.Marshal(envelope.Data)
	require.NoError(t, err)

	var payload struct {
		Items    []map[string]any `json:"items"`
		Total    int              `json:"total"`
		RawTotal int              `json:"raw_total"`
		Filters  map[string]any   `json:"filters"`
	}
	require.NoError(t, json.Unmarshal(raw, &payload))
	require.Equal(t, 1, payload.Total)
	require.Equal(t, 1, payload.RawTotal)
	require.Len(t, payload.Items, 1)
	require.Equal(t, "req-usage", payload.Items[0]["request_id"])
	require.Equal(t, "usage_alert", payload.Items[0]["kind"])
	require.Equal(t, float64(11), payload.Items[0]["api_key_id"])
	require.Equal(t, float64(22), payload.Items[0]["group_id"])
	require.Contains(t, payload.Items[0], "manual_hint")
	require.Equal(t, "req-usage", payload.Filters["request_id"])
}

func TestOpsUsageLogNotPersistedHandler_ListPushesExtraFiltersIntoQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now().UTC()
	repo := &opsUsageLogRepoStub{
		logs: []*service.OpsSystemLog{
			{
				ID:        1,
				Component: service.OpsRuntimeUsageLogComponent,
				Message:   "unmatched",
				CreatedAt: now,
				RequestID: "req-unmatched",
				Extra: map[string]any{
					"api_key_id": int64(10),
				},
			},
			{
				ID:        2,
				Component: service.OpsRuntimeUsageLogComponent,
				Message:   "matched",
				CreatedAt: now.Add(-time.Minute),
				RequestID: "req-match",
				Extra: map[string]any{
					"api_key_id": int64(11),
					"group_id":   int64(22),
				},
			},
		},
	}
	opsSvc := service.NewOpsService(repo, nil, &config.Config{Ops: config.OpsConfig{Enabled: true}}, nil, nil, nil, nil, nil, nil, nil, nil)
	handler := NewOpsHandler(opsSvc)
	router := gin.New()
	router.GET("/admin/ops/usage-log-not-persisted", handler.ListUsageLogNotPersisted)

	req := httptest.NewRequest(http.MethodGet, "/admin/ops/usage-log-not-persisted?page=1&page_size=1&api_key_id=11&group_id=22", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var envelope response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	raw, err := json.Marshal(envelope.Data)
	require.NoError(t, err)

	var payload struct {
		Items []map[string]any `json:"items"`
		Total int              `json:"total"`
	}
	require.NoError(t, json.Unmarshal(raw, &payload))
	require.Equal(t, 1, payload.Total)
	require.Len(t, payload.Items, 1)
	require.Equal(t, "req-match", payload.Items[0]["request_id"])
}

func TestDescribeUsageLogEntry_FallsBackToLastFields(t *testing.T) {
	now := time.Now().UTC()
	entry := describeUsageLogEntry(&service.OpsSystemLog{
		ID:        9,
		Component: service.OpsRuntimeUsageLogComponent,
		Message:   "usage fallback",
		CreatedAt: now,
		RequestID: "req-fallback",
		Extra: map[string]any{
			"last": map[string]any{
				"request_id":      "req-fallback",
				"api_key_id":      int64(101),
				"group_id":        int64(202),
				"requested_model": "gpt-4.1",
				"error":           "sync fallback failed",
				"account_id":      int64(303),
			},
		},
	})

	require.NotNil(t, entry)
	require.Equal(t, "usage_alert", entry["kind"])
	require.Equal(t, int64(101), entry["api_key_id"])
	require.Equal(t, int64(202), entry["group_id"])
	require.Equal(t, "gpt-4.1", entry["requested_model"])
	require.Equal(t, "sync fallback failed", entry["error"])
	hint, ok := entry["manual_hint"].(gin.H)
	require.True(t, ok)
	require.Equal(t, "req-fallback", hint["request_id"])
	require.Equal(t, int64(303), hint["account_id"])
}

func TestOpsUsageLogNotPersistedHandler_InvalidApiKeyID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	opsSvc := service.NewOpsService(nil, nil, &config.Config{Ops: config.OpsConfig{Enabled: true}}, nil, nil, nil, nil, nil, nil, nil, nil)
	handler := NewOpsHandler(opsSvc)
	router := gin.New()
	router.GET("/admin/ops/usage-log-not-persisted", handler.ListUsageLogNotPersisted)

	req := httptest.NewRequest(http.MethodGet, "/admin/ops/usage-log-not-persisted?api_key_id=abc", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestOpsUsageLogNotPersistedHandler_InvalidGroupID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	opsSvc := service.NewOpsService(nil, nil, &config.Config{Ops: config.OpsConfig{Enabled: true}}, nil, nil, nil, nil, nil, nil, nil, nil)
	handler := NewOpsHandler(opsSvc)
	router := gin.New()
	router.GET("/admin/ops/usage-log-not-persisted", handler.ListUsageLogNotPersisted)

	req := httptest.NewRequest(http.MethodGet, "/admin/ops/usage-log-not-persisted?group_id=abc", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestOpsUsageLogNotPersistedHandler_NonPositiveAPIKeyID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	opsSvc := service.NewOpsService(nil, nil, &config.Config{Ops: config.OpsConfig{Enabled: true}}, nil, nil, nil, nil, nil, nil, nil, nil)
	handler := NewOpsHandler(opsSvc)
	router := gin.New()
	router.GET("/admin/ops/usage-log-not-persisted", handler.ListUsageLogNotPersisted)

	req := httptest.NewRequest(http.MethodGet, "/admin/ops/usage-log-not-persisted?api_key_id=0", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestOpsUsageLogNotPersistedHandler_NegativeAPIKeyID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	opsSvc := service.NewOpsService(nil, nil, &config.Config{Ops: config.OpsConfig{Enabled: true}}, nil, nil, nil, nil, nil, nil, nil, nil)
	handler := NewOpsHandler(opsSvc)
	router := gin.New()
	router.GET("/admin/ops/usage-log-not-persisted", handler.ListUsageLogNotPersisted)

	req := httptest.NewRequest(http.MethodGet, "/admin/ops/usage-log-not-persisted?api_key_id=-3", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestOpsUsageLogNotPersistedHandler_NonPositiveGroupID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	opsSvc := service.NewOpsService(nil, nil, &config.Config{Ops: config.OpsConfig{Enabled: true}}, nil, nil, nil, nil, nil, nil, nil, nil)
	handler := NewOpsHandler(opsSvc)
	router := gin.New()
	router.GET("/admin/ops/usage-log-not-persisted", handler.ListUsageLogNotPersisted)

	req := httptest.NewRequest(http.MethodGet, "/admin/ops/usage-log-not-persisted?group_id=0", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestOpsUsageLogNotPersistedHandler_NegativeGroupID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	opsSvc := service.NewOpsService(nil, nil, &config.Config{Ops: config.OpsConfig{Enabled: true}}, nil, nil, nil, nil, nil, nil, nil, nil)
	handler := NewOpsHandler(opsSvc)
	router := gin.New()
	router.GET("/admin/ops/usage-log-not-persisted", handler.ListUsageLogNotPersisted)

	req := httptest.NewRequest(http.MethodGet, "/admin/ops/usage-log-not-persisted?group_id=-3", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestOpsUsageLogNotPersistedHandler_GetDetail(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now().UTC()
	repo := &opsUsageLogRepoStub{
		logs: []*service.OpsSystemLog{
			{
				ID:        1,
				Component: service.OpsRuntimeUsageLogComponent,
				Message:   "detail usage failed",
				CreatedAt: now,
				RequestID: "detail-req",
				Extra: map[string]any{
					"request_id": "detail-req",
					"api_key_id": int64(11),
					"group_id":   int64(22),
					"last": map[string]any{
						"request_id": "detail-req",
					},
				},
			},
		},
	}
	opsSvc := service.NewOpsService(repo, nil, &config.Config{Ops: config.OpsConfig{Enabled: true}}, nil, nil, nil, nil, nil, nil, nil, nil)
	handler := NewOpsHandler(opsSvc)
	router := gin.New()
	router.GET("/admin/ops/usage-log-not-persisted/:request_id", handler.GetUsageLogNotPersistedDetail)

	req := httptest.NewRequest(http.MethodGet, "/admin/ops/usage-log-not-persisted/detail-req?api_key_id=11", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var envelope response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	raw, err := json.Marshal(envelope.Data)
	require.NoError(t, err)
	var payload struct {
		Entry   map[string]any `json:"entry"`
		Count   int            `json:"count"`
		Filters map[string]any `json:"filters"`
	}
	require.NoError(t, json.Unmarshal(raw, &payload))
	require.Equal(t, 1, payload.Count)
	require.Equal(t, "detail-req", payload.Entry["request_id"])
	require.Contains(t, payload.Entry, "manual_hint")
	require.Equal(t, "detail-req", payload.Filters["request_id"])
	require.Equal(t, float64(11), payload.Filters["api_key_id"])
	require.Nil(t, payload.Filters["group_id"])
}

func TestOpsUsageLogNotPersistedHandler_GetDetailMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	opsSvc := service.NewOpsService(nil, nil, &config.Config{Ops: config.OpsConfig{Enabled: true}}, nil, nil, nil, nil, nil, nil, nil, nil)
	handler := NewOpsHandler(opsSvc)
	router := gin.New()
	router.GET("/admin/ops/usage-log-not-persisted/:request_id", handler.GetUsageLogNotPersistedDetail)

	req := httptest.NewRequest(http.MethodGet, "/admin/ops/usage-log-not-persisted/%20", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestOpsUsageLogNotPersistedHandler_GetDetailNotFoundByFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now().UTC()
	repo := &opsUsageLogRepoStub{
		logs: []*service.OpsSystemLog{
			{
				ID:        17,
				Component: service.OpsRuntimeUsageLogComponent,
				Message:   "only",
				CreatedAt: now,
				RequestID: "usage-req",
				Extra: map[string]any{
					"request_id": "usage-req",
					"group_id":   int64(11),
				},
			},
		},
	}
	opsSvc := service.NewOpsService(repo, nil, &config.Config{Ops: config.OpsConfig{Enabled: true}}, nil, nil, nil, nil, nil, nil, nil, nil)
	handler := NewOpsHandler(opsSvc)
	router := gin.New()
	router.GET("/admin/ops/usage-log-not-persisted/:request_id", handler.GetUsageLogNotPersistedDetail)

	req := httptest.NewRequest(http.MethodGet, "/admin/ops/usage-log-not-persisted/usage-req?group_id=22", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestOpsUsageLogNotPersistedHandler_DetailInvalidFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	opsSvc := service.NewOpsService(nil, nil, &config.Config{Ops: config.OpsConfig{Enabled: true}}, nil, nil, nil, nil, nil, nil, nil, nil)
	handler := NewOpsHandler(opsSvc)
	router := gin.New()
	router.GET("/admin/ops/usage-log-not-persisted/:request_id", handler.GetUsageLogNotPersistedDetail)

	req := httptest.NewRequest(http.MethodGet, "/admin/ops/usage-log-not-persisted/detail-req?group_id=bad", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestOpsUsageLogNotPersistedHandler_DetailInvalidAPIKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	opsSvc := service.NewOpsService(nil, nil, &config.Config{Ops: config.OpsConfig{Enabled: true}}, nil, nil, nil, nil, nil, nil, nil, nil)
	handler := NewOpsHandler(opsSvc)
	router := gin.New()
	router.GET("/admin/ops/usage-log-not-persisted/:request_id", handler.GetUsageLogNotPersistedDetail)

	req := httptest.NewRequest(http.MethodGet, "/admin/ops/usage-log-not-persisted/detail-req?api_key_id=bad", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestOpsUsageLogNotPersistedHandler_DetailNonPositiveAPIKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	opsSvc := service.NewOpsService(nil, nil, &config.Config{Ops: config.OpsConfig{Enabled: true}}, nil, nil, nil, nil, nil, nil, nil, nil)
	handler := NewOpsHandler(opsSvc)
	router := gin.New()
	router.GET("/admin/ops/usage-log-not-persisted/:request_id", handler.GetUsageLogNotPersistedDetail)

	req := httptest.NewRequest(http.MethodGet, "/admin/ops/usage-log-not-persisted/detail-req?api_key_id=0", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestOpsUsageLogNotPersistedHandler_DetailNegativeAPIKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	opsSvc := service.NewOpsService(nil, nil, &config.Config{Ops: config.OpsConfig{Enabled: true}}, nil, nil, nil, nil, nil, nil, nil, nil)
	handler := NewOpsHandler(opsSvc)
	router := gin.New()
	router.GET("/admin/ops/usage-log-not-persisted/:request_id", handler.GetUsageLogNotPersistedDetail)

	req := httptest.NewRequest(http.MethodGet, "/admin/ops/usage-log-not-persisted/detail-req?api_key_id=-7", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestOpsUsageLogNotPersistedHandler_DetailZeroGroupID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	opsSvc := service.NewOpsService(nil, nil, &config.Config{Ops: config.OpsConfig{Enabled: true}}, nil, nil, nil, nil, nil, nil, nil, nil)
	handler := NewOpsHandler(opsSvc)
	router := gin.New()
	router.GET("/admin/ops/usage-log-not-persisted/:request_id", handler.GetUsageLogNotPersistedDetail)

	req := httptest.NewRequest(http.MethodGet, "/admin/ops/usage-log-not-persisted/detail-req?group_id=0", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestOpsUsageLogNotPersistedHandler_DetailNegativeGroupID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	opsSvc := service.NewOpsService(nil, nil, &config.Config{Ops: config.OpsConfig{Enabled: true}}, nil, nil, nil, nil, nil, nil, nil, nil)
	handler := NewOpsHandler(opsSvc)
	router := gin.New()
	router.GET("/admin/ops/usage-log-not-persisted/:request_id", handler.GetUsageLogNotPersistedDetail)

	req := httptest.NewRequest(http.MethodGet, "/admin/ops/usage-log-not-persisted/detail-req?group_id=-5", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}
