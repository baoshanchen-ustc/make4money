-- Creates storage for scheduler outbox watermark checkpoints so Redis failures have a fallback.
CREATE TABLE IF NOT EXISTS scheduler_outbox_watermarks (
  name TEXT PRIMARY KEY,
  watermark BIGINT NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
