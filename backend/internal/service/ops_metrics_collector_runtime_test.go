package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOpsMetricsCollectorPersistRuntimeAnomalySummary_BaselineThenDelta(t *testing.T) {
	resetUsageLogNotPersistedAlertsForTest()
	resetBillingCompensationCandidatesForTest()
	resetSchedulerOutboxRuntimeMetricsForTest()

	var captured []*OpsInsertSystemLogInput
	repo := &opsRepoMock{
		BatchInsertSystemLogsFn: func(ctx context.Context, inputs []*OpsInsertSystemLogInput) (int64, error) {
			captured = append(captured, inputs...)
			return int64(len(inputs)), nil
		},
	}
	collector := &OpsMetricsCollector{opsRepo: repo}
	now := time.Now().UTC()

	require.NoError(t, collector.persistRuntimeAnomalySummary(context.Background(), now))
	require.Empty(t, captured)

	logUsageLogNotPersisted("service.test", &UsageLog{
		RequestID: "req-usage-1",
		UserID:    11,
		AccountID: 22,
	}, assertErr("usage failed"))
	recordBillingCompensationCandidate("service.test", &UsageLog{
		RequestID:   "req-billing-1",
		UserID:      111,
		AccountID:   222,
		BillingType: 2,
	}, assertErr("billing usage gap"))
	recordSchedulerOutboxPoisonEvent(SchedulerOutboxEvent{
		ID:        33,
		EventType: SchedulerOutboxEventAccountChanged,
	}, markOutboxPoison(assertErr("malformed payload")))

	require.NoError(t, collector.persistRuntimeAnomalySummary(context.Background(), now.Add(time.Minute)))
	require.Len(t, captured, 3)
	require.Equal(t, OpsRuntimeUsageLogSummaryComponent, captured[0].Component)
	require.Equal(t, OpsRuntimeBillingCompensationSummaryComponent, captured[1].Component)
	require.Equal(t, OpsRuntimeSchedulerOutboxSummaryComponent, captured[2].Component)

	var usageExtra map[string]any
	require.NoError(t, json.Unmarshal([]byte(captured[0].ExtraJSON), &usageExtra))
	require.Equal(t, float64(1), usageExtra["delta"])
	require.Equal(t, float64(1), usageExtra["recent_1m_total"])

	var billingExtra map[string]any
	require.NoError(t, json.Unmarshal([]byte(captured[1].ExtraJSON), &billingExtra))
	require.Equal(t, float64(1), billingExtra["delta"])
	require.Equal(t, float64(1), billingExtra["recent_1m_total"])

	var schedulerExtra map[string]any
	require.NoError(t, json.Unmarshal([]byte(captured[2].ExtraJSON), &schedulerExtra))
	require.Equal(t, float64(1), schedulerExtra["poison_delta"])
	require.Equal(t, float64(0), schedulerExtra["transient_delta"])
}

func TestOpsMetricsCollectorPersistRuntimeAnomalySummary_NoDuplicateWhenNoNewDelta(t *testing.T) {
	resetUsageLogNotPersistedAlertsForTest()
	resetBillingCompensationCandidatesForTest()
	resetSchedulerOutboxRuntimeMetricsForTest()

	callCount := 0
	repo := &opsRepoMock{
		BatchInsertSystemLogsFn: func(ctx context.Context, inputs []*OpsInsertSystemLogInput) (int64, error) {
			callCount++
			return int64(len(inputs)), nil
		},
	}
	collector := &OpsMetricsCollector{opsRepo: repo}
	now := time.Now().UTC()

	require.NoError(t, collector.persistRuntimeAnomalySummary(context.Background(), now))
	logUsageLogNotPersisted("service.test", &UsageLog{
		RequestID: "req-usage-2",
		UserID:    44,
		AccountID: 55,
	}, assertErr("usage failed"))
	require.NoError(t, collector.persistRuntimeAnomalySummary(context.Background(), now.Add(time.Minute)))
	require.Equal(t, 1, callCount)

	require.NoError(t, collector.persistRuntimeAnomalySummary(context.Background(), now.Add(2*time.Minute)))
	require.Equal(t, 1, callCount)
}

func TestOpsMetricsCollectorDBConnWaitingCapturedByDelta(t *testing.T) {
	var captured *OpsInsertSystemMetricsInput
	repo := &captureOpsRepo{
		insertMetricsFn: func(ctx context.Context, input *OpsInsertSystemMetricsInput) error {
			captured = input
			return nil
		},
	}
	collector := &OpsMetricsCollector{
		opsRepo: repo,
		dbPoolStatsFn: func() (int, int, uint64, time.Duration) {
			return 1, 1, 9, 20 * time.Millisecond
		},
	}
	collector.lastDBWaitCount = 3

	require.NoError(t, collector.collectAndPersist(context.Background()))
	require.NotNil(t, captured)
	require.NotNil(t, captured.DBConnWaiting)
	require.Equal(t, 6, *captured.DBConnWaiting)
}

func TestOpsMetricsCollectorPersistRuntimeAnomalySummary_UsageWorkerAndRedis(t *testing.T) {
	resetUsageLogNotPersistedAlertsForTest()
	resetBillingCompensationCandidatesForTest()
	resetSchedulerOutboxRuntimeMetricsForTest()

	var captured []*OpsInsertSystemLogInput
	repo := &opsRepoMock{
		BatchInsertSystemLogsFn: func(ctx context.Context, inputs []*OpsInsertSystemLogInput) (int64, error) {
			captured = append(captured, inputs...)
			return int64(len(inputs)), nil
		},
	}
	pool := &UsageRecordWorkerPool{}
	defaultUsageRecordWorkerPool.Store(pool)
	defer defaultUsageRecordWorkerPool.Store(nil)

	redisStats := opsRedisPoolRuntimeStats{}
	now := time.Now().UTC()
	collector := &OpsMetricsCollector{
		opsRepo: repo,
		redisPoolStatsFn: func() opsRedisPoolRuntimeStats {
			return redisStats
		},
	}

	require.NoError(t, collector.persistRuntimeAnomalySummary(context.Background(), now))

	pool.droppedQueueFull.Add(1)
	pool.syncFallback.Add(1)
	pool.taskTimeouts.Add(1)
	pool.taskPanics.Add(1)
	redisStats.Stalls = 2
	redisStats.Timeouts = 3

	require.NoError(t, collector.persistRuntimeAnomalySummary(context.Background(), now.Add(time.Minute)))
	require.Len(t, captured, 2)
	require.Equal(t, OpsRuntimeUsageWorkerSummaryComponent, captured[0].Component)
	require.Equal(t, OpsRuntimeRedisPoolSummaryComponent, captured[1].Component)

	var workerExtra map[string]any
	require.NoError(t, json.Unmarshal([]byte(captured[0].ExtraJSON), &workerExtra))
	require.Equal(t, float64(1), workerExtra["dropped_delta"])
	require.Equal(t, float64(1), workerExtra["sync_fallback_delta"])
	require.Equal(t, float64(1), workerExtra["timeout_delta"])
	require.Equal(t, float64(1), workerExtra["panic_delta"])

	var redisExtra map[string]any
	require.NoError(t, json.Unmarshal([]byte(captured[1].ExtraJSON), &redisExtra))
	require.Equal(t, float64(2), redisExtra["stall_delta"])
	require.Equal(t, float64(3), redisExtra["timeout_delta"])
	require.Equal(t, float64(2), redisExtra["stalls"])
	require.Equal(t, float64(3), redisExtra["timeouts"])
}

func TestOpsMetricsCollectorPersistRuntimeAnomalySummary_StorageGovernanceRisk(t *testing.T) {
	resetUsageLogNotPersistedAlertsForTest()
	resetBillingCompensationCandidatesForTest()
	resetSchedulerOutboxRuntimeMetricsForTest()

	var captured []*OpsInsertSystemLogInput
	repo := &opsRepoMock{
		BatchInsertSystemLogsFn: func(ctx context.Context, inputs []*OpsInsertSystemLogInput) (int64, error) {
			captured = append(captured, inputs...)
			return int64(len(inputs)), nil
		},
	}

	call := 0
	collector := &OpsMetricsCollector{
		opsRepo: repo,
		storageGovernanceStatsFn: func(ctx context.Context) (opsStorageGovernanceRuntimeSnapshot, error) {
			call++
			if call == 1 {
				return opsStorageGovernanceRuntimeSnapshot{
					Kind:                  "summary",
					SampledTables:         3,
					SampleIntervalMinutes: int(opsStorageGovernanceSampleInterval / time.Minute),
					Tables: []opsStorageGovernanceTableSnapshot{
						{Table: "usage_logs", SizeRisk: "none", ColdIndexRisk: "none", RetentionRisk: "none"},
						{Table: "ops_system_logs", SizeRisk: "none", ColdIndexRisk: "none", RetentionRisk: "none"},
						{Table: "ops_error_logs", SizeRisk: "none", ColdIndexRisk: "none", RetentionRisk: "none"},
					},
				}, nil
			}
			return opsStorageGovernanceRuntimeSnapshot{
				Kind:                  "summary",
				SampledTables:         3,
				RiskTables:            2,
				WarnTables:            1,
				CriticalTables:        1,
				HasRisk:               true,
				SampleIntervalMinutes: int(opsStorageGovernanceSampleInterval / time.Minute),
				Tables: []opsStorageGovernanceTableSnapshot{
					{
						Table:                   "usage_logs",
						TotalBytes:              600 * bytesPerMB,
						TotalMB:                 600,
						LiveRows:                100000,
						ColdIndexCandidateCount: 4,
						ColdIndexCandidateBytes: 24 * bytesPerMB,
						RetentionDays:           90,
						RetentionEnabled:        true,
						RetentionManagedBy:      "dashboard_aggregation.retention.usage_logs_days",
						RetentionStrategy:       "dashboard_aggregation_cleanup",
						SizeRisk:                "critical",
						ColdIndexRisk:           "warn",
						RetentionRisk:           "warn",
						Reasons:                 []string{"table_size_critical", "non_partitioned_delete_retention"},
					},
					{
						Table:              "ops_system_logs",
						TotalBytes:         700 * bytesPerMB,
						TotalMB:            700,
						RetentionDays:      30,
						RetentionEnabled:   true,
						RetentionManagedBy: "ops.cleanup.error_log_retention_days",
						RetentionStrategy:  "ops_cleanup_batch_delete",
						SizeRisk:           "warn",
						ColdIndexRisk:      "none",
						RetentionRisk:      "warn",
						Reasons:            []string{"table_size_warn", "batch_delete_retention_on_large_table"},
					},
				},
			}, nil
		},
	}

	now := time.Now().UTC()
	require.NoError(t, collector.persistRuntimeAnomalySummary(context.Background(), now))
	require.Empty(t, captured)

	require.NoError(t, collector.persistRuntimeAnomalySummary(context.Background(), now.Add(opsStorageGovernanceSampleInterval)))
	require.Len(t, captured, 1)
	require.Equal(t, opsRuntimeStorageGovernanceSummaryComponent, captured[0].Component)
	require.Equal(t, "error", captured[0].Level)

	var extra map[string]any
	require.NoError(t, json.Unmarshal([]byte(captured[0].ExtraJSON), &extra))
	require.Equal(t, float64(2), extra["risk_tables"])
	require.Equal(t, float64(1), extra["critical_tables"])
	require.Equal(t, float64(1), extra["warn_tables"])
}

func TestOpsMetricsCollectorPersistRuntimeAnomalySummary_StorageGovernanceNoDuplicateAndRecovery(t *testing.T) {
	resetUsageLogNotPersistedAlertsForTest()
	resetBillingCompensationCandidatesForTest()
	resetSchedulerOutboxRuntimeMetricsForTest()

	var captured []*OpsInsertSystemLogInput
	repo := &opsRepoMock{
		BatchInsertSystemLogsFn: func(ctx context.Context, inputs []*OpsInsertSystemLogInput) (int64, error) {
			captured = append(captured, inputs...)
			return int64(len(inputs)), nil
		},
	}

	call := 0
	collector := &OpsMetricsCollector{
		opsRepo: repo,
		storageGovernanceStatsFn: func(ctx context.Context) (opsStorageGovernanceRuntimeSnapshot, error) {
			call++
			switch call {
			case 1:
				return opsStorageGovernanceRuntimeSnapshot{
					Kind:                  "summary",
					SampledTables:         3,
					RiskTables:            1,
					WarnTables:            1,
					HasRisk:               true,
					SampleIntervalMinutes: int(opsStorageGovernanceSampleInterval / time.Minute),
					Tables: []opsStorageGovernanceTableSnapshot{
						{Table: "usage_logs", SizeRisk: "warn", ColdIndexRisk: "warn", RetentionRisk: "warn"},
					},
				}, nil
			case 2:
				return opsStorageGovernanceRuntimeSnapshot{
					Kind:                  "summary",
					SampledTables:         3,
					RiskTables:            1,
					WarnTables:            1,
					HasRisk:               true,
					SampleIntervalMinutes: int(opsStorageGovernanceSampleInterval / time.Minute),
					Tables: []opsStorageGovernanceTableSnapshot{
						{Table: "usage_logs", SizeRisk: "warn", ColdIndexRisk: "warn", RetentionRisk: "warn"},
					},
				}, nil
			default:
				return opsStorageGovernanceRuntimeSnapshot{
					Kind:                  "summary",
					SampledTables:         3,
					SampleIntervalMinutes: int(opsStorageGovernanceSampleInterval / time.Minute),
					Tables: []opsStorageGovernanceTableSnapshot{
						{Table: "usage_logs", SizeRisk: "none", ColdIndexRisk: "none", RetentionRisk: "none"},
					},
				}, nil
			}
		},
	}

	now := time.Now().UTC()
	require.NoError(t, collector.persistRuntimeAnomalySummary(context.Background(), now))
	require.NoError(t, collector.persistRuntimeAnomalySummary(context.Background(), now.Add(opsStorageGovernanceSampleInterval)))
	require.Len(t, captured, 0)

	require.NoError(t, collector.persistRuntimeAnomalySummary(context.Background(), now.Add(2*opsStorageGovernanceSampleInterval)))
	require.Len(t, captured, 1)
	require.Equal(t, "info", captured[0].Level)
	require.Equal(t, opsRuntimeStorageGovernanceSummaryComponent, captured[0].Component)
}

type captureOpsRepo struct {
	opsRepoMock
	insertMetricsFn func(ctx context.Context, input *OpsInsertSystemMetricsInput) error
}

func (r *captureOpsRepo) InsertSystemMetrics(ctx context.Context, input *OpsInsertSystemMetricsInput) error {
	if r.insertMetricsFn != nil {
		return r.insertMetricsFn(ctx, input)
	}
	return nil
}

func assertErr(msg string) error {
	return &runtimeTestErr{msg: msg}
}

type runtimeTestErr struct {
	msg string
}

func (e *runtimeTestErr) Error() string {
	return e.msg
}
