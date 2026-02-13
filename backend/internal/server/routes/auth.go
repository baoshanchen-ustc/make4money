package routes

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/middleware"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// RegisterAuthRoutes 注册认证相关路由
func RegisterAuthRoutes(
	v1 *gin.RouterGroup,
	h *handler.Handlers,
	jwtAuth servermiddleware.JWTAuthMiddleware,
	redisClient *redis.Client,
) {
	// 创建速率限制器
	rateLimiter := middleware.NewRateLimiter(redisClient)

	// 公开接口
	auth := v1.Group("/auth")
	{
		auth.POST("/register", h.Auth.Register)
		auth.POST("/login", h.Auth.Login)
		auth.POST("/login/2fa", h.Auth.Login2FA)
		auth.POST("/send-verify-code", h.Auth.SendVerifyCode)
		// Token刷新接口添加速率限制：每分钟最多 30 次（Redis 故障时 fail-close）
		auth.POST("/refresh", rateLimiter.LimitWithOptions("refresh-token", 30, time.Minute, middleware.RateLimitOptions{
			FailureMode: middleware.RateLimitFailClose,
		}), h.Auth.RefreshToken)
		// 登出接口（公开，允许未认证用户调用以撤销Refresh Token）
		auth.POST("/logout", h.Auth.Logout)
		// 优惠码验证接口添加速率限制：每分钟最多 10 次（Redis 故障时 fail-close）
		auth.POST("/validate-promo-code", rateLimiter.LimitWithOptions("validate-promo", 10, time.Minute, middleware.RateLimitOptions{
			FailureMode: middleware.RateLimitFailClose,
		}), h.Auth.ValidatePromoCode)
		// 邀请码验证接口添加速率限制：每分钟最多 10 次（Redis 故障时 fail-close）
		auth.POST("/validate-invitation-code", rateLimiter.LimitWithOptions("validate-invitation", 10, time.Minute, middleware.RateLimitOptions{
			FailureMode: middleware.RateLimitFailClose,
		}), h.Auth.ValidateInvitationCode)
		// 忘记密码接口添加速率限制：每分钟最多 5 次（Redis 故障时 fail-close）
		auth.POST("/forgot-password", rateLimiter.LimitWithOptions("forgot-password", 5, time.Minute, middleware.RateLimitOptions{
			FailureMode: middleware.RateLimitFailClose,
		}), h.Auth.ForgotPassword)
		// 重置密码接口添加速率限制：每分钟最多 10 次（Redis 故障时 fail-close）
		auth.POST("/reset-password", rateLimiter.LimitWithOptions("reset-password", 10, time.Minute, middleware.RateLimitOptions{
			FailureMode: middleware.RateLimitFailClose,
		}), h.Auth.ResetPassword)
		auth.GET("/oauth/linuxdo/start", h.Auth.LinuxDoOAuthStart)
		auth.GET("/oauth/linuxdo/callback", h.Auth.LinuxDoOAuthCallback)
		// 微信公众号验证码登录
		auth.GET("/oauth/wechat", rateLimiter.LimitWithOptions("wechat-auth", 20, 20*time.Minute, middleware.RateLimitOptions{
			FailureMode: middleware.RateLimitFailClose,
		}), h.Auth.WeChatAuth)
		// 微信订阅号扫码自动登录
		auth.GET("/wechat/scan/init", rateLimiter.LimitWithOptions("wechat-scan-init", 30, time.Minute, middleware.RateLimitOptions{
			FailureMode: middleware.RateLimitFailClose,
		}), h.Auth.WeChatScanInit)
		auth.GET("/wechat/scan/poll", rateLimiter.LimitWithOptions("wechat-scan-poll", 120, time.Minute, middleware.RateLimitOptions{
			FailureMode: middleware.RateLimitFailClose,
		}), h.Auth.WeChatScanPoll)
	}

	// 公开设置（无需认证）
	settings := v1.Group("/settings")
	{
		settings.GET("/public", h.Setting.GetPublicSettings)
		settings.GET("/gallery", h.Setting.GetHomeGallery)
	}

	// 充值配置（无需认证）
	recharge := v1.Group("/recharge")
	{
		recharge.GET("/config", h.Recharge.GetConfig)
	}

	// 需要认证的当前用户信息
	authenticated := v1.Group("")
	authenticated.Use(gin.HandlerFunc(jwtAuth))
	{
		authenticated.GET("/auth/me", h.Auth.GetCurrentUser)
		// 微信账号绑定（需要登录）
		authenticated.GET("/auth/oauth/wechat/bind", rateLimiter.LimitWithOptions("wechat-bind", 20, 20*time.Minute, middleware.RateLimitOptions{
			FailureMode: middleware.RateLimitFailClose,
		}), h.Auth.WeChatBind)
		// 微信账号解绑（需要登录）
		authenticated.DELETE("/auth/oauth/wechat/bind", rateLimiter.LimitWithOptions("wechat-unbind", 10, time.Minute, middleware.RateLimitOptions{
			FailureMode: middleware.RateLimitFailClose,
		}), h.Auth.WeChatUnbind)
		// 微信扫码绑定轮询（需要登录，订阅号模式）
		authenticated.GET("/auth/wechat/scan/bind/poll", rateLimiter.LimitWithOptions("wechat-scan-bind-poll", 120, time.Minute, middleware.RateLimitOptions{
			FailureMode: middleware.RateLimitFailClose,
		}), h.Auth.WeChatScanBindPoll)
		// 发送绑定邮箱验证码（需要登录，用于微信登录用户绑定真实邮箱）
		authenticated.POST("/auth/send-bind-email-code", rateLimiter.LimitWithOptions("send-bind-email-code", 5, time.Minute, middleware.RateLimitOptions{
			FailureMode: middleware.RateLimitFailClose,
		}), h.Auth.SendBindEmailCode)
		// 邮箱绑定（需要登录，用于微信登录用户绑定真实邮箱）
		authenticated.POST("/auth/bind-email", rateLimiter.LimitWithOptions("bind-email", 10, time.Minute, middleware.RateLimitOptions{
			FailureMode: middleware.RateLimitFailClose,
		}), h.Auth.BindEmail)
		// 撤销所有会话（需要认证）
		authenticated.POST("/auth/revoke-all-sessions", h.Auth.RevokeAllSessions)
	}
}
