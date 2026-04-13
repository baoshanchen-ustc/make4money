package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/runtimeprobe"
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
	handler.SetOpsService(service.NewOpsService(nil, nil, &config.Config{
		Database: config.DatabaseConfig{
			MaxOpenConns:           256,
			MaxIdleConns:           128,
			ConnMaxLifetimeMinutes: 30,
			ConnMaxIdleTimeMinutes: 10,
		},
		Redis: config.RedisConfig{
			PoolSize:            4096,
			MinIdleConns:        256,
			DialTimeoutSeconds:  5,
			ReadTimeoutSeconds:  5,
			WriteTimeoutSeconds: 5,
		},
		Gateway: config.GatewayConfig{
			MaxIdleConns:              1024,
			MaxIdleConnsPerHost:       512,
			MaxConnsPerHost:           1024,
			MaxUpstreamClients:        5000,
			ClientIdleTTLSeconds:      300,
			ConcurrencySlotTTLMinutes: 15,
			SessionIdleTimeoutMinutes: 60,
		},
		Ops: config.OpsConfig{
			Cleanup: config.OpsCleanupConfig{
				Enabled:                    true,
				ErrorLogRetentionDays:      14,
				MinuteMetricsRetentionDays: 7,
				HourlyMetricsRetentionDays: 30,
				SystemLogMaxRows:           100000,
				ErrorLogMaxRows:            50000,
				MaxRowsEnabled:             false,
				MaxRowsDryRun:              true,
			},
		},
		UsageCleanup: config.UsageCleanupConfig{
			Enabled:               true,
			MaxRangeDays:          30,
			BatchSize:             1000,
			WorkerIntervalSeconds: 60,
			TaskTimeoutSeconds:    120,
		},
		DashboardAgg: config.DashboardAggregationConfig{
			Retention: config.DashboardAggregationRetentionConfig{
				UsageLogsDays:    30,
				UsageLogsMaxRows: 250000,
			},
		},
	}, nil, nil, nil, nil, nil, nil, nil, nil))
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
	require.Contains(t, resp.Data, "db_pool")
	require.Contains(t, resp.Data, "redis_pool")
	require.Contains(t, resp.Data, "http_upstream")
	require.Contains(t, resp.Data, "usage_log_not_persisted")
	require.Contains(t, resp.Data, "billing_compensation")
	require.Contains(t, resp.Data, "scheduler_outbox")
	schedulerOutbox, ok := resp.Data["scheduler_outbox"].(map[string]any)
	require.True(t, ok)
	require.Contains(t, schedulerOutbox, "last_redis_watermark")
	require.Contains(t, schedulerOutbox, "watermark_drift")
	require.Contains(t, schedulerOutbox, "backlog_rows")
	require.Contains(t, schedulerOutbox, "lag_seconds")
	require.Contains(t, schedulerOutbox, "lag_failure_streak")
	require.Contains(t, schedulerOutbox, "blocked_event_clear_total")
	require.Contains(t, schedulerOutbox, "bucket_rebuild_failure_total")
	require.Contains(t, schedulerOutbox, "bucket_rebuild_lock_contention_total")
	require.Contains(t, resp.Data, "usage_record_worker_pool")
	require.Contains(t, resp.Data, "gateway_hotpath")
	require.Contains(t, resp.Data, "cleanup_status")
	require.Contains(t, resp.Data, "usage_cleanup_status")
	require.Contains(t, resp.Data, "ops_runtime_summary")
	runtimeSummary, ok := resp.Data["ops_runtime_summary"].(map[string]any)
	require.True(t, ok)
	require.Contains(t, runtimeSummary, "resource_pressure_summary")
	resourceSummary, ok := runtimeSummary["resource_pressure_summary"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "normal", resourceSummary["severity"])
	require.Contains(t, resourceSummary, "note")
	dbSummary, ok := resourceSummary["db"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "normal", dbSummary["state"])
	require.Contains(t, dbSummary, "open_connections")
	redisSummary, ok := resourceSummary["redis"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "normal", redisSummary["state"])
	require.Contains(t, redisSummary, "note")
	httpSummary, ok := resourceSummary["http_upstream"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "normal", httpSummary["state"])
	require.Contains(t, httpSummary, "note")
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
	dbPool, ok := resp.Data["db_pool"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, float64(0), dbPool["open_connections"])
	require.Equal(t, float64(0), dbPool["wait_count"])
	httpUpstream, ok := resp.Data["http_upstream"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, float64(0), httpUpstream["cached_clients"])
	require.Equal(t, float64(0), httpUpstream["total_inflight"])
	worker, ok := resp.Data["usage_record_worker_pool"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, float64(0), worker["task_timeouts"])
	require.Equal(t, float64(0), worker["task_panics"])
	hotpath, ok := resp.Data["gateway_hotpath"].(map[string]any)
	require.True(t, ok)
	stickySessions, ok := hotpath["sticky_sessions"].(map[string]any)
	require.True(t, ok)
	compareDelete, ok := stickySessions["compare_delete"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, float64(0), compareDelete["attempt_total"])
	require.Equal(t, float64(0), compareDelete["deleted_total"])
	require.Equal(t, float64(0), compareDelete["miss_total"])
	consistency, ok := stickySessions["consistency"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, float64(0), consistency["total_hits"])
	require.Equal(t, float64(0), consistency["ghost_invalidations"])
	compatFallback, ok := stickySessions["compatibility_fallback"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, float64(0), compatFallback["session_hash_legacy_read_fallback_total"])
	require.Equal(t, float64(0), compatFallback["session_hash_legacy_read_fallback_hit"])
	stickyCleanup, ok := hotpath["sticky_session_cleanup"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, float64(0), stickyCleanup["cleanup_total"])
	require.Equal(t, float64(0), stickyCleanup["compare_delete_miss_total"])
	openAIScheduler, ok := hotpath["openai_account_scheduler"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, float64(0), openAIScheduler["select_total"])
	require.Equal(t, float64(0), openAIScheduler["sticky_session_ghost_total"])
	require.Equal(t, float64(0), openAIScheduler["sticky_session_stale_total"])
	require.Equal(t, float64(0), openAIScheduler["sticky_wait_conflict_total"])
	require.Equal(t, float64(0), openAIScheduler["load_batch_call_total"])
	require.Equal(t, float64(0), openAIScheduler["sticky_session_account_fetch_total"])
	require.Contains(t, openAIScheduler, "shadow_reason_totals")
	require.Contains(t, openAIScheduler, "cleanup_reason_totals")
	require.Contains(t, openAIScheduler, "compare_delete_miss_reason_totals")
	usage, ok := resp.Data["usage_log_not_persisted"].(map[string]any)
	require.True(t, ok)
	info, ok := usage["ops_search"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "/api/v1/admin/ops/usage-log-not-persisted", info["endpoint"])
	require.NotEmpty(t, info["note"])
	require.Equal(t, "/api/v1/admin/ops/usage-log-not-persisted/:request_id", info["detail_endpoint_template"])
	_, hasUsageLastDetail := info["last_detail_endpoint"]
	require.False(t, hasUsageLastDetail)
	opsRuntime, ok := resp.Data["ops_runtime_summary"].(map[string]any)
	require.True(t, ok)
	guardrails, ok := opsRuntime["resource_budget_guardrails"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, guardrails)
	firstGuardrail, ok := guardrails[0].(map[string]any)
	require.True(t, ok)
	thresholds, ok := firstGuardrail["watch_thresholds"].(map[string]any)
	require.True(t, ok)
	require.NotEmpty(t, thresholds)
	resourceThresholds, ok := opsRuntime["resource_budget_watch_thresholds"].(map[string]any)
	require.True(t, ok)
	require.Contains(t, resourceThresholds, "db_conn_waiting")
	safetyZone, ok := opsRuntime["resource_budget_safety_zone"].(map[string]any)
	require.True(t, ok)
	require.Contains(t, safetyZone, "status")
	dryRun, ok := opsRuntime["storage_governance_dry_run"].(map[string]any)
	require.True(t, ok)
	require.Contains(t, dryRun, "ops_cleanup")
	require.Contains(t, dryRun, "usage_cleanup")
	require.Contains(t, opsRuntime, "usage_logs_governance")
	sharedResourceRuntime, ok := opsRuntime["shared_resource_runtime"].(map[string]any)
	require.True(t, ok)
	require.Contains(t, sharedResourceRuntime, "db_pool")
	require.Contains(t, sharedResourceRuntime, "redis_pool")
	require.Contains(t, sharedResourceRuntime, "http_upstream")
	_, hasControlPlaneDrift := opsRuntime["control_plane_drift"]
	require.False(t, hasControlPlaneDrift)
	_, hasUsageIntegrity := opsRuntime["usage_integrity"]
	require.False(t, hasUsageIntegrity)
	detailEndpoints, ok := opsRuntime["detail_endpoints"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "/api/v1/admin/ops/dashboard/overview", detailEndpoints["overview"])
	require.Equal(t, "/api/v1/admin/ops/dashboard/snapshot-v2", detailEndpoints["snapshot_v2"])
	queryPathHint, ok := opsRuntime["dashboard_query_path_hint"].(map[string]any)
	require.True(t, ok)
	require.Contains(t, queryPathHint["note"], "snapshot-v2")
	require.Contains(t, queryPathHint["fields"], "overview.query_path")
	protocolHint, ok := opsRuntime["protocol_failure_diagnostics"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "/api/v1/admin/ops/requests?kind=error&sort=duration_desc", protocolHint["request_details_endpoint"])
	require.Equal(t, "/api/v1/admin/ops/request-errors/:id", protocolHint["request_error_endpoint_template"])
	require.Equal(t, "/api/v1/admin/ops/request-errors/:id/upstream-errors?include_detail=true", protocolHint["upstream_errors_endpoint_template"])
	require.Contains(t, protocolHint, "failure_split")
	require.Contains(t, protocolHint, "do_not_failover_until_checked")
	require.Contains(t, protocolHint, "classification_hint")
	require.Equal(t, "info", protocolHint["severity"])
}

func TestBuildControlPlaneDriftPayload_ExposesSignals(t *testing.T) {
	payload := buildControlPlaneDriftPayload(
		service.StickyConsistencyMetricsSnapshot{
			TotalHits:          10,
			GhostInvalidations: 2,
			GhostRatio:         0.2,
			GhostReasons: map[string]int64{
				"transport_mismatch": 2,
			},
		},
		service.SchedulerOutboxRuntimeMetrics{
			WatermarkDrift:   12,
			BacklogRows:      3,
			LagSeconds:       7,
			DriftTrendStatus: "degrading",
			DriftTrendDetail: "backlog=3 lag=7s",
		},
		runtimeprobe.RedisPoolSnapshot{
			Timeouts: 1,
			Stalls:   2,
		},
	)
	require.NotNil(t, payload)
	require.Contains(t, payload, "signals")
	require.Equal(t, "critical", payload["severity"])
	require.Contains(t, payload, "active_domains")
	require.Contains(t, payload, "summary")
	require.Contains(t, payload, "sticky_consistency")
	require.Contains(t, payload, "scheduler")
	require.Contains(t, payload, "redis")
	attribution, ok := payload["attribution"].(gin.H)
	require.True(t, ok)
	sticky, ok := attribution["sticky"].(gin.H)
	require.True(t, ok)
	require.Equal(t, "active", sticky["state"])
	require.Equal(t, "transport_mismatch", sticky["dominant_reason"])
	scheduler, ok := attribution["scheduler"].(gin.H)
	require.True(t, ok)
	require.Equal(t, "active", scheduler["state"])
	redis, ok := attribution["redis"].(gin.H)
	require.True(t, ok)
	require.Equal(t, "critical", redis["state"])
	require.Contains(t, payload, "investigation_order")
	require.Contains(t, payload, "correlation_hint")
	require.Equal(t, "mixed_control_plane_drift", payload["likely_root_cause"])
	require.Contains(t, payload["note"], "control-plane inconsistency")
	trend, ok := payload["trend"].(gin.H)
	require.True(t, ok)
	require.Equal(t, "degrading", trend["status"])
	require.Contains(t, trend["detail"], "backlog=3")
}

func TestBuildResourcePressureSummaryPayload_PromotesCritical(t *testing.T) {
	summary := buildResourcePressureSummaryPayload(
		&service.OpsResourceBudgetSummary{},
		runtimeprobe.DBPoolSnapshot{},
		runtimeprobe.RedisPoolSnapshot{Timeouts: 1},
		runtimeprobe.HTTPUpstreamRuntimeSnapshot{},
	)
	require.NotNil(t, summary)
	require.Equal(t, "critical", summary["severity"])

	redisSummary, ok := summary["redis"].(gin.H)
	require.True(t, ok)
	require.Equal(t, "critical", redisSummary["state"])
}

func TestBuildOpsRuntimeSummaryPayload_ExposesFailureSplitSummary(t *testing.T) {
	handler := &DashboardHandler{opsService: &service.OpsService{}}
	payload := handler.buildOpsRuntimeSummaryPayload(
		service.CleanupStats{},
		service.UsageCleanupStats{},
		&service.OpsRealtimeSummaryBundle{
			FailureSplitSummary: &service.OpsFailureSplitSummary{
				ProtocolOrRequestShape: 2,
				AccountOrAuth:          1,
				LikelyPrimary:          "protocol_or_request_shape",
				Suggestion:             "Validate request shape first.",
			},
		},
	)

	failureSplit, ok := payload["failure_split_summary"].(*service.OpsFailureSplitSummary)
	require.True(t, ok)
	require.Equal(t, int64(2), failureSplit.ProtocolOrRequestShape)
	require.Equal(t, "protocol_or_request_shape", failureSplit.LikelyPrimary)
	actionPlan, ok := payload["operator_action_plan"].(*service.OpsOperatorActionPlan)
	require.True(t, ok)
	require.NotEmpty(t, actionPlan.FailureSplitActions)
	require.NotEmpty(t, actionPlan.ProtocolDiagnostics)
}
