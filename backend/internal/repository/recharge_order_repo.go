package repository

import (
	"context"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/rechargeorder"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type rechargeOrderRepository struct {
	client *dbent.Client
}

// NewRechargeOrderRepository 创建充值订单仓储
func NewRechargeOrderRepository(client *dbent.Client) service.RechargeOrderRepository {
	return &rechargeOrderRepository{client: client}
}

func (r *rechargeOrderRepository) Create(ctx context.Context, order *service.RechargeOrder) error {
	client := clientFromContext(ctx, r.client)
	created, err := client.RechargeOrder.Create().
		SetOrderNo(order.OrderNo).
		SetUserID(order.UserID).
		SetAmount(order.Amount).
		SetPaymentMethod(order.PaymentMethod).
		SetPaymentChannel(order.PaymentChannel).
		SetStatus(order.Status).
		SetExpireAt(order.ExpireAt).
		SetNotes(order.Notes).
		Save(ctx)
	if err != nil {
		return err
	}
	order.ID = created.ID
	order.CreatedAt = created.CreatedAt
	order.UpdatedAt = created.UpdatedAt
	return nil
}

func (r *rechargeOrderRepository) GetByID(ctx context.Context, id int64) (*service.RechargeOrder, error) {
	m, err := r.client.RechargeOrder.Query().
		Where(rechargeorder.IDEQ(id)).
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, service.ErrRechargeOrderNotFound
		}
		return nil, err
	}
	return rechargeOrderEntityToService(m), nil
}

func (r *rechargeOrderRepository) GetByOrderNo(ctx context.Context, orderNo string) (*service.RechargeOrder, error) {
	m, err := r.client.RechargeOrder.Query().
		Where(rechargeorder.OrderNoEQ(orderNo)).
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, service.ErrRechargeOrderNotFound
		}
		return nil, err
	}
	return rechargeOrderEntityToService(m), nil
}

func (r *rechargeOrderRepository) Update(ctx context.Context, order *service.RechargeOrder) error {
	client := clientFromContext(ctx, r.client)
	up := client.RechargeOrder.UpdateOneID(order.ID).
		SetStatus(order.Status).
		SetNotes(order.Notes)

	if order.WeChatTransactionID != nil {
		up.SetWechatTransactionID(*order.WeChatTransactionID)
	}
	if order.QRCodeURL != nil {
		up.SetQrcodeURL(*order.QRCodeURL)
	}
	if order.PaidAt != nil {
		up.SetPaidAt(*order.PaidAt)
	}

	updated, err := up.Save(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return service.ErrRechargeOrderNotFound
		}
		return err
	}
	order.UpdatedAt = updated.UpdatedAt
	return nil
}

func (r *rechargeOrderRepository) ExistsByOrderNo(ctx context.Context, orderNo string) (bool, error) {
	exists, err := r.client.RechargeOrder.Query().
		Where(rechargeorder.OrderNoEQ(orderNo)).
		Exist(ctx)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func rechargeOrderEntityToService(m *dbent.RechargeOrder) *service.RechargeOrder {
	if m == nil {
		return nil
	}
	return &service.RechargeOrder{
		ID:                  m.ID,
		OrderNo:             m.OrderNo,
		UserID:              m.UserID,
		Amount:              m.Amount,
		PaymentMethod:       m.PaymentMethod,
		PaymentChannel:      m.PaymentChannel,
		Status:              m.Status,
		WeChatTransactionID: m.WechatTransactionID,
		QRCodeURL:           m.QrcodeURL,
		ExpireAt:            m.ExpireAt,
		PaidAt:              m.PaidAt,
		Notes:               m.Notes,
		CreatedAt:           m.CreatedAt,
		UpdatedAt:           m.UpdatedAt,
	}
}
