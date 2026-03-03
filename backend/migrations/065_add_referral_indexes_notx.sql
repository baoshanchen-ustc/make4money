-- +goose Up
-- +goose StatementBegin
-- 并发创建索引（不阻塞写入），需 _notx 后缀
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_urp_user_id_concurrent
    ON user_referral_profiles(user_id);

CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_urp_referral_code_concurrent
    ON user_referral_profiles(referral_code);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX CONCURRENTLY IF EXISTS idx_urp_user_id_concurrent;
DROP INDEX CONCURRENTLY IF EXISTS idx_urp_referral_code_concurrent;
-- +goose StatementEnd
