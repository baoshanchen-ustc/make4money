package service

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// TestSafeHeaderValueForLogRedactsAuthorization 既有行为回归。
func TestSafeHeaderValueForLogRedactsAuthorization(t *testing.T) {
	cases := []struct {
		key, in, want string
	}{
		{"authorization", "Bearer abc123", "Bearer [redacted]"},
		{"Authorization", "Bearer abc123", "Bearer [redacted]"}, // 大小写
		{"x-api-key", "sk-zzz...", "[redacted]"},
		{"X-API-KEY", "sk-zzz...", "[redacted]"},
	}
	for _, tc := range cases {
		got := safeHeaderValueForLog(tc.key, tc.in)
		if got != tc.want {
			t.Errorf("safeHeaderValueForLog(%q, %q) = %q, want %q", tc.key, tc.in, got, tc.want)
		}
	}
}

// TestSafeHeaderValueForLogRedactsCookie 新增 cookie / set-cookie 全部脱敏。
func TestSafeHeaderValueForLogRedactsCookie(t *testing.T) {
	cases := []struct {
		key, in, want string
	}{
		{"cookie", "session=abc; csrf=xyz", "[redacted]"},
		{"Cookie", "session=abc", "[redacted]"},
		{"set-cookie", "session=abc; HttpOnly", "[redacted]"},
		{"Set-Cookie", "session=abc; HttpOnly", "[redacted]"},
		{"cookie", "", ""}, // 空值不输出 [redacted]
	}
	for _, tc := range cases {
		got := safeHeaderValueForLog(tc.key, tc.in)
		if got != tc.want {
			t.Errorf("safeHeaderValueForLog(%q, %q) = %q, want %q", tc.key, tc.in, got, tc.want)
		}
	}
}

// TestSafeHeaderValueForLogHashesRemoteIdentifiers 远程容器 / 会话 / 保护标识 hash 后入日志。
func TestSafeHeaderValueForLogHashesRemoteIdentifiers(t *testing.T) {
	cases := []struct {
		key, in string
	}{
		{"x-claude-remote-container-id", "container-abc-123"},
		{"X-Claude-Remote-Container-Id", "container-abc-123"},
		{"x-claude-remote-session-id", "session-xyz-456"},
		{"x-anthropic-additional-protection", "prot-def-789"},
	}
	for _, tc := range cases {
		got := safeHeaderValueForLog(tc.key, tc.in)
		if !strings.HasPrefix(got, "sha256:") || !strings.HasSuffix(got, "...") {
			t.Errorf("safeHeaderValueForLog(%q, %q) should hash to sha256:xxxxxxxx...; got %q", tc.key, tc.in, got)
		}
		if strings.Contains(got, tc.in) {
			t.Errorf("safeHeaderValueForLog(%q, %q) leaked the original value into %q", tc.key, tc.in, got)
		}
	}
}

// TestSafeHeaderValueForLogPreservesNonSensitive 非敏感 header 原样返回。
func TestSafeHeaderValueForLogPreservesNonSensitive(t *testing.T) {
	cases := []struct {
		key, in, want string
	}{
		{"user-agent", "claude-cli/2.1.92 (external, cli)", "claude-cli/2.1.92 (external, cli)"},
		{"x-client-app", "cli", "cli"},
		{"x-app", "cli", "cli"},
		{"content-type", "application/json", "application/json"},
	}
	for _, tc := range cases {
		got := safeHeaderValueForLog(tc.key, tc.in)
		if got != tc.want {
			t.Errorf("safeHeaderValueForLog(%q, %q) = %q, want %q", tc.key, tc.in, got, tc.want)
		}
	}
}

// TestHashSummaryStableAndShort 同一输入应稳定，不同输入应不同；空输入返回空。
func TestHashSummaryStableAndShort(t *testing.T) {
	a := hashSummary("session_abc_user_xyz")
	b := hashSummary("session_abc_user_xyz")
	c := hashSummary("session_abc_user_zzz")
	if a == "" {
		t.Fatal("hashSummary should not return empty for non-empty input")
	}
	if a != b {
		t.Errorf("hashSummary should be deterministic; got %q vs %q", a, b)
	}
	if a == c {
		t.Errorf("hashSummary should differ for different inputs")
	}
	if !strings.HasPrefix(a, "sha256:") {
		t.Errorf("hashSummary missing sha256 prefix: %q", a)
	}
	// "sha256:" + 8 hex + "..." = 7 + 8 + 3 = 18
	if len(a) != 18 {
		t.Errorf("hashSummary length = %d, want 18 (sha256:XXXXXXXX...)", len(a))
	}
	if hashSummary("") != "" {
		t.Errorf("hashSummary(\"\") should be empty, got %q", hashSummary(""))
	}
}

// TestBuildClaudeMimicDebugLineHashesMetadataUserID
// metadata.user_id 不能以原文出现在 debug log 中，必须 hash。
func TestBuildClaudeMimicDebugLineHashesMetadataUserID(t *testing.T) {
	rawUserID := "session_abc_user_xyz_super_secret"
	body := []byte(`{"metadata":{"user_id":"` + rawUserID + `"}}`)
	req := &http.Request{
		URL:    mustParseURL(t, "https://api.anthropic.com/v1/messages"),
		Header: http.Header{},
	}

	line := buildClaudeMimicDebugLine(req, body, nil, "oauth", true)

	if strings.Contains(line, rawUserID) {
		t.Fatalf("debug line leaked raw metadata.user_id; line=%s", line)
	}
	if !strings.Contains(line, "meta.user_id.hash=") {
		t.Fatalf("debug line should record hashed meta.user_id.hash=...; line=%s", line)
	}
	if !strings.Contains(line, "sha256:") {
		t.Fatalf("debug line should include sha256 hash prefix; line=%s", line)
	}
}

// TestBuildClaudeMimicDebugLineHashesSystemContent
// 完整 system 内容不应直接进入日志；预览应被截断到 80 字符。
func TestBuildClaudeMimicDebugLineHashesSystemContent(t *testing.T) {
	const secret = "secret_marker_must_not_appear_in_full_in_logs_AAAA"
	longSys := strings.Repeat("a", 500) + " " + secret
	body := []byte(`{"system":"` + longSys + `"}`)
	req := &http.Request{
		URL:    mustParseURL(t, "https://api.anthropic.com/v1/messages"),
		Header: http.Header{},
	}

	line := buildClaudeMimicDebugLine(req, body, nil, "oauth", true)

	// 必须包含 system.hash
	if !strings.Contains(line, "system.hash=") {
		t.Fatalf("debug line should include system.hash; line=%s", line)
	}
	if !strings.Contains(line, "sha256:") {
		t.Fatalf("debug line should include sha256 hash prefix; line=%s", line)
	}
	// 完整 secret 不应出现（被截断到 80 字符 + ...）
	if strings.Contains(line, secret) {
		t.Fatalf("debug line leaked full secret marker through preview; line=%s", line)
	}
}

// TestBuildClaudeMimicDebugLineIncludesConditionalHeadersHashed
// 条件头应被记录但 hash，不应出现原文。
func TestBuildClaudeMimicDebugLineIncludesConditionalHeadersHashed(t *testing.T) {
	const containerID = "container-abc-123-very-secret"
	const sessionID = "session-xyz-456-very-secret"
	req := &http.Request{
		URL:    mustParseURL(t, "https://api.anthropic.com/v1/messages"),
		Header: http.Header{},
	}
	req.Header.Set("x-claude-remote-container-id", containerID)
	req.Header.Set("x-claude-remote-session-id", sessionID)
	req.Header.Set("x-client-app", "vscode")

	line := buildClaudeMimicDebugLine(req, []byte(`{}`), nil, "oauth", true)

	if strings.Contains(line, containerID) {
		t.Fatalf("debug line leaked raw remote-container-id; line=%s", line)
	}
	if strings.Contains(line, sessionID) {
		t.Fatalf("debug line leaked raw remote-session-id; line=%s", line)
	}
	if !strings.Contains(line, "x-claude-remote-container-id=") {
		t.Fatalf("debug line should reference x-claude-remote-container-id key; line=%s", line)
	}
	// x-client-app 是非敏感的应用类型标识，可保留原文
	if !strings.Contains(line, `x-client-app="vscode"`) {
		t.Fatalf("debug line should include verbatim x-client-app=\"vscode\"; line=%s", line)
	}
}

// TestBuildClaudeMimicDebugLineRedactsAuthorization 主路径回归：authorization 不能漏。
func TestBuildClaudeMimicDebugLineRedactsAuthorization(t *testing.T) {
	const token = "Bearer aaaaa.bbbbb.ccccc"
	req := &http.Request{
		URL:    mustParseURL(t, "https://api.anthropic.com/v1/messages"),
		Header: http.Header{},
	}
	req.Header.Set("authorization", token)

	line := buildClaudeMimicDebugLine(req, []byte(`{}`), nil, "oauth", true)
	if strings.Contains(line, token) {
		t.Fatalf("authorization token leaked into debug line; line=%s", line)
	}
	if !strings.Contains(line, "Bearer [redacted]") {
		t.Fatalf("expected Bearer [redacted] in debug line; line=%s", line)
	}
}

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse url %q: %v", raw, err)
	}
	return u
}
