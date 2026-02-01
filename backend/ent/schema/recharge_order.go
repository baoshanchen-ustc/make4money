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

// RechargeOrder holds the schema definition for the RechargeOrder entity.
type RechargeOrder struct {
	ent.Schema
}

func (RechargeOrder) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "recharge_orders"},
	}
}

func (RechargeOrder) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (RechargeOrder) Fields() []ent.Field {
	return []ent.Field{
		// 订单号：RECH + 年月日时分秒 + 10位随机字符串
		field.String("order_no").
			MaxLen(50).
			NotEmpty().
			Unique(),

		// 用户ID
		field.Int64("user_id").
			Positive(),

		// 充值金额
		field.Float("amount").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,2)"}).
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

		// 备注
		field.String("notes").
			SchemaType(map[string]string{dialect.Postgres: "text"}).
			Default(""),

		// 退款相关字段
		// 退款单号
		field.String("refund_no").
			MaxLen(50).
			Optional().
			Nillable().
			Comment("退款单号"),

		// 退款状态: pending/processing/success/failed
		field.String("refund_status").
			MaxLen(20).
			Optional().
			Nillable().
			Comment("退款状态"),

		// 退款时间
		field.Time("refunded_at").
			SchemaType(map[string]string{
				dialect.Postgres: "timestamptz",
			}).
			Optional().
			Nillable().
			Comment("退款时间"),

		// 退款原因
		field.String("refund_reason").
			SchemaType(map[string]string{dialect.Postgres: "text"}).
			Optional().
			Nillable().
			Comment("退款原因"),

		// 退款操作人ID
		field.Int64("refund_admin_id").
			Optional().
			Nillable().
			Comment("退款操作人ID"),

		// 微信退款单号
		field.String("wechat_refund_id").
			MaxLen(64).
			Optional().
			Nillable().
			Comment("微信退款单号"),
	}
}

func (RechargeOrder) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("recharge_orders").
			Field("user_id").
			Required().
			Unique(),
	}
}

func (RechargeOrder) Indexes() []ent.Index {
	return []ent.Index{
		// 订单号唯一索引（在 Fields 中已声明 Unique()）
		index.Fields("user_id"),
		index.Fields("status"),
		index.Fields("expire_at"),
		index.Fields("user_id", "status"),
	}
}
