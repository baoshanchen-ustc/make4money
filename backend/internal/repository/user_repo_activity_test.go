package repository

import (
	"context"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsqldriver "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/stretchr/testify/require"
)

func TestUserRepositoryLoadUserActivityTimestamps(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	client := dbent.NewClient(dbent.Driver(entsqldriver.OpenDB(dialect.Postgres, db)))
	defer client.Close()

	repo := &userRepository{client: client, sql: db}
	now := time.Now().UTC().Truncate(time.Second)
	lastUsed := now.Add(-15 * time.Minute)

	mock.ExpectQuery("SELECT .*id.*,.*last_login_at.* FROM .*users.* WHERE .* IN .*").
		WithArgs(int64(11), int64(12)).
		WillReturnRows(
			sqlmock.NewRows([]string{"id", "last_login_at"}).
				AddRow(int64(11), now).
				AddRow(int64(12), nil),
		)
	mock.ExpectQuery("SELECT .*\"user_id\".*, MAX\\(created_at\\) AS last_used_at.* FROM .*\"usage_logs\".* WHERE .* IN .* GROUP BY .*\"user_id\".*").
		WithArgs(int64(11), int64(12)).
		WillReturnRows(
			sqlmock.NewRows([]string{"user_id", "last_used_at"}).
				AddRow(int64(11), lastUsed),
		)

	result, err := repo.loadUserActivityTimestamps(context.Background(), []int64{11, 12})
	require.NoError(t, err)
	require.Len(t, result, 2)
	require.NotNil(t, result[11].LastLoginAt)
	require.NotNil(t, result[11].LastUsedAt)
	require.WithinDuration(t, now, *result[11].LastLoginAt, time.Second)
	require.WithinDuration(t, lastUsed, *result[11].LastUsedAt, time.Second)
	require.Nil(t, result[12].LastLoginAt)
	require.Nil(t, result[12].LastUsedAt)
	require.NoError(t, mock.ExpectationsWereMet())
}
