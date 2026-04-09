CREATE TABLE IF NOT EXISTS user_checkins (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    checkin_date DATE NOT NULL,
    reward_amount DECIMAL(20,8) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_user_checkins_user_date
ON user_checkins (user_id, checkin_date);

CREATE INDEX IF NOT EXISTS idx_user_checkins_date
ON user_checkins (checkin_date);

CREATE INDEX IF NOT EXISTS idx_user_checkins_user_created_at
ON user_checkins (user_id, created_at DESC);
