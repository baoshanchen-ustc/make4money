-- 添加 Group 表的可购买配置字段
-- 用于配置哪些分组可以被用户在线购买

-- 添加可购买标识
ALTER TABLE groups ADD COLUMN IF NOT EXISTS is_purchasable BOOLEAN NOT NULL DEFAULT false;

-- 添加价格（人民币）
ALTER TABLE groups ADD COLUMN IF NOT EXISTS price_cny DECIMAL(20,2);

-- 添加显示排序
ALTER TABLE groups ADD COLUMN IF NOT EXISTS display_order INT NOT NULL DEFAULT 0;

-- 添加可购买描述（给用户展示的套餐说明）
ALTER TABLE groups ADD COLUMN IF NOT EXISTS purchasable_description TEXT;

-- 添加字段注释
COMMENT ON COLUMN groups.is_purchasable IS '是否可在线购买';
COMMENT ON COLUMN groups.price_cny IS '套餐价格（人民币）';
COMMENT ON COLUMN groups.display_order IS '显示排序（小的在前）';
COMMENT ON COLUMN groups.purchasable_description IS '套餐描述（展示给用户）';

-- 创建可购买分组的部分索引
CREATE INDEX IF NOT EXISTS idx_groups_purchasable
ON groups(display_order, id)
WHERE is_purchasable = true AND status = 'active' AND deleted_at IS NULL;
