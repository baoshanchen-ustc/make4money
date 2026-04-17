package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestBuildWeChatAuthorizeURLUsesOpenModeDefaults(t *testing.T) {
	u, err := buildWeChatAuthorizeURL(config.WeChatConnectConfig{
		AppID: "wx123456",
	}, "open", "snsapi_login", "state-123", "https://example.com/api/v1/auth/oauth/wechat/callback")
	require.NoError(t, err)
	require.Contains(t, u, "connect/qrconnect")
	require.Contains(t, u, "scope=snsapi_login")
	require.Contains(t, u, "redirect_uri=https%3A%2F%2Fexample.com%2Fapi%2Fv1%2Fauth%2Foauth%2Fwechat%2Fcallback")
}

func TestBuildWeChatPaymentAuthorizeURL(t *testing.T) {
	u, err := buildWeChatPaymentAuthorizeURL(config.WeChatConnectConfig{
		AppID:       "wx123456",
		RedirectURL: "https://example.com/api/v1/auth/oauth/wechat/callback",
	}, "snsapi_base", "state-123")
	require.NoError(t, err)
	require.Contains(t, u, "appid=wx123456")
	require.Contains(t, u, "scope=snsapi_base")
	require.Contains(t, u, "state=state-123")
	require.Contains(t, u, "redirect_uri=https%3A%2F%2Fexample.com%2Fapi%2Fv1%2Fauth%2Foauth%2Fwechat%2Fpayment%2Fcallback")
	require.True(t, strings.HasSuffix(u, "#wechat_redirect"))
}

func TestWeChatOAuthCallbackRedirectsToInvitationFlowWithProviderIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)

	origExchange := wechatLoginExchangeCode
	origUserInfo := wechatLoginFetchUserInfo
	t.Cleanup(func() {
		wechatLoginExchangeCode = origExchange
		wechatLoginFetchUserInfo = origUserInfo
	})
	wechatLoginExchangeCode = func(ctx context.Context, cfg config.WeChatConnectConfig, code string) (*weChatLoginTokenResult, error) {
		require.Equal(t, "code-123", code)
		return &weChatLoginTokenResult{
			AccessToken: "access-token",
			OpenID:      "openid-123",
			UnionID:     "unionid-456",
			Scope:       "snsapi_login",
		}, nil
	}
	wechatLoginFetchUserInfo = func(ctx context.Context, tokenResult *weChatLoginTokenResult) (*weChatLoginUserInfo, error) {
		return &weChatLoginUserInfo{
			OpenID:   tokenResult.OpenID,
			UnionID:  tokenResult.UnionID,
			Nickname: "Alice",
		}, nil
	}

	settingSvc := service.NewSettingService(&settingHandlerRepoStub{values: map[string]string{
		service.SettingKeyRegistrationEnabled:   "true",
		service.SettingKeyInvitationCodeEnabled: "true",
	}}, &config.Config{
		JWT: config.JWTConfig{
			Secret:                 "test-secret-wechat-oauth",
			ExpireHour:             1,
			RefreshTokenExpireDays: 7,
		},
		Default: config.DefaultConfig{
			UserBalance:     0,
			UserConcurrency: 1,
		},
		WeChat: config.WeChatConnectConfig{
			Enabled:             true,
			AppID:               "wx123456",
			AppSecret:           "wechat-secret",
			Mode:                "open",
			Scopes:              "snsapi_login",
			RedirectURL:         "https://example.com/api/v1/auth/oauth/wechat/callback",
			FrontendRedirectURL: "/auth/wechat/callback",
		},
	})
	authSvc := service.NewAuthService(
		nil,
		&wechatOAuthUserRepoStub{},
		nil,
		wechatOAuthRefreshTokenCacheStub{},
		&config.Config{
			JWT: config.JWTConfig{
				Secret:                 "test-secret-wechat-oauth",
				ExpireHour:             1,
				RefreshTokenExpireDays: 7,
			},
			Default: config.DefaultConfig{
				UserBalance:     0,
				UserConcurrency: 1,
			},
		},
		settingSvc,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	authHandler := NewAuthHandler(&config.Config{}, authSvc, nil, settingSvc, nil, nil, nil)

	startRec := httptest.NewRecorder()
	startCtx, _ := gin.CreateTestContext(startRec)
	startCtx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/auth/oauth/wechat/start?redirect=%2Fdashboard", nil)
	authHandler.WeChatOAuthStart(startCtx)
	require.Equal(t, http.StatusFound, startRec.Code)

	startURL, err := url.Parse(startRec.Header().Get("Location"))
	require.NoError(t, err)
	state := startURL.Query().Get("state")
	require.NotEmpty(t, state)

	callbackRec := httptest.NewRecorder()
	callbackCtx, _ := gin.CreateTestContext(callbackRec)
	callbackReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oauth/wechat/callback?code=code-123&state="+url.QueryEscape(state), nil)
	for _, cookie := range startRec.Result().Cookies() {
		callbackReq.AddCookie(cookie)
	}
	callbackCtx.Request = callbackReq

	authHandler.WeChatOAuthCallback(callbackCtx)
	require.Equal(t, http.StatusFound, callbackRec.Code)

	location := callbackRec.Header().Get("Location")
	redirectURL, err := url.Parse(location)
	require.NoError(t, err)
	require.Equal(t, "/auth/wechat/callback", redirectURL.Path)

	fragment, err := url.ParseQuery(redirectURL.Fragment)
	require.NoError(t, err)
	require.Equal(t, "unbound_oauth_account", fragment.Get("error"))
	require.Equal(t, "wechat", fragment.Get("provider"))
	require.Equal(t, "unionid-456", fragment.Get("provider_subject"))
	require.NotEmpty(t, fragment.Get("provider_identity_key"))
	require.NotEmpty(t, fragment.Get("pending_oauth_token"))
	require.Equal(t, "/dashboard", fragment.Get("redirect"))
}

func TestWeChatPaymentOAuthCallbackCarriesOpenIDBackToFrontend(t *testing.T) {
	gin.SetMode(gin.TestMode)

	origExchange := wechatPaymentExchangeCode
	t.Cleanup(func() {
		wechatPaymentExchangeCode = origExchange
	})
	wechatPaymentExchangeCode = func(ctx context.Context, cfg config.WeChatConnectConfig, code string) (*weChatPaymentTokenResult, error) {
		require.Equal(t, "code-123", code)
		require.Equal(t, "wx123456", cfg.AppID)
		return &weChatPaymentTokenResult{
			OpenID: "openid-abc",
			Scope:  "snsapi_base",
		}, nil
	}

	settingSvc := service.NewSettingService(&settingHandlerRepoStub{values: map[string]string{}}, &config.Config{
		WeChat: config.WeChatConnectConfig{
			Enabled:             true,
			AppID:               "wx123456",
			AppSecret:           "wechat-secret",
			Scopes:              "snsapi_login snsapi_base",
			RedirectURL:         "https://example.com/api/v1/auth/oauth/wechat/callback",
			FrontendRedirectURL: "/auth/wechat/callback",
		},
	})
	authHandler := NewAuthHandler(&config.Config{}, nil, nil, settingSvc, nil, nil, nil)

	startRec := httptest.NewRecorder()
	startCtx, _ := gin.CreateTestContext(startRec)
	startCtx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/auth/oauth/wechat/payment/start?payment_type=wxpay&amount=12.5&order_type=balance&plan_id=9&redirect=%2Fpurchase%3Ffrom%3Dwechat", nil)
	authHandler.WeChatPaymentOAuthStart(startCtx)
	require.Equal(t, http.StatusFound, startRec.Code)

	startLocation := startRec.Header().Get("Location")
	startURL, err := url.Parse(startLocation)
	require.NoError(t, err)
	state := startURL.Query().Get("state")
	require.NotEmpty(t, state)

	callbackRec := httptest.NewRecorder()
	callbackCtx, _ := gin.CreateTestContext(callbackRec)
	callbackReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oauth/wechat/payment/callback?code=code-123&state="+url.QueryEscape(state), nil)
	for _, cookie := range startRec.Result().Cookies() {
		callbackReq.AddCookie(cookie)
	}
	callbackCtx.Request = callbackReq
	authHandler.WeChatPaymentOAuthCallback(callbackCtx)
	require.Equal(t, http.StatusFound, callbackRec.Code)

	location := callbackRec.Header().Get("Location")
	redirectURL, err := url.Parse(location)
	require.NoError(t, err)
	require.Equal(t, "/auth/wechat/payment/callback", redirectURL.Path)

	fragment, err := url.ParseQuery(redirectURL.Fragment)
	require.NoError(t, err)
	require.Equal(t, "openid-abc", fragment.Get("openid"))
	require.Equal(t, "snsapi_base", fragment.Get("scope"))
	require.Equal(t, "wxpay", fragment.Get("payment_type"))
	require.Equal(t, "12.5", fragment.Get("amount"))
	require.Equal(t, "balance", fragment.Get("order_type"))
	require.Equal(t, "9", fragment.Get("plan_id"))
	require.Equal(t, "/purchase?from=wechat", fragment.Get("redirect"))
	require.Empty(t, fragment.Get("access_token"))
}

func TestWeChatPaymentOAuthCallbackRejectsInvalidState(t *testing.T) {
	gin.SetMode(gin.TestMode)

	settingSvc := service.NewSettingService(&settingHandlerRepoStub{values: map[string]string{}}, &config.Config{
		WeChat: config.WeChatConnectConfig{
			Enabled:             true,
			AppID:               "wx123456",
			AppSecret:           "wechat-secret",
			RedirectURL:         "https://example.com/api/v1/auth/oauth/wechat/callback",
			FrontendRedirectURL: "/auth/wechat/callback",
		},
	})
	authHandler := NewAuthHandler(&config.Config{}, nil, nil, settingSvc, nil, nil, nil)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oauth/wechat/payment/callback?code=code-123&state=wrong", nil)
	req.AddCookie(&http.Cookie{
		Name:  weChatPaymentOAuthStateCookieName,
		Value: encodeCookieValue("expected"),
		Path:  weChatPaymentOAuthCookiePath,
	})
	c.Request = req

	authHandler.WeChatPaymentOAuthCallback(c)
	require.Equal(t, http.StatusFound, rec.Code)

	location := rec.Header().Get("Location")
	redirectURL, err := url.Parse(location)
	require.NoError(t, err)
	fragment, err := url.ParseQuery(redirectURL.Fragment)
	require.NoError(t, err)
	require.Equal(t, "invalid_state", fragment.Get("error"))
}

func TestWeChatPaymentOAuthStartStoresContextCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)

	settingSvc := service.NewSettingService(&settingHandlerRepoStub{values: map[string]string{}}, &config.Config{
		WeChat: config.WeChatConnectConfig{
			Enabled:             true,
			AppID:               "wx123456",
			AppSecret:           "wechat-secret",
			RedirectURL:         "https://example.com/api/v1/auth/oauth/wechat/callback",
			FrontendRedirectURL: "/auth/wechat/callback",
		},
	})
	authHandler := NewAuthHandler(&config.Config{}, nil, nil, settingSvc, nil, nil, nil)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/auth/oauth/wechat/payment/start?payment_type=wxpay_direct&amount=88&order_type=subscription&plan_id=12&redirect=%2Fpurchase", nil)

	authHandler.WeChatPaymentOAuthStart(c)
	require.Equal(t, http.StatusFound, rec.Code)

	cookies := rec.Result().Cookies()
	var rawCtx string
	for _, cookie := range cookies {
		if cookie.Name == weChatPaymentOAuthContextCookieName {
			rawCtx = cookie.Value
			break
		}
	}
	require.NotEmpty(t, rawCtx)

	decoded, err := decodeCookieValue(rawCtx)
	require.NoError(t, err)

	var payload weChatPaymentOAuthContext
	require.NoError(t, json.Unmarshal([]byte(decoded), &payload))
	require.Equal(t, "wxpay_direct", payload.PaymentType)
	require.Equal(t, "88", payload.Amount)
	require.Equal(t, "subscription", payload.OrderType)
	require.EqualValues(t, 12, payload.PlanID)
}

func TestWeChatOAuthCallbackRejectsInactiveBoundUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	origExchange := wechatLoginExchangeCode
	origUserInfo := wechatLoginFetchUserInfo
	t.Cleanup(func() {
		wechatLoginExchangeCode = origExchange
		wechatLoginFetchUserInfo = origUserInfo
	})
	wechatLoginExchangeCode = func(ctx context.Context, cfg config.WeChatConnectConfig, code string) (*weChatLoginTokenResult, error) {
		return &weChatLoginTokenResult{
			AccessToken: "access-token",
			OpenID:      "openid-123",
			UnionID:     "unionid-456",
			Scope:       "snsapi_userinfo",
		}, nil
	}
	wechatLoginFetchUserInfo = func(ctx context.Context, tokenResult *weChatLoginTokenResult) (*weChatLoginUserInfo, error) {
		return &weChatLoginUserInfo{
			OpenID:   tokenResult.OpenID,
			UnionID:  tokenResult.UnionID,
			Nickname: "Alice",
		}, nil
	}

	repo := &wechatOAuthUserRepoStub{
		usersByID: map[int64]*service.User{
			9: {
				ID:     9,
				Email:  "disabled@example.com",
				Status: service.StatusDisabled,
				Role:   service.RoleUser,
			},
		},
		externalIdentityByProviderSubject: map[string]*service.UserExternalIdentity{
			"wechat|unionid-456": {
				UserID:         9,
				Provider:       service.ExternalIdentityProviderWeChat,
				ProviderUserID: "unionid-456",
			},
		},
	}
	settingSvc := service.NewSettingService(&settingHandlerRepoStub{values: map[string]string{
		service.SettingKeyRegistrationEnabled: "true",
	}}, &config.Config{
		WeChat: config.WeChatConnectConfig{
			Enabled:             true,
			AppID:               "wx123456",
			AppSecret:           "wechat-secret",
			Mode:                "open",
			Scopes:              "snsapi_userinfo",
			RedirectURL:         "https://example.com/api/v1/auth/oauth/wechat/callback",
			FrontendRedirectURL: "/auth/wechat/callback",
		},
		JWT: config.JWTConfig{
			Secret:                 "test-secret-wechat-oauth",
			ExpireHour:             1,
			RefreshTokenExpireDays: 7,
		},
	})
	authSvc := service.NewAuthService(
		nil,
		repo,
		nil,
		wechatOAuthRefreshTokenCacheStub{},
		&config.Config{
			JWT: config.JWTConfig{
				Secret:                 "test-secret-wechat-oauth",
				ExpireHour:             1,
				RefreshTokenExpireDays: 7,
			},
		},
		settingSvc,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	authHandler := NewAuthHandler(&config.Config{}, authSvc, nil, settingSvc, nil, nil, nil)

	startRec := httptest.NewRecorder()
	startCtx, _ := gin.CreateTestContext(startRec)
	startCtx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/auth/oauth/wechat/start?redirect=%2Fdashboard", nil)
	authHandler.WeChatOAuthStart(startCtx)
	require.Equal(t, http.StatusFound, startRec.Code)

	startURL, err := url.Parse(startRec.Header().Get("Location"))
	require.NoError(t, err)
	state := startURL.Query().Get("state")
	require.NotEmpty(t, state)

	callbackRec := httptest.NewRecorder()
	callbackCtx, _ := gin.CreateTestContext(callbackRec)
	callbackReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oauth/wechat/callback?code=code-123&state="+url.QueryEscape(state), nil)
	for _, cookie := range startRec.Result().Cookies() {
		callbackReq.AddCookie(cookie)
	}
	callbackCtx.Request = callbackReq
	authHandler.WeChatOAuthCallback(callbackCtx)

	require.Equal(t, http.StatusFound, callbackRec.Code)
	location := callbackRec.Header().Get("Location")
	redirectURL, err := url.Parse(location)
	require.NoError(t, err)
	fragment, err := url.ParseQuery(redirectURL.Fragment)
	require.NoError(t, err)
	require.Equal(t, "login_failed", fragment.Get("error"))
	require.Equal(t, "USER_NOT_ACTIVE", fragment.Get("error_message"))
	require.Empty(t, fragment.Get("access_token"))
}

type wechatOAuthUserRepoStub struct {
	usersByID                         map[int64]*service.User
	usersByEmail                      map[string]*service.User
	externalIdentityByProviderSubject map[string]*service.UserExternalIdentity
}

func (s *wechatOAuthUserRepoStub) Create(ctx context.Context, user *service.User) error {
	return nil
}

func (s *wechatOAuthUserRepoStub) GetByID(ctx context.Context, id int64) (*service.User, error) {
	if s.usersByID != nil {
		if user, ok := s.usersByID[id]; ok {
			clone := *user
			return &clone, nil
		}
	}
	return nil, service.ErrUserNotFound
}

func (s *wechatOAuthUserRepoStub) GetByEmail(ctx context.Context, email string) (*service.User, error) {
	if s.usersByEmail != nil {
		if user, ok := s.usersByEmail[email]; ok {
			clone := *user
			return &clone, nil
		}
	}
	return nil, service.ErrUserNotFound
}

func (s *wechatOAuthUserRepoStub) GetFirstAdmin(ctx context.Context) (*service.User, error) {
	return nil, service.ErrUserNotFound
}

func (s *wechatOAuthUserRepoStub) Update(ctx context.Context, user *service.User) error {
	return nil
}

func (s *wechatOAuthUserRepoStub) Delete(ctx context.Context, id int64) error {
	return nil
}

func (s *wechatOAuthUserRepoStub) List(ctx context.Context, params pagination.PaginationParams) ([]service.User, *pagination.PaginationResult, error) {
	return nil, nil, errors.New("not implemented")
}

func (s *wechatOAuthUserRepoStub) ListWithFilters(ctx context.Context, params pagination.PaginationParams, filters service.UserListFilters) ([]service.User, *pagination.PaginationResult, error) {
	return nil, nil, errors.New("not implemented")
}

func (s *wechatOAuthUserRepoStub) UpdateBalance(ctx context.Context, id int64, amount float64) error {
	return nil
}

func (s *wechatOAuthUserRepoStub) DeductBalance(ctx context.Context, id int64, amount float64) error {
	return nil
}

func (s *wechatOAuthUserRepoStub) UpdateConcurrency(ctx context.Context, id int64, amount int) error {
	return nil
}

func (s *wechatOAuthUserRepoStub) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	return false, nil
}

func (s *wechatOAuthUserRepoStub) RemoveGroupFromAllowedGroups(ctx context.Context, groupID int64) (int64, error) {
	return 0, nil
}

func (s *wechatOAuthUserRepoStub) AddGroupToAllowedGroups(ctx context.Context, userID int64, groupID int64) error {
	return nil
}

func (s *wechatOAuthUserRepoStub) RemoveGroupFromUserAllowedGroups(ctx context.Context, userID int64, groupID int64) error {
	return nil
}

func (s *wechatOAuthUserRepoStub) UpdateTotpSecret(ctx context.Context, userID int64, encryptedSecret *string) error {
	return nil
}

func (s *wechatOAuthUserRepoStub) EnableTotp(ctx context.Context, userID int64) error {
	return nil
}

func (s *wechatOAuthUserRepoStub) DisableTotp(ctx context.Context, userID int64) error {
	return nil
}

func (s *wechatOAuthUserRepoStub) ListExternalIdentities(ctx context.Context, userID int64) ([]service.UserExternalIdentity, error) {
	return nil, nil
}

func (s *wechatOAuthUserRepoStub) UpsertExternalIdentity(ctx context.Context, userID int64, input service.UpsertUserExternalIdentityInput) (*service.UserExternalIdentity, error) {
	return &service.UserExternalIdentity{
		UserID:         userID,
		Provider:       input.Provider,
		ProviderUserID: input.ProviderUserID,
		DisplayName:    input.DisplayName,
	}, nil
}

func (s *wechatOAuthUserRepoStub) FindExternalIdentity(ctx context.Context, provider, providerUserID string) (*service.UserExternalIdentity, error) {
	if s.externalIdentityByProviderSubject != nil {
		if identity, ok := s.externalIdentityByProviderSubject[provider+"|"+providerUserID]; ok {
			clone := *identity
			return &clone, nil
		}
	}
	return nil, service.ErrExternalIdentityNotFound
}

func (s *wechatOAuthUserRepoStub) DeleteExternalIdentity(ctx context.Context, userID int64, provider string) error {
	return nil
}

func (s *wechatOAuthUserRepoStub) GetAvatar(ctx context.Context, userID int64) (*service.UserAvatar, error) {
	return nil, service.ErrUserAvatarNotFound
}

func (s *wechatOAuthUserRepoStub) UpsertAvatar(ctx context.Context, userID int64, input service.UpsertUserAvatarInput) (*service.UserAvatar, error) {
	return &service.UserAvatar{
		UserID:          userID,
		StorageProvider: input.StorageProvider,
		StorageKey:      input.StorageKey,
		URL:             input.URL,
		ContentType:     input.ContentType,
		ByteSize:        input.ByteSize,
	}, nil
}

func (s *wechatOAuthUserRepoStub) DeleteAvatar(ctx context.Context, userID int64) error {
	return nil
}

type wechatOAuthRefreshTokenCacheStub struct{}

func (wechatOAuthRefreshTokenCacheStub) StoreRefreshToken(ctx context.Context, tokenHash string, data *service.RefreshTokenData, ttl time.Duration) error {
	return nil
}

func (wechatOAuthRefreshTokenCacheStub) GetRefreshToken(ctx context.Context, tokenHash string) (*service.RefreshTokenData, error) {
	return nil, service.ErrRefreshTokenNotFound
}

func (wechatOAuthRefreshTokenCacheStub) DeleteRefreshToken(ctx context.Context, tokenHash string) error {
	return nil
}

func (wechatOAuthRefreshTokenCacheStub) DeleteUserRefreshTokens(ctx context.Context, userID int64) error {
	return nil
}

func (wechatOAuthRefreshTokenCacheStub) DeleteTokenFamily(ctx context.Context, familyID string) error {
	return nil
}

func (wechatOAuthRefreshTokenCacheStub) AddToUserTokenSet(ctx context.Context, userID int64, tokenHash string, ttl time.Duration) error {
	return nil
}

func (wechatOAuthRefreshTokenCacheStub) AddToFamilyTokenSet(ctx context.Context, familyID string, tokenHash string, ttl time.Duration) error {
	return nil
}

func (wechatOAuthRefreshTokenCacheStub) GetUserTokenHashes(ctx context.Context, userID int64) ([]string, error) {
	return nil, nil
}

func (wechatOAuthRefreshTokenCacheStub) GetFamilyTokenHashes(ctx context.Context, familyID string) ([]string, error) {
	return nil, nil
}

func (wechatOAuthRefreshTokenCacheStub) IsTokenInFamily(ctx context.Context, familyID string, tokenHash string) (bool, error) {
	return false, nil
}
