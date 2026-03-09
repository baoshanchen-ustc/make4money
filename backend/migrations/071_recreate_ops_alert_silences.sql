-- Recreate ops_alert_silences table.
--
-- Migration 037 used goose-style annotations (-- +goose Up / -- +goose Down),
-- but the project's migration runner does not parse goose directives.
-- It executed the entire file as plain SQL, including the Down section
-- (DROP TABLE IF EXISTS ops_alert_silences), which immediately destroyed
-- the table that the Up section had just created.
--
-- This migration recreates the table using IF NOT EXISTS for idempotency.

CREATE TABLE IF NOT EXISTS ops_alert_silences (
    id BIGSERIAL PRIMARY KEY,

    rule_id BIGINT NOT NULL,
    platform VARCHAR(64) NOT NULL,
    group_id BIGINT,
    region VARCHAR(64),

    until TIMESTAMPTZ NOT NULL,
    reason TEXT,

    created_by BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ops_alert_silences_lookup
    ON ops_alert_silences (rule_id, platform, group_id, region, until);
