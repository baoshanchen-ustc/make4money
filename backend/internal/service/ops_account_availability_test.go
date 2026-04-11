package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

type opsAccountAvailabilityRepoStub struct {
	accounts []Account
}

func (s *opsAccountAvailabilityRepoStub) ListWithFilters(ctx context.Context, params pagination.PaginationParams, platform, accountType, status, search string, groupID int64, privacyMode string) ([]Account, *pagination.PaginationResult, error) {
	start := (params.Page - 1) * params.PageSize
	if start < 0 {
		start = 0
	}
	if start >= len(s.accounts) {
		return []Account{}, &pagination.PaginationResult{Total: int64(len(s.accounts)), Page: params.Page, PageSize: params.PageSize}, nil
	}
	end := start + params.PageSize
	if end > len(s.accounts) {
		end = len(s.accounts)
	}
	out := make([]Account, 0, end-start)
	for _, acc := range s.accounts[start:end] {
		if platform != "" && acc.Platform != platform {
			continue
		}
		out = append(out, acc)
	}
	return out, &pagination.PaginationResult{Total: int64(len(s.accounts)), Page: params.Page, PageSize: params.PageSize}, nil
}

func (s *opsAccountAvailabilityRepoStub) Create(context.Context, *Account) error {
	panic("unexpected call")
}
func (s *opsAccountAvailabilityRepoStub) GetByID(context.Context, int64) (*Account, error) {
	panic("unexpected call")
}
func (s *opsAccountAvailabilityRepoStub) GetByIDs(context.Context, []int64) ([]*Account, error) {
	panic("unexpected call")
}
func (s *opsAccountAvailabilityRepoStub) ExistsByID(context.Context, int64) (bool, error) {
	panic("unexpected call")
}
func (s *opsAccountAvailabilityRepoStub) GetByCRSAccountID(context.Context, string) (*Account, error) {
	panic("unexpected call")
}
func (s *opsAccountAvailabilityRepoStub) FindByExtraField(context.Context, string, any) ([]Account, error) {
	panic("unexpected call")
}
func (s *opsAccountAvailabilityRepoStub) ListCRSAccountIDs(context.Context) (map[string]int64, error) {
	panic("unexpected call")
}
func (s *opsAccountAvailabilityRepoStub) Update(context.Context, *Account) error {
	panic("unexpected call")
}
func (s *opsAccountAvailabilityRepoStub) Delete(context.Context, int64) error {
	panic("unexpected call")
}
func (s *opsAccountAvailabilityRepoStub) List(context.Context, pagination.PaginationParams) ([]Account, *pagination.PaginationResult, error) {
	panic("unexpected call")
}
func (s *opsAccountAvailabilityRepoStub) ListByGroup(context.Context, int64) ([]Account, error) {
	panic("unexpected call")
}
func (s *opsAccountAvailabilityRepoStub) ListActive(context.Context) ([]Account, error) {
	panic("unexpected call")
}
func (s *opsAccountAvailabilityRepoStub) ListByPlatform(context.Context, string) ([]Account, error) {
	panic("unexpected call")
}
func (s *opsAccountAvailabilityRepoStub) UpdateLastUsed(context.Context, int64) error {
	panic("unexpected call")
}
func (s *opsAccountAvailabilityRepoStub) BatchUpdateLastUsed(context.Context, map[int64]time.Time) error {
	panic("unexpected call")
}
func (s *opsAccountAvailabilityRepoStub) SetError(context.Context, int64, string) error {
	panic("unexpected call")
}
func (s *opsAccountAvailabilityRepoStub) ClearError(context.Context, int64) error {
	panic("unexpected call")
}
func (s *opsAccountAvailabilityRepoStub) SetSchedulable(context.Context, int64, bool) error {
	panic("unexpected call")
}
func (s *opsAccountAvailabilityRepoStub) AutoPauseExpiredAccounts(context.Context, time.Time) (int64, error) {
	panic("unexpected call")
}
func (s *opsAccountAvailabilityRepoStub) BindGroups(context.Context, int64, []int64) error {
	panic("unexpected call")
}
func (s *opsAccountAvailabilityRepoStub) ListSchedulable(context.Context) ([]Account, error) {
	panic("unexpected call")
}
func (s *opsAccountAvailabilityRepoStub) ListSchedulableByGroupID(context.Context, int64) ([]Account, error) {
	panic("unexpected call")
}
func (s *opsAccountAvailabilityRepoStub) ListSchedulableByPlatform(context.Context, string) ([]Account, error) {
	panic("unexpected call")
}
func (s *opsAccountAvailabilityRepoStub) ListSchedulableByGroupIDAndPlatform(context.Context, int64, string) ([]Account, error) {
	panic("unexpected call")
}
func (s *opsAccountAvailabilityRepoStub) ListSchedulableByPlatforms(context.Context, []string) ([]Account, error) {
	panic("unexpected call")
}
func (s *opsAccountAvailabilityRepoStub) ListSchedulableByGroupIDAndPlatforms(context.Context, int64, []string) ([]Account, error) {
	panic("unexpected call")
}
func (s *opsAccountAvailabilityRepoStub) ListSchedulableUngroupedByPlatform(context.Context, string) ([]Account, error) {
	panic("unexpected call")
}
func (s *opsAccountAvailabilityRepoStub) ListSchedulableUngroupedByPlatforms(context.Context, []string) ([]Account, error) {
	panic("unexpected call")
}
func (s *opsAccountAvailabilityRepoStub) SetRateLimited(context.Context, int64, time.Time) error {
	panic("unexpected call")
}
func (s *opsAccountAvailabilityRepoStub) SetModelRateLimit(context.Context, int64, string, time.Time) error {
	panic("unexpected call")
}
func (s *opsAccountAvailabilityRepoStub) SetOverloaded(context.Context, int64, time.Time) error {
	panic("unexpected call")
}
func (s *opsAccountAvailabilityRepoStub) SetTempUnschedulable(context.Context, int64, time.Time, string) error {
	panic("unexpected call")
}
func (s *opsAccountAvailabilityRepoStub) ClearTempUnschedulable(context.Context, int64) error {
	panic("unexpected call")
}
func (s *opsAccountAvailabilityRepoStub) ClearRateLimit(context.Context, int64) error {
	panic("unexpected call")
}
func (s *opsAccountAvailabilityRepoStub) ClearAntigravityQuotaScopes(context.Context, int64) error {
	panic("unexpected call")
}
func (s *opsAccountAvailabilityRepoStub) ClearModelRateLimits(context.Context, int64) error {
	panic("unexpected call")
}
func (s *opsAccountAvailabilityRepoStub) UpdateSessionWindow(context.Context, int64, *time.Time, *time.Time, string) error {
	panic("unexpected call")
}
func (s *opsAccountAvailabilityRepoStub) UpdateExtra(context.Context, int64, map[string]any) error {
	panic("unexpected call")
}
func (s *opsAccountAvailabilityRepoStub) BulkUpdate(context.Context, []int64, AccountBulkUpdate) (int64, error) {
	panic("unexpected call")
}
func (s *opsAccountAvailabilityRepoStub) IncrementQuotaUsed(context.Context, int64, float64) error {
	panic("unexpected call")
}
func (s *opsAccountAvailabilityRepoStub) ResetQuotaUsed(context.Context, int64) error {
	panic("unexpected call")
}

func TestOpsServiceGetAccountAvailabilityStats_ExposesTokenRefreshFailureFields(t *testing.T) {
	svc := &OpsService{
		cfg: &testConfigOpsEnabled,
		accountRepo: &opsAccountAvailabilityRepoStub{
			accounts: []Account{
				{
					ID:          101,
					Name:        "oauth-bad",
					Platform:    PlatformOpenAI,
					Status:      StatusError,
					Schedulable: true,
					Extra: map[string]any{
						"token_refresh_failure_reason": "token_revoked",
						"token_refresh_failure_class":  "permanent",
						"token_refresh_failed_at":      "2026-04-10T12:00:00Z",
					},
				},
			},
		},
	}

	platform, _, accounts, _, err := svc.GetAccountAvailabilityStats(context.Background(), "", nil)
	if err != nil {
		t.Fatalf("GetAccountAvailabilityStats error = %v", err)
	}
	if platform[PlatformOpenAI] == nil || platform[PlatformOpenAI].TokenRefreshFailureCount != 1 {
		t.Fatalf("expected platform token refresh failure count 1, got %#v", platform[PlatformOpenAI])
	}
	item := accounts[101]
	if item == nil {
		t.Fatal("expected account availability item")
	}
	if item.TokenRefreshFailureReason != "token_revoked" {
		t.Fatalf("TokenRefreshFailureReason = %q", item.TokenRefreshFailureReason)
	}
	if item.TokenRefreshFailureClass != "permanent" {
		t.Fatalf("TokenRefreshFailureClass = %q", item.TokenRefreshFailureClass)
	}
	if item.TokenRefreshFailedAt != "2026-04-10T12:00:00Z" {
		t.Fatalf("TokenRefreshFailedAt = %q", item.TokenRefreshFailedAt)
	}
}

func TestSummarizeTokenRefreshFailures(t *testing.T) {
	summary := SummarizeTokenRefreshFailures(map[int64]*AccountAvailability{
		10: {
			AccountID:                 10,
			TokenRefreshFailureReason: "token_revoked",
			TokenRefreshFailureClass:  "permanent",
		},
		11: {
			AccountID:                 11,
			TokenRefreshFailureReason: "refresh_token_reused",
			TokenRefreshFailureClass:  "permanent",
		},
		12: {
			AccountID:                 12,
			TokenRefreshFailureReason: "token_revoked",
			TokenRefreshFailureClass:  "temporary",
		},
		13: {
			AccountID: 13,
		},
	})
	if summary == nil {
		t.Fatal("expected summary")
	}
	if summary.TotalAccounts != 3 {
		t.Fatalf("expected total=3 got=%d", summary.TotalAccounts)
	}
	if summary.PermanentCount != 2 {
		t.Fatalf("expected permanent=2 got=%d", summary.PermanentCount)
	}
	if summary.ByReason["token_revoked"] != 2 {
		t.Fatalf("expected token_revoked=2 got=%d", summary.ByReason["token_revoked"])
	}
	if summary.ByClass["permanent"] != 2 {
		t.Fatalf("expected permanent class=2 got=%d", summary.ByClass["permanent"])
	}
	if len(summary.AffectedAccountIDs) != 3 {
		t.Fatalf("expected 3 affected accounts got=%d", len(summary.AffectedAccountIDs))
	}
}
