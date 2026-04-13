package admin

import (
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// ListUsageLogNotPersisted provides persisted usage log alerts retrieved from system logs.
// GET /api/v1/admin/ops/usage-log-not-persisted
func (h *OpsHandler) ListUsageLogNotPersisted(c *gin.Context) {
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

	filter := &service.OpsSystemLogFilter{
		Page:       page,
		PageSize:   pageSize,
		StartTime:  &startTime,
		EndTime:    &endTime,
		Level:      "warn",
		Component:  service.OpsRuntimeUsageLogComponent,
		ExactTotal: true,
	}

	apiKeyID, groupID, invalidField, parseErr := parseOpsAPIKeyAndGroupID(c.Query("api_key_id"), c.Query("group_id"))
	if parseErr != nil {
		response.BadRequest(c, "Invalid "+invalidField)
		return
	}
	filter.ResolvedRequestID = strings.TrimSpace(c.Query("request_id"))
	filter.ExtraAPIKeyID = apiKeyID
	filter.ExtraGroupID = groupID

	logs, err := h.opsService.ListSystemLogs(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	items := make([]gin.H, 0, len(logs.Logs))
	for _, log := range logs.Logs {
		if entry := describeUsageLogEntry(log); entry != nil {
			items = append(items, entry)
		}
	}

	response.Success(c, gin.H{
		"component": filter.Component,
		"page":      logs.Page,
		"page_size": logs.PageSize,
		"total":     logs.Total,
		"raw_total": logs.Total,
		"filters": gin.H{
			"request_id": strings.TrimSpace(c.Query("request_id")),
			"api_key_id": apiKeyID,
			"group_id":   groupID,
			"start_time": startTime.UTC().Format(time.RFC3339),
			"end_time":   endTime.UTC().Format(time.RFC3339),
		},
		"items": items,
	})
}

func describeUsageLogEntry(log *service.OpsSystemLog) gin.H {
	entry := describeLog(log)
	if entry == nil {
		return nil
	}
	entry["kind"] = "usage_alert"
	for _, key := range []string{
		"log_key",
		"api_key_id",
		"group_id",
		"subscription_id",
		"model",
		"requested_model",
		"error",
		"total_cost",
		"actual_cost",
	} {
		if value, ok := extractUsageLogField(log, key); ok {
			entry[key] = value
		}
	}
	if hint := loggedUsageManualHint(log); hint != nil {
		entry["manual_hint"] = hint
	}
	return entry
}

func extractUsageLogField(log *service.OpsSystemLog, key string) (any, bool) {
	if log == nil || key == "" {
		return nil, false
	}
	if log.Extra == nil {
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

func loggedUsageManualHint(log *service.OpsSystemLog) gin.H {
	if log == nil || log.Extra == nil {
		return nil
	}
	lastRaw, ok := log.Extra["last"]
	if !ok {
		return nil
	}
	last, ok := lastRaw.(map[string]any)
	if !ok {
		return nil
	}
	hint := gin.H{
		"note":          "Persisted usage log alert detected; replay the request_id below to verify billing.",
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

// GetUsageLogNotPersistedDetail returns a single persisted usage log alert by request_id.
// GET /api/v1/admin/ops/usage-log-not-persisted/:request_id
func (h *OpsHandler) GetUsageLogNotPersistedDetail(c *gin.Context) {
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

	filter := &service.OpsSystemLogFilter{
		Page:              1,
		PageSize:          25,
		StartTime:         &startTime,
		EndTime:           &endTime,
		Level:             "warn",
		Component:         service.OpsRuntimeUsageLogComponent,
		ResolvedRequestID: requestID,
		ExactTotal:        true,
	}
	apiKeyID, groupID, invalidField, parseErr := parseOpsAPIKeyAndGroupID(c.Query("api_key_id"), c.Query("group_id"))
	if parseErr != nil {
		response.BadRequest(c, "Invalid "+invalidField)
		return
	}

	filter.ExtraAPIKeyID = apiKeyID
	filter.ExtraGroupID = groupID

	logs, err := h.opsService.ListSystemLogs(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	if len(logs.Logs) == 0 {
		response.NotFound(c, "usage log not persisted entry not found")
		return
	}

	if entry := describeUsageLogEntry(logs.Logs[0]); entry != nil {
		response.Success(c, gin.H{
			"entry": entry,
			"filters": gin.H{
				"request_id": requestID,
				"api_key_id": apiKeyID,
				"group_id":   groupID,
				"start_time": startTime.UTC().Format(time.RFC3339),
				"end_time":   endTime.UTC().Format(time.RFC3339),
			},
			"count": logs.Total,
		})
		return
	}
	response.NotFound(c, "usage log not persisted entry not found")
}
