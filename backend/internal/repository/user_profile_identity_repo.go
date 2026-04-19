package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

var errAuthIdentityOwnershipConflict = errors.New("auth identity belongs to another user")
var errAuthIdentityChannelOwnershipConflict = errors.New("auth identity channel belongs to another identity")

type UpsertAuthIdentityInput struct {
	ProviderType    string
	ProviderKey     string
	ProviderSubject string
	Issuer          *string
	VerifiedAt      *time.Time
	Metadata        map[string]any
}

type AuthIdentityRecord struct {
	ID              int64
	UserID          int64
	ProviderType    string
	ProviderKey     string
	ProviderSubject string
	VerifiedAt      *time.Time
	Issuer          *string
	Metadata        map[string]any
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type identityAdoptionDecisionRow struct {
	IdentityID       int64
	AdoptDisplayName bool
	AdoptAvatar      bool
	DecidedAt        time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type UpsertAuthIdentityChannelInput struct {
	ProviderType   string
	ProviderKey    string
	Channel        string
	ChannelAppID   string
	ChannelSubject string
	Metadata       map[string]any
}

type AuthIdentityChannelRecord struct {
	ID             int64
	IdentityID     int64
	ProviderType   string
	ProviderKey    string
	Channel        string
	ChannelAppID   string
	ChannelSubject string
	Metadata       map[string]any
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type CreatePendingAuthSessionInput struct {
	SessionToken        string
	Intent              string
	ProviderType        string
	ProviderKey         string
	ProviderSubject     string
	TargetUserID        *int64
	RedirectTo          string
	Metadata            map[string]any
	ResolvedEmail       string
	PendingPasswordHash string
	UpstreamIdentity    map[string]any
	EmailVerifiedAt     *time.Time
	PasswordVerifiedAt  *time.Time
	TOTPVerifiedAt      *time.Time
	ExpiresAt           time.Time
}

type PendingAuthSessionRecord struct {
	ID                  int64
	SessionToken        string
	Intent              string
	ProviderType        string
	ProviderKey         string
	ProviderSubject     string
	TargetUserID        *int64
	RedirectTo          string
	Metadata            map[string]any
	ResolvedEmail       string
	PendingPasswordHash string
	UpstreamIdentity    map[string]any
	EmailVerifiedAt     *time.Time
	PasswordVerifiedAt  *time.Time
	TOTPVerifiedAt      *time.Time
	ExpiresAt           time.Time
	ConsumedAt          *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type userAvatarRow struct {
	ID              int64
	UserID          int64
	StorageProvider string
	StorageKey      string
	URL             string
	ContentType     string
	ByteSize        int64
	SHA256          string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (r *userRepository) sqlExec(ctx context.Context) sqlExecutor {
	if tx := dbent.TxFromContext(ctx); tx != nil {
		return tx.Client()
	}
	return r.sql
}

func (r *userRepository) TryCreateProviderDefaultBindGrant(ctx context.Context, userID int64, providerType string) (bool, error) {
	if r == nil || r.sql == nil {
		return false, fmt.Errorf("nil sql executor")
	}

	result, err := r.sqlExec(ctx).ExecContext(ctx, `
INSERT INTO user_provider_default_grants (user_id, provider_type, granted_at, created_at)
VALUES ($1, $2, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (user_id, provider_type) DO NOTHING`,
		userID,
		strings.TrimSpace(providerType),
	)
	if err != nil {
		return false, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rowsAffected > 0, nil
}

func (r *userRepository) DeleteProviderDefaultBindGrant(ctx context.Context, userID int64, providerType string) error {
	if r == nil || r.sql == nil {
		return fmt.Errorf("nil sql executor")
	}

	_, err := r.sqlExec(ctx).ExecContext(ctx, `
DELETE FROM user_provider_default_grants
WHERE user_id = $1 AND provider_type = $2`,
		userID,
		strings.TrimSpace(providerType),
	)
	return err
}

func (r *userRepository) UpsertAuthIdentity(ctx context.Context, userID int64, input UpsertAuthIdentityInput) (*AuthIdentityRecord, error) {
	if r == nil || r.sql == nil {
		return nil, fmt.Errorf("nil sql executor")
	}

	providerType := strings.TrimSpace(input.ProviderType)
	providerKey := strings.TrimSpace(input.ProviderKey)
	providerSubject := strings.TrimSpace(input.ProviderSubject)
	if userID == 0 || providerType == "" || providerKey == "" || providerSubject == "" {
		return nil, fmt.Errorf("user id, provider type, provider key, and provider subject are required")
	}

	existing, err := r.FindAuthIdentity(ctx, providerType, providerKey, providerSubject)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if existing != nil {
		if existing.UserID != userID {
			return nil, errAuthIdentityOwnershipConflict
		}
		if err := r.updateAuthIdentity(ctx, existing.ID, input); err != nil {
			return nil, err
		}
		return r.GetAuthIdentityByID(ctx, existing.ID)
	}

	metadataJSON, err := marshalAuthIdentityMetadata(input.Metadata)
	if err != nil {
		return nil, err
	}

	if _, err := r.sql.ExecContext(ctx, `
INSERT INTO auth_identities (
	user_id,
	provider_type,
	provider_key,
	provider_subject,
	verified_at,
	issuer,
	metadata,
	created_at,
	updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		userID,
		providerType,
		providerKey,
		providerSubject,
		nullableTime(input.VerifiedAt),
		nullableString(input.Issuer),
		metadataJSON,
	); err != nil {
		return nil, err
	}

	return r.FindAuthIdentity(ctx, providerType, providerKey, providerSubject)
}

func (r *userRepository) GetAvatar(ctx context.Context, userID int64) (*service.UserAvatar, error) {
	if r == nil || r.sql == nil {
		return nil, fmt.Errorf("nil sql executor")
	}

	row := userAvatarRow{}
	if err := scanSingleRow(ctx, r.sql, `
SELECT
	id,
	user_id,
	storage_provider,
	storage_key,
	url,
	content_type,
	byte_size,
	sha256,
	created_at,
	updated_at
FROM user_avatars
WHERE user_id = $1`,
		[]any{userID},
		&row.ID,
		&row.UserID,
		&row.StorageProvider,
		&row.StorageKey,
		&row.URL,
		&row.ContentType,
		&row.ByteSize,
		&row.SHA256,
		&row.CreatedAt,
		&row.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrUserAvatarNotFound
		}
		return nil, err
	}

	return &service.UserAvatar{
		ID:              row.ID,
		UserID:          row.UserID,
		StorageProvider: row.StorageProvider,
		StorageKey:      row.StorageKey,
		URL:             row.URL,
		ContentType:     row.ContentType,
		ByteSize:        row.ByteSize,
		SHA256:          row.SHA256,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}, nil
}

func (r *userRepository) UpsertAvatar(ctx context.Context, userID int64, input service.UpsertUserAvatarInput) (*service.UserAvatar, error) {
	if r == nil || r.sql == nil {
		return nil, fmt.Errorf("nil sql executor")
	}
	if err := input.Validate(service.DefaultUserAvatarMaxBytes); err != nil {
		return nil, err
	}

	row := userAvatarRow{}
	if err := scanSingleRow(ctx, r.sql, `
INSERT INTO user_avatars (
	user_id,
	storage_provider,
	storage_key,
	url,
	content_type,
	byte_size,
	sha256,
	created_at,
	updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (user_id) DO UPDATE
SET storage_provider = EXCLUDED.storage_provider,
	storage_key = EXCLUDED.storage_key,
	url = EXCLUDED.url,
	content_type = EXCLUDED.content_type,
	byte_size = EXCLUDED.byte_size,
	sha256 = EXCLUDED.sha256,
	updated_at = CURRENT_TIMESTAMP
RETURNING
	id,
	user_id,
	storage_provider,
	storage_key,
	url,
	content_type,
	byte_size,
	sha256,
	created_at,
	updated_at`,
		[]any{
			userID,
			strings.TrimSpace(input.StorageProvider),
			strings.TrimSpace(input.StorageKey),
			strings.TrimSpace(input.URL),
			strings.TrimSpace(input.ContentType),
			input.ByteSize,
			strings.TrimSpace(input.SHA256),
		},
		&row.ID,
		&row.UserID,
		&row.StorageProvider,
		&row.StorageKey,
		&row.URL,
		&row.ContentType,
		&row.ByteSize,
		&row.SHA256,
		&row.CreatedAt,
		&row.UpdatedAt,
	); err != nil {
		return nil, err
	}

	return &service.UserAvatar{
		ID:              row.ID,
		UserID:          row.UserID,
		StorageProvider: row.StorageProvider,
		StorageKey:      row.StorageKey,
		URL:             row.URL,
		ContentType:     row.ContentType,
		ByteSize:        row.ByteSize,
		SHA256:          row.SHA256,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}, nil
}

func (r *userRepository) FindAuthIdentity(ctx context.Context, providerType, providerKey, providerSubject string) (*AuthIdentityRecord, error) {
	if r == nil || r.sql == nil {
		return nil, fmt.Errorf("nil sql executor")
	}

	row := authIdentityRow{}
	if err := scanSingleRow(ctx, r.sql, `
SELECT
	id,
	user_id,
	provider_type,
	provider_key,
	provider_subject,
	verified_at,
	issuer,
	metadata,
	created_at,
	updated_at
FROM auth_identities
WHERE provider_type = $1 AND provider_key = $2 AND provider_subject = $3`,
		[]any{strings.TrimSpace(providerType), strings.TrimSpace(providerKey), strings.TrimSpace(providerSubject)},
		&row.ID,
		&row.UserID,
		&row.ProviderType,
		&row.ProviderKey,
		&row.ProviderSubject,
		&row.VerifiedAt,
		&row.Issuer,
		&row.Metadata,
		&row.CreatedAt,
		&row.UpdatedAt,
	); err != nil {
		return nil, err
	}

	return row.toRecord()
}

func (r *userRepository) GetAuthIdentityByID(ctx context.Context, id int64) (*AuthIdentityRecord, error) {
	if r == nil || r.sql == nil {
		return nil, fmt.Errorf("nil sql executor")
	}

	row := authIdentityRow{}
	if err := scanSingleRow(ctx, r.sql, `
SELECT
	id,
	user_id,
	provider_type,
	provider_key,
	provider_subject,
	verified_at,
	issuer,
	metadata,
	created_at,
	updated_at
FROM auth_identities
WHERE id = $1`,
		[]any{id},
		&row.ID,
		&row.UserID,
		&row.ProviderType,
		&row.ProviderKey,
		&row.ProviderSubject,
		&row.VerifiedAt,
		&row.Issuer,
		&row.Metadata,
		&row.CreatedAt,
		&row.UpdatedAt,
	); err != nil {
		return nil, err
	}

	return row.toRecord()
}

func (r *userRepository) ListUserExternalIdentities(ctx context.Context, userID int64) ([]service.UserExternalIdentity, error) {
	if r == nil || r.sql == nil {
		return nil, fmt.Errorf("nil sql executor")
	}

	rows, err := r.sql.QueryContext(ctx, `
SELECT provider_type, provider_subject
FROM auth_identities
WHERE user_id = $1
ORDER BY provider_type ASC, created_at ASC, id ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	identities := make([]service.UserExternalIdentity, 0)
	for rows.Next() {
		var providerType string
		var providerSubject string
		if err := rows.Scan(&providerType, &providerSubject); err != nil {
			return nil, err
		}
		identities = append(identities, service.UserExternalIdentity{
			Provider:       service.NormalizeExternalIdentityProvider(service.ExternalIdentityProvider(providerType)),
			ProviderUserID: strings.TrimSpace(providerSubject),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return identities, nil
}

func (r *userRepository) DeleteUserExternalIdentity(ctx context.Context, userID int64, provider service.ExternalIdentityProvider) (bool, error) {
	if r == nil || r.sql == nil {
		return false, fmt.Errorf("nil sql executor")
	}

	rows, err := r.sql.QueryContext(ctx, `
SELECT id
FROM auth_identities
WHERE user_id = $1 AND provider_type = $2
ORDER BY id ASC`, userID, string(service.NormalizeExternalIdentityProvider(provider)))
	if err != nil {
		return false, err
	}
	defer func() { _ = rows.Close() }()

	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return false, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	if len(ids) == 0 {
		return false, nil
	}

	for _, id := range ids {
		if err := r.deleteAuthIdentityByID(ctx, id); err != nil {
			return false, err
		}
	}
	return true, nil
}

func (r *userRepository) GetIdentityAdoptionDecision(
	ctx context.Context,
	userID int64,
	ref service.IdentityAdoptionDecisionRef,
) (*service.IdentityAdoptionDecision, error) {
	identity, err := r.FindAuthIdentity(ctx, strings.TrimSpace(ref.ProviderType), normalizedProviderKey(ref), strings.TrimSpace(ref.ProviderSubject))
	if err != nil {
		return nil, err
	}
	if identity == nil || identity.UserID != userID {
		return nil, service.ErrUserNotFound
	}

	row := identityAdoptionDecisionRow{}
	if err := scanSingleRow(ctx, r.sql, `
SELECT
	identity_id,
	adopt_display_name,
	adopt_avatar,
	decided_at,
	created_at,
	updated_at
FROM identity_adoption_decisions
WHERE identity_id = $1`,
		[]any{identity.ID},
		&row.IdentityID,
		&row.AdoptDisplayName,
		&row.AdoptAvatar,
		&row.DecidedAt,
		&row.CreatedAt,
		&row.UpdatedAt,
	); err != nil {
		return nil, err
	}

	return &service.IdentityAdoptionDecision{
		Ref: service.IdentityAdoptionDecisionRef{
			ProviderType:    identity.ProviderType,
			ProviderKey:     identity.ProviderKey,
			ProviderSubject: identity.ProviderSubject,
		},
		AdoptDisplayName: row.AdoptDisplayName,
		AdoptAvatar:      row.AdoptAvatar,
		DecidedAt:        row.DecidedAt,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
	}, nil
}

func (r *userRepository) UpsertIdentityAdoptionDecision(
	ctx context.Context,
	userID int64,
	input service.UpsertIdentityAdoptionDecisionInput,
) (*service.IdentityAdoptionDecision, error) {
	providerType := strings.TrimSpace(input.Ref.ProviderType)
	providerSubject := strings.TrimSpace(input.Ref.ProviderSubject)
	if providerType == "" || providerSubject == "" {
		return nil, fmt.Errorf("provider type and provider subject are required")
	}

	identity, err := r.FindAuthIdentity(ctx, providerType, normalizedProviderKey(input.Ref), providerSubject)
	if err != nil {
		return nil, err
	}
	if identity == nil || identity.UserID != userID {
		return nil, service.ErrUserNotFound
	}

	if _, err := r.sql.ExecContext(ctx, `
INSERT INTO identity_adoption_decisions (
	identity_id,
	adopt_display_name,
	adopt_avatar,
	decided_at,
	created_at,
	updated_at
) VALUES (
	$1,
	COALESCE($2, FALSE),
	COALESCE($3, FALSE),
	CURRENT_TIMESTAMP,
	CURRENT_TIMESTAMP,
	CURRENT_TIMESTAMP
)
ON CONFLICT (identity_id) DO UPDATE
SET adopt_display_name = COALESCE($2, identity_adoption_decisions.adopt_display_name),
	adopt_avatar = COALESCE($3, identity_adoption_decisions.adopt_avatar),
	decided_at = CURRENT_TIMESTAMP,
	updated_at = CURRENT_TIMESTAMP`,
		identity.ID,
		nullableBool(input.AdoptDisplayName),
		nullableBool(input.AdoptAvatar),
	); err != nil {
		return nil, err
	}

	return r.GetIdentityAdoptionDecision(ctx, userID, input.Ref)
}

func (r *userRepository) UpsertAuthIdentityChannel(ctx context.Context, identityID int64, input UpsertAuthIdentityChannelInput) (*AuthIdentityChannelRecord, error) {
	if r == nil || r.sql == nil {
		return nil, fmt.Errorf("nil sql executor")
	}

	providerType := strings.TrimSpace(input.ProviderType)
	providerKey := strings.TrimSpace(input.ProviderKey)
	channel := strings.TrimSpace(input.Channel)
	channelAppID := strings.TrimSpace(input.ChannelAppID)
	channelSubject := strings.TrimSpace(input.ChannelSubject)
	if identityID == 0 || providerType == "" || providerKey == "" || channel == "" || channelAppID == "" || channelSubject == "" {
		return nil, fmt.Errorf("identity id, provider type, provider key, channel, channel app id, and channel subject are required")
	}

	existing, err := r.FindAuthIdentityChannel(ctx, providerType, providerKey, channel, channelAppID, channelSubject)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if existing != nil {
		if existing.IdentityID != identityID {
			return nil, errAuthIdentityChannelOwnershipConflict
		}
		metadataJSON, err := marshalAuthIdentityMetadata(input.Metadata)
		if err != nil {
			return nil, err
		}
		if _, err := r.sql.ExecContext(ctx, `
UPDATE auth_identity_channels
SET metadata = $2, updated_at = CURRENT_TIMESTAMP
WHERE id = $1`,
			existing.ID,
			metadataJSON,
		); err != nil {
			return nil, err
		}
		return r.GetAuthIdentityChannelByID(ctx, existing.ID)
	}

	metadataJSON, err := marshalAuthIdentityMetadata(input.Metadata)
	if err != nil {
		return nil, err
	}

	if _, err := r.sql.ExecContext(ctx, `
INSERT INTO auth_identity_channels (
	identity_id,
	provider_type,
	provider_key,
	channel,
	channel_app_id,
	channel_subject,
	metadata,
	created_at,
	updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		identityID,
		providerType,
		providerKey,
		channel,
		channelAppID,
		channelSubject,
		metadataJSON,
	); err != nil {
		return nil, err
	}

	return r.FindAuthIdentityChannel(ctx, providerType, providerKey, channel, channelAppID, channelSubject)
}

func (r *userRepository) FindAuthIdentityChannel(ctx context.Context, providerType, providerKey, channel, channelAppID, channelSubject string) (*AuthIdentityChannelRecord, error) {
	row := authIdentityChannelRow{}
	if err := scanSingleRow(ctx, r.sql, `
SELECT
	id,
	identity_id,
	provider_type,
	provider_key,
	channel,
	channel_app_id,
	channel_subject,
	metadata,
	created_at,
	updated_at
FROM auth_identity_channels
WHERE provider_type = $1
  AND provider_key = $2
  AND channel = $3
  AND channel_app_id = $4
  AND channel_subject = $5`,
		[]any{
			strings.TrimSpace(providerType),
			strings.TrimSpace(providerKey),
			strings.TrimSpace(channel),
			strings.TrimSpace(channelAppID),
			strings.TrimSpace(channelSubject),
		},
		&row.ID,
		&row.IdentityID,
		&row.ProviderType,
		&row.ProviderKey,
		&row.Channel,
		&row.ChannelAppID,
		&row.ChannelSubject,
		&row.Metadata,
		&row.CreatedAt,
		&row.UpdatedAt,
	); err != nil {
		return nil, err
	}

	return row.toRecord()
}

func (r *userRepository) GetAuthIdentityChannelByID(ctx context.Context, id int64) (*AuthIdentityChannelRecord, error) {
	row := authIdentityChannelRow{}
	if err := scanSingleRow(ctx, r.sql, `
SELECT
	id,
	identity_id,
	provider_type,
	provider_key,
	channel,
	channel_app_id,
	channel_subject,
	metadata,
	created_at,
	updated_at
FROM auth_identity_channels
WHERE id = $1`,
		[]any{id},
		&row.ID,
		&row.IdentityID,
		&row.ProviderType,
		&row.ProviderKey,
		&row.Channel,
		&row.ChannelAppID,
		&row.ChannelSubject,
		&row.Metadata,
		&row.CreatedAt,
		&row.UpdatedAt,
	); err != nil {
		return nil, err
	}

	return row.toRecord()
}

func (r *userRepository) CreatePendingAuthSession(ctx context.Context, input CreatePendingAuthSessionInput) (*PendingAuthSessionRecord, error) {
	if r == nil || r.sql == nil {
		return nil, fmt.Errorf("nil sql executor")
	}

	sessionToken := strings.TrimSpace(input.SessionToken)
	intent := strings.TrimSpace(input.Intent)
	providerType := strings.TrimSpace(input.ProviderType)
	providerKey := strings.TrimSpace(input.ProviderKey)
	providerSubject := strings.TrimSpace(input.ProviderSubject)
	if sessionToken == "" || intent == "" || providerType == "" || providerKey == "" || providerSubject == "" || input.ExpiresAt.IsZero() {
		return nil, fmt.Errorf("session token, intent, provider type, provider key, provider subject, and expires at are required")
	}

	payloadJSON, err := marshalAuthIdentityMetadata(input.UpstreamIdentity)
	if err != nil {
		return nil, err
	}
	metadataJSON, err := marshalAuthIdentityMetadata(input.Metadata)
	if err != nil {
		return nil, err
	}

	if _, err := r.sql.ExecContext(ctx, `
INSERT INTO pending_auth_sessions (
	session_token,
	intent,
	provider_type,
	provider_key,
	provider_subject,
	target_user_id,
	redirect_to,
	metadata,
	resolved_email,
	pending_password_hash,
	upstream_identity_payload,
	email_verified_at,
	password_verified_at,
	totp_verified_at,
	expires_at,
	consumed_at,
	created_at,
	updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, NULL, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT(session_token) DO UPDATE SET
	intent = excluded.intent,
	provider_type = excluded.provider_type,
	provider_key = excluded.provider_key,
	provider_subject = excluded.provider_subject,
	target_user_id = excluded.target_user_id,
	redirect_to = excluded.redirect_to,
	metadata = excluded.metadata,
	resolved_email = excluded.resolved_email,
	pending_password_hash = excluded.pending_password_hash,
	upstream_identity_payload = excluded.upstream_identity_payload,
	email_verified_at = excluded.email_verified_at,
	password_verified_at = excluded.password_verified_at,
	totp_verified_at = excluded.totp_verified_at,
	expires_at = excluded.expires_at,
	updated_at = CURRENT_TIMESTAMP`,
		sessionToken,
		intent,
		providerType,
		providerKey,
		providerSubject,
		nullableInt64(input.TargetUserID),
		strings.TrimSpace(input.RedirectTo),
		metadataJSON,
		strings.TrimSpace(input.ResolvedEmail),
		strings.TrimSpace(input.PendingPasswordHash),
		payloadJSON,
		nullableTime(input.EmailVerifiedAt),
		nullableTime(input.PasswordVerifiedAt),
		nullableTime(input.TOTPVerifiedAt),
		input.ExpiresAt.UTC(),
	); err != nil {
		return nil, err
	}

	return r.GetPendingAuthSessionByToken(ctx, sessionToken)
}

func (r *userRepository) GetPendingAuthSessionByToken(ctx context.Context, sessionToken string) (*PendingAuthSessionRecord, error) {
	if r == nil || r.sql == nil {
		return nil, fmt.Errorf("nil sql executor")
	}

	row := pendingAuthSessionRow{}
	if err := scanSingleRow(ctx, r.sql, `
	SELECT
	id,
	session_token,
	intent,
	provider_type,
	provider_key,
	provider_subject,
	target_user_id,
	redirect_to,
	metadata,
	resolved_email,
	pending_password_hash,
	upstream_identity_payload,
	email_verified_at,
	password_verified_at,
	totp_verified_at,
	expires_at,
	consumed_at,
	created_at,
	updated_at
FROM pending_auth_sessions
WHERE session_token = $1`,
		[]any{strings.TrimSpace(sessionToken)},
		&row.ID,
		&row.SessionToken,
		&row.Intent,
		&row.ProviderType,
		&row.ProviderKey,
		&row.ProviderSubject,
		&row.TargetUserID,
		&row.RedirectTo,
		&row.Metadata,
		&row.ResolvedEmail,
		&row.PendingPasswordHash,
		&row.UpstreamIdentity,
		&row.EmailVerifiedAt,
		&row.PasswordVerifiedAt,
		&row.TOTPVerifiedAt,
		&row.ExpiresAt,
		&row.ConsumedAt,
		&row.CreatedAt,
		&row.UpdatedAt,
	); err != nil {
		return nil, err
	}

	return row.toRecord()
}

func (r *userRepository) UpdatePendingAuthSessionByToken(ctx context.Context, session *PendingAuthSessionRecord) error {
	if r == nil || r.sql == nil {
		return fmt.Errorf("nil sql executor")
	}
	if session == nil {
		return fmt.Errorf("pending auth session is required")
	}

	upstreamIdentityJSON, err := marshalAuthIdentityMetadata(session.UpstreamIdentity)
	if err != nil {
		return err
	}
	metadataJSON, err := marshalAuthIdentityMetadata(session.Metadata)
	if err != nil {
		return err
	}

	_, err = r.sql.ExecContext(ctx, `
UPDATE pending_auth_sessions
SET intent = $2,
	provider_type = $3,
	provider_key = $4,
	provider_subject = $5,
	target_user_id = $6,
	redirect_to = $7,
	metadata = $8,
	resolved_email = $9,
	pending_password_hash = $10,
	upstream_identity_payload = $11,
	email_verified_at = $12,
	password_verified_at = $13,
	totp_verified_at = $14,
	expires_at = $15,
	consumed_at = $16,
	updated_at = CURRENT_TIMESTAMP
WHERE session_token = $1`,
		strings.TrimSpace(session.SessionToken),
		strings.TrimSpace(session.Intent),
		strings.TrimSpace(session.ProviderType),
		strings.TrimSpace(session.ProviderKey),
		strings.TrimSpace(session.ProviderSubject),
		nullableInt64(session.TargetUserID),
		strings.TrimSpace(session.RedirectTo),
		metadataJSON,
		strings.TrimSpace(session.ResolvedEmail),
		strings.TrimSpace(session.PendingPasswordHash),
		upstreamIdentityJSON,
		nullableTime(session.EmailVerifiedAt),
		nullableTime(session.PasswordVerifiedAt),
		nullableTime(session.TOTPVerifiedAt),
		session.ExpiresAt.UTC(),
		nullableTime(session.ConsumedAt),
	)
	return err
}

func (r *userRepository) ConsumePendingAuthSession(ctx context.Context, sessionToken string) (*PendingAuthSessionRecord, error) {
	if r == nil || r.sql == nil {
		return nil, fmt.Errorf("nil sql executor")
	}

	if _, err := r.sql.ExecContext(ctx, `
UPDATE pending_auth_sessions
SET consumed_at = COALESCE(consumed_at, CURRENT_TIMESTAMP), updated_at = CURRENT_TIMESTAMP
WHERE session_token = $1`,
		strings.TrimSpace(sessionToken),
	); err != nil {
		return nil, err
	}

	return r.GetPendingAuthSessionByToken(ctx, sessionToken)
}

func (r *userRepository) BindPendingAuthIdentity(ctx context.Context, session *service.PendingAuthSessionRecord, userID int64) error {
	if r == nil || r.sql == nil {
		return fmt.Errorf("nil sql executor")
	}
	if session == nil {
		return fmt.Errorf("pending auth session is required")
	}

	providerType := strings.TrimSpace(session.ProviderType)
	providerKey := strings.TrimSpace(session.ProviderKey)
	if providerType == "" {
		return fmt.Errorf("provider type is required")
	}
	if providerKey == "" {
		providerKey = providerType
	}

	metadata := mergeAuthIdentityMetadata(nil, session.Metadata)
	verifiedAt := time.Now().UTC()
	if providerType == "wechat" {
		return r.bindPendingWeChatIdentity(ctx, userID, providerKey, strings.TrimSpace(session.ProviderSubject), metadata, &verifiedAt)
	}

	providerSubject := strings.TrimSpace(session.ProviderSubject)
	if providerSubject == "" {
		return fmt.Errorf("provider subject is required")
	}

	existing, err := r.findOptionalAuthIdentity(ctx, providerType, providerKey, providerSubject)
	if err != nil {
		return err
	}
	if existing != nil && existing.UserID != userID {
		return errAuthIdentityOwnershipConflict
	}

	mergedMetadata := metadata
	if existing != nil {
		mergedMetadata = mergeAuthIdentityMetadata(existing.Metadata, metadata)
	}
	_, err = r.UpsertAuthIdentity(ctx, userID, UpsertAuthIdentityInput{
		ProviderType:    providerType,
		ProviderKey:     providerKey,
		ProviderSubject: providerSubject,
		VerifiedAt:      &verifiedAt,
		Metadata:        mergedMetadata,
	})
	return err
}

func (r *userRepository) bindPendingWeChatIdentity(
	ctx context.Context,
	userID int64,
	providerKey string,
	fallbackSubject string,
	metadata map[string]any,
	verifiedAt *time.Time,
) error {
	unionid := authIdentityMetadataString(metadata, "unionid")
	openid := authIdentityMetadataString(metadata, "openid")
	channel := authIdentityMetadataString(metadata, "channel")
	appid := authIdentityMetadataString(metadata, "appid")

	primarySubject := firstNonEmptyAuthIdentityValue(unionid, fallbackSubject, openid)
	if primarySubject == "" {
		return fmt.Errorf("provider subject is required")
	}

	primaryIdentity, err := r.findOptionalAuthIdentity(ctx, "wechat", providerKey, primarySubject)
	if err != nil {
		return err
	}
	if primaryIdentity != nil && primaryIdentity.UserID != userID {
		return errAuthIdentityOwnershipConflict
	}

	channelIdentity, err := r.findWeChatIdentityByChannel(ctx, providerKey, channel, appid, openid)
	if err != nil {
		return err
	}
	if channelIdentity != nil && channelIdentity.UserID != userID {
		return errAuthIdentityOwnershipConflict
	}

	targetIdentity := primaryIdentity
	switch {
	case targetIdentity != nil && channelIdentity != nil && channelIdentity.ID != targetIdentity.ID:
		if err := r.reassignAuthIdentityChannels(ctx, channelIdentity.ID, targetIdentity.ID); err != nil {
			return err
		}
		if err := r.deleteAuthIdentityByID(ctx, channelIdentity.ID); err != nil {
			return err
		}
	case targetIdentity == nil && channelIdentity != nil:
		targetIdentity = channelIdentity
	case targetIdentity == nil:
		targetIdentity, err = r.UpsertAuthIdentity(ctx, userID, UpsertAuthIdentityInput{
			ProviderType:    "wechat",
			ProviderKey:     providerKey,
			ProviderSubject: primarySubject,
			VerifiedAt:      verifiedAt,
			Metadata:        metadata,
		})
		if err != nil {
			return err
		}
	}

	if targetIdentity == nil {
		return fmt.Errorf("wechat identity resolution failed")
	}

	mergedMetadata := mergeAuthIdentityMetadata(targetIdentity.Metadata, metadata)
	if targetIdentity.ProviderSubject != primarySubject {
		if err := r.rekeyAuthIdentity(ctx, targetIdentity.ID, UpsertAuthIdentityInput{
			ProviderSubject: primarySubject,
			VerifiedAt:      verifiedAt,
			Metadata:        mergedMetadata,
		}); err != nil {
			return err
		}
	} else if err := r.updateAuthIdentity(ctx, targetIdentity.ID, UpsertAuthIdentityInput{
		VerifiedAt: verifiedAt,
		Metadata:   mergedMetadata,
	}); err != nil {
		return err
	}

	if openid != "" && channel != "" && appid != "" {
		if _, err := r.UpsertAuthIdentityChannel(ctx, targetIdentity.ID, UpsertAuthIdentityChannelInput{
			ProviderType:   "wechat",
			ProviderKey:    providerKey,
			Channel:        channel,
			ChannelAppID:   appid,
			ChannelSubject: openid,
			Metadata:       mergedMetadata,
		}); err != nil {
			return err
		}
	}

	return nil
}

func (r *userRepository) updateAuthIdentity(ctx context.Context, id int64, input UpsertAuthIdentityInput) error {
	metadataJSON, err := marshalAuthIdentityMetadata(input.Metadata)
	if err != nil {
		return err
	}

	_, err = r.sql.ExecContext(ctx, `
UPDATE auth_identities
SET verified_at = $2,
	issuer = $3,
	metadata = $4,
	updated_at = CURRENT_TIMESTAMP
WHERE id = $1`,
		id,
		nullableTime(input.VerifiedAt),
		nullableString(input.Issuer),
		metadataJSON,
	)
	return err
}

func (r *userRepository) rekeyAuthIdentity(ctx context.Context, id int64, input UpsertAuthIdentityInput) error {
	metadataJSON, err := marshalAuthIdentityMetadata(input.Metadata)
	if err != nil {
		return err
	}

	_, err = r.sql.ExecContext(ctx, `
UPDATE auth_identities
SET provider_subject = $2,
	verified_at = $3,
	issuer = $4,
	metadata = $5,
	updated_at = CURRENT_TIMESTAMP
WHERE id = $1`,
		id,
		strings.TrimSpace(input.ProviderSubject),
		nullableTime(input.VerifiedAt),
		nullableString(input.Issuer),
		metadataJSON,
	)
	return err
}

func marshalAuthIdentityMetadata(metadata map[string]any) ([]byte, error) {
	if metadata == nil {
		metadata = map[string]any{}
	}
	return json.Marshal(metadata)
}

func authIdentityMetadataString(metadata map[string]any, key string) string {
	if len(metadata) == 0 {
		return ""
	}
	raw, ok := metadata[key]
	if !ok || raw == nil {
		return ""
	}
	switch value := raw.(type) {
	case string:
		return strings.TrimSpace(value)
	default:
		return strings.TrimSpace(fmt.Sprint(value))
	}
}

func firstNonEmptyAuthIdentityValue(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func (r *userRepository) findOptionalAuthIdentity(ctx context.Context, providerType, providerKey, providerSubject string) (*AuthIdentityRecord, error) {
	record, err := r.FindAuthIdentity(ctx, providerType, providerKey, providerSubject)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return record, nil
}

func (r *userRepository) findOptionalAuthIdentityChannel(
	ctx context.Context,
	providerType string,
	providerKey string,
	channel string,
	channelAppID string,
	channelSubject string,
) (*AuthIdentityChannelRecord, error) {
	record, err := r.FindAuthIdentityChannel(ctx, providerType, providerKey, channel, channelAppID, channelSubject)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return record, nil
}

func (r *userRepository) findWeChatIdentityByChannel(
	ctx context.Context,
	providerKey string,
	channel string,
	channelAppID string,
	channelSubject string,
) (*AuthIdentityRecord, error) {
	if channel == "" || channelAppID == "" || channelSubject == "" {
		return nil, nil
	}

	record, err := r.findOptionalAuthIdentityChannel(ctx, "wechat", providerKey, channel, channelAppID, channelSubject)
	if err != nil || record == nil {
		return nil, err
	}

	identity, err := r.GetAuthIdentityByID(ctx, record.IdentityID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return identity, nil
}

func (r *userRepository) reassignAuthIdentityChannels(ctx context.Context, sourceIdentityID, targetIdentityID int64) error {
	if sourceIdentityID == 0 || targetIdentityID == 0 || sourceIdentityID == targetIdentityID {
		return nil
	}

	if _, err := r.sql.ExecContext(ctx, `
DELETE FROM auth_identity_channels
WHERE identity_id = $1
  AND id IN (
	SELECT src.id
	FROM auth_identity_channels src
	JOIN auth_identity_channels dst
	  ON dst.identity_id = $2
	 AND dst.provider_type = src.provider_type
	 AND dst.provider_key = src.provider_key
	 AND dst.channel = src.channel
	 AND dst.channel_app_id = src.channel_app_id
	 AND dst.channel_subject = src.channel_subject
	WHERE src.identity_id = $1
)`,
		sourceIdentityID,
		targetIdentityID,
	); err != nil {
		return err
	}

	_, err := r.sql.ExecContext(ctx, `
UPDATE auth_identity_channels
SET identity_id = $2,
	updated_at = CURRENT_TIMESTAMP
WHERE identity_id = $1`,
		sourceIdentityID,
		targetIdentityID,
	)
	return err
}

func (r *userRepository) deleteAuthIdentityByID(ctx context.Context, id int64) error {
	if id == 0 {
		return nil
	}
	_, err := r.sql.ExecContext(ctx, `DELETE FROM auth_identities WHERE id = $1`, id)
	return err
}

func nullableString(v *string) any {
	if v == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*v)
	if trimmed == "" {
		return nil
	}
	return trimmed
}

func nullableInt64(v *int64) any {
	if v == nil {
		return nil
	}
	return *v
}

func nullableTime(v *time.Time) any {
	if v == nil || v.IsZero() {
		return nil
	}
	return v.UTC()
}

type authIdentityRow struct {
	ID              int64
	UserID          int64
	ProviderType    string
	ProviderKey     string
	ProviderSubject string
	VerifiedAt      sql.NullTime
	Issuer          sql.NullString
	Metadata        []byte
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (r authIdentityRow) toRecord() (*AuthIdentityRecord, error) {
	metadata, err := unmarshalAuthIdentityMetadata(r.Metadata)
	if err != nil {
		return nil, err
	}
	return &AuthIdentityRecord{
		ID:              r.ID,
		UserID:          r.UserID,
		ProviderType:    r.ProviderType,
		ProviderKey:     r.ProviderKey,
		ProviderSubject: r.ProviderSubject,
		VerifiedAt:      nullTimePtr(r.VerifiedAt),
		Issuer:          nullStringPtr(r.Issuer),
		Metadata:        metadata,
		CreatedAt:       r.CreatedAt,
		UpdatedAt:       r.UpdatedAt,
	}, nil
}

type authIdentityChannelRow struct {
	ID             int64
	IdentityID     int64
	ProviderType   string
	ProviderKey    string
	Channel        string
	ChannelAppID   string
	ChannelSubject string
	Metadata       []byte
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (r authIdentityChannelRow) toRecord() (*AuthIdentityChannelRecord, error) {
	metadata, err := unmarshalAuthIdentityMetadata(r.Metadata)
	if err != nil {
		return nil, err
	}
	return &AuthIdentityChannelRecord{
		ID:             r.ID,
		IdentityID:     r.IdentityID,
		ProviderType:   r.ProviderType,
		ProviderKey:    r.ProviderKey,
		Channel:        r.Channel,
		ChannelAppID:   r.ChannelAppID,
		ChannelSubject: r.ChannelSubject,
		Metadata:       metadata,
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
	}, nil
}

type pendingAuthSessionRow struct {
	ID                  int64
	SessionToken        string
	Intent              string
	ProviderType        string
	ProviderKey         string
	ProviderSubject     string
	TargetUserID        sql.NullInt64
	RedirectTo          string
	Metadata            []byte
	ResolvedEmail       string
	PendingPasswordHash string
	UpstreamIdentity    []byte
	EmailVerifiedAt     sql.NullTime
	PasswordVerifiedAt  sql.NullTime
	TOTPVerifiedAt      sql.NullTime
	ExpiresAt           time.Time
	ConsumedAt          sql.NullTime
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

func (r pendingAuthSessionRow) toRecord() (*PendingAuthSessionRecord, error) {
	metadata, err := unmarshalAuthIdentityMetadata(r.Metadata)
	if err != nil {
		return nil, err
	}
	payload, err := unmarshalAuthIdentityMetadata(r.UpstreamIdentity)
	if err != nil {
		return nil, err
	}
	return &PendingAuthSessionRecord{
		ID:                  r.ID,
		SessionToken:        r.SessionToken,
		Intent:              r.Intent,
		ProviderType:        r.ProviderType,
		ProviderKey:         r.ProviderKey,
		ProviderSubject:     r.ProviderSubject,
		TargetUserID:        nullInt64Ptr(r.TargetUserID),
		RedirectTo:          r.RedirectTo,
		Metadata:            metadata,
		ResolvedEmail:       r.ResolvedEmail,
		PendingPasswordHash: r.PendingPasswordHash,
		UpstreamIdentity:    payload,
		EmailVerifiedAt:     nullTimePtr(r.EmailVerifiedAt),
		PasswordVerifiedAt:  nullTimePtr(r.PasswordVerifiedAt),
		TOTPVerifiedAt:      nullTimePtr(r.TOTPVerifiedAt),
		ExpiresAt:           r.ExpiresAt,
		ConsumedAt:          nullTimePtr(r.ConsumedAt),
		CreatedAt:           r.CreatedAt,
		UpdatedAt:           r.UpdatedAt,
	}, nil
}

func normalizedProviderKey(ref service.IdentityAdoptionDecisionRef) string {
	providerKey := strings.TrimSpace(ref.ProviderKey)
	if providerKey != "" {
		return providerKey
	}
	return strings.TrimSpace(ref.ProviderType)
}

func nullableBool(v *bool) any {
	if v == nil {
		return nil
	}
	return *v
}

func mergeAuthIdentityMetadata(base, extra map[string]any) map[string]any {
	merged := cloneStringAnyMap(base)
	if merged == nil {
		merged = make(map[string]any, len(extra))
	}
	for key, value := range extra {
		merged[key] = value
	}
	return merged
}

func cloneStringAnyMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func unmarshalAuthIdentityMetadata(raw []byte) (map[string]any, error) {
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if out == nil {
		return map[string]any{}, nil
	}
	return out, nil
}

func nullStringPtr(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	value := v.String
	return &value
}

func nullInt64Ptr(v sql.NullInt64) *int64 {
	if !v.Valid {
		return nil
	}
	value := v.Int64
	return &value
}

func nullTimePtr(v sql.NullTime) *time.Time {
	if !v.Valid {
		return nil
	}
	value := v.Time
	return &value
}
