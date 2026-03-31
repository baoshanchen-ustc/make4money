package service

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type hotPoolColdCandidateLister interface {
	ListColdCandidatesByPlatform(ctx context.Context, platform string, limit int, excludeIDs []int64) ([]Account, error)
}

type hotPoolColdCandidateCounter interface {
	CountColdCandidatesByPlatform(ctx context.Context, platform string, excludeIDs []int64) (int, error)
}

// HotPoolService manages platform-level hot pools used by extreme performance mode.
type HotPoolService struct {
	cache             HotPoolCache
	accountRepo       AccountRepository
	onPlatformChanged func(context.Context, string) error
}

func NewHotPoolService(cache HotPoolCache, accountRepo AccountRepository) *HotPoolService {
	return &HotPoolService{
		cache:       cache,
		accountRepo: accountRepo,
	}
}

// SetOnPlatformChanged registers a callback invoked after hot-pool membership changes.
func (s *HotPoolService) SetOnPlatformChanged(callback func(context.Context, string) error) {
	s.onPlatformChanged = callback
}

func (s *HotPoolService) BuildInitialPools(ctx context.Context, settings *ExtremePerformanceSettings) error {
	if s == nil || !isExtremePerformanceEnabled(settings) {
		return nil
	}
	for _, platform := range extremePerformancePlatforms() {
		if err := s.RebuildPlatform(ctx, settings, platform); err != nil {
			return err
		}
	}
	return nil
}

func (s *HotPoolService) GetPlatformStatus(ctx context.Context, settings *ExtremePerformanceSettings, platform string) (*HotPoolPlatformStatus, error) {
	if s == nil || s.cache == nil {
		return &HotPoolPlatformStatus{Platform: platform}, nil
	}
	meta, err := s.cache.GetMeta(ctx, platform)
	if err != nil {
		return nil, err
	}
	count, err := s.cache.CountMembers(ctx, platform)
	if err != nil {
		return nil, err
	}
	if meta == nil {
		meta = &HotPoolPlatformMeta{}
	}
	status := &HotPoolPlatformStatus{
		Platform:      platform,
		TargetSize:    extremePerformanceTargetSize(settings, platform),
		CurrentSize:   count,
		LastRefillAt:  meta.LastRefillAt,
		LastRebuildAt: meta.LastRebuildAt,
		Version:       meta.Version,
	}
	if counter, ok := s.accountRepo.(hotPoolColdCandidateCounter); ok {
		memberIDs, memberErr := s.cache.ListMembers(ctx, platform)
		if memberErr == nil {
			coldCount, countErr := counter.CountColdCandidatesByPlatform(ctx, platform, memberIDs)
			if countErr == nil {
				status.ColdCandidates = coldCount
			}
		}
	}
	return status, nil
}

func (s *HotPoolService) ListMembers(ctx context.Context, platform string) ([]int64, error) {
	if s == nil || s.cache == nil {
		return nil, nil
	}
	return s.cache.ListMembers(ctx, platform)
}

func (s *HotPoolService) MarkDeadAccount(ctx context.Context, accountID int64) error {
	if s == nil || s.cache == nil {
		return nil
	}
	return s.cache.MarkDeadAccount(ctx, accountID, hotPoolDeadAccountTTL)
}

func (s *HotPoolService) IsDeadAccount(ctx context.Context, accountID int64) (bool, error) {
	if s == nil || s.cache == nil {
		return false, nil
	}
	return s.cache.IsDeadAccount(ctx, accountID)
}

func (s *HotPoolService) RemoveAccount(ctx context.Context, platform string, accountID int64) error {
	if s == nil || s.cache == nil || accountID <= 0 {
		return nil
	}
	if err := s.cache.RemoveMember(ctx, platform, accountID); err != nil {
		return err
	}
	return s.refreshMetaAndNotify(ctx, platform, 0, false)
}

func (s *HotPoolService) RefillPlatform(ctx context.Context, settings *ExtremePerformanceSettings, platform string) error {
	if s == nil || s.cache == nil || !isExtremePerformanceEnabled(settings) {
		return nil
	}
	if _, ok := extremePerformancePlatformLimit(settings, platform); !ok {
		return fmt.Errorf("unsupported platform %q", platform)
	}
	locked, err := s.cache.TryAcquireRefillLock(ctx, platform, 30*time.Second)
	if err != nil {
		return err
	}
	if !locked {
		return nil
	}

	currentIDs, err := s.cache.ListMembers(ctx, platform)
	if err != nil {
		return err
	}
	target := extremePerformanceTargetSize(settings, platform)
	gap := target - len(currentIDs)
	if gap < settings.Pool.RefillTriggerGap {
		return s.refreshMetaAndNotify(ctx, platform, target, false)
	}

	batchSize := gap
	if batchSize > settings.Pool.RefillBatchSize {
		batchSize = settings.Pool.RefillBatchSize
	}
	if batchSize <= 0 {
		return s.refreshMetaAndNotify(ctx, platform, target, false)
	}

	candidates, err := s.listColdCandidates(ctx, platform, batchSize*5, currentIDs)
	if err != nil {
		return err
	}
	selected := make([]int64, 0, batchSize)
	for _, candidate := range candidates {
		if len(selected) >= batchSize {
			break
		}
		dead, deadErr := s.cache.IsDeadAccount(ctx, candidate.ID)
		if deadErr != nil {
			return deadErr
		}
		if dead {
			continue
		}
		selected = append(selected, candidate.ID)
	}
	if err := s.cache.AddMembers(ctx, platform, selected); err != nil {
		return err
	}
	return s.refreshMetaAndNotify(ctx, platform, target, false)
}

func (s *HotPoolService) RebuildPlatform(ctx context.Context, settings *ExtremePerformanceSettings, platform string) error {
	if s == nil || s.cache == nil || !isExtremePerformanceEnabled(settings) {
		return nil
	}
	if _, ok := extremePerformancePlatformLimit(settings, platform); !ok {
		return fmt.Errorf("unsupported platform %q", platform)
	}
	locked, err := s.cache.TryAcquireRefillLock(ctx, platform, 30*time.Second)
	if err != nil {
		return err
	}
	if !locked {
		return nil
	}

	target := extremePerformanceTargetSize(settings, platform)
	candidates, err := s.listColdCandidates(ctx, platform, target*5, nil)
	if err != nil {
		return err
	}
	members := make([]int64, 0, target)
	for _, candidate := range candidates {
		if len(members) >= target {
			break
		}
		dead, deadErr := s.cache.IsDeadAccount(ctx, candidate.ID)
		if deadErr != nil {
			return deadErr
		}
		if dead {
			continue
		}
		members = append(members, candidate.ID)
	}
	if err := s.cache.ReplaceMembers(ctx, platform, members); err != nil {
		return err
	}
	return s.refreshMetaAndNotify(ctx, platform, target, true)
}

func (s *HotPoolService) listColdCandidates(ctx context.Context, platform string, limit int, excludeIDs []int64) ([]Account, error) {
	lister, ok := s.accountRepo.(hotPoolColdCandidateLister)
	if !ok {
		return nil, fmt.Errorf("account repository does not support cold candidate listing")
	}
	return lister.ListColdCandidatesByPlatform(ctx, platform, limit, excludeIDs)
}

func (s *HotPoolService) refreshMetaAndNotify(ctx context.Context, platform string, targetSize int, rebuilt bool) error {
	count, err := s.cache.CountMembers(ctx, platform)
	if err != nil {
		return err
	}
	meta, err := s.cache.GetMeta(ctx, platform)
	if err != nil {
		return err
	}
	if meta == nil {
		meta = &HotPoolPlatformMeta{}
	}
	now := time.Now().UTC()
	if targetSize > 0 {
		meta.TargetSize = targetSize
	}
	meta.CurrentSize = count
	meta.Version++
	if rebuilt {
		meta.LastRebuildAt = &now
	} else {
		meta.LastRefillAt = &now
	}
	if err := s.cache.SetMeta(ctx, platform, meta); err != nil {
		return err
	}
	if s.onPlatformChanged != nil {
		return s.onPlatformChanged(ctx, platform)
	}
	return nil
}

func isExtremePerformanceEnabled(settings *ExtremePerformanceSettings) bool {
	return settings != nil && settings.Enabled
}

func extremePerformancePlatforms() []string {
	return []string{PlatformOpenAI, PlatformGemini, PlatformAnthropic}
}

func extremePerformancePlatformLimit(settings *ExtremePerformanceSettings, platform string) (int, bool) {
	if settings == nil {
		return 0, false
	}
	switch strings.TrimSpace(platform) {
	case PlatformOpenAI:
		return settings.Pool.PlatformLimits.OpenAI, true
	case PlatformGemini:
		return settings.Pool.PlatformLimits.Gemini, true
	case PlatformAnthropic:
		return settings.Pool.PlatformLimits.Anthropic, true
	default:
		return 0, false
	}
}

func extremePerformanceTargetSize(settings *ExtremePerformanceSettings, platform string) int {
	limit, ok := extremePerformancePlatformLimit(settings, platform)
	if !ok {
		return 0
	}
	return limit
}
