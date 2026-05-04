package service

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

// TestIsFirstPartyAnthropicMessagesURL 覆盖 helper 的核心边界。
// 一旦放宽该函数，关键的安全前提（"代理只在 first-party 上注入 first-party 标识"）就会被破坏。
func TestIsFirstPartyAnthropicMessagesURL(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"messages happy path", "https://api.anthropic.com/v1/messages", true},
		{"messages with query", "https://api.anthropic.com/v1/messages?beta=true", true},
		{"count_tokens happy path", "https://api.anthropic.com/v1/messages/count_tokens", true},
		{"count_tokens with query", "https://api.anthropic.com/v1/messages/count_tokens?beta=true&proxy=foo", true},
		{"http rejected", "http://api.anthropic.com/v1/messages", false},
		{"host suffix attack", "https://api.anthropic.com.evil/v1/messages", false},
		{"host prefix attack", "https://evil.api.anthropic.com/v1/messages", false},
		{"hyphen variant", "https://api-anthropic.com/v1/messages", false},
		{"different host", "https://example.com/v1/messages", false},
		{"messages subpath", "https://api.anthropic.com/v1/messages/extra", false},
		{"models endpoint", "https://api.anthropic.com/v1/models", false},
		{"empty string", "", false},
		{"malformed", ":://garbage", false},
		{"trailing slash", "https://api.anthropic.com/v1/messages/", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isFirstPartyAnthropicMessagesURL(tc.in)
			if got != tc.want {
				t.Fatalf("isFirstPartyAnthropicMessagesURL(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

// TestIsFirstPartyAnthropicMessagesURL_HostCaseInsensitive
// host 比较走 EqualFold；上游可能返回大小写混写的 host。
func TestIsFirstPartyAnthropicMessagesURL_HostCaseInsensitive(t *testing.T) {
	cases := []string{
		"https://API.ANTHROPIC.COM/v1/messages",
		"https://Api.Anthropic.Com/v1/messages/count_tokens?beta=true",
	}
	for _, in := range cases {
		if !isFirstPartyAnthropicMessagesURL(in) {
			t.Errorf("isFirstPartyAnthropicMessagesURL(%q) = false, want true", in)
		}
	}
}

// TestEnsureClaudeFirstPartyRequestID_OAuthEnabled 默认 OAuth 路径在 first-party 上自动生成。
func TestEnsureClaudeFirstPartyRequestID_OAuthEnabled(t *testing.T) {
	req := newRequest(t, "https://api.anthropic.com/v1/messages")
	cfg := config.ClaudeRequestIDConfig{AutoGenerateOAuth: true}

	generated := ensureClaudeFirstPartyRequestID(req, "https://api.anthropic.com/v1/messages", cfg, "oauth")
	if !generated {
		t.Fatal("expected auto-generation on first-party OAuth")
	}
	if v := getHeaderRaw(req.Header, "x-client-request-id"); v == "" {
		t.Fatalf("x-client-request-id should be set; header=%v", req.Header)
	}
}

// TestEnsureClaudeFirstPartyRequestID_OAuthDisabled OAuth 开关关闭时不生成。
func TestEnsureClaudeFirstPartyRequestID_OAuthDisabled(t *testing.T) {
	req := newRequest(t, "https://api.anthropic.com/v1/messages")
	cfg := config.ClaudeRequestIDConfig{AutoGenerateOAuth: false}

	generated := ensureClaudeFirstPartyRequestID(req, "https://api.anthropic.com/v1/messages", cfg, "oauth")
	if generated {
		t.Fatal("expected NO auto-generation when OAuth switch is off")
	}
	if v := getHeaderRaw(req.Header, "x-client-request-id"); v != "" {
		t.Fatalf("x-client-request-id should be empty when disabled; got %q", v)
	}
}

// TestEnsureClaudeFirstPartyRequestID_PassthroughOff API key passthrough 默认关闭。
func TestEnsureClaudeFirstPartyRequestID_PassthroughOff(t *testing.T) {
	req := newRequest(t, "https://api.anthropic.com/v1/messages")
	// 默认 cfg：AutoGenerateAPIKeyPassthrough=false（即使 OAuth 是 true）
	cfg := config.ClaudeRequestIDConfig{AutoGenerateOAuth: true, AutoGenerateAPIKeyPassthrough: false}

	if generated := ensureClaudeFirstPartyRequestID(req, "https://api.anthropic.com/v1/messages", cfg, "api_key"); generated {
		t.Fatal("expected NO auto-generation for api_key passthrough by default")
	}
	if v := getHeaderRaw(req.Header, "x-client-request-id"); v != "" {
		t.Fatalf("x-client-request-id should be empty for default passthrough; got %q", v)
	}
}

// TestEnsureClaudeFirstPartyRequestID_PassthroughExplicitOn 管理员显式启用后才会生成。
func TestEnsureClaudeFirstPartyRequestID_PassthroughExplicitOn(t *testing.T) {
	req := newRequest(t, "https://api.anthropic.com/v1/messages")
	cfg := config.ClaudeRequestIDConfig{AutoGenerateAPIKeyPassthrough: true}

	if generated := ensureClaudeFirstPartyRequestID(req, "https://api.anthropic.com/v1/messages", cfg, "api_key"); !generated {
		t.Fatal("expected auto-generation when AutoGenerateAPIKeyPassthrough=true")
	}
}

// TestEnsureClaudeFirstPartyRequestID_NotFirstParty 第三方域 / custom relay 永不触发。
func TestEnsureClaudeFirstPartyRequestID_NotFirstParty(t *testing.T) {
	cases := []string{
		"https://example.com/v1/messages",
		"http://api.anthropic.com/v1/messages",
		"https://api.anthropic.com.evil/v1/messages",
		"https://api.anthropic.com/v1/messages/extra",
		"https://api.anthropic.com/v1/models",
	}
	cfg := config.ClaudeRequestIDConfig{AutoGenerateOAuth: true, AutoGenerateAPIKeyPassthrough: true}
	for _, target := range cases {
		t.Run(target, func(t *testing.T) {
			req := newRequest(t, target)
			if ensureClaudeFirstPartyRequestID(req, target, cfg, "oauth") {
				t.Fatalf("must not auto-generate for non-first-party URL %q", target)
			}
			if v := getHeaderRaw(req.Header, "x-client-request-id"); v != "" {
				t.Fatalf("x-client-request-id should remain empty for %q; got %q", target, v)
			}
		})
	}
}

// TestEnsureClaudeFirstPartyRequestID_PreservesClientSupplied
// 客户端已带 x-client-request-id 时，代理不应覆盖。
func TestEnsureClaudeFirstPartyRequestID_PreservesClientSupplied(t *testing.T) {
	req := newRequest(t, "https://api.anthropic.com/v1/messages")
	const supplied = "client-supplied-uuid-1234"
	setHeaderRaw(req.Header, "x-client-request-id", supplied)
	cfg := config.ClaudeRequestIDConfig{AutoGenerateOAuth: true}

	if ensureClaudeFirstPartyRequestID(req, "https://api.anthropic.com/v1/messages", cfg, "oauth") {
		t.Fatal("must not overwrite client-supplied x-client-request-id")
	}
	if got := getHeaderRaw(req.Header, "x-client-request-id"); got != supplied {
		t.Fatalf("client-supplied id changed from %q to %q", supplied, got)
	}
}

// TestEnsureClaudeFirstPartyRequestID_PreservesViaCanonicalCase
// Go canonical 形式的 X-Client-Request-Id 也应被识别为已存在。
func TestEnsureClaudeFirstPartyRequestID_PreservesViaCanonicalCase(t *testing.T) {
	req := newRequest(t, "https://api.anthropic.com/v1/messages")
	// 用 canonical 形式（http.Header.Set 会自动转 canonical）
	req.Header.Set("X-Client-Request-Id", "canonical-uuid")
	cfg := config.ClaudeRequestIDConfig{AutoGenerateOAuth: true}

	if ensureClaudeFirstPartyRequestID(req, "https://api.anthropic.com/v1/messages", cfg, "oauth") {
		t.Fatal("must detect canonical-cased x-client-request-id and skip generation")
	}
}

// TestEnsureClaudeFirstPartyRequestID_CountTokens count_tokens path 与 messages 同等。
func TestEnsureClaudeFirstPartyRequestID_CountTokens(t *testing.T) {
	const target = "https://api.anthropic.com/v1/messages/count_tokens?beta=true"
	req := newRequest(t, target)
	cfg := config.ClaudeRequestIDConfig{AutoGenerateOAuth: true}

	if !ensureClaudeFirstPartyRequestID(req, target, cfg, "oauth") {
		t.Fatal("expected auto-generation on count_tokens first-party")
	}
}

// TestShouldAutoGenerateClaudeRequestID_TokenTypeNormalization
// 未识别 / 空 tokenType 走 AutoGenerateAPIKeyPassthrough 通道，避免 OAuth 开关被错误命中。
func TestShouldAutoGenerateClaudeRequestID_TokenTypeNormalization(t *testing.T) {
	req := newRequest(t, "https://api.anthropic.com/v1/messages")
	cfg := config.ClaudeRequestIDConfig{AutoGenerateOAuth: true, AutoGenerateAPIKeyPassthrough: false}

	cases := []string{"", "API_KEY", "bedrock", "unknown"}
	for _, tt := range cases {
		got := shouldAutoGenerateClaudeRequestID(req, "https://api.anthropic.com/v1/messages", cfg, tt)
		if got {
			t.Errorf("tokenType=%q should not pass when AutoGenerateAPIKeyPassthrough=false; got generated=true", tt)
		}
	}
	// "OAuth" 大写也应识别（通过 ToLower / TrimSpace 归一化）
	if !shouldAutoGenerateClaudeRequestID(req, "https://api.anthropic.com/v1/messages", cfg, "  OAuth  ") {
		t.Error(`tokenType="  OAuth  " should be normalized to "oauth" and trigger autogen`)
	}
}

func newRequest(t *testing.T, target string) *http.Request {
	t.Helper()
	u, err := url.Parse(target)
	if err != nil {
		t.Fatalf("parse url %q: %v", target, err)
	}
	return &http.Request{
		URL:    u,
		Header: http.Header{},
	}
}
