package handler

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

type oidcCompleteRegistrationRepoStub struct {
	*oauthCallbackIdentityRepoStub
	nextUserID int64
}

func newOIDCCompleteRegistrationHandlerForTest(t *testing.T) (*AuthHandler, *service.AuthService, *oidcCompleteRegistrationRepoStub) {
	t.Helper()

	repo := &oidcCompleteRegistrationRepoStub{
		oauthCallbackIdentityRepoStub: newOAuthCallbackIdentityRepoStub(t),
		nextUserID:                    100,
	}
	cfg := &config.Config{
		JWT: config.JWTConfig{
			Secret:                   "oidc-complete-registration-secret",
			ExpireHour:               1,
			AccessTokenExpireMinutes: 60,
			RefreshTokenExpireDays:   7,
		},
	}
	settingSvc := service.NewSettingService(&pendingAuthSettingRepoStub{
		values: map[string]string{
			service.SettingKeyRegistrationEnabled:   "true",
			service.SettingKeyInvitationCodeEnabled: "false",
		},
	}, cfg)

	authSvc := service.NewAuthService(nil, repo, nil, pendingAuthRefreshCacheStub{}, cfg, settingSvc, nil, nil, nil, nil, nil)
	userSvc := service.NewUserService(repo, nil, nil, nil)
	handler := NewAuthHandler(cfg, authSvc, userSvc, settingSvc, nil, nil, nil)
	return handler, authSvc, repo
}

func (s *oidcCompleteRegistrationRepoStub) Create(_ context.Context, user *service.User) error {
	if _, exists := s.usersByMail[user.Email]; exists {
		return service.ErrEmailExists
	}
	s.nextUserID++
	user.ID = s.nextUserID
	copied := *user
	s.users[copied.ID] = &copied
	s.usersByMail[copied.Email] = &copied
	return nil
}

func (s *oidcCompleteRegistrationRepoStub) BindPendingAuthIdentity(_ context.Context, session *service.PendingAuthSessionRecord, userID int64) error {
	s.bindIdentity(userID, session.ProviderType, session.ProviderKey, session.ProviderSubject)
	return nil
}

func TestOIDCSyntheticEmailStableAndDistinct(t *testing.T) {
	k1 := oidcIdentityKey("https://issuer.example.com", "subject-a")
	k2 := oidcIdentityKey("https://issuer.example.com", "subject-b")

	e1 := oidcSyntheticEmailFromIdentityKey(k1)
	e1Again := oidcSyntheticEmailFromIdentityKey(k1)
	e2 := oidcSyntheticEmailFromIdentityKey(k2)

	require.Equal(t, e1, e1Again)
	require.NotEqual(t, e1, e2)
	require.Contains(t, e1, "@oidc-connect.invalid")
}

func TestOIDCSelectLoginEmailAlwaysUsesSyntheticEmail(t *testing.T) {
	identityKey := oidcIdentityKey("https://issuer.example.com", "subject-a")

	email := oidcSelectLoginEmail(identityKey)
	require.Contains(t, email, "@oidc-connect.invalid")
	require.Equal(t, oidcSyntheticEmailFromIdentityKey(identityKey), email)
}

func TestBuildOIDCAuthorizeURLIncludesNonceAndPKCE(t *testing.T) {
	cfg := config.OIDCConnectConfig{
		AuthorizeURL: "https://issuer.example.com/auth",
		ClientID:     "cid",
		Scopes:       "openid email profile",
		UsePKCE:      true,
	}

	u, err := buildOIDCAuthorizeURL(cfg, "state123", "nonce123", "challenge123", "https://app.example.com/callback")
	require.NoError(t, err)
	require.Contains(t, u, "nonce=nonce123")
	require.Contains(t, u, "code_challenge=challenge123")
	require.Contains(t, u, "code_challenge_method=S256")
	require.Contains(t, u, "scope=openid+email+profile")
}

func TestOIDCParseAndValidateIDToken(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	kid := "kid-1"
	jwks := oidcJWKSet{Keys: []oidcJWK{buildRSAJWK(kid, &priv.PublicKey)}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewEncoder(w).Encode(jwks))
	}))
	defer srv.Close()

	now := time.Now()
	claims := oidcIDTokenClaims{
		Nonce: "nonce-ok",
		Azp:   "client-1",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "https://issuer.example.com",
			Subject:   "subject-1",
			Audience:  jwt.ClaimStrings{"client-1", "another-aud"},
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now.Add(-30 * time.Second)),
			ExpiresAt: jwt.NewNumericDate(now.Add(5 * time.Minute)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = kid
	signed, err := tok.SignedString(priv)
	require.NoError(t, err)

	cfg := config.OIDCConnectConfig{
		ClientID:           "client-1",
		IssuerURL:          "https://issuer.example.com",
		JWKSURL:            srv.URL,
		AllowedSigningAlgs: "RS256",
		ClockSkewSeconds:   120,
	}

	parsed, err := oidcParseAndValidateIDToken(context.Background(), cfg, signed, "nonce-ok")
	require.NoError(t, err)
	require.Equal(t, "subject-1", parsed.Subject)
	require.Equal(t, "https://issuer.example.com", parsed.Issuer)

	_, err = oidcParseAndValidateIDToken(context.Background(), cfg, signed, "bad-nonce")
	require.Error(t, err)
}

func buildRSAJWK(kid string, pub *rsa.PublicKey) oidcJWK {
	n := base64.RawURLEncoding.EncodeToString(pub.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes())
	return oidcJWK{
		Kty: "RSA",
		Kid: kid,
		Use: "sig",
		Alg: "RS256",
		N:   n,
		E:   e,
	}
}

func TestOIDCOAuthCallback_BoundLoginRedirectsWithTokenPair(t *testing.T) {
	gin.SetMode(gin.TestMode)

	priv := buildOIDCTestKey(t)
	kid := "kid-oidc"
	issuer := "https://issuer.example.com"
	now := time.Now()
	claims := oidcIDTokenClaims{
		Nonce:             "nonce-1",
		PreferredUsername: "alice",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   "subject-1",
			Audience:  jwt.ClaimStrings{"cid"},
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now.Add(-30 * time.Second)),
			ExpiresAt: jwt.NewNumericDate(now.Add(5 * time.Minute)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = kid
	signed, err := tok.SignedString(priv)
	require.NoError(t, err)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			_, _ = w.Write([]byte(`{"access_token":"provider-token","token_type":"Bearer","expires_in":3600,"id_token":"` + signed + `"}`))
		case "/userinfo":
			_, _ = w.Write([]byte(`{"sub":"subject-1","preferred_username":"alice","email":"alice@example.com","email_verified":false}`))
		case "/jwks":
			require.NoError(t, json.NewEncoder(w).Encode(oidcJWKSet{Keys: []oidcJWK{buildRSAJWK(kid, &priv.PublicKey)}}))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	handler, _, repo := newOAuthCallbackHandlerForTest(t)
	handler.cfg.OIDC = config.OIDCConnectConfig{
		Enabled:              true,
		ClientID:             "cid",
		ClientSecret:         "secret",
		IssuerURL:            issuer,
		AuthorizeURL:         issuer + "/auth",
		TokenURL:             srv.URL + "/token",
		UserInfoURL:          srv.URL + "/userinfo",
		JWKSURL:              srv.URL + "/jwks",
		AllowedSigningAlgs:   "RS256",
		ValidateIDToken:      true,
		RequireEmailVerified: false,
		RedirectURL:          "https://api.example.com/api/v1/auth/oauth/oidc/callback",
		FrontendRedirectURL:  "/auth/oidc/callback",
		Scopes:               "openid profile email",
	}
	repo.bindIdentity(7, "oidc", issuer, "subject-1")

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oauth/oidc/callback?code=ok&state=state-1", nil)
	req.AddCookie(&http.Cookie{Name: oidcOAuthStateCookieName, Value: encodeCookieValue("state-1")})
	req.AddCookie(&http.Cookie{Name: oidcOAuthRedirectCookie, Value: encodeCookieValue("/dashboard")})
	req.AddCookie(&http.Cookie{Name: oidcOAuthIntentCookieName, Value: encodeCookieValue(service.PendingAuthIntentLogin)})
	req.AddCookie(&http.Cookie{Name: oidcOAuthNonceCookie, Value: encodeCookieValue("nonce-1")})
	ctx.Request = req

	handler.OIDCOAuthCallback(ctx)

	require.Equal(t, http.StatusFound, rec.Code)
	fragment := parseRedirectFragment(t, rec.Header().Get("Location"))
	require.NotEmpty(t, fragment.Get("access_token"))
	require.NotEmpty(t, fragment.Get("refresh_token"))
	require.Equal(t, "oidc", fragment.Get("provider"))
	require.Equal(t, service.PendingAuthIntentLogin, fragment.Get("intent"))
	require.Equal(t, "/dashboard", fragment.Get("redirect"))
	require.Empty(t, fragment.Get("pending_auth_token"))
	require.Empty(t, repo.sessions)
}

func TestOIDCOAuthCallback_UnboundLoginRedirectsWithPendingSession(t *testing.T) {
	gin.SetMode(gin.TestMode)

	priv := buildOIDCTestKey(t)
	kid := "kid-oidc"
	issuer := "https://issuer.example.com"
	now := time.Now()
	claims := oidcIDTokenClaims{
		Nonce:             "nonce-2",
		Email:             "fresh@example.com",
		PreferredUsername: "fresh-user",
		Name:              "Fresh User",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   "subject-2",
			Audience:  jwt.ClaimStrings{"cid"},
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now.Add(-30 * time.Second)),
			ExpiresAt: jwt.NewNumericDate(now.Add(5 * time.Minute)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = kid
	signed, err := tok.SignedString(priv)
	require.NoError(t, err)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			_, _ = w.Write([]byte(`{"access_token":"provider-token","token_type":"Bearer","expires_in":3600,"id_token":"` + signed + `"}`))
		case "/userinfo":
			_, _ = w.Write([]byte(`{"sub":"subject-2","preferred_username":"fresh-user","name":"Fresh User","picture":"https://example.com/oidc-avatar.png","email":"fresh@example.com","email_verified":false}`))
		case "/jwks":
			require.NoError(t, json.NewEncoder(w).Encode(oidcJWKSet{Keys: []oidcJWK{buildRSAJWK(kid, &priv.PublicKey)}}))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	handler, authSvc, repo := newOAuthCallbackHandlerForTest(t)
	handler.cfg.OIDC = config.OIDCConnectConfig{
		Enabled:              true,
		ClientID:             "cid",
		ClientSecret:         "secret",
		IssuerURL:            issuer,
		AuthorizeURL:         issuer + "/auth",
		TokenURL:             srv.URL + "/token",
		UserInfoURL:          srv.URL + "/userinfo",
		JWKSURL:              srv.URL + "/jwks",
		AllowedSigningAlgs:   "RS256",
		ValidateIDToken:      true,
		RequireEmailVerified: false,
		RedirectURL:          "https://api.example.com/api/v1/auth/oauth/oidc/callback",
		FrontendRedirectURL:  "/auth/oidc/callback",
		Scopes:               "openid profile email",
	}

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oauth/oidc/callback?code=ok&state=state-2", nil)
	req.AddCookie(&http.Cookie{Name: oidcOAuthStateCookieName, Value: encodeCookieValue("state-2")})
	req.AddCookie(&http.Cookie{Name: oidcOAuthRedirectCookie, Value: encodeCookieValue("/register")})
	req.AddCookie(&http.Cookie{Name: oidcOAuthIntentCookieName, Value: encodeCookieValue(service.PendingAuthIntentLogin)})
	req.AddCookie(&http.Cookie{Name: oidcOAuthNonceCookie, Value: encodeCookieValue("nonce-2")})
	ctx.Request = req

	handler.OIDCOAuthCallback(ctx)

	require.Equal(t, http.StatusFound, rec.Code)
	fragment := parseRedirectFragment(t, rec.Header().Get("Location"))
	require.Equal(t, "pending_session", fragment.Get("auth_result"))
	require.NotEmpty(t, fragment.Get("pending_auth_token"))
	require.NotEmpty(t, fragment.Get("pending_oauth_token"))
	require.Equal(t, "oidc", fragment.Get("provider"))
	require.Equal(t, service.PendingAuthIntentLogin, fragment.Get("intent"))
	require.Equal(t, "/register", fragment.Get("redirect"))
	require.Equal(t, "true", fragment.Get("adoption_required"))
	require.Equal(t, "Fresh User", fragment.Get("suggested_display_name"))
	require.Equal(t, "https://example.com/oidc-avatar.png", fragment.Get("suggested_avatar_url"))

	session, err := authSvc.GetPendingAuthSessionForProgress(context.Background(), fragment.Get("pending_auth_token"), nil)
	require.NoError(t, err)
	require.Equal(t, service.PendingAuthIntentLogin, session.Intent)
	require.Equal(t, "oidc", session.ProviderType)
	require.Equal(t, issuer, session.ProviderKey)
	require.Equal(t, "subject-2", session.ProviderSubject)
	require.Equal(t, "Fresh User", session.Metadata["suggested_display_name"])
	require.Equal(t, "https://example.com/oidc-avatar.png", session.Metadata["suggested_avatar_url"])
	require.Len(t, repo.sessions, 1)
}

func TestOIDCOAuthCallback_RejectsUnverifiedEmailWhenRequired(t *testing.T) {
	gin.SetMode(gin.TestMode)

	priv := buildOIDCTestKey(t)
	kid := "kid-oidc"
	issuer := "https://issuer.example.com"
	now := time.Now()
	claims := oidcIDTokenClaims{
		Nonce:             "nonce-verify",
		PreferredUsername: "fresh-user",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   "subject-verify",
			Audience:  jwt.ClaimStrings{"cid"},
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now.Add(-30 * time.Second)),
			ExpiresAt: jwt.NewNumericDate(now.Add(5 * time.Minute)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = kid
	signed, err := tok.SignedString(priv)
	require.NoError(t, err)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			_, _ = w.Write([]byte(`{"access_token":"provider-token","token_type":"Bearer","expires_in":3600,"id_token":"` + signed + `"}`))
		case "/userinfo":
			_, _ = w.Write([]byte(`{"sub":"subject-verify","preferred_username":"fresh-user","email_verified":false}`))
		case "/jwks":
			require.NoError(t, json.NewEncoder(w).Encode(oidcJWKSet{Keys: []oidcJWK{buildRSAJWK(kid, &priv.PublicKey)}}))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	handler, authSvc, repo := newOAuthCallbackHandlerForTest(t)
	handler.cfg.OIDC = config.OIDCConnectConfig{
		Enabled:              true,
		ClientID:             "cid",
		ClientSecret:         "secret",
		IssuerURL:            issuer,
		AuthorizeURL:         issuer + "/auth",
		TokenURL:             srv.URL + "/token",
		UserInfoURL:          srv.URL + "/userinfo",
		JWKSURL:              srv.URL + "/jwks",
		AllowedSigningAlgs:   "RS256",
		ValidateIDToken:      true,
		RequireEmailVerified: true,
		RedirectURL:          "https://api.example.com/api/v1/auth/oauth/oidc/callback",
		FrontendRedirectURL:  "/auth/oidc/callback",
		Scopes:               "openid profile email",
	}

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oauth/oidc/callback?code=ok&state=state-verify", nil)
	req.AddCookie(&http.Cookie{Name: oidcOAuthStateCookieName, Value: encodeCookieValue("state-verify")})
	req.AddCookie(&http.Cookie{Name: oidcOAuthRedirectCookie, Value: encodeCookieValue("/register")})
	req.AddCookie(&http.Cookie{Name: oidcOAuthIntentCookieName, Value: encodeCookieValue(service.PendingAuthIntentLogin)})
	req.AddCookie(&http.Cookie{Name: oidcOAuthNonceCookie, Value: encodeCookieValue("nonce-verify")})
	ctx.Request = req

	handler.OIDCOAuthCallback(ctx)

	require.Equal(t, http.StatusFound, rec.Code)
	fragment := parseRedirectFragment(t, rec.Header().Get("Location"))
	require.Equal(t, "email_not_verified", fragment.Get("error"))
	require.Empty(t, fragment.Get("pending_auth_token"))
	require.Empty(t, fragment.Get("access_token"))
	require.Empty(t, repo.sessions)
	_, err = authSvc.GetPendingAuthSessionForProgress(context.Background(), fragment.Get("pending_auth_token"), nil)
	require.Error(t, err)
}

func TestCompleteOIDCOAuthRegistration_AcceptsPendingAuthTokenAndBindsIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler, authSvc, repo := newOIDCCompleteRegistrationHandlerForTest(t)
	pendingAuthToken, err := authSvc.CreatePendingAuthSession(context.Background(), service.PendingAuthSessionInput{
		Intent:          service.PendingAuthIntentLogin,
		ProviderType:    "oidc",
		ProviderKey:     "https://issuer.example.com",
		ProviderSubject: "subject-3",
		Metadata: map[string]any{
			"compat_email":    "oidc-user@example.com",
			"compat_username": "oidc-user",
		},
	})
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/oauth/oidc/complete-registration", bytes.NewBufferString(`{
		"pending_auth_token":"`+pendingAuthToken+`",
		"pending_oauth_token":"`+pendingAuthToken+`",
		"invitation_code":"invite-1"
	}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	handler.CompleteOIDCOAuthRegistration(ctx)

	require.Equal(t, http.StatusOK, rec.Code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.NotEmpty(t, payload["access_token"])
	require.NotEmpty(t, payload["refresh_token"])

	user, err := repo.GetByEmail(context.Background(), "oidc-user@example.com")
	require.NoError(t, err)

	identity, err := repo.FindAuthIdentity(context.Background(), "oidc", "https://issuer.example.com", "subject-3")
	require.NoError(t, err)
	require.NotNil(t, identity)
	require.Equal(t, user.ID, identity.UserID)

	_, err = authSvc.GetPendingAuthSessionForProgress(context.Background(), pendingAuthToken, nil)
	require.ErrorIs(t, err, service.ErrInvalidToken)
}
