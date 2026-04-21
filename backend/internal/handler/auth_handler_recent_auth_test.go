package handler

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/require"
)

type authHandlerSettingRepoStub struct {
	values map[string]string
}

func (s *authHandlerSettingRepoStub) Get(ctx context.Context, key string) (*service.Setting, error) {
	panic("unexpected Get call")
}

func (s *authHandlerSettingRepoStub) GetValue(ctx context.Context, key string) (string, error) {
	if value, ok := s.values[key]; ok {
		return value, nil
	}
	return "", service.ErrSettingNotFound
}

func (s *authHandlerSettingRepoStub) Set(ctx context.Context, key, value string) error {
	if s.values == nil {
		s.values = map[string]string{}
	}
	s.values[key] = value
	return nil
}

func (s *authHandlerSettingRepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := s.values[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}

func (s *authHandlerSettingRepoStub) SetMultiple(ctx context.Context, settings map[string]string) error {
	if s.values == nil {
		s.values = map[string]string{}
	}
	for key, value := range settings {
		s.values[key] = value
	}
	return nil
}

func (s *authHandlerSettingRepoStub) GetAll(ctx context.Context) (map[string]string, error) {
	out := make(map[string]string, len(s.values))
	for key, value := range s.values {
		out[key] = value
	}
	return out, nil
}

func (s *authHandlerSettingRepoStub) Delete(ctx context.Context, key string) error {
	delete(s.values, key)
	return nil
}

type authHandlerUserRepoStub struct {
	byID    map[int64]*service.User
	byEmail map[string]*service.User
}

func newAuthHandlerUserRepoStub() *authHandlerUserRepoStub {
	return &authHandlerUserRepoStub{
		byID:    map[int64]*service.User{},
		byEmail: map[string]*service.User{},
	}
}

func (s *authHandlerUserRepoStub) addUser(user *service.User) {
	clone := *user
	s.byID[user.ID] = &clone
	s.byEmail[user.Email] = &clone
}

func (s *authHandlerUserRepoStub) Create(ctx context.Context, user *service.User) error {
	return errors.New("not implemented")
}

func (s *authHandlerUserRepoStub) GetByID(ctx context.Context, id int64) (*service.User, error) {
	if user, ok := s.byID[id]; ok {
		clone := *user
		return &clone, nil
	}
	return nil, service.ErrUserNotFound
}

func (s *authHandlerUserRepoStub) GetByEmail(ctx context.Context, email string) (*service.User, error) {
	if user, ok := s.byEmail[email]; ok {
		clone := *user
		return &clone, nil
	}
	return nil, service.ErrUserNotFound
}

func (s *authHandlerUserRepoStub) GetFirstAdmin(ctx context.Context) (*service.User, error) {
	return nil, service.ErrUserNotFound
}

func (s *authHandlerUserRepoStub) Update(ctx context.Context, user *service.User) error {
	clone := *user
	s.byID[user.ID] = &clone
	s.byEmail[user.Email] = &clone
	return nil
}

func (s *authHandlerUserRepoStub) Delete(ctx context.Context, id int64) error {
	return errors.New("not implemented")
}

func (s *authHandlerUserRepoStub) List(ctx context.Context, params pagination.PaginationParams) ([]service.User, *pagination.PaginationResult, error) {
	return nil, nil, errors.New("not implemented")
}

func (s *authHandlerUserRepoStub) ListWithFilters(ctx context.Context, params pagination.PaginationParams, filters service.UserListFilters) ([]service.User, *pagination.PaginationResult, error) {
	return nil, nil, errors.New("not implemented")
}

func (s *authHandlerUserRepoStub) UpdateBalance(ctx context.Context, id int64, amount float64) error {
	return errors.New("not implemented")
}

func (s *authHandlerUserRepoStub) DeductBalance(ctx context.Context, id int64, amount float64) error {
	return errors.New("not implemented")
}

func (s *authHandlerUserRepoStub) UpdateConcurrency(ctx context.Context, id int64, amount int) error {
	return errors.New("not implemented")
}

func (s *authHandlerUserRepoStub) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	_, ok := s.byEmail[email]
	return ok, nil
}

func (s *authHandlerUserRepoStub) RemoveGroupFromAllowedGroups(ctx context.Context, groupID int64) (int64, error) {
	return 0, errors.New("not implemented")
}

func (s *authHandlerUserRepoStub) AddGroupToAllowedGroups(ctx context.Context, userID int64, groupID int64) error {
	return errors.New("not implemented")
}

func (s *authHandlerUserRepoStub) RemoveGroupFromUserAllowedGroups(ctx context.Context, userID int64, groupID int64) error {
	return errors.New("not implemented")
}

func (s *authHandlerUserRepoStub) UpdateTotpSecret(ctx context.Context, userID int64, encryptedSecret *string) error {
	return errors.New("not implemented")
}

func (s *authHandlerUserRepoStub) EnableTotp(ctx context.Context, userID int64) error {
	return errors.New("not implemented")
}

func (s *authHandlerUserRepoStub) DisableTotp(ctx context.Context, userID int64) error {
	return errors.New("not implemented")
}

type authHandlerTotpCacheStub struct {
	setupSessions  map[int64]*service.TotpSetupSession
	loginSessions  map[string]*service.TotpLoginSession
	verifyAttempts map[int64]int
}

func newAuthHandlerTotpCacheStub() *authHandlerTotpCacheStub {
	return &authHandlerTotpCacheStub{
		setupSessions:  map[int64]*service.TotpSetupSession{},
		loginSessions:  map[string]*service.TotpLoginSession{},
		verifyAttempts: map[int64]int{},
	}
}

func (s *authHandlerTotpCacheStub) GetSetupSession(ctx context.Context, userID int64) (*service.TotpSetupSession, error) {
	if session, ok := s.setupSessions[userID]; ok {
		copySession := *session
		return &copySession, nil
	}
	return nil, nil
}

func (s *authHandlerTotpCacheStub) SetSetupSession(ctx context.Context, userID int64, session *service.TotpSetupSession, ttl time.Duration) error {
	copySession := *session
	s.setupSessions[userID] = &copySession
	return nil
}

func (s *authHandlerTotpCacheStub) DeleteSetupSession(ctx context.Context, userID int64) error {
	delete(s.setupSessions, userID)
	return nil
}

func (s *authHandlerTotpCacheStub) GetLoginSession(ctx context.Context, tempToken string) (*service.TotpLoginSession, error) {
	if session, ok := s.loginSessions[tempToken]; ok {
		copySession := *session
		return &copySession, nil
	}
	return nil, nil
}

func (s *authHandlerTotpCacheStub) SetLoginSession(ctx context.Context, tempToken string, session *service.TotpLoginSession, ttl time.Duration) error {
	copySession := *session
	s.loginSessions[tempToken] = &copySession
	return nil
}

func (s *authHandlerTotpCacheStub) DeleteLoginSession(ctx context.Context, tempToken string) error {
	delete(s.loginSessions, tempToken)
	return nil
}

func (s *authHandlerTotpCacheStub) IncrementVerifyAttempts(ctx context.Context, userID int64) (int, error) {
	s.verifyAttempts[userID]++
	return s.verifyAttempts[userID], nil
}

func (s *authHandlerTotpCacheStub) GetVerifyAttempts(ctx context.Context, userID int64) (int, error) {
	return s.verifyAttempts[userID], nil
}

func (s *authHandlerTotpCacheStub) ClearVerifyAttempts(ctx context.Context, userID int64) error {
	delete(s.verifyAttempts, userID)
	return nil
}

type authHandlerEncryptorStub struct {
	decrypted string
}

func (s *authHandlerEncryptorStub) Encrypt(plaintext string) (string, error) {
	return "encrypted", nil
}

func (s *authHandlerEncryptorStub) Decrypt(ciphertext string) (string, error) {
	return s.decrypted, nil
}

type authHandlerPasskeyServiceStub struct {
	beginResult *service.PasskeyAuthenticationBeginResult
	beginErr    error
	finishUser  *service.User
	finishErr   error

	lastFlowID string
}

func (s *authHandlerPasskeyServiceStub) BeginRegistration(ctx context.Context, userID int64) (*service.PasskeyRegistrationBeginResult, error) {
	return nil, errors.New("not implemented")
}

func (s *authHandlerPasskeyServiceStub) FinishRegistration(ctx context.Context, userID int64, flowID, friendlyName string, request *http.Request) (*service.PasskeyRegistrationFinishResult, error) {
	return nil, errors.New("not implemented")
}

func (s *authHandlerPasskeyServiceStub) BeginAuthentication(ctx context.Context) (*service.PasskeyAuthenticationBeginResult, error) {
	if s.beginErr != nil {
		return nil, s.beginErr
	}
	if s.beginResult != nil {
		return s.beginResult, nil
	}
	return &service.PasskeyAuthenticationBeginResult{}, nil
}

func (s *authHandlerPasskeyServiceStub) FinishAuthentication(ctx context.Context, flowID string, request *http.Request) (*service.User, error) {
	s.lastFlowID = flowID
	if s.finishErr != nil {
		return nil, s.finishErr
	}
	return s.finishUser, nil
}

type handlerRecentAuthEntry struct {
	marker    *service.RecentAuthMarker
	expiresAt time.Time
}

type handlerAuthStateCacheStub struct {
	now        time.Time
	recentAuth map[int64]handlerRecentAuthEntry
}

func newHandlerAuthStateCacheStub() *handlerAuthStateCacheStub {
	return &handlerAuthStateCacheStub{recentAuth: map[int64]handlerRecentAuthEntry{}}
}

func (s *handlerAuthStateCacheStub) nowTime() time.Time {
	if s.now.IsZero() {
		return time.Now().UTC()
	}
	return s.now
}

func (s *handlerAuthStateCacheStub) SetPasskeyChallenge(ctx context.Context, flowID string, record *service.PasskeyChallengeRecord, ttl time.Duration) error {
	return nil
}

func (s *handlerAuthStateCacheStub) ConsumePasskeyChallenge(ctx context.Context, flowID string) (*service.PasskeyChallengeRecord, service.PasskeyChallengeConsumeStatus, error) {
	return nil, service.PasskeyChallengeConsumeMissing, nil
}

func (s *handlerAuthStateCacheStub) SetRecentAuthMarker(ctx context.Context, userID int64, marker *service.RecentAuthMarker, ttl time.Duration) error {
	copyMarker := *marker
	s.recentAuth[userID] = handlerRecentAuthEntry{marker: &copyMarker, expiresAt: s.nowTime().Add(ttl)}
	return nil
}

func (s *handlerAuthStateCacheStub) GetRecentAuthMarker(ctx context.Context, userID int64) (*service.RecentAuthMarker, error) {
	entry, ok := s.recentAuth[userID]
	if !ok {
		return nil, nil
	}
	if s.nowTime().After(entry.expiresAt) {
		delete(s.recentAuth, userID)
		return nil, nil
	}
	copyMarker := *entry.marker
	return &copyMarker, nil
}

func newAuthHandlerConfig() *config.Config {
	return &config.Config{
		Server: config.ServerConfig{Mode: "debug"},
		JWT: config.JWTConfig{
			Secret:     "test-secret",
			ExpireHour: 1,
		},
	}
}

func TestAuthHandler_Login_IssuesRecentAuthMarker(t *testing.T) {
	gin.SetMode(gin.TestMode)

	userRepo := newAuthHandlerUserRepoStub()
	user := &service.User{ID: 101, Email: "user@example.com", Role: service.RoleUser, Status: service.StatusActive}
	require.NoError(t, user.SetPassword("password123"))
	userRepo.addUser(user)

	settingSvc := service.NewSettingService(&authHandlerSettingRepoStub{values: map[string]string{
		service.SettingKeyTotpEnabled:        "false",
		service.SettingKeyBackendModeEnabled: "false",
	}}, newAuthHandlerConfig())
	authSvc := service.NewAuthService(nil, userRepo, nil, nil, newAuthHandlerConfig(), settingSvc, nil, nil, nil, nil, nil)
	userSvc := service.NewUserService(userRepo, nil, nil, nil)
	recentCache := newHandlerAuthStateCacheStub()
	recentAuthSvc := service.NewRecentAuthService(recentCache)

	h := NewAuthHandler(newAuthHandlerConfig(), authSvc, userSvc, settingSvc, nil, nil, nil, recentAuthSvc)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"email":"user@example.com","password":"password123"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Login(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	marker, err := recentCache.GetRecentAuthMarker(context.Background(), user.ID)
	require.NoError(t, err)
	require.NotNil(t, marker)
	require.Equal(t, service.RecentAuthMethodPassword, marker.Method)
}

func TestAuthHandler_Login2FA_IssuesRecentAuthMarker(t *testing.T) {
	gin.SetMode(gin.TestMode)

	userRepo := newAuthHandlerUserRepoStub()
	key, err := totp.Generate(totp.GenerateOpts{Issuer: "Sub2API", AccountName: "totp@example.com"})
	require.NoError(t, err)
	encryptedSecret := "encrypted-secret"
	user := &service.User{
		ID:                  202,
		Email:               "totp@example.com",
		Role:                service.RoleUser,
		Status:              service.StatusActive,
		TotpEnabled:         true,
		TotpSecretEncrypted: &encryptedSecret,
	}
	require.NoError(t, user.SetPassword("password123"))
	userRepo.addUser(user)

	settingSvc := service.NewSettingService(&authHandlerSettingRepoStub{values: map[string]string{
		service.SettingKeyTotpEnabled:        "true",
		service.SettingKeyBackendModeEnabled: "false",
	}}, newAuthHandlerConfig())
	totpSvc := service.NewTotpService(userRepo, &authHandlerEncryptorStub{decrypted: key.Secret()}, newAuthHandlerTotpCacheStub(), settingSvc, nil, nil)
	tempToken, err := totpSvc.CreateLoginSession(context.Background(), user.ID, user.Email)
	require.NoError(t, err)
	code, err := totp.GenerateCode(key.Secret(), time.Now())
	require.NoError(t, err)

	authSvc := service.NewAuthService(nil, userRepo, nil, nil, newAuthHandlerConfig(), settingSvc, nil, nil, nil, nil, nil)
	userSvc := service.NewUserService(userRepo, nil, nil, nil)
	recentCache := newHandlerAuthStateCacheStub()
	recentAuthSvc := service.NewRecentAuthService(recentCache)

	h := NewAuthHandler(newAuthHandlerConfig(), authSvc, userSvc, settingSvc, nil, nil, totpSvc, recentAuthSvc)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login/2fa", bytes.NewBufferString(`{"temp_token":"`+tempToken+`","totp_code":"`+code+`"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Login2FA(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	marker, err := recentCache.GetRecentAuthMarker(context.Background(), user.ID)
	require.NoError(t, err)
	require.NotNil(t, marker)
	require.Equal(t, service.RecentAuthMethodPasswordTOTP, marker.Method)
}

func TestAuthHandler_FinishPasskeyAuthentication_IssuesRecentAuthMarker(t *testing.T) {
	gin.SetMode(gin.TestMode)

	userRepo := newAuthHandlerUserRepoStub()
	user := &service.User{ID: 303, Email: "passkey@example.com", Role: service.RoleUser, Status: service.StatusActive}
	userRepo.addUser(user)

	setttingValues := map[string]string{
		service.SettingKeyBackendModeEnabled: "false",
	}
	settingSvc := service.NewSettingService(&authHandlerSettingRepoStub{values: setttingValues}, newAuthHandlerConfig())
	authSvc := service.NewAuthService(nil, userRepo, nil, nil, newAuthHandlerConfig(), settingSvc, nil, nil, nil, nil, nil)
	userSvc := service.NewUserService(userRepo, nil, nil, nil)
	recentCache := newHandlerAuthStateCacheStub()
	recentAuthSvc := service.NewRecentAuthService(recentCache)

	h := NewAuthHandler(newAuthHandlerConfig(), authSvc, userSvc, settingSvc, nil, nil, nil, recentAuthSvc)
	passkeyStub := &authHandlerPasskeyServiceStub{finishUser: user}
	h.passkeyService = passkeyStub

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/passkeys/login/finish?flow_id=flow-auth-1", bytes.NewBufferString(`{"id":"credential"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.FinishPasskeyAuthentication(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "flow-auth-1", passkeyStub.lastFlowID)
	marker, err := recentCache.GetRecentAuthMarker(context.Background(), user.ID)
	require.NoError(t, err)
	require.NotNil(t, marker)
	require.Equal(t, service.RecentAuthMethodPasskey, marker.Method)
}

func TestAuthHandler_FinishPasskeyAuthentication_BackendModeRejectsNonAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	userRepo := newAuthHandlerUserRepoStub()
	user := &service.User{ID: 404, Email: "backend-user@example.com", Role: service.RoleUser, Status: service.StatusActive}
	userRepo.addUser(user)

	settingSvc := service.NewSettingService(&authHandlerSettingRepoStub{values: map[string]string{
		service.SettingKeyBackendModeEnabled: "true",
	}}, newAuthHandlerConfig())
	settings, err := settingSvc.GetAllSettings(context.Background())
	require.NoError(t, err)
	settings.BackendModeEnabled = true
	require.NoError(t, settingSvc.UpdateSettings(context.Background(), settings))
	authSvc := service.NewAuthService(nil, userRepo, nil, nil, newAuthHandlerConfig(), settingSvc, nil, nil, nil, nil, nil)
	userSvc := service.NewUserService(userRepo, nil, nil, nil)
	recentCache := newHandlerAuthStateCacheStub()
	recentAuthSvc := service.NewRecentAuthService(recentCache)

	h := NewAuthHandler(newAuthHandlerConfig(), authSvc, userSvc, settingSvc, nil, nil, nil, recentAuthSvc)
	h.passkeyService = &authHandlerPasskeyServiceStub{finishUser: user}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/passkeys/login/finish?flow_id=flow-auth-2", bytes.NewBufferString(`{"id":"credential"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.FinishPasskeyAuthentication(c)

	require.Equal(t, http.StatusForbidden, recorder.Code)
	marker, err := recentCache.GetRecentAuthMarker(context.Background(), user.ID)
	require.NoError(t, err)
	require.Nil(t, marker)
}
