// Package schema 定义 Ent ORM 的数据库 schema。
package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"
)

// CopilotBudgetAlert 存储每个 Copilot 账户的预算告警配置。
//
// 每个账户最多一条记录（account_id UNIQUE）。
// 告警在每次实时配额查询时检查，不依赖定时任务。
type CopilotBudgetAlert struct {
	ent.Schema
}

// Annotations 返回 schema 的注解配置。
func (CopilotBudgetAlert) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "copilot_budget_alerts"},
	}
}

// Mixin 返回该 schema 使用的混入组件。
func (CopilotBudgetAlert) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

// Fields 定义预算告警实体的所有字段。
func (CopilotBudgetAlert) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("account_id").Unique(),
		// monthly_budget: 月预算上限（美元），0 表示不限制
		field.Float("monthly_budget").
			Default(0).
			SchemaType(map[string]string{dialect.Postgres: "decimal(10,2)"}),
		// alert_threshold: 配额使用率告警阈值（百分比，默认 80 表示 80%）
		field.Int("alert_threshold").Default(80),
		field.Bool("enabled").Default(true),
		// last_alerted_at: 最近一次触发告警的时间，用于去重（同一天只告警一次）
		field.Time("last_alerted_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

// Indexes 定义数据库索引。
func (CopilotBudgetAlert) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("account_id"),
		index.Fields("enabled"),
	}
}
