package service

import "testing"

// TestGatewayServiceIsModelSupportedByAccount_CopilotNoMappingAllowsAll verifies
// that a Copilot account with no model_mapping accepts every model.
func TestGatewayServiceIsModelSupportedByAccount_CopilotNoMappingAllowsAll(t *testing.T) {
	svc := &GatewayService{}
	account := &Account{
		Platform:    PlatformCopilot,
		Credentials: map[string]any{},
	}

	for _, model := range []string{"claude-sonnet-4.6", "gpt-4o", "claude-sonnet-4-5", "claude-opus-4-6"} {
		if !svc.isModelSupportedByAccount(account, model) {
			t.Fatalf("expected model %q to be supported when model_mapping is empty", model)
		}
	}
}

// TestGatewayServiceIsModelSupportedByAccount_CopilotMappingDoesNotActAsAllowlist verifies
// that adding a model_mapping to a Copilot account does NOT restrict which models
// can be routed to it. The mapping only controls upstream model name rewriting.
func TestGatewayServiceIsModelSupportedByAccount_CopilotMappingDoesNotActAsAllowlist(t *testing.T) {
	svc := &GatewayService{}
	account := &Account{
		Platform: PlatformCopilot,
		Credentials: map[string]any{
			"model_mapping": map[string]any{
				"claude-sonnet-4-5": "claude-sonnet-4.6",
				"claude-sonnet-4.5": "claude-sonnet-4.6",
			},
		},
	}

	// These models are NOT in the mapping keys but must still be routable.
	for _, model := range []string{"claude-sonnet-4.6", "gpt-4o", "claude-opus-4-6", "claude-haiku-4.5"} {
		if !svc.isModelSupportedByAccount(account, model) {
			t.Fatalf("model %q should be supported even though it is not a model_mapping key; "+
				"Copilot model_mapping must not act as an allowlist", model)
		}
	}

	// The mapped models themselves should also be routable.
	for _, model := range []string{"claude-sonnet-4-5", "claude-sonnet-4.5"} {
		if !svc.isModelSupportedByAccount(account, model) {
			t.Fatalf("model %q (a mapping key) should be supported", model)
		}
	}
}

// TestGatewayServiceIsModelSupportedByAccount_CopilotWhitelistFilters 验证
// 当账号设置了 model_whitelist 时，只有白名单内的模型被允许。
func TestGatewayServiceIsModelSupportedByAccount_CopilotWhitelistFilters(t *testing.T) {
	svc := &GatewayService{}
	account := &Account{
		Platform: PlatformCopilot,
		Credentials: map[string]any{
			"model_whitelist": []any{"claude-sonnet-4.6", "gpt-4o"},
		},
	}
	allowed := []string{"claude-sonnet-4.6", "gpt-4o"}
	blocked := []string{"claude-opus-4.6", "claude-haiku-4.5"}
	for _, m := range allowed {
		if !svc.isModelSupportedByAccount(account, m) {
			t.Errorf("model %q should be allowed by whitelist", m)
		}
	}
	for _, m := range blocked {
		if svc.isModelSupportedByAccount(account, m) {
			t.Errorf("model %q should be blocked by whitelist", m)
		}
	}
}

// TestGatewayServiceIsModelSupportedByAccount_CopilotEmptyWhitelistAllowsAll 验证
// 当账号 model_whitelist 为空时允许所有模型。
func TestGatewayServiceIsModelSupportedByAccount_CopilotEmptyWhitelistAllowsAll(t *testing.T) {
	svc := &GatewayService{}
	account := &Account{
		Platform:    PlatformCopilot,
		Credentials: map[string]any{
			// model_whitelist 未设置
		},
	}
	for _, m := range []string{"claude-sonnet-4.6", "gpt-4o", "claude-opus-4.6"} {
		if !svc.isModelSupportedByAccount(account, m) {
			t.Errorf("model %q should be allowed when whitelist is empty", m)
		}
	}
}

// TestCopilotWhitelistContains_DashDotNormalization 验证 copilotWhitelistContains
// 对 Claude 模型 dash/dot 两种 ID 格式做归一化后可以互认，避免因格式不一致导致误拦。
func TestCopilotWhitelistContains_DashDotNormalization(t *testing.T) {
	tests := []struct {
		name      string
		whitelist []string
		model     string
		want      bool
	}{
		{
			name:      "whitelist存dot，请求用dash",
			whitelist: []string{"claude-sonnet-4.6", "gpt-4o"},
			model:     "claude-sonnet-4-6",
			want:      true,
		},
		{
			name:      "whitelist存dash，请求用dot",
			whitelist: []string{"claude-sonnet-4-6", "gpt-4o"},
			model:     "claude-sonnet-4.6",
			want:      true,
		},
		{
			name:      "opus dash/dot互认",
			whitelist: []string{"claude-opus-4.6"},
			model:     "claude-opus-4-6",
			want:      true,
		},
		{
			name:      "不在白名单中的模型被正确拦截",
			whitelist: []string{"claude-sonnet-4.6", "gpt-4o"},
			model:     "claude-opus-4-6",
			want:      false,
		},
		{
			name:      "非Claude模型精确匹配",
			whitelist: []string{"gpt-4o", "gpt-4.1"},
			model:     "gpt-4o",
			want:      true,
		},
		{
			name:      "非Claude模型不在白名单",
			whitelist: []string{"gpt-4o"},
			model:     "gpt-4.1",
			want:      false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := copilotWhitelistContains(tt.whitelist, tt.model)
			if got != tt.want {
				t.Errorf("copilotWhitelistContains(%v, %q) = %v, want %v", tt.whitelist, tt.model, got, tt.want)
			}
		})
	}
}
