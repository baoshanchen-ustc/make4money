package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type oauthConfirmSettingRepoStub struct {
	values map[string]string
}

func (s *oauthConfirmSettingRepoStub) Get(context.Context, string) (*Setting, error) {
	return nil, ErrSettingNotFound
}

func (s *oauthConfirmSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	if value, ok := s.values[key]; ok {
		return value, nil
	}
	return "", ErrSettingNotFound
}

func (s *oauthConfirmSettingRepoStub) Set(context.Context, string, string) error { return nil }

func (s *oauthConfirmSettingRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := s.values[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}

func (s *oauthConfirmSettingRepoStub) SetMultiple(context.Context, map[string]string) error {
	return nil
}
func (s *oauthConfirmSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	return s.values, nil
}
func (s *oauthConfirmSettingRepoStub) Delete(context.Context, string) error { return nil }

type oauthConfirmEmailCacheStub struct {
	data *VerificationCodeData
}

func (s *oauthConfirmEmailCacheStub) GetVerificationCode(context.Context, string) (*VerificationCodeData, error) {
	return s.data, nil
}
func (s *oauthConfirmEmailCacheStub) SetVerificationCode(context.Context, string, *VerificationCodeData, time.Duration) error {
	return nil
}
func (s *oauthConfirmEmailCacheStub) DeleteVerificationCode(context.Context, string) error {
	return nil
}
func (s *oauthConfirmEmailCacheStub) GetNotifyVerifyCode(context.Context, string) (*VerificationCodeData, error) {
	return nil, nil
}
func (s *oauthConfirmEmailCacheStub) SetNotifyVerifyCode(context.Context, string, *VerificationCodeData, time.Duration) error {
	return nil
}
func (s *oauthConfirmEmailCacheStub) DeleteNotifyVerifyCode(context.Context, string) error {
	return nil
}
func (s *oauthConfirmEmailCacheStub) IncrNotifyCodeUserRate(context.Context, int64, time.Duration) (int64, error) {
	return 0, nil
}
func (s *oauthConfirmEmailCacheStub) GetNotifyCodeUserRate(context.Context, int64) (int64, error) {
	return 0, nil
}
func (s *oauthConfirmEmailCacheStub) GetPasswordResetToken(context.Context, string) (*PasswordResetTokenData, error) {
	return nil, nil
}
func (s *oauthConfirmEmailCacheStub) SetPasswordResetToken(context.Context, string, *PasswordResetTokenData, time.Duration) error {
	return nil
}
func (s *oauthConfirmEmailCacheStub) DeletePasswordResetToken(context.Context, string) error {
	return nil
}
func (s *oauthConfirmEmailCacheStub) IsPasswordResetEmailInCooldown(context.Context, string) bool {
	return false
}
func (s *oauthConfirmEmailCacheStub) SetPasswordResetEmailCooldown(context.Context, string, time.Duration) error {
	return nil
}

type oauthFlowUserRepoStub struct {
	usersByID    map[int64]*User
	usersByEmail map[string]*User
	nextID       int64
	created      []*User
	upserted     []UpsertUserExternalIdentityInput
}

func (s *oauthFlowUserRepoStub) Create(_ context.Context, user *User) error {
	if s.usersByID == nil {
		s.usersByID = map[int64]*User{}
	}
	if s.usersByEmail == nil {
		s.usersByEmail = map[string]*User{}
	}
	if s.nextID == 0 {
		s.nextID = 1
	}
	if user.ID == 0 {
		user.ID = s.nextID
		s.nextID++
	}
	clone := *user
	s.created = append(s.created, &clone)
	s.usersByID[user.ID] = &clone
	s.usersByEmail[user.Email] = &clone
	return nil
}

func (s *oauthFlowUserRepoStub) GetByID(_ context.Context, id int64) (*User, error) {
	if user, ok := s.usersByID[id]; ok {
		clone := *user
		return &clone, nil
	}
	return nil, ErrUserNotFound
}

func (s *oauthFlowUserRepoStub) GetByEmail(_ context.Context, email string) (*User, error) {
	if user, ok := s.usersByEmail[email]; ok {
		clone := *user
		return &clone, nil
	}
	return nil, ErrUserNotFound
}

func (s *oauthFlowUserRepoStub) GetFirstAdmin(context.Context) (*User, error) {
	return nil, ErrUserNotFound
}

func (s *oauthFlowUserRepoStub) Update(_ context.Context, user *User) error {
	if s.usersByID == nil {
		s.usersByID = map[int64]*User{}
	}
	if s.usersByEmail == nil {
		s.usersByEmail = map[string]*User{}
	}
	clone := *user
	s.usersByID[user.ID] = &clone
	s.usersByEmail[user.Email] = &clone
	return nil
}

func (s *oauthFlowUserRepoStub) Delete(context.Context, int64) error { panic("unexpected Delete call") }

func (s *oauthFlowUserRepoStub) List(context.Context, pagination.PaginationParams) ([]User, *pagination.PaginationResult, error) {
	panic("unexpected List call")
}

func (s *oauthFlowUserRepoStub) ListWithFilters(context.Context, pagination.PaginationParams, UserListFilters) ([]User, *pagination.PaginationResult, error) {
	panic("unexpected ListWithFilters call")
}

func (s *oauthFlowUserRepoStub) UpdateBalance(context.Context, int64, float64) error {
	panic("unexpected UpdateBalance call")
}

func (s *oauthFlowUserRepoStub) DeductBalance(context.Context, int64, float64) error {
	panic("unexpected DeductBalance call")
}

func (s *oauthFlowUserRepoStub) UpdateConcurrency(context.Context, int64, int) error {
	panic("unexpected UpdateConcurrency call")
}

func (s *oauthFlowUserRepoStub) ExistsByEmail(_ context.Context, email string) (bool, error) {
	_, ok := s.usersByEmail[email]
	return ok, nil
}

func (s *oauthFlowUserRepoStub) RemoveGroupFromAllowedGroups(context.Context, int64) (int64, error) {
	panic("unexpected RemoveGroupFromAllowedGroups call")
}

func (s *oauthFlowUserRepoStub) RemoveGroupFromUserAllowedGroups(context.Context, int64, int64) error {
	panic("unexpected RemoveGroupFromUserAllowedGroups call")
}

func (s *oauthFlowUserRepoStub) AddGroupToAllowedGroups(context.Context, int64, int64) error {
	panic("unexpected AddGroupToAllowedGroups call")
}

func (s *oauthFlowUserRepoStub) UpdateTotpSecret(context.Context, int64, *string) error {
	panic("unexpected UpdateTotpSecret call")
}

func (s *oauthFlowUserRepoStub) EnableTotp(context.Context, int64) error {
	panic("unexpected EnableTotp call")
}

func (s *oauthFlowUserRepoStub) DisableTotp(context.Context, int64) error {
	panic("unexpected DisableTotp call")
}

func (s *oauthFlowUserRepoStub) ListExternalIdentities(context.Context, int64) ([]UserExternalIdentity, error) {
	return nil, nil
}

func (s *oauthFlowUserRepoStub) UpsertExternalIdentity(_ context.Context, userID int64, input UpsertUserExternalIdentityInput) (*UserExternalIdentity, error) {
	s.upserted = append(s.upserted, input)
	return &UserExternalIdentity{
		UserID:         userID,
		Provider:       input.Provider,
		ProviderUserID: input.ProviderUserID,
		DisplayName:    input.DisplayName,
	}, nil
}

func (s *oauthFlowUserRepoStub) DeleteExternalIdentity(context.Context, int64, string) error {
	panic("unexpected DeleteExternalIdentity call")
}

func (s *oauthFlowUserRepoStub) GetAvatar(context.Context, int64) (*UserAvatar, error) {
	return nil, ErrUserAvatarNotFound
}

func (s *oauthFlowUserRepoStub) UpsertAvatar(context.Context, int64, UpsertUserAvatarInput) (*UserAvatar, error) {
	panic("unexpected UpsertAvatar call")
}

func (s *oauthFlowUserRepoStub) DeleteAvatar(context.Context, int64) error {
	panic("unexpected DeleteAvatar call")
}

type oauthFlowRefreshTokenCacheStub struct{}

func (oauthFlowRefreshTokenCacheStub) StoreRefreshToken(context.Context, string, *RefreshTokenData, time.Duration) error {
	return nil
}
func (oauthFlowRefreshTokenCacheStub) GetRefreshToken(context.Context, string) (*RefreshTokenData, error) {
	return nil, ErrRefreshTokenNotFound
}
func (oauthFlowRefreshTokenCacheStub) DeleteRefreshToken(context.Context, string) error { return nil }
func (oauthFlowRefreshTokenCacheStub) DeleteUserRefreshTokens(context.Context, int64) error {
	return nil
}
func (oauthFlowRefreshTokenCacheStub) DeleteTokenFamily(context.Context, string) error { return nil }
func (oauthFlowRefreshTokenCacheStub) AddToUserTokenSet(context.Context, int64, string, time.Duration) error {
	return nil
}
func (oauthFlowRefreshTokenCacheStub) AddToFamilyTokenSet(context.Context, string, string, time.Duration) error {
	return nil
}
func (oauthFlowRefreshTokenCacheStub) GetUserTokenHashes(context.Context, int64) ([]string, error) {
	return nil, nil
}
func (oauthFlowRefreshTokenCacheStub) GetFamilyTokenHashes(context.Context, string) ([]string, error) {
	return nil, nil
}
func (oauthFlowRefreshTokenCacheStub) IsTokenInFamily(context.Context, string, string) (bool, error) {
	return false, nil
}

func newAuthServiceForOAuthConfirmTest(repo *oauthFlowUserRepoStub, settings map[string]string, emailCache EmailCache, queue *EmailQueueService) *AuthService {
	cfg := &config.Config{
		JWT: config.JWTConfig{
			Secret:                 "oauth-confirm-test-secret",
			ExpireHour:             1,
			RefreshTokenExpireDays: 7,
		},
		Default: config.DefaultConfig{
			UserBalance:     0,
			UserConcurrency: 1,
		},
	}
	settingService := NewSettingService(&oauthConfirmSettingRepoStub{values: settings}, cfg)
	var emailService *EmailService
	if emailCache != nil {
		emailService = NewEmailService(&oauthConfirmSettingRepoStub{values: settings}, emailCache)
	}
	return NewAuthService(
		nil,
		repo,
		nil,
		oauthFlowRefreshTokenCacheStub{},
		cfg,
		settingService,
		emailService,
		nil,
		queue,
		nil,
		nil,
	)
}

func TestAuthService_SendOAuthVerifyCode_AllowsExistingEmail(t *testing.T) {
	repo := &oauthFlowUserRepoStub{
		usersByID: map[int64]*User{
			1: {ID: 1, Email: "owner@example.com", Status: StatusActive},
		},
		usersByEmail: map[string]*User{
			"owner@example.com": {ID: 1, Email: "owner@example.com", Status: StatusActive},
		},
	}
	queue := &EmailQueueService{taskChan: make(chan EmailTask, 1)}
	svc := newAuthServiceForOAuthConfirmTest(repo, map[string]string{
		SettingKeyRegistrationEnabled: "false",
	}, nil, queue)

	result, err := svc.SendOAuthVerifyCode(context.Background(), "owner@example.com")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 60, result.Countdown)

	select {
	case task := <-queue.taskChan:
		require.Equal(t, TaskTypeVerifyCode, task.TaskType)
		require.Equal(t, "owner@example.com", task.Email)
	case <-time.After(time.Second):
		t.Fatal("expected oauth verify task to be enqueued")
	}
}

func TestAuthService_CreateAccountFromPendingOAuthIdentity_BindsVerifiedExistingEmail(t *testing.T) {
	user := &User{ID: 7, Email: "owner@example.com", Username: "owner", Role: RoleUser, Status: StatusActive}
	repo := &oauthFlowUserRepoStub{
		usersByID:    map[int64]*User{user.ID: user},
		usersByEmail: map[string]*User{user.Email: user},
		nextID:       8,
	}
	cache := &oauthConfirmEmailCacheStub{
		data: &VerificationCodeData{
			Code:      "123456",
			ExpiresAt: time.Now().Add(5 * time.Minute),
		},
	}
	svc := newAuthServiceForOAuthConfirmTest(repo, map[string]string{
		SettingKeyRegistrationEnabled: "false",
	}, cache, nil)

	tokenPair, boundUser, err := svc.CreateAccountFromPendingOAuthIdentity(context.Background(), PendingOAuthIdentity{
		Provider: "linuxdo",
		Subject:  "linuxdo-subject-1",
		Username: "linuxdo_owner",
	}, "owner@example.com", "123456", "")
	require.NoError(t, err)
	require.NotNil(t, tokenPair)
	require.NotEmpty(t, tokenPair.AccessToken)
	require.NotNil(t, boundUser)
	require.Equal(t, int64(7), boundUser.ID)
	require.Len(t, repo.created, 0)
	require.Len(t, repo.upserted, 1)
	require.Equal(t, ExternalIdentityProviderLinuxDo, repo.upserted[0].Provider)
	require.Equal(t, "linuxdo-subject-1", repo.upserted[0].ProviderUserID)
}

func TestAuthService_CreateAccountFromPendingOAuthIdentity_CreatesVerifiedEmailUser(t *testing.T) {
	repo := &oauthFlowUserRepoStub{
		usersByID:    map[int64]*User{},
		usersByEmail: map[string]*User{},
		nextID:       11,
	}
	cache := &oauthConfirmEmailCacheStub{
		data: &VerificationCodeData{
			Code:      "654321",
			ExpiresAt: time.Now().Add(5 * time.Minute),
		},
	}
	svc := newAuthServiceForOAuthConfirmTest(repo, map[string]string{
		SettingKeyRegistrationEnabled: "true",
	}, cache, nil)

	tokenPair, user, err := svc.CreateAccountFromPendingOAuthIdentity(context.Background(), PendingOAuthIdentity{
		Provider: "wechat",
		Subject:  "wechat-union-1",
		Username: "wechat_owner",
	}, "new-owner@example.com", "654321", "")
	require.NoError(t, err)
	require.NotNil(t, tokenPair)
	require.NotNil(t, user)
	require.Equal(t, "new-owner@example.com", user.Email)
	require.Len(t, repo.created, 1)
	require.Equal(t, "new-owner@example.com", repo.created[0].Email)
	require.Len(t, repo.upserted, 1)
	require.Equal(t, ExternalIdentityProviderWeChat, repo.upserted[0].Provider)
	require.Equal(t, "wechat-union-1", repo.upserted[0].ProviderUserID)
}

func TestAuthService_CreateAccountFromPendingOAuthIdentity_RequiresVerifyCode(t *testing.T) {
	repo := &oauthFlowUserRepoStub{
		usersByID:    map[int64]*User{},
		usersByEmail: map[string]*User{},
	}
	svc := newAuthServiceForOAuthConfirmTest(repo, map[string]string{
		SettingKeyRegistrationEnabled: "true",
	}, nil, nil)

	_, _, err := svc.CreateAccountFromPendingOAuthIdentity(context.Background(), PendingOAuthIdentity{
		Provider: "linuxdo",
		Subject:  "linuxdo-subject-2",
	}, "user@example.com", "", "")
	require.ErrorIs(t, err, ErrPendingOAuthVerifyCodeRequired)
	require.False(t, errors.Is(err, ErrServiceUnavailable))
}

func TestAuthService_CreateAccountFromPendingOAuthIdentity_RejectsBindOnlyIntent(t *testing.T) {
	repo := &oauthFlowUserRepoStub{
		usersByID:    map[int64]*User{},
		usersByEmail: map[string]*User{},
		nextID:       12,
	}
	cache := &oauthConfirmEmailCacheStub{
		data: &VerificationCodeData{
			Code:      "123456",
			ExpiresAt: time.Now().Add(5 * time.Minute),
		},
	}
	svc := newAuthServiceForOAuthConfirmTest(repo, map[string]string{
		SettingKeyRegistrationEnabled: "true",
	}, cache, nil)

	tokenPair, user, err := svc.CreateAccountFromPendingOAuthIdentity(context.Background(), PendingOAuthIdentity{
		Provider: "linuxdo",
		Subject:  "linuxdo-subject-bind",
		Username: "linuxdo_owner",
		Intent:   "bind",
	}, "owner@example.com", "123456", "")
	require.ErrorIs(t, err, ErrPendingOAuthBindOnly)
	require.Nil(t, tokenPair)
	require.Nil(t, user)
	require.Len(t, repo.created, 0)
	require.Len(t, repo.upserted, 0)
}
