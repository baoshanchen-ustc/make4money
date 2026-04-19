package handler

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
	"unsafe"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/oauth"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/imroc/req/v3"
	"github.com/tidwall/gjson"
)

const (
	linuxDoOAuthCookiePath         = "/api/v1/auth/oauth/linuxdo"
	oauthBindAccessTokenCookiePath = "/api/v1/auth/oauth"
	linuxDoOAuthStateCookieName    = "linuxdo_oauth_state"
	linuxDoOAuthVerifierCookie     = "linuxdo_oauth_verifier"
	linuxDoOAuthRedirectCookie     = "linuxdo_oauth_redirect"
	linuxDoOAuthIntentCookieName   = "linuxdo_oauth_intent"
	oauthBindAccessTokenCookieName = "oauth_bind_access_token"
	linuxDoOAuthCookieMaxAgeSec    = 10 * 60 // 10 minutes
	linuxDoOAuthDefaultRedirectTo  = "/dashboard"
	linuxDoOAuthDefaultFrontendCB  = "/auth/linuxdo/callback"
	linuxDoOAuthProviderKey        = "linuxdo"

	linuxDoOAuthMaxRedirectLen      = 2048
	linuxDoOAuthMaxFragmentValueLen = 512
	linuxDoOAuthMaxSubjectLen       = 64 - len("linuxdo-")
)

type linuxDoTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

type linuxDoTokenExchangeError struct {
	StatusCode          int
	ProviderError       string
	ProviderDescription string
	Body                string
}

type linuxDoUserInfoClaims struct {
	Email       string
	Username    string
	Subject     string
	DisplayName string
	AvatarURL   string
}

func (e *linuxDoTokenExchangeError) Error() string {
	if e == nil {
		return ""
	}
	parts := []string{fmt.Sprintf("token exchange status=%d", e.StatusCode)}
	if strings.TrimSpace(e.ProviderError) != "" {
		parts = append(parts, "error="+strings.TrimSpace(e.ProviderError))
	}
	if strings.TrimSpace(e.ProviderDescription) != "" {
		parts = append(parts, "error_description="+strings.TrimSpace(e.ProviderDescription))
	}
	return strings.Join(parts, " ")
}

// LinuxDoOAuthStart 启动 LinuxDo Connect OAuth 登录流程。
// GET /api/v1/auth/oauth/linuxdo/start?redirect=/dashboard
func (h *AuthHandler) LinuxDoOAuthStart(c *gin.Context) {
	cfg, err := h.getLinuxDoOAuthConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	state, err := oauth.GenerateState()
	if err != nil {
		response.ErrorFrom(c, infraerrors.InternalServer("OAUTH_STATE_GEN_FAILED", "failed to generate oauth state").WithCause(err))
		return
	}

	redirectTo := sanitizeFrontendRedirectPath(c.Query("redirect"))
	if redirectTo == "" {
		redirectTo = linuxDoOAuthDefaultRedirectTo
	}
	intent := normalizeOAuthIntent(c.Query("intent"))

	secureCookie := isRequestHTTPS(c)
	setCookie(c, linuxDoOAuthStateCookieName, encodeCookieValue(state), linuxDoOAuthCookieMaxAgeSec, secureCookie)
	setCookie(c, linuxDoOAuthRedirectCookie, encodeCookieValue(redirectTo), linuxDoOAuthCookieMaxAgeSec, secureCookie)
	setCookie(c, linuxDoOAuthIntentCookieName, encodeCookieValue(intent), linuxDoOAuthCookieMaxAgeSec, secureCookie)

	codeChallenge := ""
	if cfg.UsePKCE {
		verifier, err := oauth.GenerateCodeVerifier()
		if err != nil {
			response.ErrorFrom(c, infraerrors.InternalServer("OAUTH_PKCE_GEN_FAILED", "failed to generate pkce verifier").WithCause(err))
			return
		}
		codeChallenge = oauth.GenerateCodeChallenge(verifier)
		setCookie(c, linuxDoOAuthVerifierCookie, encodeCookieValue(verifier), linuxDoOAuthCookieMaxAgeSec, secureCookie)
	}

	redirectURI := strings.TrimSpace(cfg.RedirectURL)
	if redirectURI == "" {
		response.ErrorFrom(c, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "oauth redirect url not configured"))
		return
	}

	authURL, err := buildLinuxDoAuthorizeURL(cfg, state, codeChallenge, redirectURI)
	if err != nil {
		response.ErrorFrom(c, infraerrors.InternalServer("OAUTH_BUILD_URL_FAILED", "failed to build oauth authorization url").WithCause(err))
		return
	}

	c.Redirect(http.StatusFound, authURL)
}

// LinuxDoOAuthCallback 处理 OAuth 回调：创建/登录用户，然后重定向到前端。
// GET /api/v1/auth/oauth/linuxdo/callback?code=...&state=...
func (h *AuthHandler) LinuxDoOAuthCallback(c *gin.Context) {
	cfg, cfgErr := h.getLinuxDoOAuthConfig(c.Request.Context())
	if cfgErr != nil {
		response.ErrorFrom(c, cfgErr)
		return
	}

	frontendCallback := strings.TrimSpace(cfg.FrontendRedirectURL)
	if frontendCallback == "" {
		frontendCallback = linuxDoOAuthDefaultFrontendCB
	}

	if providerErr := strings.TrimSpace(c.Query("error")); providerErr != "" {
		redirectOAuthError(c, frontendCallback, "provider_error", providerErr, c.Query("error_description"))
		return
	}

	code := strings.TrimSpace(c.Query("code"))
	state := strings.TrimSpace(c.Query("state"))
	if code == "" || state == "" {
		redirectOAuthError(c, frontendCallback, "missing_params", "missing code/state", "")
		return
	}

	secureCookie := isRequestHTTPS(c)
	defer func() {
		clearCookie(c, linuxDoOAuthStateCookieName, secureCookie)
		clearCookie(c, linuxDoOAuthVerifierCookie, secureCookie)
		clearCookie(c, linuxDoOAuthRedirectCookie, secureCookie)
		clearCookie(c, linuxDoOAuthIntentCookieName, secureCookie)
	}()

	expectedState, err := readCookieDecoded(c, linuxDoOAuthStateCookieName)
	if err != nil || expectedState == "" || state != expectedState {
		redirectOAuthError(c, frontendCallback, "invalid_state", "invalid oauth state", "")
		return
	}

	redirectTo, _ := readCookieDecoded(c, linuxDoOAuthRedirectCookie)
	redirectTo = sanitizeFrontendRedirectPath(redirectTo)
	if redirectTo == "" {
		redirectTo = linuxDoOAuthDefaultRedirectTo
	}
	intent := normalizedOAuthIntentFromCookie(c, linuxDoOAuthIntentCookieName)

	codeVerifier := ""
	if cfg.UsePKCE {
		codeVerifier, _ = readCookieDecoded(c, linuxDoOAuthVerifierCookie)
		if codeVerifier == "" {
			redirectOAuthError(c, frontendCallback, "missing_verifier", "missing pkce verifier", "")
			return
		}
	}

	redirectURI := strings.TrimSpace(cfg.RedirectURL)
	if redirectURI == "" {
		redirectOAuthError(c, frontendCallback, "config_error", "oauth redirect url not configured", "")
		return
	}

	tokenResp, err := linuxDoExchangeCode(c.Request.Context(), cfg, code, redirectURI, codeVerifier)
	if err != nil {
		description := ""
		var exchangeErr *linuxDoTokenExchangeError
		if errors.As(err, &exchangeErr) && exchangeErr != nil {
			log.Printf(
				"[LinuxDo OAuth] token exchange failed: status=%d provider_error=%q provider_description=%q body=%s",
				exchangeErr.StatusCode,
				exchangeErr.ProviderError,
				exchangeErr.ProviderDescription,
				truncateLogValue(exchangeErr.Body, 2048),
			)
			description = exchangeErr.Error()
		} else {
			log.Printf("[LinuxDo OAuth] token exchange failed: %v", err)
			description = err.Error()
		}
		redirectOAuthError(c, frontendCallback, "token_exchange_failed", "failed to exchange oauth code", singleLine(description))
		return
	}

	userInfo, err := linuxDoFetchUserInfo(c.Request.Context(), cfg, tokenResp)
	if err != nil {
		log.Printf("[LinuxDo OAuth] userinfo fetch failed: %v", err)
		redirectOAuthError(c, frontendCallback, "userinfo_failed", "failed to fetch user info", "")
		return
	}

	// 安全考虑：不要把第三方返回的 email 直接映射到本地账号（可能与本地邮箱用户冲突导致账号被接管）。
	// 统一使用基于 subject 的稳定合成邮箱来做账号绑定。
	if userInfo.Subject != "" {
		userInfo.Email = linuxDoSyntheticEmail(userInfo.Subject)
	}

	h.completeOAuthCallback(c, frontendCallback, redirectTo, oauthCallbackIdentity{
		Provider:             "linuxdo",
		Intent:               intent,
		ProviderType:         "linuxdo",
		ProviderKey:          linuxDoOAuthProviderKey,
		ProviderSubject:      userInfo.Subject,
		CompatEmail:          userInfo.Email,
		CompatUsername:       userInfo.Username,
		SuggestedDisplayName: userInfo.DisplayName,
		SuggestedAvatarURL:   userInfo.AvatarURL,
	})
}

type completeLinuxDoOAuthRequest struct {
	PendingAuthToken  string `json:"pending_auth_token,omitempty"`
	PendingOAuthToken string `json:"pending_oauth_token,omitempty"`
	InvitationCode    string `json:"invitation_code" binding:"required"`
}

// CompleteLinuxDoOAuthRegistration completes a pending OAuth registration by validating
// the invitation code and creating the user account.
// POST /api/v1/auth/oauth/linuxdo/complete-registration
func (h *AuthHandler) CompleteLinuxDoOAuthRegistration(c *gin.Context) {
	var req completeLinuxDoOAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_REQUEST", "message": err.Error()})
		return
	}

	email, username, pendingAuthToken, err := h.resolveLinuxDoRegistrationIdentity(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "INVALID_TOKEN", "message": "invalid or expired registration token"})
		return
	}

	tokenPair, user, err := h.authService.LoginOrRegisterOAuthWithTokenPair(c.Request.Context(), email, username, req.InvitationCode)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if pendingAuthToken != "" && user != nil {
		if _, err := h.authService.CompletePendingAuthSessionBind(c.Request.Context(), pendingAuthToken, user.ID); err != nil {
			response.ErrorFrom(c, err)
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token":  tokenPair.AccessToken,
		"refresh_token": tokenPair.RefreshToken,
		"expires_in":    tokenPair.ExpiresIn,
		"token_type":    "Bearer",
	})
}

func (h *AuthHandler) resolveLinuxDoRegistrationIdentity(ctx context.Context, req completeLinuxDoOAuthRequest) (email, username, pendingAuthToken string, err error) {
	if pendingAuthToken = strings.TrimSpace(req.PendingAuthToken); pendingAuthToken != "" {
		session, sessionErr := h.authService.GetPendingAuthSessionForProgress(ctx, pendingAuthToken, nil)
		if sessionErr != nil {
			return "", "", "", sessionErr
		}

		email = strings.TrimSpace(oauthMetadataString(session.Metadata, "compat_email"))
		username = strings.TrimSpace(oauthMetadataString(session.Metadata, "compat_username"))
		if email == "" || username == "" {
			fallbackSubject := strings.TrimSpace(session.ProviderSubject)
			if fallbackSubject == "" {
				return "", "", "", service.ErrInvalidToken
			}
			email = linuxDoSyntheticEmail(fallbackSubject)
			username = fallbackSubject
		}
		return email, username, pendingAuthToken, nil
	}

	email, username, err = h.authService.VerifyPendingOAuthToken(strings.TrimSpace(req.PendingOAuthToken))
	return email, username, "", err
}

func (h *AuthHandler) getLinuxDoOAuthConfig(ctx context.Context) (config.LinuxDoConnectConfig, error) {
	if h != nil && h.settingSvc != nil {
		return h.settingSvc.GetLinuxDoConnectOAuthConfig(ctx)
	}
	if h == nil || h.cfg == nil {
		return config.LinuxDoConnectConfig{}, infraerrors.ServiceUnavailable("CONFIG_NOT_READY", "config not loaded")
	}
	if !h.cfg.LinuxDo.Enabled {
		return config.LinuxDoConnectConfig{}, infraerrors.NotFound("OAUTH_DISABLED", "oauth login is disabled")
	}
	return h.cfg.LinuxDo, nil
}

func linuxDoExchangeCode(
	ctx context.Context,
	cfg config.LinuxDoConnectConfig,
	code string,
	redirectURI string,
	codeVerifier string,
) (*linuxDoTokenResponse, error) {
	client := req.C().SetTimeout(30 * time.Second)

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", cfg.ClientID)
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	if cfg.UsePKCE {
		form.Set("code_verifier", codeVerifier)
	}

	r := client.R().
		SetContext(ctx).
		SetHeader("Accept", "application/json")

	switch strings.ToLower(strings.TrimSpace(cfg.TokenAuthMethod)) {
	case "", "client_secret_post":
		form.Set("client_secret", cfg.ClientSecret)
	case "client_secret_basic":
		r.SetBasicAuth(cfg.ClientID, cfg.ClientSecret)
	case "none":
	default:
		return nil, fmt.Errorf("unsupported token_auth_method: %s", cfg.TokenAuthMethod)
	}

	resp, err := r.SetFormDataFromValues(form).Post(cfg.TokenURL)
	if err != nil {
		return nil, fmt.Errorf("request token: %w", err)
	}
	body := strings.TrimSpace(resp.String())
	if !resp.IsSuccessState() {
		providerErr, providerDesc := parseOAuthProviderError(body)
		return nil, &linuxDoTokenExchangeError{
			StatusCode:          resp.StatusCode,
			ProviderError:       providerErr,
			ProviderDescription: providerDesc,
			Body:                body,
		}
	}

	tokenResp, ok := parseLinuxDoTokenResponse(body)
	if !ok || strings.TrimSpace(tokenResp.AccessToken) == "" {
		return nil, &linuxDoTokenExchangeError{
			StatusCode: resp.StatusCode,
			Body:       body,
		}
	}
	if strings.TrimSpace(tokenResp.TokenType) == "" {
		tokenResp.TokenType = "Bearer"
	}
	return tokenResp, nil
}

func linuxDoFetchUserInfo(
	ctx context.Context,
	cfg config.LinuxDoConnectConfig,
	token *linuxDoTokenResponse,
) (*linuxDoUserInfoClaims, error) {
	client := req.C().SetTimeout(30 * time.Second)
	authorization, err := buildBearerAuthorization(token.TokenType, token.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("invalid token for userinfo request: %w", err)
	}

	resp, err := client.R().
		SetContext(ctx).
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", authorization).
		Get(cfg.UserInfoURL)
	if err != nil {
		return nil, fmt.Errorf("request userinfo: %w", err)
	}
	if !resp.IsSuccessState() {
		return nil, fmt.Errorf("userinfo status=%d", resp.StatusCode)
	}

	return linuxDoParseUserInfo(resp.String(), cfg)
}

func linuxDoParseUserInfo(body string, cfg config.LinuxDoConnectConfig) (*linuxDoUserInfoClaims, error) {
	claims := &linuxDoUserInfoClaims{}
	claims.Email = firstNonEmpty(
		getGJSON(body, cfg.UserInfoEmailPath),
		getGJSON(body, "email"),
		getGJSON(body, "user.email"),
		getGJSON(body, "data.email"),
		getGJSON(body, "attributes.email"),
	)
	claims.Username = firstNonEmpty(
		getGJSON(body, cfg.UserInfoUsernamePath),
		getGJSON(body, "username"),
		getGJSON(body, "preferred_username"),
		getGJSON(body, "name"),
		getGJSON(body, "user.username"),
		getGJSON(body, "user.name"),
	)
	claims.Subject = firstNonEmpty(
		getGJSON(body, cfg.UserInfoIDPath),
		getGJSON(body, "sub"),
		getGJSON(body, "id"),
		getGJSON(body, "user_id"),
		getGJSON(body, "uid"),
		getGJSON(body, "user.id"),
	)
	claims.DisplayName = firstNonEmpty(
		getGJSON(body, "name"),
		getGJSON(body, "nickname"),
		getGJSON(body, "username"),
		getGJSON(body, "preferred_username"),
		getGJSON(body, "user.name"),
		getGJSON(body, "user.username"),
		getGJSON(body, "data.name"),
		getGJSON(body, "data.username"),
	)
	claims.AvatarURL = firstNonEmpty(
		getGJSON(body, "avatar"),
		getGJSON(body, "avatar_url"),
		getGJSON(body, "picture"),
		getGJSON(body, "profile_image_url"),
		getGJSON(body, "user.avatar"),
		getGJSON(body, "user.avatar_url"),
		getGJSON(body, "data.avatar"),
		getGJSON(body, "data.avatar_url"),
		getGJSON(body, "attributes.avatar"),
		getGJSON(body, "attributes.avatar_url"),
	)

	claims.Subject = strings.TrimSpace(claims.Subject)
	if claims.Subject == "" {
		return nil, errors.New("userinfo missing id field")
	}
	if !isSafeLinuxDoSubject(claims.Subject) {
		return nil, errors.New("userinfo returned invalid id field")
	}

	claims.Email = strings.TrimSpace(claims.Email)
	if claims.Email == "" {
		// LinuxDo Connect 的 userinfo 可能不提供 email。为兼容现有用户模型（email 必填且唯一），使用稳定的合成邮箱。
		claims.Email = linuxDoSyntheticEmail(claims.Subject)
	}

	claims.Username = strings.TrimSpace(claims.Username)
	if claims.Username == "" {
		claims.Username = "linuxdo_" + claims.Subject
	}
	claims.DisplayName = strings.TrimSpace(claims.DisplayName)
	if claims.DisplayName == "" {
		claims.DisplayName = claims.Username
	}
	claims.AvatarURL = strings.TrimSpace(claims.AvatarURL)

	return claims, nil
}

func buildLinuxDoAuthorizeURL(cfg config.LinuxDoConnectConfig, state string, codeChallenge string, redirectURI string) (string, error) {
	u, err := url.Parse(cfg.AuthorizeURL)
	if err != nil {
		return "", fmt.Errorf("parse authorize_url: %w", err)
	}

	q := u.Query()
	q.Set("response_type", "code")
	q.Set("client_id", cfg.ClientID)
	q.Set("redirect_uri", redirectURI)
	if strings.TrimSpace(cfg.Scopes) != "" {
		q.Set("scope", cfg.Scopes)
	}
	q.Set("state", state)
	if cfg.UsePKCE {
		q.Set("code_challenge", codeChallenge)
		q.Set("code_challenge_method", "S256")
	}

	u.RawQuery = q.Encode()
	return u.String(), nil
}

func redirectOAuthError(c *gin.Context, frontendCallback string, code string, message string, description string) {
	fragment := url.Values{}
	fragment.Set("error", truncateFragmentValue(code))
	if strings.TrimSpace(message) != "" {
		fragment.Set("error_message", truncateFragmentValue(message))
	}
	if strings.TrimSpace(description) != "" {
		fragment.Set("error_description", truncateFragmentValue(description))
	}
	redirectWithFragment(c, frontendCallback, fragment)
}

func redirectWithFragment(c *gin.Context, frontendCallback string, fragment url.Values) {
	u, err := url.Parse(frontendCallback)
	if err != nil {
		// 兜底：尽力跳转到默认页面，避免卡死在回调页。
		c.Redirect(http.StatusFound, linuxDoOAuthDefaultRedirectTo)
		return
	}
	if u.Scheme != "" && !strings.EqualFold(u.Scheme, "http") && !strings.EqualFold(u.Scheme, "https") {
		c.Redirect(http.StatusFound, linuxDoOAuthDefaultRedirectTo)
		return
	}
	u.Fragment = fragment.Encode()
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	c.Redirect(http.StatusFound, u.String())
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" {
			return v
		}
	}
	return ""
}

func parseOAuthProviderError(body string) (providerErr string, providerDesc string) {
	body = strings.TrimSpace(body)
	if body == "" {
		return "", ""
	}

	providerErr = firstNonEmpty(
		getGJSON(body, "error"),
		getGJSON(body, "code"),
		getGJSON(body, "error.code"),
	)
	providerDesc = firstNonEmpty(
		getGJSON(body, "error_description"),
		getGJSON(body, "error.message"),
		getGJSON(body, "message"),
		getGJSON(body, "detail"),
	)

	if providerErr != "" || providerDesc != "" {
		return providerErr, providerDesc
	}

	values, err := url.ParseQuery(body)
	if err != nil {
		return "", ""
	}
	providerErr = firstNonEmpty(values.Get("error"), values.Get("code"))
	providerDesc = firstNonEmpty(values.Get("error_description"), values.Get("error_message"), values.Get("message"))
	return providerErr, providerDesc
}

func parseLinuxDoTokenResponse(body string) (*linuxDoTokenResponse, bool) {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil, false
	}

	accessToken := strings.TrimSpace(getGJSON(body, "access_token"))
	if accessToken != "" {
		tokenType := strings.TrimSpace(getGJSON(body, "token_type"))
		refreshToken := strings.TrimSpace(getGJSON(body, "refresh_token"))
		scope := strings.TrimSpace(getGJSON(body, "scope"))
		expiresIn := gjson.Get(body, "expires_in").Int()
		return &linuxDoTokenResponse{
			AccessToken:  accessToken,
			TokenType:    tokenType,
			ExpiresIn:    expiresIn,
			RefreshToken: refreshToken,
			Scope:        scope,
		}, true
	}

	values, err := url.ParseQuery(body)
	if err != nil {
		return nil, false
	}
	accessToken = strings.TrimSpace(values.Get("access_token"))
	if accessToken == "" {
		return nil, false
	}
	expiresIn := int64(0)
	if raw := strings.TrimSpace(values.Get("expires_in")); raw != "" {
		if v, err := strconv.ParseInt(raw, 10, 64); err == nil {
			expiresIn = v
		}
	}
	return &linuxDoTokenResponse{
		AccessToken:  accessToken,
		TokenType:    strings.TrimSpace(values.Get("token_type")),
		ExpiresIn:    expiresIn,
		RefreshToken: strings.TrimSpace(values.Get("refresh_token")),
		Scope:        strings.TrimSpace(values.Get("scope")),
	}, true
}

func getGJSON(body string, path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	res := gjson.Get(body, path)
	if !res.Exists() {
		return ""
	}
	return res.String()
}

func truncateLogValue(value string, maxLen int) string {
	value = strings.TrimSpace(value)
	if value == "" || maxLen <= 0 {
		return ""
	}
	if len(value) <= maxLen {
		return value
	}
	value = value[:maxLen]
	for !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return value
}

func singleLine(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return strings.Join(strings.Fields(value), " ")
}

func sanitizeFrontendRedirectPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if len(path) > linuxDoOAuthMaxRedirectLen {
		return ""
	}
	// 只允许同源相对路径（避免开放重定向）。
	if !strings.HasPrefix(path, "/") {
		return ""
	}
	if strings.HasPrefix(path, "//") {
		return ""
	}
	if strings.Contains(path, "://") {
		return ""
	}
	if strings.ContainsAny(path, "\r\n") {
		return ""
	}
	return path
}

func isRequestHTTPS(c *gin.Context) bool {
	if c.Request.TLS != nil {
		return true
	}
	proto := strings.ToLower(strings.TrimSpace(c.GetHeader("X-Forwarded-Proto")))
	return proto == "https"
}

func encodeCookieValue(value string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(value))
}

func decodeCookieValue(value string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func readCookieDecoded(c *gin.Context, name string) (string, error) {
	ck, err := c.Request.Cookie(name)
	if err != nil {
		return "", err
	}
	return decodeCookieValue(ck.Value)
}

func (h *AuthHandler) resolveOAuthBindTargetUserID(c *gin.Context) (*int64, error) {
	if subject, ok := middleware2.GetAuthSubjectFromContext(c); ok && subject.UserID > 0 {
		return &subject.UserID, nil
	}
	if h == nil || h.authService == nil || h.userService == nil {
		return nil, service.ErrInvalidToken
	}

	ck, err := c.Request.Cookie(oauthBindAccessTokenCookieName)
	clearOAuthBindAccessTokenCookie(c, isRequestHTTPS(c))
	if err != nil {
		return nil, err
	}
	tokenString := strings.TrimSpace(ck.Value)
	if tokenString == "" {
		return nil, service.ErrInvalidToken
	}

	claims, err := h.authService.ValidateToken(tokenString)
	if err != nil {
		return nil, err
	}
	user, err := h.userService.GetByID(c.Request.Context(), claims.UserID)
	if err != nil {
		return nil, err
	}
	if user == nil || !user.IsActive() || claims.TokenVersion != user.TokenVersion {
		return nil, service.ErrInvalidToken
	}
	return &user.ID, nil
}

func setCookie(c *gin.Context, name string, value string, maxAgeSec int, secure bool) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     linuxDoOAuthCookiePath,
		MaxAge:   maxAgeSec,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearCookie(c *gin.Context, name string, secure bool) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     linuxDoOAuthCookiePath,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearOAuthBindAccessTokenCookie(c *gin.Context, secure bool) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     oauthBindAccessTokenCookieName,
		Value:    "",
		Path:     oauthBindAccessTokenCookiePath,
		MaxAge:   -1,
		HttpOnly: false,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func truncateFragmentValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if len(value) > linuxDoOAuthMaxFragmentValueLen {
		value = value[:linuxDoOAuthMaxFragmentValueLen]
		for !utf8.ValidString(value) {
			value = value[:len(value)-1]
		}
	}
	return value
}

func buildBearerAuthorization(tokenType, accessToken string) (string, error) {
	tokenType = strings.TrimSpace(tokenType)
	if tokenType == "" {
		tokenType = "Bearer"
	}
	if !strings.EqualFold(tokenType, "Bearer") {
		return "", fmt.Errorf("unsupported token_type: %s", tokenType)
	}

	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		return "", errors.New("missing access_token")
	}
	if strings.ContainsAny(accessToken, " \t\r\n") {
		return "", errors.New("access_token contains whitespace")
	}
	return "Bearer " + accessToken, nil
}

func isSafeLinuxDoSubject(subject string) bool {
	subject = strings.TrimSpace(subject)
	if subject == "" || len(subject) > linuxDoOAuthMaxSubjectLen {
		return false
	}
	for _, r := range subject {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r == '_' || r == '-':
		default:
			return false
		}
	}
	return true
}

func linuxDoSyntheticEmail(subject string) string {
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return ""
	}
	return "linuxdo-" + subject + service.LinuxDoConnectSyntheticEmailDomain
}

type oauthCallbackIdentity struct {
	Provider             string
	Intent               string
	ProviderType         string
	ProviderKey          string
	ProviderSubject      string
	CompatEmail          string
	CompatUsername       string
	Metadata             map[string]any
	SuggestedDisplayName string
	SuggestedAvatarURL   string
}

type authIdentityFinder interface {
	FindAuthIdentity(ctx context.Context, providerType, providerKey, providerSubject string) (*repository.AuthIdentityRecord, error)
}

type authIdentityChannelFinder interface {
	FindAuthIdentityChannel(ctx context.Context, providerType, providerKey, channel, channelAppID, channelSubject string) (*repository.AuthIdentityChannelRecord, error)
}

type authIdentityByIDGetter interface {
	GetAuthIdentityByID(ctx context.Context, id int64) (*repository.AuthIdentityRecord, error)
}

func normalizeOAuthIntent(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "login", "sign_in", "signin", "register", "signup":
		return service.PendingAuthIntentLogin
	case "bind", service.PendingAuthIntentBindCurrentUser:
		return service.PendingAuthIntentBindCurrentUser
	default:
		return service.PendingAuthIntentLogin
	}
}

func normalizedOAuthIntentFromCookie(c *gin.Context, cookieName string) string {
	value, _ := readCookieDecoded(c, cookieName)
	return normalizeOAuthIntent(value)
}

func (h *AuthHandler) completeOAuthCallback(c *gin.Context, frontendCallback, redirectTo string, identity oauthCallbackIdentity) {
	intent := normalizeOAuthIntent(identity.Intent)
	var bindTargetUserID *int64
	if intent == service.PendingAuthIntentBindCurrentUser {
		resolvedUserID, err := h.resolveOAuthBindTargetUserID(c)
		if err != nil || resolvedUserID == nil || *resolvedUserID <= 0 {
			redirectOAuthError(c, frontendCallback, "auth_required", "missing authenticated subject", "")
			return
		}
		bindTargetUserID = resolvedUserID
	}
	boundUserID, err := h.lookupBoundOAuthUserID(c.Request.Context(), identity.ProviderType, identity.ProviderKey, identity.ProviderSubject, identity.Metadata)
	if err != nil {
		redirectOAuthError(c, frontendCallback, "login_failed", "identity_lookup_failed", "")
		return
	}

	if intent == service.PendingAuthIntentLogin && boundUserID != nil {
		h.redirectOAuthLoginSuccess(c, frontendCallback, redirectTo, identity.Provider, intent, *boundUserID)
		return
	}

	pendingInput := service.PendingAuthSessionInput{
		Intent:          intent,
		ProviderType:    identity.ProviderType,
		ProviderKey:     identity.ProviderKey,
		ProviderSubject: strings.TrimSpace(identity.ProviderSubject),
		TargetUserID:    bindTargetUserID,
		RedirectTo:      redirectTo,
		Metadata:        cloneOAuthMetadataMap(identity.Metadata),
	}
	adoptionRequired := false
	if pendingInput.Metadata == nil {
		pendingInput.Metadata = make(map[string]any, 2)
	}
	if displayName := strings.TrimSpace(identity.SuggestedDisplayName); displayName != "" {
		pendingInput.Metadata["suggested_display_name"] = displayName
		adoptionRequired = true
	}
	if avatarURL := strings.TrimSpace(identity.SuggestedAvatarURL); avatarURL != "" {
		pendingInput.Metadata["suggested_avatar_url"] = avatarURL
		adoptionRequired = true
	}
	pendingAuthToken, err := h.authService.CreatePendingAuthSession(c.Request.Context(), pendingInput)
	if err != nil {
		redirectOAuthError(c, frontendCallback, "login_failed", "service_error", "")
		return
	}

	fragment := url.Values{}
	fragment.Set("auth_result", "pending_session")
	fragment.Set("pending_auth_token", pendingAuthToken)
	fragment.Set("provider", truncateFragmentValue(identity.Provider))
	fragment.Set("intent", truncateFragmentValue(intent))
	fragment.Set("redirect", truncateFragmentValue(redirectTo))
	if adoptionRequired {
		fragment.Set("adoption_required", "true")
	}

	if compatEmail := strings.TrimSpace(identity.CompatEmail); compatEmail != "" && strings.TrimSpace(identity.CompatUsername) != "" {
		if compatToken, compatErr := h.authService.CreatePendingOAuthToken(compatEmail, identity.CompatUsername); compatErr == nil {
			fragment.Set("pending_oauth_token", compatToken)
		}
	}
	if displayName := strings.TrimSpace(identity.SuggestedDisplayName); displayName != "" {
		fragment.Set("suggested_display_name", truncateFragmentValue(displayName))
	}
	if avatarURL := strings.TrimSpace(identity.SuggestedAvatarURL); avatarURL != "" {
		fragment.Set("suggested_avatar_url", truncateFragmentValue(avatarURL))
	}
	redirectWithFragment(c, frontendCallback, fragment)
}

func (h *AuthHandler) redirectOAuthLoginSuccess(c *gin.Context, frontendCallback, redirectTo, provider, intent string, userID int64) {
	user, err := h.userService.GetByID(c.Request.Context(), userID)
	if err != nil {
		redirectOAuthError(c, frontendCallback, "login_failed", infraerrors.Reason(err), infraerrors.Message(err))
		return
	}
	if user == nil || !user.IsActive() {
		redirectOAuthError(c, frontendCallback, "login_failed", "user_not_active", "")
		return
	}
	if h.totpService != nil && h.settingSvc != nil && h.settingSvc.IsTotpEnabled(c.Request.Context()) && user.TotpEnabled {
		tempToken, err := h.totpService.CreateLoginSession(c.Request.Context(), user.ID, user.Email)
		if err != nil {
			redirectOAuthError(c, frontendCallback, "login_failed", "service_error", "")
			return
		}

		fragment := url.Values{}
		fragment.Set("requires_2fa", "true")
		fragment.Set("temp_token", tempToken)
		fragment.Set("user_email_masked", service.MaskEmail(user.Email))
		fragment.Set("provider", truncateFragmentValue(provider))
		fragment.Set("intent", truncateFragmentValue(intent))
		fragment.Set("redirect", truncateFragmentValue(redirectTo))
		redirectWithFragment(c, frontendCallback, fragment)
		return
	}

	tokenPair, err := h.authService.GenerateTokenPair(c.Request.Context(), user, "")
	if err != nil {
		redirectOAuthError(c, frontendCallback, "login_failed", "service_error", "")
		return
	}

	fragment := url.Values{}
	fragment.Set("access_token", tokenPair.AccessToken)
	fragment.Set("refresh_token", tokenPair.RefreshToken)
	fragment.Set("expires_in", fmt.Sprintf("%d", tokenPair.ExpiresIn))
	fragment.Set("token_type", "Bearer")
	fragment.Set("provider", truncateFragmentValue(provider))
	fragment.Set("intent", truncateFragmentValue(intent))
	fragment.Set("redirect", truncateFragmentValue(redirectTo))
	redirectWithFragment(c, frontendCallback, fragment)
}

func (h *AuthHandler) lookupBoundOAuthUserID(ctx context.Context, providerType, providerKey, providerSubject string, metadata map[string]any) (*int64, error) {
	repo := extractAuthServiceUserRepo(h.authService)
	if repo == nil {
		return nil, nil
	}

	if providerType == "wechat" {
		userID, err := lookupBoundWeChatUserID(ctx, repo, providerKey, metadata)
		if userID != nil || err != nil {
			return userID, err
		}
	}

	return lookupBoundIdentityUserID(ctx, repo, providerType, providerKey, providerSubject)
}

func lookupBoundWeChatUserID(ctx context.Context, repo any, providerKey string, metadata map[string]any) (*int64, error) {
	if unionid := oauthMetadataString(metadata, "unionid"); unionid != "" {
		return lookupBoundIdentityUserID(ctx, repo, "wechat", providerKey, unionid)
	}

	openid := oauthMetadataString(metadata, "openid")
	channel := oauthMetadataString(metadata, "channel")
	appid := oauthMetadataString(metadata, "appid")
	if openid == "" || channel == "" || appid == "" {
		return nil, nil
	}

	channelFinder, ok := repo.(authIdentityChannelFinder)
	if !ok {
		return nil, nil
	}
	channelRecord, err := channelFinder.FindAuthIdentityChannel(ctx, "wechat", providerKey, channel, appid, openid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if channelRecord == nil {
		return nil, nil
	}
	identityGetter, ok := repo.(authIdentityByIDGetter)
	if !ok {
		return nil, nil
	}
	identity, err := identityGetter.GetAuthIdentityByID(ctx, channelRecord.IdentityID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if identity == nil {
		return nil, nil
	}
	userID := identity.UserID
	return &userID, nil
}

func lookupBoundIdentityUserID(ctx context.Context, repo any, providerType, providerKey, providerSubject string) (*int64, error) {
	providerSubject = strings.TrimSpace(providerSubject)
	if providerSubject == "" {
		return nil, nil
	}

	finder, ok := repo.(authIdentityFinder)
	if !ok {
		return nil, nil
	}
	identity, err := finder.FindAuthIdentity(ctx, providerType, providerKey, providerSubject)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if identity == nil {
		return nil, nil
	}
	userID := identity.UserID
	return &userID, nil
}

func oauthMetadataString(metadata map[string]any, key string) string {
	if len(metadata) == 0 {
		return ""
	}
	raw, ok := metadata[key]
	if !ok || raw == nil {
		return ""
	}
	switch v := raw.(type) {
	case string:
		return strings.TrimSpace(v)
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func extractAuthServiceUserRepo(authSvc *service.AuthService) any {
	if authSvc == nil {
		return nil
	}
	value := reflect.ValueOf(authSvc)
	if !value.IsValid() || value.IsNil() {
		return nil
	}
	elem := value.Elem()
	field := elem.FieldByName("userRepo")
	if !field.IsValid() || !field.CanAddr() {
		return nil
	}
	return reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Interface()
}

func cloneOAuthMetadataMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
