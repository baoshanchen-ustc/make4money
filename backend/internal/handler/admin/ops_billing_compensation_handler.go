package admin

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// ListBillingCompensation returns persisted billing compensation candidates reconstructed from system logs.
// GET /api/v1/admin/ops/billing-compensation
func (h *OpsHandler) ListBillingCompensation(c *gin.Context) {
	if h.opsService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Ops service not available")
		return
	}
	if err := h.opsService.RequireMonitoringEnabled(c.Request.Context()); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	page, pageSize := response.ParsePagination(c)
	if pageSize > 100 {
		pageSize = 100
	}
	startTime, endTime, err := parseOpsTimeRange(c, "1h")
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	apiKeyID, groupID, invalidField, parseErr := parseOpsAPIKeyAndGroupID(c.Query("api_key_id"), c.Query("group_id"))
	if parseErr != nil {
		response.BadRequest(c, "Invalid "+invalidField)
		return
	}

	filter := &service.OpsDashboardFilter{
		StartTime: startTime,
		EndTime:   endTime,
		Platform:  strings.TrimSpace(c.Query("platform")),
	}
	perComponentLimit := page * pageSize
	if perComponentLimit < 50 {
		perComponentLimit = 50
	}
	if perComponentLimit > 200 {
		perComponentLimit = 200
	}

	logs := collectPersistedBillingLogsWithLimit(c.Request.Context(), h.opsService, filter, perComponentLimit)
	requestID := strings.TrimSpace(c.Query("request_id"))

	items := make([]gin.H, 0, len(logs))
	for _, log := range logs {
		if !matchesBillingCompensationFilters(log, requestID, apiKeyID, groupID) {
			continue
		}
		if entry := buildBillingCompensationLogEntry(log); entry != nil {
			items = append(items, entry)
		}
	}

	sort.SliceStable(items, func(i, j int) bool {
		return strings.Compare(asString(items[i]["created_at"]), asString(items[j]["created_at"])) > 0
	})

	total := len(items)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	response.Paginated(c, items[start:end], int64(total), page, pageSize)
}

func collectPersistedBillingLogsWithLimit(ctx context.Context, opsSvc *service.OpsService, filter *service.OpsDashboardFilter, perComponentLimit int) []*service.OpsSystemLog {
	if perComponentLimit <= 0 {
		perComponentLimit = 50
	}
	if perComponentLimit > 200 {
		perComponentLimit = 200
	}
	if opsSvc == nil {
		return nil
	}
	components := []string{
		service.OpsRuntimeBillingCompensationComponent,
		service.OpsRuntimeBillingCompensationSummaryComponent,
	}
	result := make([]*service.OpsSystemLog, 0, len(components)*perComponentLimit)
	for _, component := range components {
		entries := fetchRuntimeLogsForComponent(ctx, opsSvc, filter, component, perComponentLimit)
		if len(entries) == 0 {
			continue
		}
		result = append(result, entries...)
	}
	return result
}

func matchesBillingCompensationFilters(log *service.OpsSystemLog, requestID string, apiKeyID, groupID *int64) bool {
	if log == nil {
		return false
	}
	if requestID != "" {
		resolvedRequestID := resolveBillingCompensationRequestID(log)
		if !strings.EqualFold(resolvedRequestID, requestID) {
			return false
		}
	}
	if apiKeyID != nil {
		value, ok := extractBillingCompensationInt64(log, "api_key_id")
		if !ok || value != *apiKeyID {
			return false
		}
	}
	if groupID != nil {
		value, ok := extractBillingCompensationInt64(log, "group_id")
		if !ok || value != *groupID {
			return false
		}
	}
	return true
}

func buildBillingCompensationLogEntry(log *service.OpsSystemLog) gin.H {
	entry := describeLog(log)
	if entry == nil {
		return nil
	}
	entry["kind"] = billingCompensationLogKind(log)
	if log.UserID != nil {
		entry["user_id"] = *log.UserID
	}
	if log.AccountID != nil {
		entry["account_id"] = *log.AccountID
	}
	for _, key := range []string{
		"log_key",
		"api_key_id",
		"group_id",
		"subscription_id",
		"model",
		"requested_model",
		"error",
		"delta",
		"total",
		"recent_1m_total",
		"recent_5m_total",
		"recent_15m_total",
		"total_cost",
		"actual_cost",
	} {
		if value, ok := extractBillingCompensationField(log, key); ok {
			entry[key] = value
		}
	}
	if hint := manualHintFromLog(log); hint != nil {
		entry["manual_hint"] = hint
	}
	return entry
}

func billingCompensationLogKind(log *service.OpsSystemLog) string {
	if log == nil {
		return ""
	}
	if log.Component == service.OpsRuntimeBillingCompensationSummaryComponent {
		return "summary"
	}
	return "candidate"
}

func resolveBillingCompensationRequestID(log *service.OpsSystemLog) string {
	if log == nil {
		return ""
	}
	if requestID := strings.TrimSpace(log.RequestID); requestID != "" {
		return requestID
	}
	if value, ok := extractBillingCompensationField(log, "request_id"); ok {
		if requestID := strings.TrimSpace(asString(value)); requestID != "" {
			return requestID
		}
	}
	return ""
}

func extractBillingCompensationField(log *service.OpsSystemLog, key string) (any, bool) {
	if log == nil || key == "" || log.Extra == nil {
		return nil, false
	}
	if value, ok := log.Extra[key]; ok {
		return value, true
	}
	if lastRaw, ok := log.Extra["last"]; ok {
		if last, ok := lastRaw.(map[string]any); ok {
			value, ok := last[key]
			return value, ok
		}
	}
	return nil, false
}

func extractBillingCompensationInt64(log *service.OpsSystemLog, key string) (int64, bool) {
	value, ok := extractBillingCompensationField(log, key)
	if !ok {
		return 0, false
	}
	return coerceInt64(value)
}

// GetBillingCompensationDetail returns persisted entries for the supplied request_id.
// GET /api/v1/admin/ops/billing-compensation/:request_id
func (h *OpsHandler) GetBillingCompensationDetail(c *gin.Context) {
	if h.opsService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Ops service not available")
		return
	}
	if err := h.opsService.RequireMonitoringEnabled(c.Request.Context()); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	requestID := strings.TrimSpace(c.Param("request_id"))
	if requestID == "" {
		response.BadRequest(c, "request_id is required")
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
	}
	apiKeyID, groupID, invalidField, parseErr := parseOpsAPIKeyAndGroupID(c.Query("api_key_id"), c.Query("group_id"))
	if parseErr != nil {
		response.BadRequest(c, "Invalid "+invalidField)
		return
	}

	logs := collectPersistedBillingLogsWithLimit(c.Request.Context(), h.opsService, filter, 50)

	entries := make([]gin.H, 0, len(logs))
	for _, log := range logs {
		if !matchesBillingCompensationFilters(log, requestID, apiKeyID, groupID) {
			continue
		}
		if entry := buildBillingCompensationLogEntry(log); entry != nil {
			entries = append(entries, entry)
		}
	}

	if len(entries) == 0 {
		response.NotFound(c, "billing compensation entry not found")
		return
	}

	response.Success(c, gin.H{
		"request_id": requestID,
		"count":      len(entries),
		"items":      entries,
		"filters": gin.H{
			"request_id": requestID,
			"api_key_id": apiKeyID,
			"group_id":   groupID,
			"start_time": startTime.UTC().Format(time.RFC3339),
			"end_time":   endTime.UTC().Format(time.RFC3339),
		},
	})
}

func asString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	default:
		return ""
	}
}
