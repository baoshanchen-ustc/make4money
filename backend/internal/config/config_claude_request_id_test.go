package config

import (
	"testing"
)

// TestLoadDefaultClaudeRequestIDConfig 验证 first-party x-client-request-id 自动生成的默认值。
// 默认策略：OAuth 路径开启（贴合真实 CLI），API key passthrough 关闭（保持透明语义）。
func TestLoadDefaultClaudeRequestIDConfig(t *testing.T) {
	resetViperWithJWTSecret(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if !cfg.Gateway.ClaudeRequestID.AutoGenerateOAuth {
		t.Fatalf("Gateway.ClaudeRequestID.AutoGenerateOAuth = false, want true (default for OAuth normal path)")
	}
	if cfg.Gateway.ClaudeRequestID.AutoGenerateAPIKeyPassthrough {
		t.Fatalf("Gateway.ClaudeRequestID.AutoGenerateAPIKeyPassthrough = true, want false (passthrough should stay transparent)")
	}
}

// TestLoadClaudeRequestIDConfigFromEnv 验证 env 覆盖默认值的能力，作为回滚开关闭环测试。
func TestLoadClaudeRequestIDConfigFromEnv(t *testing.T) {
	resetViperWithJWTSecret(t)
	t.Setenv("GATEWAY_CLAUDE_REQUEST_ID_AUTO_GENERATE_OAUTH", "false")
	t.Setenv("GATEWAY_CLAUDE_REQUEST_ID_AUTO_GENERATE_API_KEY_PASSTHROUGH", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Gateway.ClaudeRequestID.AutoGenerateOAuth {
		t.Fatalf("Gateway.ClaudeRequestID.AutoGenerateOAuth should be overridden to false via env, got true")
	}
	if !cfg.Gateway.ClaudeRequestID.AutoGenerateAPIKeyPassthrough {
		t.Fatalf("Gateway.ClaudeRequestID.AutoGenerateAPIKeyPassthrough should be overridden to true via env, got false")
	}
}

// TestLoadDefaultResponseHeaderGatewayTraceOverride 确认危险 override 默认关闭。
func TestLoadDefaultResponseHeaderGatewayTraceOverride(t *testing.T) {
	resetViperWithJWTSecret(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Security.ResponseHeaders.AllowGatewayTraceHeaders {
		t.Fatalf("Security.ResponseHeaders.AllowGatewayTraceHeaders = true, want false (dangerous override must default off)")
	}
}

// TestLoadResponseHeaderGatewayTraceOverrideFromEnv 验证 env 可以开启危险 override（仅诊断使用）。
func TestLoadResponseHeaderGatewayTraceOverrideFromEnv(t *testing.T) {
	resetViperWithJWTSecret(t)
	t.Setenv("SECURITY_RESPONSE_HEADERS_ALLOW_GATEWAY_TRACE_HEADERS", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if !cfg.Security.ResponseHeaders.AllowGatewayTraceHeaders {
		t.Fatalf("Security.ResponseHeaders.AllowGatewayTraceHeaders should be overridden to true via env, got false")
	}
}
