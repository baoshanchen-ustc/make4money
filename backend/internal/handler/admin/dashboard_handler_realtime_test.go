package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type dashboardRealtimeUsageRepoStub struct {
	service.UsageLogRepository
	stats *usagestats.DashboardStats
}

func (s *dashboardRealtimeUsageRepoStub) GetDashboardStats(ctx context.Context) (*usagestats.DashboardStats, error) {
	if s.stats != nil {
		return s.stats, nil
	}
	return &usagestats.DashboardStats{}, nil
}

func TestDashboardHandler_GetRealtimeMetrics_UsesRuntimeSnapshots(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &dashboardRealtimeUsageRepoStub{
		stats: &usagestats.DashboardStats{
			Rpm:               123,
			AverageDurationMs: 45.5,
		},
	}
	handler := NewDashboardHandler(service.NewDashboardService(repo, nil, nil, nil), nil)
	router := gin.New()
	router.GET("/admin/dashboard/realtime", handler.GetRealtimeMetrics)

	req := httptest.NewRequest(http.MethodGet, "/admin/dashboard/realtime", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Code int            `json:"code"`
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.Equal(t, float64(123), resp.Data["requests_per_minute"])
	require.Equal(t, 45.5, resp.Data["average_response_time"])
	require.Contains(t, resp.Data, "redis_pool")
	require.Contains(t, resp.Data, "usage_log_not_persisted")
	require.Contains(t, resp.Data, "billing_compensation")
	require.Contains(t, resp.Data, "scheduler_outbox")
	require.Contains(t, resp.Data, "usage_record_worker_pool")
	require.Contains(t, resp.Data, "gateway_hotpath")
	require.Contains(t, resp.Data, "cleanup_status")
	require.Contains(t, resp.Data, "usage_cleanup_status")
	billing, ok := resp.Data["billing_compensation"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, float64(0), billing["total"])
	require.Contains(t, billing, "details")
	require.Contains(t, billing, "snapshot")
	require.IsType(t, []any{}, billing["details"])
	fallback, ok := billing["fallback"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "snapshot_empty", fallback["reason"])
	require.Contains(t, fallback["recent_logs"], service.OpsRuntimeBillingCompensationComponent)
	hint, ok := billing["ops_search"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "/api/v1/admin/ops/billing-compensation", hint["endpoint"])
	require.NotEmpty(t, hint["note"])
	require.Equal(t, "/api/v1/admin/ops/billing-compensation/:request_id", hint["detail_endpoint_template"])
	_, hasBillingLastDetail := hint["last_detail_endpoint"]
	require.False(t, hasBillingLastDetail)
	redisPool, ok := resp.Data["redis_pool"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, float64(0), redisPool["hits"])
	require.Equal(t, float64(0), redisPool["timeouts"])
	require.Equal(t, float64(0), redisPool["misses"])
	require.Equal(t, float64(0), redisPool["stalls"])
	require.Equal(t, float64(0), redisPool["total_conns"])
	require.Equal(t, float64(0), redisPool["idle_conns"])
	worker, ok := resp.Data["usage_record_worker_pool"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, float64(0), worker["task_timeouts"])
	require.Equal(t, float64(0), worker["task_panics"])
	usage, ok := resp.Data["usage_log_not_persisted"].(map[string]any)
	require.True(t, ok)
	info, ok := usage["ops_search"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "/api/v1/admin/ops/usage-log-not-persisted", info["endpoint"])
	require.NotEmpty(t, info["note"])
	require.Equal(t, "/api/v1/admin/ops/usage-log-not-persisted/:request_id", info["detail_endpoint_template"])
	_, hasUsageLastDetail := info["last_detail_endpoint"]
	require.False(t, hasUsageLastDetail)
}
