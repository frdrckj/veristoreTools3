-- Add last_synced_at column to track which terminals were synced from a report.
-- Terminals with a non-NULL last_synced_at that is older than the current sync
-- are considered deleted from TMS and will be removed during sync cleanup.
-- V2-copied terminals have NULL last_synced_at and are never touched by cleanup.
ALTER TABLE terminal ADD COLUMN IF NOT EXISTS last_synced_at DATETIME DEFAULT NULL;
