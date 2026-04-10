-- 086_model_pricing_override_litellm.sql
-- 新增 override_litellm 字段：
-- 设为 true 时，该条目的数据库价格优先于 LiteLLM 动态价格生效，
-- 适用于免费模型或需要强制覆盖 LiteLLM 价格的场景。

ALTER TABLE model_pricings
    ADD COLUMN IF NOT EXISTS override_litellm BOOLEAN NOT NULL DEFAULT FALSE;
