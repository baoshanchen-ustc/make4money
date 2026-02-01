package service

import (
	"context"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/redis/go-redis/v9"
)

var (
	ErrRechargeRateLimitExceeded      = infraerrors.TooManyRequests("RECHARGE_RATE_LIMIT_EXCEEDED", "操作过于频繁，请稍后重试")
	ErrRechargeDailyLimitExceeded     = infraerrors.TooManyRequests("RECHARGE_DAILY_LIMIT_EXCEEDED", "今日充值次数已达上限，请明天再试")
)

// RechargeRateLimitResult 限流检查结果
type RechargeRateLimitResult struct {
	Allowed    bool
	Remaining  int
	RetryAfter int // 秒
	Message    string
}

// DailyRateLimitResult 日级限流检查结果
type DailyRateLimitResult struct {
	Allowed   bool
	Remaining int
	ResetTime time.Time // 重置时间（次日凌晨）
	Message   string
}

// CombinedRateLimitResult 组合限流检查结果
type CombinedRateLimitResult struct {
	Allowed         bool
	LimitType       string    // "minute" 或 "daily"
	Message         string
	RetryAfter      int       // 秒（分钟级限流）
	ResetTime       time.Time // 重置时间（日级限流）
	MinuteRemaining int
	DailyRemaining  int
}

// RechargeRateLimitService 充值限流服务
type RechargeRateLimitService struct {
	cfg   *config.Config
	redis *redis.Client
}

// NewRechargeRateLimitService 创建充值限流服务
func NewRechargeRateLimitService(cfg *config.Config, redis *redis.Client) *RechargeRateLimitService {
	return &RechargeRateLimitService{
		cfg:   cfg,
		redis: redis,
	}
}

// slidingWindowScript 滑动窗口限流 Lua 脚本
// 使用 Sorted Set 实现滑动窗口限流
var slidingWindowScript = redis.NewScript(`
	local key = KEYS[1]
	local window_start = tonumber(ARGV[1])
	local now = tonumber(ARGV[2])
	local limit = tonumber(ARGV[3])
	local window_ms = tonumber(ARGV[4])

	-- 移除窗口外的请求
	redis.call('ZREMRANGEBYSCORE', key, '-inf', window_start)

	-- 获取当前窗口内的请求数
	local count = redis.call('ZCARD', key)

	if count < limit then
		-- 添加当前请求
		redis.call('ZADD', key, now, now)
		-- 设置过期时间
		redis.call('PEXPIRE', key, window_ms)
		return {1, limit - count - 1, 0}
	else
		-- 获取最早的请求时间
		local oldest = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')
		local retry_after = 0
		if #oldest > 0 then
			retry_after = math.ceil((tonumber(oldest[2]) + window_ms - now) / 1000)
			if retry_after < 0 then retry_after = 0 end
		end
		return {0, 0, retry_after}
	end
`)

// CheckRechargeMinuteLimit 检查充值分钟级限流
func (s *RechargeRateLimitService) CheckRechargeMinuteLimit(ctx context.Context, userID int64) (*RechargeRateLimitResult, error) {
	// 检查是否启用限流
	if !s.cfg.RateLimit.Recharge.Enabled {
		return &RechargeRateLimitResult{Allowed: true, Remaining: -1}, nil
	}

	key := fmt.Sprintf("ratelimit:recharge:user:%d:minute", userID)
	limit := s.cfg.RateLimit.Recharge.MinuteLimit
	if limit <= 0 {
		limit = 3 // 默认每分钟3次
	}
	windowSec := s.cfg.RateLimit.Recharge.MinuteWindowSec
	if windowSec <= 0 {
		windowSec = 60 // 默认60秒
	}
	window := time.Duration(windowSec) * time.Second

	allowed, remaining, retryAfter, err := s.checkSlidingWindow(ctx, key, limit, window)
	if err != nil {
		return nil, fmt.Errorf("check rate limit failed: %w", err)
	}

	result := &RechargeRateLimitResult{
		Allowed:    allowed,
		Remaining:  remaining,
		RetryAfter: retryAfter,
	}

	if !allowed {
		result.Message = fmt.Sprintf("操作过于频繁，请在 %d 秒后重试", retryAfter)
	}

	return result, nil
}

// checkSlidingWindow 滑动窗口限流检查
func (s *RechargeRateLimitService) checkSlidingWindow(ctx context.Context, key string, limit int, window time.Duration) (bool, int, int, error) {
	now := time.Now()
	windowStart := now.Add(-window).UnixMilli()
	nowMilli := now.UnixMilli()

	result, err := slidingWindowScript.Run(ctx, s.redis, []string{key},
		windowStart,
		nowMilli,
		limit,
		window.Milliseconds(),
	).Slice()

	if err != nil {
		return false, 0, 0, fmt.Errorf("rate limit script failed: %w", err)
	}

	if len(result) < 3 {
		return false, 0, 0, fmt.Errorf("unexpected result length: %d", len(result))
	}

	allowed := result[0].(int64) == 1
	remaining := int(result[1].(int64))
	retryAfter := int(result[2].(int64))

	return allowed, remaining, retryAfter, nil
}

// dailyLimitScript 日级限流 Lua 脚本
// 使用 String 计数器 + EXPIREAT 实现
var dailyLimitScript = redis.NewScript(`
	local key = KEYS[1]
	local limit = tonumber(ARGV[1])
	local expire_at = tonumber(ARGV[2])

	local current = redis.call('GET', key)
	if current == false then
		current = 0
	else
		current = tonumber(current)
	end

	if current < limit then
		redis.call('INCR', key)
		redis.call('EXPIREAT', key, expire_at)
		return {1, limit - current - 1}
	else
		return {0, 0}
	end
`)

// CheckRechargeDailyLimit 检查充值日级限流
func (s *RechargeRateLimitService) CheckRechargeDailyLimit(ctx context.Context, userID int64) (*DailyRateLimitResult, error) {
	// 检查是否启用限流
	if !s.cfg.RateLimit.Recharge.Enabled {
		return &DailyRateLimitResult{Allowed: true, Remaining: -1}, nil
	}

	limit := s.cfg.RateLimit.Recharge.DailyLimit
	if limit <= 0 {
		limit = 20 // 默认每天20次
	}

	// 生成当日日期作为 key 后缀
	now := time.Now()
	date := now.Format("2006-01-02")
	key := fmt.Sprintf("ratelimit:recharge:user:%d:daily:%s", userID, date)

	// 计算次日凌晨时间
	tomorrow := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())

	// 执行 Lua 脚本
	result, err := dailyLimitScript.Run(ctx, s.redis, []string{key},
		limit,
		tomorrow.Unix(),
	).Slice()

	if err != nil {
		return nil, fmt.Errorf("daily rate limit script failed: %w", err)
	}

	if len(result) < 2 {
		return nil, fmt.Errorf("unexpected result length: %d", len(result))
	}

	allowed := result[0].(int64) == 1
	remaining := int(result[1].(int64))

	res := &DailyRateLimitResult{
		Allowed:   allowed,
		Remaining: remaining,
		ResetTime: tomorrow,
	}

	if !allowed {
		res.Message = fmt.Sprintf("今日充值次数已达上限（%d次），请明天再试", limit)
	}

	return res, nil
}

// CheckRechargeRateLimits 检查所有充值限流（分钟级 + 日级）
// 先检查日级限流，再检查分钟级限流
func (s *RechargeRateLimitService) CheckRechargeRateLimits(ctx context.Context, userID int64) (*CombinedRateLimitResult, error) {
	// 检查是否启用限流
	if !s.cfg.RateLimit.Recharge.Enabled {
		return &CombinedRateLimitResult{Allowed: true}, nil
	}

	// 先检查日级限流（避免无效的分钟级计数）
	dailyResult, err := s.CheckRechargeDailyLimit(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("check daily limit failed: %w", err)
	}
	if !dailyResult.Allowed {
		return &CombinedRateLimitResult{
			Allowed:        false,
			LimitType:      "daily",
			Message:        dailyResult.Message,
			ResetTime:      dailyResult.ResetTime,
			DailyRemaining: 0,
		}, nil
	}

	// 再检查分钟级限流
	minuteResult, err := s.CheckRechargeMinuteLimit(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("check minute limit failed: %w", err)
	}
	if !minuteResult.Allowed {
		return &CombinedRateLimitResult{
			Allowed:         false,
			LimitType:       "minute",
			Message:         minuteResult.Message,
			RetryAfter:      minuteResult.RetryAfter,
			MinuteRemaining: 0,
			DailyRemaining:  dailyResult.Remaining,
		}, nil
	}

	return &CombinedRateLimitResult{
		Allowed:         true,
		MinuteRemaining: minuteResult.Remaining,
		DailyRemaining:  dailyResult.Remaining,
	}, nil
}
