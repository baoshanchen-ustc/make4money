package handler

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type bindOAuthLoginRequest struct {
	PendingAuthToken string `json:"pending_auth_token" binding:"required"`
	Email            string `json:"email" binding:"required,email"`
	Password         string `json:"password" binding:"required"`
	TurnstileToken   string `json:"turnstile_token,omitempty"`
	AdoptDisplayName *bool  `json:"adopt_display_name,omitempty"`
	AdoptAvatar      *bool  `json:"adopt_avatar,omitempty"`
}

type createOAuthAccountRequest struct {
	PendingAuthToken string `json:"pending_auth_token" binding:"required"`
	Email            string `json:"email,omitempty"`
	Password         string `json:"password,omitempty"`
	VerifyCode       string `json:"verify_code,omitempty"`
	InvitationCode   string `json:"invitation_code,omitempty"`
	AdoptDisplayName *bool  `json:"adopt_display_name,omitempty"`
	AdoptAvatar      *bool  `json:"adopt_avatar,omitempty"`
}

func (h *AuthHandler) BindLinuxDoOAuthLogin(c *gin.Context) { h.bindPendingOAuthLogin(c, "linuxdo") }
func (h *AuthHandler) BindOIDCOAuthLogin(c *gin.Context)    { h.bindPendingOAuthLogin(c, "oidc") }
func (h *AuthHandler) BindWeChatOAuthLogin(c *gin.Context)  { h.bindPendingOAuthLogin(c, "wechat") }

func (h *AuthHandler) CreateLinuxDoOAuthAccount(c *gin.Context) {
	h.createPendingOAuthAccount(c, "linuxdo")
}
func (h *AuthHandler) CreateOIDCOAuthAccount(c *gin.Context) { h.createPendingOAuthAccount(c, "oidc") }
func (h *AuthHandler) CreateWeChatOAuthAccount(c *gin.Context) {
	h.createPendingOAuthAccount(c, "wechat")
}

func (h *AuthHandler) bindPendingOAuthLogin(c *gin.Context, provider string) {
	var req bindOAuthLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	session, err := h.authService.GetPendingAuthSessionForProgress(c.Request.Context(), req.PendingAuthToken, nil)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if !strings.EqualFold(session.ProviderType, provider) {
		response.BadRequest(c, "Pending auth session provider mismatch")
		return
	}
	if err := h.applyPendingOAuthAdoptionDecision(c, req.PendingAuthToken, req.AdoptDisplayName, req.AdoptAvatar); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if err := h.authService.VerifyTurnstile(c.Request.Context(), req.TurnstileToken, ip.GetClientIP(c)); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	_, user, err := h.authService.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if h.settingSvc != nil && h.settingSvc.IsBackendModeEnabled(c.Request.Context()) && !user.IsAdmin() {
		response.Forbidden(c, "Backend mode is active. Only admin login is allowed.")
		return
	}
	if _, err := h.authService.MarkPendingAuthSessionPasswordVerified(c.Request.Context(), req.PendingAuthToken, user.ID); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	if h.totpService != nil && h.settingSvc != nil && h.settingSvc.IsTotpEnabled(c.Request.Context()) && user.TotpEnabled {
		tempToken, err := h.totpService.CreateLoginSessionForPendingAuth(c.Request.Context(), user.ID, user.Email, req.PendingAuthToken)
		if err != nil {
			response.InternalError(c, "Failed to create 2FA session")
			return
		}
		response.Success(c, TotpLoginResponse{
			Requires2FA:     true,
			TempToken:       tempToken,
			UserEmailMasked: service.MaskEmail(user.Email),
		})
		return
	}

	if _, err := h.authService.CompletePendingAuthSessionBind(c.Request.Context(), req.PendingAuthToken, user.ID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	h.respondWithTokenPair(c, user)
}

func (h *AuthHandler) createPendingOAuthAccount(c *gin.Context, provider string) {
	var req createOAuthAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	session, err := h.authService.GetPendingAuthSessionForProgress(c.Request.Context(), req.PendingAuthToken, nil)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if !strings.EqualFold(session.ProviderType, provider) {
		response.BadRequest(c, "Pending auth session provider mismatch")
		return
	}

	if err := h.applyPendingOAuthAdoptionDecision(c, req.PendingAuthToken, req.AdoptDisplayName, req.AdoptAvatar); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	forceEmailBinding := true
	emailVerifyEnabled := false
	if h.settingSvc != nil {
		forceEmailBinding = h.settingSvc.IsForceEmailOnThirdPartySignupEnabled(c.Request.Context())
		emailVerifyEnabled = h.settingSvc.IsEmailVerifyEnabled(c.Request.Context())
	}
	if !forceEmailBinding &&
		strings.TrimSpace(req.Email) == "" &&
		strings.TrimSpace(req.VerifyCode) == "" {
		email, username := resolvePendingOAuthSyntheticIdentity(session)
		if email == "" || username == "" {
			response.BadRequest(c, "Pending auth session identity is incomplete")
			return
		}

		tokenPair, _, err := h.authService.LoginOrRegisterSyntheticOAuthAndBindPendingSession(
			c.Request.Context(),
			req.PendingAuthToken,
			email,
			username,
			req.InvitationCode,
		)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}

		response.Success(c, gin.H{
			"access_token":  tokenPair.AccessToken,
			"refresh_token": tokenPair.RefreshToken,
			"expires_in":    tokenPair.ExpiresIn,
			"token_type":    "Bearer",
		})
		return
	}

	if strings.TrimSpace(req.Email) == "" || strings.TrimSpace(req.Password) == "" {
		response.BadRequest(c, "Email and password are required")
		return
	}
	if emailVerifyEnabled && strings.TrimSpace(req.VerifyCode) == "" {
		response.BadRequest(c, "Email, password, and verification code are required")
		return
	}

	_, user, err := h.authService.RegisterWithVerification(
		c.Request.Context(),
		req.Email,
		req.Password,
		req.VerifyCode,
		"",
		req.InvitationCode,
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	if _, err := h.authService.CompletePendingAuthSessionBind(c.Request.Context(), req.PendingAuthToken, user.ID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	h.respondWithTokenPair(c, user)
}

func (h *AuthHandler) applyPendingOAuthAdoptionDecision(
	c *gin.Context,
	pendingAuthToken string,
	adoptDisplayName *bool,
	adoptAvatar *bool,
) error {
	if adoptDisplayName == nil && adoptAvatar == nil {
		return nil
	}

	_, err := h.authService.UpdatePendingAuthSessionAdoptionDecision(
		c.Request.Context(),
		pendingAuthToken,
		adoptDisplayName != nil && *adoptDisplayName,
		adoptAvatar != nil && *adoptAvatar,
	)
	return err
}

func resolvePendingOAuthSyntheticIdentity(session *service.PendingAuthSessionRecord) (string, string) {
	if session == nil {
		return "", ""
	}

	providerType := strings.ToLower(strings.TrimSpace(session.ProviderType))
	providerKey := strings.TrimSpace(session.ProviderKey)
	providerSubject := strings.TrimSpace(session.ProviderSubject)
	displayName := oauthMetadataString(session.Metadata, "suggested_display_name")
	username := firstNonEmpty(
		oauthMetadataString(session.Metadata, "compat_username"),
		displayName,
	)
	email := oauthMetadataString(session.Metadata, "compat_email")

	switch providerType {
	case "linuxdo":
		if email == "" {
			email = linuxDoSyntheticEmail(providerSubject)
		}
		if username == "" {
			username = "linuxdo_" + providerSubject
		}
	case "oidc":
		if email == "" {
			email = oidcSyntheticEmailFromIdentityKey(oidcIdentityKey(providerKey, providerSubject))
		}
		if username == "" {
			username = oidcFallbackUsername(providerSubject)
		}
	case "wechat":
		if email == "" {
			email = wechatSyntheticEmail(providerSubject)
		}
		if username == "" {
			username = wechatFallbackUsername(providerSubject)
		}
	}

	return strings.TrimSpace(email), strings.TrimSpace(username)
}
