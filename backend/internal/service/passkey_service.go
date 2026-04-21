package service

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/passkeycredential"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
)

var (
	ErrPasskeyNotEnabled           = infraerrors.BadRequest("PASSKEY_NOT_ENABLED", "passkey feature is not enabled")
	ErrPasskeyUserIDInvalid        = infraerrors.BadRequest("PASSKEY_USER_ID_INVALID", "user id is invalid")
	ErrPasskeyCredentialIDRequired = infraerrors.BadRequest("PASSKEY_CREDENTIAL_ID_REQUIRED", "passkey credential id is required")
	ErrPasskeyCredentialNotFound   = infraerrors.NotFound("PASSKEY_CREDENTIAL_NOT_FOUND", "passkey credential not found")
	ErrPasskeyFriendlyNameRequired = infraerrors.BadRequest("PASSKEY_FRIENDLY_NAME_REQUIRED", "passkey friendly name is required")
	ErrPasskeyFriendlyNameTooLong  = infraerrors.BadRequest("PASSKEY_FRIENDLY_NAME_TOO_LONG", "passkey friendly name is too long")
	ErrPasskeyRegistrationInvalid  = infraerrors.BadRequest("PASSKEY_REGISTRATION_INVALID", "passkey registration response is invalid")
	ErrPasskeyCredentialExists     = infraerrors.Conflict("PASSKEY_CREDENTIAL_EXISTS", "passkey credential already exists")
	ErrPasskeyFlowIDRequired       = infraerrors.BadRequest("PASSKEY_FLOW_ID_REQUIRED", "passkey flow id is required")
	ErrPasskeySessionRequired      = infraerrors.BadRequest("PASSKEY_SESSION_REQUIRED", "passkey session data is required")
	ErrPasskeyFlowTypeInvalid      = infraerrors.BadRequest("PASSKEY_FLOW_TYPE_INVALID", "passkey flow type is invalid")
	ErrPasskeyFlowExpired          = infraerrors.BadRequest("PASSKEY_FLOW_EXPIRED", "passkey flow is invalid or expired")
	ErrPasskeyFlowReplayed         = infraerrors.BadRequest("PASSKEY_FLOW_REPLAYED", "passkey flow has already been consumed")
	ErrPasskeyFlowTypeMismatch     = infraerrors.BadRequest("PASSKEY_FLOW_TYPE_MISMATCH", "passkey flow type mismatch")
	ErrPasskeyFlowUserMismatch     = infraerrors.BadRequest("PASSKEY_FLOW_USER_MISMATCH", "passkey flow does not belong to the authenticated user")
	ErrPasskeyRPConfigInvalid      = infraerrors.BadRequest("PASSKEY_RP_CONFIG_INVALID", "passkey relying party configuration is invalid")

	errPasskeyCredentialLookupNotFound = errors.New("passkey credential not found")
	errPasskeyCredentialRevoked        = errors.New("passkey credential revoked")
	errPasskeyUserHandleInvalid        = errors.New("passkey user handle is invalid")
	errPasskeyUserHandleMismatch       = errors.New("passkey user handle mismatch")
)

const (
	passkeyChallengeTTL       = 5 * time.Minute
	passkeyFriendlyNameMaxLen = 100
)

type PasskeyFlowType string

const (
	PasskeyFlowTypeRegistration   PasskeyFlowType = "registration"
	PasskeyFlowTypeAuthentication PasskeyFlowType = "authentication"
)

type PasskeyChallengeConsumeStatus string

const (
	PasskeyChallengeConsumeFound    PasskeyChallengeConsumeStatus = "found"
	PasskeyChallengeConsumeMissing  PasskeyChallengeConsumeStatus = "missing"
	PasskeyChallengeConsumeReplayed PasskeyChallengeConsumeStatus = "replayed"
)

type PasskeyChallengeRecord struct {
	FlowID      string               `json:"flow_id"`
	FlowType    PasskeyFlowType      `json:"flow_type"`
	UserID      int64                `json:"user_id,omitempty"`
	SessionData webauthn.SessionData `json:"session_data"`
	CreatedAt   time.Time            `json:"created_at"`
}

type AuthStateCache interface {
	SetPasskeyChallenge(ctx context.Context, flowID string, record *PasskeyChallengeRecord, ttl time.Duration) error
	ConsumePasskeyChallenge(ctx context.Context, flowID string) (*PasskeyChallengeRecord, PasskeyChallengeConsumeStatus, error)

	SetRecentAuthMarker(ctx context.Context, userID int64, marker *RecentAuthMarker, ttl time.Duration) error
	GetRecentAuthMarker(ctx context.Context, userID int64) (*RecentAuthMarker, error)
}

type PasskeyRPConfig struct {
	RPID          string
	RPDisplayName string
	RPOrigins     []string
}

type PasskeyCredentialRecord struct {
	ID             int64
	UserID         int64
	CredentialID   string
	PublicKey      string
	SignCount      int64
	CreatedAt      time.Time
	Transports     []string
	AAGUID         string
	BackupEligible bool
	BackupState    bool
	FriendlyName   string
	LastUsedAt     *time.Time
	RevokedAt      *time.Time
}

type PasskeyCredentialStore interface {
	ListActiveByUserID(ctx context.Context, userID int64) ([]*PasskeyCredentialRecord, error)
	GetByCredentialID(ctx context.Context, credentialID string) (*PasskeyCredentialRecord, error)
	ExistsActiveByCredentialID(ctx context.Context, credentialID string) (bool, error)
	UpdateFriendlyName(ctx context.Context, id int64, friendlyName string) error
	UpdateRevokedAt(ctx context.Context, id int64, revokedAt time.Time) error
	UpdateSignCount(ctx context.Context, id int64, signCount int64) error
	UpdateLastUsedAt(ctx context.Context, id int64, lastUsedAt time.Time) error
	Create(ctx context.Context, record *PasskeyCredentialRecord) error
}

type PasskeyRegistrationBeginResult struct {
	FlowID    string                       `json:"flow_id"`
	Options   *protocol.CredentialCreation `json:"options"`
	Countdown int                          `json:"countdown"`
}

type PasskeyRegistrationFinishResult struct {
	CredentialID string `json:"credential_id"`
	FriendlyName string `json:"friendly_name"`
}

type PasskeyAuthenticationBeginResult struct {
	FlowID    string                        `json:"flow_id"`
	Options   *protocol.CredentialAssertion `json:"options"`
	Countdown int                           `json:"countdown"`
}

type PasskeyManagementStatus struct {
	FeatureEnabled            bool `json:"feature_enabled"`
	CanManage                 bool `json:"can_manage"`
	HasPasskeys               bool `json:"has_passkeys"`
	ActiveCount               int  `json:"active_count"`
	PasswordFallbackAvailable bool `json:"password_fallback_available"`
}

type PasskeyManagementCredential struct {
	CredentialID   string     `json:"credential_id"`
	FriendlyName   string     `json:"friendly_name"`
	CreatedAt      time.Time  `json:"created_at"`
	LastUsedAt     *time.Time `json:"last_used_at,omitempty"`
	BackupEligible bool       `json:"backup_eligible"`
	Synced         bool       `json:"synced"`
}

type PasskeyManagementListResult struct {
	Items []PasskeyManagementCredential `json:"items"`
}

type PasskeyManagementRevokeResult struct {
	CredentialID              string    `json:"credential_id"`
	RevokedAt                 time.Time `json:"revoked_at"`
	PasswordFallbackAvailable bool      `json:"password_fallback_available"`
}

type passkeyWebAuthnClient interface {
	BeginRegistration(user webauthn.User, opts ...webauthn.RegistrationOption) (creation *protocol.CredentialCreation, session *webauthn.SessionData, err error)
	FinishRegistration(user webauthn.User, session webauthn.SessionData, request *http.Request) (credential *webauthn.Credential, err error)
	BeginDiscoverableLogin(opts ...webauthn.LoginOption) (assertion *protocol.CredentialAssertion, session *webauthn.SessionData, err error)
	FinishPasskeyLogin(handler webauthn.DiscoverableUserHandler, session webauthn.SessionData, response *http.Request) (user webauthn.User, credential *webauthn.Credential, err error)
}

type passkeyEntCredentialStore struct {
	entClient *dbent.Client
}

type passkeyWebAuthnUser struct {
	user        *User
	credentials []webauthn.Credential
}

type PasskeyService struct {
	settingService       *SettingService
	cache                AuthStateCache
	userRepo             UserRepository
	recentAuthService    *RecentAuthService
	credentialStore      PasskeyCredentialStore
	friendlyNameResolver *passkeyFriendlyNameResolver
	webauthnFactory      func(context.Context) (passkeyWebAuthnClient, error)
	now                  func() time.Time

	mu          sync.RWMutex
	cachedCfg   PasskeyRPConfig
	cachedWA    *webauthn.WebAuthn
	hasCachedWA bool
}

func NewPasskeyService(settingService *SettingService, cache AuthStateCache) *PasskeyService {
	svc := &PasskeyService{settingService: settingService, cache: cache}
	svc.friendlyNameResolver = newPasskeyFriendlyNameResolver(nil)
	svc.now = func() time.Time {
		return time.Now().UTC()
	}
	svc.webauthnFactory = func(ctx context.Context) (passkeyWebAuthnClient, error) {
		return svc.GetWebAuthn(ctx)
	}
	return svc
}

func (s *PasskeyService) ResolveRPConfig(ctx context.Context) (PasskeyRPConfig, error) {
	if s.settingService == nil {
		return PasskeyRPConfig{}, ErrPasskeyRPConfigInvalid.WithMetadata(map[string]string{"field": "setting_service"})
	}

	settings, err := s.settingService.GetAllSettings(ctx)
	if err != nil {
		return PasskeyRPConfig{}, fmt.Errorf("get settings: %w", err)
	}

	rpID := strings.TrimSpace(settings.PasskeyRPID)
	if rpID == "" {
		return PasskeyRPConfig{}, ErrPasskeyRPConfigInvalid.WithMetadata(map[string]string{"field": "frontend_url"})
	}

	rpDisplayName := strings.TrimSpace(settings.PasskeyRPName)
	if rpDisplayName == "" {
		rpDisplayName = rpID
	}

	origins := clonePasskeyOrigins(settings.PasskeyAllowedOrigins)
	if len(origins) == 0 {
		return PasskeyRPConfig{}, ErrPasskeyRPConfigInvalid.WithMetadata(map[string]string{"field": "frontend_url"})
	}
	if err := validatePasskeyWebAuthnOrigins(origins); err != nil {
		return PasskeyRPConfig{}, err
	}

	return PasskeyRPConfig{
		RPID:          rpID,
		RPDisplayName: rpDisplayName,
		RPOrigins:     origins,
	}, nil
}

func (s *PasskeyService) GetWebAuthn(ctx context.Context) (*webauthn.WebAuthn, error) {
	cfg, err := s.ResolveRPConfig(ctx)
	if err != nil {
		return nil, err
	}

	s.mu.RLock()
	if s.hasCachedWA && passkeyRPConfigEqual(s.cachedCfg, cfg) {
		cached := s.cachedWA
		s.mu.RUnlock()
		return cached, nil
	}
	s.mu.RUnlock()

	wa, err := webauthn.New(&webauthn.Config{
		RPID:          cfg.RPID,
		RPDisplayName: cfg.RPDisplayName,
		RPOrigins:     clonePasskeyOrigins(cfg.RPOrigins),
	})
	if err != nil {
		return nil, fmt.Errorf("create webauthn: %w", err)
	}

	s.mu.Lock()
	s.cachedCfg = cfg
	s.cachedWA = wa
	s.hasCachedWA = true
	s.mu.Unlock()

	return wa, nil
}

func (s *PasskeyService) IssueChallenge(ctx context.Context, flowType PasskeyFlowType, userID int64, sessionData *webauthn.SessionData) (string, error) {
	if !isPasskeyFlowTypeValid(flowType) {
		return "", ErrPasskeyFlowTypeInvalid
	}
	if sessionData == nil {
		return "", ErrPasskeySessionRequired
	}
	if s.cache == nil {
		return "", fmt.Errorf("auth state cache is not configured")
	}

	flowID := uuid.NewString()
	record := &PasskeyChallengeRecord{
		FlowID:      flowID,
		FlowType:    flowType,
		UserID:      userID,
		SessionData: *sessionData,
		CreatedAt:   time.Now().UTC(),
	}

	if err := s.cache.SetPasskeyChallenge(ctx, flowID, record, passkeyChallengeTTL); err != nil {
		return "", fmt.Errorf("set passkey challenge: %w", err)
	}

	return flowID, nil
}

func (s *PasskeyService) ConsumeChallenge(ctx context.Context, flowID string, expectedFlowType PasskeyFlowType) (*PasskeyChallengeRecord, error) {
	flowID = strings.TrimSpace(flowID)
	if flowID == "" {
		return nil, ErrPasskeyFlowIDRequired
	}
	if expectedFlowType != "" && !isPasskeyFlowTypeValid(expectedFlowType) {
		return nil, ErrPasskeyFlowTypeInvalid
	}
	if s.cache == nil {
		return nil, fmt.Errorf("auth state cache is not configured")
	}

	record, status, err := s.cache.ConsumePasskeyChallenge(ctx, flowID)
	if err != nil {
		return nil, fmt.Errorf("consume passkey challenge: %w", err)
	}

	switch status {
	case PasskeyChallengeConsumeFound:
	case PasskeyChallengeConsumeReplayed:
		return nil, ErrPasskeyFlowReplayed
	default:
		return nil, ErrPasskeyFlowExpired
	}

	if record == nil {
		return nil, ErrPasskeyFlowExpired
	}

	if expectedFlowType != "" && record.FlowType != expectedFlowType {
		return nil, ErrPasskeyFlowTypeMismatch.WithMetadata(map[string]string{
			"expected": string(expectedFlowType),
			"actual":   string(record.FlowType),
		})
	}

	return record, nil
}

func (s *PasskeyService) UpdateAuthenticatorCounter(authenticator *webauthn.Authenticator, authDataCount uint32) uint32 {
	if authenticator == nil {
		return authDataCount
	}
	authenticator.UpdateCounter(authDataCount)
	return authenticator.SignCount
}

func (s *PasskeyService) BeginRegistration(ctx context.Context, userID int64) (*PasskeyRegistrationBeginResult, error) {
	if err := s.ensureEnrollmentAllowed(ctx, userID); err != nil {
		return nil, err
	}

	user, credentials, err := s.loadUserAndCredentials(ctx, userID)
	if err != nil {
		return nil, err
	}

	wa, err := s.getWebAuthnClient(ctx)
	if err != nil {
		return nil, err
	}

	registrationUser, err := s.newWebAuthnUser(user, credentials)
	if err != nil {
		return nil, err
	}

	creation, session, err := wa.BeginRegistration(
		registrationUser,
		webauthn.WithAuthenticatorSelection(protocol.AuthenticatorSelection{
			RequireResidentKey: protocol.ResidentKeyRequired(),
			ResidentKey:        protocol.ResidentKeyRequirementRequired,
			UserVerification:   protocol.VerificationRequired,
		}),
		webauthn.WithConveyancePreference(protocol.PreferNoAttestation),
		webauthn.WithExclusions(webauthn.Credentials(registrationUser.WebAuthnCredentials()).CredentialDescriptors()),
	)
	if err != nil {
		return nil, fmt.Errorf("begin passkey registration: %w", err)
	}

	flowID, err := s.IssueChallenge(ctx, PasskeyFlowTypeRegistration, userID, session)
	if err != nil {
		return nil, err
	}

	return &PasskeyRegistrationBeginResult{
		FlowID:    flowID,
		Options:   creation,
		Countdown: int(passkeyChallengeTTL.Seconds()),
	}, nil
}

func (s *PasskeyService) FinishRegistration(ctx context.Context, userID int64, flowID, friendlyName string, request *http.Request) (*PasskeyRegistrationFinishResult, error) {
	if err := s.ensureEnrollmentAllowed(ctx, userID); err != nil {
		return nil, err
	}
	if request == nil {
		return nil, ErrPasskeyRegistrationInvalid.WithMetadata(map[string]string{"field": "request"})
	}

	challenge, err := s.ConsumeChallenge(ctx, flowID, PasskeyFlowTypeRegistration)
	if err != nil {
		return nil, err
	}
	if challenge.UserID != userID {
		return nil, ErrPasskeyFlowUserMismatch
	}

	user, credentials, err := s.loadUserAndCredentials(ctx, userID)
	if err != nil {
		return nil, err
	}

	wa, err := s.getWebAuthnClient(ctx)
	if err != nil {
		return nil, err
	}

	registrationUser, err := s.newWebAuthnUser(user, credentials)
	if err != nil {
		return nil, err
	}

	credential, err := wa.FinishRegistration(registrationUser, challenge.SessionData, request)
	if err != nil {
		return nil, ErrPasskeyRegistrationInvalid.WithCause(err)
	}

	storedCredentialID := encodePasskeyBinary(credential.ID)
	exists, err := s.credentialExists(ctx, storedCredentialID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrPasskeyCredentialExists
	}

	aaguid := encodePasskeyAAGUID(credential.Authenticator.AAGUID)
	record := &PasskeyCredentialRecord{
		UserID:         userID,
		CredentialID:   storedCredentialID,
		PublicKey:      encodePasskeyBinary(credential.PublicKey),
		SignCount:      int64(credential.Authenticator.SignCount),
		Transports:     passkeyTransportsToStrings(credential.Transport),
		AAGUID:         aaguid,
		BackupEligible: credential.Flags.BackupEligible,
		BackupState:    credential.Flags.BackupState,
		FriendlyName:   s.resolvePasskeyFriendlyName(ctx, friendlyName, aaguid),
	}

	if err := s.createCredential(ctx, record); err != nil {
		if errors.Is(err, ErrPasskeyCredentialExists) {
			return nil, ErrPasskeyCredentialExists
		}
		return nil, fmt.Errorf("create passkey credential: %w", err)
	}

	return &PasskeyRegistrationFinishResult{
		CredentialID: record.CredentialID,
		FriendlyName: record.FriendlyName,
	}, nil
}

func (s *PasskeyService) BeginAuthentication(ctx context.Context) (*PasskeyAuthenticationBeginResult, error) {
	if s.settingService == nil || !s.settingService.IsPasskeyEnabled(ctx) {
		return nil, ErrPasskeyNotEnabled
	}

	wa, err := s.getWebAuthnClient(ctx)
	if err != nil {
		return nil, err
	}

	assertion, session, err := wa.BeginDiscoverableLogin(
		webauthn.WithUserVerification(protocol.VerificationRequired),
	)
	if err != nil {
		return nil, fmt.Errorf("begin passkey authentication: %w", err)
	}

	flowID, err := s.IssueChallenge(ctx, PasskeyFlowTypeAuthentication, 0, session)
	if err != nil {
		return nil, err
	}

	return &PasskeyAuthenticationBeginResult{
		FlowID:    flowID,
		Options:   assertion,
		Countdown: int(passkeyChallengeTTL.Seconds()),
	}, nil
}

func (s *PasskeyService) FinishAuthentication(ctx context.Context, flowID string, request *http.Request) (*User, error) {
	if s.settingService == nil || !s.settingService.IsPasskeyEnabled(ctx) {
		return nil, ErrPasskeyNotEnabled
	}
	if request == nil {
		return nil, ErrInvalidCredentials
	}

	challenge, err := s.ConsumeChallenge(ctx, flowID, PasskeyFlowTypeAuthentication)
	if err != nil {
		return nil, s.genericAuthenticationError(err)
	}

	wa, err := s.getWebAuthnClient(ctx)
	if err != nil {
		return nil, err
	}

	var (
		resolvedUser       *User
		resolvedCredential *PasskeyCredentialRecord
	)

	_, credential, err := wa.FinishPasskeyLogin(func(rawID, userHandle []byte) (webauthn.User, error) {
		loadedUser, webauthnUser, loadedCredential, lookupErr := s.loadAuthenticationUser(ctx, rawID, userHandle)
		if lookupErr != nil {
			return nil, lookupErr
		}
		resolvedUser = loadedUser
		resolvedCredential = loadedCredential
		return webauthnUser, nil
	}, challenge.SessionData, request)
	if err != nil {
		return nil, s.genericAuthenticationError(err)
	}
	if resolvedUser == nil || resolvedCredential == nil || credential == nil {
		return nil, s.genericAuthenticationError(errors.New("passkey authentication result is incomplete"))
	}

	authenticator := &webauthn.Authenticator{SignCount: passkeySignCount(resolvedCredential.SignCount)}
	updatedSignCount := int64(s.UpdateAuthenticatorCounter(authenticator, credential.Authenticator.SignCount))
	if err := s.updateCredentialSignCount(ctx, resolvedCredential.ID, updatedSignCount); err != nil {
		return nil, fmt.Errorf("update passkey credential sign count: %w", err)
	}

	if err := s.updateCredentialLastUsedAt(ctx, resolvedCredential.ID, s.currentTime()); err != nil {
		return nil, fmt.Errorf("update passkey credential last used at: %w", err)
	}

	return resolvedUser, nil
}

func newPasskeyCredentialStore(entClient *dbent.Client) PasskeyCredentialStore {
	if entClient == nil {
		return nil
	}
	return &passkeyEntCredentialStore{entClient: entClient}
}

func (s *passkeyEntCredentialStore) ListActiveByUserID(ctx context.Context, userID int64) ([]*PasskeyCredentialRecord, error) {
	if s == nil || s.entClient == nil {
		return nil, fmt.Errorf("passkey credential store is not configured")
	}

	rows, err := s.entClient.PasskeyCredential.Query().
		Where(
			passkeycredential.UserIDEQ(userID),
			passkeycredential.RevokedAtIsNil(),
		).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query passkey credentials: %w", err)
	}

	records := make([]*PasskeyCredentialRecord, 0, len(rows))
	for _, row := range rows {
		records = append(records, &PasskeyCredentialRecord{
			ID:             row.ID,
			UserID:         row.UserID,
			CredentialID:   row.CredentialID,
			PublicKey:      row.PublicKey,
			SignCount:      row.SignCount,
			CreatedAt:      row.CreatedAt,
			Transports:     slices.Clone(row.Transports),
			AAGUID:         row.Aaguid,
			BackupEligible: row.BackupEligible,
			BackupState:    row.BackupState,
			FriendlyName:   row.FriendlyName,
			LastUsedAt:     row.LastUsedAt,
			RevokedAt:      row.RevokedAt,
		})
	}

	return records, nil
}

func (s *passkeyEntCredentialStore) GetByCredentialID(ctx context.Context, credentialID string) (*PasskeyCredentialRecord, error) {
	if s == nil || s.entClient == nil {
		return nil, fmt.Errorf("passkey credential store is not configured")
	}

	row, err := s.entClient.PasskeyCredential.Query().
		Where(passkeycredential.CredentialIDEQ(strings.TrimSpace(credentialID))).
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, errPasskeyCredentialLookupNotFound
		}
		return nil, fmt.Errorf("query passkey credential by id: %w", err)
	}

	return &PasskeyCredentialRecord{
		ID:             row.ID,
		UserID:         row.UserID,
		CredentialID:   row.CredentialID,
		PublicKey:      row.PublicKey,
		SignCount:      row.SignCount,
		CreatedAt:      row.CreatedAt,
		Transports:     slices.Clone(row.Transports),
		AAGUID:         row.Aaguid,
		BackupEligible: row.BackupEligible,
		BackupState:    row.BackupState,
		FriendlyName:   row.FriendlyName,
		LastUsedAt:     row.LastUsedAt,
		RevokedAt:      row.RevokedAt,
	}, nil
}

func (s *passkeyEntCredentialStore) ExistsActiveByCredentialID(ctx context.Context, credentialID string) (bool, error) {
	if s == nil || s.entClient == nil {
		return false, fmt.Errorf("passkey credential store is not configured")
	}

	exists, err := s.entClient.PasskeyCredential.Query().
		Where(
			passkeycredential.CredentialIDEQ(strings.TrimSpace(credentialID)),
			passkeycredential.RevokedAtIsNil(),
		).
		Exist(ctx)
	if err != nil {
		return false, fmt.Errorf("query passkey credential: %w", err)
	}

	return exists, nil
}

func (s *passkeyEntCredentialStore) UpdateFriendlyName(ctx context.Context, id int64, friendlyName string) error {
	if s == nil || s.entClient == nil {
		return fmt.Errorf("passkey credential store is not configured")
	}

	if _, err := s.entClient.PasskeyCredential.Update().
		Where(passkeycredential.IDEQ(id)).
		SetFriendlyName(friendlyName).
		Save(ctx); err != nil {
		return fmt.Errorf("update passkey friendly name: %w", err)
	}

	return nil
}

func (s *passkeyEntCredentialStore) UpdateRevokedAt(ctx context.Context, id int64, revokedAt time.Time) error {
	if s == nil || s.entClient == nil {
		return fmt.Errorf("passkey credential store is not configured")
	}

	if _, err := s.entClient.PasskeyCredential.Update().
		Where(passkeycredential.IDEQ(id)).
		SetRevokedAt(revokedAt).
		Save(ctx); err != nil {
		return fmt.Errorf("update passkey revoked at: %w", err)
	}

	return nil
}

func (s *passkeyEntCredentialStore) UpdateSignCount(ctx context.Context, id int64, signCount int64) error {
	if s == nil || s.entClient == nil {
		return fmt.Errorf("passkey credential store is not configured")
	}

	if _, err := s.entClient.PasskeyCredential.Update().
		Where(passkeycredential.IDEQ(id)).
		SetSignCount(signCount).
		Save(ctx); err != nil {
		return fmt.Errorf("update passkey sign count: %w", err)
	}

	return nil
}

func (s *passkeyEntCredentialStore) UpdateLastUsedAt(ctx context.Context, id int64, lastUsedAt time.Time) error {
	if s == nil || s.entClient == nil {
		return fmt.Errorf("passkey credential store is not configured")
	}

	if _, err := s.entClient.PasskeyCredential.Update().
		Where(passkeycredential.IDEQ(id)).
		SetLastUsedAt(lastUsedAt).
		Save(ctx); err != nil {
		return fmt.Errorf("update passkey last used at: %w", err)
	}

	return nil
}

func (s *passkeyEntCredentialStore) Create(ctx context.Context, record *PasskeyCredentialRecord) error {
	if s == nil || s.entClient == nil {
		return fmt.Errorf("passkey credential store is not configured")
	}
	if record == nil {
		return fmt.Errorf("passkey credential record is required")
	}

	create := s.entClient.PasskeyCredential.Create().
		SetUserID(record.UserID).
		SetCredentialID(record.CredentialID).
		SetPublicKey(record.PublicKey).
		SetSignCount(record.SignCount).
		SetTransports(slices.Clone(record.Transports)).
		SetAaguid(record.AAGUID).
		SetBackupEligible(record.BackupEligible).
		SetBackupState(record.BackupState).
		SetFriendlyName(record.FriendlyName)

	if record.LastUsedAt != nil {
		create.SetLastUsedAt(*record.LastUsedAt)
	}
	if record.RevokedAt != nil {
		create.SetRevokedAt(*record.RevokedAt)
	}

	if _, err := create.Save(ctx); err != nil {
		if dbent.IsConstraintError(err) {
			return ErrPasskeyCredentialExists.WithCause(err)
		}
		return err
	}

	return nil
}

func (u *passkeyWebAuthnUser) WebAuthnID() []byte {
	if u == nil || u.user == nil {
		return nil
	}
	return []byte(strconv.FormatInt(u.user.ID, 10))
}

func (u *passkeyWebAuthnUser) WebAuthnName() string {
	if u == nil || u.user == nil {
		return ""
	}
	return u.user.Email
}

func (u *passkeyWebAuthnUser) WebAuthnDisplayName() string {
	if u == nil || u.user == nil {
		return ""
	}
	if displayName := strings.TrimSpace(u.user.Username); displayName != "" {
		return displayName
	}
	return u.user.Email
}

func (u *passkeyWebAuthnUser) WebAuthnCredentials() []webauthn.Credential {
	if u == nil {
		return nil
	}
	return u.credentials
}

func (s *PasskeyService) ensureEnrollmentAllowed(ctx context.Context, userID int64) error {
	if userID <= 0 {
		return ErrPasskeyUserIDInvalid
	}
	if !s.isPasskeyEnabled(ctx) {
		return ErrPasskeyNotEnabled
	}
	if s.recentAuthService == nil {
		return ErrRecentAuthRequired
	}
	if err := s.recentAuthService.RequireRecentAuth(ctx, userID); err != nil {
		return err
	}
	return nil
}

func (s *PasskeyService) GetManagementStatus(ctx context.Context, userID int64) (*PasskeyManagementStatus, error) {
	if userID <= 0 {
		return nil, ErrPasskeyUserIDInvalid
	}

	status := &PasskeyManagementStatus{
		FeatureEnabled:            s.isPasskeyEnabled(ctx),
		PasswordFallbackAvailable: true,
	}
	if !status.FeatureEnabled {
		return status, nil
	}

	credentials, err := s.listActiveCredentials(ctx, userID)
	if err != nil {
		return nil, err
	}

	status.ActiveCount = len(credentials)
	status.HasPasskeys = len(credentials) > 0
	status.CanManage = s.hasRecentAuth(ctx, userID)

	return status, nil
}

func (s *PasskeyService) ListManagementCredentials(ctx context.Context, userID int64) (*PasskeyManagementListResult, error) {
	if err := s.ensureManagementReadAllowed(ctx, userID); err != nil {
		return nil, err
	}

	records, err := s.listActiveCredentials(ctx, userID)
	if err != nil {
		return nil, err
	}

	items := make([]PasskeyManagementCredential, 0, len(records))
	for _, record := range records {
		if record == nil {
			continue
		}
		items = append(items, passkeyRecordToManagementCredential(record))
	}

	return &PasskeyManagementListResult{Items: items}, nil
}

func (s *PasskeyService) RenameCredential(ctx context.Context, userID int64, credentialID, friendlyName string) (*PasskeyManagementCredential, error) {
	if err := s.ensureManagementMutationAllowed(ctx, userID); err != nil {
		return nil, err
	}

	normalizedName, err := normalizePasskeyFriendlyName(friendlyName)
	if err != nil {
		return nil, err
	}

	record, err := s.loadOwnedCredential(ctx, userID, credentialID)
	if err != nil {
		return nil, err
	}
	if record.RevokedAt != nil {
		return nil, ErrPasskeyCredentialNotFound
	}

	if err := s.updateCredentialFriendlyName(ctx, record.ID, normalizedName); err != nil {
		return nil, fmt.Errorf("update passkey credential friendly name: %w", err)
	}

	record.FriendlyName = normalizedName
	credential := passkeyRecordToManagementCredential(record)
	return &credential, nil
}

func (s *PasskeyService) RevokeCredential(ctx context.Context, userID int64, credentialID string) (*PasskeyManagementRevokeResult, error) {
	if err := s.ensureManagementMutationAllowed(ctx, userID); err != nil {
		return nil, err
	}

	record, err := s.loadOwnedCredential(ctx, userID, credentialID)
	if err != nil {
		return nil, err
	}

	revokedAt := s.currentTime()
	if record.RevokedAt != nil {
		revokedAt = record.RevokedAt.UTC()
	} else {
		if err := s.updateCredentialRevokedAt(ctx, record.ID, revokedAt); err != nil {
			return nil, fmt.Errorf("revoke passkey credential: %w", err)
		}
	}

	return &PasskeyManagementRevokeResult{
		CredentialID:              record.CredentialID,
		RevokedAt:                 revokedAt,
		PasswordFallbackAvailable: true,
	}, nil
}

func (s *PasskeyService) ensureManagementReadAllowed(ctx context.Context, userID int64) error {
	if userID <= 0 {
		return ErrPasskeyUserIDInvalid
	}
	if !s.isPasskeyEnabled(ctx) {
		return ErrPasskeyNotEnabled
	}
	return nil
}

func (s *PasskeyService) ensureManagementMutationAllowed(ctx context.Context, userID int64) error {
	if err := s.ensureManagementReadAllowed(ctx, userID); err != nil {
		return err
	}
	if s.recentAuthService == nil {
		return ErrRecentAuthRequired
	}
	return s.recentAuthService.RequireRecentAuth(ctx, userID)
}

func (s *PasskeyService) isPasskeyEnabled(ctx context.Context) bool {
	return s.settingService != nil && s.settingService.IsPasskeyEnabled(ctx)
}

func (s *PasskeyService) hasRecentAuth(ctx context.Context, userID int64) bool {
	if s.recentAuthService == nil {
		return false
	}
	marker, err := s.recentAuthService.GetRecentAuth(ctx, userID)
	if err != nil {
		return false
	}
	return marker != nil
}

func (s *PasskeyService) loadUserAndCredentials(ctx context.Context, userID int64) (*User, []*PasskeyCredentialRecord, error) {
	if s.userRepo == nil {
		return nil, nil, fmt.Errorf("passkey user repository is not configured")
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("get user: %w", err)
	}

	credentials, err := s.listActiveCredentials(ctx, userID)
	if err != nil {
		return nil, nil, err
	}

	return user, credentials, nil
}

func (s *PasskeyService) listActiveCredentials(ctx context.Context, userID int64) ([]*PasskeyCredentialRecord, error) {
	if s.credentialStore == nil {
		return nil, fmt.Errorf("passkey credential store is not configured")
	}
	credentials, err := s.credentialStore.ListActiveByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list passkey credentials: %w", err)
	}
	return credentials, nil
}

func (s *PasskeyService) credentialExists(ctx context.Context, credentialID string) (bool, error) {
	if s.credentialStore == nil {
		return false, fmt.Errorf("passkey credential store is not configured")
	}
	exists, err := s.credentialStore.ExistsActiveByCredentialID(ctx, credentialID)
	if err != nil {
		return false, fmt.Errorf("check passkey credential: %w", err)
	}
	return exists, nil
}

func (s *PasskeyService) createCredential(ctx context.Context, record *PasskeyCredentialRecord) error {
	if s.credentialStore == nil {
		return fmt.Errorf("passkey credential store is not configured")
	}
	return s.credentialStore.Create(ctx, record)
}

func (s *PasskeyService) getCredentialByID(ctx context.Context, credentialID string) (*PasskeyCredentialRecord, error) {
	if s.credentialStore == nil {
		return nil, fmt.Errorf("passkey credential store is not configured")
	}

	record, err := s.credentialStore.GetByCredentialID(ctx, credentialID)
	if err != nil {
		return nil, err
	}

	return record, nil
}

func (s *PasskeyService) loadOwnedCredential(ctx context.Context, userID int64, credentialID string) (*PasskeyCredentialRecord, error) {
	credentialID = strings.TrimSpace(credentialID)
	if credentialID == "" {
		return nil, ErrPasskeyCredentialIDRequired
	}

	record, err := s.getCredentialByID(ctx, credentialID)
	if err != nil {
		if errors.Is(err, errPasskeyCredentialLookupNotFound) {
			return nil, ErrPasskeyCredentialNotFound
		}
		return nil, fmt.Errorf("get passkey credential: %w", err)
	}
	if record == nil || record.UserID != userID {
		return nil, ErrPasskeyCredentialNotFound
	}

	return record, nil
}

func (s *PasskeyService) updateCredentialFriendlyName(ctx context.Context, id int64, friendlyName string) error {
	if s.credentialStore == nil {
		return fmt.Errorf("passkey credential store is not configured")
	}
	return s.credentialStore.UpdateFriendlyName(ctx, id, friendlyName)
}

func (s *PasskeyService) updateCredentialRevokedAt(ctx context.Context, id int64, revokedAt time.Time) error {
	if s.credentialStore == nil {
		return fmt.Errorf("passkey credential store is not configured")
	}
	return s.credentialStore.UpdateRevokedAt(ctx, id, revokedAt)
}

func (s *PasskeyService) updateCredentialSignCount(ctx context.Context, id int64, signCount int64) error {
	if s.credentialStore == nil {
		return fmt.Errorf("passkey credential store is not configured")
	}
	return s.credentialStore.UpdateSignCount(ctx, id, signCount)
}

func (s *PasskeyService) updateCredentialLastUsedAt(ctx context.Context, id int64, usedAt time.Time) error {
	if s.credentialStore == nil {
		return fmt.Errorf("passkey credential store is not configured")
	}
	return s.credentialStore.UpdateLastUsedAt(ctx, id, usedAt)
}

func (s *PasskeyService) loadAuthenticationUser(ctx context.Context, rawCredentialID, userHandle []byte) (*User, *passkeyWebAuthnUser, *PasskeyCredentialRecord, error) {
	storedCredentialID := encodePasskeyBinary(rawCredentialID)
	if strings.TrimSpace(storedCredentialID) == "" {
		return nil, nil, nil, errPasskeyCredentialLookupNotFound
	}

	credentialRecord, err := s.getCredentialByID(ctx, storedCredentialID)
	if err != nil {
		return nil, nil, nil, err
	}
	if credentialRecord == nil {
		return nil, nil, nil, errPasskeyCredentialLookupNotFound
	}
	if credentialRecord.RevokedAt != nil {
		return nil, nil, nil, errPasskeyCredentialRevoked
	}

	handleUserID, err := parsePasskeyUserHandle(userHandle)
	if err != nil {
		return nil, nil, nil, err
	}
	if handleUserID != credentialRecord.UserID {
		return nil, nil, nil, errPasskeyUserHandleMismatch
	}

	user, credentials, err := s.loadUserAndCredentials(ctx, credentialRecord.UserID)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil, nil, nil, errPasskeyCredentialLookupNotFound
		}
		return nil, nil, nil, err
	}

	webauthnUser, err := s.newWebAuthnUser(user, credentials)
	if err != nil {
		return nil, nil, nil, err
	}

	return user, webauthnUser, credentialRecord, nil
}

func (s *PasskeyService) genericAuthenticationError(cause error) error {
	if cause == nil {
		return ErrInvalidCredentials
	}
	return ErrInvalidCredentials.WithCause(cause)
}

func parsePasskeyUserHandle(userHandle []byte) (int64, error) {
	handle := strings.TrimSpace(string(userHandle))
	if handle == "" {
		return 0, errPasskeyUserHandleInvalid
	}

	userID, err := strconv.ParseInt(handle, 10, 64)
	if err != nil || userID <= 0 {
		return 0, errPasskeyUserHandleInvalid
	}

	return userID, nil
}

func (s *PasskeyService) newWebAuthnUser(user *User, records []*PasskeyCredentialRecord) (*passkeyWebAuthnUser, error) {
	credentials, err := passkeyRecordsToWebAuthnCredentials(records)
	if err != nil {
		return nil, err
	}
	return &passkeyWebAuthnUser{user: user, credentials: credentials}, nil
}

func (s *PasskeyService) getWebAuthnClient(ctx context.Context) (passkeyWebAuthnClient, error) {
	if s.webauthnFactory != nil {
		return s.webauthnFactory(ctx)
	}
	return s.GetWebAuthn(ctx)
}

func (s *PasskeyService) SetPasskeyAAGUIDMetadataCache(metadataCache PasskeyAAGUIDMetadataCache) {
	if s == nil {
		return
	}
	s.friendlyNameResolver = newPasskeyFriendlyNameResolver(metadataCache)
}

func (s *PasskeyService) currentTime() time.Time {
	if s.now != nil {
		return s.now().UTC()
	}
	return time.Now().UTC()
}

func (s *PasskeyService) resolvePasskeyFriendlyName(ctx context.Context, providedFriendlyName, aaguid string) string {
	if s != nil && s.friendlyNameResolver != nil {
		return s.friendlyNameResolver.Resolve(ctx, providedFriendlyName, aaguid, s.currentTime())
	}
	return passkeyFriendlyName(providedFriendlyName, s.currentTime())
}

func passkeyRecordsToWebAuthnCredentials(records []*PasskeyCredentialRecord) ([]webauthn.Credential, error) {
	if len(records) == 0 {
		return nil, nil
	}

	credentials := make([]webauthn.Credential, 0, len(records))
	for _, record := range records {
		if record == nil {
			continue
		}

		credentialID, err := decodePasskeyBinary(record.CredentialID)
		if err != nil {
			return nil, fmt.Errorf("decode passkey credential id: %w", err)
		}
		publicKey, err := decodePasskeyBinary(record.PublicKey)
		if err != nil {
			return nil, fmt.Errorf("decode passkey public key: %w", err)
		}
		aaguid, err := decodePasskeyAAGUID(record.AAGUID)
		if err != nil {
			return nil, fmt.Errorf("decode passkey aaguid: %w", err)
		}

		credentials = append(credentials, webauthn.Credential{
			ID:        credentialID,
			PublicKey: publicKey,
			Transport: passkeyStringsToTransports(record.Transports),
			Flags: webauthn.CredentialFlags{
				BackupEligible: record.BackupEligible,
				BackupState:    record.BackupState,
			},
			Authenticator: webauthn.Authenticator{
				AAGUID:    aaguid,
				SignCount: passkeySignCount(record.SignCount),
			},
		})
	}

	return credentials, nil
}

func passkeyFriendlyName(friendlyName string, now time.Time) string {
	if trimmed := strings.TrimSpace(friendlyName); trimmed != "" {
		return trimmed
	}
	return fmt.Sprintf("Passkey %s", now.UTC().Format("2006-01-02 15:04"))
}

func normalizePasskeyFriendlyName(friendlyName string) (string, error) {
	trimmed := strings.TrimSpace(friendlyName)
	if trimmed == "" {
		return "", ErrPasskeyFriendlyNameRequired
	}
	if len(trimmed) > passkeyFriendlyNameMaxLen {
		return "", ErrPasskeyFriendlyNameTooLong.WithMetadata(map[string]string{"max_length": strconv.Itoa(passkeyFriendlyNameMaxLen)})
	}
	return trimmed, nil
}

func passkeyRecordToManagementCredential(record *PasskeyCredentialRecord) PasskeyManagementCredential {
	if record == nil {
		return PasskeyManagementCredential{}
	}

	var lastUsedAt *time.Time
	if record.LastUsedAt != nil {
		usedAt := record.LastUsedAt.UTC()
		lastUsedAt = &usedAt
	}

	return PasskeyManagementCredential{
		CredentialID:   record.CredentialID,
		FriendlyName:   record.FriendlyName,
		CreatedAt:      record.CreatedAt.UTC(),
		LastUsedAt:     lastUsedAt,
		BackupEligible: record.BackupEligible,
		Synced:         record.BackupState,
	}
}

func passkeySignCount(value int64) uint32 {
	if value <= 0 {
		return 0
	}
	return uint32(value)
}

func encodePasskeyBinary(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodePasskeyBinary(value string) ([]byte, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(trimmed)
	if err != nil {
		return nil, err
	}
	return decoded, nil
}

func encodePasskeyAAGUID(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	if len(raw) == 16 {
		var parsed uuid.UUID
		copy(parsed[:], raw)
		return parsed.String()
	}
	return hex.EncodeToString(raw)
}

func decodePasskeyAAGUID(value string) ([]byte, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}
	if parsed, err := uuid.Parse(trimmed); err == nil {
		decoded := make([]byte, len(parsed))
		copy(decoded, parsed[:])
		return decoded, nil
	}
	decoded, err := hex.DecodeString(trimmed)
	if err != nil {
		return nil, err
	}
	return decoded, nil
}

func passkeyTransportsToStrings(transports []protocol.AuthenticatorTransport) []string {
	if len(transports) == 0 {
		return nil
	}
	out := make([]string, 0, len(transports))
	for _, transport := range transports {
		if trimmed := strings.TrimSpace(string(transport)); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func passkeyStringsToTransports(transports []string) []protocol.AuthenticatorTransport {
	if len(transports) == 0 {
		return nil
	}
	out := make([]protocol.AuthenticatorTransport, 0, len(transports))
	for _, transport := range transports {
		if trimmed := strings.TrimSpace(transport); trimmed != "" {
			out = append(out, protocol.AuthenticatorTransport(trimmed))
		}
	}
	return out
}

func passkeyRPConfigEqual(a, b PasskeyRPConfig) bool {
	return a.RPID == b.RPID && a.RPDisplayName == b.RPDisplayName && slices.Equal(a.RPOrigins, b.RPOrigins)
}

func isPasskeyFlowTypeValid(flowType PasskeyFlowType) bool {
	switch flowType {
	case PasskeyFlowTypeRegistration, PasskeyFlowTypeAuthentication:
		return true
	default:
		return false
	}
}

func clonePasskeyOrigins(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, len(items))
	copy(out, items)
	return out
}

func validatePasskeyWebAuthnOrigins(origins []string) error {
	for _, origin := range origins {
		if err := validatePasskeyWebAuthnOrigin(origin); err != nil {
			return err
		}
	}

	return nil
}

func validatePasskeyWebAuthnOrigin(origin string) error {
	trimmed := strings.TrimSpace(origin)
	u, err := url.Parse(trimmed)
	if err != nil || u == nil || u.Host == "" {
		return ErrPasskeyRPConfigInvalid.WithMetadata(map[string]string{
			"field":  "frontend_url",
			"origin": trimmed,
		})
	}

	scheme := strings.ToLower(strings.TrimSpace(u.Scheme))
	if scheme == "https" {
		return nil
	}

	host := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(u.Hostname())), ".")
	if scheme == "http" && (host == "localhost" || host == "127.0.0.1") {
		return nil
	}

	return ErrPasskeyRPConfigInvalid.WithMetadata(map[string]string{
		"field":  "frontend_url",
		"origin": trimmed,
	})
}
