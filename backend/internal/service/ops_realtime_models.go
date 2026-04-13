package service

import "time"

// PlatformConcurrencyInfo aggregates concurrency usage by platform.
type PlatformConcurrencyInfo struct {
	Platform       string  `json:"platform"`
	CurrentInUse   int64   `json:"current_in_use"`
	MaxCapacity    int64   `json:"max_capacity"`
	LoadPercentage float64 `json:"load_percentage"`
	WaitingInQueue int64   `json:"waiting_in_queue"`
}

// GroupConcurrencyInfo aggregates concurrency usage by group.
//
// Note: one account can belong to multiple groups; group totals are therefore not additive across groups.
type GroupConcurrencyInfo struct {
	GroupID        int64   `json:"group_id"`
	GroupName      string  `json:"group_name"`
	Platform       string  `json:"platform"`
	CurrentInUse   int64   `json:"current_in_use"`
	MaxCapacity    int64   `json:"max_capacity"`
	LoadPercentage float64 `json:"load_percentage"`
	WaitingInQueue int64   `json:"waiting_in_queue"`
}

// AccountConcurrencyInfo represents real-time concurrency usage for a single account.
type AccountConcurrencyInfo struct {
	AccountID      int64   `json:"account_id"`
	AccountName    string  `json:"account_name"`
	Platform       string  `json:"platform"`
	GroupID        int64   `json:"group_id"`
	GroupName      string  `json:"group_name"`
	CurrentInUse   int64   `json:"current_in_use"`
	MaxCapacity    int64   `json:"max_capacity"`
	LoadPercentage float64 `json:"load_percentage"`
	WaitingInQueue int64   `json:"waiting_in_queue"`
}

// UserConcurrencyInfo represents real-time concurrency usage for a single user.
type UserConcurrencyInfo struct {
	UserID         int64   `json:"user_id"`
	UserEmail      string  `json:"user_email"`
	Username       string  `json:"username"`
	CurrentInUse   int64   `json:"current_in_use"`
	MaxCapacity    int64   `json:"max_capacity"`
	LoadPercentage float64 `json:"load_percentage"`
	WaitingInQueue int64   `json:"waiting_in_queue"`
}

// PlatformAvailability aggregates account availability by platform.
type PlatformAvailability struct {
	Platform                  string `json:"platform"`
	TotalAccounts             int64  `json:"total_accounts"`
	AvailableCount            int64  `json:"available_count"`
	RateLimitCount            int64  `json:"rate_limit_count"`
	ErrorCount                int64  `json:"error_count"`
	TokenRefreshFailureCount  int64  `json:"token_refresh_failure_count"`
	AuthFailureCount          int64  `json:"auth_failure_count"`
	PermanentAuthFailureCount int64  `json:"permanent_auth_failure_count"`
	TemporaryAuthFailureCount int64  `json:"temporary_auth_failure_count"`
}

// GroupAvailability aggregates account availability by group.
type GroupAvailability struct {
	GroupID                   int64  `json:"group_id"`
	GroupName                 string `json:"group_name"`
	Platform                  string `json:"platform"`
	TotalAccounts             int64  `json:"total_accounts"`
	AvailableCount            int64  `json:"available_count"`
	RateLimitCount            int64  `json:"rate_limit_count"`
	ErrorCount                int64  `json:"error_count"`
	TokenRefreshFailureCount  int64  `json:"token_refresh_failure_count"`
	AuthFailureCount          int64  `json:"auth_failure_count"`
	PermanentAuthFailureCount int64  `json:"permanent_auth_failure_count"`
	TemporaryAuthFailureCount int64  `json:"temporary_auth_failure_count"`
}

// AccountAvailability represents current availability for a single account.
type AccountAvailability struct {
	AccountID   int64  `json:"account_id"`
	AccountName string `json:"account_name"`
	Platform    string `json:"platform"`
	GroupID     int64  `json:"group_id"`
	GroupName   string `json:"group_name"`

	Status string `json:"status"`

	IsAvailable   bool `json:"is_available"`
	IsRateLimited bool `json:"is_rate_limited"`
	IsOverloaded  bool `json:"is_overloaded"`
	HasError      bool `json:"has_error"`

	RateLimitResetAt          *time.Time `json:"rate_limit_reset_at"`
	RateLimitRemainingSec     *int64     `json:"rate_limit_remaining_sec"`
	OverloadUntil             *time.Time `json:"overload_until"`
	OverloadRemainingSec      *int64     `json:"overload_remaining_sec"`
	ErrorMessage              string     `json:"error_message"`
	TempUnschedulableUntil    *time.Time `json:"temp_unschedulable_until,omitempty"`
	TokenRefreshFailureReason string     `json:"token_refresh_failure_reason,omitempty"`
	TokenRefreshFailureClass  string     `json:"token_refresh_failure_class,omitempty"`
	TokenRefreshFailedAt      string     `json:"token_refresh_failed_at,omitempty"`
	AuthFailureReason         string     `json:"auth_failure_reason,omitempty"`
	AuthFailureClass          string     `json:"auth_failure_class,omitempty"`
	AuthFailureSource         string     `json:"auth_failure_source,omitempty"`
	AuthState                 string     `json:"auth_state,omitempty"`
	AuthStateChangedAt        string     `json:"auth_state_changed_at,omitempty"`
	AuthRecoveryAction        string     `json:"auth_recovery_action,omitempty"`
	AuthDispatchSuppressed    bool       `json:"auth_dispatch_suppressed,omitempty"`
	AuthBackgroundRecovery    bool       `json:"auth_background_recovery,omitempty"`
	AuthManualReviewRequired  bool       `json:"auth_manual_review_required,omitempty"`
}

type TokenRefreshFailureRealtimeSummary struct {
	TotalAccounts      int64            `json:"total_accounts"`
	PermanentCount     int64            `json:"permanent_count"`
	ByReason           map[string]int64 `json:"by_reason,omitempty"`
	ByClass            map[string]int64 `json:"by_class,omitempty"`
	AffectedAccountIDs []int64          `json:"affected_account_ids,omitempty"`
}

type AccountAuthFailureRealtimeSummary struct {
	TotalAccounts          int64            `json:"total_accounts"`
	PermanentCount         int64            `json:"permanent_count"`
	TemporaryCount         int64            `json:"temporary_count"`
	DispatchSuppressed     int64            `json:"dispatch_suppressed"`
	BackgroundRecovery     int64            `json:"background_recovery"`
	ManualReviewRequired   int64            `json:"manual_review_required"`
	ByReason               map[string]int64 `json:"by_reason,omitempty"`
	ByClass                map[string]int64 `json:"by_class,omitempty"`
	BySource               map[string]int64 `json:"by_source,omitempty"`
	ByState                map[string]int64 `json:"by_state,omitempty"`
	AffectedAccountIDs     []int64          `json:"affected_account_ids,omitempty"`
	RecoveryActionExamples map[string]int64 `json:"recovery_action_examples,omitempty"`
}

type OpsRealtimeSummaryBundle struct {
	ResourceBudgetSummary *OpsResourceBudgetSummary            `json:"resource_budget_summary,omitempty"`
	StorageGovernance     *OpsStorageGovernanceSummary         `json:"storage_governance,omitempty"`
	ErrorFamilySummary    *OpsErrorFamilySummary               `json:"error_family_summary,omitempty"`
	FailureSplitSummary   *OpsFailureSplitSummary              `json:"failure_split_summary,omitempty"`
	StickySessionCleanup  *StickySessionCleanupMetricsSnapshot `json:"sticky_session_cleanup,omitempty"`
	StickyConsistency     *StickyConsistencyMetricsSnapshot    `json:"sticky_consistency,omitempty"`
	UsageIntegrity        *OpsUsageIntegritySummary            `json:"usage_integrity,omitempty"`
	DataSource            *OpsDataSourceSummary                `json:"data_source,omitempty"`
}
