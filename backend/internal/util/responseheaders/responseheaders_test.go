package responseheaders

import (
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

func TestFilterHeadersDisabledUsesDefaultAllowlist(t *testing.T) {
	src := http.Header{}
	src.Add("Content-Type", "application/json")
	src.Add("X-Request-Id", "req-123")
	src.Add("X-Test", "ok")
	src.Add("Connection", "keep-alive")
	src.Add("Content-Length", "123")

	cfg := config.ResponseHeaderConfig{
		Enabled:     false,
		ForceRemove: []string{"x-request-id"},
	}

	filtered := FilterHeaders(src, CompileHeaderFilter(cfg))
	if filtered.Get("Content-Type") != "application/json" {
		t.Fatalf("expected Content-Type passthrough, got %q", filtered.Get("Content-Type"))
	}
	if filtered.Get("X-Request-Id") != "req-123" {
		t.Fatalf("expected X-Request-Id allowed, got %q", filtered.Get("X-Request-Id"))
	}
	if filtered.Get("X-Test") != "" {
		t.Fatalf("expected X-Test removed, got %q", filtered.Get("X-Test"))
	}
	if filtered.Get("Connection") != "" {
		t.Fatalf("expected Connection to be removed, got %q", filtered.Get("Connection"))
	}
	if filtered.Get("Content-Length") != "" {
		t.Fatalf("expected Content-Length to be removed, got %q", filtered.Get("Content-Length"))
	}
}

func TestFilterHeadersEnabledUsesAllowlist(t *testing.T) {
	src := http.Header{}
	src.Add("Content-Type", "application/json")
	src.Add("X-Extra", "ok")
	src.Add("X-Remove", "nope")
	src.Add("X-Blocked", "nope")

	cfg := config.ResponseHeaderConfig{
		Enabled:           true,
		AdditionalAllowed: []string{"x-extra"},
		ForceRemove:       []string{"x-remove"},
	}

	filtered := FilterHeaders(src, CompileHeaderFilter(cfg))
	if filtered.Get("Content-Type") != "application/json" {
		t.Fatalf("expected Content-Type allowed, got %q", filtered.Get("Content-Type"))
	}
	if filtered.Get("X-Extra") != "ok" {
		t.Fatalf("expected X-Extra allowed, got %q", filtered.Get("X-Extra"))
	}
	if filtered.Get("X-Remove") != "" {
		t.Fatalf("expected X-Remove removed, got %q", filtered.Get("X-Remove"))
	}
	if filtered.Get("X-Blocked") != "" {
		t.Fatalf("expected X-Blocked removed, got %q", filtered.Get("X-Blocked"))
	}
}

// Gateway trace prefix denylist 测试：覆盖默认过滤、大小写混淆、additional_allowed
// 误放行、危险 override 开启四种场景。

// gatewayTraceProbes 是测试用的"已知 gateway 痕迹响应头"样本，覆盖每个 prefix。
var gatewayTraceProbes = map[string]string{
	"x-litellm-model-id":   "gpt-4-turbo",
	"X-Litellm-Key-Name":   "test-key",     // 大小写混淆：仍应命中
	"helicone-id":          "helicone-123",
	"Helicone-Cache-Hit":   "true",
	"x-portkey-trace-id":   "portkey-abc",
	"cf-aig-cache-status":  "MISS",
	"CF-AIG-Trace-Id":      "cf-trace-1",   // 大小写混淆：仍应命中
	"x-kong-proxy-latency": "12",
	"x-bt-host":            "bt-host-1",
}

// TestFilterHeadersGatewayTracePrefixDenylistDefaults 默认配置（disabled）下也必须过滤 gateway 痕迹。
func TestFilterHeadersGatewayTracePrefixDenylistDefaults(t *testing.T) {
	src := http.Header{}
	src.Add("Content-Type", "application/json")
	for k, v := range gatewayTraceProbes {
		src.Add(k, v)
	}

	// disabled config：仍应使用默认白名单 + denylist
	filtered := FilterHeaders(src, CompileHeaderFilter(config.ResponseHeaderConfig{}))
	if filtered.Get("Content-Type") != "application/json" {
		t.Fatalf("Content-Type should pass default allowlist")
	}
	for k := range gatewayTraceProbes {
		if filtered.Get(k) != "" {
			t.Errorf("expected %q to be filtered by gateway trace denylist, got %q", k, filtered.Get(k))
		}
	}
}

// TestFilterHeadersGatewayTracePrefixCaseInsensitive 任意大小写混合的 gateway 痕迹头都应被过滤。
func TestFilterHeadersGatewayTracePrefixCaseInsensitive(t *testing.T) {
	src := http.Header{}
	mixed := []string{
		"X-LiteLLM-Model-Id",
		"X-LITELLM-MODEL-ID",
		"Helicone-User-Id",
		"HELICONE-USER-ID",
		"X-Portkey-Trace-Id",
		"X-PORTKEY-TRACE-ID",
		"Cf-Aig-Trace-Id",
		"CF-AIG-Trace-Id",
		"X-KONG-Proxy-Latency",
		"X-Bt-Trace",
	}
	for _, k := range mixed {
		src.Add(k, "should-not-leak")
	}

	cfg := config.ResponseHeaderConfig{Enabled: true}
	filtered := FilterHeaders(src, CompileHeaderFilter(cfg))
	for _, k := range mixed {
		if filtered.Get(k) != "" {
			t.Errorf("mixed-case gateway header %q should be filtered, got %q", k, filtered.Get(k))
		}
	}
}

// TestFilterHeadersGatewayTraceDenylistBeatsAdditionalAllowed
// 即使管理员把 gateway header 加入 additional_allowed，denylist 仍然生效。
func TestFilterHeadersGatewayTraceDenylistBeatsAdditionalAllowed(t *testing.T) {
	src := http.Header{}
	src.Add("X-Litellm-Model-Id", "leak")
	src.Add("Helicone-Id", "leak")

	cfg := config.ResponseHeaderConfig{
		Enabled: true,
		AdditionalAllowed: []string{
			"x-litellm-model-id",
			"helicone-id",
		},
	}
	filtered := FilterHeaders(src, CompileHeaderFilter(cfg))
	if filtered.Get("X-Litellm-Model-Id") != "" {
		t.Fatalf("denylist must beat additional_allowed for gateway prefix; got %q", filtered.Get("X-Litellm-Model-Id"))
	}
	if filtered.Get("Helicone-Id") != "" {
		t.Fatalf("denylist must beat additional_allowed for helicone-id; got %q", filtered.Get("Helicone-Id"))
	}
}

// TestFilterHeadersForceRemoveExactDoesNotReplacePrefix
// 用 ForceRemove(exact) 试图替代 prefix denylist 不应该等价：
// ForceRemove 的 exact-match 只能拦明确的字符串，新的派生头（如 "x-litellm-team-id"）会漏过；
// 内置 prefix denylist 才是稳定的安全边界。
func TestFilterHeadersForceRemoveExactDoesNotReplacePrefix(t *testing.T) {
	src := http.Header{}
	src.Add("X-Litellm-Model-Id", "should-be-blocked-by-denylist")
	src.Add("X-Litellm-Team-Id", "future-derived-header")
	src.Add("X-Custom", "ok") // ForceRemove 的对照：仅 exact match 命中

	// 模拟管理员只配 ForceRemove，没有意识到 denylist 的存在。
	cfg := config.ResponseHeaderConfig{
		Enabled:           true,
		AdditionalAllowed: []string{"x-custom"},
		ForceRemove:       []string{"x-custom"},
	}
	filtered := FilterHeaders(src, CompileHeaderFilter(cfg))

	// X-Custom 被 ForceRemove
	if filtered.Get("X-Custom") != "" {
		t.Fatalf("X-Custom should be removed by ForceRemove; got %q", filtered.Get("X-Custom"))
	}
	// X-Litellm-* 全部被 prefix denylist 拦截，不依赖 ForceRemove
	if filtered.Get("X-Litellm-Model-Id") != "" {
		t.Fatalf("X-Litellm-Model-Id should be blocked by prefix denylist; got %q", filtered.Get("X-Litellm-Model-Id"))
	}
	if filtered.Get("X-Litellm-Team-Id") != "" {
		t.Fatalf("X-Litellm-Team-Id should be blocked by prefix denylist (proves prefix > exact match); got %q",
			filtered.Get("X-Litellm-Team-Id"))
	}
}

// TestFilterHeadersGatewayTraceOverrideAllowsPassthrough
// 显式开启 AllowGatewayTraceHeaders 时（仅诊断），denylist 退让；
// 此时 header 仍需在 allowlist / additional_allowed 中才能透传。
func TestFilterHeadersGatewayTraceOverrideAllowsPassthrough(t *testing.T) {
	src := http.Header{}
	src.Add("X-Litellm-Model-Id", "diagnostic")
	src.Add("Helicone-Id", "diagnostic")
	src.Add("X-Random-Other", "should-still-be-filtered")

	cfg := config.ResponseHeaderConfig{
		Enabled:                  true,
		AllowGatewayTraceHeaders: true, // 危险诊断 override
		AdditionalAllowed: []string{
			"x-litellm-model-id",
			"helicone-id",
		},
	}
	filtered := FilterHeaders(src, CompileHeaderFilter(cfg))

	if filtered.Get("X-Litellm-Model-Id") != "diagnostic" {
		t.Fatalf("override should allow x-litellm-model-id passthrough; got %q", filtered.Get("X-Litellm-Model-Id"))
	}
	if filtered.Get("Helicone-Id") != "diagnostic" {
		t.Fatalf("override should allow helicone-id passthrough; got %q", filtered.Get("Helicone-Id"))
	}
	// 即便 override 开启，未在 allowlist 的随机头仍被默认白名单拦截
	if filtered.Get("X-Random-Other") != "" {
		t.Fatalf("override should not turn into a global passthrough; got %q", filtered.Get("X-Random-Other"))
	}
}

// TestFilterHeadersGatewayTracePreservesBusinessHeaders
// gateway trace denylist 不应误伤业务 header（x-request-id / rate-limit / retry-after）。
func TestFilterHeadersGatewayTracePreservesBusinessHeaders(t *testing.T) {
	src := http.Header{}
	src.Add("X-Request-Id", "req-1")
	src.Add("Retry-After", "10")
	src.Add("X-Ratelimit-Limit-Requests", "100")
	src.Add("X-Litellm-Model-Id", "should-be-filtered")

	filtered := FilterHeaders(src, CompileHeaderFilter(config.ResponseHeaderConfig{Enabled: true}))
	if filtered.Get("X-Request-Id") != "req-1" {
		t.Fatalf("X-Request-Id must remain; got %q", filtered.Get("X-Request-Id"))
	}
	if filtered.Get("Retry-After") != "10" {
		t.Fatalf("Retry-After must remain; got %q", filtered.Get("Retry-After"))
	}
	if filtered.Get("X-Ratelimit-Limit-Requests") != "100" {
		t.Fatalf("X-Ratelimit-Limit-Requests must remain; got %q", filtered.Get("X-Ratelimit-Limit-Requests"))
	}
	if filtered.Get("X-Litellm-Model-Id") != "" {
		t.Fatalf("X-Litellm-Model-Id should be filtered; got %q", filtered.Get("X-Litellm-Model-Id"))
	}
}
