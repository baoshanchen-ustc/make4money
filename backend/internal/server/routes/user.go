package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterUserRoutes 注册用户相关路由（需要认证）
func RegisterUserRoutes(
	v1 *gin.RouterGroup,
	h *handler.Handlers,
	jwtAuth middleware.JWTAuthMiddleware,
) {
	authenticated := v1.Group("")
	authenticated.Use(gin.HandlerFunc(jwtAuth))
	{
		// 用户接口
		user := authenticated.Group("/user")
		{
			user.GET("/profile", h.User.GetProfile)
			user.PUT("/password", h.User.ChangePassword)
			user.PUT("", h.User.UpdateProfile)

			// TOTP 双因素认证
			totp := user.Group("/totp")
			{
				totp.GET("/status", h.Totp.GetStatus)
				totp.GET("/verification-method", h.Totp.GetVerificationMethod)
				totp.POST("/send-code", h.Totp.SendVerifyCode)
				totp.POST("/setup", h.Totp.InitiateSetup)
				totp.POST("/enable", h.Totp.Enable)
				totp.POST("/disable", h.Totp.Disable)
			}
		}

		// API Key管理
		keys := authenticated.Group("/keys")
		{
			keys.GET("", h.APIKey.List)
			keys.GET("/:id", h.APIKey.GetByID)
			keys.POST("", h.APIKey.Create)
			keys.PUT("/:id", h.APIKey.Update)
			keys.DELETE("/:id", h.APIKey.Delete)
		}

		// 用户可用分组（非管理员接口）
		groups := authenticated.Group("/groups")
		{
			groups.GET("/available", h.APIKey.GetAvailableGroups)
		}

		// 使用记录
		usage := authenticated.Group("/usage")
		{
			usage.GET("", h.Usage.List)
			usage.GET("/:id", h.Usage.GetByID)
			usage.GET("/stats", h.Usage.Stats)
			// User dashboard endpoints
			usage.GET("/dashboard/stats", h.Usage.DashboardStats)
			usage.GET("/dashboard/trend", h.Usage.DashboardTrend)
			usage.GET("/dashboard/models", h.Usage.DashboardModels)
			usage.POST("/dashboard/api-keys-usage", h.Usage.DashboardAPIKeysUsage)
		}

		// 卡密兑换
		redeem := authenticated.Group("/redeem")
		{
			redeem.POST("", h.Redeem.Redeem)
			redeem.GET("/history", h.Redeem.GetHistory)
		}

		// 用户订阅
		subscriptions := authenticated.Group("/subscriptions")
		{
			subscriptions.GET("", h.Subscription.List)
			subscriptions.GET("/active", h.Subscription.GetActive)
			subscriptions.GET("/progress", h.Subscription.GetProgress)
			subscriptions.GET("/summary", h.Subscription.GetSummary)
		}

		// 使用报告
		usageReport := authenticated.Group("/usage-report")
		{
			usageReport.GET("/config", h.UsageReport.GetConfig)
			usageReport.PUT("/config", h.UsageReport.UpdateConfig)
			usageReport.POST("/test", h.UsageReport.SendTestReport)
		}

		// 充值（需认证的接口）
		recharge := authenticated.Group("/recharge")
		{
			recharge.POST("/validate-amount", h.Recharge.ValidateAmount)
			recharge.GET("/orders", h.Recharge.ListOrders)
			recharge.POST("/orders", h.Recharge.CreateOrder)
			recharge.GET("/orders/:order_no", h.Recharge.GetOrder)
			recharge.POST("/orders/:order_no/pay", h.Recharge.InitiatePayment)
			recharge.POST("/orders/:order_no/cancel", h.Recharge.CancelOrder)
			recharge.POST("/orders/:order_no/sync", h.Recharge.SyncOrderStatus)
		}

		// 订阅套餐订单（需认证的接口）
		subscriptionOrders := authenticated.Group("/subscription-orders")
		{
			subscriptionOrders.POST("", h.SubscriptionPlan.CreateOrder)
			subscriptionOrders.GET("", h.SubscriptionPlan.ListOrders)
			subscriptionOrders.GET("/:order_no", h.SubscriptionPlan.GetOrder)
			subscriptionOrders.POST("/:order_no/pay", h.SubscriptionPlan.InitiatePayment)
			subscriptionOrders.POST("/:order_no/cancel", h.SubscriptionPlan.CancelOrder)
			subscriptionOrders.POST("/:order_no/sync", h.SubscriptionPlan.SyncOrderStatus)
		}
	}
}
