package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/oauth"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/imroc/req/v3"
)

const (
	weChatOAuthCookiePath                     = "/api/v1/auth/oauth/wechat"
	weChatOAuthStateCookieName                = "wechat_oauth_state"
	weChatOAuthRedirectCookieName             = "wechat_oauth_redirect"
	weChatOAuthIntentCookieName               = "wechat_oauth_intent"
	weChatOAuthModeCookieName                 = "wechat_oauth_mode"
	weChatOAuthScopeCookieName                = "wechat_oauth_scope"
	weChatOAuthCookieMaxAgeSec                = 10 * 60
	weChatOAuthDefaultRedirectTo              = "/dashboard"
	weChatOAuthDefaultFrontendCallback        = "/auth/wechat/callback"
	weChatOAuthOpenAuthorizeURL               = "https://open.weixin.qq.com/connect/qrconnect"
	weChatOAuthMPAuthorizeURL                 = "https://open.weixin.qq.com/connect/oauth2/authorize"
	weChatOAuthUserInfoURL                    = "https://api.weixin.qq.com/sns/userinfo"
	weChatConnectModeOpen                     = "open"
	weChatConnectModeMP                       = "mp"
	weChatPaymentOAuthCookiePath              = "/api/v1/auth/oauth/wechat/payment"
	weChatPaymentOAuthStateCookieName         = "wechat_payment_oauth_state"
	weChatPaymentOAuthRedirectCookieName      = "wechat_payment_oauth_redirect"
	weChatPaymentOAuthContextCookieName       = "wechat_payment_oauth_context"
	weChatPaymentOAuthScopeCookieName         = "wechat_payment_oauth_scope"
	weChatPaymentOAuthCookieMaxAgeSec         = 10 * 60
	weChatPaymentOAuthDefaultRedirectTo       = "/purchase"
	weChatPaymentOAuthDefaultFrontendCallback = "/auth/wechat/payment/callback"
	weChatPaymentOAuthAuthorizeURL            = "https://open.weixin.qq.com/connect/oauth2/authorize"
	weChatPaymentOAuthTokenURL                = "https://api.weixin.qq.com/sns/oauth2/access_token"
)

type weChatOAuthExchangeError struct {
	StatusCode int
	ErrCode    string
	ErrMsg     string
	Body       string
}

func (e *weChatOAuthExchangeError) Error() string {
	if e == nil {
		return ""
	}
	parts := []string{fmt.Sprintf("openid exchange status=%d", e.StatusCode)}
	if strings.TrimSpace(e.ErrCode) != "" {
		parts = append(parts, "errcode="+strings.TrimSpace(e.ErrCode))
	}
	if strings.TrimSpace(e.ErrMsg) != "" {
		parts = append(parts, "errmsg="+strings.TrimSpace(e.ErrMsg))
	}
	return strings.Join(parts, " ")
}

type weChatPaymentTokenResult struct {
	OpenID string
	Scope  string
}

type weChatLoginTokenResult struct {
	AccessToken string
	OpenID      string
	Scope       string
	UnionID     string
}

type weChatLoginUserInfo struct {
	OpenID    string
	UnionID   string
	Nickname  string
	AvatarURL string
}

type weChatPaymentOAuthContext struct {
	PaymentType string `json:"payment_type"`
	Amount      string `json:"amount,omitempty"`
	OrderType   string `json:"order_type,omitempty"`
	PlanID      int64  `json:"plan_id,omitempty"`
}

// WeChatOAuthStart starts the login-purpose WeChat OAuth flow.
// GET /api/v1/auth/oauth/wechat/start?redirect=/dashboard
func (h *AuthHandler) WeChatOAuthStart(c *gin.Context) {
	cfg, err := h.getWeChatOAuthConfig(c.Request.Context())
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
		redirectTo = weChatOAuthDefaultRedirectTo
	}
	intent := normalizeOAuthIntent(c.Query("intent"))

	mode := normalizeWeChatLoginMode(cfg.Mode)
	scope := normalizeWeChatLoginScope(c.Query("scope"), mode, cfg.Scopes)

	secureCookie := isRequestHTTPS(c)
	weChatSetCookie(c, weChatOAuthStateCookieName, encodeCookieValue(state), weChatOAuthCookieMaxAgeSec, secureCookie)
	weChatSetCookie(c, weChatOAuthRedirectCookieName, encodeCookieValue(redirectTo), weChatOAuthCookieMaxAgeSec, secureCookie)
	weChatSetCookie(c, weChatOAuthIntentCookieName, encodeCookieValue(intent), weChatOAuthCookieMaxAgeSec, secureCookie)
	weChatSetCookie(c, weChatOAuthModeCookieName, encodeCookieValue(mode), weChatOAuthCookieMaxAgeSec, secureCookie)
	weChatSetCookie(c, weChatOAuthScopeCookieName, encodeCookieValue(scope), weChatOAuthCookieMaxAgeSec, secureCookie)

	authURL, err := buildWeChatAuthorizeURL(cfg, mode, scope, state, strings.TrimSpace(cfg.RedirectURL))
	if err != nil {
		response.ErrorFrom(c, infraerrors.InternalServer("OAUTH_BUILD_URL_FAILED", "failed to build oauth authorization url").WithCause(err))
		return
	}

	c.Redirect(http.StatusFound, authURL)
}

// WeChatOAuthCallback handles the login-purpose WeChat OAuth callback.
// GET /api/v1/auth/oauth/wechat/callback?code=...&state=...
func (h *AuthHandler) WeChatOAuthCallback(c *gin.Context) {
	cfg, cfgErr := h.getWeChatOAuthConfig(c.Request.Context())
	if cfgErr != nil {
		response.ErrorFrom(c, cfgErr)
		return
	}

	frontendCallback := strings.TrimSpace(cfg.FrontendRedirectURL)
	if frontendCallback == "" {
		frontendCallback = weChatOAuthDefaultFrontendCallback
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
		weChatClearCookie(c, weChatOAuthStateCookieName, secureCookie)
		weChatClearCookie(c, weChatOAuthRedirectCookieName, secureCookie)
		weChatClearCookie(c, weChatOAuthIntentCookieName, secureCookie)
		weChatClearCookie(c, weChatOAuthModeCookieName, secureCookie)
		weChatClearCookie(c, weChatOAuthScopeCookieName, secureCookie)
	}()

	expectedState, err := readCookieDecoded(c, weChatOAuthStateCookieName)
	if err != nil || expectedState == "" || state != expectedState {
		redirectOAuthError(c, frontendCallback, "invalid_state", "invalid oauth state", "")
		return
	}

	redirectTo, _ := readCookieDecoded(c, weChatOAuthRedirectCookieName)
	redirectTo = sanitizeFrontendRedirectPath(redirectTo)
	if redirectTo == "" {
		redirectTo = weChatOAuthDefaultRedirectTo
	}
	intent, _ := readCookieDecoded(c, weChatOAuthIntentCookieName)
	intent = normalizeOAuthIntent(intent)

	mode, _ := readCookieDecoded(c, weChatOAuthModeCookieName)
	mode = normalizeWeChatLoginMode(firstNonEmpty(mode, cfg.Mode))

	scope, _ := readCookieDecoded(c, weChatOAuthScopeCookieName)
	scope = normalizeWeChatLoginScope(scope, mode, cfg.Scopes)

	tokenResult, err := wechatLoginExchangeCode(c.Request.Context(), cfg, code)
	if err != nil {
		description := err.Error()
		var exchangeErr *weChatOAuthExchangeError
		if errors.As(err, &exchangeErr) && exchangeErr != nil {
			log.Printf(
				"[WeChat OAuth] login token exchange failed: status=%d errcode=%q errmsg=%q body=%s",
				exchangeErr.StatusCode,
				exchangeErr.ErrCode,
				exchangeErr.ErrMsg,
				truncateLogValue(exchangeErr.Body, 2048),
			)
		} else {
			log.Printf("[WeChat OAuth] login token exchange failed: %v", err)
		}
		redirectOAuthError(c, frontendCallback, "token_exchange_failed", "failed to exchange oauth code", singleLine(description))
		return
	}

	subject := strings.TrimSpace(firstNonEmpty(tokenResult.UnionID, tokenResult.OpenID))
	if subject == "" {
		redirectOAuthError(c, frontendCallback, "missing_subject", "missing wechat subject", "")
		return
	}

	userInfo, userInfoErr := wechatLoginFetchUserInfo(c.Request.Context(), tokenResult)
	if userInfoErr != nil {
		log.Printf("[WeChat OAuth] userinfo fetch failed, fallback to subject-based username: %v", userInfoErr)
	}

	hasUnionID := strings.TrimSpace(tokenResult.UnionID) != ""
	if userInfo != nil && strings.TrimSpace(userInfo.UnionID) != "" {
		subject = strings.TrimSpace(userInfo.UnionID)
		hasUnionID = true
	}

	if tokenResult != nil && strings.TrimSpace(tokenResult.Scope) != "" {
		scope = strings.TrimSpace(tokenResult.Scope)
	}

	identityKey := weChatIdentityKey(mode, cfg.AppID, subject, hasUnionID)
	legacyEmail := weChatSyntheticEmailFromIdentityKey(identityKey)
	username := firstNonEmpty(weChatLoginNickname(userInfo), weChatFallbackUsername(subject))
	pendingIdentity := service.PendingOAuthIdentity{
		Email:       "",
		Username:    username,
		Provider:    "wechat",
		Subject:     subject,
		IdentityKey: identityKey,
		AvatarURL:   weChatLoginAvatarURL(userInfo),
		Intent:      intent,
	}

	if existingUser, lookupErr := h.authService.FindUserByExternalIdentity(c.Request.Context(), pendingIdentity.Provider, pendingIdentity.Subject); lookupErr == nil && existingUser != nil {
		if isOAuthBindIntent(intent) {
			fragment := url.Values{}
			fragment.Set("error", "external_identity_already_bound")
			fragment.Set("provider", "wechat")
			appendOAuthFlowFragment(fragment, redirectTo, intent)
			redirectWithFragment(c, frontendCallback, fragment)
			return
		}
		h.redirectOAuthSuccessForUser(c, frontendCallback, redirectTo, existingUser)
		return
	}

	if !isOAuthBindIntent(intent) && h.userService != nil && legacyEmail != "" {
		if legacyUser, legacyErr := h.userService.GetByEmail(c.Request.Context(), legacyEmail); legacyErr == nil && legacyUser != nil {
			if _, bindErr := h.authService.BindPendingOAuthIdentityToUser(c.Request.Context(), legacyUser.ID, pendingIdentity); bindErr == nil {
				// Backward compatibility for pre-identity WeChat users migrated from
				// the synthetic-email login flow.
				h.redirectOAuthSuccessForUser(c, frontendCallback, redirectTo, legacyUser)
				return
			}
		}
	}

	pendingToken, tokenErr := h.authService.CreatePendingOAuthTokenWithIdentity(pendingIdentity)
	if tokenErr != nil {
		redirectOAuthError(c, frontendCallback, "login_failed", "service_error", "")
		return
	}
	fragment := url.Values{}
	fragment.Set("error", "unbound_oauth_account")
	fragment.Set("pending_oauth_token", pendingToken)
	fragment.Set("provider", "wechat")
	fragment.Set("provider_subject", subject)
	fragment.Set("provider_identity_key", identityKey)
	fragment.Set("scope", scope)
	fragment.Set("mode", mode)
	appendOAuthFlowFragment(fragment, redirectTo, intent)
	redirectWithFragment(c, frontendCallback, fragment)
	return
}

// WeChatPaymentOAuthStart starts the payment-purpose WeChat OAuth flow and redirects
// the browser to WeChat so the callback can obtain an OpenID.
// GET /api/v1/auth/oauth/wechat/payment/start?payment_type=wxpay&redirect=/payment
func (h *AuthHandler) WeChatPaymentOAuthStart(c *gin.Context) {
	cfg, err := h.getWeChatOAuthConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	paymentType := normalizeWeChatPaymentType(c.Query("payment_type"))
	if paymentType == "" {
		response.BadRequest(c, "Invalid payment type")
		return
	}

	state, err := oauth.GenerateState()
	if err != nil {
		response.ErrorFrom(c, infraerrors.InternalServer("OAUTH_STATE_GEN_FAILED", "failed to generate oauth state").WithCause(err))
		return
	}

	redirectTo := normalizeWeChatPaymentRedirectPath(sanitizeFrontendRedirectPath(c.Query("redirect")))
	if redirectTo == "" {
		redirectTo = weChatPaymentOAuthDefaultRedirectTo
	}
	scope := normalizeWeChatPaymentScope(c.Query("scope"), cfg.Scopes)
	ctxCookie, err := encodeWeChatPaymentOAuthContext(weChatPaymentOAuthContext{
		PaymentType: paymentType,
		Amount:      strings.TrimSpace(c.Query("amount")),
		OrderType:   strings.TrimSpace(c.Query("order_type")),
		PlanID:      parseInt64Default(c.Query("plan_id"), 0),
	})
	if err != nil {
		response.ErrorFrom(c, infraerrors.InternalServer("OAUTH_CONTEXT_ENCODE_FAILED", "failed to encode oauth context").WithCause(err))
		return
	}

	secureCookie := isRequestHTTPS(c)
	weChatPaymentSetCookie(c, weChatPaymentOAuthStateCookieName, encodeCookieValue(state), weChatPaymentOAuthCookieMaxAgeSec, secureCookie)
	weChatPaymentSetCookie(c, weChatPaymentOAuthRedirectCookieName, encodeCookieValue(redirectTo), weChatPaymentOAuthCookieMaxAgeSec, secureCookie)
	weChatPaymentSetCookie(c, weChatPaymentOAuthContextCookieName, encodeCookieValue(ctxCookie), weChatPaymentOAuthCookieMaxAgeSec, secureCookie)
	weChatPaymentSetCookie(c, weChatPaymentOAuthScopeCookieName, encodeCookieValue(scope), weChatPaymentOAuthCookieMaxAgeSec, secureCookie)

	authURL, err := buildWeChatPaymentAuthorizeURL(cfg, scope, state)
	if err != nil {
		response.ErrorFrom(c, infraerrors.InternalServer("OAUTH_BUILD_URL_FAILED", "failed to build oauth authorization url").WithCause(err))
		return
	}

	c.Redirect(http.StatusFound, authURL)
}

// WeChatPaymentOAuthCallback handles the payment-purpose WeChat OAuth callback. It
// exchanges the code for an OpenID and redirects back to the frontend callback
// route with only the fields required to resume create-order.
// GET /api/v1/auth/oauth/wechat/payment/callback?code=...&state=...
func (h *AuthHandler) WeChatPaymentOAuthCallback(c *gin.Context) {
	cfg, cfgErr := h.getWeChatOAuthConfig(c.Request.Context())
	if cfgErr != nil {
		response.ErrorFrom(c, cfgErr)
		return
	}

	frontendCallback := weChatPaymentFrontendCallback(cfg)
	if frontendCallback == "" {
		frontendCallback = weChatPaymentOAuthDefaultFrontendCallback
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
		weChatPaymentClearCookie(c, weChatPaymentOAuthStateCookieName, secureCookie)
		weChatPaymentClearCookie(c, weChatPaymentOAuthRedirectCookieName, secureCookie)
		weChatPaymentClearCookie(c, weChatPaymentOAuthContextCookieName, secureCookie)
		weChatPaymentClearCookie(c, weChatPaymentOAuthScopeCookieName, secureCookie)
	}()

	expectedState, err := readCookieDecoded(c, weChatPaymentOAuthStateCookieName)
	if err != nil || expectedState == "" || state != expectedState {
		redirectOAuthError(c, frontendCallback, "invalid_state", "invalid oauth state", "")
		return
	}

	redirectTo, _ := readCookieDecoded(c, weChatPaymentOAuthRedirectCookieName)
	redirectTo = normalizeWeChatPaymentRedirectPath(sanitizeFrontendRedirectPath(redirectTo))
	if redirectTo == "" {
		redirectTo = weChatPaymentOAuthDefaultRedirectTo
	}

	rawContext, _ := readCookieDecoded(c, weChatPaymentOAuthContextCookieName)
	oauthContext, err := decodeWeChatPaymentOAuthContext(rawContext)
	if err != nil {
		redirectOAuthError(c, frontendCallback, "invalid_context", "invalid oauth context", "")
		return
	}
	if oauthContext.PaymentType == "" {
		oauthContext.PaymentType = payment.TypeWxpay
	}

	scope, _ := readCookieDecoded(c, weChatPaymentOAuthScopeCookieName)
	scope = normalizeWeChatPaymentScope(scope, cfg.Scopes)

	tokenResult, err := wechatPaymentExchangeCode(c.Request.Context(), cfg, code)
	if err != nil {
		description := err.Error()
		var exchangeErr *weChatOAuthExchangeError
		if errors.As(err, &exchangeErr) && exchangeErr != nil {
			log.Printf(
				"[WeChat OAuth] openid exchange failed: status=%d errcode=%q errmsg=%q body=%s",
				exchangeErr.StatusCode,
				exchangeErr.ErrCode,
				exchangeErr.ErrMsg,
				truncateLogValue(exchangeErr.Body, 2048),
			)
		} else {
			log.Printf("[WeChat OAuth] openid exchange failed: %v", err)
		}
		redirectOAuthError(c, frontendCallback, "token_exchange_failed", "failed to exchange oauth code", singleLine(description))
		return
	}
	if tokenResult != nil && strings.TrimSpace(tokenResult.Scope) != "" {
		scope = strings.TrimSpace(tokenResult.Scope)
	}
	if tokenResult == nil || strings.TrimSpace(tokenResult.OpenID) == "" {
		redirectOAuthError(c, frontendCallback, "missing_openid", "missing openid", "")
		return
	}

	fragment := url.Values{}
	fragment.Set("openid", strings.TrimSpace(tokenResult.OpenID))
	fragment.Set("payment_type", oauthContext.PaymentType)
	if oauthContext.Amount != "" {
		fragment.Set("amount", oauthContext.Amount)
	}
	if oauthContext.OrderType != "" {
		fragment.Set("order_type", oauthContext.OrderType)
	}
	if oauthContext.PlanID > 0 {
		fragment.Set("plan_id", strconv.FormatInt(oauthContext.PlanID, 10))
	}
	fragment.Set("redirect", redirectTo)
	fragment.Set("scope", scope)
	redirectWithFragment(c, frontendCallback, fragment)
}

func (h *AuthHandler) getWeChatOAuthConfig(ctx context.Context) (config.WeChatConnectConfig, error) {
	if h != nil && h.settingSvc != nil {
		return h.settingSvc.GetWeChatConnectOAuthConfig(ctx)
	}
	if h == nil || h.cfg == nil {
		return config.WeChatConnectConfig{}, infraerrors.ServiceUnavailable("CONFIG_NOT_READY", "config not loaded")
	}
	if !h.cfg.WeChat.Enabled {
		return config.WeChatConnectConfig{}, infraerrors.NotFound("OAUTH_DISABLED", "oauth login is disabled")
	}
	return h.cfg.WeChat, nil
}

func buildWeChatAuthorizeURL(cfg config.WeChatConnectConfig, mode, scope, state, redirectURI string) (string, error) {
	authorizeURL := weChatOAuthMPAuthorizeURL
	if normalizeWeChatLoginMode(mode) == weChatConnectModeOpen {
		authorizeURL = weChatOAuthOpenAuthorizeURL
	}
	u, err := url.Parse(authorizeURL)
	if err != nil {
		return "", fmt.Errorf("parse authorize url: %w", err)
	}
	q := u.Query()
	q.Set("appid", strings.TrimSpace(cfg.AppID))
	q.Set("redirect_uri", strings.TrimSpace(redirectURI))
	q.Set("response_type", "code")
	q.Set("scope", strings.TrimSpace(scope))
	q.Set("state", strings.TrimSpace(state))
	u.RawQuery = q.Encode()
	u.Fragment = "wechat_redirect"
	return u.String(), nil
}

func buildWeChatPaymentAuthorizeURL(cfg config.WeChatConnectConfig, scope, state string) (string, error) {
	return buildWeChatAuthorizeURL(
		cfg,
		weChatConnectModeMP,
		normalizeWeChatPaymentScope(scope, cfg.Scopes),
		state,
		weChatPaymentRedirectURL(cfg),
	)
}

func normalizeWeChatLoginMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", weChatConnectModeOpen, "qrconnect", "website", "web":
		return weChatConnectModeOpen
	case weChatConnectModeMP, "official", "oa", "oauth2", "public":
		return weChatConnectModeMP
	default:
		return weChatConnectModeOpen
	}
}

func normalizeWeChatLoginScope(raw, mode, fallback string) string {
	mode = normalizeWeChatLoginMode(mode)
	combined := strings.TrimSpace(firstNonEmpty(raw, fallback))
	if mode == weChatConnectModeOpen {
		return "snsapi_login"
	}
	if combined == "" {
		return "snsapi_userinfo"
	}
	for _, part := range strings.FieldsFunc(combined, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
	}) {
		switch strings.TrimSpace(part) {
		case "snsapi_userinfo":
			return "snsapi_userinfo"
		case "snsapi_base":
			return "snsapi_base"
		}
	}
	return "snsapi_userinfo"
}

func normalizeWeChatPaymentType(raw string) string {
	raw = strings.TrimSpace(raw)
	switch raw {
	case payment.TypeWxpay, payment.TypeWxpayDirect:
		return raw
	default:
		return ""
	}
}

func normalizeWeChatPaymentScope(raw, fallback string) string {
	combined := strings.TrimSpace(firstNonEmpty(raw, fallback))
	if combined == "" {
		return "snsapi_base"
	}
	for _, part := range strings.FieldsFunc(combined, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
	}) {
		switch strings.TrimSpace(part) {
		case "snsapi_base":
			return "snsapi_base"
		case "snsapi_userinfo":
			return "snsapi_userinfo"
		}
	}
	return "snsapi_base"
}

var wechatPaymentExchangeCode = func(ctx context.Context, cfg config.WeChatConnectConfig, code string) (*weChatPaymentTokenResult, error) {
	client := req.C().SetTimeout(30 * time.Second)
	resp, err := client.R().
		SetContext(ctx).
		SetHeader("Accept", "application/json").
		SetQueryParam("appid", strings.TrimSpace(cfg.AppID)).
		SetQueryParam("secret", strings.TrimSpace(cfg.AppSecret)).
		SetQueryParam("code", strings.TrimSpace(code)).
		SetQueryParam("grant_type", "authorization_code").
		Get(weChatPaymentOAuthTokenURL)
	if err != nil {
		return nil, fmt.Errorf("request openid: %w", err)
	}

	body := strings.TrimSpace(resp.String())
	if !resp.IsSuccessState() {
		return nil, &weChatOAuthExchangeError{
			StatusCode: resp.StatusCode,
			ErrCode:    strings.TrimSpace(getGJSON(body, "errcode")),
			ErrMsg:     strings.TrimSpace(firstNonEmpty(getGJSON(body, "errmsg"), getGJSON(body, "error_description"))),
			Body:       body,
		}
	}

	if errCode := strings.TrimSpace(getGJSON(body, "errcode")); errCode != "" && errCode != "0" {
		return nil, &weChatOAuthExchangeError{
			StatusCode: resp.StatusCode,
			ErrCode:    errCode,
			ErrMsg:     strings.TrimSpace(getGJSON(body, "errmsg")),
			Body:       body,
		}
	}

	openID := strings.TrimSpace(getGJSON(body, "openid"))
	if openID == "" {
		return nil, &weChatOAuthExchangeError{StatusCode: resp.StatusCode, Body: body}
	}
	return &weChatPaymentTokenResult{
		OpenID: openID,
		Scope:  strings.TrimSpace(getGJSON(body, "scope")),
	}, nil
}

var wechatLoginExchangeCode = func(ctx context.Context, cfg config.WeChatConnectConfig, code string) (*weChatLoginTokenResult, error) {
	client := req.C().SetTimeout(30 * time.Second)
	resp, err := client.R().
		SetContext(ctx).
		SetHeader("Accept", "application/json").
		SetQueryParam("appid", strings.TrimSpace(cfg.AppID)).
		SetQueryParam("secret", strings.TrimSpace(cfg.AppSecret)).
		SetQueryParam("code", strings.TrimSpace(code)).
		SetQueryParam("grant_type", "authorization_code").
		Get(weChatPaymentOAuthTokenURL)
	if err != nil {
		return nil, fmt.Errorf("request access token: %w", err)
	}

	body := strings.TrimSpace(resp.String())
	if !resp.IsSuccessState() {
		return nil, &weChatOAuthExchangeError{
			StatusCode: resp.StatusCode,
			ErrCode:    strings.TrimSpace(getGJSON(body, "errcode")),
			ErrMsg:     strings.TrimSpace(firstNonEmpty(getGJSON(body, "errmsg"), getGJSON(body, "error_description"))),
			Body:       body,
		}
	}

	if errCode := strings.TrimSpace(getGJSON(body, "errcode")); errCode != "" && errCode != "0" {
		return nil, &weChatOAuthExchangeError{
			StatusCode: resp.StatusCode,
			ErrCode:    errCode,
			ErrMsg:     strings.TrimSpace(getGJSON(body, "errmsg")),
			Body:       body,
		}
	}

	openID := strings.TrimSpace(getGJSON(body, "openid"))
	accessToken := strings.TrimSpace(getGJSON(body, "access_token"))
	if openID == "" || accessToken == "" {
		return nil, &weChatOAuthExchangeError{StatusCode: resp.StatusCode, Body: body}
	}

	return &weChatLoginTokenResult{
		AccessToken: accessToken,
		OpenID:      openID,
		Scope:       strings.TrimSpace(getGJSON(body, "scope")),
		UnionID:     strings.TrimSpace(getGJSON(body, "unionid")),
	}, nil
}

var wechatLoginFetchUserInfo = func(ctx context.Context, tokenResult *weChatLoginTokenResult) (*weChatLoginUserInfo, error) {
	if tokenResult == nil {
		return nil, errors.New("token result is nil")
	}
	accessToken := strings.TrimSpace(tokenResult.AccessToken)
	openID := strings.TrimSpace(tokenResult.OpenID)
	if accessToken == "" || openID == "" {
		return nil, errors.New("missing access token or openid")
	}

	client := req.C().SetTimeout(30 * time.Second)
	resp, err := client.R().
		SetContext(ctx).
		SetHeader("Accept", "application/json").
		SetQueryParam("access_token", accessToken).
		SetQueryParam("openid", openID).
		SetQueryParam("lang", "zh_CN").
		Get(weChatOAuthUserInfoURL)
	if err != nil {
		return nil, fmt.Errorf("request userinfo: %w", err)
	}

	body := strings.TrimSpace(resp.String())
	if !resp.IsSuccessState() {
		return nil, &weChatOAuthExchangeError{
			StatusCode: resp.StatusCode,
			ErrCode:    strings.TrimSpace(getGJSON(body, "errcode")),
			ErrMsg:     strings.TrimSpace(firstNonEmpty(getGJSON(body, "errmsg"), getGJSON(body, "error_description"))),
			Body:       body,
		}
	}

	if errCode := strings.TrimSpace(getGJSON(body, "errcode")); errCode != "" && errCode != "0" {
		return nil, &weChatOAuthExchangeError{
			StatusCode: resp.StatusCode,
			ErrCode:    errCode,
			ErrMsg:     strings.TrimSpace(getGJSON(body, "errmsg")),
			Body:       body,
		}
	}

	return &weChatLoginUserInfo{
		OpenID:    strings.TrimSpace(getGJSON(body, "openid")),
		UnionID:   strings.TrimSpace(getGJSON(body, "unionid")),
		Nickname:  strings.TrimSpace(getGJSON(body, "nickname")),
		AvatarURL: strings.TrimSpace(getGJSON(body, "headimgurl")),
	}, nil
}

func weChatSetCookie(c *gin.Context, name, value string, maxAgeSec int, secure bool) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     weChatOAuthCookiePath,
		MaxAge:   maxAgeSec,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func weChatClearCookie(c *gin.Context, name string, secure bool) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     weChatOAuthCookiePath,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func weChatPaymentSetCookie(c *gin.Context, name, value string, maxAgeSec int, secure bool) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     weChatPaymentOAuthCookiePath,
		MaxAge:   maxAgeSec,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func weChatPaymentClearCookie(c *gin.Context, name string, secure bool) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     weChatPaymentOAuthCookiePath,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func encodeWeChatPaymentOAuthContext(payload weChatPaymentOAuthContext) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func decodeWeChatPaymentOAuthContext(raw string) (weChatPaymentOAuthContext, error) {
	var payload weChatPaymentOAuthContext
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return payload, nil
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return weChatPaymentOAuthContext{}, err
	}
	payload.PaymentType = normalizeWeChatPaymentType(payload.PaymentType)
	return payload, nil
}

func parseInt64Default(raw string, fallback int64) int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return fallback
	}
	return value
}

func normalizeWeChatPaymentRedirectPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if path == "/payment" {
		return weChatPaymentOAuthDefaultRedirectTo
	}
	if strings.HasPrefix(path, "/payment?") {
		return weChatPaymentOAuthDefaultRedirectTo + strings.TrimPrefix(path, "/payment")
	}
	return path
}

func weChatPaymentRedirectURL(cfg config.WeChatConnectConfig) string {
	redirectURI := strings.TrimSpace(cfg.RedirectURL)
	if redirectURI == "" {
		return ""
	}

	u, err := url.Parse(redirectURI)
	if err != nil {
		return redirectURI
	}

	path := strings.TrimRight(u.Path, "/")
	switch {
	case strings.HasSuffix(path, "/oauth/wechat/callback"):
		u.Path = strings.TrimSuffix(path, "/oauth/wechat/callback") + "/oauth/wechat/payment/callback"
	case strings.HasSuffix(path, "/oauth/wechat/payment/callback"):
	default:
		return redirectURI
	}
	return u.String()
}

func weChatPaymentFrontendCallback(cfg config.WeChatConnectConfig) string {
	redirectPath := strings.TrimSpace(cfg.FrontendRedirectURL)
	if redirectPath == "" {
		return ""
	}
	switch {
	case redirectPath == "/auth/wechat/callback":
		return "/auth/wechat/payment/callback"
	case strings.HasSuffix(redirectPath, "/auth/wechat/callback"):
		return strings.TrimSuffix(redirectPath, "/auth/wechat/callback") + "/auth/wechat/payment/callback"
	default:
		return redirectPath
	}
}

func weChatIdentityKey(mode, appID, subject string, hasUnionID bool) string {
	kind := "openid"
	if hasUnionID {
		kind = "unionid"
	}
	return strings.Join([]string{
		"wechat",
		kind,
		normalizeWeChatLoginMode(mode),
		strings.TrimSpace(appID),
		strings.TrimSpace(subject),
	}, "\x1f")
}

func weChatLoginAvatarURL(info *weChatLoginUserInfo) string {
	if info == nil {
		return ""
	}
	return strings.TrimSpace(info.AvatarURL)
}

func weChatSyntheticEmailFromIdentityKey(identityKey string) string {
	identityKey = strings.TrimSpace(identityKey)
	if identityKey == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(identityKey))
	return "wechat-" + hex.EncodeToString(sum[:16]) + service.WeChatConnectSyntheticEmailDomain
}

func weChatFallbackUsername(subject string) string {
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return "wechat_user"
	}
	sum := sha256.Sum256([]byte(subject))
	return "wechat_" + hex.EncodeToString(sum[:])[:12]
}

func weChatLoginNickname(userInfo *weChatLoginUserInfo) string {
	if userInfo == nil {
		return ""
	}
	return strings.TrimSpace(userInfo.Nickname)
}
