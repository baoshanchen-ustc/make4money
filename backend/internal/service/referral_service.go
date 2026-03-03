package service

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

// referralCodeCharset 邀请码字符集：大写字母 + 数字，去除易混淆字符
const referralCodeCharset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// referralCodeLength 邀请码长度：8 位
const referralCodeLength = 8

// referralCodeMaxRetries 邀请码冲突时最大重试次数
const referralCodeMaxRetries = 5

// ReferralService 裂变推广业务逻辑服务
type ReferralService struct {
	referralRepo        ReferralRepository
	userRepo            UserRepository
	settingService      *SettingService
	billingCacheService *BillingCacheService
	entClient           *dbent.Client
}

// NewReferralService 创建裂变推广服务实例
func NewReferralService(
	referralRepo ReferralRepository,
	userRepo UserRepository,
	settingService *SettingService,
	billingCacheService *BillingCacheService,
	entClient *dbent.Client,
) *ReferralService {
	return &ReferralService{
		referralRepo:        referralRepo,
		userRepo:            userRepo,
		settingService:      settingService,
		billingCacheService: billingCacheService,
		entClient:           entClient,
	}
}

// GetOrCreateProfile 获取或懒加载创建用户的专属邀请码 Profile
func (s *ReferralService) GetOrCreateProfile(ctx context.Context, userID int64) (*UserReferralProfile, error) {
	// 先查询
	profile, err := s.referralRepo.GetProfileByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get referral profile: %w", err)
	}
	if profile != nil {
		return profile, nil
	}

	// 不存在则生成并创建（带重试应对极小概率的码冲突）
	for i := 0; i < referralCodeMaxRetries; i++ {
		code, err := generateReferralCode()
		if err != nil {
			return nil, fmt.Errorf("generate referral code: %w", err)
		}
		created, err := s.referralRepo.CreateProfile(ctx, userID, code)
		if err == nil {
			return created, nil
		}
		// 唯一约束冲突时重试，其他错误直接返回
		if !isUniqueConflict(err) {
			return nil, fmt.Errorf("create referral profile: %w", err)
		}
	}
	return nil, fmt.Errorf("failed to generate unique referral code after %d retries", referralCodeMaxRetries)
}

// ValidateReferralCode 注册前验证邀请码有效性
// 返回 nil, nil 表示空码（不报错）
func (s *ReferralService) ValidateReferralCode(ctx context.Context, code string, inviteeID int64) (*UserReferralProfile, error) {
	code = strings.TrimSpace(strings.ToUpper(code))
	if code == "" {
		return nil, nil
	}

	profile, err := s.referralRepo.GetProfileByCode(ctx, code)
	if err != nil {
		return nil, err // ErrReferralCodeNotFound
	}

	// 自己不能使用自己的邀请码
	if profile.UserID == inviteeID {
		return nil, ErrSelfReferralNotAllowed
	}

	return profile, nil
}

// GrantRewardsInTx 在已有事务上下文中完成邀请关系创建和奖励发放。
//
// 调用者（AuthService）负责事务管理，本方法仅在 txCtx 中执行。
// 幂等保障：invitee_id UNIQUE 约束（DB 层）+ reward_granted 标志（应用层）。
func (s *ReferralService) GrantRewardsInTx(ctx context.Context, inviterID, inviteeID int64, inviterReward, inviteeReward float64) error {
	relation := &ReferralRelation{
		InviterID:     inviterID,
		InviteeID:     inviteeID,
		InviterReward: inviterReward,
		InviteeReward: inviteeReward,
		RewardGranted: false,
	}

	// 创建邀请关系（invitee_id UNIQUE 约束保证幂等）
	if err := s.referralRepo.CreateRelation(ctx, relation); err != nil {
		return fmt.Errorf("create referral relation: %w", err)
	}

	// 发放邀请人奖励
	if inviterReward > 0 {
		if err := s.userRepo.UpdateBalance(ctx, inviterID, inviterReward); err != nil {
			return fmt.Errorf("grant inviter reward: %w", err)
		}
	}

	// 发放被邀请人奖励
	if inviteeReward > 0 {
		if err := s.userRepo.UpdateBalance(ctx, inviteeID, inviteeReward); err != nil {
			return fmt.Errorf("grant invitee reward: %w", err)
		}
	}

	// 标记奖励已发放（应用层幂等标志）
	if err := s.referralRepo.MarkRewardGranted(ctx, relation.ID); err != nil {
		return fmt.Errorf("mark reward granted: %w", err)
	}

	return nil
}

// GetMyReferralInfo 获取当前用户的邀请统计信息
func (s *ReferralService) GetMyReferralInfo(ctx context.Context, userID int64, siteBaseURL string) (*ReferralInfo, error) {
	// 获取或创建 Profile
	profile, err := s.GetOrCreateProfile(ctx, userID)
	if err != nil {
		return nil, err
	}

	// 统计数据
	totalInvitees, err := s.referralRepo.CountByInviterID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("count invitees: %w", err)
	}

	totalRewardEarned, err := s.referralRepo.SumRewardsByInviterID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("sum rewards: %w", err)
	}

	info := &ReferralInfo{
		ReferralCode:      profile.ReferralCode,
		ReferralLink:      buildReferralLink(siteBaseURL, profile.ReferralCode),
		TotalInvitees:     totalInvitees,
		TotalRewardEarned: totalRewardEarned,
	}

	// 如果该用户自己是被邀请的，填充邀请人信息
	relation, err := s.referralRepo.GetRelationByInviteeID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get inviter relation: %w", err)
	}
	if relation != nil {
		inviter, err := s.userRepo.GetByID(ctx, relation.InviterID)
		if err == nil && inviter != nil {
			info.InviterInfo = &InviterInfo{
				EmailMasked: maskEmail(inviter.Email),
			}
		}
	}

	return info, nil
}

// ListMyInvitees 分页获取当前用户邀请的用户列表
func (s *ReferralService) ListMyInvitees(ctx context.Context, userID int64, params pagination.PaginationParams) ([]ReferralInvitee, *pagination.PaginationResult, error) {
	relations, result, err := s.referralRepo.ListByInviterID(ctx, userID, params)
	if err != nil {
		return nil, nil, fmt.Errorf("list invitees: %w", err)
	}

	invitees := make([]ReferralInvitee, len(relations))
	for i, rel := range relations {
		invitees[i] = ReferralInvitee{
			EmailMasked:  maskEmail(rel.InviteeEmail),
			RegisteredAt: rel.CreatedAt,
			RewardEarned: rel.InviterReward,
		}
	}

	return invitees, result, nil
}

// GetPlatformStats 获取全平台邀请统计（管理员）
func (s *ReferralService) GetPlatformStats(ctx context.Context) (*ReferralStats, error) {
	return s.referralRepo.GetPlatformStats(ctx)
}

// ProcessReferralRegistration 在注册完成后处理邀请奖励（自带事务）。
// 从 SettingService 读取奖励金额后，在单一事务内创建邀请关系并更新双方余额。
// 失败不影响注册结果，调用方应仅记录日志。
func (s *ReferralService) ProcessReferralRegistration(ctx context.Context, inviterID, inviteeID int64) error {
	inviterReward, inviteeReward, err := s.settingService.GetReferralRewards(ctx)
	if err != nil {
		return fmt.Errorf("get referral rewards: %w", err)
	}

	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin referral tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	txCtx := dbent.NewTxContext(ctx, tx)

	if err := s.GrantRewardsInTx(txCtx, inviterID, inviteeID, inviterReward, inviteeReward); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit referral tx: %w", err)
	}

	s.InvalidateRewardCaches(inviterID, inviteeID)
	return nil
}

// InvalidateRewardCaches 异步失效奖励相关的余额缓存（在事务提交后调用）
func (s *ReferralService) InvalidateRewardCaches(inviterID, inviteeID int64) {
	if s.billingCacheService == nil {
		return
	}
	go func() {
		cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.billingCacheService.InvalidateUserBalance(cacheCtx, inviterID)
		_ = s.billingCacheService.InvalidateUserBalance(cacheCtx, inviteeID)
	}()
}

// =====================
// 辅助函数
// =====================

// generateReferralCode 使用 crypto/rand 生成 8 位大写字母+数字的邀请码
func generateReferralCode() (string, error) {
	buf := make([]byte, referralCodeLength)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	code := make([]byte, referralCodeLength)
	for i, b := range buf {
		code[i] = referralCodeCharset[int(b)%len(referralCodeCharset)]
	}
	return string(code), nil
}

// maskEmail 对邮箱地址进行脱敏处理，只保留前3位和域名部分
// 例如：user@example.com → use***@example.com
func maskEmail(email string) string {
	if email == "" {
		return ""
	}
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 {
		return email
	}
	local := parts[0]
	domain := parts[1]
	if len(local) <= 3 {
		return local + "***@" + domain
	}
	return local[:3] + "***@" + domain
}

// buildReferralLink 构建邀请链接
func buildReferralLink(siteBaseURL, code string) string {
	base := strings.TrimSuffix(strings.TrimSpace(siteBaseURL), "/")
	if base == "" {
		return "?ref=" + code
	}
	return base + "/register?ref=" + code
}

// isUniqueConflict 判断错误是否为唯一约束冲突（用于邀请码重试逻辑）
func isUniqueConflict(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate key") ||
		strings.Contains(msg, "unique constraint") ||
		strings.Contains(msg, "duplicate entry")
}
