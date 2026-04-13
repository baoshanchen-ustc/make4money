//go:build unit

package service

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGatewayShouldRetryUpstreamError(t *testing.T) {
	svc := &GatewayService{}

	t.Run("oauth_only_retries_403", func(t *testing.T) {
		account := &Account{Type: AccountTypeOAuth, Platform: PlatformAnthropic}
		require.True(t, svc.shouldRetryUpstreamError(account, http.StatusForbidden))
		require.False(t, svc.shouldRetryUpstreamError(account, http.StatusServiceUnavailable))
	})

	t.Run("custom_error_code_miss_only_retries_transient_statuses", func(t *testing.T) {
		account := &Account{
			Type:     AccountTypeAPIKey,
			Platform: PlatformAnthropic,
			Credentials: map[string]any{
				"custom_error_codes_enabled": true,
				"custom_error_codes":         []any{float64(http.StatusTooManyRequests)},
			},
		}

		require.True(t, svc.shouldRetryUpstreamError(account, http.StatusServiceUnavailable))
		require.False(t, svc.shouldRetryUpstreamError(account, http.StatusNotFound))
	})

	t.Run("custom_error_code_hit_keeps_failover_path", func(t *testing.T) {
		account := &Account{
			Type:     AccountTypeAPIKey,
			Platform: PlatformAnthropic,
			Credentials: map[string]any{
				"custom_error_codes_enabled": true,
				"custom_error_codes":         []any{float64(http.StatusServiceUnavailable)},
			},
		}

		require.False(t, svc.shouldRetryUpstreamError(account, http.StatusServiceUnavailable))
	})

	t.Run("default_api_key_accounts_do_not_add_new_same_account_retries", func(t *testing.T) {
		account := &Account{Type: AccountTypeAPIKey, Platform: PlatformAnthropic}
		require.False(t, svc.shouldRetryUpstreamError(account, http.StatusServiceUnavailable))
	})
}

func TestParseRetryAfterHeader(t *testing.T) {
	now := time.Date(2026, time.April, 12, 10, 0, 0, 0, time.UTC)

	require.Equal(t, 2*time.Second, parseRetryAfterHeader("2", now))
	require.Equal(t, 0*time.Second, parseRetryAfterHeader("0", now))
	require.Equal(t, 0*time.Second, parseRetryAfterHeader("invalid", now))
	require.Equal(
		t,
		1500*time.Millisecond,
		parseRetryAfterHeader("Sun, 12 Apr 2026 10:00:01 GMT", time.Date(2026, time.April, 12, 9, 59, 59, 500000000, time.UTC)),
	)
}

func TestRetryBackoffDelayForResponse(t *testing.T) {
	resp := &http.Response{
		Header: http.Header{
			"Retry-After": []string{"2"},
		},
	}
	require.Equal(t, 2*time.Second, retryBackoffDelayForResponse(resp, 1))

	resp.Header.Set("Retry-After", "30")
	require.Equal(t, retryMaxDelay, retryBackoffDelayForResponse(resp, 1))

	resp.Header.Set("Retry-After", "invalid")
	require.Equal(t, retryBackoffDelay(2), retryBackoffDelayForResponse(resp, 2))
}
