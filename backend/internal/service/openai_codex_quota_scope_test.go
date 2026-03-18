//go:build unit

package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestResolveOpenAICodexModelRateLimitKey(t *testing.T) {
	require.Equal(t,
		openAICodexModelRateLimitKeySpark,
		resolveOpenAICodexModelRateLimitKey("gpt-5.3-codex-spark", "gpt-5.3-codex"),
	)
	require.Equal(t,
		openAICodexModelRateLimitKeySpark,
		resolveOpenAICodexModelRateLimitKey("gpt-5.3-codex-spark-high", "gpt-5.3-codex"),
	)
	require.Equal(t,
		openAICodexModelRateLimitKeyCodex,
		resolveOpenAICodexModelRateLimitKey("gpt-5.3-codex", "gpt-5.3-codex"),
	)
	require.Equal(t,
		openAICodexModelRateLimitKeyCodex,
		resolveOpenAICodexModelRateLimitKey("gpt-5.4", "gpt-5.4"),
	)
	require.Equal(t,
		"",
		resolveOpenAICodexModelRateLimitKey("text-embedding-3-large", "text-embedding-3-large"),
	)
}

func TestOpenAICodexGlobalRateLimitResetAtFromExtra(t *testing.T) {
	now := time.Now().UTC()
	codexReset := now.Add(2 * time.Hour).UTC()
	sparkReset := now.Add(1 * time.Hour).UTC()

	t.Run("both_supported_only_codex_exhausted_returns_nil", func(t *testing.T) {
		account := &Account{
			Platform: PlatformOpenAI,
			Credentials: map[string]any{
				"model_mapping": map[string]any{
					"gpt-5.3-codex":       "gpt-5.3-codex",
					"gpt-5.3-codex-spark": "gpt-5.3-codex",
				},
			},
			Extra: map[string]any{
				"codex_7d_used_percent": 100.0,
				"codex_7d_reset_at":     codexReset.Format(time.RFC3339),
			},
		}
		require.Nil(t, openAICodexGlobalRateLimitResetAtFromExtra(account, now))
	})

	t.Run("both_supported_both_exhausted_returns_earliest_reset", func(t *testing.T) {
		account := &Account{
			Platform: PlatformOpenAI,
			Credentials: map[string]any{
				"model_mapping": map[string]any{
					"gpt-5.3-codex":       "gpt-5.3-codex",
					"gpt-5.3-codex-spark": "gpt-5.3-codex",
				},
			},
			Extra: map[string]any{
				"codex_7d_used_percent":       100.0,
				"codex_7d_reset_at":           codexReset.Format(time.RFC3339),
				"codex_spark_7d_used_percent": 100.0,
				"codex_spark_7d_reset_at":     sparkReset.Format(time.RFC3339),
			},
		}
		resetAt := openAICodexGlobalRateLimitResetAtFromExtra(account, now)
		require.NotNil(t, resetAt)
		require.WithinDuration(t, sparkReset, *resetAt, time.Second)
	})

	t.Run("codex_only_account_uses_codex_snapshot", func(t *testing.T) {
		account := &Account{
			Platform: PlatformOpenAI,
			Credentials: map[string]any{
				"model_mapping": map[string]any{
					"gpt-5.3-codex": "gpt-5.3-codex",
				},
			},
			Extra: map[string]any{
				"codex_7d_used_percent": 100.0,
				"codex_7d_reset_at":     codexReset.Format(time.RFC3339),
			},
		}
		resetAt := openAICodexGlobalRateLimitResetAtFromExtra(account, now)
		require.NotNil(t, resetAt)
		require.WithinDuration(t, codexReset, *resetAt, time.Second)
	})
}

