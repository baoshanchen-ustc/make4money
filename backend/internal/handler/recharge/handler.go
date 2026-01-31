package recharge

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// RechargeHandler 充值相关接口处理器
type RechargeHandler struct {
	wechatPayService *service.WeChatPayService
}

// NewRechargeHandler 创建充值处理器
func NewRechargeHandler(wechatPayService *service.WeChatPayService) *RechargeHandler {
	return &RechargeHandler{
		wechatPayService: wechatPayService,
	}
}

// RechargeConfigResponse 充值配置响应（公开接口）
type RechargeConfigResponse struct {
	Enabled        bool      `json:"enabled"`
	MinAmount      float64   `json:"min_amount"`
	MaxAmount      float64   `json:"max_amount"`
	DefaultAmounts []float64 `json:"default_amounts"`
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
		})
		return
	}

	// 从 service 获取充值配置
	cfg := h.wechatPayService.GetRechargeConfig()
	response.Success(c, RechargeConfigResponse{
		Enabled:        true,
		MinAmount:      cfg.MinAmount,
		MaxAmount:      cfg.MaxAmount,
		DefaultAmounts: cfg.DefaultAmounts,
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
