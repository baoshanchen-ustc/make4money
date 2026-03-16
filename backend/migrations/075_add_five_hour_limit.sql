-- Add five-hour limit to groups table
ALTER TABLE groups ADD COLUMN IF NOT EXISTS five_hour_limit_usd decimal(20,8);

-- Add five-hour usage tracking to user_subscriptions table
ALTER TABLE user_subscriptions ADD COLUMN IF NOT EXISTS five_hour_window_start timestamptz;
ALTER TABLE user_subscriptions ADD COLUMN IF NOT EXISTS five_hour_usage_usd decimal(20,10) NOT NULL DEFAULT 0;
