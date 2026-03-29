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

// CopilotAnalyticsHandler handles Copilot cost analytics admin endpoints.
type CopilotAnalyticsHandler struct {
	analyticsSvc *service.CopilotAnalyticsService
	quotaCache   *service.CopilotQuotaCacheService
	adminSvc     service.AdminService
	alertRepo    service.CopilotBudgetAlertRepository
}

// NewCopilotAnalyticsHandler constructs the analytics handler.
func NewCopilotAnalyticsHandler(
	analyticsSvc *service.CopilotAnalyticsService,
	quotaCache *service.CopilotQuotaCacheService,
	adminSvc service.AdminService,
	alertRepo service.CopilotBudgetAlertRepository,
) *CopilotAnalyticsHandler {
	return &CopilotAnalyticsHandler{
		analyticsSvc: analyticsSvc,
		quotaCache:   quotaCache,
		adminSvc:     adminSvc,
		alertRepo:    alertRepo,
	}
}

// ─────────────────────────────────────────────
// 用户维度 API
// ─────────────────────────────────────────────

// GetUserStats handles GET /api/v1/admin/copilot/users/stats
// Query params: date (YYYY-MM-DD, default today), user_id (optional)
func (h *CopilotAnalyticsHandler) GetUserStats(c *gin.Context) {
	date := strings.TrimSpace(c.Query("date"))
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		response.BadRequest(c, "invalid date format, expected YYYY-MM-DD")
		return
	}

	var userID int64
	if raw := strings.TrimSpace(c.Query("user_id")); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			response.BadRequest(c, "invalid user_id")
			return
		}
		userID = id
	}

	result, err := h.analyticsSvc.GetUserStats(c.Request.Context(), date, userID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, result)
}

// GetUserTimeline handles GET /api/v1/admin/copilot/users/:id/timeline
// Query params: date (YYYY-MM-DD, default today)
func (h *CopilotAnalyticsHandler) GetUserTimeline(c *gin.Context) {
	userID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	date := strings.TrimSpace(c.Query("date"))
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		response.BadRequest(c, "invalid date format, expected YYYY-MM-DD")
		return
	}

	result, err := h.analyticsSvc.GetUserTimeline(c.Request.Context(), userID, date)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, result)
}

// GetUserRequests handles GET /api/v1/admin/copilot/users/:id/requests
// Query params: date, page, page_size
func (h *CopilotAnalyticsHandler) GetUserRequests(c *gin.Context) {
	userID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	date := strings.TrimSpace(c.Query("date"))
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		response.BadRequest(c, "invalid date format, expected YYYY-MM-DD")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	result, err := h.analyticsSvc.GetUserRequests(c.Request.Context(), userID, date, page, pageSize)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, result)
}

// ─────────────────────────────────────────────
// 账户维度 API
// ─────────────────────────────────────────────

// GetAccountsDailyStats handles GET /api/v1/admin/copilot/accounts/daily-stats
// Query params: days (default 30, max 90)
func (h *CopilotAnalyticsHandler) GetAccountsDailyStats(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	if days < 1 || days > 90 {
		days = 30
	}

	result, err := h.analyticsSvc.GetAccountsDailyStats(c.Request.Context(), days)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, result)
}

// GetAccountsOverview handles GET /api/v1/admin/copilot/accounts/overview
func (h *CopilotAnalyticsHandler) GetAccountsOverview(c *gin.Context) {
	result, err := h.analyticsSvc.GetAccountsOverview(c.Request.Context())
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, result)
}

// GetAccountQuotaTrend handles GET /api/v1/admin/copilot/accounts/:id/quota-trend
// Query params: days (default 30)
func (h *CopilotAnalyticsHandler) GetAccountQuotaTrend(c *gin.Context) {
	accountID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	if days < 1 || days > 365 {
		days = 30
	}

	snapshots, err := h.analyticsSvc.GetAccountQuotaTrend(c.Request.Context(), accountID, days)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, gin.H{
		"account_id": accountID,
		"trend":      snapshots,
	})
}

// GetAccountHourlyStats handles GET /api/v1/admin/copilot/accounts/:id/hourly-stats
// Query params: date (YYYY-MM-DD, default today)
func (h *CopilotAnalyticsHandler) GetAccountHourlyStats(c *gin.Context) {
	accountID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	date := strings.TrimSpace(c.Query("date"))
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		response.BadRequest(c, "invalid date format, expected YYYY-MM-DD")
		return
	}

	hourly, err := h.analyticsSvc.GetAccountHourlyStats(c.Request.Context(), accountID, date)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, gin.H{
		"account_id": accountID,
		"date":       date,
		"hourly":     hourly,
	})
}

// QuotaRefresh handles POST /api/v1/admin/copilot/accounts/:id/quota-refresh
// Force-refreshes the quota cache for a single account and returns the latest data.
func (h *CopilotAnalyticsHandler) QuotaRefresh(c *gin.Context) {
	accountID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	ctx := c.Request.Context()
	account, err := h.adminSvc.GetAccount(ctx, accountID)
	if err != nil || account == nil {
		response.Error(c, http.StatusNotFound, "account not found")
		return
	}

	entry, err := h.quotaCache.ForceRefresh(ctx, account)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, entry)
}

// UpsertBudgetAlert handles PUT /api/v1/admin/copilot/accounts/:id/budget
func (h *CopilotAnalyticsHandler) UpsertBudgetAlert(c *gin.Context) {
	accountID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	var body struct {
		MonthlyBudget  float64 `json:"monthly_budget"`
		AlertThreshold int     `json:"alert_threshold"`
		Enabled        bool    `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "invalid request body: "+err.Error())
		return
	}
	if body.AlertThreshold < 0 || body.AlertThreshold > 100 {
		response.BadRequest(c, "alert_threshold must be between 0 and 100")
		return
	}
	if body.MonthlyBudget < 0 {
		response.BadRequest(c, "monthly_budget must be non-negative")
		return
	}

	// Verify that the account exists and is a Copilot account.
	acc, err := h.adminSvc.GetAccount(c.Request.Context(), accountID)
	if err != nil {
		response.NotFound(c, "account not found")
		return
	}
	if acc.Platform != service.PlatformCopilot {
		response.BadRequest(c, "account is not a Copilot account")
		return
	}

	alert := &service.CopilotBudgetAlert{
		AccountID:      accountID,
		MonthlyBudget:  body.MonthlyBudget,
		AlertThreshold: body.AlertThreshold,
		Enabled:        body.Enabled,
	}
	if err := h.alertRepo.Upsert(c.Request.Context(), alert); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, alert)
}

// ─────────────────────────────────────────────
// 辅助函数
// ─────────────────────────────────────────────

// parseIDParam parses an int64 route param; writes a 400 response and returns false on failure.
func parseIDParam(c *gin.Context, param string) (int64, bool) {
	raw := strings.TrimSpace(c.Param(param))
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid "+param)
		return 0, false
	}
	return id, true
}
