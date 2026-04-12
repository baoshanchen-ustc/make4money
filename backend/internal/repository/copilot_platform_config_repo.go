package repository

import (
	"context"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/copilotplatformconfig"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type copilotPlatformConfigRepository struct {
	client *dbent.Client
}

// NewCopilotPlatformConfigRepository 创建平台配置仓储。
func NewCopilotPlatformConfigRepository(client *dbent.Client) service.CopilotPlatformConfigRepository {
	return &copilotPlatformConfigRepository{client: client}
}

func (r *copilotPlatformConfigRepository) GetAll(ctx context.Context) ([]service.CopilotPlatformConfigEntry, error) {
	// 从数据库拉取全部行，返回时按 AllCopilotPlanTypes 固定顺序重排。
	rows, err := r.client.CopilotPlatformConfig.Query().All(ctx)
	if err != nil {
		return nil, err
	}
	byPlanType := make(map[string]service.CopilotPlatformConfigEntry, len(rows))
	for _, row := range rows {
		byPlanType[row.PlanType] = entToServiceConfig(row)
	}
	out := make([]service.CopilotPlatformConfigEntry, 0, len(service.AllCopilotPlanTypes))
	for _, pt := range service.AllCopilotPlanTypes {
		if e, ok := byPlanType[pt]; ok {
			out = append(out, e)
		}
	}
	return out, nil
}

func (r *copilotPlatformConfigRepository) GetByPlanType(ctx context.Context, planType string) (*service.CopilotPlatformConfigEntry, error) {
	row, err := r.client.CopilotPlatformConfig.Query().
		Where(copilotplatformconfig.PlanType(planType)).
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, service.ErrCopilotPlatformConfigNotFound
		}
		return nil, err
	}
	e := entToServiceConfig(row)
	return &e, nil
}

func (r *copilotPlatformConfigRepository) Upsert(ctx context.Context, planType string, patch service.CopilotPlatformConfigPatch) (*service.CopilotPlatformConfigEntry, error) {
	// 先查出现有行（行由迁移预插入，始终存在）
	existing, err := r.client.CopilotPlatformConfig.Query().
		Where(copilotplatformconfig.PlanType(planType)).
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, service.ErrCopilotPlatformConfigNotFound
		}
		return nil, err
	}

	updater := r.client.CopilotPlatformConfig.UpdateOne(existing)

	if patch.SetMaxOutputTokens {
		if patch.MaxOutputTokens == nil {
			updater.ClearMaxOutputTokens()
		} else {
			updater.SetMaxOutputTokens(*patch.MaxOutputTokens)
		}
	}
	if patch.SetMaxBodyKB {
		if patch.MaxBodyKB == nil {
			updater.ClearMaxBodyKB()
		} else {
			updater.SetMaxBodyKB(*patch.MaxBodyKB)
		}
	}
	if patch.SetModelMapping {
		if patch.ModelMapping == nil {
			updater.ClearModelMapping()
		} else {
			updater.SetModelMapping(patch.ModelMapping)
		}
	}
	if patch.SetModelWhitelist {
		if patch.ModelWhitelist == nil {
			updater.ClearModelWhitelist()
		} else {
			updater.SetModelWhitelist(patch.ModelWhitelist)
		}
	}

	row, err := updater.Save(ctx)
	if err != nil {
		return nil, err
	}
	e := entToServiceConfig(row)
	return &e, nil
}

func entToServiceConfig(row *dbent.CopilotPlatformConfig) service.CopilotPlatformConfigEntry {
	return service.CopilotPlatformConfigEntry{
		ID:              row.ID,
		PlanType:        row.PlanType,
		MaxOutputTokens: row.MaxOutputTokens,
		MaxBodyKB:       row.MaxBodyKB,
		ModelMapping:    row.ModelMapping,
		ModelWhitelist:  row.ModelWhitelist,
	}
}
