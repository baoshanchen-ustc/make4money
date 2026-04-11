package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	OpsRuntimeUsageLogComponent                   = "ops.runtime.usage_log"
	OpsRuntimeUsageLogSummaryComponent            = "ops.runtime.usage_log.summary"
	OpsRuntimeSchedulerOutboxSummaryComponent     = "ops.runtime.scheduler_outbox.summary"
	OpsRuntimeUsageWorkerSummaryComponent         = "ops.runtime.usage_worker.summary"
	OpsRuntimeRedisPoolSummaryComponent           = "ops.runtime.redis_pool.summary"
	OpsRuntimeStorageGovernanceSummaryComponent   = "ops.runtime.storage_governance.summary"
	OpsRuntimeBillingCompensationComponent        = "ops.runtime.billing_compensation"
	OpsRuntimeBillingCompensationSummaryComponent = "ops.runtime.billing_compensation.summary"
	opsDashboardRuntimeAnomalyLimit               = 6
)

const tokenRefreshFailureDetailsLimit = 20

const (
	tokenRefreshObservabilityTitle        = "Token refresh failures detected"
	schedulerCheckpointObservabilityTitle = "Scheduler checkpoint issues detected"
)

const (
	resourceBudgetDBMaxOpenGuardrail           = 500
	resourceBudgetDBIdleRatioGuardrail         = 0.90
	resourceBudgetRedisPoolGuardrail           = 512
	resourceBudgetRedisMinIdleRatioGuardrail   = 0.80
	resourceBudgetHTTPMaxIdleGuardrail         = 1024
	resourceBudgetHTTPIdlePerHostGuardrail     = 512
	resourceBudgetHTTPMaxConnsPerHostGuardrail = 1024
	resourceBudgetHTTPClientCacheGuardrail     = 5000
	resourceBudgetLowUsageThresholdPercent     = 25.0
	slowPathRequestThresholdMs                 = 1000
	slowPathP99WarningMs                       = 2000
)

func (s *OpsService) GetDashboardOverview(ctx context.Context, filter *OpsDashboardFilter) (*OpsDashboardOverview, error) {
	if err := s.RequireMonitoringEnabled(ctx); err != nil {
		return nil, err
	}
	if s.opsRepo == nil {
		return nil, infraerrors.ServiceUnavailable("OPS_REPO_UNAVAILABLE", "Ops repository not available")
	}
	if filter == nil {
		return nil, infraerrors.BadRequest("OPS_FILTER_REQUIRED", "filter is required")
	}
	if filter.StartTime.IsZero() || filter.EndTime.IsZero() {
		return nil, infraerrors.BadRequest("OPS_TIME_RANGE_REQUIRED", "start_time/end_time are required")
	}
	if filter.StartTime.After(filter.EndTime) {
		return nil, infraerrors.BadRequest("OPS_TIME_RANGE_INVALID", "start_time must be <= end_time")
	}

	// Resolve query mode (requested via query param, or DB default).
	filter.QueryMode = s.resolveOpsQueryMode(ctx, filter.QueryMode)

	overview, err := s.opsRepo.GetDashboardOverview(ctx, filter)
	if err != nil && shouldFallbackOpsPreagg(filter, err) {
		rawFilter := cloneOpsFilterWithMode(filter, OpsQueryModeRaw)
		overview, err = s.opsRepo.GetDashboardOverview(ctx, rawFilter)
	}
	if err != nil {
		if errors.Is(err, ErrOpsPreaggregatedNotPopulated) {
			return nil, infraerrors.Conflict("OPS_PREAGG_NOT_READY", "Pre-aggregated ops metrics are not populated yet")
		}
		return nil, err
	}

	// Best-effort system health + jobs; dashboard metrics should still render if these are missing.
	if metrics, err := s.opsRepo.GetLatestSystemMetrics(ctx, 1); err == nil {
		// Attach config-derived limits so the UI can show "current / max" for connection pools.
		// These are best-effort and should never block the dashboard rendering.
		if s != nil && s.cfg != nil {
			if s.cfg.Database.MaxOpenConns > 0 {
				metrics.DBMaxOpenConns = intPtr(s.cfg.Database.MaxOpenConns)
			}
			if s.cfg.Redis.PoolSize > 0 {
				metrics.RedisPoolSize = intPtr(s.cfg.Redis.PoolSize)
			}
		}
		overview.SystemMetrics = metrics
	} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
		log.Printf("[Ops] GetLatestSystemMetrics failed: %v", err)
	}

	if heartbeats, err := s.opsRepo.ListJobHeartbeats(ctx); err == nil {
		overview.JobHeartbeats = heartbeats
	} else {
		log.Printf("[Ops] ListJobHeartbeats failed: %v", err)
	}

	anomalies, err := s.listRecentRuntimeAnomalies(ctx, filter, opsDashboardRuntimeAnomalyLimit)
	if len(anomalies) > 0 {
		overview.RuntimeAnomalies = anomalies
	}
	if err != nil {
		log.Printf("[Ops] ListRecentRuntimeAnomalies failed: %v", err)
	}

	overview.TokenRefreshSummary = s.TokenRefreshFailureSummary(ctx, filter.Platform, filter.GroupID)
	overview.SchedulerCheckpoint = SchedulerCheckpointSummary()
	overview.ResourceBudgetSummary = s.ResourceBudgetSummary(overview.SystemMetrics)
	cleanupStats := SnapshotCleanupStats()
	overview.CleanupStats = &cleanupStats
	usageCleanupStats := SnapshotUsageCleanupStats()
	overview.UsageCleanupStats = &usageCleanupStats
	overview.StorageGovernance = s.StorageGovernanceSummary(cleanupStats, usageCleanupStats, overview.JobHeartbeats)
	overview.SlowPathDiagnostics = buildSlowPathDiagnostics(overview)
	overview.HealthScore = computeDashboardHealthScore(time.Now().UTC(), overview)
	if notes := s.collectObservabilityNotes(filter, overview); len(notes) > 0 {
		overview.Observability = append(overview.Observability, notes...)
	}
	budgetNotes := s.collectResourceBudgetNotices()
	budgetNotes = append(budgetNotes, resourceBudgetGuardrailNotes(overview.ResourceBudgetSummary)...)
	if len(budgetNotes) > 0 {
		overview.Observability = append(overview.Observability, budgetNotes...)
	}

	return overview, nil
}

func (s *OpsService) TokenRefreshFailureSummary(ctx context.Context, platformFilter string, groupIDFilter *int64) *OpsTokenRefreshSummary {
	if s == nil || s.opsRepo == nil {
		return nil
	}
	if s.tokenRefreshSummaryFn != nil {
		return s.tokenRefreshSummaryFn(ctx, platformFilter, groupIDFilter)
	}
	platformStats, groupStats, accountStats, _, err := s.GetAccountAvailabilityStats(ctx, platformFilter, groupIDFilter)
	if err != nil {
		return nil
	}
	summary := &OpsTokenRefreshSummary{
		Platform: make(map[string]int64),
		Group:    make(map[int64]int64),
	}
	var total int64
	for name, platform := range platformStats {
		if platform == nil {
			continue
		}
		if platform.TokenRefreshFailureCount > 0 {
			summary.Platform[name] = platform.TokenRefreshFailureCount
			total += platform.TokenRefreshFailureCount
		}
	}
	for id, group := range groupStats {
		if group == nil {
			continue
		}
		if group.TokenRefreshFailureCount > 0 {
			summary.Group[id] = group.TokenRefreshFailureCount
		}
	}
	if len(accountStats) > 0 {
		failures := make([]*OpsTokenRefreshFailure, 0, len(accountStats))
		for _, acct := range accountStats {
			if acct == nil {
				continue
			}
			if acct.TokenRefreshFailureReason == "" && acct.TokenRefreshFailureClass == "" && acct.TokenRefreshFailedAt == "" {
				continue
			}
			failures = append(failures, &OpsTokenRefreshFailure{
				AccountID:   acct.AccountID,
				AccountName: acct.AccountName,
				Platform:    acct.Platform,
				GroupID:     acct.GroupID,
				GroupName:   acct.GroupName,
				Reason:      acct.TokenRefreshFailureReason,
				Class:       acct.TokenRefreshFailureClass,
				At:          acct.TokenRefreshFailedAt,
			})
		}
		if len(failures) > 0 {
			sort.Slice(failures, func(i, j int) bool {
				if failures[i].AccountID == failures[j].AccountID {
					return failures[i].At < failures[j].At
				}
				return failures[i].AccountID < failures[j].AccountID
			})
			if len(failures) > tokenRefreshFailureDetailsLimit {
				failures = failures[:tokenRefreshFailureDetailsLimit]
			}
			summary.Failures = failures
		}
	}
	if total == 0 && len(summary.Platform) == 0 && len(summary.Group) == 0 {
		return nil
	}
	summary.Total = total
	return summary
}

func SchedulerCheckpointSummary() *OpsSchedulerCheckpointSummary {
	metrics := SnapshotSchedulerOutboxRuntimeMetrics()
	return &OpsSchedulerCheckpointSummary{
		LastCheckpointWatermark: metrics.LastCheckpointWatermark,
		CheckpointFallbackTotal: metrics.CheckpointFallbackTotal,
		CheckpointReadFailures:  metrics.CheckpointReadFailureTotal,
		CheckpointWriteFailures: metrics.CheckpointWriteFailureTotal,
	}
}

func (s *OpsService) ResourceBudgetSummary(metrics *OpsSystemMetricsSnapshot) *OpsResourceBudgetSummary {
	if s == nil || s.cfg == nil {
		return nil
	}
	cfg := s.cfg
	dbSummary := &OpsDatabaseBudgetSummary{}
	if cfg.Database.MaxOpenConns > 0 {
		dbSummary.MaxOpenConns = intPtr(cfg.Database.MaxOpenConns)
	}
	if cfg.Database.MaxIdleConns > 0 {
		dbSummary.MaxIdleConns = intPtr(cfg.Database.MaxIdleConns)
	}
	if cfg.Database.ConnMaxLifetimeMinutes > 0 {
		dbSummary.ConnMaxLifetimeMinutes = intPtr(cfg.Database.ConnMaxLifetimeMinutes)
	}
	if cfg.Database.ConnMaxIdleTimeMinutes > 0 {
		dbSummary.ConnMaxIdleTimeMinutes = intPtr(cfg.Database.ConnMaxIdleTimeMinutes)
	}
	redisSummary := &OpsRedisBudgetSummary{}
	if cfg.Redis.PoolSize > 0 {
		redisSummary.PoolSize = intPtr(cfg.Redis.PoolSize)
	}
	if cfg.Redis.MinIdleConns > 0 {
		redisSummary.MinIdleConns = intPtr(cfg.Redis.MinIdleConns)
	}
	if cfg.Redis.DialTimeoutSeconds > 0 {
		redisSummary.DialTimeoutSeconds = intPtr(cfg.Redis.DialTimeoutSeconds)
	}
	if cfg.Redis.ReadTimeoutSeconds > 0 {
		redisSummary.ReadTimeoutSeconds = intPtr(cfg.Redis.ReadTimeoutSeconds)
	}
	if cfg.Redis.WriteTimeoutSeconds > 0 {
		redisSummary.WriteTimeoutSeconds = intPtr(cfg.Redis.WriteTimeoutSeconds)
	}
	if metrics != nil {
		dbSummary.Active = metrics.DBConnActive
		dbSummary.Idle = metrics.DBConnIdle
		dbSummary.Waiting = metrics.DBConnWaiting
		dbSummary.UsagePercent = usagePercent(metrics.DBConnActive, cfg.Database.MaxOpenConns)
		redisSummary.Total = metrics.RedisConnTotal
		redisSummary.Idle = metrics.RedisConnIdle
		redisSummary.UsagePercent = usagePercent(metrics.RedisConnTotal, cfg.Redis.PoolSize)
	}
	httpSummary := &OpsHTTPUpstreamBudgetSummary{}
	if cfg.Gateway.MaxIdleConns > 0 {
		httpSummary.MaxIdleConns = intPtr(cfg.Gateway.MaxIdleConns)
	}
	if cfg.Gateway.MaxIdleConnsPerHost > 0 {
		httpSummary.MaxIdleConnsPerHost = intPtr(cfg.Gateway.MaxIdleConnsPerHost)
	}
	if cfg.Gateway.MaxConnsPerHost > 0 {
		httpSummary.MaxConnsPerHost = intPtr(cfg.Gateway.MaxConnsPerHost)
	}
	if cfg.Gateway.MaxUpstreamClients > 0 {
		httpSummary.MaxUpstreamClients = intPtr(cfg.Gateway.MaxUpstreamClients)
	}
	if cfg.Gateway.ClientIdleTTLSeconds > 0 {
		httpSummary.ClientIdleTTLSeconds = intPtr(cfg.Gateway.ClientIdleTTLSeconds)
	}
	if cfg.Gateway.ConcurrencySlotTTLMinutes > 0 {
		httpSummary.ConcurrencySlotTTLMinute = intPtr(cfg.Gateway.ConcurrencySlotTTLMinutes)
	}
	if cfg.Gateway.SessionIdleTimeoutMinutes > 0 {
		httpSummary.SessionIdleTimeoutMinute = intPtr(cfg.Gateway.SessionIdleTimeoutMinutes)
	}
	storageSummary := &OpsStorageGovernanceBudgetSummary{
		MaxRowsEnabled: cfg.Ops.Cleanup.MaxRowsEnabled,
		MaxRowsDryRun:  cfg.Ops.Cleanup.MaxRowsDryRun,
	}
	if cfg.Ops.Cleanup.SystemLogMaxRows > 0 {
		storageSummary.OpsSystemLogsMaxRows = int64PtrOpsDashboard(cfg.Ops.Cleanup.SystemLogMaxRows)
	}
	if cfg.Ops.Cleanup.ErrorLogMaxRows > 0 {
		storageSummary.OpsErrorLogsMaxRows = int64PtrOpsDashboard(cfg.Ops.Cleanup.ErrorLogMaxRows)
	}
	if cfg.DashboardAgg.Retention.UsageLogsMaxRows > 0 {
		storageSummary.UsageLogsMaxRows = int64PtrOpsDashboard(cfg.DashboardAgg.Retention.UsageLogsMaxRows)
	}
	recommendations := buildResourceBudgetRecommendations(cfg, dbSummary, redisSummary, httpSummary)
	return &OpsResourceBudgetSummary{
		Database:          dbSummary,
		Redis:             redisSummary,
		HTTPUpstream:      httpSummary,
		StorageGovernance: storageSummary,
		Recommendations:   recommendations,
	}
}

func resourceBudgetGuardrailNotes(summary *OpsResourceBudgetSummary) []*OpsObservabilityNotice {
	if summary == nil {
		return nil
	}
	notes := make([]*OpsObservabilityNotice, 0, 2)
	if summary.Database != nil && summary.Database.UsagePercent != nil {
		if *summary.Database.UsagePercent >= 95 {
			notes = append(notes, &OpsObservabilityNotice{
				Level:      "warning",
				Title:      "DB connections near pool limit",
				Detail:     fmt.Sprintf("Active connections %.1f%% of max_open_conns=%d", *summary.Database.UsagePercent, ptrToInt(summary.Database.MaxOpenConns)),
				Suggestion: "Review connection churn or scale Postgres before additional bursty jobs hit the pool.",
			})
		}
	}
	if summary.Redis != nil && summary.Redis.UsagePercent != nil {
		if *summary.Redis.UsagePercent >= 95 {
			notes = append(notes, &OpsObservabilityNotice{
				Level:      "warning",
				Title:      "Redis pool saturation risk",
				Detail:     fmt.Sprintf("Current usage %.1f%% of pool_size=%d", *summary.Redis.UsagePercent, ptrToInt(summary.Redis.PoolSize)),
				Suggestion: "Consider throttling Redis-heavy workloads or gradually raising redis.pool_size while monitoring stall counts.",
			})
		}
	}
	return notes
}

func ptrToInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

func usagePercent(current *int, limit int) *float64 {
	if current == nil || limit <= 0 {
		return nil
	}
	pct := float64(*current) / float64(limit) * 100
	if pct < 0 {
		pct = 0
	}
	if pct > 200 {
		pct = 200
	}
	return float64Ptr(roundTo1DP(pct))
}

func int64PtrOpsDashboard(v int64) *int64 { return &v }

func (s *OpsService) StorageGovernanceSummary(cleanupStats CleanupStats, usageCleanupStats UsageCleanupStats, heartbeats []*OpsJobHeartbeat) *OpsStorageGovernanceSummary {
	if s == nil || s.cfg == nil {
		return nil
	}
	cfg := s.cfg
	opsCleanupHeartbeat := summarizeJobHeartbeat(findJobHeartbeat(heartbeats, opsCleanupJobName))
	return &OpsStorageGovernanceSummary{
		OpsCleanup: &OpsCleanupGovernanceSummary{
			Enabled:                    cfg.Ops.Cleanup.Enabled,
			ErrorLogRetentionDays:      cfg.Ops.Cleanup.ErrorLogRetentionDays,
			MinuteMetricsRetentionDays: cfg.Ops.Cleanup.MinuteMetricsRetentionDays,
			HourlyMetricsRetentionDays: cfg.Ops.Cleanup.HourlyMetricsRetentionDays,
			SystemLogMaxRows:           cfg.Ops.Cleanup.SystemLogMaxRows,
			ErrorLogMaxRows:            cfg.Ops.Cleanup.ErrorLogMaxRows,
			MaxRowsEnabled:             cfg.Ops.Cleanup.MaxRowsEnabled,
			MaxRowsDryRun:              cfg.Ops.Cleanup.MaxRowsDryRun,
			SystemLogRows:              cleanupStats.SystemLogRows,
			ErrorLogRows:               cleanupStats.ErrorLogRows,
			MaxRowsHit:                 cleanupStats.MaxRowsHit,
			Heartbeat:                  opsCleanupHeartbeat,
		},
		UsageCleanup: &OpsUsageCleanupGovernanceInfo{
			Enabled:                cfg.UsageCleanup.Enabled,
			MaxRangeDays:           cfg.UsageCleanup.MaxRangeDays,
			BatchSize:              cfg.UsageCleanup.BatchSize,
			WorkerIntervalSeconds:  cfg.UsageCleanup.WorkerIntervalSeconds,
			TaskTimeoutSeconds:     cfg.UsageCleanup.TaskTimeoutSeconds,
			UsageLogsRetentionDays: cfg.DashboardAgg.Retention.UsageLogsDays,
			UsageLogsMaxRows:       cfg.DashboardAgg.Retention.UsageLogsMaxRows,
			LastTaskID:             usageCleanupStats.LastTaskID,
			LastDeletedRows:        usageCleanupStats.LastDeletedRows,
		},
	}
}

func findJobHeartbeat(heartbeats []*OpsJobHeartbeat, jobName string) *OpsJobHeartbeat {
	for _, hb := range heartbeats {
		if hb != nil && hb.JobName == jobName {
			return hb
		}
	}
	return nil
}

func summarizeJobHeartbeat(hb *OpsJobHeartbeat) *OpsJobHeartbeatSummary {
	if hb == nil {
		return nil
	}
	return &OpsJobHeartbeatSummary{
		LastRunAt:      hb.LastRunAt,
		LastSuccessAt:  hb.LastSuccessAt,
		LastErrorAt:    hb.LastErrorAt,
		LastError:      hb.LastError,
		LastDurationMs: hb.LastDurationMs,
		LastResult:     hb.LastResult,
		UpdatedAt:      hb.UpdatedAt,
	}
}

func buildSlowPathDiagnostics(overview *OpsDashboardOverview) *OpsSlowPathDiagnostics {
	if overview == nil {
		return nil
	}
	diag := &OpsSlowPathDiagnostics{
		SlowRequestThresholdMs: slowPathRequestThresholdMs,
		RequestDetailsEndpoint: "/api/v1/admin/ops/requests?sort=duration_desc&min_duration_ms=1000",
		RequestDetailsHint:     "Use the request details endpoint with min_duration_ms to drill into the slowest requests in the current window.",
		DurationP95Ms:          overview.Duration.P95,
		DurationP99Ms:          overview.Duration.P99,
		DurationMaxMs:          overview.Duration.Max,
		TTFTP95Ms:              overview.TTFT.P95,
		TTFTP99Ms:              overview.TTFT.P99,
	}
	if overview.SystemMetrics != nil {
		diag.DBConnWaiting = overview.SystemMetrics.DBConnWaiting
		diag.SQLObservabilityReady = true
		diag.SQLObservabilityDetail = "System metrics are available for this window; use DB wait counts and ops request drilldowns before running ad-hoc SQL."
	} else {
		diag.SQLObservabilityReady = false
		diag.SQLObservabilityDetail = "System metrics are unavailable for this window, so SQL-level observability is degraded; confirm pg_stat_statements ingestion first."
	}
	signals := make([]string, 0, 4)
	if overview.Duration.P95 != nil && *overview.Duration.P95 >= slowPathRequestThresholdMs {
		signals = append(signals, fmt.Sprintf("duration_p95_ge_%dms", slowPathRequestThresholdMs))
	}
	if overview.Duration.P99 != nil && *overview.Duration.P99 >= slowPathP99WarningMs {
		signals = append(signals, fmt.Sprintf("duration_p99_ge_%dms", slowPathP99WarningMs))
	}
	if overview.TTFT.P95 != nil && *overview.TTFT.P95 >= slowPathRequestThresholdMs {
		signals = append(signals, fmt.Sprintf("ttft_p95_ge_%dms", slowPathRequestThresholdMs))
	}
	if overview.SystemMetrics != nil && overview.SystemMetrics.DBConnWaiting != nil && *overview.SystemMetrics.DBConnWaiting > 0 {
		signals = append(signals, "db_conn_waiting_positive")
	}
	if len(signals) > 0 {
		diag.SlowSignals = signals
	}
	return diag
}

func buildResourceBudgetRecommendations(
	cfg *config.Config,
	dbSummary *OpsDatabaseBudgetSummary,
	redisSummary *OpsRedisBudgetSummary,
	httpSummary *OpsHTTPUpstreamBudgetSummary,
) []*OpsBudgetRecommendation {
	if cfg == nil {
		return nil
	}
	recommendations := make([]*OpsBudgetRecommendation, 0, 6)

	if cfg.Database.MaxOpenConns > resourceBudgetDBMaxOpenGuardrail {
		recommendations = append(recommendations, &OpsBudgetRecommendation{
			Area:      "database",
			Level:     "info",
			Current:   fmt.Sprintf("max_open_conns=%d", cfg.Database.MaxOpenConns),
			Suggested: fmt.Sprintf("phase 1: reduce toward <= %d and watch db_conn_waiting / WaitCount deltas", resourceBudgetDBMaxOpenGuardrail),
			Reason:    "Configured DB pool cap is above the service guardrail and may over-reserve shared PostgreSQL connections under burst load.",
		})
	}
	if cfg.Database.MaxOpenConns > 0 && cfg.Database.MaxIdleConns > 0 {
		idleRatio := float64(cfg.Database.MaxIdleConns) / float64(cfg.Database.MaxOpenConns)
		if idleRatio >= resourceBudgetDBIdleRatioGuardrail {
			recommendations = append(recommendations, &OpsBudgetRecommendation{
				Area:      "database",
				Level:     "info",
				Current:   fmt.Sprintf("max_idle_conns=%d (%.0f%% of max_open_conns)", cfg.Database.MaxIdleConns, idleRatio*100),
				Suggested: "phase 1: trim idle connections below 90% of max_open_conns before shrinking further",
				Reason:    "High idle ratio keeps many database connections warm even when concurrency is low, which reduces shared headroom.",
			})
		}
	}
	if dbSummary != nil && dbSummary.UsagePercent != nil && *dbSummary.UsagePercent < resourceBudgetLowUsageThresholdPercent && cfg.Database.MaxOpenConns > 0 {
		recommendations = append(recommendations, &OpsBudgetRecommendation{
			Area:      "database",
			Level:     "info",
			Current:   fmt.Sprintf("observed db usage %.1f%% of configured pool", *dbSummary.UsagePercent),
			Suggested: "phase 1: trial a 20-25% pool reduction during off-peak, then compare db_conn_waiting before and after",
			Reason:    "Observed active DB usage is well below the configured cap, so there may be room to reclaim capacity without hurting throughput.",
		})
	}

	if cfg.Redis.PoolSize > resourceBudgetRedisPoolGuardrail {
		recommendations = append(recommendations, &OpsBudgetRecommendation{
			Area:      "redis",
			Level:     "info",
			Current:   fmt.Sprintf("pool_size=%d", cfg.Redis.PoolSize),
			Suggested: fmt.Sprintf("phase 1: reduce toward <= %d and watch stalls/timeouts after each step", resourceBudgetRedisPoolGuardrail),
			Reason:    "Configured Redis pool exceeds the service guardrail and can monopolize shared Redis connections on a single host.",
		})
	}
	if cfg.Redis.PoolSize > 0 && cfg.Redis.MinIdleConns > 0 {
		idleRatio := float64(cfg.Redis.MinIdleConns) / float64(cfg.Redis.PoolSize)
		if idleRatio >= resourceBudgetRedisMinIdleRatioGuardrail {
			recommendations = append(recommendations, &OpsBudgetRecommendation{
				Area:      "redis",
				Level:     "info",
				Current:   fmt.Sprintf("min_idle_conns=%d (%.0f%% of pool_size)", cfg.Redis.MinIdleConns, idleRatio*100),
				Suggested: "phase 1: lower min_idle_conns before reducing pool_size so hot connections stay available with less reservation",
				Reason:    "High Redis idle reservation can hold unnecessary sockets under steady load and hides the real working-set size.",
			})
		}
	}
	if redisSummary != nil && redisSummary.UsagePercent != nil && *redisSummary.UsagePercent < resourceBudgetLowUsageThresholdPercent && cfg.Redis.PoolSize > 0 {
		recommendations = append(recommendations, &OpsBudgetRecommendation{
			Area:      "redis",
			Level:     "info",
			Current:   fmt.Sprintf("observed redis usage %.1f%% of configured pool", *redisSummary.UsagePercent),
			Suggested: "phase 1: trial a 20-25% pool reduction and compare stalls/timeouts before moving further",
			Reason:    "Observed Redis pool usage is low relative to the configured cap, suggesting room for a cautious gray shrink.",
		})
	}

	if cfg.Gateway.MaxIdleConns > resourceBudgetHTTPMaxIdleGuardrail ||
		cfg.Gateway.MaxIdleConnsPerHost > resourceBudgetHTTPIdlePerHostGuardrail ||
		cfg.Gateway.MaxConnsPerHost > resourceBudgetHTTPMaxConnsPerHostGuardrail ||
		cfg.Gateway.MaxUpstreamClients > resourceBudgetHTTPClientCacheGuardrail {
		current := fmt.Sprintf("max_idle=%d max_idle_per_host=%d max_conns_per_host=%d max_upstream_clients=%d",
			cfg.Gateway.MaxIdleConns,
			cfg.Gateway.MaxIdleConnsPerHost,
			cfg.Gateway.MaxConnsPerHost,
			cfg.Gateway.MaxUpstreamClients,
		)
		recommendations = append(recommendations, &OpsBudgetRecommendation{
			Area:    "http_upstream",
			Level:   "info",
			Current: current,
			Suggested: fmt.Sprintf("phase 1: shrink toward idle<=%d per_host<=%d conns<=%d clients<=%d with canary traffic",
				resourceBudgetHTTPMaxIdleGuardrail,
				resourceBudgetHTTPIdlePerHostGuardrail,
				resourceBudgetHTTPMaxConnsPerHostGuardrail,
				resourceBudgetHTTPClientCacheGuardrail),
			Reason: "Aggressive upstream HTTP pools can pin sockets and client cache entries, reducing room for burst concurrency on the same host.",
		})
	}

	if len(recommendations) == 0 {
		return nil
	}
	return recommendations
}

func (s *OpsService) listRecentRuntimeAnomalies(ctx context.Context, filter *OpsDashboardFilter, limit int) ([]*OpsSystemLog, error) {
	if s == nil || s.opsRepo == nil || filter == nil || limit <= 0 {
		return nil, nil
	}

	components := []string{
		OpsRuntimeUsageLogComponent,
		OpsRuntimeUsageLogSummaryComponent,
		OpsRuntimeBillingCompensationComponent,
		OpsRuntimeBillingCompensationSummaryComponent,
		OpsRuntimeUsageWorkerSummaryComponent,
		OpsRuntimeRedisPoolSummaryComponent,
		OpsRuntimeSchedulerOutboxSummaryComponent,
		OpsRuntimeStorageGovernanceSummaryComponent,
		"ops.runtime.cleanup.summary",
		"ops.runtime.cleanup.usage.summary",
	}
	logs := make([]*OpsSystemLog, 0, len(components)*limit)
	var errs []error
	for _, component := range components {
		list, err := s.opsRepo.ListSystemLogs(ctx, &OpsSystemLogFilter{
			Page:      1,
			PageSize:  limit,
			Level:     "warn",
			Component: component,
			StartTime: &filter.StartTime,
			EndTime:   &filter.EndTime,
		})
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", component, err))
			continue
		}
		if list != nil {
			logs = append(logs, list.Logs...)
		}
	}
	if len(logs) == 0 {
		if len(errs) > 0 {
			return nil, errors.Join(errs...)
		}
		return nil, nil
	}
	sort.SliceStable(logs, func(i, j int) bool {
		return logs[i].CreatedAt.After(logs[j].CreatedAt)
	})
	if len(logs) > limit {
		logs = logs[:limit]
	}
	if len(errs) > 0 {
		return logs, errors.Join(errs...)
	}
	return logs, nil
}

func (s *OpsService) resolveOpsQueryMode(ctx context.Context, requested OpsQueryMode) OpsQueryMode {
	if requested.IsValid() {
		// Allow "auto" to be disabled via config until preagg is proven stable in production.
		// Forced `preagg` via query param still works.
		if requested == OpsQueryModeAuto && s != nil && s.cfg != nil && !s.cfg.Ops.UsePreaggregatedTables {
			return OpsQueryModeRaw
		}
		return requested
	}

	mode := OpsQueryModeAuto
	if s != nil && s.settingRepo != nil {
		if raw, err := s.settingRepo.GetValue(ctx, SettingKeyOpsQueryModeDefault); err == nil {
			mode = ParseOpsQueryMode(raw)
		}
	}

	if mode == OpsQueryModeAuto && s != nil && s.cfg != nil && !s.cfg.Ops.UsePreaggregatedTables {
		return OpsQueryModeRaw
	}
	return mode
}

func (s *OpsService) collectResourceBudgetNotices() []*OpsObservabilityNotice {
	if s == nil || s.cfg == nil {
		return nil
	}
	var notices []*OpsObservabilityNotice
	const (
		dbMaxOpenThreshold           = 500
		redisPoolSizeThreshold       = 512
		redisMinIdleRatioThreshold   = 0.85
		httpMaxIdleThreshold         = 1024
		httpMaxIdlePerHostThreshold  = 512
		httpMaxConnsPerHostThreshold = 1024
		httpMaxUpstreamClientsHint   = 5000
	)

	if s.cfg.Database.MaxOpenConns > dbMaxOpenThreshold {
		notices = append(notices, &OpsObservabilityNotice{
			Level:      "info",
			Title:      "Database pool sized aggressively",
			Detail:     fmt.Sprintf("MaxOpenConns=%d may exceed deployment capacity; idle/conn usage should stay under %d.", s.cfg.Database.MaxOpenConns, dbMaxOpenThreshold),
			Suggestion: "Consider scaling the pool to observed concurrent connections before expanding pg_stat_statements coverage.",
		})
	}

	redisCfg := s.cfg.Redis
	if redisCfg.PoolSize > redisPoolSizeThreshold {
		notices = append(notices, &OpsObservabilityNotice{
			Level:      "info",
			Title:      "Redis pool configured high",
			Detail:     fmt.Sprintf("PoolSize=%d can tie up many connections; watch for ratelimit/memory pressure.", redisCfg.PoolSize),
			Suggestion: "Tune PoolSize/MinIdle down while monitoring redis timeout/stall metrics.",
		})
	}
	if redisCfg.PoolSize > 0 && redisCfg.MinIdleConns > 0 {
		ratio := float64(redisCfg.MinIdleConns) / float64(redisCfg.PoolSize)
		if ratio >= redisMinIdleRatioThreshold {
			notices = append(notices, &OpsObservabilityNotice{
				Level:      "info",
				Title:      "Redis min idle ratio high",
				Detail:     fmt.Sprintf("MinIdleConns=%d is %.0f%% of PoolSize=%d.", redisCfg.MinIdleConns, ratio*100, redisCfg.PoolSize),
				Suggestion: "Ensure idle connections stay busy by lowering MinIdle or increasing request fanout before reusing redis shards.",
			})
		}
	}

	gw := s.cfg.Gateway
	if gw.MaxIdleConns > httpMaxIdleThreshold ||
		gw.MaxIdleConnsPerHost > httpMaxIdlePerHostThreshold ||
		gw.MaxConnsPerHost > httpMaxConnsPerHostThreshold {
		notices = append(notices, &OpsObservabilityNotice{
			Level: "info",
			Title: "HTTP upstream pool configured aggressively",
			Detail: fmt.Sprintf("MaxIdleConns=%d MaxIdleConnsPerHost=%d MaxConnsPerHost=%d; large pools can hold sockets indefinitely.",
				gw.MaxIdleConns, gw.MaxIdleConnsPerHost, gw.MaxConnsPerHost),
			Suggestion: "Shrink HTTP pool settings toward gateway traffic baselines before adding more outbound connections.",
		})
	}
	if gw.MaxUpstreamClients > httpMaxUpstreamClientsHint {
		notices = append(notices, &OpsObservabilityNotice{
			Level:      "info",
			Title:      "HTTP client cache wide open",
			Detail:     fmt.Sprintf("MaxUpstreamClients=%d may keep thousands of idle clients; this uses shared connection limits.", gw.MaxUpstreamClients),
			Suggestion: "Limit cached clients and rely on per-account isolation to avoid cross-account resource exhaustion.",
		})
	}
	return notices
}

func (s *OpsService) collectObservabilityNotes(filter *OpsDashboardFilter, overview *OpsDashboardOverview) []*OpsObservabilityNotice {
	if overview == nil {
		return nil
	}
	notes := make([]*OpsObservabilityNotice, 0, 2)
	if overview.SystemMetrics == nil {
		notes = append(notes, &OpsObservabilityNotice{
			Level:      "warning",
			Title:      "SQL metrics unavailable",
			Detail:     "System metrics (pg_stat_statements) were not collected for this window, so SQL-level observability is degraded.",
			Suggestion: "Enable pg_stat_statements and ensure ops.system_metrics ingestion is healthy before relying on SQL insights.",
		})
	} else if overview.SystemMetrics.DBOK != nil && !*overview.SystemMetrics.DBOK {
		notes = append(notes, &OpsObservabilityNotice{
			Level:      "warning",
			Title:      "Database reporting degraded",
			Detail:     "The latest database health check indicates problems, which may impact SQL observation windows.",
			Suggestion: "Investigate Postgres availability and SQL stats access before triggering new ad-hoc queries.",
		})
	}
	if filter != nil && filter.QueryMode == OpsQueryModeRaw {
		notes = append(notes, &OpsObservabilityNotice{
			Level:      "info",
			Title:      "Raw query fallback",
			Detail:     "Pre-aggregated tables are disabled or unavailable, so the dashboard is running raw SQL scans and metrics appear degraded.",
			Suggestion: "Schedule this window as read-only or restore pre-aggregated tables to reduce load on pg_stat_statements.",
		})
	}
	if overview.TokenRefreshSummary != nil && overview.TokenRefreshSummary.Total > 0 {
		notes = append(notes, &OpsObservabilityNotice{
			Level:      "warning",
			Title:      tokenRefreshObservabilityTitle,
			Detail:     fmt.Sprintf("Detected %d token refresh failure(s) in the current window. Inspect affected accounts for token_refresh_failure_reason/class metadata.", overview.TokenRefreshSummary.Total),
			Suggestion: "Review token refresh queues, verify OAuth provider quotas, and consider temporary manual retries for permanent failures.",
		})
	}
	if overview.SchedulerCheckpoint != nil {
		cp := overview.SchedulerCheckpoint
		if cp.CheckpointFallbackTotal > 0 || cp.CheckpointReadFailures > 0 || cp.CheckpointWriteFailures > 0 {
			notes = append(notes, &OpsObservabilityNotice{
				Level:      "warning",
				Title:      schedulerCheckpointObservabilityTitle,
				Detail:     fmt.Sprintf("Scheduler checkpoint ran %d fallback(s) with %d read failure(s) and %d write failure(s). Watermark=%d.", cp.CheckpointFallbackTotal, cp.CheckpointReadFailures, cp.CheckpointWriteFailures, cp.LastCheckpointWatermark),
				Suggestion: "Check scheduler_outbox checkpointing, ensure Postgres connectivity, and confirm outbox jobs are writing to persistent storage.",
			})
		}
	}
	if overview.SlowPathDiagnostics != nil && len(overview.SlowPathDiagnostics.SlowSignals) > 0 {
		notes = append(notes, &OpsObservabilityNotice{
			Level:      "info",
			Title:      "Slow-path investigation suggested",
			Detail:     fmt.Sprintf("Detected %d slow-path signal(s): %s.", len(overview.SlowPathDiagnostics.SlowSignals), strings.Join(overview.SlowPathDiagnostics.SlowSignals, ", ")),
			Suggestion: fmt.Sprintf("Inspect %s for the slowest requests in this window before issuing heavier SQL diagnostics.", overview.SlowPathDiagnostics.RequestDetailsEndpoint),
		})
	}
	return notes
}
