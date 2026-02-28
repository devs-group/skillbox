-- +goose Up
-- Add CHECK constraint to prevent invalid execution status values.
ALTER TABLE sandbox.executions
    ADD CONSTRAINT executions_status_check
    CHECK (status IN ('running', 'success', 'failed', 'timeout'));

-- +goose Down
ALTER TABLE sandbox.executions
    DROP CONSTRAINT IF EXISTS executions_status_check;
