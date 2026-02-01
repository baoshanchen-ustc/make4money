package repository

import (
	"context"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/balancelog"
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

// GetByOrderNo 根据订单号查询余额日志
func (r *balanceLogRepository) GetByOrderNo(ctx context.Context, orderNo string) ([]*service.BalanceLog, error) {
	logs, err := r.client.BalanceLog.Query().
		Where(balancelog.RelatedOrderNoEQ(orderNo)).
		Order(dbent.Desc(balancelog.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]*service.BalanceLog, len(logs))
	for i, log := range logs {
		var relatedOrderNo *string
		if log.RelatedOrderNo != nil {
			relatedOrderNo = log.RelatedOrderNo
		}
		result[i] = &service.BalanceLog{
			ID:             log.ID,
			UserID:         log.UserID,
			ChangeType:     log.ChangeType,
			Amount:         log.Amount,
			BalanceBefore:  log.BalanceBefore,
			BalanceAfter:   log.BalanceAfter,
			RelatedOrderNo: relatedOrderNo,
			Description:    log.Description,
			OperatorID:     log.OperatorID,
			OperatorType:   log.OperatorType,
			CreatedAt:      log.CreatedAt,
		}
	}
	return result, nil
}
