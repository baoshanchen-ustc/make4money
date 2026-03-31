package service

import (
	"context"
	"fmt"
	"time"
)

// AccountFailurePolicyService handles terminal account failures such as account-level upstream 401s.
type AccountFailurePolicyService struct {
	accountRepo       AccountRepository
	settingService    *SettingService
	hotPoolService    *HotPoolService
	schedulerSnapshot *SchedulerSnapshotService
}

func NewAccountFailurePolicyService(
	accountRepo AccountRepository,
	settingService *SettingService,
	hotPoolService *HotPoolService,
	schedulerSnapshot *SchedulerSnapshotService,
) *AccountFailurePolicyService {
	return &AccountFailurePolicyService{
		accountRepo:       accountRepo,
		settingService:    settingService,
		hotPoolService:    hotPoolService,
		schedulerSnapshot: schedulerSnapshot,
	}
}

func (s *AccountFailurePolicyService) ShouldDeleteOnAnyUpstream401(ctx context.Context) bool {
	if s == nil || s.settingService == nil {
		return false
	}
	settings, err := s.settingService.GetExtremePerformanceSettings(ctx)
	if err != nil || settings == nil || !settings.Enabled {
		return false
	}
	return settings.AccountPolicy.DeleteOnAnyUpstream401
}

// HandleUpstream401 immediately removes the account from hot-path selection, then deletes it asynchronously.
func (s *AccountFailurePolicyService) HandleUpstream401(ctx context.Context, account *Account, source string) error {
	if s == nil || account == nil || account.ID <= 0 {
		return nil
	}
	if !s.ShouldDeleteOnAnyUpstream401(ctx) {
		return nil
	}
	if s.hotPoolService != nil {
		if err := s.hotPoolService.MarkDeadAccount(ctx, account.ID); err != nil {
			return fmt.Errorf("mark dead account: %w", err)
		}
		if err := s.hotPoolService.RemoveAccount(ctx, account.Platform, account.ID); err != nil {
			return fmt.Errorf("remove hot pool account: %w", err)
		}
	}
	if s.schedulerSnapshot != nil {
		if err := s.schedulerSnapshot.DeleteAccountFromCache(ctx, account.ID); err != nil {
			return fmt.Errorf("delete scheduler account cache: %w", err)
		}
	}

	go s.asyncDeleteAndRefill(account.ID, account.Platform, source)
	return nil
}

func (s *AccountFailurePolicyService) asyncDeleteAndRefill(accountID int64, platform string, source string) {
	if s == nil || accountID <= 0 {
		return
	}
	deleteCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if s.accountRepo != nil {
		_ = s.accountRepo.Delete(deleteCtx, accountID)
	}
	if s.schedulerSnapshot != nil {
		_ = s.schedulerSnapshot.RebuildPlatformBucket(deleteCtx, platform)
	}
	if s.hotPoolService != nil && s.settingService != nil {
		settings, err := s.settingService.GetExtremePerformanceSettings(deleteCtx)
		if err == nil && settings != nil && settings.Enabled {
			_ = s.hotPoolService.RefillPlatform(deleteCtx, settings, platform)
		}
	}
	_ = source
}
