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
	UserID          int64
	Username        string
	PremiumRequests int
	AgentRequests   int
	TotalRequests   int
	Models          []string
	LastRequestAt   *time.Time
}

// CopilotUserStatsResult is the response for the user stats endpoint.
type CopilotUserStatsResult struct {
	Date                 string
	TotalPremiumRequests int
	TotalAgentRequests   int
	ActiveUsers          int
	Users                []CopilotUserStatEntry
}

// CopilotHourlyBucket is one hour's premium + agent count.
type CopilotHourlyBucket struct {
	Hour         int
	PremiumCount int
	AgentCount   int
}

// CopilotUserTimelineResult is the response for the per-user hourly timeline.
type CopilotUserTimelineResult struct {
	UserID int64
	Date   string
	Hourly []CopilotHourlyBucket
}

// CopilotRequestItem represents a single usage log entry with optional sub-requests.
type CopilotRequestItem struct {
	RequestID   string
	Model       string
	Initiator   string
	CreatedAt   time.Time
	DurationMs  *int
	SubRequests []CopilotRequestItem
}

// CopilotUserRequestsResult is the response for the per-user requests list.
type CopilotUserRequestsResult struct {
	Total int
	Items []CopilotRequestItem
}

// ─────────────────────────────────────────────
// 账户维度数据类型
// ─────────────────────────────────────────────

// CopilotAccountOverviewEntry holds one account's summary.
type CopilotAccountOverviewEntry struct {
	AccountID                    int64
	Name                         string
	PlanType                     string
	SeatCount                    int
	MonthlyCost                  float64
	CostPerPremiumRequest        float64
	SystemTodayPremiumRequests   int
	SystemMonthPremiumRequests   int
	QuotaSnapshot                *CopilotAccountQuotaSnapshot
	BudgetAlert                  *CopilotAccountBudgetAlertInfo
	AlertStatus                  string
}

// CopilotAccountQuotaSnapshot is the quota data returned in the overview.
type CopilotAccountQuotaSnapshot struct {
	Entitlement     int
	Remaining       int
	GitHubTotalUsed int
	Overage         int
	Unlimited       bool
	ExternalUsed    int    // = GitHubTotalUsed - SystemMonthPremiumRequests
	CachedAt        *time.Time
}

// CopilotAccountBudgetAlertInfo is the budget alert config returned in the overview.
type CopilotAccountBudgetAlertInfo struct {
	MonthlyBudget  float64
	AlertThreshold int
	Enabled        bool
}

// CopilotAccountsOverviewResult is the response for the accounts overview endpoint.
type CopilotAccountsOverviewResult struct {
	TotalAccounts          int
	EstimatedMonthlyCost   float64
	TodayPremiumRequests   int
	AlertCount             int
	Accounts               []CopilotAccountOverviewEntry
}

// ─────────────────────────────────────────────
// 用户维度查询
// ─────────────────────────────────────────────

// GetUserStats returns per-user Copilot request counts for the given date.
// date must be in "YYYY-MM-DD" format.
func (s *CopilotAnalyticsService) GetUserStats(ctx context.Context, date string, userID int64) (*CopilotUserStatsResult, error) {
	// Determine date bounds in UTC.
	day, err := time.Parse("2006-01-02", date)
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
	day, err := time.Parse("2006-01-02", date)
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
	day, err := time.Parse("2006-01-02", date)
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

	// 2. Load all Copilot accounts.
	accounts, _, err := s.adminSvc.ListAccounts(ctx, 1, 500, PlatformCopilot, "", StatusActive, "", 0)
	if err != nil {
		return nil, fmt.Errorf("copilot analytics: list accounts: %w", err)
	}

	// 3. Fetch system usage counts for today and current month.
	todayCounts, monthCounts, err := s.fetchAccountUsageCounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("copilot analytics: fetch usage counts: %w", err)
	}

	// 4. Fetch all alert configs.
	alerts, err := s.alertRepo.ListEnabled(ctx)
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
					usageRate := float64(pi.Used) / float64(pi.Entitlement) * 100
					alertCfg := alertByAccount[acc.ID]
					threshold := 80
					if alertCfg != nil {
						threshold = alertCfg.AlertThreshold
					}
					alertStatus = AlertStatus(usageRate, threshold)
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

// GetAccountHourlyStats returns 24-hour bucket stats for an account on a given date.
func (s *CopilotAnalyticsService) GetAccountHourlyStats(ctx context.Context, accountID int64, date string) ([]CopilotHourlyBucket, error) {
	day, err := time.Parse("2006-01-02", date)
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
	now := time.Now().UTC()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

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
