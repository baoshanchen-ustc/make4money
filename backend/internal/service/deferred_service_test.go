//go:build unit

package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type deferredRepoStub struct {
	batchErr     error
	batchCalls   int
	batchUpdates map[int64]time.Time
}

func (s *deferredRepoStub) BatchUpdateLastUsed(_ context.Context, updates map[int64]time.Time) error {
	s.batchCalls++
	s.batchUpdates = cloneTimeMap(updates)
	return s.batchErr
}

type deferredSchedulerCacheStub struct {
	updateErr     error
	updateCalls   int
	updatePayload map[int64]time.Time
}

func (s *deferredSchedulerCacheStub) GetSnapshot(context.Context, SchedulerBucket) ([]*Account, bool, error) {
	return nil, false, nil
}
func (s *deferredSchedulerCacheStub) SetSnapshot(context.Context, SchedulerBucket, []Account) error {
	return nil
}
func (s *deferredSchedulerCacheStub) GetAccount(context.Context, int64) (*Account, error) { return nil, nil }
func (s *deferredSchedulerCacheStub) SetAccount(context.Context, *Account) error           { return nil }
func (s *deferredSchedulerCacheStub) DeleteAccount(context.Context, int64) error           { return nil }
func (s *deferredSchedulerCacheStub) UpdateLastUsed(_ context.Context, updates map[int64]time.Time) error {
	s.updateCalls++
	s.updatePayload = cloneTimeMap(updates)
	return s.updateErr
}
func (s *deferredSchedulerCacheStub) TryLockBucket(context.Context, SchedulerBucket, time.Duration) (bool, error) {
	return false, nil
}
func (s *deferredSchedulerCacheStub) ListBuckets(context.Context) ([]SchedulerBucket, error) {
	return nil, nil
}
func (s *deferredSchedulerCacheStub) GetOutboxWatermark(context.Context) (int64, error) { return 0, nil }
func (s *deferredSchedulerCacheStub) SetOutboxWatermark(context.Context, int64) error    { return nil }

func TestDeferredServiceFlushLastUsedUpdatesSchedulerCache(t *testing.T) {
	repo := &deferredRepoStub{}
	cache := &deferredSchedulerCacheStub{}
	svc := NewDeferredService(repo, cache, nil, time.Second)
	now := time.Now().UTC().Truncate(time.Second)
	svc.lastUsedUpdates.Store(int64(11), now)
	svc.lastUsedUpdates.Store(int64(22), now.Add(time.Second))

	svc.flushLastUsed()

	require.Equal(t, 1, repo.batchCalls)
	require.Len(t, repo.batchUpdates, 2)
	require.Equal(t, 1, cache.updateCalls)
	require.Equal(t, repo.batchUpdates, cache.updatePayload)
	requireLastUsedQueueEmpty(t, &svc.lastUsedUpdates)
}

func TestDeferredServiceFlushLastUsedRequeuesOnDBFailure(t *testing.T) {
	repo := &deferredRepoStub{batchErr: errors.New("db down")}
	cache := &deferredSchedulerCacheStub{}
	svc := NewDeferredService(repo, cache, nil, time.Second)
	now := time.Now().UTC().Truncate(time.Second)
	svc.lastUsedUpdates.Store(int64(33), now)

	svc.flushLastUsed()

	require.Equal(t, 1, repo.batchCalls)
	require.Equal(t, 0, cache.updateCalls)
	requireLastUsedQueueSize(t, &svc.lastUsedUpdates, 1)
}

func TestDeferredServiceFlushLastUsedRequeuesOnCacheFailure(t *testing.T) {
	repo := &deferredRepoStub{}
	cache := &deferredSchedulerCacheStub{updateErr: errors.New("redis down")}
	svc := NewDeferredService(repo, cache, nil, time.Second)
	now := time.Now().UTC().Truncate(time.Second)
	svc.lastUsedUpdates.Store(int64(44), now)

	svc.flushLastUsed()

	require.Equal(t, 1, repo.batchCalls)
	require.Equal(t, 1, cache.updateCalls)
	requireLastUsedQueueSize(t, &svc.lastUsedUpdates, 1)
}

func cloneTimeMap(src map[int64]time.Time) map[int64]time.Time {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[int64]time.Time, len(src))
	for id, ts := range src {
		dst[id] = ts
	}
	return dst
}

func requireLastUsedQueueEmpty(t *testing.T, m *sync.Map) {
	t.Helper()
	requireLastUsedQueueSize(t, m, 0)
}

func requireLastUsedQueueSize(t *testing.T, m *sync.Map, want int) {
	t.Helper()
	count := 0
	m.Range(func(_, _ any) bool {
		count++
		return true
	})
	require.Equal(t, want, count)
}
