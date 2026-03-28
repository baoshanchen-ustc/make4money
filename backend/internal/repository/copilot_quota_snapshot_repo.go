package repository

import (
	"context"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	dbcqs "github.com/Wei-Shaw/sub2api/ent/copilotquotasnapshot"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type copilotQuotaSnapshotRepository struct {
	client *dbent.Client
}

// NewCopilotQuotaSnapshotRepository returns a CopilotQuotaSnapshotRepository backed by Ent.
func NewCopilotQuotaSnapshotRepository(client *dbent.Client) service.CopilotQuotaSnapshotRepository {
	return &copilotQuotaSnapshotRepository{client: client}
}

// Upsert inserts or updates the snapshot for (account_id, snapshot_date).
// On conflict the quota fields are overwritten with the new values.
func (r *copilotQuotaSnapshotRepository) Upsert(ctx context.Context, snap *service.CopilotQuotaSnapshot) error {
	client := clientFromContext(ctx, r.client)

	err := client.CopilotQuotaSnapshot.Create().
		SetAccountID(snap.AccountID).
		SetSnapshotDate(snap.SnapshotDate).
		SetNillablePlanType(snap.PlanType).
		SetPremiumEntitlement(snap.PremiumEntitlement).
		SetPremiumRemaining(snap.PremiumRemaining).
		SetPremiumUsed(snap.PremiumUsed).
		SetPremiumOverage(snap.PremiumOverage).
		SetUnlimited(snap.Unlimited).
		OnConflictColumns(dbcqs.FieldAccountID, dbcqs.FieldSnapshotDate).
		UpdateNewValues().
		Exec(ctx)
	return err
}

// ListByAccountID returns snapshots for the account ordered by snapshot_date ASC.
// Pass limit=0 to return all rows.
func (r *copilotQuotaSnapshotRepository) ListByAccountID(
	ctx context.Context, accountID int64, limit int,
) ([]*service.CopilotQuotaSnapshot, error) {
	client := clientFromContext(ctx, r.client)

	q := client.CopilotQuotaSnapshot.Query().
		Where(dbcqs.AccountID(accountID)).
		Order(dbent.Asc(dbcqs.FieldSnapshotDate))

	if limit > 0 {
		q = q.Limit(limit)
	}

	rows, err := q.All(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]*service.CopilotQuotaSnapshot, len(rows))
	for i, row := range rows {
		result[i] = copilotQuotaSnapshotFromEnt(row)
	}
	return result, nil
}

// GetLatestByAccountID returns the most recent snapshot for the account, or nil.
func (r *copilotQuotaSnapshotRepository) GetLatestByAccountID(
	ctx context.Context, accountID int64,
) (*service.CopilotQuotaSnapshot, error) {
	client := clientFromContext(ctx, r.client)

	row, err := client.CopilotQuotaSnapshot.Query().
		Where(dbcqs.AccountID(accountID)).
		Order(dbent.Desc(dbcqs.FieldSnapshotDate)).
		First(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return copilotQuotaSnapshotFromEnt(row), nil
}

// copilotQuotaSnapshotFromEnt converts an Ent entity to a service struct.
func copilotQuotaSnapshotFromEnt(e *dbent.CopilotQuotaSnapshot) *service.CopilotQuotaSnapshot {
	return &service.CopilotQuotaSnapshot{
		ID:                 e.ID,
		AccountID:          e.AccountID,
		SnapshotDate:       e.SnapshotDate,
		PlanType:           e.PlanType,
		PremiumEntitlement: e.PremiumEntitlement,
		PremiumRemaining:   e.PremiumRemaining,
		PremiumUsed:        e.PremiumUsed,
		PremiumOverage:     e.PremiumOverage,
		Unlimited:          e.Unlimited,
		CreatedAt:          e.CreatedAt,
	}
}
