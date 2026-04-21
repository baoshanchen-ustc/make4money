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
	passkeyFlowKeyPrefix      = "auth:passkey:flow:"
	passkeyReplayKeyPrefix    = "auth:passkey:flow:replay:"
	recentAuthMarkerKeyPrefix = "auth:recent:"
	authStateReplayMarkerTTL  = 5 * time.Minute
)

var consumePasskeyFlowScript = redis.NewScript(`
local flowKey = KEYS[1]
local replayKey = KEYS[2]
local replayTTLSeconds = tonumber(ARGV[1])

local value = redis.call('GET', flowKey)
if value then
  redis.call('DEL', flowKey)
  redis.call('SET', replayKey, '1', 'EX', replayTTLSeconds)
  return {1, value}
end

if redis.call('EXISTS', replayKey) == 1 then
  return {2, ''}
end

return {0, ''}
`)

type authStateCache struct {
	rdb *redis.Client
}

func NewAuthStateCache(rdb *redis.Client) service.AuthStateCache {
	return &authStateCache{rdb: rdb}
}

func (c *authStateCache) SetPasskeyChallenge(ctx context.Context, flowID string, record *service.PasskeyChallengeRecord, ttl time.Duration) error {
	key := passkeyFlowKey(flowID)
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal passkey challenge: %w", err)
	}

	if err := c.rdb.Set(ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("set passkey challenge: %w", err)
	}

	return nil
}

func (c *authStateCache) ConsumePasskeyChallenge(ctx context.Context, flowID string) (*service.PasskeyChallengeRecord, service.PasskeyChallengeConsumeStatus, error) {
	flowKey := passkeyFlowKey(flowID)
	replayKey := passkeyReplayKey(flowID)

	result, err := consumePasskeyFlowScript.Run(
		ctx,
		c.rdb,
		[]string{flowKey, replayKey},
		int(authStateReplayMarkerTTL.Seconds()),
	).Result()
	if err != nil {
		return nil, service.PasskeyChallengeConsumeMissing, fmt.Errorf("consume passkey challenge: %w", err)
	}

	statusCode, payload, err := parsePasskeyConsumeScriptResult(result)
	if err != nil {
		return nil, service.PasskeyChallengeConsumeMissing, err
	}

	switch statusCode {
	case 1:
		var record service.PasskeyChallengeRecord
		if err := json.Unmarshal([]byte(payload), &record); err != nil {
			return nil, service.PasskeyChallengeConsumeMissing, fmt.Errorf("unmarshal passkey challenge: %w", err)
		}
		return &record, service.PasskeyChallengeConsumeFound, nil
	case 2:
		return nil, service.PasskeyChallengeConsumeReplayed, nil
	default:
		return nil, service.PasskeyChallengeConsumeMissing, nil
	}
}

func (c *authStateCache) SetRecentAuthMarker(ctx context.Context, userID int64, marker *service.RecentAuthMarker, ttl time.Duration) error {
	key := recentAuthMarkerKey(userID)
	data, err := json.Marshal(marker)
	if err != nil {
		return fmt.Errorf("marshal recent auth marker: %w", err)
	}

	if err := c.rdb.Set(ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("set recent auth marker: %w", err)
	}

	return nil
}

func (c *authStateCache) GetRecentAuthMarker(ctx context.Context, userID int64) (*service.RecentAuthMarker, error) {
	key := recentAuthMarkerKey(userID)
	data, err := c.rdb.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("get recent auth marker: %w", err)
	}

	var marker service.RecentAuthMarker
	if err := json.Unmarshal(data, &marker); err != nil {
		return nil, fmt.Errorf("unmarshal recent auth marker: %w", err)
	}

	return &marker, nil
}

func passkeyFlowKey(flowID string) string {
	return passkeyFlowKeyPrefix + flowID
}

func passkeyReplayKey(flowID string) string {
	return passkeyReplayKeyPrefix + flowID
}

func recentAuthMarkerKey(userID int64) string {
	return recentAuthMarkerKeyPrefix + strconv.FormatInt(userID, 10)
}

func parsePasskeyConsumeScriptResult(result any) (int64, string, error) {
	items, ok := result.([]any)
	if !ok || len(items) != 2 {
		return 0, "", fmt.Errorf("unexpected consume passkey challenge response")
	}

	statusCode, ok := items[0].(int64)
	if !ok {
		return 0, "", fmt.Errorf("unexpected consume passkey challenge status type %T", items[0])
	}

	switch payload := items[1].(type) {
	case string:
		return statusCode, payload, nil
	case []byte:
		return statusCode, string(payload), nil
	case nil:
		return statusCode, "", nil
	default:
		return 0, "", fmt.Errorf("unexpected consume passkey challenge payload type %T", payload)
	}
}
