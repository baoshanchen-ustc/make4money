package repository

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const (
	stickySessionPrefix  = "sticky_session:"
	clientAffinityPrefix = "client_affinity:"
)

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

func (c *gatewayCache) GetSessionAccountID(ctx context.Context, groupID int64, sessionHash string) (int64, error) {
	key := buildSessionKey(groupID, sessionHash)
	return c.rdb.Get(ctx, key).Int64()
}

func (c *gatewayCache) SetSessionAccountID(ctx context.Context, groupID int64, sessionHash string, accountID int64, ttl time.Duration) error {
	key := buildSessionKey(groupID, sessionHash)
	return c.rdb.Set(ctx, key, accountID, ttl).Err()
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
	return c.rdb.Del(ctx, key).Err()
}

// buildAffinityKey 构建客户端亲和 key
// 格式: client_affinity:{groupID}:{clientID}
func buildAffinityKey(groupID int64, clientID string) string {
	return fmt.Sprintf("%s%d:%s", clientAffinityPrefix, groupID, clientID)
}

// getAffinityScript: 清理过期成员后返回亲和账号列表（按最近使用降序）
// KEYS[1] = client_affinity:{groupID}:{clientID}
// ARGV[1] = 过期阈值时间戳 (now - ttl)
var getAffinityScript = redis.NewScript(`
redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', ARGV[1])
return redis.call('ZREVRANGE', KEYS[1], 0, -1)
`)

// updateAffinityScript: 清理过期成员、添加/更新亲和关系、刷新 TTL
// KEYS[1] = client_affinity:{groupID}:{clientID}
// ARGV[1] = 当前时间戳
// ARGV[2] = TTL 秒数
// ARGV[3] = accountID
// ARGV[4] = 过期阈值时间戳 (now - ttl)
var updateAffinityScript = redis.NewScript(`
redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', ARGV[4])
redis.call('ZADD', KEYS[1], ARGV[1], ARGV[3])
redis.call('EXPIRE', KEYS[1], ARGV[2])
return 1
`)

func (c *gatewayCache) GetClientAffinityAccounts(ctx context.Context, groupID int64, clientID string, ttl time.Duration) ([]int64, error) {
	key := buildAffinityKey(groupID, clientID)
	now := time.Now().Unix()
	expireThreshold := now - int64(ttl.Seconds())

	result, err := getAffinityScript.Run(ctx, c.rdb, []string{key}, expireThreshold).StringSlice()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}

	accountIDs := make([]int64, 0, len(result))
	for _, s := range result {
		id, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			continue
		}
		accountIDs = append(accountIDs, id)
	}
	return accountIDs, nil
}

func (c *gatewayCache) UpdateClientAffinity(ctx context.Context, groupID int64, clientID string, accountID int64, ttl time.Duration) error {
	key := buildAffinityKey(groupID, clientID)
	now := time.Now().Unix()
	ttlSeconds := int64(ttl.Seconds())
	expireThreshold := now - ttlSeconds

	return updateAffinityScript.Run(ctx, c.rdb, []string{key},
		now, ttlSeconds, accountID, expireThreshold,
	).Err()
}

func (c *gatewayCache) RemoveClientAffinity(ctx context.Context, groupID int64, clientID string, accountID int64) error {
	key := buildAffinityKey(groupID, clientID)
	return c.rdb.ZRem(ctx, key, accountID).Err()
}
