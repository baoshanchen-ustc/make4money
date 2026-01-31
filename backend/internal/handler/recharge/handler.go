package recharge

import (
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

	// 使用默认配置值（后续 Story 1.3 实现动态配置后可替换）
	response.Success(c, RechargeConfigResponse{
		Enabled:        true,
		MinAmount:      1.0,
		MaxAmount:      1000.0,
		DefaultAmounts: []float64{10, 50, 100, 200, 500},
	})
}
