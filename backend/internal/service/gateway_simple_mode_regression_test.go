package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type simpleModeMixedGroupAwareRepo struct {
	stubOpenAIAccountRepo
	groupedCalls  int
	platformCalls int
}

func (r *simpleModeMixedGroupAwareRepo) ListSchedulableByPlatforms(_ context.Context, platforms []string) ([]Account, error) {
	r.platformCalls++
	platformSet := make(map[string]struct{}, len(platforms))
	for _, platform := range platforms {
		platformSet[platform] = struct{}{}
	}
	result := make([]Account, 0, len(r.accounts))
	for _, acc := range r.accounts {
		if _, ok := platformSet[acc.Platform]; ok && acc.IsSchedulable() {
			result = append(result, acc)
		}
	}
	return result, nil
}

func (r *simpleModeMixedGroupAwareRepo) ListSchedulableByGroupIDAndPlatforms(_ context.Context, groupID int64, platforms []string) ([]Account, error) {
	r.groupedCalls++
	platformSet := make(map[string]struct{}, len(platforms))
	for _, platform := range platforms {
		platformSet[platform] = struct{}{}
	}
	result := make([]Account, 0, len(r.accounts))
	for _, acc := range r.accounts {
		if _, ok := platformSet[acc.Platform]; !ok || !acc.IsSchedulable() {
			continue
		}
		for _, ag := range acc.AccountGroups {
			if ag.GroupID == groupID {
				result = append(result, acc)
				break
			}
		}
	}
	return result, nil
}

func TestGatewayService_SelectAccountWithMixedScheduling_SimpleModeIgnoresGroupFilter(t *testing.T) {
	t.Parallel()

	groupID := int64(100)
	repo := &simpleModeMixedGroupAwareRepo{
		stubOpenAIAccountRepo: stubOpenAIAccountRepo{
			accounts: []Account{
				{
					ID:          1,
					Platform:    PlatformAnthropic,
					Priority:    1,
					Status:      StatusActive,
					Schedulable: true,
				},
				{
					ID:          2,
					Platform:    PlatformAntigravity,
					Priority:    2,
					Status:      StatusActive,
					Schedulable: true,
					Extra: map[string]any{
						"mixed_scheduling": true,
					},
				},
			},
		},
	}

	svc := &GatewayService{
		accountRepo: repo,
		cache:       &stubGatewayCache{},
		cfg:         &config.Config{RunMode: config.RunModeSimple},
	}

	account, err := svc.selectAccountWithMixedScheduling(context.Background(), &groupID, "", "", nil, PlatformAnthropic)
	require.NoError(t, err)
	require.NotNil(t, account)
	require.Equal(t, int64(1), account.ID)
	require.Equal(t, 1, repo.platformCalls)
	require.Zero(t, repo.groupedCalls)
}
