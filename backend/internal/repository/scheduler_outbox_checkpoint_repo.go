package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type schedulerOutboxCheckpointRepository struct {
	db *sql.DB
}

func NewSchedulerOutboxCheckpointRepository(db *sql.DB) service.SchedulerOutboxCheckpointRepository {
	return &schedulerOutboxCheckpointRepository{db: db}
}

const schedulerOutboxCheckpointName = "primary"

func (r *schedulerOutboxCheckpointRepository) GetCheckpointWatermark(ctx context.Context) (int64, error) {
	if r == nil || r.db == nil {
		return 0, errors.New("checkpoint repository unavailable")
	}
	var watermark int64
	err := r.db.QueryRowContext(ctx, `
		SELECT watermark
		FROM scheduler_outbox_watermarks
		WHERE name = $1
		LIMIT 1
	`, schedulerOutboxCheckpointName).Scan(&watermark)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return watermark, err
}

func (r *schedulerOutboxCheckpointRepository) SetCheckpointWatermark(ctx context.Context, watermark int64) error {
	if r == nil || r.db == nil {
		return errors.New("checkpoint repository unavailable")
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO scheduler_outbox_watermarks (name, watermark, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (name) DO UPDATE SET watermark = EXCLUDED.watermark, updated_at = EXCLUDED.updated_at
	`, schedulerOutboxCheckpointName, watermark)
	return err
}
