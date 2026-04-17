package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type oauthIntentUserRepoStub struct {
	wechatOAuthUserRepoStub
	users      map[int64]*service.User
	identities map[string]*service.UserExternalIdentity
}

func (s *oauthIntentUserRepoStub) GetByID(ctx context.Context, id int64) (*service.User, error) {
	if user, ok := s.users[id]; ok {
		clone := *user
		return &clone, nil
	}
	return nil, service.ErrUserNotFound
}

func (s *oauthIntentUserRepoStub) FindExternalIdentity(ctx context.Context, provider, providerUserID string) (*service.UserExternalIdentity, error) {
	if identity, ok := s.identities[provider+":"+providerUserID]; ok {
		clone := *identity
		return &clone, nil
	}
	return nil, service.ErrExternalIdentityNotFound
}

func newLinuxDoOAuthHandlerForIntentTest(t *testing.T, repo *oauthIntentUserRepoStub, providerServer *httptest.Server) *AuthHandler {
	t.Helper()

	cfg := &config.Config{
		JWT: config.JWTConfig{
			Secret:                 "linuxdo-intent-test-secret",
			ExpireHour:             1,
			RefreshTokenExpireDays: 7,
		},
		Default: config.DefaultConfig{
			UserBalance:     0,
			UserConcurrency: 1,
		},
		LinuxDo: config.LinuxDoConnectConfig{
			Enabled:             true,
			ClientID:            "cid",
			ClientSecret:        "secret",
			AuthorizeURL:        providerServer.URL + "/authorize",
			TokenURL:            providerServer.URL + "/token",
			UserInfoURL:         providerServer.URL + "/userinfo",
			RedirectURL:         "https://example.com/api/v1/auth/oauth/linuxdo/callback",
			FrontendRedirectURL: "/auth/linuxdo/callback",
		},
	}
	settingSvc := service.NewSettingService(&settingHandlerRepoStub{values: map[string]string{}}, cfg)
	authSvc := service.NewAuthService(nil, repo, nil, wechatOAuthRefreshTokenCacheStub{}, cfg, settingSvc, nil, nil, nil, nil, nil)
	return NewAuthHandler(cfg, authSvc, nil, settingSvc, nil, nil, nil)
}

func newLinuxDoProviderServer(t *testing.T, payload map[string]any) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"access-token","token_type":"Bearer","expires_in":3600}`))
		case "/userinfo":
			w.Header().Set("Content-Type", "application/json")
			require.NoError(t, json.NewEncoder(w).Encode(payload))
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
}

func startLinuxDoOAuthForTest(t *testing.T, handler *AuthHandler, rawQuery string) (*httptest.ResponseRecorder, string) {
	t.Helper()

	startRec := httptest.NewRecorder()
	startCtx, _ := gin.CreateTestContext(startRec)
	startCtx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/auth/oauth/linuxdo/start?"+rawQuery, nil)
	handler.LinuxDoOAuthStart(startCtx)
	require.Equal(t, http.StatusFound, startRec.Code)

	startURL, err := url.Parse(startRec.Header().Get("Location"))
	require.NoError(t, err)
	state := startURL.Query().Get("state")
	require.NotEmpty(t, state)
	return startRec, state
}

func TestLinuxDoOAuthCallback_BindIntentDoesNotSwitchSessionForBoundIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)

	providerServer := newLinuxDoProviderServer(t, map[string]any{
		"id":       "linuxdo-user-1",
		"email":    "owner@example.com",
		"username": "linuxdo_owner",
	})
	defer providerServer.Close()

	repo := &oauthIntentUserRepoStub{
		users: map[int64]*service.User{
			7: {ID: 7, Email: "owner@example.com", Role: service.RoleUser, Status: service.StatusActive},
		},
		identities: map[string]*service.UserExternalIdentity{
			"linuxdo:linuxdo-user-1": {
				UserID:         7,
				Provider:       service.ExternalIdentityProviderLinuxDo,
				ProviderUserID: "linuxdo-user-1",
			},
		},
	}
	authHandler := newLinuxDoOAuthHandlerForIntentTest(t, repo, providerServer)

	startRec, state := startLinuxDoOAuthForTest(t, authHandler, "redirect=%2Fprofile&intent=bind")

	callbackRec := httptest.NewRecorder()
	callbackCtx, _ := gin.CreateTestContext(callbackRec)
	callbackReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oauth/linuxdo/callback?code=code-123&state="+url.QueryEscape(state), nil)
	for _, cookie := range startRec.Result().Cookies() {
		callbackReq.AddCookie(cookie)
	}
	callbackCtx.Request = callbackReq

	authHandler.LinuxDoOAuthCallback(callbackCtx)
	require.Equal(t, http.StatusFound, callbackRec.Code)

	location := callbackRec.Header().Get("Location")
	redirectURL, err := url.Parse(location)
	require.NoError(t, err)
	fragment, err := url.ParseQuery(redirectURL.Fragment)
	require.NoError(t, err)
	require.Equal(t, "external_identity_already_bound", fragment.Get("error"))
	require.Equal(t, "bind", fragment.Get("intent"))
	require.Equal(t, "/profile", fragment.Get("redirect"))
	require.Empty(t, fragment.Get("access_token"))
	require.Empty(t, fragment.Get("refresh_token"))
}

func TestLinuxDoOAuthCallback_RejectsInactiveBoundUserBeforeIssuingTokens(t *testing.T) {
	gin.SetMode(gin.TestMode)

	providerServer := newLinuxDoProviderServer(t, map[string]any{
		"id":       "linuxdo-user-2",
		"email":    "owner@example.com",
		"username": "linuxdo_owner",
	})
	defer providerServer.Close()

	repo := &oauthIntentUserRepoStub{
		users: map[int64]*service.User{
			8: {ID: 8, Email: "owner@example.com", Role: service.RoleUser, Status: service.StatusDisabled},
		},
		identities: map[string]*service.UserExternalIdentity{
			"linuxdo:linuxdo-user-2": {
				UserID:         8,
				Provider:       service.ExternalIdentityProviderLinuxDo,
				ProviderUserID: "linuxdo-user-2",
			},
		},
	}
	authHandler := newLinuxDoOAuthHandlerForIntentTest(t, repo, providerServer)

	startRec, state := startLinuxDoOAuthForTest(t, authHandler, "redirect=%2Fdashboard")

	callbackRec := httptest.NewRecorder()
	callbackCtx, _ := gin.CreateTestContext(callbackRec)
	callbackReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oauth/linuxdo/callback?code=code-123&state="+url.QueryEscape(state), nil)
	for _, cookie := range startRec.Result().Cookies() {
		callbackReq.AddCookie(cookie)
	}
	callbackCtx.Request = callbackReq

	authHandler.LinuxDoOAuthCallback(callbackCtx)
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

func TestOIDCOAuthStart_BindIntentRedirectsUnsupported(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		OIDC: config.OIDCConnectConfig{
			Enabled:             true,
			AuthorizeURL:        "https://issuer.example.com/auth",
			RedirectURL:         "https://example.com/api/v1/auth/oauth/oidc/callback",
			FrontendRedirectURL: "/auth/oidc/callback",
		},
	}
	authHandler := NewAuthHandler(cfg, nil, nil, nil, nil, nil, nil)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/auth/oauth/oidc/start?redirect=%2Fprofile&intent=bind", nil)

	authHandler.OIDCOAuthStart(ctx)
	require.Equal(t, http.StatusFound, rec.Code)

	location := rec.Header().Get("Location")
	redirectURL, err := url.Parse(location)
	require.NoError(t, err)
	require.Equal(t, "/auth/oidc/callback", redirectURL.Path)
	fragment, err := url.ParseQuery(redirectURL.Fragment)
	require.NoError(t, err)
	require.Equal(t, "binding_not_supported", fragment.Get("error"))
	require.Equal(t, "bind", fragment.Get("intent"))
	require.Equal(t, "/profile", fragment.Get("redirect"))
}
