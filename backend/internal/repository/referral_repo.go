package repository

// 注意：此文件依赖 Ent 代码生成产物。
// 修改 ent/schema/ 下的 Schema 文件后，必须先在 backend/ 目录执行：
//
//	go generate ./ent/...
//
// 才能使本文件编译通过。

import (
	"context"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/referralrelation"
	"github.com/Wei-Shaw/sub2api/ent/userreferralprofile"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type referralRepository struct {
	client *dbent.Client
}

// NewReferralRepository 创建裂变推广仓储实例
func NewReferralRepository(client *dbent.Client) service.ReferralRepository {
	return &referralRepository{client: client}
}

// =====================
// Profile 操作
// =====================

func (r *referralRepository) CreateProfile(ctx context.Context, userID int64, referralCode string) (*service.UserReferralProfile, error) {
	client := clientFromContext(ctx, r.client)
	m, err := client.UserReferralProfile.Create().
		SetUserID(userID).
		SetReferralCode(referralCode).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return referralProfileEntityToService(m), nil
}

func (r *referralRepository) GetProfileByUserID(ctx context.Context, userID int64) (*service.UserReferralProfile, error) {
	client := clientFromContext(ctx, r.client)
	m, err := client.UserReferralProfile.Query().
		Where(userreferralprofile.UserIDEQ(userID)).
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, nil // 惰性创建场景：profile 不存在返回 nil
		}
		return nil, err
	}
	return referralProfileEntityToService(m), nil
}

func (r *referralRepository) GetProfileByCode(ctx context.Context, code string) (*service.UserReferralProfile, error) {
	client := clientFromContext(ctx, r.client)
	m, err := client.UserReferralProfile.Query().
		Where(userreferralprofile.ReferralCodeEQ(code)).
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, service.ErrReferralCodeNotFound
		}
		return nil, err
	}
	return referralProfileEntityToService(m), nil
}

// =====================
// 关系操作
// =====================

func (r *referralRepository) CreateRelation(ctx context.Context, relation *service.ReferralRelation) error {
	client := clientFromContext(ctx, r.client)
	m, err := client.ReferralRelation.Create().
		SetInviterID(relation.InviterID).
		SetInviteeID(relation.InviteeID).
		SetInviterReward(relation.InviterReward).
		SetInviteeReward(relation.InviteeReward).
		SetRewardGranted(relation.RewardGranted).
		Save(ctx)
	if err != nil {
		return err
	}
	relation.ID = m.ID
	relation.CreatedAt = m.CreatedAt
	return nil
}

func (r *referralRepository) GetRelationByInviteeID(ctx context.Context, inviteeID int64) (*service.ReferralRelation, error) {
	client := clientFromContext(ctx, r.client)
	m, err := client.ReferralRelation.Query().
		Where(referralrelation.InviteeIDEQ(inviteeID)).
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, nil // 该用户未被邀请
		}
		return nil, err
	}
	return referralRelationEntityToService(m), nil
}

// MarkRewardGranted 将邀请关系的 reward_granted 标志设为 true，
// 表示奖励已成功发放。必须在事务上下文中调用。
func (r *referralRepository) MarkRewardGranted(ctx context.Context, id int64) error {
	client := clientFromContext(ctx, r.client)
	return client.ReferralRelation.UpdateOneID(id).SetRewardGranted(true).Exec(ctx)
}

// =====================
// 查询操作
// =====================

func (r *referralRepository) ListByInviterID(ctx context.Context, inviterID int64, params pagination.PaginationParams) ([]service.ReferralRelation, *pagination.PaginationResult, error) {
	client := clientFromContext(ctx, r.client)
	q := client.ReferralRelation.Query().
		Where(referralrelation.InviterIDEQ(inviterID)).
		WithInvitee()

	total, err := q.Clone().Count(ctx)
	if err != nil {
		return nil, nil, err
	}

	entities, err := q.
		Order(dbent.Desc(referralrelation.FieldID)).
		Offset(params.Offset()).
		Limit(params.Limit()).
		All(ctx)
	if err != nil {
		return nil, nil, err
	}

	out := make([]service.ReferralRelation, len(entities))
	for i, e := range entities {
		out[i] = *referralRelationEntityToService(e)
	}

	return out, paginationResultFromTotal(int64(total), params), nil
}

func (r *referralRepository) CountByInviterID(ctx context.Context, inviterID int64) (int64, error) {
	client := clientFromContext(ctx, r.client)
	n, err := client.ReferralRelation.Query().
		Where(referralrelation.InviterIDEQ(inviterID)).
		Count(ctx)
	return int64(n), err
}

func (r *referralRepository) SumRewardsByInviterID(ctx context.Context, inviterID int64) (float64, error) {
	client := clientFromContext(ctx, r.client)
	var result []struct {
		Sum float64 `json:"sum"`
	}
	err := client.ReferralRelation.Query().
		Where(referralrelation.InviterIDEQ(inviterID)).
		Aggregate(dbent.As(dbent.Sum(referralrelation.FieldInviterReward), "sum")).
		Scan(ctx, &result)
	if err != nil {
		return 0, err
	}
	if len(result) == 0 {
		return 0, nil
	}
	return result[0].Sum, nil
}

// =====================
// 管理员统计
// =====================

func (r *referralRepository) GetPlatformStats(ctx context.Context) (*service.ReferralStats, error) {
	client := clientFromContext(ctx, r.client)
	total, err := client.ReferralRelation.Query().Count(ctx)
	if err != nil {
		return nil, err
	}

	var inviterSums []struct {
		Sum float64 `json:"sum"`
	}
	if err := client.ReferralRelation.Query().
		Aggregate(dbent.As(dbent.Sum(referralrelation.FieldInviterReward), "sum")).
		Scan(ctx, &inviterSums); err != nil {
		return nil, err
	}

	var inviteeSums []struct {
		Sum float64 `json:"sum"`
	}
	if err := client.ReferralRelation.Query().
		Aggregate(dbent.As(dbent.Sum(referralrelation.FieldInviteeReward), "sum")).
		Scan(ctx, &inviteeSums); err != nil {
		return nil, err
	}

	stats := &service.ReferralStats{TotalRelations: int64(total)}
	if len(inviterSums) > 0 {
		stats.TotalInviterRewardGiven = inviterSums[0].Sum
	}
	if len(inviteeSums) > 0 {
		stats.TotalInviteeRewardGiven = inviteeSums[0].Sum
	}

	return stats, nil
}

// =====================
// 实体 → 服务模型映射
// =====================

func referralProfileEntityToService(m *dbent.UserReferralProfile) *service.UserReferralProfile {
	if m == nil {
		return nil
	}
	return &service.UserReferralProfile{
		ID:           m.ID,
		UserID:       m.UserID,
		ReferralCode: m.ReferralCode,
		CreatedAt:    m.CreatedAt,
	}
}

func referralRelationEntityToService(m *dbent.ReferralRelation) *service.ReferralRelation {
	if m == nil {
		return nil
	}
	rel := &service.ReferralRelation{
		ID:            m.ID,
		InviterID:     m.InviterID,
		InviteeID:     m.InviteeID,
		InviterReward: m.InviterReward,
		InviteeReward: m.InviteeReward,
		RewardGranted: m.RewardGranted,
		CreatedAt:     m.CreatedAt,
	}
	// 填充辅助字段（通过 WithInvitee() 预加载）
	if invitee := m.Edges.Invitee; invitee != nil {
		rel.InviteeEmail = invitee.Email
	}
	return rel
}
