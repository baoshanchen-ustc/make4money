package routes

import (
	"reflect"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/middleware"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func resolveOptionalGinHandler(receiver any, methodNames ...string) gin.HandlerFunc {
	value := reflect.ValueOf(receiver)
	if !value.IsValid() {
		return nil
	}

	ginContextType := reflect.TypeOf(&gin.Context{})
	for _, methodName := range methodNames {
		method := value.MethodByName(methodName)
		if !method.IsValid() {
			continue
		}
		if method.Type().NumIn() != 1 || method.Type().In(0) != ginContextType || method.Type().NumOut() != 0 {
			continue
		}

		return func(c *gin.Context) {
			method.Call([]reflect.Value{reflect.ValueOf(c)})
		}
	}

	return nil
}

func registerOptionalPostRoute(group *gin.RouterGroup, path string, middlewares []gin.HandlerFunc, receiver any, methodNames ...string) {
	handler := resolveOptionalGinHandler(receiver, methodNames...)
	if handler == nil {
		return
	}

	handlers := append(append([]gin.HandlerFunc{}, middlewares...), handler)
	group.POST(path, handlers...)
}

func registerOptionalGetRoute(group *gin.RouterGroup, path string, middlewares []gin.HandlerFunc, receiver any, methodNames ...string) {
	handler := resolveOptionalGinHandler(receiver, methodNames...)
	if handler == nil {
		return
	}

	handlers := append(append([]gin.HandlerFunc{}, middlewares...), handler)
	group.GET(path, handlers...)
}

// RegisterAuthRoutes 注册认证相关路由
func RegisterAuthRoutes(
	v1 *gin.RouterGroup,
	h *handler.Handlers,
	jwtAuth servermiddleware.JWTAuthMiddleware,
	redisClient *redis.Client,
	settingService *service.SettingService,
) {
	// 创建速率限制器
	rateLimiter := middleware.NewRateLimiter(redisClient)

	// 公开接口
	auth := v1.Group("/auth")
	auth.Use(servermiddleware.BackendModeAuthGuard(settingService))
	{
		// 注册/登录/2FA/验证码发送均属于高风险入口，增加服务端兜底限流（Redis 故障时 fail-close）
		auth.POST("/register", rateLimiter.LimitWithOptions("auth-register", 5, time.Minute, middleware.RateLimitOptions{
			FailureMode: middleware.RateLimitFailClose,
		}), h.Auth.Register)
		auth.POST("/login", rateLimiter.LimitWithOptions("auth-login", 20, time.Minute, middleware.RateLimitOptions{
			FailureMode: middleware.RateLimitFailClose,
		}), h.Auth.Login)
		auth.POST("/login/2fa", rateLimiter.LimitWithOptions("auth-login-2fa", 20, time.Minute, middleware.RateLimitOptions{
			FailureMode: middleware.RateLimitFailClose,
		}), h.Auth.Login2FA)
		auth.POST("/send-verify-code", rateLimiter.LimitWithOptions("auth-send-verify-code", 5, time.Minute, middleware.RateLimitOptions{
			FailureMode: middleware.RateLimitFailClose,
		}), h.Auth.SendVerifyCode)
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
		auth.POST("/oauth/confirm-bind",
			rateLimiter.LimitWithOptions("oauth-confirm-bind", 20, time.Minute, middleware.RateLimitOptions{
				FailureMode: middleware.RateLimitFailClose,
			}),
			h.Auth.ConfirmPendingAuthBind,
		)
		auth.POST("/oauth/confirm-bind/2fa",
			rateLimiter.LimitWithOptions("oauth-confirm-bind-2fa", 20, time.Minute, middleware.RateLimitOptions{
				FailureMode: middleware.RateLimitFailClose,
			}),
			h.Auth.ConfirmPendingAuthBind2FA,
		)
		auth.GET("/oauth/linuxdo/start", h.Auth.LinuxDoOAuthStart)
		auth.GET("/oauth/linuxdo/callback", h.Auth.LinuxDoOAuthCallback)
		registerOptionalPostRoute(auth, "/oauth/linuxdo/bind-existing", []gin.HandlerFunc{
			rateLimiter.LimitWithOptions("oauth-linuxdo-bind-existing", 20, time.Minute, middleware.RateLimitOptions{
				FailureMode: middleware.RateLimitFailClose,
			}),
		}, h.Auth, "BindLinuxDoOAuthExisting", "CompleteLinuxDoOAuthBind", "BindLinuxDoOAuthLogin")
		auth.POST("/oauth/linuxdo/complete-registration",
			rateLimiter.LimitWithOptions("oauth-linuxdo-complete", 10, time.Minute, middleware.RateLimitOptions{
				FailureMode: middleware.RateLimitFailClose,
			}),
			h.Auth.CompleteLinuxDoOAuthRegistration,
		)
		registerOptionalPostRoute(auth, "/oauth/linuxdo/bind-login", []gin.HandlerFunc{
			rateLimiter.LimitWithOptions("oauth-linuxdo-bind-login", 20, time.Minute, middleware.RateLimitOptions{
				FailureMode: middleware.RateLimitFailClose,
			}),
		}, h.Auth, "BindLinuxDoOAuthLogin", "CompleteLinuxDoOAuthBind", "BindLinuxDoOAuthExisting")
		registerOptionalPostRoute(auth, "/oauth/linuxdo/create-account", []gin.HandlerFunc{
			rateLimiter.LimitWithOptions("oauth-linuxdo-create-account", 10, time.Minute, middleware.RateLimitOptions{
				FailureMode: middleware.RateLimitFailClose,
			}),
		}, h.Auth, "CreateLinuxDoOAuthAccount", "CreateLinuxDoOAuthRegistration", "CompleteLinuxDoOAuthRegistration")
		auth.GET("/oauth/oidc/start", h.Auth.OIDCOAuthStart)
		auth.GET("/oauth/oidc/callback", h.Auth.OIDCOAuthCallback)
		registerOptionalPostRoute(auth, "/oauth/oidc/bind-existing", []gin.HandlerFunc{
			rateLimiter.LimitWithOptions("oauth-oidc-bind-existing", 20, time.Minute, middleware.RateLimitOptions{
				FailureMode: middleware.RateLimitFailClose,
			}),
		}, h.Auth, "BindOIDCOAuthExisting", "CompleteOIDCOAuthBind", "BindOIDCOAuthLogin")
		auth.POST("/oauth/oidc/complete-registration",
			rateLimiter.LimitWithOptions("oauth-oidc-complete", 10, time.Minute, middleware.RateLimitOptions{
				FailureMode: middleware.RateLimitFailClose,
			}),
			h.Auth.CompleteOIDCOAuthRegistration,
		)
		registerOptionalPostRoute(auth, "/oauth/oidc/bind-login", []gin.HandlerFunc{
			rateLimiter.LimitWithOptions("oauth-oidc-bind-login", 20, time.Minute, middleware.RateLimitOptions{
				FailureMode: middleware.RateLimitFailClose,
			}),
		}, h.Auth, "BindOIDCOAuthLogin", "CompleteOIDCOAuthBind", "BindOIDCOAuthExisting")
		registerOptionalPostRoute(auth, "/oauth/oidc/create-account", []gin.HandlerFunc{
			rateLimiter.LimitWithOptions("oauth-oidc-create-account", 10, time.Minute, middleware.RateLimitOptions{
				FailureMode: middleware.RateLimitFailClose,
			}),
		}, h.Auth, "CreateOIDCOAuthAccount", "CreateOIDCOAuthRegistration", "CompleteOIDCOAuthRegistration")
		registerOptionalGetRoute(auth, "/oauth/wechat/start", nil, h.Auth, "WeChatOAuthStart", "WechatOAuthStart")
		auth.GET("/oauth/wechat/callback", h.Auth.WeChatOAuthCallback)
		registerOptionalGetRoute(auth, "/oauth/wechat/payment/start", nil, h.Auth, "WeChatPaymentOAuthStart", "WechatPaymentOAuthStart")
		registerOptionalGetRoute(auth, "/oauth/wechat/payment/callback", nil, h.Auth, "WeChatPaymentOAuthCallback", "WechatPaymentOAuthCallback")
		registerOptionalPostRoute(auth, "/oauth/wechat/bind-existing", []gin.HandlerFunc{
			rateLimiter.LimitWithOptions("oauth-wechat-bind-existing", 20, time.Minute, middleware.RateLimitOptions{
				FailureMode: middleware.RateLimitFailClose,
			}),
		}, h.Auth, "BindWeChatOAuthExisting", "CompleteWeChatOAuthBind", "BindWeChatOAuthLogin", "BindWechatOAuthExisting", "CompleteWechatOAuthBind", "BindWechatOAuthLogin")
		registerOptionalPostRoute(auth, "/oauth/wechat/bind-login", []gin.HandlerFunc{
			rateLimiter.LimitWithOptions("oauth-wechat-bind-login", 20, time.Minute, middleware.RateLimitOptions{
				FailureMode: middleware.RateLimitFailClose,
			}),
		}, h.Auth, "BindWeChatOAuthLogin", "CompleteWeChatOAuthBind", "BindWeChatOAuthExisting", "BindWechatOAuthLogin", "CompleteWechatOAuthBind", "BindWechatOAuthExisting")
		registerOptionalPostRoute(auth, "/oauth/wechat/create-account", []gin.HandlerFunc{
			rateLimiter.LimitWithOptions("oauth-wechat-create-account", 10, time.Minute, middleware.RateLimitOptions{
				FailureMode: middleware.RateLimitFailClose,
			}),
		}, h.Auth, "CreateWeChatOAuthAccount", "CreateWeChatOAuthRegistration", "CreateWechatOAuthAccount", "CreateWechatOAuthRegistration")
		registerOptionalPostRoute(auth, "/oauth/wechat/complete-registration", []gin.HandlerFunc{
			rateLimiter.LimitWithOptions("oauth-wechat-complete", 10, time.Minute, middleware.RateLimitOptions{
				FailureMode: middleware.RateLimitFailClose,
			}),
		}, h.Auth, "CompleteWeChatOAuthRegistration", "CompleteWechatOAuthRegistration")
	}

	// 公开设置（无需认证）
	settings := v1.Group("/settings")
	{
		settings.GET("/public", h.Setting.GetPublicSettings)
	}

	// 需要认证的当前用户信息
	authenticated := v1.Group("")
	authenticated.Use(gin.HandlerFunc(jwtAuth))
	authenticated.Use(servermiddleware.BackendModeUserGuard(settingService))
	{
		authenticated.GET("/auth/me", h.Auth.GetCurrentUser)
		// 撤销所有会话（需要认证）
		authenticated.POST("/auth/revoke-all-sessions", h.Auth.RevokeAllSessions)
	}
}
