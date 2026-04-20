package handler

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type oauthCallbackIdentityRepoStub struct {
	*pendingAuthHandlerUserRepoStub
	identities     map[string]*repository.AuthIdentityRecord
	identitiesByID map[int64]*repository.AuthIdentityRecord
	channels       map[string]*repository.AuthIdentityChannelRecord
}

func newOAuthCallbackIdentityRepoStub(t *testing.T) *oauthCallbackIdentityRepoStub {
	t.Helper()

	base := newPendingAuthHandlerUserRepoStub()
	passwordHash, err := service.NewAuthService(nil, nil, nil, nil, &config.Config{
		JWT: config.JWTConfig{Secret: "oauth-callback-hash"},
	}, nil, nil, nil, nil, nil, nil).HashPassword("password-123")
	require.NoError(t, err)

	user := &service.User{
		ID:           7,
		Email:        "owner@example.com",
		PasswordHash: passwordHash,
		Role:         service.RoleUser,
		Status:       service.StatusActive,
	}
	base.users[user.ID] = user
	base.usersByMail[user.Email] = user

	return &oauthCallbackIdentityRepoStub{
		pendingAuthHandlerUserRepoStub: base,
		identities:                     make(map[string]*repository.AuthIdentityRecord),
		identitiesByID:                 make(map[int64]*repository.AuthIdentityRecord),
		channels:                       make(map[string]*repository.AuthIdentityChannelRecord),
	}
}

func (s *oauthCallbackIdentityRepoStub) FindAuthIdentity(_ context.Context, providerType, providerKey, providerSubject string) (*repository.AuthIdentityRecord, error) {
	if identity, ok := s.identities[providerType+"\x1f"+providerKey+"\x1f"+providerSubject]; ok {
		copied := *identity
		return &copied, nil
	}
	return nil, nil
}

func (s *oauthCallbackIdentityRepoStub) FindAuthIdentityChannel(_ context.Context, providerType, providerKey, channel, channelAppID, channelSubject string) (*repository.AuthIdentityChannelRecord, error) {
	if record, ok := s.channels[providerType+"\x1f"+providerKey+"\x1f"+channel+"\x1f"+channelAppID+"\x1f"+channelSubject]; ok {
		copied := *record
		return &copied, nil
	}
	return nil, nil
}

func (s *oauthCallbackIdentityRepoStub) GetAuthIdentityByID(_ context.Context, id int64) (*repository.AuthIdentityRecord, error) {
	if identity, ok := s.identitiesByID[id]; ok {
		copied := *identity
		return &copied, nil
	}
	return nil, nil
}

func (s *oauthCallbackIdentityRepoStub) CreatePendingAuthSession(_ context.Context, input service.PendingAuthSessionInput) (*service.PendingAuthSessionRecord, error) {
	s.nextSession++
	id := string(rune('a' + s.nextSession - 1))
	session := &service.PendingAuthSessionRecord{
		ID:              id,
		Intent:          input.Intent,
		ProviderType:    input.ProviderType,
		ProviderKey:     input.ProviderKey,
		ProviderSubject: input.ProviderSubject,
		Metadata:        cloneOAuthMetadataMap(input.Metadata),
		RedirectTo:      input.RedirectTo,
		ExpiresAt:       time.Now().Add(10 * time.Minute),
	}
	if input.TargetUserID != nil {
		target := *input.TargetUserID
		session.TargetUserID = &target
	}
	s.sessions[id] = clonePendingAuthSession(session)
	return clonePendingAuthSession(session), nil
}

func (s *oauthCallbackIdentityRepoStub) bindIdentity(userID int64, providerType, providerKey, providerSubject string) *repository.AuthIdentityRecord {
	identityID := int64(len(s.identitiesByID) + 1)
	record := &repository.AuthIdentityRecord{
		ID:              identityID,
		UserID:          userID,
		ProviderType:    providerType,
		ProviderKey:     providerKey,
		ProviderSubject: providerSubject,
	}
	s.identities[providerType+"\x1f"+providerKey+"\x1f"+providerSubject] = record
	s.identitiesByID[identityID] = record
	return record
}

func (s *oauthCallbackIdentityRepoStub) bindChannel(identityID int64, providerType, providerKey, channel, channelAppID, channelSubject string) {
	s.channels[providerType+"\x1f"+providerKey+"\x1f"+channel+"\x1f"+channelAppID+"\x1f"+channelSubject] = &repository.AuthIdentityChannelRecord{
		ID:             int64(len(s.channels) + 1),
		IdentityID:     identityID,
		ProviderType:   providerType,
		ProviderKey:    providerKey,
		Channel:        channel,
		ChannelAppID:   channelAppID,
		ChannelSubject: channelSubject,
	}
}

func (s *oauthCallbackIdentityRepoStub) BindPendingAuthIdentity(_ context.Context, session *service.PendingAuthSessionRecord, userID int64) error {
	if session == nil {
		return nil
	}

	providerType := session.ProviderType
	providerKey := session.ProviderKey
	if providerKey == "" {
		providerKey = providerType
	}

	if providerType != "wechat" {
		if existing, ok := s.identities[s.identityKey(providerType, providerKey, session.ProviderSubject)]; ok {
			existing.UserID = userID
			return nil
		}
		s.bindIdentity(userID, providerType, providerKey, session.ProviderSubject)
		return nil
	}

	unionid := oauthMetadataString(session.Metadata, "unionid")
	openid := oauthMetadataString(session.Metadata, "openid")
	channel := oauthMetadataString(session.Metadata, "channel")
	appid := oauthMetadataString(session.Metadata, "appid")

	primarySubject := firstNonEmpty(unionid, session.ProviderSubject, openid)
	if primarySubject == "" {
		return nil
	}

	primaryIdentity := s.identities[s.identityKey("wechat", providerKey, primarySubject)]
	var channelIdentity *repository.AuthIdentityRecord
	if channel != "" && appid != "" && openid != "" {
		if channelRecord := s.channels[s.channelKey("wechat", providerKey, channel, appid, openid)]; channelRecord != nil {
			channelIdentity = s.identitiesByID[channelRecord.IdentityID]
		}
	}

	targetIdentity := primaryIdentity
	switch {
	case targetIdentity != nil && channelIdentity != nil && targetIdentity.ID != channelIdentity.ID:
		for key, record := range s.channels {
			if record.IdentityID == channelIdentity.ID {
				record.IdentityID = targetIdentity.ID
				s.channels[key] = record
			}
		}
		delete(s.identities, s.identityKey(channelIdentity.ProviderType, channelIdentity.ProviderKey, channelIdentity.ProviderSubject))
		delete(s.identitiesByID, channelIdentity.ID)
	case targetIdentity == nil && channelIdentity != nil:
		targetIdentity = channelIdentity
	case targetIdentity == nil:
		targetIdentity = s.bindIdentity(userID, "wechat", providerKey, primarySubject)
	}

	if targetIdentity == nil {
		return nil
	}

	targetIdentity.UserID = userID
	if targetIdentity.ProviderSubject != primarySubject {
		delete(s.identities, s.identityKey(targetIdentity.ProviderType, targetIdentity.ProviderKey, targetIdentity.ProviderSubject))
		targetIdentity.ProviderSubject = primarySubject
		s.identities[s.identityKey(targetIdentity.ProviderType, targetIdentity.ProviderKey, targetIdentity.ProviderSubject)] = targetIdentity
	}

	if channel != "" && appid != "" && openid != "" {
		channelKey := s.channelKey("wechat", providerKey, channel, appid, openid)
		record, ok := s.channels[channelKey]
		if !ok {
			record = &repository.AuthIdentityChannelRecord{
				ID:             int64(len(s.channels) + 1),
				ProviderType:   "wechat",
				ProviderKey:    providerKey,
				Channel:        channel,
				ChannelAppID:   appid,
				ChannelSubject: openid,
			}
		}
		record.IdentityID = targetIdentity.ID
		s.channels[channelKey] = record
	}

	s.identitiesByID[targetIdentity.ID] = targetIdentity
	s.identities[s.identityKey(targetIdentity.ProviderType, targetIdentity.ProviderKey, targetIdentity.ProviderSubject)] = targetIdentity
	return nil
}

func (s *oauthCallbackIdentityRepoStub) identityKey(providerType, providerKey, providerSubject string) string {
	return providerType + "\x1f" + providerKey + "\x1f" + providerSubject
}

func (s *oauthCallbackIdentityRepoStub) channelKey(providerType, providerKey, channel, channelAppID, channelSubject string) string {
	return providerType + "\x1f" + providerKey + "\x1f" + channel + "\x1f" + channelAppID + "\x1f" + channelSubject
}

func newOAuthCallbackHandlerForTest(t *testing.T) (*AuthHandler, *service.AuthService, *oauthCallbackIdentityRepoStub) {
	t.Helper()

	repo := newOAuthCallbackIdentityRepoStub(t)
	cfg := &config.Config{
		JWT: config.JWTConfig{
			Secret:                   "oauth-callback-secret",
			ExpireHour:               1,
			AccessTokenExpireMinutes: 60,
			RefreshTokenExpireDays:   7,
		},
	}

	authSvc := service.NewAuthService(nil, repo, nil, pendingAuthRefreshCacheStub{}, cfg, nil, nil, nil, nil, nil, nil)
	userSvc := service.NewUserService(repo, nil, nil, nil)
	handler := NewAuthHandler(cfg, authSvc, userSvc, nil, nil, nil, nil)
	return handler, authSvc, repo
}

func parseRedirectFragment(t *testing.T, location string) url.Values {
	t.Helper()
	parsed, err := url.Parse(location)
	require.NoError(t, err)
	fragment, err := url.ParseQuery(parsed.Fragment)
	require.NoError(t, err)
	return fragment
}

func cookieValueByName(t *testing.T, rec *httptest.ResponseRecorder, name string) string {
	t.Helper()
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == name {
			return cookie.Value
		}
	}
	t.Fatalf("cookie %q not found", name)
	return ""
}

func TestLinuxDoOAuthStart_PersistsBindIntentCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _, _ := newOAuthCallbackHandlerForTest(t)
	handler.cfg.LinuxDo = config.LinuxDoConnectConfig{
		Enabled:             true,
		AuthorizeURL:        "https://connect.linux.do/oauth2/authorize",
		ClientID:            "cid",
		RedirectURL:         "https://api.example.com/api/v1/auth/oauth/linuxdo/callback",
		FrontendRedirectURL: "/auth/linuxdo/callback",
	}

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/auth/oauth/linuxdo/start?intent=bind&redirect=/profile", nil)

	handler.LinuxDoOAuthStart(ctx)

	require.Equal(t, http.StatusFound, rec.Code)
	raw := cookieValueByName(t, rec, linuxDoOAuthIntentCookieName)
	decoded, err := decodeCookieValue(raw)
	require.NoError(t, err)
	require.Equal(t, service.PendingAuthIntentBindCurrentUser, decoded)
}

func TestOIDCOAuthStart_DefaultsIntentCookieToLogin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _, _ := newOAuthCallbackHandlerForTest(t)
	handler.cfg.OIDC = config.OIDCConnectConfig{
		Enabled:             true,
		AuthorizeURL:        "https://issuer.example.com/auth",
		ClientID:            "cid",
		RedirectURL:         "https://api.example.com/api/v1/auth/oauth/oidc/callback",
		FrontendRedirectURL: "/auth/oidc/callback",
		Scopes:              "openid profile email",
	}

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/auth/oauth/oidc/start?redirect=/dashboard", nil)

	handler.OIDCOAuthStart(ctx)

	require.Equal(t, http.StatusFound, rec.Code)
	raw := cookieValueByName(t, rec, oidcOAuthIntentCookieName)
	decoded, err := decodeCookieValue(raw)
	require.NoError(t, err)
	require.Equal(t, service.PendingAuthIntentLogin, decoded)
}

func TestNormalizedOAuthIntentFromCookie_FallsBackToBindRedirect(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/auth/oauth/linuxdo/callback", nil)

	require.Equal(
		t,
		service.PendingAuthIntentBindCurrentUser,
		normalizedOAuthIntentFromCookie(ctx, linuxDoOAuthIntentCookieName, "/profile?oauth_intent=bind"),
	)
}

func TestNormalizedOAuthIntentFromCookie_DefaultsToLoginWithoutBindRedirect(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/auth/oauth/linuxdo/callback", nil)

	require.Equal(
		t,
		service.PendingAuthIntentLogin,
		normalizedOAuthIntentFromCookie(ctx, linuxDoOAuthIntentCookieName, "/profile"),
	)
}

func TestCompleteOAuthCallback_WeChatLateUnionIDRekeysChannelBoundIdentityOnBind(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, authSvc, repo := newOAuthCallbackHandlerForTest(t)

	identity := repo.bindIdentity(7, "wechat", wechatOAuthProviderKey, "open-user-merge-1")
	repo.bindChannel(identity.ID, "wechat", wechatOAuthProviderKey, "open", "wx-open-app", "open-user-merge-1")

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 7})
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/auth/oauth/wechat/callback", nil)

	handler.completeOAuthCallback(ctx, "/auth/wechat/callback", "/profile", oauthCallbackIdentity{
		Provider:        "wechat",
		Intent:          service.PendingAuthIntentBindCurrentUser,
		ProviderType:    "wechat",
		ProviderKey:     wechatOAuthProviderKey,
		ProviderSubject: "union-merge-1",
		Metadata: map[string]any{
			"unionid": "union-merge-1",
			"openid":  "open-user-merge-1",
			"channel": "open",
			"appid":   "wx-open-app",
		},
	})

	require.Equal(t, http.StatusFound, rec.Code)
	fragment := parseRedirectFragment(t, rec.Header().Get("Location"))
	require.Equal(t, "pending_session", fragment.Get("auth_result"))

	_, err := authSvc.CompletePendingAuthSessionBind(context.Background(), fragment.Get("pending_auth_token"), 7)
	require.NoError(t, err)

	mergedIdentity, err := repo.FindAuthIdentity(context.Background(), "wechat", wechatOAuthProviderKey, "union-merge-1")
	require.NoError(t, err)
	require.NotNil(t, mergedIdentity)
	require.Equal(t, int64(7), mergedIdentity.UserID)

	legacyIdentity, err := repo.FindAuthIdentity(context.Background(), "wechat", wechatOAuthProviderKey, "open-user-merge-1")
	require.NoError(t, err)
	require.Nil(t, legacyIdentity)

	channelRecord, err := repo.FindAuthIdentityChannel(context.Background(), "wechat", wechatOAuthProviderKey, "open", "wx-open-app", "open-user-merge-1")
	require.NoError(t, err)
	require.NotNil(t, channelRecord)
	require.Equal(t, mergedIdentity.ID, channelRecord.IdentityID)
}

func buildOIDCTestKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return key
}
