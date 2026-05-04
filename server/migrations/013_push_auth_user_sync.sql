-- Track whether local users were provisioned in ProIdentity Push Auth.
-- This is sync metadata only; the API key remains in server-side settings.

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS push_auth_synced_at TIMESTAMP NULL DEFAULT NULL,
    ADD COLUMN IF NOT EXISTS push_auth_sync_status VARCHAR(64) NULL DEFAULT NULL,
    ADD COLUMN IF NOT EXISTS push_auth_sync_error TEXT NULL DEFAULT NULL;
