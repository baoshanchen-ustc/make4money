//go:build unit

package service

import (
	"fmt"
	"testing"
	"time"
)

func TestCopilotSessionCache_FirstSeen_ReturnsPassthrough(t *testing.T) {
	c := newCopilotSessionCache(2 * time.Hour)
	// 首次见到 session key：不覆盖，返回 false（由调用方决定 initiator）
	if c.markAndCheckSeen("sess-abc") {
		t.Fatal("first call: expected false (not yet seen), got true")
	}
}

func TestCopilotSessionCache_SecondSeen_ReturnsAgent(t *testing.T) {
	c := newCopilotSessionCache(2 * time.Hour)
	c.markAndCheckSeen("sess-abc") // 首次
	// 第二次：session 已存在，应返回 true（调用方应使用 "agent"）
	if !c.markAndCheckSeen("sess-abc") {
		t.Fatal("second call: expected true (already seen), got false")
	}
}

func TestCopilotSessionCache_DifferentKeys_Independent(t *testing.T) {
	c := newCopilotSessionCache(2 * time.Hour)
	c.markAndCheckSeen("sess-aaa")
	// 不同 key 首次应该返回 false
	if c.markAndCheckSeen("sess-bbb") {
		t.Fatal("different key: expected false (first time), got true")
	}
}

func TestCopilotSessionCache_TTLExpiry(t *testing.T) {
	ttl := 50 * time.Millisecond
	c := newCopilotSessionCache(ttl)
	c.markAndCheckSeen("sess-ttl")
	// 等 TTL 过期
	time.Sleep(ttl + 20*time.Millisecond)
	c.evictExpired() // 手动触发清理
	// 过期后再访问，应视为全新 session
	if c.markAndCheckSeen("sess-ttl") {
		t.Fatal("after TTL: expected false (evicted), got true")
	}
}

// TestCopilotSessionCache_AccountIsolation verifies that two different accounts
// with the same raw session key do NOT share cache state.
func TestCopilotSessionCache_AccountIsolation(t *testing.T) {
	c := newCopilotSessionCache(2 * time.Hour)
	const rawKey = "shared-session-key"

	// Account 1, first time → cache miss
	k1 := fmt.Sprintf("%d:%s", int64(1), rawKey)
	if c.markAndCheckSeen(k1) {
		t.Fatal("account 1 first call: expected false")
	}
	// Account 2, same raw key, first time → still a miss (different namespace)
	k2 := fmt.Sprintf("%d:%s", int64(2), rawKey)
	if c.markAndCheckSeen(k2) {
		t.Fatal("account 2 first call: expected false (isolated from account 1)")
	}
	// Account 1, second time → cache hit
	if !c.markAndCheckSeen(k1) {
		t.Fatal("account 1 second call: expected true (cache hit)")
	}
}

func TestExtractSessionKeyFromOpenAIBody(t *testing.T) {
	// CC 风格的 user 字段（legacy metadata.user_id 格式，含 session_id）
	body := []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}],"user":"user_a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2_account__session_12345678-1234-1234-1234-123456789abc"}`)
	key := extractSessionKeyFromOpenAIBody(body)
	const want = "12345678-1234-1234-1234-123456789abc"
	if key != want {
		t.Fatalf("got %q, want %q", key, want)
	}
}

func TestExtractSessionKeyFromOpenAIBody_NoUser(t *testing.T) {
	body := []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}]}`)
	if key := extractSessionKeyFromOpenAIBody(body); key != "" {
		t.Fatalf("expected empty key, got %q", key)
	}
}

func TestExtractSessionKeyFromAnthropicBody(t *testing.T) {
	body := []byte(`{"model":"claude-sonnet-4-5","messages":[{"role":"user","content":"hi"}],"metadata":{"user_id":"user_a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2_account__session_12345678-1234-1234-1234-123456789abc"}}`)
	key := extractSessionKeyFromAnthropicBody(body)
	const want = "12345678-1234-1234-1234-123456789abc"
	if key != want {
		t.Fatalf("got %q, want %q", key, want)
	}
}
