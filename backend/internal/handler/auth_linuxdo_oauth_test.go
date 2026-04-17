package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSanitizeFrontendRedirectPath(t *testing.T) {
	require.Equal(t, "/dashboard", sanitizeFrontendRedirectPath("/dashboard"))
	require.Equal(t, "/dashboard", sanitizeFrontendRedirectPath(" /dashboard "))
	require.Equal(t, "", sanitizeFrontendRedirectPath("dashboard"))
	require.Equal(t, "", sanitizeFrontendRedirectPath("//evil.com"))
	require.Equal(t, "", sanitizeFrontendRedirectPath("https://evil.com"))
	require.Equal(t, "", sanitizeFrontendRedirectPath("/\nfoo"))

	long := "/" + strings.Repeat("a", linuxDoOAuthMaxRedirectLen)
	require.Equal(t, "", sanitizeFrontendRedirectPath(long))
}

func TestBuildBearerAuthorization(t *testing.T) {
	auth, err := buildBearerAuthorization("", "token123")
	require.NoError(t, err)
	require.Equal(t, "Bearer token123", auth)

	auth, err = buildBearerAuthorization("bearer", "token123")
	require.NoError(t, err)
	require.Equal(t, "Bearer token123", auth)

	_, err = buildBearerAuthorization("MAC", "token123")
	require.Error(t, err)

	_, err = buildBearerAuthorization("Bearer", "token 123")
	require.Error(t, err)
}

func TestLinuxDoParseUserInfoParsesIDAndUsername(t *testing.T) {
	cfg := config.LinuxDoConnectConfig{
		UserInfoURL: "https://connect.linux.do/api/user",
	}

	email, username, subject, err := linuxDoParseUserInfo(`{"id":123,"username":"alice"}`, cfg)
	require.NoError(t, err)
	require.Equal(t, "123", subject)
	require.Equal(t, "alice", username)
	require.Equal(t, "linuxdo-123@linuxdo-connect.invalid", email)
}

func TestLinuxDoParseUserInfoDefaultsUsername(t *testing.T) {
	cfg := config.LinuxDoConnectConfig{
		UserInfoURL: "https://connect.linux.do/api/user",
	}

	email, username, subject, err := linuxDoParseUserInfo(`{"id":"123"}`, cfg)
	require.NoError(t, err)
	require.Equal(t, "123", subject)
	require.Equal(t, "linuxdo_123", username)
	require.Equal(t, "linuxdo-123@linuxdo-connect.invalid", email)
}

func TestLinuxDoParseUserInfoRejectsUnsafeSubject(t *testing.T) {
	cfg := config.LinuxDoConnectConfig{
		UserInfoURL: "https://connect.linux.do/api/user",
	}

	_, _, _, err := linuxDoParseUserInfo(`{"id":"123@456"}`, cfg)
	require.Error(t, err)

	tooLong := strings.Repeat("a", linuxDoOAuthMaxSubjectLen+1)
	_, _, _, err = linuxDoParseUserInfo(`{"id":"`+tooLong+`"}`, cfg)
	require.Error(t, err)
}

func TestParseOAuthProviderErrorJSON(t *testing.T) {
	code, desc := parseOAuthProviderError(`{"error":"invalid_client","error_description":"bad secret"}`)
	require.Equal(t, "invalid_client", code)
	require.Equal(t, "bad secret", desc)
}

func TestParseOAuthProviderErrorForm(t *testing.T) {
	code, desc := parseOAuthProviderError("error=invalid_request&error_description=Missing+code_verifier")
	require.Equal(t, "invalid_request", code)
	require.Equal(t, "Missing code_verifier", desc)
}

func TestParseLinuxDoTokenResponseJSON(t *testing.T) {
	token, ok := parseLinuxDoTokenResponse(`{"access_token":"t1","token_type":"Bearer","expires_in":3600,"scope":"user"}`)
	require.True(t, ok)
	require.Equal(t, "t1", token.AccessToken)
	require.Equal(t, "Bearer", token.TokenType)
	require.Equal(t, int64(3600), token.ExpiresIn)
	require.Equal(t, "user", token.Scope)
}

func TestParseLinuxDoTokenResponseForm(t *testing.T) {
	token, ok := parseLinuxDoTokenResponse("access_token=t2&token_type=bearer&expires_in=60")
	require.True(t, ok)
	require.Equal(t, "t2", token.AccessToken)
	require.Equal(t, "bearer", token.TokenType)
	require.Equal(t, int64(60), token.ExpiresIn)
}

func TestSingleLineStripsWhitespace(t *testing.T) {
	require.Equal(t, "hello world", singleLine("hello\r\nworld"))
	require.Equal(t, "", singleLine("\n\t\r"))
}

func TestCreateLinuxDoOAuthAccount_PersistsExternalIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)

	emailCache := &linuxDoOAuthEmailCacheStub{
		data: map[string]*service.VerificationCodeData{
			"user@example.com": {
				Code:      "123456",
				CreatedAt: time.Now(),
				ExpiresAt: time.Now().Add(5 * time.Minute),
			},
		},
	}
	repo := &linuxDoOAuthUserRepoStub{nextID: 12}
	settingsRepo := &linuxDoOAuthSettingRepoStub{values: map[string]string{
		service.SettingKeyRegistrationEnabled:   "true",
		service.SettingKeyInvitationCodeEnabled: "false",
	}}
	settingSvc := service.NewSettingService(settingsRepo, &config.Config{
		JWT: config.JWTConfig{
			Secret:                 "test-secret-linuxdo-create",
			ExpireHour:             1,
			RefreshTokenExpireDays: 7,
		},
		Default: config.DefaultConfig{
			UserBalance:     0,
			UserConcurrency: 1,
		},
	})
	emailSvc := service.NewEmailService(settingsRepo, emailCache)
	authSvc := service.NewAuthService(
		nil,
		repo,
		nil,
		wechatOAuthRefreshTokenCacheStub{},
		&config.Config{
			JWT: config.JWTConfig{
				Secret:                 "test-secret-linuxdo-create",
				ExpireHour:             1,
				RefreshTokenExpireDays: 7,
			},
			Default: config.DefaultConfig{
				UserBalance:     0,
				UserConcurrency: 1,
			},
		},
		settingSvc,
		emailSvc,
		nil,
		nil,
		nil,
		nil,
	)
	authHandler := NewAuthHandler(&config.Config{}, authSvc, nil, settingSvc, nil, nil, nil)

	pendingToken, err := authSvc.CreatePendingOAuthTokenWithIdentity(service.PendingOAuthIdentity{
		Email:       "linuxdo-123@linuxdo-connect.invalid",
		Username:    "linuxdo_alice",
		Provider:    "linuxdo",
		Subject:     "123",
		IdentityKey: "linuxdo\x1f123",
	})
	require.NoError(t, err)

	body := map[string]any{
		"pending_oauth_token": pendingToken,
		"email":               "user@example.com",
		"verify_code":         "123456",
	}
	raw, err := json.Marshal(body)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/oauth/linuxdo/create-account", bytes.NewReader(raw))
	c.Request.Header.Set("Content-Type", "application/json")

	authHandler.CreateLinuxDoOAuthAccount(c)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, repo.upsertInputs, 1)
	require.Equal(t, service.ExternalIdentityProviderLinuxDo, repo.upsertInputs[0].Provider)
	require.Equal(t, "123", repo.upsertInputs[0].ProviderUserID)
	require.NotNil(t, repo.userByEmail["user@example.com"])
}

type linuxDoOAuthEmailCacheStub struct {
	data map[string]*service.VerificationCodeData
}

type linuxDoOAuthSettingRepoStub struct {
	values map[string]string
}

func (s *linuxDoOAuthSettingRepoStub) Get(context.Context, string) (*service.Setting, error) {
	panic("unexpected Get call")
}

func (s *linuxDoOAuthSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	if value, ok := s.values[key]; ok {
		return value, nil
	}
	return "", service.ErrSettingNotFound
}

func (s *linuxDoOAuthSettingRepoStub) Set(context.Context, string, string) error {
	panic("unexpected Set call")
}

func (s *linuxDoOAuthSettingRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := s.values[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}

func (s *linuxDoOAuthSettingRepoStub) SetMultiple(context.Context, map[string]string) error {
	panic("unexpected SetMultiple call")
}

func (s *linuxDoOAuthSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	out := make(map[string]string, len(s.values))
	for key, value := range s.values {
		out[key] = value
	}
	return out, nil
}

func (s *linuxDoOAuthSettingRepoStub) Delete(context.Context, string) error {
	panic("unexpected Delete call")
}

func (s *linuxDoOAuthEmailCacheStub) GetVerificationCode(_ context.Context, email string) (*service.VerificationCodeData, error) {
	if data, ok := s.data[email]; ok {
		return data, nil
	}
	return nil, service.ErrInvalidVerifyCode
}

func (s *linuxDoOAuthEmailCacheStub) SetVerificationCode(context.Context, string, *service.VerificationCodeData, time.Duration) error {
	return nil
}

func (s *linuxDoOAuthEmailCacheStub) DeleteVerificationCode(context.Context, string) error {
	return nil
}
func (s *linuxDoOAuthEmailCacheStub) GetNotifyVerifyCode(context.Context, string) (*service.VerificationCodeData, error) {
	return nil, nil
}
func (s *linuxDoOAuthEmailCacheStub) SetNotifyVerifyCode(context.Context, string, *service.VerificationCodeData, time.Duration) error {
	return nil
}
func (s *linuxDoOAuthEmailCacheStub) DeleteNotifyVerifyCode(context.Context, string) error {
	return nil
}
func (s *linuxDoOAuthEmailCacheStub) IncrNotifyCodeUserRate(context.Context, int64, time.Duration) (int64, error) {
	return 0, nil
}
func (s *linuxDoOAuthEmailCacheStub) GetNotifyCodeUserRate(context.Context, int64) (int64, error) {
	return 0, nil
}
func (s *linuxDoOAuthEmailCacheStub) GetPasswordResetToken(context.Context, string) (*service.PasswordResetTokenData, error) {
	return nil, nil
}
func (s *linuxDoOAuthEmailCacheStub) SetPasswordResetToken(context.Context, string, *service.PasswordResetTokenData, time.Duration) error {
	return nil
}
func (s *linuxDoOAuthEmailCacheStub) DeletePasswordResetToken(context.Context, string) error {
	return nil
}
func (s *linuxDoOAuthEmailCacheStub) IsPasswordResetEmailInCooldown(context.Context, string) bool {
	return false
}
func (s *linuxDoOAuthEmailCacheStub) SetPasswordResetEmailCooldown(context.Context, string, time.Duration) error {
	return nil
}

type linuxDoOAuthUserRepoStub struct {
	nextID       int64
	userByID     map[int64]*service.User
	userByEmail  map[string]*service.User
	upsertInputs []service.UpsertUserExternalIdentityInput
}

func (s *linuxDoOAuthUserRepoStub) Create(_ context.Context, user *service.User) error {
	if s.userByID == nil {
		s.userByID = make(map[int64]*service.User)
	}
	if s.userByEmail == nil {
		s.userByEmail = make(map[string]*service.User)
	}
	if s.nextID == 0 {
		s.nextID = 1
	}
	if user.ID == 0 {
		user.ID = s.nextID
		s.nextID++
	}
	clone := *user
	s.userByID[user.ID] = &clone
	s.userByEmail[user.Email] = &clone
	return nil
}

func (s *linuxDoOAuthUserRepoStub) GetByID(_ context.Context, id int64) (*service.User, error) {
	if user, ok := s.userByID[id]; ok {
		clone := *user
		return &clone, nil
	}
	return nil, service.ErrUserNotFound
}

func (s *linuxDoOAuthUserRepoStub) GetByEmail(_ context.Context, email string) (*service.User, error) {
	if user, ok := s.userByEmail[email]; ok {
		clone := *user
		return &clone, nil
	}
	return nil, service.ErrUserNotFound
}

func (s *linuxDoOAuthUserRepoStub) GetFirstAdmin(context.Context) (*service.User, error) {
	return nil, service.ErrUserNotFound
}

func (s *linuxDoOAuthUserRepoStub) Update(_ context.Context, user *service.User) error {
	if s.userByID == nil {
		s.userByID = make(map[int64]*service.User)
	}
	if s.userByEmail == nil {
		s.userByEmail = make(map[string]*service.User)
	}
	clone := *user
	s.userByID[user.ID] = &clone
	s.userByEmail[user.Email] = &clone
	return nil
}

func (s *linuxDoOAuthUserRepoStub) Delete(context.Context, int64) error { return nil }
func (s *linuxDoOAuthUserRepoStub) List(context.Context, pagination.PaginationParams) ([]service.User, *pagination.PaginationResult, error) {
	return nil, nil, errors.New("not implemented")
}
func (s *linuxDoOAuthUserRepoStub) ListWithFilters(context.Context, pagination.PaginationParams, service.UserListFilters) ([]service.User, *pagination.PaginationResult, error) {
	return nil, nil, errors.New("not implemented")
}
func (s *linuxDoOAuthUserRepoStub) UpdateBalance(context.Context, int64, float64) error { return nil }
func (s *linuxDoOAuthUserRepoStub) DeductBalance(context.Context, int64, float64) error { return nil }
func (s *linuxDoOAuthUserRepoStub) UpdateConcurrency(context.Context, int64, int) error { return nil }
func (s *linuxDoOAuthUserRepoStub) ExistsByEmail(_ context.Context, email string) (bool, error) {
	_, ok := s.userByEmail[email]
	return ok, nil
}
func (s *linuxDoOAuthUserRepoStub) RemoveGroupFromAllowedGroups(context.Context, int64) (int64, error) {
	return 0, nil
}
func (s *linuxDoOAuthUserRepoStub) AddGroupToAllowedGroups(context.Context, int64, int64) error {
	return nil
}
func (s *linuxDoOAuthUserRepoStub) RemoveGroupFromUserAllowedGroups(context.Context, int64, int64) error {
	return nil
}
func (s *linuxDoOAuthUserRepoStub) UpdateTotpSecret(context.Context, int64, *string) error {
	return nil
}
func (s *linuxDoOAuthUserRepoStub) EnableTotp(context.Context, int64) error  { return nil }
func (s *linuxDoOAuthUserRepoStub) DisableTotp(context.Context, int64) error { return nil }
func (s *linuxDoOAuthUserRepoStub) ListExternalIdentities(context.Context, int64) ([]service.UserExternalIdentity, error) {
	return nil, nil
}
func (s *linuxDoOAuthUserRepoStub) UpsertExternalIdentity(_ context.Context, userID int64, input service.UpsertUserExternalIdentityInput) (*service.UserExternalIdentity, error) {
	s.upsertInputs = append(s.upsertInputs, input)
	return &service.UserExternalIdentity{
		UserID:         userID,
		Provider:       input.Provider,
		ProviderUserID: input.ProviderUserID,
		DisplayName:    input.DisplayName,
	}, nil
}
func (s *linuxDoOAuthUserRepoStub) DeleteExternalIdentity(context.Context, int64, string) error {
	return nil
}
func (s *linuxDoOAuthUserRepoStub) GetAvatar(context.Context, int64) (*service.UserAvatar, error) {
	return nil, service.ErrUserAvatarNotFound
}
func (s *linuxDoOAuthUserRepoStub) UpsertAvatar(context.Context, int64, service.UpsertUserAvatarInput) (*service.UserAvatar, error) {
	return nil, errors.New("not implemented")
}
func (s *linuxDoOAuthUserRepoStub) DeleteAvatar(context.Context, int64) error { return nil }
