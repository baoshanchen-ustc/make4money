package routes

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type passkeyRouteSettingRepo struct {
	values map[string]string
}

func (r *passkeyRouteSettingRepo) Get(context.Context, string) (*service.Setting, error) {
	panic("unexpected Get call")
}

func (r *passkeyRouteSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	if value, ok := r.values[key]; ok {
		return value, nil
	}
	return "", service.ErrSettingNotFound
}

func (r *passkeyRouteSettingRepo) Set(context.Context, string, string) error {
	panic("unexpected Set call")
}

func (r *passkeyRouteSettingRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	result := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := r.values[key]; ok {
			result[key] = value
		}
	}
	return result, nil
}

func (r *passkeyRouteSettingRepo) SetMultiple(_ context.Context, settings map[string]string) error {
	if r.values == nil {
		r.values = make(map[string]string, len(settings))
	}
	for key, value := range settings {
		r.values[key] = value
	}
	return nil
}

func (r *passkeyRouteSettingRepo) GetAll(context.Context) (map[string]string, error) {
	result := make(map[string]string, len(r.values))
	for key, value := range r.values {
		result[key] = value
	}
	return result, nil
}

func (r *passkeyRouteSettingRepo) Delete(context.Context, string) error {
	return nil
}

type passkeyRouteAuthStateCacheStub struct {
	recentAuth map[int64]*service.RecentAuthMarker
}

func newPasskeyRouteAuthStateCacheStub() *passkeyRouteAuthStateCacheStub {
	return &passkeyRouteAuthStateCacheStub{recentAuth: map[int64]*service.RecentAuthMarker{}}
}

func (s *passkeyRouteAuthStateCacheStub) SetPasskeyChallenge(context.Context, string, *service.PasskeyChallengeRecord, time.Duration) error {
	return nil
}

func (s *passkeyRouteAuthStateCacheStub) ConsumePasskeyChallenge(context.Context, string) (*service.PasskeyChallengeRecord, service.PasskeyChallengeConsumeStatus, error) {
	return nil, service.PasskeyChallengeConsumeMissing, nil
}

func (s *passkeyRouteAuthStateCacheStub) SetRecentAuthMarker(_ context.Context, userID int64, marker *service.RecentAuthMarker, _ time.Duration) error {
	copyMarker := *marker
	s.recentAuth[userID] = &copyMarker
	return nil
}

func (s *passkeyRouteAuthStateCacheStub) GetRecentAuthMarker(_ context.Context, userID int64) (*service.RecentAuthMarker, error) {
	if marker, ok := s.recentAuth[userID]; ok {
		copyMarker := *marker
		return &copyMarker, nil
	}
	return nil, nil
}

type passkeyRouteRefreshTokenCacheStub struct {
	refreshTokens map[string]*service.RefreshTokenData
}

func newPasskeyRouteRefreshTokenCacheStub() *passkeyRouteRefreshTokenCacheStub {
	return &passkeyRouteRefreshTokenCacheStub{refreshTokens: map[string]*service.RefreshTokenData{}}
}

func (s *passkeyRouteRefreshTokenCacheStub) StoreRefreshToken(_ context.Context, tokenHash string, data *service.RefreshTokenData, _ time.Duration) error {
	copyData := *data
	s.refreshTokens[tokenHash] = &copyData
	return nil
}

func (s *passkeyRouteRefreshTokenCacheStub) GetRefreshToken(_ context.Context, tokenHash string) (*service.RefreshTokenData, error) {
	if data, ok := s.refreshTokens[tokenHash]; ok {
		copyData := *data
		return &copyData, nil
	}
	return nil, service.ErrRefreshTokenNotFound
}

func (s *passkeyRouteRefreshTokenCacheStub) DeleteRefreshToken(_ context.Context, tokenHash string) error {
	delete(s.refreshTokens, tokenHash)
	return nil
}

func (s *passkeyRouteRefreshTokenCacheStub) DeleteUserRefreshTokens(context.Context, int64) error {
	return nil
}

func (s *passkeyRouteRefreshTokenCacheStub) DeleteTokenFamily(context.Context, string) error {
	return nil
}

func (s *passkeyRouteRefreshTokenCacheStub) AddToUserTokenSet(context.Context, int64, string, time.Duration) error {
	return nil
}

func (s *passkeyRouteRefreshTokenCacheStub) AddToFamilyTokenSet(context.Context, string, string, time.Duration) error {
	return nil
}

func (s *passkeyRouteRefreshTokenCacheStub) GetUserTokenHashes(context.Context, int64) ([]string, error) {
	return nil, nil
}

func (s *passkeyRouteRefreshTokenCacheStub) GetFamilyTokenHashes(context.Context, string) ([]string, error) {
	return nil, nil
}

func (s *passkeyRouteRefreshTokenCacheStub) IsTokenInFamily(context.Context, string, string) (bool, error) {
	return false, nil
}

type passkeyRouteServiceStub struct {
	beginRegistrationResult   *service.PasskeyRegistrationBeginResult
	finishRegistrationResult  *service.PasskeyRegistrationFinishResult
	beginAuthenticationResult *service.PasskeyAuthenticationBeginResult
	finishAuthenticationUser  *service.User
	statusResult              *service.PasskeyManagementStatus
	listResult                *service.PasskeyManagementListResult
	renameResult              *service.PasskeyManagementCredential
	revokeResult              *service.PasskeyManagementRevokeResult

	lastBeginRegistrationUserID        int64
	lastFinishRegistrationUserID       int64
	lastFinishRegistrationFlowID       string
	lastFinishRegistrationFriendlyName string
	lastFinishRegistrationBody         string
	lastFinishAuthenticationFlowID     string
	lastFinishAuthenticationBody       string
	lastListUserID                     int64
	lastStatusUserID                   int64
	lastRenameUserID                   int64
	lastRenameCredentialID             string
	lastRenameFriendlyName             string
	lastRevokeUserID                   int64
	lastRevokeCredentialID             string
	beginAuthenticationCalls           int
}

func (s *passkeyRouteServiceStub) BeginRegistration(_ context.Context, userID int64) (*service.PasskeyRegistrationBeginResult, error) {
	s.lastBeginRegistrationUserID = userID
	if s.beginRegistrationResult == nil {
		return &service.PasskeyRegistrationBeginResult{}, nil
	}
	return s.beginRegistrationResult, nil
}

func (s *passkeyRouteServiceStub) FinishRegistration(_ context.Context, userID int64, flowID, friendlyName string, request *http.Request) (*service.PasskeyRegistrationFinishResult, error) {
	body, err := io.ReadAll(request.Body)
	if err != nil {
		return nil, err
	}
	s.lastFinishRegistrationUserID = userID
	s.lastFinishRegistrationFlowID = flowID
	s.lastFinishRegistrationFriendlyName = friendlyName
	s.lastFinishRegistrationBody = string(body)
	if s.finishRegistrationResult == nil {
		return &service.PasskeyRegistrationFinishResult{}, nil
	}
	return s.finishRegistrationResult, nil
}

func (s *passkeyRouteServiceStub) BeginAuthentication(context.Context) (*service.PasskeyAuthenticationBeginResult, error) {
	s.beginAuthenticationCalls++
	if s.beginAuthenticationResult == nil {
		return &service.PasskeyAuthenticationBeginResult{}, nil
	}
	return s.beginAuthenticationResult, nil
}

func (s *passkeyRouteServiceStub) FinishAuthentication(_ context.Context, flowID string, request *http.Request) (*service.User, error) {
	body, err := io.ReadAll(request.Body)
	if err != nil {
		return nil, err
	}
	s.lastFinishAuthenticationFlowID = flowID
	s.lastFinishAuthenticationBody = string(body)
	return s.finishAuthenticationUser, nil
}

func (s *passkeyRouteServiceStub) GetManagementStatus(_ context.Context, userID int64) (*service.PasskeyManagementStatus, error) {
	s.lastStatusUserID = userID
	if s.statusResult == nil {
		return &service.PasskeyManagementStatus{}, nil
	}
	return s.statusResult, nil
}

func (s *passkeyRouteServiceStub) ListManagementCredentials(_ context.Context, userID int64) (*service.PasskeyManagementListResult, error) {
	s.lastListUserID = userID
	if s.listResult == nil {
		return &service.PasskeyManagementListResult{}, nil
	}
	return s.listResult, nil
}

func (s *passkeyRouteServiceStub) RenameCredential(_ context.Context, userID int64, credentialID, friendlyName string) (*service.PasskeyManagementCredential, error) {
	s.lastRenameUserID = userID
	s.lastRenameCredentialID = credentialID
	s.lastRenameFriendlyName = friendlyName
	if s.renameResult == nil {
		return &service.PasskeyManagementCredential{}, nil
	}
	return s.renameResult, nil
}

func (s *passkeyRouteServiceStub) RevokeCredential(_ context.Context, userID int64, credentialID string) (*service.PasskeyManagementRevokeResult, error) {
	s.lastRevokeUserID = userID
	s.lastRevokeCredentialID = credentialID
	if s.revokeResult == nil {
		return &service.PasskeyManagementRevokeResult{}, nil
	}
	return s.revokeResult, nil
}

type passkeyRouteResponseEnvelope[T any] struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

func newPasskeyRouteConfig() *config.Config {
	return &config.Config{
		Server: config.ServerConfig{Mode: "debug"},
		JWT: config.JWTConfig{
			Secret:                 "passkey-route-test-secret",
			ExpireHour:             1,
			RefreshTokenExpireDays: 7,
		},
	}
}

func newPasskeyRouteSettingService(t *testing.T, backendModeEnabled bool) *service.SettingService {
	t.Helper()
	repo := &passkeyRouteSettingRepo{values: map[string]string{
		service.SettingKeyBackendModeEnabled: strings.ToLower(strconv.FormatBool(backendModeEnabled)),
	}}
	svc := service.NewSettingService(repo, newPasskeyRouteConfig())
	settings, err := svc.GetAllSettings(context.Background())
	require.NoError(t, err)
	settings.BackendModeEnabled = backendModeEnabled
	require.NoError(t, svc.UpdateSettings(context.Background(), settings))
	return svc
}

func setPasskeyRouteField(target any, fieldName string, value any) {
	field := reflect.ValueOf(target).Elem().FieldByName(fieldName)
	reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Set(reflect.ValueOf(value))
}

func decodePasskeyRouteResponse[T any](t *testing.T, recorder *httptest.ResponseRecorder) passkeyRouteResponseEnvelope[T] {
	t.Helper()
	var envelope passkeyRouteResponseEnvelope[T]
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	return envelope
}

func newPasskeyUserRoutesTestRouter(t *testing.T, stub *passkeyRouteServiceStub) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	settingSvc := newPasskeyRouteSettingService(t, false)
	recentAuthSvc := service.NewRecentAuthService(newPasskeyRouteAuthStateCacheStub())
	authSvc := service.NewAuthService(nil, nil, nil, newPasskeyRouteRefreshTokenCacheStub(), newPasskeyRouteConfig(), settingSvc, nil, nil, nil, nil, nil)
	authHandler := handler.NewAuthHandler(newPasskeyRouteConfig(), authSvc, nil, settingSvc, nil, nil, nil, recentAuthSvc)
	passkeyHandler := &handler.PasskeyHandler{}
	setPasskeyRouteField(authHandler, "passkeyService", stub)
	setPasskeyRouteField(passkeyHandler, "passkeyService", stub)

	router := gin.New()
	v1 := router.Group("/api/v1")
	RegisterUserRoutes(
		v1,
		&handler.Handlers{
			Auth:    authHandler,
			Passkey: passkeyHandler,
		},
		servermiddleware.JWTAuthMiddleware(func(c *gin.Context) {
			c.Set(string(servermiddleware.ContextKeyUser), servermiddleware.AuthSubject{UserID: 42, Concurrency: 1})
			c.Set(string(servermiddleware.ContextKeyUserRole), service.RoleUser)
			c.Next()
		}),
		nil,
	)
	return router
}

func TestUserPasskeyRoutesRegistered(t *testing.T) {
	router := newPasskeyUserRoutesTestRouter(t, &passkeyRouteServiceStub{})
	routesByMethodAndPath := make(map[string]struct{}, len(router.Routes()))
	for _, route := range router.Routes() {
		routesByMethodAndPath[route.Method+" "+route.Path] = struct{}{}
	}

	expected := []string{
		http.MethodPost + " /api/v1/user/passkeys/register/begin",
		http.MethodPost + " /api/v1/user/passkeys/register/finish",
		http.MethodGet + " /api/v1/user/passkeys/status",
		http.MethodGet + " /api/v1/user/passkeys",
		http.MethodPut + " /api/v1/user/passkeys/:credentialId",
		http.MethodDelete + " /api/v1/user/passkeys/:credentialId",
	}

	for _, route := range expected {
		_, ok := routesByMethodAndPath[route]
		require.True(t, ok, "missing route %s", route)
	}
}

func TestUserPasskeyRoutesReturnMeaningfulResponses(t *testing.T) {
	createdAt := time.Date(2026, time.March, 29, 12, 0, 0, 0, time.UTC)
	lastUsedAt := createdAt.Add(2 * time.Hour)
	revokedAt := createdAt.Add(4 * time.Hour)

	tests := []struct {
		name   string
		method string
		path   string
		body   string
		stub   *passkeyRouteServiceStub
		assert func(t *testing.T, recorder *httptest.ResponseRecorder, stub *passkeyRouteServiceStub)
	}{
		{
			name:   "register begin",
			method: http.MethodPost,
			path:   "/api/v1/user/passkeys/register/begin",
			body:   `{}`,
			stub:   &passkeyRouteServiceStub{beginRegistrationResult: &service.PasskeyRegistrationBeginResult{FlowID: "register-flow", Countdown: 300}},
			assert: func(t *testing.T, recorder *httptest.ResponseRecorder, stub *passkeyRouteServiceStub) {
				envelope := decodePasskeyRouteResponse[service.PasskeyRegistrationBeginResult](t, recorder)
				require.Equal(t, 0, envelope.Code)
				require.Equal(t, "success", envelope.Message)
				require.Equal(t, "register-flow", envelope.Data.FlowID)
				require.Equal(t, 300, envelope.Data.Countdown)
				require.Equal(t, int64(42), stub.lastBeginRegistrationUserID)
			},
		},
		{
			name:   "register finish",
			method: http.MethodPost,
			path:   "/api/v1/user/passkeys/register/finish?flow_id=reg-flow-1&friendly_name=Office%20Key",
			body:   `{"id":"credential-1"}`,
			stub:   &passkeyRouteServiceStub{finishRegistrationResult: &service.PasskeyRegistrationFinishResult{CredentialID: "credential-1", FriendlyName: "Office Key"}},
			assert: func(t *testing.T, recorder *httptest.ResponseRecorder, stub *passkeyRouteServiceStub) {
				envelope := decodePasskeyRouteResponse[service.PasskeyRegistrationFinishResult](t, recorder)
				require.Equal(t, "credential-1", envelope.Data.CredentialID)
				require.Equal(t, "Office Key", envelope.Data.FriendlyName)
				require.Equal(t, int64(42), stub.lastFinishRegistrationUserID)
				require.Equal(t, "reg-flow-1", stub.lastFinishRegistrationFlowID)
				require.Equal(t, "Office Key", stub.lastFinishRegistrationFriendlyName)
				require.JSONEq(t, `{"id":"credential-1"}`, stub.lastFinishRegistrationBody)
			},
		},
		{
			name:   "list",
			method: http.MethodGet,
			path:   "/api/v1/user/passkeys",
			stub:   &passkeyRouteServiceStub{listResult: &service.PasskeyManagementListResult{Items: []service.PasskeyManagementCredential{{CredentialID: "cred-a", FriendlyName: "Laptop", CreatedAt: createdAt, LastUsedAt: &lastUsedAt, BackupEligible: true, Synced: true}}}},
			assert: func(t *testing.T, recorder *httptest.ResponseRecorder, stub *passkeyRouteServiceStub) {
				envelope := decodePasskeyRouteResponse[handler.PasskeyListResponse](t, recorder)
				require.Len(t, envelope.Data.Items, 1)
				require.Equal(t, "cred-a", envelope.Data.Items[0].CredentialID)
				require.Equal(t, "Laptop", envelope.Data.Items[0].FriendlyName)
				require.Equal(t, createdAt.Unix(), envelope.Data.Items[0].CreatedAt)
				require.NotNil(t, envelope.Data.Items[0].LastUsedAt)
				require.Equal(t, lastUsedAt.Unix(), *envelope.Data.Items[0].LastUsedAt)
				require.True(t, envelope.Data.Items[0].BackupEligible)
				require.True(t, envelope.Data.Items[0].Synced)
				require.Equal(t, int64(42), stub.lastListUserID)
			},
		},
		{
			name:   "rename",
			method: http.MethodPut,
			path:   "/api/v1/user/passkeys/cred-b",
			body:   `{"friendly_name":"Travel Key"}`,
			stub:   &passkeyRouteServiceStub{renameResult: &service.PasskeyManagementCredential{CredentialID: "cred-b", FriendlyName: "Travel Key", CreatedAt: createdAt, BackupEligible: false, Synced: false}},
			assert: func(t *testing.T, recorder *httptest.ResponseRecorder, stub *passkeyRouteServiceStub) {
				envelope := decodePasskeyRouteResponse[handler.PasskeyRenameResponse](t, recorder)
				require.Equal(t, "cred-b", envelope.Data.Credential.CredentialID)
				require.Equal(t, "Travel Key", envelope.Data.Credential.FriendlyName)
				require.Equal(t, createdAt.Unix(), envelope.Data.Credential.CreatedAt)
				require.Equal(t, int64(42), stub.lastRenameUserID)
				require.Equal(t, "cred-b", stub.lastRenameCredentialID)
				require.Equal(t, "Travel Key", stub.lastRenameFriendlyName)
			},
		},
		{
			name:   "revoke",
			method: http.MethodDelete,
			path:   "/api/v1/user/passkeys/cred-c",
			stub:   &passkeyRouteServiceStub{revokeResult: &service.PasskeyManagementRevokeResult{CredentialID: "cred-c", RevokedAt: revokedAt, PasswordFallbackAvailable: true}},
			assert: func(t *testing.T, recorder *httptest.ResponseRecorder, stub *passkeyRouteServiceStub) {
				envelope := decodePasskeyRouteResponse[handler.PasskeyRevokeResponse](t, recorder)
				require.True(t, envelope.Data.Success)
				require.Equal(t, "cred-c", envelope.Data.CredentialID)
				require.Equal(t, revokedAt.Unix(), envelope.Data.RevokedAt)
				require.True(t, envelope.Data.PasswordFallbackAvailable)
				require.Equal(t, int64(42), stub.lastRevokeUserID)
				require.Equal(t, "cred-c", stub.lastRevokeCredentialID)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			router := newPasskeyUserRoutesTestRouter(t, tc.stub)
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)

			require.Equal(t, http.StatusOK, recorder.Code)
			tc.assert(t, recorder, tc.stub)
		})
	}
}
