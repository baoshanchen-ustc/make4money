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

	// QueryPath captures whether this overview was served from the pre-aggregated tables or from raw logs.
	QueryPath *OpsDashboardQueryPath `json:"query_path,omitempty"`

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

	TokenRefreshSummary    *OpsTokenRefreshSummary              `json:"token_refresh_summary,omitempty"`
	AccountAuthSummary     *OpsAccountAuthFailureSummary        `json:"account_auth_summary,omitempty"`
	OpenAIAccountScheduler *OpsOpenAIAccountSchedulerSummary    `json:"openai_account_scheduler,omitempty"`
	StickySessionCleanup   *StickySessionCleanupMetricsSnapshot `json:"sticky_session_cleanup,omitempty"`
	StickyConsistency      *StickyConsistencyMetricsSnapshot    `json:"sticky_consistency,omitempty"`
	SchedulerCheckpoint    *OpsSchedulerCheckpointSummary       `json:"scheduler_checkpoint,omitempty"`
	SchedulerOutboxRuntime *OpsSchedulerOutboxRuntimeSummary    `json:"scheduler_outbox_runtime,omitempty"`
	ResourceBudgetSummary  *OpsResourceBudgetSummary            `json:"resource_budget_summary,omitempty"`
	CleanupStats           *CleanupStats                        `json:"cleanup_stats,omitempty"`
	UsageCleanupStats      *UsageCleanupStats                   `json:"usage_cleanup_stats,omitempty"`
	StorageGovernance      *OpsStorageGovernanceSummary         `json:"storage_governance,omitempty"`
	ErrorFamilySummary     *OpsErrorFamilySummary               `json:"error_family_summary,omitempty"`
	FailureSplitSummary    *OpsFailureSplitSummary              `json:"failure_split_summary,omitempty"`
	UsageIntegrity         *OpsUsageIntegritySummary            `json:"usage_integrity,omitempty"`
	SlowPathDiagnostics    *OpsSlowPathDiagnostics              `json:"slow_path_diagnostics,omitempty"`
	DataSource             *OpsDataSourceSummary                `json:"data_source,omitempty"`
	DriftTrendSummary      *OpsDriftTrendSummary                `json:"drift_trend_summary,omitempty"`
}

type OpsDataSourceSummary struct {
	Mode          string `json:"mode,omitempty"`
	RequestedMode string `json:"requested_mode,omitempty"`
}

type OpsDashboardQueryPath struct {
	Mode          string `json:"mode,omitempty"`
	RequestedMode string `json:"requested_mode,omitempty"`
	Reason        string `json:"reason,omitempty"`
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

type OpsAccountAuthFailureSummary struct {
	Total                int64                    `json:"total"`
	PermanentCount       int64                    `json:"permanent_count"`
	TemporaryCount       int64                    `json:"temporary_count"`
	DispatchSuppressed   int64                    `json:"dispatch_suppressed"`
	BackgroundRecovery   int64                    `json:"background_recovery"`
	ManualReviewRequired int64                    `json:"manual_review_required"`
	Platform             map[string]int64         `json:"platform,omitempty"`
	Group                map[int64]int64          `json:"group,omitempty"`
	ByReason             map[string]int64         `json:"by_reason,omitempty"`
	ByClass              map[string]int64         `json:"by_class,omitempty"`
	BySource             map[string]int64         `json:"by_source,omitempty"`
	ByState              map[string]int64         `json:"by_state,omitempty"`
	Failures             []*OpsAccountAuthFailure `json:"failures,omitempty"`
}

type OpsAccountAuthFailure struct {
	AccountID   int64  `json:"account_id"`
	AccountName string `json:"account_name"`

	Platform  string `json:"platform"`
	GroupID   int64  `json:"group_id"`
	GroupName string `json:"group_name"`

	Reason               string `json:"reason,omitempty"`
	Class                string `json:"class,omitempty"`
	Source               string `json:"source,omitempty"`
	State                string `json:"state,omitempty"`
	RecoveryAction       string `json:"recovery_action,omitempty"`
	DispatchSuppressed   bool   `json:"dispatch_suppressed,omitempty"`
	BackgroundRecovery   bool   `json:"background_recovery,omitempty"`
	ManualReviewRequired bool   `json:"manual_review_required,omitempty"`
	At                   string `json:"failed_at,omitempty"`
}

type OpsOpenAIAccountSchedulerSummary struct {
	SelectTotal                    int64            `json:"select_total"`
	StickyPreviousHitTotal         int64            `json:"sticky_previous_hit_total"`
	StickySessionHitTotal          int64            `json:"sticky_session_hit_total"`
	StickySessionLookupTotal       int64            `json:"sticky_session_lookup_total"`
	StickySessionWaitTotal         int64            `json:"sticky_session_wait_total"`
	StickySessionClearedTotal      int64            `json:"sticky_session_cleared_total"`
	StickySessionGhostTotal        int64            `json:"sticky_session_ghost_total"`
	StickySessionStaleTotal        int64            `json:"sticky_session_stale_total"`
	StickyWaitConflictTotal        int64            `json:"sticky_wait_conflict_total"`
	LoadBalanceSelectTotal         int64            `json:"load_balance_select_total"`
	AccountSwitchTotal             int64            `json:"account_switch_total"`
	StickyHitRatio                 float64          `json:"sticky_hit_ratio"`
	AccountSwitchRate              float64          `json:"account_switch_rate"`
	SchedulerLatencyMsAvg          float64          `json:"scheduler_latency_ms_avg"`
	StickyGhostRatio               float64          `json:"sticky_ghost_ratio"`
	StickyStaleRatio               float64          `json:"sticky_stale_ratio"`
	StickyWaitConflictRatio        float64          `json:"sticky_wait_conflict_ratio"`
	StickySessionAccountFetchTotal int64            `json:"sticky_session_account_fetch_total"`
	StickySessionDBRecheckTotal    int64            `json:"sticky_session_db_recheck_total"`
	LoadBatchCallTotal             int64            `json:"load_batch_call_total"`
	LoadBatchCandidateTotal        int64            `json:"load_batch_candidate_total"`
	LoadBatchFallbackTotal         int64            `json:"load_batch_fallback_total"`
	CompareDeleteMissTotal         int64            `json:"compare_delete_miss_total"`
	RuntimeStatsAccountCount       int              `json:"runtime_stats_account_count"`
	ShadowReasonTotals             map[string]int64 `json:"shadow_reason_totals,omitempty"`
	CleanupReasonTotals            map[string]int64 `json:"cleanup_reason_totals,omitempty"`
	CompareDeleteMissReasonTotals  map[string]int64 `json:"compare_delete_miss_reason_totals,omitempty"`
}

type OpsSchedulerCheckpointSummary struct {
	RedisWatermark               int64                        `json:"redis_watermark"`
	LastCheckpointWatermark      int64                        `json:"last_checkpoint_watermark"`
	WatermarkDrift               int64                        `json:"watermark_drift"`
	CheckpointFallbackTotal      int64                        `json:"checkpoint_fallback_total"`
	CheckpointFallbackStreak     int64                        `json:"checkpoint_fallback_streak"`
	CheckpointReadFailures       int64                        `json:"checkpoint_read_failure_total"`
	CheckpointWriteFailures      int64                        `json:"checkpoint_write_failure_total"`
	CheckpointLastFallbackAt     string                       `json:"checkpoint_last_fallback_at,omitempty"`
	CheckpointLastFallbackReason string                       `json:"checkpoint_last_fallback_reason,omitempty"`
	CheckpointLastReadFailureAt  string                       `json:"checkpoint_last_read_failure_at,omitempty"`
	CheckpointLastWriteFailureAt string                       `json:"checkpoint_last_write_failure_at,omitempty"`
	BlockedEvent                 *SchedulerOutboxBlockedEvent `json:"blocked_event,omitempty"`
}

type OpsSchedulerOutboxRuntimeSummary struct {
	LastRedisWatermark           int64                        `json:"last_redis_watermark"`
	LastCheckpointWatermark      int64                        `json:"last_checkpoint_watermark"`
	WatermarkDrift               int64                        `json:"watermark_drift"`
	DriftTrendStatus             string                       `json:"drift_trend_status,omitempty"`
	DriftTrendDetail             string                       `json:"drift_trend_detail,omitempty"`
	DriftTrendNarrative          string                       `json:"drift_trend_narrative,omitempty"`
	CheckpointFallbackStreak     int64                        `json:"checkpoint_fallback_streak"`
	CheckpointLastFallbackReason string                       `json:"checkpoint_last_fallback_reason,omitempty"`
	BacklogRows                  int64                        `json:"backlog_rows"`
	LagSeconds                   int64                        `json:"lag_seconds"`
	LagFailureStreak             int64                        `json:"lag_failure_streak"`
	CoalescedBatchTotal          int64                        `json:"coalesced_batch_total"`
	CoalescedEventSavedTotal     int64                        `json:"coalesced_event_saved_total"`
	LagRebuildTotal              int64                        `json:"lag_rebuild_total"`
	BacklogRebuildTotal          int64                        `json:"backlog_rebuild_total"`
	RebuildCooldownSkipTotal     int64                        `json:"rebuild_cooldown_skip_total"`
	LastLagRebuildAt             string                       `json:"last_lag_rebuild_at,omitempty"`
	LastBacklogRebuildAt         string                       `json:"last_backlog_rebuild_at,omitempty"`
	BlockedEventClearTotal       int64                        `json:"blocked_event_clear_total"`
	BlockedEventLastClearedID    int64                        `json:"blocked_event_last_cleared_id,omitempty"`
	BlockedEventLastClearedAt    string                       `json:"blocked_event_last_cleared_at,omitempty"`
	BlockedEventLastClearReason  string                       `json:"blocked_event_last_clear_reason,omitempty"`
	BucketRebuildSuccessTotal    int64                        `json:"bucket_rebuild_success_total"`
	BucketRebuildFailureTotal    int64                        `json:"bucket_rebuild_failure_total"`
	BucketRebuildLockContention  int64                        `json:"bucket_rebuild_lock_contention_total"`
	BusyBucketSkipTotal          int64                        `json:"busy_bucket_skip_total"`
	LastBucketRebuildAt          string                       `json:"last_bucket_rebuild_at,omitempty"`
	LastBucketRebuildReason      string                       `json:"last_bucket_rebuild_reason,omitempty"`
	LastBucketRebuildStatus      string                       `json:"last_bucket_rebuild_status,omitempty"`
	LastBucketRebuildBucket      string                       `json:"last_bucket_rebuild_bucket,omitempty"`
	BlockedEvent                 *SchedulerOutboxBlockedEvent `json:"blocked_event,omitempty"`
}

type OpsDriftTrendSummary struct {
	Severity       string   `json:"severity"`
	Status         string   `json:"status"`
	Trend          string   `json:"trend,omitempty"`
	StickyState    string   `json:"sticky_state,omitempty"`
	SchedulerState string   `json:"scheduler_state,omitempty"`
	RedisState     string   `json:"redis_state,omitempty"`
	Notes          []string `json:"notes,omitempty"`
}

type OpsResourceBudgetSummary struct {
	Database          *OpsDatabaseBudgetSummary          `json:"database,omitempty"`
	Redis             *OpsRedisBudgetSummary             `json:"redis,omitempty"`
	HTTPUpstream      *OpsHTTPUpstreamBudgetSummary      `json:"http_upstream,omitempty"`
	StorageGovernance *OpsStorageGovernanceBudgetSummary `json:"storage_governance,omitempty"`
	WatchThresholds   map[string]string                  `json:"watch_thresholds,omitempty"`
	SafetyZone        *OpsResourceSafetyZone             `json:"safety_zone,omitempty"`
	JointSignals      []*OpsBudgetJointSignal            `json:"joint_signals,omitempty"`
	Recommendations   []*OpsBudgetRecommendation         `json:"recommendations,omitempty"`
	Guardrails        []*OpsBudgetGuardrail              `json:"guardrails,omitempty"`
}

type OpsResourceSafetyZone struct {
	Status  string   `json:"status"`
	Detail  string   `json:"detail,omitempty"`
	Reasons []string `json:"reasons,omitempty"`
}

type OpsBudgetJointSignal struct {
	Level      string `json:"level"`
	Title      string `json:"title"`
	Detail     string `json:"detail,omitempty"`
	Suggestion string `json:"suggestion,omitempty"`
}

type OpsBudgetRecommendation struct {
	Area      string `json:"area"`
	Level     string `json:"level"`
	Current   string `json:"current"`
	Suggested string `json:"suggested"`
	Reason    string `json:"reason"`
}

type OpsBudgetGuardrail struct {
	Area            string            `json:"area"`
	Phase           string            `json:"phase"`
	Action          string            `json:"action"`
	Watch           []string          `json:"watch,omitempty"`
	WatchThresholds map[string]string `json:"watch_thresholds,omitempty"`
	Rollback        string            `json:"rollback,omitempty"`
	Rationale       string            `json:"rationale,omitempty"`
}

type OpsSlowPathDiagnostics struct {
	SlowRequestThresholdMs int      `json:"slow_request_threshold_ms"`
	RequestDetailsEndpoint string   `json:"request_details_endpoint"`
	RequestDetailsHint     string   `json:"request_details_hint"`
	PreferSummaryFirst     bool     `json:"prefer_summary_first"`
	RawRiskLevel           string   `json:"raw_risk_level,omitempty"`
	GuardrailHint          string   `json:"guardrail_hint,omitempty"`
	DrilldownEndpoints     []string `json:"drilldown_endpoints,omitempty"`
	RawPathUsed            bool     `json:"raw_path_used"`
	QueryPathReason        string   `json:"query_path_reason,omitempty"`
	CorrelationHint        string   `json:"correlation_hint,omitempty"`
	InvestigationOrder     []string `json:"investigation_order,omitempty"`

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
	HeadroomPercent        *float64 `json:"headroom_percent,omitempty"`
	IdleRatioPercent       *float64 `json:"idle_ratio_percent,omitempty"`
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
	HeadroomPercent     *float64 `json:"headroom_percent,omitempty"`
	IdleRatioPercent    *float64 `json:"idle_ratio_percent,omitempty"`
}

type OpsHTTPUpstreamBudgetSummary struct {
	MaxIdleConns             *int    `json:"max_idle_conns,omitempty"`
	MaxIdleConnsPerHost      *int    `json:"max_idle_conns_per_host,omitempty"`
	MaxConnsPerHost          *int    `json:"max_conns_per_host,omitempty"`
	MaxUpstreamClients       *int    `json:"max_upstream_clients,omitempty"`
	ClientIdleTTLSeconds     *int    `json:"client_idle_ttl_seconds,omitempty"`
	ConcurrencySlotTTLMinute *int    `json:"concurrency_slot_ttl_minutes,omitempty"`
	SessionIdleTimeoutMinute *int    `json:"session_idle_timeout_minutes,omitempty"`
	IsolationMode            *string `json:"isolation_mode,omitempty"`
}

type OpsStorageGovernanceBudgetSummary struct {
	OpsSystemLogsMaxRows *int64 `json:"ops_system_logs_max_rows,omitempty"`
	OpsErrorLogsMaxRows  *int64 `json:"ops_error_logs_max_rows,omitempty"`
	UsageLogsMaxRows     *int64 `json:"usage_logs_max_rows,omitempty"`
	MaxRowsEnabled       bool   `json:"max_rows_enabled"`
	MaxRowsDryRun        bool   `json:"max_rows_dry_run"`
}

type OpsStorageGovernanceSummary struct {
	OpsCleanup   *OpsCleanupGovernanceSummary       `json:"ops_cleanup,omitempty"`
	UsageCleanup *OpsUsageCleanupGovernanceInfo     `json:"usage_cleanup,omitempty"`
	UsageLogs    *OpsUsageLogsGovernanceSummary     `json:"usage_logs,omitempty"`
	DryRun       *OpsStorageGovernanceDryRunSummary `json:"dry_run,omitempty"`
	Runtime      *OpsStorageGovernanceRuntimeInfo   `json:"runtime,omitempty"`
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
	Enabled                bool       `json:"enabled"`
	MaxRangeDays           int        `json:"max_range_days"`
	BatchSize              int        `json:"batch_size"`
	WorkerIntervalSeconds  int        `json:"worker_interval_seconds"`
	TaskTimeoutSeconds     int        `json:"task_timeout_seconds"`
	UsageLogsRetentionDays int        `json:"usage_logs_retention_days"`
	UsageLogsMaxRows       int64      `json:"usage_logs_max_rows"`
	LastTaskID             int64      `json:"last_task_id"`
	LastDeletedRows        int64      `json:"last_deleted_rows"`
	StartedTotal           int64      `json:"started_total"`
	SucceededTotal         int64      `json:"succeeded_total"`
	FailedTotal            int64      `json:"failed_total"`
	CanceledTotal          int64      `json:"canceled_total"`
	LastStartedAt          *time.Time `json:"last_started_at,omitempty"`
	LastSucceededAt        *time.Time `json:"last_succeeded_at,omitempty"`
	LastFailedAt           *time.Time `json:"last_failed_at,omitempty"`
	LastCanceledAt         *time.Time `json:"last_canceled_at,omitempty"`
	LastDurationMs         *int64     `json:"last_duration_ms,omitempty"`
	LastStatus             string     `json:"last_status,omitempty"`
	LastError              string     `json:"last_error,omitempty"`
}

type OpsStorageGovernanceDryRunSummary struct {
	OpsCleanup   *OpsCleanupDryRunSummary   `json:"ops_cleanup,omitempty"`
	UsageCleanup *OpsUsageCleanupDryRunInfo `json:"usage_cleanup,omitempty"`
}

type OpsCleanupDryRunSummary struct {
	Enabled               bool     `json:"enabled"`
	DryRun                bool     `json:"dry_run"`
	SystemLogRows         int64    `json:"system_log_rows"`
	SystemLogLimit        int64    `json:"system_log_limit"`
	SystemLogUsagePercent *float64 `json:"system_log_usage_percent,omitempty"`
	SystemLogOverLimit    bool     `json:"system_log_over_limit"`
	ErrorLogRows          int64    `json:"error_log_rows"`
	ErrorLogLimit         int64    `json:"error_log_limit"`
	ErrorLogUsagePercent  *float64 `json:"error_log_usage_percent,omitempty"`
	ErrorLogOverLimit     bool     `json:"error_log_over_limit"`
	MaxRowsHit            bool     `json:"max_rows_hit"`
}

type OpsUsageCleanupDryRunInfo struct {
	Enabled                bool       `json:"enabled"`
	UsageLogsMaxRows       int64      `json:"usage_logs_max_rows"`
	UsageLogsRetentionDays int        `json:"usage_logs_retention_days"`
	LastTaskID             int64      `json:"last_task_id"`
	LastDeletedRows        int64      `json:"last_deleted_rows"`
	UsageLogRowsEstimated  int64      `json:"usage_log_rows_estimated"`
	UsageLogUsagePercent   *float64   `json:"usage_log_usage_percent,omitempty"`
	UsageLogOverLimit      bool       `json:"usage_log_over_limit"`
	RetentionRisk          string     `json:"retention_risk,omitempty"`
	Reasons                []string   `json:"reasons,omitempty"`
	LastSampledAt          *time.Time `json:"last_sampled_at,omitempty"`
	EnforcementMode        string     `json:"enforcement_mode"`
	Note                   string     `json:"note,omitempty"`
}

type OpsUsageLogsGovernanceSummary struct {
	EstimatedLiveRows       int64      `json:"estimated_live_rows"`
	EstimatedDeadRows       int64      `json:"estimated_dead_rows"`
	TotalMB                 float64    `json:"total_mb"`
	Partitioned             bool       `json:"partitioned"`
	RetentionDays           int        `json:"retention_days"`
	RetentionEnabled        bool       `json:"retention_enabled"`
	MaxRowsConfigured       int64      `json:"max_rows_configured"`
	MaxRowsEnabled          bool       `json:"max_rows_enabled"`
	MaxRowsExceeded         bool       `json:"max_rows_exceeded"`
	SizeRisk                string     `json:"size_risk,omitempty"`
	RetentionRisk           string     `json:"retention_risk,omitempty"`
	ColdIndexRisk           string     `json:"cold_index_risk,omitempty"`
	Reasons                 []string   `json:"reasons,omitempty"`
	LastSampledAt           *time.Time `json:"last_sampled_at,omitempty"`
	ApproxBytesPerLiveRow   float64    `json:"approx_bytes_per_live_row"`
	ColdIndexCandidateCount int        `json:"cold_index_candidate_count"`
	ColdIndexCandidateMB    float64    `json:"cold_index_candidate_mb"`
}

type OpsStorageGovernanceRuntimeInfo struct {
	SampledAt      *time.Time                              `json:"sampled_at,omitempty"`
	SampledTables  int                                     `json:"sampled_tables"`
	RiskTables     int                                     `json:"risk_tables"`
	WarnTables     int                                     `json:"warn_tables"`
	CriticalTables int                                     `json:"critical_tables"`
	Tables         []*OpsStorageGovernanceRuntimeTableInfo `json:"tables,omitempty"`
}

type OpsStorageGovernanceRuntimeTableInfo struct {
	Table                 string   `json:"table"`
	LiveRows              int64    `json:"live_rows"`
	DeadRows              int64    `json:"dead_rows"`
	TotalBytes            int64    `json:"total_bytes"`
	TotalMB               float64  `json:"total_mb"`
	Partitioned           bool     `json:"partitioned"`
	MaxRowsConfigured     int64    `json:"max_rows_configured"`
	MaxRowsEnabled        bool     `json:"max_rows_enabled"`
	MaxRowsExceeded       bool     `json:"max_rows_exceeded"`
	RetentionDays         int      `json:"retention_days"`
	RetentionEnabled      bool     `json:"retention_enabled"`
	RetentionRisk         string   `json:"retention_risk,omitempty"`
	SizeRisk              string   `json:"size_risk,omitempty"`
	ColdIndexRisk         string   `json:"cold_index_risk,omitempty"`
	ApproxBytesPerLiveRow float64  `json:"approx_bytes_per_live_row"`
	Reasons               []string `json:"reasons,omitempty"`
}

type OpsErrorFamilySummary struct {
	SampledAt     *time.Time             `json:"sampled_at,omitempty"`
	WindowMinutes int                    `json:"window_minutes"`
	TotalErrors   int64                  `json:"total_errors"`
	Families      []*OpsErrorFamilyEntry `json:"families,omitempty"`
}

type OpsErrorFamilyEntry struct {
	Phase           string  `json:"phase,omitempty"`
	Type            string  `json:"type,omitempty"`
	Owner           string  `json:"owner,omitempty"`
	StatusCode      int     `json:"status_code"`
	InboundEndpoint string  `json:"inbound_endpoint,omitempty"`
	Count           int64   `json:"count"`
	SharePercent    float64 `json:"share_percent"`
}

type OpsFailureSplitSummary struct {
	ProtocolOrRequestShape int64  `json:"protocol_or_request_shape"`
	AccountOrAuth          int64  `json:"account_or_auth"`
	ProviderOrUpstream     int64  `json:"provider_or_upstream"`
	LocalProcessing        int64  `json:"local_processing"`
	LikelyPrimary          string `json:"likely_primary,omitempty"`
	Suggestion             string `json:"suggestion,omitempty"`
	OperatorAction         string `json:"operator_action,omitempty"`
}

type OpsOperatorActionPlan struct {
	FailureSplitActions []string `json:"failure_split_actions,omitempty"`
	ProtocolDiagnostics []string `json:"protocol_diagnostics,omitempty"`
	ControlPlaneDrift   []string `json:"control_plane_drift,omitempty"`
}

type OpsUsageIntegritySummary struct {
	UsageLogNotPersisted *OpsUsageIntegritySignal `json:"usage_log_not_persisted,omitempty"`
	BillingCompensation  *OpsUsageIntegritySignal `json:"billing_compensation,omitempty"`
	Advisory             string                   `json:"advisory,omitempty"`
}

type OpsUsageIntegritySignal struct {
	Total                  int64      `json:"total"`
	Recent1mTotal          int64      `json:"recent_1m_total"`
	Recent5mTotal          int64      `json:"recent_5m_total"`
	Recent15mTotal         int64      `json:"recent_15m_total"`
	LastSeenAt             *time.Time `json:"last_seen_at,omitempty"`
	LastRequestID          string     `json:"last_request_id,omitempty"`
	LastAccountID          int64      `json:"last_account_id,omitempty"`
	LastError              string     `json:"last_error,omitempty"`
	Endpoint               string     `json:"endpoint,omitempty"`
	DetailEndpointTemplate string     `json:"detail_endpoint_template,omitempty"`
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
