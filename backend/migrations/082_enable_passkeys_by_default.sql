INSERT INTO settings (key, value, updated_at)
VALUES ('passkey_enabled', 'true', NOW())
ON CONFLICT (key) DO UPDATE
SET value = EXCLUDED.value,
    updated_at = EXCLUDED.updated_at
WHERE settings.value IS DISTINCT FROM EXCLUDED.value;
