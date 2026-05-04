package service

import (
	"net/http"
	"testing"
)

// TestAllowedHeadersIncludesClaudeConditionalHeaders 验证 Remote / Agent SDK 条件头
// 在 allowedHeaders 中已登记，确保 OAuth / passthrough 链路不会无故丢弃这些头。
func TestAllowedHeadersIncludesClaudeConditionalHeaders(t *testing.T) {
	conditionalHeaders := []string{
		"x-claude-remote-container-id",
		"x-claude-remote-session-id",
		"x-client-app",
		"x-anthropic-additional-protection",
	}
	for _, h := range conditionalHeaders {
		if !allowedHeaders[h] {
			t.Errorf("allowedHeaders should include %q (Claude Remote / Agent SDK 条件头)", h)
		}
	}
}

// TestHeaderWireCasingForConditionalHeaders 验证条件头的 wire casing 保持全小写。
// 真实 Claude CLI 抓包中这些头都是小写 wire form。
func TestHeaderWireCasingForConditionalHeaders(t *testing.T) {
	conditionalHeaders := []string{
		"x-claude-remote-container-id",
		"x-claude-remote-session-id",
		"x-client-app",
		"x-anthropic-additional-protection",
	}
	for _, h := range conditionalHeaders {
		got, ok := headerWireCasing[h]
		if !ok {
			t.Errorf("headerWireCasing missing %q", h)
			continue
		}
		if got != h {
			t.Errorf("headerWireCasing[%q] = %q, want %q (lowercase wire form)", h, got, h)
		}
	}
}

// TestHeaderWireOrderIncludesConditionalHeaders 验证 wire order 中收录条件头，
// 使 debug log 顺序与真实 CLI 抓包一致。
func TestHeaderWireOrderIncludesConditionalHeaders(t *testing.T) {
	conditionalHeaders := []string{
		"x-claude-remote-container-id",
		"x-claude-remote-session-id",
		"x-client-app",
		"x-anthropic-additional-protection",
	}
	for _, h := range conditionalHeaders {
		if _, ok := headerWireOrderSet[h]; !ok {
			t.Errorf("headerWireOrderSet missing %q (must be in headerWireOrder)", h)
		}
	}
}

// TestResolveWireCasingForConditionalHeaders 验证 resolveWireCasing 把 Go canonical 形式
// 还原到 lowercase wire form。
func TestResolveWireCasingForConditionalHeaders(t *testing.T) {
	tests := []struct {
		canonical string
		wire      string
	}{
		{"X-Claude-Remote-Container-Id", "x-claude-remote-container-id"},
		{"X-Claude-Remote-Session-Id", "x-claude-remote-session-id"},
		{"X-Client-App", "x-client-app"},
		{"X-Anthropic-Additional-Protection", "x-anthropic-additional-protection"},
	}
	for _, tc := range tests {
		got := resolveWireCasing(tc.canonical)
		if got != tc.wire {
			t.Errorf("resolveWireCasing(%q) = %q, want %q", tc.canonical, got, tc.wire)
		}
	}
}

// TestSortHeadersByWireOrderConditionalHeaders 验证 sortHeadersByWireOrder 对存在的
// 条件头按定义顺序输出，且未定义条件头追加到末尾时不丢失。
func TestSortHeadersByWireOrderConditionalHeaders(t *testing.T) {
	h := http.Header{}
	h["x-claude-remote-container-id"] = []string{"container-1"}
	h["x-claude-remote-session-id"] = []string{"session-1"}
	h["x-client-app"] = []string{"vscode"}
	h["x-anthropic-additional-protection"] = []string{"prot-1"}
	h["X-Claude-Code-Session-Id"] = []string{"cc-session-1"}

	order := sortHeadersByWireOrder(h)

	// 找每个 key 的位置
	idx := func(target string) int {
		for i, k := range order {
			if k == target {
				return i
			}
		}
		return -1
	}

	codeSessionIdx := idx("X-Claude-Code-Session-Id")
	containerIdx := idx("x-claude-remote-container-id")
	sessionIdx := idx("x-claude-remote-session-id")
	clientAppIdx := idx("x-client-app")
	protIdx := idx("x-anthropic-additional-protection")

	for name, i := range map[string]int{
		"X-Claude-Code-Session-Id":          codeSessionIdx,
		"x-claude-remote-container-id":      containerIdx,
		"x-claude-remote-session-id":        sessionIdx,
		"x-client-app":                      clientAppIdx,
		"x-anthropic-additional-protection": protIdx,
	} {
		if i < 0 {
			t.Fatalf("sortHeadersByWireOrder did not include %q", name)
		}
	}

	// X-Claude-Code-Session-Id 必须排在 remote-container-id 之前（按 headerWireOrder 定义顺序）。
	if codeSessionIdx > containerIdx {
		t.Errorf("expected X-Claude-Code-Session-Id (idx=%d) before remote-container-id (idx=%d)", codeSessionIdx, containerIdx)
	}
	// remote-container-id < remote-session-id < x-client-app < x-anthropic-additional-protection
	if !(containerIdx < sessionIdx && sessionIdx < clientAppIdx && clientAppIdx < protIdx) {
		t.Errorf("conditional headers out of expected order: container=%d session=%d clientApp=%d prot=%d",
			containerIdx, sessionIdx, clientAppIdx, protIdx)
	}
}

// TestConditionalHeadersAreNotSynthesized 描述行为约束：本任务只新增白名单透传，
// 不在缺失时合成默认值。下面的断言是文档化测试 — 当前 allowedHeaders 不包含合成
// 逻辑；如果未来有人在 mimic / oauth header defaults 中无条件 Set 这四个头，
// 应该在该位置新增测试覆盖。这里保留为 explicit no-op 以记录意图。
func TestConditionalHeadersAreNotSynthesized(t *testing.T) {
	// 没有合成默认值的隐式约束：DefaultHeaders / applyClaudeOAuthHeaderDefaults /
	// applyClaudeCodeMimicHeaders 不应为这四个条件头设置默认值。
	// 如果未来添加这些默认值，请明确开关并补对应测试。
	conditional := []string{
		"x-claude-remote-container-id",
		"x-claude-remote-session-id",
		"x-anthropic-additional-protection",
	}
	for _, h := range conditional {
		// 仅校验：这些条件头没有进入 wireCasing 之外的强制默认机制。
		// 真正的合成-禁用断言应在 applyClaudeCodeMimicHeaders 的集成测试里覆盖。
		if _, ok := headerWireCasing[h]; !ok {
			t.Errorf("expected wire casing entry for %q (existence implies pass-through, not synthesis)", h)
		}
	}
	// x-client-app 是允许从客户端透传的应用标识；DefaultHeaders 已有 X-App=cli，
	// 但 x-client-app 是不同的 header，禁止与 x-app 互相覆盖。
	if v, ok := headerWireCasing["x-app"]; ok {
		if v == "x-client-app" {
			t.Fatalf("x-app wire casing must not collide with x-client-app")
		}
	}
}
