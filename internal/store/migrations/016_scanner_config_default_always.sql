-- +goose Up
ALTER TABLE sandbox.scanner_config ALTER COLUMN approval_policy SET DEFAULT 'always';

-- +goose Down
ALTER TABLE sandbox.scanner_config ALTER COLUMN approval_policy SET DEFAULT 'auto';
