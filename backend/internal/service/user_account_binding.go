package service

import (
	"context"
	"time"
)

// UserAccountBinding represents a long-term binding between a downstream user/project
// and an upstream account. Used for P0-2 sticky session hardening.
type UserAccountBinding struct {
	ID        int64
	ProjectFP string // SHA256 hash of device_id or fallback key, truncated
	AccountID int64
	GroupID   int64
	ExpiresAt time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

// AccountUserFanoutSnapshot summarizes how many downstream users/projects are bound
// to each upstream account. It is used by runtime metrics to watch whether P0-2 is
// converging toward one external user per upstream account.
type AccountUserFanoutSnapshot struct {
	AccountCount     int     `json:"account_count"`
	ExternalUsersP95 float64 `json:"external_users_p95"`
	ExternalUsersMax int64   `json:"external_users_max"`
}

// UserAccountBindingRepository defines the interface for managing user-account bindings.
type UserAccountBindingRepository interface {
	// GetBinding looks up a binding by (project_fp, group_id).
	// Returns nil, nil if not found.
	GetBinding(ctx context.Context, projectFP string, groupID int64) (*UserAccountBinding, error)

	// UpsertBinding creates or updates a binding.
	// On conflict (project_fp, group_id), updates account_id, expires_at, and updated_at.
	UpsertBinding(ctx context.Context, projectFP string, accountID int64, groupID int64, expiresAt time.Time) error

	// DeleteBinding removes a specific binding.
	// Called when the bound account becomes unschedulable for this user.
	DeleteBinding(ctx context.Context, projectFP string, groupID int64) error

	// DeleteByAccountID removes all bindings for a given account.
	// Called when an account is banned or permanently disabled.
	DeleteByAccountID(ctx context.Context, accountID int64) (int, error)

	// DeleteExpired removes all bindings that have passed their expires_at.
	// Returns the number of deleted rows.
	DeleteExpired(ctx context.Context) (int, error)

	// SnapshotAccountUserFanout returns current per-account downstream user/project fanout.
	SnapshotAccountUserFanout(ctx context.Context) (*AccountUserFanoutSnapshot, error)
}
