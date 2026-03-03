package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// UserReferralProfile holds the schema definition for the UserReferralProfile entity.
//
// 每个用户的专属邀请码 Profile，在用户注册时自动创建（懒加载）。
// 与 User 表一对一关系，通过独立表实现，不修改现有 users 表。
//
// 删除策略：硬删除（与 RedeemCode 相同，邀请码 Profile 删除无需保留历史）
type UserReferralProfile struct {
	ent.Schema
}

func (UserReferralProfile) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "user_referral_profiles"},
	}
}

func (UserReferralProfile) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id").
			Positive(),
		field.String("referral_code").
			MaxLen(8).
			NotEmpty().
			Unique().
			Comment("8位大写字母+数字的专属邀请码"),
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			SchemaType(map[string]string{"postgres": "timestamptz"}),
	}
}

func (UserReferralProfile) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("referral_profile").
			Field("user_id").
			Unique().
			Required(),
	}
}

func (UserReferralProfile) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id").Unique(),
		index.Fields("referral_code").Unique(),
	}
}
