-- 创建余额变动日志表
CREATE TABLE IF NOT EXISTS balance_logs (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    change_type VARCHAR(20) NOT NULL,
    amount DECIMAL(20,2) NOT NULL,
    balance_before DECIMAL(20,2) NOT NULL,
    balance_after DECIMAL(20,2) NOT NULL,
    related_order_no VARCHAR(50),
    description TEXT DEFAULT '',
    operator_id BIGINT NOT NULL DEFAULT 0,
    operator_type VARCHAR(20) NOT NULL DEFAULT 'system',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 创建索引
CREATE INDEX IF NOT EXISTS idx_balance_logs_user_id ON balance_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_balance_logs_change_type ON balance_logs(change_type);
CREATE INDEX IF NOT EXISTS idx_balance_logs_related_order_no ON balance_logs(related_order_no);
CREATE INDEX IF NOT EXISTS idx_balance_logs_created_at ON balance_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_balance_logs_user_id_change_type ON balance_logs(user_id, change_type);

-- 添加注释
COMMENT ON TABLE balance_logs IS '余额变动日志表（只允许插入，不允许修改和删除）';
COMMENT ON COLUMN balance_logs.user_id IS '用户ID';
COMMENT ON COLUMN balance_logs.change_type IS '变动类型: recharge(充值), consume(消费), refund(退款), adjust(调整)';
COMMENT ON COLUMN balance_logs.amount IS '变动金额（正数增加，负数减少）';
COMMENT ON COLUMN balance_logs.balance_before IS '变动前余额';
COMMENT ON COLUMN balance_logs.balance_after IS '变动后余额';
COMMENT ON COLUMN balance_logs.related_order_no IS '关联订单号';
COMMENT ON COLUMN balance_logs.description IS '变动描述';
COMMENT ON COLUMN balance_logs.operator_id IS '操作人ID（系统操作时为0）';
COMMENT ON COLUMN balance_logs.operator_type IS '操作人类型: system(系统), admin(管理员), user(用户)';
