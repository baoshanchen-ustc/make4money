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
	requestID := strings.TrimSpace(c.Query("request_id"))
	items, total := listBillingCompensationEntries(c.Request.Context(), h.opsService, filter, requestID, apiKeyID, groupID, page*pageSize)
	sort.SliceStable(items, func(i, j int) bool {
		return strings.Compare(asString(items[i]["created_at"]), asString(items[j]["created_at"])) > 0
	})

	start := (page - 1) * pageSize
	if start > len(items) {
		start = len(items)
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}

	response.Paginated(c, items[start:end], total, page, pageSize)
}

func listBillingCompensationEntries(
	ctx context.Context,
	opsSvc *service.OpsService,
	filter *service.OpsDashboardFilter,
	requestID string,
	apiKeyID, groupID *int64,
	limit int,
) ([]gin.H, int64) {
	logs, total := collectPersistedBillingCompensationLogs(ctx, opsSvc, filter, requestID, apiKeyID, groupID, limit)
	items := make([]gin.H, 0, len(logs))
	for _, log := range logs {
		if entry := buildBillingCompensationLogEntry(log); entry != nil {
			items = append(items, entry)
		}
	}
	return items, total
}

func collectPersistedBillingCompensationLogs(
	ctx context.Context,
	opsSvc *service.OpsService,
	filter *service.OpsDashboardFilter,
	requestID string,
	apiKeyID, groupID *int64,
	limit int,
) ([]*service.OpsSystemLog, int64) {
	if opsSvc == nil {
		return nil, 0
	}
	components := []string{
		service.OpsRuntimeBillingCompensationComponent,
		service.OpsRuntimeBillingCompensationSummaryComponent,
	}
	result := make([]*service.OpsSystemLog, 0, len(components)*50)
	var total int64
	for _, component := range components {
		entries, componentTotal := fetchBillingCompensationLogsForComponent(ctx, opsSvc, filter, component, requestID, apiKeyID, groupID, limit)
		total += componentTotal
		if len(entries) == 0 {
			continue
		}
		result = append(result, entries...)
	}
	return result, total
}

func fetchBillingCompensationLogsForComponent(
	ctx context.Context,
	opsSvc *service.OpsService,
	base *service.OpsDashboardFilter,
	component string,
	requestID string,
	apiKeyID, groupID *int64,
	limit int,
) ([]*service.OpsSystemLog, int64) {
	if opsSvc == nil {
		return nil, 0
	}
	pageSize := billingCompensationPageSize(limit)

	collected := make([]*service.OpsSystemLog, 0, pageSize)
	var total int64
	for page := 1; ; page++ {
		filter := buildRuntimeLogFilter(base, component, pageSize)
		applyBillingCompensationLogFilters(filter, requestID, apiKeyID, groupID)
		filter.ExactTotal = true
		filter.Page = page
		result, err := opsSvc.ListSystemLogs(ctx, filter)
		if err != nil || result == nil || len(result.Logs) == 0 {
			break
		}
		if page == 1 {
			total = int64(result.Total)
		}
		collected = append(collected, result.Logs...)
		if shouldStopBillingLogScan(result, len(collected), limit) {
			break
		}
	}
	return collected, total
}

func billingCompensationPageSize(limit int) int {
	switch {
	case limit <= 0:
		return 200
	case limit < 50:
		return 50
	case limit > 200:
		return 200
	default:
		return limit
	}
}

func applyBillingCompensationLogFilters(filter *service.OpsSystemLogFilter, requestID string, apiKeyID, groupID *int64) {
	if filter == nil {
		return
	}
	filter.ResolvedRequestID = strings.TrimSpace(requestID)
	filter.ExtraAPIKeyID = apiKeyID
	filter.ExtraGroupID = groupID
}

func shouldStopBillingLogScan(page *service.OpsSystemLogList, collected int, limit int) bool {
	if page == nil {
		return true
	}
	if page.PageSize <= 0 {
		page.PageSize = len(page.Logs)
	}
	if page.Page <= 0 {
		page.Page = 1
	}
	if limit > 0 && collected >= limit {
		return true
	}
	if page.Total > 0 && page.Page*page.PageSize >= page.Total {
		return true
	}
	return false
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

	entries, _ := listBillingCompensationEntries(c.Request.Context(), h.opsService, filter, requestID, apiKeyID, groupID, 0)

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
