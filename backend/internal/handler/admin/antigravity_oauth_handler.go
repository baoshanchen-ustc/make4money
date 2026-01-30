package admin

import (
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type AntigravityOAuthHandler struct {
	antigravityOAuthService *service.AntigravityOAuthService
}

func NewAntigravityOAuthHandler(antigravityOAuthService *service.AntigravityOAuthService) *AntigravityOAuthHandler {
	return &AntigravityOAuthHandler{antigravityOAuthService: antigravityOAuthService}
}

type AntigravityGenerateAuthURLRequest struct {
	ProxyID *int64 `json:"proxy_id"`
}

// GenerateAuthURL generates Google OAuth authorization URL
// POST /api/v1/admin/antigravity/oauth/auth-url
func (h *AntigravityOAuthHandler) GenerateAuthURL(c *gin.Context) {
	var req AntigravityGenerateAuthURLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求无效: "+err.Error())
		return
	}

	redirectURI := deriveRedirectURI(c)
	result, err := h.antigravityOAuthService.GenerateAuthURL(c.Request.Context(), req.ProxyID, redirectURI)
	if err != nil {
		response.InternalError(c, "生成授权链接失败: "+err.Error())
		return
	}

	response.Success(c, result)
}

type AntigravityExchangeCodeRequest struct {
	SessionID string `json:"session_id" binding:"required"`
	State     string `json:"state" binding:"required"`
	Code      string `json:"code" binding:"required"`
	ProxyID   *int64 `json:"proxy_id"`
}

// ExchangeCode 用 authorization code 交换 token
// POST /api/v1/admin/antigravity/oauth/exchange-code
func (h *AntigravityOAuthHandler) ExchangeCode(c *gin.Context) {
	var req AntigravityExchangeCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求无效: "+err.Error())
		return
	}

	tokenInfo, err := h.antigravityOAuthService.ExchangeCode(c.Request.Context(), &service.AntigravityExchangeCodeInput{
		SessionID: req.SessionID,
		State:     req.State,
		Code:      req.Code,
		ProxyID:   req.ProxyID,
	})
	if err != nil {
		response.BadRequest(c, "Token 交换失败: "+err.Error())
		return
	}

	response.Success(c, tokenInfo)
}

func deriveRedirectURI(c *gin.Context) string {
	origin := strings.TrimSpace(c.GetHeader("Origin"))
	if origin != "" {
		return strings.TrimRight(origin, "/") + "/auth/callback"
	}

	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	if xfProto := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto")); xfProto != "" {
		scheme = strings.TrimSpace(strings.Split(xfProto, ",")[0])
	}

	host := strings.TrimSpace(c.Request.Host)
	if xfHost := strings.TrimSpace(c.GetHeader("X-Forwarded-Host")); xfHost != "" {
		host = strings.TrimSpace(strings.Split(xfHost, ",")[0])
	}

	return fmt.Sprintf("%s://%s/auth/callback", scheme, host)
}
