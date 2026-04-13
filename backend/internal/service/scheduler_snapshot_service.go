package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

var (
	ErrSchedulerCacheNotReady   = errors.New("scheduler cache not ready")
	ErrSchedulerFallbackLimited = errors.New("scheduler db fallback limited")
	errOutboxPoisonEvent        = errors.New("scheduler outbox poison event")
	errOutboxRetryable          = errors.New("scheduler outbox retryable error")
	schedulerBucketLockOwnerSeq atomic.Uint64
)

// SchedulerOutboxCheckpointRepository persists watermark checkpoints for resilience.
type SchedulerOutboxCheckpointRepository interface {
	GetCheckpointWatermark(ctx context.Context) (int64, error)
	SetCheckpointWatermark(ctx context.Context, watermark int64) error
}

type SchedulerOutboxRuntimeEvent struct {
	Timestamp          time.Time `json:"timestamp"`
	ID                 int64     `json:"id"`
	EventType          string    `json:"event_type"`
	Reason             string    `json:"reason"`
	Error              string    `json:"error"`
	PayloadDecodeError string    `json:"payload_decode_error,omitempty"`
}

type SchedulerOutboxBlockedEvent struct {
	Timestamp          time.Time `json:"timestamp"`
	FirstSeenAt        string    `json:"first_seen_at,omitempty"`
	LastSeenAt         string    `json:"last_seen_at,omitempty"`
	AgeSeconds         int64     `json:"age_seconds,omitempty"`
	ID                 int64     `json:"id"`
	EventType          string    `json:"event_type"`
	Reason             string    `json:"reason"`
	Attempts           int       `json:"attempts"`
	NextAttemptAt      string    `json:"next_attempt_at,omitempty"`
	Error              string    `json:"error,omitempty"`
	PayloadDecodeError string    `json:"payload_decode_error,omitempty"`
}

type SchedulerOutboxRuntimeMetrics struct {
	PoisonTotal                  int64                        `json:"poison_total"`
	TransientTotal               int64                        `json:"transient_total"`
	PayloadDecodePoisonTotal     int64                        `json:"payload_decode_poison_total"`
	MalformedPayloadPoisonTotal  int64                        `json:"malformed_payload_poison_total"`
	UnknownEventPoisonTotal      int64                        `json:"unknown_event_poison_total"`
	LockContentionTransientTotal int64                        `json:"lock_contention_transient_total"`
	DBTransientTotal             int64                        `json:"db_transient_total"`
	CacheTransientTotal          int64                        `json:"cache_transient_total"`
	OtherTransientTotal          int64                        `json:"other_transient_total"`
	CoalescedBatchTotal          int64                        `json:"coalesced_batch_total"`
	CoalescedEventSavedTotal     int64                        `json:"coalesced_event_saved_total"`
	CheckpointFallbackTotal      int64                        `json:"checkpoint_fallback_total"`
	CheckpointFallbackStreak     int64                        `json:"checkpoint_fallback_streak"`
	CheckpointReadFailureTotal   int64                        `json:"checkpoint_read_failure_total"`
	CheckpointWriteFailureTotal  int64                        `json:"checkpoint_write_failure_total"`
	CheckpointLastFallbackAt     string                       `json:"checkpoint_last_fallback_at,omitempty"`
	CheckpointLastFallbackReason string                       `json:"checkpoint_last_fallback_reason,omitempty"`
	CheckpointLastReadFailureAt  string                       `json:"checkpoint_last_read_failure_at,omitempty"`
	CheckpointLastWriteFailureAt string                       `json:"checkpoint_last_write_failure_at,omitempty"`
	LastRedisWatermark           int64                        `json:"last_redis_watermark"`
	LastCheckpointWatermark      int64                        `json:"last_checkpoint_watermark"`
	WatermarkDrift               int64                        `json:"watermark_drift"`
	BacklogRows                  int64                        `json:"backlog_rows"`
	LagSeconds                   int64                        `json:"lag_seconds"`
	LagFailureStreak             int64                        `json:"lag_failure_streak"`
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
	LastPoison                   *SchedulerOutboxRuntimeEvent `json:"last_poison,omitempty"`
	LastTransient                *SchedulerOutboxRuntimeEvent `json:"last_transient,omitempty"`
	BlockedEventSummary          string                       `json:"blocked_event_summary,omitempty"`
	CheckpointFallbackSummary    string                       `json:"checkpoint_fallback_summary,omitempty"`
	LagStreakSummary             string                       `json:"lag_streak_summary,omitempty"`
	RebuildContentionSummary     string                       `json:"rebuild_contention_summary,omitempty"`
	DriftTrendStatus             string                       `json:"drift_trend_status,omitempty"`
	DriftTrendDetail             string                       `json:"drift_trend_detail,omitempty"`
	DriftTrendNarrative          string                       `json:"drift_trend_narrative,omitempty"`
}

var (
	schedulerOutboxPoisonTotal                  atomic.Int64
	schedulerOutboxTransientTotal               atomic.Int64
	schedulerOutboxPayloadDecodePoisonTotal     atomic.Int64
	schedulerOutboxMalformedPayloadPoisonTotal  atomic.Int64
	schedulerOutboxUnknownEventPoisonTotal      atomic.Int64
	schedulerOutboxLockContentionTransientTotal atomic.Int64
	schedulerOutboxDBTransientTotal             atomic.Int64
	schedulerOutboxCacheTransientTotal          atomic.Int64
	schedulerOutboxOtherTransientTotal          atomic.Int64
	schedulerOutboxCoalescedBatchTotal          atomic.Int64
	schedulerOutboxCoalescedEventSavedTotal     atomic.Int64
	schedulerOutboxCheckpointFallbackTotal      atomic.Int64
	schedulerOutboxCheckpointFallbackStreak     atomic.Int64
	schedulerOutboxCheckpointReadFailureTotal   atomic.Int64
	schedulerOutboxCheckpointWriteFailureTotal  atomic.Int64
	schedulerOutboxLastRedisWatermark           atomic.Int64
	schedulerOutboxLastCheckpointWatermark      atomic.Int64
	schedulerOutboxWatermarkDrift               atomic.Int64
	schedulerOutboxBacklogRows                  atomic.Int64
	schedulerOutboxLagSeconds                   atomic.Int64
	schedulerOutboxLagFailureStreak             atomic.Int64
	schedulerOutboxLagRebuildTotal              atomic.Int64
	schedulerOutboxBacklogRebuildTotal          atomic.Int64
	schedulerOutboxRebuildCooldownSkipTotal     atomic.Int64
	schedulerOutboxBlockedEventClearTotal       atomic.Int64
	schedulerOutboxBucketRebuildSuccessTotal    atomic.Int64
	schedulerOutboxBucketRebuildFailureTotal    atomic.Int64
	schedulerOutboxBucketRebuildLockContention  atomic.Int64
	schedulerOutboxBusyBucketSkipTotal          atomic.Int64
	schedulerOutboxRuntimeMu                    sync.Mutex
	schedulerOutboxBlockedEvent                 *SchedulerOutboxBlockedEvent
	schedulerOutboxLastPoison                   *SchedulerOutboxRuntimeEvent
	schedulerOutboxLastTransient                *SchedulerOutboxRuntimeEvent
	schedulerOutboxCheckpointLastFallbackAt     string
	schedulerOutboxCheckpointLastFallbackReason string
	schedulerOutboxCheckpointLastReadFailureAt  string
	schedulerOutboxCheckpointLastWriteFailureAt string
	schedulerOutboxLastLagRebuildAt             string
	schedulerOutboxLastBacklogRebuildAt         string
	schedulerOutboxBlockedEventLastClearedAt    string
	schedulerOutboxBlockedEventLastClearReason  string
	schedulerOutboxBlockedEventLastClearedID    int64
	schedulerOutboxLastBucketRebuildAt          string
	schedulerOutboxLastBucketRebuildReason      string
	schedulerOutboxLastBucketRebuildStatus      string
	schedulerOutboxLastBucketRebuildBucket      string
)

const (
	fallbackCooldownDuration              = 30 * time.Second
	schedulerBucketLockTTL                = 30 * time.Second
	outboxRetryBaseBackoff                = 2 * time.Second
	outboxRetryLockContentionBackoff      = 5 * time.Second
	outboxRetryMaxBackoff                 = 30 * time.Second
	defaultSchedulerOutboxPollTimeout     = 10 * time.Second
	defaultSchedulerOutboxCommitTimeout   = 5 * time.Second
	defaultSchedulerOutboxLagCheckTimeout = 10 * time.Second
	defaultSchedulerFullRebuildTimeout    = 2 * time.Minute
)

type schedulerOutboxRetryState struct {
	NextAttemptAt time.Time
	Attempts      int
	LastReason    string
	FirstSeenAt   time.Time
	LastSeenAt    time.Time
	LastError     string
}

func markOutboxPoison(err error) error {
	if err == nil {
		return errOutboxPoisonEvent
	}
	if errors.Is(err, errOutboxPoisonEvent) {
		return err
	}
	return fmt.Errorf("%w: %v", errOutboxPoisonEvent, err)
}

func wrapOutboxRetryable(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, errOutboxRetryable) || errors.Is(err, errOutboxPoisonEvent) {
		return err
	}
	return fmt.Errorf("%w: %v", errOutboxRetryable, err)
}

func SnapshotSchedulerOutboxRuntimeMetrics() SchedulerOutboxRuntimeMetrics {
	schedulerOutboxRuntimeMu.Lock()
	defer schedulerOutboxRuntimeMu.Unlock()

	snapshot := SchedulerOutboxRuntimeMetrics{
		PoisonTotal:                  schedulerOutboxPoisonTotal.Load(),
		TransientTotal:               schedulerOutboxTransientTotal.Load(),
		PayloadDecodePoisonTotal:     schedulerOutboxPayloadDecodePoisonTotal.Load(),
		MalformedPayloadPoisonTotal:  schedulerOutboxMalformedPayloadPoisonTotal.Load(),
		UnknownEventPoisonTotal:      schedulerOutboxUnknownEventPoisonTotal.Load(),
		LockContentionTransientTotal: schedulerOutboxLockContentionTransientTotal.Load(),
		DBTransientTotal:             schedulerOutboxDBTransientTotal.Load(),
		CacheTransientTotal:          schedulerOutboxCacheTransientTotal.Load(),
		OtherTransientTotal:          schedulerOutboxOtherTransientTotal.Load(),
		CoalescedBatchTotal:          schedulerOutboxCoalescedBatchTotal.Load(),
		CoalescedEventSavedTotal:     schedulerOutboxCoalescedEventSavedTotal.Load(),
		CheckpointFallbackTotal:      schedulerOutboxCheckpointFallbackTotal.Load(),
		CheckpointFallbackStreak:     schedulerOutboxCheckpointFallbackStreak.Load(),
		CheckpointReadFailureTotal:   schedulerOutboxCheckpointReadFailureTotal.Load(),
		CheckpointWriteFailureTotal:  schedulerOutboxCheckpointWriteFailureTotal.Load(),
		LastRedisWatermark:           schedulerOutboxLastRedisWatermark.Load(),
		LastCheckpointWatermark:      schedulerOutboxLastCheckpointWatermark.Load(),
		WatermarkDrift:               schedulerOutboxWatermarkDrift.Load(),
		BacklogRows:                  schedulerOutboxBacklogRows.Load(),
		LagSeconds:                   schedulerOutboxLagSeconds.Load(),
		LagFailureStreak:             schedulerOutboxLagFailureStreak.Load(),
		LagRebuildTotal:              schedulerOutboxLagRebuildTotal.Load(),
		BacklogRebuildTotal:          schedulerOutboxBacklogRebuildTotal.Load(),
		RebuildCooldownSkipTotal:     schedulerOutboxRebuildCooldownSkipTotal.Load(),
		BlockedEventClearTotal:       schedulerOutboxBlockedEventClearTotal.Load(),
		BucketRebuildSuccessTotal:    schedulerOutboxBucketRebuildSuccessTotal.Load(),
		BucketRebuildFailureTotal:    schedulerOutboxBucketRebuildFailureTotal.Load(),
		BucketRebuildLockContention:  schedulerOutboxBucketRebuildLockContention.Load(),
		BusyBucketSkipTotal:          schedulerOutboxBusyBucketSkipTotal.Load(),
	}
	snapshot.CheckpointLastFallbackAt = schedulerOutboxCheckpointLastFallbackAt
	snapshot.CheckpointLastFallbackReason = schedulerOutboxCheckpointLastFallbackReason
	snapshot.CheckpointLastReadFailureAt = schedulerOutboxCheckpointLastReadFailureAt
	snapshot.CheckpointLastWriteFailureAt = schedulerOutboxCheckpointLastWriteFailureAt
	snapshot.LastLagRebuildAt = schedulerOutboxLastLagRebuildAt
	snapshot.LastBacklogRebuildAt = schedulerOutboxLastBacklogRebuildAt
	snapshot.BlockedEventLastClearedAt = schedulerOutboxBlockedEventLastClearedAt
	snapshot.BlockedEventLastClearReason = schedulerOutboxBlockedEventLastClearReason
	snapshot.BlockedEventLastClearedID = schedulerOutboxBlockedEventLastClearedID
	snapshot.LastBucketRebuildAt = schedulerOutboxLastBucketRebuildAt
	snapshot.LastBucketRebuildReason = schedulerOutboxLastBucketRebuildReason
	snapshot.LastBucketRebuildStatus = schedulerOutboxLastBucketRebuildStatus
	snapshot.LastBucketRebuildBucket = schedulerOutboxLastBucketRebuildBucket
	if schedulerOutboxBlockedEvent != nil {
		last := *schedulerOutboxBlockedEvent
		snapshot.BlockedEvent = &last
	}
	if schedulerOutboxLastPoison != nil {
		last := *schedulerOutboxLastPoison
		snapshot.LastPoison = &last
	}
	if schedulerOutboxLastTransient != nil {
		last := *schedulerOutboxLastTransient
		snapshot.LastTransient = &last
	}
	snapshot.BlockedEventSummary = buildBlockedEventSummary(&snapshot)
	snapshot.CheckpointFallbackSummary = buildCheckpointFallbackSummary(&snapshot)
	snapshot.LagStreakSummary = buildLagStreakSummary(&snapshot)
	snapshot.RebuildContentionSummary = buildRebuildContentionSummary(&snapshot)
	status, detail := buildDriftTrendSummary(&snapshot)
	snapshot.DriftTrendStatus = status
	snapshot.DriftTrendDetail = detail
	snapshot.DriftTrendNarrative = buildDriftNarrative(&snapshot)
	return snapshot
}

func recordSchedulerOutboxPoisonEvent(event SchedulerOutboxEvent, err error) {
	schedulerOutboxPoisonTotal.Add(1)
	reason := classifySchedulerOutboxPoisonReason(event, err)
	switch reason {
	case "payload_decode":
		schedulerOutboxPayloadDecodePoisonTotal.Add(1)
	case "malformed_payload":
		schedulerOutboxMalformedPayloadPoisonTotal.Add(1)
	case "unknown_event":
		schedulerOutboxUnknownEventPoisonTotal.Add(1)
	}
	schedulerOutboxRuntimeMu.Lock()
	defer schedulerOutboxRuntimeMu.Unlock()
	schedulerOutboxLastPoison = &SchedulerOutboxRuntimeEvent{
		Timestamp:          time.Now(),
		ID:                 event.ID,
		EventType:          event.EventType,
		Reason:             reason,
		Error:              errorString(err),
		PayloadDecodeError: event.PayloadDecodeError,
	}
}

func recordSchedulerOutboxTransientEvent(event SchedulerOutboxEvent, err error) {
	schedulerOutboxTransientTotal.Add(1)
	reason := classifySchedulerOutboxTransientReason(err)
	switch reason {
	case "lock_contention":
		schedulerOutboxLockContentionTransientTotal.Add(1)
	case "db":
		schedulerOutboxDBTransientTotal.Add(1)
	case "cache":
		schedulerOutboxCacheTransientTotal.Add(1)
	default:
		schedulerOutboxOtherTransientTotal.Add(1)
	}
	schedulerOutboxRuntimeMu.Lock()
	defer schedulerOutboxRuntimeMu.Unlock()
	schedulerOutboxLastTransient = &SchedulerOutboxRuntimeEvent{
		Timestamp:          time.Now(),
		ID:                 event.ID,
		EventType:          event.EventType,
		Reason:             reason,
		Error:              errorString(err),
		PayloadDecodeError: event.PayloadDecodeError,
	}
}

func resetSchedulerOutboxRuntimeMetricsForTest() {
	schedulerOutboxPoisonTotal.Store(0)
	schedulerOutboxTransientTotal.Store(0)
	schedulerOutboxPayloadDecodePoisonTotal.Store(0)
	schedulerOutboxMalformedPayloadPoisonTotal.Store(0)
	schedulerOutboxUnknownEventPoisonTotal.Store(0)
	schedulerOutboxLockContentionTransientTotal.Store(0)
	schedulerOutboxDBTransientTotal.Store(0)
	schedulerOutboxCacheTransientTotal.Store(0)
	schedulerOutboxOtherTransientTotal.Store(0)
	schedulerOutboxCoalescedBatchTotal.Store(0)
	schedulerOutboxCoalescedEventSavedTotal.Store(0)
	schedulerOutboxCheckpointFallbackTotal.Store(0)
	schedulerOutboxCheckpointFallbackStreak.Store(0)
	schedulerOutboxCheckpointReadFailureTotal.Store(0)
	schedulerOutboxCheckpointWriteFailureTotal.Store(0)
	schedulerOutboxLastRedisWatermark.Store(0)
	schedulerOutboxLastCheckpointWatermark.Store(0)
	schedulerOutboxWatermarkDrift.Store(0)
	schedulerOutboxBacklogRows.Store(0)
	schedulerOutboxLagSeconds.Store(0)
	schedulerOutboxLagFailureStreak.Store(0)
	schedulerOutboxLagRebuildTotal.Store(0)
	schedulerOutboxBacklogRebuildTotal.Store(0)
	schedulerOutboxRebuildCooldownSkipTotal.Store(0)
	schedulerOutboxBlockedEventClearTotal.Store(0)
	schedulerOutboxBucketRebuildSuccessTotal.Store(0)
	schedulerOutboxBucketRebuildFailureTotal.Store(0)
	schedulerOutboxBucketRebuildLockContention.Store(0)
	schedulerOutboxBusyBucketSkipTotal.Store(0)
	schedulerOutboxRuntimeMu.Lock()
	schedulerOutboxBlockedEvent = nil
	schedulerOutboxLastPoison = nil
	schedulerOutboxLastTransient = nil
	schedulerOutboxCheckpointLastFallbackAt = ""
	schedulerOutboxCheckpointLastFallbackReason = ""
	schedulerOutboxCheckpointLastReadFailureAt = ""
	schedulerOutboxCheckpointLastWriteFailureAt = ""
	schedulerOutboxLastLagRebuildAt = ""
	schedulerOutboxLastBacklogRebuildAt = ""
	schedulerOutboxBlockedEventLastClearedAt = ""
	schedulerOutboxBlockedEventLastClearReason = ""
	schedulerOutboxBlockedEventLastClearedID = 0
	schedulerOutboxLastBucketRebuildAt = ""
	schedulerOutboxLastBucketRebuildReason = ""
	schedulerOutboxLastBucketRebuildStatus = ""
	schedulerOutboxLastBucketRebuildBucket = ""
	schedulerOutboxRuntimeMu.Unlock()
}

func classifySchedulerOutboxPoisonReason(event SchedulerOutboxEvent, err error) string {
	if event.PayloadDecodeError != "" {
		return "payload_decode"
	}
	if event.EventType == "" {
		return "unknown_event"
	}
	if err != nil {
		msg := strings.ToLower(err.Error())
		switch {
		case strings.Contains(msg, "unsupported outbox event"), strings.Contains(msg, "unknown outbox event"):
			return "unknown_event"
		case strings.Contains(msg, "malformed"), strings.Contains(msg, "missing"), strings.Contains(msg, "invalid payload"):
			return "malformed_payload"
		}
	}
	return "other_poison"
}

func classifySchedulerOutboxTransientReason(err error) string {
	if err == nil {
		return "other"
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "lock contention"), strings.Contains(msg, "lock busy"):
		return "lock_contention"
	case strings.Contains(msg, "cache"):
		return "cache"
	case strings.Contains(msg, "db"), strings.Contains(msg, "database"), strings.Contains(msg, "query"):
		return "db"
	default:
		return "other"
	}
}

func isSchedulerLockContentionError(err error) bool {
	return classifySchedulerOutboxTransientReason(err) == "lock_contention"
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func buildBlockedEventSummary(metrics *SchedulerOutboxRuntimeMetrics) string {
	if metrics == nil {
		return "no blocked event"
	}
	if metrics.BlockedEvent != nil {
		e := metrics.BlockedEvent
		parts := []string{
			fmt.Sprintf("blocked id=%d", e.ID),
			fmt.Sprintf("reason=%s", nonEmpty(e.Reason, "unknown")),
			fmt.Sprintf("attempts=%d", e.Attempts),
			fmt.Sprintf("age=%ds", e.AgeSeconds),
		}
		if e.NextAttemptAt != "" {
			parts = append(parts, "next="+e.NextAttemptAt)
		}
		return strings.Join(parts, " | ")
	}
	if metrics.BlockedEventLastClearedID != 0 {
		return fmt.Sprintf("cleared id=%d reason=%s", metrics.BlockedEventLastClearedID, nonEmpty(metrics.BlockedEventLastClearReason, "unknown"))
	}
	return "no blocked event"
}

func buildCheckpointFallbackSummary(metrics *SchedulerOutboxRuntimeMetrics) string {
	if metrics == nil {
		return "no checkpoint fallback"
	}
	if metrics.CheckpointFallbackStreak > 0 {
		return fmt.Sprintf("streak=%d reason=%s", metrics.CheckpointFallbackStreak, nonEmpty(metrics.CheckpointLastFallbackReason, "unknown"))
	}
	if metrics.CheckpointFallbackTotal > 0 {
		return fmt.Sprintf("total=%d last=%s", metrics.CheckpointFallbackTotal, nonEmpty(metrics.CheckpointLastFallbackReason, "unknown"))
	}
	return "no checkpoint fallback"
}

func buildLagStreakSummary(metrics *SchedulerOutboxRuntimeMetrics) string {
	if metrics == nil {
		return "no lag streak"
	}
	if metrics.LagFailureStreak > 0 {
		return fmt.Sprintf("streak=%d lag=%ds rebuilds=%d", metrics.LagFailureStreak, metrics.LagSeconds, metrics.LagRebuildTotal)
	}
	if metrics.LagSeconds > 0 {
		return fmt.Sprintf("lag=%ds", metrics.LagSeconds)
	}
	return "no lag streak"
}

func buildRebuildContentionSummary(metrics *SchedulerOutboxRuntimeMetrics) string {
	if metrics == nil {
		return "no rebuild activity"
	}
	total := metrics.BucketRebuildSuccessTotal + metrics.BucketRebuildFailureTotal
	if total == 0 {
		return "no rebuild activity"
	}
	return fmt.Sprintf("success=%d fail=%d contention=%d status=%s reason=%s", metrics.BucketRebuildSuccessTotal, metrics.BucketRebuildFailureTotal, metrics.BucketRebuildLockContention, nonEmpty(metrics.LastBucketRebuildStatus, "unknown"), nonEmpty(metrics.LastBucketRebuildReason, "unknown"))
}

func buildDriftTrendSummary(metrics *SchedulerOutboxRuntimeMetrics) (string, string) {
	if metrics == nil {
		return "stable", "no drift"
	}
	if metrics.LagFailureStreak > 0 || metrics.CheckpointFallbackStreak > 0 {
		return "degrading", "lag or checkpoint fallback persisting"
	}
	if metrics.BacklogRows > 0 || metrics.LagSeconds > 0 {
		return "degrading", fmt.Sprintf("backlog=%d lag=%ds", metrics.BacklogRows, metrics.LagSeconds)
	}
	if metrics.BlockedEvent != nil {
		return "flapping", "blocked event retrying"
	}
	if metrics.WatermarkDrift != 0 {
		return "worsening", fmt.Sprintf("watermark drift=%d", metrics.WatermarkDrift)
	}
	return "stable", "no drift"
}

func buildDriftNarrative(metrics *SchedulerOutboxRuntimeMetrics) string {
	if metrics == nil {
		return "no actionable drift"
	}
	switch {
	case metrics.LagFailureStreak > 0 || metrics.CheckpointFallbackStreak > 0:
		return "lag/checkpoint fallback persisting—stacked rebuild/retry"
	case metrics.BacklogRows > 0 || metrics.LagSeconds > 0:
		return fmt.Sprintf("backlog=%d lag=%ds, pressure building", metrics.BacklogRows, metrics.LagSeconds)
	case metrics.BlockedEvent != nil:
		return "blocked event holding watermark, awaiting resolution"
	case metrics.WatermarkDrift != 0:
		return fmt.Sprintf("watermark drift=%d ahead of checkpoint", metrics.WatermarkDrift)
	default:
		return "drift stable—no anomalies detected"
	}
}

func nonEmpty(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

const outboxEventTimeout = 2 * time.Minute

type SchedulerSnapshotService struct {
	cache                 SchedulerCache
	outboxRepo            SchedulerOutboxRepository
	accountRepo           AccountRepository
	groupRepo             GroupRepository
	cfg                   *config.Config
	checkpointRepo        SchedulerOutboxCheckpointRepository
	stopCh                chan struct{}
	stopOnce              sync.Once
	wg                    sync.WaitGroup
	fallbackLimit         *fallbackLimiter
	lagMu                 sync.Mutex
	lagFailures           int
	outboxRetryMu         sync.Mutex
	outboxRetryState      map[int64]schedulerOutboxRetryState
	outboxRebuildMu       sync.Mutex
	outboxRebuildNextAt   time.Time
	outboxRebuildReason   string
	outboxEventHandler    func(context.Context, SchedulerOutboxEvent) error
	fullRebuildRunning    atomic.Bool
	outboxPollTimeout     time.Duration
	outboxCommitTimeout   time.Duration
	outboxLagCheckTimeout time.Duration
	fullRebuildTimeout    time.Duration
}

func NewSchedulerSnapshotService(
	cache SchedulerCache,
	outboxRepo SchedulerOutboxRepository,
	accountRepo AccountRepository,
	groupRepo GroupRepository,
	checkpointRepo SchedulerOutboxCheckpointRepository,
	cfg *config.Config,
) *SchedulerSnapshotService {
	maxQPS := 0
	if cfg != nil {
		maxQPS = cfg.Gateway.Scheduling.DbFallbackMaxQPS
	}
	service := &SchedulerSnapshotService{
		cache:                 cache,
		outboxRepo:            outboxRepo,
		accountRepo:           accountRepo,
		groupRepo:             groupRepo,
		checkpointRepo:        checkpointRepo,
		cfg:                   cfg,
		stopCh:                make(chan struct{}),
		fallbackLimit:         newFallbackLimiter(maxQPS, fallbackCooldownDuration),
		outboxRetryState:      make(map[int64]schedulerOutboxRetryState),
		outboxPollTimeout:     defaultSchedulerOutboxPollTimeout,
		outboxCommitTimeout:   defaultSchedulerOutboxCommitTimeout,
		outboxLagCheckTimeout: defaultSchedulerOutboxLagCheckTimeout,
		fullRebuildTimeout:    defaultSchedulerFullRebuildTimeout,
	}
	service.outboxEventHandler = service.handleOutboxEvent
	return service
}

func (s *SchedulerSnapshotService) Start() {
	if s == nil || s.cache == nil {
		return
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.runInitialRebuild()
	}()

	interval := s.outboxPollInterval()
	if s.outboxRepo != nil && interval > 0 {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.runOutboxWorker(interval)
		}()
	}

	fullInterval := s.fullRebuildInterval()
	if fullInterval > 0 {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.runFullRebuildWorker(fullInterval)
		}()
	}
}

func (s *SchedulerSnapshotService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
	s.wg.Wait()
}

func (s *SchedulerSnapshotService) ListSchedulableAccounts(ctx context.Context, groupID *int64, platform string, hasForcePlatform bool) ([]Account, bool, error) {
	useMixed := (platform == PlatformAnthropic || platform == PlatformGemini) && !hasForcePlatform
	mode := s.resolveMode(platform, hasForcePlatform)
	bucket := s.bucketFor(groupID, platform, mode)

	if s.cache != nil {
		cached, hit, err := s.cache.GetSnapshot(ctx, bucket)
		if err != nil {
			logger.LegacyPrintf("service.scheduler_snapshot", "[Scheduler] cache read failed: bucket=%s err=%v", bucket.String(), err)
		} else if hit {
			return derefAccounts(cached), useMixed, nil
		}
	}

	if err := s.guardFallback(ctx); err != nil {
		return nil, useMixed, err
	}

	fallbackCtx, cancel := s.withFallbackTimeout(ctx)
	defer cancel()

	accounts, err := s.loadAccountsFromDB(fallbackCtx, bucket, useMixed)
	if err != nil {
		return nil, useMixed, err
	}

	if s.cache != nil {
		if err := s.cache.SetSnapshot(fallbackCtx, bucket, accounts); err != nil {
			logger.LegacyPrintf("service.scheduler_snapshot", "[Scheduler] cache write failed: bucket=%s err=%v", bucket.String(), err)
		}
	}

	return accounts, useMixed, nil
}

func (s *SchedulerSnapshotService) GetAccount(ctx context.Context, accountID int64) (*Account, error) {
	if accountID <= 0 {
		return nil, nil
	}
	if s.cache != nil {
		account, err := s.cache.GetAccount(ctx, accountID)
		if err != nil {
			logger.LegacyPrintf("service.scheduler_snapshot", "[Scheduler] account cache read failed: id=%d err=%v", accountID, err)
		} else if account != nil {
			return account, nil
		}
	}

	if err := s.guardFallback(ctx); err != nil {
		return nil, err
	}
	fallbackCtx, cancel := s.withFallbackTimeout(ctx)
	defer cancel()
	return s.accountRepo.GetByID(fallbackCtx, accountID)
}

// GetGroupByID 获取分组信息（供调度器使用）
func (s *SchedulerSnapshotService) GetGroupByID(ctx context.Context, groupID int64) (*Group, error) {
	if s.groupRepo == nil {
		return nil, nil
	}
	return s.groupRepo.GetByID(ctx, groupID)
}

// UpdateAccountInCache 立即更新 Redis 中单个账号的数据（用于模型限流后立即生效）
func (s *SchedulerSnapshotService) UpdateAccountInCache(ctx context.Context, account *Account) error {
	if s.cache == nil || account == nil {
		return nil
	}
	return s.cache.SetAccount(ctx, account)
}

func (s *SchedulerSnapshotService) runInitialRebuild() {
	if s.cache == nil {
		return
	}
	if err := s.triggerFullRebuild("startup"); err != nil {
		logger.LegacyPrintf("service.scheduler_snapshot", "[Scheduler] rebuild startup failed: %v", err)
	}
}

func (s *SchedulerSnapshotService) runOutboxWorker(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	s.pollOutbox()
	for {
		select {
		case <-ticker.C:
			s.pollOutbox()
		case <-s.stopCh:
			return
		}
	}
}

func (s *SchedulerSnapshotService) runFullRebuildWorker(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := s.triggerFullRebuild("interval"); err != nil {
				logger.LegacyPrintf("service.scheduler_snapshot", "[Scheduler] full rebuild failed: %v", err)
			}
		case <-s.stopCh:
			return
		}
	}
}

func (s *SchedulerSnapshotService) pollOutbox() {
	if s.outboxRepo == nil || s.cache == nil {
		return
	}
	readCtx, cancelRead := s.newBackgroundTimeoutContext(s.outboxPollTimeout)
	defer cancelRead()

	watermark, err := s.cache.GetOutboxWatermark(readCtx)
	if err != nil {
		logger.LegacyPrintf("service.scheduler_snapshot", "[Scheduler] outbox watermark read failed: %v", err)
		checkpointCtx, cancelCheckpoint := s.newBackgroundTimeoutContext(s.outboxCommitTimeout)
		if fallback, fallbackErr := s.loadCheckpointWatermark(checkpointCtx); fallbackErr == nil {
			watermark = fallback
			cancelCheckpoint()
		} else {
			cancelCheckpoint()
			return
		}
	} else {
		recordSchedulerOutboxRedisWatermark(watermark)
	}

	events, err := s.outboxRepo.ListAfter(readCtx, watermark, 200)
	if err != nil {
		logger.LegacyPrintf("service.scheduler_snapshot", "[Scheduler] outbox poll failed: %v", err)
		return
	}
	if len(events) == 0 {
		schedulerOutboxBacklogRows.Store(0)
		schedulerOutboxLagSeconds.Store(0)
		schedulerOutboxLagFailureStreak.Store(0)
		clearSchedulerOutboxBlockedEvent("queue_drained")
		return
	}
	events = coalesceAdjacentSchedulerOutboxEvents(events)

	watermarkForCheck := watermark
	lastAdvanceID := watermark
	handler := s.outboxEventHandler
	if handler == nil {
		handler = s.handleOutboxEvent
	}
	var blockedEvent *SchedulerOutboxEvent
	for _, event := range events {
		if event.PayloadDecodeError != "" {
			decodeErr := fmt.Errorf("payload decode failed: %s", event.PayloadDecodeError)
			s.reportPoisonOutboxEvent(event, markOutboxPoison(decodeErr))
			lastAdvanceID = event.ID
			continue
		}
		if retryState, blocked := s.peekOutboxRetryState(event.ID); blocked {
			s.recordBlockedOutboxEvent(event, retryState, nil)
			blockedEvent = &event
			break
		}
		eventCtx, cancel := context.WithTimeout(context.Background(), outboxEventTimeout)
		err := handler(eventCtx, event)
		cancel()
		if err != nil {
			if errors.Is(err, errOutboxPoisonEvent) {
				s.reportPoisonOutboxEvent(event, err)
				s.clearOutboxRetryState(event.ID)
				lastAdvanceID = event.ID
				continue
			}
			s.markOutboxRetryState(event.ID, classifySchedulerOutboxTransientReason(err))
			s.reportTransientOutboxError(event, err)
			retryState, _ := s.peekOutboxRetryState(event.ID)
			s.recordBlockedOutboxEvent(event, retryState, err)
			blockedEvent = &event
			break
		}
		s.clearOutboxRetryState(event.ID)
		lastAdvanceID = event.ID
	}

	if lastAdvanceID > watermark {
		commitCtx, cancelCommit := s.newBackgroundTimeoutContext(s.outboxCommitTimeout)
		if err := s.cache.SetOutboxWatermark(commitCtx, lastAdvanceID); err != nil {
			logger.LegacyPrintf("service.scheduler_snapshot", "[Scheduler] outbox watermark write failed: %v", err)
		} else {
			watermarkForCheck = lastAdvanceID
			recordSchedulerOutboxRedisWatermark(lastAdvanceID)
			s.clearOutboxRetryStateUpTo(lastAdvanceID)
			checkpointCtx, cancelCheckpoint := s.newBackgroundTimeoutContext(s.outboxCommitTimeout)
			s.persistCheckpointWatermark(checkpointCtx, lastAdvanceID)
			cancelCheckpoint()
		}
		cancelCommit()
	}

	lagCtx, cancelLag := s.newBackgroundTimeoutContext(s.outboxLagCheckTimeout)
	if blockedEvent != nil {
		s.checkOutboxLag(lagCtx, *blockedEvent, watermarkForCheck)
		cancelLag()
		return
	}
	clearSchedulerOutboxBlockedEvent("recovered")

	s.checkOutboxLag(lagCtx, events[0], watermarkForCheck)
	cancelLag()
}

func (s *SchedulerSnapshotService) handleOutboxEvent(ctx context.Context, event SchedulerOutboxEvent) error {
	switch event.EventType {
	case SchedulerOutboxEventAccountLastUsed:
		return s.handleLastUsedEvent(ctx, event.Payload)
	case SchedulerOutboxEventAccountBulkChanged:
		return s.handleBulkAccountEvent(ctx, event.Payload)
	case SchedulerOutboxEventAccountGroupsChanged:
		if event.AccountID == nil || *event.AccountID <= 0 {
			return markOutboxPoison(fmt.Errorf("malformed account event: missing account_id"))
		}
		return s.handleAccountEvent(ctx, event.AccountID, event.Payload)
	case SchedulerOutboxEventAccountChanged:
		if event.AccountID == nil || *event.AccountID <= 0 {
			return markOutboxPoison(fmt.Errorf("malformed account event: missing account_id"))
		}
		return s.handleAccountEvent(ctx, event.AccountID, event.Payload)
	case SchedulerOutboxEventGroupChanged:
		if event.GroupID == nil || *event.GroupID <= 0 {
			return markOutboxPoison(fmt.Errorf("malformed group event: missing group_id"))
		}
		return s.handleGroupEvent(ctx, event.GroupID)
	case SchedulerOutboxEventFullRebuild:
		return s.triggerFullRebuild("outbox")
	default:
		return markOutboxPoison(fmt.Errorf("unsupported outbox event type: %s", event.EventType))
	}
}

func (s *SchedulerSnapshotService) handleLastUsedEvent(ctx context.Context, payload map[string]any) error {
	if s.cache == nil || payload == nil {
		return markOutboxPoison(errors.New("malformed last_used payload"))
	}
	raw, ok := payload["last_used"].(map[string]any)
	if !ok || len(raw) == 0 {
		return markOutboxPoison(errors.New("malformed last_used payload"))
	}
	updates := make(map[int64]time.Time, len(raw))
	for key, value := range raw {
		id, err := strconv.ParseInt(key, 10, 64)
		if err != nil || id <= 0 {
			continue
		}
		sec, ok := toInt64(value)
		if !ok || sec <= 0 {
			continue
		}
		updates[id] = time.Unix(sec, 0)
	}
	if len(updates) == 0 {
		return markOutboxPoison(errors.New("malformed last_used payload"))
	}
	return s.cache.UpdateLastUsed(ctx, updates)
}

func (s *SchedulerSnapshotService) handleBulkAccountEvent(ctx context.Context, payload map[string]any) error {
	if payload == nil {
		return markOutboxPoison(errors.New("malformed bulk account payload"))
	}
	if s.accountRepo == nil {
		return nil
	}

	rawIDs := parseInt64Slice(payload["account_ids"])
	if len(rawIDs) == 0 {
		return markOutboxPoison(errors.New("malformed bulk account payload"))
	}

	ids := make([]int64, 0, len(rawIDs))
	seen := make(map[int64]struct{}, len(rawIDs))
	for _, id := range rawIDs {
		if id <= 0 {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return markOutboxPoison(errors.New("malformed bulk account payload"))
	}

	preloadGroupIDs := parseInt64Slice(payload["group_ids"])
	accounts, err := s.accountRepo.GetByIDs(ctx, ids)
	if err != nil {
		return err
	}

	found := make(map[int64]struct{}, len(accounts))
	rebuildGroupSet := make(map[int64]struct{}, len(preloadGroupIDs))
	for _, gid := range preloadGroupIDs {
		if gid > 0 {
			rebuildGroupSet[gid] = struct{}{}
		}
	}

	for _, account := range accounts {
		if account == nil || account.ID <= 0 {
			continue
		}
		found[account.ID] = struct{}{}
		if s.cache != nil {
			if err := s.cache.SetAccount(ctx, account); err != nil {
				return err
			}
		}
		for _, gid := range account.GroupIDs {
			if gid > 0 {
				rebuildGroupSet[gid] = struct{}{}
			}
		}
	}

	if s.cache != nil {
		for _, id := range ids {
			if _, ok := found[id]; ok {
				continue
			}
			if err := s.cache.DeleteAccount(ctx, id); err != nil {
				return err
			}
		}
	}

	rebuildGroupIDs := make([]int64, 0, len(rebuildGroupSet))
	for gid := range rebuildGroupSet {
		rebuildGroupIDs = append(rebuildGroupIDs, gid)
	}
	return s.rebuildByGroupIDs(ctx, rebuildGroupIDs, "account_bulk_change")
}

func (s *SchedulerSnapshotService) handleAccountEvent(ctx context.Context, accountID *int64, payload map[string]any) error {
	if accountID == nil || *accountID <= 0 {
		return nil
	}
	if s.accountRepo == nil {
		return nil
	}

	account, err := s.accountRepo.GetByID(ctx, *accountID)
	if err != nil {
		if errors.Is(err, ErrAccountNotFound) {
			if s.cache != nil {
				if err := s.cache.DeleteAccount(ctx, *accountID); err != nil {
					return err
				}
			}
			groupIDs := resolveSchedulerAccountEventGroupIDs(nil, payload)
			return s.rebuildByGroupIDs(ctx, groupIDs, "account_miss")
		}
		return err
	}
	if s.cache != nil {
		if err := s.cache.SetAccount(ctx, account); err != nil {
			return err
		}
	}
	groupIDs := resolveSchedulerAccountEventGroupIDs(account, payload)
	return s.rebuildByAccount(ctx, account, groupIDs, "account_change")
}

func (s *SchedulerSnapshotService) handleGroupEvent(ctx context.Context, groupID *int64) error {
	if groupID == nil || *groupID <= 0 {
		return nil
	}
	groupIDs := []int64{*groupID}
	return s.rebuildByGroupIDs(ctx, groupIDs, "group_change")
}

func (s *SchedulerSnapshotService) rebuildByAccount(ctx context.Context, account *Account, groupIDs []int64, reason string) error {
	if account == nil {
		return nil
	}
	groupIDs = s.normalizeGroupIDs(groupIDs)
	if len(groupIDs) == 0 {
		return nil
	}

	var firstErr error
	if err := s.rebuildBucketsForPlatform(ctx, account.Platform, groupIDs, reason); err != nil && firstErr == nil {
		firstErr = err
	}
	if account.Platform == PlatformAntigravity && account.IsMixedSchedulingEnabled() {
		if err := s.rebuildBucketsForPlatform(ctx, PlatformAnthropic, groupIDs, reason); err != nil && firstErr == nil {
			firstErr = err
		}
		if err := s.rebuildBucketsForPlatform(ctx, PlatformGemini, groupIDs, reason); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func coalesceAdjacentSchedulerOutboxEvents(events []SchedulerOutboxEvent) []SchedulerOutboxEvent {
	if len(events) <= 1 {
		return events
	}

	out := make([]SchedulerOutboxEvent, 0, len(events))
	current := events[0]
	for i := 1; i < len(events); i++ {
		if merged, ok := mergeAdjacentSchedulerOutboxEvents(current, events[i]); ok {
			current = merged
			continue
		}
		out = append(out, current)
		current = events[i]
	}
	out = append(out, current)
	if saved := len(events) - len(out); saved > 0 {
		schedulerOutboxCoalescedBatchTotal.Add(1)
		schedulerOutboxCoalescedEventSavedTotal.Add(int64(saved))
	}
	return out
}

func mergeAdjacentSchedulerOutboxEvents(prev, next SchedulerOutboxEvent) (SchedulerOutboxEvent, bool) {
	if prev.PayloadDecodeError != "" || next.PayloadDecodeError != "" {
		return SchedulerOutboxEvent{}, false
	}
	if prev.EventType != next.EventType {
		return SchedulerOutboxEvent{}, false
	}

	merged := prev
	merged.ID = next.ID
	merged.CreatedAt = earliestNonZeroTime(prev.CreatedAt, next.CreatedAt)

	switch prev.EventType {
	case SchedulerOutboxEventAccountChanged, SchedulerOutboxEventAccountGroupsChanged:
		if prev.AccountID == nil || next.AccountID == nil || *prev.AccountID != *next.AccountID {
			return SchedulerOutboxEvent{}, false
		}
		merged.Payload = mergeSchedulerGroupPayload(prev.Payload, next.Payload)
		return merged, true
	case SchedulerOutboxEventGroupChanged:
		if prev.GroupID == nil || next.GroupID == nil || *prev.GroupID != *next.GroupID {
			return SchedulerOutboxEvent{}, false
		}
		return merged, true
	case SchedulerOutboxEventFullRebuild:
		return merged, true
	case SchedulerOutboxEventAccountBulkChanged:
		merged.Payload = mergeSchedulerBulkAccountPayload(prev.Payload, next.Payload)
		return merged, true
	case SchedulerOutboxEventAccountLastUsed:
		merged.Payload = mergeSchedulerLastUsedPayload(prev.Payload, next.Payload)
		return merged, true
	default:
		return SchedulerOutboxEvent{}, false
	}
}

func mergeSchedulerGroupPayload(a, b map[string]any) map[string]any {
	groupIDs := stableUniqueInt64(
		parseInt64Slice(payloadSliceValue(a, "group_ids")),
		parseInt64Slice(payloadSliceValue(b, "group_ids")),
	)
	if len(groupIDs) == 0 {
		return nil
	}
	return map[string]any{"group_ids": int64SliceToAny(groupIDs)}
}

func mergeSchedulerBulkAccountPayload(a, b map[string]any) map[string]any {
	accountIDs := stableUniqueInt64(
		parseInt64Slice(payloadSliceValue(a, "account_ids")),
		parseInt64Slice(payloadSliceValue(b, "account_ids")),
	)
	groupIDs := stableUniqueInt64(
		parseInt64Slice(payloadSliceValue(a, "group_ids")),
		parseInt64Slice(payloadSliceValue(b, "group_ids")),
	)
	if len(accountIDs) == 0 && len(groupIDs) == 0 {
		return nil
	}

	payload := make(map[string]any, 2)
	if len(accountIDs) > 0 {
		payload["account_ids"] = int64SliceToAny(accountIDs)
	}
	if len(groupIDs) > 0 {
		payload["group_ids"] = int64SliceToAny(groupIDs)
	}
	return payload
}

func mergeSchedulerLastUsedPayload(a, b map[string]any) map[string]any {
	lastUsed := make(map[string]any)
	mergeOne := func(payload map[string]any) {
		if payload == nil {
			return
		}
		raw, ok := payload["last_used"].(map[string]any)
		if !ok {
			return
		}
		for key, value := range raw {
			sec, ok := toInt64(value)
			if !ok || sec <= 0 {
				continue
			}
			if existing, exists := lastUsed[key]; exists {
				if prev, ok := toInt64(existing); ok && prev >= sec {
					continue
				}
			}
			lastUsed[key] = sec
		}
	}

	mergeOne(a)
	mergeOne(b)
	if len(lastUsed) == 0 {
		return nil
	}
	return map[string]any{"last_used": lastUsed}
}

func payloadSliceValue(payload map[string]any, key string) any {
	if payload == nil {
		return nil
	}
	return payload[key]
}

func int64SliceToAny(values []int64) []any {
	if len(values) == 0 {
		return nil
	}
	out := make([]any, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	return out
}

func stableUniqueInt64(groups ...[]int64) []int64 {
	seen := make(map[int64]struct{})
	out := make([]int64, 0)
	for _, values := range groups {
		for _, value := range values {
			if value <= 0 {
				continue
			}
			if _, ok := seen[value]; ok {
				continue
			}
			seen[value] = struct{}{}
			out = append(out, value)
		}
	}
	return out
}

func earliestNonZeroTime(a, b time.Time) time.Time {
	if a.IsZero() {
		return b
	}
	if b.IsZero() {
		return a
	}
	if b.Before(a) {
		return b
	}
	return a
}

func (s *SchedulerSnapshotService) rebuildByGroupIDs(ctx context.Context, groupIDs []int64, reason string) error {
	groupIDs = s.normalizeGroupIDs(groupIDs)
	if len(groupIDs) == 0 {
		return nil
	}
	platforms := []string{PlatformAnthropic, PlatformGemini, PlatformOpenAI, PlatformAntigravity}
	var firstErr error
	for _, platform := range platforms {
		if err := s.rebuildBucketsForPlatform(ctx, platform, groupIDs, reason); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (s *SchedulerSnapshotService) rebuildBucketsForPlatform(ctx context.Context, platform string, groupIDs []int64, reason string) error {
	if platform == "" {
		return nil
	}
	var firstErr error
	for _, gid := range groupIDs {
		if err := s.rebuildBucket(ctx, SchedulerBucket{GroupID: gid, Platform: platform, Mode: SchedulerModeSingle}, reason); err != nil && firstErr == nil {
			firstErr = err
		}
		if err := s.rebuildBucket(ctx, SchedulerBucket{GroupID: gid, Platform: platform, Mode: SchedulerModeForced}, reason); err != nil && firstErr == nil {
			firstErr = err
		}
		if platform == PlatformAnthropic || platform == PlatformGemini {
			if err := s.rebuildBucket(ctx, SchedulerBucket{GroupID: gid, Platform: platform, Mode: SchedulerModeMixed}, reason); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

func (s *SchedulerSnapshotService) rebuildBuckets(ctx context.Context, buckets []SchedulerBucket, reason string) error {
	var firstErr error
	lockContentionSkipped := 0
	for _, bucket := range buckets {
		if err := s.rebuildBucket(ctx, bucket, reason); err != nil {
			if isSchedulerLockContentionError(err) {
				lockContentionSkipped++
				continue
			}
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	if lockContentionSkipped > 0 {
		schedulerOutboxBusyBucketSkipTotal.Add(int64(lockContentionSkipped))
		logger.LegacyPrintf(
			"service.scheduler_snapshot",
			"[Scheduler] rebuild skipped busy buckets: count=%d reason=%s",
			lockContentionSkipped,
			strings.TrimSpace(reason),
		)
	}
	return firstErr
}

func (s *SchedulerSnapshotService) rebuildBucket(ctx context.Context, bucket SchedulerBucket, reason string) error {
	if s.cache == nil {
		return ErrSchedulerCacheNotReady
	}
	release, err := s.acquireBucketLock(ctx, bucket)
	if err != nil {
		recordSchedulerOutboxBucketRebuild(bucket, reason, classifySchedulerOutboxTransientReason(err))
		return err
	}
	if release != nil {
		defer release()
	}

	rebuildCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	accounts, err := s.loadAccountsFromDB(rebuildCtx, bucket, bucket.Mode == SchedulerModeMixed)
	if err != nil {
		logger.LegacyPrintf("service.scheduler_snapshot", "[Scheduler] rebuild failed: bucket=%s reason=%s err=%v", bucket.String(), reason, err)
		recordSchedulerOutboxBucketRebuild(bucket, reason, "db_failure")
		return wrapOutboxRetryable(fmt.Errorf("db load failed: %w", err))
	}
	if err := s.cache.SetSnapshot(rebuildCtx, bucket, accounts); err != nil {
		logger.LegacyPrintf("service.scheduler_snapshot", "[Scheduler] rebuild cache failed: bucket=%s reason=%s err=%v", bucket.String(), reason, err)
		recordSchedulerOutboxBucketRebuild(bucket, reason, "cache_failure")
		return wrapOutboxRetryable(fmt.Errorf("cache write failed: %w", err))
	}
	slog.Debug("[Scheduler] rebuild ok", "bucket", bucket.String(), "reason", reason, "size", len(accounts))
	recordSchedulerOutboxBucketRebuild(bucket, reason, "success")
	return nil
}

func recordSchedulerOutboxCheckpointFallback(reason string) {
	schedulerOutboxRuntimeMu.Lock()
	defer schedulerOutboxRuntimeMu.Unlock()
	schedulerOutboxCheckpointLastFallbackAt = time.Now().UTC().Format(time.RFC3339)
	schedulerOutboxCheckpointLastFallbackReason = strings.TrimSpace(reason)
}

func recordSchedulerOutboxCheckpointReadFailure() {
	schedulerOutboxRuntimeMu.Lock()
	defer schedulerOutboxRuntimeMu.Unlock()
	schedulerOutboxCheckpointLastReadFailureAt = time.Now().UTC().Format(time.RFC3339)
}

func recordSchedulerOutboxCheckpointWriteFailure() {
	schedulerOutboxRuntimeMu.Lock()
	defer schedulerOutboxRuntimeMu.Unlock()
	schedulerOutboxCheckpointLastWriteFailureAt = time.Now().UTC().Format(time.RFC3339)
}

func recordSchedulerOutboxLagRebuild() {
	schedulerOutboxRuntimeMu.Lock()
	defer schedulerOutboxRuntimeMu.Unlock()
	schedulerOutboxLastLagRebuildAt = time.Now().UTC().Format(time.RFC3339)
}

func recordSchedulerOutboxBacklogRebuild() {
	schedulerOutboxRuntimeMu.Lock()
	defer schedulerOutboxRuntimeMu.Unlock()
	schedulerOutboxLastBacklogRebuildAt = time.Now().UTC().Format(time.RFC3339)
}

func recordSchedulerOutboxBucketRebuild(bucket SchedulerBucket, reason string, status string) {
	switch status {
	case "success":
		schedulerOutboxBucketRebuildSuccessTotal.Add(1)
	case "lock_contention":
		schedulerOutboxBucketRebuildFailureTotal.Add(1)
		schedulerOutboxBucketRebuildLockContention.Add(1)
	default:
		schedulerOutboxBucketRebuildFailureTotal.Add(1)
	}

	schedulerOutboxRuntimeMu.Lock()
	defer schedulerOutboxRuntimeMu.Unlock()
	schedulerOutboxLastBucketRebuildAt = time.Now().UTC().Format(time.RFC3339)
	schedulerOutboxLastBucketRebuildReason = strings.TrimSpace(reason)
	schedulerOutboxLastBucketRebuildStatus = strings.TrimSpace(status)
	schedulerOutboxLastBucketRebuildBucket = bucket.String()
}

func (s *SchedulerSnapshotService) acquireBucketLock(ctx context.Context, bucket SchedulerBucket) (func(), error) {
	if s.cache == nil {
		return nil, ErrSchedulerCacheNotReady
	}
	if ownerCache, ok := s.cache.(SchedulerOwnedBucketLockCache); ok {
		owner := newSchedulerBucketLockOwner()
		acquired, err := ownerCache.TryLockBucketWithOwner(ctx, bucket, owner, schedulerBucketLockTTL)
		if err != nil {
			return nil, wrapOutboxRetryable(fmt.Errorf("cache lock failed: %w", err))
		}
		if !acquired {
			return nil, wrapOutboxRetryable(errors.New("lock contention while rebuilding bucket"))
		}
		return func() {
			releaseCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if err := ownerCache.ReleaseBucketLock(releaseCtx, bucket, owner); err != nil {
				logger.LegacyPrintf("service.scheduler_snapshot", "[Scheduler] bucket lock release failed: bucket=%s err=%v", bucket.String(), err)
			}
		}, nil
	}

	acquired, err := s.cache.TryLockBucket(ctx, bucket, schedulerBucketLockTTL)
	if err != nil {
		return nil, wrapOutboxRetryable(fmt.Errorf("cache lock failed: %w", err))
	}
	if !acquired {
		return nil, wrapOutboxRetryable(errors.New("lock contention while rebuilding bucket"))
	}
	return nil, nil
}

func newSchedulerBucketLockOwner() string {
	seq := schedulerBucketLockOwnerSeq.Add(1)
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), seq)
}

func (s *SchedulerSnapshotService) peekOutboxRetryState(eventID int64) (schedulerOutboxRetryState, bool) {
	if s == nil || eventID <= 0 {
		return schedulerOutboxRetryState{}, false
	}
	s.outboxRetryMu.Lock()
	defer s.outboxRetryMu.Unlock()
	state, ok := s.outboxRetryState[eventID]
	if !ok {
		return schedulerOutboxRetryState{}, false
	}
	if time.Now().Before(state.NextAttemptAt) {
		return state, true
	}
	return state, false
}

func (s *SchedulerSnapshotService) markOutboxRetryState(eventID int64, reason string) {
	if s == nil || eventID <= 0 {
		return
	}
	s.outboxRetryMu.Lock()
	defer s.outboxRetryMu.Unlock()
	state := s.outboxRetryState[eventID]
	if state.LastReason != reason {
		state.Attempts = 0
	}
	now := time.Now()
	if state.FirstSeenAt.IsZero() {
		state.FirstSeenAt = now
	}
	state.Attempts++
	state.LastReason = reason
	state.LastSeenAt = now
	state.NextAttemptAt = now.Add(outboxRetryBackoff(reason, state.Attempts))
	s.outboxRetryState[eventID] = state
}

func (s *SchedulerSnapshotService) clearOutboxRetryState(eventID int64) {
	if s == nil || eventID <= 0 {
		return
	}
	s.outboxRetryMu.Lock()
	delete(s.outboxRetryState, eventID)
	s.outboxRetryMu.Unlock()
}

func (s *SchedulerSnapshotService) clearOutboxRetryStateUpTo(eventID int64) {
	if s == nil || eventID <= 0 {
		return
	}
	s.outboxRetryMu.Lock()
	for id := range s.outboxRetryState {
		if id <= eventID {
			delete(s.outboxRetryState, id)
		}
	}
	s.outboxRetryMu.Unlock()
}

func outboxRetryBackoff(reason string, attempts int) time.Duration {
	if attempts <= 0 {
		attempts = 1
	}
	backoff := outboxRetryBaseBackoff
	if reason == "lock_contention" {
		backoff = outboxRetryLockContentionBackoff
	}
	for i := 1; i < attempts; i++ {
		backoff *= 2
		if backoff >= outboxRetryMaxBackoff {
			return outboxRetryMaxBackoff
		}
	}
	if backoff > outboxRetryMaxBackoff {
		return outboxRetryMaxBackoff
	}
	return backoff
}

func (s *SchedulerSnapshotService) outboxRebuildCooldown() time.Duration {
	if s == nil || s.cfg == nil {
		return 0
	}
	seconds := s.cfg.Gateway.Scheduling.OutboxRebuildCooldownSeconds
	if seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

func (s *SchedulerSnapshotService) allowOutboxTriggeredRebuild(reason string) bool {
	if s == nil {
		return false
	}
	cooldown := s.outboxRebuildCooldown()
	if cooldown <= 0 {
		return true
	}

	now := time.Now()
	s.outboxRebuildMu.Lock()
	defer s.outboxRebuildMu.Unlock()
	if now.Before(s.outboxRebuildNextAt) {
		schedulerOutboxRebuildCooldownSkipTotal.Add(1)
		return false
	}
	s.outboxRebuildNextAt = now.Add(cooldown)
	s.outboxRebuildReason = strings.TrimSpace(reason)
	return true
}

func (s *SchedulerSnapshotService) triggerFullRebuild(reason string) error {
	if s.cache == nil {
		return ErrSchedulerCacheNotReady
	}
	if !s.fullRebuildRunning.CompareAndSwap(false, true) {
		logger.LegacyPrintf("service.scheduler_snapshot", "[Scheduler] full rebuild skipped: already running (reason=%s)", strings.TrimSpace(reason))
		return nil
	}
	defer s.fullRebuildRunning.Store(false)

	ctx, cancel := s.newBackgroundTimeoutContext(s.fullRebuildTimeout)
	defer cancel()

	buckets, authoritative, err := s.loadFullRebuildBuckets(ctx)
	if err != nil {
		return err
	}
	if authoritative {
		if syncer, ok := s.cache.(SchedulerBucketRegistrySyncCache); ok {
			if err := syncer.ReplaceBuckets(ctx, buckets); err != nil {
				logger.LegacyPrintf("service.scheduler_snapshot", "[Scheduler] bucket registry sync failed: %v", err)
			}
		}
	}
	return s.rebuildBuckets(ctx, buckets, reason)
}

func (s *SchedulerSnapshotService) loadFullRebuildBuckets(ctx context.Context) ([]SchedulerBucket, bool, error) {
	if buckets, err := s.defaultBuckets(ctx); err == nil && len(buckets) > 0 {
		return buckets, true, nil
	} else if err != nil {
		logger.LegacyPrintf("service.scheduler_snapshot", "[Scheduler] default buckets failed: %v", err)
	}

	buckets, err := s.cache.ListBuckets(ctx)
	if err != nil {
		logger.LegacyPrintf("service.scheduler_snapshot", "[Scheduler] list buckets failed: %v", err)
		return nil, false, err
	}
	if len(buckets) == 0 {
		buckets, err = s.defaultBuckets(ctx)
		if err != nil {
			logger.LegacyPrintf("service.scheduler_snapshot", "[Scheduler] default buckets failed: %v", err)
			return nil, false, err
		}
		return buckets, true, nil
	}
	return buckets, false, nil
}

func (s *SchedulerSnapshotService) newBackgroundTimeoutContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return context.WithCancel(context.Background())
	}
	return context.WithTimeout(context.Background(), timeout)
}

func (s *SchedulerSnapshotService) checkOutboxLag(ctx context.Context, oldest SchedulerOutboxEvent, watermark int64) {
	if oldest.CreatedAt.IsZero() || s.cfg == nil {
		return
	}

	lag := time.Since(oldest.CreatedAt)
	schedulerOutboxLagSeconds.Store(int64(lag.Seconds()))
	if lagSeconds := int(lag.Seconds()); lagSeconds >= s.cfg.Gateway.Scheduling.OutboxLagWarnSeconds && s.cfg.Gateway.Scheduling.OutboxLagWarnSeconds > 0 {
		logger.LegacyPrintf("service.scheduler_snapshot", "[Scheduler] outbox lag warning: %ds", lagSeconds)
	}

	if s.cfg.Gateway.Scheduling.OutboxLagRebuildSeconds > 0 && int(lag.Seconds()) >= s.cfg.Gateway.Scheduling.OutboxLagRebuildSeconds {
		s.lagMu.Lock()
		s.lagFailures++
		failures := s.lagFailures
		s.lagMu.Unlock()
		schedulerOutboxLagFailureStreak.Store(int64(failures))

		if failures >= s.cfg.Gateway.Scheduling.OutboxLagRebuildFailures {
			s.lagMu.Lock()
			s.lagFailures = 0
			s.lagMu.Unlock()
			schedulerOutboxLagFailureStreak.Store(0)
			if s.allowOutboxTriggeredRebuild("outbox_lag") {
				logger.LegacyPrintf("service.scheduler_snapshot", "[Scheduler] outbox lag rebuild triggered: lag=%s failures=%d", lag, failures)
				schedulerOutboxLagRebuildTotal.Add(1)
				recordSchedulerOutboxLagRebuild()
				if err := s.triggerFullRebuild("outbox_lag"); err != nil {
					logger.LegacyPrintf("service.scheduler_snapshot", "[Scheduler] outbox lag rebuild failed: %v", err)
				}
			}
		}
	} else {
		s.lagMu.Lock()
		s.lagFailures = 0
		s.lagMu.Unlock()
		schedulerOutboxLagFailureStreak.Store(0)
	}

	threshold := s.cfg.Gateway.Scheduling.OutboxBacklogRebuildRows
	if threshold <= 0 || s.outboxRepo == nil {
		return
	}
	maxID, err := s.outboxRepo.MaxID(ctx)
	if err != nil {
		return
	}
	schedulerOutboxBacklogRows.Store(maxID - watermark)
	if maxID-watermark >= int64(threshold) && s.allowOutboxTriggeredRebuild("outbox_backlog") {
		logger.LegacyPrintf("service.scheduler_snapshot", "[Scheduler] outbox backlog rebuild triggered: backlog=%d", maxID-watermark)
		schedulerOutboxBacklogRebuildTotal.Add(1)
		recordSchedulerOutboxBacklogRebuild()
		if err := s.triggerFullRebuild("outbox_backlog"); err != nil {
			logger.LegacyPrintf("service.scheduler_snapshot", "[Scheduler] outbox backlog rebuild failed: %v", err)
		}
	}
}

func (s *SchedulerSnapshotService) reportPoisonOutboxEvent(event SchedulerOutboxEvent, err error) {
	if err == nil {
		return
	}
	recordSchedulerOutboxPoisonEvent(event, err)
	logger.LegacyPrintf("service.scheduler_snapshot", "[Scheduler] outbox event skipped (poison): id=%d type=%s err=%v%s",
		event.ID, event.EventType, err, outboxEventSnippet(event))
}

func (s *SchedulerSnapshotService) reportTransientOutboxError(event SchedulerOutboxEvent, err error) {
	if err == nil {
		return
	}
	recordSchedulerOutboxTransientEvent(event, err)
	logger.LegacyPrintf("service.scheduler_snapshot", "[Scheduler] outbox event transient error: id=%d type=%s err=%v%s",
		event.ID, event.EventType, err, outboxEventSnippet(event))
}

func outboxEventSnippet(event SchedulerOutboxEvent) string {
	if len(event.Payload) > 0 {
		return outboxPayloadSnippet(event.Payload)
	}
	if event.PayloadDecodeError == "" && len(event.PayloadRaw) == 0 {
		return ""
	}
	snippet := ""
	if len(event.PayloadRaw) > 0 {
		snippet = string(event.PayloadRaw)
		if len(snippet) > 256 {
			snippet = snippet[:256] + "..."
		}
	}
	if event.PayloadDecodeError == "" {
		return " payload_raw=" + snippet
	}
	if snippet == "" {
		return " payload_decode_error=" + event.PayloadDecodeError
	}
	return fmt.Sprintf(" payload_decode_error=%s payload_raw=%s", event.PayloadDecodeError, snippet)
}

func outboxPayloadSnippet(payload map[string]any) string {
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Sprintf(" payload_snippet_error=%v", err)
	}
	snippet := string(raw)
	if len(snippet) > 256 {
		snippet = snippet[:256] + "..."
	}
	return " payload=" + snippet
}

func (s *SchedulerSnapshotService) loadCheckpointWatermark(ctx context.Context) (int64, error) {
	if s == nil || s.checkpointRepo == nil {
		return 0, fmt.Errorf("checkpoint repo unavailable")
	}
	watermark, err := s.checkpointRepo.GetCheckpointWatermark(ctx)
	if err != nil {
		schedulerOutboxCheckpointReadFailureTotal.Add(1)
		recordSchedulerOutboxCheckpointReadFailure()
		return 0, err
	}
	schedulerOutboxCheckpointFallbackTotal.Add(1)
	schedulerOutboxCheckpointFallbackStreak.Add(1)
	recordSchedulerOutboxCheckpointFallback("redis_watermark_unavailable")
	schedulerOutboxLastCheckpointWatermark.Store(watermark)
	syncSchedulerOutboxWatermarkDrift()
	return watermark, nil
}

func (s *SchedulerSnapshotService) persistCheckpointWatermark(ctx context.Context, watermark int64) {
	if s == nil || s.checkpointRepo == nil {
		return
	}
	if err := s.checkpointRepo.SetCheckpointWatermark(ctx, watermark); err != nil {
		schedulerOutboxCheckpointWriteFailureTotal.Add(1)
		recordSchedulerOutboxCheckpointWriteFailure()
		logger.LegacyPrintf("service.scheduler_snapshot", "[Scheduler] checkpoint watermark write failed: %v", err)
		return
	}
	schedulerOutboxLastCheckpointWatermark.Store(watermark)
	syncSchedulerOutboxWatermarkDrift()
}

func recordSchedulerOutboxRedisWatermark(watermark int64) {
	schedulerOutboxLastRedisWatermark.Store(watermark)
	schedulerOutboxCheckpointFallbackStreak.Store(0)
	syncSchedulerOutboxWatermarkDrift()
}

func syncSchedulerOutboxWatermarkDrift() {
	redisWatermark := schedulerOutboxLastRedisWatermark.Load()
	checkpointWatermark := schedulerOutboxLastCheckpointWatermark.Load()
	schedulerOutboxWatermarkDrift.Store(redisWatermark - checkpointWatermark)
}

func (s *SchedulerSnapshotService) recordBlockedOutboxEvent(event SchedulerOutboxEvent, retryState schedulerOutboxRetryState, err error) {
	schedulerOutboxRuntimeMu.Lock()
	defer schedulerOutboxRuntimeMu.Unlock()

	blocked := &SchedulerOutboxBlockedEvent{
		Timestamp:          time.Now(),
		ID:                 event.ID,
		EventType:          event.EventType,
		Reason:             strings.TrimSpace(retryState.LastReason),
		Attempts:           retryState.Attempts,
		Error:              errorString(err),
		PayloadDecodeError: event.PayloadDecodeError,
	}
	if retryState.FirstSeenAt.After(time.Time{}) {
		blocked.FirstSeenAt = retryState.FirstSeenAt.UTC().Format(time.RFC3339)
		blocked.AgeSeconds = int64(time.Since(retryState.FirstSeenAt).Seconds())
		if blocked.AgeSeconds < 0 {
			blocked.AgeSeconds = 0
		}
	}
	if retryState.LastSeenAt.After(time.Time{}) {
		blocked.LastSeenAt = retryState.LastSeenAt.UTC().Format(time.RFC3339)
	}
	if blocked.Reason == "" && err != nil {
		blocked.Reason = classifySchedulerOutboxTransientReason(err)
	}
	if retryState.NextAttemptAt.After(time.Time{}) {
		blocked.NextAttemptAt = retryState.NextAttemptAt.UTC().Format(time.RFC3339)
	}
	schedulerOutboxBlockedEvent = blocked
}

func clearSchedulerOutboxBlockedEvent(reason string) {
	schedulerOutboxRuntimeMu.Lock()
	defer schedulerOutboxRuntimeMu.Unlock()
	if schedulerOutboxBlockedEvent != nil {
		schedulerOutboxBlockedEventClearTotal.Add(1)
		schedulerOutboxBlockedEventLastClearedID = schedulerOutboxBlockedEvent.ID
		schedulerOutboxBlockedEventLastClearedAt = time.Now().UTC().Format(time.RFC3339)
		schedulerOutboxBlockedEventLastClearReason = strings.TrimSpace(reason)
	}
	schedulerOutboxBlockedEvent = nil
}

func (s *SchedulerSnapshotService) loadAccountsFromDB(ctx context.Context, bucket SchedulerBucket, useMixed bool) ([]Account, error) {
	if s.accountRepo == nil {
		return nil, ErrSchedulerCacheNotReady
	}
	groupID := bucket.GroupID
	if s.isRunModeSimple() {
		groupID = 0
	}

	if useMixed {
		platforms := []string{bucket.Platform, PlatformAntigravity}
		var accounts []Account
		var err error
		if groupID > 0 {
			accounts, err = s.accountRepo.ListSchedulableByGroupIDAndPlatforms(ctx, groupID, platforms)
		} else if s.isRunModeSimple() {
			accounts, err = s.accountRepo.ListSchedulableByPlatforms(ctx, platforms)
		} else {
			accounts, err = s.accountRepo.ListSchedulableUngroupedByPlatforms(ctx, platforms)
		}
		if err != nil {
			return nil, err
		}
		filtered := make([]Account, 0, len(accounts))
		for _, acc := range accounts {
			if acc.Platform == PlatformAntigravity && !acc.IsMixedSchedulingEnabled() {
				continue
			}
			filtered = append(filtered, acc)
		}
		return filtered, nil
	}

	if groupID > 0 {
		return s.accountRepo.ListSchedulableByGroupIDAndPlatform(ctx, groupID, bucket.Platform)
	}
	if s.isRunModeSimple() {
		return s.accountRepo.ListSchedulableByPlatform(ctx, bucket.Platform)
	}
	return s.accountRepo.ListSchedulableUngroupedByPlatform(ctx, bucket.Platform)
}

func (s *SchedulerSnapshotService) bucketFor(groupID *int64, platform string, mode string) SchedulerBucket {
	return SchedulerBucket{
		GroupID:  s.normalizeGroupID(groupID),
		Platform: platform,
		Mode:     mode,
	}
}

func (s *SchedulerSnapshotService) normalizeGroupID(groupID *int64) int64 {
	if s.isRunModeSimple() {
		return 0
	}
	if groupID == nil || *groupID <= 0 {
		return 0
	}
	return *groupID
}

func (s *SchedulerSnapshotService) normalizeGroupIDs(groupIDs []int64) []int64 {
	if s.isRunModeSimple() {
		return []int64{0}
	}
	if len(groupIDs) == 0 {
		return []int64{0}
	}
	seen := make(map[int64]struct{}, len(groupIDs))
	out := make([]int64, 0, len(groupIDs))
	for _, id := range groupIDs {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	if len(out) == 0 {
		return []int64{0}
	}
	return out
}

func (s *SchedulerSnapshotService) resolveMode(platform string, hasForcePlatform bool) string {
	if hasForcePlatform {
		return SchedulerModeForced
	}
	if platform == PlatformAnthropic || platform == PlatformGemini {
		return SchedulerModeMixed
	}
	return SchedulerModeSingle
}

func (s *SchedulerSnapshotService) guardFallback(ctx context.Context) error {
	if s.cfg == nil || s.cfg.Gateway.Scheduling.DbFallbackEnabled {
		if s.fallbackLimit == nil || s.fallbackLimit.Allow() {
			return nil
		}
		logger.LegacyPrintf("service.scheduler_snapshot", "[Scheduler] db fallback limited; cooldown=%s", fallbackCooldownDuration)
		return ErrSchedulerFallbackLimited
	}
	return ErrSchedulerCacheNotReady
}

func (s *SchedulerSnapshotService) withFallbackTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if s.cfg == nil || s.cfg.Gateway.Scheduling.DbFallbackTimeoutSeconds <= 0 {
		return context.WithCancel(ctx)
	}
	timeout := time.Duration(s.cfg.Gateway.Scheduling.DbFallbackTimeoutSeconds) * time.Second
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return context.WithCancel(ctx)
		}
		if remaining < timeout {
			timeout = remaining
		}
	}
	return context.WithTimeout(ctx, timeout)
}

func (s *SchedulerSnapshotService) isRunModeSimple() bool {
	return s.cfg != nil && s.cfg.RunMode == config.RunModeSimple
}

func (s *SchedulerSnapshotService) outboxPollInterval() time.Duration {
	if s.cfg == nil {
		return time.Second
	}
	sec := s.cfg.Gateway.Scheduling.OutboxPollIntervalSeconds
	if sec <= 0 {
		return time.Second
	}
	return time.Duration(sec) * time.Second
}

func (s *SchedulerSnapshotService) fullRebuildInterval() time.Duration {
	if s.cfg == nil {
		return 0
	}
	sec := s.cfg.Gateway.Scheduling.FullRebuildIntervalSeconds
	if sec <= 0 {
		return 0
	}
	return time.Duration(sec) * time.Second
}

func (s *SchedulerSnapshotService) defaultBuckets(ctx context.Context) ([]SchedulerBucket, error) {
	buckets := make([]SchedulerBucket, 0)
	platforms := []string{PlatformAnthropic, PlatformGemini, PlatformOpenAI, PlatformAntigravity}
	for _, platform := range platforms {
		buckets = append(buckets, SchedulerBucket{GroupID: 0, Platform: platform, Mode: SchedulerModeSingle})
		buckets = append(buckets, SchedulerBucket{GroupID: 0, Platform: platform, Mode: SchedulerModeForced})
		if platform == PlatformAnthropic || platform == PlatformGemini {
			buckets = append(buckets, SchedulerBucket{GroupID: 0, Platform: platform, Mode: SchedulerModeMixed})
		}
	}

	if s.isRunModeSimple() || s.groupRepo == nil {
		return dedupeBuckets(buckets), nil
	}

	groups, err := s.groupRepo.ListActive(ctx)
	if err != nil {
		return dedupeBuckets(buckets), nil
	}
	for _, group := range groups {
		if group.Platform == "" {
			continue
		}
		buckets = append(buckets, SchedulerBucket{GroupID: group.ID, Platform: group.Platform, Mode: SchedulerModeSingle})
		buckets = append(buckets, SchedulerBucket{GroupID: group.ID, Platform: group.Platform, Mode: SchedulerModeForced})
		if group.Platform == PlatformAnthropic || group.Platform == PlatformGemini {
			buckets = append(buckets, SchedulerBucket{GroupID: group.ID, Platform: group.Platform, Mode: SchedulerModeMixed})
		}
	}
	return dedupeBuckets(buckets), nil
}

func dedupeBuckets(in []SchedulerBucket) []SchedulerBucket {
	seen := make(map[string]struct{}, len(in))
	out := make([]SchedulerBucket, 0, len(in))
	for _, bucket := range in {
		key := bucket.String()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, bucket)
	}
	return out
}

func derefAccounts(accounts []*Account) []Account {
	if len(accounts) == 0 {
		return []Account{}
	}
	out := make([]Account, 0, len(accounts))
	for _, account := range accounts {
		if account == nil {
			continue
		}
		out = append(out, *account)
	}
	return out
}

func parseInt64Slice(value any) []int64 {
	switch raw := value.(type) {
	case []any:
		out := make([]int64, 0, len(raw))
		for _, item := range raw {
			if v, ok := toInt64(item); ok && v > 0 {
				out = append(out, v)
			}
		}
		return out
	case []int64:
		return stableUniqueInt64(raw)
	case []int:
		out := make([]int64, 0, len(raw))
		for _, item := range raw {
			if item > 0 {
				out = append(out, int64(item))
			}
		}
		return stableUniqueInt64(out)
	default:
		return nil
	}
}

func toInt64(value any) (int64, bool) {
	switch v := value.(type) {
	case float64:
		return int64(v), true
	case int64:
		return v, true
	case int:
		return int64(v), true
	case json.Number:
		parsed, err := strconv.ParseInt(v.String(), 10, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func resolveSchedulerAccountEventGroupIDs(account *Account, payload map[string]any) []int64 {
	groupIDs := parseInt64Slice(payloadSliceValue(payload, "group_ids"))
	if account == nil {
		return stableUniqueInt64(groupIDs)
	}
	return stableUniqueInt64(groupIDs, account.GroupIDs)
}

type fallbackLimiter struct {
	maxQPS        int
	mu            sync.Mutex
	window        time.Time
	count         int
	cooldown      time.Duration
	cooldownUntil time.Time
}

func newFallbackLimiter(maxQPS int, cooldown time.Duration) *fallbackLimiter {
	if maxQPS <= 0 && cooldown <= 0 {
		return nil
	}
	return &fallbackLimiter{
		maxQPS:   maxQPS,
		window:   time.Now(),
		cooldown: cooldown,
	}
}

func (l *fallbackLimiter) Allow() bool {
	if l == nil || (l.maxQPS <= 0 && l.cooldown <= 0) {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	if l.cooldown > 0 && now.Before(l.cooldownUntil) {
		return false
	}
	if l.maxQPS <= 0 {
		return true
	}
	if now.Sub(l.window) >= time.Second {
		l.window = now
		l.count = 0
	}
	if l.count >= l.maxQPS {
		if l.cooldown > 0 {
			l.cooldownUntil = now.Add(l.cooldown)
		}
		return false
	}
	l.count++
	return true
}
