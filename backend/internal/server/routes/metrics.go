package routes

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const prometheusContentType = "text/plain; version=0.0.4; charset=utf-8"

func metricsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Content-Type", prometheusContentType)
		c.String(http.StatusOK, renderRuntimeMetrics())
	}
}

func renderRuntimeMetrics() string {
	db := repository.SnapshotDefaultDBPoolStats()
	redis := repository.SnapshotDefaultRedisPoolStats()
	scheduler := service.SnapshotSchedulerOutboxRuntimeMetrics()
	usageNotPersisted := service.SnapshotUsageLogNotPersistedMetrics()
	cleanup := service.SnapshotCleanupStats()
	usageCleanup := service.SnapshotUsageCleanupStats()

	var b strings.Builder
	writePromGauge(&b, "sub2api_up", 1, "Whether the sub2api metrics endpoint is serving.")

	writePromGauge(&b, "sub2api_db_pool_open_connections", float64(db.OpenConnections), "Current number of open database connections.")
	writePromGauge(&b, "sub2api_db_pool_in_use_connections", float64(db.InUse), "Current number of in-use database connections.")
	writePromGauge(&b, "sub2api_db_pool_idle_connections", float64(db.Idle), "Current number of idle database connections.")
	writePromGauge(&b, "sub2api_db_pool_max_open_connections", float64(db.MaxOpenConnections), "Configured database max_open_connections.")
	writePromCounter(&b, "sub2api_db_pool_wait_count_total", float64(db.WaitCount), "Cumulative database pool wait count.")
	writePromCounter(&b, "sub2api_db_pool_wait_duration_ms_total", float64(db.WaitDurationMs), "Cumulative database pool wait duration in milliseconds.")
	writePromGaugePtr(&b, "sub2api_db_pool_usage_percent", db.UsagePercent, "Database pool open connection usage percentage.")
	writePromGaugePtr(&b, "sub2api_db_pool_in_use_percent", db.InUsePercent, "Database pool in-use percentage.")
	writePromGaugePtr(&b, "sub2api_db_pool_idle_reservation_ratio", db.IdleReservationRatio, "Database pool idle reservation percentage.")

	writePromCounter(&b, "sub2api_redis_pool_hits_total", float64(redis.Hits), "Cumulative Redis pool hits.")
	writePromCounter(&b, "sub2api_redis_pool_misses_total", float64(redis.Misses), "Cumulative Redis pool misses.")
	writePromCounter(&b, "sub2api_redis_pool_stalls_total", float64(redis.Stalls), "Cumulative Redis pool waits/stalls.")
	writePromCounter(&b, "sub2api_redis_pool_timeouts_total", float64(redis.Timeouts), "Cumulative Redis pool timeouts.")
	writePromGauge(&b, "sub2api_redis_pool_total_connections", float64(redis.TotalConns), "Current Redis pool total connections.")
	writePromGauge(&b, "sub2api_redis_pool_idle_connections", float64(redis.IdleConns), "Current Redis pool idle connections.")
	writePromGaugePtr(&b, "sub2api_redis_pool_usage_percent", redis.UsagePercent, "Redis pool usage percentage.")
	writePromGaugePtr(&b, "sub2api_redis_pool_idle_reservation_ratio", redis.IdleReservationRatio, "Redis idle reservation percentage.")
	writePromGaugePtr(&b, "sub2api_redis_pool_hit_rate_percent", redis.HitRatePercent, "Redis pool hit rate percentage.")
	writePromGaugePtr(&b, "sub2api_redis_pool_connection_pressure_percent", redis.ConnectionPressurePercent, "Redis pool connection pressure percentage.")
	writePromGaugePtr(&b, "sub2api_redis_pool_headroom_percent", redis.PoolHeadroomPercent, "Redis pool headroom percentage.")

	writePromCounter(&b, "sub2api_scheduler_outbox_poison_total", float64(scheduler.PoisonTotal), "Cumulative scheduler outbox poison events.")
	writePromCounter(&b, "sub2api_scheduler_outbox_transient_total", float64(scheduler.TransientTotal), "Cumulative scheduler outbox transient events.")
	writePromCounter(&b, "sub2api_scheduler_outbox_coalesced_batch_total", float64(scheduler.CoalescedBatchTotal), "Cumulative scheduler outbox poll batches where adjacent events were coalesced.")
	writePromCounter(&b, "sub2api_scheduler_outbox_coalesced_event_saved_total", float64(scheduler.CoalescedEventSavedTotal), "Cumulative scheduler outbox events avoided by adjacent-event coalescing.")
	writePromCounter(&b, "sub2api_scheduler_outbox_checkpoint_fallback_total", float64(scheduler.CheckpointFallbackTotal), "Cumulative scheduler checkpoint fallback count.")
	writePromCounter(&b, "sub2api_scheduler_outbox_checkpoint_read_failure_total", float64(scheduler.CheckpointReadFailureTotal), "Cumulative scheduler checkpoint read failures.")
	writePromCounter(&b, "sub2api_scheduler_outbox_checkpoint_write_failure_total", float64(scheduler.CheckpointWriteFailureTotal), "Cumulative scheduler checkpoint write failures.")
	writePromGauge(&b, "sub2api_scheduler_outbox_watermark_drift", float64(scheduler.WatermarkDrift), "Current scheduler watermark drift.")
	writePromGauge(&b, "sub2api_scheduler_outbox_backlog_rows", float64(scheduler.BacklogRows), "Current scheduler outbox backlog row count.")
	writePromGauge(&b, "sub2api_scheduler_outbox_lag_seconds", float64(scheduler.LagSeconds), "Current scheduler outbox lag in seconds.")
	writePromGauge(&b, "sub2api_scheduler_outbox_lag_failure_streak", float64(scheduler.LagFailureStreak), "Current scheduler outbox lag failure streak.")
	writePromCounter(&b, "sub2api_scheduler_outbox_lag_rebuild_total", float64(scheduler.LagRebuildTotal), "Cumulative scheduler lag-triggered rebuild count.")
	writePromCounter(&b, "sub2api_scheduler_outbox_backlog_rebuild_total", float64(scheduler.BacklogRebuildTotal), "Cumulative scheduler backlog-triggered rebuild count.")
	writePromCounter(&b, "sub2api_scheduler_outbox_rebuild_cooldown_skip_total", float64(scheduler.RebuildCooldownSkipTotal), "Cumulative scheduler rebuild attempts suppressed by the outbox rebuild cooldown window.")
	writePromCounter(&b, "sub2api_scheduler_outbox_bucket_rebuild_success_total", float64(scheduler.BucketRebuildSuccessTotal), "Cumulative successful scheduler bucket rebuilds.")
	writePromCounter(&b, "sub2api_scheduler_outbox_bucket_rebuild_failure_total", float64(scheduler.BucketRebuildFailureTotal), "Cumulative failed scheduler bucket rebuilds.")
	writePromCounter(&b, "sub2api_scheduler_outbox_bucket_rebuild_lock_contention_total", float64(scheduler.BucketRebuildLockContention), "Cumulative scheduler bucket rebuild lock contention events.")
	writePromCounter(&b, "sub2api_scheduler_outbox_busy_bucket_skip_total", float64(scheduler.BusyBucketSkipTotal), "Cumulative busy scheduler buckets skipped during full rebuild sweeps.")

	writePromCounter(&b, "sub2api_usage_log_not_persisted_total", float64(usageNotPersisted.Total), "Cumulative usage-log-not-persisted alerts.")
	writePromGauge(&b, "sub2api_usage_log_not_persisted_recent_1m", float64(usageNotPersisted.Recent1mTotal), "Usage-log-not-persisted alerts seen in the last 1 minute.")
	writePromGauge(&b, "sub2api_usage_log_not_persisted_recent_5m", float64(usageNotPersisted.Recent5mTotal), "Usage-log-not-persisted alerts seen in the last 5 minutes.")
	writePromGauge(&b, "sub2api_usage_log_not_persisted_recent_15m", float64(usageNotPersisted.Recent15mTotal), "Usage-log-not-persisted alerts seen in the last 15 minutes.")

	writePromGauge(&b, "sub2api_ops_cleanup_system_log_rows", float64(cleanup.SystemLogRows), "Estimated ops_system_logs row count used by cleanup.")
	writePromGauge(&b, "sub2api_ops_cleanup_error_log_rows", float64(cleanup.ErrorLogRows), "Estimated ops_error_logs row count used by cleanup.")
	writePromGauge(&b, "sub2api_ops_cleanup_system_log_limit", float64(cleanup.SystemLogLimit), "Configured ops_system_logs cleanup row limit.")
	writePromGauge(&b, "sub2api_ops_cleanup_error_log_limit", float64(cleanup.ErrorLogLimit), "Configured ops_error_logs cleanup row limit.")
	writePromGauge(&b, "sub2api_ops_cleanup_max_rows_enabled", boolProm(cleanup.MaxRowsEnabled), "Whether cleanup max-row guard is enabled.")
	writePromGauge(&b, "sub2api_ops_cleanup_max_rows_dry_run", boolProm(cleanup.MaxRowsDryRun), "Whether cleanup max-row guard is in dry-run mode.")
	writePromGauge(&b, "sub2api_ops_cleanup_max_rows_hit", boolProm(cleanup.MaxRowsHit), "Whether cleanup max-row guard was hit on the latest run.")

	writePromGauge(&b, "sub2api_usage_cleanup_last_task_id", float64(usageCleanup.LastTaskID), "Latest usage cleanup task id.")
	writePromGauge(&b, "sub2api_usage_cleanup_last_deleted_rows", float64(usageCleanup.LastDeletedRows), "Rows deleted by the latest usage cleanup task.")
	writePromCounter(&b, "sub2api_usage_cleanup_started_total", float64(usageCleanup.StartedTotal), "Cumulative usage cleanup tasks started.")
	writePromCounter(&b, "sub2api_usage_cleanup_succeeded_total", float64(usageCleanup.SucceededTotal), "Cumulative usage cleanup tasks succeeded.")
	writePromCounter(&b, "sub2api_usage_cleanup_failed_total", float64(usageCleanup.FailedTotal), "Cumulative usage cleanup tasks failed.")
	writePromCounter(&b, "sub2api_usage_cleanup_canceled_total", float64(usageCleanup.CanceledTotal), "Cumulative usage cleanup tasks canceled.")
	writePromGaugePtr(&b, "sub2api_usage_cleanup_last_duration_ms", usageCleanup.LastDurationMs, "Duration of the latest usage cleanup task in milliseconds.")

	return b.String()
}

func writePromGauge(b *strings.Builder, name string, value float64, help string) {
	writePromMetric(b, name, "gauge", value, help)
}

func writePromCounter(b *strings.Builder, name string, value float64, help string) {
	writePromMetric(b, name, "counter", value, help)
}

func writePromGaugePtr[T ~float64 | ~int64](b *strings.Builder, name string, value *T, help string) {
	if value == nil {
		return
	}
	writePromGauge(b, name, float64(*value), help)
}

func writePromMetric(b *strings.Builder, name, metricType string, value float64, help string) {
	_, _ = b.WriteString("# HELP ")
	_, _ = b.WriteString(name)
	_ = b.WriteByte(' ')
	_, _ = b.WriteString(help)
	_ = b.WriteByte('\n')
	_, _ = b.WriteString("# TYPE ")
	_, _ = b.WriteString(name)
	_ = b.WriteByte(' ')
	_, _ = b.WriteString(metricType)
	_ = b.WriteByte('\n')
	_, _ = b.WriteString(name)
	_ = b.WriteByte(' ')
	_, _ = b.WriteString(strconv.FormatFloat(value, 'f', -1, 64))
	_ = b.WriteByte('\n')
}

func boolProm(value bool) float64 {
	if value {
		return 1
	}
	return 0
}
