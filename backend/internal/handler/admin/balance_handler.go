package admin

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// BalanceHandler handles admin balance management
type BalanceHandler struct {
	usageService *service.UsageService
}

// NewBalanceHandler creates a new admin balance handler
func NewBalanceHandler(usageService *service.UsageService) *BalanceHandler {
	return &BalanceHandler{
		usageService: usageService,
	}
}

// GetStats handles GET /api/v1/admin/balance/stats
func (h *BalanceHandler) GetStats(c *gin.Context) {
	groupIDStr := c.Query("group_id")
	if groupIDStr == "" {
		response.Error(c, http.StatusBadRequest, "group_id is required")
		return
	}
	groupID, err := strconv.ParseInt(groupIDStr, 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid group_id")
		return
	}

	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")
	if startDateStr == "" || endDateStr == "" {
		response.Error(c, http.StatusBadRequest, "start_date and end_date are required")
		return
	}

	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid start_date format, expected YYYY-MM-DD")
		return
	}
	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid end_date format, expected YYYY-MM-DD")
		return
	}
	// end_date 需要加一天，以包含当天的数据
	endDate = endDate.AddDate(0, 0, 1)

	page, pageSize := response.ParsePagination(c)
	sortBy := c.DefaultQuery("sort_by", "total_cost")
	sortOrder := c.DefaultQuery("sort_order", "desc")
	search := c.Query("search")

	params := &usagestats.BalanceGroupUserStatsParams{
		GroupID:   groupID,
		StartDate: &startDate,
		EndDate:   &endDate,
		Page:      page,
		PageSize:  pageSize,
		SortBy:    sortBy,
		SortOrder: sortOrder,
		Search:    search,
	}

	result, err := h.usageService.GetBalanceGroupUserStats(c.Request.Context(), params)
	if err != nil {
		// Service validation errors contain safe messages; internal errors should not be exposed
		errMsg := err.Error()
		if strings.Contains(errMsg, "is required") ||
			strings.Contains(errMsg, "must be after") ||
			strings.Contains(errMsg, "must not exceed") {
			response.Error(c, http.StatusBadRequest, errMsg)
		} else {
			slog.Error("failed to get balance group user stats", "error", err)
			response.Error(c, http.StatusInternalServerError, "failed to get balance stats")
		}
		return
	}

	response.Success(c, result)
}
