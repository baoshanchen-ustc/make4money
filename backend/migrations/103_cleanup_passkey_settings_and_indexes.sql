DELETE FROM settings
WHERE key IN ('passkey_rp_id', 'passkey_rp_name', 'passkey_allowed_origins');

DROP INDEX IF EXISTS idx_passkey_credentials_user_active;
DROP INDEX IF EXISTS idx_passkey_credentials_user_last_used;
DROP INDEX IF EXISTS idx_passkey_credentials_revoked_at;

CREATE INDEX IF NOT EXISTS idx_passkey_credentials_user_id_revoked_at
  ON passkey_credentials(user_id, revoked_at);
