package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const (
	hotPoolMembersPrefix     = "hotpool:members:"
	hotPoolMetaPrefix        = "hotpool:meta:"
	hotPoolDeadAccountPrefix = "hotpool:dead_account:"
	hotPoolRefillLockPrefix  = "hotpool:refill_lock:"
)

type hotPoolCache struct {
	rdb *redis.Client
}

func NewHotPoolCache(rdb *redis.Client) service.HotPoolCache {
	return &hotPoolCache{rdb: rdb}
}

func (c *hotPoolCache) ListMembers(ctx context.Context, platform string) ([]int64, error) {
	values, err := c.rdb.SMembers(ctx, hotPoolMembersKey(platform)).Result()
	if err != nil {
		return nil, err
	}
	out := make([]int64, 0, len(values))
	for _, raw := range values {
		id, convErr := strconv.ParseInt(raw, 10, 64)
		if convErr != nil || id <= 0 {
			continue
		}
		out = append(out, id)
	}
	return out, nil
}

func (c *hotPoolCache) AddMembers(ctx context.Context, platform string, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	members := make([]any, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		members = append(members, strconv.FormatInt(id, 10))
	}
	if len(members) == 0 {
		return nil
	}
	return c.rdb.SAdd(ctx, hotPoolMembersKey(platform), members...).Err()
}

func (c *hotPoolCache) ReplaceMembers(ctx context.Context, platform string, ids []int64) error {
	pipe := c.rdb.TxPipeline()
	membersKey := hotPoolMembersKey(platform)
	pipe.Del(ctx, membersKey)
	members := make([]any, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		members = append(members, strconv.FormatInt(id, 10))
	}
	if len(members) > 0 {
		pipe.SAdd(ctx, membersKey, members...)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (c *hotPoolCache) RemoveMember(ctx context.Context, platform string, accountID int64) error {
	if accountID <= 0 {
		return nil
	}
	return c.rdb.SRem(ctx, hotPoolMembersKey(platform), strconv.FormatInt(accountID, 10)).Err()
}

func (c *hotPoolCache) CountMembers(ctx context.Context, platform string) (int, error) {
	count, err := c.rdb.SCard(ctx, hotPoolMembersKey(platform)).Result()
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

func (c *hotPoolCache) GetMeta(ctx context.Context, platform string) (*service.HotPoolPlatformMeta, error) {
	value, err := c.rdb.Get(ctx, hotPoolMetaKey(platform)).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var meta service.HotPoolPlatformMeta
	if err := json.Unmarshal([]byte(value), &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

func (c *hotPoolCache) SetMeta(ctx context.Context, platform string, meta *service.HotPoolPlatformMeta) error {
	if meta == nil {
		return nil
	}
	payload, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, hotPoolMetaKey(platform), payload, 0).Err()
}

func (c *hotPoolCache) MarkDeadAccount(ctx context.Context, accountID int64, ttl time.Duration) error {
	if accountID <= 0 {
		return nil
	}
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return c.rdb.Set(ctx, hotPoolDeadAccountKey(accountID), "1", ttl).Err()
}

func (c *hotPoolCache) IsDeadAccount(ctx context.Context, accountID int64) (bool, error) {
	if accountID <= 0 {
		return false, nil
	}
	count, err := c.rdb.Exists(ctx, hotPoolDeadAccountKey(accountID)).Result()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (c *hotPoolCache) TryAcquireRefillLock(ctx context.Context, platform string, ttl time.Duration) (bool, error) {
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	return c.rdb.SetNX(ctx, hotPoolRefillLockKey(platform), time.Now().UnixNano(), ttl).Result()
}

func hotPoolMembersKey(platform string) string {
	return hotPoolMembersPrefix + platform
}

func hotPoolMetaKey(platform string) string {
	return hotPoolMetaPrefix + platform
}

func hotPoolDeadAccountKey(accountID int64) string {
	return fmt.Sprintf("%s%d", hotPoolDeadAccountPrefix, accountID)
}

func hotPoolRefillLockKey(platform string) string {
	return hotPoolRefillLockPrefix + platform
}
