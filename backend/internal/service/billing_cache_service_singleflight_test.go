//go:build unit

package service

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type billingCacheMissStub struct {
	setBalanceCalls      atomic.Int64
	setSubscriptionCalls atomic.Int64
}

func (s *billingCacheMissStub) GetUserBalance(ctx context.Context, userID int64) (float64, error) {
	return 0, errors.New("cache miss")
}

func (s *billingCacheMissStub) SetUserBalance(ctx context.Context, userID int64, balance float64) error {
	s.setBalanceCalls.Add(1)
	return nil
}

func (s *billingCacheMissStub) DeductUserBalance(ctx context.Context, userID int64, amount float64) error {
	return nil
}

func (s *billingCacheMissStub) InvalidateUserBalance(ctx context.Context, userID int64) error {
	return nil
}

func (s *billingCacheMissStub) GetSubscriptionCache(ctx context.Context, userID, groupID int64) (*SubscriptionCacheData, error) {
	return nil, errors.New("cache miss")
}

func (s *billingCacheMissStub) SetSubscriptionCache(ctx context.Context, userID, groupID int64, data *SubscriptionCacheData) error {
	s.setSubscriptionCalls.Add(1)
	return nil
}

func (s *billingCacheMissStub) UpdateSubscriptionUsage(ctx context.Context, userID, groupID int64, cost float64) error {
	return nil
}

func (s *billingCacheMissStub) InvalidateSubscriptionCache(ctx context.Context, userID, groupID int64) error {
	return nil
}

func (s *billingCacheMissStub) GetAPIKeyRateLimit(ctx context.Context, keyID int64) (*APIKeyRateLimitCacheData, error) {
	return nil, errors.New("cache miss")
}

func (s *billingCacheMissStub) SetAPIKeyRateLimit(ctx context.Context, keyID int64, data *APIKeyRateLimitCacheData) error {
	return nil
}

func (s *billingCacheMissStub) UpdateAPIKeyRateLimitUsage(ctx context.Context, keyID int64, cost float64) error {
	return nil
}

func (s *billingCacheMissStub) InvalidateAPIKeyRateLimit(ctx context.Context, keyID int64) error {
	return nil
}

type balanceLoadUserRepoStub struct {
	mockUserRepo
	calls   atomic.Int64
	delay   time.Duration
	balance float64
}

func (s *balanceLoadUserRepoStub) GetByID(ctx context.Context, id int64) (*User, error) {
	s.calls.Add(1)
	if s.delay > 0 {
		select {
		case <-time.After(s.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return &User{ID: id, Balance: s.balance}, nil
}

func TestBillingCacheServiceGetUserBalance_Singleflight(t *testing.T) {
	cache := &billingCacheMissStub{}
	userRepo := &balanceLoadUserRepoStub{
		delay:   80 * time.Millisecond,
		balance: 12.34,
	}
	svc := NewBillingCacheService(cache, userRepo, nil, nil, &config.Config{})
	t.Cleanup(svc.Stop)

	const goroutines = 16
	start := make(chan struct{})
	var wg sync.WaitGroup
	errCh := make(chan error, goroutines)
	balCh := make(chan float64, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			bal, err := svc.GetUserBalance(context.Background(), 99)
			errCh <- err
			balCh <- bal
		}()
	}

	close(start)
	wg.Wait()
	close(errCh)
	close(balCh)

	for err := range errCh {
		require.NoError(t, err)
	}
	for bal := range balCh {
		require.Equal(t, 12.34, bal)
	}

	require.Equal(t, int64(1), userRepo.calls.Load(), "并发穿透应被 singleflight 合并")
	require.Eventually(t, func() bool {
		return cache.setBalanceCalls.Load() >= 1
	}, time.Second, 10*time.Millisecond)
}

type subscriptionLoadUserSubRepoStub struct {
	calls        atomic.Int64
	delay        time.Duration
	subscription *UserSubscription
}

func (s *subscriptionLoadUserSubRepoStub) Create(ctx context.Context, sub *UserSubscription) error {
	panic("unexpected Create call")
}

func (s *subscriptionLoadUserSubRepoStub) GetByID(ctx context.Context, id int64) (*UserSubscription, error) {
	panic("unexpected GetByID call")
}

func (s *subscriptionLoadUserSubRepoStub) GetByUserIDAndGroupID(ctx context.Context, userID, groupID int64) (*UserSubscription, error) {
	panic("unexpected GetByUserIDAndGroupID call")
}

func (s *subscriptionLoadUserSubRepoStub) GetActiveByUserIDAndGroupID(ctx context.Context, userID, groupID int64) (*UserSubscription, error) {
	s.calls.Add(1)
	if s.delay > 0 {
		select {
		case <-time.After(s.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if s.subscription == nil {
		return nil, ErrSubscriptionNotFound
	}
	cp := *s.subscription
	return &cp, nil
}

func (s *subscriptionLoadUserSubRepoStub) Update(ctx context.Context, sub *UserSubscription) error {
	panic("unexpected Update call")
}

func (s *subscriptionLoadUserSubRepoStub) Delete(ctx context.Context, id int64) error {
	panic("unexpected Delete call")
}

func (s *subscriptionLoadUserSubRepoStub) ListByUserID(ctx context.Context, userID int64) ([]UserSubscription, error) {
	panic("unexpected ListByUserID call")
}

func (s *subscriptionLoadUserSubRepoStub) ListActiveByUserID(ctx context.Context, userID int64) ([]UserSubscription, error) {
	panic("unexpected ListActiveByUserID call")
}

func (s *subscriptionLoadUserSubRepoStub) ListByGroupID(ctx context.Context, groupID int64, params pagination.PaginationParams) ([]UserSubscription, *pagination.PaginationResult, error) {
	panic("unexpected ListByGroupID call")
}

func (s *subscriptionLoadUserSubRepoStub) List(ctx context.Context, params pagination.PaginationParams, userID, groupID *int64, status, platform, sortBy, sortOrder string) ([]UserSubscription, *pagination.PaginationResult, error) {
	panic("unexpected List call")
}

func (s *subscriptionLoadUserSubRepoStub) ExistsByUserIDAndGroupID(ctx context.Context, userID, groupID int64) (bool, error) {
	panic("unexpected ExistsByUserIDAndGroupID call")
}

func (s *subscriptionLoadUserSubRepoStub) ExtendExpiry(ctx context.Context, subscriptionID int64, newExpiresAt time.Time) error {
	panic("unexpected ExtendExpiry call")
}

func (s *subscriptionLoadUserSubRepoStub) UpdateStatus(ctx context.Context, subscriptionID int64, status string) error {
	panic("unexpected UpdateStatus call")
}

func (s *subscriptionLoadUserSubRepoStub) UpdateNotes(ctx context.Context, subscriptionID int64, notes string) error {
	panic("unexpected UpdateNotes call")
}

func (s *subscriptionLoadUserSubRepoStub) ActivateWindows(ctx context.Context, id int64, start time.Time) error {
	panic("unexpected ActivateWindows call")
}

func (s *subscriptionLoadUserSubRepoStub) ResetDailyUsage(ctx context.Context, id int64, newWindowStart time.Time) error {
	panic("unexpected ResetDailyUsage call")
}

func (s *subscriptionLoadUserSubRepoStub) ResetWeeklyUsage(ctx context.Context, id int64, newWindowStart time.Time) error {
	panic("unexpected ResetWeeklyUsage call")
}

func (s *subscriptionLoadUserSubRepoStub) ResetMonthlyUsage(ctx context.Context, id int64, newWindowStart time.Time) error {
	panic("unexpected ResetMonthlyUsage call")
}

func (s *subscriptionLoadUserSubRepoStub) IncrementUsage(ctx context.Context, id int64, costUSD float64) error {
	panic("unexpected IncrementUsage call")
}

func (s *subscriptionLoadUserSubRepoStub) BatchUpdateExpiredStatus(ctx context.Context) (int64, error) {
	panic("unexpected BatchUpdateExpiredStatus call")
}

func TestBillingCacheServiceGetSubscriptionStatus_Singleflight(t *testing.T) {
	cache := &billingCacheMissStub{}
	repo := &subscriptionLoadUserSubRepoStub{
		delay: 80 * time.Millisecond,
		subscription: &UserSubscription{
			ID:              101,
			UserID:          99,
			GroupID:         12,
			Status:          SubscriptionStatusActive,
			ExpiresAt:       time.Now().Add(time.Hour),
			DailyUsageUSD:   0.0,
			WeeklyUsageUSD:  0.0,
			MonthlyUsageUSD: 0.0,
		},
	}
	svc := NewBillingCacheService(cache, nil, repo, nil, &config.Config{})
	t.Cleanup(svc.Stop)

	const goroutines = 16
	start := make(chan struct{})
	var wg sync.WaitGroup
	errCh := make(chan error, goroutines)
	dataCh := make(chan *subscriptionCacheData, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			data, err := svc.GetSubscriptionStatus(context.Background(), 99, 12)
			errCh <- err
			dataCh <- data
		}()
	}

	close(start)
	wg.Wait()
	close(errCh)
	close(dataCh)

	for err := range errCh {
		require.NoError(t, err)
	}
	for data := range dataCh {
		require.NotNil(t, data)
		require.Equal(t, SubscriptionStatusActive, data.Status)
	}

	require.Equal(t, int64(1), repo.calls.Load(), "singleflight 应该合并并发订阅请求")
	require.Equal(t, int64(1), cache.setSubscriptionCalls.Load(), "缓存写入只需一次")
}
