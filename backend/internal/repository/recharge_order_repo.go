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
	if order.PrepayID != nil {
		up.SetPrepayID(*order.PrepayID)
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

func (r *rechargeOrderRepository) ListByUserID(ctx context.Context, userID int64, req *service.ListRechargeOrdersRequest) (*service.ListRechargeOrdersResult, error) {
	query := r.client.RechargeOrder.Query().
		Where(rechargeorder.UserIDEQ(userID))

	// 状态筛选
	if req.Status != "" {
		query = query.Where(rechargeorder.StatusEQ(req.Status))
	}

	// 时间范围筛选
	if req.StartTime != nil {
		query = query.Where(rechargeorder.CreatedAtGTE(*req.StartTime))
	}
	if req.EndTime != nil {
		query = query.Where(rechargeorder.CreatedAtLTE(*req.EndTime))
	}

	// 获取总数
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, err
	}

	// 按创建时间倒序，分页
	orders, err := query.
		Order(dbent.Desc(rechargeorder.FieldCreatedAt)).
		Offset(req.Offset()).
		Limit(req.Limit()).
		All(ctx)
	if err != nil {
		return nil, err
	}

	// 转换结果
	result := make([]*service.RechargeOrder, len(orders))
	for i, order := range orders {
		result[i] = rechargeOrderEntityToService(order)
	}

	return &service.ListRechargeOrdersResult{
		Orders:     result,
		Pagination: paginationResultFromTotal(int64(total), req.PaginationParams),
	}, nil
}

// UpdateStatusWithCondition 使用乐观锁更新订单状态
// 只有当订单当前状态等于 expectedStatus 时才更新为 newStatus
// 返回受影响的行数，如果为 0 则表示状态已改变（并发冲突）
func (r *rechargeOrderRepository) UpdateStatusWithCondition(ctx context.Context, id int64, expectedStatus, newStatus, notes string) (int64, error) {
	client := clientFromContext(ctx, r.client)
	rowsAffected, err := client.RechargeOrder.Update().
		Where(
			rechargeorder.IDEQ(id),
			rechargeorder.StatusEQ(expectedStatus),
		).
		SetStatus(newStatus).
		SetNotes(notes).
		Save(ctx)
	if err != nil {
		return 0, err
	}
	return int64(rowsAffected), nil
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
		PrepayID:            m.PrepayID,
		ExpireAt:            m.ExpireAt,
		PaidAt:              m.PaidAt,
		Notes:               m.Notes,
		CreatedAt:           m.CreatedAt,
		UpdatedAt:           m.UpdatedAt,
	}
}
