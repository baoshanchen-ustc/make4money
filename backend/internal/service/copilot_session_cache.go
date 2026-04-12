package service

import (
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// copilotSessionCache tracks which session keys have been seen.
//
// The first request within a session pays the Premium quota (X-Initiator: user);
// all subsequent requests in the same session are free (X-Initiator: agent).
//
// Implementation uses a sync.Mutex + map with per-entry expiry timestamps.
// Periodic eviction is handled by a background ticker goroutine started in
// NewCopilotGatewayService; evictExpired() is also exposed for tests.
type copilotSessionCache struct {
	mu      sync.Mutex
	entries map[string]time.Time // key → expiry
	ttl     time.Duration
}

func newCopilotSessionCache(ttl time.Duration) *copilotSessionCache {
	return &copilotSessionCache{
		entries: make(map[string]time.Time),
		ttl:     ttl,
	}
}

// markAndCheckSeen records a session key as seen and reports whether it was
// already present (true = already seen → caller should use "agent").
//
// On first call for a key: stores it with a TTL, returns false.
// On subsequent calls within TTL: refreshes TTL, returns true.
// After TTL expires and eviction: behaves like first call again.
func (c *copilotSessionCache) markAndCheckSeen(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	if exp, ok := c.entries[key]; ok && now.Before(exp) {
		// Already seen and not expired — refresh TTL and signal "agent".
		c.entries[key] = now.Add(c.ttl)
		return true
	}
	// First time (or expired): record and signal "user" (caller decides).
	c.entries[key] = now.Add(c.ttl)
	return false
}

// evictExpired removes all entries whose TTL has elapsed. Call periodically
// to prevent unbounded memory growth (e.g. from a ticker in the service).
func (c *copilotSessionCache) evictExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for k, exp := range c.entries {
		if now.After(exp) {
			delete(c.entries, k)
		}
	}
}

// size returns the number of live entries (for metrics/testing).
func (c *copilotSessionCache) size() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}

// --------------------------------------------------------------------------
// Session key extraction helpers
// --------------------------------------------------------------------------

// sessionKeyFromHeader returns the X-Session-ID header value, trimmed.
// Returns "" if absent or empty.
func sessionKeyFromHeader(c *gin.Context) string {
	return strings.TrimSpace(c.GetHeader("X-Session-ID"))
}

// extractSessionKeyFromOpenAIBody tries to extract a session key from an
// OpenAI-format request body.
//
// Claude Code (and compatible clients) populate the OpenAI "user" field with
// the Anthropic metadata.user_id value, which contains a session UUID that is
// stable across all requests in the same conversation (including sub-agents).
//
// Returns "" if the body has no parseable session ID.
func extractSessionKeyFromOpenAIBody(body []byte) string {
	var req struct {
		User string `json:"user"`
	}
	if err := json.Unmarshal(body, &req); err != nil || req.User == "" {
		return ""
	}
	parsed := ParseMetadataUserID(req.User)
	if parsed == nil || parsed.SessionID == "" {
		return ""
	}
	return parsed.SessionID
}

// extractSessionKeyFromAnthropicBody tries to extract a session key from an
// Anthropic-format request body (metadata.user_id).
//
// Returns "" if the body has no parseable session ID.
func extractSessionKeyFromAnthropicBody(body []byte) string {
	var req struct {
		Metadata struct {
			UserID string `json:"user_id"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(body, &req); err != nil || req.Metadata.UserID == "" {
		return ""
	}
	parsed := ParseMetadataUserID(req.Metadata.UserID)
	if parsed == nil || parsed.SessionID == "" {
		return ""
	}
	return parsed.SessionID
}
