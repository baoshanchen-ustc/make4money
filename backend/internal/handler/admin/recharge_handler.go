package admin

import (
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// RechargeHandler 管理端充值处理器
type RechargeHandler struct {
	rechargeOrderService *service.RechargeOrderService
	balanceLogRepo       service.BalanceLogRepository
}

// NewRechargeHandler 创建管理端充值处理器
func NewRechargeHandler(
	rechargeOrderService *service.RechargeOrderService,
	balanceLogRepo service.BalanceLogRepository,
) *RechargeHandler {
	return &RechargeHandler{
		rechargeOrderService: rechargeOrderService,
		balanceLogRepo:       balanceLogRepo,
	}
}

// AdminOrderDetailResponse 管理端订单详情响应
type AdminOrderDetailResponse struct {
	ID             int64      `json:"id"`
	OrderNo        string     `json:"order_no"`
	UserID         int64      `json:"user_id"`
	Amount         float64    `json:"amount"`
	Status         string     `json:"status"`
	PaymentMethod  string     `json:"payment_method"`
	PaymentChannel string     `json:"payment_channel"`
	TransactionID  *string    `json:"transaction_id,omitempty"`
	ExpireAt       time.Time  `json:"expire_at"`
	PaidAt         *time.Time `json:"paid_at,omitempty"`
	RefundNo       *string    `json:"refund_no,omitempty"`
	RefundStatus   *string    `json:"refund_status,omitempty"`
	RefundedAt     *time.Time `json:"refunded_at,omitempty"`
	RefundReason   *string    `json:"refund_reason,omitempty"`
	RefundAdminID  *int64     `json:"refund_admin_id,omitempty"`
	Notes          string     `json:"notes,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// GetOrder 获取订单详情
// GET /api/v1/admin/recharge/orders/:order_no
func (h *RechargeHandler) GetOrder(c *gin.Context) {
	orderNo := c.Param("order_no")
	if orderNo == "" {
		response.BadRequest(c, "订单号不能为空")
		return
	}

	order, err := h.rechargeOrderService.GetOrder(c.Request.Context(), orderNo)
	if err != nil {
		if !response.ErrorFrom(c, err) {
			response.InternalError(c, "查询订单失败")
		}
		return
	}

	response.Success(c, AdminOrderDetailResponse{
		ID:             order.ID,
		OrderNo:        order.OrderNo,
		UserID:         order.UserID,
		Amount:         order.Amount,
		Status:         order.Status,
		PaymentMethod:  order.PaymentMethod,
		PaymentChannel: order.PaymentChannel,
		TransactionID:  order.WeChatTransactionID,
		ExpireAt:       order.ExpireAt,
		PaidAt:         order.PaidAt,
		RefundNo:       order.RefundNo,
		RefundStatus:   order.RefundStatus,
		RefundedAt:     order.RefundedAt,
		RefundReason:   order.RefundReason,
		RefundAdminID:  order.RefundAdminID,
		Notes:          order.Notes,
		CreatedAt:      order.CreatedAt,
		UpdatedAt:      order.UpdatedAt,
	})
}

// AdminOrderListItem 管理端订单列表项
type AdminOrderListItem struct {
	ID            int64      `json:"id"`
	OrderNo       string     `json:"order_no"`
	UserID        int64      `json:"user_id"`
	Amount        float64    `json:"amount"`
	Status        string     `json:"status"`
	PaymentMethod string     `json:"payment_method"`
	CreatedAt     time.Time  `json:"created_at"`
	PaidAt        *time.Time `json:"paid_at,omitempty"`
}

// AdminListOrdersResponse 管理端订单列表响应
type AdminListOrdersResponse struct {
	Orders   []AdminOrderListItem `json:"orders"`
	Total    int64                `json:"total"`
	Page     int                  `json:"page"`
	PageSize int                  `json:"page_size"`
}

// ListOrders 获取订单列表
// GET /api/v1/admin/recharge/orders
func (h *RechargeHandler) ListOrders(c *gin.Context) {
	// 解析分页参数
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	status := c.Query("status")
	userIDStr := c.Query("user_id")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	// 构建查询请求
	listReq := &service.ListRechargeOrdersRequest{
		Status: status,
	}
	listReq.Page = page
	listReq.PageSize = pageSize

	// 如果指定了用户ID，按用户查询
	var result *service.ListRechargeOrdersResult
	var err error

	if userIDStr != "" {
		userID, parseErr := strconv.ParseInt(userIDStr, 10, 64)
		if parseErr != nil {
			response.BadRequest(c, "无效的用户ID")
			return
		}
		result, err = h.rechargeOrderService.ListUserOrders(c.Request.Context(), userID, listReq)
	} else {
		// 查询所有订单
		result, err = h.rechargeOrderService.ListAllOrders(c.Request.Context(), listReq)
	}

	if err != nil {
		response.InternalError(c, "查询订单失败")
		return
	}

	// 转换响应
	orders := make([]AdminOrderListItem, len(result.Orders))
	for i, order := range result.Orders {
		orders[i] = AdminOrderListItem{
			ID:            order.ID,
			OrderNo:       order.OrderNo,
			UserID:        order.UserID,
			Amount:        order.Amount,
			Status:        order.Status,
			PaymentMethod: order.PaymentMethod,
			CreatedAt:     order.CreatedAt,
			PaidAt:        order.PaidAt,
		}
	}

	response.Success(c, AdminListOrdersResponse{
		Orders:   orders,
		Total:    result.Pagination.Total,
		Page:     result.Pagination.Page,
		PageSize: result.Pagination.PageSize,
	})
}

// AdminRefundOrderRequest 管理员退款请求
type AdminRefundOrderRequest struct {
	Reason string `json:"reason" binding:"required,min=2,max=500"`
}

// AdminRefundOrderResponse 管理员退款响应
type AdminRefundOrderResponse struct {
	OrderNo      string     `json:"order_no"`
	RefundNo     string     `json:"refund_no"`
	Status       string     `json:"status"`
	RefundStatus string     `json:"refund_status"`
	WeChatStatus string     `json:"wechat_status,omitempty"`
	RefundedAt   *time.Time `json:"refunded_at,omitempty"`
	Message      string     `json:"message"`
}

// RefundOrder 退款订单
// POST /api/v1/admin/recharge/orders/:order_no/refund
func (h *RechargeHandler) RefundOrder(c *gin.Context) {
	orderNo := c.Param("order_no")
	if orderNo == "" {
		response.BadRequest(c, "订单号不能为空")
		return
	}

	var req AdminRefundOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请输入退款原因（2-500字）")
		return
	}

	// 获取当前管理员 ID
	adminID := c.GetInt64("user_id")

	// 调用退款服务
	result, err := h.rechargeOrderService.RefundOrder(c.Request.Context(), service.RefundOrderParams{
		OrderNo: orderNo,
		Reason:  req.Reason,
		AdminID: adminID,
	})

	if err != nil {
		if !response.ErrorFrom(c, err) {
			response.InternalError(c, "退款失败")
		}
		return
	}

	// 根据退款状态生成消息
	var message string
	switch result.RefundStatus {
	case "success":
		message = "退款成功"
	case "processing":
		message = "退款处理中，请等待微信回调"
	case "failed":
		message = "退款失败"
	default:
		message = "退款处理中"
	}

	response.Success(c, AdminRefundOrderResponse{
		OrderNo:      result.OrderNo,
		RefundNo:     result.RefundNo,
		Status:       result.Status,
		RefundStatus: result.RefundStatus,
		WeChatStatus: result.WeChatStatus,
		RefundedAt:   result.RefundedAt,
		Message:      message,
	})
}

// AdminBalanceLogItem 管理端余额日志项
type AdminBalanceLogItem struct {
	ID             int64     `json:"id"`
	UserID         int64     `json:"user_id"`
	ChangeType     string    `json:"change_type"`
	Amount         float64   `json:"amount"`
	BalanceBefore  float64   `json:"balance_before"`
	BalanceAfter   float64   `json:"balance_after"`
	RelatedOrderNo *string   `json:"related_order_no,omitempty"`
	Description    string    `json:"description"`
	OperatorID     int64     `json:"operator_id"`
	OperatorType   string    `json:"operator_type"`
	CreatedAt      time.Time `json:"created_at"`
}

// AdminOrderLogsResponse 管理端订单日志响应
type AdminOrderLogsResponse struct {
	Logs []AdminBalanceLogItem `json:"logs"`
}

// GetOrderLogs 获取订单相关的余额日志
// GET /api/v1/admin/recharge/orders/:order_no/logs
func (h *RechargeHandler) GetOrderLogs(c *gin.Context) {
	orderNo := c.Param("order_no")
	if orderNo == "" {
		response.BadRequest(c, "订单号不能为空")
		return
	}

	logs, err := h.balanceLogRepo.GetByOrderNo(c.Request.Context(), orderNo)
	if err != nil {
		response.InternalError(c, "查询日志失败")
		return
	}

	// 转换响应
	items := make([]AdminBalanceLogItem, len(logs))
	for i, log := range logs {
		items[i] = AdminBalanceLogItem{
			ID:             log.ID,
			UserID:         log.UserID,
			ChangeType:     log.ChangeType,
			Amount:         log.Amount,
			BalanceBefore:  log.BalanceBefore,
			BalanceAfter:   log.BalanceAfter,
			RelatedOrderNo: log.RelatedOrderNo,
			Description:    log.Description,
			OperatorID:     log.OperatorID,
			OperatorType:   log.OperatorType,
			CreatedAt:      log.CreatedAt,
		}
	}

	response.Success(c, AdminOrderLogsResponse{
		Logs: items,
	})
}
