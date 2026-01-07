package repository

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	stickySessionPrefix = "sticky_session:"
	latestUserAgentKey  = "latest_user_agent"
)

type gatewayCache struct {
	rdb *redis.Client
}

func NewGatewayCache(rdb *redis.Client) *gatewayCache {
	return &gatewayCache{rdb: rdb}
}

func (c *gatewayCache) GetSessionAccountID(ctx context.Context, sessionHash string) (int64, error) {
	key := stickySessionPrefix + sessionHash
	return c.rdb.Get(ctx, key).Int64()
}

func (c *gatewayCache) SetSessionAccountID(ctx context.Context, sessionHash string, accountID int64, ttl time.Duration) error {
	key := stickySessionPrefix + sessionHash
	return c.rdb.Set(ctx, key, accountID, ttl).Err()
}

func (c *gatewayCache) RefreshSessionTTL(ctx context.Context, sessionHash string, ttl time.Duration) error {
	key := stickySessionPrefix + sessionHash
	return c.rdb.Expire(ctx, key, ttl).Err()
}

// GetLatestUserAgent 获取缓存的最新 User-Agent
func (c *gatewayCache) GetLatestUserAgent(ctx context.Context) (string, error) {
	value, err := c.rdb.Get(ctx, latestUserAgentKey).Result()
	if err == redis.Nil {
		return "", nil
	}
	return value, err
}

// SetLatestUserAgent 设置最新 User-Agent 到缓存
func (c *gatewayCache) SetLatestUserAgent(ctx context.Context, userAgent string, ttl time.Duration) error {
	return c.rdb.Set(ctx, latestUserAgentKey, userAgent, ttl).Err()
}
