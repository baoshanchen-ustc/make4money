package repository

import (
	"context"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type balanceLogRepository struct {
	client *dbent.Client
}

// NewBalanceLogRepository 创建余额日志仓储
func NewBalanceLogRepository(client *dbent.Client) service.BalanceLogRepository {
	return &balanceLogRepository{client: client}
}

func (r *balanceLogRepository) Create(ctx context.Context, log *service.BalanceLog) error {
	client := clientFromContext(ctx, r.client)
	builder := client.BalanceLog.Create().
		SetUserID(log.UserID).
		SetChangeType(log.ChangeType).
		SetAmount(log.Amount).
		SetBalanceBefore(log.BalanceBefore).
		SetBalanceAfter(log.BalanceAfter).
		SetDescription(log.Description).
		SetOperatorID(log.OperatorID).
		SetOperatorType(log.OperatorType)

	if log.RelatedOrderNo != nil {
		builder.SetRelatedOrderNo(*log.RelatedOrderNo)
	}

	created, err := builder.Save(ctx)
	if err != nil {
		return err
	}
	log.ID = created.ID
	log.CreatedAt = created.CreatedAt
	return nil
}
