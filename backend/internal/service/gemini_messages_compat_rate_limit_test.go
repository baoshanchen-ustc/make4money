package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type geminiRateLimitRepo struct {
	stubOpenAIAccountRepo
	rateLimitedAccountID int64
	rateLimitedUntil     *time.Time
}

func (r *geminiRateLimitRepo) SetRateLimited(_ context.Context, id int64, resetAt time.Time) error {
	r.rateLimitedAccountID = id
	t := resetAt
	r.rateLimitedUntil = &t
	return nil
}

func TestHandleGeminiUpstreamError_GoogleOne429UsesTierCooldown(t *testing.T) {
	t.Parallel()

	repo := &geminiRateLimitRepo{}
	svc := &GeminiMessagesCompatService{
		accountRepo: repo,
	}
	account := &Account{
		ID:       41,
		Platform: PlatformGemini,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"oauth_type": "google_one",
			"tier_id":    GeminiTierGoogleOneFree,
		},
	}

	before := time.Now()
	svc.handleGeminiUpstreamError(context.Background(), account, 429, nil, []byte(`{"error":{"code":429,"message":"rate limited"}}`))
	after := time.Now()

	require.Equal(t, account.ID, repo.rateLimitedAccountID)
	require.NotNil(t, repo.rateLimitedUntil)

	cooldown := geminiCooldownForTier(GeminiTierGoogleOneFree)
	lowerBound := before.Add(cooldown - 2*time.Second)
	upperBound := after.Add(cooldown + 2*time.Second)
	require.False(t, repo.rateLimitedUntil.Before(lowerBound), "google_one cooldown should use tier-based fallback")
	require.False(t, repo.rateLimitedUntil.After(upperBound), "google_one cooldown should not jump to daily reset")
}

func TestHandleGeminiUpstreamError_AIStudio429FallsBackToDailyReset(t *testing.T) {
	t.Parallel()

	repo := &geminiRateLimitRepo{}
	svc := &GeminiMessagesCompatService{
		accountRepo: repo,
	}
	account := &Account{
		ID:       42,
		Platform: PlatformGemini,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"oauth_type": "ai_studio",
			"tier_id":    GeminiTierAIStudioFree,
		},
	}

	expectedResetUnix := nextGeminiDailyResetUnix()
	require.NotNil(t, expectedResetUnix)

	svc.handleGeminiUpstreamError(context.Background(), account, 429, nil, []byte(`{"error":{"code":429,"message":"rate limited"}}`))

	require.Equal(t, account.ID, repo.rateLimitedAccountID)
	require.NotNil(t, repo.rateLimitedUntil)
	require.Equal(t, *expectedResetUnix, repo.rateLimitedUntil.Unix())
}
