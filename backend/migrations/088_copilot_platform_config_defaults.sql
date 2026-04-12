-- 088_copilot_platform_config_defaults.sql
-- 为 copilot_platform_configs 表设置各套餐默认值：
--   max_output_tokens = 0（不限制）
--   model_whitelist   按套餐预置（free/pro/pro+ 使用 pro+ 模型列表，business/enterprise 使用 business 模型列表）
--
-- 使用 UPDATE ... WHERE max_output_tokens IS NULL 保证幂等：
-- 已被管理员手动修改过的行不会被覆盖。

-- individual_free / individual_pro / individual_pro_plus：pro+ 账户可用模型（44 个）
UPDATE copilot_platform_configs
SET
    max_output_tokens = 0,
    model_whitelist   = '["claude-opus-4.6","claude-sonnet-4.6","claude-sonnet-4","claude-sonnet-4.5","claude-opus-4.5","claude-haiku-4.5","gemini-3.1-pro-preview","gemini-3-flash-preview","gemini-2.5-pro","goldeneye-free-auto","gpt-5.4","gpt-5.4-mini","gpt-5.3-codex","gpt-5.2-codex","gpt-5.2","gpt-5.1","gpt-5-mini","gpt-41-copilot","gpt-4.1","gpt-4.1-2025-04-14","gpt-4o","gpt-4o-2024-05-13","gpt-4o-2024-08-06","gpt-4o-2024-11-20","gpt-4o-mini","gpt-4o-mini-2024-07-18","gpt-4","gpt-4-0613","gpt-4-0125-preview","gpt-4-o-preview","gpt-3.5-turbo","gpt-3.5-turbo-0613","grok-code-fast-1","minimax-m2.5","oswe-vscode-prime","oswe-vscode-secondary","text-embedding-3-small","text-embedding-3-small-inference","text-embedding-ada-002","accounts/msft/routers/mp3yn0h7","accounts/msft/routers/yaqq2gxh","accounts/msft/routers/f185i3v4","accounts/msft/routers/fmfeto88","accounts/msft/routers/gdjv4v2v"]'
WHERE plan_type IN ('individual_free', 'individual_pro', 'individual_pro_plus')
  AND max_output_tokens IS NULL;

-- business / enterprise：business 账户可用模型（30 个）
UPDATE copilot_platform_configs
SET
    max_output_tokens = 0,
    model_whitelist   = '["claude-opus-4.6","claude-sonnet-4.6","gemini-3.1-pro-preview","gemini-3-flash-preview","gpt-5.4","gpt-5.4-mini","gpt-5.3-codex","gpt-5.2","gpt-41-copilot","gpt-4.1","gpt-4.1-2025-04-14","gpt-4o","gpt-4o-2024-05-13","gpt-4o-2024-08-06","gpt-4o-2024-11-20","gpt-4o-mini","gpt-4o-mini-2024-07-18","gpt-4","gpt-4-0613","gpt-4-0125-preview","gpt-4-o-preview","gpt-3.5-turbo","gpt-3.5-turbo-0613","grok-code-fast-1","text-embedding-3-small","text-embedding-3-small-inference","text-embedding-ada-002b","accounts/msft/routers/f185i3v4","accounts/msft/routers/fmfeto88","accounts/msft/routers/gdjv4v2v"]'
WHERE plan_type IN ('business', 'enterprise')
  AND max_output_tokens IS NULL;
