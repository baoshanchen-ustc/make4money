//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/domain"
)

// TestAccount_DeepSeekDefaultModelMapping 验证 DeepSeek 平台账号在未配置 model_mapping 时
// 自动回退到 DefaultDeepSeekModelMapping，将 Claude 各档位映射到 V4 Pro/Flash。
func TestAccount_DeepSeekDefaultModelMapping(t *testing.T) {
	tests := []struct {
		name           string
		requestedModel string
		accountMapping map[string]any // nil 表示不配置 model_mapping
		expectedModel  string
		expectedMatch  bool
	}{
		// 1. 默认映射 - Opus → V4 Pro
		{
			name:           "默认映射 - claude-opus-4-7 → deepseek-v4-pro",
			requestedModel: "claude-opus-4-7",
			expectedModel:  "deepseek-v4-pro",
			expectedMatch:  true,
		},
		{
			name:           "默认映射 - claude-opus-4-6 → deepseek-v4-pro",
			requestedModel: "claude-opus-4-6",
			expectedModel:  "deepseek-v4-pro",
			expectedMatch:  true,
		},
		{
			name:           "默认映射 - claude-opus-4-6-thinking → deepseek-v4-pro",
			requestedModel: "claude-opus-4-6-thinking",
			expectedModel:  "deepseek-v4-pro",
			expectedMatch:  true,
		},
		{
			name:           "默认映射 - claude-opus-4-5-thinking → deepseek-v4-pro",
			requestedModel: "claude-opus-4-5-thinking",
			expectedModel:  "deepseek-v4-pro",
			expectedMatch:  true,
		},

		// 2. 默认映射 - Sonnet → V4 Flash
		{
			name:           "默认映射 - claude-sonnet-4-6 → deepseek-v4-flash",
			requestedModel: "claude-sonnet-4-6",
			expectedModel:  "deepseek-v4-flash",
			expectedMatch:  true,
		},
		{
			name:           "默认映射 - claude-sonnet-4-5-20250929 → deepseek-v4-flash",
			requestedModel: "claude-sonnet-4-5-20250929",
			expectedModel:  "deepseek-v4-flash",
			expectedMatch:  true,
		},
		{
			name:           "默认映射 - claude-sonnet-4-5-thinking → deepseek-v4-flash",
			requestedModel: "claude-sonnet-4-5-thinking",
			expectedModel:  "deepseek-v4-flash",
			expectedMatch:  true,
		},

		// 3. 默认映射 - Haiku → V4 Flash（无小模型档）
		{
			name:           "默认映射 - claude-haiku-4-5 → deepseek-v4-flash",
			requestedModel: "claude-haiku-4-5",
			expectedModel:  "deepseek-v4-flash",
			expectedMatch:  true,
		},
		{
			name:           "默认映射 - claude-haiku-4-5-20251001 → deepseek-v4-flash",
			requestedModel: "claude-haiku-4-5-20251001",
			expectedModel:  "deepseek-v4-flash",
			expectedMatch:  true,
		},

		// 4. 旧版 Claude 兼容
		{
			name:           "旧版兼容 - claude-3-5-sonnet-20241022 → deepseek-v4-flash",
			requestedModel: "claude-3-5-sonnet-20241022",
			expectedModel:  "deepseek-v4-flash",
			expectedMatch:  true,
		},
		{
			name:           "旧版兼容 - claude-3-5-haiku-20241022 → deepseek-v4-flash",
			requestedModel: "claude-3-5-haiku-20241022",
			expectedModel:  "deepseek-v4-flash",
			expectedMatch:  true,
		},

		// 5. [1m] 后缀：strip 用于查 mapping，命中后拼回到 target
		{
			name:           "[1m] 拼回 - claude-opus-4-6[1m] → deepseek-v4-pro[1m]",
			requestedModel: "claude-opus-4-6[1m]",
			expectedModel:  "deepseek-v4-pro[1m]",
			expectedMatch:  true,
		},
		{
			name:           "[1m] 拼回 - claude-opus-4-7[1m] → deepseek-v4-pro[1m]",
			requestedModel: "claude-opus-4-7[1m]",
			expectedModel:  "deepseek-v4-pro[1m]",
			expectedMatch:  true,
		},
		{
			name:           "[1M] 大写拼回 - claude-sonnet-4-6[1M] → deepseek-v4-flash[1M]",
			requestedModel: "claude-sonnet-4-6[1M]",
			expectedModel:  "deepseek-v4-flash[1M]",
			expectedMatch:  true,
		},
		{
			name:           "[1m] 拼回 - claude-haiku-4-5[1m] → deepseek-v4-flash[1m]",
			requestedModel: "claude-haiku-4-5[1m]",
			expectedModel:  "deepseek-v4-flash[1m]",
			expectedMatch:  true,
		},

		// 6. 未在默认映射中 → 原样透传
		{
			name:           "未知模型 - claude-3-opus-20240229 透传",
			requestedModel: "claude-3-opus-20240229",
			expectedModel:  "claude-3-opus-20240229",
			expectedMatch:  false,
		},
		{
			name:           "未知模型 - gpt-4o 透传",
			requestedModel: "gpt-4o",
			expectedModel:  "gpt-4o",
			expectedMatch:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account := &Account{
				Platform: PlatformDeepSeek,
			}
			if tt.accountMapping != nil {
				account.Credentials = map[string]any{
					"model_mapping": tt.accountMapping,
				}
			}

			got, matched := account.ResolveMappedModel(tt.requestedModel)
			require.Equal(t, tt.expectedModel, got, "model: %s", tt.requestedModel)
			require.Equal(t, tt.expectedMatch, matched, "model: %s", tt.requestedModel)
		})
	}
}

// TestAccount_DeepSeekUserMappingOverridesDefault 验证账号自定义 model_mapping 优先于默认映射。
func TestAccount_DeepSeekUserMappingOverridesDefault(t *testing.T) {
	account := &Account{
		Platform: PlatformDeepSeek,
		Credentials: map[string]any{
			"model_mapping": map[string]any{
				"claude-opus-4-7": "deepseek-v4-flash", // 用户强制 Opus 走便宜的 Flash
			},
		},
	}

	got, matched := account.ResolveMappedModel("claude-opus-4-7")
	require.Equal(t, "deepseek-v4-flash", got)
	require.True(t, matched)

	// 未在自定义映射中的模型，按 ResolveMappedModel 现有契约返回原样（false）
	// 这与 Antigravity 一致：自定义映射存在时不再 fall through 到 Default
	got2, matched2 := account.ResolveMappedModel("claude-sonnet-4-6")
	require.Equal(t, "claude-sonnet-4-6", got2)
	require.False(t, matched2)
}

// TestAccount_DeepSeekContextSuffix_ExactUserMappingWins 验证用户精确写带 [1m] 的 mapping key 时，
// 不走 normalize 拼回逻辑，直接尊重用户精确映射。
func TestAccount_DeepSeekContextSuffix_ExactUserMappingWins(t *testing.T) {
	account := &Account{
		Platform: PlatformDeepSeek,
		Credentials: map[string]any{
			"model_mapping": map[string]any{
				"claude-opus-4-6[1m]": "deepseek-v4-flash", // 用户强制：1M 模式走 flash 省钱
			},
		},
	}
	got, matched := account.ResolveMappedModel("claude-opus-4-6[1m]")
	require.Equal(t, "deepseek-v4-flash", got, "精确命中时不应额外拼接 [1m]")
	require.True(t, matched)
}

// TestAccount_DeepSeekContextSuffix_NoDoubleSuffix 验证 target 已带 [1m] 时不重复拼接。
func TestAccount_DeepSeekContextSuffix_NoDoubleSuffix(t *testing.T) {
	account := &Account{
		Platform: PlatformDeepSeek,
		Credentials: map[string]any{
			"model_mapping": map[string]any{
				"claude-opus-4-6": "deepseek-v4-pro[1m]", // target 自带后缀
			},
		},
	}
	got, matched := account.ResolveMappedModel("claude-opus-4-6[1m]")
	require.Equal(t, "deepseek-v4-pro[1m]", got, "target 已带后缀时不应出现 [1m][1m]")
	require.True(t, matched)
}

// TestDefaultDeepSeekModelMapping_TargetsAreV4Only 验证默认映射的目标只指向官方支持的两个 V4 模型。
func TestDefaultDeepSeekModelMapping_TargetsAreV4Only(t *testing.T) {
	allowedTargets := map[string]struct{}{
		"deepseek-v4-pro":   {},
		"deepseek-v4-flash": {},
	}
	for from, to := range domain.DefaultDeepSeekModelMapping {
		_, ok := allowedTargets[to]
		require.Truef(t, ok, "%s → %s 目标不在 V4 白名单内", from, to)
	}
}
