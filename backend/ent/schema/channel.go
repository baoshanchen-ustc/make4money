package schema

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Channel holds the schema definition for the Channel entity.
type Channel struct {
	ent.Schema
}

func (Channel) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "channels"},
	}
}

func (Channel) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			MaxLen(100).
			NotEmpty(),
		field.String("description").
			Optional().
			Default("").
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.String("status").
			MaxLen(20).
			Default(domain.StatusActive),
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			Annotations(entsql.Default("NOW()")).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now).
			Annotations(entsql.Default("NOW()")).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.JSON("model_mapping", map[string]map[string]string{}).
			Optional().
			Default(func() map[string]map[string]string { return map[string]map[string]string{} }).
			Annotations(entsql.Default("'{}'::jsonb")).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.String("billing_model_source").
			MaxLen(20).
			Optional().
			Default("channel_mapped"),
		field.Bool("restrict_models").
			Optional().
			Default(false),
		field.String("features").
			Default("").
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.JSON("features_config", map[string]any{}).
			Default(func() map[string]any { return map[string]any{} }).
			Annotations(entsql.Default("'{}'::jsonb")).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.Bool("apply_pricing_to_account_stats").
			Default(false),
	}
}

func (Channel) Edges() []ent.Edge {
	return []ent.Edge{}
}

func (Channel) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("name").
			Unique().
			StorageKey("idx_channels_name"),
		index.Fields("status").
			StorageKey("idx_channels_status"),
	}
}
