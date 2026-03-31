//go:build unit

package service

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type hotPoolCacheStub struct {
	members map[string]map[int64]struct{}
	metas   map[string]*HotPoolPlatformMeta
	dead    map[int64]bool
	locks   map[string]bool
}

func (s *hotPoolCacheStub) ListMembers(ctx context.Context, platform string) ([]int64, error) {
	set := s.members[platform]
	out := make([]int64, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
}

func (s *hotPoolCacheStub) AddMembers(ctx context.Context, platform string, ids []int64) error {
	if s.members == nil {
		s.members = map[string]map[int64]struct{}{}
	}
	if s.members[platform] == nil {
		s.members[platform] = map[int64]struct{}{}
	}
	for _, id := range ids {
		s.members[platform][id] = struct{}{}
	}
	return nil
}

func (s *hotPoolCacheStub) ReplaceMembers(ctx context.Context, platform string, ids []int64) error {
	if s.members == nil {
		s.members = map[string]map[int64]struct{}{}
	}
	s.members[platform] = map[int64]struct{}{}
	for _, id := range ids {
		s.members[platform][id] = struct{}{}
	}
	return nil
}

func (s *hotPoolCacheStub) RemoveMember(ctx context.Context, platform string, accountID int64) error {
	if s.members != nil && s.members[platform] != nil {
		delete(s.members[platform], accountID)
	}
	return nil
}

func (s *hotPoolCacheStub) CountMembers(ctx context.Context, platform string) (int, error) {
	return len(s.members[platform]), nil
}

func (s *hotPoolCacheStub) GetMeta(ctx context.Context, platform string) (*HotPoolPlatformMeta, error) {
	if s.metas == nil {
		return nil, nil
	}
	return s.metas[platform], nil
}

func (s *hotPoolCacheStub) SetMeta(ctx context.Context, platform string, meta *HotPoolPlatformMeta) error {
	if s.metas == nil {
		s.metas = map[string]*HotPoolPlatformMeta{}
	}
	clone := *meta
	if meta.LastRefillAt != nil {
		t := *meta.LastRefillAt
		clone.LastRefillAt = &t
	}
	if meta.LastRebuildAt != nil {
		t := *meta.LastRebuildAt
		clone.LastRebuildAt = &t
	}
	s.metas[platform] = &clone
	return nil
}

func (s *hotPoolCacheStub) MarkDeadAccount(ctx context.Context, accountID int64, ttl time.Duration) error {
	if s.dead == nil {
		s.dead = map[int64]bool{}
	}
	s.dead[accountID] = true
	return nil
}

func (s *hotPoolCacheStub) IsDeadAccount(ctx context.Context, accountID int64) (bool, error) {
	return s.dead[accountID], nil
}

func (s *hotPoolCacheStub) TryAcquireRefillLock(ctx context.Context, platform string, ttl time.Duration) (bool, error) {
	if s.locks == nil {
		s.locks = map[string]bool{}
	}
	if s.locks[platform] {
		return false, nil
	}
	s.locks[platform] = true
	return true, nil
}

type hotPoolAccountRepoStub struct {
	AccountRepository
	cold map[string][]Account
}

func (s *hotPoolAccountRepoStub) ListColdCandidatesByPlatform(ctx context.Context, platform string, limit int, excludeIDs []int64) ([]Account, error) {
	excluded := map[int64]struct{}{}
	for _, id := range excludeIDs {
		excluded[id] = struct{}{}
	}
	out := make([]Account, 0, limit)
	for _, account := range s.cold[platform] {
		if _, skip := excluded[account.ID]; skip {
			continue
		}
		out = append(out, account)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func TestHotPoolService_RebuildPlatform_UsesImportedOrderAndSkipsDeadAccounts(t *testing.T) {
	cache := &hotPoolCacheStub{}
	repo := &hotPoolAccountRepoStub{
		cold: map[string][]Account{
			PlatformOpenAI: {
				{ID: 11, Platform: PlatformOpenAI},
				{ID: 12, Platform: PlatformOpenAI},
				{ID: 13, Platform: PlatformOpenAI},
			},
		},
	}
	cache.dead = map[int64]bool{12: true}
	svc := NewHotPoolService(cache, repo)
	settings := DefaultExtremePerformanceSettings()
	settings.Enabled = true
	settings.Pool.PlatformLimits.OpenAI = 2

	err := svc.RebuildPlatform(context.Background(), settings, PlatformOpenAI)
	require.NoError(t, err)

	members, err := svc.ListMembers(context.Background(), PlatformOpenAI)
	require.NoError(t, err)
	require.Equal(t, []int64{11, 13}, members)
}

func TestHotPoolService_RefillPlatform_FillsGapAndUpdatesMeta(t *testing.T) {
	cache := &hotPoolCacheStub{
		members: map[string]map[int64]struct{}{
			PlatformOpenAI: {101: {}, 102: {}},
		},
	}
	repo := &hotPoolAccountRepoStub{
		cold: map[string][]Account{
			PlatformOpenAI: {
				{ID: 201, Platform: PlatformOpenAI},
				{ID: 202, Platform: PlatformOpenAI},
			},
		},
	}
	svc := NewHotPoolService(cache, repo)
	settings := DefaultExtremePerformanceSettings()
	settings.Enabled = true
	settings.Pool.PlatformLimits.OpenAI = 4
	settings.Pool.RefillTriggerGap = 1
	settings.Pool.RefillBatchSize = 2

	err := svc.RefillPlatform(context.Background(), settings, PlatformOpenAI)
	require.NoError(t, err)

	members, err := svc.ListMembers(context.Background(), PlatformOpenAI)
	require.NoError(t, err)
	require.Equal(t, []int64{101, 102, 201, 202}, members)

	meta, err := cache.GetMeta(context.Background(), PlatformOpenAI)
	require.NoError(t, err)
	require.NotNil(t, meta)
	require.Equal(t, 4, meta.CurrentSize)
	require.NotNil(t, meta.LastRefillAt)
	require.Equal(t, int64(1), meta.Version)
}
