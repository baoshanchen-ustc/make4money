package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

func TestPollOutbox_SkipsPoisonEvent(t *testing.T) {
	resetSchedulerOutboxRuntimeMetricsForTest()
	repo := &stubPoisonOutboxRepo{
		events: []SchedulerOutboxEvent{
			{ID: 11, EventType: SchedulerOutboxEventAccountChanged},
			{ID: 12, EventType: SchedulerOutboxEventAccountChanged},
		},
	}
	cache := &stubPoisonSchedulerCache{}
	svc := NewSchedulerSnapshotService(cache, repo, nil, nil, nil, nil)
	svc.outboxEventHandler = func(ctx context.Context, event SchedulerOutboxEvent) error {
		if event.ID == 11 {
			return markOutboxPoison(errors.New("poison payload"))
		}
		return nil
	}

	svc.pollOutbox()

	if cache.watermark != 12 {
		t.Fatalf("expected watermark to advance past poison event, got %d", cache.watermark)
	}
	if !repo.read {
		t.Fatalf("expected outbox repo to be drained for one poll")
	}
	metrics := SnapshotSchedulerOutboxRuntimeMetrics()
	if metrics.PoisonTotal != 1 {
		t.Fatalf("expected poison total to be 1, got %d", metrics.PoisonTotal)
	}
	if metrics.LastRedisWatermark != 12 {
		t.Fatalf("expected last redis watermark 12, got %d", metrics.LastRedisWatermark)
	}
	if metrics.LastPoison == nil || metrics.LastPoison.ID != 11 {
		t.Fatalf("expected last poison event id=11, got %#v", metrics.LastPoison)
	}
}

type stubPoisonSchedulerCache struct {
	watermark int64
	failGet   bool
	lockOK    bool
	lockSet   bool
}

func (c *stubPoisonSchedulerCache) GetSnapshot(ctx context.Context, bucket SchedulerBucket) ([]*Account, bool, error) {
	return nil, false, nil
}

func (c *stubPoisonSchedulerCache) SetSnapshot(ctx context.Context, bucket SchedulerBucket, accounts []Account) error {
	return nil
}

func (c *stubPoisonSchedulerCache) GetAccount(ctx context.Context, accountID int64) (*Account, error) {
	return nil, nil
}

func (c *stubPoisonSchedulerCache) SetAccount(ctx context.Context, account *Account) error {
	return nil
}

func (c *stubPoisonSchedulerCache) DeleteAccount(ctx context.Context, accountID int64) error {
	return nil
}

func (c *stubPoisonSchedulerCache) UpdateLastUsed(ctx context.Context, updates map[int64]time.Time) error {
	return nil
}

func (c *stubPoisonSchedulerCache) TryLockBucket(ctx context.Context, bucket SchedulerBucket, ttl time.Duration) (bool, error) {
	if c.lockSet {
		return c.lockOK, nil
	}
	return true, nil
}

func (c *stubPoisonSchedulerCache) ListBuckets(ctx context.Context) ([]SchedulerBucket, error) {
	return nil, nil
}

func (c *stubPoisonSchedulerCache) GetOutboxWatermark(ctx context.Context) (int64, error) {
	if c.failGet {
		return 0, errors.New("cache failure")
	}
	return c.watermark, nil
}

func (c *stubPoisonSchedulerCache) SetOutboxWatermark(ctx context.Context, id int64) error {
	c.watermark = id
	return nil
}

type stubPoisonOutboxRepo struct {
	events []SchedulerOutboxEvent
	read   bool
}

func (r *stubPoisonOutboxRepo) ListAfter(ctx context.Context, afterID int64, limit int) ([]SchedulerOutboxEvent, error) {
	if r.read {
		return nil, nil
	}
	r.read = true
	return r.events, nil
}

func (r *stubPoisonOutboxRepo) MaxID(ctx context.Context) (int64, error) {
	if len(r.events) == 0 {
		return 0, nil
	}
	return r.events[len(r.events)-1].ID, nil
}

type repeatingPoisonOutboxRepo struct {
	events []SchedulerOutboxEvent
	reads  int
}

func (r *repeatingPoisonOutboxRepo) ListAfter(ctx context.Context, afterID int64, limit int) ([]SchedulerOutboxEvent, error) {
	r.reads++
	return r.events, nil
}

func (r *repeatingPoisonOutboxRepo) MaxID(ctx context.Context) (int64, error) {
	if len(r.events) == 0 {
		return 0, nil
	}
	return r.events[len(r.events)-1].ID, nil
}

func TestPollOutbox_StopsOnTransientError(t *testing.T) {
	resetSchedulerOutboxRuntimeMetricsForTest()
	repo := &stubPoisonOutboxRepo{
		events: []SchedulerOutboxEvent{
			{ID: 21, EventType: SchedulerOutboxEventAccountChanged},
		},
	}
	cache := &stubPoisonSchedulerCache{}
	svc := NewSchedulerSnapshotService(cache, repo, nil, nil, nil, nil)
	svc.outboxEventHandler = func(ctx context.Context, event SchedulerOutboxEvent) error {
		return wrapOutboxRetryable(errors.New("transient fail"))
	}

	svc.pollOutbox()

	if cache.watermark != 0 {
		t.Fatalf("expected watermark to stay at 0 on transient error, got %d", cache.watermark)
	}
	if !repo.read {
		t.Fatalf("expected outbox repo to have been read once")
	}
	metrics := SnapshotSchedulerOutboxRuntimeMetrics()
	if metrics.TransientTotal != 1 {
		t.Fatalf("expected transient total to be 1, got %d", metrics.TransientTotal)
	}
	if metrics.LastTransient == nil || metrics.LastTransient.ID != 21 {
		t.Fatalf("expected last transient event id=21, got %#v", metrics.LastTransient)
	}
}

func TestPollOutbox_AdvancesWatermarkPastSuccessfulEventsBeforeTransient(t *testing.T) {
	resetSchedulerOutboxRuntimeMetricsForTest()
	repo := &stubPoisonOutboxRepo{
		events: []SchedulerOutboxEvent{
			{ID: 91, EventType: SchedulerOutboxEventAccountLastUsed, Payload: map[string]any{"last_used": map[string]any{"1": int64(1700000000)}}},
			{ID: 92, EventType: SchedulerOutboxEventAccountChanged},
		},
	}
	cache := &stubPoisonSchedulerCache{}
	svc := NewSchedulerSnapshotService(cache, repo, nil, nil, nil, nil)
	svc.outboxEventHandler = func(ctx context.Context, event SchedulerOutboxEvent) error {
		if event.ID == 92 {
			return wrapOutboxRetryable(errors.New("lock contention while rebuilding bucket"))
		}
		return nil
	}

	svc.pollOutbox()

	if cache.watermark != 91 {
		t.Fatalf("expected watermark to advance to last successful event, got %d", cache.watermark)
	}
	metrics := SnapshotSchedulerOutboxRuntimeMetrics()
	if metrics.TransientTotal != 1 {
		t.Fatalf("expected transient total to be 1, got %d", metrics.TransientTotal)
	}
}

func TestPollOutbox_CoalescesAdjacentAccountChangeEvents(t *testing.T) {
	resetSchedulerOutboxRuntimeMetricsForTest()
	accountID := int64(42)
	repo := &stubPoisonOutboxRepo{
		events: []SchedulerOutboxEvent{
			{ID: 101, EventType: SchedulerOutboxEventAccountChanged, AccountID: &accountID, Payload: map[string]any{"group_ids": []any{int64(1)}}},
			{ID: 102, EventType: SchedulerOutboxEventAccountChanged, AccountID: &accountID, Payload: map[string]any{"group_ids": []any{int64(2)}}},
		},
	}
	cache := &stubPoisonSchedulerCache{}
	svc := NewSchedulerSnapshotService(cache, repo, nil, nil, nil, nil)
	calls := 0
	var got SchedulerOutboxEvent
	svc.outboxEventHandler = func(ctx context.Context, event SchedulerOutboxEvent) error {
		calls++
		got = event
		return nil
	}

	svc.pollOutbox()

	if calls != 1 {
		t.Fatalf("expected adjacent account changes to be coalesced into one handler call, got %d", calls)
	}
	if cache.watermark != 102 {
		t.Fatalf("expected watermark 102 after coalesced success, got %d", cache.watermark)
	}
	if got.ID != 102 {
		t.Fatalf("expected merged event id 102, got %d", got.ID)
	}
	if groups := parseInt64Slice(got.Payload["group_ids"]); len(groups) != 2 || groups[0] != 1 || groups[1] != 2 {
		t.Fatalf("expected merged group ids [1 2], got %#v", got.Payload["group_ids"])
	}
	metrics := SnapshotSchedulerOutboxRuntimeMetrics()
	if metrics.CoalescedBatchTotal != 1 {
		t.Fatalf("expected coalesced batch total 1, got %d", metrics.CoalescedBatchTotal)
	}
	if metrics.CoalescedEventSavedTotal != 1 {
		t.Fatalf("expected coalesced event saved total 1, got %d", metrics.CoalescedEventSavedTotal)
	}
}

func TestPollOutbox_DoesNotCoalesceNonAdjacentEvents(t *testing.T) {
	resetSchedulerOutboxRuntimeMetricsForTest()
	accountID := int64(52)
	groupID := int64(7)
	repo := &stubPoisonOutboxRepo{
		events: []SchedulerOutboxEvent{
			{ID: 111, EventType: SchedulerOutboxEventAccountChanged, AccountID: &accountID},
			{ID: 112, EventType: SchedulerOutboxEventGroupChanged, GroupID: &groupID},
			{ID: 113, EventType: SchedulerOutboxEventAccountChanged, AccountID: &accountID},
		},
	}
	cache := &stubPoisonSchedulerCache{}
	svc := NewSchedulerSnapshotService(cache, repo, nil, nil, nil, nil)
	calls := 0
	svc.outboxEventHandler = func(ctx context.Context, event SchedulerOutboxEvent) error {
		calls++
		return nil
	}

	svc.pollOutbox()

	if calls != 3 {
		t.Fatalf("expected non-adjacent events to stay separate, got %d handler calls", calls)
	}
}

func TestResolveSchedulerAccountEventGroupIDs_MergesPayloadAndCurrentGroups(t *testing.T) {
	account := &Account{GroupIDs: []int64{2, 3}}
	payload := map[string]any{"group_ids": []int64{1, 2}}

	got := resolveSchedulerAccountEventGroupIDs(account, payload)

	if len(got) != 3 || got[0] != 1 || got[1] != 2 || got[2] != 3 {
		t.Fatalf("expected merged group ids [1 2 3], got %#v", got)
	}
}

func TestParseInt64Slice_AcceptsInt64Slices(t *testing.T) {
	got := parseInt64Slice([]int64{3, 3, 7, -1})

	if len(got) != 2 || got[0] != 3 || got[1] != 7 {
		t.Fatalf("expected parsed ids [3 7], got %#v", got)
	}
}

func TestSnapshotSchedulerOutboxRuntimeMetricsSummaries(t *testing.T) {
	resetSchedulerOutboxRuntimeMetricsForTest()
	schedulerOutboxRuntimeMu.Lock()
	schedulerOutboxBlockedEvent = &SchedulerOutboxBlockedEvent{
		ID:        101,
		EventType: SchedulerOutboxEventAccountChanged,
		Reason:    "lock_contention",
		Attempts:  2,
	}
	schedulerOutboxRuntimeMu.Unlock()
	schedulerOutboxCheckpointFallbackStreak.Store(2)
	schedulerOutboxCheckpointLastFallbackReason = "redis_down"
	schedulerOutboxLagFailureStreak.Store(3)
	schedulerOutboxLagSeconds.Store(45)
	schedulerOutboxLagRebuildTotal.Store(1)
	schedulerOutboxBucketRebuildSuccessTotal.Store(1)
	schedulerOutboxBucketRebuildFailureTotal.Store(1)
	schedulerOutboxBucketRebuildLockContention.Store(1)
	schedulerOutboxLastBucketRebuildStatus = "lock_contention"
	schedulerOutboxLastBucketRebuildReason = "busy"
	metrics := SnapshotSchedulerOutboxRuntimeMetrics()
	expectedBlocked := "blocked id=101 | reason=lock_contention | attempts=2 | age=0s"
	if metrics.BlockedEventSummary != expectedBlocked {
		t.Fatalf("blocked summary mismatch: %q", metrics.BlockedEventSummary)
	}
	if metrics.CheckpointFallbackSummary != "streak=2 reason=redis_down" {
		t.Fatalf("checkpoint summary mismatch: %q", metrics.CheckpointFallbackSummary)
	}
	if metrics.LagStreakSummary != "streak=3 lag=45s rebuilds=1" {
		t.Fatalf("lag summary mismatch: %q", metrics.LagStreakSummary)
	}
	if metrics.RebuildContentionSummary != "success=1 fail=1 contention=1 status=lock_contention reason=busy" {
		t.Fatalf("rebuild summary mismatch: %q", metrics.RebuildContentionSummary)
	}
	if metrics.DriftTrendStatus != "degrading" {
		t.Fatalf("expected drift status degrading, got %q", metrics.DriftTrendStatus)
	}
	if metrics.DriftTrendDetail != "lag or checkpoint fallback persisting" {
		t.Fatalf("expected fallback detail, got %q", metrics.DriftTrendDetail)
	}
	if metrics.DriftTrendNarrative != "lag/checkpoint fallback persisting—stacked rebuild/retry" {
		t.Fatalf("unexpected drift narrative: %q", metrics.DriftTrendNarrative)
	}
}

func TestPollOutbox_SkipsPayloadDecodeError(t *testing.T) {
	resetSchedulerOutboxRuntimeMetricsForTest()
	repo := &stubPoisonOutboxRepo{
		events: []SchedulerOutboxEvent{
			{ID: 31, EventType: SchedulerOutboxEventAccountChanged, PayloadDecodeError: "invalid payload"},
		},
	}
	cache := &stubPoisonSchedulerCache{}
	svc := NewSchedulerSnapshotService(cache, repo, nil, nil, nil, nil)

	svc.pollOutbox()

	if cache.watermark != 31 {
		t.Fatalf("expected watermark to advance past event with decode error, got %d", cache.watermark)
	}
	metrics := SnapshotSchedulerOutboxRuntimeMetrics()
	if metrics.PoisonTotal != 1 || metrics.PayloadDecodePoisonTotal != 1 {
		t.Fatalf("expected poison totals to be 1/1, got poison=%d decode=%d", metrics.PoisonTotal, metrics.PayloadDecodePoisonTotal)
	}
	if metrics.LastPoison == nil || metrics.LastPoison.PayloadDecodeError != "invalid payload" {
		t.Fatalf("expected payload decode poison details, got %#v", metrics.LastPoison)
	}
	if metrics.LastPoison.Reason != "payload_decode" {
		t.Fatalf("expected payload_decode reason, got %#v", metrics.LastPoison)
	}
}

func TestPollOutbox_FallsBackToCheckpoint(t *testing.T) {
	resetSchedulerOutboxRuntimeMetricsForTest()
	repo := &stubPoisonOutboxRepo{
		events: []SchedulerOutboxEvent{
			{ID: 121, EventType: SchedulerOutboxEventAccountChanged},
		},
	}
	cache := &stubPoisonSchedulerCache{failGet: true}
	checkpoint := &stubSchedulerCheckpointRepo{watermark: 88}
	svc := NewSchedulerSnapshotService(cache, repo, nil, nil, checkpoint, nil)
	svc.pollOutbox()
	if cache.watermark != 121 {
		t.Fatalf("expected cache watermark to advance after checkpoint fallback, got %d", cache.watermark)
	}
	if !repo.read {
		t.Fatalf("expected repo to be drained when checkpoint fallback used")
	}
	if checkpoint.setWatermark != 121 {
		t.Fatalf("expected checkpoint to persist new watermark, got %d", checkpoint.setWatermark)
	}
	metrics := SnapshotSchedulerOutboxRuntimeMetrics()
	if metrics.CheckpointFallbackTotal != 1 {
		t.Fatalf("expected checkpoint fallback total 1, got %d", metrics.CheckpointFallbackTotal)
	}
	if metrics.LastCheckpointWatermark != 121 {
		t.Fatalf("expected last checkpoint watermark 121, got %d", metrics.LastCheckpointWatermark)
	}
}

func TestPollOutbox_ClassifiesUnknownEventPoison(t *testing.T) {
	resetSchedulerOutboxRuntimeMetricsForTest()
	repo := &stubPoisonOutboxRepo{
		events: []SchedulerOutboxEvent{
			{ID: 41, EventType: "unknown.event.type"},
		},
	}
	cache := &stubPoisonSchedulerCache{}
	svc := NewSchedulerSnapshotService(cache, repo, nil, nil, nil, nil)

	svc.pollOutbox()

	metrics := SnapshotSchedulerOutboxRuntimeMetrics()
	if metrics.UnknownEventPoisonTotal != 1 {
		t.Fatalf("expected unknown event poison total 1, got %d", metrics.UnknownEventPoisonTotal)
	}
	if metrics.LastPoison == nil || metrics.LastPoison.Reason != "unknown_event" {
		t.Fatalf("expected unknown_event poison reason, got %#v", metrics.LastPoison)
	}
	if cache.watermark != 41 {
		t.Fatalf("expected watermark to advance past unknown poison event, got %d", cache.watermark)
	}
}

func TestPollOutbox_ClassifiesMalformedPayloadPoison(t *testing.T) {
	resetSchedulerOutboxRuntimeMetricsForTest()
	repo := &stubPoisonOutboxRepo{
		events: []SchedulerOutboxEvent{
			{ID: 51, EventType: SchedulerOutboxEventAccountChanged},
		},
	}
	cache := &stubPoisonSchedulerCache{}
	svc := NewSchedulerSnapshotService(cache, repo, nil, nil, nil, nil)

	svc.pollOutbox()

	metrics := SnapshotSchedulerOutboxRuntimeMetrics()
	if metrics.MalformedPayloadPoisonTotal != 1 {
		t.Fatalf("expected malformed payload poison total 1, got %d", metrics.MalformedPayloadPoisonTotal)
	}
	if metrics.LastPoison == nil || metrics.LastPoison.Reason != "malformed_payload" {
		t.Fatalf("expected malformed_payload reason, got %#v", metrics.LastPoison)
	}
	if cache.watermark != 51 {
		t.Fatalf("expected watermark to advance past malformed payload poison event, got %d", cache.watermark)
	}
}

func TestPollOutbox_ClassifiesLockContentionTransient(t *testing.T) {
	resetSchedulerOutboxRuntimeMetricsForTest()
	repo := &stubPoisonOutboxRepo{
		events: []SchedulerOutboxEvent{
			{ID: 61, EventType: SchedulerOutboxEventAccountChanged, AccountID: poisonInt64Ptr(10)},
		},
	}
	cache := &stubPoisonSchedulerCache{}
	svc := NewSchedulerSnapshotService(cache, repo, nil, nil, nil, nil)
	svc.outboxEventHandler = func(ctx context.Context, event SchedulerOutboxEvent) error {
		return wrapOutboxRetryable(errors.New("lock contention while rebuilding bucket"))
	}

	svc.pollOutbox()

	metrics := SnapshotSchedulerOutboxRuntimeMetrics()
	if metrics.LockContentionTransientTotal != 1 {
		t.Fatalf("expected lock contention transient total 1, got %d", metrics.LockContentionTransientTotal)
	}
	if metrics.LastTransient == nil || metrics.LastTransient.Reason != "lock_contention" {
		t.Fatalf("expected lock_contention transient reason, got %#v", metrics.LastTransient)
	}
	if metrics.BlockedEvent == nil || metrics.BlockedEvent.ID != 61 {
		t.Fatalf("expected blocked event id=61, got %#v", metrics.BlockedEvent)
	}
	if metrics.BlockedEvent.Reason != "lock_contention" {
		t.Fatalf("expected blocked event reason lock_contention, got %#v", metrics.BlockedEvent)
	}
	if metrics.BlockedEvent.Attempts != 1 {
		t.Fatalf("expected blocked event attempts 1, got %#v", metrics.BlockedEvent)
	}
}

func TestPollOutbox_LockContentionCooldownSuppressesImmediateRetry(t *testing.T) {
	resetSchedulerOutboxRuntimeMetricsForTest()
	repo := &repeatingPoisonOutboxRepo{
		events: []SchedulerOutboxEvent{
			{ID: 71, EventType: SchedulerOutboxEventAccountChanged},
		},
	}
	cache := &stubPoisonSchedulerCache{}
	svc := NewSchedulerSnapshotService(cache, repo, nil, nil, nil, nil)
	calls := 0
	svc.outboxEventHandler = func(ctx context.Context, event SchedulerOutboxEvent) error {
		calls++
		return wrapOutboxRetryable(errors.New("lock contention while rebuilding bucket"))
	}

	svc.pollOutbox()
	svc.pollOutbox()

	if calls != 1 {
		t.Fatalf("expected handler to be called once during cooldown window, got %d", calls)
	}
	if repo.reads != 2 {
		t.Fatalf("expected repo to be polled twice, got %d", repo.reads)
	}
	metrics := SnapshotSchedulerOutboxRuntimeMetrics()
	if metrics.TransientTotal != 1 {
		t.Fatalf("expected transient total to stay at 1 during cooldown, got %d", metrics.TransientTotal)
	}
}

func TestPollOutbox_TracksLagAndBacklogRuntimeMetrics(t *testing.T) {
	resetSchedulerOutboxRuntimeMetricsForTest()
	cfg := &config.Config{}
	cfg.Gateway.Scheduling.OutboxLagWarnSeconds = 1
	cfg.Gateway.Scheduling.OutboxBacklogRebuildRows = 100

	repo := &repeatingPoisonOutboxRepo{
		events: []SchedulerOutboxEvent{
			{ID: 81, EventType: SchedulerOutboxEventAccountChanged, CreatedAt: time.Now().Add(-5 * time.Second)},
		},
	}
	cache := &stubPoisonSchedulerCache{watermark: 70}
	svc := NewSchedulerSnapshotService(cache, repo, nil, nil, nil, cfg)
	svc.outboxEventHandler = func(ctx context.Context, event SchedulerOutboxEvent) error {
		return wrapOutboxRetryable(errors.New("db load failed: timeout"))
	}

	svc.pollOutbox()

	metrics := SnapshotSchedulerOutboxRuntimeMetrics()
	if metrics.BacklogRows != 11 {
		t.Fatalf("expected backlog rows 11, got %d", metrics.BacklogRows)
	}
	if metrics.LagSeconds < 1 {
		t.Fatalf("expected lag seconds >= 1, got %d", metrics.LagSeconds)
	}
}

func TestPollOutbox_ClearsBlockedEventLifecycle(t *testing.T) {
	resetSchedulerOutboxRuntimeMetricsForTest()
	repo := &repeatingPoisonOutboxRepo{
		events: []SchedulerOutboxEvent{
			{ID: 181, EventType: SchedulerOutboxEventAccountChanged},
		},
	}
	cache := &stubPoisonSchedulerCache{}
	svc := NewSchedulerSnapshotService(cache, repo, nil, nil, nil, nil)
	calls := 0
	svc.outboxEventHandler = func(ctx context.Context, event SchedulerOutboxEvent) error {
		calls++
		if calls == 1 {
			return wrapOutboxRetryable(errors.New("lock contention while rebuilding bucket"))
		}
		return nil
	}

	svc.pollOutbox()
	svc.clearOutboxRetryState(181)
	svc.pollOutbox()

	metrics := SnapshotSchedulerOutboxRuntimeMetrics()
	if metrics.BlockedEvent != nil {
		t.Fatalf("expected blocked event to be cleared after recovery, got %#v", metrics.BlockedEvent)
	}
	if metrics.BlockedEventClearTotal != 1 {
		t.Fatalf("expected blocked event clear total 1, got %d", metrics.BlockedEventClearTotal)
	}
	if metrics.BlockedEventLastClearedID != 181 {
		t.Fatalf("expected cleared blocked event id 181, got %d", metrics.BlockedEventLastClearedID)
	}
	if metrics.BlockedEventLastClearReason != "recovered" {
		t.Fatalf("expected blocked event clear reason recovered, got %q", metrics.BlockedEventLastClearReason)
	}
	if metrics.BlockedEventLastClearedAt == "" {
		t.Fatal("expected blocked event last cleared at to be recorded")
	}
}

func TestPollOutbox_TracksCheckpointTimestamps(t *testing.T) {
	resetSchedulerOutboxRuntimeMetricsForTest()

	readFailSvc := NewSchedulerSnapshotService(nil, nil, nil, nil, &stubSchedulerCheckpointRepo{failRead: true}, nil)
	_, _ = readFailSvc.loadCheckpointWatermark(context.Background())

	writeFailSvc := NewSchedulerSnapshotService(nil, nil, nil, nil, &stubSchedulerCheckpointRepo{failWrite: true}, nil)
	writeFailSvc.persistCheckpointWatermark(context.Background(), 77)

	fallbackSvc := NewSchedulerSnapshotService(nil, nil, nil, nil, &stubSchedulerCheckpointRepo{watermark: 66}, nil)
	_, _ = fallbackSvc.loadCheckpointWatermark(context.Background())

	metrics := SnapshotSchedulerOutboxRuntimeMetrics()
	if metrics.CheckpointLastReadFailureAt == "" {
		t.Fatal("expected checkpoint read failure timestamp")
	}
	if metrics.CheckpointLastWriteFailureAt == "" {
		t.Fatal("expected checkpoint write failure timestamp")
	}
	if metrics.CheckpointLastFallbackAt == "" {
		t.Fatal("expected checkpoint fallback timestamp")
	}
	if metrics.CheckpointFallbackStreak != 1 {
		t.Fatalf("expected checkpoint fallback streak 1, got %d", metrics.CheckpointFallbackStreak)
	}
	if metrics.CheckpointLastFallbackReason != "redis_watermark_unavailable" {
		t.Fatalf("expected checkpoint fallback reason redis_watermark_unavailable, got %q", metrics.CheckpointLastFallbackReason)
	}
}

func TestPollOutbox_UsesFreshCommitContextAfterSlowEventHandling(t *testing.T) {
	resetSchedulerOutboxRuntimeMetricsForTest()
	repo := &stubPoisonOutboxRepo{
		events: []SchedulerOutboxEvent{
			{ID: 171, EventType: SchedulerOutboxEventAccountChanged},
		},
	}
	cache := &timeoutAwareSchedulerCache{}
	svc := NewSchedulerSnapshotService(cache, repo, nil, nil, nil, nil)
	svc.outboxPollTimeout = 5 * time.Millisecond
	svc.outboxCommitTimeout = 50 * time.Millisecond
	svc.outboxEventHandler = func(ctx context.Context, event SchedulerOutboxEvent) error {
		time.Sleep(20 * time.Millisecond)
		return nil
	}

	svc.pollOutbox()

	if cache.watermark != 171 {
		t.Fatalf("expected watermark to advance with fresh commit context, got %d", cache.watermark)
	}
	if cache.setErr != nil {
		t.Fatalf("expected watermark write to use a fresh context, got %v", cache.setErr)
	}
}

func TestPollOutbox_TracksLagRebuildLifecycle(t *testing.T) {
	resetSchedulerOutboxRuntimeMetricsForTest()
	cfg := &config.Config{}
	cfg.Gateway.Scheduling.OutboxLagWarnSeconds = 1
	cfg.Gateway.Scheduling.OutboxLagRebuildSeconds = 1
	cfg.Gateway.Scheduling.OutboxLagRebuildFailures = 2

	repo := &repeatingPoisonOutboxRepo{
		events: []SchedulerOutboxEvent{
			{ID: 191, EventType: SchedulerOutboxEventAccountChanged, CreatedAt: time.Now().Add(-5 * time.Second)},
		},
	}
	cache := &stubPoisonSchedulerCache{}
	svc := NewSchedulerSnapshotService(cache, repo, nil, nil, nil, cfg)
	svc.outboxEventHandler = func(ctx context.Context, event SchedulerOutboxEvent) error {
		return wrapOutboxRetryable(errors.New("db load failed: timeout"))
	}

	svc.pollOutbox()
	metrics := SnapshotSchedulerOutboxRuntimeMetrics()
	if metrics.LagFailureStreak != 1 {
		t.Fatalf("expected lag failure streak 1 after first poll, got %d", metrics.LagFailureStreak)
	}

	svc.pollOutbox()
	metrics = SnapshotSchedulerOutboxRuntimeMetrics()
	if metrics.LagRebuildTotal != 1 {
		t.Fatalf("expected lag rebuild total 1, got %d", metrics.LagRebuildTotal)
	}
	if metrics.LastLagRebuildAt == "" {
		t.Fatal("expected last lag rebuild timestamp")
	}
	if metrics.LagFailureStreak != 0 {
		t.Fatalf("expected lag failure streak reset after rebuild trigger, got %d", metrics.LagFailureStreak)
	}
}

func TestPollOutbox_LagRebuildCooldownSuppressesRepeatedTriggers(t *testing.T) {
	resetSchedulerOutboxRuntimeMetricsForTest()
	cfg := &config.Config{}
	cfg.Gateway.Scheduling.OutboxLagWarnSeconds = 1
	cfg.Gateway.Scheduling.OutboxLagRebuildSeconds = 1
	cfg.Gateway.Scheduling.OutboxLagRebuildFailures = 1
	cfg.Gateway.Scheduling.OutboxRebuildCooldownSeconds = 60

	repo := &repeatingPoisonOutboxRepo{
		events: []SchedulerOutboxEvent{
			{ID: 291, EventType: SchedulerOutboxEventAccountChanged, CreatedAt: time.Now().Add(-5 * time.Second)},
		},
	}
	cache := &stubPoisonSchedulerCache{}
	svc := NewSchedulerSnapshotService(cache, repo, nil, nil, nil, cfg)
	svc.outboxEventHandler = func(ctx context.Context, event SchedulerOutboxEvent) error {
		return wrapOutboxRetryable(errors.New("db load failed: timeout"))
	}

	svc.pollOutbox()
	svc.pollOutbox()

	metrics := SnapshotSchedulerOutboxRuntimeMetrics()
	if metrics.LagRebuildTotal != 1 {
		t.Fatalf("expected lag rebuild total to stay at 1 during cooldown, got %d", metrics.LagRebuildTotal)
	}
	if metrics.RebuildCooldownSkipTotal != 1 {
		t.Fatalf("expected rebuild cooldown skip total 1, got %d", metrics.RebuildCooldownSkipTotal)
	}
}

func TestPollOutbox_BacklogRebuildCooldownSuppressesRepeatedTriggers(t *testing.T) {
	resetSchedulerOutboxRuntimeMetricsForTest()
	cfg := &config.Config{}
	cfg.Gateway.Scheduling.OutboxBacklogRebuildRows = 1
	cfg.Gateway.Scheduling.OutboxRebuildCooldownSeconds = 60

	repo := &repeatingPoisonOutboxRepo{
		events: []SchedulerOutboxEvent{
			{ID: 391, EventType: SchedulerOutboxEventAccountChanged, CreatedAt: time.Now().Add(-5 * time.Second)},
		},
	}
	cache := &stubPoisonSchedulerCache{}
	svc := NewSchedulerSnapshotService(cache, repo, nil, nil, nil, cfg)
	svc.outboxEventHandler = func(ctx context.Context, event SchedulerOutboxEvent) error {
		return wrapOutboxRetryable(errors.New("db load failed: timeout"))
	}

	svc.pollOutbox()
	svc.pollOutbox()

	metrics := SnapshotSchedulerOutboxRuntimeMetrics()
	if metrics.BacklogRebuildTotal != 1 {
		t.Fatalf("expected backlog rebuild total to stay at 1 during cooldown, got %d", metrics.BacklogRebuildTotal)
	}
	if metrics.RebuildCooldownSkipTotal != 1 {
		t.Fatalf("expected rebuild cooldown skip total 1, got %d", metrics.RebuildCooldownSkipTotal)
	}
}

func TestTriggerFullRebuild_SkipsConcurrentRebuilds(t *testing.T) {
	cache := &blockingBucketListSchedulerCache{
		replaceStarted: make(chan struct{}),
		releaseReplace: make(chan struct{}),
	}
	svc := NewSchedulerSnapshotService(cache, nil, nil, nil, nil, nil)

	firstErrCh := make(chan error, 1)
	go func() {
		firstErrCh <- svc.triggerFullRebuild("interval")
	}()

	select {
	case <-cache.replaceStarted:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected first rebuild to start syncing bucket registry")
	}

	skippedErrCh := make(chan error, 1)
	go func() {
		skippedErrCh <- svc.triggerFullRebuild("outbox_lag")
	}()

	select {
	case err := <-skippedErrCh:
		if err != nil {
			t.Fatalf("expected concurrent rebuild to be skipped without error, got %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected concurrent rebuild to return immediately")
	}

	close(cache.releaseReplace)
	if err := <-firstErrCh; err == nil {
		t.Fatal("expected first rebuild to return cache list failure")
	}
	if cache.replaceCalls != 1 {
		t.Fatalf("expected only one registry sync call, got %d", cache.replaceCalls)
	}
}

func TestRebuildBuckets_IgnoresLockContentionForFullRebuildSummary(t *testing.T) {
	resetSchedulerOutboxRuntimeMetricsForTest()
	cache := &stubPoisonSchedulerCache{lockSet: true, lockOK: false}
	svc := NewSchedulerSnapshotService(cache, nil, nil, nil, nil, nil)

	err := svc.rebuildBuckets(context.Background(), []SchedulerBucket{
		{GroupID: 1, Platform: PlatformOpenAI, Mode: SchedulerModeSingle},
		{GroupID: 2, Platform: PlatformOpenAI, Mode: SchedulerModeSingle},
	}, "interval")
	if err != nil {
		t.Fatalf("expected full rebuild summary to ignore pure lock contention, got %v", err)
	}

	metrics := SnapshotSchedulerOutboxRuntimeMetrics()
	if metrics.BucketRebuildLockContention != 2 {
		t.Fatalf("expected bucket rebuild lock contention total 2, got %d", metrics.BucketRebuildLockContention)
	}
	if metrics.BusyBucketSkipTotal != 2 {
		t.Fatalf("expected busy bucket skip total 2, got %d", metrics.BusyBucketSkipTotal)
	}
}

func TestRebuildBuckets_StillReturnsNonLockFailures(t *testing.T) {
	resetSchedulerOutboxRuntimeMetricsForTest()
	cache := &stubPoisonSchedulerCache{}
	svc := NewSchedulerSnapshotService(cache, nil, nil, nil, nil, nil)

	err := svc.rebuildBuckets(context.Background(), []SchedulerBucket{
		{GroupID: 1, Platform: PlatformOpenAI, Mode: SchedulerModeSingle},
	}, "interval")
	if err == nil {
		t.Fatal("expected non-lock rebuild failure to still be returned")
	}
}

func TestRebuildBucket_TracksLockContentionAndRecoverySignals(t *testing.T) {
	resetSchedulerOutboxRuntimeMetricsForTest()
	cache := &stubPoisonSchedulerCache{lockSet: true, lockOK: false}
	svc := NewSchedulerSnapshotService(cache, nil, nil, nil, nil, nil)
	bucket := SchedulerBucket{GroupID: 1, Platform: PlatformOpenAI, Mode: SchedulerModeSingle}

	err := svc.rebuildBucket(context.Background(), bucket, "unit_test")
	if err == nil {
		t.Fatal("expected lock contention error")
	}

	metrics := SnapshotSchedulerOutboxRuntimeMetrics()
	if metrics.BucketRebuildFailureTotal != 1 {
		t.Fatalf("expected bucket rebuild failure total 1, got %d", metrics.BucketRebuildFailureTotal)
	}
	if metrics.BucketRebuildLockContention != 1 {
		t.Fatalf("expected bucket rebuild lock contention total 1, got %d", metrics.BucketRebuildLockContention)
	}
	if metrics.LastBucketRebuildStatus != "lock_contention" {
		t.Fatalf("expected last bucket rebuild status lock_contention, got %q", metrics.LastBucketRebuildStatus)
	}
	if metrics.LastBucketRebuildBucket != bucket.String() {
		t.Fatalf("expected last bucket rebuild bucket %s, got %q", bucket.String(), metrics.LastBucketRebuildBucket)
	}
}

func TestClassifySchedulerOutboxTransientReason(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{name: "db", err: wrapOutboxRetryable(errors.New("db load failed: timeout")), want: "db"},
		{name: "cache", err: wrapOutboxRetryable(errors.New("cache write failed: redis down")), want: "cache"},
		{name: "lock", err: wrapOutboxRetryable(errors.New("lock contention while rebuilding bucket")), want: "lock_contention"},
		{name: "other", err: wrapOutboxRetryable(errors.New("upstream retryable")), want: "other"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := classifySchedulerOutboxTransientReason(tc.err)
			if got != tc.want {
				t.Fatalf("expected %s, got %s", tc.want, got)
			}
		})
	}
}

func poisonInt64Ptr(v int64) *int64 {
	return &v
}

func TestGuardFallbackCooldown(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.Scheduling.DbFallbackEnabled = true
	cfg.Gateway.Scheduling.DbFallbackMaxQPS = 1
	svc := NewSchedulerSnapshotService(nil, nil, nil, nil, nil, cfg)

	if err := svc.guardFallback(context.Background()); err != nil {
		t.Fatalf("unexpected first fallback guard error: %v", err)
	}
	if err := svc.guardFallback(context.Background()); !errors.Is(err, ErrSchedulerFallbackLimited) {
		t.Fatalf("expected fallback limiter to block second call, got %v", err)
	}
}

func TestLoadCheckpointWatermark_ReadFailureTracked(t *testing.T) {
	resetSchedulerOutboxRuntimeMetricsForTest()
	svc := NewSchedulerSnapshotService(nil, nil, nil, nil, &stubSchedulerCheckpointRepo{failRead: true}, nil)

	_, err := svc.loadCheckpointWatermark(context.Background())
	if err == nil {
		t.Fatal("expected checkpoint read error")
	}

	metrics := SnapshotSchedulerOutboxRuntimeMetrics()
	if metrics.CheckpointReadFailureTotal != 1 {
		t.Fatalf("expected checkpoint read failure total 1, got %d", metrics.CheckpointReadFailureTotal)
	}
	if metrics.CheckpointFallbackTotal != 0 {
		t.Fatalf("expected checkpoint fallback total 0 after read failure, got %d", metrics.CheckpointFallbackTotal)
	}
}

func TestRecordSchedulerOutboxRedisWatermark_ResetsFallbackStreak(t *testing.T) {
	resetSchedulerOutboxRuntimeMetricsForTest()
	schedulerOutboxCheckpointFallbackStreak.Store(3)

	recordSchedulerOutboxRedisWatermark(88)

	metrics := SnapshotSchedulerOutboxRuntimeMetrics()
	if metrics.CheckpointFallbackStreak != 0 {
		t.Fatalf("expected checkpoint fallback streak reset to 0, got %d", metrics.CheckpointFallbackStreak)
	}
	if metrics.LastRedisWatermark != 88 {
		t.Fatalf("expected last redis watermark 88, got %d", metrics.LastRedisWatermark)
	}
}

func TestPersistCheckpointWatermark_WriteFailureTracked(t *testing.T) {
	resetSchedulerOutboxRuntimeMetricsForTest()
	svc := NewSchedulerSnapshotService(nil, nil, nil, nil, &stubSchedulerCheckpointRepo{failWrite: true}, nil)

	svc.persistCheckpointWatermark(context.Background(), 99)

	metrics := SnapshotSchedulerOutboxRuntimeMetrics()
	if metrics.CheckpointWriteFailureTotal != 1 {
		t.Fatalf("expected checkpoint write failure total 1, got %d", metrics.CheckpointWriteFailureTotal)
	}
	if metrics.LastCheckpointWatermark != 0 {
		t.Fatalf("expected last checkpoint watermark unchanged on write failure, got %d", metrics.LastCheckpointWatermark)
	}
}

type stubSchedulerCheckpointRepo struct {
	watermark    int64
	setWatermark int64
	failRead     bool
	failWrite    bool
}

func (r *stubSchedulerCheckpointRepo) GetCheckpointWatermark(ctx context.Context) (int64, error) {
	if r.failRead {
		return 0, errors.New("checkpoint read fail")
	}
	return r.watermark, nil
}

func (r *stubSchedulerCheckpointRepo) SetCheckpointWatermark(ctx context.Context, watermark int64) error {
	if r.failWrite {
		return errors.New("checkpoint write fail")
	}
	r.setWatermark = watermark
	return nil
}

type timeoutAwareSchedulerCache struct {
	stubPoisonSchedulerCache
	setErr error
}

func (c *timeoutAwareSchedulerCache) SetOutboxWatermark(ctx context.Context, id int64) error {
	if err := ctx.Err(); err != nil {
		c.setErr = err
		return err
	}
	c.watermark = id
	return nil
}

type blockingBucketListSchedulerCache struct {
	stubPoisonSchedulerCache
	replaceStarted chan struct{}
	releaseReplace chan struct{}
	replaceCalls   int
}

func (c *blockingBucketListSchedulerCache) ReplaceBuckets(ctx context.Context, buckets []SchedulerBucket) error {
	c.replaceCalls++
	if c.replaceCalls == 1 {
		close(c.replaceStarted)
	}
	<-c.releaseReplace
	return errors.New("bucket registry sync failed")
}
