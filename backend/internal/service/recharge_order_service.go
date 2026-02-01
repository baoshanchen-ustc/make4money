package service

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/redis/go-redis/v9"
)

// 订单状态常量
const (
	OrderStatusPending   = "pending"   // 待支付
	OrderStatusPaid      = "paid"      // 已支付
	OrderStatusFailed    = "failed"    // 支付失败
	OrderStatusExpired   = "expired"   // 已过期
	OrderStatusCancelled = "cancelled" // 已取消
)

// 支付方式常量
const (
	PaymentMethodWeChatPay = "wechat_pay"
	PaymentMethodAlipay    = "alipay"
)

// 支付渠道常量
const (
	PaymentChannelNative = "native" // 扫码支付
	PaymentChannelJSAPI  = "jsapi"  // 公众号/小程序支付
	PaymentChannelH5     = "h5"     // H5 支付
)

// 退款状态常量
const (
	RefundStatusPending    = "pending"    // 退款待处理
	RefundStatusProcessing = "processing" // 退款处理中
	RefundStatusSuccess    = "success"    // 退款成功
	RefundStatusFailed     = "failed"     // 退款失败
	RefundStatusAbnormal   = "abnormal"   // 退款异常
)

var (
	ErrRechargeOrderNotFound    = infraerrors.NotFound("RECHARGE_ORDER_NOT_FOUND", "recharge order not found")
	ErrRechargeOrderExpired     = infraerrors.BadRequest("RECHARGE_ORDER_EXPIRED", "recharge order has expired")
	ErrRechargeNotEnabled       = infraerrors.BadRequest("RECHARGE_NOT_ENABLED", "recharge is not enabled")
	ErrInvalidAmount            = infraerrors.BadRequest("INVALID_AMOUNT", "invalid recharge amount")
	ErrOrderCannotBeCancelled   = infraerrors.BadRequest("ORDER_CANNOT_BE_CANCELLED", "order cannot be cancelled")
	ErrOrderCancelConflict      = infraerrors.Conflict("ORDER_CANCEL_CONFLICT", "order status has changed")
	ErrOrderNotBelongToUser     = infraerrors.Forbidden("ORDER_NOT_BELONG_TO_USER", "order does not belong to you")
	ErrOrderNotRefundable       = infraerrors.BadRequest("ORDER_NOT_REFUNDABLE", "only paid orders can be refunded")
	ErrRefundFailed             = infraerrors.InternalServer("REFUND_FAILED", "refund request failed")
	ErrRefundAlreadyInProgress  = infraerrors.Conflict("REFUND_ALREADY_IN_PROGRESS", "refund is already in progress")
)

// RechargeOrder 充值订单模型
type RechargeOrder struct {
	ID                   int64      `json:"id"`
	OrderNo              string     `json:"order_no"`
	UserID               int64      `json:"user_id"`
	Amount               float64    `json:"amount"`
	PaymentMethod        string     `json:"payment_method"`
	PaymentChannel       string     `json:"payment_channel"`
	Status               string     `json:"status"`
	WeChatTransactionID  *string    `json:"wechat_transaction_id,omitempty"`
	QRCodeURL            *string    `json:"qrcode_url,omitempty"`
	PrepayID             *string    `json:"prepay_id,omitempty"`
	ExpireAt             time.Time  `json:"expire_at"`
	PaidAt               *time.Time `json:"paid_at,omitempty"`
	Notes                string     `json:"notes,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
	// 退款相关字段
	RefundNo       *string    `json:"refund_no,omitempty"`
	RefundStatus   *string    `json:"refund_status,omitempty"`
	RefundedAt     *time.Time `json:"refunded_at,omitempty"`
	RefundReason   *string    `json:"refund_reason,omitempty"`
	RefundAdminID  *int64     `json:"refund_admin_id,omitempty"`
	WeChatRefundID *string    `json:"wechat_refund_id,omitempty"`
}

// CreateRechargeOrderRequest 创建充值订单请求
type CreateRechargeOrderRequest struct {
	Amount         float64 `json:"amount" binding:"required,gt=0"`
	PaymentMethod  string  `json:"payment_method" binding:"required"`
	PaymentChannel string  `json:"payment_channel"`
}

// ListRechargeOrdersRequest 查询充值订单列表请求
type ListRechargeOrdersRequest struct {
	pagination.PaginationParams
	Status    string     // 可选：按状态筛选
	StartTime *time.Time // 可选：开始时间
	EndTime   *time.Time // 可选：结束时间
}

// ListRechargeOrdersResult 查询充值订单列表结果
type ListRechargeOrdersResult struct {
	Orders     []*RechargeOrder
	Pagination *pagination.PaginationResult
}

// RechargeOrderRepository 充值订单仓储接口
type RechargeOrderRepository interface {
	Create(ctx context.Context, order *RechargeOrder) error
	GetByID(ctx context.Context, id int64) (*RechargeOrder, error)
	GetByOrderNo(ctx context.Context, orderNo string) (*RechargeOrder, error)
	Update(ctx context.Context, order *RechargeOrder) error
	ExistsByOrderNo(ctx context.Context, orderNo string) (bool, error)
	ListByUserID(ctx context.Context, userID int64, req *ListRechargeOrdersRequest) (*ListRechargeOrdersResult, error)
	// ListAll 获取所有订单（管理端使用）
	ListAll(ctx context.Context, req *ListRechargeOrdersRequest) (*ListRechargeOrdersResult, error)
	// UpdateStatusWithCondition 使用乐观锁更新订单状态
	// 只有当订单当前状态等于 expectedStatus 时才更新为 newStatus
	// 返回受影响的行数，如果为 0 则表示状态已改变（并发冲突）
	UpdateStatusWithCondition(ctx context.Context, id int64, expectedStatus, newStatus, notes string) (int64, error)
	// MarkExpiredOrders 批量将已过期的 pending 订单标记为 expired
	// 返回过期的订单号列表
	MarkExpiredOrders(ctx context.Context, limit int) ([]string, error)
	// AppendNotes 追加订单备注
	AppendNotes(ctx context.Context, orderNo, notes string) error
	// GetPendingOrdersForCompensation 获取待补偿的订单（用于定时任务）
	// 返回 status='pending' 且 created_at < thresholdTime 的订单，按创建时间升序
	GetPendingOrdersForCompensation(ctx context.Context, thresholdTime time.Time, limit int) ([]*RechargeOrder, error)
	// MarkOrderExpired 将指定订单标记为过期（用于同步到微信 CLOSED 状态）
	MarkOrderExpired(ctx context.Context, orderNo string) error
	// MarkOrderFailed 将指定订单标记为失败（用于同步到微信 PAYERROR 状态）
	MarkOrderFailed(ctx context.Context, orderNo string) error
	// UpdateRefundStatus 更新订单退款状态
	// params: orderNo, refundNo, refundStatus, refundReason, refundAdminID, wechatRefundID, refundedAt
	UpdateRefundStatus(ctx context.Context, orderNo string, refundNo, refundStatus, refundReason string, refundAdminID int64, wechatRefundID string, refundedAt *time.Time) error
	// UpdateRefundResult 更新退款结果（用于回调处理）
	UpdateRefundResult(ctx context.Context, refundNo string, refundStatus string, refundedAt *time.Time) error
	// GetByRefundNo 根据退款单号获取订单
	GetByRefundNo(ctx context.Context, refundNo string) (*RechargeOrder, error)
	// MarkOrderRefunded 将订单标记为已退款（在事务中使用）
	MarkOrderRefunded(ctx context.Context, orderNo string, notes string) error
}

// RechargeOrderService 充值订单服务
type RechargeOrderService struct {
	cfg                    *config.Config
	repo                   RechargeOrderRepository
	wechatPayService       *WeChatPayService
	paymentCallbackService *PaymentCallbackService
	entClient              *dbent.Client
	userRepo               UserRepository
	balanceLogRepo         BalanceLogRepository
	redisClient            *redis.Client
}

// NewRechargeOrderService 创建充值订单服务
func NewRechargeOrderService(
	cfg *config.Config,
	repo RechargeOrderRepository,
	wechatPayService *WeChatPayService,
	paymentCallbackService *PaymentCallbackService,
	entClient *dbent.Client,
	userRepo UserRepository,
	balanceLogRepo BalanceLogRepository,
	redisClient *redis.Client,
) *RechargeOrderService {
	return &RechargeOrderService{
		cfg:                    cfg,
		repo:                   repo,
		wechatPayService:       wechatPayService,
		paymentCallbackService: paymentCallbackService,
		entClient:              entClient,
		userRepo:               userRepo,
		balanceLogRepo:         balanceLogRepo,
		redisClient:            redisClient,
	}
}

// GenerateOrderNo 生成订单号
// 格式：RECH + 年月日时分秒（14位）+ 10位随机字符串
// 示例：RECH20260124150000AbCd1234Ef
func GenerateOrderNo() string {
	timestamp := time.Now().Format("20060102150405")
	randomStr := generateRandomString(10)
	return fmt.Sprintf("RECH%s%s", timestamp, randomStr)
}

// GenerateRefundNo 生成退款单号
// 格式：REFD + 年月日时分秒（14位）+ 6位随机字符串
// 示例：REFD20260124150000AbCd12
func GenerateRefundNo() string {
	timestamp := time.Now().Format("20060102150405")
	randomStr := generateRandomString(6)
	return fmt.Sprintf("REFD%s%s", timestamp, randomStr)
}

// generateRandomString 生成指定长度的随机字符串
// 包含大小写字母和数字
func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			// 如果加密随机失败，使用简单的时间戳作为备选
			result[i] = charset[time.Now().UnixNano()%int64(len(charset))]
		} else {
			result[i] = charset[n.Int64()]
		}
	}
	return string(result)
}

// CreateOrder 创建充值订单
func (s *RechargeOrderService) CreateOrder(ctx context.Context, userID int64, req *CreateRechargeOrderRequest) (*RechargeOrder, error) {
	// 检查微信支付是否启用
	if !s.wechatPayService.IsEnabled() {
		return nil, ErrRechargeNotEnabled
	}

	// 验证金额
	if err := s.wechatPayService.ValidateRechargeAmount(req.Amount); err != nil {
		return nil, infraerrors.BadRequest("INVALID_AMOUNT", err.Error())
	}

	// 设置默认支付渠道
	paymentChannel := req.PaymentChannel
	if paymentChannel == "" {
		paymentChannel = PaymentChannelNative
	}

	// 生成唯一订单号（重试机制确保唯一性）
	var orderNo string
	for i := 0; i < 3; i++ {
		orderNo = GenerateOrderNo()
		exists, err := s.repo.ExistsByOrderNo(ctx, orderNo)
		if err != nil {
			return nil, fmt.Errorf("check order no exists: %w", err)
		}
		if !exists {
			break
		}
		if i == 2 {
			return nil, fmt.Errorf("failed to generate unique order no after 3 attempts")
		}
	}

	// 计算过期时间
	expireMinutes := s.cfg.WeChatPay.OrderExpireMinutes
	if expireMinutes <= 0 {
		expireMinutes = 30 // 默认 30 分钟
	}
	expireAt := time.Now().Add(time.Duration(expireMinutes) * time.Minute)

	order := &RechargeOrder{
		OrderNo:        orderNo,
		UserID:         userID,
		Amount:         req.Amount,
		PaymentMethod:  req.PaymentMethod,
		PaymentChannel: paymentChannel,
		Status:         OrderStatusPending,
		ExpireAt:       expireAt,
	}

	if err := s.repo.Create(ctx, order); err != nil {
		return nil, fmt.Errorf("create order: %w", err)
	}

	return order, nil
}

// GetOrder 获取订单详情
func (s *RechargeOrderService) GetOrder(ctx context.Context, orderNo string) (*RechargeOrder, error) {
	return s.repo.GetByOrderNo(ctx, orderNo)
}

// GetUserOrder 获取用户的订单详情（带权限校验）
// 只能查询自己的订单，如果订单不存在或不属于该用户则返回错误
func (s *RechargeOrderService) GetUserOrder(ctx context.Context, userID int64, orderNo string) (*RechargeOrder, error) {
	order, err := s.repo.GetByOrderNo(ctx, orderNo)
	if err != nil {
		return nil, err
	}

	// 校验用户权限：只能查询自己的订单
	if order.UserID != userID {
		return nil, ErrRechargeOrderNotFound
	}

	return order, nil
}

// GetOrderByID 根据ID获取订单
func (s *RechargeOrderService) GetOrderByID(ctx context.Context, id int64) (*RechargeOrder, error) {
	return s.repo.GetByID(ctx, id)
}

// GetExpireMinutes 获取订单过期时间（分钟）
func (s *RechargeOrderService) GetExpireMinutes() int {
	if s.cfg.WeChatPay.OrderExpireMinutes > 0 {
		return s.cfg.WeChatPay.OrderExpireMinutes
	}
	return 30
}

// UpdatePaymentResult 更新订单支付结果（保存 prepay_id 或 qrcode_url）
func (s *RechargeOrderService) UpdatePaymentResult(ctx context.Context, orderNo string, qrcodeURL, prepayID *string) error {
	order, err := s.repo.GetByOrderNo(ctx, orderNo)
	if err != nil {
		return err
	}

	order.QRCodeURL = qrcodeURL
	order.PrepayID = prepayID

	return s.repo.Update(ctx, order)
}

// ListUserOrders 获取用户的充值订单列表
func (s *RechargeOrderService) ListUserOrders(ctx context.Context, userID int64, req *ListRechargeOrdersRequest) (*ListRechargeOrdersResult, error) {
	// 设置默认分页参数
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 {
		req.PageSize = 10
	}

	return s.repo.ListByUserID(ctx, userID, req)
}

// CancelOrder 取消未支付订单
// 只能取消状态为 pending 的订单
// 取消后状态变为 failed，并记录取消原因
func (s *RechargeOrderService) CancelOrder(ctx context.Context, userID int64, orderNo string) error {
	// 获取订单并校验权限
	order, err := s.repo.GetByOrderNo(ctx, orderNo)
	if err != nil {
		return err
	}

	// 校验用户权限：只能取消自己的订单
	if order.UserID != userID {
		return ErrRechargeOrderNotFound
	}

	// 检查订单状态：只能取消 pending 状态的订单
	if order.Status != OrderStatusPending {
		return ErrOrderCannotBeCancelled
	}

	// 检查订单是否已过期
	if time.Now().After(order.ExpireAt) {
		return ErrRechargeOrderExpired
	}

	// 使用并发安全的条件更新
	rowsAffected, err := s.repo.UpdateStatusWithCondition(ctx, order.ID, OrderStatusPending, OrderStatusFailed, "用户主动取消")
	if err != nil {
		return fmt.Errorf("cancel order: %w", err)
	}

	// 如果没有更新到任何行，说明订单状态已经改变（并发冲突）
	if rowsAffected == 0 {
		return ErrOrderCancelConflict
	}

	return nil
}

// SyncOrderStatusResult 同步订单状态结果
type SyncOrderStatusResult struct {
	OrderNo      string    // 订单号
	Status       string    // 本地订单状态
	WeChatStatus string    // 微信侧原始状态
	SyncedAt     time.Time // 同步时间
}

// SyncOrderStatus 同步订单状态
// 调用微信支付查询接口获取真实支付状态，与本地订单状态进行对比
// 如果微信显示已支付但本地未到账，自动触发补偿到账
func (s *RechargeOrderService) SyncOrderStatus(ctx context.Context, userID int64, orderNo string) (*SyncOrderStatusResult, error) {
	// 1. 查询本地订单
	order, err := s.repo.GetByOrderNo(ctx, orderNo)
	if err != nil {
		return nil, err
	}

	// 2. 验证订单归属
	if order.UserID != userID {
		return nil, ErrOrderNotBelongToUser
	}

	// 3. 如果订单已经是终态，直接返回（无需查询微信）
	if order.Status != OrderStatusPending {
		return &SyncOrderStatusResult{
			OrderNo:      orderNo,
			Status:       order.Status,
			WeChatStatus: "",
			SyncedAt:     time.Now(),
		}, nil
	}

	// 4. 调用微信支付查询接口
	wechatResult, err := s.wechatPayService.QueryOrder(ctx, orderNo)
	if err != nil {
		return nil, fmt.Errorf("query wechat order: %w", err)
	}

	// 5. 根据微信状态映射本地状态
	localStatus := mapWeChatStatusToLocal(wechatResult.TradeState)

	// 6. 如果微信显示已支付但本地是 pending，触发补偿到账
	if wechatResult.TradeState == "SUCCESS" && order.Status == OrderStatusPending {
		if s.paymentCallbackService != nil {
			result := s.paymentCallbackService.ProcessPaymentSuccess(ctx, ProcessPaymentSuccessParams{
				OrderNo:       orderNo,
				TransactionID: wechatResult.TransactionID,
				AmountInFen:   0, // 补偿时不验证金额（查询接口可能不返回）
				Source:        PaymentSourceManualSync,
			})
			if result.Success {
				localStatus = OrderStatusPaid
			} else if result.ErrorMessage != "" {
				// 记录补偿失败但不影响返回（用户至少知道微信已支付）
				fmt.Printf("[SyncOrderStatus] Compensation failed: order_no=%s, error=%s\n",
					orderNo, result.ErrorMessage)
			}
		}
	}

	return &SyncOrderStatusResult{
		OrderNo:      orderNo,
		Status:       localStatus,
		WeChatStatus: wechatResult.TradeState,
		SyncedAt:     time.Now(),
	}, nil
}

// mapWeChatStatusToLocal 将微信支付状态映射为本地状态
func mapWeChatStatusToLocal(wechatStatus string) string {
	switch wechatStatus {
	case "SUCCESS":
		return OrderStatusPaid
	case "REFUND":
		return "refunded"
	case "NOTPAY", "USERPAYING":
		return OrderStatusPending
	case "CLOSED":
		return OrderStatusExpired
	case "PAYERROR":
		return OrderStatusFailed
	default:
		return OrderStatusPending
	}
}

// ListAllOrders 获取所有订单（管理端使用）
func (s *RechargeOrderService) ListAllOrders(ctx context.Context, req *ListRechargeOrdersRequest) (*ListRechargeOrdersResult, error) {
	// 设置默认分页参数
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 {
		req.PageSize = 20
	}

	return s.repo.ListAll(ctx, req)
}

// RefundOrderParams 退款参数
type RefundOrderParams struct {
	OrderNo string
	Reason  string
	AdminID int64
}

// RefundOrderResult 退款结果
type RefundOrderResult struct {
	OrderNo      string
	Status       string
	RefundNo     string
	RefundStatus string
	WeChatStatus string // 微信原始返回状态
	RefundedAt   *time.Time
}

// RefundOrder 退款订单
// 1. 验证订单状态（只有 paid 状态可退款）
// 2. 检查是否已有退款在处理中
// 3. 生成退款单号
// 4. 调用微信退款 API
// 5. 根据退款结果更新订单状态
func (s *RechargeOrderService) RefundOrder(ctx context.Context, params RefundOrderParams) (*RefundOrderResult, error) {
	// 1. 查询订单
	order, err := s.repo.GetByOrderNo(ctx, params.OrderNo)
	if err != nil {
		return nil, err
	}

	// 2. 验证订单状态：只有 paid 状态可以退款
	if order.Status != OrderStatusPaid {
		return nil, ErrOrderNotRefundable
	}

	// 3. 检查是否已有退款在处理中
	if order.RefundStatus != nil && (*order.RefundStatus == RefundStatusProcessing || *order.RefundStatus == RefundStatusSuccess) {
		return nil, ErrRefundAlreadyInProgress
	}

	// 4. 生成退款单号
	refundNo := GenerateRefundNo()

	// 5. 获取微信支付订单号
	transactionID := ""
	if order.WeChatTransactionID != nil {
		transactionID = *order.WeChatTransactionID
	}

	// 6. 调用微信退款 API
	refundResult, err := s.wechatPayService.Refund(ctx, RefundParams{
		OrderNo:       params.OrderNo,
		RefundNo:      refundNo,
		Amount:        order.Amount,
		TotalAmount:   order.Amount, // 全额退款
		Reason:        params.Reason,
		TransactionID: transactionID,
	})

	if err != nil {
		// 记录退款请求失败
		notes := fmt.Sprintf("退款申请失败: %s (管理员ID: %d, 退款单号: %s)", err.Error(), params.AdminID, refundNo)
		_ = s.repo.AppendNotes(ctx, params.OrderNo, notes)
		return nil, fmt.Errorf("%w: %v", ErrRefundFailed, err)
	}

	// 7. 根据微信返回的退款状态更新订单
	var localRefundStatus string
	var refundedAt *time.Time

	switch refundResult.Status {
	case "SUCCESS":
		localRefundStatus = RefundStatusSuccess
		now := time.Now()
		refundedAt = &now
	case "PROCESSING":
		localRefundStatus = RefundStatusProcessing
	case "CLOSED", "ABNORMAL":
		localRefundStatus = RefundStatusFailed
	default:
		localRefundStatus = RefundStatusProcessing
	}

	// 8. 更新订单退款状态
	err = s.repo.UpdateRefundStatus(ctx, params.OrderNo, refundNo, localRefundStatus, params.Reason, params.AdminID, refundResult.RefundID, refundedAt)
	if err != nil {
		// 退款已成功但本地更新失败，需要告警
		notes := fmt.Sprintf("退款成功但本地更新失败: %s (微信状态: %s, 退款单号: %s)", err.Error(), refundResult.Status, refundNo)
		_ = s.repo.AppendNotes(ctx, params.OrderNo, notes)
		// 仍然返回成功，因为微信侧已退款
	}

	// 9. 如果退款成功，处理余额扣减和订单状态更新
	if localRefundStatus == RefundStatusSuccess {
		err = s.processRefundSuccess(ctx, ProcessRefundSuccessParams{
			OrderNo:  params.OrderNo,
			RefundNo: refundNo,
			Amount:   order.Amount,
			Reason:   params.Reason,
			AdminID:  params.AdminID,
			UserID:   order.UserID,
		})
		if err != nil {
			log.Printf("[RechargeOrderService] RefundOrder: processRefundSuccess failed: order_no=%s, error=%v",
				params.OrderNo, err)
			// 不返回错误，因为微信退款已成功
		}
	}

	return &RefundOrderResult{
		OrderNo:      params.OrderNo,
		Status:       order.Status,
		RefundNo:     refundNo,
		RefundStatus: localRefundStatus,
		WeChatStatus: refundResult.Status,
		RefundedAt:   refundedAt,
	}, nil
}

// HandleRefundCallback 处理退款回调
// 当微信退款结果异步回调时调用
func (s *RechargeOrderService) HandleRefundCallback(ctx context.Context, notification *RefundNotification) error {
	// 1. 根据退款单号查找订单
	order, err := s.repo.GetByRefundNo(ctx, notification.OutRefundNo)
	if err != nil {
		return fmt.Errorf("find order by refund_no failed: %w", err)
	}

	// 2. 检查退款状态是否需要更新
	if order.RefundStatus != nil && *order.RefundStatus == RefundStatusSuccess {
		// 已经是成功状态，无需重复处理
		return nil
	}

	// 3. 根据回调状态更新
	var localRefundStatus string
	var refundedAt *time.Time

	switch notification.RefundStatus {
	case "SUCCESS":
		localRefundStatus = RefundStatusSuccess
		now := time.Now()
		refundedAt = &now
	case "CLOSED", "ABNORMAL":
		localRefundStatus = RefundStatusFailed
	default:
		localRefundStatus = RefundStatusProcessing
	}

	// 4. 更新退款结果
	err = s.repo.UpdateRefundResult(ctx, notification.OutRefundNo, localRefundStatus, refundedAt)
	if err != nil {
		return fmt.Errorf("update refund result failed: %w", err)
	}

	// 5. 如果退款成功，处理余额扣减并更新订单状态
	if localRefundStatus == RefundStatusSuccess {
		adminID := int64(0)
		if order.RefundAdminID != nil {
			adminID = *order.RefundAdminID
		}
		reason := ""
		if order.RefundReason != nil {
			reason = *order.RefundReason
		}
		err = s.processRefundSuccess(ctx, ProcessRefundSuccessParams{
			OrderNo:  order.OrderNo,
			RefundNo: notification.OutRefundNo,
			Amount:   order.Amount,
			Reason:   reason,
			AdminID:  adminID,
			UserID:   order.UserID,
		})
		if err != nil {
			log.Printf("[RechargeOrderService] HandleRefundCallback: processRefundSuccess failed: order_no=%s, error=%v",
				order.OrderNo, err)
			// 不返回错误，因为退款已经成功，只是本地处理失败
		}
	}

	return nil
}

// 分布式锁常量
const (
	refundLockKeyPrefix = "recharge:refund:"
	refundLockTTL       = 30 * time.Second
)

// ProcessRefundSuccessParams 处理退款成功的参数
type ProcessRefundSuccessParams struct {
	OrderNo  string
	RefundNo string
	Amount   float64
	Reason   string
	AdminID  int64
	UserID   int64
}

// processRefundSuccess 处理退款成功
// 在事务中：扣减用户余额 + 记录余额日志 + 更新订单状态
func (s *RechargeOrderService) processRefundSuccess(ctx context.Context, params ProcessRefundSuccessParams) error {
	orderNo := params.OrderNo

	// 1. 获取分布式锁（如果 Redis 可用）
	if s.redisClient != nil {
		lockKey := refundLockKeyPrefix + orderNo
		locked, err := s.redisClient.SetNX(ctx, lockKey, "1", refundLockTTL).Result()
		if err != nil {
			log.Printf("[RechargeOrderService] processRefundSuccess: acquire lock failed: order_no=%s, error=%v",
				orderNo, err)
			// 锁获取失败，继续处理（依赖数据库事务保证一致性）
		} else if !locked {
			// 其他进程正在处理
			log.Printf("[RechargeOrderService] processRefundSuccess: order being processed by another goroutine: order_no=%s",
				orderNo)
			return nil
		} else {
			// 成功获取锁，确保释放
			defer func() {
				if err := s.redisClient.Del(ctx, lockKey).Err(); err != nil {
					log.Printf("[RechargeOrderService] processRefundSuccess: release lock failed: order_no=%s, error=%v",
						orderNo, err)
				}
			}()
		}
	}

	// 2. 开启数据库事务
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction failed: %w", err)
	}

	defer func() {
		if r := recover(); r != nil {
			_ = tx.Rollback()
			panic(r)
		}
	}()

	// 使用事务上下文
	txCtx := dbent.NewTxContext(ctx, tx)

	// 3. 查询用户当前余额
	user, err := s.userRepo.GetByID(txCtx, params.UserID)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("query user failed: %w", err)
	}

	balanceBefore := user.Balance
	balanceAfter := balanceBefore - params.Amount

	// 4. 检查余额是否足够（允许余额为负）
	if balanceAfter < 0 {
		log.Printf("[RechargeOrderService] processRefundSuccess: user balance insufficient for refund, allowing negative balance: "+
			"user_id=%d, balance_before=%.2f, refund_amount=%.2f, balance_after=%.2f, order_no=%s",
			params.UserID, balanceBefore, params.Amount, balanceAfter, orderNo)
		// 继续处理，允许余额为负（用户可能已消费部分）
	}

	// 5. 更新用户余额（扣减）
	if err := s.userRepo.UpdateBalance(txCtx, params.UserID, -params.Amount); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("update user balance failed: %w", err)
	}

	// 6. 记录余额变动日志
	description := fmt.Sprintf("充值退款 - %s", params.Reason)
	balanceLog := &BalanceLog{
		UserID:         params.UserID,
		ChangeType:     BalanceChangeTypeRefund,
		Amount:         -params.Amount, // 负数表示扣减
		BalanceBefore:  balanceBefore,
		BalanceAfter:   balanceAfter,
		RelatedOrderNo: &params.OrderNo,
		Description:    description,
		OperatorID:     params.AdminID,
		OperatorType:   OperatorTypeAdmin,
	}
	if err := s.balanceLogRepo.Create(txCtx, balanceLog); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("create balance log failed: %w", err)
	}

	// 7. 更新订单状态为 refunded
	notes := fmt.Sprintf("退款成功: %s (余额: %.2f -> %.2f)", params.RefundNo, balanceBefore, balanceAfter)
	if err := s.repo.MarkOrderRefunded(txCtx, params.OrderNo, notes); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("mark order refunded failed: %w", err)
	}

	// 提交事务
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction failed: %w", err)
	}

	log.Printf("[RechargeOrderService] processRefundSuccess: refund processed successfully: "+
		"order_no=%s, user_id=%d, amount=%.2f, balance_before=%.2f, balance_after=%.2f, admin_id=%d",
		orderNo, params.UserID, params.Amount, balanceBefore, balanceAfter, params.AdminID)

	return nil
}
