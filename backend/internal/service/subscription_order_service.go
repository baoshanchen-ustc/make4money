package service

import (
	"context"
	"fmt"
	"log"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/redis/go-redis/v9"
)

// 订阅订单前缀
const (
	SubscriptionOrderPrefix = "SUBS"
)

var (
	ErrSubscriptionOrderNotFound      = infraerrors.NotFound("SUBSCRIPTION_ORDER_NOT_FOUND", "subscription order not found")
	ErrSubscriptionOrderExpired       = infraerrors.BadRequest("SUBSCRIPTION_ORDER_EXPIRED", "subscription order has expired")
	ErrGroupNotPurchasable            = infraerrors.BadRequest("GROUP_NOT_PURCHASABLE", "group is not available for purchase")
	ErrGroupPriceNotSet               = infraerrors.BadRequest("GROUP_PRICE_NOT_SET", "group price is not set")
	ErrSubscriptionOrderNotBelongUser = infraerrors.Forbidden("ORDER_NOT_BELONG_TO_USER", "order does not belong to you")
)

// SubscriptionOrder 订阅订单模型
type SubscriptionOrder struct {
	ID                  int64      `json:"id"`
	OrderNo             string     `json:"order_no"`
	UserID              int64      `json:"user_id"`
	GroupID             int64      `json:"group_id"`
	Amount              float64    `json:"amount"`
	ValidityDays        int        `json:"validity_days"`
	PaymentMethod       string     `json:"payment_method"`
	PaymentChannel      string     `json:"payment_channel"`
	Status              string     `json:"status"`
	WeChatTransactionID *string    `json:"wechat_transaction_id,omitempty"`
	QRCodeURL           *string    `json:"qrcode_url,omitempty"`
	PrepayID            *string    `json:"prepay_id,omitempty"`
	ExpireAt            time.Time  `json:"expire_at"`
	PaidAt              *time.Time `json:"paid_at,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`

	// 关联信息
	Group *Group `json:"group,omitempty"`
}

// CreateSubscriptionOrderRequest 创建订阅订单请求
type CreateSubscriptionOrderRequest struct {
	GroupID        int64  `json:"group_id" binding:"required"`
	PaymentMethod  string `json:"payment_method" binding:"required"`
	PaymentChannel string `json:"payment_channel"`
}

// ListSubscriptionOrdersRequest 查询订阅订单列表请求
type ListSubscriptionOrdersRequest struct {
	pagination.PaginationParams
	Status    string     // 可选：按状态筛选
	StartTime *time.Time // 可选：开始时间
	EndTime   *time.Time // 可选：结束时间
}

// ListSubscriptionOrdersResult 查询订阅订单列表结果
type ListSubscriptionOrdersResult struct {
	Orders     []*SubscriptionOrder
	Pagination *pagination.PaginationResult
}

// SubscriptionOrderRepository 订阅订单仓储接口
type SubscriptionOrderRepository interface {
	Create(ctx context.Context, order *SubscriptionOrder) error
	GetByID(ctx context.Context, id int64) (*SubscriptionOrder, error)
	GetByOrderNo(ctx context.Context, orderNo string) (*SubscriptionOrder, error)
	GetByOrderNoWithGroup(ctx context.Context, orderNo string) (*SubscriptionOrder, error)
	Update(ctx context.Context, order *SubscriptionOrder) error
	ExistsByOrderNo(ctx context.Context, orderNo string) (bool, error)
	ListByUserID(ctx context.Context, userID int64, req *ListSubscriptionOrdersRequest) (*ListSubscriptionOrdersResult, error)
	ListAll(ctx context.Context, req *ListSubscriptionOrdersRequest) (*ListSubscriptionOrdersResult, error)
	UpdateStatusWithCondition(ctx context.Context, id int64, expectedStatus, newStatus string) (int64, error)
	MarkExpiredOrders(ctx context.Context, limit int) ([]string, error)
	GetPendingOrdersForCompensation(ctx context.Context, thresholdTime time.Time, limit int) ([]*SubscriptionOrder, error)
	MarkOrderExpired(ctx context.Context, orderNo string) error
	MarkOrderFailed(ctx context.Context, orderNo string) error
}

// SubscriptionOrderService 订阅订单服务
type SubscriptionOrderService struct {
	cfg                 *config.Config
	repo                SubscriptionOrderRepository
	groupRepo           GroupRepository
	wechatPayService    *WeChatPayService
	subscriptionService *SubscriptionService
	entClient           *dbent.Client
	redisClient         *redis.Client
	emailService        *EmailService
	settingService      *SettingService
	userRepo            UserRepository
}

// NewSubscriptionOrderService 创建订阅订单服务
func NewSubscriptionOrderService(
	cfg *config.Config,
	repo SubscriptionOrderRepository,
	groupRepo GroupRepository,
	wechatPayService *WeChatPayService,
	subscriptionService *SubscriptionService,
	entClient *dbent.Client,
	redisClient *redis.Client,
	emailService *EmailService,
	settingService *SettingService,
	userRepo UserRepository,
) *SubscriptionOrderService {
	return &SubscriptionOrderService{
		cfg:                 cfg,
		repo:                repo,
		groupRepo:           groupRepo,
		wechatPayService:    wechatPayService,
		subscriptionService: subscriptionService,
		entClient:           entClient,
		redisClient:         redisClient,
		emailService:        emailService,
		settingService:      settingService,
		userRepo:            userRepo,
	}
}

// GenerateSubscriptionOrderNo 生成订阅订单号
// 格式：SUBS + 年月日时分秒（14位）+ 10位随机字符串
// 示例：SUBS20260124150000AbCd1234Ef
func GenerateSubscriptionOrderNo() string {
	timestamp := time.Now().Format("20060102150405")
	randomStr := generateRandomString(10)
	return fmt.Sprintf("%s%s%s", SubscriptionOrderPrefix, timestamp, randomStr)
}

// IsSubscriptionOrder 判断订单号是否为订阅订单
func IsSubscriptionOrder(orderNo string) bool {
	return len(orderNo) >= 4 && orderNo[:4] == SubscriptionOrderPrefix
}

// CreateOrder 创建订阅订单
func (s *SubscriptionOrderService) CreateOrder(ctx context.Context, userID int64, req *CreateSubscriptionOrderRequest) (*SubscriptionOrder, error) {
	// 检查微信支付是否启用
	if !s.wechatPayService.IsEnabled() {
		return nil, ErrRechargeNotEnabled
	}

	// 获取分组信息
	group, err := s.groupRepo.GetByID(ctx, req.GroupID)
	if err != nil {
		return nil, err
	}

	// 检查分组是否可购买
	if !group.IsPurchasable {
		return nil, ErrGroupNotPurchasable
	}

	// 检查价格是否设置
	if group.PriceCNY == nil || *group.PriceCNY <= 0 {
		return nil, ErrGroupPriceNotSet
	}

	// 设置默认支付渠道
	paymentChannel := req.PaymentChannel
	if paymentChannel == "" {
		paymentChannel = PaymentChannelNative
	}

	// 生成唯一订单号（重试机制确保唯一性）
	var orderNo string
	for i := 0; i < 3; i++ {
		orderNo = GenerateSubscriptionOrderNo()
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

	// 获取有效期天数
	validityDays := group.DefaultValidityDays
	if validityDays <= 0 {
		validityDays = 30
	}

	order := &SubscriptionOrder{
		OrderNo:        orderNo,
		UserID:         userID,
		GroupID:        req.GroupID,
		Amount:         *group.PriceCNY,
		ValidityDays:   validityDays,
		PaymentMethod:  req.PaymentMethod,
		PaymentChannel: paymentChannel,
		Status:         OrderStatusPending,
		ExpireAt:       expireAt,
		Group:          group,
	}

	if err := s.repo.Create(ctx, order); err != nil {
		return nil, fmt.Errorf("create order: %w", err)
	}

	return order, nil
}

// GetOrder 获取订单详情
func (s *SubscriptionOrderService) GetOrder(ctx context.Context, orderNo string) (*SubscriptionOrder, error) {
	return s.repo.GetByOrderNoWithGroup(ctx, orderNo)
}

// GetUserOrder 获取用户的订单详情（带权限校验）
func (s *SubscriptionOrderService) GetUserOrder(ctx context.Context, userID int64, orderNo string) (*SubscriptionOrder, error) {
	order, err := s.repo.GetByOrderNoWithGroup(ctx, orderNo)
	if err != nil {
		return nil, err
	}

	// 校验用户权限：只能查询自己的订单
	if order.UserID != userID {
		return nil, ErrSubscriptionOrderNotFound
	}

	return order, nil
}

// GetExpireMinutes 获取订单过期时间（分钟）
func (s *SubscriptionOrderService) GetExpireMinutes() int {
	if s.cfg.WeChatPay.OrderExpireMinutes > 0 {
		return s.cfg.WeChatPay.OrderExpireMinutes
	}
	return 30
}

// UpdatePaymentResult 更新订单支付结果（保存 prepay_id 或 qrcode_url）
func (s *SubscriptionOrderService) UpdatePaymentResult(ctx context.Context, orderNo string, qrcodeURL, prepayID *string) error {
	order, err := s.repo.GetByOrderNo(ctx, orderNo)
	if err != nil {
		return err
	}

	order.QRCodeURL = qrcodeURL
	order.PrepayID = prepayID

	return s.repo.Update(ctx, order)
}

// ListUserOrders 获取用户的订阅订单列表
func (s *SubscriptionOrderService) ListUserOrders(ctx context.Context, userID int64, req *ListSubscriptionOrdersRequest) (*ListSubscriptionOrdersResult, error) {
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 {
		req.PageSize = 10
	}

	return s.repo.ListByUserID(ctx, userID, req)
}

// CancelOrder 取消未支付订单
func (s *SubscriptionOrderService) CancelOrder(ctx context.Context, userID int64, orderNo string) error {
	order, err := s.repo.GetByOrderNo(ctx, orderNo)
	if err != nil {
		return err
	}

	if order.UserID != userID {
		return ErrSubscriptionOrderNotFound
	}

	if order.Status != OrderStatusPending {
		return ErrOrderCannotBeCancelled
	}

	if time.Now().After(order.ExpireAt) {
		return ErrSubscriptionOrderExpired
	}

	rowsAffected, err := s.repo.UpdateStatusWithCondition(ctx, order.ID, OrderStatusPending, OrderStatusFailed)
	if err != nil {
		return fmt.Errorf("cancel order: %w", err)
	}

	if rowsAffected == 0 {
		return ErrOrderCancelConflict
	}

	return nil
}

// ProcessPaymentSuccessResult 处理支付成功结果
type ProcessPaymentSuccessResult struct {
	Success       bool
	AlreadyPaid   bool
	OrderNo       string
	TransactionID string
	Amount        float64
	UserID        int64
	GroupID       int64
	ValidityDays  int
	IsExtended    bool // 是否是续期
	ErrorMessage  string
}

// 分布式锁常量
const (
	subscriptionCallbackLockKeyPrefix = "subscription:callback:"
	subscriptionCallbackLockTTL       = 30 * time.Second
)

// ProcessPaymentSuccess 处理订阅订单支付成功
func (s *SubscriptionOrderService) ProcessPaymentSuccess(ctx context.Context, orderNo, transactionID string, amountInFen int64) *ProcessPaymentSuccessResult {
	result := &ProcessPaymentSuccessResult{
		OrderNo: orderNo,
	}

	// 1. 获取分布式锁
	lockKey := subscriptionCallbackLockKeyPrefix + orderNo
	locked, err := s.redisClient.SetNX(ctx, lockKey, 1, subscriptionCallbackLockTTL).Result()
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("acquire lock failed: %v", err)
		return result
	}
	if !locked {
		result.Success = true
		result.AlreadyPaid = true
		log.Printf("[SubscriptionOrderService] Order already being processed: order_no=%s", orderNo)
		return result
	}
	defer func() {
		if err := s.redisClient.Del(ctx, lockKey).Err(); err != nil {
			log.Printf("[SubscriptionOrderService] Failed to release lock: order_no=%s, error=%v", orderNo, err)
		}
	}()

	// 2. 查询订单
	order, err := s.repo.GetByOrderNoWithGroup(ctx, orderNo)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("get order failed: %v", err)
		return result
	}
	result.Amount = order.Amount
	result.UserID = order.UserID
	result.GroupID = order.GroupID
	result.ValidityDays = order.ValidityDays
	result.TransactionID = transactionID

	// 3. 检查订单状态
	if order.Status != OrderStatusPending {
		result.Success = true
		result.AlreadyPaid = true
		log.Printf("[SubscriptionOrderService] Order already processed: order_no=%s, status=%s", orderNo, order.Status)
		return result
	}

	// 4. 校验支付金额（防止金额篡改）
	if amountInFen > 0 {
		expectedAmountInFen := int64(order.Amount * 100)
		if amountInFen != expectedAmountInFen {
			result.ErrorMessage = fmt.Sprintf("amount mismatch: expected %d fen, got %d fen", expectedAmountInFen, amountInFen)
			log.Printf("[SubscriptionOrderService] Amount mismatch: order_no=%s, expected=%d, got=%d",
				orderNo, expectedAmountInFen, amountInFen)
			return result
		}
	}

	// 5. 在事务中处理：更新订单状态 + 分配订阅
	err = s.processPaymentInTransaction(ctx, order, transactionID)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("transaction failed: %v", err)
		return result
	}

	result.Success = true
	log.Printf("[SubscriptionOrderService] Payment processed successfully: order_no=%s, user_id=%d, group_id=%d, days=%d",
		orderNo, order.UserID, order.GroupID, order.ValidityDays)

	// 6. 异步发送通知邮件
	go s.sendSubscriptionSuccessNotification(result, order.Group)

	return result
}

// processPaymentInTransaction 在事务中处理支付
func (s *SubscriptionOrderService) processPaymentInTransaction(ctx context.Context, order *SubscriptionOrder, transactionID string) error {
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return fmt.Errorf("start transaction: %w", err)
	}

	defer func() {
		if r := recover(); r != nil {
			_ = tx.Rollback()
			panic(r)
		}
	}()

	txCtx := dbent.NewTxContext(ctx, tx)

	// 1. 更新订单状态
	now := time.Now()
	order.Status = OrderStatusPaid
	order.WeChatTransactionID = &transactionID
	order.PaidAt = &now

	if err := s.repo.Update(txCtx, order); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("update order: %w", err)
	}

	// 2. 分配或续期订阅
	notes := fmt.Sprintf("订阅购买订单 %s", order.OrderNo)
	_, isExtended, err := s.subscriptionService.AssignOrExtendSubscription(txCtx, &AssignSubscriptionInput{
		UserID:       order.UserID,
		GroupID:      order.GroupID,
		ValidityDays: order.ValidityDays,
		AssignedBy:   0, // 系统分配
		Notes:        notes,
	})
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("assign subscription: %w", err)
	}

	_ = isExtended // 可用于日志

	// 3. 提交事务
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// sendSubscriptionSuccessNotification 发送订阅成功通知
func (s *SubscriptionOrderService) sendSubscriptionSuccessNotification(result *ProcessPaymentSuccessResult, group *Group) {
	if s.emailService == nil {
		log.Printf("[SubscriptionOrderService] Email service not available, skip notification: order_no=%s", result.OrderNo)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	user, err := s.userRepo.GetByID(ctx, result.UserID)
	if err != nil {
		log.Printf("[SubscriptionOrderService] Failed to get user for notification: order_no=%s, user_id=%d, error=%v",
			result.OrderNo, result.UserID, err)
		return
	}

	if user.Email == "" {
		log.Printf("[SubscriptionOrderService] User has no email, skip notification: order_no=%s, user_id=%d",
			result.OrderNo, result.UserID)
		return
	}

	subject := "订阅开通成功"
	body := s.buildSubscriptionSuccessEmailBody(result, group, user)

	if err := s.emailService.SendEmail(ctx, user.Email, subject, body); err != nil {
		log.Printf("[SubscriptionOrderService] Failed to send notification email: order_no=%s, email=%s, error=%v",
			result.OrderNo, user.Email, err)
		return
	}

	log.Printf("[SubscriptionOrderService] Notification email sent: order_no=%s, email=%s", result.OrderNo, user.Email)
}

// buildSubscriptionSuccessEmailBody 构建订阅成功邮件内容
func (s *SubscriptionOrderService) buildSubscriptionSuccessEmailBody(result *ProcessPaymentSuccessResult, group *Group, user *User) string {
	paidAt := time.Now().Format("2006-01-02 15:04:05")

	siteName := "Code80"
	siteURL := ""
	ctx := context.Background()
	if s.settingService != nil {
		siteName = s.settingService.GetSiteName(ctx)
		if publicSettings, err := s.settingService.GetPublicSettings(ctx); err == nil && publicSettings.APIBaseURL != "" {
			siteURL = publicSettings.APIBaseURL
		}
	}

	groupName := ""
	dailyLimit := ""
	if group != nil {
		groupName = group.Name
		if group.DailyLimitUSD != nil {
			dailyLimit = fmt.Sprintf("$%.2f/天", *group.DailyLimitUSD)
		}
	}

	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'PingFang SC', 'Hiragino Sans GB', 'Microsoft YaHei', sans-serif;
            background: #fafafa;
            padding: 20px;
            line-height: 1.6;
        }
        .container {
            max-width: 600px;
            margin: 0 auto;
            background: #ffffff;
            border-radius: 16px;
            overflow: hidden;
            box-shadow: 0 2px 12px rgba(217, 119, 87, 0.15);
        }
        .header {
            background: linear-gradient(135deg, #d97757 0%%, #c45a3a 100%%);
            color: white;
            padding: 32px;
            text-align: center;
        }
        .logo {
            width: 48px;
            height: 48px;
            margin: 0 auto 16px;
            background: rgba(255,255,255,0.2);
            border-radius: 12px;
            display: flex;
            align-items: center;
            justify-content: center;
        }
        .logo img {
            width: 32px;
            height: 32px;
        }
        .header h2 {
            font-size: 24px;
            font-weight: 700;
            margin-bottom: 8px;
        }
        .header .subtitle {
            font-size: 14px;
            opacity: 0.9;
        }
        .plan-card {
            background: linear-gradient(135deg, #d97757 0%%, #c45a3a 100%%);
            margin: -20px 24px 24px;
            padding: 24px;
            border-radius: 12px;
            text-align: center;
            color: white;
            box-shadow: 0 4px 16px rgba(217, 119, 87, 0.3);
        }
        .plan-name {
            font-size: 20px;
            font-weight: 700;
            margin-bottom: 8px;
        }
        .plan-limit {
            font-size: 14px;
            opacity: 0.9;
        }
        .content {
            padding: 0 24px 24px;
        }
        .info-card {
            background: #f8f9fa;
            border-radius: 12px;
            padding: 20px;
            margin-bottom: 16px;
        }
        .info-row {
            display: flex;
            justify-content: space-between;
            padding: 12px 0;
            border-bottom: 1px solid #eee;
        }
        .info-row:last-child {
            border-bottom: none;
        }
        .label {
            color: #666;
            font-size: 14px;
        }
        .value {
            font-weight: 600;
            color: #333;
            font-size: 14px;
        }
        .footer {
            padding: 20px 24px;
            text-align: center;
            color: #999;
            font-size: 12px;
            border-top: 1px solid #f0f0f0;
        }
        .site-name {
            color: #d97757;
            font-weight: 600;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <div class="logo">
                <img src="%s/email-logo.png" alt="%s" onerror="this.style.display='none'">
            </div>
            <h2>订阅开通成功</h2>
            <div class="subtitle">您的订阅已生效</div>
        </div>
        <div class="plan-card">
            <div class="plan-name">%s</div>
            <div class="plan-limit">%s · %d天有效期</div>
        </div>
        <div class="content">
            <div class="info-card">
                <div class="info-row">
                    <span class="label">订单号</span>
                    <span class="value">%s</span>
                </div>
                <div class="info-row">
                    <span class="label">支付金额</span>
                    <span class="value">¥%.2f</span>
                </div>
                <div class="info-row">
                    <span class="label">支付时间</span>
                    <span class="value">%s</span>
                </div>
            </div>
        </div>
        <div class="footer">
            <p>感谢您的支持！如有问题，请联系客服。</p>
            <p style="margin-top: 8px;"><span class="site-name">%s</span></p>
        </div>
    </div>
</body>
</html>
`, siteURL, siteName, groupName, dailyLimit, result.ValidityDays, result.OrderNo, result.Amount, paidAt, siteName)
}

// SyncOrderStatus 同步订单状态
func (s *SubscriptionOrderService) SyncOrderStatus(ctx context.Context, userID int64, orderNo string) (*SyncOrderStatusResult, error) {
	order, err := s.repo.GetByOrderNo(ctx, orderNo)
	if err != nil {
		return nil, err
	}

	if order.UserID != userID {
		return nil, ErrSubscriptionOrderNotBelongUser
	}

	if order.Status != OrderStatusPending {
		return &SyncOrderStatusResult{
			OrderNo:      orderNo,
			Status:       order.Status,
			WeChatStatus: "",
			SyncedAt:     time.Now(),
		}, nil
	}

	wechatResult, err := s.wechatPayService.QueryOrder(ctx, orderNo)
	if err != nil {
		return nil, fmt.Errorf("query wechat order: %w", err)
	}

	localStatus := mapWeChatStatusToLocal(wechatResult.TradeState)

	if wechatResult.TradeState == "SUCCESS" && order.Status == OrderStatusPending {
		result := s.ProcessPaymentSuccess(ctx, orderNo, wechatResult.TransactionID, 0)
		if result.Success {
			localStatus = OrderStatusPaid
		} else if result.ErrorMessage != "" {
			log.Printf("[SyncOrderStatus] Compensation failed: order_no=%s, error=%s", orderNo, result.ErrorMessage)
		}
	}

	return &SyncOrderStatusResult{
		OrderNo:      orderNo,
		Status:       localStatus,
		WeChatStatus: wechatResult.TradeState,
		SyncedAt:     time.Now(),
	}, nil
}

// ListAllOrders 获取所有订单（管理端使用）
func (s *SubscriptionOrderService) ListAllOrders(ctx context.Context, req *ListSubscriptionOrdersRequest) (*ListSubscriptionOrdersResult, error) {
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 {
		req.PageSize = 20
	}

	return s.repo.ListAll(ctx, req)
}
