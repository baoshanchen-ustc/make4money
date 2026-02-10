package handler

import (
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"

	"github.com/gin-gonic/gin"
)

// 合成邮箱域名常量（用于识别微信/LinuxDo等OAuth用户）
const (
	LinuxDoSyntheticEmailDomain = "@linuxdo.invalid"
)

// BindEmailRequest 邮箱绑定请求
type BindEmailRequest struct {
	Email      string `json:"email" binding:"required,email"`
	VerifyCode string `json:"verify_code" binding:"required"`
	Password   string `json:"password"` // 绑定时同时设置密码（可选）
}

// BindEmailResponse 邮箱绑定响应
type BindEmailResponse struct {
	Email   string `json:"email"`
	Message string `json:"message"`
}

// SendBindEmailCodeRequest 发送绑定邮箱验证码请求
type SendBindEmailCodeRequest struct {
	Email          string `json:"email" binding:"required,email"`
	TurnstileToken string `json:"turnstile_token"`
}

// SendBindEmailCode 发送绑定邮箱验证码（已登录用户）
// POST /api/v1/auth/send-bind-email-code
func (h *AuthHandler) SendBindEmailCode(c *gin.Context) {
	var req SendBindEmailCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数无效: "+err.Error())
		return
	}

	// 获取当前登录用户
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "用户未登录")
		return
	}

	email := strings.TrimSpace(strings.ToLower(req.Email))

	// 验证邮箱不是合成邮箱格式
	if isSyntheticEmail(email) {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_EMAIL", "不允许使用该邮箱格式"))
		return
	}

	// 获取当前用户信息
	currentUser, err := h.userService.GetByID(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// 检查用户当前邮箱是否为合成邮箱（只有合成邮箱用户才需要绑定真实邮箱）
	if !isSyntheticEmail(currentUser.Email) {
		response.ErrorFrom(c, infraerrors.BadRequest("ALREADY_HAS_EMAIL", "您已绑定真实邮箱"))
		return
	}

	// 检查邮箱是否已被其他用户使用
	existingUser, err := h.userService.GetByEmail(c.Request.Context(), email)
	if err == nil && existingUser != nil && existingUser.ID != subject.UserID {
		response.ErrorFrom(c, infraerrors.Conflict("EMAIL_ALREADY_USED", "该邮箱已被其他用户使用"))
		return
	}

	// Turnstile 验证
	if err := h.authService.VerifyTurnstile(c.Request.Context(), req.TurnstileToken, ip.GetClientIP(c)); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// 发送验证码
	if err := h.authService.SendBindEmailCode(c.Request.Context(), email); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, SendVerifyCodeResponse{
		Message:   "验证码已发送",
		Countdown: 60,
	})
}

// BindEmail 绑定邮箱（已登录用户）
// POST /api/v1/auth/bind-email
func (h *AuthHandler) BindEmail(c *gin.Context) {
	var req BindEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数无效: "+err.Error())
		return
	}

	// 获取当前登录用户
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "用户未登录")
		return
	}

	email := strings.TrimSpace(strings.ToLower(req.Email))
	verifyCode := strings.TrimSpace(req.VerifyCode)
	password := req.Password

	// 验证邮箱不是合成邮箱格式
	if isSyntheticEmail(email) {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_EMAIL", "不允许使用该邮箱格式"))
		return
	}

	// 验证密码长度（如果提供了密码）
	if password != "" && len(password) < 6 {
		response.ErrorFrom(c, infraerrors.BadRequest("PASSWORD_TOO_SHORT", "密码至少需要6个字符"))
		return
	}

	// 获取当前用户信息
	currentUser, err := h.userService.GetByID(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// 检查用户当前邮箱是否为合成邮箱（只有合成邮箱用户才需要绑定真实邮箱）
	if !isSyntheticEmail(currentUser.Email) {
		response.ErrorFrom(c, infraerrors.BadRequest("ALREADY_HAS_EMAIL", "您已绑定真实邮箱"))
		return
	}

	// 检查邮箱是否已被其他用户使用
	existingUser, err := h.userService.GetByEmail(c.Request.Context(), email)
	if err == nil && existingUser != nil && existingUser.ID != subject.UserID {
		response.ErrorFrom(c, infraerrors.Conflict("EMAIL_ALREADY_USED", "该邮箱已被其他用户使用"))
		return
	}

	// 验证验证码
	if err := h.authService.VerifyEmailCode(c.Request.Context(), email, verifyCode); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// 如果用户原邮箱是微信合成邮箱，先保存 WeChat OpenID
	// 防止绑定邮箱后微信再次登录时无法识别用户
	if isWeChatSyntheticEmail(currentUser.Email) {
		wechatOpenID := extractWeChatOpenID(currentUser.Email)
		if wechatOpenID != "" {
			if err := h.userService.BindWeChatOpenID(c.Request.Context(), subject.UserID, wechatOpenID); err != nil {
				// 只记录日志，不阻断邮箱绑定流程
				// 因为 wechat_openid 可能已经存在（用户之前绑定过微信）
				_ = err
			}
		}
	}

	// 绑定邮箱并设置密码（如果提供了密码）
	if err := h.userService.BindEmailWithPassword(c.Request.Context(), subject.UserID, email, password); err != nil {
		response.ErrorFrom(c, infraerrors.InternalServer("UPDATE_FAILED", "更新邮箱失败"))
		return
	}

	response.Success(c, BindEmailResponse{
		Email:   email,
		Message: "邮箱绑定成功",
	})
}

// isSyntheticEmail 判断是否为合成邮箱
func isSyntheticEmail(email string) bool {
	email = strings.ToLower(email)
	return strings.HasSuffix(email, WeChatSyntheticEmailDomain) ||
		strings.HasSuffix(email, LinuxDoSyntheticEmailDomain)
}

// isWeChatSyntheticEmail 判断是否为微信合成邮箱
func isWeChatSyntheticEmail(email string) bool {
	return strings.HasSuffix(strings.ToLower(email), WeChatSyntheticEmailDomain)
}

// extractWeChatOpenID 从微信合成邮箱中提取 OpenID
// 例如：wechat-o_xxx@wechat-auth.invalid -> o_xxx
// 注意：只对域名后缀做大小写不敏感检查，保留 OpenID 原始大小写
func extractWeChatOpenID(email string) string {
	lowerEmail := strings.ToLower(email)
	lowerDomain := strings.ToLower(WeChatSyntheticEmailDomain)
	if !strings.HasSuffix(lowerEmail, lowerDomain) {
		return ""
	}
	// 用原始 email 截取，保留 OpenID 大小写
	localPart := email[:len(email)-len(WeChatSyntheticEmailDomain)]
	// 去掉前缀 wechat-（大小写不敏感匹配前缀，保留 OpenID 部分原始大小写）
	if strings.HasPrefix(strings.ToLower(localPart), "wechat-") {
		return localPart[len("wechat-"):]
	}
	return ""
}
