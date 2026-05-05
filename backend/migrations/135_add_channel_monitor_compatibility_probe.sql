-- Migration: 135_add_channel_monitor_compatibility_probe
-- Adds an opt-in monitor probe compatibility switch.

ALTER TABLE channel_monitors
    ADD COLUMN IF NOT EXISTS compatibility_probe_enabled BOOLEAN NOT NULL DEFAULT FALSE;
