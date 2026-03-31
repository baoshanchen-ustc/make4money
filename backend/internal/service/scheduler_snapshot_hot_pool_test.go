package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type schedulerHotPoolAccountRepoStub struct {
	AccountRepository
	accounts map[int64]*Account
}

func (s *schedulerHotPoolAccountRepoStub) GetByIDs(ctx context.Context, ids []int64) ([]*Account, error) {
	out := make([]*Account, 0, len(ids))
	for _, id := range ids {
		if account := s.accounts[id]; account != nil {
			clone := *account
			out = append(out, &clone)
		}
	}
	return out, nil
}

type schedulerSnapshotHotPoolCacheStub struct {
	members map[string]map[int64]struct{}
	dead    map[int64]bool
}

func (s *schedulerSnapshotHotPoolCacheStub) ListMembers(ctx context.Context, platform string) ([]int64, error) {
	set := s.members[platform]
	out := make([]int64, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	return out, nil
}
func (s *schedulerSnapshotHotPoolCacheStub) AddMembers(ctx context.Context, platform string, ids []int64) error {
	return nil
}
func (s *schedulerSnapshotHotPoolCacheStub) ReplaceMembers(ctx context.Context, platform string, ids []int64) error {
	return nil
}
func (s *schedulerSnapshotHotPoolCacheStub) RemoveMember(ctx context.Context, platform string, accountID int64) error {
	return nil
}
func (s *schedulerSnapshotHotPoolCacheStub) CountMembers(ctx context.Context, platform string) (int, error) {
	return len(s.members[platform]), nil
}
func (s *schedulerSnapshotHotPoolCacheStub) GetMeta(ctx context.Context, platform string) (*HotPoolPlatformMeta, error) {
	return &HotPoolPlatformMeta{}, nil
}
func (s *schedulerSnapshotHotPoolCacheStub) SetMeta(ctx context.Context, platform string, meta *HotPoolPlatformMeta) error {
	return nil
}
func (s *schedulerSnapshotHotPoolCacheStub) MarkDeadAccount(ctx context.Context, accountID int64, ttl time.Duration) error {
	if s.dead == nil {
		s.dead = map[int64]bool{}
	}
	s.dead[accountID] = true
	return nil
}
func (s *schedulerSnapshotHotPoolCacheStub) IsDeadAccount(ctx context.Context, accountID int64) (bool, error) {
	return s.dead[accountID], nil
}
func (s *schedulerSnapshotHotPoolCacheStub) TryAcquireRefillLock(ctx context.Context, platform string, ttl time.Duration) (bool, error) {
	return true, nil
}

type schedulerSnapshotCacheStub struct {
	SchedulerCache
	account *Account
}

func (s *schedulerSnapshotCacheStub) GetAccount(ctx context.Context, accountID int64) (*Account, error) {
	if s.account == nil || s.account.ID != accountID {
		return nil, nil
	}
	clone := *s.account
	return &clone, nil
}

func TestSchedulerSnapshotService_LoadAccountsFromHotPool_UngroupedOnly(t *testing.T) {
	cache := &schedulerSnapshotHotPoolCacheStub{
		members: map[string]map[int64]struct{}{
			PlatformOpenAI: {1: {}, 2: {}},
		},
	}
	hotPoolService := NewHotPoolService(cache, nil)
	repo := &schedulerHotPoolAccountRepoStub{
		accounts: map[int64]*Account{
			1: {ID: 1, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, GroupIDs: nil},
			2: {ID: 2, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, GroupIDs: []int64{10}},
		},
	}
	svc := NewSchedulerSnapshotService(nil, nil, repo, nil, &config.Config{})
	svc.SetHotPoolService(hotPoolService)
	svc.SetExtremePerformanceResolver(func(context.Context) (*ExtremePerformanceSettings, error) {
		settings := DefaultExtremePerformanceSettings()
		settings.Enabled = true
		return settings, nil
	})

	accounts, err := svc.loadAccountsFromDB(context.Background(), SchedulerBucket{GroupID: 0, Platform: PlatformOpenAI, Mode: SchedulerModeSingle}, false)
	require.NoError(t, err)
	require.Len(t, accounts, 1)
	require.Equal(t, int64(1), accounts[0].ID)
}

func TestSchedulerSnapshotService_GetAccount_ReturnsNilForDeadAccount(t *testing.T) {
	cache := &schedulerSnapshotCacheStub{
		account: &Account{ID: 88, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true},
	}
	hotCache := &schedulerSnapshotHotPoolCacheStub{dead: map[int64]bool{88: true}}
	hotPoolService := NewHotPoolService(hotCache, nil)
	svc := NewSchedulerSnapshotService(cache, nil, nil, nil, &config.Config{})
	svc.SetHotPoolService(hotPoolService)

	account, err := svc.GetAccount(context.Background(), 88)
	require.NoError(t, err)
	require.Nil(t, account)
}
