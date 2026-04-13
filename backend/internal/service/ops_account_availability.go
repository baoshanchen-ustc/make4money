package service

import (
	"context"
	"errors"
	"strings"
	"time"
)

// GetAccountAvailabilityStats returns current account availability stats.
//
// Query-level filtering is intentionally limited to platform/group to match the dashboard scope.
func (s *OpsService) GetAccountAvailabilityStats(ctx context.Context, platformFilter string, groupIDFilter *int64) (
	map[string]*PlatformAvailability,
	map[int64]*GroupAvailability,
	map[int64]*AccountAvailability,
	*time.Time,
	error,
) {
	if err := s.RequireMonitoringEnabled(ctx); err != nil {
		return nil, nil, nil, nil, err
	}

	accounts, err := s.listAllAccountsForOps(ctx, platformFilter)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	if groupIDFilter != nil && *groupIDFilter > 0 {
		filtered := make([]Account, 0, len(accounts))
		for _, acc := range accounts {
			for _, grp := range acc.Groups {
				if grp != nil && grp.ID == *groupIDFilter {
					filtered = append(filtered, acc)
					break
				}
			}
		}
		accounts = filtered
	}

	now := time.Now()
	collectedAt := now

	platform := make(map[string]*PlatformAvailability)
	group := make(map[int64]*GroupAvailability)
	account := make(map[int64]*AccountAvailability)

	for _, acc := range accounts {
		if acc.ID <= 0 {
			continue
		}

		displayGroupID := int64(0)
		displayGroupName := ""
		groupsForStats := make([]*Group, 0, len(acc.Groups))
		if groupIDFilter != nil && *groupIDFilter > 0 {
			for _, grp := range acc.Groups {
				if grp != nil && grp.ID == *groupIDFilter {
					groupsForStats = append(groupsForStats, grp)
					displayGroupID = grp.ID
					displayGroupName = grp.Name
					break
				}
			}
		} else {
			groupsForStats = acc.Groups
			if len(acc.Groups) > 0 && acc.Groups[0] != nil {
				displayGroupID = acc.Groups[0].ID
				displayGroupName = acc.Groups[0].Name
			}
		}

		isTempUnsched := false
		if acc.TempUnschedulableUntil != nil && now.Before(*acc.TempUnschedulableUntil) {
			isTempUnsched = true
		}

		isRateLimited := acc.RateLimitResetAt != nil && now.Before(*acc.RateLimitResetAt)
		isOverloaded := acc.OverloadUntil != nil && now.Before(*acc.OverloadUntil)
		hasError := acc.Status == StatusError
		tokenRefreshFailureClass := strings.TrimSpace(acc.GetExtraString("token_refresh_failure_class"))
		hasPermanentRefreshFailure := tokenRefreshFailureClass == "permanent"
		authReason, authClass, authSource, authState, authChangedAt, authRecovery := effectiveAccountAuthFailure(&acc)
		hasAuthFailure := authReason != "" || authClass != "" || authSource != "" || authState != ""
		authDispatchSuppressed := accountAuthDispatchSuppressed(authState, authClass)
		authBackgroundRecovery := accountAuthBackgroundRecoveryEligible(&acc, authState, authClass, authRecovery)
		authManualReviewRequired := accountAuthManualInterventionRequired(authState, authClass, authRecovery)

		// Normalize exclusive status flags so the UI doesn't show conflicting badges.
		if hasError {
			isRateLimited = false
			isOverloaded = false
		}

		isAvailable := acc.Status == StatusActive && acc.Schedulable && !isRateLimited && !isOverloaded && !isTempUnsched

		if acc.Platform != "" {
			if _, ok := platform[acc.Platform]; !ok {
				platform[acc.Platform] = &PlatformAvailability{
					Platform: acc.Platform,
				}
			}
			p := platform[acc.Platform]
			p.TotalAccounts++
			if isAvailable {
				p.AvailableCount++
			}
			if isRateLimited {
				p.RateLimitCount++
			}
			if hasError {
				p.ErrorCount++
			}
			if hasPermanentRefreshFailure {
				p.TokenRefreshFailureCount++
			}
			if hasAuthFailure {
				p.AuthFailureCount++
			}
			if authClass == accountAuthClassPermanent {
				p.PermanentAuthFailureCount++
			}
			if authClass == accountAuthClassTemporary {
				p.TemporaryAuthFailureCount++
			}
		}

		for _, grp := range groupsForStats {
			if grp == nil || grp.ID <= 0 {
				continue
			}
			if _, ok := group[grp.ID]; !ok {
				group[grp.ID] = &GroupAvailability{
					GroupID:   grp.ID,
					GroupName: grp.Name,
					Platform:  grp.Platform,
				}
			}
			g := group[grp.ID]
			g.TotalAccounts++
			if isAvailable {
				g.AvailableCount++
			}
			if isRateLimited {
				g.RateLimitCount++
			}
			if hasError {
				g.ErrorCount++
			}
			if hasPermanentRefreshFailure {
				g.TokenRefreshFailureCount++
			}
			if hasAuthFailure {
				g.AuthFailureCount++
			}
			if authClass == accountAuthClassPermanent {
				g.PermanentAuthFailureCount++
			}
			if authClass == accountAuthClassTemporary {
				g.TemporaryAuthFailureCount++
			}
		}

		item := &AccountAvailability{
			AccountID:   acc.ID,
			AccountName: acc.Name,
			Platform:    acc.Platform,
			GroupID:     displayGroupID,
			GroupName:   displayGroupName,
			Status:      acc.Status,

			IsAvailable:   isAvailable,
			IsRateLimited: isRateLimited,
			IsOverloaded:  isOverloaded,
			HasError:      hasError,

			ErrorMessage:              acc.ErrorMessage,
			TokenRefreshFailureReason: strings.TrimSpace(acc.GetExtraString("token_refresh_failure_reason")),
			TokenRefreshFailureClass:  strings.TrimSpace(acc.GetExtraString("token_refresh_failure_class")),
			TokenRefreshFailedAt:      strings.TrimSpace(acc.GetExtraString("token_refresh_failed_at")),
			AuthFailureReason:         authReason,
			AuthFailureClass:          authClass,
			AuthFailureSource:         authSource,
			AuthState:                 authState,
			AuthStateChangedAt:        authChangedAt,
			AuthRecoveryAction:        authRecovery,
			AuthDispatchSuppressed:    authDispatchSuppressed,
			AuthBackgroundRecovery:    authBackgroundRecovery,
			AuthManualReviewRequired:  authManualReviewRequired,
		}

		if isRateLimited && acc.RateLimitResetAt != nil {
			item.RateLimitResetAt = acc.RateLimitResetAt
			remainingSec := int64(time.Until(*acc.RateLimitResetAt).Seconds())
			if remainingSec > 0 {
				item.RateLimitRemainingSec = &remainingSec
			}
		}
		if isOverloaded && acc.OverloadUntil != nil {
			item.OverloadUntil = acc.OverloadUntil
			remainingSec := int64(time.Until(*acc.OverloadUntil).Seconds())
			if remainingSec > 0 {
				item.OverloadRemainingSec = &remainingSec
			}
		}
		if isTempUnsched && acc.TempUnschedulableUntil != nil {
			item.TempUnschedulableUntil = acc.TempUnschedulableUntil
		}

		account[acc.ID] = item
	}

	return platform, group, account, &collectedAt, nil
}

type OpsAccountAvailability struct {
	Group       *GroupAvailability
	Accounts    map[int64]*AccountAvailability
	CollectedAt *time.Time
}

func (s *OpsService) GetAccountAvailability(ctx context.Context, platformFilter string, groupIDFilter *int64) (*OpsAccountAvailability, error) {
	if s == nil {
		return nil, errors.New("ops service is nil")
	}

	if s.getAccountAvailability != nil {
		return s.getAccountAvailability(ctx, platformFilter, groupIDFilter)
	}

	_, groupStats, accountStats, collectedAt, err := s.GetAccountAvailabilityStats(ctx, platformFilter, groupIDFilter)
	if err != nil {
		return nil, err
	}

	var group *GroupAvailability
	if groupIDFilter != nil && *groupIDFilter > 0 {
		group = groupStats[*groupIDFilter]
	}

	if accountStats == nil {
		accountStats = map[int64]*AccountAvailability{}
	}

	return &OpsAccountAvailability{
		Group:       group,
		Accounts:    accountStats,
		CollectedAt: collectedAt,
	}, nil
}

func SummarizeTokenRefreshFailures(accounts map[int64]*AccountAvailability) *TokenRefreshFailureRealtimeSummary {
	if len(accounts) == 0 {
		return nil
	}
	summary := &TokenRefreshFailureRealtimeSummary{
		ByReason: make(map[string]int64),
		ByClass:  make(map[string]int64),
	}
	for accountID, acc := range accounts {
		if acc == nil {
			continue
		}
		reason := strings.TrimSpace(acc.TokenRefreshFailureReason)
		class := strings.TrimSpace(acc.TokenRefreshFailureClass)
		if reason == "" && class == "" {
			continue
		}
		summary.TotalAccounts++
		summary.AffectedAccountIDs = append(summary.AffectedAccountIDs, accountID)
		if reason != "" {
			summary.ByReason[reason]++
		}
		if class != "" {
			summary.ByClass[class]++
			if class == "permanent" {
				summary.PermanentCount++
			}
		}
	}
	if summary.TotalAccounts == 0 {
		return nil
	}
	if len(summary.AffectedAccountIDs) > 20 {
		summary.AffectedAccountIDs = summary.AffectedAccountIDs[:20]
	}
	if len(summary.ByReason) == 0 {
		summary.ByReason = nil
	}
	if len(summary.ByClass) == 0 {
		summary.ByClass = nil
	}
	return summary
}

func SummarizeAccountAuthFailures(accounts map[int64]*AccountAvailability) *AccountAuthFailureRealtimeSummary {
	if len(accounts) == 0 {
		return nil
	}
	summary := &AccountAuthFailureRealtimeSummary{
		ByReason:               make(map[string]int64),
		ByClass:                make(map[string]int64),
		BySource:               make(map[string]int64),
		ByState:                make(map[string]int64),
		RecoveryActionExamples: make(map[string]int64),
	}
	for accountID, acc := range accounts {
		if acc == nil {
			continue
		}
		reason := strings.TrimSpace(acc.AuthFailureReason)
		class := strings.TrimSpace(acc.AuthFailureClass)
		source := strings.TrimSpace(acc.AuthFailureSource)
		state := strings.TrimSpace(acc.AuthState)
		recovery := strings.TrimSpace(acc.AuthRecoveryAction)
		if reason == "" && class == "" && source == "" && state == "" {
			continue
		}
		summary.TotalAccounts++
		summary.AffectedAccountIDs = append(summary.AffectedAccountIDs, accountID)
		if reason != "" {
			summary.ByReason[reason]++
		}
		if class != "" {
			summary.ByClass[class]++
			switch class {
			case accountAuthClassPermanent:
				summary.PermanentCount++
			case accountAuthClassTemporary:
				summary.TemporaryCount++
			}
		}
		if source != "" {
			summary.BySource[source]++
		}
		if state != "" {
			summary.ByState[state]++
		}
		if acc.AuthDispatchSuppressed {
			summary.DispatchSuppressed++
		}
		if acc.AuthBackgroundRecovery {
			summary.BackgroundRecovery++
		}
		if acc.AuthManualReviewRequired {
			summary.ManualReviewRequired++
		}
		if recovery != "" {
			summary.RecoveryActionExamples[recovery]++
		}
	}
	if summary.TotalAccounts == 0 {
		return nil
	}
	if len(summary.AffectedAccountIDs) > 20 {
		summary.AffectedAccountIDs = summary.AffectedAccountIDs[:20]
	}
	if len(summary.ByReason) == 0 {
		summary.ByReason = nil
	}
	if len(summary.ByClass) == 0 {
		summary.ByClass = nil
	}
	if len(summary.BySource) == 0 {
		summary.BySource = nil
	}
	if len(summary.ByState) == 0 {
		summary.ByState = nil
	}
	if len(summary.RecoveryActionExamples) == 0 {
		summary.RecoveryActionExamples = nil
	}
	return summary
}
