package repository

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"

	"github.com/redis/go-redis/v9"
)

var defaultRedisClient atomic.Pointer[redis.Client]

var ErrRedisClientNotInitialized = errors.New("redis client not initialized")

// InitRedis 初始化 Redis 客户端
//
// 性能优化说明：
// 原实现使用 go-redis 默认配置，未设置连接池和超时参数：
// 1. 默认连接池大小可能不足以支撑高并发
// 2. 无超时控制可能导致慢操作阻塞
//
// 新实现支持可配置的连接池和超时参数：
// 1. PoolSize: 控制最大并发连接数（默认 128）
// 2. MinIdleConns: 保持最小空闲连接，减少冷启动延迟（默认 10）
// 3. DialTimeout/ReadTimeout/WriteTimeout: 精确控制各阶段超时
func InitRedis(cfg *config.Config) *redis.Client {
	client := redis.NewClient(buildRedisOptions(cfg))
	defaultRedisClient.Store(client)
	return client
}

const (
	defaultRedisPoolSize        = 128
	defaultRedisPoolTimeout     = 30 * time.Second
	defaultRedisConnMaxIdleTime = 5 * time.Minute
)

// buildRedisOptions 构建 Redis 连接选项
// 从配置文件读取连接池和超时参数，支持生产环境调优
func buildRedisOptions(cfg *config.Config) *redis.Options {
	if cfg == nil {
		cfg = &config.Config{}
	}
	poolSize := cfg.Redis.PoolSize
	if poolSize <= 0 {
		poolSize = defaultRedisPoolSize
	}
	minIdle := cfg.Redis.MinIdleConns
	if minIdle < 0 {
		minIdle = 0
	}
	if minIdle > poolSize {
		minIdle = poolSize
	}

	opts := &redis.Options{
		Addr:            cfg.Redis.Address(),
		Password:        cfg.Redis.Password,
		DB:              cfg.Redis.DB,
		DialTimeout:     time.Duration(cfg.Redis.DialTimeoutSeconds) * time.Second,  // 建连超时
		ReadTimeout:     time.Duration(cfg.Redis.ReadTimeoutSeconds) * time.Second,  // 读取超时
		WriteTimeout:    time.Duration(cfg.Redis.WriteTimeoutSeconds) * time.Second, // 写入超时
		PoolSize:        poolSize,
		MinIdleConns:    minIdle,
		PoolTimeout:     defaultRedisPoolTimeout,
		ConnMaxIdleTime: defaultRedisConnMaxIdleTime,
	}

	if cfg.Redis.EnableTLS {
		opts.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
			ServerName: cfg.Redis.Host,
		}
	}
	warnings := evaluateRedisPoolWarnings(opts.PoolSize, opts.MinIdleConns)
	logRedisPoolWarnings(warnings, opts.PoolSize, opts.MinIdleConns)

	return opts
}

const (
	redisPoolSizeWarningThreshold      = 512
	redisMinIdleRatioWarningThreshold  = 0.8
	redisMinIdleHardLimitWarningThresh = 1.0
	redisMinIdleAbsoluteWarningThresh  = 64
)

func evaluateRedisPoolWarnings(poolSize, minIdle int) []string {
	var warnings []string
	if poolSize > redisPoolSizeWarningThreshold {
		warnings = append(warnings, fmt.Sprintf("pool_size (%d) exceeds %d threshold", poolSize, redisPoolSizeWarningThreshold))
	}
	if minIdle > redisMinIdleAbsoluteWarningThresh {
		warnings = append(warnings, fmt.Sprintf("min_idle_conns (%d) exceeds conservative threshold %d", minIdle, redisMinIdleAbsoluteWarningThresh))
	}
	if poolSize > 0 {
		ratio := float64(minIdle) / float64(poolSize)
		if ratio >= redisMinIdleRatioWarningThreshold {
			warnings = append(warnings, fmt.Sprintf("min_idle_conns (%d) is %.0f%% of pool_size", minIdle, ratio*100))
		}
		if ratio >= redisMinIdleHardLimitWarningThresh {
			warnings = append(warnings, "min_idle_conns equals or exceeds pool_size")
		}
	}
	return warnings
}

func logRedisPoolWarnings(warnings []string, poolSize, minIdle int) {
	if len(warnings) == 0 {
		return
	}
	logger.WriteSinkEvent("warn", "repository.redis", "redis pool configuration guard", map[string]any{
		"warnings": warnings,
		"configured": map[string]any{
			"pool_size": poolSize,
			"min_idle":  minIdle,
		},
	})
}

// RedisPoolSnapshot captures runtime pool metrics that are valuable to ops.
type RedisPoolSnapshot struct {
	Hits                      int64    `json:"hits"`
	Misses                    int64    `json:"misses"`
	Stalls                    int64    `json:"stalls"`
	Timeouts                  int64    `json:"timeouts"`
	TotalConns                int64    `json:"total_conns"`
	IdleConns                 int64    `json:"idle_conns"`
	UsagePercent              *float64 `json:"usage_percent,omitempty"`
	IdleReservationRatio      *float64 `json:"idle_reservation_ratio,omitempty"`
	HitRatePercent            *float64 `json:"hit_rate_percent,omitempty"`
	ConnectionPressurePercent *float64 `json:"connection_pressure_percent,omitempty"`
	PoolHeadroomPercent       *float64 `json:"pool_headroom_percent,omitempty"`
}

// SnapshotRedisPoolStats returns pool metrics from the given client.
func SnapshotRedisPoolStats(client *redis.Client) RedisPoolSnapshot {
	if client == nil {
		return RedisPoolSnapshot{}
	}
	poolSize := 0
	if client.Options() != nil {
		poolSize = client.Options().PoolSize
	}
	return RedisPoolSnapshotFromStats(client.PoolStats(), poolSize)
}

// SnapshotDefaultRedisPoolStats returns pool stats for the default initialized client.
func SnapshotDefaultRedisPoolStats() RedisPoolSnapshot {
	return SnapshotRedisPoolStats(defaultRedisClient.Load())
}

func ProbeRedis(ctx context.Context, client *redis.Client) error {
	if client == nil {
		return ErrRedisClientNotInitialized
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return client.Ping(ctx).Err()
}

func ProbeDefaultRedis(ctx context.Context) error {
	return ProbeRedis(ctx, defaultRedisClient.Load())
}

// RedisPoolSnapshotFromStats converts go-redis stats into our snapshot structure.
func RedisPoolSnapshotFromStats(stats *redis.PoolStats, poolSize int) RedisPoolSnapshot {
	if stats == nil {
		return RedisPoolSnapshot{}
	}
	totalLookups := stats.Hits + stats.Misses
	poolSize64 := int64(poolSize)
	headroom := poolSize64 - int64(stats.TotalConns)
	if headroom < 0 {
		headroom = 0
	}
	return RedisPoolSnapshot{
		Hits:   int64(stats.Hits),
		Misses: int64(stats.Misses),
		// go-redis v9 exposes wait count instead of a direct "stalls" field.
		// We keep the outward-facing name stable for existing ops payloads.
		Stalls:                    int64(stats.WaitCount),
		Timeouts:                  int64(stats.Timeouts),
		TotalConns:                int64(stats.TotalConns),
		IdleConns:                 int64(stats.IdleConns),
		UsagePercent:              ratioPercentInt64(int64(stats.TotalConns), poolSize64),
		IdleReservationRatio:      ratioPercentInt64(int64(stats.IdleConns), poolSize64),
		HitRatePercent:            ratioPercentInt64(int64(stats.Hits), int64(totalLookups)),
		ConnectionPressurePercent: ratioPercentInt64(int64(stats.TotalConns), poolSize64),
		PoolHeadroomPercent:       ratioPercentInt64(headroom, poolSize64),
	}
}

// EvaluateRedisPressure summarizes Redis pool pressure for quick resource checks.
func EvaluateRedisPressure(snapshot RedisPoolSnapshot) (string, string) {
	state := "normal"
	note := "Redis pool healthy"
	if snapshot.Timeouts > 0 {
		state = "critical"
		note = fmt.Sprintf("redis timeouts=%d", snapshot.Timeouts)
		return state, note
	}
	if snapshot.Stalls > 0 {
		state = "warning"
		note = fmt.Sprintf("redis stalls=%d", snapshot.Stalls)
	}
	if snapshot.ConnectionPressurePercent != nil {
		pressure := *snapshot.ConnectionPressurePercent
		if pressure >= 95 {
			state = "warning"
			note = fmt.Sprintf("connections at %.1f%% of pool", pressure)
		}
	}
	if state == "normal" && snapshot.UsagePercent != nil {
		usage := *snapshot.UsagePercent
		if usage >= 90 {
			state = "warning"
			note = fmt.Sprintf("usage %.1f%%", usage)
		}
	}
	return state, note
}

// SummarizeRedisPressureTrend provides a short trend narrative for ops dashboards.
func SummarizeRedisPressureTrend(snapshot RedisPoolSnapshot) (string, string) {
	if snapshot.Timeouts > 0 {
		return "critical", fmt.Sprintf("timeouts=%d breaches, autorepair recommended", snapshot.Timeouts)
	}
	if snapshot.Stalls > 0 {
		return "warning", fmt.Sprintf("stalls=%d, backpressure rising", snapshot.Stalls)
	}
	if snapshot.ConnectionPressurePercent != nil {
		pressure := *snapshot.ConnectionPressurePercent
		if pressure >= 85 {
			return "warning", fmt.Sprintf("connections at %.1f%% of pool", pressure)
		}
		if pressure >= 70 {
			return "info", fmt.Sprintf("connections at %.1f%%", pressure)
		}
	}
	if snapshot.UsagePercent != nil {
		usage := *snapshot.UsagePercent
		if usage >= 80 {
			return "info", fmt.Sprintf("usage %.1f%%, monitor trends", usage)
		}
	}
	return "stable", "pressure normal"
}

func ratioPercentInt64(current, limit int64) *float64 {
	if limit <= 0 || current < 0 {
		return nil
	}
	pct := float64(current) / float64(limit) * 100
	if pct < 0 {
		pct = 0
	}
	if pct > 200 {
		pct = 200
	}
	rounded := float64(int(pct*10+0.5)) / 10
	return &rounded
}
