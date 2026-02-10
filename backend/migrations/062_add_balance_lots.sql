-- 创建余额批次表（Balance Lots）
-- 每次资金入账（充值、兑换码、优惠码、管理员调整）创建一条记录
-- 消费时按过期时间顺序扣减（FEFO - First Expire, First Out）
CREATE TABLE IF NOT EXISTS balance_lots (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    source_type VARCHAR(20) NOT NULL,
    source_ref VARCHAR(100),
    original_amount DECIMAL(20,8) NOT NULL,
    remaining_amount DECIMAL(20,8) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    expires_at TIMESTAMPTZ NOT NULL,
    expired_at TIMESTAMPTZ,
    description TEXT DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 创建索引
-- 扣费查询：按用户查活跃批次，按过期时间排序（FEFO）
CREATE INDEX IF NOT EXISTS idx_balance_lots_user_status_expires ON balance_lots(user_id, status, expires_at);
-- 过期调度器查询：查找已过期的活跃批次
CREATE INDEX IF NOT EXISTS idx_balance_lots_status_expires ON balance_lots(status, expires_at);
-- 用户ID索引
CREATE INDEX IF NOT EXISTS idx_balance_lots_user_id ON balance_lots(user_id);

-- 添加注释
COMMENT ON TABLE balance_lots IS '余额批次表：每次资金入账创建一条，消费时按过期时间顺序扣减';
COMMENT ON COLUMN balance_lots.user_id IS '用户ID';
COMMENT ON COLUMN balance_lots.source_type IS '来源类型: recharge(充值), redeem(兑换码), promo(优惠码), adjust(管理员调整), migration(迁移)';
COMMENT ON COLUMN balance_lots.source_ref IS '关联来源引用（订单号、兑换码等）';
COMMENT ON COLUMN balance_lots.original_amount IS '入账原始金额';
COMMENT ON COLUMN balance_lots.remaining_amount IS '当前剩余金额';
COMMENT ON COLUMN balance_lots.status IS '批次状态: active(活跃), depleted(已耗尽), expired(已过期)';
COMMENT ON COLUMN balance_lots.expires_at IS '过期时间';
COMMENT ON COLUMN balance_lots.expired_at IS '实际过期处理时间（过期调度器处理时设置）';
COMMENT ON COLUMN balance_lots.description IS '描述';
