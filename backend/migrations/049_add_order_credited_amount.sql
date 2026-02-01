-- Migration: Add credited_amount and exchange_rate_used to recharge_orders
-- These fields store the actual credit amount after exchange rate conversion

ALTER TABLE recharge_orders ADD COLUMN IF NOT EXISTS credited_amount DECIMAL(20,2);
ALTER TABLE recharge_orders ADD COLUMN IF NOT EXISTS exchange_rate_used DECIMAL(10,4);

COMMENT ON COLUMN recharge_orders.credited_amount IS '到账额度（汇率转换后）';
COMMENT ON COLUMN recharge_orders.exchange_rate_used IS '使用的汇率';
