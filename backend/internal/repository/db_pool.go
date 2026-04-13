package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

var defaultDBPool atomic.Pointer[sql.DB]

var ErrDBPoolNotInitialized = errors.New("database pool not initialized")

type dbPoolSettings struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

func buildDBPoolSettings(cfg *config.Config) dbPoolSettings {
	return dbPoolSettings{
		MaxOpenConns:    cfg.Database.MaxOpenConns,
		MaxIdleConns:    cfg.Database.MaxIdleConns,
		ConnMaxLifetime: time.Duration(cfg.Database.ConnMaxLifetimeMinutes) * time.Minute,
		ConnMaxIdleTime: time.Duration(cfg.Database.ConnMaxIdleTimeMinutes) * time.Minute,
	}
}

func applyDBPoolSettings(db *sql.DB, cfg *config.Config) {
	if db == nil || cfg == nil {
		return
	}

	defaultDBPool.Store(db)
	settings := buildDBPoolSettings(cfg)
	db.SetMaxOpenConns(settings.MaxOpenConns)
	db.SetMaxIdleConns(settings.MaxIdleConns)
	db.SetConnMaxLifetime(settings.ConnMaxLifetime)
	db.SetConnMaxIdleTime(settings.ConnMaxIdleTime)
	logDBPoolStartupWarnings(settings)
	logDBPoolRuntimeWarnings(db)
}

const (
	dbPoolStartMaxOpenWarningThreshold     = 500
	dbPoolStartConservativeOpenThreshold   = 128
	dbPoolStartConservativeIdleThreshold   = 64
	dbPoolStartIdleRatioWarningThreshold   = 0.9
	dbPoolRuntimeIdleRatioWarningThreshold = 0.85
	dbPoolRuntimeUsageWarningThreshold     = 0.85
)

type DBPoolSnapshot struct {
	OpenConnections      int      `json:"open_connections"`
	InUse                int      `json:"in_use"`
	Idle                 int      `json:"idle"`
	MaxOpenConnections   int      `json:"max_open_connections"`
	WaitCount            int64    `json:"wait_count"`
	WaitDurationMs       int64    `json:"wait_duration_ms"`
	UsagePercent         *float64 `json:"usage_percent,omitempty"`
	InUsePercent         *float64 `json:"in_use_percent,omitempty"`
	IdleReservationRatio *float64 `json:"idle_reservation_ratio,omitempty"`
}

func SnapshotDBPoolStats(db *sql.DB) DBPoolSnapshot {
	if db == nil {
		return DBPoolSnapshot{}
	}
	return dbPoolSnapshotFromStats(db.Stats())
}

func SnapshotDefaultDBPoolStats() DBPoolSnapshot {
	return SnapshotDBPoolStats(defaultDBPool.Load())
}

func ProbeDB(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return ErrDBPoolNotInitialized
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return db.PingContext(ctx)
}

func ProbeDefaultDB(ctx context.Context) error {
	return ProbeDB(ctx, defaultDBPool.Load())
}

func dbPoolSnapshotFromStats(stats sql.DBStats) DBPoolSnapshot {
	return DBPoolSnapshot{
		OpenConnections:      stats.OpenConnections,
		InUse:                stats.InUse,
		Idle:                 stats.Idle,
		MaxOpenConnections:   stats.MaxOpenConnections,
		WaitCount:            stats.WaitCount,
		WaitDurationMs:       stats.WaitDuration.Milliseconds(),
		UsagePercent:         ratioPercent(stats.OpenConnections, stats.MaxOpenConnections),
		InUsePercent:         ratioPercent(stats.InUse, stats.MaxOpenConnections),
		IdleReservationRatio: ratioPercent(stats.Idle, stats.MaxOpenConnections),
	}
}

func logDBPoolStartupWarnings(settings dbPoolSettings) {
	warnings := evaluateDBPoolConfigWarnings(settings)
	if len(warnings) == 0 {
		return
	}
	logger.WriteSinkEvent("warn", "repository.db_pool", "database pool configuration guard", map[string]any{
		"warnings":       warnings,
		"max_open_conns": settings.MaxOpenConns,
		"max_idle_conns": settings.MaxIdleConns,
		"conn_max_life":  settings.ConnMaxLifetime.String(),
		"conn_max_idle":  settings.ConnMaxIdleTime.String(),
	})
}

func logDBPoolRuntimeWarnings(db *sql.DB) {
	if db == nil {
		return
	}
	stats := db.Stats()
	warnings := evaluateDBPoolStatsWarnings(stats)
	if len(warnings) == 0 {
		return
	}
	logger.WriteSinkEvent("warn", "repository.db_pool", "database pool runtime guard", map[string]any{
		"warnings":         warnings,
		"open_connections": stats.OpenConnections,
		"idle_connections": stats.Idle,
		"max_open_conns":   stats.MaxOpenConnections,
		"in_use":           stats.InUse,
	})
}

func evaluateDBPoolConfigWarnings(settings dbPoolSettings) []string {
	var warnings []string
	if settings.MaxOpenConns > dbPoolStartMaxOpenWarningThreshold {
		warnings = append(warnings, fmt.Sprintf("max_open_conns (%d) exceeds %d safety threshold", settings.MaxOpenConns, dbPoolStartMaxOpenWarningThreshold))
	}
	if settings.MaxOpenConns > dbPoolStartConservativeOpenThreshold {
		warnings = append(warnings, fmt.Sprintf("max_open_conns (%d) exceeds conservative threshold %d; verify against postgres max_connections", settings.MaxOpenConns, dbPoolStartConservativeOpenThreshold))
	}
	if settings.MaxIdleConns > dbPoolStartConservativeIdleThreshold {
		warnings = append(warnings, fmt.Sprintf("max_idle_conns (%d) exceeds conservative threshold %d", settings.MaxIdleConns, dbPoolStartConservativeIdleThreshold))
	}
	if settings.MaxOpenConns > 0 && settings.MaxIdleConns > 0 {
		ratio := float64(settings.MaxIdleConns) / float64(settings.MaxOpenConns)
		if ratio >= dbPoolStartIdleRatioWarningThreshold {
			warnings = append(warnings, fmt.Sprintf("max_idle_conns (%d) is %.0f%% of max_open_conns", settings.MaxIdleConns, ratio*100))
		}
	}
	if settings.MaxIdleConns >= settings.MaxOpenConns && settings.MaxOpenConns > 0 {
		warnings = append(warnings, fmt.Sprintf("max_idle_conns (%d) >= max_open_conns (%d)", settings.MaxIdleConns, settings.MaxOpenConns))
	}
	return warnings
}

func evaluateDBPoolStatsWarnings(stats sql.DBStats) []string {
	var warnings []string
	if stats.MaxOpenConnections > 0 {
		ratio := float64(stats.Idle) / float64(stats.MaxOpenConnections)
		if ratio >= dbPoolRuntimeIdleRatioWarningThreshold {
			warnings = append(warnings, fmt.Sprintf("idle_connections (%.0f%%) close to max_open_conns", ratio*100))
		}
		openRatio := float64(stats.OpenConnections) / float64(stats.MaxOpenConnections)
		if openRatio >= dbPoolRuntimeUsageWarningThreshold {
			warnings = append(warnings, fmt.Sprintf("open_connections (%.0f%%) close to max_open_conns", openRatio*100))
		}
		if stats.InUse >= stats.MaxOpenConnections {
			warnings = append(warnings, "in_use connections reached max_open_conns")
		}
	}
	if stats.WaitCount > 0 {
		warnings = append(warnings, fmt.Sprintf("wait_count=%d indicates pool contention", stats.WaitCount))
	}
	return warnings
}

func ratioPercent(current, limit int) *float64 {
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
