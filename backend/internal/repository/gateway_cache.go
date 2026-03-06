package repository

import (
	"context"
	_ "embed"
	"fmt"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const (
	stickySessionPrefix         = "sticky_session:"
	clientAffinityPrefix        = "client_affinity:"
	clientAffinityReversePrefix = "client_affinity_rev:"
)

var (
	//go:embed lua/get_affinity.lua
	getAffinityLua string
	//go:embed lua/update_affinity.lua
	updateAffinityLua string
	//go:embed lua/get_affinity_count.lua
	getAffinityCountLua string

	getAffinityScript      = redis.NewScript(getAffinityLua)
	updateAffinityScript   = redis.NewScript(updateAffinityLua)
	getAffinityCountScript = redis.NewScript(getAffinityCountLua)
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
func (c *gatewayCache) DeleteSessionAccountID(ctx context.Context, groupID int64, sessionHash string) error {
	key := buildSessionKey(groupID, sessionHash)
	return c.rdb.Del(ctx, key).Err()
}

// buildAffinityKey 构建正向亲和 key（client → accounts）
// 格式: client_affinity:{groupID}:{clientID}
func buildAffinityKey(groupID int64, clientID string) string {
	return fmt.Sprintf("%s%d:%s", clientAffinityPrefix, groupID, clientID)
}

// buildAffinityReverseKey 构建反向亲和 key（account → clients）
// 格式: client_affinity_rev:{groupID}:{accountID}
func buildAffinityReverseKey(groupID int64, accountID int64) string {
	return fmt.Sprintf("%s%d:%d", clientAffinityReversePrefix, groupID, accountID)
}

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
	fwdKey := buildAffinityKey(groupID, clientID)
	revKey := buildAffinityReverseKey(groupID, accountID)
	now := time.Now().Unix()
	ttlSeconds := int64(ttl.Seconds())
	expireThreshold := now - ttlSeconds

	return updateAffinityScript.Run(ctx, c.rdb, []string{fwdKey, revKey},
		now, ttlSeconds, accountID, expireThreshold, clientID,
	).Err()
}

// GetAccountAffinityCountBatch 批量获取账号的亲和客户端数量（惰性清理过期成员）
func (c *gatewayCache) GetAccountAffinityCountBatch(ctx context.Context, groupID int64, accountIDs []int64, ttl time.Duration) (map[int64]int64, error) {
	if len(accountIDs) == 0 {
		return map[int64]int64{}, nil
	}

	now := time.Now().Unix()
	expireThreshold := now - int64(ttl.Seconds())

	pipe := c.rdb.Pipeline()
	cmds := make([]*redis.Cmd, len(accountIDs))
	for i, accID := range accountIDs {
		key := buildAffinityReverseKey(groupID, accID)
		cmds[i] = getAffinityCountScript.Run(ctx, pipe, []string{key}, expireThreshold)
	}
	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return nil, err
	}

	result := make(map[int64]int64, len(accountIDs))
	for i, accID := range accountIDs {
		count, _ := cmds[i].Int64()
		result[accID] = count
	}
	return result, nil
}
