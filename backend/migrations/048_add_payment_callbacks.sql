-- 创建支付回调记录表
CREATE TABLE IF NOT EXISTS payment_callbacks (
    id BIGSERIAL PRIMARY KEY,
    order_no VARCHAR(50),
    payment_method VARCHAR(20) NOT NULL,
    transaction_id VARCHAR(64),
    request_headers JSONB NOT NULL DEFAULT '{}',
    request_body TEXT DEFAULT '',
    signature_valid BOOLEAN NOT NULL DEFAULT FALSE,
    process_status VARCHAR(20) NOT NULL DEFAULT 'received',
    process_message TEXT DEFAULT '',
    response_code VARCHAR(20) DEFAULT '',
    response_message TEXT DEFAULT '',
    process_time_ms BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 创建索引
CREATE INDEX IF NOT EXISTS idx_payment_callbacks_order_no ON payment_callbacks(order_no);
CREATE INDEX IF NOT EXISTS idx_payment_callbacks_transaction_id ON payment_callbacks(transaction_id);
CREATE INDEX IF NOT EXISTS idx_payment_callbacks_payment_method ON payment_callbacks(payment_method);
CREATE INDEX IF NOT EXISTS idx_payment_callbacks_process_status ON payment_callbacks(process_status);
CREATE INDEX IF NOT EXISTS idx_payment_callbacks_created_at ON payment_callbacks(created_at);

-- 添加注释
COMMENT ON TABLE payment_callbacks IS '支付回调记录表（用于审计和调试）';
COMMENT ON COLUMN payment_callbacks.order_no IS '订单号（从回调数据中解析）';
COMMENT ON COLUMN payment_callbacks.payment_method IS '支付方式: wechat_pay, alipay';
COMMENT ON COLUMN payment_callbacks.transaction_id IS '支付平台订单号';
COMMENT ON COLUMN payment_callbacks.request_headers IS '请求头（JSON格式）';
COMMENT ON COLUMN payment_callbacks.request_body IS '请求体（加密前的原始数据）';
COMMENT ON COLUMN payment_callbacks.signature_valid IS '签名验证结果';
COMMENT ON COLUMN payment_callbacks.process_status IS '处理状态: received, processing, success, failed';
COMMENT ON COLUMN payment_callbacks.process_message IS '处理结果描述';
COMMENT ON COLUMN payment_callbacks.response_code IS '响应码（返回给微信）';
COMMENT ON COLUMN payment_callbacks.response_message IS '响应消息';
COMMENT ON COLUMN payment_callbacks.process_time_ms IS '处理耗时（毫秒）';
