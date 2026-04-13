package admin

import (
	"context"
	"encoding/json"
	"fmt"
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

type opsBillingCompensationRepoStub struct {
	service.OpsRepository
	candidateLogs []*service.OpsSystemLog
	summaryLogs   []*service.OpsSystemLog
}

func (s *opsBillingCompensationRepoStub) ListSystemLogs(ctx context.Context, filter *service.OpsSystemLogFilter) (*service.OpsSystemLogList, error) {
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
				requestID = strings.TrimSpace(asStringFromExtra(log.Extra, "request_id"))
			}
			if requestID != filter.ResolvedRequestID {
				return false
			}
		}
		if filter.ExtraAPIKeyID != nil {
			value, ok := extractInt64FromExtra(log.Extra, "api_key_id")
			if !ok || value != *filter.ExtraAPIKeyID {
				return false
			}
		}
		if filter.ExtraGroupID != nil {
			value, ok := extractInt64FromExtra(log.Extra, "group_id")
			if !ok || value != *filter.ExtraGroupID {
				return false
			}
		}
		return true
	}
	selectPage := func(src []*service.OpsSystemLog) *service.OpsSystemLogList {
		filtered := make([]*service.OpsSystemLog, 0, len(src))
		for _, item := range src {
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
		}
	}
	switch filter.Component {
	case service.OpsRuntimeBillingCompensationComponent:
		return selectPage(s.candidateLogs), nil
	case service.OpsRuntimeBillingCompensationSummaryComponent:
		return selectPage(s.summaryLogs), nil
	default:
		return selectPage(nil), nil
	}
}

func asStringFromExtra(extra map[string]any, key string) string {
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

func extractInt64FromExtra(extra map[string]any, key string) (int64, bool) {
	if extra == nil || key == "" {
		return 0, false
	}
	if value, ok := extra[key]; ok {
		switch v := value.(type) {
		case int64:
			return v, true
		case float64:
			return int64(v), true
		}
	}
	if lastRaw, ok := extra["last"]; ok {
		if last, ok := lastRaw.(map[string]any); ok {
			if value, ok := last[key]; ok {
				switch v := value.(type) {
				case int64:
					return v, true
				case float64:
					return int64(v), true
				}
			}
		}
	}
	return 0, false
}

func TestOpsBillingCompensationHandler_ListAndFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now().UTC()
	repo := &opsBillingCompensationRepoStub{
		candidateLogs: []*service.OpsSystemLog{
			{
				ID:        1,
				Component: service.OpsRuntimeBillingCompensationComponent,
				Message:   "candidate one",
				CreatedAt: now,
				RequestID: "req-1",
				Extra: map[string]any{
					"request_id": "req-1",
					"api_key_id": int64(7),
					"group_id":   int64(14),
					"error":      "persist failed",
				},
			},
			{
				ID:        2,
				Component: service.OpsRuntimeBillingCompensationComponent,
				Message:   "candidate two",
				CreatedAt: now.Add(-time.Minute),
				RequestID: "req-2",
				Extra: map[string]any{
					"request_id": "req-2",
					"api_key_id": int64(8),
					"group_id":   int64(15),
				},
			},
		},
		summaryLogs: []*service.OpsSystemLog{
			{
				ID:        3,
				Component: service.OpsRuntimeBillingCompensationSummaryComponent,
				Message:   "summary",
				CreatedAt: now.Add(-2 * time.Minute),
				Extra: map[string]any{
					"delta": float64(1),
					"last": map[string]any{
						"request_id": "req-2",
						"account_id": int64(88),
						"error":      "summary error",
					},
				},
			},
		},
	}
	opsSvc := service.NewOpsService(repo, nil, &config.Config{Ops: config.OpsConfig{Enabled: true}}, nil, nil, nil, nil, nil, nil, nil, nil)
	handler := NewOpsHandler(opsSvc)
	router := gin.New()
	router.GET("/admin/ops/billing-compensation", handler.ListBillingCompensation)

	req := httptest.NewRequest(http.MethodGet, "/admin/ops/billing-compensation?request_id=req-1&api_key_id=7&group_id=14", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var envelope response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))

	raw, err := json.Marshal(envelope.Data)
	require.NoError(t, err)

	var payload struct {
		Items []map[string]any `json:"items"`
		Total int64            `json:"total"`
	}
	require.NoError(t, json.Unmarshal(raw, &payload))
	require.Equal(t, int64(1), payload.Total)
	require.Len(t, payload.Items, 1)
	require.Equal(t, "req-1", payload.Items[0]["request_id"])
	require.Equal(t, "candidate", payload.Items[0]["kind"])
	require.Equal(t, float64(7), payload.Items[0]["api_key_id"])
	require.Equal(t, float64(14), payload.Items[0]["group_id"])
}

func TestOpsBillingCompensationHandler_InvalidGroupID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	opsSvc := service.NewOpsService(nil, nil, &config.Config{Ops: config.OpsConfig{Enabled: true}}, nil, nil, nil, nil, nil, nil, nil, nil)
	handler := NewOpsHandler(opsSvc)
	router := gin.New()
	router.GET("/admin/ops/billing-compensation", handler.ListBillingCompensation)

	req := httptest.NewRequest(http.MethodGet, "/admin/ops/billing-compensation?group_id=bad", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestOpsBillingCompensationHandler_InvalidAPIKeyID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	opsSvc := service.NewOpsService(nil, nil, &config.Config{Ops: config.OpsConfig{Enabled: true}}, nil, nil, nil, nil, nil, nil, nil, nil)
	handler := NewOpsHandler(opsSvc)
	router := gin.New()
	router.GET("/admin/ops/billing-compensation", handler.ListBillingCompensation)

	req := httptest.NewRequest(http.MethodGet, "/admin/ops/billing-compensation?api_key_id=bad", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestOpsBillingCompensationHandler_NonPositiveAPIKeyID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	opsSvc := service.NewOpsService(nil, nil, &config.Config{Ops: config.OpsConfig{Enabled: true}}, nil, nil, nil, nil, nil, nil, nil, nil)
	handler := NewOpsHandler(opsSvc)
	router := gin.New()
	router.GET("/admin/ops/billing-compensation", handler.ListBillingCompensation)

	req := httptest.NewRequest(http.MethodGet, "/admin/ops/billing-compensation?api_key_id=0", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestOpsBillingCompensationHandler_NegativeAPIKeyID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	opsSvc := service.NewOpsService(nil, nil, &config.Config{Ops: config.OpsConfig{Enabled: true}}, nil, nil, nil, nil, nil, nil, nil, nil)
	handler := NewOpsHandler(opsSvc)
	router := gin.New()
	router.GET("/admin/ops/billing-compensation", handler.ListBillingCompensation)

	req := httptest.NewRequest(http.MethodGet, "/admin/ops/billing-compensation?api_key_id=-9", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestOpsBillingCompensationHandler_NonPositiveGroupID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	opsSvc := service.NewOpsService(nil, nil, &config.Config{Ops: config.OpsConfig{Enabled: true}}, nil, nil, nil, nil, nil, nil, nil, nil)
	handler := NewOpsHandler(opsSvc)
	router := gin.New()
	router.GET("/admin/ops/billing-compensation", handler.ListBillingCompensation)

	req := httptest.NewRequest(http.MethodGet, "/admin/ops/billing-compensation?group_id=0", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestOpsBillingCompensationHandler_NegativeGroupID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	opsSvc := service.NewOpsService(nil, nil, &config.Config{Ops: config.OpsConfig{Enabled: true}}, nil, nil, nil, nil, nil, nil, nil, nil)
	handler := NewOpsHandler(opsSvc)
	router := gin.New()
	router.GET("/admin/ops/billing-compensation", handler.ListBillingCompensation)

	req := httptest.NewRequest(http.MethodGet, "/admin/ops/billing-compensation?group_id=-9", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestOpsBillingCompensationHandler_Detail(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now().UTC()
	repo := &opsBillingCompensationRepoStub{
		candidateLogs: []*service.OpsSystemLog{
			{
				ID:        42,
				Component: service.OpsRuntimeBillingCompensationComponent,
				Message:   "detail candidate",
				CreatedAt: now,
				RequestID: "detail-req",
				Extra: map[string]any{
					"request_id": "detail-req",
					"api_key_id": int64(21),
				},
			},
		},
	}
	opsSvc := service.NewOpsService(repo, nil, &config.Config{Ops: config.OpsConfig{Enabled: true}}, nil, nil, nil, nil, nil, nil, nil, nil)
	handler := NewOpsHandler(opsSvc)
	router := gin.New()
	router.GET("/admin/ops/billing-compensation/:request_id", handler.GetBillingCompensationDetail)

	req := httptest.NewRequest(http.MethodGet, "/admin/ops/billing-compensation/detail-req", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var envelope response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	raw, err := json.Marshal(envelope.Data)
	require.NoError(t, err)

	var payload struct {
		RequestID string           `json:"request_id"`
		Count     int              `json:"count"`
		Items     []map[string]any `json:"items"`
		Filters   map[string]any   `json:"filters"`
	}
	require.NoError(t, json.Unmarshal(raw, &payload))
	require.Equal(t, "detail-req", payload.RequestID)
	require.Equal(t, 1, payload.Count)
	require.Equal(t, "detail-req", payload.Items[0]["request_id"])
	require.Equal(t, "candidate", payload.Items[0]["kind"])
	require.Equal(t, "detail-req", payload.Filters["request_id"])
	require.Nil(t, payload.Filters["group_id"])
}

func TestOpsBillingCompensationHandler_DetailFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now().UTC()
	repo := &opsBillingCompensationRepoStub{
		candidateLogs: []*service.OpsSystemLog{
			{
				ID:        42,
				Component: service.OpsRuntimeBillingCompensationComponent,
				Message:   "filtered candidate",
				CreatedAt: now,
				RequestID: "detail-req",
				Extra: map[string]any{
					"request_id": "detail-req",
					"api_key_id": int64(21),
					"group_id":   int64(57),
				},
			},
		},
	}
	opsSvc := service.NewOpsService(repo, nil, &config.Config{Ops: config.OpsConfig{Enabled: true}}, nil, nil, nil, nil, nil, nil, nil, nil)
	handler := NewOpsHandler(opsSvc)
	router := gin.New()
	router.GET("/admin/ops/billing-compensation/:request_id", handler.GetBillingCompensationDetail)

	req := httptest.NewRequest(http.MethodGet, "/admin/ops/billing-compensation/detail-req?group_id=57", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	var envelope response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	var payload struct {
		Filters map[string]any `json:"filters"`
	}
	raw, err := json.Marshal(envelope.Data)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &payload))
	require.Equal(t, float64(57), payload.Filters["group_id"])
}

func TestOpsBillingCompensationHandler_DetailInvalidApiKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	opsSvc := service.NewOpsService(nil, nil, &config.Config{Ops: config.OpsConfig{Enabled: true}}, nil, nil, nil, nil, nil, nil, nil, nil)
	handler := NewOpsHandler(opsSvc)
	router := gin.New()
	router.GET("/admin/ops/billing-compensation/:request_id", handler.GetBillingCompensationDetail)

	req := httptest.NewRequest(http.MethodGet, "/admin/ops/billing-compensation/detail-req?api_key_id=bad", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestOpsBillingCompensationHandler_DetailNonPositiveApiKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	opsSvc := service.NewOpsService(nil, nil, &config.Config{Ops: config.OpsConfig{Enabled: true}}, nil, nil, nil, nil, nil, nil, nil, nil)
	handler := NewOpsHandler(opsSvc)
	router := gin.New()
	router.GET("/admin/ops/billing-compensation/:request_id", handler.GetBillingCompensationDetail)

	req := httptest.NewRequest(http.MethodGet, "/admin/ops/billing-compensation/detail-req?api_key_id=0", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestOpsBillingCompensationHandler_DetailNegativeApiKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	opsSvc := service.NewOpsService(nil, nil, &config.Config{Ops: config.OpsConfig{Enabled: true}}, nil, nil, nil, nil, nil, nil, nil, nil)
	handler := NewOpsHandler(opsSvc)
	router := gin.New()
	router.GET("/admin/ops/billing-compensation/:request_id", handler.GetBillingCompensationDetail)

	req := httptest.NewRequest(http.MethodGet, "/admin/ops/billing-compensation/detail-req?api_key_id=-7", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestOpsBillingCompensationHandler_DetailInvalidGroupID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	opsSvc := service.NewOpsService(nil, nil, &config.Config{Ops: config.OpsConfig{Enabled: true}}, nil, nil, nil, nil, nil, nil, nil, nil)
	handler := NewOpsHandler(opsSvc)
	router := gin.New()
	router.GET("/admin/ops/billing-compensation/:request_id", handler.GetBillingCompensationDetail)

	req := httptest.NewRequest(http.MethodGet, "/admin/ops/billing-compensation/detail-req?group_id=bad", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestOpsBillingCompensationHandler_DetailNonPositiveGroupID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	opsSvc := service.NewOpsService(nil, nil, &config.Config{Ops: config.OpsConfig{Enabled: true}}, nil, nil, nil, nil, nil, nil, nil, nil)
	handler := NewOpsHandler(opsSvc)
	router := gin.New()
	router.GET("/admin/ops/billing-compensation/:request_id", handler.GetBillingCompensationDetail)

	req := httptest.NewRequest(http.MethodGet, "/admin/ops/billing-compensation/detail-req?group_id=0", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestOpsBillingCompensationHandler_DetailNegativeGroupID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	opsSvc := service.NewOpsService(nil, nil, &config.Config{Ops: config.OpsConfig{Enabled: true}}, nil, nil, nil, nil, nil, nil, nil, nil)
	handler := NewOpsHandler(opsSvc)
	router := gin.New()
	router.GET("/admin/ops/billing-compensation/:request_id", handler.GetBillingCompensationDetail)

	req := httptest.NewRequest(http.MethodGet, "/admin/ops/billing-compensation/detail-req?group_id=-5", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestOpsBillingCompensationHandler_DetailNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now().UTC()
	repo := &opsBillingCompensationRepoStub{
		candidateLogs: []*service.OpsSystemLog{
			{
				ID:        42,
				Component: service.OpsRuntimeBillingCompensationComponent,
				Message:   "only candidate",
				CreatedAt: now,
				RequestID: "detail-req",
				Extra: map[string]any{
					"request_id": "detail-req",
					"api_key_id": int64(21),
				},
			},
		},
	}
	opsSvc := service.NewOpsService(repo, nil, &config.Config{Ops: config.OpsConfig{Enabled: true}}, nil, nil, nil, nil, nil, nil, nil, nil)
	handler := NewOpsHandler(opsSvc)
	router := gin.New()
	router.GET("/admin/ops/billing-compensation/:request_id", handler.GetBillingCompensationDetail)

	req := httptest.NewRequest(http.MethodGet, "/admin/ops/billing-compensation/detail-req?group_id=99", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestOpsBillingCompensationHandler_ListPaginationScansBeyondFirstComponentPage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now().UTC()
	candidateLogs := make([]*service.OpsSystemLog, 0, 240)
	for i := 0; i < 240; i++ {
		candidateLogs = append(candidateLogs, &service.OpsSystemLog{
			ID:        int64(i + 1),
			Component: service.OpsRuntimeBillingCompensationComponent,
			Message:   fmt.Sprintf("candidate-%03d", i),
			CreatedAt: now.Add(-time.Duration(i) * time.Second),
			RequestID: fmt.Sprintf("req-%03d", i),
			Extra: map[string]any{
				"request_id": fmt.Sprintf("req-%03d", i),
			},
		})
	}
	repo := &opsBillingCompensationRepoStub{candidateLogs: candidateLogs}
	opsSvc := service.NewOpsService(repo, nil, &config.Config{Ops: config.OpsConfig{Enabled: true}}, nil, nil, nil, nil, nil, nil, nil, nil)
	handler := NewOpsHandler(opsSvc)
	router := gin.New()
	router.GET("/admin/ops/billing-compensation", handler.ListBillingCompensation)

	req := httptest.NewRequest(http.MethodGet, "/admin/ops/billing-compensation?page=3&page_size=100", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var envelope response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	raw, err := json.Marshal(envelope.Data)
	require.NoError(t, err)

	var payload struct {
		Items []map[string]any `json:"items"`
		Total int64            `json:"total"`
	}
	require.NoError(t, json.Unmarshal(raw, &payload))
	require.Equal(t, int64(240), payload.Total)
	require.Len(t, payload.Items, 40)
	require.Equal(t, "req-200", payload.Items[0]["request_id"])
}

func TestOpsBillingCompensationHandler_ListReportsExactTotalOnEarlyPages(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now().UTC()
	candidateLogs := make([]*service.OpsSystemLog, 0, 240)
	for i := 0; i < 240; i++ {
		candidateLogs = append(candidateLogs, &service.OpsSystemLog{
			ID:        int64(i + 1),
			Component: service.OpsRuntimeBillingCompensationComponent,
			Message:   fmt.Sprintf("candidate-%03d", i),
			CreatedAt: now.Add(-time.Duration(i) * time.Second),
			RequestID: fmt.Sprintf("req-%03d", i),
			Extra: map[string]any{
				"request_id": fmt.Sprintf("req-%03d", i),
			},
		})
	}
	repo := &opsBillingCompensationRepoStub{candidateLogs: candidateLogs}
	opsSvc := service.NewOpsService(repo, nil, &config.Config{Ops: config.OpsConfig{Enabled: true}}, nil, nil, nil, nil, nil, nil, nil, nil)
	handler := NewOpsHandler(opsSvc)
	router := gin.New()
	router.GET("/admin/ops/billing-compensation", handler.ListBillingCompensation)

	req := httptest.NewRequest(http.MethodGet, "/admin/ops/billing-compensation?page=1&page_size=50", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var envelope response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	raw, err := json.Marshal(envelope.Data)
	require.NoError(t, err)

	var payload struct {
		Items []map[string]any `json:"items"`
		Total int64            `json:"total"`
	}
	require.NoError(t, json.Unmarshal(raw, &payload))
	require.Equal(t, int64(240), payload.Total)
	require.Len(t, payload.Items, 50)
	require.Equal(t, "req-000", payload.Items[0]["request_id"])
}

func TestOpsBillingCompensationHandler_DetailScansBeyondInitialWindow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now().UTC()
	candidateLogs := make([]*service.OpsSystemLog, 0, 75)
	for i := 0; i < 75; i++ {
		requestID := fmt.Sprintf("req-%02d", i)
		candidateLogs = append(candidateLogs, &service.OpsSystemLog{
			ID:        int64(i + 1),
			Component: service.OpsRuntimeBillingCompensationComponent,
			Message:   requestID,
			CreatedAt: now.Add(-time.Duration(i) * time.Second),
			RequestID: requestID,
			Extra: map[string]any{
				"request_id": requestID,
			},
		})
	}
	repo := &opsBillingCompensationRepoStub{candidateLogs: candidateLogs}
	opsSvc := service.NewOpsService(repo, nil, &config.Config{Ops: config.OpsConfig{Enabled: true}}, nil, nil, nil, nil, nil, nil, nil, nil)
	handler := NewOpsHandler(opsSvc)
	router := gin.New()
	router.GET("/admin/ops/billing-compensation/:request_id", handler.GetBillingCompensationDetail)

	req := httptest.NewRequest(http.MethodGet, "/admin/ops/billing-compensation/req-60", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var envelope response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	raw, err := json.Marshal(envelope.Data)
	require.NoError(t, err)

	var payload struct {
		RequestID string `json:"request_id"`
		Count     int    `json:"count"`
	}
	require.NoError(t, json.Unmarshal(raw, &payload))
	require.Equal(t, "req-60", payload.RequestID)
	require.Equal(t, 1, payload.Count)
}

func TestOpsBillingCompensationHandler_ListFiltersScanBeyondInitialWindow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now().UTC()
	candidateLogs := make([]*service.OpsSystemLog, 0, 75)
	for i := 0; i < 75; i++ {
		extra := map[string]any{
			"request_id": fmt.Sprintf("req-%02d", i),
		}
		if i == 60 {
			extra["api_key_id"] = int64(777)
		}
		candidateLogs = append(candidateLogs, &service.OpsSystemLog{
			ID:        int64(i + 1),
			Component: service.OpsRuntimeBillingCompensationComponent,
			Message:   fmt.Sprintf("candidate-%02d", i),
			CreatedAt: now.Add(-time.Duration(i) * time.Second),
			RequestID: fmt.Sprintf("req-%02d", i),
			Extra:     extra,
		})
	}
	repo := &opsBillingCompensationRepoStub{candidateLogs: candidateLogs}
	opsSvc := service.NewOpsService(repo, nil, &config.Config{Ops: config.OpsConfig{Enabled: true}}, nil, nil, nil, nil, nil, nil, nil, nil)
	handler := NewOpsHandler(opsSvc)
	router := gin.New()
	router.GET("/admin/ops/billing-compensation", handler.ListBillingCompensation)

	req := httptest.NewRequest(http.MethodGet, "/admin/ops/billing-compensation?page=1&page_size=50&api_key_id=777", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var envelope response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	raw, err := json.Marshal(envelope.Data)
	require.NoError(t, err)

	var payload struct {
		Items []map[string]any `json:"items"`
		Total int64            `json:"total"`
	}
	require.NoError(t, json.Unmarshal(raw, &payload))
	require.Equal(t, int64(1), payload.Total)
	require.Len(t, payload.Items, 1)
	require.Equal(t, "req-60", payload.Items[0]["request_id"])
	require.Equal(t, float64(777), payload.Items[0]["api_key_id"])
}
