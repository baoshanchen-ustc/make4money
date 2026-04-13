CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ops_system_logs_resolved_request_created_at
    ON ops_system_logs (
        (COALESCE(request_id, extra->>'request_id', extra->'last'->>'request_id', '')),
        created_at DESC
    );

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ops_system_logs_component_extra_api_key_created_at
    ON ops_system_logs (
        component,
        (COALESCE(extra->>'api_key_id', extra->'last'->>'api_key_id', '')),
        created_at DESC
    );

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ops_system_logs_component_extra_group_created_at
    ON ops_system_logs (
        component,
        (COALESCE(extra->>'group_id', extra->'last'->>'group_id', '')),
        created_at DESC
    );
