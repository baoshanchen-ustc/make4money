package service

import (
	"context"
	"time"
)

// ─────────────────────────────────────────────
// Copilot 套餐常量
// ─────────────────────────────────────────────

// CopilotPlanConfig holds the monthly cost and monthly premium quota for a plan.
type CopilotPlanConfig struct {
	// MonthlyCostPerSeat is the USD cost per seat per month.
	MonthlyCostPerSeat float64
	// PremiumQuotaPerSeat is the number of premium interactions per seat per month.
	PremiumQuotaPerSeat int
}

// CopilotPlanConfigs maps plan type strings to their pricing and quota config.
// Per-seat values: multiply by seat count for multi-seat plans.
var CopilotPlanConfigs = map[string]CopilotPlanConfig{
	"individual_free":    {MonthlyCostPerSeat: 0, PremiumQuotaPerSeat: 50},
	"individual":         {MonthlyCostPerSeat: 10, PremiumQuotaPerSeat: 300}, // legacy alias for individual_pro
	"individual_pro":     {MonthlyCostPerSeat: 10, PremiumQuotaPerSeat: 300},
	"individual_pro_plus": {MonthlyCostPerSeat: 39, PremiumQuotaPerSeat: 1500},
	"business":           {MonthlyCostPerSeat: 19, PremiumQuotaPerSeat: 300},
	"enterprise":         {MonthlyCostPerSeat: 39, PremiumQuotaPerSeat: 1000},
}

// ─────────────────────────────────────────────
// CopilotQuotaSnapshot 服务层结构体
// ─────────────────────────────────────────────

// CopilotQuotaSnapshot is the service-layer representation of a daily quota snapshot
// for a Copilot account. Written on every successful real-time quota fetch (UPSERT).
type CopilotQuotaSnapshot struct {
	ID                 int64
	AccountID          int64
	SnapshotDate       time.Time // date precision (no time component)
	PlanType           *string
	PremiumEntitlement int
	PremiumRemaining   int
	PremiumUsed        int
	PremiumOverage     int
	Unlimited          bool
	CreatedAt          time.Time
}

// CopilotQuotaSnapshotRepository defines the persistence interface for quota snapshots.
type CopilotQuotaSnapshotRepository interface {
	// Upsert inserts or updates the snapshot for (account_id, snapshot_date).
	// Conflicts on the unique (account_id, snapshot_date) index are resolved by
	// updating all quota fields.
	Upsert(ctx context.Context, snap *CopilotQuotaSnapshot) error

	// ListByAccountID returns snapshots for the given account ordered by snapshot_date ASC.
	// At most `limit` rows are returned. Pass 0 for unlimited.
	ListByAccountID(ctx context.Context, accountID int64, limit int) ([]*CopilotQuotaSnapshot, error)

	// GetLatestByAccountID returns the most recent snapshot for the account, or nil
	// if no snapshot exists.
	GetLatestByAccountID(ctx context.Context, accountID int64) (*CopilotQuotaSnapshot, error)
}

// ─────────────────────────────────────────────
// CopilotBudgetAlert 服务层结构体
// ─────────────────────────────────────────────

// CopilotBudgetAlert is the service-layer representation of a budget alert
// configuration for a single Copilot account.
type CopilotBudgetAlert struct {
	ID             int64
	AccountID      int64
	MonthlyBudget  float64
	AlertThreshold int     // percentage (0-100)
	Enabled        bool
	LastAlertedAt  *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// CopilotBudgetAlertRepository defines the persistence interface for budget alerts.
type CopilotBudgetAlertRepository interface {
	// Upsert inserts or updates the alert config for the account.
	// Conflicts on the unique account_id index are resolved by updating all fields.
	Upsert(ctx context.Context, alert *CopilotBudgetAlert) error

	// GetByAccountID returns the alert config for the given account, or nil if none exists.
	GetByAccountID(ctx context.Context, accountID int64) (*CopilotBudgetAlert, error)

	// ListEnabled returns all alert configs that have enabled=true.
	ListEnabled(ctx context.Context) ([]*CopilotBudgetAlert, error)

	// ListAll returns every alert config regardless of enabled status.
	// Used by GetAccountsOverview so disabled configs remain visible for editing.
	ListAll(ctx context.Context) ([]*CopilotBudgetAlert, error)
}
