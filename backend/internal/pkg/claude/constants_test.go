package claude

import (
	"fmt"
	"regexp"
	"testing"
)

// TestDefaultHeadersUserAgentBaseline 验证 DefaultHeaders["User-Agent"] 与 CLICurrentVersion
// 严格一致，且不退化到明显过旧的版本。
//
// 这条测试存在的意义：避免维护者悄悄改了 CLICurrentVersion 但忘了同步 User-Agent，
// 或反过来；也避免有人误把 1.x / 2.0.x / 2.1.22 这类旧版本 baseline 改回来。
func TestDefaultHeadersUserAgentBaseline(t *testing.T) {
	ua, ok := DefaultHeaders["User-Agent"]
	if !ok {
		t.Fatal("DefaultHeaders missing User-Agent")
	}

	// 必须形如 "claude-cli/X.Y.Z (external, cli)"
	pattern := regexp.MustCompile(`^claude-cli/2\.\d+\.\d+ \(external, cli\)$`)
	if !pattern.MatchString(ua) {
		t.Fatalf("User-Agent %q does not match expected pattern claude-cli/2.X.Y (external, cli)", ua)
	}

	// 必须与 CLICurrentVersion 严格一致
	expected := fmt.Sprintf("claude-cli/%s (external, cli)", CLICurrentVersion)
	if ua != expected {
		t.Fatalf("User-Agent %q does not match CLICurrentVersion %q (expected %q)", ua, CLICurrentVersion, expected)
	}

	// blacklist：禁止退化到已知过旧的版本串
	staleVersions := []string{
		"claude-cli/1.",
		"claude-cli/2.0.",
		"claude-cli/2.1.22 ",
		"claude-cli/2.1.50 ",
	}
	for _, stale := range staleVersions {
		// 用 HasPrefix 风格断言；2.1.22 / 2.1.50 后接空格可避免与 2.1.220 误匹配
		if len(ua) >= len(stale) && ua[:len(stale)] == stale {
			t.Fatalf("User-Agent %q reverts to known-stale baseline prefix %q", ua, stale)
		}
	}
}

// TestDefaultHeadersRequiredKeys 验证 mimicry baseline 不会丢失关键字段。
func TestDefaultHeadersRequiredKeys(t *testing.T) {
	required := []string{
		"User-Agent",
		"X-App",
		"X-Stainless-Lang",
		"X-Stainless-Package-Version",
		"X-Stainless-OS",
		"X-Stainless-Arch",
		"X-Stainless-Runtime",
		"X-Stainless-Runtime-Version",
		"X-Stainless-Retry-Count",
		"X-Stainless-Timeout",
		"Anthropic-Dangerous-Direct-Browser-Access",
	}

	for _, key := range required {
		val, ok := DefaultHeaders[key]
		if !ok {
			t.Errorf("DefaultHeaders missing required key %q", key)
			continue
		}
		if val == "" {
			t.Errorf("DefaultHeaders[%q] is empty", key)
		}
	}
}

// TestDefaultHeadersStainlessPackageVersion 防止 X-Stainless-Package-Version
// 误退化为 0.0.0 / 空 / 未定义版本号。
func TestDefaultHeadersStainlessPackageVersion(t *testing.T) {
	v := DefaultHeaders["X-Stainless-Package-Version"]
	if v == "" {
		t.Fatal("X-Stainless-Package-Version is empty")
	}
	if v == "0.0.0" {
		t.Fatalf("X-Stainless-Package-Version reverted to %q", v)
	}
	// 必须是 SemVer-ish，例如 0.70.0 / 1.0.0
	pattern := regexp.MustCompile(`^\d+\.\d+\.\d+(?:[-+][0-9A-Za-z\.\-]+)?$`)
	if !pattern.MatchString(v) {
		t.Fatalf("X-Stainless-Package-Version %q is not a recognized SemVer", v)
	}
}

// TestDefaultHeadersAnthropicBrowserAccess 验证安全相关 header 没被改成 false。
func TestDefaultHeadersAnthropicBrowserAccess(t *testing.T) {
	v, ok := DefaultHeaders["Anthropic-Dangerous-Direct-Browser-Access"]
	if !ok {
		t.Fatal("DefaultHeaders missing Anthropic-Dangerous-Direct-Browser-Access")
	}
	if v != "true" {
		t.Fatalf("Anthropic-Dangerous-Direct-Browser-Access expected \"true\", got %q", v)
	}
}

// TestCLICurrentVersionMatchesUserAgent 强制 CLICurrentVersion 与 User-Agent 中的版本号一致，
// 防止 fingerprint 与 attribution block 写不同版本。
func TestCLICurrentVersionMatchesUserAgent(t *testing.T) {
	ua := DefaultHeaders["User-Agent"]
	expectedSubstring := "claude-cli/" + CLICurrentVersion + " "
	if len(ua) < len(expectedSubstring) || ua[:len(expectedSubstring)] != expectedSubstring {
		t.Fatalf("User-Agent %q does not contain CLICurrentVersion prefix %q", ua, expectedSubstring)
	}
}
