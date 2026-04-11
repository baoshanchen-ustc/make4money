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
	if metrics.LastPoison == nil || metrics.LastPoison.ID != 11 {
		t.Fatalf("expected last poison event id=11, got %#v", metrics.LastPoison)
	}
}

type stubPoisonSchedulerCache struct {
	watermark int64
	failGet   bool
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
