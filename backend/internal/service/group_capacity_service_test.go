package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type groupCapacityAccountRepo struct {
	stubOpenAIAccountRepo
	listSchedulableCalls        int
	listSchedulableByGroupCalls int
}

func (r *groupCapacityAccountRepo) ListSchedulable(ctx context.Context) ([]Account, error) {
	r.listSchedulableCalls++
	return append([]Account(nil), r.accounts...), nil
}

func (r *groupCapacityAccountRepo) ListSchedulableByGroupID(ctx context.Context, groupID int64) ([]Account, error) {
	r.listSchedulableByGroupCalls++
	return nil, nil
}

type groupCapacityGroupRepo struct {
	active []Group
}

func (r *groupCapacityGroupRepo) ListActive(context.Context) ([]Group, error) {
	return append([]Group(nil), r.active...), nil
}

type groupCapacitySessionCache struct {
	counts map[int64]int
}

func (c *groupCapacitySessionCache) RegisterSession(context.Context, int64, string, int, time.Duration) (bool, error) {
	return true, nil
}
func (c *groupCapacitySessionCache) RefreshSession(context.Context, int64, string, time.Duration) error {
	return nil
}
func (c *groupCapacitySessionCache) GetActiveSessionCount(context.Context, int64) (int, error) {
	return 0, nil
}
func (c *groupCapacitySessionCache) GetActiveSessionCountBatch(context.Context, []int64, map[int64]time.Duration) (map[int64]int, error) {
	return c.counts, nil
}
func (c *groupCapacitySessionCache) IsSessionActive(context.Context, int64, string) (bool, error) {
	return false, nil
}
func (c *groupCapacitySessionCache) GetWindowCost(context.Context, int64) (float64, bool, error) {
	return 0, false, nil
}
func (c *groupCapacitySessionCache) SetWindowCost(context.Context, int64, float64) error {
	return nil
}
func (c *groupCapacitySessionCache) GetWindowCostBatch(context.Context, []int64) (map[int64]float64, error) {
	return nil, nil
}

type groupCapacityRPMCache struct {
	counts map[int64]int
}

func (c *groupCapacityRPMCache) IncrementRPM(context.Context, int64) (int, error) {
	return 0, nil
}
func (c *groupCapacityRPMCache) GetRPM(context.Context, int64) (int, error) {
	return 0, nil
}
func (c *groupCapacityRPMCache) GetRPMBatch(context.Context, []int64) (map[int64]int, error) {
	return c.counts, nil
}

func TestGroupCapacityService_GetAllGroupCapacity_AggregatesSchedulableAccountsOnce(t *testing.T) {
	t.Parallel()

	groupRepo := &groupCapacityGroupRepo{
		active: []Group{
			{ID: 1, Name: "g1"},
			{ID: 2, Name: "g2"},
			{ID: 3, Name: "g3"},
		},
	}
	accountRepo := &groupCapacityAccountRepo{
		stubOpenAIAccountRepo: stubOpenAIAccountRepo{
			accounts: []Account{
				{
					ID:          10,
					Platform:    PlatformAnthropic,
					Type:        AccountTypeOAuth,
					Status:      StatusActive,
					Schedulable: true,
					Concurrency: 2,
					GroupIDs:    []int64{1},
					Extra: map[string]any{
						"max_sessions":                 3,
						"session_idle_timeout_minutes": 10,
						"base_rpm":                     5,
					},
				},
				{
					ID:          11,
					Platform:    PlatformOpenAI,
					Type:        AccountTypeOAuth,
					Status:      StatusActive,
					Schedulable: true,
					Concurrency: 4,
					GroupIDs:    []int64{1, 2},
					Extra: map[string]any{
						"base_rpm": 7,
					},
				},
				{
					ID:          12,
					Platform:    PlatformOpenAI,
					Type:        AccountTypeOAuth,
					Status:      StatusActive,
					Schedulable: true,
					Concurrency: 1,
					GroupIDs:    []int64{99},
				},
			},
		},
	}
	concurrencySvc := NewConcurrencyService(&groupCapacityConcurrencyCache{
		counts: map[int64]int{
			10: 1,
			11: 2,
			12: 9,
		},
	})
	sessionCache := &groupCapacitySessionCache{counts: map[int64]int{10: 2}}
	rpmCache := &groupCapacityRPMCache{counts: map[int64]int{10: 4, 11: 6, 12: 8}}

	svc := NewGroupCapacityService(accountRepo, groupRepo, concurrencySvc, sessionCache, rpmCache)

	results, err := svc.GetAllGroupCapacity(context.Background())
	require.NoError(t, err)
	require.Len(t, results, 3)
	require.Equal(t, 1, accountRepo.listSchedulableCalls)
	require.Zero(t, accountRepo.listSchedulableByGroupCalls)

	require.Equal(t, GroupCapacitySummary{
		GroupID:         1,
		ConcurrencyUsed: 3,
		ConcurrencyMax:  6,
		SessionsUsed:    2,
		SessionsMax:     3,
		RPMUsed:         10,
		RPMMax:          12,
	}, results[0])
	require.Equal(t, GroupCapacitySummary{
		GroupID:         2,
		ConcurrencyUsed: 2,
		ConcurrencyMax:  4,
		SessionsUsed:    0,
		SessionsMax:     0,
		RPMUsed:         6,
		RPMMax:          7,
	}, results[1])
	require.Equal(t, GroupCapacitySummary{
		GroupID:         3,
		ConcurrencyUsed: 0,
		ConcurrencyMax:  0,
		SessionsUsed:    0,
		SessionsMax:     0,
		RPMUsed:         0,
		RPMMax:          0,
	}, results[2])
}

type groupCapacityConcurrencyCache struct {
	counts map[int64]int
}

func (c *groupCapacityConcurrencyCache) AcquireAccountSlot(context.Context, int64, int, string) (bool, error) {
	return true, nil
}
func (c *groupCapacityConcurrencyCache) ReleaseAccountSlot(context.Context, int64, string) error {
	return nil
}
func (c *groupCapacityConcurrencyCache) GetAccountConcurrency(context.Context, int64) (int, error) {
	return 0, nil
}
func (c *groupCapacityConcurrencyCache) GetAccountConcurrencyBatch(_ context.Context, accountIDs []int64) (map[int64]int, error) {
	result := make(map[int64]int, len(accountIDs))
	for _, accountID := range accountIDs {
		result[accountID] = c.counts[accountID]
	}
	return result, nil
}
func (c *groupCapacityConcurrencyCache) IncrementAccountWaitCount(context.Context, int64, int) (bool, error) {
	return true, nil
}
func (c *groupCapacityConcurrencyCache) DecrementAccountWaitCount(context.Context, int64) error {
	return nil
}
func (c *groupCapacityConcurrencyCache) GetAccountWaitingCount(context.Context, int64) (int, error) {
	return 0, nil
}
func (c *groupCapacityConcurrencyCache) AcquireUserSlot(context.Context, int64, int, string) (bool, error) {
	return true, nil
}
func (c *groupCapacityConcurrencyCache) ReleaseUserSlot(context.Context, int64, string) error {
	return nil
}
func (c *groupCapacityConcurrencyCache) GetUserConcurrency(context.Context, int64) (int, error) {
	return 0, nil
}
func (c *groupCapacityConcurrencyCache) IncrementWaitCount(context.Context, int64, int) (bool, error) {
	return true, nil
}
func (c *groupCapacityConcurrencyCache) DecrementWaitCount(context.Context, int64) error {
	return nil
}
func (c *groupCapacityConcurrencyCache) GetAccountsLoadBatch(context.Context, []AccountWithConcurrency) (map[int64]*AccountLoadInfo, error) {
	return nil, nil
}
func (c *groupCapacityConcurrencyCache) GetUsersLoadBatch(context.Context, []UserWithConcurrency) (map[int64]*UserLoadInfo, error) {
	return nil, nil
}
func (c *groupCapacityConcurrencyCache) CleanupExpiredAccountSlots(context.Context, int64) error {
	return nil
}
func (c *groupCapacityConcurrencyCache) CleanupStaleProcessSlots(context.Context, string) error {
	return nil
}

var _ GroupRepository = (*groupCapacityGroupRepo)(nil)
var _ AccountRepository = (*groupCapacityAccountRepo)(nil)
var _ SessionLimitCache = (*groupCapacitySessionCache)(nil)
var _ RPMCache = (*groupCapacityRPMCache)(nil)
var _ ConcurrencyCache = (*groupCapacityConcurrencyCache)(nil)

func (r *groupCapacityGroupRepo) List(context.Context, pagination.PaginationParams) ([]Group, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (r *groupCapacityGroupRepo) Create(context.Context, *Group) error { return nil }
func (r *groupCapacityGroupRepo) GetByID(context.Context, int64) (*Group, error) {
	return nil, nil
}
func (r *groupCapacityGroupRepo) GetByIDLite(context.Context, int64) (*Group, error) {
	return nil, nil
}
func (r *groupCapacityGroupRepo) Update(context.Context, *Group) error { return nil }
func (r *groupCapacityGroupRepo) Delete(context.Context, int64) error  { return nil }
func (r *groupCapacityGroupRepo) DeleteCascade(context.Context, int64) ([]int64, error) {
	return nil, nil
}
func (r *groupCapacityGroupRepo) ListWithFilters(context.Context, pagination.PaginationParams, string, string, string, *bool) ([]Group, *pagination.PaginationResult, error) {
	return nil, nil, nil
}
func (r *groupCapacityGroupRepo) ListActiveByPlatform(context.Context, string) ([]Group, error) {
	return nil, nil
}
func (r *groupCapacityGroupRepo) ExistsByName(context.Context, string) (bool, error) {
	return false, nil
}
func (r *groupCapacityGroupRepo) GetAccountCount(context.Context, int64) (int64, int64, error) {
	return 0, 0, nil
}
func (r *groupCapacityGroupRepo) DeleteAccountGroupsByGroupID(context.Context, int64) (int64, error) {
	return 0, nil
}
func (r *groupCapacityGroupRepo) GetAccountIDsByGroupIDs(context.Context, []int64) ([]int64, error) {
	return nil, nil
}
func (r *groupCapacityGroupRepo) BindAccountsToGroup(context.Context, int64, []int64) error {
	return nil
}
func (r *groupCapacityGroupRepo) UpdateSortOrders(context.Context, []GroupSortOrderUpdate) error {
	return nil
}
