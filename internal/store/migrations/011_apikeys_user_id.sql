-- +goose Up
ALTER TABLE sandbox.api_keys ADD COLUMN IF NOT EXISTS user_id TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE sandbox.api_keys DROP COLUMN IF EXISTS user_id;
