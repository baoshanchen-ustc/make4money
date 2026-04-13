package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/runtimeprobe"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"golang.org/x/sync/errgroup"
)

var opsDashboardSnapshotV2Cache = newSnapshotCache(30 * time.Second)

type opsDashboardSnapshotV2Response struct {
	GeneratedAt string `json:"generated_at"`

	Overview                  *service.OpsDashboardOverview        `json:"overview"`
	ThroughputTrend           *service.OpsThroughputTrendResponse  `json:"throughput_trend"`
	ErrorTrend                *service.OpsErrorTrendResponse       `json:"error_trend"`
	BillingCompensation       gin.H                                `json:"billing_compensation"`
	UsageLogNotPersisted      gin.H                                `json:"usage_log_not_persisted"`
	TokenRefreshSummary       gin.H                                `json:"token_refresh_summary"`
	SchedulerCheckpoint       gin.H                                `json:"scheduler_checkpoint"`
	CleanupStats              service.CleanupStats                 `json:"cleanup_stats"`
	UsageCleanupStats         service.UsageCleanupStats            `json:"usage_cleanup_stats"`
	StorageGovernance         *service.OpsStorageGovernanceSummary `json:"storage_governance,omitempty"`
	FailureSplitSummary       *service.OpsFailureSplitSummary      `json:"failure_split_summary,omitempty"`
	FailureSplitGuidance      gin.H                                `json:"failure_split_guidance,omitempty"`
	OperatorActionPlan        *service.OpsOperatorActionPlan       `json:"operator_action_plan,omitempty"`
	OperatorActionPlanSummary gin.H                                `json:"operator_action_plan_summary,omitempty"`
	DriftTrendSummary         gin.H                                `json:"drift_trend_summary,omitempty"`
	ControlPlaneDrift         gin.H                                `json:"control_plane_drift,omitempty"`
}

type opsDashboardSnapshotV2CacheKey struct {
	StartTime    string               `json:"start_time"`
	EndTime      string               `json:"end_time"`
	Platform     string               `json:"platform"`
	GroupID      *int64               `json:"group_id"`
	QueryMode    service.OpsQueryMode `json:"mode"`
	BucketSecond int                  `json:"bucket_second"`
}

// GetDashboardSnapshotV2 returns ops dashboard core snapshot in one request.
// GET /api/v1/admin/ops/dashboard/snapshot-v2
func (h *OpsHandler) GetDashboardSnapshotV2(c *gin.Context) {
	if h.opsService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Ops service not available")
		return
	}
	if err := h.opsService.RequireMonitoringEnabled(c.Request.Context()); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	startTime, endTime, err := parseOpsTimeRange(c, "1h")
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	filter := &service.OpsDashboardFilter{
		StartTime: startTime,
		EndTime:   endTime,
		Platform:  strings.TrimSpace(c.Query("platform")),
		QueryMode: parseOpsQueryMode(c),
	}
	if v := strings.TrimSpace(c.Query("group_id")); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id <= 0 {
			response.BadRequest(c, "Invalid group_id")
			return
		}
		filter.GroupID = &id
	}
	bucketSeconds := pickThroughputBucketSeconds(endTime.Sub(startTime))

	keyRaw, _ := json.Marshal(opsDashboardSnapshotV2CacheKey{
		StartTime:    startTime.UTC().Format(time.RFC3339),
		EndTime:      endTime.UTC().Format(time.RFC3339),
		Platform:     filter.Platform,
		GroupID:      filter.GroupID,
		QueryMode:    filter.QueryMode,
		BucketSecond: bucketSeconds,
	})
	cacheKey := string(keyRaw)

	if cached, ok := opsDashboardSnapshotV2Cache.Get(cacheKey); ok {
		if cached.ETag != "" {
			c.Header("ETag", cached.ETag)
			c.Header("Vary", "If-None-Match")
			if ifNoneMatchMatched(c.GetHeader("If-None-Match"), cached.ETag) {
				c.Status(http.StatusNotModified)
				return
			}
		}
		c.Header("X-Snapshot-Cache", "hit")
		response.Success(c, cached.Payload)
		return
	}

	var (
		overview *service.OpsDashboardOverview
		trend    *service.OpsThroughputTrendResponse
		errTrend *service.OpsErrorTrendResponse
	)
	g, gctx := errgroup.WithContext(c.Request.Context())
	g.Go(func() error {
		f := *filter
		result, err := h.opsService.GetDashboardOverview(gctx, &f)
		if err != nil {
			return err
		}
		overview = result
		return nil
	})
	g.Go(func() error {
		f := *filter
		result, err := h.opsService.GetThroughputTrend(gctx, &f, bucketSeconds)
		if err != nil {
			return err
		}
		trend = result
		return nil
	})
	g.Go(func() error {
		f := *filter
		result, err := h.opsService.GetErrorTrend(gctx, &f, bucketSeconds)
		if err != nil {
			return err
		}
		errTrend = result
		return nil
	})
	if err := g.Wait(); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	billingCompensationSnapshot := billingCompensationPayload()
	enrichBillingCompensationFallback(c.Request.Context(), h.opsService, filter, billingCompensationSnapshot)

	usageLogSnapshot := usageLogNotPersistedPayload()
	enrichUsageLogFallback(c.Request.Context(), h.opsService, filter, usageLogSnapshot)

	cleanupStats := service.SnapshotCleanupStats()
	usageCleanupStats := service.SnapshotUsageCleanupStats()

	tokenSummary := buildTokenRefreshSummaryPayload(c.Request.Context(), h.opsService, filter.Platform, filter.GroupID)
	schedulerCheckpoint := buildSchedulerCheckpointPayload()
	stickyMetrics := service.SnapshotStickyConsistencyMetrics()
	schedulerMetrics := service.SnapshotSchedulerOutboxRuntimeMetrics()
	redisMetrics := runtimeprobe.SnapshotDefaultRedisPoolStats()
	controlPlaneDrift := buildControlPlaneDriftPayload(
		stickyMetrics,
		schedulerMetrics,
		redisMetrics,
	)
	driftTrendPayload := buildDriftTrendPayload(schedulerMetrics)
	driftTrendSummary := buildSnapshotDriftTrendSummary(controlPlaneDrift, driftTrendPayload)
	plan := buildOperatorActionPlan(overview.FailureSplitSummary, controlPlaneDrift)
	resp := &opsDashboardSnapshotV2Response{
		GeneratedAt:               time.Now().UTC().Format(time.RFC3339),
		Overview:                  overview,
		ThroughputTrend:           trend,
		ErrorTrend:                errTrend,
		BillingCompensation:       billingCompensationSnapshot,
		UsageLogNotPersisted:      usageLogSnapshot,
		CleanupStats:              cleanupStats,
		UsageCleanupStats:         usageCleanupStats,
		StorageGovernance:         h.opsService.StorageGovernanceSummary(cleanupStats, usageCleanupStats, overview.JobHeartbeats),
		TokenRefreshSummary:       tokenSummary,
		SchedulerCheckpoint:       schedulerCheckpoint,
		FailureSplitSummary:       buildSnapshotFailureSplitSummaryPayload(overview),
		FailureSplitGuidance:      buildFailureSplitGuidancePayload(overview),
		OperatorActionPlan:        plan,
		OperatorActionPlanSummary: buildSnapshotOperatorActionPlanSummary(plan),
		DriftTrendSummary:         driftTrendSummary,
		ControlPlaneDrift:         controlPlaneDrift,
	}

	cached := opsDashboardSnapshotV2Cache.Set(cacheKey, resp)
	if cached.ETag != "" {
		c.Header("ETag", cached.ETag)
		c.Header("Vary", "If-None-Match")
	}
	c.Header("X-Snapshot-Cache", "miss")
	response.Success(c, resp)
}

func enrichBillingCompensationFallback(ctx context.Context, opsSvc *service.OpsService, filter *service.OpsDashboardFilter, payload gin.H) {
	if opsSvc == nil || payload == nil {
		return
	}
	snapshotVal, ok := payload["snapshot"]
	if !ok {
		return
	}
	var snapshot service.BillingCompensationSnapshot
	switch v := snapshotVal.(type) {
	case service.BillingCompensationSnapshot:
		snapshot = v
	case *service.BillingCompensationSnapshot:
		if v == nil {
			return
		}
		snapshot = *v
	default:
		return
	}
	if snapshot.Total > 0 || len(snapshot.Recent) > 0 {
		return
	}
	fallback, _ := payload["fallback"].(gin.H)
	if fallback == nil {
		fallback = gin.H{}
	}
	logs, summary := collectPersistedBillingLogs(ctx, opsSvc, filter)
	if len(logs) > 0 {
		fallback["persisted_logs"] = describeLogs(logs)
	}
	if summary != nil {
		if d := describeLog(summary); d != nil {
			fallback["persisted_summary_log"] = d
		}
		if hint := manualHintFromLog(summary); hint != nil {
			if existing, ok := fallback["manual_hint"].(gin.H); ok {
				fallback["manual_hint"] = mergeManualHints(existing, hint)
			} else {
				fallback["manual_hint"] = hint
			}
		}
	}
	if _, ok := fallback["manual_hint"]; !ok {
		fallback["manual_hint"] = gin.H{
			"note": "Billing compensation snapshot is empty; inspect ops.runtime.billing_compensation(.summary) logs to reconstruct candidates before manual replays.",
		}
	}
	payload["fallback"] = fallback
}

func usageLogNotPersistedPayload() gin.H {
	snapshot := service.SnapshotUsageLogNotPersistedMetrics()
	payload := gin.H{
		"total":            snapshot.Total,
		"recent_1m_total":  snapshot.Recent1mTotal,
		"recent_5m_total":  snapshot.Recent5mTotal,
		"recent_15m_total": snapshot.Recent15mTotal,
		"recent_count":     len(snapshot.Recent),
		"details":          snapshot.Recent,
		"snapshot":         snapshot,
	}
	if snapshot.Last != nil {
		payload["last_request_id"] = snapshot.Last.RequestID
		payload["last_error"] = snapshot.Last.Error
		payload["last_account_id"] = snapshot.Last.AccountID
		payload["manual_hint"] = gin.H{
			"note":       "手动核对 usage 落库是否缺失，并按 request_id 追查对应 billing 记录",
			"request_id": snapshot.Last.RequestID,
			"account_id": snapshot.Last.AccountID,
		}
	}
	if snapshot.Total == 0 && len(snapshot.Recent) == 0 {
		payload["fallback"] = gin.H{
			"reason":      "snapshot_empty",
			"recent_logs": []string{service.OpsRuntimeUsageLogComponent, service.OpsRuntimeUsageLogSummaryComponent},
			"items":       snapshot.Recent,
		}
	}
	lastRequestID := ""
	if snapshot.Last != nil {
		lastRequestID = snapshot.Last.RequestID
	}
	payload["ops_search"] = attachOpsSearchLastDetailEndpoint(opsSearchHint(
		"/api/v1/admin/ops/usage-log-not-persisted",
		"/api/v1/admin/ops/usage-log-not-persisted/:request_id",
		"Tap this ops endpoint to explore persisted usage log alerts and link them to manual replay steps.",
	), lastRequestID)
	return payload
}

func enrichUsageLogFallback(ctx context.Context, opsSvc *service.OpsService, filter *service.OpsDashboardFilter, payload gin.H) {
	if opsSvc == nil || payload == nil {
		return
	}
	snapshotVal, ok := payload["snapshot"]
	if !ok {
		return
	}
	var snapshot service.UsageLogNotPersistedMetricsSnapshot
	switch v := snapshotVal.(type) {
	case service.UsageLogNotPersistedMetricsSnapshot:
		snapshot = v
	case *service.UsageLogNotPersistedMetricsSnapshot:
		if v == nil {
			return
		}
		snapshot = *v
	default:
		return
	}
	if snapshot.Total > 0 || len(snapshot.Recent) > 0 {
		return
	}
	fallback, _ := payload["fallback"].(gin.H)
	if fallback == nil {
		fallback = gin.H{}
	}
	logs, summary := collectPersistedUsageLogs(ctx, opsSvc, filter)
	if len(logs) > 0 {
		fallback["persisted_logs"] = describeLogs(logs)
	}
	if summary != nil {
		if d := describeLog(summary); d != nil {
			fallback["persisted_summary_log"] = d
		}
		if hint := usageManualHintFromLog(summary); hint != nil {
			if existing, ok := fallback["manual_hint"].(gin.H); ok {
				fallback["manual_hint"] = mergeManualHints(existing, hint)
			} else {
				fallback["manual_hint"] = hint
			}
		}
	}
	if _, ok := fallback["manual_hint"]; !ok {
		fallback["manual_hint"] = gin.H{
			"note": "Usage log snapshot is empty; inspect ops.runtime.usage_log(.summary) logs to reconstruct missing usage rows.",
		}
	}
	payload["fallback"] = fallback
}

func collectPersistedUsageLogs(ctx context.Context, opsSvc *service.OpsService, dashboardFilter *service.OpsDashboardFilter) ([]*service.OpsSystemLog, *service.OpsSystemLog) {
	components := []string{
		service.OpsRuntimeUsageLogSummaryComponent,
		service.OpsRuntimeUsageLogComponent,
	}
	logs := make([]*service.OpsSystemLog, 0, len(components)*3)
	var summary *service.OpsSystemLog
	for _, component := range components {
		entries := fetchRuntimeLogsForComponent(ctx, opsSvc, dashboardFilter, component, 3)
		if len(entries) == 0 {
			continue
		}
		if component == service.OpsRuntimeUsageLogSummaryComponent && summary == nil {
			summary = entries[0]
		}
		logs = append(logs, entries...)
	}
	return logs, summary
}

func collectPersistedBillingLogs(ctx context.Context, opsSvc *service.OpsService, dashboardFilter *service.OpsDashboardFilter) ([]*service.OpsSystemLog, *service.OpsSystemLog) {
	components := []string{
		service.OpsRuntimeBillingCompensationSummaryComponent,
		service.OpsRuntimeBillingCompensationComponent,
	}
	logs := make([]*service.OpsSystemLog, 0, len(components)*3)
	var summary *service.OpsSystemLog
	for _, component := range components {
		entries := fetchRuntimeLogsForComponent(ctx, opsSvc, dashboardFilter, component, 3)
		if len(entries) == 0 {
			continue
		}
		if component == service.OpsRuntimeBillingCompensationSummaryComponent && summary == nil {
			summary = entries[0]
		}
		logs = append(logs, entries...)
	}
	return logs, summary
}

func fetchRuntimeLogsForComponent(ctx context.Context, opsSvc *service.OpsService, dashboardFilter *service.OpsDashboardFilter, component string, limit int) []*service.OpsSystemLog {
	if opsSvc == nil {
		return nil
	}
	filter := buildRuntimeLogFilter(dashboardFilter, component, limit)
	result, err := opsSvc.ListSystemLogs(ctx, filter)
	if err != nil || result == nil || len(result.Logs) == 0 {
		return nil
	}
	return result.Logs
}

func buildRuntimeLogFilter(base *service.OpsDashboardFilter, component string, pageSize int) *service.OpsSystemLogFilter {
	if pageSize <= 0 {
		pageSize = 3
	}
	filter := &service.OpsSystemLogFilter{
		Component: component,
		Level:     "warn",
		Page:      1,
		PageSize:  pageSize,
	}
	if base == nil {
		return filter
	}
	if base.Platform != "" {
		filter.Platform = base.Platform
	}
	if !base.StartTime.IsZero() {
		filter.StartTime = copyTimePtr(base.StartTime)
	}
	if !base.EndTime.IsZero() {
		filter.EndTime = copyTimePtr(base.EndTime)
	}
	return filter
}

func copyTimePtr(value time.Time) *time.Time {
	tmp := value
	return &tmp
}

func describeLogs(logs []*service.OpsSystemLog) []gin.H {
	result := make([]gin.H, 0, len(logs))
	for _, log := range logs {
		if entry := describeLog(log); entry != nil {
			result = append(result, entry)
		}
	}
	return result
}

func describeLog(log *service.OpsSystemLog) gin.H {
	if log == nil {
		return nil
	}
	entry := gin.H{
		"id":         log.ID,
		"component":  log.Component,
		"message":    log.Message,
		"created_at": log.CreatedAt.UTC().Format(time.RFC3339),
	}
	if log.RequestID != "" {
		entry["request_id"] = log.RequestID
	}
	if log.ClientRequestID != "" {
		entry["client_request_id"] = log.ClientRequestID
	}
	return entry
}

func manualHintFromLog(log *service.OpsSystemLog) gin.H {
	if log == nil || log.Extra == nil {
		return nil
	}
	last, ok := log.Extra["last"].(map[string]any)
	if !ok {
		return nil
	}
	hint := gin.H{
		"note":          "Persisted billing compensation candidate detected; use the request_id below to replay and verify costs.",
		"log_component": log.Component,
		"log_time":      log.CreatedAt.UTC().Format(time.RFC3339),
	}
	if requestID, ok := last["request_id"].(string); ok && requestID != "" {
		hint["request_id"] = requestID
	}
	if accountID, ok := coerceInt64(last["account_id"]); ok {
		hint["account_id"] = accountID
	}
	if errText, ok := last["error"].(string); ok && errText != "" {
		hint["error"] = errText
	}
	return hint
}

func usageManualHintFromLog(log *service.OpsSystemLog) gin.H {
	if log == nil {
		return nil
	}
	var (
		requestID string
		errText   string
		accountID int64
		hasAcct   bool
	)
	if log.Extra != nil {
		if value, ok := log.Extra["request_id"].(string); ok && strings.TrimSpace(value) != "" {
			requestID = strings.TrimSpace(value)
		}
		if value, ok := log.Extra["error"].(string); ok && strings.TrimSpace(value) != "" {
			errText = strings.TrimSpace(value)
		}
		if value, ok := coerceInt64(log.Extra["account_id"]); ok {
			accountID = value
			hasAcct = true
		}
		if last, ok := log.Extra["last"].(map[string]any); ok {
			if requestID == "" {
				if value, ok := last["request_id"].(string); ok && strings.TrimSpace(value) != "" {
					requestID = strings.TrimSpace(value)
				}
			}
			if errText == "" {
				if value, ok := last["error"].(string); ok && strings.TrimSpace(value) != "" {
					errText = strings.TrimSpace(value)
				}
			}
			if !hasAcct {
				if value, ok := coerceInt64(last["account_id"]); ok {
					accountID = value
					hasAcct = true
				}
			}
		}
	}
	if requestID == "" && errText == "" && !hasAcct {
		return nil
	}
	hint := gin.H{
		"note":          "Persisted usage-log failure detected; use the request_id below to backfill usage and reconcile billing.",
		"log_component": log.Component,
		"log_time":      log.CreatedAt.UTC().Format(time.RFC3339),
	}
	if requestID != "" {
		hint["request_id"] = requestID
	}
	if errText != "" {
		hint["error"] = errText
	}
	if hasAcct {
		hint["account_id"] = accountID
	}
	return hint
}

func mergeManualHints(existing gin.H, extra gin.H) gin.H {
	if extra == nil {
		return existing
	}
	if existing == nil {
		return extra
	}
	for key, value := range extra {
		if _, ok := existing[key]; !ok {
			existing[key] = value
		}
	}
	return existing
}

func coerceInt64(value any) (int64, bool) {
	switch v := value.(type) {
	case int:
		return int64(v), true
	case int64:
		return v, true
	case float64:
		return int64(v), true
	case string:
		if v == "" {
			return 0, false
		}
		if parsed, err := strconv.ParseInt(v, 10, 64); err == nil {
			return parsed, true
		}
	}
	return 0, false
}

func buildTokenRefreshSummaryPayload(ctx context.Context, opsSvc *service.OpsService, platform string, groupID *int64) gin.H {
	if opsSvc == nil {
		return gin.H{
			"total":    0,
			"platform": gin.H{},
			"group":    gin.H{},
			"failures": []gin.H{},
		}
	}
	summary := opsSvc.TokenRefreshFailureSummary(ctx, platform, groupID)
	if summary == nil {
		return gin.H{
			"total":    0,
			"platform": gin.H{},
			"group":    gin.H{},
			"failures": []gin.H{},
		}
	}
	return gin.H{
		"total":    summary.Total,
		"platform": summary.Platform,
		"group":    summary.Group,
		"failures": summary.Failures,
	}
}

func buildSnapshotFailureSplitSummaryPayload(overview *service.OpsDashboardOverview) *service.OpsFailureSplitSummary {
	if overview == nil || overview.FailureSplitSummary == nil {
		return nil
	}
	return overview.FailureSplitSummary
}

func buildFailureSplitGuidancePayload(overview *service.OpsDashboardOverview) gin.H {
	if overview == nil || overview.FailureSplitSummary == nil {
		return nil
	}
	summary := overview.FailureSplitSummary
	return gin.H{
		"totals": gin.H{
			"protocol_or_request_shape": summary.ProtocolOrRequestShape,
			"account_or_auth":           summary.AccountOrAuth,
			"provider_or_upstream":      summary.ProviderOrUpstream,
			"local_processing":          summary.LocalProcessing,
		},
		"likely_primary":  summary.LikelyPrimary,
		"suggestion":      summary.Suggestion,
		"operator_action": summary.OperatorAction,
		"investigation_order": []string{
			"Open /api/v1/admin/ops/requests?kind=error&sort=duration_desc to identify the leading request_id and error duration.",
			"Inspect /api/v1/admin/ops/request-errors/:id for the client-visible failure and correlation identifiers.",
			"Inspect /api/v1/admin/ops/request-errors/:id/upstream-errors?include_detail=true to trace requested->mapped->upstream transitions.",
		},
		"focus_hint":       failureSplitFocusHint(summary.LikelyPrimary),
		"correlation_hint": "Correlate request_id, client_request_id, and upstream identifiers before adjusting dispatch or increasing failover pressure.",
	}
}

func failureSplitFocusHint(primary string) string {
	switch strings.TrimSpace(primary) {
	case "protocol_or_request_shape":
		return "Confirm requested->mapped->upstream transitions and request schema before touching account health."
	case "account_or_auth":
		return "Isolate the credential, keep it out of dispatch, and focus on refresh/manual reauth workflows."
	case "provider_or_upstream":
		return "Inspect account health, saturation, and upstream limits before changing request validation."
	case "local_processing":
		return "Inspect sticky/session drift, scheduler backlog, and Redis signals before adjusting failover."
	default:
		return "Use the failure split totals to choose whether protocol, account, upstream, or control-plane investigation should lead."
	}
}

func buildSchedulerCheckpointPayload() gin.H {
	summary := service.SchedulerCheckpointSummary()
	if summary == nil {
		return gin.H{}
	}
	return gin.H{
		"last_checkpoint_watermark":      summary.LastCheckpointWatermark,
		"checkpoint_fallback_total":      summary.CheckpointFallbackTotal,
		"checkpoint_read_failure_total":  summary.CheckpointReadFailures,
		"checkpoint_write_failure_total": summary.CheckpointWriteFailures,
	}
}

func buildSnapshotOperatorActionPlanSummary(plan *service.OpsOperatorActionPlan) gin.H {
	if plan == nil {
		return nil
	}
	summary := gin.H{}
	if len(plan.FailureSplitActions) > 0 {
		summary["primary_action"] = plan.FailureSplitActions[0]
		summary["failure_split_action_count"] = len(plan.FailureSplitActions)
	}
	if len(plan.ProtocolDiagnostics) > 0 {
		summary["protocol_diagnostic"] = plan.ProtocolDiagnostics[0]
	}
	if len(plan.ControlPlaneDrift) > 0 {
		summary["control_plane_hint"] = plan.ControlPlaneDrift[0]
	}
	return summary
}

func buildSnapshotDriftTrendSummary(controlPlaneDrift, trend gin.H) gin.H {
	if len(controlPlaneDrift) == 0 && len(trend) == 0 {
		return nil
	}
	summary := gin.H{}
	if severity, ok := controlPlaneDrift["severity"]; ok {
		summary["severity"] = severity
	}
	if cause, ok := controlPlaneDrift["likely_root_cause"]; ok {
		summary["likely_root_cause"] = cause
	}
	if trend != nil {
		if status, ok := trend["status"]; ok && status != "" {
			summary["trend_status"] = status
		}
		if detail, ok := trend["detail"]; ok && detail != "" {
			summary["trend_detail"] = detail
		}
	}
	return summary
}
