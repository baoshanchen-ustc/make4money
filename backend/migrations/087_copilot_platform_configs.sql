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
