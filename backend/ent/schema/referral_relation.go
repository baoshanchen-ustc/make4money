package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// ReferralRelation holds the schema definition for the ReferralRelation entity.
//
// 用户邀请关系记录：记录邀请人(inviter)与被邀请人(invitee)的关联，
// 以及双方获得的奖励金额快照和奖励发放状态。
//
// 关键约束：invitee_id 唯一，确保同一用户只能被邀请一次（幂等性保障）。
//
// 删除策略：硬删除（关系一旦建立不应更改，如需审计请在删除前记录日志）
type ReferralRelation struct {
	ent.Schema
}

func (ReferralRelation) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "referral_relations"},
	}
}

func (ReferralRelation) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("inviter_id").
			Positive().
			Comment("邀请人的 user_id"),
		field.Int64("invitee_id").
			Positive().
			Comment("被邀请人的 user_id，唯一约束确保一人只能被邀请一次"),
		field.Float("inviter_reward").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Default(0).
			Comment("发放给邀请人的余额奖励（创建时快照，不受后续配置变更影响）"),
		field.Float("invitee_reward").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Default(0).
			Comment("发放给被邀请人的余额奖励（创建时快照）"),
		field.Bool("reward_granted").
			Default(false).
			Comment("奖励是否已发放，防止重复发放（幂等标志）"),
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (ReferralRelation) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("inviter", User.Type).
			Ref("referrals_given").
			Field("inviter_id").
			Unique().
			Required(),
		edge.From("invitee", User.Type).
			Ref("referral_received").
			Field("invitee_id").
			Unique().
			Required(),
	}
}

func (ReferralRelation) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("invitee_id").Unique(), // 幂等约束：同一被邀请人只能出现一次
		index.Fields("inviter_id"),          // 按邀请人查询列表（分页高频）
	}
}
