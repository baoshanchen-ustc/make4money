package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type stubOpenAIAccountScheduler struct {
	snapshot OpenAIAccountSchedulerMetricsSnapshot
}

func (s *stubOpenAIAccountScheduler) Select(ctx context.Context, req OpenAIAccountScheduleRequest) (*AccountSelectionResult, OpenAIAccountScheduleDecision, error) {
	return nil, OpenAIAccountScheduleDecision{}, nil
}

func (s *stubOpenAIAccountScheduler) ReportResult(accountID int64, success bool, firstTokenMs *int) {}

func (s *stubOpenAIAccountScheduler) ReportSwitch() {}

func (s *stubOpenAIAccountScheduler) SnapshotMetrics() OpenAIAccountSchedulerMetricsSnapshot {
	return s.snapshot
}

func TestOpsServiceGetDashboardOverview_IncludesRuntimeAnomalies(t *testing.T) {
	now := time.Now().UTC()
	resetStickySessionCleanupMetricsForTest()
	resetSchedulerOutboxRuntimeMetricsForTest()
	repo := &opsRepoMock{
		GetDashboardOverviewFn: func(ctx context.Context, filter *OpsDashboardFilter) (*OpsDashboardOverview, error) {
			return &OpsDashboardOverview{DataSource: &OpsDataSourceSummary{Mode: string(OpsQueryModeRaw)}}, nil
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
		opsRepo:              repo,
		cfg:                  &testConfigOpsEnabled,
		openAIGatewayService: &OpenAIGatewayService{},
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
	svc.openAIGatewayService.openaiScheduler = &stubOpenAIAccountScheduler{
		snapshot: OpenAIAccountSchedulerMetricsSnapshot{
			SelectTotal:                    8,
			StickySessionLookupTotal:       5,
			StickySessionWaitTotal:         1,
			StickySessionWaitConflictTotal: 1,
			AccountSwitchTotal:             2,
			StickyHitRatio:                 0.5,
			AccountSwitchRate:              0.25,
			StickySessionShadowReasonTotals: map[string]int64{
				openAIStickySessionShadowReasonClearedUnschedulable: 1,
				openAIStickySessionShadowReasonAccountMissing:       1,
			},
			StickySessionGhostTotal:        1,
			StickySessionStaleTotal:        1,
			StickyGhostRatio:               0.2,
			StickyStaleRatio:               0.2,
			StickyWaitConflictRatio:        0.2,
			StickySessionAccountFetchTotal: 5,
			StickySessionDBRecheckTotal:    3,
			LoadBatchCallTotal:             2,
			LoadBatchCandidateTotal:        11,
			LoadBatchFallbackTotal:         1,
		},
	}
	beforeStickyCleanup := SnapshotStickySessionCleanupMetrics()
	recordStickySessionCleanupReason(stickyCleanupReasonBindingInvalidated)
	recordStickySessionCompareDeleteMissReason(stickyCleanupReasonBindingInvalidated)
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
	require.Contains(t, titles, stickyCleanupObservabilityTitle)
	require.Contains(t, titles, "OpenAI sticky-session pressure detected")
	require.NotNil(t, out.TokenRefreshSummary)
	require.Equal(t, int64(1), out.TokenRefreshSummary.Total)
	require.Equal(t, int64(1), out.TokenRefreshSummary.Platform["openai"])
	require.NotNil(t, out.OpenAIAccountScheduler)
	require.Equal(t, int64(8), out.OpenAIAccountScheduler.SelectTotal)
	require.Equal(t, int64(1), out.OpenAIAccountScheduler.ShadowReasonTotals[openAIStickySessionShadowReasonClearedUnschedulable])
	require.Equal(t, int64(1), out.OpenAIAccountScheduler.StickySessionGhostTotal)
	require.Equal(t, int64(1), out.OpenAIAccountScheduler.StickySessionStaleTotal)
	require.Equal(t, int64(1), out.OpenAIAccountScheduler.StickyWaitConflictTotal)
	require.Equal(t, int64(1), out.OpenAIAccountScheduler.CompareDeleteMissTotal)
	require.Equal(t, int64(5), out.OpenAIAccountScheduler.StickySessionAccountFetchTotal)
	require.Equal(t, int64(3), out.OpenAIAccountScheduler.StickySessionDBRecheckTotal)
	require.Equal(t, int64(2), out.OpenAIAccountScheduler.LoadBatchCallTotal)
	require.Equal(t, int64(11), out.OpenAIAccountScheduler.LoadBatchCandidateTotal)
	require.Equal(t, int64(1), out.OpenAIAccountScheduler.LoadBatchFallbackTotal)
	require.NotNil(t, out.OpenAIAccountScheduler.CleanupReasonTotals)
	require.NotNil(t, out.OpenAIAccountScheduler.CompareDeleteMissReasonTotals)
	require.NotNil(t, out.StickySessionCleanup)
	require.GreaterOrEqual(t, out.StickySessionCleanup.CleanupTotal, beforeStickyCleanup.CleanupTotal+1)
	require.GreaterOrEqual(t, out.StickySessionCleanup.CompareDeleteMissTotal, beforeStickyCleanup.CompareDeleteMissTotal+1)
	require.NotNil(t, out.SchedulerCheckpoint)
	require.Equal(t, int64(777), out.SchedulerCheckpoint.LastCheckpointWatermark)
	require.Equal(t, int64(3), out.SchedulerCheckpoint.CheckpointFallbackTotal)
	require.Equal(t, int64(1), out.SchedulerCheckpoint.CheckpointReadFailures)
	require.Equal(t, int64(2), out.SchedulerCheckpoint.CheckpointWriteFailures)
	require.NotNil(t, out.DriftTrendSummary)
	require.Equal(t, "warning", out.DriftTrendSummary.Severity)
	require.Equal(t, "lag_streak", out.DriftTrendSummary.SchedulerState)
	require.NotNil(t, out.ResourceBudgetSummary)
	require.NotNil(t, out.ResourceBudgetSummary.StorageGovernance)
	require.NotEmpty(t, out.ResourceBudgetSummary.Guardrails)
	require.NotNil(t, out.ResourceBudgetSummary.WatchThresholds)
	require.Contains(t, out.ResourceBudgetSummary.WatchThresholds, "db_conn_waiting")
	require.NotNil(t, out.ResourceBudgetSummary.SafetyZone)
	require.NotNil(t, out.CleanupStats)
	require.NotNil(t, out.UsageCleanupStats)
	require.NotNil(t, out.StorageGovernance)
	require.NotNil(t, out.StorageGovernance.OpsCleanup)
	require.NotNil(t, out.StorageGovernance.UsageCleanup)
	require.NotNil(t, out.StorageGovernance.DryRun)
	require.NotNil(t, out.StorageGovernance.DryRun.OpsCleanup)
	require.NotNil(t, out.StorageGovernance.DryRun.UsageCleanup)
	require.NotNil(t, out.StorageGovernance.OpsCleanup.Heartbeat)
	require.Equal(t, lastDuration, *out.StorageGovernance.OpsCleanup.Heartbeat.LastDurationMs)
	require.NotNil(t, out.SlowPathDiagnostics)
	require.False(t, out.SlowPathDiagnostics.SQLObservabilityReady)
	require.Equal(t, "/api/v1/admin/ops/requests?sort=duration_desc&min_duration_ms=1000", out.SlowPathDiagnostics.RequestDetailsEndpoint)
	require.NotEmpty(t, out.SlowPathDiagnostics.InvestigationOrder)
	require.NotEmpty(t, out.SlowPathDiagnostics.CorrelationHint)
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
			DashboardAgg: config.DashboardAggregationConfig{
				Retention: config.DashboardAggregationRetentionConfig{
					UsageLogsDays: 30,
				},
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
	require.NotEmpty(t, out.ResourceBudgetSummary.Guardrails)
	require.NotNil(t, out.ResourceBudgetSummary.WatchThresholds)
	require.Contains(t, out.ResourceBudgetSummary.WatchThresholds, "http_pool_saturation")
	require.NotNil(t, out.ResourceBudgetSummary.SafetyZone)
	require.NotEmpty(t, out.ResourceBudgetSummary.JointSignals)
	require.NotNil(t, out.StorageGovernance)
	require.NotNil(t, out.StorageGovernance.OpsCleanup)
	require.Equal(t, 0, out.StorageGovernance.OpsCleanup.ErrorLogRetentionDays)
	require.NotNil(t, out.StorageGovernance.UsageCleanup)
	require.NotNil(t, out.StorageGovernance.DryRun)
	require.NotNil(t, out.StorageGovernance.DryRun.OpsCleanup)
	require.NotNil(t, out.StorageGovernance.DryRun.UsageCleanup)
	require.Equal(t, "retention_only", out.StorageGovernance.DryRun.UsageCleanup.EnforcementMode)
	require.NotNil(t, out.SlowPathDiagnostics)
	require.True(t, out.SlowPathDiagnostics.SQLObservabilityReady)
	require.NotEmpty(t, out.SlowPathDiagnostics.InvestigationOrder)
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
		switch note.Title {
		case "Database pool sized aggressively":
			foundDB = true
		case "Redis pool configured high":
			foundRedis = true
		case "HTTP upstream pool configured aggressively":
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
			return &OpsDashboardOverview{DataSource: &OpsDataSourceSummary{Mode: string(OpsQueryModeRaw)}}, nil
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
	require.NotNil(t, out.DataSource)
	require.Equal(t, string(OpsQueryModeRaw), out.DataSource.Mode)
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

func TestOpsServiceGetDashboardOverview_AutoModeRawDataSourceAddsObservabilityNote(t *testing.T) {
	now := time.Now().UTC()
	repo := &opsRepoMock{
		GetDashboardOverviewFn: func(ctx context.Context, filter *OpsDashboardFilter) (*OpsDashboardOverview, error) {
			return &OpsDashboardOverview{DataSource: &OpsDataSourceSummary{Mode: string(OpsQueryModeRaw)}}, nil
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
		QueryMode: OpsQueryModeAuto,
	}
	out, err := svc.GetDashboardOverview(context.Background(), filter)
	require.NoError(t, err)
	require.NotNil(t, out.DataSource)
	require.Equal(t, string(OpsQueryModeRaw), out.DataSource.Mode)
	require.Equal(t, string(OpsQueryModeAuto), out.DataSource.RequestedMode)
	require.NotNil(t, out.QueryPath)
	require.Equal(t, string(OpsQueryModeRaw), out.QueryPath.Mode)
	require.Equal(t, string(OpsQueryModeAuto), out.QueryPath.RequestedMode)
	require.Equal(t, "auto_fallback_to_raw", out.QueryPath.Reason)
	require.True(t, containsObservabilityTitle(out.Observability, "Raw query fallback"))
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

func TestOpsServiceGetDashboardOverview_AccountAuthSummary(t *testing.T) {
	now := time.Now().UTC()
	svc := &OpsService{
		opsRepo: &opsRepoMock{
			GetDashboardOverviewFn: func(ctx context.Context, filter *OpsDashboardFilter) (*OpsDashboardOverview, error) {
				return &OpsDashboardOverview{}, nil
			},
		},
		accountRepo: &opsAccountAvailabilityRepoStub{
			accounts: []Account{
				{
					ID:          201,
					Name:        "oauth-reauth",
					Platform:    PlatformOpenAI,
					Status:      StatusError,
					Schedulable: true,
					Extra: map[string]any{
						accountAuthFailureReasonKey: "token_revoked",
						accountAuthFailureClassKey:  accountAuthClassPermanent,
						accountAuthFailureSourceKey: accountAuthSourceTokenRefresh,
						accountAuthStateKey:         accountAuthStateReauthorizeRequired,
						accountAuthStateAtKey:       now.Format(time.RFC3339),
						accountAuthRecoveryKey:      accountAuthRecoveryActionReauthorize,
					},
				},
			},
		},
		cfg: &testConfigOpsEnabled,
	}

	out, err := svc.GetDashboardOverview(context.Background(), &OpsDashboardFilter{
		StartTime: now.Add(-time.Hour),
		EndTime:   now,
	})
	require.NoError(t, err)
	require.NotNil(t, out.AccountAuthSummary)
	require.Equal(t, int64(1), out.AccountAuthSummary.Total)
	require.Equal(t, int64(1), out.AccountAuthSummary.PermanentCount)
	require.Equal(t, int64(1), out.AccountAuthSummary.DispatchSuppressed)
	require.Equal(t, int64(1), out.AccountAuthSummary.ManualReviewRequired)
	require.Equal(t, int64(1), out.AccountAuthSummary.ByReason["token_revoked"])
	require.NotNil(t, out.FailureSplitSummary)
	require.Equal(t, int64(1), out.FailureSplitSummary.AccountOrAuth)
	require.Equal(t, "account_or_auth", out.FailureSplitSummary.LikelyPrimary)
	require.Contains(t, out.FailureSplitSummary.Suggestion, "account/auth")
	require.Contains(t, out.FailureSplitSummary.OperatorAction, "auth")

	titles := make([]string, 0, len(out.Observability))
	for _, note := range out.Observability {
		titles = append(titles, note.Title)
	}
	require.Contains(t, titles, "Account auth failures visible")
	require.Contains(t, titles, "Failure split guidance available")
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
	schedulerOutboxLastRedisWatermark.Store(9991)
	schedulerOutboxCheckpointFallbackTotal.Store(4)
	schedulerOutboxCheckpointFallbackStreak.Store(2)
	schedulerOutboxCheckpointReadFailureTotal.Store(2)
	schedulerOutboxCheckpointWriteFailureTotal.Store(1)
	schedulerOutboxLastCheckpointWatermark.Store(8888)
	schedulerOutboxWatermarkDrift.Store(103)
	schedulerOutboxLagFailureStreak.Store(2)
	schedulerOutboxCoalescedBatchTotal.Store(5)
	schedulerOutboxCoalescedEventSavedTotal.Store(11)
	schedulerOutboxBucketRebuildFailureTotal.Store(3)
	schedulerOutboxBucketRebuildLockContention.Store(2)
	schedulerOutboxRebuildCooldownSkipTotal.Store(4)
	schedulerOutboxBusyBucketSkipTotal.Store(6)
	schedulerOutboxRuntimeMu.Lock()
	schedulerOutboxCheckpointLastFallbackAt = now.Add(-2 * time.Minute).UTC().Format(time.RFC3339)
	schedulerOutboxCheckpointLastFallbackReason = "redis_watermark_unavailable"
	schedulerOutboxCheckpointLastReadFailureAt = now.Add(-90 * time.Second).UTC().Format(time.RFC3339)
	schedulerOutboxCheckpointLastWriteFailureAt = now.Add(-45 * time.Second).UTC().Format(time.RFC3339)
	schedulerOutboxLastLagRebuildAt = now.Add(-30 * time.Second).UTC().Format(time.RFC3339)
	schedulerOutboxLastBucketRebuildAt = now.Add(-15 * time.Second).UTC().Format(time.RFC3339)
	schedulerOutboxLastBucketRebuildReason = "outbox_lag"
	schedulerOutboxLastBucketRebuildStatus = "lock_contention"
	schedulerOutboxLastBucketRebuildBucket = "4:openai:single"
	schedulerOutboxBlockedEventLastClearedAt = now.Add(-10 * time.Second).UTC().Format(time.RFC3339)
	schedulerOutboxBlockedEventLastClearReason = "recovered"
	schedulerOutboxBlockedEventLastClearedID = 320
	schedulerOutboxBlockedEvent = &SchedulerOutboxBlockedEvent{
		ID:         321,
		EventType:  SchedulerOutboxEventAccountChanged,
		Reason:     "lock_contention",
		Attempts:   3,
		AgeSeconds: 12,
	}
	schedulerOutboxRuntimeMu.Unlock()

	filter := &OpsDashboardFilter{
		StartTime: now.Add(-1 * time.Hour),
		EndTime:   now,
		QueryMode: OpsQueryModePreagg,
	}
	out, err := svc.GetDashboardOverview(context.Background(), filter)
	require.NoError(t, err)
	require.True(t, containsObservabilityTitle(out.Observability, schedulerCheckpointObservabilityTitle))
	require.NotNil(t, out.SchedulerCheckpoint)
	require.Equal(t, int64(9991), out.SchedulerCheckpoint.RedisWatermark)
	require.Equal(t, int64(103), out.SchedulerCheckpoint.WatermarkDrift)
	require.Equal(t, int64(2), out.SchedulerCheckpoint.CheckpointFallbackStreak)
	require.NotEmpty(t, out.SchedulerCheckpoint.CheckpointLastFallbackAt)
	require.Equal(t, "redis_watermark_unavailable", out.SchedulerCheckpoint.CheckpointLastFallbackReason)
	require.NotEmpty(t, out.SchedulerCheckpoint.CheckpointLastReadFailureAt)
	require.NotEmpty(t, out.SchedulerCheckpoint.CheckpointLastWriteFailureAt)
	require.NotNil(t, out.SchedulerCheckpoint.BlockedEvent)
	require.Equal(t, int64(321), out.SchedulerCheckpoint.BlockedEvent.ID)
	require.Equal(t, int64(12), out.SchedulerCheckpoint.BlockedEvent.AgeSeconds)
	require.NotNil(t, out.SchedulerOutboxRuntime)
	require.Equal(t, int64(9991), out.SchedulerOutboxRuntime.LastRedisWatermark)
	require.Equal(t, int64(2), out.SchedulerOutboxRuntime.LagFailureStreak)
	require.Equal(t, int64(2), out.SchedulerOutboxRuntime.CheckpointFallbackStreak)
	require.Equal(t, "redis_watermark_unavailable", out.SchedulerOutboxRuntime.CheckpointLastFallbackReason)
	require.Equal(t, int64(5), out.SchedulerOutboxRuntime.CoalescedBatchTotal)
	require.Equal(t, int64(11), out.SchedulerOutboxRuntime.CoalescedEventSavedTotal)
	require.Equal(t, int64(3), out.SchedulerOutboxRuntime.BucketRebuildFailureTotal)
	require.Equal(t, int64(2), out.SchedulerOutboxRuntime.BucketRebuildLockContention)
	require.Equal(t, int64(4), out.SchedulerOutboxRuntime.RebuildCooldownSkipTotal)
	require.Equal(t, int64(6), out.SchedulerOutboxRuntime.BusyBucketSkipTotal)
	require.Equal(t, int64(320), out.SchedulerOutboxRuntime.BlockedEventLastClearedID)
	require.Equal(t, "recovered", out.SchedulerOutboxRuntime.BlockedEventLastClearReason)
	require.NotEmpty(t, out.SchedulerOutboxRuntime.LastBucketRebuildAt)
	require.Equal(t, "lock_contention", out.SchedulerOutboxRuntime.LastBucketRebuildStatus)
}

func TestOpsServiceGetDashboardOverview_ControlPlaneDriftObservabilityNote(t *testing.T) {
	now := time.Now().UTC()
	resetStickyConsistencyTracker()
	repo := &opsRepoMock{
		GetDashboardOverviewFn: func(ctx context.Context, filter *OpsDashboardFilter) (*OpsDashboardOverview, error) {
			return &OpsDashboardOverview{}, nil
		},
	}
	repo.GetLatestSystemMetricsFn = func(ctx context.Context, windowMinutes int) (*OpsSystemMetricsSnapshot, error) {
		dbOK := true
		return &OpsSystemMetricsSnapshot{DBOK: &dbOK}, nil
	}
	svc := &OpsService{
		opsRepo: repo,
		cfg:     &testConfigOpsEnabled,
	}

	resetSchedulerOutboxRuntimeMetricsForTest()
	schedulerOutboxWatermarkDrift.Store(9)
	schedulerOutboxBacklogRows.Store(2)
	TrackStickyConsistencyHit()
	TrackStickyConsistencyGhost("transport_mismatch")

	out, err := svc.GetDashboardOverview(context.Background(), &OpsDashboardFilter{
		StartTime: now.Add(-time.Hour),
		EndTime:   now,
	})
	require.NoError(t, err)
	require.True(t, containsObservabilityTitle(out.Observability, "Control-plane drift signals detected"))
	found := false
	for _, note := range out.Observability {
		if note != nil && note.Title == "Control-plane drift signals detected" {
			require.Contains(t, note.Detail, "likely_root_cause=sticky_and_scheduler_drift")
			found = true
		}
	}
	require.True(t, found)
}

func TestOpsServiceGetRealtimeSummaryBundle(t *testing.T) {
	now := time.Now().UTC()
	resetUsageLogNotPersistedAlertsForTest()
	resetBillingCompensationCandidatesForTest()
	resetStickySessionCleanupMetricsForTest()
	resetLatestStorageGovernanceRuntimeForTest()
	resetLatestErrorFamilySummaryForTest()
	resetUsageCleanupStatsForTest()
	lastRun := now.Add(-3 * time.Minute)
	lastSuccess := now.Add(-90 * time.Second)
	lastDuration := int64(321)
	repo := &opsRepoMock{
		GetLatestSystemMetricsFn: func(ctx context.Context, windowMinutes int) (*OpsSystemMetricsSnapshot, error) {
			dbOK := true
			redisOK := true
			return &OpsSystemMetricsSnapshot{
				DBOK:           &dbOK,
				RedisOK:        &redisOK,
				DBConnActive:   intPtr(12),
				DBConnIdle:     intPtr(8),
				DBConnWaiting:  intPtr(1),
				RedisConnTotal: intPtr(48),
				RedisConnIdle:  intPtr(16),
			}, nil
		},
		ListJobHeartbeatsFn: func(ctx context.Context) ([]*OpsJobHeartbeat, error) {
			return []*OpsJobHeartbeat{
				{
					JobName:        opsCleanupJobName,
					LastRunAt:      &lastRun,
					LastSuccessAt:  &lastSuccess,
					LastDurationMs: &lastDuration,
					UpdatedAt:      now,
				},
			}, nil
		},
	}
	svc := &OpsService{
		opsRepo: repo,
		cfg: &config.Config{
			Ops:      config.OpsConfig{Enabled: true},
			Database: config.DatabaseConfig{MaxOpenConns: 64, MaxIdleConns: 16},
			Redis:    config.RedisConfig{PoolSize: 128, MinIdleConns: 24},
		},
	}

	before := SnapshotStickySessionCleanupMetrics()
	recordStickySessionCleanupReason(stickyCleanupReasonTransportMismatch)
	usageLogNotPersistedAlerts.Store(1)
	recordUsageLogNotPersistedAlert(UsageLogNotPersistedAlert{
		Timestamp: now,
		RequestID: "usage-gap-1",
		AccountID: 88,
		Error:     "persist failed",
	})
	recordBillingCompensationCandidate("svc.test", &UsageLog{
		RequestID: "bill-gap-1",
		AccountID: 99,
		Model:     "gpt-test",
	}, errors.New("billing succeeded but usage missing"))
	recordLatestStorageGovernanceRuntimeSnapshot(now, opsStorageGovernanceRuntimeSnapshot{
		SampledTables: 1,
		Tables: []opsStorageGovernanceTableSnapshot{
			{
				Table:             "usage_logs",
				LiveRows:          240000,
				DeadRows:          12000,
				TotalMB:           640,
				Partitioned:       false,
				RetentionDays:     30,
				RetentionEnabled:  true,
				MaxRowsConfigured: 200000,
				MaxRowsEnabled:    true,
				MaxRowsExceeded:   true,
				SizeRisk:          "warn",
				RetentionRisk:     "warn",
				ColdIndexRisk:     "warn",
				Reasons:           []string{"usage_logs_max_rows_exceeded", "non_partitioned_delete_retention"},
			},
		},
	})
	recordLatestErrorFamilySummary(now, opsRuntimeErrorFamilySummary{
		Kind:          "summary",
		WindowMinutes: 15,
		TotalErrors:   6,
		Families: []opsRuntimeErrorFamilyItem{
			{Phase: "upstream", Type: "api_error", Owner: "provider", StatusCode: 503, InboundEndpoint: "/v1/chat/completions", Count: 4, SharePercent: 66.7},
			{Phase: "request", Type: "bad_request", Owner: "client", StatusCode: 400, InboundEndpoint: "/v1/messages", Count: 2, SharePercent: 33.3},
		},
	})

	out := svc.GetRealtimeSummaryBundle(context.Background())
	require.NotNil(t, out)
	require.NotNil(t, out.ResourceBudgetSummary)
	require.NotNil(t, out.ResourceBudgetSummary.Database)
	require.NotNil(t, out.ResourceBudgetSummary.Database.MaxOpenConns)
	require.Equal(t, 64, *out.ResourceBudgetSummary.Database.MaxOpenConns)
	require.NotNil(t, out.ResourceBudgetSummary.Redis)
	require.NotNil(t, out.ResourceBudgetSummary.Redis.PoolSize)
	require.Equal(t, 128, *out.ResourceBudgetSummary.Redis.PoolSize)
	require.NotNil(t, out.ResourceBudgetSummary.WatchThresholds)
	require.NotNil(t, out.ResourceBudgetSummary.SafetyZone)
	require.NotNil(t, out.StorageGovernance)
	require.NotNil(t, out.StorageGovernance.OpsCleanup)
	require.NotNil(t, out.StorageGovernance.OpsCleanup.Heartbeat)
	require.Equal(t, lastDuration, *out.StorageGovernance.OpsCleanup.Heartbeat.LastDurationMs)
	require.NotNil(t, out.StorageGovernance.UsageLogs)
	require.Equal(t, int64(240000), out.StorageGovernance.UsageLogs.EstimatedLiveRows)
	require.True(t, out.StorageGovernance.UsageLogs.MaxRowsExceeded)
	require.NotNil(t, out.StorageGovernance.Runtime)
	require.Equal(t, 1, out.StorageGovernance.Runtime.SampledTables)
	require.NotNil(t, out.ErrorFamilySummary)
	require.Equal(t, int64(6), out.ErrorFamilySummary.TotalErrors)
	require.Len(t, out.ErrorFamilySummary.Families, 2)
	require.NotNil(t, out.FailureSplitSummary)
	require.Equal(t, int64(2), out.FailureSplitSummary.ProtocolOrRequestShape)
	require.Equal(t, int64(4), out.FailureSplitSummary.ProviderOrUpstream)
	require.Equal(t, "provider_or_upstream", out.FailureSplitSummary.LikelyPrimary)
	require.Contains(t, out.FailureSplitSummary.Suggestion, "provider/upstream")
	require.Contains(t, out.FailureSplitSummary.OperatorAction, "upstream")
	require.NotNil(t, out.StickySessionCleanup)
	require.GreaterOrEqual(t, out.StickySessionCleanup.CleanupTotal, before.CleanupTotal+1)
	require.NotNil(t, out.UsageIntegrity)
	require.NotNil(t, out.UsageIntegrity.UsageLogNotPersisted)
	require.Equal(t, int64(1), out.UsageIntegrity.UsageLogNotPersisted.Total)
	require.NotNil(t, out.UsageIntegrity.BillingCompensation)
	require.Nil(t, out.DataSource)
	require.Equal(t, "bill-gap-1", out.UsageIntegrity.BillingCompensation.LastRequestID)
}

func TestOpsServiceGetRealtimeSummaryBundle_UsesAuthFailuresWithoutOpsRepo(t *testing.T) {
	now := time.Now().UTC()
	resetLatestErrorFamilySummaryForTest()
	recordLatestErrorFamilySummary(now, opsRuntimeErrorFamilySummary{
		Kind:          "summary",
		WindowMinutes: 15,
		TotalErrors:   1,
		Families: []opsRuntimeErrorFamilyItem{
			{Phase: "request", Type: "bad_request", Owner: "client", StatusCode: 400, InboundEndpoint: "/v1/messages", Count: 1, SharePercent: 100},
		},
	})

	svc := &OpsService{
		cfg: &testConfigOpsEnabled,
		accountRepo: &opsAccountAvailabilityRepoStub{
			accounts: []Account{
				{
					ID:          301,
					Name:        "oauth-auth-gap",
					Platform:    PlatformOpenAI,
					Status:      StatusError,
					Schedulable: true,
					Extra: map[string]any{
						accountAuthFailureReasonKey: "token_revoked",
						accountAuthFailureClassKey:  accountAuthClassPermanent,
						accountAuthFailureSourceKey: accountAuthSourceTokenRefresh,
						accountAuthStateKey:         accountAuthStateReauthorizeRequired,
						accountAuthStateAtKey:       now.Format(time.RFC3339),
						accountAuthRecoveryKey:      accountAuthRecoveryActionReauthorize,
					},
				},
			},
		},
	}

	out := svc.GetRealtimeSummaryBundle(context.Background())
	require.NotNil(t, out)
	require.NotNil(t, out.FailureSplitSummary)
	require.Equal(t, int64(1), out.FailureSplitSummary.ProtocolOrRequestShape)
	require.Equal(t, int64(1), out.FailureSplitSummary.AccountOrAuth)
	require.Equal(t, "protocol_or_request_shape", out.FailureSplitSummary.LikelyPrimary)
	require.Contains(t, out.FailureSplitSummary.Suggestion, "protocol/request shape")
	require.Contains(t, out.FailureSplitSummary.OperatorAction, "protocol")
}

func TestOpsServiceGetDashboardOverview_UsageGovernanceAndIntegrityNotes(t *testing.T) {
	now := time.Now().UTC()
	resetUsageLogNotPersistedAlertsForTest()
	resetBillingCompensationCandidatesForTest()
	resetLatestStorageGovernanceRuntimeForTest()
	resetLatestErrorFamilySummaryForTest()
	resetUsageCleanupStatsForTest()

	usageLogNotPersistedAlerts.Store(2)
	recordUsageLogNotPersistedAlert(UsageLogNotPersistedAlert{
		Timestamp: now,
		RequestID: "usage-gap-note",
		AccountID: 77,
		Error:     "sync fallback failed",
	})
	recordBillingCompensationCandidate("svc.note", &UsageLog{
		RequestID: "bill-gap-note",
		AccountID: 66,
		Model:     "gpt-note",
	}, errors.New("billing gap"))
	recordLatestStorageGovernanceRuntimeSnapshot(now, opsStorageGovernanceRuntimeSnapshot{
		Tables: []opsStorageGovernanceTableSnapshot{
			{
				Table:                   "usage_logs",
				LiveRows:                500000,
				DeadRows:                10000,
				TotalMB:                 700,
				Partitioned:             false,
				RetentionDays:           30,
				RetentionEnabled:        true,
				MaxRowsConfigured:       250000,
				MaxRowsEnabled:          true,
				MaxRowsExceeded:         true,
				SizeRisk:                "warn",
				RetentionRisk:           "warn",
				ColdIndexRisk:           "none",
				ApproxBytesPerLiveRow:   123.4,
				ColdIndexCandidateCount: 3,
				Reasons:                 []string{"usage_logs_max_rows_exceeded"},
			},
		},
	})
	recordLatestErrorFamilySummary(now, opsRuntimeErrorFamilySummary{
		Kind:          "summary",
		WindowMinutes: 15,
		TotalErrors:   5,
		Families: []opsRuntimeErrorFamilyItem{
			{Phase: "upstream", Type: "api_error", Owner: "provider", StatusCode: 503, InboundEndpoint: "/v1/chat/completions", Count: 5, SharePercent: 100},
		},
	})
	recordUsageCleanupTaskStarted(now.Add(-2 * time.Minute))
	recordUsageCleanupTaskFailed(now.Add(-1*time.Minute), "cleanup timeout")

	repo := &opsRepoMock{
		GetDashboardOverviewFn: func(ctx context.Context, filter *OpsDashboardFilter) (*OpsDashboardOverview, error) {
			return &OpsDashboardOverview{
				DataSource: &OpsDataSourceSummary{Mode: string(OpsQueryModeRaw)},
			}, nil
		},
	}
	repo.GetLatestSystemMetricsFn = func(ctx context.Context, windowMinutes int) (*OpsSystemMetricsSnapshot, error) {
		dbOK := true
		return &OpsSystemMetricsSnapshot{DBOK: &dbOK}, nil
	}
	svc := &OpsService{opsRepo: repo, cfg: &testConfigOpsEnabled}

	out, err := svc.GetDashboardOverview(context.Background(), &OpsDashboardFilter{
		StartTime: now.Add(-time.Hour),
		EndTime:   now,
		QueryMode: OpsQueryModeRaw,
	})
	require.NoError(t, err)
	require.NotNil(t, out.StorageGovernance)
	require.NotNil(t, out.StorageGovernance.UsageLogs)
	require.Equal(t, int64(500000), out.StorageGovernance.UsageLogs.EstimatedLiveRows)
	require.NotNil(t, out.StorageGovernance.Runtime)
	require.NotNil(t, out.StorageGovernance.DryRun)
	require.NotNil(t, out.StorageGovernance.DryRun.UsageCleanup)
	require.Equal(t, int64(500000), out.StorageGovernance.DryRun.UsageCleanup.UsageLogRowsEstimated)
	require.True(t, out.StorageGovernance.DryRun.UsageCleanup.UsageLogOverLimit)
	require.Equal(t, "warn", out.StorageGovernance.DryRun.UsageCleanup.RetentionRisk)
	require.NotNil(t, out.ErrorFamilySummary)
	require.Equal(t, int64(5), out.ErrorFamilySummary.TotalErrors)
	require.NotNil(t, out.UsageIntegrity)
	require.Equal(t, int64(2), out.UsageIntegrity.UsageLogNotPersisted.Total)
	require.Equal(t, "bill-gap-note", out.UsageIntegrity.BillingCompensation.LastRequestID)
	require.NotNil(t, out.SlowPathDiagnostics)
	require.True(t, out.SlowPathDiagnostics.RawPathUsed)
	require.Equal(t, "raw_requested", out.SlowPathDiagnostics.QueryPathReason)
	require.True(t, out.SlowPathDiagnostics.PreferSummaryFirst)
	require.Equal(t, "warn", out.SlowPathDiagnostics.RawRiskLevel)
	require.NotEmpty(t, out.SlowPathDiagnostics.GuardrailHint)
	require.Len(t, out.SlowPathDiagnostics.DrilldownEndpoints, 3)
	require.Contains(t, out.SlowPathDiagnostics.SlowSignals, "raw_query_path")
	require.NotEmpty(t, out.SlowPathDiagnostics.InvestigationOrder)
	require.True(t, containsObservabilityTitle(out.Observability, "Usage integrity gaps detected"))
	require.True(t, containsObservabilityTitle(out.Observability, "Avoid raw usage queries during peak load"))
	require.True(t, containsObservabilityTitle(out.Observability, "High-frequency error family observed"))
	require.True(t, containsObservabilitySuggestionSubstring(out.Observability, "upstream/provider issue"))
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

func containsObservabilitySuggestionSubstring(notes []*OpsObservabilityNotice, needle string) bool {
	if len(notes) == 0 || needle == "" {
		return false
	}
	for _, note := range notes {
		if note != nil && strings.Contains(note.Suggestion, needle) {
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
