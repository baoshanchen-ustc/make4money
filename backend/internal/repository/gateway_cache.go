package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const stickySessionPrefix = "sticky_session:"
const stickySessionIndexPrefix = "sticky_session_index:"

type gatewayCache struct {
	rdb *redis.Client
}

func NewGatewayCache(rdb *redis.Client) service.GatewayCache {
	return &gatewayCache{rdb: rdb}
}

// buildSessionKey 构建 session key，包含 groupID 实现分组隔离
// 格式: sticky_session:{groupID}:{sessionHash}
func buildSessionKey(groupID int64, sessionHash string) string {
	return fmt.Sprintf("%s%d:%s", stickySessionPrefix, groupID, sessionHash)
}

// buildSessionIndexKey 构建反向索引 key，用于从 accountID 查找 sessionHash
// 格式: sticky_session_index:{groupID}:{accountID}
func buildSessionIndexKey(groupID int64, accountID int64) string {
	return fmt.Sprintf("%s%d:%d", stickySessionIndexPrefix, groupID, accountID)
}

func (c *gatewayCache) GetSessionAccountID(ctx context.Context, groupID int64, sessionHash string) (int64, error) {
	key := buildSessionKey(groupID, sessionHash)
	return c.rdb.Get(ctx, key).Int64()
}

func (c *gatewayCache) SetSessionAccountID(ctx context.Context, groupID int64, sessionHash string, accountID int64, ttl time.Duration) error {
	key := buildSessionKey(groupID, sessionHash)
	indexKey := buildSessionIndexKey(groupID, accountID)

	pipe := c.rdb.Pipeline()
	pipe.Set(ctx, key, accountID, ttl)
	pipe.SAdd(ctx, indexKey, sessionHash)
	pipe.Expire(ctx, indexKey, ttl)
	_, err := pipe.Exec(ctx)
	return err
}

func (c *gatewayCache) RefreshSessionTTL(ctx context.Context, groupID int64, sessionHash string, ttl time.Duration) error {
	key := buildSessionKey(groupID, sessionHash)
	return c.rdb.Expire(ctx, key, ttl).Err()
}

// DeleteSessionAccountID 删除粘性会话与账号的绑定关系。
// 当检测到绑定的账号不可用（如状态错误、禁用、不可调度等）时调用，
// 以便下次请求能够重新选择可用账号。
//
// DeleteSessionAccountID removes the sticky session binding for the given session.
// Called when the bound account becomes unavailable (e.g., error status, disabled,
// or unschedulable), allowing subsequent requests to select a new available account.
func (c *gatewayCache) DeleteSessionAccountID(ctx context.Context, groupID int64, sessionHash string) error {
	key := buildSessionKey(groupID, sessionHash)
	// 注意：这里不清理反向索引，因为不知道对应的 accountID
	// 反向索引会在 GetSessionAccountID 返回错误时由调用方清理，或在 DeleteStickySessionsByAccount 中批量清理
	return c.rdb.Del(ctx, key).Err()
}

// DeleteStickySessionsByAccount 删除指定账号在指定分组中的所有粘性会话。
// 当账号被移除分组时调用，确保该账号不会继续被 sticky session 使用。
//
// DeleteStickySessionsByAccount deletes all sticky sessions for the given account in the given group.
// Called when account is removed from a group to ensure it won't be used by sticky sessions.
func (c *gatewayCache) DeleteStickySessionsByAccount(ctx context.Context, groupID int64, accountID int64) error {
	indexKey := buildSessionIndexKey(groupID, accountID)

	// 获取该账号的所有 sessionHash
	sessionHashes, err := c.rdb.SMembers(ctx, indexKey).Result()
	if err != nil {
		return fmt.Errorf("get session index failed: %w", err)
	}

	if len(sessionHashes) == 0 {
		return nil
	}

	// 批量删除 sticky session
	pipe := c.rdb.Pipeline()
	for _, sessionHash := range sessionHashes {
		sessionKey := buildSessionKey(groupID, sessionHash)
		pipe.Del(ctx, sessionKey)
	}
	// 删除反向索引
	pipe.Del(ctx, indexKey)

	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete sticky sessions failed: %w", err)
	}
	return nil
}
