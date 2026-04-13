package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestShouldBillByImageCount(t *testing.T) {
	t.Run("gemini image with image tokens uses token billing", func(t *testing.T) {
		result := &ForwardResult{
			ImageCount: 1,
			Usage: ClaudeUsage{
				ImageOutputTokens: 128,
			},
		}
		require.False(t, shouldBillByImageCount(result))
	})

	t.Run("image response without image tokens falls back to image billing", func(t *testing.T) {
		result := &ForwardResult{
			ImageCount: 1,
		}
		require.True(t, shouldBillByImageCount(result))
	})

	t.Run("no generated image never uses image billing", func(t *testing.T) {
		result := &ForwardResult{
			ImageCount: 0,
			Usage: ClaudeUsage{
				ImageOutputTokens: 128,
			},
		}
		require.False(t, shouldBillByImageCount(result))
	})
}

func TestResolveBillingMode_PrefersTokenWhenImageTokensPresent(t *testing.T) {
	mode := resolveBillingMode(&ForwardResult{
		ImageCount: 1,
		Usage: ClaudeUsage{
			ImageOutputTokens: 64,
		},
	}, nil)

	require.NotNil(t, mode)
	require.Equal(t, string(BillingModeToken), *mode)
}

