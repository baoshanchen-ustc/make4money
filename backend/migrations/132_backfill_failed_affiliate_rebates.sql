-- Backfill balance-order affiliate rebates that failed during the initial
-- audit-claim rollout because Postgres could not infer the shared $1 type.
-- Idempotency gate: only orders with a matching FAILED audit and without an
-- APPLIED/SKIPPED rebate audit are eligible.

WITH cfg AS (
    SELECT
        COALESCE((SELECT value = 'true' FROM settings WHERE key = 'affiliate_enabled'), false) AS enabled,
        COALESCE((
            SELECT value::numeric
            FROM settings
            WHERE key = 'affiliate_rebate_rate'
              AND value ~ '^-?[0-9]+(\.[0-9]+)?$'
        ), 20) AS global_rate
),
eligible AS (
    SELECT
        po.id::text AS order_id,
        po.user_id AS invitee_user_id,
        invitee_aff.inviter_id AS inviter_id,
        po.amount::numeric AS base_amount,
        ROUND((po.amount::numeric * COALESCE(inviter_aff.aff_rebate_rate_percent, cfg.global_rate) / 100), 8) AS rebate_amount
    FROM payment_orders po
    JOIN user_affiliates invitee_aff
      ON invitee_aff.user_id = po.user_id
     AND invitee_aff.inviter_id IS NOT NULL
    JOIN user_affiliates inviter_aff
      ON inviter_aff.user_id = invitee_aff.inviter_id
    CROSS JOIN cfg
    WHERE cfg.enabled
      AND po.order_type = 'balance'
      AND po.status = 'COMPLETED'
      AND po.amount > 0
      AND EXISTS (
          SELECT 1
          FROM payment_audit_logs failed
          WHERE failed.order_id = po.id::text
            AND failed.action = 'AFFILIATE_REBATE_FAILED'
            AND failed.detail LIKE '%inconsistent types deduced for parameter%'
      )
      AND NOT EXISTS (
          SELECT 1
          FROM payment_audit_logs done
          WHERE done.order_id = po.id::text
            AND done.action IN ('AFFILIATE_REBATE_APPLIED', 'AFFILIATE_REBATE_SKIPPED')
      )
),
filtered AS (
    SELECT *
    FROM eligible
    WHERE rebate_amount > 0
),
aggregated AS (
    SELECT inviter_id, SUM(rebate_amount) AS total_rebate
    FROM filtered
    GROUP BY inviter_id
)
UPDATE user_affiliates ua
SET aff_quota = ua.aff_quota + aggregated.total_rebate,
    aff_history_quota = ua.aff_history_quota + aggregated.total_rebate,
    updated_at = NOW()
FROM aggregated
WHERE ua.user_id = aggregated.inviter_id;

WITH cfg AS (
    SELECT
        COALESCE((SELECT value = 'true' FROM settings WHERE key = 'affiliate_enabled'), false) AS enabled,
        COALESCE((
            SELECT value::numeric
            FROM settings
            WHERE key = 'affiliate_rebate_rate'
              AND value ~ '^-?[0-9]+(\.[0-9]+)?$'
        ), 20) AS global_rate
),
eligible AS (
    SELECT
        po.id::text AS order_id,
        po.user_id AS invitee_user_id,
        invitee_aff.inviter_id AS inviter_id,
        po.amount::numeric AS base_amount,
        ROUND((po.amount::numeric * COALESCE(inviter_aff.aff_rebate_rate_percent, cfg.global_rate) / 100), 8) AS rebate_amount
    FROM payment_orders po
    JOIN user_affiliates invitee_aff
      ON invitee_aff.user_id = po.user_id
     AND invitee_aff.inviter_id IS NOT NULL
    JOIN user_affiliates inviter_aff
      ON inviter_aff.user_id = invitee_aff.inviter_id
    CROSS JOIN cfg
    WHERE cfg.enabled
      AND po.order_type = 'balance'
      AND po.status = 'COMPLETED'
      AND po.amount > 0
      AND EXISTS (
          SELECT 1
          FROM payment_audit_logs failed
          WHERE failed.order_id = po.id::text
            AND failed.action = 'AFFILIATE_REBATE_FAILED'
            AND failed.detail LIKE '%inconsistent types deduced for parameter%'
      )
      AND NOT EXISTS (
          SELECT 1
          FROM payment_audit_logs done
          WHERE done.order_id = po.id::text
            AND done.action IN ('AFFILIATE_REBATE_APPLIED', 'AFFILIATE_REBATE_SKIPPED')
      )
)
INSERT INTO user_affiliate_ledger (user_id, action, amount, source_user_id, created_at, updated_at)
SELECT inviter_id, 'accrue', rebate_amount, invitee_user_id, NOW(), NOW()
FROM eligible
WHERE rebate_amount > 0;

WITH cfg AS (
    SELECT
        COALESCE((SELECT value = 'true' FROM settings WHERE key = 'affiliate_enabled'), false) AS enabled,
        COALESCE((
            SELECT value::numeric
            FROM settings
            WHERE key = 'affiliate_rebate_rate'
              AND value ~ '^-?[0-9]+(\.[0-9]+)?$'
        ), 20) AS global_rate
),
eligible AS (
    SELECT
        po.id::text AS order_id,
        po.amount::numeric AS base_amount,
        ROUND((po.amount::numeric * COALESCE(inviter_aff.aff_rebate_rate_percent, cfg.global_rate) / 100), 8) AS rebate_amount
    FROM payment_orders po
    JOIN user_affiliates invitee_aff
      ON invitee_aff.user_id = po.user_id
     AND invitee_aff.inviter_id IS NOT NULL
    JOIN user_affiliates inviter_aff
      ON inviter_aff.user_id = invitee_aff.inviter_id
    CROSS JOIN cfg
    WHERE cfg.enabled
      AND po.order_type = 'balance'
      AND po.status = 'COMPLETED'
      AND po.amount > 0
      AND EXISTS (
          SELECT 1
          FROM payment_audit_logs failed
          WHERE failed.order_id = po.id::text
            AND failed.action = 'AFFILIATE_REBATE_FAILED'
            AND failed.detail LIKE '%inconsistent types deduced for parameter%'
      )
      AND NOT EXISTS (
          SELECT 1
          FROM payment_audit_logs done
          WHERE done.order_id = po.id::text
            AND done.action IN ('AFFILIATE_REBATE_APPLIED', 'AFFILIATE_REBATE_SKIPPED')
      )
)
INSERT INTO payment_audit_logs (order_id, action, detail, operator, created_at)
SELECT
    order_id,
    'AFFILIATE_REBATE_APPLIED',
    json_build_object(
        'baseAmount', base_amount,
        'rebateAmount', rebate_amount,
        'reason', 'backfill failed affiliate rebate claim'
    )::text,
    'system',
    NOW()
FROM eligible
WHERE rebate_amount > 0
ON CONFLICT (order_id, action) DO NOTHING;
