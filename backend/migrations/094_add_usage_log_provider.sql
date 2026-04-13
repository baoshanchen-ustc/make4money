-- Align usage_logs schema with runtime observability fields.
-- provider captures the selected upstream provider snapshot, typically account.Platform.

ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS provider VARCHAR(50);

CREATE INDEX IF NOT EXISTS idx_usage_logs_provider_created_at
    ON usage_logs (provider, created_at);
