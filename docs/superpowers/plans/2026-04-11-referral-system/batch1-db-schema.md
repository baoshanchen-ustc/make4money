# Batch 1 — Ent Schema + SQL Migration

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans.

**Goal:** 新增 5 张表的 Ent schema 定义，并生成对应的 SQL 迁移文件。

**Prerequisites:** 无

---

### Task 1: Ent Schema — `referral_codes`

**Files:**
- Create: `backend/ent/schema/referral_code.go`

- [ ] **Step 1: 写 schema 文件**

```go
// backend/ent/schema/referral_code.go
package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"
)

// ReferralCode holds the schema definition for the ReferralCode entity.
// 每个用户对应唯一一条邀请码记录，注册时自动生成。
type ReferralCode struct {
	ent.Schema
}

func (ReferralCode) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "referral_codes"},
	}
}

func (ReferralCode) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (ReferralCode) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id").
			Unique(),
		field.String("code").
			MaxLen(32).
			NotEmpty().
			Unique(),
	}
}

func (ReferralCode) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("referral_code").
			Field("user_id").
			Unique().
			Required(),
	}
}

func (ReferralCode) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("code"),
	}
}
```

- [ ] **Step 2: 在 User schema 新增 edge（`backend/ent/schema/user.go`）**

在 `Edges()` 方法末尾追加：
```go
edge.To("referral_code", ReferralCode.Type),
```

最终 `Edges()` 末尾变为：
```go
    edge.To("promo_code_usages", PromoCodeUsage.Type),
    edge.To("referral_code", ReferralCode.Type),
```

---

### Task 2: Ent Schema — `referral_relations`

**Files:**
- Create: `backend/ent/schema/referral_relation.go`

- [ ] **Step 1: 写 schema 文件**

```go
// backend/ent/schema/referral_relation.go
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

// ReferralRelation 邀请关系表，存图结构 + 建立时策略快照。
// level=1: 直接邀请关系（参与返佣结算）
// level=2: 间接邀请关系（仅用于关系链报表，不结算返佣）
type ReferralRelation struct {
	ent.Schema
}

func (ReferralRelation) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "referral_relations"},
	}
}

func (ReferralRelation) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (ReferralRelation) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("inviter_id"),
		field.Int64("invitee_id"),
		field.Int8("level"),
		field.String("inviter_role_snapshot").
			MaxLen(20),
		field.String("status").
			MaxLen(20).
			Default("active"),
		// 注册奖励快照
		field.Float("signup_bonus_inviter").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Default(0),
		field.Float("signup_bonus_invitee").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Default(0),
		// 返佣比例快照
		field.Float("commission_rate").
			SchemaType(map[string]string{dialect.Postgres: "decimal(8,4)"}).
			Default(0),
	}
}

func (ReferralRelation) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("inviter_id"),
		index.Fields("invitee_id"),
		index.Fields("status"),
		// 唯一约束在 SQL migration 中通过 UNIQUE(inviter_id, invitee_id, level) 实现
	}
}
```

---

### Task 3: Ent Schema — `referral_signup_reward_events`

**Files:**
- Create: `backend/ent/schema/referral_signup_reward_event.go`

- [ ] **Step 1: 写 schema 文件**

```go
// backend/ent/schema/referral_signup_reward_event.go
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

// ReferralSignupRewardEvent 注册奖励事件表（outbox 模式）。
// 每次成功发放的注册奖励记录一条，支持重试和对账。
type ReferralSignupRewardEvent struct {
	ent.Schema
}

func (ReferralSignupRewardEvent) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "referral_signup_reward_events"},
	}
}

func (ReferralSignupRewardEvent) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (ReferralSignupRewardEvent) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("relation_id"),
		field.Int64("beneficiary_id"),
		field.String("reward_type").
			MaxLen(20), // inviter_bonus / invitee_bonus
		field.Float("amount").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}),
		field.String("status").
			MaxLen(20).
			Default("pending"), // pending / settled / failed
		field.Time("settled_at").
			Optional().
			Nillable(),
		field.Text("error_msg").
			Optional().
			Nillable(),
		field.Int("retry_count").
			Default(0),
	}
}

func (ReferralSignupRewardEvent) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("status"),
		index.Fields("beneficiary_id"),
		// UNIQUE(relation_id, reward_type) 在 SQL migration 中实现
	}
}
```

---

### Task 4: Ent Schema — `referral_commission_events`

**Files:**
- Create: `backend/ent/schema/referral_commission_event.go`

- [ ] **Step 1: 写 schema 文件**

```go
// backend/ent/schema/referral_commission_event.go
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

// ReferralCommissionEvent 消费返佣事件表（outbox 模式）。
// 幂等键为 (usage_request_id, beneficiary_id)。
type ReferralCommissionEvent struct {
	ent.Schema
}

func (ReferralCommissionEvent) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "referral_commission_events"},
	}
}

func (ReferralCommissionEvent) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (ReferralCommissionEvent) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("relation_id"),
		field.String("usage_request_id").
			MaxLen(128),
		field.Int64("beneficiary_id"),
		field.Int64("source_user_id"),
		field.Float("spend_amount").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}),
		field.Float("commission_rate").
			SchemaType(map[string]string{dialect.Postgres: "decimal(8,4)"}),
		field.Float("commission_amount").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}),
		field.String("status").
			MaxLen(20).
			Default("pending"), // pending / settled / failed
		field.Time("settled_at").
			Optional().
			Nillable(),
		field.Text("error_msg").
			Optional().
			Nillable(),
		field.Int("retry_count").
			Default(0),
	}
}

func (ReferralCommissionEvent) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("status"),
		index.Fields("beneficiary_id"),
		index.Fields("relation_id"),
		// UNIQUE(usage_request_id, beneficiary_id) 在 SQL migration 中实现
	}
}
```

---

### Task 5: Ent Schema — `distributor_applications`

**Files:**
- Create: `backend/ent/schema/distributor_application.go`

- [ ] **Step 1: 写 schema 文件**

```go
// backend/ent/schema/distributor_application.go
package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"
)

// DistributorApplication 分销员申请表。
type DistributorApplication struct {
	ent.Schema
}

func (DistributorApplication) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "distributor_applications"},
	}
}

func (DistributorApplication) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (DistributorApplication) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.String("status").
			MaxLen(20).
			Default("pending"), // pending / approved / rejected
		field.Text("reason").
			Optional().
			Nillable(),
		field.Text("admin_notes").
			Optional().
			Nillable(),
		field.Int64("reviewed_by").
			Optional().
			Nillable(),
		field.Time("reviewed_at").
			Optional().
			Nillable(),
	}
}

func (DistributorApplication) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id"),
		index.Fields("status"),
	}
}
```

---

### Task 6: SQL Migration 文件

**Files:**
- Create: `backend/migrations/087_referral_system.sql`

- [ ] **Step 1: 写 SQL 迁移文件**

```sql
-- 087_referral_system.sql
-- 分销邀请系统：5 张新表

-- 1. 用户专属邀请码
CREATE TABLE referral_codes (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL UNIQUE,
    code        VARCHAR(32) NOT NULL UNIQUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_referral_codes_code ON referral_codes (code);

-- 2. 邀请关系表（图结构 + 建立时策略快照）
CREATE TABLE referral_relations (
    id                    BIGSERIAL PRIMARY KEY,
    inviter_id            BIGINT NOT NULL,
    invitee_id            BIGINT NOT NULL,
    level                 SMALLINT NOT NULL,
    inviter_role_snapshot VARCHAR(20) NOT NULL,
    status                VARCHAR(20) NOT NULL DEFAULT 'active',
    signup_bonus_inviter  DECIMAL(20,8) NOT NULL DEFAULT 0,
    signup_bonus_invitee  DECIMAL(20,8) NOT NULL DEFAULT 0,
    commission_rate       DECIMAL(8,4)  NOT NULL DEFAULT 0,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_referral_relations_inviter_invitee_level
        UNIQUE (inviter_id, invitee_id, level)
);

CREATE INDEX idx_referral_relations_inviter ON referral_relations (inviter_id);
CREATE INDEX idx_referral_relations_invitee ON referral_relations (invitee_id);
CREATE INDEX idx_referral_relations_status  ON referral_relations (status);

-- 3. 注册奖励事件表（outbox）
CREATE TABLE referral_signup_reward_events (
    id              BIGSERIAL PRIMARY KEY,
    relation_id     BIGINT NOT NULL,
    beneficiary_id  BIGINT NOT NULL,
    reward_type     VARCHAR(20) NOT NULL,
    amount          DECIMAL(20,8) NOT NULL,
    status          VARCHAR(20) NOT NULL DEFAULT 'pending',
    settled_at      TIMESTAMPTZ,
    error_msg       TEXT,
    retry_count     INT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_signup_reward_relation_type
        UNIQUE (relation_id, reward_type)
);

CREATE INDEX idx_signup_reward_status      ON referral_signup_reward_events (status);
CREATE INDEX idx_signup_reward_beneficiary ON referral_signup_reward_events (beneficiary_id);

-- 4. 消费返佣事件表（outbox）
CREATE TABLE referral_commission_events (
    id                BIGSERIAL PRIMARY KEY,
    relation_id       BIGINT NOT NULL,
    usage_request_id  VARCHAR(128) NOT NULL,
    beneficiary_id    BIGINT NOT NULL,
    source_user_id    BIGINT NOT NULL,
    spend_amount      DECIMAL(20,8) NOT NULL,
    commission_rate   DECIMAL(8,4)  NOT NULL,
    commission_amount DECIMAL(20,8) NOT NULL,
    status            VARCHAR(20) NOT NULL DEFAULT 'pending',
    settled_at        TIMESTAMPTZ,
    error_msg         TEXT,
    retry_count       INT NOT NULL DEFAULT 0,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_commission_request_beneficiary
        UNIQUE (usage_request_id, beneficiary_id)
);

CREATE INDEX idx_commission_events_status      ON referral_commission_events (status);
CREATE INDEX idx_commission_events_beneficiary ON referral_commission_events (beneficiary_id);
CREATE INDEX idx_commission_events_relation    ON referral_commission_events (relation_id);

-- 5. 分销员申请表
CREATE TABLE distributor_applications (
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT NOT NULL,
    status       VARCHAR(20) NOT NULL DEFAULT 'pending',
    reason       TEXT,
    admin_notes  TEXT,
    reviewed_by  BIGINT,
    reviewed_at  TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_distributor_applications_user_id ON distributor_applications (user_id);
CREATE INDEX idx_distributor_applications_status  ON distributor_applications (status);
```

- [ ] **Step 2: 生成 Ent 代码**

```bash
cd backend
go generate ./ent/...
```

Expected: ent 目录下生成 `referralcode/`, `referralrelation/`, `referralsignuprewardevent/`, `referralcommissionevent/`, `distributorapplication/` 等目录，以及对应的 CRUD client。

- [ ] **Step 3: 验证编译**

```bash
cd backend
go build ./...
```

Expected: 无编译错误。

- [ ] **Step 4: Commit**

```bash
git add backend/ent/schema/ backend/migrations/087_referral_system.sql
git commit -m "Feature: 新增分销系统 Ent schema 和 SQL 迁移"
```
