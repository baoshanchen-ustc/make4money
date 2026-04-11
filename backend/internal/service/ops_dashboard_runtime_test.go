package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestOpsServiceGetDashboardOverview_IncludesRuntimeAnomalies(t *testing.T) {
	now := time.Now().UTC()
	repo := &opsRepoMock{
		GetDashboardOverviewFn: func(ctx context.Context, filter *OpsDashboardFilter) (*OpsDashboardOverview, error) {
			return &OpsDashboardOverview{}, nil
		},
		ListSystemLogsFn: func(ctx context.Context, filter *OpsSystemLogFilter) (*OpsSystemLogList, error) {
			switch filter.Component {
			case OpsRuntimeBillingCompensationSummaryComponent:
				return &OpsSystemLogList{
					Logs: []*OpsSystemLog{
						{ID: 6, Component: filter.Component, Message: "billing summary", CreatedAt: now.Add(5 * time.Second)},
					},
					Total:    1,
					Page:     1,
					PageSize: 5,
				}, nil
			case "ops.runtime.scheduler_outbox.summary":
				return &OpsSystemLogList{
					Logs: []*OpsSystemLog{
						{ID: 2, Component: filter.Component, Message: "outbox summary", CreatedAt: now},
					},
					Total:    1,
					Page:     1,
					PageSize: 5,
				}, nil
			case "ops.runtime.usage_worker.summary":
				return &OpsSystemLogList{
					Logs: []*OpsSystemLog{
						{ID: 4, Component: filter.Component, Message: "worker summary", CreatedAt: now.Add(-30 * time.Second)},
					},
					Total:    1,
					Page:     1,
					PageSize: 5,
				}, nil
			case "ops.runtime.redis_pool.summary":
				return &OpsSystemLogList{
					Logs: []*OpsSystemLog{
						{ID: 5, Component: filter.Component, Message: "redis summary", CreatedAt: now.Add(-45 * time.Second)},
					},
					Total:    1,
					Page:     1,
					PageSize: 5,
				}, nil
			case OpsRuntimeStorageGovernanceSummaryComponent:
				return &OpsSystemLogList{
					Logs: []*OpsSystemLog{
						{ID: 7, Component: filter.Component, Message: "storage governance summary", CreatedAt: now.Add(-15 * time.Second)},
					},
					Total:    1,
					Page:     1,
					PageSize: 5,
				}, nil
			case "ops.runtime.cleanup.summary":
				return &OpsSystemLogList{
					Logs: []*OpsSystemLog{
						{ID: 8, Component: filter.Component, Message: "cleanup summary", CreatedAt: now.Add(-10 * time.Second)},
					},
					Total:    1,
					Page:     1,
					PageSize: 5,
				}, nil
			case "ops.runtime.cleanup.usage.summary":
				return &OpsSystemLogList{
					Logs: []*OpsSystemLog{
						{ID: 9, Component: filter.Component, Message: "usage cleanup summary", CreatedAt: now.Add(-5 * time.Second)},
					},
					Total:    1,
					Page:     1,
					PageSize: 5,
				}, nil
			case "ops.runtime.usage_log.summary":
				return &OpsSystemLogList{
					Logs: []*OpsSystemLog{
						{ID: 1, Component: filter.Component, Message: "usage summary", CreatedAt: now.Add(-1 * time.Minute)},
					},
					Total:    1,
					Page:     1,
					PageSize: 5,
				}, nil
			default:
				return &OpsSystemLogList{}, nil
			}
		},
	}
	repo.GetLatestSystemMetricsFn = func(ctx context.Context, windowMinutes int) (*OpsSystemMetricsSnapshot, error) {
		return nil, errors.New("system metrics missing")
	}
	lastRun := now.Add(-2 * time.Minute)
	lastSuccess := now.Add(-1 * time.Minute)
	lastDuration := int64(1234)
	lastResult := "error_logs=1 system_logs=2"
	repo.ListJobHeartbeatsFn = func(ctx context.Context) ([]*OpsJobHeartbeat, error) {
		return []*OpsJobHeartbeat{
			{
				JobName:        opsCleanupJobName,
				LastRunAt:      &lastRun,
				LastSuccessAt:  &lastSuccess,
				LastDurationMs: &lastDuration,
				LastResult:     &lastResult,
				UpdatedAt:      now,
			},
		}, nil
	}
	svc := &OpsService{
		opsRepo: repo,
		cfg:     &testConfigOpsEnabled,
	}
	svc.tokenRefreshSummaryFn = func(ctx context.Context, platformFilter string, groupIDFilter *int64) *OpsTokenRefreshSummary {
		return &OpsTokenRefreshSummary{
			Total: 1,
			Platform: map[string]int64{
				"openai": 1,
			},
			Group: map[int64]int64{
				1: 1,
			},
		}
	}
	resetSchedulerOutboxRuntimeMetricsForTest()
	schedulerOutboxCheckpointFallbackTotal.Store(3)
	schedulerOutboxCheckpointReadFailureTotal.Store(1)
	schedulerOutboxCheckpointWriteFailureTotal.Store(2)
	schedulerOutboxLastCheckpointWatermark.Store(777)

	out, err := svc.GetDashboardOverview(context.Background(), &OpsDashboardFilter{
		StartTime: now.Add(-1 * time.Hour),
		EndTime:   now,
	})
	require.NoError(t, err)
	expectedComponents := []string{
		OpsRuntimeBillingCompensationSummaryComponent,
		OpsRuntimeSchedulerOutboxSummaryComponent,
		OpsRuntimeUsageWorkerSummaryComponent,
		OpsRuntimeStorageGovernanceSummaryComponent,
		"ops.runtime.cleanup.summary",
		"ops.runtime.cleanup.usage.summary",
	}
	require.Len(t, out.RuntimeAnomalies, opsDashboardRuntimeAnomalyLimit)
	componentCounts := make(map[string]int)
	for _, log := range out.RuntimeAnomalies {
		componentCounts[log.Component]++
	}
	for _, comp := range expectedComponents {
		require.Greater(t, componentCounts[comp], 0)
	}
	require.GreaterOrEqual(t, len(out.Observability), 2)
	titles := make([]string, 0, len(out.Observability))
	for _, note := range out.Observability {
		titles = append(titles, note.Title)
	}
	require.Contains(t, titles, "SQL metrics unavailable")
	require.Contains(t, titles, "Raw query fallback")
	require.Contains(t, titles, tokenRefreshObservabilityTitle)
	require.Contains(t, titles, schedulerCheckpointObservabilityTitle)
	require.NotNil(t, out.TokenRefreshSummary)
	require.Equal(t, int64(1), out.TokenRefreshSummary.Total)
	require.Equal(t, int64(1), out.TokenRefreshSummary.Platform["openai"])
	require.NotNil(t, out.SchedulerCheckpoint)
	require.Equal(t, int64(777), out.SchedulerCheckpoint.LastCheckpointWatermark)
	require.Equal(t, int64(3), out.SchedulerCheckpoint.CheckpointFallbackTotal)
	require.Equal(t, int64(1), out.SchedulerCheckpoint.CheckpointReadFailures)
	require.Equal(t, int64(2), out.SchedulerCheckpoint.CheckpointWriteFailures)
	require.NotNil(t, out.ResourceBudgetSummary)
	require.NotNil(t, out.ResourceBudgetSummary.StorageGovernance)
	require.NotNil(t, out.CleanupStats)
	require.NotNil(t, out.UsageCleanupStats)
	require.NotNil(t, out.StorageGovernance)
	require.NotNil(t, out.StorageGovernance.OpsCleanup)
	require.NotNil(t, out.StorageGovernance.UsageCleanup)
	require.NotNil(t, out.StorageGovernance.OpsCleanup.Heartbeat)
	require.Equal(t, lastDuration, *out.StorageGovernance.OpsCleanup.Heartbeat.LastDurationMs)
	require.NotNil(t, out.SlowPathDiagnostics)
	require.False(t, out.SlowPathDiagnostics.SQLObservabilityReady)
	require.Equal(t, "/api/v1/admin/ops/requests?sort=duration_desc&min_duration_ms=1000", out.SlowPathDiagnostics.RequestDetailsEndpoint)
}

func TestOpsServiceGetDashboardOverview_RuntimeLogErrorsDoNotDropAnomalies(t *testing.T) {
	now := time.Now().UTC()
	repo := &opsRepoMock{
		GetDashboardOverviewFn: func(ctx context.Context, filter *OpsDashboardFilter) (*OpsDashboardOverview, error) {
			return &OpsDashboardOverview{}, nil
		},
		ListSystemLogsFn: runtimeLogsWithBillingFailure(now),
	}
	svc := &OpsService{
		opsRepo: repo,
		cfg:     &testConfigOpsEnabled,
	}

	out, err := svc.GetDashboardOverview(context.Background(), &OpsDashboardFilter{
		StartTime: now.Add(-1 * time.Hour),
		EndTime:   now,
	})
	require.NoError(t, err)
	expectedComponents := []string{
		OpsRuntimeBillingCompensationSummaryComponent,
		OpsRuntimeSchedulerOutboxSummaryComponent,
		OpsRuntimeUsageWorkerSummaryComponent,
		OpsRuntimeRedisPoolSummaryComponent,
		OpsRuntimeStorageGovernanceSummaryComponent,
		OpsRuntimeUsageLogSummaryComponent,
	}
	require.Len(t, out.RuntimeAnomalies, len(expectedComponents))
	componentCounts := make(map[string]int)
	for _, log := range out.RuntimeAnomalies {
		componentCounts[log.Component]++
	}
	for _, comp := range expectedComponents {
		require.Greater(t, componentCounts[comp], 0)
	}
	require.NoError(t, err)
}

func TestOpsServiceListRecentRuntimeAnomalies_PartialComponentFailure(t *testing.T) {
	now := time.Now().UTC()
	repo := &opsRepoMock{
		ListSystemLogsFn: runtimeLogsWithBillingFailure(now),
	}
	svc := &OpsService{
		opsRepo: repo,
	}

	anomalies, err := svc.listRecentRuntimeAnomalies(context.Background(), &OpsDashboardFilter{
		StartTime: now.Add(-1 * time.Hour),
		EndTime:   now,
	}, opsDashboardRuntimeAnomalyLimit)
	require.Error(t, err)
	require.NotNil(t, anomalies)
	expectedComponents := []string{
		OpsRuntimeBillingCompensationSummaryComponent,
		OpsRuntimeSchedulerOutboxSummaryComponent,
		OpsRuntimeUsageWorkerSummaryComponent,
		OpsRuntimeRedisPoolSummaryComponent,
		OpsRuntimeStorageGovernanceSummaryComponent,
		OpsRuntimeUsageLogSummaryComponent,
	}
	require.Len(t, anomalies, len(expectedComponents))
	componentCounts := make(map[string]int)
	for _, log := range anomalies {
		componentCounts[log.Component]++
	}
	for _, comp := range expectedComponents {
		require.Greater(t, componentCounts[comp], 0)
	}
	require.ErrorContains(t, err, "billing_compensation")
}

func TestOpsServiceGetDashboardOverview_ResourceBudgetNotices(t *testing.T) {
	now := time.Now().UTC()
	repo := &opsRepoMock{
		GetDashboardOverviewFn: func(ctx context.Context, filter *OpsDashboardFilter) (*OpsDashboardOverview, error) {
			return &OpsDashboardOverview{}, nil
		},
		ListSystemLogsFn: runtimeLogsWithBillingFailure(now),
	}
	svc := &OpsService{
		opsRepo: repo,
		cfg: &config.Config{
			Ops:      config.OpsConfig{Enabled: true},
			Database: config.DatabaseConfig{MaxOpenConns: 800},
			Redis: config.RedisConfig{
				PoolSize:     1024,
				MinIdleConns: 900,
			},
			Gateway: config.GatewayConfig{
				MaxIdleConns:        1500,
				MaxIdleConnsPerHost: 900,
				MaxConnsPerHost:     1600,
				MaxUpstreamClients:  9000,
			},
		},
	}

	filter := &OpsDashboardFilter{
		StartTime: now.Add(-1 * time.Hour),
		EndTime:   now,
	}
	out, err := svc.GetDashboardOverview(context.Background(), filter)
	require.NoError(t, err)
	require.NotEmpty(t, out.Observability)
	require.NotNil(t, out.ResourceBudgetSummary)
	require.NotNil(t, out.ResourceBudgetSummary.Database)
	require.NotNil(t, out.ResourceBudgetSummary.Database.MaxOpenConns)
	require.Equal(t, 800, *out.ResourceBudgetSummary.Database.MaxOpenConns)
	require.NotNil(t, out.ResourceBudgetSummary.Redis)
	require.NotNil(t, out.ResourceBudgetSummary.Redis.PoolSize)
	require.Equal(t, 1024, *out.ResourceBudgetSummary.Redis.PoolSize)
	require.NotNil(t, out.ResourceBudgetSummary.HTTPUpstream)
	require.NotNil(t, out.ResourceBudgetSummary.HTTPUpstream.MaxUpstreamClients)
	require.Equal(t, 9000, *out.ResourceBudgetSummary.HTTPUpstream.MaxUpstreamClients)
	require.NotNil(t, out.StorageGovernance)
	require.NotNil(t, out.StorageGovernance.OpsCleanup)
	require.Equal(t, 0, out.StorageGovernance.OpsCleanup.ErrorLogRetentionDays)
	require.NotNil(t, out.StorageGovernance.UsageCleanup)
	require.NotNil(t, out.SlowPathDiagnostics)
	require.True(t, out.SlowPathDiagnostics.SQLObservabilityReady)
	require.NotEmpty(t, out.ResourceBudgetSummary.Recommendations)
	foundRecommendationAreas := make(map[string]bool)
	for _, rec := range out.ResourceBudgetSummary.Recommendations {
		if rec != nil {
			foundRecommendationAreas[rec.Area] = true
		}
	}
	require.True(t, foundRecommendationAreas["database"])
	require.True(t, foundRecommendationAreas["redis"])
	require.True(t, foundRecommendationAreas["http_upstream"])
	foundDB := false
	foundRedis := false
	foundHTTP := false
	for _, note := range out.Observability {
		switch {
		case note.Title == "Database pool sized aggressively":
			foundDB = true
		case note.Title == "Redis pool configured high":
			foundRedis = true
		case note.Title == "HTTP upstream pool configured aggressively":
			foundHTTP = true
		}
	}
	require.True(t, foundDB, "expected database pool notice")
	require.True(t, foundRedis, "expected redis pool notice")
	require.True(t, foundHTTP, "expected http pool notice")
}

func TestOpsServiceGetDashboardOverview_RawQueryObservabilityNote(t *testing.T) {
	now := time.Now().UTC()
	repo := &opsRepoMock{
		GetDashboardOverviewFn: func(ctx context.Context, filter *OpsDashboardFilter) (*OpsDashboardOverview, error) {
			return &OpsDashboardOverview{}, nil
		},
	}
	repo.GetLatestSystemMetricsFn = func(ctx context.Context, windowMinutes int) (*OpsSystemMetricsSnapshot, error) {
		dbOK := true
		return &OpsSystemMetricsSnapshot{
			DBOK: &dbOK,
		}, nil
	}
	svc := &OpsService{
		opsRepo: repo,
		cfg:     &testConfigOpsEnabled,
	}

	filter := &OpsDashboardFilter{
		StartTime: now.Add(-1 * time.Hour),
		EndTime:   now,
		QueryMode: OpsQueryModeRaw,
	}
	out, err := svc.GetDashboardOverview(context.Background(), filter)
	require.NoError(t, err)
	require.NotEmpty(t, out.Observability)
	found := false
	for _, note := range out.Observability {
		if note != nil && note.Title == "Raw query fallback" {
			found = true
			break
		}
	}
	require.True(t, found, "expected raw query observability note")
}

func TestOpsServiceGetDashboardOverview_TokenRefreshObservabilityNote(t *testing.T) {
	now := time.Now().UTC()
	repo := &opsRepoMock{
		GetDashboardOverviewFn: func(ctx context.Context, filter *OpsDashboardFilter) (*OpsDashboardOverview, error) {
			return &OpsDashboardOverview{}, nil
		},
	}
	repo.GetLatestSystemMetricsFn = func(ctx context.Context, windowMinutes int) (*OpsSystemMetricsSnapshot, error) {
		dbOK := true
		return &OpsSystemMetricsSnapshot{
			DBOK: &dbOK,
		}, nil
	}
	svc := &OpsService{
		opsRepo: repo,
		cfg:     &testConfigOpsEnabled,
	}
	svc.tokenRefreshSummaryFn = func(ctx context.Context, platformFilter string, groupIDFilter *int64) *OpsTokenRefreshSummary {
		return &OpsTokenRefreshSummary{
			Total: 2,
			Platform: map[string]int64{
				"openai": 1,
				"claude": 1,
			},
		}
	}

	filter := &OpsDashboardFilter{
		StartTime: now.Add(-1 * time.Hour),
		EndTime:   now,
		QueryMode: OpsQueryModePreagg,
	}
	out, err := svc.GetDashboardOverview(context.Background(), filter)
	require.NoError(t, err)
	require.True(t, containsObservabilityTitle(out.Observability, tokenRefreshObservabilityTitle))
}

func TestOpsServiceGetDashboardOverview_SchedulerCheckpointObservabilityNote(t *testing.T) {
	now := time.Now().UTC()
	repo := &opsRepoMock{
		GetDashboardOverviewFn: func(ctx context.Context, filter *OpsDashboardFilter) (*OpsDashboardOverview, error) {
			return &OpsDashboardOverview{}, nil
		},
	}
	repo.GetLatestSystemMetricsFn = func(ctx context.Context, windowMinutes int) (*OpsSystemMetricsSnapshot, error) {
		dbOK := true
		return &OpsSystemMetricsSnapshot{
			DBOK: &dbOK,
		}, nil
	}
	svc := &OpsService{
		opsRepo: repo,
		cfg:     &testConfigOpsEnabled,
	}

	resetSchedulerOutboxRuntimeMetricsForTest()
	schedulerOutboxCheckpointFallbackTotal.Store(4)
	schedulerOutboxCheckpointReadFailureTotal.Store(2)
	schedulerOutboxCheckpointWriteFailureTotal.Store(1)
	schedulerOutboxLastCheckpointWatermark.Store(8888)

	filter := &OpsDashboardFilter{
		StartTime: now.Add(-1 * time.Hour),
		EndTime:   now,
		QueryMode: OpsQueryModePreagg,
	}
	out, err := svc.GetDashboardOverview(context.Background(), filter)
	require.NoError(t, err)
	require.True(t, containsObservabilityTitle(out.Observability, schedulerCheckpointObservabilityTitle))
}

func runtimeLogsWithBillingFailure(now time.Time) func(ctx context.Context, filter *OpsSystemLogFilter) (*OpsSystemLogList, error) {
	return func(ctx context.Context, filter *OpsSystemLogFilter) (*OpsSystemLogList, error) {
		switch filter.Component {
		case OpsRuntimeBillingCompensationComponent:
			return nil, errors.New("billing log failure")
		case OpsRuntimeBillingCompensationSummaryComponent:
			return &OpsSystemLogList{
				Logs: []*OpsSystemLog{
					{ID: 6, Component: filter.Component, Message: "billing summary", CreatedAt: now.Add(5 * time.Second)},
				},
				Total:    1,
				Page:     1,
				PageSize: 5,
			}, nil
		case OpsRuntimeSchedulerOutboxSummaryComponent:
			return &OpsSystemLogList{
				Logs: []*OpsSystemLog{
					{ID: 2, Component: filter.Component, Message: "outbox summary", CreatedAt: now},
				},
				Total:    1,
				Page:     1,
				PageSize: 5,
			}, nil
		case OpsRuntimeUsageWorkerSummaryComponent:
			return &OpsSystemLogList{
				Logs: []*OpsSystemLog{
					{ID: 4, Component: filter.Component, Message: "usage worker summary", CreatedAt: now.Add(-15 * time.Second)},
				},
				Total:    1,
				Page:     1,
				PageSize: 5,
			}, nil
		case OpsRuntimeRedisPoolSummaryComponent:
			return &OpsSystemLogList{
				Logs: []*OpsSystemLog{
					{ID: 5, Component: filter.Component, Message: "redis pool summary", CreatedAt: now.Add(-10 * time.Second)},
				},
				Total:    1,
				Page:     1,
				PageSize: 5,
			}, nil
		case OpsRuntimeStorageGovernanceSummaryComponent:
			return &OpsSystemLogList{
				Logs: []*OpsSystemLog{
					{ID: 7, Component: filter.Component, Message: "storage governance summary", CreatedAt: now.Add(-20 * time.Second)},
				},
				Total:    1,
				Page:     1,
				PageSize: 5,
			}, nil
		case OpsRuntimeUsageLogSummaryComponent:
			return &OpsSystemLogList{
				Logs: []*OpsSystemLog{
					{ID: 1, Component: filter.Component, Message: "usage summary", CreatedAt: now.Add(-45 * time.Second)},
				},
				Total:    1,
				Page:     1,
				PageSize: 5,
			}, nil
		default:
			return &OpsSystemLogList{}, nil
		}
	}
}

func containsObservabilityTitle(notes []*OpsObservabilityNotice, title string) bool {
	if len(notes) == 0 {
		return false
	}
	for _, note := range notes {
		if note != nil && note.Title == title {
			return true
		}
	}
	return false
}

var testConfigOpsEnabled = func() config.Config {
	cfg := config.Config{}
	cfg.Ops.Enabled = true
	return cfg
}()
