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

// UserCheckIn holds the schema definition for the UserCheckIn entity.
type UserCheckIn struct {
	ent.Schema
}

func (UserCheckIn) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "user_checkins"},
	}
}

func (UserCheckIn) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id").
			Comment("签到用户 ID"),
		field.Time("checkin_date").
			SchemaType(map[string]string{dialect.Postgres: "date"}).
			Comment("业务签到日期"),
		field.Float("reward_amount").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Default(0).
			Comment("签到奖励快照"),
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}).
			Comment("签到时间"),
	}
}

func (UserCheckIn) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("checkins").
			Field("user_id").
			Required().
			Unique(),
	}
}

func (UserCheckIn) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "checkin_date").Unique(),
		index.Fields("checkin_date"),
	}
}
