//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type accountFailureRepoStub struct {
	AccountRepository
	deleted []int64
}

func (s *accountFailureRepoStub) Delete(ctx context.Context, id int64) error {
	s.deleted = append(s.deleted, id)
	return nil
}

func TestAccountFailurePolicyService_HandleUpstream401_RemovesFromHotPathImmediately(t *testing.T) {
	repo := &accountFailureRepoStub{}
	settingRepo := &extremePerformanceSettingRepoStub{}
	settingService := NewSettingService(settingRepo, &config.Config{})
	require.NoError(t, settingService.SetExtremePerformanceSettings(context.Background(), &ExtremePerformanceSettings{
		Enabled: true,
		Admin:   DefaultExtremePerformanceSettings().Admin,
		Pool:    DefaultExtremePerformanceSettings().Pool,
		AccountPolicy: ExtremePerformanceAccountPolicySettings{
			DeleteOnAnyUpstream401:               true,
			CooldownOn429Minutes:                 5,
			CooldownOn5xxMinutes:                 1,
			RemoveFromHotPoolOnOverload:          true,
			RemoveFromHotPoolOnTempUnschedulable: true,
		},
	}))

	hotCache := &hotPoolCacheStub{members: map[string]map[int64]struct{}{PlatformOpenAI: {99: {}}}}
	hotPoolService := NewHotPoolService(hotCache, repo)
	snapshotCache := &schedulerSnapshotCacheStub{account: &Account{ID: 99, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true}}
	snapshotService := NewSchedulerSnapshotService(snapshotCache, nil, repo, nil, &config.Config{})
	snapshotService.SetHotPoolService(hotPoolService)
	policy := NewAccountFailurePolicyService(repo, settingService, hotPoolService, snapshotService)

	err := policy.HandleUpstream401(context.Background(), &Account{ID: 99, Platform: PlatformOpenAI}, "test")
	require.NoError(t, err)

	dead, deadErr := hotPoolService.IsDeadAccount(context.Background(), 99)
	require.NoError(t, deadErr)
	require.True(t, dead)
	members, listErr := hotPoolService.ListMembers(context.Background(), PlatformOpenAI)
	require.NoError(t, listErr)
	require.NotContains(t, members, int64(99))

	account, getErr := snapshotService.GetAccount(context.Background(), 99)
	require.NoError(t, getErr)
	require.Nil(t, account)
}

func TestAccountFailurePolicyService_ShouldDeleteOnAnyUpstream401_RequiresExtremeModeEnabled(t *testing.T) {
	settingRepo := &extremePerformanceSettingRepoStub{}
	settingService := NewSettingService(settingRepo, &config.Config{})
	policy := NewAccountFailurePolicyService(nil, settingService, nil, nil)

	require.False(t, policy.ShouldDeleteOnAnyUpstream401(context.Background()))

	require.NoError(t, settingService.SetExtremePerformanceSettings(context.Background(), &ExtremePerformanceSettings{
		Enabled: true,
		Admin:   DefaultExtremePerformanceSettings().Admin,
		Pool:    DefaultExtremePerformanceSettings().Pool,
		AccountPolicy: ExtremePerformanceAccountPolicySettings{
			DeleteOnAnyUpstream401:               true,
			CooldownOn429Minutes:                 5,
			CooldownOn5xxMinutes:                 1,
			RemoveFromHotPoolOnOverload:          true,
			RemoveFromHotPoolOnTempUnschedulable: true,
		},
	}))

	require.True(t, policy.ShouldDeleteOnAnyUpstream401(context.Background()))
}

func TestAccountFailurePolicyService_AsyncDeleteRuns(t *testing.T) {
	repo := &accountFailureRepoStub{}
	settingRepo := &extremePerformanceSettingRepoStub{}
	settingService := NewSettingService(settingRepo, &config.Config{})
	require.NoError(t, settingService.SetExtremePerformanceSettings(context.Background(), DefaultExtremePerformanceSettings()))
	settings, err := settingService.GetExtremePerformanceSettings(context.Background())
	require.NoError(t, err)
	settings.Enabled = true
	require.NoError(t, settingService.SetExtremePerformanceSettings(context.Background(), settings))

	policy := NewAccountFailurePolicyService(repo, settingService, nil, nil)
	require.NoError(t, policy.HandleUpstream401(context.Background(), &Account{ID: 77, Platform: PlatformOpenAI}, "test"))

	require.Eventually(t, func() bool {
		return len(repo.deleted) == 1 && repo.deleted[0] == 77
	}, time.Second, 20*time.Millisecond)
}
