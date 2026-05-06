// Package middleware provides HTTP middleware for authentication, authorization, and request processing.
package middleware

import (
	"crypto/subtle"
	"errors"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// NewAdminAuthMiddleware 创建管理员认证中间件
func NewAdminAuthMiddleware(
	authService *service.AuthService,
	userService *service.UserService,
	settingService *service.SettingService,
) AdminAuthMiddleware {
	return AdminAuthMiddleware(adminAuth(authService, userService, settingService))
}

// NewScopedAdminAuthMiddleware 创建支持 scoped admin 的管理员认证中间件
func NewScopedAdminAuthMiddleware(
	authService *service.AuthService,
	userService *service.UserService,
	settingService *service.SettingService,
) ScopedAdminAuthMiddleware {
	return ScopedAdminAuthMiddleware(scopedAdminAuth(authService, userService, settingService))
}

// adminAuth 管理员认证中间件实现
// 支持两种认证方式（通过不同的 header 区分）：
// 1. Admin API Key: x-api-key: <admin-api-key>
// 2. JWT Token: Authorization: Bearer <jwt-token> (需要管理员角色)
func adminAuth(
	authService *service.AuthService,
	userService *service.UserService,
	settingService *service.SettingService,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, authMethod, ok := authenticateAdminRequest(c, authService, userService, settingService)
		if !ok {
			return
		}
		if !user.IsAdmin() {
			AbortWithError(c, 403, "FORBIDDEN", "Admin access required")
			return
		}
		setAuthenticatedAdminContext(c, user, authMethod)
		c.Next()
	}
}

func scopedAdminAuth(
	authService *service.AuthService,
	userService *service.UserService,
	settingService *service.SettingService,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, authMethod, ok := authenticateAdminRequest(c, authService, userService, settingService)
		if !ok {
			return
		}
		if !user.IsScopedAdmin() {
			AbortWithError(c, 403, "FORBIDDEN", "Admin access required")
			return
		}
		setAuthenticatedAdminContext(c, user, authMethod)
		c.Next()
	}
}

func authenticateAdminRequest(
	c *gin.Context,
	authService *service.AuthService,
	userService *service.UserService,
	settingService *service.SettingService,
) (*service.User, string, bool) {
	// WebSocket upgrade requests cannot set Authorization headers in browsers.
	// For admin WebSocket endpoints (e.g. Ops realtime), allow passing the JWT via
	// Sec-WebSocket-Protocol (subprotocol list) using a prefixed token item:
	//   Sec-WebSocket-Protocol: sub2api-admin, jwt.<token>
	if isWebSocketUpgradeRequest(c) {
		if token := extractJWTFromWebSocketSubprotocol(c); token != "" {
			user, ok := authenticateAdminJWT(c, token, authService, userService)
			return user, "jwt", ok
		}
	}

	// 检查 x-api-key header（Admin API Key 认证）
	apiKey := c.GetHeader("x-api-key")
	if apiKey != "" {
		user, ok := authenticateAdminAPIKey(c, apiKey, settingService, userService)
		return user, "admin_api_key", ok
	}

	// 检查 Authorization header（JWT 认证）
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			token := strings.TrimSpace(parts[1])
			if token == "" {
				AbortWithError(c, 401, "UNAUTHORIZED", "Authorization required")
				return nil, "", false
			}
			user, ok := authenticateAdminJWT(c, token, authService, userService)
			return user, "jwt", ok
		}
	}

	// 无有效认证信息
	AbortWithError(c, 401, "UNAUTHORIZED", "Authorization required")
	return nil, "", false
}

func isWebSocketUpgradeRequest(c *gin.Context) bool {
	if c == nil || c.Request == nil {
		return false
	}
	// RFC6455 handshake uses:
	//   Connection: Upgrade
	//   Upgrade: websocket
	upgrade := strings.ToLower(strings.TrimSpace(c.GetHeader("Upgrade")))
	if upgrade != "websocket" {
		return false
	}
	connection := strings.ToLower(c.GetHeader("Connection"))
	return strings.Contains(connection, "upgrade")
}

func extractJWTFromWebSocketSubprotocol(c *gin.Context) string {
	if c == nil {
		return ""
	}
	raw := strings.TrimSpace(c.GetHeader("Sec-WebSocket-Protocol"))
	if raw == "" {
		return ""
	}

	// The header is a comma-separated list of tokens. We reserve the prefix "jwt."
	// for carrying the admin JWT.
	for _, part := range strings.Split(raw, ",") {
		p := strings.TrimSpace(part)
		if strings.HasPrefix(p, "jwt.") {
			token := strings.TrimSpace(strings.TrimPrefix(p, "jwt."))
			if token != "" {
				return token
			}
		}
	}
	return ""
}

func setAuthenticatedAdminContext(c *gin.Context, user *service.User, authMethod string) {
	c.Set(string(ContextKeyUser), AuthSubject{
		UserID:      user.ID,
		Concurrency: user.Concurrency,
	})
	c.Set(string(ContextKeyUserRole), user.Role)
	c.Set("auth_method", authMethod)
}

// authenticateAdminAPIKey 验证管理员 API Key
func authenticateAdminAPIKey(
	c *gin.Context,
	key string,
	settingService *service.SettingService,
	userService *service.UserService,
) (*service.User, bool) {
	storedKey, err := settingService.GetAdminAPIKey(c.Request.Context())
	if err != nil {
		AbortWithError(c, 500, "INTERNAL_ERROR", "Internal server error")
		return nil, false
	}

	// 未配置或不匹配，统一返回相同错误（避免信息泄露）
	if storedKey == "" || subtle.ConstantTimeCompare([]byte(key), []byte(storedKey)) != 1 {
		AbortWithError(c, 401, "INVALID_ADMIN_KEY", "Invalid admin API key")
		return nil, false
	}

	// 获取真实的管理员用户
	admin, err := userService.GetFirstAdmin(c.Request.Context())
	if err != nil {
		AbortWithError(c, 500, "INTERNAL_ERROR", "No admin user found")
		return nil, false
	}

	return admin, true
}

// authenticateAdminJWT 验证 JWT 并返回用户
func authenticateAdminJWT(
	c *gin.Context,
	token string,
	authService *service.AuthService,
	userService *service.UserService,
) (*service.User, bool) {
	// 验证 JWT token
	claims, err := authService.ValidateToken(token)
	if err != nil {
		if errors.Is(err, service.ErrTokenExpired) {
			AbortWithError(c, 401, "TOKEN_EXPIRED", "Token has expired")
			return nil, false
		}
		AbortWithError(c, 401, "INVALID_TOKEN", "Invalid token")
		return nil, false
	}

	// 从数据库获取用户
	user, err := userService.GetByID(c.Request.Context(), claims.UserID)
	if err != nil {
		AbortWithError(c, 401, "USER_NOT_FOUND", "User not found")
		return nil, false
	}

	// 检查用户状态
	if !user.IsActive() {
		AbortWithError(c, 401, "USER_INACTIVE", "User account is not active")
		return nil, false
	}

	// 校验 TokenVersion，确保管理员改密后旧 token 失效
	if claims.TokenVersion != user.TokenVersion {
		AbortWithError(c, 401, "TOKEN_REVOKED", "Token has been revoked (password changed)")
		return nil, false
	}

	return user, true
}
