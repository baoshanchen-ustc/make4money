-- 079_copilot_analytics.sql
-- Copilot 成本分析系统数据层变更

-- 1. usage_logs 新增 initiator 字段
-- 'user'  = 用户首轮请求，消耗 GitHub Copilot Premium Interactions 配额
-- 'agent' = 任务内子请求（含 assistant/tool 历史消息），消耗标准配额
-- 非 Copilot 平台统一写 'user'（默认值）
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS initiator TEXT NOT NULL DEFAULT 'user';

-- 注：usage_logs 的 CONCURRENTLY 复合索引已移至 079_copilot_analytics_indexes_notx.sql

-- 2. Copilot 配额快照表
-- 每次实时查询后 UPSERT 当天记录，用于趋势图历史数据
CREATE TABLE IF NOT EXISTS copilot_quota_snapshots (
    id                  BIGSERIAL PRIMARY KEY,
    account_id          BIGINT NOT NULL,
    snapshot_date       DATE NOT NULL,
    plan_type           TEXT,
    premium_entitlement INTEGER NOT NULL DEFAULT 0,
    premium_remaining   INTEGER NOT NULL DEFAULT 0,
    premium_used        INTEGER NOT NULL DEFAULT 0,
    premium_overage     INTEGER NOT NULL DEFAULT 0,
    unlimited           BOOLEAN NOT NULL DEFAULT FALSE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (account_id, snapshot_date)
);

CREATE INDEX IF NOT EXISTS copilot_quota_snapshots_account_id_idx
    ON copilot_quota_snapshots (account_id);

CREATE INDEX IF NOT EXISTS copilot_quota_snapshots_snapshot_date_idx
    ON copilot_quota_snapshots (snapshot_date);

-- 3. Copilot 预算告警配置表
-- 每个账户最多一条记录
CREATE TABLE IF NOT EXISTS copilot_budget_alerts (
    id               BIGSERIAL PRIMARY KEY,
    account_id       BIGINT NOT NULL UNIQUE,
    monthly_budget   DECIMAL(10,2) NOT NULL DEFAULT 0,
    alert_threshold  INTEGER NOT NULL DEFAULT 80,
    enabled          BOOLEAN NOT NULL DEFAULT TRUE,
    last_alerted_at  TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS copilot_budget_alerts_account_id_idx
    ON copilot_budget_alerts (account_id);

CREATE INDEX IF NOT EXISTS copilot_budget_alerts_enabled_idx
    ON copilot_budget_alerts (enabled);


-- 2. Copilot 配额快照表
-- 每次实时查询后 UPSERT 当天记录，用于趋势图历史数据
CREATE TABLE IF NOT EXISTS copilot_quota_snapshots (
    id                  BIGSERIAL PRIMARY KEY,
    account_id          BIGINT NOT NULL,
    snapshot_date       DATE NOT NULL,
    plan_type           TEXT,
    premium_entitlement INTEGER NOT NULL DEFAULT 0,
    premium_remaining   INTEGER NOT NULL DEFAULT 0,
    premium_used        INTEGER NOT NULL DEFAULT 0,
    premium_overage     INTEGER NOT NULL DEFAULT 0,
    unlimited           BOOLEAN NOT NULL DEFAULT FALSE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (account_id, snapshot_date)
);

CREATE INDEX IF NOT EXISTS copilot_quota_snapshots_account_id_idx
    ON copilot_quota_snapshots (account_id);

CREATE INDEX IF NOT EXISTS copilot_quota_snapshots_snapshot_date_idx
    ON copilot_quota_snapshots (snapshot_date);

-- 3. Copilot 预算告警配置表
-- 每个账户最多一条记录
CREATE TABLE IF NOT EXISTS copilot_budget_alerts (
    id               BIGSERIAL PRIMARY KEY,
    account_id       BIGINT NOT NULL UNIQUE,
    monthly_budget   DECIMAL(10,2) NOT NULL DEFAULT 0,
    alert_threshold  INTEGER NOT NULL DEFAULT 80,
    enabled          BOOLEAN NOT NULL DEFAULT TRUE,
    last_alerted_at  TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS copilot_budget_alerts_account_id_idx
    ON copilot_budget_alerts (account_id);

CREATE INDEX IF NOT EXISTS copilot_budget_alerts_enabled_idx
    ON copilot_budget_alerts (enabled);
