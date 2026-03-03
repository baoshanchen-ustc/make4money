-- +goose Up
-- +goose StatementBegin
-- 用户邀请裂变推广机制 - 新增两张独立表，不修改任何现有表
-- user_referral_profiles: 每个用户的专属邀请码（懒加载，注册时创建）
-- referral_relations: 邀请关系记录（含奖励金额快照和幂等标志）

CREATE TABLE user_referral_profiles (
    id         BIGSERIAL    PRIMARY KEY,
    user_id    BIGINT       NOT NULL REFERENCES users(id),
    referral_code VARCHAR(8) NOT NULL,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_urp_user_id       UNIQUE (user_id),
    CONSTRAINT uq_urp_referral_code UNIQUE (referral_code)
);

CREATE TABLE referral_relations (
    id             BIGSERIAL      PRIMARY KEY,
    inviter_id     BIGINT         NOT NULL REFERENCES users(id),
    invitee_id     BIGINT         NOT NULL REFERENCES users(id),
    inviter_reward DECIMAL(20,8)  NOT NULL DEFAULT 0,
    invitee_reward DECIMAL(20,8)  NOT NULL DEFAULT 0,
    reward_granted BOOLEAN        NOT NULL DEFAULT FALSE,
    created_at     TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_rr_invitee_id UNIQUE (invitee_id)
);

-- 按邀请人查询邀请记录（分页列表高频查询）
CREATE INDEX idx_referral_relations_inviter_id ON referral_relations(inviter_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS referral_relations;
DROP TABLE IF EXISTS user_referral_profiles;
-- +goose StatementEnd
