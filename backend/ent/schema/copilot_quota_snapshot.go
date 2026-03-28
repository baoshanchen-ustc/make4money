// Package schema 定义 Ent ORM 的数据库 schema。
package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// CopilotQuotaSnapshot 记录每次实时查询 GitHub Copilot 配额的结果快照。
//
// 每次查询后 UPSERT 当天记录，用于趋势图的历史数据。
// 同一账户同一天的多次查询会覆盖当天记录（保留最新状态）。
type CopilotQuotaSnapshot struct {
	ent.Schema
}

// Annotations 返回 schema 的注解配置。
func (CopilotQuotaSnapshot) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "copilot_quota_snapshots"},
	}
}

// Fields 定义配额快照实体的所有字段。
func (CopilotQuotaSnapshot) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("account_id"),
		// snapshot_date: 快照日期（每天最后一次查询覆盖当天记录）
		field.Time("snapshot_date").
			SchemaType(map[string]string{dialect.Postgres: "date"}),
		// plan_type: 快照时的套餐类型（from GitHub API），如 "individual_pro_plus"
		field.String("plan_type").
			MaxLen(30).
			Optional().
			Nillable(),
		field.Int("premium_entitlement").Default(0), // 月配额总量
		field.Int("premium_remaining").Default(0),   // 当前剩余量
		field.Int("premium_used").Default(0),         // 已使用量（全渠道，entitlement - remaining + overage）
		field.Int("premium_overage").Default(0),      // 超额使用量
		field.Bool("unlimited").Default(false),
		field.Time("created_at").
			Default(time.Now).
			Immutable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

// Indexes 定义数据库索引。
func (CopilotQuotaSnapshot) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("account_id", "snapshot_date").Unique(),
		index.Fields("snapshot_date"),
		index.Fields("account_id"),
	}
}
