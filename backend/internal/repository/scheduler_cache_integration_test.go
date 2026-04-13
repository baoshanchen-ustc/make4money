//go:build integration

package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestSchedulerCacheSnapshotUsesSlimMetadataButKeepsFullAccount(t *testing.T) {
	ctx := context.Background()
	rdb := testRedis(t)
	cache := NewSchedulerCache(rdb)

	bucket := service.SchedulerBucket{GroupID: 2, Platform: service.PlatformGemini, Mode: service.SchedulerModeSingle}
	now := time.Now().UTC().Truncate(time.Second)
	limitReset := now.Add(10 * time.Minute)
	overloadUntil := now.Add(2 * time.Minute)
	tempUnschedUntil := now.Add(3 * time.Minute)
	windowEnd := now.Add(5 * time.Hour)

	account := service.Account{
		ID:          101,
		Name:        "gemini-heavy",
		Platform:    service.PlatformGemini,
		Type:        service.AccountTypeOAuth,
		Status:      service.StatusActive,
		Schedulable: true,
		Concurrency: 3,
		Priority:    7,
		LastUsedAt:  &now,
		Credentials: map[string]any{
			"api_key":       "gemini-api-key",
			"access_token":  "secret-access-token",
			"project_id":    "proj-1",
			"oauth_type":    "ai_studio",
			"model_mapping": map[string]any{"gemini-2.5-pro": "gemini-2.5-pro"},
			"huge_blob":     strings.Repeat("x", 4096),
		},
		Extra: map[string]any{
			"mixed_scheduling":             true,
			"window_cost_limit":            12.5,
			"window_cost_sticky_reserve":   8.0,
			"max_sessions":                 4,
			"session_idle_timeout_minutes": 11,
			"unused_large_field":           strings.Repeat("y", 4096),
		},
		RateLimitResetAt:       &limitReset,
		OverloadUntil:          &overloadUntil,
		TempUnschedulableUntil: &tempUnschedUntil,
		SessionWindowStart:     &now,
		SessionWindowEnd:       &windowEnd,
		SessionWindowStatus:    "active",
	}

	require.NoError(t, cache.SetSnapshot(ctx, bucket, []service.Account{account}))

	snapshot, hit, err := cache.GetSnapshot(ctx, bucket)
	require.NoError(t, err)
	require.True(t, hit)
	require.Len(t, snapshot, 1)

	got := snapshot[0]
	require.NotNil(t, got)
	require.Equal(t, "gemini-api-key", got.GetCredential("api_key"))
	require.Equal(t, "proj-1", got.GetCredential("project_id"))
	require.Equal(t, "ai_studio", got.GetCredential("oauth_type"))
	require.NotEmpty(t, got.GetModelMapping())
	require.Empty(t, got.GetCredential("access_token"))
	require.Empty(t, got.GetCredential("huge_blob"))
	require.Equal(t, true, got.Extra["mixed_scheduling"])
	require.Equal(t, 12.5, got.GetWindowCostLimit())
	require.Equal(t, 8.0, got.GetWindowCostStickyReserve())
	require.Equal(t, 4, got.GetMaxSessions())
	require.Equal(t, 11, got.GetSessionIdleTimeoutMinutes())
	require.Nil(t, got.Extra["unused_large_field"])

	full, err := cache.GetAccount(ctx, account.ID)
	require.NoError(t, err)
	require.NotNil(t, full)
	require.Equal(t, "secret-access-token", full.GetCredential("access_token"))
	require.Equal(t, strings.Repeat("x", 4096), full.GetCredential("huge_blob"))
}

func TestSchedulerCacheBucketLockWithOwnerRelease(t *testing.T) {
	ctx := context.Background()
	rdb := testRedis(t)
	cache := NewSchedulerCache(rdb)

	owned, ok := cache.(service.SchedulerOwnedBucketLockCache)
	require.True(t, ok, "scheduler cache should support owner bucket locks")

	bucket := service.SchedulerBucket{GroupID: 4, Platform: service.PlatformOpenAI, Mode: service.SchedulerModeSingle}
	require.NoError(t, rdb.Del(ctx, "sched:lock:4:openai:single").Err())

	ok, err := owned.TryLockBucketWithOwner(ctx, bucket, "owner-A", 30*time.Second)
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = owned.TryLockBucketWithOwner(ctx, bucket, "owner-B", 30*time.Second)
	require.NoError(t, err)
	require.False(t, ok)

	require.NoError(t, owned.ReleaseBucketLock(ctx, bucket, "owner-B"))

	val, err := rdb.Get(ctx, "sched:lock:4:openai:single").Result()
	require.NoError(t, err)
	require.Equal(t, "owner-A", val)

	require.NoError(t, owned.ReleaseBucketLock(ctx, bucket, "owner-A"))

	_, err = rdb.Get(ctx, "sched:lock:4:openai:single").Result()
	require.ErrorIs(t, err, redis.Nil)
}

func TestSchedulerCacheReplaceBucketsPrunesStaleRegistry(t *testing.T) {
	ctx := context.Background()
	rdb := testRedis(t)
	cache := NewSchedulerCache(rdb)

	syncer, ok := cache.(service.SchedulerBucketRegistrySyncCache)
	require.True(t, ok, "scheduler cache should support bucket registry sync")

	stale := service.SchedulerBucket{GroupID: 9, Platform: service.PlatformOpenAI, Mode: service.SchedulerModeSingle}
	desired := service.SchedulerBucket{GroupID: 3, Platform: service.PlatformAnthropic, Mode: service.SchedulerModeMixed}

	require.NoError(t, cache.SetSnapshot(ctx, stale, nil))
	require.NoError(t, cache.SetSnapshot(ctx, desired, nil))

	activeKey := "sched:active:" + stale.String()
	activeVersion, err := rdb.Get(ctx, activeKey).Result()
	require.NoError(t, err)
	require.NotEmpty(t, activeVersion)

	require.NoError(t, syncer.ReplaceBuckets(ctx, []service.SchedulerBucket{desired}))

	buckets, err := cache.ListBuckets(ctx)
	require.NoError(t, err)
	require.Equal(t, []service.SchedulerBucket{desired}, buckets)

	_, err = rdb.Get(ctx, "sched:ready:"+stale.String()).Result()
	require.ErrorIs(t, err, redis.Nil)
	_, err = rdb.Get(ctx, activeKey).Result()
	require.ErrorIs(t, err, redis.Nil)
	_, err = rdb.Get(ctx, "sched:ver:"+stale.String()).Result()
	require.ErrorIs(t, err, redis.Nil)
	_, err = rdb.Get(ctx, "sched:lock:"+stale.String()).Result()
	require.ErrorIs(t, err, redis.Nil)
	_, err = rdb.ZRange(ctx, "sched:"+stale.String()+":v"+activeVersion, 0, -1).Result()
	require.NoError(t, err)
	require.Equal(t, int64(0), rdb.Exists(ctx, "sched:"+stale.String()+":v"+activeVersion).Val())
}
