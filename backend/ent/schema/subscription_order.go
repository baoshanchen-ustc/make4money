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

// SubscriptionOrder holds the schema definition for the SubscriptionOrder entity.
type SubscriptionOrder struct {
	ent.Schema
}

func (SubscriptionOrder) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "subscription_orders"},
	}
}

func (SubscriptionOrder) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (SubscriptionOrder) Fields() []ent.Field {
	return []ent.Field{
		// 订单号：SUBS + 年月日时分秒 + 10位随机字符串
		field.String("order_no").
			MaxLen(50).
			NotEmpty().
			Unique(),

		// 用户ID
		field.Int64("user_id").
			Positive(),

		// 分组（套餐）ID
		field.Int64("group_id").
			Positive(),

		// 订单金额（人民币）
		field.Float("amount").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,2)"}).
			Positive(),

		// 有效期天数
		field.Int("validity_days").
			Default(30).
			Positive(),

		// 支付方式：wechat_pay, alipay
		field.String("payment_method").
			MaxLen(20).
			NotEmpty(),

		// 支付渠道：native（扫码支付）, jsapi（公众号支付）, h5（H5支付）
		field.String("payment_channel").
			MaxLen(20).
			Default("native"),

		// 订单状态：pending（待支付）, paid（已支付）, failed（支付失败）, expired（已过期）, cancelled（已取消）
		field.String("status").
			MaxLen(20).
			Default("pending"),

		// 微信支付订单号（支付成功后填充）
		field.String("wechat_transaction_id").
			MaxLen(64).
			Optional().
			Nillable(),

		// 支付二维码URL（Native支付时填充）
		field.String("qrcode_url").
			SchemaType(map[string]string{dialect.Postgres: "text"}).
			Optional().
			Nillable(),

		// 预支付交易会话标识（JSAPI支付时填充）
		field.String("prepay_id").
			MaxLen(64).
			Optional().
			Nillable(),

		// 订单过期时间
		field.Time("expire_at").
			SchemaType(map[string]string{
				dialect.Postgres: "timestamptz",
			}),

		// 支付完成时间
		field.Time("paid_at").
			SchemaType(map[string]string{
				dialect.Postgres: "timestamptz",
			}).
			Optional().
			Nillable(),
	}
}

func (SubscriptionOrder) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("subscription_orders").
			Field("user_id").
			Required().
			Unique(),
		edge.From("group", Group.Type).
			Ref("subscription_orders").
			Field("group_id").
			Required().
			Unique(),
	}
}

func (SubscriptionOrder) Indexes() []ent.Index {
	return []ent.Index{
		// 订单号唯一索引（在 Fields 中已声明 Unique()）
		index.Fields("user_id"),
		index.Fields("group_id"),
		index.Fields("status"),
		index.Fields("expire_at"),
		index.Fields("user_id", "status"),
	}
}
