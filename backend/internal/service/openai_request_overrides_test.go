package service

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestAccountGetOpenAIRequestOverrides(t *testing.T) {
	t.Run("returns overrides for openai account", func(t *testing.T) {
		account := &Account{
			Platform: PlatformOpenAI,
			Extra: map[string]any{
				openAIRequestOverridesExtraKey: map[string]any{
					"service_tier": "fast",
				},
			},
		}

		got := account.GetOpenAIRequestOverrides()
		require.Equal(t, "fast", got["service_tier"])
	})

	t.Run("strips disallowed top level model override", func(t *testing.T) {
		account := &Account{
			Platform: PlatformOpenAI,
			Extra: map[string]any{
				openAIRequestOverridesExtraKey: map[string]any{
					"model":        "gpt-5.4",
					"service_tier": "fast",
				},
			},
		}

		got := account.GetOpenAIRequestOverrides()
		require.NotNil(t, got)
		require.Equal(t, "fast", got["service_tier"])
		_, exists := got["model"]
		require.False(t, exists)
	})

	t.Run("ignores non-object overrides", func(t *testing.T) {
		account := &Account{
			Platform: PlatformOpenAI,
			Extra: map[string]any{
				openAIRequestOverridesExtraKey: "fast",
			},
		}

		require.Nil(t, account.GetOpenAIRequestOverrides())
	})
}

func TestApplyOpenAIRequestOverridesToBody(t *testing.T) {
	account := &Account{
		Platform: PlatformOpenAI,
		Extra: map[string]any{
			openAIRequestOverridesExtraKey: map[string]any{
				"service_tier": "fast",
				"reasoning": map[string]any{
					"effort": "high",
				},
			},
		},
	}

	body := []byte(`{"model":"gpt-5.2","reasoning":{"summary":"auto"},"input":[{"type":"text","text":"hi"}]}`)
	got, modified, err := applyOpenAIRequestOverridesToBody(body, account)
	require.NoError(t, err)
	require.True(t, modified)
	require.Equal(t, "fast", gjson.GetBytes(got, "service_tier").String())
	require.Equal(t, "high", gjson.GetBytes(got, "reasoning.effort").String())
	require.Equal(t, "auto", gjson.GetBytes(got, "reasoning.summary").String())
	require.Equal(t, "hi", gjson.GetBytes(got, "input.0.text").String())
}

func TestApplyOpenAIRequestOverridesToBody_NoOverrideChange(t *testing.T) {
	account := &Account{
		Platform: PlatformOpenAI,
		Extra: map[string]any{
			openAIRequestOverridesExtraKey: map[string]any{
				"service_tier": "fast",
			},
		},
	}

	body := []byte(`{"service_tier":"fast"}`)
	got, modified, err := applyOpenAIRequestOverridesToBody(body, account)
	require.NoError(t, err)
	require.False(t, modified)
	require.Equal(t, body, got)
}

func TestApplyOpenAIRequestOverridesToBody_IgnoresModelOverride(t *testing.T) {
	account := &Account{
		Platform: PlatformOpenAI,
		Extra: map[string]any{
			openAIRequestOverridesExtraKey: map[string]any{
				"model":        "gpt-5.4",
				"service_tier": "fast",
			},
		},
	}

	body := []byte(`{"model":"gpt-5.2"}`)
	got, modified, err := applyOpenAIRequestOverridesToBody(body, account)
	require.NoError(t, err)
	require.True(t, modified)
	require.Equal(t, "gpt-5.2", gjson.GetBytes(got, "model").String())
	require.Equal(t, "fast", gjson.GetBytes(got, "service_tier").String())
}
