package repository

import (
	"context"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/modelpricing"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type modelPricingRepository struct {
	client *dbent.Client
}

func NewModelPricingRepository(client *dbent.Client) service.ModelPricingRepository {
	return &modelPricingRepository{client: client}
}

func (r *modelPricingRepository) List(ctx context.Context) ([]service.ModelPricingEntry, error) {
	rows, err := r.client.ModelPricing.Query().
		Order(dbent.Asc(modelpricing.FieldModelKey)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]service.ModelPricingEntry, 0, len(rows))
	for _, row := range rows {
		out = append(out, entToServiceModelPricing(row))
	}
	return out, nil
}

func (r *modelPricingRepository) GetByKey(ctx context.Context, modelKey string) (*service.ModelPricingEntry, error) {
	row, err := r.client.ModelPricing.Query().
		Where(modelpricing.ModelKey(modelKey)).
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, service.ErrModelPricingNotFound
		}
		return nil, err
	}
	e := entToServiceModelPricing(row)
	return &e, nil
}

func (r *modelPricingRepository) GetByID(ctx context.Context, id int64) (*service.ModelPricingEntry, error) {
	row, err := r.client.ModelPricing.Get(ctx, id)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, service.ErrModelPricingNotFound
		}
		return nil, err
	}
	e := entToServiceModelPricing(row)
	return &e, nil
}

func (r *modelPricingRepository) Create(ctx context.Context, entry *service.ModelPricingEntry) (*service.ModelPricingEntry, error) {
	row, err := r.client.ModelPricing.Create().
		SetModelKey(entry.ModelKey).
		SetNillableDisplayName(nilIfEmpty(entry.DisplayName)).
		SetInputPricePerMillion(entry.InputPricePerMillion).
		SetOutputPricePerMillion(entry.OutputPricePerMillion).
		SetInputPricePerMillionPriority(entry.InputPricePerMillionPriority).
		SetOutputPricePerMillionPriority(entry.OutputPricePerMillionPriority).
		SetCacheReadPricePerMillion(entry.CacheReadPricePerMillion).
		SetCacheReadPricePerMillionPriority(entry.CacheReadPricePerMillionPriority).
		SetCacheCreationPricePerMillion(entry.CacheCreationPricePerMillion).
		SetEnabled(entry.Enabled).
		SetOverrideLitellm(entry.OverrideLitellm).
		SetNillableNote(nilIfEmpty(entry.Note)).
		Save(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, nil, service.ErrModelPricingExists)
	}
	e := entToServiceModelPricing(row)
	return &e, nil
}

func (r *modelPricingRepository) Update(ctx context.Context, id int64, entry *service.ModelPricingEntry) (*service.ModelPricingEntry, error) {
	row, err := r.client.ModelPricing.UpdateOneID(id).
		SetModelKey(entry.ModelKey).
		SetNillableDisplayName(nilIfEmpty(entry.DisplayName)).
		SetInputPricePerMillion(entry.InputPricePerMillion).
		SetOutputPricePerMillion(entry.OutputPricePerMillion).
		SetInputPricePerMillionPriority(entry.InputPricePerMillionPriority).
		SetOutputPricePerMillionPriority(entry.OutputPricePerMillionPriority).
		SetCacheReadPricePerMillion(entry.CacheReadPricePerMillion).
		SetCacheReadPricePerMillionPriority(entry.CacheReadPricePerMillionPriority).
		SetCacheCreationPricePerMillion(entry.CacheCreationPricePerMillion).
		SetEnabled(entry.Enabled).
		SetOverrideLitellm(entry.OverrideLitellm).
		SetNillableNote(nilIfEmpty(entry.Note)).
		Save(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrModelPricingNotFound, service.ErrModelPricingExists)
	}
	e := entToServiceModelPricing(row)
	return &e, nil
}

func (r *modelPricingRepository) Delete(ctx context.Context, id int64) error {
	err := r.client.ModelPricing.DeleteOneID(id).Exec(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return service.ErrModelPricingNotFound
		}
		return err
	}
	return nil
}

func entToServiceModelPricing(row *dbent.ModelPricing) service.ModelPricingEntry {
	return service.ModelPricingEntry{
		ID:                               row.ID,
		CreatedAt:                        row.CreatedAt,
		UpdatedAt:                        row.UpdatedAt,
		ModelKey:                         row.ModelKey,
		DisplayName:                      row.DisplayName,
		InputPricePerMillion:             row.InputPricePerMillion,
		OutputPricePerMillion:            row.OutputPricePerMillion,
		InputPricePerMillionPriority:     row.InputPricePerMillionPriority,
		OutputPricePerMillionPriority:    row.OutputPricePerMillionPriority,
		CacheReadPricePerMillion:         row.CacheReadPricePerMillion,
		CacheReadPricePerMillionPriority: row.CacheReadPricePerMillionPriority,
		CacheCreationPricePerMillion:     row.CacheCreationPricePerMillion,
		Enabled:                          row.Enabled,
		OverrideLitellm:                  row.OverrideLitellm,
		Note:                             row.Note,
	}
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
