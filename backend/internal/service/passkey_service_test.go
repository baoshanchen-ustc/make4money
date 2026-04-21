package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type passkeySvcSettingRepoStub struct {
	all map[string]string
}

func (s *passkeySvcSettingRepoStub) Get(ctx context.Context, key string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *passkeySvcSettingRepoStub) GetValue(ctx context.Context, key string) (string, error) {
	if v, ok := s.all[key]; ok {
		return v, nil
	}
	return "", ErrSettingNotFound
}

func (s *passkeySvcSettingRepoStub) Set(ctx context.Context, key, value string) error {
	panic("unexpected Set call")
}

func (s *passkeySvcSettingRepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	panic("unexpected GetMultiple call")
}

func (s *passkeySvcSettingRepoStub) SetMultiple(ctx context.Context, settings map[string]string) error {
	panic("unexpected SetMultiple call")
}

func (s *passkeySvcSettingRepoStub) GetAll(ctx context.Context) (map[string]string, error) {
	cloned := make(map[string]string, len(s.all))
	for key, value := range s.all {
		cloned[key] = value
	}
	return cloned, nil
}

func (s *passkeySvcSettingRepoStub) Delete(ctx context.Context, key string) error {
	panic("unexpected Delete call")
}

type passkeySvcChallengeEntry struct {
	record    *PasskeyChallengeRecord
	expiresAt time.Time
}

type passkeySvcRecentAuthEntry struct {
	marker    *RecentAuthMarker
	expiresAt time.Time
}

type passkeySvcAuthStateCacheStub struct {
	now         time.Time
	challenges  map[string]passkeySvcChallengeEntry
	replay      map[string]struct{}
	recentAuths map[int64]passkeySvcRecentAuthEntry
}

func newPasskeySvcAuthStateCacheStub() *passkeySvcAuthStateCacheStub {
	return &passkeySvcAuthStateCacheStub{
		challenges:  map[string]passkeySvcChallengeEntry{},
		replay:      map[string]struct{}{},
		recentAuths: map[int64]passkeySvcRecentAuthEntry{},
	}
}

func (s *passkeySvcAuthStateCacheStub) currentTime() time.Time {
	if s.now.IsZero() {
		return time.Now().UTC()
	}
	return s.now
}

func (s *passkeySvcAuthStateCacheStub) SetPasskeyChallenge(ctx context.Context, flowID string, record *PasskeyChallengeRecord, ttl time.Duration) error {
	recordCopy := *record
	s.challenges[flowID] = passkeySvcChallengeEntry{
		record:    &recordCopy,
		expiresAt: s.currentTime().Add(ttl),
	}
	return nil
}

func (s *passkeySvcAuthStateCacheStub) ConsumePasskeyChallenge(ctx context.Context, flowID string) (*PasskeyChallengeRecord, PasskeyChallengeConsumeStatus, error) {
	if entry, ok := s.challenges[flowID]; ok {
		if s.currentTime().After(entry.expiresAt) {
			delete(s.challenges, flowID)
			return nil, PasskeyChallengeConsumeMissing, nil
		}
		delete(s.challenges, flowID)
		s.replay[flowID] = struct{}{}
		recordCopy := *entry.record
		return &recordCopy, PasskeyChallengeConsumeFound, nil
	}

	if _, ok := s.replay[flowID]; ok {
		return nil, PasskeyChallengeConsumeReplayed, nil
	}

	return nil, PasskeyChallengeConsumeMissing, nil
}

func (s *passkeySvcAuthStateCacheStub) SetRecentAuthMarker(ctx context.Context, userID int64, marker *RecentAuthMarker, ttl time.Duration) error {
	markerCopy := *marker
	s.recentAuths[userID] = passkeySvcRecentAuthEntry{
		marker:    &markerCopy,
		expiresAt: s.currentTime().Add(ttl),
	}
	return nil
}

func (s *passkeySvcAuthStateCacheStub) GetRecentAuthMarker(ctx context.Context, userID int64) (*RecentAuthMarker, error) {
	entry, ok := s.recentAuths[userID]
	if !ok {
		return nil, nil
	}
	if s.currentTime().After(entry.expiresAt) {
		delete(s.recentAuths, userID)
		return nil, nil
	}
	markerCopy := *entry.marker
	return &markerCopy, nil
}

type passkeySvcUserRepoStub struct {
	byID map[int64]*User
}

func newPasskeySvcUserRepoStub(users ...*User) *passkeySvcUserRepoStub {
	stub := &passkeySvcUserRepoStub{byID: map[int64]*User{}}
	for _, user := range users {
		if user == nil {
			continue
		}
		clone := *user
		stub.byID[user.ID] = &clone
	}
	return stub
}

func (s *passkeySvcUserRepoStub) Create(ctx context.Context, user *User) error {
	return errors.New("not implemented")
}

func (s *passkeySvcUserRepoStub) GetByID(ctx context.Context, id int64) (*User, error) {
	if user, ok := s.byID[id]; ok {
		clone := *user
		return &clone, nil
	}
	return nil, ErrUserNotFound
}

func (s *passkeySvcUserRepoStub) GetByEmail(ctx context.Context, email string) (*User, error) {
	return nil, ErrUserNotFound
}

func (s *passkeySvcUserRepoStub) GetFirstAdmin(ctx context.Context) (*User, error) {
	return nil, ErrUserNotFound
}

func (s *passkeySvcUserRepoStub) Update(ctx context.Context, user *User) error {
	return errors.New("not implemented")
}

func (s *passkeySvcUserRepoStub) Delete(ctx context.Context, id int64) error {
	return errors.New("not implemented")
}

func (s *passkeySvcUserRepoStub) List(ctx context.Context, params pagination.PaginationParams) ([]User, *pagination.PaginationResult, error) {
	return nil, nil, errors.New("not implemented")
}

func (s *passkeySvcUserRepoStub) ListWithFilters(ctx context.Context, params pagination.PaginationParams, filters UserListFilters) ([]User, *pagination.PaginationResult, error) {
	return nil, nil, errors.New("not implemented")
}

func (s *passkeySvcUserRepoStub) UpdateBalance(ctx context.Context, id int64, amount float64) error {
	return errors.New("not implemented")
}

func (s *passkeySvcUserRepoStub) DeductBalance(ctx context.Context, id int64, amount float64) error {
	return errors.New("not implemented")
}

func (s *passkeySvcUserRepoStub) UpdateConcurrency(ctx context.Context, id int64, amount int) error {
	return errors.New("not implemented")
}

func (s *passkeySvcUserRepoStub) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	return false, errors.New("not implemented")
}

func (s *passkeySvcUserRepoStub) RemoveGroupFromAllowedGroups(ctx context.Context, groupID int64) (int64, error) {
	return 0, errors.New("not implemented")
}

func (s *passkeySvcUserRepoStub) AddGroupToAllowedGroups(ctx context.Context, userID int64, groupID int64) error {
	return errors.New("not implemented")
}

func (s *passkeySvcUserRepoStub) RemoveGroupFromUserAllowedGroups(ctx context.Context, userID int64, groupID int64) error {
	return errors.New("not implemented")
}

func (s *passkeySvcUserRepoStub) UpdateTotpSecret(ctx context.Context, userID int64, encryptedSecret *string) error {
	return errors.New("not implemented")
}

func (s *passkeySvcUserRepoStub) EnableTotp(ctx context.Context, userID int64) error {
	return errors.New("not implemented")
}

func (s *passkeySvcUserRepoStub) DisableTotp(ctx context.Context, userID int64) error {
	return errors.New("not implemented")
}

type passkeySvcCredentialStoreStub struct {
	byUser      map[int64][]*PasskeyCredentialRecord
	byID        map[string]*PasskeyCredentialRecord
	byRecordID  map[int64]*PasskeyCredentialRecord
	nextID      int64
	lastCreated *PasskeyCredentialRecord
	createErr   error
}

func newPasskeySvcCredentialStoreStub(records ...*PasskeyCredentialRecord) *passkeySvcCredentialStoreStub {
	stub := &passkeySvcCredentialStoreStub{
		byUser:     map[int64][]*PasskeyCredentialRecord{},
		byID:       map[string]*PasskeyCredentialRecord{},
		byRecordID: map[int64]*PasskeyCredentialRecord{},
		nextID:     1,
	}
	for _, record := range records {
		stub.seed(record)
	}
	return stub
}

func (s *passkeySvcCredentialStoreStub) seed(record *PasskeyCredentialRecord) {
	if record == nil {
		return
	}
	clone := clonePasskeyCredentialRecord(record)
	if clone.ID <= 0 {
		clone.ID = s.nextID
		s.nextID++
	}
	s.byUser[clone.UserID] = append(s.byUser[clone.UserID], clone)
	s.byID[clone.CredentialID] = clone
	s.byRecordID[clone.ID] = clone
}

func (s *passkeySvcCredentialStoreStub) ListActiveByUserID(ctx context.Context, userID int64) ([]*PasskeyCredentialRecord, error) {
	records := s.byUser[userID]
	out := make([]*PasskeyCredentialRecord, 0, len(records))
	for _, record := range records {
		if record.RevokedAt != nil {
			continue
		}
		out = append(out, clonePasskeyCredentialRecord(record))
	}
	return out, nil
}

func (s *passkeySvcCredentialStoreStub) GetByCredentialID(ctx context.Context, credentialID string) (*PasskeyCredentialRecord, error) {
	record, ok := s.byID[credentialID]
	if !ok {
		return nil, errPasskeyCredentialLookupNotFound
	}
	return clonePasskeyCredentialRecord(record), nil
}

func (s *passkeySvcCredentialStoreStub) ExistsActiveByCredentialID(ctx context.Context, credentialID string) (bool, error) {
	record, ok := s.byID[credentialID]
	if !ok {
		return false, nil
	}
	return record.RevokedAt == nil, nil
}

func (s *passkeySvcCredentialStoreStub) UpdateFriendlyName(ctx context.Context, id int64, friendlyName string) error {
	record, ok := s.byRecordID[id]
	if !ok {
		return errPasskeyCredentialLookupNotFound
	}
	record.FriendlyName = friendlyName
	return nil
}

func (s *passkeySvcCredentialStoreStub) UpdateRevokedAt(ctx context.Context, id int64, revokedAt time.Time) error {
	record, ok := s.byRecordID[id]
	if !ok {
		return errPasskeyCredentialLookupNotFound
	}
	t := revokedAt.UTC()
	record.RevokedAt = &t
	return nil
}

func (s *passkeySvcCredentialStoreStub) UpdateSignCount(ctx context.Context, id int64, signCount int64) error {
	record, ok := s.byRecordID[id]
	if !ok {
		return errPasskeyCredentialLookupNotFound
	}
	record.SignCount = signCount
	return nil
}

func (s *passkeySvcCredentialStoreStub) UpdateLastUsedAt(ctx context.Context, id int64, lastUsedAt time.Time) error {
	record, ok := s.byRecordID[id]
	if !ok {
		return errPasskeyCredentialLookupNotFound
	}
	t := lastUsedAt.UTC()
	record.LastUsedAt = &t
	return nil
}

func (s *passkeySvcCredentialStoreStub) Create(ctx context.Context, record *PasskeyCredentialRecord) error {
	if s.createErr != nil {
		return s.createErr
	}
	if _, ok := s.byID[record.CredentialID]; ok {
		return ErrPasskeyCredentialExists
	}
	clone := clonePasskeyCredentialRecord(record)
	if clone.ID <= 0 {
		clone.ID = s.nextID
		s.nextID++
	}
	s.lastCreated = clone
	s.byUser[clone.UserID] = append(s.byUser[clone.UserID], clone)
	s.byID[clone.CredentialID] = clone
	s.byRecordID[clone.ID] = clone
	return nil
}

type passkeyWebAuthnClientStub struct {
	beginCreation *protocol.CredentialCreation
	beginSession  *webauthn.SessionData
	beginErr      error

	beginAssertion *protocol.CredentialAssertion
	beginAuthErr   error

	finishCredential *webauthn.Credential
	finishErr        error
	finishPasskeyErr error

	finishPasskeyRawID      []byte
	finishPasskeyUserHandle []byte
	finishPasskeyCredential *webauthn.Credential

	lastBeginUser    webauthn.User
	lastBeginOptions protocol.PublicKeyCredentialCreationOptions
	lastBeginRequest protocol.PublicKeyCredentialRequestOptions
	lastFinishUser   webauthn.User
	lastFinishBody   string
	lastFinishFlow   webauthn.SessionData
}

func (s *passkeyWebAuthnClientStub) BeginRegistration(user webauthn.User, opts ...webauthn.RegistrationOption) (*protocol.CredentialCreation, *webauthn.SessionData, error) {
	if s.beginErr != nil {
		return nil, nil, s.beginErr
	}

	creation := &protocol.CredentialCreation{}
	if s.beginCreation != nil {
		copyCreation := *s.beginCreation
		copyCreation.Response.CredentialExcludeList = append([]protocol.CredentialDescriptor(nil), s.beginCreation.Response.CredentialExcludeList...)
		creation = &copyCreation
	}
	for _, opt := range opts {
		opt(&creation.Response)
	}

	session := &webauthn.SessionData{}
	if s.beginSession != nil {
		copySession := *s.beginSession
		session = &copySession
	}

	s.lastBeginUser = user
	s.lastBeginOptions = creation.Response
	return creation, session, nil
}

func (s *passkeyWebAuthnClientStub) FinishRegistration(user webauthn.User, session webauthn.SessionData, request *http.Request) (*webauthn.Credential, error) {
	if request != nil && request.Body != nil {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
		s.lastFinishBody = string(body)
		request.Body = io.NopCloser(bytes.NewReader(body))
	}

	s.lastFinishUser = user
	s.lastFinishFlow = session

	if s.finishErr != nil {
		return nil, s.finishErr
	}
	if s.finishCredential == nil {
		return nil, nil
	}

	copyCredential := *s.finishCredential
	copyCredential.ID = append([]byte(nil), s.finishCredential.ID...)
	copyCredential.PublicKey = append([]byte(nil), s.finishCredential.PublicKey...)
	copyCredential.Transport = append([]protocol.AuthenticatorTransport(nil), s.finishCredential.Transport...)
	copyCredential.Authenticator.AAGUID = append([]byte(nil), s.finishCredential.Authenticator.AAGUID...)
	return &copyCredential, nil
}

func (s *passkeyWebAuthnClientStub) BeginDiscoverableLogin(opts ...webauthn.LoginOption) (*protocol.CredentialAssertion, *webauthn.SessionData, error) {
	if s.beginAuthErr != nil {
		return nil, nil, s.beginAuthErr
	}

	assertion := &protocol.CredentialAssertion{}
	if s.beginAssertion != nil {
		copyAssertion := *s.beginAssertion
		copyAssertion.Response.AllowedCredentials = append([]protocol.CredentialDescriptor(nil), s.beginAssertion.Response.AllowedCredentials...)
		assertion = &copyAssertion
	}
	for _, opt := range opts {
		opt(&assertion.Response)
	}

	session := &webauthn.SessionData{}
	if s.beginSession != nil {
		copySession := *s.beginSession
		session = &copySession
	}

	s.lastBeginRequest = assertion.Response
	return assertion, session, nil
}

func (s *passkeyWebAuthnClientStub) FinishPasskeyLogin(handler webauthn.DiscoverableUserHandler, session webauthn.SessionData, request *http.Request) (webauthn.User, *webauthn.Credential, error) {
	if request != nil && request.Body != nil {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, nil, err
		}
		s.lastFinishBody = string(body)
		request.Body = io.NopCloser(bytes.NewReader(body))
	}

	s.lastFinishFlow = session

	if s.finishPasskeyErr != nil {
		return nil, nil, s.finishPasskeyErr
	}

	rawID := s.finishPasskeyRawID
	if len(rawID) == 0 {
		rawID = []byte("auth-credential")
	}
	userHandle := s.finishPasskeyUserHandle
	if len(userHandle) == 0 {
		userHandle = []byte("7")
	}

	user, err := handler(rawID, userHandle)
	if err != nil {
		return nil, nil, err
	}

	credential := s.finishPasskeyCredential
	if credential == nil {
		credential = &webauthn.Credential{Authenticator: webauthn.Authenticator{SignCount: 0}}
	}

	copyCredential := *credential
	copyCredential.ID = append([]byte(nil), credential.ID...)
	copyCredential.PublicKey = append([]byte(nil), credential.PublicKey...)
	copyCredential.Transport = append([]protocol.AuthenticatorTransport(nil), credential.Transport...)
	copyCredential.Authenticator.AAGUID = append([]byte(nil), credential.Authenticator.AAGUID...)

	return user, &copyCredential, nil
}

func clonePasskeyCredentialRecord(record *PasskeyCredentialRecord) *PasskeyCredentialRecord {
	if record == nil {
		return nil
	}
	clone := *record
	clone.Transports = append([]string(nil), record.Transports...)
	if record.LastUsedAt != nil {
		t := *record.LastUsedAt
		clone.LastUsedAt = &t
	}
	if record.RevokedAt != nil {
		t := *record.RevokedAt
		clone.RevokedAt = &t
	}
	return &clone
}

func newPasskeyEnrollmentServiceForTest(t *testing.T, fixedNow time.Time, existingCredentials ...*PasskeyCredentialRecord) (*PasskeyService, *passkeySvcAuthStateCacheStub, *passkeySvcCredentialStoreStub, *passkeyWebAuthnClientStub, *User) {
	t.Helper()

	user := &User{ID: 7, Email: "user@example.com", Username: "User Example", Role: RoleUser, Status: StatusActive}
	cache := newPasskeySvcAuthStateCacheStub()
	cache.now = fixedNow
	settingSvc := NewSettingService(&passkeySvcSettingRepoStub{all: map[string]string{SettingKeyPasskeyEnabled: "true"}}, &config.Config{})
	store := newPasskeySvcCredentialStoreStub(existingCredentials...)
	webauthnClient := &passkeyWebAuthnClientStub{
		beginSession: &webauthn.SessionData{
			Challenge:      "registration-challenge",
			RelyingPartyID: "app.example.com",
			Expires:        fixedNow.Add(time.Minute),
		},
	}

	svc := NewPasskeyService(settingSvc, cache)
	svc.userRepo = newPasskeySvcUserRepoStub(user)
	svc.recentAuthService = NewRecentAuthService(cache)
	svc.credentialStore = store
	svc.webauthnFactory = func(ctx context.Context) (passkeyWebAuthnClient, error) {
		return webauthnClient, nil
	}
	svc.now = func() time.Time {
		return fixedNow
	}

	return svc, cache, store, webauthnClient, user
}

func TestPasskeyService_GetWebAuthn_DerivesSettingsConfig(t *testing.T) {
	settingSvc := NewSettingService(
		&passkeySvcSettingRepoStub{all: map[string]string{SettingKeyFrontendURL: "https://App.Example.com:7443/login"}},
		&config.Config{},
	)
	cache := newPasskeySvcAuthStateCacheStub()
	svc := NewPasskeyService(settingSvc, cache)

	wa, err := svc.GetWebAuthn(context.Background())
	require.NoError(t, err)
	require.NotNil(t, wa)
	require.Equal(t, "app.example.com", wa.Config.GetRPID())
	require.Equal(t, "Sub2API", wa.Config.RPDisplayName)
	require.Equal(t, []string{"https://app.example.com:7443"}, wa.Config.GetOrigins())
}

func TestPasskeyService_GetWebAuthn_RejectsHTTPNonLocalhostOrigin(t *testing.T) {
	settingSvc := NewSettingService(
		&passkeySvcSettingRepoStub{all: map[string]string{
			SettingKeyFrontendURL: "http://accounts.example.com",
		}},
		&config.Config{},
	)

	svc := NewPasskeyService(settingSvc, newPasskeySvcAuthStateCacheStub())

	wa, err := svc.GetWebAuthn(context.Background())
	require.Nil(t, wa)
	require.ErrorIs(t, err, ErrPasskeyRPConfigInvalid)
}

func TestPasskeyService_GetWebAuthn_AllowsHTTPLocalhostOrigins(t *testing.T) {
	settingSvc := NewSettingService(
		&passkeySvcSettingRepoStub{all: map[string]string{
			SettingKeyFrontendURL: "http://localhost:5173",
		}},
		&config.Config{},
	)

	svc := NewPasskeyService(settingSvc, newPasskeySvcAuthStateCacheStub())

	wa, err := svc.GetWebAuthn(context.Background())
	require.NoError(t, err)
	require.NotNil(t, wa)
	require.Equal(t, []string{"http://localhost:5173"}, wa.Config.GetOrigins())
}

func TestPasskeyService_IssueChallenge_GeneratesUniqueFlowIDs(t *testing.T) {
	settingSvc := NewSettingService(
		&passkeySvcSettingRepoStub{all: map[string]string{SettingKeyFrontendURL: "https://app.example.com"}},
		&config.Config{},
	)
	cache := newPasskeySvcAuthStateCacheStub()
	svc := NewPasskeyService(settingSvc, cache)

	session := &webauthn.SessionData{
		Challenge:      "challenge",
		RelyingPartyID: "app.example.com",
		Expires:        time.Now().Add(time.Minute),
	}

	flowID1, err := svc.IssueChallenge(context.Background(), PasskeyFlowTypeRegistration, 7, session)
	require.NoError(t, err)
	flowID2, err := svc.IssueChallenge(context.Background(), PasskeyFlowTypeRegistration, 7, session)
	require.NoError(t, err)

	require.NotEmpty(t, flowID1)
	require.NotEmpty(t, flowID2)
	require.NotEqual(t, flowID1, flowID2)
	require.Len(t, cache.challenges, 2)
}

func TestPasskeyService_ConsumeChallenge_Expired(t *testing.T) {
	svc := NewPasskeyService(nil, newPasskeySvcAuthStateCacheStub())

	_, err := svc.ConsumeChallenge(context.Background(), "missing-flow", PasskeyFlowTypeAuthentication)
	require.ErrorIs(t, err, ErrPasskeyFlowExpired)
}

func TestPasskeyService_ConsumeChallenge_ReplayRejected(t *testing.T) {
	settingSvc := NewSettingService(
		&passkeySvcSettingRepoStub{all: map[string]string{SettingKeyFrontendURL: "https://app.example.com"}},
		&config.Config{},
	)
	cache := newPasskeySvcAuthStateCacheStub()
	svc := NewPasskeyService(settingSvc, cache)

	session := &webauthn.SessionData{
		Challenge:      "challenge",
		RelyingPartyID: "app.example.com",
		Expires:        time.Now().Add(time.Minute),
	}

	flowID, err := svc.IssueChallenge(context.Background(), PasskeyFlowTypeAuthentication, 9, session)
	require.NoError(t, err)

	_, err = svc.ConsumeChallenge(context.Background(), flowID, PasskeyFlowTypeAuthentication)
	require.NoError(t, err)

	_, err = svc.ConsumeChallenge(context.Background(), flowID, PasskeyFlowTypeAuthentication)
	require.ErrorIs(t, err, ErrPasskeyFlowReplayed)
}

func TestPasskeyService_UpdateAuthenticatorCounter_AllowsZeroCounter(t *testing.T) {
	svc := NewPasskeyService(nil, nil)
	authenticator := &webauthn.Authenticator{SignCount: 0}

	updated := svc.UpdateAuthenticatorCounter(authenticator, 0)
	require.Equal(t, uint32(0), updated)
	require.False(t, authenticator.CloneWarning)
}

func TestPasskeyService_BeginAndFinishRegistration_HappyPath(t *testing.T) {
	fixedNow := time.Date(2026, 3, 29, 10, 45, 0, 0, time.UTC)
	existingCredential := &PasskeyCredentialRecord{
		UserID:       7,
		CredentialID: encodePasskeyBinary([]byte("existing-credential")),
		PublicKey:    encodePasskeyBinary([]byte("existing-public-key")),
		SignCount:    12,
		Transports:   []string{string(protocol.Internal)},
	}

	svc, _, store, webauthnClient, user := newPasskeyEnrollmentServiceForTest(t, fixedNow, existingCredential)
	require.NoError(t, svc.recentAuthService.IssueRecentAuth(context.Background(), user.ID, RecentAuthMethodPassword))

	aaguid := uuid.MustParse("00112233-4455-6677-8899-aabbccddeeff")
	webauthnClient.finishCredential = &webauthn.Credential{
		ID:        []byte("new-credential-id"),
		PublicKey: []byte("new-public-key"),
		Transport: []protocol.AuthenticatorTransport{protocol.Internal, protocol.Hybrid},
		Flags: webauthn.CredentialFlags{
			BackupEligible: true,
			BackupState:    true,
		},
		Authenticator: webauthn.Authenticator{
			AAGUID:    aaguid[:],
			SignCount: 23,
		},
	}

	beginResult, err := svc.BeginRegistration(context.Background(), user.ID)
	require.NoError(t, err)
	require.NotEmpty(t, beginResult.FlowID)
	require.Equal(t, int(passkeyChallengeTTL.Seconds()), beginResult.Countdown)
	require.Equal(t, protocol.ResidentKeyRequirementRequired, webauthnClient.lastBeginOptions.AuthenticatorSelection.ResidentKey)
	require.NotNil(t, webauthnClient.lastBeginOptions.AuthenticatorSelection.RequireResidentKey)
	require.True(t, *webauthnClient.lastBeginOptions.AuthenticatorSelection.RequireResidentKey)
	require.Equal(t, protocol.VerificationRequired, webauthnClient.lastBeginOptions.AuthenticatorSelection.UserVerification)
	require.Empty(t, webauthnClient.lastBeginOptions.AuthenticatorSelection.AuthenticatorAttachment)
	require.Equal(t, protocol.PreferNoAttestation, webauthnClient.lastBeginOptions.Attestation)
	require.Len(t, webauthnClient.lastBeginOptions.CredentialExcludeList, 1)
	require.Equal(t, []byte("existing-credential"), []byte(webauthnClient.lastBeginOptions.CredentialExcludeList[0].CredentialID))

	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/passkeys/register/finish", bytes.NewBufferString(`{"id":"credential"}`))
	request.Header.Set("Content-Type", "application/json")

	finishResult, err := svc.FinishRegistration(context.Background(), user.ID, beginResult.FlowID, "", request)
	require.NoError(t, err)
	require.Equal(t, encodePasskeyBinary([]byte("new-credential-id")), finishResult.CredentialID)
	require.Equal(t, "Passkey 2026-03-29 10:45", finishResult.FriendlyName)
	require.Equal(t, `{"id":"credential"}`, webauthnClient.lastFinishBody)

	require.NotNil(t, store.lastCreated)
	require.Equal(t, user.ID, store.lastCreated.UserID)
	require.Equal(t, encodePasskeyBinary([]byte("new-credential-id")), store.lastCreated.CredentialID)
	require.Equal(t, encodePasskeyBinary([]byte("new-public-key")), store.lastCreated.PublicKey)
	require.Equal(t, int64(23), store.lastCreated.SignCount)
	require.Equal(t, []string{"internal", "hybrid"}, store.lastCreated.Transports)
	require.Equal(t, "00112233-4455-6677-8899-aabbccddeeff", store.lastCreated.AAGUID)
	require.True(t, store.lastCreated.BackupEligible)
	require.True(t, store.lastCreated.BackupState)
	require.Equal(t, "Passkey 2026-03-29 10:45", store.lastCreated.FriendlyName)
	require.Nil(t, store.lastCreated.LastUsedAt)
}

func TestPasskeyService_FinishRegistration_UsesCuratedAAGUIDFriendlyName(t *testing.T) {
	fixedNow := time.Date(2026, 3, 29, 10, 55, 0, 0, time.UTC)
	svc, _, store, webauthnClient, user := newPasskeyEnrollmentServiceForTest(t, fixedNow)
	require.NoError(t, svc.recentAuthService.IssueRecentAuth(context.Background(), user.ID, RecentAuthMethodPassword))

	knownAAGUID := uuid.MustParse("de1e552d-db1d-4423-a619-566b625cdc84")
	webauthnClient.finishCredential = &webauthn.Credential{
		ID:        []byte("known-credential-id"),
		PublicKey: []byte("known-public-key"),
		Authenticator: webauthn.Authenticator{
			AAGUID: knownAAGUID[:],
		},
	}

	beginResult, err := svc.BeginRegistration(context.Background(), user.ID)
	require.NoError(t, err)

	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/passkeys/register/finish", bytes.NewBufferString(`{"id":"credential"}`))
	request.Header.Set("Content-Type", "application/json")

	finishResult, err := svc.FinishRegistration(context.Background(), user.ID, beginResult.FlowID, "", request)
	require.NoError(t, err)
	require.Equal(t, "Microsoft Authenticator (iOS)", finishResult.FriendlyName)
	require.NotNil(t, store.lastCreated)
	require.Equal(t, "Microsoft Authenticator (iOS)", store.lastCreated.FriendlyName)
}

func TestPasskeyService_FinishRegistration_UsesMetadataCacheFriendlyName(t *testing.T) {
	fixedNow := time.Date(2026, 3, 29, 10, 56, 0, 0, time.UTC)
	svc, _, store, webauthnClient, user := newPasskeyEnrollmentServiceForTest(t, fixedNow)
	require.NoError(t, svc.recentAuthService.IssueRecentAuth(context.Background(), user.ID, RecentAuthMethodPassword))

	metadataAAGUID := uuid.MustParse("11111111-2222-4333-8444-555555555555")
	svc.SetPasskeyAAGUIDMetadataCache(NewStaticPasskeyAAGUIDMetadataCache(map[string]string{
		metadataAAGUID.String(): "FIDO Metadata Security Key",
	}))

	webauthnClient.finishCredential = &webauthn.Credential{
		ID:        []byte("metadata-credential-id"),
		PublicKey: []byte("metadata-public-key"),
		Authenticator: webauthn.Authenticator{
			AAGUID: metadataAAGUID[:],
		},
	}

	beginResult, err := svc.BeginRegistration(context.Background(), user.ID)
	require.NoError(t, err)

	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/passkeys/register/finish", bytes.NewBufferString(`{"id":"credential"}`))
	request.Header.Set("Content-Type", "application/json")

	finishResult, err := svc.FinishRegistration(context.Background(), user.ID, beginResult.FlowID, "", request)
	require.NoError(t, err)
	require.Equal(t, "FIDO Metadata Security Key", finishResult.FriendlyName)
	require.NotNil(t, store.lastCreated)
	require.Equal(t, "FIDO Metadata Security Key", store.lastCreated.FriendlyName)
}

func TestPasskeyService_FinishRegistration_RejectsDuplicateCredential(t *testing.T) {
	fixedNow := time.Date(2026, 3, 29, 11, 0, 0, 0, time.UTC)
	existingCredential := &PasskeyCredentialRecord{
		UserID:       7,
		CredentialID: encodePasskeyBinary([]byte("duplicate-credential")),
		PublicKey:    encodePasskeyBinary([]byte("existing-public-key")),
	}

	svc, _, store, webauthnClient, user := newPasskeyEnrollmentServiceForTest(t, fixedNow, existingCredential)
	require.NoError(t, svc.recentAuthService.IssueRecentAuth(context.Background(), user.ID, RecentAuthMethodPassword))
	webauthnClient.finishCredential = &webauthn.Credential{
		ID:        []byte("duplicate-credential"),
		PublicKey: []byte("new-public-key"),
	}

	beginResult, err := svc.BeginRegistration(context.Background(), user.ID)
	require.NoError(t, err)

	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/passkeys/register/finish", bytes.NewBufferString(`{"id":"duplicate"}`))
	request.Header.Set("Content-Type", "application/json")

	finishResult, err := svc.FinishRegistration(context.Background(), user.ID, beginResult.FlowID, "Laptop", request)
	require.Nil(t, finishResult)
	require.ErrorIs(t, err, ErrPasskeyCredentialExists)
	require.Nil(t, store.lastCreated)
}

func TestPasskeyService_FinishRegistration_ExpiredChallenge(t *testing.T) {
	fixedNow := time.Date(2026, 3, 29, 11, 15, 0, 0, time.UTC)
	svc, cache, _, webauthnClient, user := newPasskeyEnrollmentServiceForTest(t, fixedNow)
	require.NoError(t, svc.recentAuthService.IssueRecentAuth(context.Background(), user.ID, RecentAuthMethodPassword))

	beginResult, err := svc.BeginRegistration(context.Background(), user.ID)
	require.NoError(t, err)

	cache.now = fixedNow.Add(passkeyChallengeTTL + time.Second)
	require.NoError(t, svc.recentAuthService.IssueRecentAuth(context.Background(), user.ID, RecentAuthMethodPassword))
	webauthnClient.finishCredential = &webauthn.Credential{ID: []byte("new-credential")}

	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/passkeys/register/finish", bytes.NewBufferString(`{"id":"expired"}`))
	request.Header.Set("Content-Type", "application/json")

	finishResult, err := svc.FinishRegistration(context.Background(), user.ID, beginResult.FlowID, "", request)
	require.Nil(t, finishResult)
	require.ErrorIs(t, err, ErrPasskeyFlowExpired)
}

func TestPasskeyService_BeginRegistration_RequiresRecentAuth(t *testing.T) {
	fixedNow := time.Date(2026, 3, 29, 11, 30, 0, 0, time.UTC)
	svc, _, _, _, user := newPasskeyEnrollmentServiceForTest(t, fixedNow)

	beginResult, err := svc.BeginRegistration(context.Background(), user.ID)
	require.Nil(t, beginResult)
	require.ErrorIs(t, err, ErrRecentAuthRequired)
}

func TestPasskeyService_BeginAuthentication_UsesDiscoverableUVRequired(t *testing.T) {
	fixedNow := time.Date(2026, 3, 29, 12, 0, 0, 0, time.UTC)
	svc, cache, _, webauthnClient, _ := newPasskeyEnrollmentServiceForTest(t, fixedNow)

	webauthnClient.beginAssertion = &protocol.CredentialAssertion{
		Response: protocol.PublicKeyCredentialRequestOptions{
			Challenge:      []byte("discoverable-challenge"),
			RelyingPartyID: "app.example.com",
		},
	}

	result, err := svc.BeginAuthentication(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotEmpty(t, result.FlowID)
	require.Equal(t, int(passkeyChallengeTTL.Seconds()), result.Countdown)
	require.Equal(t, protocol.VerificationRequired, webauthnClient.lastBeginRequest.UserVerification)
	require.Empty(t, webauthnClient.lastBeginRequest.AllowedCredentials)

	storedFlow, ok := cache.challenges[result.FlowID]
	require.True(t, ok)
	require.Equal(t, PasskeyFlowTypeAuthentication, storedFlow.record.FlowType)
	require.Equal(t, int64(0), storedFlow.record.UserID)
}

func TestPasskeyService_FinishAuthentication_SuccessUpdatesCounterAndLastUsedAt(t *testing.T) {
	fixedNow := time.Date(2026, 3, 29, 12, 15, 0, 0, time.UTC)
	credentialIDRaw := []byte("auth-credential")
	storedCredential := &PasskeyCredentialRecord{
		ID:           41,
		UserID:       7,
		CredentialID: encodePasskeyBinary(credentialIDRaw),
		PublicKey:    encodePasskeyBinary([]byte("credential-public-key")),
		SignCount:    4,
		Transports:   []string{"internal"},
	}

	svc, _, store, webauthnClient, user := newPasskeyEnrollmentServiceForTest(t, fixedNow, storedCredential)
	webauthnClient.finishPasskeyRawID = credentialIDRaw
	webauthnClient.finishPasskeyUserHandle = []byte("7")
	webauthnClient.finishPasskeyCredential = &webauthn.Credential{
		ID: credentialIDRaw,
		Authenticator: webauthn.Authenticator{
			SignCount: 12,
		},
	}

	beginResult, err := svc.BeginAuthentication(context.Background())
	require.NoError(t, err)

	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/passkeys/login/finish", bytes.NewBufferString(`{"id":"auth-credential"}`))
	request.Header.Set("Content-Type", "application/json")

	authedUser, err := svc.FinishAuthentication(context.Background(), beginResult.FlowID, request)
	require.NoError(t, err)
	require.Equal(t, user.ID, authedUser.ID)

	updatedRecord := store.byID[storedCredential.CredentialID]
	require.Equal(t, int64(12), updatedRecord.SignCount)
	require.NotNil(t, updatedRecord.LastUsedAt)
	require.Equal(t, fixedNow, updatedRecord.LastUsedAt.UTC())
}

func TestPasskeyService_FinishAuthentication_RevokedCredentialRejected(t *testing.T) {
	fixedNow := time.Date(2026, 3, 29, 12, 20, 0, 0, time.UTC)
	revokedAt := fixedNow.Add(-time.Hour)
	credentialIDRaw := []byte("revoked-credential")
	storedCredential := &PasskeyCredentialRecord{
		ID:           52,
		UserID:       7,
		CredentialID: encodePasskeyBinary(credentialIDRaw),
		PublicKey:    encodePasskeyBinary([]byte("credential-public-key")),
		SignCount:    8,
		RevokedAt:    &revokedAt,
	}

	svc, _, store, webauthnClient, _ := newPasskeyEnrollmentServiceForTest(t, fixedNow, storedCredential)
	webauthnClient.finishPasskeyRawID = credentialIDRaw
	webauthnClient.finishPasskeyUserHandle = []byte("7")

	beginResult, err := svc.BeginAuthentication(context.Background())
	require.NoError(t, err)

	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/passkeys/login/finish", bytes.NewBufferString(`{"id":"revoked"}`))
	request.Header.Set("Content-Type", "application/json")

	authedUser, err := svc.FinishAuthentication(context.Background(), beginResult.FlowID, request)
	require.Nil(t, authedUser)
	require.ErrorIs(t, err, ErrInvalidCredentials)

	updatedRecord := store.byID[storedCredential.CredentialID]
	require.Equal(t, int64(8), updatedRecord.SignCount)
	require.Nil(t, updatedRecord.LastUsedAt)
}

func TestPasskeyService_FinishAuthentication_ExpiredFlowReturnsGenericAuthError(t *testing.T) {
	fixedNow := time.Date(2026, 3, 29, 12, 25, 0, 0, time.UTC)
	storedCredential := &PasskeyCredentialRecord{
		ID:           63,
		UserID:       7,
		CredentialID: encodePasskeyBinary([]byte("exp-credential")),
		PublicKey:    encodePasskeyBinary([]byte("credential-public-key")),
	}

	svc, cache, _, webauthnClient, _ := newPasskeyEnrollmentServiceForTest(t, fixedNow, storedCredential)
	webauthnClient.finishPasskeyRawID = []byte("exp-credential")
	webauthnClient.finishPasskeyUserHandle = []byte("7")

	beginResult, err := svc.BeginAuthentication(context.Background())
	require.NoError(t, err)

	cache.now = fixedNow.Add(passkeyChallengeTTL + time.Second)

	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/passkeys/login/finish", bytes.NewBufferString(`{"id":"expired"}`))
	request.Header.Set("Content-Type", "application/json")

	authedUser, err := svc.FinishAuthentication(context.Background(), beginResult.FlowID, request)
	require.Nil(t, authedUser)
	require.ErrorIs(t, err, ErrInvalidCredentials)
}

func TestPasskeyService_FinishAuthentication_ReplayedFlowReturnsGenericAuthError(t *testing.T) {
	fixedNow := time.Date(2026, 3, 29, 12, 30, 0, 0, time.UTC)
	storedCredential := &PasskeyCredentialRecord{
		ID:           74,
		UserID:       7,
		CredentialID: encodePasskeyBinary([]byte("replay-credential")),
		PublicKey:    encodePasskeyBinary([]byte("credential-public-key")),
	}

	svc, _, _, webauthnClient, _ := newPasskeyEnrollmentServiceForTest(t, fixedNow, storedCredential)
	webauthnClient.finishPasskeyRawID = []byte("replay-credential")
	webauthnClient.finishPasskeyUserHandle = []byte("7")

	beginResult, err := svc.BeginAuthentication(context.Background())
	require.NoError(t, err)

	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/passkeys/login/finish", bytes.NewBufferString(`{"id":"replay"}`))
	request.Header.Set("Content-Type", "application/json")

	firstUser, err := svc.FinishAuthentication(context.Background(), beginResult.FlowID, request)
	require.NoError(t, err)
	require.NotNil(t, firstUser)

	replayRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/passkeys/login/finish", bytes.NewBufferString(`{"id":"replay"}`))
	replayRequest.Header.Set("Content-Type", "application/json")
	secondUser, err := svc.FinishAuthentication(context.Background(), beginResult.FlowID, replayRequest)
	require.Nil(t, secondUser)
	require.ErrorIs(t, err, ErrInvalidCredentials)
}

func TestPasskeyService_FinishAuthentication_OriginMismatchMappedToGenericAuthError(t *testing.T) {
	fixedNow := time.Date(2026, 3, 29, 12, 40, 0, 0, time.UTC)
	storedCredential := &PasskeyCredentialRecord{
		ID:           85,
		UserID:       7,
		CredentialID: encodePasskeyBinary([]byte("origin-credential")),
		PublicKey:    encodePasskeyBinary([]byte("credential-public-key")),
	}

	svc, _, _, webauthnClient, _ := newPasskeyEnrollmentServiceForTest(t, fixedNow, storedCredential)
	webauthnClient.finishPasskeyErr = protocol.ErrVerification.WithDetails("origin mismatch")

	beginResult, err := svc.BeginAuthentication(context.Background())
	require.NoError(t, err)

	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/passkeys/login/finish", bytes.NewBufferString(`{"id":"origin"}`))
	request.Header.Set("Content-Type", "application/json")

	authedUser, err := svc.FinishAuthentication(context.Background(), beginResult.FlowID, request)
	require.Nil(t, authedUser)
	require.ErrorIs(t, err, ErrInvalidCredentials)
}

func TestPasskeyService_FinishAuthentication_CounterEdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		storedCount   int64
		returnedCount uint32
		expectedCount int64
	}{
		{name: "zero counter preserved", storedCount: 0, returnedCount: 0, expectedCount: 0},
		{name: "non incrementing counter keeps stored value", storedCount: 23, returnedCount: 4, expectedCount: 23},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixedNow := time.Date(2026, 3, 29, 12, 50, 0, 0, time.UTC)
			credentialIDRaw := []byte("counter-credential")
			storedCredential := &PasskeyCredentialRecord{
				ID:           96,
				UserID:       7,
				CredentialID: encodePasskeyBinary(credentialIDRaw),
				PublicKey:    encodePasskeyBinary([]byte("credential-public-key")),
				SignCount:    tt.storedCount,
			}

			svc, _, store, webauthnClient, _ := newPasskeyEnrollmentServiceForTest(t, fixedNow, storedCredential)
			webauthnClient.finishPasskeyRawID = credentialIDRaw
			webauthnClient.finishPasskeyUserHandle = []byte("7")
			webauthnClient.finishPasskeyCredential = &webauthn.Credential{
				ID: credentialIDRaw,
				Authenticator: webauthn.Authenticator{
					SignCount: tt.returnedCount,
				},
			}

			beginResult, err := svc.BeginAuthentication(context.Background())
			require.NoError(t, err)

			request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/passkeys/login/finish", bytes.NewBufferString(`{"id":"counter"}`))
			request.Header.Set("Content-Type", "application/json")

			authedUser, err := svc.FinishAuthentication(context.Background(), beginResult.FlowID, request)
			require.NoError(t, err)
			require.NotNil(t, authedUser)

			updatedRecord := store.byID[storedCredential.CredentialID]
			require.Equal(t, tt.expectedCount, updatedRecord.SignCount)
			require.NotNil(t, updatedRecord.LastUsedAt)
		})
	}
}

func TestPasskeyService_ListManagementCredentials_ActiveOnlySafeMetadata(t *testing.T) {
	fixedNow := time.Date(2026, 3, 29, 13, 0, 0, 0, time.UTC)
	createdAt := fixedNow.Add(-48 * time.Hour)
	lastUsedAt := fixedNow.Add(-2 * time.Hour)
	revokedAt := fixedNow.Add(-time.Hour)
	active := &PasskeyCredentialRecord{
		ID:             111,
		UserID:         7,
		CredentialID:   encodePasskeyBinary([]byte("active-management")),
		PublicKey:      encodePasskeyBinary([]byte("public-key")),
		FriendlyName:   "Work Laptop",
		CreatedAt:      createdAt,
		LastUsedAt:     &lastUsedAt,
		BackupEligible: true,
		BackupState:    true,
	}
	revoked := &PasskeyCredentialRecord{
		ID:           112,
		UserID:       7,
		CredentialID: encodePasskeyBinary([]byte("revoked-management")),
		PublicKey:    encodePasskeyBinary([]byte("public-key-revoked")),
		FriendlyName: "Retired Key",
		CreatedAt:    fixedNow.Add(-72 * time.Hour),
		RevokedAt:    &revokedAt,
	}

	svc, _, _, _, user := newPasskeyEnrollmentServiceForTest(t, fixedNow, active, revoked)

	result, err := svc.ListManagementCredentials(context.Background(), user.ID)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Items, 1)
	require.Equal(t, active.CredentialID, result.Items[0].CredentialID)
	require.Equal(t, "Work Laptop", result.Items[0].FriendlyName)
	require.Equal(t, createdAt, result.Items[0].CreatedAt)
	require.NotNil(t, result.Items[0].LastUsedAt)
	require.Equal(t, lastUsedAt, result.Items[0].LastUsedAt.UTC())
	require.True(t, result.Items[0].BackupEligible)
	require.True(t, result.Items[0].Synced)
}

func TestPasskeyService_GetManagementStatus_ReflectsRecentAuthAndActiveCount(t *testing.T) {
	fixedNow := time.Date(2026, 3, 29, 13, 5, 0, 0, time.UTC)
	credential := &PasskeyCredentialRecord{
		ID:           121,
		UserID:       7,
		CredentialID: encodePasskeyBinary([]byte("status-management")),
		PublicKey:    encodePasskeyBinary([]byte("status-public-key")),
		FriendlyName: "Phone",
		CreatedAt:    fixedNow.Add(-24 * time.Hour),
	}

	svc, _, _, _, user := newPasskeyEnrollmentServiceForTest(t, fixedNow, credential)

	status, err := svc.GetManagementStatus(context.Background(), user.ID)
	require.NoError(t, err)
	require.True(t, status.FeatureEnabled)
	require.False(t, status.CanManage)
	require.True(t, status.HasPasskeys)
	require.Equal(t, 1, status.ActiveCount)
	require.True(t, status.PasswordFallbackAvailable)

	require.NoError(t, svc.recentAuthService.IssueRecentAuth(context.Background(), user.ID, RecentAuthMethodPasskey))

	status, err = svc.GetManagementStatus(context.Background(), user.ID)
	require.NoError(t, err)
	require.True(t, status.CanManage)
}

func TestPasskeyService_GetManagementStatus_FeatureDisabledFailsClosed(t *testing.T) {
	settingSvc := NewSettingService(&passkeySvcSettingRepoStub{all: map[string]string{SettingKeyPasskeyEnabled: "false"}}, &config.Config{})
	svc := NewPasskeyService(settingSvc, newPasskeySvcAuthStateCacheStub())

	status, err := svc.GetManagementStatus(context.Background(), 7)
	require.NoError(t, err)
	require.False(t, status.FeatureEnabled)
	require.False(t, status.CanManage)
	require.False(t, status.HasPasskeys)
	require.Zero(t, status.ActiveCount)
	require.True(t, status.PasswordFallbackAvailable)

	listResult, err := svc.ListManagementCredentials(context.Background(), 7)
	require.Nil(t, listResult)
	require.ErrorIs(t, err, ErrPasskeyNotEnabled)
}

func TestPasskeyService_RenameCredential_RequiresRecentAuth(t *testing.T) {
	fixedNow := time.Date(2026, 3, 29, 13, 10, 0, 0, time.UTC)
	credential := &PasskeyCredentialRecord{
		ID:           131,
		UserID:       7,
		CredentialID: encodePasskeyBinary([]byte("rename-management")),
		PublicKey:    encodePasskeyBinary([]byte("rename-public-key")),
		FriendlyName: "Old Name",
		CreatedAt:    fixedNow.Add(-time.Hour),
	}

	svc, _, store, _, user := newPasskeyEnrollmentServiceForTest(t, fixedNow, credential)

	renamed, err := svc.RenameCredential(context.Background(), user.ID, credential.CredentialID, "New Name")
	require.Nil(t, renamed)
	require.ErrorIs(t, err, ErrRecentAuthRequired)
	require.Equal(t, "Old Name", store.byID[credential.CredentialID].FriendlyName)
}

func TestPasskeyService_RenameCredential_OwnershipCheck(t *testing.T) {
	fixedNow := time.Date(2026, 3, 29, 13, 15, 0, 0, time.UTC)
	foreignCredential := &PasskeyCredentialRecord{
		ID:           141,
		UserID:       88,
		CredentialID: encodePasskeyBinary([]byte("foreign-rename")),
		PublicKey:    encodePasskeyBinary([]byte("foreign-public-key")),
		FriendlyName: "Foreign Name",
		CreatedAt:    fixedNow.Add(-2 * time.Hour),
	}

	svc, _, store, _, user := newPasskeyEnrollmentServiceForTest(t, fixedNow, foreignCredential)
	require.NoError(t, svc.recentAuthService.IssueRecentAuth(context.Background(), user.ID, RecentAuthMethodPassword))

	renamed, err := svc.RenameCredential(context.Background(), user.ID, foreignCredential.CredentialID, "Hijacked")
	require.Nil(t, renamed)
	require.ErrorIs(t, err, ErrPasskeyCredentialNotFound)
	require.Equal(t, "Foreign Name", store.byID[foreignCredential.CredentialID].FriendlyName)
}

func TestPasskeyService_RenameCredential_UpdatesFriendlyName(t *testing.T) {
	fixedNow := time.Date(2026, 3, 29, 13, 20, 0, 0, time.UTC)
	createdAt := fixedNow.Add(-3 * time.Hour)
	credential := &PasskeyCredentialRecord{
		ID:           151,
		UserID:       7,
		CredentialID: encodePasskeyBinary([]byte("rename-success")),
		PublicKey:    encodePasskeyBinary([]byte("rename-success-key")),
		FriendlyName: "Desktop",
		CreatedAt:    createdAt,
	}

	svc, _, store, _, user := newPasskeyEnrollmentServiceForTest(t, fixedNow, credential)
	require.NoError(t, svc.recentAuthService.IssueRecentAuth(context.Background(), user.ID, RecentAuthMethodPasswordTOTP))

	renamed, err := svc.RenameCredential(context.Background(), user.ID, credential.CredentialID, "  Office Desktop  ")
	require.NoError(t, err)
	require.NotNil(t, renamed)
	require.Equal(t, "Office Desktop", renamed.FriendlyName)
	require.Equal(t, createdAt, renamed.CreatedAt)
	require.Equal(t, "Office Desktop", store.byID[credential.CredentialID].FriendlyName)
}

func TestPasskeyService_RevokeCredential_RequiresRecentAuthAndRevokesDurably(t *testing.T) {
	fixedNow := time.Date(2026, 3, 29, 13, 25, 0, 0, time.UTC)
	credential := &PasskeyCredentialRecord{
		ID:           161,
		UserID:       7,
		CredentialID: encodePasskeyBinary([]byte("revoke-success")),
		PublicKey:    encodePasskeyBinary([]byte("revoke-public-key")),
		FriendlyName: "Tablet",
		CreatedAt:    fixedNow.Add(-4 * time.Hour),
	}

	svc, _, store, _, user := newPasskeyEnrollmentServiceForTest(t, fixedNow, credential)

	revokeResult, err := svc.RevokeCredential(context.Background(), user.ID, credential.CredentialID)
	require.Nil(t, revokeResult)
	require.ErrorIs(t, err, ErrRecentAuthRequired)

	require.NoError(t, svc.recentAuthService.IssueRecentAuth(context.Background(), user.ID, RecentAuthMethodPasskey))

	revokeResult, err = svc.RevokeCredential(context.Background(), user.ID, credential.CredentialID)
	require.NoError(t, err)
	require.NotNil(t, revokeResult)
	require.Equal(t, credential.CredentialID, revokeResult.CredentialID)
	require.Equal(t, fixedNow, revokeResult.RevokedAt)
	require.True(t, revokeResult.PasswordFallbackAvailable)
	require.NotNil(t, store.byID[credential.CredentialID].RevokedAt)
	require.Equal(t, fixedNow, store.byID[credential.CredentialID].RevokedAt.UTC())

	listResult, err := svc.ListManagementCredentials(context.Background(), user.ID)
	require.NoError(t, err)
	require.Empty(t, listResult.Items)
}

func TestPasskeyService_RevokeCredential_ExpiredRecentAuth(t *testing.T) {
	fixedNow := time.Date(2026, 3, 29, 13, 30, 0, 0, time.UTC)
	credential := &PasskeyCredentialRecord{
		ID:           171,
		UserID:       7,
		CredentialID: encodePasskeyBinary([]byte("revoke-expired")),
		PublicKey:    encodePasskeyBinary([]byte("revoke-expired-key")),
		FriendlyName: "Backup Key",
		CreatedAt:    fixedNow.Add(-5 * time.Hour),
	}

	svc, cache, store, _, user := newPasskeyEnrollmentServiceForTest(t, fixedNow, credential)
	require.NoError(t, svc.recentAuthService.IssueRecentAuth(context.Background(), user.ID, RecentAuthMethodPassword))

	cache.now = fixedNow.Add(recentAuthTTL + time.Second)
	revokeResult, err := svc.RevokeCredential(context.Background(), user.ID, credential.CredentialID)
	require.Nil(t, revokeResult)
	require.ErrorIs(t, err, ErrRecentAuthRequired)
	require.Nil(t, store.byID[credential.CredentialID].RevokedAt)
}

func TestPasskeyService_RevokeCredential_OwnershipCheck(t *testing.T) {
	fixedNow := time.Date(2026, 3, 29, 13, 35, 0, 0, time.UTC)
	foreignCredential := &PasskeyCredentialRecord{
		ID:           181,
		UserID:       99,
		CredentialID: encodePasskeyBinary([]byte("foreign-revoke")),
		PublicKey:    encodePasskeyBinary([]byte("foreign-revoke-key")),
		FriendlyName: "Foreign Key",
		CreatedAt:    fixedNow.Add(-6 * time.Hour),
	}

	svc, _, store, _, user := newPasskeyEnrollmentServiceForTest(t, fixedNow, foreignCredential)
	require.NoError(t, svc.recentAuthService.IssueRecentAuth(context.Background(), user.ID, RecentAuthMethodPasskey))

	revokeResult, err := svc.RevokeCredential(context.Background(), user.ID, foreignCredential.CredentialID)
	require.Nil(t, revokeResult)
	require.ErrorIs(t, err, ErrPasskeyCredentialNotFound)
	require.Nil(t, store.byID[foreignCredential.CredentialID].RevokedAt)
}
