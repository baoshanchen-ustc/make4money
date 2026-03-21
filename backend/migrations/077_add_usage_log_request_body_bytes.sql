-- 077: Add request_body_bytes to usage_logs for tracking request payload size.
-- ops_error_logs already has this column; this adds it to the success path too.

ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS request_body_bytes INT;
