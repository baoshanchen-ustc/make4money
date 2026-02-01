package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/gin-gonic/gin"
)

// RegisterWebhookRoutes 注册 Webhook 路由（公开接口，无需认证）
// 用于接收第三方平台的回调通知
func RegisterWebhookRoutes(v1 *gin.RouterGroup, h *handler.Handlers) {
	webhook := v1.Group("/webhook")
	{
		// 微信支付回调
		wechat := webhook.Group("/wechat")
		{
			// POST /api/v1/webhook/wechat/payment
			// 接收微信支付平台的支付结果通知
			wechat.POST("/payment", h.WeChatPayWebhook.HandlePaymentNotify)

			// POST /api/v1/webhook/wechat/refund
			// 接收微信支付平台的退款结果通知
			wechat.POST("/refund", h.WeChatPayWebhook.HandleRefundNotify)
		}
	}
}
