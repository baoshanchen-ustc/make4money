-- 083_add_spans_indexes_notx.sql
-- Partial GIN indexes for spans JSONB columns.
-- Only index rows that actually have spans data to avoid bloat
-- from the majority of rows without span data.
-- Must run outside a transaction (CONCURRENTLY).
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ops_error_logs_spans_gin
  ON ops_error_logs USING gin (spans)
  WHERE spans IS NOT NULL;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_usage_logs_spans_gin
  ON usage_logs USING gin (spans)
  WHERE spans IS NOT NULL;
