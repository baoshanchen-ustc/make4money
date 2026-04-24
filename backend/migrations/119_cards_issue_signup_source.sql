-- Add 'cards_issue' to the allowed signup_source values so users created via
-- the cards issue integration endpoint pass the CHECK constraint introduced
-- in migration 108.

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'users_signup_source_check'
    ) THEN
        ALTER TABLE users DROP CONSTRAINT users_signup_source_check;
    END IF;

    ALTER TABLE users
        ADD CONSTRAINT users_signup_source_check
        CHECK (signup_source IN ('email', 'linuxdo', 'wechat', 'oidc', 'cards_issue'));
END $$;
