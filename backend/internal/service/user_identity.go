package service

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type ExternalIdentityProvider string

const (
	ExternalIdentityProviderLinuxDo ExternalIdentityProvider = "linuxdo"
	ExternalIdentityProviderOIDC    ExternalIdentityProvider = "oidc"
	ExternalIdentityProviderWeChat  ExternalIdentityProvider = "wechat"

	UserAvatarStorageProviderInline       = "inline"
	DefaultUserAvatarMaxBytes       int64 = 100 * 1024
)

var (
	ErrUserAvatarNotFound             = infraerrors.NotFound("USER_AVATAR_NOT_FOUND", "user avatar not found")
	ErrUserAvatarStorageProviderEmpty = infraerrors.BadRequest("USER_AVATAR_STORAGE_PROVIDER_REQUIRED", "avatar storage provider is required")
	ErrUserAvatarStorageKeyEmpty      = infraerrors.BadRequest("USER_AVATAR_STORAGE_KEY_REQUIRED", "avatar storage key is required")
	ErrUserAvatarContentTypeEmpty     = infraerrors.BadRequest("USER_AVATAR_CONTENT_TYPE_REQUIRED", "avatar content type is required")
	ErrUserAvatarTooLarge             = infraerrors.BadRequest("USER_AVATAR_TOO_LARGE", "avatar exceeds 100KB limit")
)

// WeChatConnectSyntheticEmailDomain keeps migration compatibility with synthetic
// emails used for third-party-only accounts that do not have a usable local login.
const WeChatConnectSyntheticEmailDomain = "@wechat-connect.invalid"

type UserExternalIdentity struct {
	Provider       ExternalIdentityProvider
	ProviderUserID string
}

type UserAvatar struct {
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

type UpsertUserAvatarInput struct {
	StorageProvider string
	StorageKey      string
	URL             string
	ContentType     string
	ByteSize        int64
	SHA256          string
}

type ProviderAvailability struct {
	Enabled     bool
	ConfigValid bool
}

type UserIdentityState struct {
	User               *User
	ExternalIdentities []UserExternalIdentity
}

func BuildUserIdentityState(user *User, identities []UserExternalIdentity) UserIdentityState {
	if len(identities) == 0 && user != nil && len(user.ExternalIdentities) > 0 {
		identities = user.ExternalIdentities
	}
	normalized := make([]UserExternalIdentity, 0, len(identities))
	for _, identity := range identities {
		identity = normalizeUserExternalIdentity(identity)
		if identity.Provider == "" || identity.ProviderUserID == "" {
			continue
		}
		normalized = append(normalized, identity)
	}
	state := UserIdentityState{
		User:               user,
		ExternalIdentities: normalized,
	}
	if len(state.ExternalIdentities) == 0 {
		state.ExternalIdentities = inferLegacyExternalIdentities(user)
	}
	return state
}

func (in UpsertUserAvatarInput) Validate(maxBytes int64) error {
	if maxBytes <= 0 {
		maxBytes = DefaultUserAvatarMaxBytes
	}
	if strings.TrimSpace(in.StorageProvider) == "" {
		return ErrUserAvatarStorageProviderEmpty
	}
	if strings.TrimSpace(in.StorageKey) == "" {
		return ErrUserAvatarStorageKeyEmpty
	}
	if strings.TrimSpace(in.ContentType) == "" {
		return ErrUserAvatarContentTypeEmpty
	}
	if in.ByteSize < 0 || in.ByteSize > maxBytes {
		return ErrUserAvatarTooLarge.WithMetadata(map[string]string{"max_bytes": fmt.Sprintf("%d", maxBytes)})
	}
	return nil
}

func (s UserIdentityState) HasUsableLocalLogin() bool {
	if s.User == nil {
		return false
	}
	if strings.TrimSpace(s.User.PasswordHash) == "" {
		return false
	}
	return !isSyntheticOAuthEmail(s.User.Email)
}

func CanDisconnectExternalIdentity(state UserIdentityState, provider ExternalIdentityProvider, availability map[ExternalIdentityProvider]ProviderAvailability) bool {
	if state.HasUsableLocalLogin() {
		return true
	}

	provider = NormalizeExternalIdentityProvider(provider)
	normalizedAvailability := make(map[ExternalIdentityProvider]ProviderAvailability, len(availability))
	for key, status := range availability {
		normalizedAvailability[NormalizeExternalIdentityProvider(key)] = status
	}

	remaining := make(map[ExternalIdentityProvider]struct{}, len(state.ExternalIdentities))
	for _, identity := range state.ExternalIdentities {
		if identity.Provider == provider {
			continue
		}
		remaining[identity.Provider] = struct{}{}
	}

	for remainingProvider := range remaining {
		status, ok := normalizedAvailability[NormalizeExternalIdentityProvider(remainingProvider)]
		if ok && status.Enabled && status.ConfigValid {
			return true
		}
	}

	return false
}

func inferLegacyExternalIdentities(user *User) []UserExternalIdentity {
	if user == nil {
		return nil
	}

	email := strings.TrimSpace(user.Email)
	switch {
	case strings.HasSuffix(email, LinuxDoConnectSyntheticEmailDomain):
		return []UserExternalIdentity{{
			Provider:       ExternalIdentityProviderLinuxDo,
			ProviderUserID: strings.TrimSuffix(strings.TrimPrefix(email, "linuxdo-"), LinuxDoConnectSyntheticEmailDomain),
		}}
	case strings.HasSuffix(email, OIDCConnectSyntheticEmailDomain):
		return []UserExternalIdentity{{
			Provider:       ExternalIdentityProviderOIDC,
			ProviderUserID: strings.TrimSuffix(strings.TrimPrefix(email, "oidc-"), OIDCConnectSyntheticEmailDomain),
		}}
	case strings.HasSuffix(email, WeChatConnectSyntheticEmailDomain):
		return []UserExternalIdentity{{
			Provider:       ExternalIdentityProviderWeChat,
			ProviderUserID: strings.TrimSuffix(strings.TrimPrefix(email, "wechat-"), WeChatConnectSyntheticEmailDomain),
		}}
	default:
		return nil
	}
}

func isSyntheticOAuthEmail(email string) bool {
	normalized := strings.ToLower(strings.TrimSpace(email))
	return strings.HasSuffix(normalized, LinuxDoConnectSyntheticEmailDomain) ||
		strings.HasSuffix(normalized, OIDCConnectSyntheticEmailDomain) ||
		strings.HasSuffix(normalized, WeChatConnectSyntheticEmailDomain)
}

func NormalizeExternalIdentityProvider(provider ExternalIdentityProvider) ExternalIdentityProvider {
	switch normalized := strings.ToLower(strings.TrimSpace(string(provider))); normalized {
	case string(ExternalIdentityProviderLinuxDo):
		return ExternalIdentityProviderLinuxDo
	case string(ExternalIdentityProviderOIDC):
		return ExternalIdentityProviderOIDC
	case string(ExternalIdentityProviderWeChat):
		return ExternalIdentityProviderWeChat
	default:
		return ExternalIdentityProvider(normalized)
	}
}

func ExternalIdentityProviderDisplayName(provider ExternalIdentityProvider) string {
	switch NormalizeExternalIdentityProvider(provider) {
	case ExternalIdentityProviderLinuxDo:
		return "LinuxDo"
	case ExternalIdentityProviderOIDC:
		return "OIDC"
	case ExternalIdentityProviderWeChat:
		return "WeChat"
	default:
		return strings.TrimSpace(string(provider))
	}
}

func normalizeUserExternalIdentity(identity UserExternalIdentity) UserExternalIdentity {
	identity.Provider = NormalizeExternalIdentityProvider(identity.Provider)
	identity.ProviderUserID = strings.TrimSpace(identity.ProviderUserID)
	return identity
}

func ResolvePreferredUserAvatarURL(avatar *UserAvatar) string {
	if avatar == nil {
		return ""
	}
	return strings.TrimSpace(avatar.URL)
}

func BuildInlineUserAvatarInput(payload []byte, contentType string) UpsertUserAvatarInput {
	sum := sha256.Sum256(payload)
	shaHex := hex.EncodeToString(sum[:])
	return UpsertUserAvatarInput{
		StorageProvider: UserAvatarStorageProviderInline,
		StorageKey:      shaHex,
		URL:             "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(payload),
		ContentType:     contentType,
		ByteSize:        int64(len(payload)),
		SHA256:          shaHex,
	}
}
