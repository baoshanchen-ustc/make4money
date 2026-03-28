package repository

import (
	"context"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	dbcba "github.com/Wei-Shaw/sub2api/ent/copilotbudgetalert"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type copilotBudgetAlertRepository struct {
	client *dbent.Client
}

// NewCopilotBudgetAlertRepository returns a CopilotBudgetAlertRepository backed by Ent.
func NewCopilotBudgetAlertRepository(client *dbent.Client) service.CopilotBudgetAlertRepository {
	return &copilotBudgetAlertRepository{client: client}
}

// Upsert inserts or updates the alert config for the account.
// Conflicts on the unique account_id index are resolved by updating all fields.
func (r *copilotBudgetAlertRepository) Upsert(ctx context.Context, alert *service.CopilotBudgetAlert) error {
	client := clientFromContext(ctx, r.client)

	err := client.CopilotBudgetAlert.Create().
		SetAccountID(alert.AccountID).
		SetMonthlyBudget(alert.MonthlyBudget).
		SetAlertThreshold(alert.AlertThreshold).
		SetEnabled(alert.Enabled).
		SetNillableLastAlertedAt(alert.LastAlertedAt).
		OnConflictColumns(dbcba.FieldAccountID).
		UpdateNewValues().
		Exec(ctx)
	return err
}

// GetByAccountID returns the alert config for the given account, or nil if none exists.
func (r *copilotBudgetAlertRepository) GetByAccountID(
	ctx context.Context, accountID int64,
) (*service.CopilotBudgetAlert, error) {
	client := clientFromContext(ctx, r.client)

	row, err := client.CopilotBudgetAlert.Query().
		Where(dbcba.AccountID(accountID)).
		First(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return copilotBudgetAlertFromEnt(row), nil
}

// ListEnabled returns all alert configs that have enabled=true.
func (r *copilotBudgetAlertRepository) ListEnabled(ctx context.Context) ([]*service.CopilotBudgetAlert, error) {
	client := clientFromContext(ctx, r.client)

	rows, err := client.CopilotBudgetAlert.Query().
		Where(dbcba.Enabled(true)).
		All(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]*service.CopilotBudgetAlert, len(rows))
	for i, row := range rows {
		result[i] = copilotBudgetAlertFromEnt(row)
	}
	return result, nil
}

// copilotBudgetAlertFromEnt converts an Ent entity to a service struct.
func copilotBudgetAlertFromEnt(e *dbent.CopilotBudgetAlert) *service.CopilotBudgetAlert {
	return &service.CopilotBudgetAlert{
		ID:             e.ID,
		AccountID:      e.AccountID,
		MonthlyBudget:  e.MonthlyBudget,
		AlertThreshold: e.AlertThreshold,
		Enabled:        e.Enabled,
		LastAlertedAt:  e.LastAlertedAt,
		CreatedAt:      e.CreatedAt,
		UpdatedAt:      e.UpdatedAt,
	}
}
