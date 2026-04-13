package service

import (
	"context"
	"log/slog"
	"strings"
	"time"
)

const (
	accountAuthFailureReasonKey = "account_auth_failure_reason"
	accountAuthFailureClassKey  = "account_auth_failure_class"
	accountAuthFailureSourceKey = "account_auth_failure_source"
	accountAuthStateKey         = "account_auth_state"
	accountAuthStateAtKey       = "account_auth_state_changed_at"
	accountAuthRecoveryKey      = "account_auth_recovery_action"
	accountAuthStateVersionKey  = "account_auth_state_version"

	accountAuthClassTemporary = "temporary"
	accountAuthClassPermanent = "permanent"

	accountAuthSourceTokenRefresh = "token_refresh"
	accountAuthSourceUpstreamAuth = "upstream_auth"

	accountAuthStateRefreshPending         = "refresh_pending"
	accountAuthStateReauthorizeRequired    = "reauthorize_required"
	accountAuthStateAccountDeleted         = "account_deleted_or_revoked"
	accountAuthStateClientReconfigure      = "client_reconfiguration_required"
	accountAuthStateWorkspaceBlocked       = "workspace_or_billing_blocked"
	accountAuthStatePermissionDenied       = "permission_denied"
	accountAuthStateUnknownPermanent       = "unknown_permanent_failure"
	accountAuthStateUnknownTemporary       = "unknown_temporary_failure"
	accountAuthRecoveryActionBackground    = "background_refresh"
	accountAuthRecoveryActionReauthorize   = "reauthorize"
	accountAuthRecoveryActionReconfigure   = "reconfigure_client"
	accountAuthRecoveryActionManualReview  = "manual_review"
	accountAuthRecoveryActionBillingReview = "billing_or_workspace_review"
)

func classifyAccountAuthState(reason, class string) (string, string) {
	reason = strings.TrimSpace(reason)
	class = strings.TrimSpace(class)

	switch reason {
	case "upstream_401_refresh_required", "refresh_retry_exhausted":
		return accountAuthStateRefreshPending, accountAuthRecoveryActionBackground
	case "token_revoked", "refresh_token_reused", "invalid_bearer_token", "access_denied", "invalid_grant":
		return accountAuthStateReauthorizeRequired, accountAuthRecoveryActionReauthorize
	case "invalid_client", "unauthorized_client", "missing_project_id", "missing_refresh_token":
		return accountAuthStateClientReconfigure, accountAuthRecoveryActionReconfigure
	case "account_deleted", "organization_disabled":
		return accountAuthStateAccountDeleted, accountAuthRecoveryActionManualReview
	case "workspace_deactivated", "billing_required":
		return accountAuthStateWorkspaceBlocked, accountAuthRecoveryActionBillingReview
	case "validation_required", "policy_violation", "forbidden":
		return accountAuthStatePermissionDenied, accountAuthRecoveryActionManualReview
	}

	if class == accountAuthClassPermanent {
		return accountAuthStateUnknownPermanent, accountAuthRecoveryActionManualReview
	}
	if class == accountAuthClassTemporary {
		return accountAuthStateUnknownTemporary, accountAuthRecoveryActionBackground
	}
	return "", ""
}

func markAccountAuthState(ctx context.Context, repo AccountRepository, accountID int64, reason, class, source string) {
	if repo == nil || accountID <= 0 {
		return
	}
	reason = strings.TrimSpace(reason)
	class = strings.TrimSpace(class)
	source = strings.TrimSpace(source)
	if reason == "" && class == "" && source == "" {
		return
	}
	state, recovery := classifyAccountAuthState(reason, class)
	updates := map[string]any{
		accountAuthFailureReasonKey: reason,
		accountAuthFailureClassKey:  class,
		accountAuthFailureSourceKey: source,
		accountAuthStateKey:         state,
		accountAuthStateAtKey:       time.Now().UTC().Format(time.RFC3339Nano),
		accountAuthRecoveryKey:      recovery,
		accountAuthStateVersionKey:  1,
	}
	if err := repo.UpdateExtra(ctx, accountID, updates); err != nil {
		slog.Warn("account_auth_state.mark_failed",
			"account_id", accountID,
			"reason", reason,
			"class", class,
			"source", source,
			"error", err,
		)
	}
}

func clearAccountAuthState(ctx context.Context, repo AccountRepository, accountID int64) {
	if repo == nil || accountID <= 0 {
		return
	}
	updates := map[string]any{
		accountAuthFailureReasonKey: nil,
		accountAuthFailureClassKey:  nil,
		accountAuthFailureSourceKey: nil,
		accountAuthStateKey:         nil,
		accountAuthStateAtKey:       nil,
		accountAuthRecoveryKey:      nil,
		accountAuthStateVersionKey:  nil,
	}
	if err := repo.UpdateExtra(ctx, accountID, updates); err != nil {
		slog.Warn("account_auth_state.clear_failed",
			"account_id", accountID,
			"error", err,
		)
	}
}

func effectiveAccountAuthFailure(account *Account) (reason, class, source, state, changedAt, recovery string) {
	if account == nil {
		return "", "", "", "", "", ""
	}

	reason = strings.TrimSpace(account.GetExtraString(accountAuthFailureReasonKey))
	class = strings.TrimSpace(account.GetExtraString(accountAuthFailureClassKey))
	source = strings.TrimSpace(account.GetExtraString(accountAuthFailureSourceKey))
	state = strings.TrimSpace(account.GetExtraString(accountAuthStateKey))
	changedAt = strings.TrimSpace(account.GetExtraString(accountAuthStateAtKey))
	recovery = strings.TrimSpace(account.GetExtraString(accountAuthRecoveryKey))

	if reason == "" {
		reason = strings.TrimSpace(account.GetExtraString("token_refresh_failure_reason"))
	}
	if class == "" {
		class = strings.TrimSpace(account.GetExtraString("token_refresh_failure_class"))
	}
	if source == "" && reason != "" {
		source = accountAuthSourceTokenRefresh
	}
	if changedAt == "" {
		changedAt = strings.TrimSpace(account.GetExtraString("token_refresh_failed_at"))
	}
	if state == "" {
		state, recovery = classifyAccountAuthState(reason, class)
	}
	return reason, class, source, state, changedAt, recovery
}

func hasPermanentAccountAuthFailure(account *Account) bool {
	_, class, _, _, _, _ := effectiveAccountAuthFailure(account)
	return class == accountAuthClassPermanent
}

func accountAuthDispatchSuppressed(state, class string) bool {
	state = strings.TrimSpace(state)
	class = strings.TrimSpace(class)
	if class == accountAuthClassPermanent {
		return true
	}
	switch state {
	case accountAuthStateRefreshPending,
		accountAuthStateReauthorizeRequired,
		accountAuthStateAccountDeleted,
		accountAuthStateClientReconfigure,
		accountAuthStateWorkspaceBlocked,
		accountAuthStatePermissionDenied:
		return true
	default:
		return false
	}
}

func accountAuthBackgroundRecoveryEligible(account *Account, state, class, recovery string) bool {
	if account == nil || account.Type != AccountTypeOAuth {
		return false
	}
	state = strings.TrimSpace(state)
	class = strings.TrimSpace(class)
	recovery = strings.TrimSpace(recovery)
	if class == accountAuthClassPermanent {
		return false
	}
	if recovery == accountAuthRecoveryActionBackground {
		return true
	}
	switch state {
	case accountAuthStateRefreshPending, accountAuthStateUnknownTemporary:
		return true
	default:
		return false
	}
}

func accountAuthManualInterventionRequired(state, class, recovery string) bool {
	class = strings.TrimSpace(class)
	recovery = strings.TrimSpace(recovery)
	if class == accountAuthClassPermanent {
		return true
	}
	switch recovery {
	case accountAuthRecoveryActionReauthorize,
		accountAuthRecoveryActionReconfigure,
		accountAuthRecoveryActionManualReview,
		accountAuthRecoveryActionBillingReview:
		return true
	default:
		return false
	}
}
