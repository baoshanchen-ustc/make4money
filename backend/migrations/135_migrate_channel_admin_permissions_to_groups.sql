SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

DO $$
BEGIN
    IF to_regclass('public.user_channel_permissions') IS NOT NULL THEN
        INSERT INTO user_allowed_groups (user_id, group_id, created_at)
        SELECT DISTINCT ucp.user_id, cg.group_id, NOW()
        FROM user_channel_permissions ucp
        JOIN channel_groups cg ON cg.channel_id = ucp.channel_id
        JOIN groups g ON g.id = cg.group_id
        JOIN users u ON u.id = ucp.user_id
        WHERE u.role = 'channel_admin'
          AND g.deleted_at IS NULL
        ON CONFLICT (user_id, group_id) DO NOTHING;
    END IF;
END $$;

DROP TABLE IF EXISTS user_channel_permissions;
