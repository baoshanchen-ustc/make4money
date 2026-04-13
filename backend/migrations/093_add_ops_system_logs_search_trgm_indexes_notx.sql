CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ops_system_logs_message_trgm
    ON ops_system_logs USING gin (message gin_trgm_ops);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ops_system_logs_request_id_trgm
    ON ops_system_logs USING gin (request_id gin_trgm_ops);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ops_system_logs_client_request_id_trgm
    ON ops_system_logs USING gin (client_request_id gin_trgm_ops);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ops_system_logs_extra_text_trgm
    ON ops_system_logs USING gin ((COALESCE(extra::text, '')) gin_trgm_ops)
    WHERE extra <> '{}'::jsonb;
