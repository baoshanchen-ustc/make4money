package service

import (
	"context"
	"time"
)

// GroupCapacitySummary holds aggregated capacity for a single group.
type GroupCapacitySummary struct {
	GroupID         int64 `json:"group_id"`
	ConcurrencyUsed int   `json:"concurrency_used"`
	ConcurrencyMax  int   `json:"concurrency_max"`
	SessionsUsed    int   `json:"sessions_used"`
	SessionsMax     int   `json:"sessions_max"`
	RPMUsed         int   `json:"rpm_used"`
	RPMMax          int   `json:"rpm_max"`
}

// GroupCapacityService aggregates per-group capacity from runtime data.
type GroupCapacityService struct {
	accountRepo        AccountRepository
	groupRepo          GroupRepository
	concurrencyService *ConcurrencyService
	sessionLimitCache  SessionLimitCache
	rpmCache           RPMCache
}

// NewGroupCapacityService creates a new GroupCapacityService.
func NewGroupCapacityService(
	accountRepo AccountRepository,
	groupRepo GroupRepository,
	concurrencyService *ConcurrencyService,
	sessionLimitCache SessionLimitCache,
	rpmCache RPMCache,
) *GroupCapacityService {
	return &GroupCapacityService{
		accountRepo:        accountRepo,
		groupRepo:          groupRepo,
		concurrencyService: concurrencyService,
		sessionLimitCache:  sessionLimitCache,
		rpmCache:           rpmCache,
	}
}

// GetAllGroupCapacity returns capacity summary for all active groups.
func (s *GroupCapacityService) GetAllGroupCapacity(ctx context.Context) ([]GroupCapacitySummary, error) {
	groups, err := s.groupRepo.ListActive(ctx)
	if err != nil {
		return nil, err
	}

	results := make([]GroupCapacitySummary, len(groups))
	groupIndex := make(map[int64]int, len(groups))
	for i := range groups {
		results[i] = GroupCapacitySummary{GroupID: groups[i].ID}
		groupIndex[groups[i].ID] = i
	}

	accounts, err := s.accountRepo.ListSchedulable(ctx)
	if err != nil {
		return nil, err
	}
	if len(accounts) == 0 {
		return results, nil
	}

	accountIDs := make([]int64, 0, len(accounts))
	sessionTimeouts := make(map[int64]time.Duration)

	for i := range accounts {
		acc := &accounts[i]
		accountIDs = append(accountIDs, acc.ID)
		applyAccountCapacityToGroups(acc, groupIndex, results, sessionTimeouts)
	}

	concurrencyMap := make(map[int64]int, len(accountIDs))
	if s.concurrencyService != nil {
		concurrencyMap, _ = s.concurrencyService.GetAccountConcurrencyBatch(ctx, accountIDs)
		if concurrencyMap == nil {
			concurrencyMap = make(map[int64]int, len(accountIDs))
		}
	}

	var sessionsMap map[int64]int
	if len(sessionTimeouts) > 0 && s.sessionLimitCache != nil {
		sessionsMap, _ = s.sessionLimitCache.GetActiveSessionCountBatch(ctx, accountIDs, sessionTimeouts)
	}

	var rpmMap map[int64]int
	if s.rpmCache != nil {
		rpmMap, _ = s.rpmCache.GetRPMBatch(ctx, accountIDs)
	}

	for i := range accounts {
		acc := &accounts[i]
		applyAccountRuntimeToGroups(acc, groupIndex, results, concurrencyMap[acc.ID], sessionsMap, rpmMap)
	}

	return results, nil
}

func applyAccountCapacityToGroups(
	acc *Account,
	groupIndex map[int64]int,
	results []GroupCapacitySummary,
	sessionTimeouts map[int64]time.Duration,
) {
	for _, groupID := range acc.GroupIDs {
		idx, ok := groupIndex[groupID]
		if !ok {
			continue
		}
		results[idx].ConcurrencyMax += acc.Concurrency

		if ms := acc.GetMaxSessions(); ms > 0 {
			results[idx].SessionsMax += ms
			timeout := time.Duration(acc.GetSessionIdleTimeoutMinutes()) * time.Minute
			if timeout <= 0 {
				timeout = 5 * time.Minute
			}
			sessionTimeouts[acc.ID] = timeout
		}

		if rpm := acc.GetBaseRPM(); rpm > 0 {
			results[idx].RPMMax += rpm
		}
	}
}

func applyAccountRuntimeToGroups(
	acc *Account,
	groupIndex map[int64]int,
	results []GroupCapacitySummary,
	concurrencyUsed int,
	sessionsMap map[int64]int,
	rpmMap map[int64]int,
) {
	sessionsUsed := 0
	rpmUsed := 0
	if sessionsMap != nil {
		sessionsUsed = sessionsMap[acc.ID]
	}
	if rpmMap != nil {
		rpmUsed = rpmMap[acc.ID]
	}

	for _, groupID := range acc.GroupIDs {
		idx, ok := groupIndex[groupID]
		if !ok {
			continue
		}
		results[idx].ConcurrencyUsed += concurrencyUsed
		results[idx].SessionsUsed += sessionsUsed
		results[idx].RPMUsed += rpmUsed
	}
}
