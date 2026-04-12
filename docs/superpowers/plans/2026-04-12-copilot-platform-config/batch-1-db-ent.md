# Copilot 平台配置 — Batch 1: DB 迁移 + Ent Schema

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 新建 `copilot_platform_configs` 表并预插入 5 行，同时生成对应的 Ent schema。

**Architecture:** 新增迁移文件 `087_copilot_platform_configs.sql`，然后在 Ent schema 目录新增 `copilot_platform_config.go`，执行 `go generate` 生成 ORM 代码。

**Tech Stack:** PostgreSQL · Go · entgo.io/ent

**Spec:** `docs/superpowers/specs/2026-04-12-copilot-platform-config-design.md`

---

### Task 1: DB 迁移文件

**Files:**
- Create: `backend/migrations/087_copilot_platform_configs.sql`

- [ ] **Step 1: 创建迁移文件**

```sql
-- 087_copilot_platform_configs.sql
-- Copilot 平台级参数配置表
-- 按 plan_type 存储 max_output_tokens / max_body_kb / model_mapping / model_whitelist 的默认值
-- 账号级配置优先于此处配置，此处配置优先于系统默认

CREATE TABLE IF NOT EXISTS copilot_platform_configs (
    id                BIGSERIAL PRIMARY KEY,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    plan_type         VARCHAR(32) NOT NULL,
    -- 枚举值: individual_free / individual_pro / individual_pro_plus / business / enterprise

    max_output_tokens BIGINT,      -- NULL 表示不设默认
    max_body_kb       INTEGER,     -- NULL 表示不设默认
    model_mapping     JSONB,       -- {"from_model": "to_model", ...}，NULL 表示不设默认
    model_whitelist   JSONB        -- ["model-a", "model-b"]，NULL 表示不设默认
);

CREATE UNIQUE INDEX IF NOT EXISTS copilot_platform_configs_plan_type_unique_idx
    ON copilot_platform_configs (plan_type);

CREATE INDEX IF NOT EXISTS copilot_platform_configs_plan_type_idx
    ON copilot_platform_configs (plan_type);

-- 预插入 5 行（全字段 NULL），确保后端始终能查到记录
INSERT INTO copilot_platform_configs (plan_type) VALUES
    ('individual_free'),
    ('individual_pro'),
    ('individual_pro_plus'),
    ('business'),
    ('enterprise')
ON CONFLICT (plan_type) DO NOTHING;
```

- [ ] **Step 2: 确认迁移文件存在**

```bash
ls backend/migrations/087_copilot_platform_configs.sql
```

Expected: 文件存在，无错误输出。

- [ ] **Step 3: Commit**

```bash
git add backend/migrations/087_copilot_platform_configs.sql
git commit -m "Feature: 新增 copilot_platform_configs 迁移文件"
```

---

### Task 2: Ent Schema

**Files:**
- Create: `backend/ent/schema/copilot_platform_config.go`

背景：Ent schema 在 `backend/ent/schema/` 目录，参考 `model_pricing.go` 的写法。此表无软删除（不需要历史审计），只有 TimeMixin。

- [ ] **Step 1: 创建 schema 文件**

```go
// backend/ent/schema/copilot_platform_config.go
package schema

import (
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// CopilotPlatformConfig holds the schema definition for Copilot platform-level defaults.
//
// 存储按 plan_type 分组的平台级默认参数。
// 账号级配置（credentials 字段）优先；账号未设置时继承此处配置；两者都没有时使用系统默认。
type CopilotPlatformConfig struct {
	ent.Schema
}

func (CopilotPlatformConfig) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "copilot_platform_configs"},
	}
}

func (CopilotPlatformConfig) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (CopilotPlatformConfig) Fields() []ent.Field {
	return []ent.Field{
		// 枚举值: individual_free / individual_pro / individual_pro_plus / business / enterprise
		field.String("plan_type").
			MaxLen(32).
			NotEmpty().
			Unique().
			Comment("Copilot plan type: individual_free / individual_pro / individual_pro_plus / business / enterprise"),

		// NULL 表示该 plan_type 不设默认值
		field.Int64("max_output_tokens").
			Optional().
			Nillable().
			Comment("Max output tokens for this plan type; NULL = use system default"),

		field.Int("max_body_kb").
			Optional().
			Nillable().
			Comment("Max request body size in KB for this plan type; NULL = use system default"),

		// JSONB 存储为 map[string]string
		field.JSON("model_mapping", map[string]string{}).
			Optional().
			Comment("Model name rewriting map {from: to}; NULL = no mapping default"),

		// JSONB 存储为 []string
		field.JSON("model_whitelist", []string{}).
			Optional().
			Comment("Allowed model list; NULL = allow all (no whitelist default)"),
	}
}

func (CopilotPlatformConfig) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("plan_type").Unique(),
	}
}
```

- [ ] **Step 2: 运行 Ent 代码生成**

```bash
cd backend && go generate ./ent/...
```

Expected: 无错误，生成 `backend/ent/copilotplatformconfig/` 目录和相关文件。

- [ ] **Step 3: 验证生成文件存在**

```bash
ls backend/ent/copilotplatformconfig/
```

Expected: 列出 `copilotplatformconfig.go`、`where.go` 等文件。

- [ ] **Step 4: 确认编译通过**

```bash
cd backend && go build ./...
```

Expected: 无编译错误。

- [ ] **Step 5: Commit**

```bash
git add backend/ent/schema/copilot_platform_config.go backend/ent/
git commit -m "Feature: 新增 CopilotPlatformConfig Ent schema 并生成 ORM 代码"
```
