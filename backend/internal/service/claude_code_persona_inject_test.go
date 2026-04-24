package service

import (
	"strings"
	"testing"

	"github.com/tidwall/gjson"
)

// 通用断言：注入后的 body 中至少出现一次 sentinel；二次注入不会重复。
func assertContainsPersonaOnce(t *testing.T, body []byte) {
	t.Helper()
	count := strings.Count(string(body), claudeCodePersonaSentinel)
	if count != 1 {
		t.Fatalf("expected sentinel exactly once, got %d. body=%s", count, string(body))
	}
}

func TestInjectClaudeCodePersonaAnthropic_NoSystem(t *testing.T) {
	body := []byte(`{"model":"claude-3","messages":[{"role":"user","content":"hi"}]}`)
	out := InjectClaudeCodePersonaAnthropic(body)
	assertContainsPersonaOnce(t, out)
	if got := gjson.GetBytes(out, "system").String(); !strings.Contains(got, claudeCodePersonaSentinel) {
		t.Fatalf("system field missing persona: %s", got)
	}
}

func TestInjectClaudeCodePersonaAnthropic_StringSystem(t *testing.T) {
	body := []byte(`{"model":"claude-3","system":"keep this","messages":[]}`)
	out := InjectClaudeCodePersonaAnthropic(body)
	assertContainsPersonaOnce(t, out)
	got := gjson.GetBytes(out, "system").String()
	if !strings.Contains(got, "keep this") {
		t.Fatalf("expected original system content preserved, got: %s", got)
	}
}

func TestInjectClaudeCodePersonaAnthropic_ArraySystem(t *testing.T) {
	body := []byte(`{"model":"claude-3","system":[{"type":"text","text":"original"}],"messages":[]}`)
	out := InjectClaudeCodePersonaAnthropic(body)
	assertContainsPersonaOnce(t, out)
	arr := gjson.GetBytes(out, "system").Array()
	if len(arr) != 2 {
		t.Fatalf("expected 2 system blocks, got %d", len(arr))
	}
	if !strings.Contains(arr[0].Get("text").String(), claudeCodePersonaSentinel) {
		t.Fatalf("persona not first in array")
	}
	if arr[1].Get("text").String() != "original" {
		t.Fatalf("original block lost or reordered")
	}
}

func TestInjectClaudeCodePersonaAnthropic_Idempotent(t *testing.T) {
	body := []byte(`{"model":"claude-3","messages":[]}`)
	once := InjectClaudeCodePersonaAnthropic(body)
	twice := InjectClaudeCodePersonaAnthropic(once)
	assertContainsPersonaOnce(t, twice)
}

func TestInjectClaudeCodePersonaChatMessages_NoMessages(t *testing.T) {
	body := []byte(`{"model":"gpt-4"}`)
	out := InjectClaudeCodePersonaChatMessages(body)
	assertContainsPersonaOnce(t, out)
	first := gjson.GetBytes(out, "messages.0")
	if first.Get("role").String() != "system" {
		t.Fatalf("expected first role=system, got %s", first.Get("role").String())
	}
}

func TestInjectClaudeCodePersonaChatMessages_PrependWhenNoSystem(t *testing.T) {
	body := []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"hi"}]}`)
	out := InjectClaudeCodePersonaChatMessages(body)
	assertContainsPersonaOnce(t, out)
	arr := gjson.GetBytes(out, "messages").Array()
	if len(arr) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(arr))
	}
	if arr[0].Get("role").String() != "system" {
		t.Fatalf("expected first role=system, got %s", arr[0].Get("role").String())
	}
}

func TestInjectClaudeCodePersonaChatMessages_MergeIntoExistingSystem(t *testing.T) {
	body := []byte(`{"model":"gpt-4","messages":[{"role":"system","content":"keep me"},{"role":"user","content":"hi"}]}`)
	out := InjectClaudeCodePersonaChatMessages(body)
	assertContainsPersonaOnce(t, out)
	arr := gjson.GetBytes(out, "messages").Array()
	if len(arr) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(arr))
	}
	merged := arr[0].Get("content").String()
	if !strings.Contains(merged, claudeCodePersonaSentinel) || !strings.Contains(merged, "keep me") {
		t.Fatalf("merged system missing persona or original: %s", merged)
	}
}

func TestInjectClaudeCodePersonaChatMessages_Idempotent(t *testing.T) {
	body := []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"hi"}]}`)
	once := InjectClaudeCodePersonaChatMessages(body)
	twice := InjectClaudeCodePersonaChatMessages(once)
	assertContainsPersonaOnce(t, twice)
}

func TestInjectClaudeCodePersonaInstructions_Empty(t *testing.T) {
	body := []byte(`{"model":"gpt-4"}`)
	out := InjectClaudeCodePersonaInstructions(body)
	assertContainsPersonaOnce(t, out)
}

func TestInjectClaudeCodePersonaInstructions_PrependExisting(t *testing.T) {
	body := []byte(`{"model":"gpt-4","instructions":"existing instructions"}`)
	out := InjectClaudeCodePersonaInstructions(body)
	assertContainsPersonaOnce(t, out)
	got := gjson.GetBytes(out, "instructions").String()
	if !strings.Contains(got, "existing instructions") {
		t.Fatalf("existing instructions lost: %s", got)
	}
	if !strings.HasPrefix(got, claudeCodePersonaSentinel[:20]) {
		t.Fatalf("persona should be prepended, got prefix: %q", got[:40])
	}
}

func TestInjectClaudeCodePersonaInstructions_Idempotent(t *testing.T) {
	body := []byte(`{"model":"gpt-4","instructions":"x"}`)
	once := InjectClaudeCodePersonaInstructions(body)
	twice := InjectClaudeCodePersonaInstructions(once)
	assertContainsPersonaOnce(t, twice)
}

func TestInjectClaudeCodePersonaGemini_NoSystemInstruction(t *testing.T) {
	body := []byte(`{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`)
	out := InjectClaudeCodePersonaGemini(body)
	assertContainsPersonaOnce(t, out)
	if got := gjson.GetBytes(out, "systemInstruction.parts.0.text").String(); !strings.Contains(got, claudeCodePersonaSentinel) {
		t.Fatalf("systemInstruction missing persona: %s", got)
	}
}

func TestInjectClaudeCodePersonaGemini_MergeExisting(t *testing.T) {
	body := []byte(`{"systemInstruction":{"parts":[{"text":"old prompt"}]},"contents":[]}`)
	out := InjectClaudeCodePersonaGemini(body)
	assertContainsPersonaOnce(t, out)
	got := gjson.GetBytes(out, "systemInstruction.parts.0.text").String()
	if !strings.Contains(got, "old prompt") {
		t.Fatalf("old prompt lost: %s", got)
	}
}

func TestInjectClaudeCodePersonaGemini_Idempotent(t *testing.T) {
	body := []byte(`{"contents":[]}`)
	once := InjectClaudeCodePersonaGemini(body)
	twice := InjectClaudeCodePersonaGemini(once)
	assertContainsPersonaOnce(t, twice)
}

func TestClaudeCodePersonaPromptStartsWithSentinel(t *testing.T) {
	if !strings.HasPrefix(claudeCodePersonaPromptText, claudeCodePersonaSentinel) {
		t.Fatalf("prompt text must begin with sentinel for idempotent detection; got prefix: %q", claudeCodePersonaPromptText[:50])
	}
}

func TestEmptyBodyReturnsUnchanged(t *testing.T) {
	for name, fn := range map[string]func([]byte) []byte{
		"anthropic":    InjectClaudeCodePersonaAnthropic,
		"chat":         InjectClaudeCodePersonaChatMessages,
		"instructions": InjectClaudeCodePersonaInstructions,
		"gemini":       InjectClaudeCodePersonaGemini,
	} {
		out := fn(nil)
		if out != nil {
			t.Fatalf("%s: nil body should return nil, got %q", name, string(out))
		}
	}
}
