package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	OpsRuntimeUsageLogComponent                   = "ops.runtime.usage_log"
	OpsRuntimeUsageLogSummaryComponent            = "ops.runtime.usage_log.summary"
	OpsRuntimeSchedulerOutboxSummaryComponent     = "ops.runtime.scheduler_outbox.summary"
	OpsRuntimeUsageWorkerSummaryComponent         = "ops.runtime.usage_worker.summary"
	OpsRuntimeRedisPoolSummaryComponent           = "ops.runtime.redis_pool.summary"
	OpsRuntimeStorageGovernanceSummaryComponent   = "ops.runtime.storage_governance.summary"
	OpsRuntimeErrorFamilySummaryComponent         = "ops.runtime.error_family.summary"
	OpsRuntimeBillingCompensationComponent        = "ops.runtime.billing_compensation"
	OpsRuntimeBillingCompensationSummaryComponent = "ops.runtime.billing_compensation.summary"
	opsDashboardRuntimeAnomalyLimit               = 6
)

const tokenRefreshFailureDetailsLimit = 20

const (
	tokenRefreshObservabilityTitle        = "Token refresh failures detected"
	schedulerCheckpointObservabilityTitle = "Scheduler checkpoint issues detected"
	stickyCleanupObservabilityTitle       = "Sticky session cleanup activity detected"
)

const (
	resourceBudgetDBMaxOpenGuardrail           = 500
	resourceBudgetDBIdleRatioGuardrail         = 0.90
	resourceBudgetRedisPoolGuardrail           = 512
	resourceBudgetRedisMinIdleRatioGuardrail   = 0.80
	resourceBudgetHTTPMaxIdleGuardrail         = 1024
	resourceBudgetHTTPIdlePerHostGuardrail     = 512
	resourceBudgetHTTPMaxConnsPerHostGuardrail = 1024
	resourceBudgetHTTPClientCacheGuardrail     = 5000
	resourceBudgetLowUsageThresholdPercent     = 25.0
	slowPathRequestThresholdMs                 = 1000
	slowPathP99WarningMs                       = 2000
)

func (s *OpsService) GetDashboardOverview(ctx context.Context, filter *OpsDashboardFilter) (*OpsDashboardOverview, error) {
	if err := s.RequireMonitoringEnabled(ctx); err != nil {
		return nil, err
	}
	if s.opsRepo == nil {
		return nil, infraerrors.ServiceUnavailable("OPS_REPO_UNAVAILABLE", "Ops repository not available")
	}
	if filter == nil {
		return nil, infraerrors.BadRequest("OPS_FILTER_REQUIRED", "filter is required")
	}
	if filter.StartTime.IsZero() || filter.EndTime.IsZero() {
		return nil, infraerrors.BadRequest("OPS_TIME_RANGE_REQUIRED", "start_time/end_time are required")
	}
	if filter.StartTime.After(filter.EndTime) {
		return nil, infraerrors.BadRequest("OPS_TIME_RANGE_INVALID", "start_time must be <= end_time")
	}

	// Resolve query mode (requested via query param, or DB default).
	requestedQueryMode := filter.QueryMode
	filter.QueryMode = s.resolveOpsQueryMode(ctx, filter.QueryMode)

	overview, err := s.opsRepo.GetDashboardOverview(ctx, filter)
	if err != nil && shouldFallbackOpsPreagg(filter, err) {
		rawFilter := cloneOpsFilterWithMode(filter, OpsQueryModeRaw)
		overview, err = s.opsRepo.GetDashboardOverview(ctx, rawFilter)
	}
	if err != nil {
		if errors.Is(err, ErrOpsPreaggregatedNotPopulated) {
			return nil, infraerrors.Conflict("OPS_PREAGG_NOT_READY", "Pre-aggregated ops metrics are not populated yet")
		}
		return nil, err
	}
	annotateOverviewQueryPath(requestedQueryMode, filter, overview)

	// Best-effort system health + jobs; dashboard metrics should still render if these are missing.
	if metrics, err := s.opsRepo.GetLatestSystemMetrics(ctx, 1); err == nil {
		s.attachSystemMetricsBudgetLimits(metrics)
		overview.SystemMetrics = metrics
	} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
		log.Printf("[Ops] GetLatestSystemMetrics failed: %v", err)
	}

	if heartbeats, err := s.opsRepo.ListJobHeartbeats(ctx); err == nil {
		overview.JobHeartbeats = heartbeats
	} else {
		log.Printf("[Ops] ListJobHeartbeats failed: %v", err)
	}

	anomalies, err := s.listRecentRuntimeAnomalies(ctx, filter, opsDashboardRuntimeAnomalyLimit)
	if len(anomalies) > 0 {
		overview.RuntimeAnomalies = anomalies
	}
	if err != nil {
		log.Printf("[Ops] ListRecentRuntimeAnomalies failed: %v", err)
	}

	overview.TokenRefreshSummary = s.TokenRefreshFailureSummary(ctx, filter.Platform, filter.GroupID)
	overview.AccountAuthSummary = s.AccountAuthFailureSummary(ctx, filter.Platform, filter.GroupID)
	overview.OpenAIAccountScheduler = s.OpenAIAccountSchedulerSummary()
	if stickyCleanup := SnapshotStickySessionCleanupMetrics(); stickyCleanup.CleanupTotal > 0 || stickyCleanup.CompareDeleteMissTotal > 0 || len(stickyCleanup.CleanupReasonTotals) > 0 || len(stickyCleanup.CompareDeleteMissReasonTotals) > 0 {
		overview.StickySessionCleanup = &stickyCleanup
	}
	if stickyConsistency := stickyConsistencySummary(); stickyConsistency != nil {
		overview.StickyConsistency = stickyConsistency
	}
	overview.SchedulerCheckpoint = SchedulerCheckpointSummary()
	if runtime := SchedulerOutboxRuntimeSummary(); runtime != nil {
		overview.SchedulerOutboxRuntime = runtime
	}
	overview.ResourceBudgetSummary = s.ResourceBudgetSummary(overview.SystemMetrics)
	cleanupStats := SnapshotCleanupStats()
	overview.CleanupStats = &cleanupStats
	usageCleanupStats := SnapshotUsageCleanupStats()
	overview.UsageCleanupStats = &usageCleanupStats
	overview.StorageGovernance = s.StorageGovernanceSummary(cleanupStats, usageCleanupStats, overview.JobHeartbeats)
	overview.ErrorFamilySummary = buildErrorFamilySummary()
	overview.FailureSplitSummary = buildFailureSplitSummary(overview.ErrorFamilySummary, overview.AccountAuthSummary)
	overview.UsageIntegrity = UsageIntegritySummary()
	overview.SlowPathDiagnostics = buildSlowPathDiagnostics(overview)
	overview.HealthScore = computeDashboardHealthScore(time.Now().UTC(), overview)
	if notes := s.collectObservabilityNotes(filter, overview); len(notes) > 0 {
		overview.Observability = append(overview.Observability, notes...)
	}
	budgetNotes := s.collectResourceBudgetNotices()
	budgetNotes = append(budgetNotes, resourceBudgetGuardrailNotes(overview.ResourceBudgetSummary)...)
	if len(budgetNotes) > 0 {
		overview.Observability = append(overview.Observability, budgetNotes...)
	}
	if summary := buildOpsDriftTrendSummary(overview); summary != nil {
		overview.DriftTrendSummary = summary
	}

	return overview, nil
}

func (s *OpsService) GetRealtimeSummaryBundle(ctx context.Context) *OpsRealtimeSummaryBundle {
	if s == nil {
		return nil
	}

	var metrics *OpsSystemMetricsSnapshot
	if s.opsRepo != nil {
		if snapshot, err := s.opsRepo.GetLatestSystemMetrics(ctx, 1); err == nil {
			s.attachSystemMetricsBudgetLimits(snapshot)
			metrics = snapshot
		} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
			log.Printf("[Ops] GetLatestSystemMetrics (realtime bundle) failed: %v", err)
		}
	}

	var heartbeats []*OpsJobHeartbeat
	if s.opsRepo != nil {
		if items, err := s.opsRepo.ListJobHeartbeats(ctx); err == nil {
			heartbeats = items
		} else {
			log.Printf("[Ops] ListJobHeartbeats (realtime bundle) failed: %v", err)
		}
	}

	cleanupStats := SnapshotCleanupStats()
	usageCleanupStats := SnapshotUsageCleanupStats()
	stickyCleanup := SnapshotStickySessionCleanupMetrics()

	bundle := &OpsRealtimeSummaryBundle{
		ResourceBudgetSummary: s.ResourceBudgetSummary(metrics),
		StorageGovernance:     s.StorageGovernanceSummary(cleanupStats, usageCleanupStats, heartbeats),
		ErrorFamilySummary:    buildErrorFamilySummary(),
		FailureSplitSummary:   buildFailureSplitSummary(buildErrorFamilySummary(), s.AccountAuthFailureSummary(ctx, "", nil)),
		StickySessionCleanup:  &stickyCleanup,
		StickyConsistency:     stickyConsistencySummary(),
		UsageIntegrity:        UsageIntegritySummary(),
	}
	if bundle.ResourceBudgetSummary == nil && bundle.StorageGovernance == nil {
		return nil
	}
	return bundle
}

func annotateOverviewQueryPath(requestedMode OpsQueryMode, filter *OpsDashboardFilter, overview *OpsDashboardOverview) {
	if overview == nil {
		return
	}
	if !requestedMode.IsValid() {
		requestedMode = OpsQueryModeAuto
		if filter != nil && filter.QueryMode.IsValid() {
			requestedMode = filter.QueryMode
		}
	}
	if overview.DataSource == nil {
		overview.DataSource = &OpsDataSourceSummary{}
	}
	if strings.TrimSpace(overview.DataSource.Mode) == "" {
		overview.DataSource.Mode = string(requestedMode)
	}
	if strings.TrimSpace(overview.DataSource.RequestedMode) == "" {
		overview.DataSource.RequestedMode = string(requestedMode)
	}
	actualMode := ParseOpsQueryMode(overview.DataSource.Mode)
	overview.QueryPath = &OpsDashboardQueryPath{
		Mode:          string(actualMode),
		RequestedMode: string(requestedMode),
		Reason:        deriveOpsQueryPathReason(requestedMode, actualMode),
	}
}

func deriveOpsQueryPathReason(requestedMode, actualMode OpsQueryMode) string {
	switch {
	case requestedMode == OpsQueryModeAuto && actualMode == OpsQueryModePreagg:
		return "auto_selected_preagg"
	case requestedMode == OpsQueryModeAuto && actualMode == OpsQueryModeRaw:
		return "auto_fallback_to_raw"
	case requestedMode == OpsQueryModePreagg && actualMode == OpsQueryModeRaw:
		return "preagg_unavailable_fallback"
	case requestedMode == OpsQueryModePreagg && actualMode == OpsQueryModePreagg:
		return "preagg_requested"
	case requestedMode == OpsQueryModeRaw && actualMode == OpsQueryModeRaw:
		return "raw_requested"
	default:
		return "requested_mode_used"
	}
}

func stickyConsistencySummary() *StickyConsistencyMetricsSnapshot {
	snapshot := SnapshotStickyConsistencyMetrics()
	if snapshot.TotalHits == 0 && snapshot.GhostInvalidations == 0 && len(snapshot.GhostReasons) == 0 {
		return nil
	}
	return &snapshot
}

func (s *OpsService) TokenRefreshFailureSummary(ctx context.Context, platformFilter string, groupIDFilter *int64) *OpsTokenRefreshSummary {
	if s == nil {
		return nil
	}
	if s.tokenRefreshSummaryFn != nil {
		return s.tokenRefreshSummaryFn(ctx, platformFilter, groupIDFilter)
	}
	platformStats, groupStats, accountStats, _, err := s.GetAccountAvailabilityStats(ctx, platformFilter, groupIDFilter)
	if err != nil {
		return nil
	}
	summary := &OpsTokenRefreshSummary{
		Platform: make(map[string]int64),
		Group:    make(map[int64]int64),
	}
	var total int64
	for name, platform := range platformStats {
		if platform == nil {
			continue
		}
		if platform.TokenRefreshFailureCount > 0 {
			summary.Platform[name] = platform.TokenRefreshFailureCount
			total += platform.TokenRefreshFailureCount
		}
	}
	for id, group := range groupStats {
		if group == nil {
			continue
		}
		if group.TokenRefreshFailureCount > 0 {
			summary.Group[id] = group.TokenRefreshFailureCount
		}
	}
	if len(accountStats) > 0 {
		failures := make([]*OpsTokenRefreshFailure, 0, len(accountStats))
		for _, acct := range accountStats {
			if acct == nil {
				continue
			}
			if acct.TokenRefreshFailureReason == "" && acct.TokenRefreshFailureClass == "" && acct.TokenRefreshFailedAt == "" {
				continue
			}
			failures = append(failures, &OpsTokenRefreshFailure{
				AccountID:   acct.AccountID,
				AccountName: acct.AccountName,
				Platform:    acct.Platform,
				GroupID:     acct.GroupID,
				GroupName:   acct.GroupName,
				Reason:      acct.TokenRefreshFailureReason,
				Class:       acct.TokenRefreshFailureClass,
				At:          acct.TokenRefreshFailedAt,
			})
		}
		if len(failures) > 0 {
			sort.Slice(failures, func(i, j int) bool {
				if failures[i].AccountID == failures[j].AccountID {
					return failures[i].At < failures[j].At
				}
				return failures[i].AccountID < failures[j].AccountID
			})
			if len(failures) > tokenRefreshFailureDetailsLimit {
				failures = failures[:tokenRefreshFailureDetailsLimit]
			}
			summary.Failures = failures
		}
	}
	if total == 0 && len(summary.Platform) == 0 && len(summary.Group) == 0 {
		return nil
	}
	summary.Total = total
	return summary
}

func (s *OpsService) AccountAuthFailureSummary(ctx context.Context, platformFilter string, groupIDFilter *int64) *OpsAccountAuthFailureSummary {
	if s == nil {
		return nil
	}
	platformStats, groupStats, accountStats, _, err := s.GetAccountAvailabilityStats(ctx, platformFilter, groupIDFilter)
	if err != nil {
		return nil
	}
	summary := &OpsAccountAuthFailureSummary{
		Platform: make(map[string]int64),
		Group:    make(map[int64]int64),
		ByReason: make(map[string]int64),
		ByClass:  make(map[string]int64),
		BySource: make(map[string]int64),
		ByState:  make(map[string]int64),
	}
	var total int64
	for name, platform := range platformStats {
		if platform == nil || platform.AuthFailureCount <= 0 {
			continue
		}
		summary.Platform[name] = platform.AuthFailureCount
		total += platform.AuthFailureCount
	}
	for id, group := range groupStats {
		if group == nil || group.AuthFailureCount <= 0 {
			continue
		}
		summary.Group[id] = group.AuthFailureCount
	}
	if len(accountStats) > 0 {
		failures := make([]*OpsAccountAuthFailure, 0, len(accountStats))
		for _, acct := range accountStats {
			if acct == nil {
				continue
			}
			if acct.AuthFailureReason == "" && acct.AuthFailureClass == "" && acct.AuthFailureSource == "" && acct.AuthState == "" {
				continue
			}
			if acct.AuthFailureReason != "" {
				summary.ByReason[acct.AuthFailureReason]++
			}
			if acct.AuthFailureClass != "" {
				summary.ByClass[acct.AuthFailureClass]++
				switch acct.AuthFailureClass {
				case accountAuthClassPermanent:
					summary.PermanentCount++
				case accountAuthClassTemporary:
					summary.TemporaryCount++
				}
			}
			if acct.AuthFailureSource != "" {
				summary.BySource[acct.AuthFailureSource]++
			}
			if acct.AuthState != "" {
				summary.ByState[acct.AuthState]++
			}
			if acct.AuthDispatchSuppressed {
				summary.DispatchSuppressed++
			}
			if acct.AuthBackgroundRecovery {
				summary.BackgroundRecovery++
			}
			if acct.AuthManualReviewRequired {
				summary.ManualReviewRequired++
			}
			failures = append(failures, &OpsAccountAuthFailure{
				AccountID:            acct.AccountID,
				AccountName:          acct.AccountName,
				Platform:             acct.Platform,
				GroupID:              acct.GroupID,
				GroupName:            acct.GroupName,
				Reason:               acct.AuthFailureReason,
				Class:                acct.AuthFailureClass,
				Source:               acct.AuthFailureSource,
				State:                acct.AuthState,
				RecoveryAction:       acct.AuthRecoveryAction,
				DispatchSuppressed:   acct.AuthDispatchSuppressed,
				BackgroundRecovery:   acct.AuthBackgroundRecovery,
				ManualReviewRequired: acct.AuthManualReviewRequired,
				At:                   acct.AuthStateChangedAt,
			})
		}
		if len(failures) > 0 {
			sort.Slice(failures, func(i, j int) bool {
				if failures[i].AccountID == failures[j].AccountID {
					return failures[i].At < failures[j].At
				}
				return failures[i].AccountID < failures[j].AccountID
			})
			if len(failures) > tokenRefreshFailureDetailsLimit {
				failures = failures[:tokenRefreshFailureDetailsLimit]
			}
			summary.Failures = failures
		}
	}
	if total == 0 && len(summary.ByReason) == 0 && len(summary.ByClass) == 0 && len(summary.BySource) == 0 && len(summary.ByState) == 0 {
		return nil
	}
	if len(summary.Platform) == 0 {
		summary.Platform = nil
	}
	if len(summary.Group) == 0 {
		summary.Group = nil
	}
	if len(summary.ByReason) == 0 {
		summary.ByReason = nil
	}
	if len(summary.ByClass) == 0 {
		summary.ByClass = nil
	}
	if len(summary.BySource) == 0 {
		summary.BySource = nil
	}
	if len(summary.ByState) == 0 {
		summary.ByState = nil
	}
	summary.Total = total
	return summary
}

func SchedulerCheckpointSummary() *OpsSchedulerCheckpointSummary {
	metrics := SnapshotSchedulerOutboxRuntimeMetrics()
	return &OpsSchedulerCheckpointSummary{
		RedisWatermark:               metrics.LastRedisWatermark,
		LastCheckpointWatermark:      metrics.LastCheckpointWatermark,
		WatermarkDrift:               metrics.WatermarkDrift,
		CheckpointFallbackTotal:      metrics.CheckpointFallbackTotal,
		CheckpointFallbackStreak:     metrics.CheckpointFallbackStreak,
		CheckpointReadFailures:       metrics.CheckpointReadFailureTotal,
		CheckpointWriteFailures:      metrics.CheckpointWriteFailureTotal,
		CheckpointLastFallbackAt:     metrics.CheckpointLastFallbackAt,
		CheckpointLastFallbackReason: metrics.CheckpointLastFallbackReason,
		CheckpointLastReadFailureAt:  metrics.CheckpointLastReadFailureAt,
		CheckpointLastWriteFailureAt: metrics.CheckpointLastWriteFailureAt,
		BlockedEvent:                 metrics.BlockedEvent,
	}
}

func buildErrorFamilySummary() *OpsErrorFamilySummary {
	summary, sampledAt := snapshotLatestErrorFamilySummary()
	if summary == nil {
		return nil
	}
	out := &OpsErrorFamilySummary{
		WindowMinutes: summary.WindowMinutes,
		TotalErrors:   summary.TotalErrors,
	}
	if sampledAt != nil {
		out.SampledAt = sampledAt
	}
	if len(summary.Families) > 0 {
		out.Families = make([]*OpsErrorFamilyEntry, 0, len(summary.Families))
		for _, family := range summary.Families {
			out.Families = append(out.Families, &OpsErrorFamilyEntry{
				Phase:           strings.TrimSpace(family.Phase),
				Type:            strings.TrimSpace(family.Type),
				Owner:           strings.TrimSpace(family.Owner),
				StatusCode:      family.StatusCode,
				InboundEndpoint: strings.TrimSpace(family.InboundEndpoint),
				Count:           family.Count,
				SharePercent:    family.SharePercent,
			})
		}
	}
	if out.TotalErrors == 0 && len(out.Families) == 0 {
		return nil
	}
	return out
}

func buildFailureSplitSummary(errorSummary *OpsErrorFamilySummary, authSummary *OpsAccountAuthFailureSummary) *OpsFailureSplitSummary {
	if errorSummary == nil && authSummary == nil {
		return nil
	}
	out := &OpsFailureSplitSummary{}
	if authSummary != nil {
		out.AccountOrAuth += authSummary.Total
	}
	if errorSummary != nil {
		for _, family := range errorSummary.Families {
			if family == nil || family.Count <= 0 {
				continue
			}
			switch categorizeErrorFamily(family) {
			case "protocol_or_request_shape":
				out.ProtocolOrRequestShape += family.Count
			case "account_or_auth":
				out.AccountOrAuth += family.Count
			case "provider_or_upstream":
				out.ProviderOrUpstream += family.Count
			case "local_processing":
				out.LocalProcessing += family.Count
			}
		}
	}
	out.LikelyPrimary = likelyPrimaryFailureFamily(out)
	out.Suggestion = failureSplitSuggestion(out.LikelyPrimary)
	out.OperatorAction = failureSplitActionHint(out.LikelyPrimary)
	out.OperatorAction = failureSplitOperatorAction(out.LikelyPrimary)
	if out.ProtocolOrRequestShape == 0 && out.AccountOrAuth == 0 && out.ProviderOrUpstream == 0 && out.LocalProcessing == 0 {
		return nil
	}
	return out
}

func categorizeErrorFamily(top *OpsErrorFamilyEntry) string {
	if top == nil {
		return "local_processing"
	}
	phase := strings.ToLower(strings.TrimSpace(top.Phase))
	owner := strings.ToLower(strings.TrimSpace(top.Owner))
	errorType := strings.ToLower(strings.TrimSpace(top.Type))
	switch {
	case strings.Contains(errorType, "auth"), strings.Contains(errorType, "token"), top.StatusCode == 401, top.StatusCode == 403:
		return "account_or_auth"
	case phase == "request" || owner == "client" || top.StatusCode == 400:
		return "protocol_or_request_shape"
	case phase == "upstream" || owner == "provider" || top.StatusCode >= 500:
		return "provider_or_upstream"
	case phase == "local" || owner == "gateway":
		return "local_processing"
	default:
		return "local_processing"
	}
}

func likelyPrimaryFailureFamily(summary *OpsFailureSplitSummary) string {
	if summary == nil {
		return ""
	}
	type candidate struct {
		name  string
		count int64
	}
	candidates := []candidate{
		{name: "protocol_or_request_shape", count: summary.ProtocolOrRequestShape},
		{name: "account_or_auth", count: summary.AccountOrAuth},
		{name: "provider_or_upstream", count: summary.ProviderOrUpstream},
		{name: "local_processing", count: summary.LocalProcessing},
	}
	best := candidate{}
	for _, c := range candidates {
		if c.count > best.count {
			best = c
		}
	}
	return best.name
}

func failureSplitSuggestion(primary string) string {
	label := failureSplitLabel(primary)
	switch strings.TrimSpace(primary) {
	case "protocol_or_request_shape":
		return fmt.Sprintf("%s primary: Validate protocol/request shape and requested->mapped->upstream model transitions before increasing failover or rotating accounts.", label)
	case "account_or_auth":
		return fmt.Sprintf("%s primary: Keep bad credentials out of dispatch, separate refresh-pending from manual reauthorization, and avoid masking auth churn with more failover.", label)
	case "provider_or_upstream":
		return fmt.Sprintf("%s primary: Inspect account health, saturation, and retry amplification before changing request validation.", label)
	case "local_processing":
		return fmt.Sprintf("%s primary: Inspect local gateway/control-plane pressure, including Redis, scheduler drift, and usage persistence side effects.", label)
	default:
		return ""
	}
}

func failureSplitActionHint(primary string) string {
	switch strings.TrimSpace(primary) {
	case "protocol_or_request_shape":
		return "Start with the request/protocol diagnostics and confirm the requested-to-upstream model mappings."
	case "account_or_auth":
		return "Quarantine auth failures, avoid rotating accounts, and prioritize reauthorization flow."
	case "provider_or_upstream":
		return "Probe upstream/provider saturation before touching request validation knobs."
	case "local_processing":
		return "Look at Redis, scheduler drift, and usage persistence before touching gateway logic."
	default:
		return "Follow the general failure investigation steps for the most recent incident."
	}
}

func failureSplitOperatorAction(primary string) string {
	return failureSplitActionHint(primary)
}

func failureSplitLabel(primary string) string {
	switch strings.TrimSpace(primary) {
	case "protocol_or_request_shape":
		return "protocol/request shape"
	case "account_or_auth":
		return "account/auth"
	case "provider_or_upstream":
		return "provider/upstream"
	case "local_processing":
		return "local processing"
	default:
		return "failure"
	}
}

func SchedulerOutboxRuntimeSummary() *OpsSchedulerOutboxRuntimeSummary {
	metrics := SnapshotSchedulerOutboxRuntimeMetrics()
	if metrics.LastRedisWatermark == 0 &&
		metrics.LastCheckpointWatermark == 0 &&
		metrics.WatermarkDrift == 0 &&
		metrics.CheckpointFallbackStreak == 0 &&
		metrics.BacklogRows == 0 &&
		metrics.LagSeconds == 0 &&
		metrics.LagFailureStreak == 0 &&
		metrics.CoalescedBatchTotal == 0 &&
		metrics.CoalescedEventSavedTotal == 0 &&
		metrics.LagRebuildTotal == 0 &&
		metrics.BacklogRebuildTotal == 0 &&
		metrics.RebuildCooldownSkipTotal == 0 &&
		metrics.BlockedEventClearTotal == 0 &&
		metrics.BucketRebuildSuccessTotal == 0 &&
		metrics.BucketRebuildFailureTotal == 0 &&
		metrics.BucketRebuildLockContention == 0 &&
		metrics.BusyBucketSkipTotal == 0 &&
		metrics.CheckpointLastFallbackAt == "" &&
		metrics.CheckpointLastReadFailureAt == "" &&
		metrics.CheckpointLastWriteFailureAt == "" &&
		metrics.BlockedEvent == nil {
		return nil
	}
	return &OpsSchedulerOutboxRuntimeSummary{
		LastRedisWatermark:           metrics.LastRedisWatermark,
		LastCheckpointWatermark:      metrics.LastCheckpointWatermark,
		WatermarkDrift:               metrics.WatermarkDrift,
		DriftTrendStatus:             metrics.DriftTrendStatus,
		DriftTrendDetail:             metrics.DriftTrendDetail,
		DriftTrendNarrative:          metrics.DriftTrendNarrative,
		CheckpointFallbackStreak:     metrics.CheckpointFallbackStreak,
		CheckpointLastFallbackReason: metrics.CheckpointLastFallbackReason,
		BacklogRows:                  metrics.BacklogRows,
		LagSeconds:                   metrics.LagSeconds,
		LagFailureStreak:             metrics.LagFailureStreak,
		CoalescedBatchTotal:          metrics.CoalescedBatchTotal,
		CoalescedEventSavedTotal:     metrics.CoalescedEventSavedTotal,
		LagRebuildTotal:              metrics.LagRebuildTotal,
		BacklogRebuildTotal:          metrics.BacklogRebuildTotal,
		RebuildCooldownSkipTotal:     metrics.RebuildCooldownSkipTotal,
		LastLagRebuildAt:             metrics.LastLagRebuildAt,
		LastBacklogRebuildAt:         metrics.LastBacklogRebuildAt,
		BlockedEventClearTotal:       metrics.BlockedEventClearTotal,
		BlockedEventLastClearedID:    metrics.BlockedEventLastClearedID,
		BlockedEventLastClearedAt:    metrics.BlockedEventLastClearedAt,
		BlockedEventLastClearReason:  metrics.BlockedEventLastClearReason,
		BucketRebuildSuccessTotal:    metrics.BucketRebuildSuccessTotal,
		BucketRebuildFailureTotal:    metrics.BucketRebuildFailureTotal,
		BucketRebuildLockContention:  metrics.BucketRebuildLockContention,
		BusyBucketSkipTotal:          metrics.BusyBucketSkipTotal,
		LastBucketRebuildAt:          metrics.LastBucketRebuildAt,
		LastBucketRebuildReason:      metrics.LastBucketRebuildReason,
		LastBucketRebuildStatus:      metrics.LastBucketRebuildStatus,
		LastBucketRebuildBucket:      metrics.LastBucketRebuildBucket,
		BlockedEvent:                 metrics.BlockedEvent,
	}
}

func (s *OpsService) OpenAIAccountSchedulerSummary() *OpsOpenAIAccountSchedulerSummary {
	if s == nil || s.openAIGatewayService == nil {
		return nil
	}
	snapshot := s.openAIGatewayService.SnapshotOpenAIAccountSchedulerMetrics()
	stickyCleanup := SnapshotStickySessionCleanupMetrics()
	if snapshot.SelectTotal == 0 &&
		snapshot.StickySessionLookupTotal == 0 &&
		snapshot.StickySessionWaitTotal == 0 &&
		snapshot.StickySessionClearedTotal == 0 &&
		snapshot.AccountSwitchTotal == 0 &&
		len(snapshot.StickySessionShadowReasonTotals) == 0 &&
		stickyCleanup.CleanupTotal == 0 &&
		stickyCleanup.CompareDeleteMissTotal == 0 {
		return nil
	}
	return &OpsOpenAIAccountSchedulerSummary{
		SelectTotal:                    snapshot.SelectTotal,
		StickyPreviousHitTotal:         snapshot.StickyPreviousHitTotal,
		StickySessionHitTotal:          snapshot.StickySessionHitTotal,
		StickySessionLookupTotal:       snapshot.StickySessionLookupTotal,
		StickySessionWaitTotal:         snapshot.StickySessionWaitTotal,
		StickySessionClearedTotal:      snapshot.StickySessionClearedTotal,
		StickySessionGhostTotal:        snapshot.StickySessionGhostTotal,
		StickySessionStaleTotal:        snapshot.StickySessionStaleTotal,
		StickyWaitConflictTotal:        snapshot.StickySessionWaitConflictTotal,
		LoadBalanceSelectTotal:         snapshot.LoadBalanceSelectTotal,
		AccountSwitchTotal:             snapshot.AccountSwitchTotal,
		StickyHitRatio:                 snapshot.StickyHitRatio,
		AccountSwitchRate:              snapshot.AccountSwitchRate,
		SchedulerLatencyMsAvg:          snapshot.SchedulerLatencyMsAvg,
		StickyGhostRatio:               snapshot.StickyGhostRatio,
		StickyStaleRatio:               snapshot.StickyStaleRatio,
		StickyWaitConflictRatio:        snapshot.StickyWaitConflictRatio,
		StickySessionAccountFetchTotal: snapshot.StickySessionAccountFetchTotal,
		StickySessionDBRecheckTotal:    snapshot.StickySessionDBRecheckTotal,
		LoadBatchCallTotal:             snapshot.LoadBatchCallTotal,
		LoadBatchCandidateTotal:        snapshot.LoadBatchCandidateTotal,
		LoadBatchFallbackTotal:         snapshot.LoadBatchFallbackTotal,
		CompareDeleteMissTotal:         stickyCleanup.CompareDeleteMissTotal,
		RuntimeStatsAccountCount:       snapshot.RuntimeStatsAccountCount,
		ShadowReasonTotals:             snapshot.StickySessionShadowReasonTotals,
		CleanupReasonTotals:            stickyCleanup.CleanupReasonTotals,
		CompareDeleteMissReasonTotals:  stickyCleanup.CompareDeleteMissReasonTotals,
	}
}

func (s *OpsService) ResourceBudgetSummary(metrics *OpsSystemMetricsSnapshot) *OpsResourceBudgetSummary {
	if s == nil || s.cfg == nil {
		return nil
	}
	cfg := s.cfg
	dbSummary := &OpsDatabaseBudgetSummary{}
	if cfg.Database.MaxOpenConns > 0 {
		dbSummary.MaxOpenConns = intPtr(cfg.Database.MaxOpenConns)
	}
	if cfg.Database.MaxIdleConns > 0 {
		dbSummary.MaxIdleConns = intPtr(cfg.Database.MaxIdleConns)
	}
	if cfg.Database.ConnMaxLifetimeMinutes > 0 {
		dbSummary.ConnMaxLifetimeMinutes = intPtr(cfg.Database.ConnMaxLifetimeMinutes)
	}
	if cfg.Database.ConnMaxIdleTimeMinutes > 0 {
		dbSummary.ConnMaxIdleTimeMinutes = intPtr(cfg.Database.ConnMaxIdleTimeMinutes)
	}
	redisSummary := &OpsRedisBudgetSummary{}
	if cfg.Redis.PoolSize > 0 {
		redisSummary.PoolSize = intPtr(cfg.Redis.PoolSize)
	}
	if cfg.Redis.MinIdleConns > 0 {
		redisSummary.MinIdleConns = intPtr(cfg.Redis.MinIdleConns)
	}
	if cfg.Redis.DialTimeoutSeconds > 0 {
		redisSummary.DialTimeoutSeconds = intPtr(cfg.Redis.DialTimeoutSeconds)
	}
	if cfg.Redis.ReadTimeoutSeconds > 0 {
		redisSummary.ReadTimeoutSeconds = intPtr(cfg.Redis.ReadTimeoutSeconds)
	}
	if cfg.Redis.WriteTimeoutSeconds > 0 {
		redisSummary.WriteTimeoutSeconds = intPtr(cfg.Redis.WriteTimeoutSeconds)
	}
	if metrics != nil {
		dbSummary.Active = metrics.DBConnActive
		dbSummary.Idle = metrics.DBConnIdle
		dbSummary.Waiting = metrics.DBConnWaiting
		dbSummary.UsagePercent = usagePercent(metrics.DBConnActive, cfg.Database.MaxOpenConns)
		dbSummary.HeadroomPercent = remainingPercent(dbSummary.UsagePercent)
		dbSummary.IdleRatioPercent = usagePercent(metrics.DBConnIdle, cfg.Database.MaxOpenConns)
		redisSummary.Total = metrics.RedisConnTotal
		redisSummary.Idle = metrics.RedisConnIdle
		redisSummary.UsagePercent = usagePercent(metrics.RedisConnTotal, cfg.Redis.PoolSize)
		redisSummary.HeadroomPercent = remainingPercent(redisSummary.UsagePercent)
		redisSummary.IdleRatioPercent = usagePercent(metrics.RedisConnIdle, cfg.Redis.PoolSize)
	}
	httpSummary := &OpsHTTPUpstreamBudgetSummary{}
	if cfg.Gateway.MaxIdleConns > 0 {
		httpSummary.MaxIdleConns = intPtr(cfg.Gateway.MaxIdleConns)
	}
	if cfg.Gateway.MaxIdleConnsPerHost > 0 {
		httpSummary.MaxIdleConnsPerHost = intPtr(cfg.Gateway.MaxIdleConnsPerHost)
	}
	if cfg.Gateway.MaxConnsPerHost > 0 {
		httpSummary.MaxConnsPerHost = intPtr(cfg.Gateway.MaxConnsPerHost)
	}
	if cfg.Gateway.MaxUpstreamClients > 0 {
		httpSummary.MaxUpstreamClients = intPtr(cfg.Gateway.MaxUpstreamClients)
	}
	if cfg.Gateway.ClientIdleTTLSeconds > 0 {
		httpSummary.ClientIdleTTLSeconds = intPtr(cfg.Gateway.ClientIdleTTLSeconds)
	}
	if cfg.Gateway.ConcurrencySlotTTLMinutes > 0 {
		httpSummary.ConcurrencySlotTTLMinute = intPtr(cfg.Gateway.ConcurrencySlotTTLMinutes)
	}
	if cfg.Gateway.SessionIdleTimeoutMinutes > 0 {
		httpSummary.SessionIdleTimeoutMinute = intPtr(cfg.Gateway.SessionIdleTimeoutMinutes)
	}
	if cfg.Gateway.ConnectionPoolIsolation != "" {
		mode := cfg.Gateway.ConnectionPoolIsolation
		httpSummary.IsolationMode = &mode
	}
	storageSummary := &OpsStorageGovernanceBudgetSummary{
		MaxRowsEnabled: cfg.Ops.Cleanup.MaxRowsEnabled,
		MaxRowsDryRun:  cfg.Ops.Cleanup.MaxRowsDryRun,
	}
	if cfg.Ops.Cleanup.SystemLogMaxRows > 0 {
		storageSummary.OpsSystemLogsMaxRows = int64PtrOpsDashboard(cfg.Ops.Cleanup.SystemLogMaxRows)
	}
	if cfg.Ops.Cleanup.ErrorLogMaxRows > 0 {
		storageSummary.OpsErrorLogsMaxRows = int64PtrOpsDashboard(cfg.Ops.Cleanup.ErrorLogMaxRows)
	}
	if cfg.DashboardAgg.Retention.UsageLogsMaxRows > 0 {
		storageSummary.UsageLogsMaxRows = int64PtrOpsDashboard(cfg.DashboardAgg.Retention.UsageLogsMaxRows)
	}
	recommendations := buildResourceBudgetRecommendations(cfg, dbSummary, redisSummary, httpSummary)
	watchThresholds := buildResourceBudgetWatchThresholds()
	return &OpsResourceBudgetSummary{
		Database:          dbSummary,
		Redis:             redisSummary,
		HTTPUpstream:      httpSummary,
		StorageGovernance: storageSummary,
		WatchThresholds:   watchThresholds,
		SafetyZone:        buildResourceBudgetSafetyZone(cfg, dbSummary, redisSummary, httpSummary),
		JointSignals:      buildResourceBudgetJointSignals(cfg, dbSummary, redisSummary, httpSummary),
		Recommendations:   recommendations,
		Guardrails:        buildResourceBudgetGuardrails(cfg),
	}
}

func (s *OpsService) attachSystemMetricsBudgetLimits(metrics *OpsSystemMetricsSnapshot) {
	if s == nil || s.cfg == nil || metrics == nil {
		return
	}
	if s.cfg.Database.MaxOpenConns > 0 {
		metrics.DBMaxOpenConns = intPtr(s.cfg.Database.MaxOpenConns)
	}
	if s.cfg.Redis.PoolSize > 0 {
		metrics.RedisPoolSize = intPtr(s.cfg.Redis.PoolSize)
	}
}

func buildResourceBudgetGuardrails(cfg *config.Config) []*OpsBudgetGuardrail {
	if cfg == nil {
		return nil
	}
	guardrails := []*OpsBudgetGuardrail{
		{
			Area:   "database",
			Phase:  "phase_1_gray_shrink",
			Action: "Reduce max_open_conns by 20-25% off-peak before considering deeper cuts.",
			Watch:  []string{"db_conn_waiting", "error_rate", "duration_p99_ms"},
			WatchThresholds: map[string]string{
				"db_conn_waiting": "Rollback if wait count shows sustained growth across consecutive minutes instead of isolated spikes.",
				"error_rate":      "Rollback if request error rate regresses materially versus the pre-change baseline.",
				"duration_p99_ms": "Rollback if tail latency regresses by roughly 15%+ or crosses the current slow-path warning zone.",
			},
			Rollback:  "Restore the previous DB pool if db_conn_waiting rises persistently or request latency regresses.",
			Rationale: "Single-host PostgreSQL is shared capacity; shrinking oversize pools first is safer than pushing more parallel work.",
		},
		{
			Area:   "redis",
			Phase:  "phase_1_gray_shrink",
			Action: "Lower min_idle_conns first, then reduce pool_size in 20-25% steps.",
			Watch:  []string{"redis_pool.stalls", "redis_pool.timeouts", "error_rate"},
			WatchThresholds: map[string]string{
				"redis_pool.stalls":   "Rollback if stalls begin growing every minute instead of staying near zero.",
				"redis_pool.timeouts": "Rollback on any sustained timeout growth after the change window.",
				"error_rate":          "Rollback if Redis-heavy request paths start pushing overall error rate above baseline.",
			},
			Rollback:  "Revert the last Redis pool change if stalls or timeouts increase materially.",
			Rationale: "Redis is the shared control plane for scheduler, rate limiting, and sticky sessions; reducing reservation is safer than hard pool cuts first.",
		},
		{
			Area:   "http_upstream",
			Phase:  "phase_1_canary",
			Action: "Shrink idle/client-cache limits with canary traffic before touching max_conns_per_host.",
			Watch:  []string{"duration_p99_ms", "ttft_p99_ms", "upstream_error_rate"},
			WatchThresholds: map[string]string{
				"duration_p99_ms":     "Rollback if overall tail latency regresses by roughly 15%+ on canary traffic.",
				"ttft_p99_ms":         "Rollback if TTFT shows a sustained step-up instead of momentary burst noise.",
				"upstream_error_rate": "Rollback if upstream failures climb above the pre-change baseline or trigger new failover churn.",
			},
			Rollback:  "Restore the previous HTTP pool settings if tail latency or upstream errors worsen.",
			Rationale: "Large upstream idle pools pin sockets and client caches, but can be reduced safely with a canary-first approach.",
		},
	}
	return guardrails
}

func resourceBudgetGuardrailNotes(summary *OpsResourceBudgetSummary) []*OpsObservabilityNotice {
	if summary == nil {
		return nil
	}
	notes := make([]*OpsObservabilityNotice, 0, 2)
	if summary.SafetyZone != nil && summary.SafetyZone.Status != "green" {
		notes = append(notes, &OpsObservabilityNotice{
			Level:      "warning",
			Title:      "Shared resource safety zone reduced",
			Detail:     summary.SafetyZone.Detail,
			Suggestion: "Use the budget guardrails and watch thresholds before tightening pools or adding new burst workloads.",
		})
	}
	if summary.Database != nil && summary.Database.UsagePercent != nil {
		if *summary.Database.UsagePercent >= 95 {
			notes = append(notes, &OpsObservabilityNotice{
				Level:      "warning",
				Title:      "DB connections near pool limit",
				Detail:     fmt.Sprintf("Active connections %.1f%% of max_open_conns=%d", *summary.Database.UsagePercent, ptrToInt(summary.Database.MaxOpenConns)),
				Suggestion: "Review connection churn or scale Postgres before additional bursty jobs hit the pool.",
			})
		}
	}
	if summary.Redis != nil && summary.Redis.UsagePercent != nil {
		if *summary.Redis.UsagePercent >= 95 {
			notes = append(notes, &OpsObservabilityNotice{
				Level:      "warning",
				Title:      "Redis pool saturation risk",
				Detail:     fmt.Sprintf("Current usage %.1f%% of pool_size=%d", *summary.Redis.UsagePercent, ptrToInt(summary.Redis.PoolSize)),
				Suggestion: "Consider throttling Redis-heavy workloads or gradually raising redis.pool_size while monitoring stall counts.",
			})
		}
	}
	for _, signal := range summary.JointSignals {
		if signal == nil {
			continue
		}
		notes = append(notes, &OpsObservabilityNotice{
			Level:      signal.Level,
			Title:      signal.Title,
			Detail:     signal.Detail,
			Suggestion: signal.Suggestion,
		})
	}
	return notes
}

func ptrToInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

func usagePercent(current *int, limit int) *float64 {
	if current == nil || limit <= 0 {
		return nil
	}
	pct := float64(*current) / float64(limit) * 100
	if pct < 0 {
		pct = 0
	}
	if pct > 200 {
		pct = 200
	}
	return float64Ptr(roundTo1DP(pct))
}

func remainingPercent(p *float64) *float64 {
	if p == nil {
		return nil
	}
	remaining := 100 - *p
	if remaining < 0 {
		remaining = 0
	}
	return float64Ptr(roundTo1DP(remaining))
}

func buildResourceBudgetWatchThresholds() map[string]string {
	return map[string]string{
		"db_conn_waiting":                 ">0 across consecutive minutes means pool contention; >=3/minute is a hard warning for gray-shrink rollback.",
		"db_pool_usage_percent":           "Aim to keep steady-state active DB usage below ~70%; sustained >85% leaves too little headroom for burst traffic.",
		"redis_pool_usage_percent":        "Aim to keep steady-state Redis connection usage below ~60%; sustained >80% should pause further shrink steps.",
		"redis_pool.stalls":               "Stalls should stay near zero; minute-over-minute growth indicates Redis pool contention before hard timeouts appear.",
		"redis_pool.timeouts":             "Any sustained non-zero timeout growth is critical for the scheduler/rate-limit control plane.",
		"http_client_cache_usage_percent": "Keep upstream client-cache usage below ~70%; sustained >85% suggests cache churn or too-wide isolation.",
		"http_pool_saturation":            "Treat concurrent high DB wait + Redis stalls/timeouts + hot HTTP cache as a host-level burst saturation signal.",
	}
}

func buildResourceBudgetSafetyZone(
	cfg *config.Config,
	dbSummary *OpsDatabaseBudgetSummary,
	redisSummary *OpsRedisBudgetSummary,
	httpSummary *OpsHTTPUpstreamBudgetSummary,
) *OpsResourceSafetyZone {
	reasons := make([]string, 0, 4)
	status := "green"

	if dbSummary != nil && dbSummary.Waiting != nil && *dbSummary.Waiting > 0 {
		status = "yellow"
		reasons = append(reasons, "db_conn_waiting_positive")
	}
	if dbSummary != nil && dbSummary.UsagePercent != nil && *dbSummary.UsagePercent >= 85 {
		status = "yellow"
		reasons = append(reasons, "db_usage_high")
	}
	if redisSummary != nil && redisSummary.UsagePercent != nil && *redisSummary.UsagePercent >= 80 {
		status = "yellow"
		reasons = append(reasons, "redis_usage_high")
	}
	if cfg != nil && cfg.Gateway.MaxUpstreamClients > resourceBudgetHTTPClientCacheGuardrail {
		reasons = append(reasons, "http_client_cache_guardrail_exceeded")
		if status == "green" {
			status = "yellow"
		}
	}
	if dbSummary != nil && dbSummary.Waiting != nil && *dbSummary.Waiting > 0 {
		if (dbSummary.UsagePercent != nil && *dbSummary.UsagePercent >= 85) ||
			(redisSummary != nil && redisSummary.UsagePercent != nil && *redisSummary.UsagePercent >= 80) {
			status = "red"
		}
	}

	detail := "Shared resources currently have room for additional burst concurrency, but keep watch thresholds visible before any gray shrink."
	switch status {
	case "yellow":
		detail = "At least one shared resource signal is elevated. Hold aggressive pool changes and verify DB waits, Redis stalls/timeouts, and HTTP cache pressure together."
	case "red":
		detail = "Shared resource contention signals are overlapping. Treat this node as near its safe burst boundary until wait/stall pressure returns to baseline."
	}

	if len(reasons) == 0 && httpSummary != nil && httpSummary.MaxUpstreamClients != nil && *httpSummary.MaxUpstreamClients > 0 {
		reasons = append(reasons, "within_guardrails")
	}

	return &OpsResourceSafetyZone{
		Status:  status,
		Detail:  detail,
		Reasons: reasons,
	}
}

func buildResourceBudgetJointSignals(
	cfg *config.Config,
	dbSummary *OpsDatabaseBudgetSummary,
	redisSummary *OpsRedisBudgetSummary,
	httpSummary *OpsHTTPUpstreamBudgetSummary,
) []*OpsBudgetJointSignal {
	signals := make([]*OpsBudgetJointSignal, 0, 3)
	if dbSummary != nil && dbSummary.Waiting != nil && *dbSummary.Waiting > 0 &&
		redisSummary != nil && redisSummary.UsagePercent != nil && *redisSummary.UsagePercent >= 70 {
		signals = append(signals, &OpsBudgetJointSignal{
			Level:      "warning",
			Title:      "DB wait is competing with a hot Redis control plane",
			Detail:     fmt.Sprintf("db_conn_waiting=%d while redis pool usage is %.1f%% of configured capacity.", *dbSummary.Waiting, *redisSummary.UsagePercent),
			Suggestion: "Pause further pool shrink steps and inspect Redis-heavy scheduler, sticky-session, and rate-limit traffic before adding more concurrency.",
		})
	}
	if cfg != nil && cfg.Database.MaxOpenConns > resourceBudgetDBMaxOpenGuardrail &&
		cfg.Redis.PoolSize > resourceBudgetRedisPoolGuardrail {
		signals = append(signals, &OpsBudgetJointSignal{
			Level:      "info",
			Title:      "DB and Redis are both configured above single-host guardrails",
			Detail:     fmt.Sprintf("max_open_conns=%d and redis.pool_size=%d both exceed current service guardrails.", cfg.Database.MaxOpenConns, cfg.Redis.PoolSize),
			Suggestion: "Gray-shrink one layer at a time and use the watch thresholds to avoid trading idle reservation for real contention.",
		})
	}
	if cfg != nil && cfg.Gateway.MaxUpstreamClients > resourceBudgetHTTPClientCacheGuardrail &&
		httpSummary != nil && httpSummary.MaxUpstreamClients != nil {
		signals = append(signals, &OpsBudgetJointSignal{
			Level:      "info",
			Title:      "HTTP client cache can amplify shared-resource contention",
			Detail:     fmt.Sprintf("max_upstream_clients=%d keeps the upstream cache very wide relative to this host budget.", *httpSummary.MaxUpstreamClients),
			Suggestion: "Shrink client-cache and idle-pool ceilings before tightening per-host connection caps under burst traffic.",
		})
	}
	if len(signals) == 0 {
		return nil
	}
	return signals
}

func int64PtrOpsDashboard(v int64) *int64 { return &v }

func (s *OpsService) StorageGovernanceSummary(cleanupStats CleanupStats, usageCleanupStats UsageCleanupStats, heartbeats []*OpsJobHeartbeat) *OpsStorageGovernanceSummary {
	if s == nil || s.cfg == nil {
		return nil
	}
	cfg := s.cfg
	opsCleanupHeartbeat := summarizeJobHeartbeat(findJobHeartbeat(heartbeats, opsCleanupJobName))
	usageLogsGovernance := summarizeUsageLogsGovernance()
	storageRuntime := summarizeStorageGovernanceRuntime()
	return &OpsStorageGovernanceSummary{
		OpsCleanup: &OpsCleanupGovernanceSummary{
			Enabled:                    cfg.Ops.Cleanup.Enabled,
			ErrorLogRetentionDays:      cfg.Ops.Cleanup.ErrorLogRetentionDays,
			MinuteMetricsRetentionDays: cfg.Ops.Cleanup.MinuteMetricsRetentionDays,
			HourlyMetricsRetentionDays: cfg.Ops.Cleanup.HourlyMetricsRetentionDays,
			SystemLogMaxRows:           cfg.Ops.Cleanup.SystemLogMaxRows,
			ErrorLogMaxRows:            cfg.Ops.Cleanup.ErrorLogMaxRows,
			MaxRowsEnabled:             cfg.Ops.Cleanup.MaxRowsEnabled,
			MaxRowsDryRun:              cfg.Ops.Cleanup.MaxRowsDryRun,
			SystemLogRows:              cleanupStats.SystemLogRows,
			ErrorLogRows:               cleanupStats.ErrorLogRows,
			MaxRowsHit:                 cleanupStats.MaxRowsHit,
			Heartbeat:                  opsCleanupHeartbeat,
		},
		UsageCleanup: &OpsUsageCleanupGovernanceInfo{
			Enabled:                cfg.UsageCleanup.Enabled,
			MaxRangeDays:           cfg.UsageCleanup.MaxRangeDays,
			BatchSize:              cfg.UsageCleanup.BatchSize,
			WorkerIntervalSeconds:  cfg.UsageCleanup.WorkerIntervalSeconds,
			TaskTimeoutSeconds:     cfg.UsageCleanup.TaskTimeoutSeconds,
			UsageLogsRetentionDays: cfg.DashboardAgg.Retention.UsageLogsDays,
			UsageLogsMaxRows:       cfg.DashboardAgg.Retention.UsageLogsMaxRows,
			LastTaskID:             usageCleanupStats.LastTaskID,
			LastDeletedRows:        usageCleanupStats.LastDeletedRows,
			StartedTotal:           usageCleanupStats.StartedTotal,
			SucceededTotal:         usageCleanupStats.SucceededTotal,
			FailedTotal:            usageCleanupStats.FailedTotal,
			CanceledTotal:          usageCleanupStats.CanceledTotal,
			LastStartedAt:          usageCleanupStats.LastStartedAt,
			LastSucceededAt:        usageCleanupStats.LastSucceededAt,
			LastFailedAt:           usageCleanupStats.LastFailedAt,
			LastCanceledAt:         usageCleanupStats.LastCanceledAt,
			LastDurationMs:         usageCleanupStats.LastDurationMs,
			LastStatus:             usageCleanupStats.LastStatus,
			LastError:              usageCleanupStats.LastError,
		},
		UsageLogs: usageLogsGovernance,
		DryRun: &OpsStorageGovernanceDryRunSummary{
			OpsCleanup: &OpsCleanupDryRunSummary{
				Enabled:               cleanupStats.MaxRowsEnabled,
				DryRun:                cleanupStats.MaxRowsDryRun,
				SystemLogRows:         cleanupStats.SystemLogRows,
				SystemLogLimit:        cleanupStats.SystemLogLimit,
				SystemLogUsagePercent: usagePercentInt64(cleanupStats.SystemLogRows, cleanupStats.SystemLogLimit),
				SystemLogOverLimit:    cleanupStats.SystemLogLimit > 0 && cleanupStats.SystemLogRows > cleanupStats.SystemLogLimit,
				ErrorLogRows:          cleanupStats.ErrorLogRows,
				ErrorLogLimit:         cleanupStats.ErrorLogLimit,
				ErrorLogUsagePercent:  usagePercentInt64(cleanupStats.ErrorLogRows, cleanupStats.ErrorLogLimit),
				ErrorLogOverLimit:     cleanupStats.ErrorLogLimit > 0 && cleanupStats.ErrorLogRows > cleanupStats.ErrorLogLimit,
				MaxRowsHit:            cleanupStats.MaxRowsHit,
			},
			UsageCleanup: &OpsUsageCleanupDryRunInfo{
				Enabled:                cfg.UsageCleanup.Enabled,
				UsageLogsMaxRows:       cfg.DashboardAgg.Retention.UsageLogsMaxRows,
				UsageLogsRetentionDays: cfg.DashboardAgg.Retention.UsageLogsDays,
				LastTaskID:             usageCleanupStats.LastTaskID,
				LastDeletedRows:        usageCleanupStats.LastDeletedRows,
				UsageLogRowsEstimated:  usageLogRowsEstimated(usageLogsGovernance),
				UsageLogUsagePercent:   usageLogUsagePercent(usageLogsGovernance),
				UsageLogOverLimit:      usageLogsGovernance != nil && usageLogsGovernance.MaxRowsExceeded,
				RetentionRisk:          usageLogRetentionRisk(usageLogsGovernance),
				Reasons:                usageLogReasons(usageLogsGovernance),
				LastSampledAt:          usageLogSampledAt(usageLogsGovernance),
				EnforcementMode:        usageCleanupEnforcementMode(cfg),
				Note:                   usageCleanupDryRunNote(cfg),
			},
		},
		Runtime: storageRuntime,
	}
}

func usagePercentInt64(current, limit int64) *float64 {
	if limit <= 0 || current < 0 {
		return nil
	}
	pct := float64(current) / float64(limit) * 100
	if pct < 0 {
		pct = 0
	}
	if pct > 200 {
		pct = 200
	}
	return float64Ptr(roundTo1DP(pct))
}

func usageCleanupEnforcementMode(cfg *config.Config) string {
	if cfg == nil {
		return "disabled"
	}
	if cfg.DashboardAgg.Retention.UsageLogsMaxRows > 0 {
		return "retention_plus_row_cap_configured"
	}
	if cfg.DashboardAgg.Retention.UsageLogsDays > 0 {
		return "retention_only"
	}
	return "disabled"
}

func usageCleanupDryRunNote(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	if cfg.DashboardAgg.Retention.UsageLogsMaxRows > 0 {
		return "usage_logs row-cap is configured; use the estimated row-count summary and dry-run usage percent before enabling any automatic trimming in high-concurrency windows."
	}
	if cfg.DashboardAgg.Retention.UsageLogsDays > 0 {
		return "usage_logs retention is configured by days; no row-cap dry-run is active, so prefer pre-aggregated summary views during peak traffic."
	}
	return "usage_logs lifecycle governance is effectively disabled."
}

func summarizeUsageLogsGovernance() *OpsUsageLogsGovernanceSummary {
	snapshot, sampledAt := snapshotLatestStorageGovernanceRuntime()
	if snapshot == nil {
		return nil
	}
	for _, table := range snapshot.Tables {
		if table.Table != "usage_logs" {
			continue
		}
		summary := &OpsUsageLogsGovernanceSummary{
			EstimatedLiveRows:       table.LiveRows,
			EstimatedDeadRows:       table.DeadRows,
			TotalMB:                 table.TotalMB,
			Partitioned:             table.Partitioned,
			RetentionDays:           table.RetentionDays,
			RetentionEnabled:        table.RetentionEnabled,
			MaxRowsConfigured:       table.MaxRowsConfigured,
			MaxRowsEnabled:          table.MaxRowsEnabled,
			MaxRowsExceeded:         table.MaxRowsExceeded,
			SizeRisk:                table.SizeRisk,
			RetentionRisk:           table.RetentionRisk,
			ColdIndexRisk:           table.ColdIndexRisk,
			Reasons:                 cloneStringSliceOpsDashboard(table.Reasons),
			ApproxBytesPerLiveRow:   table.ApproxBytesPerLiveRow,
			ColdIndexCandidateCount: table.ColdIndexCandidateCount,
			ColdIndexCandidateMB:    roundTo1DP(float64(table.ColdIndexCandidateBytes) / float64(bytesPerMB)),
		}
		if sampledAt != nil {
			summary.LastSampledAt = sampledAt
		}
		return summary
	}
	return nil
}

func summarizeStorageGovernanceRuntime() *OpsStorageGovernanceRuntimeInfo {
	snapshot, sampledAt := snapshotLatestStorageGovernanceRuntime()
	if snapshot == nil {
		return nil
	}
	out := &OpsStorageGovernanceRuntimeInfo{
		SampledTables:  snapshot.SampledTables,
		RiskTables:     snapshot.RiskTables,
		WarnTables:     snapshot.WarnTables,
		CriticalTables: snapshot.CriticalTables,
		Tables:         make([]*OpsStorageGovernanceRuntimeTableInfo, 0, len(snapshot.Tables)),
	}
	if sampledAt != nil {
		out.SampledAt = sampledAt
	}
	for _, table := range snapshot.Tables {
		out.Tables = append(out.Tables, &OpsStorageGovernanceRuntimeTableInfo{
			Table:                 table.Table,
			LiveRows:              table.LiveRows,
			DeadRows:              table.DeadRows,
			TotalBytes:            table.TotalBytes,
			TotalMB:               table.TotalMB,
			Partitioned:           table.Partitioned,
			MaxRowsConfigured:     table.MaxRowsConfigured,
			MaxRowsEnabled:        table.MaxRowsEnabled,
			MaxRowsExceeded:       table.MaxRowsExceeded,
			RetentionDays:         table.RetentionDays,
			RetentionEnabled:      table.RetentionEnabled,
			RetentionRisk:         strings.TrimSpace(table.RetentionRisk),
			SizeRisk:              strings.TrimSpace(table.SizeRisk),
			ColdIndexRisk:         strings.TrimSpace(table.ColdIndexRisk),
			ApproxBytesPerLiveRow: table.ApproxBytesPerLiveRow,
			Reasons:               cloneStringSliceOpsDashboard(table.Reasons),
		})
	}
	return out
}

func usageLogRowsEstimated(summary *OpsUsageLogsGovernanceSummary) int64 {
	if summary == nil {
		return 0
	}
	return summary.EstimatedLiveRows
}

func usageLogUsagePercent(summary *OpsUsageLogsGovernanceSummary) *float64 {
	if summary == nil {
		return nil
	}
	return usagePercentInt64(summary.EstimatedLiveRows, summary.MaxRowsConfigured)
}

func usageLogRetentionRisk(summary *OpsUsageLogsGovernanceSummary) string {
	if summary == nil {
		return ""
	}
	return strings.TrimSpace(summary.RetentionRisk)
}

func usageLogReasons(summary *OpsUsageLogsGovernanceSummary) []string {
	if summary == nil || len(summary.Reasons) == 0 {
		return nil
	}
	return cloneStringSliceOpsDashboard(summary.Reasons)
}

func usageLogSampledAt(summary *OpsUsageLogsGovernanceSummary) *time.Time {
	if summary == nil || summary.LastSampledAt == nil {
		return nil
	}
	return summary.LastSampledAt
}

func UsageIntegritySummary() *OpsUsageIntegritySummary {
	usage := SnapshotUsageLogNotPersistedMetrics()
	billing := SnapshotBillingCompensationCandidates()
	if usage.Total == 0 && billing.Total == 0 {
		return nil
	}
	summary := &OpsUsageIntegritySummary{
		Advisory: "Inspect persisted billing-compensation and usage-log-not-persisted endpoints before replaying raw usage queries during peak load.",
	}
	if signal := buildUsageIntegritySignal(
		usage.Total,
		usage.Recent1mTotal,
		usage.Recent5mTotal,
		usage.Recent15mTotal,
		usage.Last,
		"/api/v1/admin/ops/usage-log-not-persisted",
		"/api/v1/admin/ops/usage-log-not-persisted/:request_id",
	); signal != nil {
		summary.UsageLogNotPersisted = signal
	}
	if signal := buildBillingIntegritySignal(billing); signal != nil {
		summary.BillingCompensation = signal
	}
	return summary
}

func buildUsageIntegritySignal(total, recent1m, recent5m, recent15m int64, last *UsageLogNotPersistedAlert, endpoint, detail string) *OpsUsageIntegritySignal {
	if total <= 0 && last == nil {
		return nil
	}
	out := &OpsUsageIntegritySignal{
		Total:                  total,
		Recent1mTotal:          recent1m,
		Recent5mTotal:          recent5m,
		Recent15mTotal:         recent15m,
		Endpoint:               endpoint,
		DetailEndpointTemplate: detail,
	}
	if last != nil {
		out.LastSeenAt = &last.Timestamp
		out.LastRequestID = strings.TrimSpace(last.RequestID)
		out.LastAccountID = last.AccountID
		out.LastError = strings.TrimSpace(last.Error)
	}
	return out
}

func buildBillingIntegritySignal(snapshot BillingCompensationSnapshot) *OpsUsageIntegritySignal {
	if snapshot.Total <= 0 && snapshot.Last == nil {
		return nil
	}
	out := &OpsUsageIntegritySignal{
		Total:                  snapshot.Total,
		Recent1mTotal:          snapshot.Recent1mTotal,
		Recent5mTotal:          snapshot.Recent5mTotal,
		Recent15mTotal:         snapshot.Recent15mTotal,
		Endpoint:               "/api/v1/admin/ops/billing-compensation",
		DetailEndpointTemplate: "/api/v1/admin/ops/billing-compensation/:request_id",
	}
	if snapshot.Last != nil {
		out.LastSeenAt = &snapshot.Last.Timestamp
		out.LastRequestID = strings.TrimSpace(snapshot.Last.RequestID)
		out.LastAccountID = snapshot.Last.AccountID
		out.LastError = strings.TrimSpace(snapshot.Last.Error)
	}
	return out
}

func cloneStringSliceOpsDashboard(src []string) []string {
	if len(src) == 0 {
		return nil
	}
	out := make([]string, len(src))
	copy(out, src)
	return out
}

func findJobHeartbeat(heartbeats []*OpsJobHeartbeat, jobName string) *OpsJobHeartbeat {
	for _, hb := range heartbeats {
		if hb != nil && hb.JobName == jobName {
			return hb
		}
	}
	return nil
}

func summarizeJobHeartbeat(hb *OpsJobHeartbeat) *OpsJobHeartbeatSummary {
	if hb == nil {
		return nil
	}
	return &OpsJobHeartbeatSummary{
		LastRunAt:      hb.LastRunAt,
		LastSuccessAt:  hb.LastSuccessAt,
		LastErrorAt:    hb.LastErrorAt,
		LastError:      hb.LastError,
		LastDurationMs: hb.LastDurationMs,
		LastResult:     hb.LastResult,
		UpdatedAt:      hb.UpdatedAt,
	}
}

func buildSlowPathDiagnostics(overview *OpsDashboardOverview) *OpsSlowPathDiagnostics {
	if overview == nil {
		return nil
	}
	diag := &OpsSlowPathDiagnostics{
		SlowRequestThresholdMs: slowPathRequestThresholdMs,
		RequestDetailsEndpoint: "/api/v1/admin/ops/requests?sort=duration_desc&min_duration_ms=1000",
		RequestDetailsHint:     "Use the request details endpoint with min_duration_ms to drill into the slowest requests in the current window.",
		PreferSummaryFirst:     true,
		DurationP95Ms:          overview.Duration.P95,
		DurationP99Ms:          overview.Duration.P99,
		DurationMaxMs:          overview.Duration.Max,
		TTFTP95Ms:              overview.TTFT.P95,
		TTFTP99Ms:              overview.TTFT.P99,
		DrilldownEndpoints: []string{
			"/api/v1/admin/ops/dashboard/snapshot-v2",
			"/api/v1/admin/ops/requests?sort=duration_desc&min_duration_ms=1000",
			"/api/v1/admin/ops/request-errors/:id",
		},
		InvestigationOrder: []string{
			"Check request details for the slowest requests in the current window.",
			"Inspect snapshot-v2 overview.query_path/data_source before opening heavier raw queries.",
			"Review resource_budget_summary and scheduler_outbox_runtime if DB wait or backlog signals are already elevated.",
		},
		CorrelationHint: "Correlate request_id and client_request_id across request details, runtime anomaly summaries, and persisted ops drill-down endpoints before replaying requests or increasing failover.",
	}
	if overview.QueryPath != nil {
		diag.QueryPathReason = strings.TrimSpace(overview.QueryPath.Reason)
		diag.RawPathUsed = overview.QueryPath.Mode == string(OpsQueryModeRaw)
	} else if overview.DataSource != nil {
		diag.RawPathUsed = strings.TrimSpace(overview.DataSource.Mode) == string(OpsQueryModeRaw)
	}
	if diag.RawPathUsed {
		diag.RawRiskLevel = "warn"
		diag.GuardrailHint = "This window is already using raw SQL. Narrow the time range and inspect summary endpoints before opening broader request or error scans."
	} else {
		diag.RawRiskLevel = "low"
		diag.GuardrailHint = "Summary/pre-aggregated paths are available for this window. Prefer snapshot-v2 and request detail drilldown before any ad-hoc raw investigation."
	}
	if overview.SystemMetrics != nil {
		diag.DBConnWaiting = overview.SystemMetrics.DBConnWaiting
		diag.SQLObservabilityReady = true
		diag.SQLObservabilityDetail = "System metrics are available for this window; use DB wait counts and ops request drilldowns before running ad-hoc SQL."
	} else {
		diag.SQLObservabilityReady = false
		diag.SQLObservabilityDetail = "System metrics are unavailable for this window, so SQL-level observability is degraded; confirm pg_stat_statements ingestion first."
	}
	signals := make([]string, 0, 4)
	if overview.Duration.P95 != nil && *overview.Duration.P95 >= slowPathRequestThresholdMs {
		signals = append(signals, fmt.Sprintf("duration_p95_ge_%dms", slowPathRequestThresholdMs))
	}
	if overview.Duration.P99 != nil && *overview.Duration.P99 >= slowPathP99WarningMs {
		signals = append(signals, fmt.Sprintf("duration_p99_ge_%dms", slowPathP99WarningMs))
	}
	if overview.TTFT.P95 != nil && *overview.TTFT.P95 >= slowPathRequestThresholdMs {
		signals = append(signals, fmt.Sprintf("ttft_p95_ge_%dms", slowPathRequestThresholdMs))
	}
	if overview.SystemMetrics != nil && overview.SystemMetrics.DBConnWaiting != nil && *overview.SystemMetrics.DBConnWaiting > 0 {
		signals = append(signals, "db_conn_waiting_positive")
	}
	if diag.RawPathUsed {
		signals = append(signals, "raw_query_path")
	}
	if len(signals) > 0 {
		diag.SlowSignals = signals
	}
	return diag
}

func buildResourceBudgetRecommendations(
	cfg *config.Config,
	dbSummary *OpsDatabaseBudgetSummary,
	redisSummary *OpsRedisBudgetSummary,
	httpSummary *OpsHTTPUpstreamBudgetSummary,
) []*OpsBudgetRecommendation {
	if cfg == nil {
		return nil
	}
	recommendations := make([]*OpsBudgetRecommendation, 0, 6)

	if cfg.Database.MaxOpenConns > resourceBudgetDBMaxOpenGuardrail {
		recommendations = append(recommendations, &OpsBudgetRecommendation{
			Area:      "database",
			Level:     "info",
			Current:   fmt.Sprintf("max_open_conns=%d", cfg.Database.MaxOpenConns),
			Suggested: fmt.Sprintf("phase 1: reduce toward <= %d and watch db_conn_waiting / WaitCount deltas", resourceBudgetDBMaxOpenGuardrail),
			Reason:    "Configured DB pool cap is above the service guardrail and may over-reserve shared PostgreSQL connections under burst load.",
		})
	}
	if cfg.Database.MaxOpenConns > 0 && cfg.Database.MaxIdleConns > 0 {
		idleRatio := float64(cfg.Database.MaxIdleConns) / float64(cfg.Database.MaxOpenConns)
		if idleRatio >= resourceBudgetDBIdleRatioGuardrail {
			recommendations = append(recommendations, &OpsBudgetRecommendation{
				Area:      "database",
				Level:     "info",
				Current:   fmt.Sprintf("max_idle_conns=%d (%.0f%% of max_open_conns)", cfg.Database.MaxIdleConns, idleRatio*100),
				Suggested: "phase 1: trim idle connections below 90% of max_open_conns before shrinking further",
				Reason:    "High idle ratio keeps many database connections warm even when concurrency is low, which reduces shared headroom.",
			})
		}
	}
	if dbSummary != nil && dbSummary.UsagePercent != nil && *dbSummary.UsagePercent < resourceBudgetLowUsageThresholdPercent && cfg.Database.MaxOpenConns > 0 {
		recommendations = append(recommendations, &OpsBudgetRecommendation{
			Area:      "database",
			Level:     "info",
			Current:   fmt.Sprintf("observed db usage %.1f%% of configured pool", *dbSummary.UsagePercent),
			Suggested: "phase 1: trial a 20-25% pool reduction during off-peak, then compare db_conn_waiting before and after",
			Reason:    "Observed active DB usage is well below the configured cap, so there may be room to reclaim capacity without hurting throughput.",
		})
	}

	if cfg.Redis.PoolSize > resourceBudgetRedisPoolGuardrail {
		recommendations = append(recommendations, &OpsBudgetRecommendation{
			Area:      "redis",
			Level:     "info",
			Current:   fmt.Sprintf("pool_size=%d", cfg.Redis.PoolSize),
			Suggested: fmt.Sprintf("phase 1: reduce toward <= %d and watch stalls/timeouts after each step", resourceBudgetRedisPoolGuardrail),
			Reason:    "Configured Redis pool exceeds the service guardrail and can monopolize shared Redis connections on a single host.",
		})
	}
	if cfg.Redis.PoolSize > 0 && cfg.Redis.MinIdleConns > 0 {
		idleRatio := float64(cfg.Redis.MinIdleConns) / float64(cfg.Redis.PoolSize)
		if idleRatio >= resourceBudgetRedisMinIdleRatioGuardrail {
			recommendations = append(recommendations, &OpsBudgetRecommendation{
				Area:      "redis",
				Level:     "info",
				Current:   fmt.Sprintf("min_idle_conns=%d (%.0f%% of pool_size)", cfg.Redis.MinIdleConns, idleRatio*100),
				Suggested: "phase 1: lower min_idle_conns before reducing pool_size so hot connections stay available with less reservation",
				Reason:    "High Redis idle reservation can hold unnecessary sockets under steady load and hides the real working-set size.",
			})
		}
	}
	if redisSummary != nil && redisSummary.UsagePercent != nil && *redisSummary.UsagePercent < resourceBudgetLowUsageThresholdPercent && cfg.Redis.PoolSize > 0 {
		recommendations = append(recommendations, &OpsBudgetRecommendation{
			Area:      "redis",
			Level:     "info",
			Current:   fmt.Sprintf("observed redis usage %.1f%% of configured pool", *redisSummary.UsagePercent),
			Suggested: "phase 1: trial a 20-25% pool reduction and compare stalls/timeouts before moving further",
			Reason:    "Observed Redis pool usage is low relative to the configured cap, suggesting room for a cautious gray shrink.",
		})
	}

	if cfg.Gateway.MaxIdleConns > resourceBudgetHTTPMaxIdleGuardrail ||
		cfg.Gateway.MaxIdleConnsPerHost > resourceBudgetHTTPIdlePerHostGuardrail ||
		cfg.Gateway.MaxConnsPerHost > resourceBudgetHTTPMaxConnsPerHostGuardrail ||
		cfg.Gateway.MaxUpstreamClients > resourceBudgetHTTPClientCacheGuardrail {
		current := fmt.Sprintf("max_idle=%d max_idle_per_host=%d max_conns_per_host=%d max_upstream_clients=%d",
			cfg.Gateway.MaxIdleConns,
			cfg.Gateway.MaxIdleConnsPerHost,
			cfg.Gateway.MaxConnsPerHost,
			cfg.Gateway.MaxUpstreamClients,
		)
		recommendations = append(recommendations, &OpsBudgetRecommendation{
			Area:    "http_upstream",
			Level:   "info",
			Current: current,
			Suggested: fmt.Sprintf("phase 1: shrink toward idle<=%d per_host<=%d conns<=%d clients<=%d with canary traffic",
				resourceBudgetHTTPMaxIdleGuardrail,
				resourceBudgetHTTPIdlePerHostGuardrail,
				resourceBudgetHTTPMaxConnsPerHostGuardrail,
				resourceBudgetHTTPClientCacheGuardrail),
			Reason: "Aggressive upstream HTTP pools can pin sockets and client cache entries, reducing room for burst concurrency on the same host.",
		})
	}

	if len(recommendations) == 0 {
		return nil
	}
	return recommendations
}

func (s *OpsService) listRecentRuntimeAnomalies(ctx context.Context, filter *OpsDashboardFilter, limit int) ([]*OpsSystemLog, error) {
	if s == nil || s.opsRepo == nil || filter == nil || limit <= 0 {
		return nil, nil
	}

	components := []string{
		OpsRuntimeUsageLogComponent,
		OpsRuntimeUsageLogSummaryComponent,
		OpsRuntimeBillingCompensationComponent,
		OpsRuntimeBillingCompensationSummaryComponent,
		OpsRuntimeUsageWorkerSummaryComponent,
		OpsRuntimeRedisPoolSummaryComponent,
		OpsRuntimeSchedulerOutboxSummaryComponent,
		OpsRuntimeStorageGovernanceSummaryComponent,
		OpsRuntimeErrorFamilySummaryComponent,
		"ops.runtime.cleanup.summary",
		"ops.runtime.cleanup.usage.summary",
	}
	logs := make([]*OpsSystemLog, 0, len(components)*limit)
	var errs []error
	for _, component := range components {
		list, err := s.opsRepo.ListSystemLogs(ctx, &OpsSystemLogFilter{
			Page:      1,
			PageSize:  limit,
			Level:     "warn",
			Component: component,
			StartTime: &filter.StartTime,
			EndTime:   &filter.EndTime,
		})
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", component, err))
			continue
		}
		if list != nil {
			logs = append(logs, list.Logs...)
		}
	}
	if len(logs) == 0 {
		if len(errs) > 0 {
			return nil, errors.Join(errs...)
		}
		return nil, nil
	}
	sort.SliceStable(logs, func(i, j int) bool {
		return logs[i].CreatedAt.After(logs[j].CreatedAt)
	})
	if len(logs) > limit {
		logs = logs[:limit]
	}
	if len(errs) > 0 {
		return logs, errors.Join(errs...)
	}
	return logs, nil
}

func (s *OpsService) resolveOpsQueryMode(ctx context.Context, requested OpsQueryMode) OpsQueryMode {
	if requested.IsValid() {
		// Allow "auto" to be disabled via config until preagg is proven stable in production.
		// Forced `preagg` via query param still works.
		if requested == OpsQueryModeAuto && s != nil && s.cfg != nil && !s.cfg.Ops.UsePreaggregatedTables {
			return OpsQueryModeRaw
		}
		return requested
	}

	mode := OpsQueryModeAuto
	if s != nil && s.settingRepo != nil {
		if raw, err := s.settingRepo.GetValue(ctx, SettingKeyOpsQueryModeDefault); err == nil {
			mode = ParseOpsQueryMode(raw)
		}
	}

	if mode == OpsQueryModeAuto && s != nil && s.cfg != nil && !s.cfg.Ops.UsePreaggregatedTables {
		return OpsQueryModeRaw
	}
	return mode
}

func (s *OpsService) collectResourceBudgetNotices() []*OpsObservabilityNotice {
	if s == nil || s.cfg == nil {
		return nil
	}
	var notices []*OpsObservabilityNotice
	const (
		dbMaxOpenThreshold           = 500
		redisPoolSizeThreshold       = 512
		redisMinIdleRatioThreshold   = 0.85
		httpMaxIdleThreshold         = 1024
		httpMaxIdlePerHostThreshold  = 512
		httpMaxConnsPerHostThreshold = 1024
		httpMaxUpstreamClientsHint   = 5000
	)

	if s.cfg.Database.MaxOpenConns > dbMaxOpenThreshold {
		notices = append(notices, &OpsObservabilityNotice{
			Level:      "info",
			Title:      "Database pool sized aggressively",
			Detail:     fmt.Sprintf("MaxOpenConns=%d may exceed deployment capacity; idle/conn usage should stay under %d.", s.cfg.Database.MaxOpenConns, dbMaxOpenThreshold),
			Suggestion: "Consider scaling the pool to observed concurrent connections before expanding pg_stat_statements coverage.",
		})
	}

	redisCfg := s.cfg.Redis
	if redisCfg.PoolSize > redisPoolSizeThreshold {
		notices = append(notices, &OpsObservabilityNotice{
			Level:      "info",
			Title:      "Redis pool configured high",
			Detail:     fmt.Sprintf("PoolSize=%d can tie up many connections; watch for ratelimit/memory pressure.", redisCfg.PoolSize),
			Suggestion: "Tune PoolSize/MinIdle down while monitoring redis timeout/stall metrics.",
		})
	}
	if redisCfg.PoolSize > 0 && redisCfg.MinIdleConns > 0 {
		ratio := float64(redisCfg.MinIdleConns) / float64(redisCfg.PoolSize)
		if ratio >= redisMinIdleRatioThreshold {
			notices = append(notices, &OpsObservabilityNotice{
				Level:      "info",
				Title:      "Redis min idle ratio high",
				Detail:     fmt.Sprintf("MinIdleConns=%d is %.0f%% of PoolSize=%d.", redisCfg.MinIdleConns, ratio*100, redisCfg.PoolSize),
				Suggestion: "Ensure idle connections stay busy by lowering MinIdle or increasing request fanout before reusing redis shards.",
			})
		}
	}

	gw := s.cfg.Gateway
	if gw.MaxIdleConns > httpMaxIdleThreshold ||
		gw.MaxIdleConnsPerHost > httpMaxIdlePerHostThreshold ||
		gw.MaxConnsPerHost > httpMaxConnsPerHostThreshold {
		notices = append(notices, &OpsObservabilityNotice{
			Level: "info",
			Title: "HTTP upstream pool configured aggressively",
			Detail: fmt.Sprintf("MaxIdleConns=%d MaxIdleConnsPerHost=%d MaxConnsPerHost=%d; large pools can hold sockets indefinitely.",
				gw.MaxIdleConns, gw.MaxIdleConnsPerHost, gw.MaxConnsPerHost),
			Suggestion: "Shrink HTTP pool settings toward gateway traffic baselines before adding more outbound connections.",
		})
	}
	if gw.MaxUpstreamClients > httpMaxUpstreamClientsHint {
		notices = append(notices, &OpsObservabilityNotice{
			Level:      "info",
			Title:      "HTTP client cache wide open",
			Detail:     fmt.Sprintf("MaxUpstreamClients=%d may keep thousands of idle clients; this uses shared connection limits.", gw.MaxUpstreamClients),
			Suggestion: "Limit cached clients and rely on per-account isolation to avoid cross-account resource exhaustion.",
		})
	}
	return notices
}

func (s *OpsService) collectObservabilityNotes(filter *OpsDashboardFilter, overview *OpsDashboardOverview) []*OpsObservabilityNotice {
	if overview == nil {
		return nil
	}
	notes := make([]*OpsObservabilityNotice, 0, 2)
	if overview.SystemMetrics == nil {
		notes = append(notes, &OpsObservabilityNotice{
			Level:      "warning",
			Title:      "SQL metrics unavailable",
			Detail:     "System metrics (pg_stat_statements) were not collected for this window, so SQL-level observability is degraded.",
			Suggestion: "Enable pg_stat_statements and ensure ops.system_metrics ingestion is healthy before relying on SQL insights.",
		})
	} else if overview.SystemMetrics.DBOK != nil && !*overview.SystemMetrics.DBOK {
		notes = append(notes, &OpsObservabilityNotice{
			Level:      "warning",
			Title:      "Database reporting degraded",
			Detail:     "The latest database health check indicates problems, which may impact SQL observation windows.",
			Suggestion: "Investigate Postgres availability and SQL stats access before triggering new ad-hoc queries.",
		})
	}
	if overview != nil && overview.DataSource != nil && overview.DataSource.Mode == string(OpsQueryModeRaw) {
		notes = append(notes, &OpsObservabilityNotice{
			Level:      "info",
			Title:      "Raw query fallback",
			Detail:     "Pre-aggregated tables are disabled or unavailable, so the dashboard is running raw SQL scans and metrics appear degraded.",
			Suggestion: "Schedule this window as read-only or restore pre-aggregated tables to reduce load on pg_stat_statements.",
		})
	}
	if overview.TokenRefreshSummary != nil && overview.TokenRefreshSummary.Total > 0 {
		notes = append(notes, &OpsObservabilityNotice{
			Level:      "warning",
			Title:      tokenRefreshObservabilityTitle,
			Detail:     fmt.Sprintf("Detected %d token refresh failure(s) in the current window. Inspect affected accounts for token_refresh_failure_reason/class metadata.", overview.TokenRefreshSummary.Total),
			Suggestion: "Review token refresh queues, verify OAuth provider quotas, and consider temporary manual retries for permanent failures.",
		})
	}
	if overview.AccountAuthSummary != nil && overview.AccountAuthSummary.Total > 0 {
		notes = append(notes, &OpsObservabilityNotice{
			Level:      "warning",
			Title:      "Account auth failures visible",
			Detail:     fmt.Sprintf("Detected %d account auth failure(s), including %d permanent, %d temporary, %d dispatch-suppressed, and %d background-recovery-eligible cases. Review account_auth_failure_reason/class/source/state metadata before re-enabling accounts.", overview.AccountAuthSummary.Total, overview.AccountAuthSummary.PermanentCount, overview.AccountAuthSummary.TemporaryCount, overview.AccountAuthSummary.DispatchSuppressed, overview.AccountAuthSummary.BackgroundRecovery),
			Suggestion: "Separate reauthorization-required accounts from background-refresh-pending accounts so bad credentials stop recycling through scheduling.",
		})
	}
	if overview.SchedulerCheckpoint != nil {
		cp := overview.SchedulerCheckpoint
		if cp.CheckpointFallbackTotal > 0 || cp.CheckpointReadFailures > 0 || cp.CheckpointWriteFailures > 0 || cp.WatermarkDrift != 0 || cp.BlockedEvent != nil {
			detail := fmt.Sprintf("Scheduler checkpoint ran %d fallback(s) with streak=%d, %d read failure(s), and %d write failure(s). Redis=%d checkpoint=%d drift=%d.",
				cp.CheckpointFallbackTotal, cp.CheckpointFallbackStreak, cp.CheckpointReadFailures, cp.CheckpointWriteFailures, cp.RedisWatermark, cp.LastCheckpointWatermark, cp.WatermarkDrift)
			if cp.CheckpointLastFallbackAt != "" || cp.CheckpointLastReadFailureAt != "" || cp.CheckpointLastWriteFailureAt != "" {
				detail = fmt.Sprintf("%s last_fallback=%s fallback_reason=%s last_read_failure=%s last_write_failure=%s.",
					detail,
					blankToNA(cp.CheckpointLastFallbackAt),
					blankToNA(cp.CheckpointLastFallbackReason),
					blankToNA(cp.CheckpointLastReadFailureAt),
					blankToNA(cp.CheckpointLastWriteFailureAt),
				)
			}
			if cp.BlockedEvent != nil {
				detail = fmt.Sprintf("%s Blocked event id=%d type=%s reason=%s attempts=%d age_seconds=%d.",
					detail, cp.BlockedEvent.ID, cp.BlockedEvent.EventType, cp.BlockedEvent.Reason, cp.BlockedEvent.Attempts, cp.BlockedEvent.AgeSeconds)
			}
			notes = append(notes, &OpsObservabilityNotice{
				Level:      "warning",
				Title:      schedulerCheckpointObservabilityTitle,
				Detail:     detail,
				Suggestion: "Check scheduler_outbox checkpointing, ensure Postgres connectivity, and confirm outbox jobs are writing to persistent storage.",
			})
		}
	}
	if overview.SchedulerOutboxRuntime != nil && (overview.SchedulerOutboxRuntime.BacklogRows > 0 || overview.SchedulerOutboxRuntime.LagSeconds > 0 || overview.SchedulerOutboxRuntime.LagRebuildTotal > 0 || overview.SchedulerOutboxRuntime.BacklogRebuildTotal > 0) {
		runtime := overview.SchedulerOutboxRuntime
		notes = append(notes, &OpsObservabilityNotice{
			Level:      "warning",
			Title:      "Scheduler outbox lag signals detected",
			Detail:     fmt.Sprintf("Backlog=%d lag_seconds=%d lag_failure_streak=%d lag_rebuilds=%d backlog_rebuilds=%d blocked_clears=%d bucket_rebuild_failures=%d lock_contention=%d.", runtime.BacklogRows, runtime.LagSeconds, runtime.LagFailureStreak, runtime.LagRebuildTotal, runtime.BacklogRebuildTotal, runtime.BlockedEventClearTotal, runtime.BucketRebuildFailureTotal, runtime.BucketRebuildLockContention),
			Suggestion: "Inspect scheduler_outbox_runtime before enabling any heavier rebuild or cleanup actions during peak traffic.",
		})
	}
	if overview.StickySessionCleanup != nil && overview.StickySessionCleanup.CleanupTotal > 0 {
		detail := fmt.Sprintf("Detected %d sticky-session cleanup event(s)", overview.StickySessionCleanup.CleanupTotal)
		if overview.StickySessionCleanup.CompareDeleteMissTotal > 0 {
			detail = fmt.Sprintf("%s, including %d compare-delete miss(es)", detail, overview.StickySessionCleanup.CompareDeleteMissTotal)
		}
		detail += ". Review cleanup_reason_totals and compare_delete_miss_reason_totals for stale binding patterns."
		notes = append(notes, &OpsObservabilityNotice{
			Level:      "info",
			Title:      stickyCleanupObservabilityTitle,
			Detail:     detail,
			Suggestion: "Inspect sticky_session_cleanup and sticky_session_runtime metrics before enabling any automatic invalidation policy.",
		})
	}
	if overview.OpenAIAccountScheduler != nil {
		scheduler := overview.OpenAIAccountScheduler
		if scheduler.StickySessionGhostTotal > 0 || scheduler.StickySessionStaleTotal > 0 || scheduler.StickyWaitConflictTotal > 0 {
			detail := fmt.Sprintf("ghost=%d stale=%d sticky_wait_conflicts=%d over %d sticky lookup(s).",
				scheduler.StickySessionGhostTotal, scheduler.StickySessionStaleTotal, scheduler.StickyWaitConflictTotal, scheduler.StickySessionLookupTotal)
			notes = append(notes, &OpsObservabilityNotice{
				Level:      "info",
				Title:      "OpenAI sticky-session pressure detected",
				Detail:     detail,
				Suggestion: "Check openai_account_scheduler shadow_reason_totals together with sticky_session_cleanup reason totals before tuning sticky TTL or invalidation policies.",
			})
		}
	}
	if shouldWarnControlPlaneDrift(overview) {
		notes = append(notes, &OpsObservabilityNotice{
			Level:      "warning",
			Title:      "Control-plane drift signals detected",
			Detail:     buildControlPlaneDriftNoticeDetail(overview),
			Suggestion: "Treat sticky ghosting, scheduler watermark drift, and Redis control-plane pressure as a consistency problem first; avoid adding more concurrency or retry pressure until the drift source is understood.",
		})
	}
	if overview.FailureSplitSummary != nil && overview.FailureSplitSummary.LikelyPrimary != "" {
		notes = append(notes, &OpsObservabilityNotice{
			Level:      "info",
			Title:      "Failure split guidance available",
			Detail:     fmt.Sprintf("Current split suggests %s as the primary failure family.", overview.FailureSplitSummary.LikelyPrimary),
			Suggestion: overview.FailureSplitSummary.Suggestion,
		})
	}
	if overview.UsageIntegrity != nil {
		var details []string
		if usage := overview.UsageIntegrity.UsageLogNotPersisted; usage != nil && usage.Total > 0 {
			details = append(details, fmt.Sprintf("usage_log_not_persisted=%d", usage.Total))
		}
		if billing := overview.UsageIntegrity.BillingCompensation; billing != nil && billing.Total > 0 {
			details = append(details, fmt.Sprintf("billing_compensation=%d", billing.Total))
		}
		if len(details) > 0 {
			notes = append(notes, &OpsObservabilityNotice{
				Level:      "warning",
				Title:      "Usage integrity gaps detected",
				Detail:     fmt.Sprintf("Observed %s. These paths indicate billed requests whose usage trail needs operator review.", strings.Join(details, ", ")),
				Suggestion: "Use the persisted billing-compensation and usage-log-not-persisted drill-down endpoints before replaying usage or running broader raw queries.",
			})
		}
	}
	if overview.ErrorFamilySummary != nil && len(overview.ErrorFamilySummary.Families) > 0 {
		top := overview.ErrorFamilySummary.Families[0]
		notes = append(notes, &OpsObservabilityNotice{
			Level:      "warning",
			Title:      "High-frequency error family observed",
			Detail:     fmt.Sprintf("Top recent error family is owner=%s phase=%s type=%s status=%d endpoint=%s count=%d (%.1f%% of %d errors over %dm).", blankToNA(top.Owner), blankToNA(top.Phase), blankToNA(top.Type), top.StatusCode, blankToNA(top.InboundEndpoint), top.Count, top.SharePercent, overview.ErrorFamilySummary.TotalErrors, overview.ErrorFamilySummary.WindowMinutes),
			Suggestion: errorFamilyInvestigationSuggestion(top),
		})
	}
	if overview.SlowPathDiagnostics != nil && len(overview.SlowPathDiagnostics.SlowSignals) > 0 {
		notes = append(notes, &OpsObservabilityNotice{
			Level:      "info",
			Title:      "Slow-path investigation suggested",
			Detail:     fmt.Sprintf("Detected %d slow-path signal(s): %s.", len(overview.SlowPathDiagnostics.SlowSignals), strings.Join(overview.SlowPathDiagnostics.SlowSignals, ", ")),
			Suggestion: fmt.Sprintf("Inspect %s for the slowest requests in this window before issuing heavier SQL diagnostics.", overview.SlowPathDiagnostics.RequestDetailsEndpoint),
		})
	}
	if shouldWarnRawUsageAmplification(filter, overview) {
		detail := "The current dashboard window is using raw usage queries while usage_logs governance still indicates a large-table or retention risk."
		if usage := usageLogsGovernanceFromOverview(overview); usage != nil {
			detail = fmt.Sprintf("%s usage_logs live_rows=%d total_mb=%.1f retention_risk=%s.",
				detail, usage.EstimatedLiveRows, usage.TotalMB, strings.TrimSpace(usage.RetentionRisk))
		}
		notes = append(notes, &OpsObservabilityNotice{
			Level:      "warning",
			Title:      "Avoid raw usage queries during peak load",
			Detail:     detail,
			Suggestion: "Prefer overview/snapshot-v2 and persisted ops drill-down endpoints first; narrow raw usage windows before opening broader ad-hoc queries.",
		})
	}
	return notes
}

func buildOpsDriftTrendSummary(overview *OpsDashboardOverview) *OpsDriftTrendSummary {
	if overview == nil || overview.SchedulerOutboxRuntime == nil {
		return nil
	}
	scheduler := overview.SchedulerOutboxRuntime
	summary := &OpsDriftTrendSummary{
		Status: scheduler.DriftTrendStatus,
		Trend:  nonEmptyString(scheduler.DriftTrendDetail, scheduler.DriftTrendNarrative),
	}
	if summary.Trend == "" {
		summary.Trend = "no drift detected"
	}
	summary.Severity = driftSeverityFromOverview(overview)
	summary.StickyState = stickyDriftState(overview.StickyConsistency)
	summary.SchedulerState = schedulerDriftState(scheduler)
	if summary.SchedulerState == "stable" && hasCheckpointDriftSignal(overview.SchedulerCheckpoint) {
		summary.SchedulerState = "lag_streak"
	}
	summary.RedisState = redisPressureState(overview.ResourceBudgetSummary)
	if summary.StickyState != "" || summary.SchedulerState != "" || summary.RedisState != "" {
		summary.Notes = append(summary.Notes, "Sticky -> scheduler -> Redis trend highlights the shared control-plane drift horizon.")
	}
	return summary
}

func driftSeverityFromOverview(overview *OpsDashboardOverview) string {
	if overview == nil {
		return "info"
	}
	if overview.StickyConsistency != nil && overview.StickyConsistency.GhostInvalidations > 0 {
		return "warning"
	}
	if checkpoint := overview.SchedulerCheckpoint; checkpoint != nil {
		if checkpoint.CheckpointFallbackTotal > 0 ||
			checkpoint.CheckpointReadFailures > 0 ||
			checkpoint.CheckpointWriteFailures > 0 ||
			checkpoint.BlockedEvent != nil {
			return "warning"
		}
	}
	scheduler := overview.SchedulerOutboxRuntime
	if scheduler != nil && (scheduler.LagFailureStreak > 0 || scheduler.CheckpointFallbackStreak > 0 || scheduler.WatermarkDrift != 0 || scheduler.BacklogRows > 0) {
		return "warning"
	}
	budget := overview.ResourceBudgetSummary
	if budget != nil && budget.Redis != nil && budget.Redis.UsagePercent != nil {
		if *budget.Redis.UsagePercent >= 90 {
			return "warning"
		}
	}
	return "info"
}

func hasCheckpointDriftSignal(checkpoint *OpsSchedulerCheckpointSummary) bool {
	if checkpoint == nil {
		return false
	}
	return checkpoint.CheckpointFallbackTotal > 0 ||
		checkpoint.CheckpointReadFailures > 0 ||
		checkpoint.CheckpointWriteFailures > 0 ||
		checkpoint.BlockedEvent != nil
}

func stickyDriftState(snapshot *StickyConsistencyMetricsSnapshot) string {
	if snapshot == nil {
		return "clear"
	}
	switch {
	case snapshot.GhostInvalidations > 0:
		return "ghosting"
	case snapshot.GhostRatio > 0:
		return "drifting"
	default:
		return "stable"
	}
}

func schedulerDriftState(metrics *OpsSchedulerOutboxRuntimeSummary) string {
	if metrics == nil {
		return "stable"
	}
	switch {
	case metrics.LagFailureStreak > 0:
		return "lag_streak"
	case metrics.CheckpointFallbackStreak > 0:
		return "checkpoint_fallback"
	case metrics.WatermarkDrift != 0:
		return "watermark_drift"
	case metrics.BacklogRows > 0 || metrics.LagSeconds > 0:
		return "backlog"
	case metrics.BlockedEvent != nil:
		return "blocked_event"
	default:
		return "stable"
	}
}

func redisPressureState(budget *OpsResourceBudgetSummary) string {
	if budget == nil || budget.Redis == nil || budget.Redis.UsagePercent == nil {
		return "normal"
	}
	if *budget.Redis.UsagePercent >= 90 {
		return "high_usage"
	}
	return "normal"
}

func nonEmptyString(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func shouldWarnControlPlaneDrift(overview *OpsDashboardOverview) bool {
	if overview == nil {
		return false
	}
	if overview.StickyConsistency != nil && overview.StickyConsistency.GhostInvalidations > 0 {
		return true
	}
	if overview.SchedulerOutboxRuntime != nil && (overview.SchedulerOutboxRuntime.WatermarkDrift != 0 || overview.SchedulerOutboxRuntime.BacklogRows > 0 || overview.SchedulerOutboxRuntime.LagSeconds > 0) {
		return true
	}
	return false
}

func buildControlPlaneDriftNoticeDetail(overview *OpsDashboardOverview) string {
	parts := make([]string, 0, 3)
	likely := deriveOverviewControlPlaneDriftLikelyRootCause(overview)
	if likely != "" {
		parts = append(parts, "likely_root_cause="+likely)
	}
	if overview != nil && overview.StickyConsistency != nil && overview.StickyConsistency.GhostInvalidations > 0 {
		parts = append(parts, fmt.Sprintf("sticky ghost_invalidations=%d ghost_ratio=%.4f", overview.StickyConsistency.GhostInvalidations, overview.StickyConsistency.GhostRatio))
	}
	if overview != nil && overview.SchedulerOutboxRuntime != nil {
		runtime := overview.SchedulerOutboxRuntime
		if runtime.WatermarkDrift != 0 || runtime.BacklogRows > 0 || runtime.LagSeconds > 0 {
			parts = append(parts, fmt.Sprintf("scheduler watermark_drift=%d backlog=%d lag_seconds=%d", runtime.WatermarkDrift, runtime.BacklogRows, runtime.LagSeconds))
		}
	}
	if len(parts) == 0 {
		return "Multiple control-plane drift indicators are active."
	}
	return strings.Join(parts, "; ") + "."
}

func deriveOverviewControlPlaneDriftLikelyRootCause(overview *OpsDashboardOverview) string {
	if overview == nil {
		return ""
	}
	hasSticky := overview.StickyConsistency != nil && overview.StickyConsistency.GhostInvalidations > 0
	hasScheduler := false
	if overview.SchedulerOutboxRuntime != nil {
		r := overview.SchedulerOutboxRuntime
		hasScheduler = r.WatermarkDrift != 0 || r.BacklogRows > 0 || r.LagSeconds > 0 || r.CheckpointFallbackStreak > 0
	}
	switch {
	case hasSticky && hasScheduler:
		return "sticky_and_scheduler_drift"
	case hasSticky:
		return "sticky_binding_drift"
	case hasScheduler:
		return "scheduler_outbox_or_checkpoint_drift"
	default:
		return "control_plane_drift"
	}
}

func shouldWarnRawUsageAmplification(filter *OpsDashboardFilter, overview *OpsDashboardOverview) bool {
	if overview == nil {
		return false
	}
	rawMode := filter != nil && filter.QueryMode == OpsQueryModeRaw
	if !rawMode && overview.DataSource != nil && overview.DataSource.Mode == string(OpsQueryModeRaw) {
		rawMode = true
	}
	if !rawMode {
		return false
	}
	usage := usageLogsGovernanceFromOverview(overview)
	if usage == nil {
		return false
	}
	return usage.MaxRowsExceeded || usage.RetentionRisk == "warn" || usage.RetentionRisk == "critical" || usage.SizeRisk == "warn" || usage.SizeRisk == "critical"
}

func usageLogsGovernanceFromOverview(overview *OpsDashboardOverview) *OpsUsageLogsGovernanceSummary {
	if overview == nil || overview.StorageGovernance == nil {
		return nil
	}
	return overview.StorageGovernance.UsageLogs
}

func blankToNA(value string) string {
	if strings.TrimSpace(value) == "" {
		return "n/a"
	}
	return value
}

func errorFamilyInvestigationSuggestion(top *OpsErrorFamilyEntry) string {
	if top == nil {
		return "Treat protocol-shape failures, provider failures, and local gateway failures separately before increasing failover or retry pressure."
	}
	phase := strings.ToLower(strings.TrimSpace(top.Phase))
	owner := strings.ToLower(strings.TrimSpace(top.Owner))
	errorType := strings.ToLower(strings.TrimSpace(top.Type))
	switch {
	case phase == "request" || owner == "client" || top.StatusCode == 400:
		return "Start with protocol and request-shape validation. Confirm requested->mapped->upstream model semantics before allowing more failover or account switching."
	case phase == "upstream" || owner == "provider" || top.StatusCode >= 500:
		return "Treat this as an upstream/provider issue first. Check account health, provider saturation, and retry/failover pressure before changing request handling."
	case phase == "local" || owner == "gateway":
		return "Inspect local gateway processing and control-plane pressure first, including Redis, scheduler lag, and usage persistence side effects."
	case strings.Contains(errorType, "auth") || strings.Contains(errorType, "token"):
		return "Separate permanent auth failures from temporary refresh-needed accounts so bad credentials stop recycling through scheduling."
	default:
		return "Treat protocol-shape failures, provider failures, and local gateway failures separately before increasing failover or retry pressure."
	}
}
