package subscription

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

// SubscriptionPlanHandler 订阅套餐处理器
type SubscriptionPlanHandler struct {
	groupRepo                 service.GroupRepository
	wechatPayService          *service.WeChatPayService
	subscriptionOrderService  *service.SubscriptionOrderService
	rechargeRateLimitService  *service.RechargeRateLimitService
	turnstileService          *service.TurnstileService
	settingService            *service.SettingService
}

// NewSubscriptionPlanHandler 创建订阅套餐处理器
func NewSubscriptionPlanHandler(
	groupRepo service.GroupRepository,
	wechatPayService *service.WeChatPayService,
	subscriptionOrderService *service.SubscriptionOrderService,
	rechargeRateLimitService *service.RechargeRateLimitService,
	turnstileService *service.TurnstileService,
	settingService *service.SettingService,
) *SubscriptionPlanHandler {
	return &SubscriptionPlanHandler{
		groupRepo:                 groupRepo,
		wechatPayService:          wechatPayService,
		subscriptionOrderService:  subscriptionOrderService,
		rechargeRateLimitService:  rechargeRateLimitService,
		turnstileService:          turnstileService,
		settingService:            settingService,
	}
}

// PlanResponse 套餐响应
type PlanResponse struct {
	ID                  int64    `json:"id"`
	Name                string   `json:"name"`
	Description         *string  `json:"description,omitempty"`
	PriceCNY            float64  `json:"price_cny"`
	ValidityDays        int      `json:"validity_days"`
	DailyLimitUSD       *float64 `json:"daily_limit_usd,omitempty"`
	WeeklyLimitUSD      *float64 `json:"weekly_limit_usd,omitempty"`
	MonthlyLimitUSD     *float64 `json:"monthly_limit_usd,omitempty"`
	DisplayOrder        int      `json:"display_order"`
}

// ListPlansResponse 套餐列表响应
type ListPlansResponse struct {
	Plans         []PlanResponse `json:"plans"`
	PaymentEnabled bool          `json:"payment_enabled"`
	ContactInfo   string         `json:"contact_info,omitempty"`
}

// ListPlans 获取可购买套餐列表（公开接口）
// GET /api/v1/subscription-plans
func (h *SubscriptionPlanHandler) ListPlans(c *gin.Context) {
	ctx := c.Request.Context()

	// 检查支付是否启用
	paymentEnabled := h.wechatPayService.IsEnabled()

	// 获取联系信息
	contactInfo := ""
	if h.settingService != nil {
		if publicSettings, err := h.settingService.GetPublicSettings(ctx); err == nil {
			contactInfo = publicSettings.ContactInfo
		}
	}

	// 如果支付未启用，返回空列表
	if !paymentEnabled {
		response.Success(c, ListPlansResponse{
			Plans:          []PlanResponse{},
			PaymentEnabled: false,
			ContactInfo:    contactInfo,
		})
		return
	}

	// 获取可购买分组
	groups, err := h.groupRepo.ListPurchasable(ctx)
	if err != nil {
		log.Printf("[SubscriptionPlanHandler] ListPlans: failed to list purchasable groups: %v", err)
		response.InternalError(c, "获取套餐列表失败")
		return
	}

	// 转换为响应
	plans := make([]PlanResponse, 0, len(groups))
	for _, g := range groups {
		if g.PriceCNY == nil || *g.PriceCNY <= 0 {
			continue
		}
		plan := PlanResponse{
			ID:              g.ID,
			Name:            g.Name,
			Description:     g.PurchasableDescription,
			PriceCNY:        *g.PriceCNY,
			ValidityDays:    g.DefaultValidityDays,
			DailyLimitUSD:   g.DailyLimitUSD,
			WeeklyLimitUSD:  g.WeeklyLimitUSD,
			MonthlyLimitUSD: g.MonthlyLimitUSD,
			DisplayOrder:    g.DisplayOrder,
		}
		plans = append(plans, plan)
	}

	response.Success(c, ListPlansResponse{
		Plans:          plans,
		PaymentEnabled: true,
		ContactInfo:    contactInfo,
	})
}

// CreateOrderRequest 创建订阅订单请求
type CreateOrderRequest struct {
	GroupID        int64  `json:"group_id" binding:"required"`
	PaymentMethod  string `json:"payment_method" binding:"required"`
	PaymentChannel string `json:"payment_channel"`
	CaptchaToken   string `json:"captcha_token"`
}

// CreateOrderResponse 创建订单响应
type CreateOrderResponse struct {
	OrderNo        string    `json:"order_no"`
	GroupID        int64     `json:"group_id"`
	GroupName      string    `json:"group_name"`
	Amount         float64   `json:"amount"`
	ValidityDays   int       `json:"validity_days"`
	PaymentMethod  string    `json:"payment_method"`
	PaymentChannel string    `json:"payment_channel"`
	Status         string    `json:"status"`
	ExpireAt       time.Time `json:"expire_at"`
	ExpireMinutes  int       `json:"expire_minutes"`
	CreatedAt      time.Time `json:"created_at"`
}

// CreateOrder 创建订阅订单（需认证）
// POST /api/v1/subscription-orders
func (h *SubscriptionPlanHandler) CreateOrder(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "未登录")
		return
	}
	userID := subject.UserID
	ctx := c.Request.Context()

	// 检查限流
	limitResult, err := h.rechargeRateLimitService.CheckRechargeRateLimits(ctx, userID)
	if err != nil {
		log.Printf("[SubscriptionPlanHandler] check rate limit failed: %v", err)
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
		c.JSON(http.StatusTooManyRequests, respData)
		return
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
		log.Printf("[SubscriptionPlanHandler] check IP captcha failed: %v", err)
	} else if ipResult.NeedsCaptcha {
		if req.CaptchaToken == "" {
			c.JSON(http.StatusPreconditionRequired, gin.H{
				"error":           "captcha_required",
				"message":         ipResult.Message,
				"captcha_enabled": true,
			})
			return
		}
		if err := h.turnstileService.VerifyToken(ctx, req.CaptchaToken, clientIP); err != nil {
			log.Printf("[SubscriptionPlanHandler] turnstile verification failed: %v", err)
			response.BadRequest(c, "验证码验证失败，请重试")
			return
		}
	}

	// 验证支付方式
	if req.PaymentMethod != service.PaymentMethodWeChatPay && req.PaymentMethod != service.PaymentMethodAlipay {
		response.BadRequest(c, "不支持的支付方式")
		return
	}

	if req.PaymentMethod != service.PaymentMethodWeChatPay {
		response.BadRequest(c, "当前仅支持微信支付")
		return
	}

	if req.PaymentChannel != "" {
		if req.PaymentChannel != service.PaymentChannelNative &&
			req.PaymentChannel != service.PaymentChannelJSAPI &&
			req.PaymentChannel != service.PaymentChannelH5 {
			response.BadRequest(c, "不支持的支付渠道")
			return
		}
	}

	// 创建订单
	order, err := h.subscriptionOrderService.CreateOrder(ctx, userID, &service.CreateSubscriptionOrderRequest{
		GroupID:        req.GroupID,
		PaymentMethod:  req.PaymentMethod,
		PaymentChannel: req.PaymentChannel,
	})
	if err != nil {
		if !response.ErrorFrom(c, err) {
			response.InternalError(c, "创建订单失败")
		}
		return
	}

	groupName := ""
	if order.Group != nil {
		groupName = order.Group.Name
	}

	response.Success(c, CreateOrderResponse{
		OrderNo:        order.OrderNo,
		GroupID:        order.GroupID,
		GroupName:      groupName,
		Amount:         order.Amount,
		ValidityDays:   order.ValidityDays,
		PaymentMethod:  order.PaymentMethod,
		PaymentChannel: order.PaymentChannel,
		Status:         order.Status,
		ExpireAt:       order.ExpireAt,
		ExpireMinutes:  h.subscriptionOrderService.GetExpireMinutes(),
		CreatedAt:      order.CreatedAt,
	})
}

// OrderDetailResponse 订单详情响应
type OrderDetailResponse struct {
	OrderNo        string     `json:"order_no"`
	GroupID        int64      `json:"group_id"`
	GroupName      string     `json:"group_name"`
	Amount         float64    `json:"amount"`
	ValidityDays   int        `json:"validity_days"`
	Status         string     `json:"status"`
	PaymentMethod  string     `json:"payment_method"`
	PaymentChannel string     `json:"payment_channel"`
	QRCodeURL      *string    `json:"qrcode_url,omitempty"`
	PrepayID       *string    `json:"prepay_id,omitempty"`
	ExpireAt       time.Time  `json:"expire_at"`
	PaidAt         *time.Time `json:"paid_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

// GetOrder 获取订单详情（需认证）
// GET /api/v1/subscription-orders/:order_no
func (h *SubscriptionPlanHandler) GetOrder(c *gin.Context) {
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

	order, err := h.subscriptionOrderService.GetUserOrder(c.Request.Context(), userID, orderNo)
	if err != nil {
		if !response.ErrorFrom(c, err) {
			response.InternalError(c, "查询订单失败")
		}
		return
	}

	groupName := ""
	if order.Group != nil {
		groupName = order.Group.Name
	}

	response.Success(c, OrderDetailResponse{
		OrderNo:        order.OrderNo,
		GroupID:        order.GroupID,
		GroupName:      groupName,
		Amount:         order.Amount,
		ValidityDays:   order.ValidityDays,
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
	OpenID string `json:"openid"`
}

// InitiatePaymentResponse 发起支付响应
type InitiatePaymentResponse struct {
	OrderNo        string                      `json:"order_no"`
	PaymentChannel string                      `json:"payment_channel"`
	QRCodeURL      *string                     `json:"qrcode_url,omitempty"`
	PrepayID       *string                     `json:"prepay_id,omitempty"`
	JSAPIParams    *service.JSAPIPaymentParams `json:"jsapi_params,omitempty"`
}

// InitiatePayment 发起支付（需认证）
// POST /api/v1/subscription-orders/:order_no/pay
func (h *SubscriptionPlanHandler) InitiatePayment(c *gin.Context) {
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
	_ = c.ShouldBindJSON(&req)

	order, err := h.subscriptionOrderService.GetUserOrder(c.Request.Context(), userID, orderNo)
	if err != nil {
		if !response.ErrorFrom(c, err) {
			response.InternalError(c, "查询订单失败")
		}
		return
	}

	if order.Status != service.OrderStatusPending {
		response.BadRequest(c, "订单状态不允许支付")
		return
	}

	if time.Now().After(order.ExpireAt) {
		response.BadRequest(c, "订单已过期")
		return
	}

	// 已经发起过支付
	if order.QRCodeURL != nil || order.PrepayID != nil {
		if order.PaymentChannel == service.PaymentChannelJSAPI {
			if req.OpenID == "" {
				response.BadRequest(c, "JSAPI支付需要提供openid")
				return
			}

			groupName := ""
			if order.Group != nil {
				groupName = order.Group.Name
			}

			payResult, err := h.wechatPayService.CreateOrder(c.Request.Context(), &service.CreateWeChatPayOrderRequest{
				OrderNo:     order.OrderNo,
				Amount:      order.Amount,
				Description: "订阅套餐 - " + groupName,
				Channel:     order.PaymentChannel,
				OpenID:      req.OpenID,
			})
			if err != nil {
				response.InternalError(c, "发起支付失败: "+err.Error())
				return
			}

			response.Success(c, InitiatePaymentResponse{
				OrderNo:        order.OrderNo,
				PaymentChannel: order.PaymentChannel,
				PrepayID:       order.PrepayID,
				JSAPIParams:    payResult.JSAPIParams,
			})
			return
		}

		response.Success(c, InitiatePaymentResponse{
			OrderNo:        order.OrderNo,
			PaymentChannel: order.PaymentChannel,
			QRCodeURL:      order.QRCodeURL,
			PrepayID:       order.PrepayID,
		})
		return
	}

	if order.PaymentChannel == service.PaymentChannelJSAPI && req.OpenID == "" {
		response.BadRequest(c, "JSAPI支付需要提供openid")
		return
	}

	groupName := ""
	if order.Group != nil {
		groupName = order.Group.Name
	}

	payResult, err := h.wechatPayService.CreateOrder(c.Request.Context(), &service.CreateWeChatPayOrderRequest{
		OrderNo:     order.OrderNo,
		Amount:      order.Amount,
		Description: "订阅套餐 - " + groupName,
		Channel:     order.PaymentChannel,
		OpenID:      req.OpenID,
	})
	if err != nil {
		response.InternalError(c, "发起支付失败: "+err.Error())
		return
	}

	var qrcodeURL, prepayID *string
	if payResult.QRCodeURL != "" {
		qrcodeURL = &payResult.QRCodeURL
	}
	if payResult.PrepayID != "" {
		prepayID = &payResult.PrepayID
	}

	if err := h.subscriptionOrderService.UpdatePaymentResult(c.Request.Context(), order.OrderNo, qrcodeURL, prepayID); err != nil {
		c.Error(err)
	}

	response.Success(c, InitiatePaymentResponse{
		OrderNo:        order.OrderNo,
		PaymentChannel: order.PaymentChannel,
		QRCodeURL:      qrcodeURL,
		PrepayID:       prepayID,
		JSAPIParams:    payResult.JSAPIParams,
	})
}

// ListOrdersRequest 订单列表请求
type ListOrdersRequest struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	Status   string `form:"status"`
}

// OrderListItem 订单列表项
type OrderListItem struct {
	OrderNo      string     `json:"order_no"`
	GroupID      int64      `json:"group_id"`
	GroupName    string     `json:"group_name"`
	Amount       float64    `json:"amount"`
	ValidityDays int        `json:"validity_days"`
	Status       string     `json:"status"`
	CreatedAt    time.Time  `json:"created_at"`
	PaidAt       *time.Time `json:"paid_at,omitempty"`
}

// ListOrdersResponse 订单列表响应
type ListOrdersResponse struct {
	Orders   []OrderListItem `json:"orders"`
	Total    int64           `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
}

// ListOrders 获取订阅订单列表（需认证）
// GET /api/v1/subscription-orders
func (h *SubscriptionPlanHandler) ListOrders(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "未登录")
		return
	}
	userID := subject.UserID

	var req ListOrdersRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "请求参数无效")
		return
	}

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

	listReq := &service.ListSubscriptionOrdersRequest{
		Status: req.Status,
	}
	listReq.Page = page
	listReq.PageSize = pageSize

	result, err := h.subscriptionOrderService.ListUserOrders(c.Request.Context(), userID, listReq)
	if err != nil {
		response.InternalError(c, "查询订单失败")
		return
	}

	orders := make([]OrderListItem, len(result.Orders))
	for i, order := range result.Orders {
		groupName := ""
		if order.Group != nil {
			groupName = order.Group.Name
		}
		orders[i] = OrderListItem{
			OrderNo:      order.OrderNo,
			GroupID:      order.GroupID,
			GroupName:    groupName,
			Amount:       order.Amount,
			ValidityDays: order.ValidityDays,
			Status:       order.Status,
			CreatedAt:    order.CreatedAt,
			PaidAt:       order.PaidAt,
		}
	}

	response.Success(c, ListOrdersResponse{
		Orders:   orders,
		Total:    result.Pagination.Total,
		Page:     result.Pagination.Page,
		PageSize: result.Pagination.PageSize,
	})
}

// CancelOrder 取消订单（需认证）
// POST /api/v1/subscription-orders/:order_no/cancel
func (h *SubscriptionPlanHandler) CancelOrder(c *gin.Context) {
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

	if err := h.subscriptionOrderService.CancelOrder(c.Request.Context(), userID, orderNo); err != nil {
		if !response.ErrorFrom(c, err) {
			response.InternalError(c, "取消订单失败")
		}
		return
	}

	response.Success(c, gin.H{
		"order_no": orderNo,
		"status":   service.OrderStatusFailed,
		"message":  "订单已取消",
	})
}

// SyncOrderStatus 同步订单状态（需认证）
// POST /api/v1/subscription-orders/:order_no/sync
func (h *SubscriptionPlanHandler) SyncOrderStatus(c *gin.Context) {
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

	result, err := h.subscriptionOrderService.SyncOrderStatus(c.Request.Context(), userID, orderNo)
	if err != nil {
		log.Printf("[SubscriptionPlanHandler] sync order status failed: order_no=%s, user_id=%d, error=%v",
			orderNo, userID, err)
		if !response.ErrorFrom(c, err) {
			response.InternalError(c, "同步订单状态失败")
		}
		return
	}

	response.Success(c, gin.H{
		"order_no":      result.OrderNo,
		"status":        result.Status,
		"wechat_status": result.WeChatStatus,
		"synced_at":     result.SyncedAt.Format(time.RFC3339),
	})
}
