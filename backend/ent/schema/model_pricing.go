package schema

import (
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// ModelPricing holds the schema definition for model pricing overrides.
//
// 删除策略：软删除
// 用于存储管理员自定义的模型计费价格，优先级高于 LiteLLM 动态价格。
// 软删除保留历史记录便于审计。
type ModelPricing struct {
	ent.Schema
}

func (ModelPricing) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "model_pricings"},
	}
}

func (ModelPricing) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
		mixins.SoftDeleteMixin{},
	}
}

func (ModelPricing) Fields() []ent.Field {
	return []ent.Field{
		// 模型标识符（fallback key，如 "gpt-5.1"、"claude-opus-4.5"）
		field.String("model_key").
			MaxLen(200).
			NotEmpty().
			Comment("Fallback pricing key, e.g. 'gpt-5.1' or 'claude-opus-4.5'"),

		// 展示用的模型名称（可读性更好）
		field.String("display_name").
			MaxLen(200).
			Optional().
			Comment("Human-readable display name for the model"),

		// 每百万输入 token 价格（USD），存储方便展示，内部转为 per-token
		field.Float("input_price_per_million").
			Default(0).
			Comment("Input price per 1M tokens (USD)"),

		field.Float("output_price_per_million").
			Default(0).
			Comment("Output price per 1M tokens (USD)"),

		field.Float("input_price_per_million_priority").
			Default(0).
			Comment("Input price per 1M tokens for priority tier (USD), 0 = use multiplier"),

		field.Float("output_price_per_million_priority").
			Default(0).
			Comment("Output price per 1M tokens for priority tier (USD), 0 = use multiplier"),

		field.Float("cache_read_price_per_million").
			Default(0).
			Comment("Cache read price per 1M tokens (USD)"),

		field.Float("cache_read_price_per_million_priority").
			Default(0).
			Comment("Cache read price per 1M tokens for priority tier (USD)"),

		field.Float("cache_creation_price_per_million").
			Default(0).
			Comment("Cache creation price per 1M tokens (USD)"),

		// 是否启用（false 表示禁用，fallback 会跳过该条目）
		field.Bool("enabled").
			Default(true),

		// override_litellm：设为 true 时，数据库价格优先于 LiteLLM 动态价格生效
		field.Bool("override_litellm").
			Default(false).
			Comment("When true, this DB price takes priority over LiteLLM dynamic pricing"),

		// 备注
		field.String("note").
			MaxLen(500).
			Optional().
			Comment("Admin notes for this pricing entry"),
	}
}

func (ModelPricing) Indexes() []ent.Index {
	return []ent.Index{
		// active 记录的 model_key 唯一性由迁移 SQL 中的 partial unique index 保证：
		// CREATE UNIQUE INDEX ... ON model_pricings (model_key) WHERE deleted_at IS NULL
		// Ent 不支持 partial index，不在此定义复合唯一约束，避免引入不生效的 NULL 约束。
		index.Fields("enabled"),
	}
}
