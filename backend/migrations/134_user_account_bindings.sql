-- Migration: 134_user_account_bindings
-- P0-2: Long-term user-to-account bindings for sticky session hardening.
--
-- Maps (project_fingerprint, group_id) to a single account for 7-30 days.
-- Goals:
--   1. Improve upstream prompt cache hit rates (same user → same account)
--   2. Reduce account churn signals visible to Anthropic
--   3. Provide stable workspace/cwd fingerprint per user
--
-- Design notes:
--   - project_fp is SHA256(device_id + ":" + group_id) for stable clients,
--     or SHA256("ip:" + api_key_id + ":" + ip_/24 + ":" + group_id) as fallback.
--   - group_id defaults to 0 (global scope) when no group is specified.
--   - expires_at defaults to 14 days from creation; refreshed on each access.
--   - ON DELETE CASCADE on account_id not used (soft-delete); cleanup via expires_at.

CREATE TABLE IF NOT EXISTS user_account_bindings (
    id          BIGSERIAL    PRIMARY KEY,
    project_fp  VARCHAR(64)  NOT NULL,
    account_id  BIGINT       NOT NULL,
    group_id    BIGINT       NOT NULL DEFAULT 0,
    expires_at  TIMESTAMPTZ  NOT NULL,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_user_account_bindings_fp_group UNIQUE (project_fp, group_id)
);

-- Index for invalidation: delete all bindings when account is banned/disabled.
CREATE INDEX IF NOT EXISTS idx_uab_account_id ON user_account_bindings(account_id);

-- Index for cleanup: find expired bindings for periodic deletion.
CREATE INDEX IF NOT EXISTS idx_uab_expires_at ON user_account_bindings(expires_at);
