package recharge

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// RechargeHandler 充值相关接口处理器
type RechargeHandler struct {
	wechatPayService         *service.WeChatPayService
	rechargeOrderService     *service.RechargeOrderService
	rechargeRateLimitService *service.RechargeRateLimitService
	turnstileService         *service.TurnstileService
	settingService           *service.SettingService
}

// NewRechargeHandler 创建充值处理器
func NewRechargeHandler(
	wechatPayService *service.WeChatPayService,
	rechargeOrderService *service.RechargeOrderService,
	rechargeRateLimitService *service.RechargeRateLimitService,
	turnstileService *service.TurnstileService,
	settingService *service.SettingService,
) *RechargeHandler {
	return &RechargeHandler{
		wechatPayService:         wechatPayService,
		rechargeOrderService:     rechargeOrderService,
		rechargeRateLimitService: rechargeRateLimitService,
		turnstileService:         turnstileService,
		settingService:           settingService,
	}
}

// RechargeConfigResponse 充值配置响应（公开接口）
type RechargeConfigResponse struct {
	Enabled        bool      `json:"enabled"`
	MinAmount      float64   `json:"min_amount"`
	MaxAmount      float64   `json:"max_amount"`
	DefaultAmounts []float64 `json:"default_amounts"`
	ExchangeRate   float64   `json:"exchange_rate"` // 人民币兑额度汇率
}

// GetConfig 获取充值配置（无需认证）
// GET /api/v1/recharge/config
func (h *RechargeHandler) GetConfig(c *gin.Context) {
	// enabled 从 WeChatPayService 获取
	enabled := h.wechatPayService.IsEnabled()

	// 如果未启用，返回最小响应
	if !enabled {
		response.Success(c, RechargeConfigResponse{
			Enabled:        false,
			MinAmount:      0,
			MaxAmount:      0,
			DefaultAmounts: []float64{},
			ExchangeRate:   1.0,
		})
		return
	}

	// 从 SettingService 获取充值配置（含 exchange_rate）
	settings, err := h.settingService.GetRechargeSettings(c.Request.Context())
	if err != nil {
		log.Printf("[RechargeHandler] GetConfig: failed to get recharge settings: %v", err)
		// 降级：使用默认值
		response.Success(c, RechargeConfigResponse{
			Enabled:        true,
			MinAmount:      1.0,
			MaxAmount:      1000.0,
			DefaultAmounts: []float64{10, 50, 100, 200, 500},
			ExchangeRate:   1.0,
		})
		return
	}

	response.Success(c, RechargeConfigResponse{
		Enabled:        true,
		MinAmount:      settings.MinAmount,
		MaxAmount:      settings.MaxAmount,
		DefaultAmounts: settings.DefaultAmounts,
		ExchangeRate:   settings.ExchangeRate,
	})
}

// ValidateAmountRequest 金额验证请求
type ValidateAmountRequest struct {
	Amount float64 `json:"amount" binding:"required,gt=0"`
}

// ValidateAmountResponse 金额验证响应
type ValidateAmountResponse struct {
	Valid   bool   `json:"valid"`
	Message string `json:"message,omitempty"`
}

// ValidateAmount 验证充值金额（需认证）
// POST /api/v1/recharge/validate-amount
func (h *RechargeHandler) ValidateAmount(c *gin.Context) {
	var req ValidateAmountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请输入有效的金额")
		return
	}

	// 检查微信支付是否启用
	if !h.wechatPayService.IsEnabled() {
		response.Error(c, http.StatusServiceUnavailable, "充值功能暂未开放")
		return
	}

	// 验证金额范围
	if err := h.wechatPayService.ValidateRechargeAmount(req.Amount); err != nil {
		response.Success(c, ValidateAmountResponse{
			Valid:   false,
			Message: err.Error(),
		})
		return
	}

	response.Success(c, ValidateAmountResponse{
		Valid: true,
	})
}

// CreateOrderRequest 创建订单请求
type CreateOrderRequest struct {
	Amount         float64 `json:"amount" binding:"required,gt=0"`
	PaymentMethod  string  `json:"payment_method" binding:"required"`
	PaymentChannel string  `json:"payment_channel"`
	CaptchaToken   string  `json:"captcha_token"` // Turnstile 验证码 token（IP 高频时必填）
}

// CreateOrderResponse 创建订单响应
type CreateOrderResponse struct {
	OrderNo        string    `json:"order_no"`
	Amount         float64   `json:"amount"`
	PaymentMethod  string    `json:"payment_method"`
	PaymentChannel string    `json:"payment_channel"`
	Status         string    `json:"status"`
	ExpireAt       time.Time `json:"expire_at"`
	ExpireMinutes  int       `json:"expire_minutes"`
	CreatedAt      time.Time `json:"created_at"`
}

// CreateOrder 创建充值订单（需认证）
// POST /api/v1/recharge/orders
func (h *RechargeHandler) CreateOrder(c *gin.Context) {
	// 从 context 获取用户 ID
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "未登录")
		return
	}
	userID := subject.UserID

	ctx := c.Request.Context()

	// 检查组合限流（分钟级 + 日级）
	limitResult, err := h.rechargeRateLimitService.CheckRechargeRateLimits(ctx, userID)
	if err != nil {
		log.Printf("[RechargeHandler] check rate limit failed: %v", err)
		// 限流服务异常时不阻止请求，但记录日志
	} else if !limitResult.Allowed {
		respData := gin.H{
			"error":      "rate_limit_exceeded",
			"limit_type": limitResult.LimitType,
			"message":    limitResult.Message,
		}

		if limitResult.LimitType == "minute" {
			c.Header("Retry-After", strconv.Itoa(limitResult.RetryAfter))
			respData["retry_after"] = limitResult.RetryAfter
		} else {
			c.Header("X-RateLimit-Reset", limitResult.ResetTime.Format(time.RFC3339))
			respData["reset_time"] = limitResult.ResetTime.Format(time.RFC3339)
		}

		c.Header("X-RateLimit-Remaining", "0")
		c.JSON(http.StatusTooManyRequests, respData)
		return
	} else {
		if limitResult.MinuteRemaining >= 0 {
			c.Header("X-RateLimit-Minute-Remaining", strconv.Itoa(limitResult.MinuteRemaining))
		}
		if limitResult.DailyRemaining >= 0 {
			c.Header("X-RateLimit-Daily-Remaining", strconv.Itoa(limitResult.DailyRemaining))
		}
	}

	var req CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数无效")
		return
	}

	// 检查 IP 级验证码需求
	clientIP := c.ClientIP()
	ipResult, err := h.rechargeRateLimitService.CheckIPCaptchaRequired(ctx, clientIP)
	if err != nil {
		log.Printf("[RechargeHandler] check IP captcha failed: %v", err)
		// IP 限流服务异常时不阻止请求，但记录日志
	} else if ipResult.NeedsCaptcha {
		// 需要验证码
		if req.CaptchaToken == "" {
			// 未提供验证码，返回 428 Precondition Required
			c.JSON(http.StatusPreconditionRequired, gin.H{
				"error":           "captcha_required",
				"message":         ipResult.Message,
				"captcha_enabled": true,
			})
			return
		}
		// 验证 Turnstile token
		if err := h.turnstileService.VerifyToken(ctx, req.CaptchaToken, clientIP); err != nil {
			log.Printf("[RechargeHandler] turnstile verification failed: %v", err)
			response.BadRequest(c, "验证码验证失败，请重试")
			return
		}
	}

	// 验证支付方式
	if req.PaymentMethod != service.PaymentMethodWeChatPay && req.PaymentMethod != service.PaymentMethodAlipay {
		response.BadRequest(c, "不支持的支付方式")
		return
	}

	// 目前只支持微信支付
	if req.PaymentMethod != service.PaymentMethodWeChatPay {
		response.BadRequest(c, "当前仅支持微信支付")
		return
	}

	// 验证支付渠道（如果提供）
	if req.PaymentChannel != "" {
		if req.PaymentChannel != service.PaymentChannelNative &&
			req.PaymentChannel != service.PaymentChannelJSAPI &&
			req.PaymentChannel != service.PaymentChannelH5 {
			response.BadRequest(c, "不支持的支付渠道")
			return
		}
	}

	// 创建订单
	order, err := h.rechargeOrderService.CreateOrder(c.Request.Context(), userID, &service.CreateRechargeOrderRequest{
		Amount:         req.Amount,
		PaymentMethod:  req.PaymentMethod,
		PaymentChannel: req.PaymentChannel,
	})
	if err != nil {
		if !response.ErrorFrom(c, err) {
			response.InternalError(c, "创建订单失败")
		}
		return
	}

	response.Success(c, CreateOrderResponse{
		OrderNo:        order.OrderNo,
		Amount:         order.Amount,
		PaymentMethod:  order.PaymentMethod,
		PaymentChannel: order.PaymentChannel,
		Status:         order.Status,
		ExpireAt:       order.ExpireAt,
		ExpireMinutes:  h.rechargeOrderService.GetExpireMinutes(),
		CreatedAt:      order.CreatedAt,
	})
}

// OrderDetailResponse 订单详情响应
type OrderDetailResponse struct {
	OrderNo        string     `json:"order_no"`
	Amount         float64    `json:"amount"`
	Status         string     `json:"status"`
	PaymentMethod  string     `json:"payment_method"`
	PaymentChannel string     `json:"payment_channel"`
	QRCodeURL      *string    `json:"qrcode_url,omitempty"` // Native 支付二维码 URL
	PrepayID       *string    `json:"prepay_id,omitempty"`  // JSAPI 预支付 ID
	ExpireAt       time.Time  `json:"expire_at"`
	PaidAt         *time.Time `json:"paid_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

// GetOrder 获取订单详情（需认证）
// GET /api/v1/recharge/orders/:order_no
func (h *RechargeHandler) GetOrder(c *gin.Context) {
	// 从 context 获取用户 ID
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "未登录")
		return
	}
	userID := subject.UserID

	orderNo := c.Param("order_no")
	if orderNo == "" {
		response.BadRequest(c, "订单号不能为空")
		return
	}

	// 获取订单详情（带权限校验）
	order, err := h.rechargeOrderService.GetUserOrder(c.Request.Context(), userID, orderNo)
	if err != nil {
		if !response.ErrorFrom(c, err) {
			response.InternalError(c, "查询订单失败")
		}
		return
	}

	response.Success(c, OrderDetailResponse{
		OrderNo:        order.OrderNo,
		Amount:         order.Amount,
		Status:         order.Status,
		PaymentMethod:  order.PaymentMethod,
		PaymentChannel: order.PaymentChannel,
		QRCodeURL:      order.QRCodeURL,
		PrepayID:       order.PrepayID,
		ExpireAt:       order.ExpireAt,
		PaidAt:         order.PaidAt,
		CreatedAt:      order.CreatedAt,
	})
}

// InitiatePaymentRequest 发起支付请求
type InitiatePaymentRequest struct {
	OpenID string `json:"openid"` // 用户 OpenID（JSAPI 支付必填）
}

// InitiatePaymentResponse 发起支付响应
type InitiatePaymentResponse struct {
	OrderNo        string                      `json:"order_no"`
	PaymentChannel string                      `json:"payment_channel"`
	QRCodeURL      *string                     `json:"qrcode_url,omitempty"`   // Native 支付二维码 URL
	PrepayID       *string                     `json:"prepay_id,omitempty"`    // JSAPI 预支付 ID
	JSAPIParams    *service.JSAPIPaymentParams `json:"jsapi_params,omitempty"` // JSAPI 支付调起参数
}

// InitiatePayment 发起支付（需认证）
// 创建微信支付预支付订单，返回支付参数
// POST /api/v1/recharge/orders/:order_no/pay
func (h *RechargeHandler) InitiatePayment(c *gin.Context) {
	// 从 context 获取用户 ID
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "未登录")
		return
	}
	userID := subject.UserID

	orderNo := c.Param("order_no")
	if orderNo == "" {
		response.BadRequest(c, "订单号不能为空")
		return
	}

	var req InitiatePaymentRequest
	// 允许空请求体（Native 支付不需要 OpenID）
	_ = c.ShouldBindJSON(&req)

	// 获取订单详情（带权限校验）
	order, err := h.rechargeOrderService.GetUserOrder(c.Request.Context(), userID, orderNo)
	if err != nil {
		if !response.ErrorFrom(c, err) {
			response.InternalError(c, "查询订单失败")
		}
		return
	}

	// 检查订单状态
	if order.Status != service.OrderStatusPending {
		response.BadRequest(c, "订单状态不允许支付")
		return
	}

	// 检查订单是否已过期
	if time.Now().After(order.ExpireAt) {
		response.BadRequest(c, "订单已过期")
		return
	}

	// 检查是否已经有支付结果
	if order.QRCodeURL != nil || order.PrepayID != nil {
		// 已经发起过支付
		// 对于 JSAPI 支付，需要重新调用微信支付获取签名参数（签名包含时间戳，无法缓存）
		if order.PaymentChannel == service.PaymentChannelJSAPI {
			// JSAPI 支付需要 OpenID
			if req.OpenID == "" {
				response.BadRequest(c, "JSAPI支付需要提供openid")
				return
			}

			// 重新调用微信支付获取最新的签名参数
			payResult, err := h.wechatPayService.CreateOrder(c.Request.Context(), &service.CreateWeChatPayOrderRequest{
				OrderNo:     order.OrderNo,
				Amount:      order.Amount,
				Description: "账户充值",
				Channel:     order.PaymentChannel,
				OpenID:      req.OpenID,
			})
			if err != nil {
				response.InternalError(c, "发起支付失败: "+err.Error())
				return
			}

			resp := InitiatePaymentResponse{
				OrderNo:        order.OrderNo,
				PaymentChannel: order.PaymentChannel,
				PrepayID:       order.PrepayID,
				JSAPIParams:    payResult.JSAPIParams,
			}
			response.Success(c, resp)
			return
		}

		// Native 支付直接返回现有结果
		resp := InitiatePaymentResponse{
			OrderNo:        order.OrderNo,
			PaymentChannel: order.PaymentChannel,
			QRCodeURL:      order.QRCodeURL,
			PrepayID:       order.PrepayID,
		}
		response.Success(c, resp)
		return
	}

	// JSAPI 支付需要 OpenID
	if order.PaymentChannel == service.PaymentChannelJSAPI && req.OpenID == "" {
		response.BadRequest(c, "JSAPI支付需要提供openid")
		return
	}

	// 调用微信支付创建预支付订单
	payResult, err := h.wechatPayService.CreateOrder(c.Request.Context(), &service.CreateWeChatPayOrderRequest{
		OrderNo:     order.OrderNo,
		Amount:      order.Amount,
		Description: "账户充值",
		Channel:     order.PaymentChannel,
		OpenID:      req.OpenID,
	})
	if err != nil {
		response.InternalError(c, "发起支付失败: "+err.Error())
		return
	}

	// 保存支付结果到订单
	var qrcodeURL, prepayID *string
	if payResult.QRCodeURL != "" {
		qrcodeURL = &payResult.QRCodeURL
	}
	if payResult.PrepayID != "" {
		prepayID = &payResult.PrepayID
	}

	if err := h.rechargeOrderService.UpdatePaymentResult(c.Request.Context(), order.OrderNo, qrcodeURL, prepayID); err != nil {
		// 记录错误但不影响响应（支付已发起）
		c.Error(err)
	}

	// 返回支付参数
	resp := InitiatePaymentResponse{
		OrderNo:        order.OrderNo,
		PaymentChannel: order.PaymentChannel,
		QRCodeURL:      qrcodeURL,
		PrepayID:       prepayID,
		JSAPIParams:    payResult.JSAPIParams,
	}
	response.Success(c, resp)
}

// ListOrdersRequest 订单列表请求参数
type ListOrdersRequest struct {
	Page      int    `form:"page"`
	PageSize  int    `form:"page_size"`
	Status    string `form:"status"`
	StartTime string `form:"start_time"` // RFC3339 格式
	EndTime   string `form:"end_time"`   // RFC3339 格式
}

// OrderListItem 订单列表项
type OrderListItem struct {
	OrderNo   string     `json:"order_no"`
	Amount    float64    `json:"amount"`
	Status    string     `json:"status"`
	CreatedAt time.Time  `json:"created_at"`
	PaidAt    *time.Time `json:"paid_at,omitempty"`
}

// ListOrdersResponse 订单列表响应
type ListOrdersResponse struct {
	Orders   []OrderListItem `json:"orders"`
	Total    int64           `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
}

// ListOrders 获取充值记录列表（需认证）
// GET /api/v1/recharge/orders
func (h *RechargeHandler) ListOrders(c *gin.Context) {
	// 从 context 获取用户 ID
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "未登录")
		return
	}
	userID := subject.UserID

	// 解析请求参数
	var req ListOrdersRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "请求参数无效")
		return
	}

	// 设置默认分页参数
	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize < 1 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}

	// 验证状态参数
	if req.Status != "" {
		validStatuses := map[string]bool{
			service.OrderStatusPending:   true,
			service.OrderStatusPaid:      true,
			service.OrderStatusFailed:    true,
			service.OrderStatusExpired:   true,
			service.OrderStatusCancelled: true,
		}
		if !validStatuses[req.Status] {
			response.BadRequest(c, "无效的订单状态")
			return
		}
	}

	// 解析时间范围
	var startTime, endTime *time.Time
	if req.StartTime != "" {
		t, err := time.Parse(time.RFC3339, req.StartTime)
		if err != nil {
			// 尝试解析日期格式
			t, err = time.Parse("2006-01-02", req.StartTime)
			if err != nil {
				response.BadRequest(c, "开始时间格式无效")
				return
			}
		}
		startTime = &t
	}
	if req.EndTime != "" {
		t, err := time.Parse(time.RFC3339, req.EndTime)
		if err != nil {
			// 尝试解析日期格式，并设置为当天结束
			t, err = time.Parse("2006-01-02", req.EndTime)
			if err != nil {
				response.BadRequest(c, "结束时间格式无效")
				return
			}
			t = t.Add(24*time.Hour - time.Second)
		}
		endTime = &t
	}

	// 构建查询请求
	listReq := &service.ListRechargeOrdersRequest{
		Status:    req.Status,
		StartTime: startTime,
		EndTime:   endTime,
	}
	listReq.Page = page
	listReq.PageSize = pageSize

	// 查询订单列表
	result, err := h.rechargeOrderService.ListUserOrders(c.Request.Context(), userID, listReq)
	if err != nil {
		response.InternalError(c, "查询订单失败")
		return
	}

	// 转换响应
	orders := make([]OrderListItem, len(result.Orders))
	for i, order := range result.Orders {
		orders[i] = OrderListItem{
			OrderNo:   order.OrderNo,
			Amount:    order.Amount,
			Status:    order.Status,
			CreatedAt: order.CreatedAt,
			PaidAt:    order.PaidAt,
		}
	}

	response.Success(c, ListOrdersResponse{
		Orders:   orders,
		Total:    result.Pagination.Total,
		Page:     result.Pagination.Page,
		PageSize: result.Pagination.PageSize,
	})
}

// CancelOrderResponse 取消订单响应
type CancelOrderResponse struct {
	OrderNo string `json:"order_no"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

// CancelOrder 取消未支付订单（需认证）
// POST /api/v1/recharge/orders/:order_no/cancel
func (h *RechargeHandler) CancelOrder(c *gin.Context) {
	// 从 context 获取用户 ID
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "未登录")
		return
	}
	userID := subject.UserID

	orderNo := c.Param("order_no")
	if orderNo == "" {
		response.BadRequest(c, "订单号不能为空")
		return
	}

	// 取消订单
	if err := h.rechargeOrderService.CancelOrder(c.Request.Context(), userID, orderNo); err != nil {
		if !response.ErrorFrom(c, err) {
			response.InternalError(c, "取消订单失败")
		}
		return
	}

	response.Success(c, CancelOrderResponse{
		OrderNo: orderNo,
		Status:  service.OrderStatusFailed,
		Message: "订单已取消",
	})
}

// parseIntParam 解析整数参数
func parseIntParam(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return val
}

// SyncOrderStatusResponse 同步订单状态响应
type SyncOrderStatusResponse struct {
	OrderNo      string `json:"order_no"`
	Status       string `json:"status"`        // 本地订单状态: pending/paid/failed/expired/refunded
	WeChatStatus string `json:"wechat_status"` // 微信侧原始状态
	SyncedAt     string `json:"synced_at"`     // 同步时间（RFC3339格式）
}

// SyncOrderStatus 同步订单状态（需认证）
// 手动调用微信支付查询接口同步订单状态
// POST /api/v1/recharge/orders/:order_no/sync
func (h *RechargeHandler) SyncOrderStatus(c *gin.Context) {
	// 从 context 获取用户 ID
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "未登录")
		return
	}
	userID := subject.UserID

	orderNo := c.Param("order_no")
	if orderNo == "" {
		response.BadRequest(c, "订单号不能为空")
		return
	}

	// 调用服务层同步订单状态
	result, err := h.rechargeOrderService.SyncOrderStatus(c.Request.Context(), userID, orderNo)
	if err != nil {
		log.Printf("[RechargeHandler] sync order status failed: order_no=%s, user_id=%d, error=%v",
			orderNo, userID, err)
		if !response.ErrorFrom(c, err) {
			response.InternalError(c, "同步订单状态失败")
		}
		return
	}

	log.Printf("[RechargeHandler] sync order status success: order_no=%s, status=%s, wechat_status=%s",
		orderNo, result.Status, result.WeChatStatus)

	response.Success(c, SyncOrderStatusResponse{
		OrderNo:      result.OrderNo,
		Status:       result.Status,
		WeChatStatus: result.WeChatStatus,
		SyncedAt:     result.SyncedAt.Format(time.RFC3339),
	})
}
