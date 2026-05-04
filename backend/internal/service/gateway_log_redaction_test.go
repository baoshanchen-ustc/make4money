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

// TestBuildClaudeMimicDebugLineReadsRawWireCasing 防止回归：转发路径用 setHeaderRaw 把 header 以
// 原始小写 wire form 存到 map（不走 Go canonical 化）。debug line 必须用 getHeaderRaw 读取，
// 否则 Header.Get 会按 canonical 查不到这些 raw key，进而漏掉本应记录的 hashed 指纹。
//
// 这条测试与 TestBuildClaudeMimicDebugLineHashesAuthorization 等的区别在于：
//   - 那些测试用 req.Header.Set(...)，会触发 Go MIMEHeader 的 canonical 化（"authorization" → "Authorization"）；
//   - 真实转发路径用 setHeaderRaw 直接 h[key] = []string{value}，map key 保持原样小写。
//
// 如果维护者把 buildClaudeMimicDebugLine 的 Header.Get 改回去，这条测试会立刻失败。
func TestBuildClaudeMimicDebugLineReadsRawWireCasing(t *testing.T) {
	req := &http.Request{
		URL:    mustParseURL(t, "https://api.anthropic.com/v1/messages"),
		Header: http.Header{},
	}

	const (
		token       = "Bearer raw.cased.token.aaaaa"
		containerID = "container-raw-cased-id-12345"
		sessionID   = "session-raw-cased-id-67890"
		clientReqID = "raw-cased-client-request-id-uuid"
	)

	// 模拟生产转发路径：setHeaderRaw 直接以 wire casing 存储到底层 map。
	setHeaderRaw(req.Header, "authorization", token)
	setHeaderRaw(req.Header, "x-claude-remote-container-id", containerID)
	setHeaderRaw(req.Header, "x-claude-remote-session-id", sessionID)
	setHeaderRaw(req.Header, "x-client-request-id", clientReqID)
	setHeaderRaw(req.Header, "x-client-app", "vscode")
	setHeaderRaw(req.Header, "user-agent", "claude-cli/2.1.92 (external, cli)")

	// 防御性 sanity：req.Header.Get（canonical lookup）应当读不到这些 raw key，
	// 这正是本测试要避免的场景。这里 assert 一下让"为什么必须用 getHeaderRaw"显式可见。
	if got := req.Header.Get("authorization"); got != "" {
		t.Logf("note: Header.Get found %q via canonical lookup; raw-casing assumption may have changed", got)
	}

	line := buildClaudeMimicDebugLine(req, []byte(`{}`), nil, "oauth", true)

	// authorization 必须被记录并 redact —— 之前的 bug 表现是 Header.Get 找不到，整行不出现 authorization 字段。
	if !strings.Contains(line, "authorization=") {
		t.Errorf("debug line should record authorization field even when stored as raw lowercase; line=%s", line)
	}
	if strings.Contains(line, token) {
		t.Errorf("authorization token leaked into debug line; line=%s", line)
	}
	if !strings.Contains(line, "Bearer [redacted]") {
		t.Errorf("expected Bearer [redacted] for raw-cased authorization; line=%s", line)
	}

	// 远程容器 / 会话 ID 必须以 hash 形式出现，不能漏。
	for _, key := range []string{"x-claude-remote-container-id", "x-claude-remote-session-id"} {
		if !strings.Contains(line, key+"=") {
			t.Errorf("debug line should record %q (raw casing) via getHeaderRaw; line=%s", key, line)
		}
	}
	if strings.Contains(line, containerID) {
		t.Errorf("raw-cased container id leaked; line=%s", line)
	}
	if strings.Contains(line, sessionID) {
		t.Errorf("raw-cased session id leaked; line=%s", line)
	}

	// x-client-request-id 必须出现，但作为非敏感 ID 原样记录。
	if !strings.Contains(line, "x-client-request-id="+`"`+clientReqID+`"`) {
		t.Errorf("debug line should include verbatim x-client-request-id from raw casing; line=%s", line)
	}

	// x-client-app 与 user-agent 走非敏感路径，不应漏。
	if !strings.Contains(line, `x-client-app="vscode"`) {
		t.Errorf("debug line should include verbatim x-client-app; line=%s", line)
	}
	if !strings.Contains(line, `user-agent="claude-cli/2.1.92 (external, cli)"`) {
		t.Errorf("debug line should include verbatim user-agent from raw casing; line=%s", line)
	}
}
