package service

import (
	"context"
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestOpsCleanupRunCleanupOnceSnapshotsMaxRowsWithoutRetention(t *testing.T) {
	resetCleanupStatsForTest()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectQuery("SELECT COALESCE\\(st.n_live_tup::bigint, c.reltuples::bigint, 0\\)").
		WithArgs("ops_error_logs").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(1200)))
	mock.ExpectQuery("SELECT COALESCE\\(st.n_live_tup::bigint, c.reltuples::bigint, 0\\)").
		WithArgs("ops_system_logs").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(3400)))

	svc := &OpsCleanupService{
		db: db,
		cfg: &config.Config{
			Ops: config.OpsConfig{
				Cleanup: config.OpsCleanupConfig{
					Enabled:               true,
					ErrorLogRetentionDays: 0,
					SystemLogMaxRows:      3000,
					ErrorLogMaxRows:       1000,
					MaxRowsEnabled:        true,
					MaxRowsDryRun:         true,
				},
			},
		},
	}

	_, err = svc.runCleanupOnce(context.Background())
	require.NoError(t, err)

	stats := SnapshotCleanupStats()
	require.Equal(t, int64(3400), stats.SystemLogRows)
	require.Equal(t, int64(1200), stats.ErrorLogRows)
	require.Equal(t, int64(3000), stats.SystemLogLimit)
	require.Equal(t, int64(1000), stats.ErrorLogLimit)
	require.True(t, stats.MaxRowsEnabled)
	require.True(t, stats.MaxRowsDryRun)
	require.True(t, stats.MaxRowsHit)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestOpsCleanupRunCleanupOnceResetsMaxRowsHitPerRun(t *testing.T) {
	resetCleanupStatsForTest()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectQuery("SELECT COALESCE\\(st.n_live_tup::bigint, c.reltuples::bigint, 0\\)").
		WithArgs("ops_error_logs").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(1200)))
	mock.ExpectQuery("SELECT COALESCE\\(st.n_live_tup::bigint, c.reltuples::bigint, 0\\)").
		WithArgs("ops_system_logs").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(3400)))
	mock.ExpectQuery("SELECT COALESCE\\(st.n_live_tup::bigint, c.reltuples::bigint, 0\\)").
		WithArgs("ops_error_logs").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(800)))
	mock.ExpectQuery("SELECT COALESCE\\(st.n_live_tup::bigint, c.reltuples::bigint, 0\\)").
		WithArgs("ops_system_logs").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(2000)))

	svc := &OpsCleanupService{
		db: db,
		cfg: &config.Config{
			Ops: config.OpsConfig{
				Cleanup: config.OpsCleanupConfig{
					Enabled:               true,
					ErrorLogRetentionDays: 0,
					SystemLogMaxRows:      3000,
					ErrorLogMaxRows:       1000,
					MaxRowsEnabled:        true,
					MaxRowsDryRun:         true,
				},
			},
		},
	}

	_, err = svc.runCleanupOnce(context.Background())
	require.NoError(t, err)
	require.True(t, SnapshotCleanupStats().MaxRowsHit)

	_, err = svc.runCleanupOnce(context.Background())
	require.NoError(t, err)
	require.False(t, SnapshotCleanupStats().MaxRowsHit)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBuildDeleteOldRowsByIDQuery_UsesCTIDBatchDelete(t *testing.T) {
	query := buildDeleteOldRowsByIDQuery("ops_system_logs", "created_at", false)
	require.Contains(t, query, "SELECT ctid FROM ops_system_logs")
	require.Contains(t, query, "ORDER BY created_at ASC, id ASC")
	require.Contains(t, query, "DELETE FROM ops_system_logs AS t")
	require.Contains(t, query, "WHERE t.ctid = batch.ctid")
	require.NotContains(t, strings.ToLower(query), "where id in")
}

func TestBuildDeleteOldRowsByIDQuery_CastsDateCutoffWhenRequested(t *testing.T) {
	query := buildDeleteOldRowsByIDQuery("ops_metrics_daily", "bucket_date", true)
	require.Contains(t, query, "bucket_date < $1::date")
	require.Contains(t, query, "ORDER BY bucket_date ASC, id ASC")
}
