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
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/require"
)

type pendingAuthSettingRepoStub struct {
	values map[string]string
}

func (s *pendingAuthSettingRepoStub) Get(context.Context, string) (*service.Setting, error) {
	panic("unexpected Get call")
}
func (s *pendingAuthSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	if v, ok := s.values[key]; ok {
		return v, nil
	}
	return "", service.ErrSettingNotFound
}
func (s *pendingAuthSettingRepoStub) Set(context.Context, string, string) error {
	panic("unexpected Set call")
}
func (s *pendingAuthSettingRepoStub) GetMultiple(context.Context, []string) (map[string]string, error) {
	panic("unexpected GetMultiple call")
}
func (s *pendingAuthSettingRepoStub) SetMultiple(context.Context, map[string]string) error {
	panic("unexpected SetMultiple call")
}
func (s *pendingAuthSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}
func (s *pendingAuthSettingRepoStub) Delete(context.Context, string) error {
	panic("unexpected Delete call")
}

type pendingAuthHandlerUserRepoStub struct {
	sessions    map[string]*service.PendingAuthSessionRecord
	users       map[int64]*service.User
	usersByMail map[string]*service.User
	nextSession int
	nextUserID  int64
}

func newPendingAuthHandlerUserRepoStub() *pendingAuthHandlerUserRepoStub {
	return &pendingAuthHandlerUserRepoStub{
		sessions:    make(map[string]*service.PendingAuthSessionRecord),
		users:       make(map[int64]*service.User),
		usersByMail: make(map[string]*service.User),
		nextUserID:  100,
	}
}

func (s *pendingAuthHandlerUserRepoStub) Create(_ context.Context, user *service.User) error {
	s.nextUserID++
	if user.ID == 0 {
		user.ID = s.nextUserID
	}
	copied := *user
	s.users[user.ID] = &copied
	s.usersByMail[user.Email] = &copied
	return nil
}
func (s *pendingAuthHandlerUserRepoStub) GetByID(_ context.Context, id int64) (*service.User, error) {
	user, ok := s.users[id]
	if !ok {
		return nil, service.ErrUserNotFound
	}
	copied := *user
	return &copied, nil
}
func (s *pendingAuthHandlerUserRepoStub) GetByEmail(_ context.Context, email string) (*service.User, error) {
	user, ok := s.usersByMail[email]
	if !ok {
		return nil, service.ErrUserNotFound
	}
	copied := *user
	return &copied, nil
}
func (s *pendingAuthHandlerUserRepoStub) GetFirstAdmin(context.Context) (*service.User, error) {
	panic("unexpected GetFirstAdmin call")
}
func (s *pendingAuthHandlerUserRepoStub) Update(_ context.Context, user *service.User) error {
	copied := *user
	s.users[user.ID] = &copied
	s.usersByMail[user.Email] = &copied
	return nil
}
func (s *pendingAuthHandlerUserRepoStub) Delete(context.Context, int64) error {
	panic("unexpected Delete call")
}
func (s *pendingAuthHandlerUserRepoStub) List(context.Context, pagination.PaginationParams) ([]service.User, *pagination.PaginationResult, error) {
	panic("unexpected List call")
}
func (s *pendingAuthHandlerUserRepoStub) ListWithFilters(context.Context, pagination.PaginationParams, service.UserListFilters) ([]service.User, *pagination.PaginationResult, error) {
	panic("unexpected ListWithFilters call")
}
func (s *pendingAuthHandlerUserRepoStub) UpdateBalance(context.Context, int64, float64) error {
	panic("unexpected UpdateBalance call")
}
func (s *pendingAuthHandlerUserRepoStub) DeductBalance(context.Context, int64, float64) error {
	panic("unexpected DeductBalance call")
}
func (s *pendingAuthHandlerUserRepoStub) UpdateConcurrency(context.Context, int64, int) error {
	panic("unexpected UpdateConcurrency call")
}
func (s *pendingAuthHandlerUserRepoStub) ExistsByEmail(_ context.Context, email string) (bool, error) {
	_, ok := s.usersByMail[email]
	return ok, nil
}
func (s *pendingAuthHandlerUserRepoStub) RemoveGroupFromAllowedGroups(context.Context, int64) (int64, error) {
	panic("unexpected RemoveGroupFromAllowedGroups call")
}
func (s *pendingAuthHandlerUserRepoStub) AddGroupToAllowedGroups(context.Context, int64, int64) error {
	panic("unexpected AddGroupToAllowedGroups call")
}
func (s *pendingAuthHandlerUserRepoStub) RemoveGroupFromUserAllowedGroups(context.Context, int64, int64) error {
	panic("unexpected RemoveGroupFromUserAllowedGroups call")
}
func (s *pendingAuthHandlerUserRepoStub) UpdateTotpSecret(context.Context, int64, *string) error {
	panic("unexpected UpdateTotpSecret call")
}
func (s *pendingAuthHandlerUserRepoStub) EnableTotp(context.Context, int64) error {
	panic("unexpected EnableTotp call")
}
func (s *pendingAuthHandlerUserRepoStub) DisableTotp(context.Context, int64) error {
	panic("unexpected DisableTotp call")
}

func (s *pendingAuthHandlerUserRepoStub) CreatePendingAuthSession(_ context.Context, input service.PendingAuthSessionInput) (*service.PendingAuthSessionRecord, error) {
	s.nextSession++
	id := string(rune('a' + s.nextSession - 1))
	session := &service.PendingAuthSessionRecord{
		ID:              id,
		Intent:          input.Intent,
		ProviderType:    input.ProviderType,
		ProviderKey:     input.ProviderKey,
		ProviderSubject: input.ProviderSubject,
		Metadata:        map[string]any{},
		ExpiresAt:       time.Now().Add(10 * time.Minute),
	}
	if input.TargetUserID != nil {
		target := *input.TargetUserID
		session.TargetUserID = &target
	}
	s.sessions[id] = session
	return clonePendingAuthSession(session), nil
}

func (s *pendingAuthHandlerUserRepoStub) GetPendingAuthSessionByID(_ context.Context, sessionID string) (*service.PendingAuthSessionRecord, error) {
	session, ok := s.sessions[sessionID]
	if !ok {
		return nil, service.ErrUserNotFound
	}
	return clonePendingAuthSession(session), nil
}

func (s *pendingAuthHandlerUserRepoStub) UpdatePendingAuthSession(_ context.Context, session *service.PendingAuthSessionRecord) error {
	s.sessions[session.ID] = clonePendingAuthSession(session)
	return nil
}

func (s *pendingAuthHandlerUserRepoStub) BindPendingAuthIdentity(_ context.Context, session *service.PendingAuthSessionRecord, userID int64) error {
	user, ok := s.users[userID]
	if !ok || session == nil {
		return nil
	}

	provider := service.NormalizeExternalIdentityProvider(service.ExternalIdentityProvider(session.ProviderType))
	if provider == "" {
		return nil
	}
	for _, identity := range user.ExternalIdentities {
		if identity.Provider == provider && identity.ProviderUserID == session.ProviderSubject {
			return nil
		}
	}
	user.ExternalIdentities = append(user.ExternalIdentities, service.UserExternalIdentity{
		Provider:       provider,
		ProviderUserID: session.ProviderSubject,
	})
	return nil
}

func (s *pendingAuthHandlerUserRepoStub) ListUserExternalIdentities(_ context.Context, userID int64) ([]service.UserExternalIdentity, error) {
	user, ok := s.users[userID]
	if !ok {
		return nil, service.ErrUserNotFound
	}
	out := make([]service.UserExternalIdentity, len(user.ExternalIdentities))
	copy(out, user.ExternalIdentities)
	return out, nil
}

func (s *pendingAuthHandlerUserRepoStub) DeleteUserExternalIdentity(_ context.Context, userID int64, provider service.ExternalIdentityProvider) (bool, error) {
	user, ok := s.users[userID]
	if !ok {
		return false, service.ErrUserNotFound
	}
	provider = service.NormalizeExternalIdentityProvider(provider)
	filtered := make([]service.UserExternalIdentity, 0, len(user.ExternalIdentities))
	deleted := false
	for _, identity := range user.ExternalIdentities {
		if identity.Provider == provider {
			deleted = true
			continue
		}
		filtered = append(filtered, identity)
	}
	user.ExternalIdentities = filtered
	return deleted, nil
}

type pendingAuthRefreshCacheStub struct{}

func (pendingAuthRefreshCacheStub) StoreRefreshToken(context.Context, string, *service.RefreshTokenData, time.Duration) error {
	return nil
}
func (pendingAuthRefreshCacheStub) GetRefreshToken(context.Context, string) (*service.RefreshTokenData, error) {
	return nil, service.ErrRefreshTokenNotFound
}
func (pendingAuthRefreshCacheStub) DeleteRefreshToken(context.Context, string) error {
	return nil
}
func (pendingAuthRefreshCacheStub) DeleteUserRefreshTokens(context.Context, int64) error {
	return nil
}
func (pendingAuthRefreshCacheStub) DeleteTokenFamily(context.Context, string) error {
	return nil
}
func (pendingAuthRefreshCacheStub) AddToUserTokenSet(context.Context, int64, string, time.Duration) error {
	return nil
}
func (pendingAuthRefreshCacheStub) AddToFamilyTokenSet(context.Context, string, string, time.Duration) error {
	return nil
}
func (pendingAuthRefreshCacheStub) GetUserTokenHashes(context.Context, int64) ([]string, error) {
	return nil, nil
}
func (pendingAuthRefreshCacheStub) GetFamilyTokenHashes(context.Context, string) ([]string, error) {
	return nil, nil
}
func (pendingAuthRefreshCacheStub) IsTokenInFamily(context.Context, string, string) (bool, error) {
	return true, nil
}

type pendingAuthTotpCacheStub struct {
	loginSessions  map[string]*service.TotpLoginSession
	verifyAttempts map[int64]int
}

func newPendingAuthTotpCacheStub() *pendingAuthTotpCacheStub {
	return &pendingAuthTotpCacheStub{
		loginSessions:  make(map[string]*service.TotpLoginSession),
		verifyAttempts: make(map[int64]int),
	}
}

func (s *pendingAuthTotpCacheStub) GetSetupSession(context.Context, int64) (*service.TotpSetupSession, error) {
	return nil, nil
}
func (s *pendingAuthTotpCacheStub) SetSetupSession(context.Context, int64, *service.TotpSetupSession, time.Duration) error {
	return nil
}
func (s *pendingAuthTotpCacheStub) DeleteSetupSession(context.Context, int64) error {
	return nil
}
func (s *pendingAuthTotpCacheStub) GetLoginSession(_ context.Context, tempToken string) (*service.TotpLoginSession, error) {
	session, ok := s.loginSessions[tempToken]
	if !ok {
		return nil, nil
	}
	return session, nil
}
func (s *pendingAuthTotpCacheStub) SetLoginSession(_ context.Context, tempToken string, session *service.TotpLoginSession, _ time.Duration) error {
	s.loginSessions[tempToken] = session
	return nil
}
func (s *pendingAuthTotpCacheStub) DeleteLoginSession(_ context.Context, tempToken string) error {
	delete(s.loginSessions, tempToken)
	return nil
}
func (s *pendingAuthTotpCacheStub) IncrementVerifyAttempts(_ context.Context, userID int64) (int, error) {
	s.verifyAttempts[userID]++
	return s.verifyAttempts[userID], nil
}
func (s *pendingAuthTotpCacheStub) GetVerifyAttempts(_ context.Context, userID int64) (int, error) {
	return s.verifyAttempts[userID], nil
}
func (s *pendingAuthTotpCacheStub) ClearVerifyAttempts(_ context.Context, userID int64) error {
	delete(s.verifyAttempts, userID)
	return nil
}

type passthroughEncryptor struct{}

func (passthroughEncryptor) Encrypt(plaintext string) (string, error)  { return plaintext, nil }
func (passthroughEncryptor) Decrypt(ciphertext string) (string, error) { return ciphertext, nil }

func clonePendingAuthSession(in *service.PendingAuthSessionRecord) *service.PendingAuthSessionRecord {
	if in == nil {
		return nil
	}
	out := *in
	if in.Metadata != nil {
		out.Metadata = make(map[string]any, len(in.Metadata))
		for k, v := range in.Metadata {
			out.Metadata[k] = v
		}
	}
	if in.TargetUserID != nil {
		target := *in.TargetUserID
		out.TargetUserID = &target
	}
	if in.EmailVerifiedAt != nil {
		v := *in.EmailVerifiedAt
		out.EmailVerifiedAt = &v
	}
	if in.PasswordVerifiedAt != nil {
		v := *in.PasswordVerifiedAt
		out.PasswordVerifiedAt = &v
	}
	if in.TOTPVerifiedAt != nil {
		v := *in.TOTPVerifiedAt
		out.TOTPVerifiedAt = &v
	}
	if in.ConsumedAt != nil {
		v := *in.ConsumedAt
		out.ConsumedAt = &v
	}
	return &out
}

func newPendingAuthHandlerForTest(t *testing.T) (*AuthHandler, *service.AuthService, *pendingAuthHandlerUserRepoStub, *pendingAuthTotpCacheStub, string) {
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
		TotpEnabled:         true,
		TotpSecretEncrypted: &secret,
	}
	repo.users[user.ID] = user
	repo.usersByMail[user.Email] = user

	cfg := &config.Config{
		JWT: config.JWTConfig{
			Secret:                   "handler-pending-auth-secret",
			ExpireHour:               1,
			AccessTokenExpireMinutes: 60,
			RefreshTokenExpireDays:   7,
		},
	}
	settingSvc := service.NewSettingService(&pendingAuthSettingRepoStub{values: map[string]string{
		service.SettingKeyTotpEnabled: "true",
	}}, cfg)
	authSvc := service.NewAuthService(nil, repo, nil, pendingAuthRefreshCacheStub{}, cfg, settingSvc, nil, nil, nil, nil, nil)
	userSvc := service.NewUserService(repo, nil, nil, nil)
	totpCache := newPendingAuthTotpCacheStub()
	totpSvc := service.NewTotpService(repo, passthroughEncryptor{}, totpCache, settingSvc, nil, nil)
	handler := NewAuthHandler(cfg, authSvc, userSvc, settingSvc, nil, nil, totpSvc)

	sessionToken, err := authSvc.CreatePendingAuthSession(context.Background(), service.PendingAuthSessionInput{
		Intent:          service.PendingAuthIntentAdoptExistingUserByEmail,
		ProviderType:    "oidc",
		ProviderKey:     "issuer",
		ProviderSubject: "sub-1",
		TargetUserID:    &user.ID,
	})
	require.NoError(t, err)
	session, err := authSvc.GetPendingAuthSessionForProgress(context.Background(), sessionToken, nil)
	require.NoError(t, err)
	now := time.Now()
	session.EmailVerifiedAt = &now
	require.NoError(t, repo.UpdatePendingAuthSession(context.Background(), session))

	return handler, authSvc, repo, totpCache, sessionToken
}

func TestConfirmPendingAuthBind_WithTotpEnabledReturnsTempTokenAndKeepsSessionUsable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, authSvc, _, _, pendingToken := newPendingAuthHandlerForTest(t)

	body := bytes.NewBufferString(`{"pending_auth_token":"` + pendingToken + `","password":"password-123"}`)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/oauth/confirm-bind", body)
	ctx.Request.Header.Set("Content-Type", "application/json")

	handler.ConfirmPendingAuthBind(ctx)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp struct {
		Code int `json:"code"`
		Data struct {
			Requires2FA bool   `json:"requires_2fa"`
			TempToken   string `json:"temp_token"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.True(t, resp.Data.Requires2FA)
	require.NotEmpty(t, resp.Data.TempToken)

	session, err := authSvc.GetPendingAuthSessionForProgress(context.Background(), pendingToken, nil)
	require.NoError(t, err)
	require.NotNil(t, session.PasswordVerifiedAt)
	require.Nil(t, session.ConsumedAt)
}

func TestConfirmPendingAuthBind2FA_ConsumesPendingAuthSessionAfterValidTotp(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, authSvc, _, _, pendingToken := newPendingAuthHandlerForTest(t)

	stepOneBody := bytes.NewBufferString(`{"pending_auth_token":"` + pendingToken + `","password":"password-123"}`)
	stepOneRec := httptest.NewRecorder()
	stepOneCtx, _ := gin.CreateTestContext(stepOneRec)
	stepOneCtx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/oauth/confirm-bind", stepOneBody)
	stepOneCtx.Request.Header.Set("Content-Type", "application/json")
	handler.ConfirmPendingAuthBind(stepOneCtx)
	require.Equal(t, http.StatusOK, stepOneRec.Code)

	var firstResp struct {
		Data struct {
			TempToken string `json:"temp_token"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(stepOneRec.Body.Bytes(), &firstResp))
	require.NotEmpty(t, firstResp.Data.TempToken)

	code, err := totp.GenerateCode("JBSWY3DPEHPK3PXP", time.Now())
	require.NoError(t, err)

	stepTwoPayload := map[string]string{
		"pending_auth_token": pendingToken,
		"temp_token":         firstResp.Data.TempToken,
		"totp_code":          code,
	}
	payload, err := json.Marshal(stepTwoPayload)
	require.NoError(t, err)

	stepTwoRec := httptest.NewRecorder()
	stepTwoCtx, _ := gin.CreateTestContext(stepTwoRec)
	stepTwoCtx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/oauth/confirm-bind/2fa", bytes.NewReader(payload))
	stepTwoCtx.Request.Header.Set("Content-Type", "application/json")
	handler.ConfirmPendingAuthBind2FA(stepTwoCtx)

	require.Equal(t, http.StatusOK, stepTwoRec.Code)
	var secondResp struct {
		Code int `json:"code"`
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(stepTwoRec.Body.Bytes(), &secondResp))
	require.Equal(t, 0, secondResp.Code)
	require.NotEmpty(t, secondResp.Data.AccessToken)

	_, err = authSvc.GetPendingAuthSessionForProgress(context.Background(), pendingToken, nil)
	require.ErrorIs(t, err, service.ErrInvalidToken)
}

func TestConfirmPendingAuthBind2FA_WithInvalidTotpKeepsPendingAuthSessionUsable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, authSvc, _, totpCache, pendingToken := newPendingAuthHandlerForTest(t)

	stepOneBody := bytes.NewBufferString(`{"pending_auth_token":"` + pendingToken + `","password":"password-123"}`)
	stepOneRec := httptest.NewRecorder()
	stepOneCtx, _ := gin.CreateTestContext(stepOneRec)
	stepOneCtx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/oauth/confirm-bind", stepOneBody)
	stepOneCtx.Request.Header.Set("Content-Type", "application/json")
	handler.ConfirmPendingAuthBind(stepOneCtx)
	require.Equal(t, http.StatusOK, stepOneRec.Code)

	var firstResp struct {
		Data struct {
			TempToken string `json:"temp_token"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(stepOneRec.Body.Bytes(), &firstResp))
	require.NotEmpty(t, firstResp.Data.TempToken)

	validCode, err := totp.GenerateCode("JBSWY3DPEHPK3PXP", time.Now())
	require.NoError(t, err)
	invalidCode := validCode[:5] + "0"
	if invalidCode == validCode {
		invalidCode = validCode[:5] + "1"
	}

	stepTwoPayload := map[string]string{
		"pending_auth_token": pendingToken,
		"temp_token":         firstResp.Data.TempToken,
		"totp_code":          invalidCode,
	}
	payload, err := json.Marshal(stepTwoPayload)
	require.NoError(t, err)

	stepTwoRec := httptest.NewRecorder()
	stepTwoCtx, _ := gin.CreateTestContext(stepTwoRec)
	stepTwoCtx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/oauth/confirm-bind/2fa", bytes.NewReader(payload))
	stepTwoCtx.Request.Header.Set("Content-Type", "application/json")
	handler.ConfirmPendingAuthBind2FA(stepTwoCtx)

	require.Equal(t, http.StatusBadRequest, stepTwoRec.Code)

	session, err := authSvc.GetPendingAuthSessionForProgress(context.Background(), pendingToken, nil)
	require.NoError(t, err)
	require.NotNil(t, session.PasswordVerifiedAt)
	require.Nil(t, session.TOTPVerifiedAt)
	require.Nil(t, session.ConsumedAt)

	loginSession, err := totpCache.GetLoginSession(context.Background(), firstResp.Data.TempToken)
	require.NoError(t, err)
	require.NotNil(t, loginSession)
}

func TestCompleteOAuthCallback_BindCurrentUserPinsTargetUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, authSvc, _ := newOAuthCallbackHandlerForTest(t)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 7})
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/auth/oauth/linuxdo/callback", nil)

	handler.completeOAuthCallback(ctx, "/auth/linuxdo/callback", "/profile", oauthCallbackIdentity{
		Provider:             "linuxdo",
		Intent:               service.PendingAuthIntentBindCurrentUser,
		ProviderType:         "linuxdo",
		ProviderKey:          linuxDoOAuthProviderKey,
		ProviderSubject:      "subject-bind-1",
		SuggestedDisplayName: "Bind Owner",
	})

	require.Equal(t, http.StatusFound, rec.Code)
	fragment := parseRedirectFragment(t, rec.Header().Get("Location"))
	require.Equal(t, "pending_session", fragment.Get("auth_result"))
	require.Equal(t, service.PendingAuthIntentBindCurrentUser, fragment.Get("intent"))

	session, err := authSvc.GetPendingAuthSessionForProgress(context.Background(), fragment.Get("pending_auth_token"), nil)
	require.NoError(t, err)
	require.NotNil(t, session.TargetUserID)
	require.Equal(t, int64(7), *session.TargetUserID)
}

func TestCompleteOAuthCallback_BindCurrentUserPinnedSessionRejectsDifferentUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, authSvc, _ := newOAuthCallbackHandlerForTest(t)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 7})
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/auth/oauth/linuxdo/callback", nil)

	handler.completeOAuthCallback(ctx, "/auth/linuxdo/callback", "/profile", oauthCallbackIdentity{
		Provider:        "linuxdo",
		Intent:          service.PendingAuthIntentBindCurrentUser,
		ProviderType:    "linuxdo",
		ProviderKey:     linuxDoOAuthProviderKey,
		ProviderSubject: "subject-bind-2",
	})

	require.Equal(t, http.StatusFound, rec.Code)
	fragment := parseRedirectFragment(t, rec.Header().Get("Location"))

	_, err := authSvc.CompletePendingAuthSessionBind(context.Background(), fragment.Get("pending_auth_token"), 8)
	require.ErrorIs(t, err, service.ErrPendingAuthTargetMismatch)
}

func TestCompleteOAuthCallback_BindCurrentUserRequiresAuthenticatedSubject(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _, repo := newOAuthCallbackHandlerForTest(t)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/auth/oauth/linuxdo/callback", nil)

	handler.completeOAuthCallback(ctx, "/auth/linuxdo/callback", "/profile", oauthCallbackIdentity{
		Provider:        "linuxdo",
		Intent:          service.PendingAuthIntentBindCurrentUser,
		ProviderType:    "linuxdo",
		ProviderKey:     linuxDoOAuthProviderKey,
		ProviderSubject: "subject-bind-3",
	})

	require.Equal(t, http.StatusFound, rec.Code)
	fragment := parseRedirectFragment(t, rec.Header().Get("Location"))
	require.Equal(t, "auth_required", fragment.Get("error"))
	require.Equal(t, "missing authenticated subject", fragment.Get("error_message"))
	require.Empty(t, fragment.Get("pending_auth_token"))
	require.Empty(t, repo.sessions)
}

func TestCompleteOAuthCallback_BindCurrentUserAcceptsBindCookieToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, authSvc, repo := newOAuthCallbackHandlerForTest(t)

	user, err := repo.GetByID(context.Background(), 7)
	require.NoError(t, err)
	tokenPair, err := authSvc.GenerateTokenPair(context.Background(), user, "")
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oauth/linuxdo/callback", nil)
	req.AddCookie(&http.Cookie{Name: oauthBindAccessTokenCookieName, Value: tokenPair.AccessToken})
	ctx.Request = req

	handler.completeOAuthCallback(ctx, "/auth/linuxdo/callback", "/profile", oauthCallbackIdentity{
		Provider:        "linuxdo",
		Intent:          service.PendingAuthIntentBindCurrentUser,
		ProviderType:    "linuxdo",
		ProviderKey:     linuxDoOAuthProviderKey,
		ProviderSubject: "subject-bind-cookie",
	})

	require.Equal(t, http.StatusFound, rec.Code)
	fragment := parseRedirectFragment(t, rec.Header().Get("Location"))
	require.Equal(t, "pending_session", fragment.Get("auth_result"))
	require.NotEmpty(t, fragment.Get("pending_auth_token"))
}

func TestRedirectOAuthLoginSuccess_WithTotpEnabledReturnsTotpChallengeFragment(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _, _, totpCache, _ := newPendingAuthHandlerForTest(t)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/auth/oauth/linuxdo/callback", nil)

	handler.redirectOAuthLoginSuccess(ctx, "/auth/linuxdo/callback", "/profile", "linuxdo", service.PendingAuthIntentLogin, 7)

	require.Equal(t, http.StatusFound, rec.Code)
	fragment := parseRedirectFragment(t, rec.Header().Get("Location"))
	require.Equal(t, "true", fragment.Get("requires_2fa"))
	require.NotEmpty(t, fragment.Get("temp_token"))
	require.Empty(t, fragment.Get("access_token"))

	loginSession, err := totpCache.GetLoginSession(context.Background(), fragment.Get("temp_token"))
	require.NoError(t, err)
	require.NotNil(t, loginSession)
	require.Equal(t, int64(7), loginSession.UserID)
}
