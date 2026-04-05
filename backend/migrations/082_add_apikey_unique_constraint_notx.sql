-- 082_add_apikey_unique_constraint.sql
-- Add a partial unique index on accounts to prevent duplicate API key entries.
--
-- Uniqueness is defined as: (platform, api_key, normalised base_url) must be
-- unique among non-deleted apikey-type accounts.
--
-- base_url is normalised by stripping the trailing slash so that
-- "https://api.openai.com" and "https://api.openai.com/" are treated as equal.
-- Empty base_url is stored as '' and compared as ''.
--
-- The index is CONCURRENT (no-transaction) to avoid locking a busy table.
-- It uses IF NOT EXISTS for idempotency.

CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS accounts_apikey_unique_active
    ON accounts (
        platform,
        (credentials->>'api_key'),
        (COALESCE(NULLIF(TRIM(TRAILING '/' FROM credentials->>'base_url'), ''), ''))
    )
    WHERE type = 'apikey'
      AND deleted_at IS NULL;
