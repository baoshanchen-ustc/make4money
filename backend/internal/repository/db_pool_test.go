package repository

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"

	_ "github.com/lib/pq"
)

func TestBuildDBPoolSettings(t *testing.T) {
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			MaxOpenConns:           50,
			MaxIdleConns:           10,
			ConnMaxLifetimeMinutes: 30,
			ConnMaxIdleTimeMinutes: 5,
		},
	}

	settings := buildDBPoolSettings(cfg)
	require.Equal(t, 50, settings.MaxOpenConns)
	require.Equal(t, 10, settings.MaxIdleConns)
	require.Equal(t, 30*time.Minute, settings.ConnMaxLifetime)
	require.Equal(t, 5*time.Minute, settings.ConnMaxIdleTime)
}

func TestApplyDBPoolSettings(t *testing.T) {
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			MaxOpenConns:           40,
			MaxIdleConns:           8,
			ConnMaxLifetimeMinutes: 15,
			ConnMaxIdleTimeMinutes: 3,
		},
	}

	db, err := sql.Open("postgres", "host=127.0.0.1 port=5432 user=postgres sslmode=disable")
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = db.Close()
	})

	applyDBPoolSettings(db, cfg)
	stats := db.Stats()
	require.Equal(t, 40, stats.MaxOpenConnections)
}

func TestEvaluateDBPoolConfigWarnings(t *testing.T) {
	warnings := evaluateDBPoolConfigWarnings(dbPoolSettings{
		MaxOpenConns: 600,
		MaxIdleConns: 550,
	})
	require.NotEmpty(t, warnings)
	contains := false
	conservativeOpen := false
	conservativeIdle := false
	for _, warn := range warnings {
		if strings.Contains(warn, "exceeds") {
			contains = true
		}
		if strings.Contains(warn, "verify against postgres max_connections") {
			conservativeOpen = true
		}
		if strings.Contains(warn, "max_idle_conns (550) exceeds conservative threshold") {
			conservativeIdle = true
		}
	}
	require.True(t, contains)
	require.True(t, conservativeOpen)
	require.True(t, conservativeIdle)
}

func TestEvaluateDBPoolStatsWarnings(t *testing.T) {
	warnings := evaluateDBPoolStatsWarnings(sql.DBStats{
		MaxOpenConnections: 100,
		OpenConnections:    90,
		InUse:              100,
		Idle:               90,
		WaitCount:          3,
	})
	require.NotEmpty(t, warnings)
}

func TestDBPoolSnapshotFromStats(t *testing.T) {
	snapshot := dbPoolSnapshotFromStats(sql.DBStats{
		MaxOpenConnections: 100,
		OpenConnections:    40,
		InUse:              25,
		Idle:               15,
		WaitCount:          8,
		WaitDuration:       3 * time.Second,
	})
	require.Equal(t, 40, snapshot.OpenConnections)
	require.Equal(t, 25, snapshot.InUse)
	require.Equal(t, 15, snapshot.Idle)
	require.Equal(t, int64(8), snapshot.WaitCount)
	require.Equal(t, int64(3000), snapshot.WaitDurationMs)
	require.NotNil(t, snapshot.UsagePercent)
	require.Equal(t, 40.0, *snapshot.UsagePercent)
	require.NotNil(t, snapshot.InUsePercent)
	require.Equal(t, 25.0, *snapshot.InUsePercent)
	require.NotNil(t, snapshot.IdleReservationRatio)
	require.Equal(t, 15.0, *snapshot.IdleReservationRatio)
}

func TestProbeDBNil(t *testing.T) {
	require.ErrorIs(t, ProbeDB(context.TODO(), nil), ErrDBPoolNotInitialized)
}

func TestProbeDefaultDBWithoutInit(t *testing.T) {
	defaultDBPool.Store(nil)
	require.ErrorIs(t, ProbeDefaultDB(context.TODO()), ErrDBPoolNotInitialized)
}
