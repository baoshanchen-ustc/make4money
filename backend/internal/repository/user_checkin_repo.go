package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type userCheckInRepository struct {
	client *dbent.Client
	sql    sqlExecutor
}

func NewUserCheckInRepository(client *dbent.Client, sqlDB *sql.DB) service.CheckInRepository {
	return &userCheckInRepository{
		client: client,
		sql:    sqlDB,
	}
}

func (r *userCheckInRepository) TryCreate(ctx context.Context, userID int64, checkInDate string, rewardAmount float64, createdAt time.Time) (*service.UserCheckIn, bool, error) {
	executor := sqlExecutorFromContext(ctx, r.sql, r.client)
	row := &service.UserCheckIn{}
	err := scanSingleRow(ctx, executor, `
		INSERT INTO user_checkins (user_id, checkin_date, reward_amount, created_at)
		VALUES ($1, $2::date, $3, $4)
		ON CONFLICT (user_id, checkin_date) DO NOTHING
		RETURNING id, user_id, TO_CHAR(checkin_date, 'YYYY-MM-DD'), reward_amount, created_at
	`, []any{userID, checkInDate, rewardAmount, createdAt}, &row.ID, &row.UserID, &row.CheckInDate, &row.RewardAmount, &row.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return row, true, nil
}

func (r *userCheckInRepository) GetByUserAndDate(ctx context.Context, userID int64, checkInDate string) (*service.UserCheckIn, bool, error) {
	executor := sqlExecutorFromContext(ctx, r.sql, r.client)
	row := &service.UserCheckIn{}
	err := scanSingleRow(ctx, executor, `
		SELECT id, user_id, TO_CHAR(checkin_date, 'YYYY-MM-DD'), reward_amount, created_at
		FROM user_checkins
		WHERE user_id = $1 AND checkin_date = $2::date
		LIMIT 1
	`, []any{userID, checkInDate}, &row.ID, &row.UserID, &row.CheckInDate, &row.RewardAmount, &row.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return row, true, nil
}

func (r *userCheckInRepository) GetLatestByUser(ctx context.Context, userID int64) (*service.UserCheckIn, bool, error) {
	executor := sqlExecutorFromContext(ctx, r.sql, r.client)
	row := &service.UserCheckIn{}
	err := scanSingleRow(ctx, executor, `
		SELECT id, user_id, TO_CHAR(checkin_date, 'YYYY-MM-DD'), reward_amount, created_at
		FROM user_checkins
		WHERE user_id = $1
		ORDER BY checkin_date DESC, created_at DESC
		LIMIT 1
	`, []any{userID}, &row.ID, &row.UserID, &row.CheckInDate, &row.RewardAmount, &row.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return row, true, nil
}

func (r *userCheckInRepository) ListRecentByUser(ctx context.Context, userID int64, limit int) ([]service.UserCheckIn, error) {
	if limit <= 0 {
		limit = 10
	}
	executor := sqlExecutorFromContext(ctx, r.sql, r.client)
	rows, err := executor.QueryContext(ctx, `
		SELECT id, user_id, TO_CHAR(checkin_date, 'YYYY-MM-DD'), reward_amount, created_at
		FROM user_checkins
		WHERE user_id = $1
		ORDER BY checkin_date DESC, created_at DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	return scanUserCheckIns(rows)
}

func (r *userCheckInRepository) ListByUserPaginated(ctx context.Context, userID int64, params pagination.PaginationParams) ([]service.UserCheckIn, *pagination.PaginationResult, error) {
	executor := sqlExecutorFromContext(ctx, r.sql, r.client)
	var total int64
	if err := scanSingleRow(ctx, executor, `
		SELECT COUNT(*)
		FROM user_checkins
		WHERE user_id = $1
	`, []any{userID}, &total); err != nil {
		return nil, nil, err
	}

	rows, err := executor.QueryContext(ctx, `
		SELECT id, user_id, TO_CHAR(checkin_date, 'YYYY-MM-DD'), reward_amount, created_at
		FROM user_checkins
		WHERE user_id = $1
		ORDER BY checkin_date DESC, created_at DESC
		OFFSET $2
		LIMIT $3
	`, userID, params.Offset(), params.Limit())
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rows.Close() }()

	items, err := scanUserCheckIns(rows)
	if err != nil {
		return nil, nil, err
	}
	return items, paginationResultFromTotal(total, params), nil
}

func (r *userCheckInRepository) CountByUser(ctx context.Context, userID int64) (int64, error) {
	executor := sqlExecutorFromContext(ctx, r.sql, r.client)
	var total int64
	if err := scanSingleRow(ctx, executor, `
		SELECT COUNT(*)
		FROM user_checkins
		WHERE user_id = $1
	`, []any{userID}, &total); err != nil {
		return 0, err
	}
	return total, nil
}

func (r *userCheckInRepository) SumRewardByUser(ctx context.Context, userID int64) (float64, error) {
	executor := sqlExecutorFromContext(ctx, r.sql, r.client)
	var total float64
	if err := scanSingleRow(ctx, executor, `
		SELECT COALESCE(SUM(reward_amount), 0)
		FROM user_checkins
		WHERE user_id = $1
	`, []any{userID}, &total); err != nil {
		return 0, err
	}
	return total, nil
}

func scanUserCheckIns(rows *sql.Rows) ([]service.UserCheckIn, error) {
	items := make([]service.UserCheckIn, 0)
	for rows.Next() {
		var item service.UserCheckIn
		if err := rows.Scan(&item.ID, &item.UserID, &item.CheckInDate, &item.RewardAmount, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func sqlExecutorFromContext(ctx context.Context, fallback sqlExecutor, client *dbent.Client) sqlExecutor {
	if tx := dbent.TxFromContext(ctx); tx != nil {
		if exec, ok := tx.Client().Driver().(sqlExecutor); ok {
			return exec
		}
	}
	if client != nil {
		if exec, ok := clientFromContext(ctx, client).Driver().(sqlExecutor); ok {
			return exec
		}
	}
	if fallback == nil {
		panic(fmt.Sprintf("nil sql executor for context %T", ctx))
	}
	return fallback
}
