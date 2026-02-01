package repository

import (
	"context"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/subscriptionorder"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type subscriptionOrderRepository struct {
	client *dbent.Client
}

// NewSubscriptionOrderRepository 创建订阅订单仓储
func NewSubscriptionOrderRepository(client *dbent.Client) service.SubscriptionOrderRepository {
	return &subscriptionOrderRepository{client: client}
}

func (r *subscriptionOrderRepository) Create(ctx context.Context, order *service.SubscriptionOrder) error {
	client := clientFromContext(ctx, r.client)
	created, err := client.SubscriptionOrder.Create().
		SetOrderNo(order.OrderNo).
		SetUserID(order.UserID).
		SetGroupID(order.GroupID).
		SetAmount(order.Amount).
		SetValidityDays(order.ValidityDays).
		SetPaymentMethod(order.PaymentMethod).
		SetPaymentChannel(order.PaymentChannel).
		SetStatus(order.Status).
		SetExpireAt(order.ExpireAt).
		Save(ctx)
	if err != nil {
		return err
	}
	order.ID = created.ID
	order.CreatedAt = created.CreatedAt
	order.UpdatedAt = created.UpdatedAt
	return nil
}

func (r *subscriptionOrderRepository) GetByID(ctx context.Context, id int64) (*service.SubscriptionOrder, error) {
	m, err := r.client.SubscriptionOrder.Query().
		Where(subscriptionorder.IDEQ(id)).
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, service.ErrSubscriptionOrderNotFound
		}
		return nil, err
	}
	return subscriptionOrderEntityToService(m), nil
}

func (r *subscriptionOrderRepository) GetByOrderNo(ctx context.Context, orderNo string) (*service.SubscriptionOrder, error) {
	m, err := r.client.SubscriptionOrder.Query().
		Where(subscriptionorder.OrderNoEQ(orderNo)).
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, service.ErrSubscriptionOrderNotFound
		}
		return nil, err
	}
	return subscriptionOrderEntityToService(m), nil
}

func (r *subscriptionOrderRepository) GetByOrderNoWithGroup(ctx context.Context, orderNo string) (*service.SubscriptionOrder, error) {
	m, err := r.client.SubscriptionOrder.Query().
		Where(subscriptionorder.OrderNoEQ(orderNo)).
		WithGroup().
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, service.ErrSubscriptionOrderNotFound
		}
		return nil, err
	}
	result := subscriptionOrderEntityToService(m)
	if m.Edges.Group != nil {
		result.Group = groupEntityToService(m.Edges.Group)
	}
	return result, nil
}

func (r *subscriptionOrderRepository) Update(ctx context.Context, order *service.SubscriptionOrder) error {
	client := clientFromContext(ctx, r.client)
	up := client.SubscriptionOrder.UpdateOneID(order.ID).
		SetStatus(order.Status)

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
			return service.ErrSubscriptionOrderNotFound
		}
		return err
	}
	order.UpdatedAt = updated.UpdatedAt
	return nil
}

func (r *subscriptionOrderRepository) ExistsByOrderNo(ctx context.Context, orderNo string) (bool, error) {
	exists, err := r.client.SubscriptionOrder.Query().
		Where(subscriptionorder.OrderNoEQ(orderNo)).
		Exist(ctx)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (r *subscriptionOrderRepository) ListByUserID(ctx context.Context, userID int64, req *service.ListSubscriptionOrdersRequest) (*service.ListSubscriptionOrdersResult, error) {
	query := r.client.SubscriptionOrder.Query().
		Where(subscriptionorder.UserIDEQ(userID))

	if req.Status != "" {
		query = query.Where(subscriptionorder.StatusEQ(req.Status))
	}

	if req.StartTime != nil {
		query = query.Where(subscriptionorder.CreatedAtGTE(*req.StartTime))
	}
	if req.EndTime != nil {
		query = query.Where(subscriptionorder.CreatedAtLTE(*req.EndTime))
	}

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, err
	}

	orders, err := query.
		WithGroup().
		Order(dbent.Desc(subscriptionorder.FieldCreatedAt)).
		Offset(req.Offset()).
		Limit(req.Limit()).
		All(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]*service.SubscriptionOrder, len(orders))
	for i, order := range orders {
		result[i] = subscriptionOrderEntityToService(order)
		if order.Edges.Group != nil {
			result[i].Group = groupEntityToService(order.Edges.Group)
		}
	}

	return &service.ListSubscriptionOrdersResult{
		Orders:     result,
		Pagination: paginationResultFromTotal(int64(total), req.PaginationParams),
	}, nil
}

func (r *subscriptionOrderRepository) ListAll(ctx context.Context, req *service.ListSubscriptionOrdersRequest) (*service.ListSubscriptionOrdersResult, error) {
	query := r.client.SubscriptionOrder.Query()

	if req.Status != "" {
		query = query.Where(subscriptionorder.StatusEQ(req.Status))
	}

	if req.StartTime != nil {
		query = query.Where(subscriptionorder.CreatedAtGTE(*req.StartTime))
	}
	if req.EndTime != nil {
		query = query.Where(subscriptionorder.CreatedAtLTE(*req.EndTime))
	}

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, err
	}

	orders, err := query.
		WithGroup().
		Order(dbent.Desc(subscriptionorder.FieldCreatedAt)).
		Offset(req.Offset()).
		Limit(req.Limit()).
		All(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]*service.SubscriptionOrder, len(orders))
	for i, order := range orders {
		result[i] = subscriptionOrderEntityToService(order)
		if order.Edges.Group != nil {
			result[i].Group = groupEntityToService(order.Edges.Group)
		}
	}

	return &service.ListSubscriptionOrdersResult{
		Orders:     result,
		Pagination: paginationResultFromTotal(int64(total), req.PaginationParams),
	}, nil
}

func (r *subscriptionOrderRepository) UpdateStatusWithCondition(ctx context.Context, id int64, expectedStatus, newStatus string) (int64, error) {
	client := clientFromContext(ctx, r.client)
	rowsAffected, err := client.SubscriptionOrder.Update().
		Where(
			subscriptionorder.IDEQ(id),
			subscriptionorder.StatusEQ(expectedStatus),
		).
		SetStatus(newStatus).
		Save(ctx)
	if err != nil {
		return 0, err
	}
	return int64(rowsAffected), nil
}

func (r *subscriptionOrderRepository) MarkExpiredOrders(ctx context.Context, limit int) ([]string, error) {
	client := clientFromContext(ctx, r.client)
	now := time.Now()

	orders, err := client.SubscriptionOrder.Query().
		Where(
			subscriptionorder.StatusEQ(service.OrderStatusPending),
			subscriptionorder.ExpireAtLT(now),
		).
		Limit(limit).
		Select(subscriptionorder.FieldID, subscriptionorder.FieldOrderNo).
		All(ctx)
	if err != nil {
		return nil, err
	}

	if len(orders) == 0 {
		return nil, nil
	}

	orderNos := make([]string, len(orders))
	ids := make([]int64, len(orders))
	for i, order := range orders {
		orderNos[i] = order.OrderNo
		ids[i] = order.ID
	}

	_, err = client.SubscriptionOrder.Update().
		Where(subscriptionorder.IDIn(ids...)).
		SetStatus(service.OrderStatusExpired).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return orderNos, nil
}

func (r *subscriptionOrderRepository) GetPendingOrdersForCompensation(ctx context.Context, thresholdTime time.Time, limit int) ([]*service.SubscriptionOrder, error) {
	orders, err := r.client.SubscriptionOrder.Query().
		Where(
			subscriptionorder.StatusEQ(service.OrderStatusPending),
			subscriptionorder.CreatedAtLT(thresholdTime),
		).
		Order(dbent.Asc(subscriptionorder.FieldCreatedAt)).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]*service.SubscriptionOrder, len(orders))
	for i, order := range orders {
		result[i] = subscriptionOrderEntityToService(order)
	}
	return result, nil
}

func (r *subscriptionOrderRepository) MarkOrderExpired(ctx context.Context, orderNo string) error {
	client := clientFromContext(ctx, r.client)
	_, err := client.SubscriptionOrder.Update().
		Where(
			subscriptionorder.OrderNoEQ(orderNo),
			subscriptionorder.StatusEQ(service.OrderStatusPending),
		).
		SetStatus(service.OrderStatusExpired).
		Save(ctx)
	return err
}

func (r *subscriptionOrderRepository) MarkOrderFailed(ctx context.Context, orderNo string) error {
	client := clientFromContext(ctx, r.client)
	_, err := client.SubscriptionOrder.Update().
		Where(
			subscriptionorder.OrderNoEQ(orderNo),
			subscriptionorder.StatusEQ(service.OrderStatusPending),
		).
		SetStatus(service.OrderStatusFailed).
		Save(ctx)
	return err
}

func subscriptionOrderEntityToService(m *dbent.SubscriptionOrder) *service.SubscriptionOrder {
	if m == nil {
		return nil
	}
	return &service.SubscriptionOrder{
		ID:                  m.ID,
		OrderNo:             m.OrderNo,
		UserID:              m.UserID,
		GroupID:             m.GroupID,
		Amount:              m.Amount,
		ValidityDays:        m.ValidityDays,
		PaymentMethod:       m.PaymentMethod,
		PaymentChannel:      m.PaymentChannel,
		Status:              m.Status,
		WeChatTransactionID: m.WechatTransactionID,
		QRCodeURL:           m.QrcodeURL,
		PrepayID:            m.PrepayID,
		ExpireAt:            m.ExpireAt,
		PaidAt:              m.PaidAt,
		CreatedAt:           m.CreatedAt,
		UpdatedAt:           m.UpdatedAt,
	}
}
