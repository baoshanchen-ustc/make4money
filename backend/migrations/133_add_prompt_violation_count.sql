-- Track keyword-filter prompt violations per user. The gateway increments this
-- counter atomically and disables the user once the configured limit is reached.

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS prompt_violation_count INTEGER NOT NULL DEFAULT 0;

