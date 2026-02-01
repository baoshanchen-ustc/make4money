package repository

import (
	"context"
	"time"

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

// MarkExpiredOrders 批量将已过期的 pending 订单标记为 expired
// 查找 status='pending' 且 expire_at < now() 的订单，最多处理 limit 条
// 返回过期的订单号列表
func (r *rechargeOrderRepository) MarkExpiredOrders(ctx context.Context, limit int) ([]string, error) {
	client := clientFromContext(ctx, r.client)
	now := time.Now()

	// 先查询需要过期的订单，获取订单号
	orders, err := client.RechargeOrder.Query().
		Where(
			rechargeorder.StatusEQ(service.OrderStatusPending),
			rechargeorder.ExpireAtLT(now),
		).
		Limit(limit).
		Select(rechargeorder.FieldID, rechargeorder.FieldOrderNo).
		All(ctx)
	if err != nil {
		return nil, err
	}

	if len(orders) == 0 {
		return nil, nil
	}

	// 收集订单号和 ID
	orderNos := make([]string, len(orders))
	ids := make([]int64, len(orders))
	for i, order := range orders {
		orderNos[i] = order.OrderNo
		ids[i] = order.ID
	}

	// 批量更新这些订单
	_, err = client.RechargeOrder.Update().
		Where(rechargeorder.IDIn(ids...)).
		SetStatus(service.OrderStatusExpired).
		SetNotes("系统自动过期").
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return orderNos, nil
}

// AppendNotes 追加订单备注
func (r *rechargeOrderRepository) AppendNotes(ctx context.Context, orderNo, notes string) error {
	client := clientFromContext(ctx, r.client)

	// 先获取当前订单
	order, err := client.RechargeOrder.Query().
		Where(rechargeorder.OrderNoEQ(orderNo)).
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return service.ErrRechargeOrderNotFound
		}
		return err
	}

	// 追加备注
	newNotes := order.Notes
	if newNotes != "" {
		newNotes += "; "
	}
	newNotes += notes

	// 更新
	_, err = client.RechargeOrder.UpdateOneID(order.ID).
		SetNotes(newNotes).
		Save(ctx)
	return err
}

// GetPendingOrdersForCompensation 获取待补偿的订单
// 返回 status='pending' 且 created_at < thresholdTime 的订单，按创建时间升序
func (r *rechargeOrderRepository) GetPendingOrdersForCompensation(ctx context.Context, thresholdTime time.Time, limit int) ([]*service.RechargeOrder, error) {
	orders, err := r.client.RechargeOrder.Query().
		Where(
			rechargeorder.StatusEQ(service.OrderStatusPending),
			rechargeorder.CreatedAtLT(thresholdTime),
		).
		Order(dbent.Asc(rechargeorder.FieldCreatedAt)).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]*service.RechargeOrder, len(orders))
	for i, order := range orders {
		result[i] = rechargeOrderEntityToService(order)
	}
	return result, nil
}

// MarkOrderExpired 将指定订单标记为过期
func (r *rechargeOrderRepository) MarkOrderExpired(ctx context.Context, orderNo string) error {
	client := clientFromContext(ctx, r.client)
	_, err := client.RechargeOrder.Update().
		Where(
			rechargeorder.OrderNoEQ(orderNo),
			rechargeorder.StatusEQ(service.OrderStatusPending),
		).
		SetStatus(service.OrderStatusExpired).
		SetNotes("微信订单已关闭，同步更新").
		Save(ctx)
	return err
}

// MarkOrderFailed 将指定订单标记为失败
func (r *rechargeOrderRepository) MarkOrderFailed(ctx context.Context, orderNo string) error {
	client := clientFromContext(ctx, r.client)
	_, err := client.RechargeOrder.Update().
		Where(
			rechargeorder.OrderNoEQ(orderNo),
			rechargeorder.StatusEQ(service.OrderStatusPending),
		).
		SetStatus(service.OrderStatusFailed).
		SetNotes("微信支付失败，同步更新").
		Save(ctx)
	return err
}

// ListAll 获取所有订单（管理端使用）
func (r *rechargeOrderRepository) ListAll(ctx context.Context, req *service.ListRechargeOrdersRequest) (*service.ListRechargeOrdersResult, error) {
	query := r.client.RechargeOrder.Query()

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
		// 退款相关字段
		RefundNo:       m.RefundNo,
		RefundStatus:   m.RefundStatus,
		RefundedAt:     m.RefundedAt,
		RefundReason:   m.RefundReason,
		RefundAdminID:  m.RefundAdminID,
		WeChatRefundID: m.WechatRefundID,
	}
}

// UpdateRefundStatus 更新订单退款状态
func (r *rechargeOrderRepository) UpdateRefundStatus(ctx context.Context, orderNo string, refundNo, refundStatus, refundReason string, refundAdminID int64, wechatRefundID string, refundedAt *time.Time) error {
	client := clientFromContext(ctx, r.client)

	up := client.RechargeOrder.Update().
		Where(rechargeorder.OrderNoEQ(orderNo)).
		SetRefundNo(refundNo).
		SetRefundStatus(refundStatus).
		SetRefundReason(refundReason).
		SetRefundAdminID(refundAdminID)

	if wechatRefundID != "" {
		up.SetWechatRefundID(wechatRefundID)
	}
	if refundedAt != nil {
		up.SetRefundedAt(*refundedAt)
	}

	rowsAffected, err := up.Save(ctx)
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return service.ErrRechargeOrderNotFound
	}
	return nil
}

// UpdateRefundResult 更新退款结果（用于回调处理）
func (r *rechargeOrderRepository) UpdateRefundResult(ctx context.Context, refundNo string, refundStatus string, refundedAt *time.Time) error {
	client := clientFromContext(ctx, r.client)

	up := client.RechargeOrder.Update().
		Where(rechargeorder.RefundNoEQ(refundNo)).
		SetRefundStatus(refundStatus)

	if refundedAt != nil {
		up.SetRefundedAt(*refundedAt)
	}

	rowsAffected, err := up.Save(ctx)
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return service.ErrRechargeOrderNotFound
	}
	return nil
}

// GetByRefundNo 根据退款单号获取订单
func (r *rechargeOrderRepository) GetByRefundNo(ctx context.Context, refundNo string) (*service.RechargeOrder, error) {
	m, err := r.client.RechargeOrder.Query().
		Where(rechargeorder.RefundNoEQ(refundNo)).
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, service.ErrRechargeOrderNotFound
		}
		return nil, err
	}
	return rechargeOrderEntityToService(m), nil
}

// MarkOrderRefunded 将订单标记为已退款（支持事务上下文）
func (r *rechargeOrderRepository) MarkOrderRefunded(ctx context.Context, orderNo string, notes string) error {
	client := clientFromContext(ctx, r.client)
	_, err := client.RechargeOrder.Update().
		Where(
			rechargeorder.OrderNoEQ(orderNo),
			rechargeorder.StatusEQ(service.OrderStatusPaid),
		).
		SetStatus("refunded").
		SetNotes(notes).
		Save(ctx)
	return err
}
