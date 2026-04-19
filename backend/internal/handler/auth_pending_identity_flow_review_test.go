package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/require"
)

type handlerTurnstileVerifierStub struct {
	called    int
	lastToken string
	result    *service.TurnstileVerifyResponse
	err       error
}

func (s *handlerTurnstileVerifierStub) VerifyToken(_ context.Context, _ string, token, _ string) (*service.TurnstileVerifyResponse, error) {
	s.called++
	s.lastToken = token
	if s.err != nil {
		return nil, s.err
	}
	if s.result != nil {
		return s.result, nil
	}
	return &service.TurnstileVerifyResponse{Success: true}, nil
}

func newPendingAuthHandlerForReview(
	t *testing.T,
	settingValues map[string]string,
	intent string,
	provider string,
	configureUser func(*service.User),
) (*AuthHandler, *service.AuthService, *pendingAuthHandlerUserRepoStub, *pendingAuthTotpCacheStub, *handlerTurnstileVerifierStub, string) {
	t.Helper()

	repo := newPendingAuthHandlerUserRepoStub()
	passwordHash, err := service.NewAuthService(nil, nil, nil, nil, &config.Config{JWT: config.JWTConfig{Secret: "hash-only"}}, nil, nil, nil, nil, nil, nil).HashPassword("password-123")
	require.NoError(t, err)

	secret := "JBSWY3DPEHPK3PXP"
	user := &service.User{
		ID:                  7,
		Email:               "owner@example.com",
		PasswordHash:        passwordHash,
		Role:                service.RoleUser,
		Status:              service.StatusActive,
		TotpSecretEncrypted: &secret,
	}
	if configureUser != nil {
		configureUser(user)
	}
	repo.users[user.ID] = user
	repo.usersByMail[user.Email] = user

	cfg := &config.Config{
		JWT: config.JWTConfig{
			Secret:                   "review-pending-auth-secret",
			ExpireHour:               1,
			AccessTokenExpireMinutes: 60,
			RefreshTokenExpireDays:   7,
		},
		Server: config.ServerConfig{
			Mode: "release",
		},
		Turnstile: config.TurnstileConfig{
			Required: false,
		},
	}
	service.ResetBackendModeCacheForTest()
	t.Cleanup(service.ResetBackendModeCacheForTest)

	values := map[string]string{
		service.SettingKeyRegistrationEnabled: "true",
		service.SettingKeyEmailVerifyEnabled:  "false",
		service.SettingKeyTurnstileEnabled:    "false",
		service.SettingKeyTurnstileSecretKey:  "",
		service.SettingKeyBackendModeEnabled:  "false",
		service.SettingKeyTotpEnabled:         "false",
	}
	for key, value := range settingValues {
		values[key] = value
	}

	settingSvc := service.NewSettingService(&pendingAuthSettingRepoStub{values: values}, cfg)
	turnstileVerifier := &handlerTurnstileVerifierStub{}
	turnstileSvc := service.NewTurnstileService(settingSvc, turnstileVerifier)
	authSvc := service.NewAuthService(nil, repo, nil, pendingAuthRefreshCacheStub{}, cfg, settingSvc, nil, turnstileSvc, nil, nil, nil)
	userSvc := service.NewUserService(repo, nil, nil, nil)
	totpCache := newPendingAuthTotpCacheStub()
	totpSvc := service.NewTotpService(repo, passthroughEncryptor{}, totpCache, settingSvc, nil, nil)
	handler := NewAuthHandler(cfg, authSvc, userSvc, settingSvc, nil, nil, totpSvc)

	input := service.PendingAuthSessionInput{
		Intent:          intent,
		ProviderType:    provider,
		ProviderKey:     provider + "-main",
		ProviderSubject: provider + "-subject-1",
	}
	if intent == service.PendingAuthIntentAdoptExistingUserByEmail {
		input.TargetUserID = &user.ID
	}

	sessionToken, err := authSvc.CreatePendingAuthSession(context.Background(), input)
	require.NoError(t, err)
	session, err := authSvc.GetPendingAuthSessionForProgress(context.Background(), sessionToken, nil)
	require.NoError(t, err)
	if intent == service.PendingAuthIntentAdoptExistingUserByEmail {
		now := time.Now()
		session.EmailVerifiedAt = &now
		require.NoError(t, repo.UpdatePendingAuthSession(context.Background(), session))
	}

	return handler, authSvc, repo, totpCache, turnstileVerifier, sessionToken
}

func TestBindPendingOAuthLogin_RequiresTurnstileWhenEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler, _, _, _, verifier, pendingToken := newPendingAuthHandlerForReview(t, map[string]string{
		service.SettingKeyTurnstileEnabled:   "true",
		service.SettingKeyTurnstileSecretKey: "turnstile-secret",
	}, service.PendingAuthIntentLogin, "oidc", nil)

	body := bytes.NewBufferString(`{"pending_auth_token":"` + pendingToken + `","email":"owner@example.com","password":"password-123"}`)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/oauth/oidc/bind-login", body)
	ctx.Request.Header.Set("Content-Type", "application/json")

	handler.BindOIDCOAuthLogin(ctx)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Equal(t, 0, verifier.called)

	var resp struct {
		Reason string `json:"reason"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, "TURNSTILE_VERIFICATION_FAILED", resp.Reason)
}

func TestBindPendingOAuthLogin_BlocksBackendModeWithoutConsumingPendingSession(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler, authSvc, _, _, _, pendingToken := newPendingAuthHandlerForReview(t, map[string]string{
		service.SettingKeyBackendModeEnabled: "true",
	}, service.PendingAuthIntentLogin, "oidc", func(user *service.User) {
		user.TotpEnabled = false
	})

	body := bytes.NewBufferString(`{"pending_auth_token":"` + pendingToken + `","email":"owner@example.com","password":"password-123"}`)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/oauth/oidc/bind-login", body)
	ctx.Request.Header.Set("Content-Type", "application/json")

	handler.BindOIDCOAuthLogin(ctx)

	require.Equal(t, http.StatusForbidden, rec.Code)

	session, err := authSvc.GetPendingAuthSessionForProgress(context.Background(), pendingToken, nil)
	require.NoError(t, err)
	require.Nil(t, session.PasswordVerifiedAt)
	require.Nil(t, session.ConsumedAt)
}

func TestCreatePendingOAuthAccount_AllowsEmailBindingWithoutVerifyCodeWhenEmailVerificationDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler, authSvc, repo, _, _, pendingToken := newPendingAuthHandlerForReview(t, nil, service.PendingAuthIntentLogin, "oidc", func(user *service.User) {
		user.TotpEnabled = false
	})

	body := bytes.NewBufferString(`{"pending_auth_token":"` + pendingToken + `","email":"fresh@example.com","password":"secret-123"}`)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/oauth/oidc/create-account", body)
	ctx.Request.Header.Set("Content-Type", "application/json")

	handler.CreateOIDCOAuthAccount(ctx)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, repo.usersByMail, "fresh@example.com")

	_, err := authSvc.GetPendingAuthSessionForProgress(context.Background(), pendingToken, nil)
	require.ErrorIs(t, err, service.ErrInvalidToken)
}

func TestConfirmPendingAuthBind_RejectsDisabledTargetUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler, authSvc, _, _, _, pendingToken := newPendingAuthHandlerForReview(t, nil, service.PendingAuthIntentAdoptExistingUserByEmail, "oidc", func(user *service.User) {
		user.Status = service.StatusDisabled
		user.TotpEnabled = false
	})

	body := bytes.NewBufferString(`{"pending_auth_token":"` + pendingToken + `","password":"password-123"}`)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/oauth/confirm-bind", body)
	ctx.Request.Header.Set("Content-Type", "application/json")

	handler.ConfirmPendingAuthBind(ctx)

	require.Equal(t, http.StatusForbidden, rec.Code)

	session, err := authSvc.GetPendingAuthSessionForProgress(context.Background(), pendingToken, nil)
	require.NoError(t, err)
	require.Nil(t, session.ConsumedAt)
}

func TestLogin2FA_PendingAuthBackendModeDoesNotConsumeSession(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler, authSvc, _, _, _, pendingToken := newPendingAuthHandlerForReview(t, map[string]string{
		service.SettingKeyBackendModeEnabled: "true",
		service.SettingKeyTotpEnabled:        "true",
	}, service.PendingAuthIntentLogin, "linuxdo", func(user *service.User) {
		user.TotpEnabled = true
	})

	tempToken, err := handler.totpService.CreateLoginSessionForPendingAuth(context.Background(), 7, "owner@example.com", pendingToken)
	require.NoError(t, err)

	code, err := totp.GenerateCode("JBSWY3DPEHPK3PXP", time.Now())
	require.NoError(t, err)

	payload := bytes.NewBufferString(`{"temp_token":"` + tempToken + `","totp_code":"` + code + `","pending_auth_token":"` + pendingToken + `"}`)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login/2fa", payload)
	ctx.Request.Header.Set("Content-Type", "application/json")

	handler.Login2FA(ctx)

	require.Equal(t, http.StatusForbidden, rec.Code)

	session, err := authSvc.GetPendingAuthSessionForProgress(context.Background(), pendingToken, nil)
	require.NoError(t, err)
	require.Nil(t, session.TOTPVerifiedAt)
	require.Nil(t, session.ConsumedAt)
}
