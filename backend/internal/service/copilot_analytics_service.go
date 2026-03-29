package service

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"time"

	copilotpkg "github.com/Wei-Shaw/sub2api/internal/pkg/copilot"
)

// ─────────────────────────────────────────────
// CopilotAnalyticsService — 用户维度 + 账户维度聚合查询
// ─────────────────────────────────────────────

// CopilotAnalyticsService 封装 Copilot 分析所需的统计查询逻辑。
type CopilotAnalyticsService struct {
	db        *sql.DB
	quotaCache *CopilotQuotaCacheService
	adminSvc  AdminService
	snapshotRepo CopilotQuotaSnapshotRepository
	alertRepo    CopilotBudgetAlertRepository
}

// NewCopilotAnalyticsService constructs a CopilotAnalyticsService.
func NewCopilotAnalyticsService(
	db *sql.DB,
	quotaCache *CopilotQuotaCacheService,
	adminSvc AdminService,
	snapshotRepo CopilotQuotaSnapshotRepository,
	alertRepo CopilotBudgetAlertRepository,
) *CopilotAnalyticsService {
	return &CopilotAnalyticsService{
		db:           db,
		quotaCache:   quotaCache,
		adminSvc:     adminSvc,
		snapshotRepo: snapshotRepo,
		alertRepo:    alertRepo,
	}
}

// ─────────────────────────────────────────────
// 用户维度数据类型
// ─────────────────────────────────────────────

// CopilotUserStatEntry holds per-user stats for a given date.
type CopilotUserStatEntry struct {
	UserID          int64      `json:"user_id"`
	Username        string     `json:"username"`
	PremiumRequests int        `json:"premium_requests"`
	AgentRequests   int        `json:"agent_requests"`
	TotalRequests   int        `json:"total_requests"`
	Models          []string   `json:"models"`
	LastRequestAt   *time.Time `json:"last_request_at"`
}

// CopilotUserStatsResult is the response for the user stats endpoint.
type CopilotUserStatsResult struct {
	Date                 string                 `json:"date"`
	TotalPremiumRequests int                    `json:"total_premium_requests"`
	TotalAgentRequests   int                    `json:"total_agent_requests"`
	ActiveUsers          int                    `json:"active_users"`
	Users                []CopilotUserStatEntry `json:"users"`
}

// CopilotHourlyBucket is one hour's premium + agent count.
type CopilotHourlyBucket struct {
	Hour         int `json:"hour"`
	PremiumCount int `json:"premium_count"`
	AgentCount   int `json:"agent_count"`
}

// CopilotUserTimelineResult is the response for the per-user hourly timeline.
type CopilotUserTimelineResult struct {
	UserID int64                 `json:"user_id"`
	Date   string                `json:"date"`
	Hourly []CopilotHourlyBucket `json:"hourly"`
}

// CopilotRequestItem represents a single usage log entry with optional sub-requests.
type CopilotRequestItem struct {
	RequestID   string               `json:"request_id"`
	Model       string               `json:"model"`
	Initiator   string               `json:"initiator"`
	CreatedAt   time.Time            `json:"created_at"`
	DurationMs  *int                 `json:"duration_ms"`
	SubRequests []CopilotRequestItem `json:"sub_requests"`
}

// CopilotUserRequestsResult is the response for the per-user requests list.
type CopilotUserRequestsResult struct {
	Total int                  `json:"total"`
	Items []CopilotRequestItem `json:"items"`
}

// ─────────────────────────────────────────────
// 账户维度数据类型
// ─────────────────────────────────────────────

// CopilotAccountOverviewEntry holds one account's summary.
type CopilotAccountOverviewEntry struct {
	AccountID                  int64                          `json:"account_id"`
	Name                       string                         `json:"name"`
	PlanType                   string                         `json:"plan_type"`
	SeatCount                  int                            `json:"seat_count"`
	MonthlyCost                float64                        `json:"monthly_cost"`
	CostPerPremiumRequest      float64                        `json:"cost_per_premium_request"`
	SystemTodayPremiumRequests int                            `json:"system_today_premium_requests"`
	SystemMonthPremiumRequests int                            `json:"system_month_premium_requests"`
	QuotaSnapshot              *CopilotAccountQuotaSnapshot   `json:"quota_snapshot"`
	BudgetAlert                *CopilotAccountBudgetAlertInfo `json:"budget_alert"`
	AlertStatus                string                         `json:"alert_status"`
}

// CopilotAccountQuotaSnapshot is the quota data returned in the overview.
type CopilotAccountQuotaSnapshot struct {
	Entitlement     int        `json:"entitlement"`
	Remaining       int        `json:"remaining"`
	GitHubTotalUsed int        `json:"github_total_used"`
	Overage         int        `json:"overage"`
	Unlimited       bool       `json:"unlimited"`
	ExternalUsed    int        `json:"external_used"` // = GitHubTotalUsed - SystemMonthPremiumRequests
	CachedAt        *time.Time `json:"cached_at"`
}

// CopilotAccountBudgetAlertInfo is the budget alert config returned in the overview.
type CopilotAccountBudgetAlertInfo struct {
	MonthlyBudget  float64 `json:"monthly_budget"`
	AlertThreshold int     `json:"alert_threshold"`
	Enabled        bool    `json:"enabled"`
}

// CopilotAccountsOverviewResult is the response for the accounts overview endpoint.
type CopilotAccountsOverviewResult struct {
	TotalAccounts        int                           `json:"total_accounts"`
	EstimatedMonthlyCost float64                       `json:"estimated_monthly_cost"`
	TodayPremiumRequests int                           `json:"today_premium_requests"`
	AlertCount           int                           `json:"alert_count"`
	Accounts             []CopilotAccountOverviewEntry `json:"accounts"`
}

// ─────────────────────────────────────────────
// 用户维度查询
// ─────────────────────────────────────────────

// GetUserStats returns per-user Copilot request counts for the given date.
// date must be in "YYYY-MM-DD" format; interpreted in the server's local timezone
// (set via timezone.Init at startup, defaults to Asia/Shanghai).
func (s *CopilotAnalyticsService) GetUserStats(ctx context.Context, date string, userID int64) (*CopilotUserStatsResult, error) {
	// Parse date in local timezone so that date boundaries match the server's configured timezone.
	day, err := time.ParseInLocation("2006-01-02", date, time.Local)
	if err != nil {
		return nil, fmt.Errorf("copilot analytics: invalid date %q: %w", date, err)
	}
	start := day
	end := day.AddDate(0, 0, 1)

	// Build query — optionally filter by user_id.
	args := []any{start, end}
	userFilter := ""
	if userID > 0 {
		userFilter = " AND ul.user_id = $3"
		args = append(args, userID)
	}

	query := fmt.Sprintf(`
SELECT
    ul.user_id,
    COALESCE(u.username, ul.user_id::text) AS username,
    COUNT(*) FILTER (WHERE ul.initiator = 'user')  AS premium_requests,
    COUNT(*) FILTER (WHERE ul.initiator = 'agent') AS agent_requests,
    COUNT(*)                                         AS total_requests,
    ARRAY_AGG(DISTINCT ul.model ORDER BY ul.model)  AS models,
    MAX(ul.created_at)                               AS last_request_at
FROM usage_logs ul
LEFT JOIN users u ON u.id = ul.user_id
WHERE ul.created_at >= $1
  AND ul.created_at < $2
  AND ul.account_id IN (
      SELECT id FROM accounts WHERE platform = 'copilot'
  )%s
GROUP BY ul.user_id, u.username
ORDER BY premium_requests DESC, total_requests DESC
`, userFilter)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("copilot analytics: user stats query: %w", err)
	}
	defer rows.Close()

	var users []CopilotUserStatEntry
	var totalPremium, totalAgent int
	for rows.Next() {
		var e CopilotUserStatEntry
		var models []string
		if err := rows.Scan(
			&e.UserID, &e.Username,
			&e.PremiumRequests, &e.AgentRequests, &e.TotalRequests,
			pq_ArrayScan(&models), &e.LastRequestAt,
		); err != nil {
			return nil, fmt.Errorf("copilot analytics: scan user stats row: %w", err)
		}
		e.Models = models
		users = append(users, e)
		totalPremium += e.PremiumRequests
		totalAgent += e.AgentRequests
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("copilot analytics: user stats rows: %w", err)
	}

	return &CopilotUserStatsResult{
		Date:                 date,
		TotalPremiumRequests: totalPremium,
		TotalAgentRequests:   totalAgent,
		ActiveUsers:          len(users),
		Users:                users,
	}, nil
}

// GetUserTimeline returns 24-hour bucket stats for a single user on a given date.
func (s *CopilotAnalyticsService) GetUserTimeline(ctx context.Context, userID int64, date string) (*CopilotUserTimelineResult, error) {
	day, err := time.ParseInLocation("2006-01-02", date, time.Local)
	if err != nil {
		return nil, fmt.Errorf("copilot analytics: invalid date %q: %w", date, err)
	}
	start := day
	end := day.AddDate(0, 0, 1)

	query := `
SELECT
    EXTRACT(HOUR FROM created_at AT TIME ZONE 'UTC')::int AS hour,
    COUNT(*) FILTER (WHERE initiator = 'user')             AS premium_count,
    COUNT(*) FILTER (WHERE initiator = 'agent')            AS agent_count
FROM usage_logs
WHERE user_id = $1
  AND created_at >= $2
  AND created_at < $3
  AND account_id IN (SELECT id FROM accounts WHERE platform = 'copilot')
GROUP BY hour
ORDER BY hour
`

	rows, err := s.db.QueryContext(ctx, query, userID, start, end)
	if err != nil {
		return nil, fmt.Errorf("copilot analytics: user timeline query: %w", err)
	}
	defer rows.Close()

	// Initialise all 24 buckets to zero.
	hourly := make([]CopilotHourlyBucket, 24)
	for i := range hourly {
		hourly[i].Hour = i
	}
	for rows.Next() {
		var hour, premium, agent int
		if err := rows.Scan(&hour, &premium, &agent); err != nil {
			return nil, fmt.Errorf("copilot analytics: scan timeline row: %w", err)
		}
		if hour >= 0 && hour < 24 {
			hourly[hour].PremiumCount = premium
			hourly[hour].AgentCount = agent
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("copilot analytics: timeline rows: %w", err)
	}

	return &CopilotUserTimelineResult{
		UserID: userID,
		Date:   date,
		Hourly: hourly,
	}, nil
}

// GetUserRequests returns paginated request rows for a user on a given date,
// with agent sub-requests nested under the nearest preceding user request
// (within a 30-second window).
func (s *CopilotAnalyticsService) GetUserRequests(
	ctx context.Context, userID int64, date string, page, pageSize int,
) (*CopilotUserRequestsResult, error) {
	day, err := time.ParseInLocation("2006-01-02", date, time.Local)
	if err != nil {
		return nil, fmt.Errorf("copilot analytics: invalid date %q: %w", date, err)
	}
	start := day
	end := day.AddDate(0, 0, 1)

	// Total count.
	var total int
	countQuery := `
SELECT COUNT(*) FROM usage_logs
WHERE user_id = $1
  AND created_at >= $2
  AND created_at < $3
  AND account_id IN (SELECT id FROM accounts WHERE platform = 'copilot')
`
	if err := s.db.QueryRowContext(ctx, countQuery, userID, start, end).Scan(&total); err != nil {
		return nil, fmt.Errorf("copilot analytics: user requests count: %w", err)
	}

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	dataQuery := `
SELECT request_id, model, initiator, created_at, duration_ms
FROM usage_logs
WHERE user_id = $1
  AND created_at >= $2
  AND created_at < $3
  AND account_id IN (SELECT id FROM accounts WHERE platform = 'copilot')
ORDER BY created_at ASC
LIMIT $4 OFFSET $5
`
	rows, err := s.db.QueryContext(ctx, dataQuery, userID, start, end, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("copilot analytics: user requests query: %w", err)
	}
	defer rows.Close()

	var allRows []CopilotRequestItem
	for rows.Next() {
		var item CopilotRequestItem
		if err := rows.Scan(&item.RequestID, &item.Model, &item.Initiator, &item.CreatedAt, &item.DurationMs); err != nil {
			return nil, fmt.Errorf("copilot analytics: scan request row: %w", err)
		}
		allRows = append(allRows, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("copilot analytics: request rows: %w", err)
	}

	// Build hierarchy: agent requests are grouped under the nearest preceding
	// user request within a 30-second window.
	items := buildRequestHierarchy(allRows)

	return &CopilotUserRequestsResult{
		Total: total,
		Items: items,
	}, nil
}

// buildRequestHierarchy groups agent sub-requests under their nearest preceding
// user request within a 30-second window. Orphan agent requests (no matching
// user request) are promoted to top-level.
func buildRequestHierarchy(rows []CopilotRequestItem) []CopilotRequestItem {
	const windowSecs = 30

	var result []CopilotRequestItem
	var lastUserIdx int = -1

	for _, row := range rows {
		r := row
		r.SubRequests = nil

		if r.Initiator != "agent" {
			result = append(result, r)
			lastUserIdx = len(result) - 1
			continue
		}

		// Attempt to attach to the most recent user request within the window.
		if lastUserIdx >= 0 {
			parent := &result[lastUserIdx]
			diff := r.CreatedAt.Sub(parent.CreatedAt).Seconds()
			if diff >= 0 && diff <= windowSecs {
				parent.SubRequests = append(parent.SubRequests, r)
				continue
			}
		}
		// Orphan agent request — promote to top level.
		result = append(result, r)
	}
	return result
}

// ─────────────────────────────────────────────
// 账户维度查询
// ─────────────────────────────────────────────

// GetAccountsOverview returns the full accounts overview including live quota and cost data.
func (s *CopilotAnalyticsService) GetAccountsOverview(ctx context.Context) (*CopilotAccountsOverviewResult, error) {
	// 1. Fetch live quotas (may call GitHub API if cache cold).
	cachedQuotas, err := s.quotaCache.FetchAllCached(ctx)
	if err != nil {
		return nil, fmt.Errorf("copilot analytics: fetch quotas: %w", err)
	}

	quotaByAccount := make(map[int64]*CopilotCachedQuota, len(cachedQuotas))
	for i := range cachedQuotas {
		q := cachedQuotas[i]
		quotaByAccount[q.AccountID] = &q
	}

	// 2. Load all Copilot accounts (paginates internally to avoid the 500-row cap).
	accounts, err := listAllActiveCopilotAccounts(ctx, s.adminSvc)
	if err != nil {
		return nil, fmt.Errorf("copilot analytics: list accounts: %w", err)
	}

	// 3. Fetch system usage counts for today and current month.
	todayCounts, monthCounts, err := s.fetchAccountUsageCounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("copilot analytics: fetch usage counts: %w", err)
	}

	// 4. Fetch all alert configs (including disabled ones so the UI can edit them).
	alerts, err := s.alertRepo.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("copilot analytics: list alerts: %w", err)
	}
	alertByAccount := make(map[int64]*CopilotBudgetAlert, len(alerts))
	for _, a := range alerts {
		alertByAccount[a.AccountID] = a
	}

	// 5. Assemble entries.
	var entries []CopilotAccountOverviewEntry
	var totalMonthlyCost float64
	var totalTodayPremium int
	var alertCount int

	for _, acc := range accounts {
		planType, seatCount := extractCopilotPlan(&acc)
		planCfg, ok := CopilotPlanConfigs[planType]
		if !ok {
			planCfg = CopilotPlanConfigs["individual_pro"]
		}
		monthlyCost := planCfg.MonthlyCostPerSeat * float64(seatCount)
		monthlyQuota := planCfg.PremiumQuotaPerSeat * seatCount
		var costPerReq float64
		if monthlyQuota > 0 && monthlyCost > 0 {
			costPerReq = monthlyCost / float64(monthlyQuota)
		}

		todayPremium := todayCounts[acc.ID]
		monthPremium := monthCounts[acc.ID]
		totalTodayPremium += todayPremium
		totalMonthlyCost += monthlyCost

		var quotaSnap *CopilotAccountQuotaSnapshot
		alertStatus := CopilotAlertStatusOK
		if cq, ok := quotaByAccount[acc.ID]; ok && cq.QuotaInfo != nil {
			pi := cq.QuotaInfo.PremiumInteractions
			if pi != nil {
				gitHubTotal := pi.Used
				extUsed := gitHubTotal - monthPremium
				if extUsed < 0 {
					extUsed = 0
				}
				quotaSnap = &CopilotAccountQuotaSnapshot{
					Entitlement:     pi.Entitlement,
					Remaining:       pi.Remaining,
					GitHubTotalUsed: gitHubTotal,
					Overage:         pi.OverageCount,
					Unlimited:       pi.Unlimited,
					ExternalUsed:    extUsed,
					CachedAt:        &cq.CachedAt,
				}

				if !pi.Unlimited && pi.Entitlement > 0 {
					alertCfg := alertByAccount[acc.ID]
					// Only trigger alerts when an enabled budget alert config exists.
					if alertCfg != nil && alertCfg.Enabled {
						usageRate := float64(pi.Used) / float64(pi.Entitlement) * 100
						alertStatus = AlertStatus(usageRate, alertCfg.AlertThreshold)
					}
				}
			}
		}

		if alertStatus != CopilotAlertStatusOK {
			alertCount++
		}

		var budgetInfo *CopilotAccountBudgetAlertInfo
		if alertCfg := alertByAccount[acc.ID]; alertCfg != nil {
			budgetInfo = &CopilotAccountBudgetAlertInfo{
				MonthlyBudget:  alertCfg.MonthlyBudget,
				AlertThreshold: alertCfg.AlertThreshold,
				Enabled:        alertCfg.Enabled,
			}
		}

		entries = append(entries, CopilotAccountOverviewEntry{
			AccountID:                  acc.ID,
			Name:                       acc.Name,
			PlanType:                   planType,
			SeatCount:                  seatCount,
			MonthlyCost:                roundCost(monthlyCost),
			CostPerPremiumRequest:      roundCostPrecise(costPerReq),
			SystemTodayPremiumRequests: todayPremium,
			SystemMonthPremiumRequests: monthPremium,
			QuotaSnapshot:              quotaSnap,
			BudgetAlert:                budgetInfo,
			AlertStatus:                alertStatus,
		})
	}

	return &CopilotAccountsOverviewResult{
		TotalAccounts:        len(entries),
		EstimatedMonthlyCost: roundCost(totalMonthlyCost),
		TodayPremiumRequests: totalTodayPremium,
		AlertCount:           alertCount,
		Accounts:             entries,
	}, nil
}

// GetAccountQuotaTrend returns daily quota snapshots for an account over the last N days.
func (s *CopilotAnalyticsService) GetAccountQuotaTrend(ctx context.Context, accountID int64, days int) ([]*CopilotQuotaSnapshot, error) {
	if days <= 0 {
		days = 30
	}
	return s.snapshotRepo.ListByAccountID(ctx, accountID, days)
}

// CopilotAccountDailyEntry holds one account's daily request count for a single day.
type CopilotAccountDailyEntry struct {
	AccountID int64  `json:"account_id"`
	Date      string `json:"date"`
	Count     int    `json:"count"`
}

// CopilotAccountsDailyStatsResult is the response for the all-accounts daily stats endpoint.
type CopilotAccountsDailyStatsResult struct {
	Accounts []CopilotAccountDailyAccountInfo `json:"accounts"`
	Days     []CopilotAccountDailyEntry       `json:"days"`
}

// CopilotAccountDailyAccountInfo holds minimal account metadata for the chart legend.
type CopilotAccountDailyAccountInfo struct {
	AccountID int64  `json:"account_id"`
	Name      string `json:"name"`
}

// GetAccountsDailyStats returns daily premium request counts for all Copilot accounts
// over the past [days] days (default 30, max 90). Each row is (account_id, date, count).
func (s *CopilotAnalyticsService) GetAccountsDailyStats(ctx context.Context, days int) (*CopilotAccountsDailyStatsResult, error) {
	if days <= 0 {
		days = 30
	}
	if days > 90 {
		days = 90
	}

	now := time.Now()
	rangeStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local).AddDate(0, 0, -(days - 1))

	query := `
SELECT
    ul.account_id,
    a.name,
    DATE(ul.created_at AT TIME ZONE 'UTC' AT TIME ZONE current_setting('TIMEZONE')) AS req_date,
    COUNT(*) AS count
FROM usage_logs ul
JOIN accounts a ON a.id = ul.account_id
WHERE ul.initiator = 'user'
  AND ul.created_at >= $1
  AND ul.account_id IN (SELECT id FROM accounts WHERE platform = 'copilot')
GROUP BY ul.account_id, a.name, req_date
ORDER BY ul.account_id, req_date
`
	rows, err := s.db.QueryContext(ctx, query, rangeStart)
	if err != nil {
		return nil, fmt.Errorf("copilot analytics: accounts daily stats query: %w", err)
	}
	defer rows.Close()

	accountMap := make(map[int64]string)
	var entries []CopilotAccountDailyEntry

	for rows.Next() {
		var accountID int64
		var name string
		var date time.Time
		var count int
		if err := rows.Scan(&accountID, &name, &date, &count); err != nil {
			return nil, fmt.Errorf("copilot analytics: scan daily stats row: %w", err)
		}
		accountMap[accountID] = name
		entries = append(entries, CopilotAccountDailyEntry{
			AccountID: accountID,
			Date:      date.Format("2006-01-02"),
			Count:     count,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("copilot analytics: daily stats rows: %w", err)
	}

	accounts := make([]CopilotAccountDailyAccountInfo, 0, len(accountMap))
	for id, name := range accountMap {
		accounts = append(accounts, CopilotAccountDailyAccountInfo{AccountID: id, Name: name})
	}

	return &CopilotAccountsDailyStatsResult{
		Accounts: accounts,
		Days:     entries,
	}, nil
}

// GetAccountHourlyStats returns 24-hour bucket stats for an account on a given date.
func (s *CopilotAnalyticsService) GetAccountHourlyStats(ctx context.Context, accountID int64, date string) ([]CopilotHourlyBucket, error) {
	day, err := time.ParseInLocation("2006-01-02", date, time.Local)
	if err != nil {
		return nil, fmt.Errorf("copilot analytics: invalid date %q: %w", date, err)
	}
	start := day
	end := day.AddDate(0, 0, 1)

	query := `
SELECT
    EXTRACT(HOUR FROM created_at AT TIME ZONE 'UTC')::int AS hour,
    COUNT(*) FILTER (WHERE initiator = 'user')             AS premium_count,
    COUNT(*) FILTER (WHERE initiator = 'agent')            AS agent_count
FROM usage_logs
WHERE account_id = $1
  AND created_at >= $2
  AND created_at < $3
GROUP BY hour
ORDER BY hour
`
	rows, err := s.db.QueryContext(ctx, query, accountID, start, end)
	if err != nil {
		return nil, fmt.Errorf("copilot analytics: account hourly stats: %w", err)
	}
	defer rows.Close()

	hourly := make([]CopilotHourlyBucket, 24)
	for i := range hourly {
		hourly[i].Hour = i
	}
	for rows.Next() {
		var hour, premium, agent int
		if err := rows.Scan(&hour, &premium, &agent); err != nil {
			return nil, fmt.Errorf("copilot analytics: scan hourly row: %w", err)
		}
		if hour >= 0 && hour < 24 {
			hourly[hour].PremiumCount = premium
			hourly[hour].AgentCount = agent
		}
	}
	return hourly, rows.Err()
}

// ─────────────────────────────────────────────
// 内部辅助函数
// ─────────────────────────────────────────────

// fetchAccountUsageCounts returns today's and this month's premium request counts
// per Copilot account from usage_logs.
func (s *CopilotAnalyticsService) fetchAccountUsageCounts(ctx context.Context) (
	today map[int64]int, month map[int64]int, err error,
) {
	now := time.Now() // uses time.Local (set by timezone.Init to Asia/Shanghai)
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)

	query := `
SELECT
    account_id,
    COUNT(*) FILTER (WHERE created_at >= $2) AS today_count,
    COUNT(*)                                  AS month_count
FROM usage_logs
WHERE initiator = 'user'
  AND created_at >= $1
  AND account_id IN (SELECT id FROM accounts WHERE platform = 'copilot')
GROUP BY account_id
`
	rows, err := s.db.QueryContext(ctx, query, monthStart, todayStart)
	if err != nil {
		return nil, nil, fmt.Errorf("copilot analytics: account usage counts: %w", err)
	}
	defer rows.Close()

	today = make(map[int64]int)
	month = make(map[int64]int)
	for rows.Next() {
		var accountID int64
		var todayCount, monthCount int
		if err := rows.Scan(&accountID, &todayCount, &monthCount); err != nil {
			return nil, nil, fmt.Errorf("copilot analytics: scan usage count row: %w", err)
		}
		today[accountID] = todayCount
		month[accountID] = monthCount
	}
	return today, month, rows.Err()
}

// extractCopilotPlan reads plan type and seat count from the account's extra JSONB field.
func extractCopilotPlan(acc *Account) (planType string, seatCount int) {
	planType = "individual_pro" // safe default
	seatCount = 1

	if acc.Extra == nil {
		return
	}
	if v, ok := acc.Extra["copilot_plan_type"]; ok {
		if s, ok := v.(string); ok && s != "" {
			planType = s
		}
	}
	if v, ok := acc.Extra["copilot_seat_count"]; ok {
		switch n := v.(type) {
		case float64:
			if n >= 1 {
				seatCount = int(n)
			}
		case int:
			if n >= 1 {
				seatCount = n
			}
		case int64:
			if n >= 1 {
				seatCount = int(n)
			}
		}
	}
	return
}

// roundCost rounds to 2 decimal places.
func roundCost(v float64) float64 {
	return math.Round(v*100) / 100
}

// roundCostPrecise rounds to 4 decimal places (for cost-per-request).
func roundCostPrecise(v float64) float64 {
	return math.Round(v*10000) / 10000
}

// pq_ArrayScan is a helper to scan a PostgreSQL array into a Go string slice.
func pq_ArrayScan(dest *[]string) interface{} {
	return &pqStringArray{dest: dest}
}

// pqStringArray adapts *[]string to implement sql.Scanner.
type pqStringArray struct {
	dest *[]string
}

// Scan implements sql.Scanner.
func (a *pqStringArray) Scan(src interface{}) error {
	if src == nil {
		*a.dest = nil
		return nil
	}
	return pqArrayScan(src, a.dest)
}

// pqArrayScan decodes a PostgreSQL text array format like {"a","b","c"} into a []string.
// This avoids importing pq directly in the service package.
func pqArrayScan(src interface{}, dest *[]string) error {
	switch v := src.(type) {
	case []byte:
		return decodePostgresTextArray(string(v), dest)
	case string:
		return decodePostgresTextArray(v, dest)
	default:
		return fmt.Errorf("copilot analytics: unsupported array type %T", src)
	}
}

// decodePostgresTextArray decodes a PostgreSQL text array literal like {a,b,c} or {"a","b"}.
func decodePostgresTextArray(s string, dest *[]string) error {
	if s == "{}" || s == "" {
		*dest = nil
		return nil
	}
	// Strip outer braces.
	if len(s) < 2 || s[0] != '{' || s[len(s)-1] != '}' {
		return fmt.Errorf("copilot analytics: invalid pg array: %q", s)
	}
	inner := s[1 : len(s)-1]
	if inner == "" {
		*dest = nil
		return nil
	}
	// Simple split — handles unquoted elements and basic quoted elements.
	var result []string
	var current []byte
	inQuote := false
	for i := 0; i < len(inner); i++ {
		c := inner[i]
		switch {
		case c == '"' && !inQuote:
			inQuote = true
		case c == '"' && inQuote:
			inQuote = false
		case c == ',' && !inQuote:
			result = append(result, string(current))
			current = current[:0]
		default:
			current = append(current, c)
		}
	}
	result = append(result, string(current))
	*dest = result
	return nil
}

// CopilotQuotaInfoFromCache is a convenience adapter that converts a cached quota
// to the pkg type for use in handlers.
func CopilotQuotaInfoFromCache(cq *CopilotCachedQuota) *copilotpkg.CopilotQuotaInfo {
	if cq == nil {
		return nil
	}
	return cq.QuotaInfo
}
