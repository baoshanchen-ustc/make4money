CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_urp_user_id_concurrent
    ON user_referral_profiles(user_id);

CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_urp_referral_code_concurrent
    ON user_referral_profiles(referral_code);
