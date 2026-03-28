-- 079_copilot_analytics_indexes_notx.sql
-- CONCURRENTLY 索引必须在非事务模式下执行（_notx.sql）
-- 仅包含 CREATE INDEX CONCURRENTLY IF NOT EXISTS 语句

-- initiator 字段复合索引：优化 Copilot 分析查询（按账户或用户过滤 + 时间范围）
CREATE INDEX CONCURRENTLY IF NOT EXISTS usage_logs_account_initiator_created_idx
    ON usage_logs (account_id, initiator, created_at);

CREATE INDEX CONCURRENTLY IF NOT EXISTS usage_logs_user_initiator_created_idx
    ON usage_logs (user_id, initiator, created_at);
