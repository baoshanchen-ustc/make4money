package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"
)

const defaultClaudeProbeDedupTTL = 10 * time.Minute

type claudeProbeDedupCache struct {
	mu      sync.Mutex
	ttl     time.Duration
	now     func() time.Time
	entries map[string]time.Time
}

func newClaudeProbeDedupCache(ttl time.Duration) *claudeProbeDedupCache {
	if ttl <= 0 {
		ttl = defaultClaudeProbeDedupTTL
	}
	return &claudeProbeDedupCache{
		ttl:     ttl,
		now:     time.Now,
		entries: make(map[string]time.Time),
	}
}

func (c *claudeProbeDedupCache) SeenOrStore(apiKeyID, groupID int64, model string, body []byte) bool {
	if c == nil {
		return false
	}
	key := buildClaudeProbeDedupKey(apiKeyID, groupID, model, body)
	now := c.now()
	expiresAt := now.Add(c.ttl)

	c.mu.Lock()
	defer c.mu.Unlock()

	for storedKey, storedExpiresAt := range c.entries {
		if !storedExpiresAt.After(now) {
			delete(c.entries, storedKey)
		}
	}
	if existingExpiresAt, ok := c.entries[key]; ok {
		if existingExpiresAt.After(now) {
			return true
		}
	}
	c.entries[key] = expiresAt
	return false
}

func buildClaudeProbeDedupKey(apiKeyID, groupID int64, model string, body []byte) string {
	sum := sha256.Sum256(body)
	return fmt.Sprintf("%d:%d:%s:%s", apiKeyID, groupID, strings.ToLower(strings.TrimSpace(model)), hex.EncodeToString(sum[:]))
}
