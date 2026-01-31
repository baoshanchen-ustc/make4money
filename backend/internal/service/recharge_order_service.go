package service

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
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

var (
	ErrRechargeOrderNotFound = infraerrors.NotFound("RECHARGE_ORDER_NOT_FOUND", "recharge order not found")
	ErrRechargeOrderExpired  = infraerrors.BadRequest("RECHARGE_ORDER_EXPIRED", "recharge order has expired")
	ErrRechargeNotEnabled    = infraerrors.BadRequest("RECHARGE_NOT_ENABLED", "recharge is not enabled")
	ErrInvalidAmount         = infraerrors.BadRequest("INVALID_AMOUNT", "invalid recharge amount")
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
}

// RechargeOrderService 充值订单服务
type RechargeOrderService struct {
	cfg              *config.Config
	repo             RechargeOrderRepository
	wechatPayService *WeChatPayService
}

// NewRechargeOrderService 创建充值订单服务
func NewRechargeOrderService(
	cfg *config.Config,
	repo RechargeOrderRepository,
	wechatPayService *WeChatPayService,
) *RechargeOrderService {
	return &RechargeOrderService{
		cfg:              cfg,
		repo:             repo,
		wechatPayService: wechatPayService,
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
