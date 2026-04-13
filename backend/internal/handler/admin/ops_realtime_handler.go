package admin

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// GetConcurrencyStats returns real-time concurrency usage aggregated by platform/group/account.
// GET /api/v1/admin/ops/concurrency
func (h *OpsHandler) GetConcurrencyStats(c *gin.Context) {
	if h.opsService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Ops service not available")
		return
	}
	if err := h.opsService.RequireMonitoringEnabled(c.Request.Context()); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	if !h.opsService.IsRealtimeMonitoringEnabled(c.Request.Context()) {
		response.Success(c, gin.H{
			"enabled":   false,
			"platform":  map[string]*service.PlatformConcurrencyInfo{},
			"group":     map[int64]*service.GroupConcurrencyInfo{},
			"account":   map[int64]*service.AccountConcurrencyInfo{},
			"timestamp": time.Now().UTC(),
		})
		return
	}

	platformFilter := strings.TrimSpace(c.Query("platform"))
	var groupID *int64
	if v := strings.TrimSpace(c.Query("group_id")); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id <= 0 {
			response.BadRequest(c, "Invalid group_id")
			return
		}
		groupID = &id
	}

	platform, group, account, collectedAt, err := h.opsService.GetConcurrencyStats(c.Request.Context(), platformFilter, groupID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	payload := gin.H{
		"enabled":  true,
		"platform": platform,
		"group":    group,
		"account":  account,
	}
	if collectedAt != nil {
		payload["timestamp"] = collectedAt.UTC()
	}
	response.Success(c, payload)
}

// GetUserConcurrencyStats returns real-time concurrency usage for all active users.
// GET /api/v1/admin/ops/user-concurrency
func (h *OpsHandler) GetUserConcurrencyStats(c *gin.Context) {
	if h.opsService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Ops service not available")
		return
	}
	if err := h.opsService.RequireMonitoringEnabled(c.Request.Context()); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	if !h.opsService.IsRealtimeMonitoringEnabled(c.Request.Context()) {
		response.Success(c, gin.H{
			"enabled":   false,
			"user":      map[int64]*service.UserConcurrencyInfo{},
			"timestamp": time.Now().UTC(),
		})
		return
	}

	users, collectedAt, err := h.opsService.GetUserConcurrencyStats(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	payload := gin.H{
		"enabled": true,
		"user":    users,
	}
	if collectedAt != nil {
		payload["timestamp"] = collectedAt.UTC()
	}
	response.Success(c, payload)
}

// GetAccountAvailability returns account availability statistics.
// GET /api/v1/admin/ops/account-availability
//
// Query params:
// - platform: optional
// - group_id: optional
func (h *OpsHandler) GetAccountAvailability(c *gin.Context) {
	if h.opsService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Ops service not available")
		return
	}
	if err := h.opsService.RequireMonitoringEnabled(c.Request.Context()); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	if !h.opsService.IsRealtimeMonitoringEnabled(c.Request.Context()) {
		response.Success(c, gin.H{
			"enabled":   false,
			"platform":  map[string]*service.PlatformAvailability{},
			"group":     map[int64]*service.GroupAvailability{},
			"account":   map[int64]*service.AccountAvailability{},
			"timestamp": time.Now().UTC(),
		})
		return
	}

	platform := strings.TrimSpace(c.Query("platform"))
	var groupID *int64
	if v := strings.TrimSpace(c.Query("group_id")); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id <= 0 {
			response.BadRequest(c, "Invalid group_id")
			return
		}
		groupID = &id
	}

	platformStats, groupStats, accountStats, collectedAt, err := h.opsService.GetAccountAvailabilityStats(c.Request.Context(), platform, groupID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	payload := gin.H{
		"enabled":  true,
		"platform": platformStats,
		"group":    groupStats,
		"account":  accountStats,
	}
	includeGlobalFailureSummaries := platform == "" && groupID == nil
	if summary := service.SummarizeTokenRefreshFailures(accountStats); summary != nil {
		payload["token_refresh_failures"] = summary
	}
	if summary := service.SummarizeAccountAuthFailures(accountStats); summary != nil {
		payload["auth_failures"] = summary
		payload["auth_failure_guidance"] = buildAuthFailureGuidance(summary)
	}
	if bundle := h.opsService.GetRealtimeSummaryBundle(c.Request.Context()); bundle != nil {
		if bundle.ResourceBudgetSummary != nil {
			payload["resource_budget_summary"] = bundle.ResourceBudgetSummary
		}
		if bundle.StorageGovernance != nil {
			payload["storage_governance"] = bundle.StorageGovernance
		}
		if bundle.StickySessionCleanup != nil {
			payload["sticky_session_cleanup"] = bundle.StickySessionCleanup
		}
		if bundle.StickyConsistency != nil {
			payload["sticky_consistency"] = bundle.StickyConsistency
		}
		if includeGlobalFailureSummaries && bundle.ErrorFamilySummary != nil {
			payload["error_family_summary"] = bundle.ErrorFamilySummary
		}
		if includeGlobalFailureSummaries && bundle.FailureSplitSummary != nil {
			payload["failure_split_summary"] = bundle.FailureSplitSummary
		}
		if bundle.UsageIntegrity != nil {
			payload["usage_integrity"] = bundle.UsageIntegrity
		}
		if bundle.DataSource != nil {
			payload["data_source"] = bundle.DataSource
		}
	}
	if collectedAt != nil {
		payload["timestamp"] = collectedAt.UTC()
	}
	response.Success(c, payload)
}

func buildAuthFailureGuidance(summary *service.AccountAuthFailureRealtimeSummary) gin.H {
	if summary == nil {
		return gin.H{}
	}
	actions := make([]string, 0, 4)
	permanentFailureAction := "Manual-review-required accounts should stay isolated until an admin reauthorizes or retires them."
	backgroundRecoveryAction := "Keep background-refresh-eligible accounts out of scheduling until the refresh succeeds or their state escalates."
	dispatchSuppressedAction := "Avoid widening failover pressure while dispatch-suppressed accounts remain elevated."
	if summary.ManualReviewRequired > 0 {
		actions = append(actions, permanentFailureAction)
	}
	if summary.BackgroundRecovery > 0 {
		actions = append(actions, backgroundRecoveryAction)
	}
	if summary.DispatchSuppressed > 0 {
		actions = append(actions, dispatchSuppressedAction)
	}
	if len(actions) == 0 {
		actions = append(actions, "No immediate auth-specific remediation guidance.")
	}
	return gin.H{
		"severity":                   "warning",
		"dispatch_suppressed":        summary.DispatchSuppressed,
		"background_recovery":        summary.BackgroundRecovery,
		"manual_review_required":     summary.ManualReviewRequired,
		"manual_review_action":       permanentFailureAction,
		"background_recovery_action": backgroundRecoveryAction,
		"dispatch_suppressed_action": dispatchSuppressedAction,
		"actions":                    actions,
	}
}

func parseOpsRealtimeWindow(v string) (time.Duration, string, bool) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "1min", "1m":
		return 1 * time.Minute, "1min", true
	case "5min", "5m":
		return 5 * time.Minute, "5min", true
	case "30min", "30m":
		return 30 * time.Minute, "30min", true
	case "1h", "60m", "60min":
		return 1 * time.Hour, "1h", true
	default:
		return 0, "", false
	}
}

// GetRealtimeTrafficSummary returns QPS/TPS current/peak/avg for the selected window.
// GET /api/v1/admin/ops/realtime-traffic
//
// Query params:
// - window: 1min|5min|30min|1h (default: 1min)
// - platform: optional
// - group_id: optional
func (h *OpsHandler) GetRealtimeTrafficSummary(c *gin.Context) {
	if h.opsService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Ops service not available")
		return
	}
	if err := h.opsService.RequireMonitoringEnabled(c.Request.Context()); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	windowDur, windowLabel, ok := parseOpsRealtimeWindow(c.Query("window"))
	if !ok {
		response.BadRequest(c, "Invalid window")
		return
	}

	platform := strings.TrimSpace(c.Query("platform"))
	var groupID *int64
	if v := strings.TrimSpace(c.Query("group_id")); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id <= 0 {
			response.BadRequest(c, "Invalid group_id")
			return
		}
		groupID = &id
	}

	endTime := time.Now().UTC()
	startTime := endTime.Add(-windowDur)

	if !h.opsService.IsRealtimeMonitoringEnabled(c.Request.Context()) {
		disabledSummary := &service.OpsRealtimeTrafficSummary{
			Window:    windowLabel,
			StartTime: startTime,
			EndTime:   endTime,
			Platform:  platform,
			GroupID:   groupID,
			QPS:       service.OpsRateSummary{},
			TPS:       service.OpsRateSummary{},
		}
		response.Success(c, gin.H{
			"enabled":   false,
			"summary":   disabledSummary,
			"timestamp": endTime,
		})
		return
	}

	filter := &service.OpsDashboardFilter{
		StartTime: startTime,
		EndTime:   endTime,
		Platform:  platform,
		GroupID:   groupID,
		QueryMode: service.OpsQueryModeRaw,
	}

	summary, err := h.opsService.GetRealtimeTrafficSummary(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if summary != nil {
		summary.Window = windowLabel
	}
	payload := gin.H{
		"enabled":   true,
		"summary":   summary,
		"timestamp": endTime,
	}
	includeGlobalFailureSummaries := platform == "" && groupID == nil
	if bundle := h.opsService.GetRealtimeSummaryBundle(c.Request.Context()); bundle != nil {
		if bundle.ResourceBudgetSummary != nil {
			payload["resource_budget_summary"] = bundle.ResourceBudgetSummary
		}
		if bundle.StorageGovernance != nil {
			payload["storage_governance"] = bundle.StorageGovernance
		}
		if bundle.StickySessionCleanup != nil {
			payload["sticky_session_cleanup"] = bundle.StickySessionCleanup
		}
		if bundle.StickyConsistency != nil {
			payload["sticky_consistency"] = bundle.StickyConsistency
		}
		if includeGlobalFailureSummaries && bundle.ErrorFamilySummary != nil {
			payload["error_family_summary"] = bundle.ErrorFamilySummary
		}
		if includeGlobalFailureSummaries && bundle.FailureSplitSummary != nil {
			payload["failure_split_summary"] = bundle.FailureSplitSummary
		}
		if bundle.UsageIntegrity != nil {
			payload["usage_integrity"] = bundle.UsageIntegrity
		}
		if bundle.DataSource != nil {
			payload["data_source"] = bundle.DataSource
		}
	}
	response.Success(c, payload)
}
