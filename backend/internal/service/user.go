package service

import (
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	SignupSourceUnknown = "unknown"
	SignupSourceEmail   = "email"
	SignupSourceLinuxDo = "linuxdo"
	SignupSourceOIDC    = "oidc"
	SignupSourceWeChat  = "wechat"
)

type User struct {
	ID            int64
	Email         string
	Username      string
	Notes         string
	PasswordHash  string
	SignupSource  string
	Role          string
	Balance       float64
	Concurrency   int
	Status        string
	AllowedGroups []int64
	TokenVersion  int64 // Incremented on password change to invalidate existing tokens
	CreatedAt     time.Time
	UpdatedAt     time.Time

	// GroupRates 用户专属分组倍率配置
	// map[groupID]rateMultiplier
	GroupRates map[int64]float64

	// TOTP 双因素认证字段
	TotpSecretEncrypted *string    // AES-256-GCM 加密的 TOTP 密钥
	TotpEnabled         bool       // 是否启用 TOTP
	TotpEnabledAt       *time.Time // TOTP 启用时间

	// 余额不足通知
	BalanceNotifyEnabled       bool
	BalanceNotifyThresholdType string // "fixed" (default) | "percentage"
	BalanceNotifyThreshold     *float64
	BalanceNotifyExtraEmails   []NotifyEmailEntry
	TotalRecharged             float64
	Avatar                     *UserAvatar
	AvatarURL                  string
	AvatarUpdatedAt            *time.Time
	HasCustomAvatar            bool
	ExternalIdentities         []UserExternalIdentity

	APIKeys       []APIKey
	Subscriptions []UserSubscription
}

func (u *User) IsAdmin() bool {
	return u.Role == RoleAdmin
}

func (u *User) IsActive() bool {
	return u.Status == StatusActive
}

// CanBindGroup checks whether a user can bind to a given group.
// For standard groups:
// - Public groups (non-exclusive): all users can bind
// - Exclusive groups: only users with the group in AllowedGroups can bind
func (u *User) CanBindGroup(groupID int64, isExclusive bool) bool {
	// 公开分组（非专属）：所有用户都可以绑定
	if !isExclusive {
		return true
	}
	// 专属分组：需要在 AllowedGroups 中
	for _, id := range u.AllowedGroups {
		if id == groupID {
			return true
		}
	}
	return false
}

func (u *User) SetPassword(password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.PasswordHash = string(hash)
	return nil
}

func (u *User) CheckPassword(password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) == nil
}

func NormalizeSignupSource(source string) string {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case SignupSourceEmail:
		return SignupSourceEmail
	case SignupSourceLinuxDo:
		return SignupSourceLinuxDo
	case SignupSourceOIDC:
		return SignupSourceOIDC
	case SignupSourceWeChat:
		return SignupSourceWeChat
	default:
		return SignupSourceUnknown
	}
}

func (u *User) HasLocalIdentity() bool {
	if u == nil {
		return false
	}
	if strings.TrimSpace(u.PasswordHash) == "" {
		return false
	}

	email := strings.ToLower(strings.TrimSpace(u.Email))
	if email == "" {
		return false
	}

	return !isSyntheticOAuthEmail(email)
}
