package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestBuildRedisOptions(t *testing.T) {
	cfg := &config.Config{
		Redis: config.RedisConfig{
			Host:                "localhost",
			Port:                6379,
			Password:            "secret",
			DB:                  2,
			DialTimeoutSeconds:  5,
			ReadTimeoutSeconds:  3,
			WriteTimeoutSeconds: 4,
			PoolSize:            100,
			MinIdleConns:        10,
		},
	}

	opts := buildRedisOptions(cfg)
	require.Equal(t, "localhost:6379", opts.Addr)
	require.Equal(t, "secret", opts.Password)
	require.Equal(t, 2, opts.DB)
	require.Equal(t, 5*time.Second, opts.DialTimeout)
	require.Equal(t, 3*time.Second, opts.ReadTimeout)
	require.Equal(t, 4*time.Second, opts.WriteTimeout)
	require.Equal(t, 100, opts.PoolSize)
	require.Equal(t, 10, opts.MinIdleConns)
	require.Nil(t, opts.TLSConfig)
	require.Equal(t, defaultRedisPoolTimeout, opts.PoolTimeout)
	require.Equal(t, defaultRedisConnMaxIdleTime, opts.ConnMaxIdleTime)

	// Test case with TLS enabled
	cfgTLS := &config.Config{
		Redis: config.RedisConfig{
			Host:      "localhost",
			EnableTLS: true,
		},
	}
	optsTLS := buildRedisOptions(cfgTLS)
	require.NotNil(t, optsTLS.TLSConfig)
	require.Equal(t, "localhost", optsTLS.TLSConfig.ServerName)
}

func TestBuildRedisOptions_EnforcesPoolSizeHardLimit(t *testing.T) {
	cfg := &config.Config{
		Redis: config.RedisConfig{
			PoolSize:     8,
			MinIdleConns: 16,
		},
	}
	opts := buildRedisOptions(cfg)
	require.Equal(t, 8, opts.PoolSize)
	require.Equal(t, 8, opts.MinIdleConns)
}

func TestBuildRedisOptions_DefaultPoolSizeAndTimeouts(t *testing.T) {
	cfg := &config.Config{}
	opts := buildRedisOptions(cfg)
	require.Equal(t, defaultRedisPoolSize, opts.PoolSize)
	require.Equal(t, defaultRedisPoolTimeout, opts.PoolTimeout)
	require.Equal(t, defaultRedisConnMaxIdleTime, opts.ConnMaxIdleTime)
}

func TestEvaluateRedisPoolWarnings_ReportsAbsoluteMinIdle(t *testing.T) {
	warnings := evaluateRedisPoolWarnings(512, 128)
	require.NotEmpty(t, warnings)
	found := false
	for _, warning := range warnings {
		if warning == "min_idle_conns (128) exceeds conservative threshold 64" {
			found = true
			break
		}
	}
	require.True(t, found)
}

func TestSnapshotRedisPoolStatsFromStats(t *testing.T) {
	stats := redis.PoolStats{
		Hits:       42,
		Misses:     3,
		WaitCount:  7,
		Timeouts:   1,
		TotalConns: 20,
		IdleConns:  5,
	}
	snapshot := RedisPoolSnapshotFromStats(&stats, 40)
	require.Equal(t, RedisPoolSnapshot{
		Hits:                      42,
		Misses:                    3,
		Stalls:                    7,
		Timeouts:                  1,
		TotalConns:                20,
		IdleConns:                 5,
		UsagePercent:              float64PtrRedis(50),
		IdleReservationRatio:      float64PtrRedis(12.5),
		HitRatePercent:            float64PtrRedis(93.3),
		ConnectionPressurePercent: float64PtrRedis(50),
		PoolHeadroomPercent:       float64PtrRedis(50),
	}, snapshot)
}

func TestSnapshotRedisPoolStatsNilClient(t *testing.T) {
	require.Equal(t, RedisPoolSnapshot{}, SnapshotRedisPoolStats(nil))
}

func TestSnapshotDefaultRedisPoolStatsWithoutInit(t *testing.T) {
	defaultRedisClient.Store(nil)
	require.Equal(t, RedisPoolSnapshot{}, SnapshotDefaultRedisPoolStats())
}

func TestProbeRedisNil(t *testing.T) {
	require.ErrorIs(t, ProbeRedis(context.TODO(), nil), ErrRedisClientNotInitialized)
}

func TestProbeDefaultRedisWithoutInit(t *testing.T) {
	defaultRedisClient.Store(nil)
	require.ErrorIs(t, ProbeDefaultRedis(context.TODO()), ErrRedisClientNotInitialized)
}

func float64PtrRedis(v float64) *float64 { return &v }

func TestSummarizeRedisPressureTrend(t *testing.T) {
	base := RedisPoolSnapshot{
		Hits:                      100,
		Misses:                    10,
		Stalls:                    0,
		Timeouts:                  0,
		TotalConns:                50,
		IdleConns:                 10,
		UsagePercent:              float64PtrRedis(60),
		ConnectionPressurePercent: float64PtrRedis(60),
	}
	state, note := SummarizeRedisPressureTrend(base)
	require.Equal(t, "stable", state)
	require.Equal(t, "pressure normal", note)

	warningSnapshot := base
	warningSnapshot.Stalls = 5
	state, note = SummarizeRedisPressureTrend(warningSnapshot)
	require.Equal(t, "warning", state)
	require.Contains(t, note, "stalls=5")

	criticalSnapshot := base
	criticalSnapshot.Timeouts = 2
	state, note = SummarizeRedisPressureTrend(criticalSnapshot)
	require.Equal(t, "critical", state)
	require.Contains(t, note, "timeouts=2")
}
