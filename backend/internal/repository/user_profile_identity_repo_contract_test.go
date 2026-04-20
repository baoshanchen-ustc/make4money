package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"

	_ "modernc.org/sqlite"
)

func newUserProfileIdentityTestRepo(t *testing.T) *userRepository {
	t.Helper()

	name := strings.ReplaceAll(t.Name(), "/", "_")
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared&_fk=1", name)

	db, err := sql.Open("sqlite", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec(`
PRAGMA foreign_keys = ON;
CREATE TABLE users (
	id INTEGER PRIMARY KEY
);
CREATE TABLE auth_identities (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id INTEGER NOT NULL,
	provider_type TEXT NOT NULL,
	provider_key TEXT NOT NULL,
	provider_subject TEXT NOT NULL,
	verified_at DATETIME NULL,
	issuer TEXT NULL,
	metadata TEXT NOT NULL DEFAULT '{}',
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(provider_type, provider_key, provider_subject)
);
CREATE TABLE auth_identity_channels (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	identity_id INTEGER NOT NULL,
	provider_type TEXT NOT NULL,
	provider_key TEXT NOT NULL,
	channel TEXT NOT NULL,
	channel_app_id TEXT NOT NULL,
	channel_subject TEXT NOT NULL,
	metadata TEXT NOT NULL DEFAULT '{}',
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(provider_type, provider_key, channel, channel_app_id, channel_subject),
	FOREIGN KEY(identity_id) REFERENCES auth_identities(id) ON DELETE CASCADE
);
CREATE TABLE identity_adoption_decisions (
	identity_id INTEGER PRIMARY KEY,
	adopt_display_name BOOLEAN NOT NULL DEFAULT FALSE,
	adopt_avatar BOOLEAN NOT NULL DEFAULT FALSE,
	decided_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(identity_id) REFERENCES auth_identities(id) ON DELETE CASCADE
);
CREATE TABLE pending_auth_sessions (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	session_token TEXT NOT NULL UNIQUE,
	intent TEXT NOT NULL,
	provider_type TEXT NOT NULL,
	provider_key TEXT NOT NULL,
	provider_subject TEXT NOT NULL,
	target_user_id INTEGER NULL,
	redirect_to TEXT NOT NULL DEFAULT '',
	metadata TEXT NOT NULL DEFAULT '{}',
	resolved_email TEXT NOT NULL DEFAULT '',
	pending_password_hash TEXT NOT NULL DEFAULT '',
	upstream_identity_payload TEXT NOT NULL DEFAULT '{}',
	email_verified_at DATETIME NULL,
	password_verified_at DATETIME NULL,
	totp_verified_at DATETIME NULL,
	expires_at DATETIME NOT NULL,
	consumed_at DATETIME NULL,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE auth_identity_migration_reports (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	email_identity_backfilled_count INTEGER NOT NULL DEFAULT 0,
	email_identity_mismatch_count INTEGER NOT NULL DEFAULT 0,
	wechat_unionid_migrated_count INTEGER NOT NULL DEFAULT 0,
	wechat_legacy_repairable_count INTEGER NOT NULL DEFAULT 0,
	wechat_manual_repair_required_count INTEGER NOT NULL DEFAULT 0,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE user_avatars (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id INTEGER NOT NULL UNIQUE,
	storage_provider TEXT NOT NULL,
	storage_key TEXT NOT NULL,
	url TEXT NOT NULL DEFAULT '',
	content_type TEXT NOT NULL,
	byte_size INTEGER NOT NULL,
	sha256 TEXT NOT NULL DEFAULT '',
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE user_external_identities (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id INTEGER NOT NULL,
	provider TEXT NOT NULL,
	provider_user_id TEXT NOT NULL,
	provider_union_id TEXT NULL,
	provider_username TEXT NOT NULL DEFAULT '',
	display_name TEXT NOT NULL DEFAULT '',
	profile_url TEXT NOT NULL DEFAULT '',
	avatar_url TEXT NOT NULL DEFAULT '',
	metadata TEXT NOT NULL DEFAULT '{}',
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(provider, provider_user_id),
	UNIQUE(user_id, provider)
);
INSERT INTO users (id) VALUES
	(1), (2), (3), (4), (5),
	(6), (7), (8), (9), (10),
	(11), (12), (13), (14), (15),
	(16), (17), (18), (19), (20);
`)
	require.NoError(t, err)

	return &userRepository{sql: db}
}

func TestUpsertAuthIdentityChannel_AllowsSameOpenIDAcrossDifferentAppIDs(t *testing.T) {
	repo := newUserProfileIdentityTestRepo(t)
	ctx := context.Background()

	identity, err := repo.UpsertAuthIdentity(ctx, 7, UpsertAuthIdentityInput{
		ProviderType:    "wechat",
		ProviderKey:     "wechat-main",
		ProviderSubject: "union-1",
	})
	require.NoError(t, err)

	first, err := repo.UpsertAuthIdentityChannel(ctx, identity.ID, UpsertAuthIdentityChannelInput{
		ProviderType:   "wechat",
		ProviderKey:    "wechat-main",
		Channel:        "mp",
		ChannelAppID:   "wx-mp-a",
		ChannelSubject: "openid-same",
	})
	require.NoError(t, err)
	require.NotZero(t, first.ID)

	second, err := repo.UpsertAuthIdentityChannel(ctx, identity.ID, UpsertAuthIdentityChannelInput{
		ProviderType:   "wechat",
		ProviderKey:    "wechat-main",
		Channel:        "mp",
		ChannelAppID:   "wx-mp-b",
		ChannelSubject: "openid-same",
	})
	require.NoError(t, err)
	require.NotZero(t, second.ID)
	require.NotEqual(t, first.ID, second.ID)
}

func TestUpsertAuthIdentityChannel_RejectsSameOpenIDOnSameAppIDAcrossDifferentIdentities(t *testing.T) {
	repo := newUserProfileIdentityTestRepo(t)
	ctx := context.Background()

	firstIdentity, err := repo.UpsertAuthIdentity(ctx, 7, UpsertAuthIdentityInput{
		ProviderType:    "wechat",
		ProviderKey:     "wechat-main",
		ProviderSubject: "union-1",
	})
	require.NoError(t, err)

	secondIdentity, err := repo.UpsertAuthIdentity(ctx, 8, UpsertAuthIdentityInput{
		ProviderType:    "wechat",
		ProviderKey:     "wechat-main",
		ProviderSubject: "union-2",
	})
	require.NoError(t, err)
	require.NotEqual(t, firstIdentity.ID, secondIdentity.ID)

	_, err = repo.UpsertAuthIdentityChannel(ctx, firstIdentity.ID, UpsertAuthIdentityChannelInput{
		ProviderType:   "wechat",
		ProviderKey:    "wechat-main",
		Channel:        "mp",
		ChannelAppID:   "wx-mp-a",
		ChannelSubject: "openid-same",
	})
	require.NoError(t, err)

	_, err = repo.UpsertAuthIdentityChannel(ctx, secondIdentity.ID, UpsertAuthIdentityChannelInput{
		ProviderType:   "wechat",
		ProviderKey:    "wechat-main",
		Channel:        "mp",
		ChannelAppID:   "wx-mp-a",
		ChannelSubject: "openid-same",
	})
	require.ErrorIs(t, err, errAuthIdentityChannelOwnershipConflict)
}

func TestUpsertAuthIdentity_AllowsSameSubjectAcrossDifferentOIDCIssuers(t *testing.T) {
	repo := newUserProfileIdentityTestRepo(t)
	ctx := context.Background()

	first, err := repo.UpsertAuthIdentity(ctx, 7, UpsertAuthIdentityInput{
		ProviderType:    "oidc",
		ProviderKey:     "https://issuer-a.example.com",
		ProviderSubject: "sub-123",
		Issuer:          stringPtr("https://issuer-a.example.com"),
	})
	require.NoError(t, err)

	second, err := repo.UpsertAuthIdentity(ctx, 7, UpsertAuthIdentityInput{
		ProviderType:    "oidc",
		ProviderKey:     "https://issuer-b.example.com",
		ProviderSubject: "sub-123",
		Issuer:          stringPtr("https://issuer-b.example.com"),
	})
	require.NoError(t, err)
	require.NotEqual(t, first.ID, second.ID)
}

func TestPendingAuthSessionLifecycle_RoundTripsMetadataAndConsumeState(t *testing.T) {
	repo := newUserProfileIdentityTestRepo(t)
	ctx := context.Background()
	expiresAt := time.Now().UTC().Add(15 * time.Minute).Truncate(time.Second)

	created, err := repo.CreatePendingAuthSession(ctx, CreatePendingAuthSessionInput{
		SessionToken:     "pending-token-1",
		Intent:           "bind_current_user",
		ProviderType:     "wechat",
		ProviderKey:      "wechat-main",
		ProviderSubject:  "union-1",
		TargetUserID:     int64Ptr(42),
		UpstreamIdentity: map[string]any{"channel": "mp", "channel_app_id": "wx-mp-a"},
		ExpiresAt:        expiresAt,
	})
	require.NoError(t, err)
	require.Equal(t, "pending-token-1", created.SessionToken)

	loaded, err := repo.GetPendingAuthSessionByToken(ctx, "pending-token-1")
	require.NoError(t, err)
	require.Equal(t, created.ID, loaded.ID)
	require.NotNil(t, loaded.TargetUserID)
	require.Equal(t, int64(42), *loaded.TargetUserID)
	require.Equal(t, "mp", loaded.UpstreamIdentity["channel"])
	require.Equal(t, "wx-mp-a", loaded.UpstreamIdentity["channel_app_id"])
	require.Nil(t, loaded.ConsumedAt)

	loadedByID := mustGetPendingAuthSessionRowByID(t, repo, created.ID)
	require.Equal(t, created.ID, loadedByID.ID)
	require.Equal(t, "pending-token-1", loadedByID.SessionToken)
	require.NotNil(t, loadedByID.TargetUserID)
	require.Equal(t, int64(42), *loadedByID.TargetUserID)
	require.Equal(t, "mp", loadedByID.UpstreamIdentity["channel"])
	require.Equal(t, "wx-mp-a", loadedByID.UpstreamIdentity["channel_app_id"])
	require.Equal(t, "", loadedByID.RedirectTo)
	require.Nil(t, loadedByID.Metadata)
	require.Nil(t, loadedByID.EmailVerifiedAt)
	require.Nil(t, loadedByID.PasswordVerifiedAt)
	require.Nil(t, loadedByID.TOTPVerifiedAt)
	require.Nil(t, loadedByID.ConsumedAt)

	consumed, err := repo.ConsumePendingAuthSession(ctx, "pending-token-1")
	require.NoError(t, err)
	require.NotNil(t, consumed.ConsumedAt)

	reconsumed, err := repo.ConsumePendingAuthSession(ctx, "pending-token-1")
	require.NoError(t, err)
	require.NotNil(t, reconsumed.ConsumedAt)
	require.True(t, reconsumed.ConsumedAt.Equal(*consumed.ConsumedAt))

	loadedAgain, err := repo.GetPendingAuthSessionByToken(ctx, "pending-token-1")
	require.NoError(t, err)
	require.NotNil(t, loadedAgain.ConsumedAt)
	require.True(t, loadedAgain.ConsumedAt.Equal(*consumed.ConsumedAt))
}

func TestPendingAuthSessionRow_UpdateSemanticsForAuthServiceFields(t *testing.T) {
	repo := newUserProfileIdentityTestRepo(t)
	ctx := context.Background()
	expiresAt := time.Now().UTC().Add(15 * time.Minute).Truncate(time.Second)

	created, err := repo.CreatePendingAuthSession(ctx, CreatePendingAuthSessionInput{
		SessionToken:     "pending-token-update",
		Intent:           "adopt_existing_user_by_email",
		ProviderType:     "oidc",
		ProviderKey:      "issuer-a",
		ProviderSubject:  "subject-1",
		TargetUserID:     int64Ptr(7),
		UpstreamIdentity: map[string]any{"issuer": "issuer-a", "display_name": "pending-user"},
		ExpiresAt:        expiresAt,
	})
	require.NoError(t, err)

	emailVerifiedAt := time.Now().UTC().Add(-4 * time.Minute).Truncate(time.Second)
	passwordVerifiedAt := emailVerifiedAt.Add(30 * time.Second)
	totpVerifiedAt := passwordVerifiedAt.Add(30 * time.Second)
	consumedAt := totpVerifiedAt.Add(30 * time.Second)
	metadataJSON, err := json.Marshal(map[string]any{
		"adopt_display_name":     true,
		"adopt_avatar":           false,
		"suggested_display_name": "issuer-user",
	})
	require.NoError(t, err)

	_, err = repo.sql.ExecContext(ctx, `
UPDATE pending_auth_sessions
SET target_user_id = $2,
	redirect_to = $3,
	metadata = $4,
	resolved_email = $5,
	pending_password_hash = $6,
	email_verified_at = $7,
	password_verified_at = $8,
	totp_verified_at = $9,
	consumed_at = $10,
	updated_at = CURRENT_TIMESTAMP
WHERE id = $1`,
		created.ID,
		84,
		"/settings/profile",
		string(metadataJSON),
		"owner@example.com",
		"hashed-password",
		emailVerifiedAt,
		passwordVerifiedAt,
		totpVerifiedAt,
		consumedAt,
	)
	require.NoError(t, err)

	row := mustGetPendingAuthSessionRowByID(t, repo, created.ID)
	require.NotNil(t, row.TargetUserID)
	require.Equal(t, int64(84), *row.TargetUserID)
	require.Equal(t, "/settings/profile", row.RedirectTo)
	require.Equal(t, "owner@example.com", row.ResolvedEmail)
	require.Equal(t, "hashed-password", row.PendingPasswordHash)
	require.True(t, row.Metadata["adopt_display_name"].(bool))
	require.False(t, row.Metadata["adopt_avatar"].(bool))
	require.Equal(t, "issuer-user", row.Metadata["suggested_display_name"])
	require.NotNil(t, row.EmailVerifiedAt)
	require.True(t, row.EmailVerifiedAt.Equal(emailVerifiedAt))
	require.NotNil(t, row.PasswordVerifiedAt)
	require.True(t, row.PasswordVerifiedAt.Equal(passwordVerifiedAt))
	require.NotNil(t, row.TOTPVerifiedAt)
	require.True(t, row.TOTPVerifiedAt.Equal(totpVerifiedAt))
	require.NotNil(t, row.ConsumedAt)
	require.True(t, row.ConsumedAt.Equal(consumedAt))

	loadedByToken, err := repo.GetPendingAuthSessionByToken(ctx, "pending-token-update")
	require.NoError(t, err)
	require.Equal(t, created.ID, loadedByToken.ID)
	require.NotNil(t, loadedByToken.TargetUserID)
	require.Equal(t, int64(84), *loadedByToken.TargetUserID)
	require.Equal(t, "issuer-a", loadedByToken.UpstreamIdentity["issuer"])
	require.NotNil(t, loadedByToken.ConsumedAt)
	require.True(t, loadedByToken.ConsumedAt.Equal(consumedAt))
}

func TestUpsertIdentityAdoptionDecision_RoundTripsWithProviderKeyFallback(t *testing.T) {
	repo := newUserProfileIdentityTestRepo(t)
	ctx := context.Background()

	_, err := repo.UpsertAuthIdentity(ctx, 7, UpsertAuthIdentityInput{
		ProviderType:    "wechat",
		ProviderKey:     "wechat",
		ProviderSubject: "union-1",
	})
	require.NoError(t, err)

	first, err := repo.UpsertIdentityAdoptionDecision(ctx, 7, service.UpsertIdentityAdoptionDecisionInput{
		Ref: service.IdentityAdoptionDecisionRef{
			ProviderType:    "wechat",
			ProviderSubject: "union-1",
		},
		AdoptDisplayName: boolPtr(true),
	})
	require.NoError(t, err)
	require.True(t, first.AdoptDisplayName)
	require.False(t, first.AdoptAvatar)

	second, err := repo.UpsertIdentityAdoptionDecision(ctx, 7, service.UpsertIdentityAdoptionDecisionInput{
		Ref: service.IdentityAdoptionDecisionRef{
			ProviderType:    "wechat",
			ProviderSubject: "union-1",
		},
		AdoptAvatar: boolPtr(false),
	})
	require.NoError(t, err)
	require.True(t, second.AdoptDisplayName)
	require.False(t, second.AdoptAvatar)

	loaded, err := repo.GetIdentityAdoptionDecision(ctx, 7, service.IdentityAdoptionDecisionRef{
		ProviderType:    "wechat",
		ProviderSubject: "union-1",
	})
	require.NoError(t, err)
	require.True(t, loaded.AdoptDisplayName)
	require.False(t, loaded.AdoptAvatar)
}

func TestBindPendingAuthIdentity_WechatOpenIDOnlyCreatesPrimaryIdentityAndChannel(t *testing.T) {
	repo := newUserProfileIdentityTestRepo(t)
	ctx := context.Background()

	err := repo.BindPendingAuthIdentity(ctx, &service.PendingAuthSessionRecord{
		ProviderType:    "wechat",
		ProviderKey:     "wechat-main",
		ProviderSubject: "openid-only-1",
		Metadata: map[string]any{
			"openid":  "openid-only-1",
			"channel": "open",
			"appid":   "wx-open-app",
		},
	}, 7)
	require.NoError(t, err)

	identity, err := repo.FindAuthIdentity(ctx, "wechat", "wechat-main", "openid-only-1")
	require.NoError(t, err)
	require.Equal(t, int64(7), identity.UserID)
	require.Equal(t, "openid-only-1", identity.Metadata["openid"])

	channel, err := repo.FindAuthIdentityChannel(ctx, "wechat", "wechat-main", "open", "wx-open-app", "openid-only-1")
	require.NoError(t, err)
	require.Equal(t, identity.ID, channel.IdentityID)
}

func TestBindPendingAuthIdentity_ReclaimsIdentityOwnedByDeletedUser(t *testing.T) {
	repo := newUserProfileIdentityTestRepo(t)
	ctx := context.Background()

	_, err := repo.UpsertAuthIdentity(ctx, 8, UpsertAuthIdentityInput{
		ProviderType:    "linuxdo",
		ProviderKey:     "linuxdo",
		ProviderSubject: "deleted-user-identity",
	})
	require.NoError(t, err)

	_, err = repo.sql.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, 8)
	require.NoError(t, err)

	err = repo.BindPendingAuthIdentity(ctx, &service.PendingAuthSessionRecord{
		ProviderType:    "linuxdo",
		ProviderKey:     "linuxdo",
		ProviderSubject: "deleted-user-identity",
	}, 7)
	require.NoError(t, err)

	identity, err := repo.FindAuthIdentity(ctx, "linuxdo", "linuxdo", "deleted-user-identity")
	require.NoError(t, err)
	require.NotNil(t, identity)
	require.Equal(t, int64(7), identity.UserID)
}

func TestBindPendingAuthIdentity_WechatUnionIDUpgradesExistingOpenIDIdentity(t *testing.T) {
	repo := newUserProfileIdentityTestRepo(t)
	ctx := context.Background()

	openIDIdentity, err := repo.UpsertAuthIdentity(ctx, 7, UpsertAuthIdentityInput{
		ProviderType:    "wechat",
		ProviderKey:     "wechat-main",
		ProviderSubject: "openid-bridge-1",
		Metadata: map[string]any{
			"openid": "openid-bridge-1",
		},
	})
	require.NoError(t, err)

	_, err = repo.UpsertAuthIdentityChannel(ctx, openIDIdentity.ID, UpsertAuthIdentityChannelInput{
		ProviderType:   "wechat",
		ProviderKey:    "wechat-main",
		Channel:        "mp",
		ChannelAppID:   "wx-mp-app",
		ChannelSubject: "openid-bridge-1",
		Metadata: map[string]any{
			"openid": "openid-bridge-1",
		},
	})
	require.NoError(t, err)

	err = repo.BindPendingAuthIdentity(ctx, &service.PendingAuthSessionRecord{
		ProviderType:    "wechat",
		ProviderKey:     "wechat-main",
		ProviderSubject: "union-bridge-1",
		Metadata: map[string]any{
			"unionid": "union-bridge-1",
			"openid":  "openid-bridge-1",
			"channel": "mp",
			"appid":   "wx-mp-app",
		},
	}, 7)
	require.NoError(t, err)

	upgraded, err := repo.FindAuthIdentity(ctx, "wechat", "wechat-main", "union-bridge-1")
	require.NoError(t, err)
	require.Equal(t, openIDIdentity.ID, upgraded.ID)
	require.Equal(t, "union-bridge-1", upgraded.Metadata["unionid"])
	require.Equal(t, "openid-bridge-1", upgraded.Metadata["openid"])

	_, err = repo.FindAuthIdentity(ctx, "wechat", "wechat-main", "openid-bridge-1")
	require.Error(t, err)
	require.True(t, errors.Is(err, sql.ErrNoRows))

	channel, err := repo.FindAuthIdentityChannel(ctx, "wechat", "wechat-main", "mp", "wx-mp-app", "openid-bridge-1")
	require.NoError(t, err)
	require.Equal(t, upgraded.ID, channel.IdentityID)
}

func TestBindPendingAuthIdentity_WechatUnionIDUpgradesLegacyOpenIDIdentity(t *testing.T) {
	repo := newUserProfileIdentityTestRepo(t)
	ctx := context.Background()

	_, err := repo.sql.ExecContext(ctx, `
INSERT INTO user_external_identities (
	user_id,
	provider,
	provider_user_id,
	display_name,
	metadata
) VALUES ($1, $2, $3, $4, $5)`,
		7, "wechat", "openid-legacy-bridge-1", "legacy-wechat", `{"legacy":"openid-only"}`,
	)
	require.NoError(t, err)

	err = repo.BindPendingAuthIdentity(ctx, &service.PendingAuthSessionRecord{
		ProviderType:    "wechat",
		ProviderKey:     "wechat-main",
		ProviderSubject: "union-legacy-bridge-1",
		Metadata: map[string]any{
			"unionid": "union-legacy-bridge-1",
			"openid":  "openid-legacy-bridge-1",
			"channel": "open",
			"appid":   "wx-open-app",
		},
	}, 7)
	require.NoError(t, err)

	identity, err := repo.FindAuthIdentity(ctx, "wechat", "wechat-main", "union-legacy-bridge-1")
	require.NoError(t, err)
	require.Equal(t, int64(7), identity.UserID)
	require.Equal(t, "union-legacy-bridge-1", identity.ProviderSubject)
	require.Equal(t, "openid-legacy-bridge-1", identity.Metadata["openid"])
	require.Equal(t, "union-legacy-bridge-1", identity.Metadata["unionid"])

	bridgedByOpenID, err := repo.FindAuthIdentity(ctx, "wechat", "wechat-main", "openid-legacy-bridge-1")
	require.NoError(t, err)
	require.Equal(t, identity.ID, bridgedByOpenID.ID)
	require.Equal(t, "union-legacy-bridge-1", bridgedByOpenID.ProviderSubject)

	channel, err := repo.FindAuthIdentityChannel(ctx, "wechat", "wechat-main", "open", "wx-open-app", "openid-legacy-bridge-1")
	require.NoError(t, err)
	require.Equal(t, identity.ID, channel.IdentityID)

	var identityCount int
	rows, err := repo.sql.QueryContext(ctx, `SELECT COUNT(*) FROM auth_identities WHERE provider_type = 'wechat'`)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	require.True(t, rows.Next())
	require.NoError(t, rows.Scan(&identityCount))
	require.Equal(t, 1, identityCount)
}

func TestLoadLatestAuthIdentityMigrationReport_ReturnsNewestSnapshot(t *testing.T) {
	repo := newUserProfileIdentityTestRepo(t)
	ctx := context.Background()

	_, err := repo.sql.ExecContext(ctx, `
INSERT INTO auth_identity_migration_reports (
	email_identity_backfilled_count,
	email_identity_mismatch_count,
	wechat_unionid_migrated_count,
	wechat_legacy_repairable_count,
	wechat_manual_repair_required_count,
	created_at
) VALUES ($1, $2, $3, $4, $5, $6)`,
		1, 0, 2, 3, 4, time.Date(2026, 4, 19, 10, 0, 0, 0, time.UTC),
	)
	require.NoError(t, err)

	_, err = repo.sql.ExecContext(ctx, `
INSERT INTO auth_identity_migration_reports (
	email_identity_backfilled_count,
	email_identity_mismatch_count,
	wechat_unionid_migrated_count,
	wechat_legacy_repairable_count,
	wechat_manual_repair_required_count,
	created_at
) VALUES ($1, $2, $3, $4, $5, $6)`,
		9, 1, 8, 7, 6, time.Date(2026, 4, 19, 11, 0, 0, 0, time.UTC),
	)
	require.NoError(t, err)

	report, err := repo.LoadLatestAuthIdentityMigrationReport(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(9), report.EmailIdentityBackfilledCount)
	require.Equal(t, int64(1), report.EmailIdentityMismatchCount)
	require.Equal(t, int64(8), report.WeChatUnionIDMigratedCount)
	require.Equal(t, int64(7), report.WeChatLegacyRepairableCount)
	require.Equal(t, int64(6), report.WeChatManualRepairRequiredCount)
}

func TestUpsertAvatar_PersistsInlineAvatarData(t *testing.T) {
	repo := newUserProfileIdentityTestRepo(t)
	ctx := context.Background()

	avatar, err := repo.UpsertAvatar(ctx, 7, service.BuildInlineUserAvatarInput([]byte("abc"), "image/png"))
	require.NoError(t, err)
	require.Equal(t, service.UserAvatarStorageProviderInline, avatar.StorageProvider)
	require.Equal(t, "image/png", avatar.ContentType)
	require.Contains(t, avatar.URL, "data:image/png;base64,")

	loaded, err := repo.GetAvatar(ctx, 7)
	require.NoError(t, err)
	require.Equal(t, avatar.StorageKey, loaded.StorageKey)
	require.Equal(t, avatar.URL, loaded.URL)
}

func stringPtr(v string) *string {
	return &v
}

func int64Ptr(v int64) *int64 {
	return &v
}

func boolPtr(v bool) *bool {
	return &v
}

type pendingAuthSessionDBRow struct {
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

func mustGetPendingAuthSessionRowByID(t *testing.T, repo *userRepository, id int64) pendingAuthSessionDBRow {
	t.Helper()

	var (
		row                  pendingAuthSessionDBRow
		targetUserID         sql.NullInt64
		metadataJSON         []byte
		upstreamIdentityJSON []byte
		emailVerifiedAt      sql.NullTime
		passwordVerifiedAt   sql.NullTime
		totpVerifiedAt       sql.NullTime
		consumedAt           sql.NullTime
	)

	err := scanSingleRow(context.Background(), repo.sql, `
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
WHERE id = $1`,
		[]any{id},
		&row.ID,
		&row.SessionToken,
		&row.Intent,
		&row.ProviderType,
		&row.ProviderKey,
		&row.ProviderSubject,
		&targetUserID,
		&row.RedirectTo,
		&metadataJSON,
		&row.ResolvedEmail,
		&row.PendingPasswordHash,
		&upstreamIdentityJSON,
		&emailVerifiedAt,
		&passwordVerifiedAt,
		&totpVerifiedAt,
		&row.ExpiresAt,
		&consumedAt,
		&row.CreatedAt,
		&row.UpdatedAt,
	)
	require.NoError(t, err)

	if targetUserID.Valid {
		row.TargetUserID = int64Ptr(targetUserID.Int64)
	}
	row.Metadata = mustUnmarshalPendingAuthJSONMap(t, metadataJSON)
	row.UpstreamIdentity = mustUnmarshalPendingAuthJSONMap(t, upstreamIdentityJSON)
	row.EmailVerifiedAt = nullTimePtr(emailVerifiedAt)
	row.PasswordVerifiedAt = nullTimePtr(passwordVerifiedAt)
	row.TOTPVerifiedAt = nullTimePtr(totpVerifiedAt)
	row.ConsumedAt = nullTimePtr(consumedAt)
	return row
}

func mustUnmarshalPendingAuthJSONMap(t *testing.T, raw []byte) map[string]any {
	t.Helper()

	if len(raw) == 0 {
		return nil
	}

	var out map[string]any
	require.NoError(t, json.Unmarshal(raw, &out))
	if len(out) == 0 {
		return nil
	}
	return out
}

func TestFindAuthIdentity_LegacyLinuxDoRowIsMaterialized(t *testing.T) {
	repo := newUserProfileIdentityTestRepo(t)
	ctx := context.Background()

	_, err := repo.sql.ExecContext(ctx, `
INSERT INTO user_external_identities (
	user_id,
	provider,
	provider_user_id,
	provider_username,
	display_name,
	avatar_url,
	metadata
) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		7, "linuxdo", "363223", "EnjoyLeisure", "EnjoyLeisure", "https://example.com/avatar.png", `{"legacy":"linuxdo"}`,
	)
	require.NoError(t, err)

	identity, err := repo.FindAuthIdentity(ctx, "linuxdo", "linuxdo", "363223")
	require.NoError(t, err)
	require.Equal(t, int64(7), identity.UserID)
	require.Equal(t, "363223", identity.ProviderSubject)
	require.Equal(t, "EnjoyLeisure", identity.Metadata["username"])

	stored, err := repo.findStoredAuthIdentity(ctx, "linuxdo", "linuxdo", "363223")
	require.NoError(t, err)
	require.Equal(t, identity.ID, stored.ID)
}

func TestFindAuthIdentity_LegacyWeChatOpenIDMaterializesUnionIDIdentity(t *testing.T) {
	repo := newUserProfileIdentityTestRepo(t)
	ctx := context.Background()

	_, err := repo.sql.ExecContext(ctx, `
INSERT INTO user_external_identities (
	user_id,
	provider,
	provider_user_id,
	provider_union_id,
	display_name,
	metadata
) VALUES ($1, $2, $3, $4, $5, $6)`,
		9, "wechat", "openid-legacy-1", "union-legacy-1", "wechat-user", `{"legacy":"wechat"}`,
	)
	require.NoError(t, err)

	identity, err := repo.FindAuthIdentity(ctx, "wechat", "wechat", "openid-legacy-1")
	require.NoError(t, err)
	require.Equal(t, int64(9), identity.UserID)
	require.Equal(t, "union-legacy-1", identity.ProviderSubject)
	require.Equal(t, "openid-legacy-1", identity.Metadata["openid"])
	require.Equal(t, "union-legacy-1", identity.Metadata["unionid"])

	stored, err := repo.findStoredAuthIdentity(ctx, "wechat", "wechat", "union-legacy-1")
	require.NoError(t, err)
	require.Equal(t, identity.ID, stored.ID)
}

func TestFindAuthIdentity_WechatMainFallsBackToLegacyProviderKey(t *testing.T) {
	repo := newUserProfileIdentityTestRepo(t)
	ctx := context.Background()

	stored, err := repo.UpsertAuthIdentity(ctx, 9, UpsertAuthIdentityInput{
		ProviderType:    "wechat",
		ProviderKey:     "wechat",
		ProviderSubject: "union-legacy-provider-key-1",
		Metadata: map[string]any{
			"unionid": "union-legacy-provider-key-1",
			"openid":  "openid-legacy-provider-key-1",
		},
	})
	require.NoError(t, err)

	identity, err := repo.FindAuthIdentity(ctx, "wechat", "wechat-main", "union-legacy-provider-key-1")
	require.NoError(t, err)
	require.Equal(t, stored.ID, identity.ID)
	require.Equal(t, "wechat", identity.ProviderKey)
}

func TestFindAuthIdentityChannel_WechatMainFallsBackToLegacyProviderKey(t *testing.T) {
	repo := newUserProfileIdentityTestRepo(t)
	ctx := context.Background()

	identity, err := repo.UpsertAuthIdentity(ctx, 9, UpsertAuthIdentityInput{
		ProviderType:    "wechat",
		ProviderKey:     "wechat",
		ProviderSubject: "union-channel-legacy-key-1",
		Metadata: map[string]any{
			"unionid": "union-channel-legacy-key-1",
			"openid":  "openid-channel-legacy-key-1",
		},
	})
	require.NoError(t, err)

	channel, err := repo.UpsertAuthIdentityChannel(ctx, identity.ID, UpsertAuthIdentityChannelInput{
		ProviderType:   "wechat",
		ProviderKey:    "wechat",
		Channel:        "open",
		ChannelAppID:   "wx-open-app",
		ChannelSubject: "openid-channel-legacy-key-1",
		Metadata: map[string]any{
			"openid": "openid-channel-legacy-key-1",
		},
	})
	require.NoError(t, err)

	loaded, err := repo.FindAuthIdentityChannel(ctx, "wechat", "wechat-main", "open", "wx-open-app", "openid-channel-legacy-key-1")
	require.NoError(t, err)
	require.Equal(t, channel.ID, loaded.ID)
	require.Equal(t, "wechat", loaded.ProviderKey)
}

func TestListUserExternalIdentities_IncludesLegacyRows(t *testing.T) {
	repo := newUserProfileIdentityTestRepo(t)
	ctx := context.Background()

	_, err := repo.sql.ExecContext(ctx, `
INSERT INTO user_external_identities (
	user_id,
	provider,
	provider_user_id,
	provider_union_id,
	provider_username
) VALUES
	($1, $2, $3, NULL, $4),
	($1, $5, $6, $7, '')`,
		11, "linuxdo", "linuxdo-11", "legacy-linuxdo", "wechat", "openid-11", "union-11",
	)
	require.NoError(t, err)

	identities, err := repo.ListUserExternalIdentities(ctx, 11)
	require.NoError(t, err)
	require.Len(t, identities, 2)
	require.Equal(t, service.ExternalIdentityProviderLinuxDo, identities[0].Provider)
	require.Equal(t, "linuxdo-11", identities[0].ProviderUserID)
	require.Equal(t, service.ExternalIdentityProviderWeChat, identities[1].Provider)
	require.Equal(t, "union-11", identities[1].ProviderUserID)
}

func TestDeleteUserExternalIdentity_RemovesLegacyRows(t *testing.T) {
	repo := newUserProfileIdentityTestRepo(t)
	ctx := context.Background()

	_, err := repo.sql.ExecContext(ctx, `
INSERT INTO user_external_identities (
	user_id,
	provider,
	provider_user_id
) VALUES ($1, $2, $3)`,
		21, "linuxdo", "legacy-delete-1",
	)
	require.NoError(t, err)

	deleted, err := repo.DeleteUserExternalIdentity(ctx, 21, service.ExternalIdentityProviderLinuxDo)
	require.NoError(t, err)
	require.True(t, deleted)

	err = scanSingleRow(ctx, repo.sql, `
SELECT id
FROM user_external_identities
WHERE user_id = $1 AND provider = $2`,
		[]any{21, "linuxdo"},
		new(int64),
	)
	require.ErrorIs(t, err, sql.ErrNoRows)
}
