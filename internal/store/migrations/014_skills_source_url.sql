-- +goose Up
ALTER TABLE sandbox.skills ADD COLUMN IF NOT EXISTS source_url TEXT;

-- +goose Down
ALTER TABLE sandbox.skills DROP COLUMN IF EXISTS source_url;
