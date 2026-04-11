package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
)

const (
	opsMetricsCollectorJobName     = "ops_metrics_collector"
	opsMetricsCollectorMinInterval = 60 * time.Second
	opsMetricsCollectorMaxInterval = 1 * time.Hour

	opsMetricsCollectorTimeout = 10 * time.Second

	opsStorageGovernanceSampleInterval = 15 * time.Minute

	opsMetricsCollectorLeaderLockKey = "ops:metrics:collector:leader"
	opsMetricsCollectorLeaderLockTTL = 90 * time.Second

	opsMetricsCollectorHeartbeatTimeout = 2 * time.Second

	bytesPerMB = 1024 * 1024

	opsRuntimeStorageGovernanceSummaryComponent = "ops.runtime.storage_governance.summary"

	opsStorageUsageLogsWarnBytes      = 256 * bytesPerMB
	opsStorageUsageLogsCriticalBytes  = 512 * bytesPerMB
	opsStorageSystemLogsWarnBytes     = 512 * bytesPerMB
	opsStorageSystemLogsCriticalBytes = 1024 * bytesPerMB
	opsStorageErrorLogsWarnBytes      = 128 * bytesPerMB
	opsStorageErrorLogsCriticalBytes  = 256 * bytesPerMB

	opsStorageColdIndexWarnBytes           = 16 * bytesPerMB
	opsStorageColdIndexCriticalBytes       = 64 * bytesPerMB
	opsRuntimeCleanupSummaryComponent      = "ops.runtime.cleanup.summary"
	opsRuntimeCleanupUsageSummaryComponent = "ops.runtime.cleanup.usage.summary"
)

var opsMetricsCollectorAdvisoryLockID = hashAdvisoryLockID(opsMetricsCollectorLeaderLockKey)

type OpsMetricsCollector struct {
	opsRepo     OpsRepository
	settingRepo SettingRepository
	cfg         *config.Config

	accountRepo        AccountRepository
	concurrencyService *ConcurrencyService

	db          *sql.DB
	redisClient *redis.Client
	instanceID  string

	lastCgroupCPUUsageNanos uint64
	lastCgroupCPUSampleAt   time.Time

	stopCh    chan struct{}
	startOnce sync.Once
	stopOnce  sync.Once

	skipLogMu sync.Mutex
	skipLogAt time.Time

	runtimeDeltaMu              sync.Mutex
	runtimeDeltaBaselineSet     bool
	lastUsageLogNotPersisted    int64
	lastBillingCompensation     int64
	lastSchedulerPoisonTotal    int64
	lastSchedulerTransientTotal int64
	lastUsageWorkerDropped      uint64
	lastUsageWorkerSyncFallback uint64
	lastUsageWorkerTimeouts     uint64
	lastUsageWorkerPanics       uint64
	lastDBWaitCount             uint64
	lastRedisPoolStalls         int64
	lastRedisPoolTimeouts       int64
	lastStorageGovernanceSample time.Time
	lastStorageGovernanceState  string

	redisPoolStatsFn         func() opsRedisPoolRuntimeStats
	dbPoolStatsFn            func() (active int, idle int, waitCount uint64, waitDuration time.Duration)
	storageGovernanceStatsFn func(ctx context.Context) (opsStorageGovernanceRuntimeSnapshot, error)
}

func NewOpsMetricsCollector(
	opsRepo OpsRepository,
	settingRepo SettingRepository,
	accountRepo AccountRepository,
	concurrencyService *ConcurrencyService,
	db *sql.DB,
	redisClient *redis.Client,
	cfg *config.Config,
) *OpsMetricsCollector {
	return &OpsMetricsCollector{
		opsRepo:            opsRepo,
		settingRepo:        settingRepo,
		cfg:                cfg,
		accountRepo:        accountRepo,
		concurrencyService: concurrencyService,
		db:                 db,
		redisClient:        redisClient,
		instanceID:         uuid.NewString(),
	}
}

func (c *OpsMetricsCollector) Start() {
	if c == nil {
		return
	}
	c.startOnce.Do(func() {
		if c.stopCh == nil {
			c.stopCh = make(chan struct{})
		}
		go c.run()
	})
}

func (c *OpsMetricsCollector) Stop() {
	if c == nil {
		return
	}
	c.stopOnce.Do(func() {
		if c.stopCh != nil {
			close(c.stopCh)
		}
	})
}

func (c *OpsMetricsCollector) run() {
	// First run immediately so the dashboard has data soon after startup.
	c.collectOnce()

	for {
		interval := c.getInterval()
		timer := time.NewTimer(interval)
		select {
		case <-timer.C:
			c.collectOnce()
		case <-c.stopCh:
			timer.Stop()
			return
		}
	}
}

func (c *OpsMetricsCollector) getInterval() time.Duration {
	interval := opsMetricsCollectorMinInterval

	if c.settingRepo == nil {
		return interval
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	raw, err := c.settingRepo.GetValue(ctx, SettingKeyOpsMetricsIntervalSeconds)
	if err != nil {
		return interval
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return interval
	}

	seconds, err := strconv.Atoi(raw)
	if err != nil {
		return interval
	}
	if seconds < int(opsMetricsCollectorMinInterval.Seconds()) {
		seconds = int(opsMetricsCollectorMinInterval.Seconds())
	}
	if seconds > int(opsMetricsCollectorMaxInterval.Seconds()) {
		seconds = int(opsMetricsCollectorMaxInterval.Seconds())
	}
	return time.Duration(seconds) * time.Second
}

func (c *OpsMetricsCollector) collectOnce() {
	if c == nil {
		return
	}
	if c.cfg != nil && !c.cfg.Ops.Enabled {
		return
	}
	if c.opsRepo == nil {
		return
	}
	if c.db == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), opsMetricsCollectorTimeout)
	defer cancel()

	if !c.isMonitoringEnabled(ctx) {
		return
	}

	release, ok := c.tryAcquireLeaderLock(ctx)
	if !ok {
		return
	}
	if release != nil {
		defer release()
	}

	startedAt := time.Now().UTC()
	err := c.collectAndPersist(ctx)
	finishedAt := time.Now().UTC()

	durationMs := finishedAt.Sub(startedAt).Milliseconds()
	dur := durationMs
	runAt := startedAt

	if err != nil {
		msg := truncateString(err.Error(), 2048)
		errAt := finishedAt
		hbCtx, hbCancel := context.WithTimeout(context.Background(), opsMetricsCollectorHeartbeatTimeout)
		defer hbCancel()
		_ = c.opsRepo.UpsertJobHeartbeat(hbCtx, &OpsUpsertJobHeartbeatInput{
			JobName:        opsMetricsCollectorJobName,
			LastRunAt:      &runAt,
			LastErrorAt:    &errAt,
			LastError:      &msg,
			LastDurationMs: &dur,
		})
		log.Printf("[OpsMetricsCollector] collect failed: %v", err)
		return
	}

	successAt := finishedAt
	hbCtx, hbCancel := context.WithTimeout(context.Background(), opsMetricsCollectorHeartbeatTimeout)
	defer hbCancel()
	_ = c.opsRepo.UpsertJobHeartbeat(hbCtx, &OpsUpsertJobHeartbeatInput{
		JobName:        opsMetricsCollectorJobName,
		LastRunAt:      &runAt,
		LastSuccessAt:  &successAt,
		LastDurationMs: &dur,
	})
}

func (c *OpsMetricsCollector) isMonitoringEnabled(ctx context.Context) bool {
	if c == nil {
		return false
	}
	if c.cfg != nil && !c.cfg.Ops.Enabled {
		return false
	}
	if c.settingRepo == nil {
		return true
	}
	if ctx == nil {
		ctx = context.Background()
	}

	value, err := c.settingRepo.GetValue(ctx, SettingKeyOpsMonitoringEnabled)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return true
		}
		// Fail-open: collector should not become a hard dependency.
		return true
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "false", "0", "off", "disabled":
		return false
	default:
		return true
	}
}

func (c *OpsMetricsCollector) collectAndPersist(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	// Align to stable minute boundaries to avoid partial buckets and to maximize cache hits.
	now := time.Now().UTC()
	windowEnd := now.Truncate(time.Minute)
	windowStart := windowEnd.Add(-1 * time.Minute)

	sys, err := c.collectSystemStats(ctx)
	if err != nil {
		// Continue; system stats are best-effort.
		log.Printf("[OpsMetricsCollector] system stats error: %v", err)
	}

	dbOK := c.checkDB(ctx)
	redisOK := c.checkRedis(ctx)
	active, idle, dbWaitCount, _ := c.dbPoolStats()
	redisTotal, redisIdle, redisStatsOK := c.redisPoolStats()

	successCount, tokenConsumed, err := c.queryUsageCounts(ctx, windowStart, windowEnd)
	if err != nil {
		return fmt.Errorf("query usage counts: %w", err)
	}

	duration, ttft, err := c.queryUsageLatency(ctx, windowStart, windowEnd)
	if err != nil {
		return fmt.Errorf("query usage latency: %w", err)
	}

	errorTotal, businessLimited, errorSLA, upstreamExcl, upstream429, upstream529, err := c.queryErrorCounts(ctx, windowStart, windowEnd)
	if err != nil {
		return fmt.Errorf("query error counts: %w", err)
	}

	accountSwitchCount, err := c.queryAccountSwitchCount(ctx, windowStart, windowEnd)
	if err != nil {
		return fmt.Errorf("query account switch counts: %w", err)
	}

	windowSeconds := windowEnd.Sub(windowStart).Seconds()
	if windowSeconds <= 0 {
		windowSeconds = 60
	}
	requestTotal := successCount + errorTotal
	qps := float64(requestTotal) / windowSeconds
	tps := float64(tokenConsumed) / windowSeconds

	goroutines := runtime.NumGoroutine()
	concurrencyQueueDepth := c.collectConcurrencyQueueDepth(ctx)

	dbConnWaiting := int(clampNonNegativeDeltaUint(dbWaitCount, c.lastDBWaitCount))
	c.lastDBWaitCount = dbWaitCount

	input := &OpsInsertSystemMetricsInput{
		CreatedAt:     windowEnd,
		WindowMinutes: 1,

		SuccessCount:         successCount,
		ErrorCountTotal:      errorTotal,
		BusinessLimitedCount: businessLimited,
		ErrorCountSLA:        errorSLA,

		UpstreamErrorCountExcl429529: upstreamExcl,
		Upstream429Count:             upstream429,
		Upstream529Count:             upstream529,

		TokenConsumed:      tokenConsumed,
		AccountSwitchCount: accountSwitchCount,
		QPS:                float64Ptr(roundTo1DP(qps)),
		TPS:                float64Ptr(roundTo1DP(tps)),

		DurationP50Ms: duration.p50,
		DurationP90Ms: duration.p90,
		DurationP95Ms: duration.p95,
		DurationP99Ms: duration.p99,
		DurationAvgMs: duration.avg,
		DurationMaxMs: duration.max,

		TTFTP50Ms: ttft.p50,
		TTFTP90Ms: ttft.p90,
		TTFTP95Ms: ttft.p95,
		TTFTP99Ms: ttft.p99,
		TTFTAvgMs: ttft.avg,
		TTFTMaxMs: ttft.max,

		CPUUsagePercent:    sys.cpuUsagePercent,
		MemoryUsedMB:       sys.memoryUsedMB,
		MemoryTotalMB:      sys.memoryTotalMB,
		MemoryUsagePercent: sys.memoryUsagePercent,

		DBOK:    boolPtr(dbOK),
		RedisOK: boolPtr(redisOK),

		RedisConnTotal: func() *int {
			if !redisStatsOK {
				return nil
			}
			return intPtr(redisTotal)
		}(),
		RedisConnIdle: func() *int {
			if !redisStatsOK {
				return nil
			}
			return intPtr(redisIdle)
		}(),

		DBConnActive:          intPtr(active),
		DBConnIdle:            intPtr(idle),
		DBConnWaiting:         intPtr(dbConnWaiting),
		GoroutineCount:        intPtr(goroutines),
		ConcurrencyQueueDepth: concurrencyQueueDepth,
	}

	if err := c.opsRepo.InsertSystemMetrics(ctx, input); err != nil {
		return err
	}
	if err := c.persistRuntimeAnomalySummary(ctx, windowEnd); err != nil {
		log.Printf("[OpsMetricsCollector] runtime anomaly summary failed: %v", err)
	}
	return nil
}

func (c *OpsMetricsCollector) persistRuntimeAnomalySummary(ctx context.Context, createdAt time.Time) error {
	if c == nil || c.opsRepo == nil {
		return nil
	}

	usage := SnapshotUsageLogNotPersistedMetrics()
	billing := SnapshotBillingCompensationCandidates()
	scheduler := SnapshotSchedulerOutboxRuntimeMetrics()
	worker := SnapshotUsageRecordWorkerPoolStats()
	redisPool := c.snapshotRedisPoolRuntimeStats()
	storageSampleDue := c.shouldSampleStorageGovernance(createdAt)
	var storage opsStorageGovernanceRuntimeSnapshot
	var storageState string
	if storageSampleDue {
		var err error
		storage, err = c.snapshotStorageGovernanceRuntimeStats(ctx)
		if err != nil {
			return err
		}
		storageState = storage.stateFingerprint()
	}

	c.runtimeDeltaMu.Lock()
	defer c.runtimeDeltaMu.Unlock()

	if !c.runtimeDeltaBaselineSet {
		c.lastUsageLogNotPersisted = usage.Total
		c.lastBillingCompensation = billing.Total
		c.lastSchedulerPoisonTotal = scheduler.PoisonTotal
		c.lastSchedulerTransientTotal = scheduler.TransientTotal
		c.lastUsageWorkerDropped = worker.DroppedQueueFull
		c.lastUsageWorkerSyncFallback = worker.SyncFallbackTasks
		c.lastUsageWorkerTimeouts = worker.TaskTimeouts
		c.lastUsageWorkerPanics = worker.TaskPanics
		c.lastRedisPoolStalls = redisPool.Stalls
		c.lastRedisPoolTimeouts = redisPool.Timeouts
		if storageSampleDue {
			c.lastStorageGovernanceSample = createdAt
			c.lastStorageGovernanceState = storageState
		}
		c.runtimeDeltaBaselineSet = true
		return nil
	}

	usageDelta := clampNonNegativeDelta(usage.Total, c.lastUsageLogNotPersisted)
	billingDelta := clampNonNegativeDelta(billing.Total, c.lastBillingCompensation)
	poisonDelta := clampNonNegativeDelta(scheduler.PoisonTotal, c.lastSchedulerPoisonTotal)
	transientDelta := clampNonNegativeDelta(scheduler.TransientTotal, c.lastSchedulerTransientTotal)
	workerDroppedDelta := clampNonNegativeDeltaUint(worker.DroppedQueueFull, c.lastUsageWorkerDropped)
	workerSyncFallbackDelta := clampNonNegativeDeltaUint(worker.SyncFallbackTasks, c.lastUsageWorkerSyncFallback)
	workerTimeoutDelta := clampNonNegativeDeltaUint(worker.TaskTimeouts, c.lastUsageWorkerTimeouts)
	workerPanicDelta := clampNonNegativeDeltaUint(worker.TaskPanics, c.lastUsageWorkerPanics)
	redisStallDelta := clampNonNegativeDelta(redisPool.Stalls, c.lastRedisPoolStalls)
	redisTimeoutDelta := clampNonNegativeDelta(redisPool.Timeouts, c.lastRedisPoolTimeouts)

	c.lastUsageLogNotPersisted = usage.Total
	c.lastBillingCompensation = billing.Total
	c.lastSchedulerPoisonTotal = scheduler.PoisonTotal
	c.lastSchedulerTransientTotal = scheduler.TransientTotal
	c.lastUsageWorkerDropped = worker.DroppedQueueFull
	c.lastUsageWorkerSyncFallback = worker.SyncFallbackTasks
	c.lastUsageWorkerTimeouts = worker.TaskTimeouts
	c.lastUsageWorkerPanics = worker.TaskPanics
	c.lastRedisPoolStalls = redisPool.Stalls
	c.lastRedisPoolTimeouts = redisPool.Timeouts
	if storageSampleDue {
		c.lastStorageGovernanceSample = createdAt
	}

	inputs := make([]*OpsInsertSystemLogInput, 0, 6)
	if usageDelta > 0 {
		extra, err := marshalOpsRuntimeExtra(map[string]any{
			"kind":             "summary",
			"delta":            usageDelta,
			"total":            usage.Total,
			"recent_1m_total":  usage.Recent1mTotal,
			"recent_5m_total":  usage.Recent5mTotal,
			"recent_15m_total": usage.Recent15mTotal,
			"last":             usage.Last,
		})
		if err != nil {
			return err
		}
		inputs = append(inputs, &OpsInsertSystemLogInput{
			CreatedAt: createdAt,
			Level:     "warn",
			Component: OpsRuntimeUsageLogSummaryComponent,
			Message:   "usage log not persisted delta observed",
			ExtraJSON: extra,
		})
	}

	if billingDelta > 0 {
		extra, err := marshalOpsRuntimeExtra(map[string]any{
			"kind":             "summary",
			"delta":            billingDelta,
			"total":            billing.Total,
			"recent_1m_total":  billing.Recent1mTotal,
			"recent_5m_total":  billing.Recent5mTotal,
			"recent_15m_total": billing.Recent15mTotal,
			"last":             billing.Last,
		})
		if err != nil {
			return err
		}
		inputs = append(inputs, &OpsInsertSystemLogInput{
			CreatedAt: createdAt,
			Level:     "warn",
			Component: OpsRuntimeBillingCompensationSummaryComponent,
			Message:   "billing compensation summary delta observed",
			ExtraJSON: extra,
		})
	}

	if poisonDelta > 0 || transientDelta > 0 {
		extra, err := marshalOpsRuntimeExtra(map[string]any{
			"kind":                            "summary",
			"poison_delta":                    poisonDelta,
			"transient_delta":                 transientDelta,
			"poison_total":                    scheduler.PoisonTotal,
			"transient_total":                 scheduler.TransientTotal,
			"payload_decode_poison_total":     scheduler.PayloadDecodePoisonTotal,
			"malformed_payload_poison_total":  scheduler.MalformedPayloadPoisonTotal,
			"unknown_event_poison_total":      scheduler.UnknownEventPoisonTotal,
			"lock_contention_transient_total": scheduler.LockContentionTransientTotal,
			"db_transient_total":              scheduler.DBTransientTotal,
			"cache_transient_total":           scheduler.CacheTransientTotal,
			"other_transient_total":           scheduler.OtherTransientTotal,
			"last_poison":                     scheduler.LastPoison,
			"last_transient":                  scheduler.LastTransient,
		})
		if err != nil {
			return err
		}
		inputs = append(inputs, &OpsInsertSystemLogInput{
			CreatedAt: createdAt,
			Level:     "warn",
			Component: OpsRuntimeSchedulerOutboxSummaryComponent,
			Message:   "scheduler outbox anomaly delta observed",
			ExtraJSON: extra,
		})
	}

	if workerDroppedDelta > 0 || workerSyncFallbackDelta > 0 || workerTimeoutDelta > 0 || workerPanicDelta > 0 {
		extra, err := marshalOpsRuntimeExtra(map[string]any{
			"kind":                "summary",
			"dropped_delta":       workerDroppedDelta,
			"sync_fallback_delta": workerSyncFallbackDelta,
			"timeout_delta":       workerTimeoutDelta,
			"panic_delta":         workerPanicDelta,
			"queue_depth":         worker.QueueDepth,
			"waiting_tasks":       worker.WaitingTasks,
			"submitted_tasks":     worker.SubmittedTasks,
			"completed_tasks":     worker.CompletedTasks,
			"successful_tasks":    worker.SuccessfulTasks,
			"failed_tasks":        worker.FailedTasks,
			"dropped_tasks":       worker.DroppedTasks,
			"dropped_queue_full":  worker.DroppedQueueFull,
			"sync_fallback_tasks": worker.SyncFallbackTasks,
			"task_timeouts":       worker.TaskTimeouts,
			"task_panics":         worker.TaskPanics,
		})
		if err != nil {
			return err
		}
		inputs = append(inputs, &OpsInsertSystemLogInput{
			CreatedAt: createdAt,
			Level:     "warn",
			Component: OpsRuntimeUsageWorkerSummaryComponent,
			Message:   "usage record worker anomaly delta observed",
			ExtraJSON: extra,
		})
	}

	if redisStallDelta > 0 || redisTimeoutDelta > 0 {
		extra, err := marshalOpsRuntimeExtra(map[string]any{
			"kind":          "summary",
			"stall_delta":   redisStallDelta,
			"timeout_delta": redisTimeoutDelta,
			"stalls":        redisPool.Stalls,
			"timeouts":      redisPool.Timeouts,
		})
		if err != nil {
			return err
		}
		inputs = append(inputs, &OpsInsertSystemLogInput{
			CreatedAt: createdAt,
			Level:     "warn",
			Component: OpsRuntimeRedisPoolSummaryComponent,
			Message:   "redis pool anomaly delta observed",
			ExtraJSON: extra,
		})
	}

	if storageSampleDue {
		previousState := c.lastStorageGovernanceState
		c.lastStorageGovernanceState = storageState
		if storageState != previousState {
			extra, err := storage.extraJSON()
			if err != nil {
				return err
			}
			level := "info"
			message := "ops storage governance risks recovered"
			if storage.HasRisk {
				level = "warn"
				message = "ops storage governance risk summary observed"
				if storage.CriticalTables > 0 {
					level = "error"
				}
			}
			inputs = append(inputs, &OpsInsertSystemLogInput{
				CreatedAt: createdAt,
				Level:     level,
				Component: opsRuntimeStorageGovernanceSummaryComponent,
				Message:   message,
				ExtraJSON: extra,
			})
		}
	}

	cleanup := SnapshotCleanupStats()
	if cleanup.MaxRowsEnabled {
		extra, err := marshalOpsRuntimeExtra(map[string]any{
			"kind":             "summary",
			"system_rows":      cleanup.SystemLogRows,
			"error_rows":       cleanup.ErrorLogRows,
			"system_limit":     cleanup.SystemLogLimit,
			"error_limit":      cleanup.ErrorLogLimit,
			"max_rows_hit":     cleanup.MaxRowsHit,
			"max_rows_dry_run": cleanup.MaxRowsDryRun,
		})
		if err != nil {
			return err
		}
		inputs = append(inputs, &OpsInsertSystemLogInput{
			CreatedAt: createdAt,
			Level:     "warn",
			Component: opsRuntimeCleanupSummaryComponent,
			Message:   "ops cleanup max rows status",
			ExtraJSON: extra,
		})
	}

	usageCleanup := SnapshotUsageCleanupStats()
	if usageCleanup.LastTaskID > 0 {
		extra, err := marshalOpsRuntimeExtra(map[string]any{
			"kind":         "summary",
			"last_task_id": usageCleanup.LastTaskID,
			"deleted_rows": usageCleanup.LastDeletedRows,
		})
		if err != nil {
			return err
		}
		inputs = append(inputs, &OpsInsertSystemLogInput{
			CreatedAt: createdAt,
			Level:     "warn",
			Component: opsRuntimeCleanupUsageSummaryComponent,
			Message:   "usage cleanup task snapshot",
			ExtraJSON: extra,
		})
	}

	if len(inputs) == 0 {
		return nil
	}
	if ctx == nil || ctx.Err() != nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, opsMetricsCollectorTimeout)
	defer cancel()
	_, err := c.opsRepo.BatchInsertSystemLogs(ctx, inputs)
	return err
}

func (c *OpsMetricsCollector) shouldSampleStorageGovernance(createdAt time.Time) bool {
	if c == nil {
		return false
	}
	c.runtimeDeltaMu.Lock()
	defer c.runtimeDeltaMu.Unlock()
	if c.lastStorageGovernanceSample.IsZero() {
		return true
	}
	return createdAt.Sub(c.lastStorageGovernanceSample) >= opsStorageGovernanceSampleInterval
}

type opsStorageGovernanceRuntimeSnapshot struct {
	Kind                  string                              `json:"kind"`
	SampledTables         int                                 `json:"sampled_tables"`
	RiskTables            int                                 `json:"risk_tables"`
	WarnTables            int                                 `json:"warn_tables"`
	CriticalTables        int                                 `json:"critical_tables"`
	HasRisk               bool                                `json:"has_risk"`
	SampleIntervalMinutes int                                 `json:"sample_interval_minutes"`
	Tables                []opsStorageGovernanceTableSnapshot `json:"tables"`
}

type opsStorageGovernanceTableSnapshot struct {
	Table                   string   `json:"table"`
	TotalBytes              int64    `json:"total_bytes"`
	TotalMB                 float64  `json:"total_mb"`
	TableBytes              int64    `json:"table_bytes"`
	IndexBytes              int64    `json:"index_bytes"`
	LiveRows                int64    `json:"live_rows"`
	DeadRows                int64    `json:"dead_rows"`
	ApproxBytesPerLiveRow   float64  `json:"approx_bytes_per_live_row"`
	Partitioned             bool     `json:"partitioned"`
	RetentionManagedBy      string   `json:"retention_managed_by"`
	RetentionStrategy       string   `json:"retention_strategy"`
	RetentionDays           int      `json:"retention_days"`
	RetentionEnabled        bool     `json:"retention_enabled"`
	MaxRowsConfigured       int64    `json:"max_rows_configured"`
	MaxRowsEnabled          bool     `json:"max_rows_enabled"`
	MaxRowsExceeded         bool     `json:"max_rows_exceeded"`
	SizeRisk                string   `json:"size_risk"`
	ColdIndexRisk           string   `json:"cold_index_risk"`
	RetentionRisk           string   `json:"retention_risk"`
	ColdIndexCandidateCount int      `json:"cold_index_candidate_count"`
	ColdIndexCandidateBytes int64    `json:"cold_index_candidate_bytes"`
	LargestColdIndexName    string   `json:"largest_cold_index_name,omitempty"`
	LargestColdIndexBytes   int64    `json:"largest_cold_index_bytes,omitempty"`
	Reasons                 []string `json:"reasons,omitempty"`
}

type opsStorageGovernanceIndexStat struct {
	Table            string
	Name             string
	Bytes            int64
	ScanCount        int64
	Primary          bool
	ConstraintBacked bool
}

func (s opsStorageGovernanceRuntimeSnapshot) stateFingerprint() string {
	type stateTable struct {
		Table         string `json:"table"`
		SizeRisk      string `json:"size_risk"`
		ColdIndexRisk string `json:"cold_index_risk"`
		RetentionRisk string `json:"retention_risk"`
	}
	state := make([]stateTable, 0, len(s.Tables))
	for _, table := range s.Tables {
		state = append(state, stateTable{
			Table:         table.Table,
			SizeRisk:      table.SizeRisk,
			ColdIndexRisk: table.ColdIndexRisk,
			RetentionRisk: table.RetentionRisk,
		})
	}
	raw, err := marshalOpsRuntimeExtra(map[string]any{
		"risk_tables":     s.RiskTables,
		"warn_tables":     s.WarnTables,
		"critical_tables": s.CriticalTables,
		"tables":          state,
	})
	if err != nil {
		return ""
	}
	return raw
}

func (s opsStorageGovernanceRuntimeSnapshot) extraJSON() (string, error) {
	return marshalOpsRuntimeExtra(map[string]any{
		"kind":                    "summary",
		"sampled_tables":          s.SampledTables,
		"risk_tables":             s.RiskTables,
		"warn_tables":             s.WarnTables,
		"critical_tables":         s.CriticalTables,
		"sample_interval_minutes": s.SampleIntervalMinutes,
		"tables":                  s.Tables,
	})
}

func (c *OpsMetricsCollector) snapshotStorageGovernanceRuntimeStats(ctx context.Context) (opsStorageGovernanceRuntimeSnapshot, error) {
	if c == nil {
		return opsStorageGovernanceRuntimeSnapshot{}, nil
	}
	if c.storageGovernanceStatsFn != nil {
		return c.storageGovernanceStatsFn(ctx)
	}
	if c.db == nil {
		return opsStorageGovernanceRuntimeSnapshot{}, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	tableRows, err := c.queryStorageGovernanceTableStats(ctx)
	if err != nil {
		return opsStorageGovernanceRuntimeSnapshot{}, err
	}
	indexRows, err := c.queryStorageGovernanceIndexStats(ctx)
	if err != nil {
		return opsStorageGovernanceRuntimeSnapshot{}, err
	}

	indexByTable := make(map[string][]opsStorageGovernanceIndexStat, len(indexRows))
	for _, item := range indexRows {
		indexByTable[item.Table] = append(indexByTable[item.Table], item)
	}

	snapshot := opsStorageGovernanceRuntimeSnapshot{
		Kind:                  "summary",
		SampledTables:         len(tableRows),
		SampleIntervalMinutes: int(opsStorageGovernanceSampleInterval / time.Minute),
		Tables:                make([]opsStorageGovernanceTableSnapshot, 0, len(tableRows)),
	}
	for _, table := range tableRows {
		item := buildStorageGovernanceTableSnapshot(c.cfg, table, indexByTable[table.Table])
		snapshot.Tables = append(snapshot.Tables, item)
		if hasStorageRiskLevel(item.SizeRisk) || hasStorageRiskLevel(item.ColdIndexRisk) || hasStorageRiskLevel(item.RetentionRisk) {
			snapshot.HasRisk = true
			snapshot.RiskTables++
		}
		if isCriticalStorageRisk(item.SizeRisk) || isCriticalStorageRisk(item.ColdIndexRisk) || isCriticalStorageRisk(item.RetentionRisk) {
			snapshot.CriticalTables++
			continue
		}
		if isWarnStorageRisk(item.SizeRisk) || isWarnStorageRisk(item.ColdIndexRisk) || isWarnStorageRisk(item.RetentionRisk) {
			snapshot.WarnTables++
		}
	}
	return snapshot, nil
}

func buildStorageGovernanceTableSnapshot(cfg *config.Config, table opsStorageGovernanceTableSnapshot, indexes []opsStorageGovernanceIndexStat) opsStorageGovernanceTableSnapshot {
	out := table
	for _, index := range indexes {
		if index.ScanCount > 0 || index.Primary || index.ConstraintBacked {
			continue
		}
		out.ColdIndexCandidateCount++
		out.ColdIndexCandidateBytes += index.Bytes
		if index.Bytes > out.LargestColdIndexBytes {
			out.LargestColdIndexBytes = index.Bytes
			out.LargestColdIndexName = index.Name
		}
	}
	if out.LiveRows > 0 && out.TotalBytes > 0 {
		out.ApproxBytesPerLiveRow = roundTo1DP(float64(out.TotalBytes) / float64(out.LiveRows))
	}
	out.TotalMB = roundTo1DP(float64(out.TotalBytes) / float64(bytesPerMB))

	switch out.Table {
	case "usage_logs":
		out.RetentionManagedBy = "dashboard_aggregation.retention.usage_logs_days"
		out.RetentionStrategy = "dashboard_aggregation_cleanup"
		if cfg != nil {
			out.RetentionDays = cfg.DashboardAgg.Retention.UsageLogsDays
			out.RetentionEnabled = cfg.DashboardAgg.Enabled && cfg.DashboardAgg.Retention.UsageLogsDays > 0
			out.MaxRowsConfigured = cfg.DashboardAgg.Retention.UsageLogsMaxRows
			out.MaxRowsEnabled = cfg.DashboardAgg.Retention.UsageLogsMaxRows > 0
		}
	case "ops_system_logs", "ops_error_logs":
		out.RetentionManagedBy = "ops.cleanup.error_log_retention_days"
		out.RetentionStrategy = "ops_cleanup_batch_delete"
		if cfg != nil {
			out.RetentionDays = cfg.Ops.Cleanup.ErrorLogRetentionDays
			out.RetentionEnabled = cfg.Ops.Cleanup.ErrorLogRetentionDays > 0
			if out.Table == "ops_system_logs" {
				out.MaxRowsConfigured = cfg.Ops.Cleanup.SystemLogMaxRows
				out.MaxRowsEnabled = cfg.Ops.Cleanup.SystemLogMaxRows > 0
			} else {
				out.MaxRowsConfigured = cfg.Ops.Cleanup.ErrorLogMaxRows
				out.MaxRowsEnabled = cfg.Ops.Cleanup.ErrorLogMaxRows > 0
			}
		}
	default:
		out.RetentionManagedBy = "unknown"
	}
	if out.MaxRowsEnabled && out.LiveRows > out.MaxRowsConfigured {
		out.MaxRowsExceeded = true
	}

	out.SizeRisk = classifyStorageSizeRisk(out.Table, out.TotalBytes)
	out.ColdIndexRisk = classifyColdIndexRisk(out.ColdIndexCandidateCount, out.ColdIndexCandidateBytes)
	out.RetentionRisk, out.Reasons = classifyRetentionRisk(cfg, out)
	if isWarnStorageRisk(out.SizeRisk) {
		out.Reasons = append(out.Reasons, fmt.Sprintf("table_size_%s", out.SizeRisk))
	}
	if isWarnStorageRisk(out.ColdIndexRisk) {
		out.Reasons = append(out.Reasons, fmt.Sprintf("cold_secondary_indexes_%s", out.ColdIndexRisk))
	}
	return out
}

func (c *OpsMetricsCollector) queryStorageGovernanceTableStats(ctx context.Context) ([]opsStorageGovernanceTableSnapshot, error) {
	q := `
SELECT
  st.relname,
  COALESCE(pg_total_relation_size(st.relid), 0) AS total_bytes,
  COALESCE(pg_relation_size(st.relid), 0) AS table_bytes,
  COALESCE(pg_indexes_size(st.relid), 0) AS index_bytes,
  COALESCE(st.n_live_tup, 0) AS live_rows,
  COALESCE(st.n_dead_tup, 0) AS dead_rows,
  EXISTS (SELECT 1 FROM pg_inherits i WHERE i.inhparent = st.relid) AS partitioned
FROM pg_stat_user_tables st
WHERE st.relname IN ('usage_logs', 'ops_system_logs', 'ops_error_logs')
ORDER BY CASE st.relname
  WHEN 'usage_logs' THEN 1
  WHEN 'ops_system_logs' THEN 2
  WHEN 'ops_error_logs' THEN 3
  ELSE 99
END`

	rows, err := c.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]opsStorageGovernanceTableSnapshot, 0, 3)
	for rows.Next() {
		var item opsStorageGovernanceTableSnapshot
		if err := rows.Scan(
			&item.Table,
			&item.TotalBytes,
			&item.TableBytes,
			&item.IndexBytes,
			&item.LiveRows,
			&item.DeadRows,
			&item.Partitioned,
		); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *OpsMetricsCollector) queryStorageGovernanceIndexStats(ctx context.Context) ([]opsStorageGovernanceIndexStat, error) {
	q := `
SELECT
  si.relname,
  si.indexrelname,
  COALESCE(pg_relation_size(si.indexrelid), 0) AS index_bytes,
  COALESCE(si.idx_scan, 0) AS idx_scan,
  COALESCE(pi.indisprimary, false) AS is_primary,
  (con.oid IS NOT NULL) AS constraint_backed
FROM pg_stat_user_indexes si
JOIN pg_index pi ON pi.indexrelid = si.indexrelid
LEFT JOIN pg_constraint con ON con.conindid = si.indexrelid
WHERE si.relname IN ('usage_logs', 'ops_system_logs', 'ops_error_logs')
ORDER BY CASE si.relname
  WHEN 'usage_logs' THEN 1
  WHEN 'ops_system_logs' THEN 2
  WHEN 'ops_error_logs' THEN 3
  ELSE 99
END, index_bytes DESC, si.indexrelname ASC`

	rows, err := c.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]opsStorageGovernanceIndexStat, 0, 16)
	for rows.Next() {
		var item opsStorageGovernanceIndexStat
		if err := rows.Scan(
			&item.Table,
			&item.Name,
			&item.Bytes,
			&item.ScanCount,
			&item.Primary,
			&item.ConstraintBacked,
		); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func classifyStorageSizeRisk(table string, totalBytes int64) string {
	switch table {
	case "usage_logs":
		return classifyThresholdRisk(totalBytes, opsStorageUsageLogsWarnBytes, opsStorageUsageLogsCriticalBytes)
	case "ops_system_logs":
		return classifyThresholdRisk(totalBytes, opsStorageSystemLogsWarnBytes, opsStorageSystemLogsCriticalBytes)
	case "ops_error_logs":
		return classifyThresholdRisk(totalBytes, opsStorageErrorLogsWarnBytes, opsStorageErrorLogsCriticalBytes)
	default:
		return "none"
	}
}

func classifyColdIndexRisk(count int, totalBytes int64) string {
	if count <= 0 || totalBytes <= 0 {
		return "none"
	}
	if totalBytes >= opsStorageColdIndexCriticalBytes || count >= 10 {
		return "critical"
	}
	if totalBytes >= opsStorageColdIndexWarnBytes || count >= 4 {
		return "warn"
	}
	return "none"
}

func classifyRetentionRisk(cfg *config.Config, table opsStorageGovernanceTableSnapshot) (string, []string) {
	reasons := make([]string, 0, 3)
	switch table.Table {
	case "usage_logs":
		if cfg == nil || !cfg.DashboardAgg.Enabled {
			reasons = append(reasons, "dashboard_aggregation_disabled")
			return "critical", reasons
		}
		if cfg.DashboardAgg.Retention.UsageLogsDays <= 0 {
			reasons = append(reasons, "usage_logs_retention_disabled")
			return "critical", reasons
		}
		if !table.Partitioned && table.TotalBytes >= opsStorageUsageLogsWarnBytes {
			reasons = append(reasons, "non_partitioned_delete_retention")
			return "warn", reasons
		}
		if table.MaxRowsEnabled && table.MaxRowsExceeded {
			reasons = append(reasons, "usage_logs_max_rows_exceeded")
			return "warn", reasons
		}
	case "ops_system_logs", "ops_error_logs":
		if cfg == nil || cfg.Ops.Cleanup.ErrorLogRetentionDays <= 0 {
			reasons = append(reasons, "ops_cleanup_retention_disabled")
			return "critical", reasons
		}
		if !table.Partitioned && table.TotalBytes >= retentionDeleteWarnBytes(table.Table) {
			reasons = append(reasons, "batch_delete_retention_on_large_table")
			return "warn", reasons
		}
		if table.MaxRowsEnabled && table.MaxRowsExceeded {
			reasons = append(reasons, table.Table+"_max_rows_exceeded")
			return "warn", reasons
		}
	}
	return "none", reasons
}

func retentionDeleteWarnBytes(table string) int64 {
	switch table {
	case "ops_system_logs":
		return opsStorageSystemLogsWarnBytes
	case "ops_error_logs":
		return opsStorageErrorLogsWarnBytes
	default:
		return opsStorageUsageLogsWarnBytes
	}
}

func classifyThresholdRisk(value int64, warn int64, critical int64) string {
	if critical > 0 && value >= critical {
		return "critical"
	}
	if warn > 0 && value >= warn {
		return "warn"
	}
	return "none"
}

func hasStorageRiskLevel(level string) bool {
	return isWarnStorageRisk(level) || isCriticalStorageRisk(level)
}

func isWarnStorageRisk(level string) bool {
	return strings.EqualFold(strings.TrimSpace(level), "warn")
}

func isCriticalStorageRisk(level string) bool {
	return strings.EqualFold(strings.TrimSpace(level), "critical")
}

func (c *OpsMetricsCollector) snapshotRedisPoolRuntimeStats() opsRedisPoolRuntimeStats {
	if c == nil {
		return opsRedisPoolRuntimeStats{}
	}
	if c.redisPoolStatsFn != nil {
		return c.redisPoolStatsFn()
	}
	if c.redisClient == nil {
		return opsRedisPoolRuntimeStats{}
	}
	stats := c.redisClient.PoolStats()
	if stats == nil {
		return opsRedisPoolRuntimeStats{}
	}
	return opsRedisPoolRuntimeStats{
		Stalls:   int64(stats.WaitCount),
		Timeouts: int64(stats.Timeouts),
	}
}

type opsRedisPoolRuntimeStats struct {
	Stalls   int64
	Timeouts int64
}

func marshalOpsRuntimeExtra(extra map[string]any) (string, error) {
	if len(extra) == 0 {
		return "{}", nil
	}
	data, err := json.Marshal(extra)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func clampNonNegativeDelta(current, prev int64) int64 {
	if current <= prev {
		return 0
	}
	return current - prev
}

func clampNonNegativeDeltaUint(current, prev uint64) uint64 {
	if current <= prev {
		return 0
	}
	return current - prev
}

func (c *OpsMetricsCollector) collectConcurrencyQueueDepth(parentCtx context.Context) *int {
	if c == nil || c.accountRepo == nil || c.concurrencyService == nil {
		return nil
	}
	if parentCtx == nil {
		parentCtx = context.Background()
	}

	// Best-effort: never let concurrency sampling break the metrics collector.
	ctx, cancel := context.WithTimeout(parentCtx, 2*time.Second)
	defer cancel()

	accounts, err := c.accountRepo.ListSchedulable(ctx)
	if err != nil {
		return nil
	}
	if len(accounts) == 0 {
		zero := 0
		return &zero
	}

	batch := make([]AccountWithConcurrency, 0, len(accounts))
	for _, acc := range accounts {
		if acc.ID <= 0 {
			continue
		}
		batch = append(batch, AccountWithConcurrency{
			ID:             acc.ID,
			MaxConcurrency: acc.Concurrency,
		})
	}
	if len(batch) == 0 {
		zero := 0
		return &zero
	}

	loadMap, err := c.concurrencyService.GetAccountsLoadBatch(ctx, batch)
	if err != nil {
		return nil
	}

	var total int64
	for _, info := range loadMap {
		if info == nil || info.WaitingCount <= 0 {
			continue
		}
		total += int64(info.WaitingCount)
	}
	if total < 0 {
		total = 0
	}

	maxInt := int64(^uint(0) >> 1)
	if total > maxInt {
		total = maxInt
	}
	v := int(total)
	return &v
}

type opsCollectedPercentiles struct {
	p50 *int
	p90 *int
	p95 *int
	p99 *int
	avg *float64
	max *int
}

func (c *OpsMetricsCollector) queryUsageCounts(ctx context.Context, start, end time.Time) (successCount int64, tokenConsumed int64, err error) {
	if c == nil || c.db == nil {
		return 0, 0, nil
	}
	q := `
SELECT
  COALESCE(COUNT(*), 0) AS success_count,
  COALESCE(SUM(input_tokens + output_tokens + cache_creation_tokens + cache_read_tokens), 0) AS token_consumed
FROM usage_logs
WHERE created_at >= $1 AND created_at < $2`

	var tokens sql.NullInt64
	if err := c.db.QueryRowContext(ctx, q, start, end).Scan(&successCount, &tokens); err != nil {
		return 0, 0, err
	}
	if tokens.Valid {
		tokenConsumed = tokens.Int64
	}
	return successCount, tokenConsumed, nil
}

func (c *OpsMetricsCollector) queryUsageLatency(ctx context.Context, start, end time.Time) (duration opsCollectedPercentiles, ttft opsCollectedPercentiles, err error) {
	if c == nil || c.db == nil {
		return opsCollectedPercentiles{}, opsCollectedPercentiles{}, nil
	}
	{
		q := `
SELECT
  percentile_cont(0.50) WITHIN GROUP (ORDER BY duration_ms) AS p50,
  percentile_cont(0.90) WITHIN GROUP (ORDER BY duration_ms) AS p90,
  percentile_cont(0.95) WITHIN GROUP (ORDER BY duration_ms) AS p95,
  percentile_cont(0.99) WITHIN GROUP (ORDER BY duration_ms) AS p99,
  AVG(duration_ms) AS avg_ms,
  MAX(duration_ms) AS max_ms
FROM usage_logs
WHERE created_at >= $1 AND created_at < $2
  AND duration_ms IS NOT NULL`

		var p50, p90, p95, p99 sql.NullFloat64
		var avg sql.NullFloat64
		var max sql.NullInt64
		if err := c.db.QueryRowContext(ctx, q, start, end).Scan(&p50, &p90, &p95, &p99, &avg, &max); err != nil {
			return opsCollectedPercentiles{}, opsCollectedPercentiles{}, err
		}
		duration.p50 = floatToIntPtr(p50)
		duration.p90 = floatToIntPtr(p90)
		duration.p95 = floatToIntPtr(p95)
		duration.p99 = floatToIntPtr(p99)
		if avg.Valid {
			v := roundTo1DP(avg.Float64)
			duration.avg = &v
		}
		if max.Valid {
			v := int(max.Int64)
			duration.max = &v
		}
	}

	{
		q := `
SELECT
  percentile_cont(0.50) WITHIN GROUP (ORDER BY first_token_ms) AS p50,
  percentile_cont(0.90) WITHIN GROUP (ORDER BY first_token_ms) AS p90,
  percentile_cont(0.95) WITHIN GROUP (ORDER BY first_token_ms) AS p95,
  percentile_cont(0.99) WITHIN GROUP (ORDER BY first_token_ms) AS p99,
  AVG(first_token_ms) AS avg_ms,
  MAX(first_token_ms) AS max_ms
FROM usage_logs
WHERE created_at >= $1 AND created_at < $2
  AND first_token_ms IS NOT NULL`

		var p50, p90, p95, p99 sql.NullFloat64
		var avg sql.NullFloat64
		var max sql.NullInt64
		if err := c.db.QueryRowContext(ctx, q, start, end).Scan(&p50, &p90, &p95, &p99, &avg, &max); err != nil {
			return opsCollectedPercentiles{}, opsCollectedPercentiles{}, err
		}
		ttft.p50 = floatToIntPtr(p50)
		ttft.p90 = floatToIntPtr(p90)
		ttft.p95 = floatToIntPtr(p95)
		ttft.p99 = floatToIntPtr(p99)
		if avg.Valid {
			v := roundTo1DP(avg.Float64)
			ttft.avg = &v
		}
		if max.Valid {
			v := int(max.Int64)
			ttft.max = &v
		}
	}

	return duration, ttft, nil
}

func (c *OpsMetricsCollector) queryErrorCounts(ctx context.Context, start, end time.Time) (
	errorTotal int64,
	businessLimited int64,
	errorSLA int64,
	upstreamExcl429529 int64,
	upstream429 int64,
	upstream529 int64,
	err error,
) {
	if c == nil || c.db == nil {
		return 0, 0, 0, 0, 0, 0, nil
	}
	q := `
SELECT
  COALESCE(COUNT(*) FILTER (WHERE COALESCE(status_code, 0) >= 400), 0) AS error_total,
  COALESCE(COUNT(*) FILTER (WHERE COALESCE(status_code, 0) >= 400 AND is_business_limited), 0) AS business_limited,
  COALESCE(COUNT(*) FILTER (WHERE COALESCE(status_code, 0) >= 400 AND NOT is_business_limited), 0) AS error_sla,
  COALESCE(COUNT(*) FILTER (WHERE error_owner = 'provider' AND NOT is_business_limited AND COALESCE(upstream_status_code, status_code, 0) NOT IN (429, 529)), 0) AS upstream_excl,
  COALESCE(COUNT(*) FILTER (WHERE error_owner = 'provider' AND NOT is_business_limited AND COALESCE(upstream_status_code, status_code, 0) = 429), 0) AS upstream_429,
  COALESCE(COUNT(*) FILTER (WHERE error_owner = 'provider' AND NOT is_business_limited AND COALESCE(upstream_status_code, status_code, 0) = 529), 0) AS upstream_529
FROM ops_error_logs
WHERE created_at >= $1 AND created_at < $2`

	if err := c.db.QueryRowContext(ctx, q, start, end).Scan(
		&errorTotal,
		&businessLimited,
		&errorSLA,
		&upstreamExcl429529,
		&upstream429,
		&upstream529,
	); err != nil {
		return 0, 0, 0, 0, 0, 0, err
	}
	return errorTotal, businessLimited, errorSLA, upstreamExcl429529, upstream429, upstream529, nil
}

func (c *OpsMetricsCollector) queryAccountSwitchCount(ctx context.Context, start, end time.Time) (int64, error) {
	if c == nil || c.db == nil {
		return 0, nil
	}
	q := `
SELECT
  COALESCE(SUM(CASE
    WHEN split_part(ev->>'kind', ':', 1) IN ('failover', 'retry_exhausted_failover', 'failover_on_400') THEN 1
    ELSE 0
  END), 0) AS switch_count
FROM ops_error_logs o
CROSS JOIN LATERAL jsonb_array_elements(
  COALESCE(NULLIF(o.upstream_errors, 'null'::jsonb), '[]'::jsonb)
) AS ev
WHERE o.created_at >= $1 AND o.created_at < $2
  AND o.is_count_tokens = FALSE`

	var count int64
	if err := c.db.QueryRowContext(ctx, q, start, end).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

type opsCollectedSystemStats struct {
	cpuUsagePercent    *float64
	memoryUsedMB       *int64
	memoryTotalMB      *int64
	memoryUsagePercent *float64
}

func (c *OpsMetricsCollector) collectSystemStats(ctx context.Context) (*opsCollectedSystemStats, error) {
	out := &opsCollectedSystemStats{}
	if ctx == nil {
		ctx = context.Background()
	}

	sampleAt := time.Now().UTC()

	// Prefer cgroup (container) metrics when available.
	if cpuPct := c.tryCgroupCPUPercent(sampleAt); cpuPct != nil {
		out.cpuUsagePercent = cpuPct
	}

	cgroupUsed, cgroupTotal, cgroupOK := readCgroupMemoryBytes()
	if cgroupOK {
		usedMB := int64(cgroupUsed / bytesPerMB)
		out.memoryUsedMB = &usedMB
		if cgroupTotal > 0 {
			totalMB := int64(cgroupTotal / bytesPerMB)
			out.memoryTotalMB = &totalMB
			pct := roundTo1DP(float64(cgroupUsed) / float64(cgroupTotal) * 100)
			out.memoryUsagePercent = &pct
		}
	}

	// Fallback to host metrics if cgroup metrics are unavailable (or incomplete).
	if out.cpuUsagePercent == nil {
		if cpuPercents, err := cpu.PercentWithContext(ctx, 0, false); err == nil && len(cpuPercents) > 0 {
			v := roundTo1DP(cpuPercents[0])
			out.cpuUsagePercent = &v
		}
	}

	// If total memory isn't available from cgroup (e.g. memory.max = "max"), fill total from host.
	if out.memoryUsedMB == nil || out.memoryTotalMB == nil || out.memoryUsagePercent == nil {
		if vm, err := mem.VirtualMemoryWithContext(ctx); err == nil && vm != nil {
			if out.memoryUsedMB == nil {
				usedMB := int64(vm.Used / bytesPerMB)
				out.memoryUsedMB = &usedMB
			}
			if out.memoryTotalMB == nil {
				totalMB := int64(vm.Total / bytesPerMB)
				out.memoryTotalMB = &totalMB
			}
			if out.memoryUsagePercent == nil {
				if out.memoryUsedMB != nil && out.memoryTotalMB != nil && *out.memoryTotalMB > 0 {
					pct := roundTo1DP(float64(*out.memoryUsedMB) / float64(*out.memoryTotalMB) * 100)
					out.memoryUsagePercent = &pct
				} else {
					pct := roundTo1DP(vm.UsedPercent)
					out.memoryUsagePercent = &pct
				}
			}
		}
	}

	return out, nil
}

func (c *OpsMetricsCollector) tryCgroupCPUPercent(now time.Time) *float64 {
	usageNanos, ok := readCgroupCPUUsageNanos()
	if !ok {
		return nil
	}

	// Initialize baseline sample.
	if c.lastCgroupCPUSampleAt.IsZero() {
		c.lastCgroupCPUUsageNanos = usageNanos
		c.lastCgroupCPUSampleAt = now
		return nil
	}

	elapsed := now.Sub(c.lastCgroupCPUSampleAt)
	if elapsed <= 0 {
		c.lastCgroupCPUUsageNanos = usageNanos
		c.lastCgroupCPUSampleAt = now
		return nil
	}

	prev := c.lastCgroupCPUUsageNanos
	c.lastCgroupCPUUsageNanos = usageNanos
	c.lastCgroupCPUSampleAt = now

	if usageNanos < prev {
		// Counter reset (container restarted).
		return nil
	}

	deltaUsageSec := float64(usageNanos-prev) / 1e9
	elapsedSec := elapsed.Seconds()
	if elapsedSec <= 0 {
		return nil
	}

	cores := readCgroupCPULimitCores()
	if cores <= 0 {
		// Can't reliably normalize; skip and fall back to gopsutil.
		return nil
	}

	pct := (deltaUsageSec / (elapsedSec * cores)) * 100
	if pct < 0 {
		pct = 0
	}
	// Clamp to avoid noise/jitter showing impossible values.
	if pct > 100 {
		pct = 100
	}
	v := roundTo1DP(pct)
	return &v
}

func readCgroupMemoryBytes() (usedBytes uint64, totalBytes uint64, ok bool) {
	// cgroup v2 (most common in modern containers)
	if used, ok1 := readUintFile("/sys/fs/cgroup/memory.current"); ok1 {
		usedBytes = used
		rawMax, err := os.ReadFile("/sys/fs/cgroup/memory.max")
		if err == nil {
			s := strings.TrimSpace(string(rawMax))
			if s != "" && s != "max" {
				if v, err := strconv.ParseUint(s, 10, 64); err == nil {
					totalBytes = v
				}
			}
		}
		return usedBytes, totalBytes, true
	}

	// cgroup v1 fallback
	if used, ok1 := readUintFile("/sys/fs/cgroup/memory/memory.usage_in_bytes"); ok1 {
		usedBytes = used
		if limit, ok2 := readUintFile("/sys/fs/cgroup/memory/memory.limit_in_bytes"); ok2 {
			// Some environments report a very large number when unlimited.
			if limit > 0 && limit < (1<<60) {
				totalBytes = limit
			}
		}
		return usedBytes, totalBytes, true
	}

	return 0, 0, false
}

func readCgroupCPUUsageNanos() (usageNanos uint64, ok bool) {
	// cgroup v2: cpu.stat has usage_usec
	if raw, err := os.ReadFile("/sys/fs/cgroup/cpu.stat"); err == nil {
		lines := strings.Split(string(raw), "\n")
		for _, line := range lines {
			fields := strings.Fields(line)
			if len(fields) != 2 {
				continue
			}
			if fields[0] != "usage_usec" {
				continue
			}
			v, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				continue
			}
			return v * 1000, true
		}
	}

	// cgroup v1: cpuacct.usage is in nanoseconds
	if v, ok := readUintFile("/sys/fs/cgroup/cpuacct/cpuacct.usage"); ok {
		return v, true
	}

	return 0, false
}

func readCgroupCPULimitCores() float64 {
	// cgroup v2: cpu.max => "<quota> <period>" or "max <period>"
	if raw, err := os.ReadFile("/sys/fs/cgroup/cpu.max"); err == nil {
		fields := strings.Fields(string(raw))
		if len(fields) >= 2 && fields[0] != "max" {
			quota, err1 := strconv.ParseFloat(fields[0], 64)
			period, err2 := strconv.ParseFloat(fields[1], 64)
			if err1 == nil && err2 == nil && quota > 0 && period > 0 {
				return quota / period
			}
		}
	}

	// cgroup v1: cpu.cfs_quota_us / cpu.cfs_period_us
	quota, okQuota := readIntFile("/sys/fs/cgroup/cpu/cpu.cfs_quota_us")
	period, okPeriod := readIntFile("/sys/fs/cgroup/cpu/cpu.cfs_period_us")
	if okQuota && okPeriod && quota > 0 && period > 0 {
		return float64(quota) / float64(period)
	}

	return 0
}

func readUintFile(path string) (uint64, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	s := strings.TrimSpace(string(raw))
	if s == "" {
		return 0, false
	}
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

func readIntFile(path string) (int64, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	s := strings.TrimSpace(string(raw))
	if s == "" {
		return 0, false
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

func (c *OpsMetricsCollector) checkDB(ctx context.Context) bool {
	if c == nil || c.db == nil {
		return false
	}
	if ctx == nil {
		ctx = context.Background()
	}
	var one int
	if err := c.db.QueryRowContext(ctx, "SELECT 1").Scan(&one); err != nil {
		return false
	}
	return one == 1
}

func (c *OpsMetricsCollector) checkRedis(ctx context.Context) bool {
	if c == nil || c.redisClient == nil {
		return false
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return c.redisClient.Ping(ctx).Err() == nil
}

func (c *OpsMetricsCollector) redisPoolStats() (total int, idle int, ok bool) {
	if c == nil || c.redisClient == nil {
		return 0, 0, false
	}
	stats := c.redisClient.PoolStats()
	if stats == nil {
		return 0, 0, false
	}
	return int(stats.TotalConns), int(stats.IdleConns), true
}

func (c *OpsMetricsCollector) dbPoolStats() (active int, idle int, waitCount uint64, waitDuration time.Duration) {
	if c == nil {
		return 0, 0, 0, 0
	}
	if c.dbPoolStatsFn != nil {
		return c.dbPoolStatsFn()
	}
	if c.db == nil {
		return 0, 0, 0, 0
	}
	stats := c.db.Stats()
	waitCount = 0
	if stats.WaitCount > 0 {
		waitCount = uint64(stats.WaitCount)
	}
	return stats.InUse, stats.Idle, waitCount, stats.WaitDuration
}

var opsMetricsCollectorReleaseScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("DEL", KEYS[1])
end
return 0
`)

func (c *OpsMetricsCollector) tryAcquireLeaderLock(ctx context.Context) (func(), bool) {
	if c == nil || c.redisClient == nil {
		return nil, true
	}
	if ctx == nil {
		ctx = context.Background()
	}

	ok, err := c.redisClient.SetNX(ctx, opsMetricsCollectorLeaderLockKey, c.instanceID, opsMetricsCollectorLeaderLockTTL).Result()
	if err != nil {
		// Prefer fail-closed to avoid stampeding the database when Redis is flaky.
		// Fallback to a DB advisory lock when Redis is present but unavailable.
		release, ok := tryAcquireDBAdvisoryLock(ctx, c.db, opsMetricsCollectorAdvisoryLockID)
		if !ok {
			c.maybeLogSkip()
			return nil, false
		}
		return release, true
	}
	if !ok {
		c.maybeLogSkip()
		return nil, false
	}

	release := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, _ = opsMetricsCollectorReleaseScript.Run(ctx, c.redisClient, []string{opsMetricsCollectorLeaderLockKey}, c.instanceID).Result()
	}
	return release, true
}

func (c *OpsMetricsCollector) maybeLogSkip() {
	c.skipLogMu.Lock()
	defer c.skipLogMu.Unlock()

	now := time.Now()
	if !c.skipLogAt.IsZero() && now.Sub(c.skipLogAt) < time.Minute {
		return
	}
	c.skipLogAt = now
	log.Printf("[OpsMetricsCollector] leader lock held by another instance; skipping")
}

func floatToIntPtr(v sql.NullFloat64) *int {
	if !v.Valid {
		return nil
	}
	n := int(math.Round(v.Float64))
	return &n
}

func roundTo1DP(v float64) float64 {
	return math.Round(v*10) / 10
}

func truncateString(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if len(s) <= max {
		return s
	}
	cut := s[:max]
	for len(cut) > 0 && !utf8.ValidString(cut) {
		cut = cut[:len(cut)-1]
	}
	return cut
}

func boolPtr(v bool) *bool {
	out := v
	return &out
}

func intPtr(v int) *int {
	out := v
	return &out
}

func float64Ptr(v float64) *float64 {
	out := v
	return &out
}
