package service

import "time"

type OpsDashboardFilter struct {
	StartTime time.Time
	EndTime   time.Time

	Platform string
	GroupID  *int64

	// QueryMode controls whether dashboard queries should use raw logs or pre-aggregated tables.
	// Expected values: auto/raw/preagg (see OpsQueryMode).
	QueryMode OpsQueryMode
}

type OpsRateSummary struct {
	Current float64 `json:"current"`
	Peak    float64 `json:"peak"`
	Avg     float64 `json:"avg"`
}

type OpsPercentiles struct {
	P50 *int `json:"p50_ms"`
	P90 *int `json:"p90_ms"`
	P95 *int `json:"p95_ms"`
	P99 *int `json:"p99_ms"`
	Avg *int `json:"avg_ms"`
	Max *int `json:"max_ms"`
}

type OpsDashboardOverview struct {
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Platform  string    `json:"platform"`
	GroupID   *int64    `json:"group_id"`

	// HealthScore is a backend-computed overall health score (0-100).
	// It is derived from the monitored metrics in this overview, plus best-effort system metrics/job heartbeats.
	HealthScore int `json:"health_score"`

	// Latest system-level snapshot (window=1m, global).
	SystemMetrics *OpsSystemMetricsSnapshot `json:"system_metrics"`

	// Background jobs health (heartbeats).
	JobHeartbeats []*OpsJobHeartbeat `json:"job_heartbeats"`

	// Recent runtime anomaly summaries sampled by the collector.
	RuntimeAnomalies []*OpsSystemLog `json:"runtime_anomalies,omitempty"`

	SuccessCount         int64 `json:"success_count"`
	ErrorCountTotal      int64 `json:"error_count_total"`
	BusinessLimitedCount int64 `json:"business_limited_count"`

	ErrorCountSLA     int64 `json:"error_count_sla"`
	RequestCountTotal int64 `json:"request_count_total"`
	RequestCountSLA   int64 `json:"request_count_sla"`

	TokenConsumed int64 `json:"token_consumed"`

	SLA                          float64 `json:"sla"`
	ErrorRate                    float64 `json:"error_rate"`
	UpstreamErrorRate            float64 `json:"upstream_error_rate"`
	UpstreamErrorCountExcl429529 int64   `json:"upstream_error_count_excl_429_529"`
	Upstream429Count             int64   `json:"upstream_429_count"`
	Upstream529Count             int64   `json:"upstream_529_count"`

	QPS OpsRateSummary `json:"qps"`
	TPS OpsRateSummary `json:"tps"`

	Duration OpsPercentiles `json:"duration"`
	TTFT     OpsPercentiles `json:"ttft"`

	Observability []*OpsObservabilityNotice `json:"observability,omitempty"`

	TokenRefreshSummary   *OpsTokenRefreshSummary        `json:"token_refresh_summary,omitempty"`
	SchedulerCheckpoint   *OpsSchedulerCheckpointSummary `json:"scheduler_checkpoint,omitempty"`
	ResourceBudgetSummary *OpsResourceBudgetSummary      `json:"resource_budget_summary,omitempty"`
	CleanupStats          *CleanupStats                  `json:"cleanup_stats,omitempty"`
	UsageCleanupStats     *UsageCleanupStats             `json:"usage_cleanup_stats,omitempty"`
	StorageGovernance     *OpsStorageGovernanceSummary   `json:"storage_governance,omitempty"`
	SlowPathDiagnostics   *OpsSlowPathDiagnostics        `json:"slow_path_diagnostics,omitempty"`
}

type OpsTokenRefreshSummary struct {
	Total    int64                     `json:"total"`
	Platform map[string]int64          `json:"platform"`
	Group    map[int64]int64           `json:"group"`
	Failures []*OpsTokenRefreshFailure `json:"failures,omitempty"`
}

type OpsTokenRefreshFailure struct {
	AccountID   int64  `json:"account_id"`
	AccountName string `json:"account_name"`

	Platform  string `json:"platform"`
	GroupID   int64  `json:"group_id"`
	GroupName string `json:"group_name"`

	Reason string `json:"reason,omitempty"`
	Class  string `json:"class,omitempty"`
	At     string `json:"failed_at,omitempty"`
}

type OpsSchedulerCheckpointSummary struct {
	LastCheckpointWatermark int64 `json:"last_checkpoint_watermark"`
	CheckpointFallbackTotal int64 `json:"checkpoint_fallback_total"`
	CheckpointReadFailures  int64 `json:"checkpoint_read_failure_total"`
	CheckpointWriteFailures int64 `json:"checkpoint_write_failure_total"`
}

type OpsResourceBudgetSummary struct {
	Database          *OpsDatabaseBudgetSummary          `json:"database,omitempty"`
	Redis             *OpsRedisBudgetSummary             `json:"redis,omitempty"`
	HTTPUpstream      *OpsHTTPUpstreamBudgetSummary      `json:"http_upstream,omitempty"`
	StorageGovernance *OpsStorageGovernanceBudgetSummary `json:"storage_governance,omitempty"`
	Recommendations   []*OpsBudgetRecommendation         `json:"recommendations,omitempty"`
}

type OpsBudgetRecommendation struct {
	Area      string `json:"area"`
	Level     string `json:"level"`
	Current   string `json:"current"`
	Suggested string `json:"suggested"`
	Reason    string `json:"reason"`
}

type OpsSlowPathDiagnostics struct {
	SlowRequestThresholdMs int    `json:"slow_request_threshold_ms"`
	RequestDetailsEndpoint string `json:"request_details_endpoint"`
	RequestDetailsHint     string `json:"request_details_hint"`

	DurationP95Ms *int `json:"duration_p95_ms,omitempty"`
	DurationP99Ms *int `json:"duration_p99_ms,omitempty"`
	DurationMaxMs *int `json:"duration_max_ms,omitempty"`
	TTFTP95Ms     *int `json:"ttft_p95_ms,omitempty"`
	TTFTP99Ms     *int `json:"ttft_p99_ms,omitempty"`

	DBConnWaiting          *int     `json:"db_conn_waiting,omitempty"`
	SQLObservabilityReady  bool     `json:"sql_observability_ready"`
	SQLObservabilityDetail string   `json:"sql_observability_detail,omitempty"`
	SlowSignals            []string `json:"slow_signals,omitempty"`
}

type OpsDatabaseBudgetSummary struct {
	Active                 *int     `json:"active,omitempty"`
	Idle                   *int     `json:"idle,omitempty"`
	Waiting                *int     `json:"waiting,omitempty"`
	MaxOpenConns           *int     `json:"max_open_conns,omitempty"`
	MaxIdleConns           *int     `json:"max_idle_conns,omitempty"`
	ConnMaxLifetimeMinutes *int     `json:"conn_max_lifetime_minutes,omitempty"`
	ConnMaxIdleTimeMinutes *int     `json:"conn_max_idle_time_minutes,omitempty"`
	UsagePercent           *float64 `json:"usage_percent,omitempty"`
}

type OpsRedisBudgetSummary struct {
	Total               *int     `json:"total,omitempty"`
	Idle                *int     `json:"idle,omitempty"`
	PoolSize            *int     `json:"pool_size,omitempty"`
	MinIdleConns        *int     `json:"min_idle_conns,omitempty"`
	DialTimeoutSeconds  *int     `json:"dial_timeout_seconds,omitempty"`
	ReadTimeoutSeconds  *int     `json:"read_timeout_seconds,omitempty"`
	WriteTimeoutSeconds *int     `json:"write_timeout_seconds,omitempty"`
	UsagePercent        *float64 `json:"usage_percent,omitempty"`
}

type OpsHTTPUpstreamBudgetSummary struct {
	MaxIdleConns             *int `json:"max_idle_conns,omitempty"`
	MaxIdleConnsPerHost      *int `json:"max_idle_conns_per_host,omitempty"`
	MaxConnsPerHost          *int `json:"max_conns_per_host,omitempty"`
	MaxUpstreamClients       *int `json:"max_upstream_clients,omitempty"`
	ClientIdleTTLSeconds     *int `json:"client_idle_ttl_seconds,omitempty"`
	ConcurrencySlotTTLMinute *int `json:"concurrency_slot_ttl_minutes,omitempty"`
	SessionIdleTimeoutMinute *int `json:"session_idle_timeout_minutes,omitempty"`
}

type OpsStorageGovernanceBudgetSummary struct {
	OpsSystemLogsMaxRows *int64 `json:"ops_system_logs_max_rows,omitempty"`
	OpsErrorLogsMaxRows  *int64 `json:"ops_error_logs_max_rows,omitempty"`
	UsageLogsMaxRows     *int64 `json:"usage_logs_max_rows,omitempty"`
	MaxRowsEnabled       bool   `json:"max_rows_enabled"`
	MaxRowsDryRun        bool   `json:"max_rows_dry_run"`
}

type OpsStorageGovernanceSummary struct {
	OpsCleanup   *OpsCleanupGovernanceSummary   `json:"ops_cleanup,omitempty"`
	UsageCleanup *OpsUsageCleanupGovernanceInfo `json:"usage_cleanup,omitempty"`
}

type OpsCleanupGovernanceSummary struct {
	Enabled                    bool                    `json:"enabled"`
	ErrorLogRetentionDays      int                     `json:"error_log_retention_days"`
	MinuteMetricsRetentionDays int                     `json:"minute_metrics_retention_days"`
	HourlyMetricsRetentionDays int                     `json:"hourly_metrics_retention_days"`
	SystemLogMaxRows           int64                   `json:"system_log_max_rows"`
	ErrorLogMaxRows            int64                   `json:"error_log_max_rows"`
	MaxRowsEnabled             bool                    `json:"max_rows_enabled"`
	MaxRowsDryRun              bool                    `json:"max_rows_dry_run"`
	SystemLogRows              int64                   `json:"system_log_rows"`
	ErrorLogRows               int64                   `json:"error_log_rows"`
	MaxRowsHit                 bool                    `json:"max_rows_hit"`
	Heartbeat                  *OpsJobHeartbeatSummary `json:"heartbeat,omitempty"`
}

type OpsUsageCleanupGovernanceInfo struct {
	Enabled                bool  `json:"enabled"`
	MaxRangeDays           int   `json:"max_range_days"`
	BatchSize              int   `json:"batch_size"`
	WorkerIntervalSeconds  int   `json:"worker_interval_seconds"`
	TaskTimeoutSeconds     int   `json:"task_timeout_seconds"`
	UsageLogsRetentionDays int   `json:"usage_logs_retention_days"`
	UsageLogsMaxRows       int64 `json:"usage_logs_max_rows"`
	LastTaskID             int64 `json:"last_task_id"`
	LastDeletedRows        int64 `json:"last_deleted_rows"`
}

type OpsJobHeartbeatSummary struct {
	LastRunAt      *time.Time `json:"last_run_at,omitempty"`
	LastSuccessAt  *time.Time `json:"last_success_at,omitempty"`
	LastErrorAt    *time.Time `json:"last_error_at,omitempty"`
	LastError      *string    `json:"last_error,omitempty"`
	LastDurationMs *int64     `json:"last_duration_ms,omitempty"`
	LastResult     *string    `json:"last_result,omitempty"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type OpsObservabilityNotice struct {
	Level      string `json:"level"`
	Title      string `json:"title"`
	Detail     string `json:"detail"`
	Suggestion string `json:"suggestion,omitempty"`
}

type OpsLatencyHistogramBucket struct {
	Range string `json:"range"`
	Count int64  `json:"count"`
}

// OpsLatencyHistogramResponse is a coarse latency distribution histogram (success requests only).
// It is used by the Ops dashboard to quickly identify tail latency regressions.
type OpsLatencyHistogramResponse struct {
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Platform  string    `json:"platform"`
	GroupID   *int64    `json:"group_id"`

	TotalRequests int64                        `json:"total_requests"`
	Buckets       []*OpsLatencyHistogramBucket `json:"buckets"`
}
