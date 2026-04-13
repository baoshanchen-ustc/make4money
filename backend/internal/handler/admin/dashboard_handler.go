package admin

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/runtimeprobe"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// DashboardHandler handles admin dashboard statistics
type DashboardHandler struct {
	dashboardService   *service.DashboardService
	aggregationService *service.DashboardAggregationService
	opsService         *service.OpsService
	startTime          time.Time // Server start time for uptime calculation
}

// NewDashboardHandler creates a new admin dashboard handler
func NewDashboardHandler(dashboardService *service.DashboardService, aggregationService *service.DashboardAggregationService) *DashboardHandler {
	return &DashboardHandler{
		dashboardService:   dashboardService,
		aggregationService: aggregationService,
		startTime:          time.Now(),
	}
}

func (h *DashboardHandler) SetOpsService(opsService *service.OpsService) *DashboardHandler {
	if h == nil {
		return nil
	}
	h.opsService = opsService
	return h
}

// parseTimeRange parses start_date, end_date query parameters
// Uses user's timezone if provided, otherwise falls back to server timezone
func parseTimeRange(c *gin.Context) (time.Time, time.Time) {
	userTZ := c.Query("timezone") // Get user's timezone from request
	now := timezone.NowInUserLocation(userTZ)
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	var startTime, endTime time.Time

	if startDate != "" {
		if t, err := timezone.ParseInUserLocation("2006-01-02", startDate, userTZ); err == nil {
			startTime = t
		} else {
			startTime = timezone.StartOfDayInUserLocation(now.AddDate(0, 0, -7), userTZ)
		}
	} else {
		startTime = timezone.StartOfDayInUserLocation(now.AddDate(0, 0, -7), userTZ)
	}

	if endDate != "" {
		if t, err := timezone.ParseInUserLocation("2006-01-02", endDate, userTZ); err == nil {
			endTime = t.Add(24 * time.Hour) // Include the end date
		} else {
			endTime = timezone.StartOfDayInUserLocation(now.AddDate(0, 0, 1), userTZ)
		}
	} else {
		endTime = timezone.StartOfDayInUserLocation(now.AddDate(0, 0, 1), userTZ)
	}

	return startTime, endTime
}

// GetStats handles getting dashboard statistics
// GET /api/v1/admin/dashboard/stats
func (h *DashboardHandler) GetStats(c *gin.Context) {
	stats, err := h.dashboardService.GetDashboardStats(c.Request.Context())
	if err != nil {
		response.Error(c, 500, "Failed to get dashboard statistics")
		return
	}

	// Calculate uptime in seconds
	uptime := int64(time.Since(h.startTime).Seconds())

	response.Success(c, gin.H{
		// 用户统计
		"total_users":     stats.TotalUsers,
		"today_new_users": stats.TodayNewUsers,
		"active_users":    stats.ActiveUsers,

		// API Key 统计
		"total_api_keys":  stats.TotalAPIKeys,
		"active_api_keys": stats.ActiveAPIKeys,

		// 账户统计
		"total_accounts":     stats.TotalAccounts,
		"normal_accounts":    stats.NormalAccounts,
		"error_accounts":     stats.ErrorAccounts,
		"ratelimit_accounts": stats.RateLimitAccounts,
		"overload_accounts":  stats.OverloadAccounts,

		// 累计 Token 使用统计
		"total_requests":              stats.TotalRequests,
		"total_input_tokens":          stats.TotalInputTokens,
		"total_output_tokens":         stats.TotalOutputTokens,
		"total_cache_creation_tokens": stats.TotalCacheCreationTokens,
		"total_cache_read_tokens":     stats.TotalCacheReadTokens,
		"total_tokens":                stats.TotalTokens,
		"total_cost":                  stats.TotalCost,       // 标准计费
		"total_actual_cost":           stats.TotalActualCost, // 实际扣除

		// 今日 Token 使用统计
		"today_requests":              stats.TodayRequests,
		"today_input_tokens":          stats.TodayInputTokens,
		"today_output_tokens":         stats.TodayOutputTokens,
		"today_cache_creation_tokens": stats.TodayCacheCreationTokens,
		"today_cache_read_tokens":     stats.TodayCacheReadTokens,
		"today_tokens":                stats.TodayTokens,
		"today_cost":                  stats.TodayCost,       // 今日标准计费
		"today_actual_cost":           stats.TodayActualCost, // 今日实际扣除

		// 系统运行统计
		"average_duration_ms": stats.AverageDurationMs,
		"uptime":              uptime,

		// 性能指标
		"rpm": stats.Rpm,
		"tpm": stats.Tpm,

		// 预聚合新鲜度
		"hourly_active_users": stats.HourlyActiveUsers,
		"stats_updated_at":    stats.StatsUpdatedAt,
		"stats_stale":         stats.StatsStale,
	})
}

type DashboardAggregationBackfillRequest struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// BackfillAggregation handles triggering aggregation backfill
// POST /api/v1/admin/dashboard/aggregation/backfill
func (h *DashboardHandler) BackfillAggregation(c *gin.Context) {
	if h.aggregationService == nil {
		response.InternalError(c, "Aggregation service not available")
		return
	}

	var req DashboardAggregationBackfillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	start, err := time.Parse(time.RFC3339, req.Start)
	if err != nil {
		response.BadRequest(c, "Invalid start time")
		return
	}
	end, err := time.Parse(time.RFC3339, req.End)
	if err != nil {
		response.BadRequest(c, "Invalid end time")
		return
	}

	if err := h.aggregationService.TriggerBackfill(start, end); err != nil {
		if errors.Is(err, service.ErrDashboardBackfillDisabled) {
			response.Forbidden(c, "Backfill is disabled")
			return
		}
		if errors.Is(err, service.ErrDashboardBackfillTooLarge) {
			response.BadRequest(c, "Backfill range too large")
			return
		}
		response.InternalError(c, "Failed to trigger backfill")
		return
	}

	response.Success(c, gin.H{
		"status": "accepted",
	})
}

// GetRealtimeMetrics handles getting real-time system metrics
// GET /api/v1/admin/dashboard/realtime
func (h *DashboardHandler) GetRealtimeMetrics(c *gin.Context) {
	requestsPerMinute := int64(0)
	averageResponseTime := float64(0)
	if h.dashboardService != nil {
		if stats, err := h.dashboardService.GetDashboardStats(c.Request.Context()); err == nil && stats != nil {
			requestsPerMinute = stats.Rpm
			averageResponseTime = stats.AverageDurationMs
		}
	}

	windowHit, windowMiss, windowBatchSQL, windowFallback, windowErr := service.GatewayWindowCostPrefetchStats()
	rateHit, rateMiss, rateLoad, rateSingleflightShared, rateFallback := service.GatewayUserGroupRateCacheStats()
	modelsHit, modelsMiss, modelsStore := service.GatewayModelsListCacheStats()
	stickyCompareDelete := runtimeprobe.SnapshotStickySessionCompareDeleteMetrics()
	stickyCompat := service.SnapshotOpenAICompatibilityFallbackMetrics()
	stickyCleanup := service.SnapshotStickySessionCleanupMetrics()
	stickyConsistency := service.SnapshotStickyConsistencyMetrics()
	stickyConsistencyPayload := buildStickyConsistencyPayload(&stickyConsistency)
	var realtimeBundle *service.OpsRealtimeSummaryBundle
	openAIScheduler := service.OpenAIAccountSchedulerMetricsSnapshot{}
	if h != nil && h.opsService != nil {
		realtimeBundle = h.opsService.GetRealtimeSummaryBundle(c.Request.Context())
		if summary := h.opsService.OpenAIAccountSchedulerSummary(); summary != nil {
			openAIScheduler = service.OpenAIAccountSchedulerMetricsSnapshot{
				SelectTotal:                     summary.SelectTotal,
				StickyPreviousHitTotal:          summary.StickyPreviousHitTotal,
				StickySessionHitTotal:           summary.StickySessionHitTotal,
				StickySessionLookupTotal:        summary.StickySessionLookupTotal,
				StickySessionWaitTotal:          summary.StickySessionWaitTotal,
				StickySessionClearedTotal:       summary.StickySessionClearedTotal,
				StickySessionGhostTotal:         summary.StickySessionGhostTotal,
				StickySessionStaleTotal:         summary.StickySessionStaleTotal,
				StickySessionWaitConflictTotal:  summary.StickyWaitConflictTotal,
				LoadBalanceSelectTotal:          summary.LoadBalanceSelectTotal,
				AccountSwitchTotal:              summary.AccountSwitchTotal,
				SchedulerLatencyMsAvg:           summary.SchedulerLatencyMsAvg,
				StickyHitRatio:                  summary.StickyHitRatio,
				AccountSwitchRate:               summary.AccountSwitchRate,
				StickyGhostRatio:                summary.StickyGhostRatio,
				StickyStaleRatio:                summary.StickyStaleRatio,
				StickyWaitConflictRatio:         summary.StickyWaitConflictRatio,
				StickySessionAccountFetchTotal:  summary.StickySessionAccountFetchTotal,
				StickySessionDBRecheckTotal:     summary.StickySessionDBRecheckTotal,
				LoadBatchCallTotal:              summary.LoadBatchCallTotal,
				LoadBatchCandidateTotal:         summary.LoadBatchCandidateTotal,
				LoadBatchFallbackTotal:          summary.LoadBatchFallbackTotal,
				StickySessionShadowReasonTotals: summary.ShadowReasonTotals,
			}
		}
	}
	cleanupStats := service.SnapshotCleanupStats()
	usageCleanupStats := service.SnapshotUsageCleanupStats()

	response.Success(c, gin.H{
		"active_requests":          0,
		"requests_per_minute":      requestsPerMinute,
		"average_response_time":    averageResponseTime,
		"error_rate":               0.0,
		"db_pool":                  runtimeprobe.SnapshotDefaultDBPoolStats(),
		"redis_pool":               runtimeprobe.SnapshotDefaultRedisPoolStats(),
		"http_upstream":            runtimeprobe.SnapshotDefaultHTTPUpstreamRuntime(),
		"usage_log_not_persisted":  usageLogNotPersistedPayload(),
		"billing_compensation":     billingCompensationPayload(),
		"scheduler_outbox":         service.SnapshotSchedulerOutboxRuntimeMetrics(),
		"usage_record_worker_pool": service.SnapshotUsageRecordWorkerPoolStats(),
		"ops_runtime_summary":      h.buildOpsRuntimeSummaryPayload(cleanupStats, usageCleanupStats, realtimeBundle),
		"gateway_hotpath": gin.H{
			"window_cost_prefetch": gin.H{
				"cache_hit_total":  windowHit,
				"cache_miss_total": windowMiss,
				"batch_sql_total":  windowBatchSQL,
				"fallback_total":   windowFallback,
				"error_total":      windowErr,
			},
			"user_group_rate_cache": gin.H{
				"cache_hit_total":           rateHit,
				"cache_miss_total":          rateMiss,
				"load_total":                rateLoad,
				"singleflight_shared_total": rateSingleflightShared,
				"fallback_total":            rateFallback,
			},
			"models_list_cache": gin.H{
				"cache_hit_total":  modelsHit,
				"cache_miss_total": modelsMiss,
				"store_total":      modelsStore,
			},
			"sticky_sessions": gin.H{
				"compare_delete": gin.H{
					"attempt_total": stickyCompareDelete.AttemptTotal,
					"deleted_total": stickyCompareDelete.DeletedTotal,
					"miss_total":    stickyCompareDelete.MissTotal,
				},
				"consistency": stickyConsistencyPayload,
				"compatibility_fallback": gin.H{
					"session_hash_legacy_read_fallback_total": stickyCompat.SessionHashLegacyReadFallbackTotal,
					"session_hash_legacy_read_fallback_hit":   stickyCompat.SessionHashLegacyReadFallbackHit,
					"session_hash_legacy_dual_write_total":    stickyCompat.SessionHashLegacyDualWriteTotal,
					"session_hash_legacy_read_hit_rate":       stickyCompat.SessionHashLegacyReadHitRate,
					"metadata_legacy_fallback_total":          stickyCompat.MetadataLegacyFallbackTotal,
				},
			},
			"sticky_session_cleanup": gin.H{
				"cleanup_total":                     stickyCleanup.CleanupTotal,
				"compare_delete_miss_total":         stickyCleanup.CompareDeleteMissTotal,
				"cleanup_reason_totals":             stickyCleanup.CleanupReasonTotals,
				"compare_delete_miss_reason_totals": stickyCleanup.CompareDeleteMissReasonTotals,
			},
			"openai_account_scheduler": buildOpenAIAccountSchedulerPayload(openAIScheduler, stickyCleanup),
		},
		"cleanup_status":       cleanupStats,
		"usage_cleanup_status": usageCleanupStats,
	})
}

func buildOpenAIAccountSchedulerPayload(snapshot service.OpenAIAccountSchedulerMetricsSnapshot, stickyCleanup service.StickySessionCleanupMetricsSnapshot) gin.H {
	return gin.H{
		"select_total":                       snapshot.SelectTotal,
		"sticky_previous_hit_total":          snapshot.StickyPreviousHitTotal,
		"sticky_session_hit_total":           snapshot.StickySessionHitTotal,
		"sticky_session_lookup_total":        snapshot.StickySessionLookupTotal,
		"sticky_session_wait_total":          snapshot.StickySessionWaitTotal,
		"sticky_session_cleared_total":       snapshot.StickySessionClearedTotal,
		"sticky_session_ghost_total":         snapshot.StickySessionGhostTotal,
		"sticky_session_stale_total":         snapshot.StickySessionStaleTotal,
		"sticky_wait_conflict_total":         snapshot.StickySessionWaitConflictTotal,
		"load_balance_select_total":          snapshot.LoadBalanceSelectTotal,
		"account_switch_total":               snapshot.AccountSwitchTotal,
		"scheduler_latency_ms_avg":           snapshot.SchedulerLatencyMsAvg,
		"sticky_hit_ratio":                   snapshot.StickyHitRatio,
		"sticky_ghost_ratio":                 snapshot.StickyGhostRatio,
		"sticky_stale_ratio":                 snapshot.StickyStaleRatio,
		"sticky_wait_conflict_ratio":         snapshot.StickyWaitConflictRatio,
		"sticky_session_account_fetch_total": snapshot.StickySessionAccountFetchTotal,
		"sticky_session_db_recheck_total":    snapshot.StickySessionDBRecheckTotal,
		"load_batch_call_total":              snapshot.LoadBatchCallTotal,
		"load_batch_candidate_total":         snapshot.LoadBatchCandidateTotal,
		"load_batch_fallback_total":          snapshot.LoadBatchFallbackTotal,
		"shadow_reason_totals":               snapshot.StickySessionShadowReasonTotals,
		"cleanup_reason_totals":              stickyCleanup.CleanupReasonTotals,
		"compare_delete_miss_total":          stickyCleanup.CompareDeleteMissTotal,
		"compare_delete_miss_reason_totals":  stickyCleanup.CompareDeleteMissReasonTotals,
	}
}

func (h *DashboardHandler) buildOpsRuntimeSummaryPayload(cleanupStats service.CleanupStats, usageCleanupStats service.UsageCleanupStats, bundle *service.OpsRealtimeSummaryBundle) gin.H {
	payload := gin.H{
		"detail_endpoints": gin.H{
			"overview":    "/api/v1/admin/ops/dashboard/overview",
			"snapshot_v2": "/api/v1/admin/ops/dashboard/snapshot-v2",
		},
		"dashboard_query_path_hint": gin.H{
			"note":   "Use snapshot-v2 overview.query_path/data_source to confirm whether ops dashboard queries are running on pre-aggregated tables or have fallen back to raw SQL.",
			"fields": []string{"overview.query_path", "overview.data_source", "query_path", "data_source"},
		},
		"protocol_failure_diagnostics": gin.H{
			"request_details_endpoint":          "/api/v1/admin/ops/requests?kind=error&sort=duration_desc",
			"request_error_endpoint_template":   "/api/v1/admin/ops/request-errors/:id",
			"upstream_errors_endpoint_template": "/api/v1/admin/ops/request-errors/:id/upstream-errors?include_detail=true",
			"failure_split": []string{
				"protocol_or_request_shape",
				"account_or_auth",
				"provider_or_upstream",
				"local_processing",
			},
			"do_not_failover_until_checked": []string{
				"protocol_failure_reason",
				"requested_model",
				"mapped_model",
				"upstream_model",
			},
			"severity": "info",
			"fields": []string{
				"upstream_errors[].requested_model",
				"upstream_errors[].mapped_model",
				"upstream_errors[].upstream_model",
				"upstream_errors[].failure_category",
				"upstream_errors[].protocol_failure_reason",
			},
			"investigation_order": []string{
				"Open the slowest error requests in /api/v1/admin/ops/requests to identify the failing request_id.",
				"Inspect /api/v1/admin/ops/request-errors/:id for the client-visible error and upstream status context.",
				"Inspect /api/v1/admin/ops/request-errors/:id/upstream-errors?include_detail=true to review requested->mapped->upstream model transitions and protocol_failure_reason.",
			},
			"correlation_hint":    "Correlate request_id and client_request_id across request details, request-errors, and upstream-errors before blaming account health or increasing failover pressure.",
			"note":                "Use request-errors upstream drill-down to separate protocol failures from account/provider failures before increasing failover or retry pressure.",
			"classification_hint": "If protocol_failure_reason is populated or requested->mapped->upstream diverges unexpectedly, treat the incident as protocol/model-chain first; do not increase failover pressure until account/auth signals are also confirmed.",
		},
	}
	if bundle != nil && bundle.FailureSplitSummary != nil {
		payload["failure_split_summary"] = bundle.FailureSplitSummary
	}
	if h == nil || h.opsService == nil {
		return payload
	}
	budget := h.opsService.ResourceBudgetSummary(nil)
	if budget != nil && len(budget.Guardrails) > 0 {
		payload["resource_budget_guardrails"] = budget.Guardrails
		payload["resource_budget_watch_thresholds"] = budget.WatchThresholds
		payload["resource_budget_safety_zone"] = budget.SafetyZone
		if len(budget.JointSignals) > 0 {
			payload["resource_budget_joint_signals"] = budget.JointSignals
		}
	}
	if storage := h.opsService.StorageGovernanceSummary(cleanupStats, usageCleanupStats, nil); storage != nil {
		if storage.DryRun != nil {
			payload["storage_governance_dry_run"] = storage.DryRun
		}
		payload["usage_logs_governance"] = storage.UsageLogs
	}
	if usageIntegrity := service.UsageIntegritySummary(); usageIntegrity != nil {
		payload["usage_integrity"] = usageIntegrity
	}
	dbPool := runtimeprobe.SnapshotDefaultDBPoolStats()
	redisPool := runtimeprobe.SnapshotDefaultRedisPoolStats()
	httpUpstream := runtimeprobe.SnapshotDefaultHTTPUpstreamRuntime()
	stickyConsistency := service.SnapshotStickyConsistencyMetrics()
	schedulerOutbox := service.SnapshotSchedulerOutboxRuntimeMetrics()
	payload["shared_resource_runtime"] = gin.H{
		"db_pool":       dbPool,
		"redis_pool":    redisPool,
		"http_upstream": httpUpstream,
	}
	if signals := buildSharedResourceJointPressurePayload(dbPool, redisPool, httpUpstream); len(signals) > 0 {
		payload["shared_resource_joint_pressure"] = signals
	}
	if drift := buildControlPlaneDriftPayload(stickyConsistency, schedulerOutbox, redisPool); len(drift) > 0 {
		payload["control_plane_drift"] = drift
	}
	if summary := buildResourcePressureSummaryPayload(budget, dbPool, redisPool, httpUpstream); summary != nil {
		payload["resource_pressure_summary"] = summary
	}
	if plan := buildOperatorActionPlan(bundleFailureSplitSummary(bundle), payload["control_plane_drift"]); plan != nil {
		payload["operator_action_plan"] = plan
	}
	return payload
}

func bundleFailureSplitSummary(bundle *service.OpsRealtimeSummaryBundle) *service.OpsFailureSplitSummary {
	if bundle == nil {
		return nil
	}
	return bundle.FailureSplitSummary
}

func buildFailureSplitSummaryPayload(summary *service.OpsFailureSplitSummary) gin.H {
	payload := gin.H{
		"protocol_or_request_shape": summary.ProtocolOrRequestShape,
		"account_or_auth":           summary.AccountOrAuth,
		"provider_or_upstream":      summary.ProviderOrUpstream,
		"local_processing":          summary.LocalProcessing,
	}
	if summary.LikelyPrimary != "" {
		payload["likely_primary"] = summary.LikelyPrimary
		payload["suggestion"] = summary.Suggestion
	}
	return payload
}

func buildOperatorActionPlan(summary *service.OpsFailureSplitSummary, controlPlaneDrift any) *service.OpsOperatorActionPlan {
	var plan service.OpsOperatorActionPlan
	if summary != nil {
		if action := buildFailureSplitPlan(strings.TrimSpace(summary.LikelyPrimary)); action != "" {
			plan.FailureSplitActions = append(plan.FailureSplitActions, action)
		}
		if suggestion := strings.TrimSpace(summary.Suggestion); suggestion != "" {
			plan.FailureSplitActions = append(plan.FailureSplitActions, suggestion)
		}
		if operatorAction := strings.TrimSpace(summary.OperatorAction); operatorAction != "" {
			plan.FailureSplitActions = append(plan.FailureSplitActions, operatorAction)
		}
		plan.ProtocolDiagnostics = []string{
			"1. /api/v1/admin/ops/requests?kind=error&sort=duration_desc",
			"2. /api/v1/admin/ops/request-errors/:id",
			"3. /api/v1/admin/ops/request-errors/:id/upstream-errors?include_detail=true",
		}
	}
	if drift, ok := controlPlaneDrift.(gin.H); ok && len(drift) > 0 {
		plan.ControlPlaneDrift = append(plan.ControlPlaneDrift,
			"Sticky -> scheduler -> redis: confirm ghost bindings, backlog, and shared pressure in that order.",
			"Only after sticky and scheduler drift, inspect Redis timeouts/stalls before adding concurrency.")
	}
	if len(plan.FailureSplitActions) == 0 && len(plan.ProtocolDiagnostics) == 0 && len(plan.ControlPlaneDrift) == 0 {
		return nil
	}
	return &plan
}

func buildFailureSplitPlan(primary string) string {
	switch primary {
	case "protocol_or_request_shape":
		return "Protocol: confirm request shape + requested->mapped->upstream before touching failover pressure."
	case "account_or_auth":
		return "Account: keep credential out of dispatch and wait for refresh/manual reauth signals."
	case "provider_or_upstream":
		return "Provider: check upstream saturation/quota before rotating credentials."
	case "local_processing":
		return "Local: rule out sticky/scheduler/Redis drift before raising concurrency."
	default:
		return "Use the failure split totals to pick the next phase and keep investigation ordered."
	}
}

func buildResourcePressureSummaryPayload(
	budget *service.OpsResourceBudgetSummary,
	db runtimeprobe.DBPoolSnapshot,
	redis runtimeprobe.RedisPoolSnapshot,
	httpUpstream runtimeprobe.HTTPUpstreamRuntimeSnapshot,
) gin.H {
	if budget == nil {
		return nil
	}
	dbState, dbNote := evaluateDBPressure(db)
	redisState, redisNote := runtimeprobe.EvaluateRedisPressure(redis)
	httpState, httpNote := runtimeprobe.EvaluateHTTPUpstreamPressure(httpUpstream)
	overall := highestPressureSeverity(dbState, redisState, httpState)
	summary := gin.H{
		"severity": overall,
		"note":     fmt.Sprintf("db:%s redis:%s http:%s", dbState, redisState, httpState),
		"db": gin.H{
			"state":                dbState,
			"note":                 dbNote,
			"open_connections":     db.OpenConnections,
			"wait_count":           db.WaitCount,
			"usage_percent":        db.UsagePercent,
			"max_open_connections": db.MaxOpenConnections,
		},
		"redis": gin.H{
			"state":         redisState,
			"note":          redisNote,
			"timeouts":      redis.Timeouts,
			"stalls":        redis.Stalls,
			"total_conns":   redis.TotalConns,
			"idle_conns":    redis.IdleConns,
			"usage_percent": redis.UsagePercent,
		},
		"http_upstream": gin.H{
			"state":                   httpState,
			"note":                    httpNote,
			"cached_clients":          httpUpstream.CachedClients,
			"active_clients":          httpUpstream.ActiveClients,
			"idle_clients":            httpUpstream.IdleClients,
			"max_upstream_clients":    httpUpstream.MaxUpstreamClients,
			"client_headroom_percent": httpUpstream.ClientHeadroomPercent,
			"cache_usage_percent":     httpUpstream.CacheUsagePercent,
		},
	}
	return summary
}

func highestPressureSeverity(states ...string) string {
	overall := "normal"
	for _, state := range states {
		switch strings.TrimSpace(state) {
		case "critical":
			return "critical"
		case "warning":
			overall = "warning"
		}
	}
	return overall
}

func buildStickyConsistencyPayload(snapshot *service.StickyConsistencyMetricsSnapshot) gin.H {
	payload := gin.H{
		"total_hits":          int64(0),
		"ghost_invalidations": int64(0),
		"ghost_ratio":         float64(0),
	}
	if snapshot == nil {
		return payload
	}
	payload["total_hits"] = snapshot.TotalHits
	payload["ghost_invalidations"] = snapshot.GhostInvalidations
	payload["ghost_ratio"] = snapshot.GhostRatio
	if len(snapshot.GhostReasons) > 0 {
		payload["ghost_reasons"] = snapshot.GhostReasons
	}
	if len(snapshot.PrimaryReasons) > 0 {
		reasons := make([]gin.H, 0, len(snapshot.PrimaryReasons))
		for _, reason := range snapshot.PrimaryReasons {
			reasons = append(reasons, gin.H{
				"reason":     reason.Reason,
				"count":      reason.Count,
				"percentage": reason.Percentage,
				"severity":   reason.Severity,
			})
		}
		payload["primary_reasons"] = reasons
	}
	if snapshot.ActivityLevel != "" {
		payload["activity_level"] = snapshot.ActivityLevel
	}
	return payload
}

func evaluateDBPressure(db runtimeprobe.DBPoolSnapshot) (string, string) {
	state := "normal"
	note := "DB connections within safety"
	if db.WaitCount > 0 {
		state = "warning"
		note = fmt.Sprintf("wait_count %d indicates contention", db.WaitCount)
	} else if db.OpenConnections > db.MaxOpenConnections {
		state = "warning"
		note = "open connections exceed configured max"
	} else if db.UsagePercent != nil && *db.UsagePercent >= 90 {
		state = "warning"
		note = fmt.Sprintf("usage %.1f%% of max", *db.UsagePercent)
	} else if db.InUsePercent != nil && *db.InUsePercent >= 90 {
		state = "warning"
		note = fmt.Sprintf("in-use %.1f%% of max", *db.InUsePercent)
	}
	if state == "normal" && db.Idle > db.MaxOpenConnections/2 {
		note = "idle connections healthy"
	}
	return state, note
}

func buildControlPlaneDriftPayload(
	sticky service.StickyConsistencyMetricsSnapshot,
	scheduler service.SchedulerOutboxRuntimeMetrics,
	redis runtimeprobe.RedisPoolSnapshot,
) gin.H {
	signals := make([]string, 0, 4)
	activeDomains := make([]string, 0, 3)
	if sticky.GhostInvalidations > 0 {
		signals = append(signals, "sticky_ghost_invalidations")
		activeDomains = append(activeDomains, "sticky")
	}
	if scheduler.WatermarkDrift != 0 {
		signals = append(signals, "scheduler_watermark_drift")
	}
	if scheduler.BacklogRows > 0 || scheduler.LagSeconds > 0 {
		signals = append(signals, "scheduler_backlog_or_lag")
	}
	if scheduler.WatermarkDrift != 0 || scheduler.BacklogRows > 0 || scheduler.LagSeconds > 0 || scheduler.CheckpointFallbackStreak > 0 || scheduler.BlockedEvent != nil {
		activeDomains = append(activeDomains, "scheduler")
	}
	if redis.Timeouts > 0 || redis.Stalls > 0 {
		signals = append(signals, "redis_control_plane_pressure")
		activeDomains = append(activeDomains, "redis")
	}
	if len(signals) == 0 {
		return nil
	}
	likelyRootCause := deriveControlPlaneDriftLikelyRootCause(sticky, scheduler, redis)
	severity := deriveControlPlaneDriftSeverity(sticky, scheduler, redis)
	payload := gin.H{
		"severity":          severity,
		"signals":           signals,
		"active_domains":    activeDomains,
		"likely_root_cause": likelyRootCause,
		"summary":           summarizeControlPlaneDrift(likelyRootCause, activeDomains),
		"sticky_consistency": gin.H{
			"ghost_invalidations": sticky.GhostInvalidations,
			"ghost_ratio":         sticky.GhostRatio,
		},
		"scheduler": gin.H{
			"watermark_drift": scheduler.WatermarkDrift,
			"backlog_rows":    scheduler.BacklogRows,
			"lag_seconds":     scheduler.LagSeconds,
		},
		"redis": gin.H{
			"timeouts": redis.Timeouts,
			"stalls":   redis.Stalls,
		},
		"attribution": gin.H{
			"sticky":    buildStickyDriftAttribution(sticky),
			"scheduler": buildSchedulerDriftAttribution(scheduler),
			"redis":     buildRedisPressureAttribution(redis),
		},
		"investigation_order": []string{
			"Check sticky_session_runtime.consistency and sticky_session_cleanup first to confirm ghost bindings are rising.",
			"Inspect scheduler_outbox runtime for watermark drift, backlog, and blocked event signals.",
			"Only after that, inspect Redis pool pressure and shared-resource contention before adding more concurrency.",
		},
		"correlation_hint": "When sticky ghosting, scheduler drift, and Redis stalls/timeouts move together, treat the issue as shared control-plane inconsistency instead of isolated account failures.",
		"note":             "When sticky ghosting, scheduler drift, and Redis pressure rise together, treat it as control-plane inconsistency before adding more concurrency or retry pressure.",
	}

	if trend := buildDriftTrendPayload(scheduler); len(trend) > 0 {
		payload["trend"] = trend
	}
	return payload
}

func deriveControlPlaneDriftSeverity(
	sticky service.StickyConsistencyMetricsSnapshot,
	scheduler service.SchedulerOutboxRuntimeMetrics,
	redis runtimeprobe.RedisPoolSnapshot,
) string {
	hasSticky := sticky.GhostInvalidations > 0
	hasScheduler := scheduler.WatermarkDrift != 0 || scheduler.BacklogRows > 0 || scheduler.LagSeconds > 0 || scheduler.CheckpointFallbackStreak > 0 || scheduler.BlockedEvent != nil
	hasRedisTimeout := redis.Timeouts > 0
	hasRedisPressure := hasRedisTimeout || redis.Stalls > 0
	switch {
	case hasRedisTimeout && (hasSticky || hasScheduler):
		return "critical"
	case hasSticky && hasScheduler:
		return "warning"
	case hasScheduler && hasRedisPressure:
		return "warning"
	case hasSticky || hasScheduler || hasRedisPressure:
		return "warning"
	default:
		return "info"
	}
}

func summarizeControlPlaneDrift(likelyRootCause string, activeDomains []string) string {
	if len(activeDomains) == 0 {
		return ""
	}
	switch likelyRootCause {
	case "mixed_control_plane_drift":
		return "Sticky/session drift, scheduler lag, and Redis pressure are moving together."
	case "sticky_and_scheduler_drift":
		return "Sticky/session drift and scheduler drift are both active."
	case "scheduler_or_redis_control_plane_drift":
		return "Scheduler lag and Redis control-plane pressure are both active."
	case "sticky_binding_drift":
		return "Sticky/session bindings are drifting ahead of the rest of the control plane."
	case "scheduler_outbox_or_checkpoint_drift":
		return "Scheduler outbox or checkpoint drift is the dominant control-plane signal."
	case "redis_control_plane_pressure":
		return "Redis control-plane pressure is the dominant drift signal."
	default:
		return "Control-plane drift signals are active."
	}
}

func buildStickyDriftAttribution(sticky service.StickyConsistencyMetricsSnapshot) gin.H {
	if sticky.GhostInvalidations <= 0 {
		return gin.H{
			"state": "clear",
		}
	}
	dominantReason, dominantCount := dominantReasonTotal(sticky.GhostReasons)
	payload := gin.H{
		"state":               "active",
		"ghost_invalidations": sticky.GhostInvalidations,
		"ghost_ratio":         sticky.GhostRatio,
	}
	if dominantReason != "" {
		payload["dominant_reason"] = dominantReason
		payload["dominant_count"] = dominantCount
		payload["reason_totals"] = sticky.GhostReasons
	}
	return payload
}

func buildSchedulerDriftAttribution(scheduler service.SchedulerOutboxRuntimeMetrics) gin.H {
	if scheduler.WatermarkDrift == 0 &&
		scheduler.BacklogRows == 0 &&
		scheduler.LagSeconds == 0 &&
		scheduler.CheckpointFallbackStreak == 0 &&
		scheduler.BlockedEvent == nil {
		return gin.H{
			"state": "clear",
		}
	}
	payload := gin.H{
		"state":                      "active",
		"watermark_drift":            scheduler.WatermarkDrift,
		"backlog_rows":               scheduler.BacklogRows,
		"lag_seconds":                scheduler.LagSeconds,
		"checkpoint_fallback_streak": scheduler.CheckpointFallbackStreak,
	}
	if scheduler.CheckpointLastFallbackReason != "" {
		payload["checkpoint_last_fallback_reason"] = scheduler.CheckpointLastFallbackReason
	}
	if scheduler.BlockedEvent != nil {
		payload["blocked_event_reason"] = scheduler.BlockedEvent.Reason
		payload["blocked_event_attempts"] = scheduler.BlockedEvent.Attempts
		payload["blocked_event_age_seconds"] = scheduler.BlockedEvent.AgeSeconds
	}
	return payload
}

func buildRedisPressureAttribution(redis runtimeprobe.RedisPoolSnapshot) gin.H {
	if redis.Timeouts == 0 && redis.Stalls == 0 {
		return gin.H{
			"state": "clear",
		}
	}
	state := "warning"
	if redis.Timeouts > 0 {
		state = "critical"
	}
	payload := gin.H{
		"state":       state,
		"timeouts":    redis.Timeouts,
		"stalls":      redis.Stalls,
		"total_conns": redis.TotalConns,
		"idle_conns":  redis.IdleConns,
	}
	if redis.UsagePercent != nil {
		payload["usage_percent"] = *redis.UsagePercent
	}
	return payload
}

func dominantReasonTotal(reasonTotals map[string]int64) (string, int64) {
	var bestReason string
	var bestCount int64
	for reason, count := range reasonTotals {
		if count > bestCount {
			bestReason = reason
			bestCount = count
		}
	}
	return bestReason, bestCount
}

func deriveControlPlaneDriftLikelyRootCause(
	sticky service.StickyConsistencyMetricsSnapshot,
	scheduler service.SchedulerOutboxRuntimeMetrics,
	redis runtimeprobe.RedisPoolSnapshot,
) string {
	hasSticky := sticky.GhostInvalidations > 0
	hasScheduler := scheduler.WatermarkDrift != 0 || scheduler.BacklogRows > 0 || scheduler.LagSeconds > 0 || scheduler.CheckpointFallbackStreak > 0
	hasRedis := redis.Timeouts > 0 || redis.Stalls > 0
	switch {
	case hasSticky && hasScheduler && hasRedis:
		return "mixed_control_plane_drift"
	case hasSticky && hasScheduler:
		return "sticky_and_scheduler_drift"
	case hasScheduler && hasRedis:
		return "scheduler_or_redis_control_plane_drift"
	case hasSticky:
		return "sticky_binding_drift"
	case hasScheduler:
		return "scheduler_outbox_or_checkpoint_drift"
	case hasRedis:
		return "redis_control_plane_pressure"
	default:
		return "control_plane_drift"
	}
}

func buildSharedResourceJointPressurePayload(
	dbPool runtimeprobe.DBPoolSnapshot,
	redisPool runtimeprobe.RedisPoolSnapshot,
	httpUpstream runtimeprobe.HTTPUpstreamRuntimeSnapshot,
) []gin.H {
	signals := make([]gin.H, 0, 3)
	if dbPool.WaitCount > 0 && redisPool.Stalls > 0 {
		signals = append(signals, gin.H{
			"level":      "warning",
			"title":      "DB wait and Redis stalls are rising together",
			"detail":     gin.H{"db_wait_count": dbPool.WaitCount, "redis_stalls": redisPool.Stalls},
			"suggestion": "Treat this as shared control-plane contention and pause further pool shrink steps until the hot path is quieter.",
		})
	}
	if redisPool.Timeouts > 0 {
		signals = append(signals, gin.H{
			"level":      "warning",
			"title":      "Redis timeouts detected",
			"detail":     gin.H{"redis_timeouts": redisPool.Timeouts, "redis_total_conns": redisPool.TotalConns},
			"suggestion": "Any sustained Redis timeout growth is a hard stop for further concurrency increases on this host.",
		})
	}
	if httpUpstream.CacheUsagePercent != nil && *httpUpstream.CacheUsagePercent >= 85 {
		signals = append(signals, gin.H{
			"level": "info",
			"title": "HTTP upstream client cache is near saturation",
			"detail": gin.H{
				"cache_usage_percent":  *httpUpstream.CacheUsagePercent,
				"cached_clients":       httpUpstream.CachedClients,
				"max_upstream_clients": httpUpstream.MaxUpstreamClients,
			},
			"suggestion": "Shrink idle and cache ceilings before lowering per-host connection caps under burst traffic.",
		})
	}
	return signals
}

func buildDriftTrendPayload(scheduler service.SchedulerOutboxRuntimeMetrics) gin.H {
	if scheduler.DriftTrendStatus == "" && scheduler.DriftTrendDetail == "" {
		return nil
	}
	return gin.H{
		"status": scheduler.DriftTrendStatus,
		"detail": scheduler.DriftTrendDetail,
	}
}

func billingCompensationPayload() gin.H {
	snapshot := service.SnapshotBillingCompensationCandidates()
	payload := gin.H{
		"total":            snapshot.Total,
		"recent_1m_total":  snapshot.Recent1mTotal,
		"recent_5m_total":  snapshot.Recent5mTotal,
		"recent_15m_total": snapshot.Recent15mTotal,
		"recent_count":     len(snapshot.Recent),
		"details":          snapshot.Recent,
		"snapshot":         snapshot,
	}
	if snapshot.Last != nil {
		payload["last_request_id"] = snapshot.Last.RequestID
		payload["last_error"] = snapshot.Last.Error
		payload["last_account_id"] = snapshot.Last.AccountID
	}
	if len(snapshot.Recent) > 0 {
		ids := make([]string, len(snapshot.Recent))
		for i, candidate := range snapshot.Recent {
			ids[i] = candidate.RequestID
		}
		payload["recent_request_ids"] = ids
	}
	if snapshot.Last != nil {
		payload["last_candidate"] = gin.H{
			"request_id":  snapshot.Last.RequestID,
			"user_id":     snapshot.Last.UserID,
			"account_id":  snapshot.Last.AccountID,
			"api_key_id":  snapshot.Last.APIKeyID,
			"total_cost":  snapshot.Last.TotalCost,
			"actual_cost": snapshot.Last.ActualCost,
			"error":       snapshot.Last.Error,
			"timestamp":   snapshot.Last.Timestamp,
		}
		payload["manual_hint"] = gin.H{
			"note":         "手动核对并重放该 usage，确保 request_id 与 billing_key 一致",
			"request_id":   snapshot.Last.RequestID,
			"account_id":   snapshot.Last.AccountID,
			"billing_type": snapshot.Last.BillingType,
		}
	}
	if snapshot.Total == 0 && len(snapshot.Recent) == 0 {
		payload["fallback"] = gin.H{
			"reason":          "snapshot_empty",
			"recent_logs":     []string{service.OpsRuntimeBillingCompensationComponent, service.OpsRuntimeUsageLogComponent},
			"usage_alert_ids": extractUsageAlertIDs(service.SnapshotUsageLogNotPersistedMetrics()),
			"items":           snapshot.Recent,
		}
	}
	lastRequestID := ""
	if snapshot.Last != nil {
		lastRequestID = snapshot.Last.RequestID
	}
	payload["ops_search"] = attachOpsSearchLastDetailEndpoint(opsSearchHint(
		"/api/v1/admin/ops/billing-compensation",
		"/api/v1/admin/ops/billing-compensation/:request_id",
		"Go to the ops endpoint when you need to inspect persisted billing compensation candidates for replay.",
	), lastRequestID)
	return payload
}

func extractUsageAlertIDs(snapshot service.UsageLogNotPersistedMetricsSnapshot) []string {
	if len(snapshot.Recent) == 0 {
		return nil
	}
	result := make([]string, 0, len(snapshot.Recent))
	for _, alert := range snapshot.Recent {
		result = append(result, alert.RequestID)
	}
	return result
}

// GetUsageTrend handles getting usage trend data
// GET /api/v1/admin/dashboard/trend
// Query params: start_date, end_date (YYYY-MM-DD), granularity (day/hour), user_id, api_key_id, model, account_id, group_id, request_type, stream, billing_type
func (h *DashboardHandler) GetUsageTrend(c *gin.Context) {
	startTime, endTime := parseTimeRange(c)
	granularity := c.DefaultQuery("granularity", "day")

	// Parse optional filter params
	var userID, apiKeyID, accountID, groupID int64
	var model string
	var requestType *int16
	var stream *bool
	var billingType *int8

	if userIDStr := c.Query("user_id"); userIDStr != "" {
		if id, err := strconv.ParseInt(userIDStr, 10, 64); err == nil {
			userID = id
		}
	}
	if apiKeyIDStr := c.Query("api_key_id"); apiKeyIDStr != "" {
		if id, err := strconv.ParseInt(apiKeyIDStr, 10, 64); err == nil {
			apiKeyID = id
		}
	}
	if accountIDStr := c.Query("account_id"); accountIDStr != "" {
		if id, err := strconv.ParseInt(accountIDStr, 10, 64); err == nil {
			accountID = id
		}
	}
	if groupIDStr := c.Query("group_id"); groupIDStr != "" {
		if id, err := strconv.ParseInt(groupIDStr, 10, 64); err == nil {
			groupID = id
		}
	}
	if modelStr := c.Query("model"); modelStr != "" {
		model = modelStr
	}
	if requestTypeStr := strings.TrimSpace(c.Query("request_type")); requestTypeStr != "" {
		parsed, err := service.ParseUsageRequestType(requestTypeStr)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		value := int16(parsed)
		requestType = &value
	} else if streamStr := c.Query("stream"); streamStr != "" {
		if streamVal, err := strconv.ParseBool(streamStr); err == nil {
			stream = &streamVal
		} else {
			response.BadRequest(c, "Invalid stream value, use true or false")
			return
		}
	}
	if billingTypeStr := c.Query("billing_type"); billingTypeStr != "" {
		if v, err := strconv.ParseInt(billingTypeStr, 10, 8); err == nil {
			bt := int8(v)
			billingType = &bt
		} else {
			response.BadRequest(c, "Invalid billing_type")
			return
		}
	}

	trend, hit, err := h.getUsageTrendCached(c.Request.Context(), startTime, endTime, granularity, userID, apiKeyID, accountID, groupID, model, requestType, stream, billingType)
	if err != nil {
		response.Error(c, 500, "Failed to get usage trend")
		return
	}
	c.Header("X-Snapshot-Cache", cacheStatusValue(hit))

	response.Success(c, gin.H{
		"trend":       trend,
		"start_date":  startTime.Format("2006-01-02"),
		"end_date":    endTime.Add(-24 * time.Hour).Format("2006-01-02"),
		"granularity": granularity,
	})
}

// GetModelStats handles getting model usage statistics
// GET /api/v1/admin/dashboard/models
// Query params: start_date, end_date (YYYY-MM-DD), user_id, api_key_id, account_id, group_id, request_type, stream, billing_type
func (h *DashboardHandler) GetModelStats(c *gin.Context) {
	startTime, endTime := parseTimeRange(c)

	// Parse optional filter params
	var userID, apiKeyID, accountID, groupID int64
	modelSource := usagestats.ModelSourceRequested
	var requestType *int16
	var stream *bool
	var billingType *int8

	if userIDStr := c.Query("user_id"); userIDStr != "" {
		if id, err := strconv.ParseInt(userIDStr, 10, 64); err == nil {
			userID = id
		}
	}
	if apiKeyIDStr := c.Query("api_key_id"); apiKeyIDStr != "" {
		if id, err := strconv.ParseInt(apiKeyIDStr, 10, 64); err == nil {
			apiKeyID = id
		}
	}
	if accountIDStr := c.Query("account_id"); accountIDStr != "" {
		if id, err := strconv.ParseInt(accountIDStr, 10, 64); err == nil {
			accountID = id
		}
	}
	if groupIDStr := c.Query("group_id"); groupIDStr != "" {
		if id, err := strconv.ParseInt(groupIDStr, 10, 64); err == nil {
			groupID = id
		}
	}
	if rawModelSource := strings.TrimSpace(c.Query("model_source")); rawModelSource != "" {
		if !usagestats.IsValidModelSource(rawModelSource) {
			response.BadRequest(c, "Invalid model_source, use requested/upstream/mapping")
			return
		}
		modelSource = rawModelSource
	}
	if requestTypeStr := strings.TrimSpace(c.Query("request_type")); requestTypeStr != "" {
		parsed, err := service.ParseUsageRequestType(requestTypeStr)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		value := int16(parsed)
		requestType = &value
	} else if streamStr := c.Query("stream"); streamStr != "" {
		if streamVal, err := strconv.ParseBool(streamStr); err == nil {
			stream = &streamVal
		} else {
			response.BadRequest(c, "Invalid stream value, use true or false")
			return
		}
	}
	if billingTypeStr := c.Query("billing_type"); billingTypeStr != "" {
		if v, err := strconv.ParseInt(billingTypeStr, 10, 8); err == nil {
			bt := int8(v)
			billingType = &bt
		} else {
			response.BadRequest(c, "Invalid billing_type")
			return
		}
	}

	stats, hit, err := h.getModelStatsCached(c.Request.Context(), startTime, endTime, userID, apiKeyID, accountID, groupID, modelSource, requestType, stream, billingType)
	if err != nil {
		response.Error(c, 500, "Failed to get model statistics")
		return
	}
	c.Header("X-Snapshot-Cache", cacheStatusValue(hit))

	response.Success(c, gin.H{
		"models":     stats,
		"start_date": startTime.Format("2006-01-02"),
		"end_date":   endTime.Add(-24 * time.Hour).Format("2006-01-02"),
	})
}

// GetGroupStats handles getting group usage statistics
// GET /api/v1/admin/dashboard/groups
// Query params: start_date, end_date (YYYY-MM-DD), user_id, api_key_id, account_id, group_id, request_type, stream, billing_type
func (h *DashboardHandler) GetGroupStats(c *gin.Context) {
	startTime, endTime := parseTimeRange(c)

	var userID, apiKeyID, accountID, groupID int64
	var requestType *int16
	var stream *bool
	var billingType *int8

	if userIDStr := c.Query("user_id"); userIDStr != "" {
		if id, err := strconv.ParseInt(userIDStr, 10, 64); err == nil {
			userID = id
		}
	}
	if apiKeyIDStr := c.Query("api_key_id"); apiKeyIDStr != "" {
		if id, err := strconv.ParseInt(apiKeyIDStr, 10, 64); err == nil {
			apiKeyID = id
		}
	}
	if accountIDStr := c.Query("account_id"); accountIDStr != "" {
		if id, err := strconv.ParseInt(accountIDStr, 10, 64); err == nil {
			accountID = id
		}
	}
	if groupIDStr := c.Query("group_id"); groupIDStr != "" {
		if id, err := strconv.ParseInt(groupIDStr, 10, 64); err == nil {
			groupID = id
		}
	}
	if requestTypeStr := strings.TrimSpace(c.Query("request_type")); requestTypeStr != "" {
		parsed, err := service.ParseUsageRequestType(requestTypeStr)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		value := int16(parsed)
		requestType = &value
	} else if streamStr := c.Query("stream"); streamStr != "" {
		if streamVal, err := strconv.ParseBool(streamStr); err == nil {
			stream = &streamVal
		} else {
			response.BadRequest(c, "Invalid stream value, use true or false")
			return
		}
	}
	if billingTypeStr := c.Query("billing_type"); billingTypeStr != "" {
		if v, err := strconv.ParseInt(billingTypeStr, 10, 8); err == nil {
			bt := int8(v)
			billingType = &bt
		} else {
			response.BadRequest(c, "Invalid billing_type")
			return
		}
	}

	stats, hit, err := h.getGroupStatsCached(c.Request.Context(), startTime, endTime, userID, apiKeyID, accountID, groupID, requestType, stream, billingType)
	if err != nil {
		response.Error(c, 500, "Failed to get group statistics")
		return
	}
	c.Header("X-Snapshot-Cache", cacheStatusValue(hit))

	response.Success(c, gin.H{
		"groups":     stats,
		"start_date": startTime.Format("2006-01-02"),
		"end_date":   endTime.Add(-24 * time.Hour).Format("2006-01-02"),
	})
}

// GetAPIKeyUsageTrend handles getting API key usage trend data
// GET /api/v1/admin/dashboard/api-keys-trend
// Query params: start_date, end_date (YYYY-MM-DD), granularity (day/hour), limit (default 5)
func (h *DashboardHandler) GetAPIKeyUsageTrend(c *gin.Context) {
	startTime, endTime := parseTimeRange(c)
	granularity := c.DefaultQuery("granularity", "day")
	limitStr := c.DefaultQuery("limit", "5")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 5
	}

	trend, hit, err := h.getAPIKeyUsageTrendCached(c.Request.Context(), startTime, endTime, granularity, limit)
	if err != nil {
		response.Error(c, 500, "Failed to get API key usage trend")
		return
	}
	c.Header("X-Snapshot-Cache", cacheStatusValue(hit))

	response.Success(c, gin.H{
		"trend":       trend,
		"start_date":  startTime.Format("2006-01-02"),
		"end_date":    endTime.Add(-24 * time.Hour).Format("2006-01-02"),
		"granularity": granularity,
	})
}

// GetUserUsageTrend handles getting user usage trend data
// GET /api/v1/admin/dashboard/users-trend
// Query params: start_date, end_date (YYYY-MM-DD), granularity (day/hour), limit (default 12)
func (h *DashboardHandler) GetUserUsageTrend(c *gin.Context) {
	startTime, endTime := parseTimeRange(c)
	granularity := c.DefaultQuery("granularity", "day")
	limitStr := c.DefaultQuery("limit", "12")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 12
	}

	trend, hit, err := h.getUserUsageTrendCached(c.Request.Context(), startTime, endTime, granularity, limit)
	if err != nil {
		response.Error(c, 500, "Failed to get user usage trend")
		return
	}
	c.Header("X-Snapshot-Cache", cacheStatusValue(hit))

	response.Success(c, gin.H{
		"trend":       trend,
		"start_date":  startTime.Format("2006-01-02"),
		"end_date":    endTime.Add(-24 * time.Hour).Format("2006-01-02"),
		"granularity": granularity,
	})
}

// BatchUsersUsageRequest represents the request body for batch user usage stats
type BatchUsersUsageRequest struct {
	UserIDs []int64 `json:"user_ids" binding:"required"`
}

var dashboardUsersRankingCache = newSnapshotCache(5 * time.Minute)
var dashboardBatchUsersUsageCache = newSnapshotCache(30 * time.Second)
var dashboardBatchAPIKeysUsageCache = newSnapshotCache(30 * time.Second)

func parseRankingLimit(raw string) int {
	limit, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || limit <= 0 {
		return 12
	}
	if limit > 50 {
		return 50
	}
	return limit
}

// GetUserSpendingRanking handles getting user spending ranking data.
// GET /api/v1/admin/dashboard/users-ranking
func (h *DashboardHandler) GetUserSpendingRanking(c *gin.Context) {
	startTime, endTime := parseTimeRange(c)
	limit := parseRankingLimit(c.DefaultQuery("limit", "12"))

	keyRaw, _ := json.Marshal(struct {
		Start string `json:"start"`
		End   string `json:"end"`
		Limit int    `json:"limit"`
	}{
		Start: startTime.UTC().Format(time.RFC3339),
		End:   endTime.UTC().Format(time.RFC3339),
		Limit: limit,
	})
	cacheKey := string(keyRaw)
	if cached, ok := dashboardUsersRankingCache.Get(cacheKey); ok {
		c.Header("X-Snapshot-Cache", "hit")
		response.Success(c, cached.Payload)
		return
	}

	ranking, err := h.dashboardService.GetUserSpendingRanking(c.Request.Context(), startTime, endTime, limit)
	if err != nil {
		response.Error(c, 500, "Failed to get user spending ranking")
		return
	}

	payload := gin.H{
		"ranking":           ranking.Ranking,
		"total_actual_cost": ranking.TotalActualCost,
		"total_requests":    ranking.TotalRequests,
		"total_tokens":      ranking.TotalTokens,
		"start_date":        startTime.Format("2006-01-02"),
		"end_date":          endTime.Add(-24 * time.Hour).Format("2006-01-02"),
	}
	dashboardUsersRankingCache.Set(cacheKey, payload)
	c.Header("X-Snapshot-Cache", "miss")
	response.Success(c, payload)
}

// GetBatchUsersUsage handles getting usage stats for multiple users
// POST /api/v1/admin/dashboard/users-usage
func (h *DashboardHandler) GetBatchUsersUsage(c *gin.Context) {
	var req BatchUsersUsageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	userIDs := normalizeInt64IDList(req.UserIDs)
	if len(userIDs) == 0 {
		response.Success(c, gin.H{"stats": map[string]any{}})
		return
	}

	keyRaw, _ := json.Marshal(struct {
		UserIDs []int64 `json:"user_ids"`
	}{
		UserIDs: userIDs,
	})
	cacheKey := string(keyRaw)
	if cached, ok := dashboardBatchUsersUsageCache.Get(cacheKey); ok {
		c.Header("X-Snapshot-Cache", "hit")
		response.Success(c, cached.Payload)
		return
	}

	stats, err := h.dashboardService.GetBatchUserUsageStats(c.Request.Context(), userIDs, time.Time{}, time.Time{})
	if err != nil {
		response.Error(c, 500, "Failed to get user usage stats")
		return
	}

	payload := gin.H{"stats": stats}
	dashboardBatchUsersUsageCache.Set(cacheKey, payload)
	c.Header("X-Snapshot-Cache", "miss")
	response.Success(c, payload)
}

// BatchAPIKeysUsageRequest represents the request body for batch api key usage stats
type BatchAPIKeysUsageRequest struct {
	APIKeyIDs []int64 `json:"api_key_ids" binding:"required"`
}

// GetBatchAPIKeysUsage handles getting usage stats for multiple API keys
// POST /api/v1/admin/dashboard/api-keys-usage
func (h *DashboardHandler) GetBatchAPIKeysUsage(c *gin.Context) {
	var req BatchAPIKeysUsageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	apiKeyIDs := normalizeInt64IDList(req.APIKeyIDs)
	if len(apiKeyIDs) == 0 {
		response.Success(c, gin.H{"stats": map[string]any{}})
		return
	}

	keyRaw, _ := json.Marshal(struct {
		APIKeyIDs []int64 `json:"api_key_ids"`
	}{
		APIKeyIDs: apiKeyIDs,
	})
	cacheKey := string(keyRaw)
	if cached, ok := dashboardBatchAPIKeysUsageCache.Get(cacheKey); ok {
		c.Header("X-Snapshot-Cache", "hit")
		response.Success(c, cached.Payload)
		return
	}

	stats, err := h.dashboardService.GetBatchAPIKeyUsageStats(c.Request.Context(), apiKeyIDs, time.Time{}, time.Time{})
	if err != nil {
		response.Error(c, 500, "Failed to get API key usage stats")
		return
	}

	payload := gin.H{"stats": stats}
	dashboardBatchAPIKeysUsageCache.Set(cacheKey, payload)
	c.Header("X-Snapshot-Cache", "miss")
	response.Success(c, payload)
}

// GetUserBreakdown handles getting per-user usage breakdown within a dimension.
// GET /api/v1/admin/dashboard/user-breakdown
// Query params: start_date, end_date, group_id, model, endpoint, endpoint_type, limit
func (h *DashboardHandler) GetUserBreakdown(c *gin.Context) {
	startTime, endTime := parseTimeRange(c)

	dim := usagestats.UserBreakdownDimension{}
	if v := c.Query("group_id"); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			dim.GroupID = id
		}
	}
	dim.Model = c.Query("model")
	rawModelSource := strings.TrimSpace(c.DefaultQuery("model_source", usagestats.ModelSourceRequested))
	if !usagestats.IsValidModelSource(rawModelSource) {
		response.BadRequest(c, "Invalid model_source, use requested/upstream/mapping")
		return
	}
	dim.ModelType = rawModelSource
	dim.Endpoint = c.Query("endpoint")
	dim.EndpointType = c.DefaultQuery("endpoint_type", "inbound")

	// Additional filter conditions
	if v := c.Query("user_id"); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			dim.UserID = id
		}
	}
	if v := c.Query("api_key_id"); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			dim.APIKeyID = id
		}
	}
	if v := c.Query("account_id"); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			dim.AccountID = id
		}
	}
	if v := c.Query("request_type"); v != "" {
		if rt, err := strconv.ParseInt(v, 10, 16); err == nil {
			rtVal := int16(rt)
			dim.RequestType = &rtVal
		}
	}
	if v := c.Query("stream"); v != "" {
		if s, err := strconv.ParseBool(v); err == nil {
			dim.Stream = &s
		}
	}
	if v := c.Query("billing_type"); v != "" {
		if bt, err := strconv.ParseInt(v, 10, 8); err == nil {
			btVal := int8(bt)
			dim.BillingType = &btVal
		}
	}

	limit := 50
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}

	stats, err := h.dashboardService.GetUserBreakdownStats(
		c.Request.Context(), startTime, endTime, dim, limit,
	)
	if err != nil {
		response.Error(c, 500, "Failed to get user breakdown stats")
		return
	}

	response.Success(c, gin.H{
		"users":      stats,
		"start_date": startTime.Format("2006-01-02"),
		"end_date":   endTime.Add(-24 * time.Hour).Format("2006-01-02"),
	})
}
