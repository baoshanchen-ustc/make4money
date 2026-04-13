package admin

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type opsSnapshotUsageRepoStub struct {
	service.OpsRepository
	alertLogs   []*service.OpsSystemLog
	summaryLogs []*service.OpsSystemLog
}

func (s *opsSnapshotUsageRepoStub) ListSystemLogs(ctx context.Context, filter *service.OpsSystemLogFilter) (*service.OpsSystemLogList, error) {
	switch filter.Component {
	case service.OpsRuntimeUsageLogComponent:
		return &service.OpsSystemLogList{Logs: s.alertLogs, Total: len(s.alertLogs), Page: 1, PageSize: filter.PageSize}, nil
	case service.OpsRuntimeUsageLogSummaryComponent:
		return &service.OpsSystemLogList{Logs: s.summaryLogs, Total: len(s.summaryLogs), Page: 1, PageSize: filter.PageSize}, nil
	default:
		return &service.OpsSystemLogList{Logs: []*service.OpsSystemLog{}, Total: 0, Page: 1, PageSize: filter.PageSize}, nil
	}
}

func TestBillingCompensationPayload_ExposesOpsSearchTemplate(t *testing.T) {
	payload := billingCompensationPayload()

	opsSearch, ok := payload["ops_search"].(gin.H)
	require.True(t, ok)
	require.Equal(t, "/api/v1/admin/ops/billing-compensation", opsSearch["endpoint"])
	require.Equal(t, "/api/v1/admin/ops/billing-compensation/:request_id", opsSearch["detail_endpoint_template"])
	_, hasLastDetail := opsSearch["last_detail_endpoint"]
	require.False(t, hasLastDetail)
}

func TestUsageLogNotPersistedPayload_ExposesOpsSearchTemplate(t *testing.T) {
	payload := usageLogNotPersistedPayload()

	opsSearch, ok := payload["ops_search"].(gin.H)
	require.True(t, ok)
	require.Equal(t, "/api/v1/admin/ops/usage-log-not-persisted", opsSearch["endpoint"])
	require.Equal(t, "/api/v1/admin/ops/usage-log-not-persisted/:request_id", opsSearch["detail_endpoint_template"])
	_, hasLastDetail := opsSearch["last_detail_endpoint"]
	require.False(t, hasLastDetail)
}

func TestBuildSnapshotFailureSplitSummaryPayload_UsesOverviewSummary(t *testing.T) {
	summary := buildSnapshotFailureSplitSummaryPayload(&service.OpsDashboardOverview{
		FailureSplitSummary: &service.OpsFailureSplitSummary{
			ProtocolOrRequestShape: 3,
			AccountOrAuth:          1,
			LikelyPrimary:          "protocol_or_request_shape",
			Suggestion:             "Validate protocol shape first.",
			OperatorAction:         "Pause failover changes until request shape is validated.",
		},
	})

	require.NotNil(t, summary)
	require.Equal(t, int64(3), summary.ProtocolOrRequestShape)
	require.Equal(t, "protocol_or_request_shape", summary.LikelyPrimary)
}

func TestBuildFailureSplitGuidancePayload_IncludesTotals(t *testing.T) {
	payload := buildFailureSplitGuidancePayload(&service.OpsDashboardOverview{
		FailureSplitSummary: &service.OpsFailureSplitSummary{
			ProtocolOrRequestShape: 2,
			AccountOrAuth:          1,
			ProviderOrUpstream:     1,
			LocalProcessing:        0,
			LikelyPrimary:          "protocol_or_request_shape",
			Suggestion:             "Check protocol shape first.",
			OperatorAction:         "Pause failover changes until protocol shape is confirmed.",
		},
	})

	require.NotNil(t, payload)
	require.Contains(t, payload, "totals")
	totals, ok := payload["totals"].(gin.H)
	require.True(t, ok)
	require.Equal(t, int64(2), totals["protocol_or_request_shape"])
	require.Equal(t, int64(1), totals["account_or_auth"])
	require.Equal(t, "protocol_or_request_shape", payload["likely_primary"])
	require.Equal(t, "Check protocol shape first.", payload["suggestion"])
	require.Equal(t, "Pause failover changes until protocol shape is confirmed.", payload["operator_action"])
	require.Contains(t, payload, "focus_hint")
	require.Contains(t, payload, "investigation_order")
	require.Contains(t, payload, "correlation_hint")
}

func TestBuildOperatorActionPlan_IncludesFailureAndDriftActions(t *testing.T) {
	plan := buildOperatorActionPlan(&service.OpsFailureSplitSummary{
		LikelyPrimary:  "protocol_or_request_shape",
		Suggestion:     "Check protocol shape first.",
		OperatorAction: "Pause failover changes until protocol shape is confirmed.",
	}, gin.H{"severity": "warning"})

	require.NotNil(t, plan)
	require.NotEmpty(t, plan.FailureSplitActions)
	require.NotEmpty(t, plan.ProtocolDiagnostics)
	require.NotEmpty(t, plan.ControlPlaneDrift)
}

func TestBuildSnapshotOperatorAndDriftSummaries(t *testing.T) {
	planSummary := buildSnapshotOperatorActionPlanSummary(&service.OpsOperatorActionPlan{
		FailureSplitActions: []string{"check request shape"},
		ProtocolDiagnostics: []string{"open requests"},
		ControlPlaneDrift:   []string{"check sticky first"},
	})
	require.NotNil(t, planSummary)
	require.Equal(t, 1, planSummary["failure_split_action_count"])
	require.Equal(t, "check request shape", planSummary["primary_action"])

	driftSummary := buildSnapshotDriftTrendSummary(
		gin.H{
			"severity":          "warning",
			"likely_root_cause": "sticky_and_scheduler_drift",
			"summary":           "control-plane drift active",
		},
		gin.H{
			"status": "degrading",
			"detail": "backlog=3 lag=7s",
		},
	)
	require.NotNil(t, driftSummary)
	require.Equal(t, "degrading", driftSummary["trend_status"])
	require.Equal(t, "warning", driftSummary["severity"])
	require.Equal(t, "sticky_and_scheduler_drift", driftSummary["likely_root_cause"])
}

func TestEnrichBillingCompensationFallback_UsesPersistedSummaryHint(t *testing.T) {
	now := time.Now().UTC()
	repo := &opsBillingCompensationRepoStub{
		candidateLogs: []*service.OpsSystemLog{
			{
				ID:        11,
				Component: service.OpsRuntimeBillingCompensationComponent,
				Message:   "candidate",
				CreatedAt: now,
				RequestID: "candidate-req",
			},
		},
		summaryLogs: []*service.OpsSystemLog{
			{
				ID:        12,
				Component: service.OpsRuntimeBillingCompensationSummaryComponent,
				Message:   "summary",
				CreatedAt: now,
				Extra: map[string]any{
					"last": map[string]any{
						"request_id": "summary-req",
						"account_id": int64(88),
						"error":      "persist failed",
					},
				},
			},
		},
	}
	opsSvc := service.NewOpsService(repo, nil, &config.Config{Ops: config.OpsConfig{Enabled: true}}, nil, nil, nil, nil, nil, nil, nil, nil)

	payload := billingCompensationPayload()
	enrichBillingCompensationFallback(context.Background(), opsSvc, &service.OpsDashboardFilter{}, payload)

	fallback, ok := payload["fallback"].(gin.H)
	require.True(t, ok)
	require.Len(t, fallback["persisted_logs"], 2)
	manualHint, ok := fallback["manual_hint"].(gin.H)
	require.True(t, ok)
	require.Equal(t, "summary-req", manualHint["request_id"])
	require.Equal(t, int64(88), manualHint["account_id"])
	require.Equal(t, "persist failed", manualHint["error"])
}

func TestEnrichUsageLogFallback_UsesPersistedSummaryHint(t *testing.T) {
	now := time.Now().UTC()
	repo := &opsSnapshotUsageRepoStub{
		alertLogs: []*service.OpsSystemLog{
			{
				ID:        21,
				Component: service.OpsRuntimeUsageLogComponent,
				Message:   "usage alert",
				CreatedAt: now,
				RequestID: "usage-alert-req",
			},
		},
		summaryLogs: []*service.OpsSystemLog{
			{
				ID:        22,
				Component: service.OpsRuntimeUsageLogSummaryComponent,
				Message:   "usage summary",
				CreatedAt: now,
				Extra: map[string]any{
					"last": map[string]any{
						"request_id": "usage-summary-req",
						"account_id": int64(66),
						"error":      "sync fallback failed",
					},
				},
			},
		},
	}
	opsSvc := service.NewOpsService(repo, nil, &config.Config{Ops: config.OpsConfig{Enabled: true}}, nil, nil, nil, nil, nil, nil, nil, nil)

	payload := usageLogNotPersistedPayload()
	enrichUsageLogFallback(context.Background(), opsSvc, &service.OpsDashboardFilter{}, payload)

	fallback, ok := payload["fallback"].(gin.H)
	require.True(t, ok)
	require.Len(t, fallback["persisted_logs"], 2)
	manualHint, ok := fallback["manual_hint"].(gin.H)
	require.True(t, ok)
	require.Equal(t, "usage-summary-req", manualHint["request_id"])
	require.Equal(t, int64(66), manualHint["account_id"])
	require.Equal(t, "sync fallback failed", manualHint["error"])
}
