-- 分组级展示与人设增强
--
-- display_icon            : 自定义展示图标 key（覆盖默认 platform 图标），来自前后端共享白名单
-- display_name            : 自定义展示名称（覆盖默认 platform 名称）
-- display_rate_multiplier : 外显倍率（仅 UI 展示，实际计费仍走 rate_multiplier）；NULL 表示与真实倍率一致
-- claude_code_persona     : 是否在转发请求时强制注入 Claude Code 人设系统提示词

ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS display_icon            VARCHAR(32),
    ADD COLUMN IF NOT EXISTS display_name            VARCHAR(50),
    ADD COLUMN IF NOT EXISTS display_rate_multiplier DECIMAL(10,4),
    ADD COLUMN IF NOT EXISTS claude_code_persona     BOOLEAN NOT NULL DEFAULT FALSE;

COMMENT ON COLUMN groups.display_icon            IS '自定义展示图标 key（白名单受控）';
COMMENT ON COLUMN groups.display_name            IS '自定义展示名称（覆盖默认 platform 名称）';
COMMENT ON COLUMN groups.display_rate_multiplier IS '外显倍率（仅 UI 展示，NULL 表示与 rate_multiplier 一致）';
COMMENT ON COLUMN groups.claude_code_persona     IS '是否注入 Claude Code 人设系统提示词';
