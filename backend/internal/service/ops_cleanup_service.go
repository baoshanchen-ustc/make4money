package service

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"
)

const (
	opsCleanupJobName = "ops_cleanup"

	opsCleanupLeaderLockKeyDefault = "ops:cleanup:leader"
	opsCleanupLeaderLockTTLDefault = 30 * time.Minute
)

var opsCleanupCronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

var opsCleanupReleaseScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("DEL", KEYS[1])
end
return 0
`)

// OpsCleanupService periodically deletes old ops data to prevent unbounded DB growth.
//
// - Scheduling: 5-field cron spec (minute hour dom month dow).
// - Multi-instance: best-effort Redis leader lock so only one node runs cleanup.
// - Safety: deletes in batches to avoid long transactions.
type OpsCleanupService struct {
	opsRepo     OpsRepository
	db          *sql.DB
	redisClient *redis.Client
	cfg         *config.Config

	instanceID string

	cron *cron.Cron

	startOnce sync.Once
	stopOnce  sync.Once

	warnNoRedisOnce sync.Once
}

var (
	cleanupSystemLogRowCount atomic.Int64
	cleanupErrorLogRowCount  atomic.Int64
	cleanupMaxRowsTriggered  atomic.Int64
	cleanupSystemLogLimit    atomic.Int64
	cleanupErrorLogLimit     atomic.Int64
	cleanupMaxRowsEnabled    atomic.Bool
	cleanupMaxRowsDryRun     atomic.Bool
)

func NewOpsCleanupService(
	opsRepo OpsRepository,
	db *sql.DB,
	redisClient *redis.Client,
	cfg *config.Config,
) *OpsCleanupService {
	return &OpsCleanupService{
		opsRepo:     opsRepo,
		db:          db,
		redisClient: redisClient,
		cfg:         cfg,
		instanceID:  uuid.NewString(),
	}
}

func (s *OpsCleanupService) Start() {
	if s == nil {
		return
	}
	if s.cfg != nil && !s.cfg.Ops.Enabled {
		return
	}
	if s.cfg != nil && !s.cfg.Ops.Cleanup.Enabled {
		logger.LegacyPrintf("service.ops_cleanup", "[OpsCleanup] not started (disabled)")
		return
	}
	if s.opsRepo == nil || s.db == nil {
		logger.LegacyPrintf("service.ops_cleanup", "[OpsCleanup] not started (missing deps)")
		return
	}

	s.startOnce.Do(func() {
		schedule := "0 2 * * *"
		if s.cfg != nil && strings.TrimSpace(s.cfg.Ops.Cleanup.Schedule) != "" {
			schedule = strings.TrimSpace(s.cfg.Ops.Cleanup.Schedule)
		}

		loc := time.Local
		if s.cfg != nil && strings.TrimSpace(s.cfg.Timezone) != "" {
			if parsed, err := time.LoadLocation(strings.TrimSpace(s.cfg.Timezone)); err == nil && parsed != nil {
				loc = parsed
			}
		}

		c := cron.New(cron.WithParser(opsCleanupCronParser), cron.WithLocation(loc))
		_, err := c.AddFunc(schedule, func() { s.runScheduled() })
		if err != nil {
			logger.LegacyPrintf("service.ops_cleanup", "[OpsCleanup] not started (invalid schedule=%q): %v", schedule, err)
			return
		}
		s.cron = c
		s.cron.Start()
		logger.LegacyPrintf("service.ops_cleanup", "[OpsCleanup] started (schedule=%q tz=%s)", schedule, loc.String())
	})
}

func (s *OpsCleanupService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		if s.cron != nil {
			ctx := s.cron.Stop()
			select {
			case <-ctx.Done():
			case <-time.After(3 * time.Second):
				logger.LegacyPrintf("service.ops_cleanup", "[OpsCleanup] cron stop timed out")
			}
		}
	})
}

func (s *OpsCleanupService) runScheduled() {
	if s == nil || s.db == nil || s.opsRepo == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	release, ok := s.tryAcquireLeaderLock(ctx)
	if !ok {
		return
	}
	if release != nil {
		defer release()
	}

	startedAt := time.Now().UTC()
	runAt := startedAt

	counts, err := s.runCleanupOnce(ctx)
	if err != nil {
		s.recordHeartbeatError(runAt, time.Since(startedAt), err)
		logger.LegacyPrintf("service.ops_cleanup", "[OpsCleanup] cleanup failed: %v", err)
		return
	}
	s.recordHeartbeatSuccess(runAt, time.Since(startedAt), counts)
	logger.LegacyPrintf("service.ops_cleanup", "[OpsCleanup] cleanup complete: %s", counts)
}

type opsCleanupDeletedCounts struct {
	errorLogs     int64
	retryAttempts int64
	alertEvents   int64
	systemLogs    int64
	logAudits     int64
	systemMetrics int64
	hourlyPreagg  int64
	dailyPreagg   int64
}

func (c opsCleanupDeletedCounts) String() string {
	return fmt.Sprintf(
		"error_logs=%d retry_attempts=%d alert_events=%d system_logs=%d log_audits=%d system_metrics=%d hourly_preagg=%d daily_preagg=%d",
		c.errorLogs,
		c.retryAttempts,
		c.alertEvents,
		c.systemLogs,
		c.logAudits,
		c.systemMetrics,
		c.hourlyPreagg,
		c.dailyPreagg,
	)
}

func (s *OpsCleanupService) runCleanupOnce(ctx context.Context) (opsCleanupDeletedCounts, error) {
	out := opsCleanupDeletedCounts{}
	if s == nil || s.db == nil || s.cfg == nil {
		return out, nil
	}

	cleanupMaxRowsTriggered.Store(0)
	s.updateMaxRowsConfig()

	batchSize := 5000

	now := time.Now().UTC()

	// Keep row-cap visibility current even when time-based retention is disabled.
	s.recordMaxRowsSnapshot(ctx, "ops_error_logs", s.cfg.Ops.Cleanup.ErrorLogMaxRows)
	s.recordMaxRowsSnapshot(ctx, "ops_system_logs", s.cfg.Ops.Cleanup.SystemLogMaxRows)

	// Error-like tables: error logs / retry attempts / alert events.
	if days := s.cfg.Ops.Cleanup.ErrorLogRetentionDays; days > 0 {
		cutoff := now.AddDate(0, 0, -days)
		n, err := deleteOldRowsByID(ctx, s.db, "ops_error_logs", "created_at", cutoff, batchSize, false)
		if err != nil {
			return out, err
		}
		out.errorLogs = n

		n, err = deleteOldRowsByID(ctx, s.db, "ops_retry_attempts", "created_at", cutoff, batchSize, false)
		if err != nil {
			return out, err
		}
		out.retryAttempts = n

		n, err = deleteOldRowsByID(ctx, s.db, "ops_alert_events", "created_at", cutoff, batchSize, false)
		if err != nil {
			return out, err
		}
		out.alertEvents = n

		n, err = deleteOldRowsByID(ctx, s.db, "ops_system_logs", "created_at", cutoff, batchSize, false)
		if err != nil {
			return out, err
		}
		out.systemLogs = n

		n, err = deleteOldRowsByID(ctx, s.db, "ops_system_log_cleanup_audits", "created_at", cutoff, batchSize, false)
		if err != nil {
			return out, err
		}
		out.logAudits = n
	}

	// Minute-level metrics snapshots.
	if days := s.cfg.Ops.Cleanup.MinuteMetricsRetentionDays; days > 0 {
		cutoff := now.AddDate(0, 0, -days)
		n, err := deleteOldRowsByID(ctx, s.db, "ops_system_metrics", "created_at", cutoff, batchSize, false)
		if err != nil {
			return out, err
		}
		out.systemMetrics = n
	}

	// Pre-aggregation tables (hourly/daily).
	if days := s.cfg.Ops.Cleanup.HourlyMetricsRetentionDays; days > 0 {
		cutoff := now.AddDate(0, 0, -days)
		n, err := deleteOldRowsByID(ctx, s.db, "ops_metrics_hourly", "bucket_start", cutoff, batchSize, false)
		if err != nil {
			return out, err
		}
		out.hourlyPreagg = n

		n, err = deleteOldRowsByID(ctx, s.db, "ops_metrics_daily", "bucket_date", cutoff, batchSize, true)
		if err != nil {
			return out, err
		}
		out.dailyPreagg = n
	}

	return out, nil
}

func deleteOldRowsByID(
	ctx context.Context,
	db *sql.DB,
	table string,
	timeColumn string,
	cutoff time.Time,
	batchSize int,
	castCutoffToDate bool,
) (int64, error) {
	if db == nil {
		return 0, nil
	}
	if batchSize <= 0 {
		batchSize = 5000
	}

	q := buildDeleteOldRowsByIDQuery(table, timeColumn, castCutoffToDate)

	var total int64
	for {
		res, err := db.ExecContext(ctx, q, cutoff, batchSize)
		if err != nil {
			// If ops tables aren't present yet (partial deployments), treat as no-op.
			if strings.Contains(strings.ToLower(err.Error()), "does not exist") && strings.Contains(strings.ToLower(err.Error()), "relation") {
				return total, nil
			}
			return total, err
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return total, err
		}
		total += affected
		if affected == 0 {
			break
		}
	}
	return total, nil
}

func buildDeleteOldRowsByIDQuery(table string, timeColumn string, castCutoffToDate bool) string {
	where := fmt.Sprintf("%s < $1", timeColumn)
	if castCutoffToDate {
		where = fmt.Sprintf("%s < $1::date", timeColumn)
	}
	return fmt.Sprintf(`
WITH batch AS (
  SELECT ctid FROM %s
  WHERE %s
  ORDER BY %s ASC, id ASC
  LIMIT $2
)
DELETE FROM %s AS t
USING batch
WHERE t.ctid = batch.ctid
`, table, where, timeColumn, table)
}

func (s *OpsCleanupService) tryAcquireLeaderLock(ctx context.Context) (func(), bool) {
	if s == nil {
		return nil, false
	}
	// In simple run mode, assume single instance.
	if s.cfg != nil && s.cfg.RunMode == config.RunModeSimple {
		return nil, true
	}

	key := opsCleanupLeaderLockKeyDefault
	ttl := opsCleanupLeaderLockTTLDefault

	// Prefer Redis leader lock when available, but avoid stampeding the DB when Redis is flaky by
	// falling back to a DB advisory lock.
	if s.redisClient != nil {
		ok, err := s.redisClient.SetNX(ctx, key, s.instanceID, ttl).Result()
		if err == nil {
			if !ok {
				return nil, false
			}
			return func() {
				_, _ = opsCleanupReleaseScript.Run(ctx, s.redisClient, []string{key}, s.instanceID).Result()
			}, true
		}
		// Redis error: fall back to DB advisory lock.
		s.warnNoRedisOnce.Do(func() {
			logger.LegacyPrintf("service.ops_cleanup", "[OpsCleanup] leader lock SetNX failed; falling back to DB advisory lock: %v", err)
		})
	} else {
		s.warnNoRedisOnce.Do(func() {
			logger.LegacyPrintf("service.ops_cleanup", "[OpsCleanup] redis not configured; using DB advisory lock")
		})
	}

	release, ok := tryAcquireDBAdvisoryLock(ctx, s.db, hashAdvisoryLockID(key))
	if !ok {
		return nil, false
	}
	return release, true
}

func (s *OpsCleanupService) recordHeartbeatSuccess(runAt time.Time, duration time.Duration, counts opsCleanupDeletedCounts) {
	if s == nil || s.opsRepo == nil {
		return
	}
	now := time.Now().UTC()
	durMs := duration.Milliseconds()
	result := truncateString(counts.String(), 2048)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = s.opsRepo.UpsertJobHeartbeat(ctx, &OpsUpsertJobHeartbeatInput{
		JobName:        opsCleanupJobName,
		LastRunAt:      &runAt,
		LastSuccessAt:  &now,
		LastDurationMs: &durMs,
		LastResult:     &result,
	})
}

func (s *OpsCleanupService) recordHeartbeatError(runAt time.Time, duration time.Duration, err error) {
	if s == nil || s.opsRepo == nil || err == nil {
		return
	}
	now := time.Now().UTC()
	durMs := duration.Milliseconds()
	msg := truncateString(err.Error(), 2048)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = s.opsRepo.UpsertJobHeartbeat(ctx, &OpsUpsertJobHeartbeatInput{
		JobName:        opsCleanupJobName,
		LastRunAt:      &runAt,
		LastErrorAt:    &now,
		LastError:      &msg,
		LastDurationMs: &durMs,
	})
}

func (s *OpsCleanupService) updateMaxRowsConfig() {
	if s == nil || s.cfg == nil {
		return
	}
	cleanupMaxRowsEnabled.Store(s.cfg.Ops.Cleanup.MaxRowsEnabled)
	cleanupMaxRowsDryRun.Store(s.cfg.Ops.Cleanup.MaxRowsDryRun)
	cleanupSystemLogLimit.Store(s.cfg.Ops.Cleanup.SystemLogMaxRows)
	cleanupErrorLogLimit.Store(s.cfg.Ops.Cleanup.ErrorLogMaxRows)
}

func (s *OpsCleanupService) recordMaxRowsSnapshot(ctx context.Context, table string, limit int64) {
	if s == nil || s.db == nil || limit <= 0 {
		return
	}
	count, err := estimateTableRows(ctx, s.db, table)
	if err != nil {
		logger.LegacyPrintf("service.ops_cleanup", "[OpsCleanup] estimate rows failed for %s: %v", table, err)
		return
	}
	switch table {
	case "ops_system_logs":
		cleanupSystemLogRowCount.Store(count)
	case "ops_error_logs":
		cleanupErrorLogRowCount.Store(count)
	}
	if cleanupMaxRowsEnabled.Load() && count > limit {
		cleanupMaxRowsTriggered.Add(1)
		if cleanupMaxRowsDryRun.Load() {
			logger.LegacyPrintf("service.ops_cleanup", "[OpsCleanup] max rows dry-run triggered for %s (count=%d limit=%d)", table, count, limit)
		} else {
			logger.LegacyPrintf("service.ops_cleanup", "[OpsCleanup] max rows enforced for %s (count=%d limit=%d)", table, count, limit)
		}
	}
}

func estimateTableRows(ctx context.Context, db *sql.DB, table string) (int64, error) {
	if db == nil || table == "" {
		return 0, fmt.Errorf("invalid table")
	}
	switch table {
	case "ops_system_logs", "ops_error_logs":
	default:
		return 0, fmt.Errorf("unsupported table %q", table)
	}
	var count int64
	const query = `
SELECT COALESCE(st.n_live_tup::bigint, c.reltuples::bigint, 0)
FROM pg_class c
LEFT JOIN pg_stat_user_tables st ON st.relid = c.oid
WHERE c.relname = $1
LIMIT 1`
	if err := db.QueryRowContext(ctx, query, table).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func SnapshotCleanupStats() CleanupStats {
	return CleanupStats{
		SystemLogRows:  cleanupSystemLogRowCount.Load(),
		ErrorLogRows:   cleanupErrorLogRowCount.Load(),
		SystemLogLimit: cleanupSystemLogLimit.Load(),
		ErrorLogLimit:  cleanupErrorLogLimit.Load(),
		MaxRowsEnabled: cleanupMaxRowsEnabled.Load(),
		MaxRowsDryRun:  cleanupMaxRowsDryRun.Load(),
		MaxRowsHit:     cleanupMaxRowsTriggered.Load() > 0,
	}
}

type CleanupStats struct {
	SystemLogRows  int64
	ErrorLogRows   int64
	SystemLogLimit int64
	ErrorLogLimit  int64
	MaxRowsEnabled bool
	MaxRowsDryRun  bool
	MaxRowsHit     bool
}

func resetCleanupStatsForTest() {
	cleanupSystemLogRowCount.Store(0)
	cleanupErrorLogRowCount.Store(0)
	cleanupMaxRowsTriggered.Store(0)
	cleanupSystemLogLimit.Store(0)
	cleanupErrorLogLimit.Store(0)
	cleanupMaxRowsEnabled.Store(false)
	cleanupMaxRowsDryRun.Store(false)
}
