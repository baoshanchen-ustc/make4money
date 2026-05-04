// Package schema defines the ent ORM database schemas.
package schema

import (
	"time"

	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// UserAccountBinding defines the schema for long-term user-to-account bindings.
//
// This table implements P0-2 of the account-sharing hardening plan:
// mapping (downstream_user, project_fingerprint) to a single account for 7-30 days
// to improve upstream prompt cache hit rates and reduce account churn signals.
type UserAccountBinding struct {
	ent.Schema
}

// Annotations returns schema annotations.
func (UserAccountBinding) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "user_account_bindings"},
	}
}

// Mixin returns the mixins used by this schema.
func (UserAccountBinding) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

// Fields defines the fields for the UserAccountBinding entity.
func (UserAccountBinding) Fields() []ent.Field {
	return []ent.Field{
		// project_fp: SHA256 hash (truncated to 32 chars) of device_id or fallback key.
		// Format: sha256(device_id + ":" + group_id) or sha256("ip:" + api_key_id + ":" + ip_/24 + ":" + group_id)
		field.String("project_fp").
			MaxLen(64).
			NotEmpty(),

		// account_id: The bound upstream account ID.
		field.Int64("account_id"),

		// group_id: The group scope for this binding (0 = global/no group).
		field.Int64("group_id").
			Default(0),

		// expires_at: When this binding expires and should be cleaned up.
		field.Time("expires_at").
			Default(func() time.Time { return time.Now().Add(14 * 24 * time.Hour) }).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

// Indexes defines the database indexes for optimized queries.
func (UserAccountBinding) Indexes() []ent.Index {
	return []ent.Index{
		// Primary lookup: find binding by project fingerprint + group
		index.Fields("project_fp", "group_id").Unique(),
		// Invalidation: delete all bindings when an account is banned/disabled
		index.Fields("account_id"),
		// Cleanup: find expired bindings for periodic deletion
		index.Fields("expires_at"),
	}
}
