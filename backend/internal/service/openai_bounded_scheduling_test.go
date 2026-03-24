package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type openAIWindowCall struct {
	offset int
	limit  int
}

type windowedOpenAISnapshotCacheStub struct {
	openAISnapshotCacheStub
	fullCalls   int
	windowCalls []openAIWindowCall
	miss        bool
}

func (s *windowedOpenAISnapshotCacheStub) GetSnapshot(ctx context.Context, bucket SchedulerBucket) ([]*Account, bool, error) {
	s.fullCalls++
	if s.miss {
		return nil, false, nil
	}
	return s.openAISnapshotCacheStub.GetSnapshot(ctx, bucket)
}

func (s *windowedOpenAISnapshotCacheStub) GetSnapshotWindow(ctx context.Context, bucket SchedulerBucket, offset, limit int) ([]*Account, bool, error) {
	s.windowCalls = append(s.windowCalls, openAIWindowCall{offset: offset, limit: limit})
	if s.miss {
		return nil, false, nil
	}
	window := sliceAccountsWindow(derefAccounts(s.snapshotAccounts), offset, limit)
	out := make([]*Account, 0, len(window))
	for i := range window {
		account := window[i]
		out = append(out, &account)
	}
	return out, true, nil
}

type windowedOpenAIRepoStub struct {
	stubOpenAIAccountRepo
	fullCalls   int
	windowCalls []openAIWindowCall
}

func (r *windowedOpenAIRepoStub) ListSchedulableByGroupIDAndPlatform(ctx context.Context, groupID int64, platform string) ([]Account, error) {
	r.fullCalls++
	return nil, errors.New("unexpected full schedulable list call")
}

func (r *windowedOpenAIRepoStub) ListSchedulableByGroupIDAndPlatformWindow(ctx context.Context, groupID int64, platform string, offset, limit int) ([]Account, error) {
	r.windowCalls = append(r.windowCalls, openAIWindowCall{offset: offset, limit: limit})
	var filtered []Account
	for _, acc := range r.accounts {
		if acc.Platform == platform {
			filtered = append(filtered, acc)
		}
	}
	return sliceAccountsWindow(filtered, offset, limit), nil
}

func (r *windowedOpenAIRepoStub) ListSchedulableByPlatformWindow(ctx context.Context, platform string, offset, limit int) ([]Account, error) {
	return nil, errors.New("unexpected platform window call")
}

func (r *windowedOpenAIRepoStub) ListSchedulableByPlatformsWindow(ctx context.Context, platforms []string, offset, limit int) ([]Account, error) {
	return nil, errors.New("unexpected platforms window call")
}

func (r *windowedOpenAIRepoStub) ListSchedulableByGroupIDAndPlatformsWindow(ctx context.Context, groupID int64, platforms []string, offset, limit int) ([]Account, error) {
	return nil, errors.New("unexpected group platforms window call")
}

func (r *windowedOpenAIRepoStub) ListSchedulableUngroupedByPlatformWindow(ctx context.Context, platform string, offset, limit int) ([]Account, error) {
	return nil, errors.New("unexpected ungrouped platform window call")
}

func (r *windowedOpenAIRepoStub) ListSchedulableUngroupedByPlatformsWindow(ctx context.Context, platforms []string, offset, limit int) ([]Account, error) {
	return nil, errors.New("unexpected ungrouped platforms window call")
}

func TestOpenAIGatewayService_SelectAccountWithLoadAwareness_UsesWindowedSnapshotCandidates(t *testing.T) {
	groupID := int64(42)
	accounts := []*Account{
		{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 10},
		{ID: 2, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 10},
		{ID: 3, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 10},
		{ID: 4, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 10},
		{ID: 5, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 10},
		{ID: 6, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 10},
	}
	accountsByID := make(map[int64]*Account, len(accounts))
	loadMap := make(map[int64]*AccountLoadInfo, len(accounts))
	for _, account := range accounts {
		cloned := *account
		accountsByID[account.ID] = &cloned
		loadMap[account.ID] = &AccountLoadInfo{AccountID: account.ID, LoadRate: 90}
	}

	cfg := &config.Config{}
	cfg.Gateway.Scheduling.LoadBatchEnabled = true
	cfg.Gateway.Scheduling.CandidatePageSize = 2
	cfg.Gateway.Scheduling.CandidateScanLimit = 6
	cfg.Gateway.Scheduling.StickySessionMaxWaiting = 3
	cfg.Gateway.Scheduling.StickySessionWaitTimeout = 45 * time.Second
	cfg.Gateway.Scheduling.FallbackWaitTimeout = 30 * time.Second
	cfg.Gateway.Scheduling.FallbackMaxWaiting = 100

	snapshotCache := &windowedOpenAISnapshotCacheStub{
		openAISnapshotCacheStub: openAISnapshotCacheStub{
			snapshotAccounts: accounts,
			accountsByID:     accountsByID,
		},
	}
	snapshotSvc := &SchedulerSnapshotService{cache: snapshotCache}
	svc := &OpenAIGatewayService{
		cache:              &stubGatewayCache{},
		cfg:                cfg,
		schedulerSnapshot:  snapshotSvc,
		concurrencyService: NewConcurrencyService(stubConcurrencyCache{loadMap: loadMap, skipDefaultLoad: true}),
	}

	const sessionHash = "bounded-window-seed"
	pageSize, _, startPage := svc.openAICandidateWindowPlan(&groupID, sessionHash, "")
	startOffset := startPage * pageSize
	expectedID := int64(startOffset + 2)
	loadMap[expectedID] = &AccountLoadInfo{AccountID: expectedID, LoadRate: 0}

	selection, err := svc.SelectAccountWithLoadAwareness(context.Background(), &groupID, sessionHash, "", nil)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.NotNil(t, selection.Account)
	require.Equal(t, expectedID, selection.Account.ID)
	require.Equal(t, 0, snapshotCache.fullCalls)
	require.Equal(t, []openAIWindowCall{{offset: startOffset, limit: pageSize}}, snapshotCache.windowCalls)
}

func TestOpenAIGatewayService_SelectAccountWithScheduler_UsesWindowedSnapshotCandidates(t *testing.T) {
	groupID := int64(77)
	accounts := []*Account{
		{ID: 11, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 10},
		{ID: 12, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 10},
		{ID: 13, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 10},
		{ID: 14, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 10},
		{ID: 15, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 10},
		{ID: 16, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 10},
	}
	accountsByID := make(map[int64]*Account, len(accounts))
	loadMap := make(map[int64]*AccountLoadInfo, len(accounts))
	for _, account := range accounts {
		cloned := *account
		accountsByID[account.ID] = &cloned
		loadMap[account.ID] = &AccountLoadInfo{AccountID: account.ID, LoadRate: 80}
	}

	cfg := &config.Config{}
	cfg.Gateway.Scheduling.LoadBatchEnabled = true
	cfg.Gateway.Scheduling.CandidatePageSize = 2
	cfg.Gateway.Scheduling.CandidateScanLimit = 6
	cfg.Gateway.Scheduling.StickySessionMaxWaiting = 3
	cfg.Gateway.Scheduling.StickySessionWaitTimeout = 45 * time.Second
	cfg.Gateway.Scheduling.FallbackWaitTimeout = 30 * time.Second
	cfg.Gateway.Scheduling.FallbackMaxWaiting = 100
	cfg.Gateway.OpenAIWS.LBTopK = 1
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.Priority = 1
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.Load = 1
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.Queue = 1
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.ErrorRate = 1
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.TTFT = 1

	snapshotCache := &windowedOpenAISnapshotCacheStub{
		openAISnapshotCacheStub: openAISnapshotCacheStub{
			snapshotAccounts: accounts,
			accountsByID:     accountsByID,
		},
	}
	snapshotSvc := &SchedulerSnapshotService{cache: snapshotCache}
	svc := &OpenAIGatewayService{
		cache:              &stubGatewayCache{},
		cfg:                cfg,
		schedulerSnapshot:  snapshotSvc,
		concurrencyService: NewConcurrencyService(stubConcurrencyCache{loadMap: loadMap, skipDefaultLoad: true}),
	}

	const sessionHash = "bounded-scheduler-seed"
	pageSize, _, startPage := svc.openAICandidateWindowPlan(&groupID, sessionHash, "")
	startOffset := startPage * pageSize
	expectedID := int64(startOffset + 12)
	loadMap[expectedID] = &AccountLoadInfo{AccountID: expectedID, LoadRate: 0}

	selection, decision, err := svc.SelectAccountWithScheduler(
		context.Background(),
		&groupID,
		"",
		sessionHash,
		"",
		nil,
		OpenAIUpstreamTransportAny,
	)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.NotNil(t, selection.Account)
	require.Equal(t, expectedID, selection.Account.ID)
	require.Equal(t, openAIAccountScheduleLayerLoadBalance, decision.Layer)
	require.Equal(t, pageSize, decision.CandidateCount)
	require.Equal(t, 0, snapshotCache.fullCalls)
	require.Equal(t, []openAIWindowCall{{offset: startOffset, limit: pageSize}}, snapshotCache.windowCalls)
}

func TestSchedulerSnapshotService_ListSchedulableAccountsWindow_UsesWindowedDBFallback(t *testing.T) {
	groupID := int64(123)
	repo := &windowedOpenAIRepoStub{
		stubOpenAIAccountRepo: stubOpenAIAccountRepo{
			accounts: []Account{
				{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 1},
				{ID: 2, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 2},
				{ID: 3, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 3},
				{ID: 4, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 4},
			},
		},
	}
	snapshotCache := &windowedOpenAISnapshotCacheStub{miss: true}
	cfg := &config.Config{}
	cfg.Gateway.Scheduling.DbFallbackEnabled = true
	service := &SchedulerSnapshotService{
		cache:       snapshotCache,
		accountRepo: repo,
		cfg:         cfg,
	}

	accounts, useMixed, err := service.ListSchedulableAccountsWindow(context.Background(), &groupID, PlatformOpenAI, false, 2, 2)
	require.NoError(t, err)
	require.False(t, useMixed)
	require.Len(t, accounts, 2)
	require.Equal(t, []int64{3, 4}, []int64{accounts[0].ID, accounts[1].ID})
	require.Equal(t, 0, repo.fullCalls)
	require.Equal(t, []openAIWindowCall{{offset: 2, limit: 2}}, repo.windowCalls)
	require.Equal(t, []openAIWindowCall{{offset: 2, limit: 2}}, snapshotCache.windowCalls)
}
