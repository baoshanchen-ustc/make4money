package service

import (
	"context"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

// =====================
// 领域模型
// =====================

// UserReferralProfile 用户专属邀请码 Profile
type UserReferralProfile struct {
	ID           int64
	UserID       int64
	ReferralCode string
	CreatedAt    time.Time
}

// ReferralRelation 邀请关系记录
type ReferralRelation struct {
	ID            int64
	InviterID     int64
	InviteeID     int64
	InviterReward float64
	InviteeReward float64
	RewardGranted bool
	CreatedAt     time.Time

	// 辅助字段（查询时填充，非 DB 字段）
	InviteeEmail string
}

// ReferralStats 全平台邀请统计（管理员视图）
type ReferralStats struct {
	TotalRelations          int64   `json:"total_relations"`
	TotalInviterRewardGiven float64 `json:"total_inviter_reward_granted"`
	TotalInviteeRewardGiven float64 `json:"total_invitee_reward_granted"`
}

// ReferralInfo 用户端邀请信息（聚合视图）
type ReferralInfo struct {
	ReferralCode      string       `json:"referral_code"`
	ReferralLink      string       `json:"referral_link"`
	TotalInvitees     int64        `json:"total_invitees"`
	TotalRewardEarned float64      `json:"total_reward_earned"`
	InviterInfo       *InviterInfo `json:"inviter_info,omitempty"`
}

// InviterInfo 邀请人信息（脱敏）
type InviterInfo struct {
	EmailMasked string `json:"email_masked"`
}

// ReferralInvitee 邀请记录列表项
type ReferralInvitee struct {
	EmailMasked  string    `json:"email_masked"`
	RegisteredAt time.Time `json:"registered_at"`
	RewardEarned float64   `json:"reward_earned"`
}

// =====================
// Repository 接口
// =====================

// ReferralRepository 邀请推广数据仓储接口
type ReferralRepository interface {
	// Profile 操作
	CreateProfile(ctx context.Context, userID int64, referralCode string) (*UserReferralProfile, error)
	GetProfileByUserID(ctx context.Context, userID int64) (*UserReferralProfile, error)
	GetProfileByCode(ctx context.Context, code string) (*UserReferralProfile, error)

	// 关系操作
	CreateRelation(ctx context.Context, relation *ReferralRelation) error
	GetRelationByInviteeID(ctx context.Context, inviteeID int64) (*ReferralRelation, error)

	// 关系状态更新
	MarkRewardGranted(ctx context.Context, id int64) error

	// 查询操作
	ListByInviterID(ctx context.Context, inviterID int64, params pagination.PaginationParams) ([]ReferralRelation, *pagination.PaginationResult, error)
	CountByInviterID(ctx context.Context, inviterID int64) (int64, error)
	SumRewardsByInviterID(ctx context.Context, inviterID int64) (float64, error)

	// 管理员统计
	GetPlatformStats(ctx context.Context) (*ReferralStats, error)
}

// =====================
// 错误定义
// =====================

var (
	ErrReferralCodeNotFound   = newBadRequestError("REFERRAL_CODE_NOT_FOUND", "referral code not found or invalid")
	ErrReferralCodeInvalid    = newBadRequestError("REFERRAL_CODE_INVALID", "referral code is invalid")
	ErrSelfReferralNotAllowed = newBadRequestError("SELF_REFERRAL_NOT_ALLOWED", "cannot use your own referral code")
	ErrAlreadyReferred        = newBadRequestError("ALREADY_REFERRED", "this user has already been referred")
)

func newBadRequestError(code, msg string) error {
	return infraerrors.BadRequest(code, msg)
}
