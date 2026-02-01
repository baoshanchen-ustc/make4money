-- 订阅订单表
-- 用于记录用户购买订阅套餐的订单

CREATE TABLE IF NOT EXISTS subscription_orders (
    id BIGSERIAL PRIMARY KEY,

    -- 订单号：SUBS + 年月日时分秒 + 10位随机字符串
    order_no VARCHAR(50) NOT NULL UNIQUE,

    -- 用户ID
    user_id BIGINT NOT NULL,

    -- 分组（套餐）ID
    group_id BIGINT NOT NULL,

    -- 订单金额（人民币）
    amount DECIMAL(20,2) NOT NULL,

    -- 有效期天数
    validity_days INT NOT NULL DEFAULT 30,

    -- 支付方式：wechat_pay, alipay
    payment_method VARCHAR(20) NOT NULL,

    -- 支付渠道：native（扫码支付）, jsapi（公众号支付）, h5（H5支付）
    payment_channel VARCHAR(20) NOT NULL DEFAULT 'native',

    -- 订单状态：pending（待支付）, paid（已支付）, failed（支付失败）, expired（已过期）, cancelled（已取消）
    status VARCHAR(20) NOT NULL DEFAULT 'pending',

    -- 微信支付订单号（支付成功后填充）
    wechat_transaction_id VARCHAR(64),

    -- 支付二维码URL（Native支付时填充）
    qrcode_url TEXT,

    -- 预支付交易会话标识（JSAPI支付时填充）
    prepay_id VARCHAR(64),

    -- 订单过期时间
    expire_at TIMESTAMPTZ NOT NULL,

    -- 支付完成时间
    paid_at TIMESTAMPTZ,

    -- 创建时间
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- 更新时间
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- 外键约束
    CONSTRAINT fk_subscription_orders_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_subscription_orders_group FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE CASCADE
);

-- 添加字段注释
COMMENT ON TABLE subscription_orders IS '订阅订单表';
COMMENT ON COLUMN subscription_orders.order_no IS '订单号（SUBS前缀）';
COMMENT ON COLUMN subscription_orders.user_id IS '用户ID';
COMMENT ON COLUMN subscription_orders.group_id IS '套餐分组ID';
COMMENT ON COLUMN subscription_orders.amount IS '订单金额（CNY）';
COMMENT ON COLUMN subscription_orders.validity_days IS '有效期天数';
COMMENT ON COLUMN subscription_orders.payment_method IS '支付方式';
COMMENT ON COLUMN subscription_orders.payment_channel IS '支付渠道';
COMMENT ON COLUMN subscription_orders.status IS '订单状态';
COMMENT ON COLUMN subscription_orders.wechat_transaction_id IS '微信支付订单号';
COMMENT ON COLUMN subscription_orders.qrcode_url IS '支付二维码URL';
COMMENT ON COLUMN subscription_orders.prepay_id IS '预支付会话标识';
COMMENT ON COLUMN subscription_orders.expire_at IS '订单过期时间';
COMMENT ON COLUMN subscription_orders.paid_at IS '支付完成时间';

-- 创建索引
CREATE INDEX IF NOT EXISTS idx_subscription_orders_user_id ON subscription_orders(user_id);
CREATE INDEX IF NOT EXISTS idx_subscription_orders_group_id ON subscription_orders(group_id);
CREATE INDEX IF NOT EXISTS idx_subscription_orders_status ON subscription_orders(status);
CREATE INDEX IF NOT EXISTS idx_subscription_orders_expire_at ON subscription_orders(expire_at);
CREATE INDEX IF NOT EXISTS idx_subscription_orders_user_status ON subscription_orders(user_id, status);
