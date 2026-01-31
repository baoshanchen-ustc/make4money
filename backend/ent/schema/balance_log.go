package schema

import (
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// BalanceLog holds the schema definition for the BalanceLog entity.
// 记录所有余额变动日志，用于审计追溯
// 该表只允许插入，不允许修改和删除（应用层控制）
type BalanceLog struct {
	ent.Schema
}

func (BalanceLog) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "balance_logs"},
	}
}

func (BalanceLog) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (BalanceLog) Fields() []ent.Field {
	return []ent.Field{
		// 用户ID
		field.Int64("user_id").
			Positive(),

		// 变动类型：recharge（充值）, consume（消费）, refund（退款）, adjust（调整）
		field.String("change_type").
			MaxLen(20).
			NotEmpty(),

		// 变动金额（正数表示增加，负数表示减少）
		field.Float("amount").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,2)"}),

		// 变动前余额
		field.Float("balance_before").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,2)"}),

		// 变动后余额
		field.Float("balance_after").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,2)"}),

		// 关联订单号（可选）
		field.String("related_order_no").
			MaxLen(50).
			Optional().
			Nillable(),

		// 变动描述
		field.String("description").
			SchemaType(map[string]string{dialect.Postgres: "text"}).
			Default(""),

		// 操作人ID（系统操作时为0）
		field.Int64("operator_id").
			Default(0),

		// 操作人类型：system（系统）, admin（管理员）, user（用户）
		field.String("operator_type").
			MaxLen(20).
			Default("system"),
	}
}

func (BalanceLog) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("balance_logs").
			Field("user_id").
			Required().
			Unique(),
	}
}

func (BalanceLog) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id"),
		index.Fields("change_type"),
		index.Fields("related_order_no"),
		index.Fields("created_at"),
		index.Fields("user_id", "change_type"),
	}
}
