-- +goose Up
ALTER TABLE sandbox.api_keys ADD COLUMN IF NOT EXISTS is_service BOOLEAN NOT NULL DEFAULT FALSE;

-- Mark all existing keys as service keys. VectorChat is the only consumer
-- today and requires cross-tenant access. New keys default to FALSE.
UPDATE sandbox.api_keys SET is_service = TRUE WHERE revoked_at IS NULL;

-- +goose Down
ALTER TABLE sandbox.api_keys DROP COLUMN IF EXISTS is_service;
