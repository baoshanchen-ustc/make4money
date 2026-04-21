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

type PasskeyCredential struct {
	ent.Schema
}

func (PasskeyCredential) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "passkey_credentials"},
	}
}

func (PasskeyCredential) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (PasskeyCredential) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.String("credential_id").
			MaxLen(512).
			NotEmpty().
			Unique(),
		field.String("public_key").
			NotEmpty().
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Int64("sign_count").
			Default(0),
		field.JSON("transports", []string{}).
			Optional(),
		field.String("aaguid").
			MaxLen(64).
			Default(""),
		field.Bool("backup_eligible").
			Default(false),
		field.Bool("backup_state").
			Default(false),
		field.String("friendly_name").
			MaxLen(100).
			Default(""),
		field.Time("last_used_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("revoked_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (PasskeyCredential) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("passkey_credentials").
			Field("user_id").
			Unique().
			Required(),
	}
}

func (PasskeyCredential) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id"),
		index.Fields("user_id", "revoked_at"),
	}
}
