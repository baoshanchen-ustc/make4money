package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/require"
)

type oauthAuthHandlerUserRepoStub struct {
	usersByID    map[int64]*service.User
	usersByEmail map[string]*service.User
	upserted     []service.UpsertUserExternalIdentityInput
}

type oauthAuthHandlerSettingRepoStub struct {
	values map[string]string
}

func (s *oauthAuthHandlerSettingRepoStub) Get(context.Context, string) (*service.Setting, error) {
	return nil, service.ErrSettingNotFound
}

func (s *oauthAuthHandlerSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	if value, ok := s.values[key]; ok {
		return value, nil
	}
	return "", service.ErrSettingNotFound
}

func (s *oauthAuthHandlerSettingRepoStub) Set(context.Context, string, string) error { return nil }

func (s *oauthAuthHandlerSettingRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := s.values[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}

func (s *oauthAuthHandlerSettingRepoStub) SetMultiple(context.Context, map[string]string) error {
	return nil
}
func (s *oauthAuthHandlerSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	return s.values, nil
}
func (s *oauthAuthHandlerSettingRepoStub) Delete(context.Context, string) error { return nil }

func (s *oauthAuthHandlerUserRepoStub) Create(context.Context, *service.User) error {
	panic("unexpected Create call")
}

func (s *oauthAuthHandlerUserRepoStub) GetByID(_ context.Context, id int64) (*service.User, error) {
	if user, ok := s.usersByID[id]; ok {
		clone := *user
		return &clone, nil
	}
	return nil, service.ErrUserNotFound
}

func (s *oauthAuthHandlerUserRepoStub) GetByEmail(_ context.Context, email string) (*service.User, error) {
	if user, ok := s.usersByEmail[email]; ok {
		clone := *user
		return &clone, nil
	}
	return nil, service.ErrUserNotFound
}

func (s *oauthAuthHandlerUserRepoStub) GetFirstAdmin(context.Context) (*service.User, error) {
	return nil, service.ErrUserNotFound
}

func (s *oauthAuthHandlerUserRepoStub) Update(_ context.Context, user *service.User) error {
	clone := *user
	s.usersByID[user.ID] = &clone
	s.usersByEmail[user.Email] = &clone
	return nil
}

func (s *oauthAuthHandlerUserRepoStub) Delete(context.Context, int64) error {
	panic("unexpected Delete call")
}

func (s *oauthAuthHandlerUserRepoStub) List(context.Context, pagination.PaginationParams) ([]service.User, *pagination.PaginationResult, error) {
	panic("unexpected List call")
}

func (s *oauthAuthHandlerUserRepoStub) ListWithFilters(context.Context, pagination.PaginationParams, service.UserListFilters) ([]service.User, *pagination.PaginationResult, error) {
	panic("unexpected ListWithFilters call")
}

func (s *oauthAuthHandlerUserRepoStub) UpdateBalance(context.Context, int64, float64) error {
	panic("unexpected UpdateBalance call")
}

func (s *oauthAuthHandlerUserRepoStub) DeductBalance(context.Context, int64, float64) error {
	panic("unexpected DeductBalance call")
}

func (s *oauthAuthHandlerUserRepoStub) UpdateConcurrency(context.Context, int64, int) error {
	panic("unexpected UpdateConcurrency call")
}

func (s *oauthAuthHandlerUserRepoStub) ExistsByEmail(_ context.Context, email string) (bool, error) {
	_, ok := s.usersByEmail[email]
	return ok, nil
}

func (s *oauthAuthHandlerUserRepoStub) RemoveGroupFromAllowedGroups(context.Context, int64) (int64, error) {
	panic("unexpected RemoveGroupFromAllowedGroups call")
}

func (s *oauthAuthHandlerUserRepoStub) AddGroupToAllowedGroups(context.Context, int64, int64) error {
	panic("unexpected AddGroupToAllowedGroups call")
}

func (s *oauthAuthHandlerUserRepoStub) RemoveGroupFromUserAllowedGroups(context.Context, int64, int64) error {
	panic("unexpected RemoveGroupFromUserAllowedGroups call")
}

func (s *oauthAuthHandlerUserRepoStub) UpdateTotpSecret(context.Context, int64, *string) error {
	panic("unexpected UpdateTotpSecret call")
}

func (s *oauthAuthHandlerUserRepoStub) EnableTotp(context.Context, int64) error {
	panic("unexpected EnableTotp call")
}

func (s *oauthAuthHandlerUserRepoStub) DisableTotp(context.Context, int64) error {
	panic("unexpected DisableTotp call")
}

func (s *oauthAuthHandlerUserRepoStub) ListExternalIdentities(context.Context, int64) ([]service.UserExternalIdentity, error) {
	return nil, nil
}

func (s *oauthAuthHandlerUserRepoStub) UpsertExternalIdentity(_ context.Context, userID int64, input service.UpsertUserExternalIdentityInput) (*service.UserExternalIdentity, error) {
	s.upserted = append(s.upserted, input)
	return &service.UserExternalIdentity{
		UserID:         userID,
		Provider:       input.Provider,
		ProviderUserID: input.ProviderUserID,
		DisplayName:    input.DisplayName,
	}, nil
}

func (s *oauthAuthHandlerUserRepoStub) DeleteExternalIdentity(context.Context, int64, string) error {
	panic("unexpected DeleteExternalIdentity call")
}

func (s *oauthAuthHandlerUserRepoStub) GetAvatar(context.Context, int64) (*service.UserAvatar, error) {
	return nil, service.ErrUserAvatarNotFound
}

func (s *oauthAuthHandlerUserRepoStub) UpsertAvatar(context.Context, int64, service.UpsertUserAvatarInput) (*service.UserAvatar, error) {
	panic("unexpected UpsertAvatar call")
}

func (s *oauthAuthHandlerUserRepoStub) DeleteAvatar(context.Context, int64) error {
	panic("unexpected DeleteAvatar call")
}

type oauthTotpCacheStub struct {
	loginSessions map[string]*service.TotpLoginSession
	attempts      map[int64]int
}

func (s *oauthTotpCacheStub) GetSetupSession(context.Context, int64) (*service.TotpSetupSession, error) {
	return nil, nil
}
func (s *oauthTotpCacheStub) SetSetupSession(context.Context, int64, *service.TotpSetupSession, time.Duration) error {
	return nil
}
func (s *oauthTotpCacheStub) DeleteSetupSession(context.Context, int64) error { return nil }
func (s *oauthTotpCacheStub) GetLoginSession(_ context.Context, tempToken string) (*service.TotpLoginSession, error) {
	if session, ok := s.loginSessions[tempToken]; ok {
		return session, nil
	}
	return nil, service.ErrTotpUnavailable
}
func (s *oauthTotpCacheStub) SetLoginSession(_ context.Context, tempToken string, session *service.TotpLoginSession, _ time.Duration) error {
	s.loginSessions[tempToken] = session
	return nil
}
func (s *oauthTotpCacheStub) DeleteLoginSession(_ context.Context, tempToken string) error {
	delete(s.loginSessions, tempToken)
	return nil
}
func (s *oauthTotpCacheStub) IncrementVerifyAttempts(_ context.Context, userID int64) (int, error) {
	s.attempts[userID]++
	return s.attempts[userID], nil
}
func (s *oauthTotpCacheStub) GetVerifyAttempts(_ context.Context, userID int64) (int, error) {
	return s.attempts[userID], nil
}
func (s *oauthTotpCacheStub) ClearVerifyAttempts(_ context.Context, userID int64) error {
	delete(s.attempts, userID)
	return nil
}

type noopSecretEncryptor struct{}

func (noopSecretEncryptor) Encrypt(plaintext string) (string, error)  { return plaintext, nil }
func (noopSecretEncryptor) Decrypt(ciphertext string) (string, error) { return ciphertext, nil }

func newAuthHandlerForOAuthSecurityTest(t *testing.T, repo *oauthAuthHandlerUserRepoStub, settingValues map[string]string, totpSvc *service.TotpService) (*AuthHandler, *service.AuthService) {
	t.Helper()

	cfg := &config.Config{
		JWT: config.JWTConfig{
			Secret:                 "handler-oauth-security-secret",
			ExpireHour:             1,
			RefreshTokenExpireDays: 7,
		},
		Default: config.DefaultConfig{
			UserBalance:     0,
			UserConcurrency: 1,
		},
	}
	settingSvc := service.NewSettingService(&oauthAuthHandlerSettingRepoStub{values: settingValues}, cfg)
	authSvc := service.NewAuthService(
		nil,
		repo,
		nil,
		wechatOAuthRefreshTokenCacheStub{},
		cfg,
		settingSvc,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	userSvc := service.NewUserService(repo, nil, nil, nil)
	return NewAuthHandler(cfg, authSvc, userSvc, settingSvc, nil, nil, totpSvc), authSvc
}

func TestBindLinuxDoOAuthLogin_DefersBindingUntilTwoFactorCompletion(t *testing.T) {
	gin.SetMode(gin.TestMode)

	password := "Secret123!"
	user := &service.User{
		ID:          7,
		Email:       "owner@example.com",
		Username:    "owner",
		Role:        service.RoleUser,
		Status:      service.StatusActive,
		TotpEnabled: true,
	}
	require.NoError(t, user.SetPassword(password))

	secret := "JBSWY3DPEHPK3PXP"
	user.TotpSecretEncrypted = &secret

	repo := &oauthAuthHandlerUserRepoStub{
		usersByID:    map[int64]*service.User{user.ID: user},
		usersByEmail: map[string]*service.User{user.Email: user},
	}
	totpCache := &oauthTotpCacheStub{
		loginSessions: map[string]*service.TotpLoginSession{},
		attempts:      map[int64]int{},
	}
	settingValues := map[string]string{
		service.SettingKeyTotpEnabled: "true",
	}
	cfg := &config.Config{
		JWT: config.JWTConfig{
			Secret:                 "handler-oauth-security-secret",
			ExpireHour:             1,
			RefreshTokenExpireDays: 7,
		},
		Default: config.DefaultConfig{
			UserBalance:     0,
			UserConcurrency: 1,
		},
	}
	settingSvc := service.NewSettingService(&oauthAuthHandlerSettingRepoStub{values: settingValues}, cfg)
	totpSvc := service.NewTotpService(repo, noopSecretEncryptor{}, totpCache, settingSvc, nil, nil)
	authHandler, authSvc := newAuthHandlerForOAuthSecurityTest(t, repo, settingValues, totpSvc)

	pendingToken, err := authSvc.CreatePendingOAuthTokenWithIdentity(service.PendingOAuthIdentity{
		Provider: "linuxdo",
		Subject:  "linuxdo-subject-1",
		Username: "linuxdo_user",
	})
	require.NoError(t, err)

	bindBody := `{"pending_oauth_token":"` + pendingToken + `","email":"owner@example.com","password":"` + password + `"}`
	bindRec := httptest.NewRecorder()
	bindCtx, _ := gin.CreateTestContext(bindRec)
	bindCtx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/oauth/linuxdo/bind-login", bytes.NewBufferString(bindBody))
	bindCtx.Request.Header.Set("Content-Type", "application/json")

	authHandler.BindLinuxDoOAuthLogin(bindCtx)
	require.Equal(t, http.StatusOK, bindRec.Code)
	require.Empty(t, repo.upserted)

	var bindResp struct {
		Code int `json:"code"`
		Data struct {
			Requires2FA bool   `json:"requires_2fa"`
			TempToken   string `json:"temp_token"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(bindRec.Body.Bytes(), &bindResp))
	require.True(t, bindResp.Data.Requires2FA)
	require.NotEmpty(t, bindResp.Data.TempToken)

	totpCode, err := totp.GenerateCode(secret, time.Now())
	require.NoError(t, err)

	login2FABody := `{"temp_token":"` + bindResp.Data.TempToken + `","totp_code":"` + totpCode + `","pending_oauth_token":"` + pendingToken + `"}`
	login2FARec := httptest.NewRecorder()
	login2FACtx, _ := gin.CreateTestContext(login2FARec)
	login2FACtx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login/2fa", bytes.NewBufferString(login2FABody))
	login2FACtx.Request.Header.Set("Content-Type", "application/json")

	authHandler.Login2FA(login2FACtx)
	require.Equal(t, http.StatusOK, login2FARec.Code)
	require.Len(t, repo.upserted, 1)
	require.Equal(t, service.ExternalIdentityProviderLinuxDo, repo.upserted[0].Provider)
	require.Equal(t, "linuxdo-subject-1", repo.upserted[0].ProviderUserID)
}

func TestRedirectOAuthSuccessForUser_RejectsInactiveUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler, _ := newAuthHandlerForOAuthSecurityTest(t, &oauthAuthHandlerUserRepoStub{
		usersByID:    map[int64]*service.User{},
		usersByEmail: map[string]*service.User{},
	}, map[string]string{}, nil)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/auth/oauth/linuxdo/callback", nil)

	handler.redirectOAuthSuccessForUser(ctx, "/auth/linuxdo/callback", "/dashboard", &service.User{
		ID:     9,
		Email:  "disabled@example.com",
		Status: service.StatusDisabled,
	})

	require.Equal(t, http.StatusFound, rec.Code)
	location := rec.Header().Get("Location")
	u, err := url.Parse(location)
	require.NoError(t, err)
	fragment, err := url.ParseQuery(u.Fragment)
	require.NoError(t, err)
	require.Equal(t, "login_failed", fragment.Get("error"))
	require.Equal(t, "USER_NOT_ACTIVE", fragment.Get("error_message"))
	require.Equal(t, "user is not active", fragment.Get("error_description"))
}
