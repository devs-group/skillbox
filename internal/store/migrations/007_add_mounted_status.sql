-- +goose Up
-- Add 'mounted' to allowed execution status values (used by mount_only runs).
ALTER TABLE sandbox.executions
    DROP CONSTRAINT IF EXISTS executions_status_check;
ALTER TABLE sandbox.executions
    ADD CONSTRAINT executions_status_check
    CHECK (status IN ('running', 'success', 'failed', 'timeout', 'mounted'));

-- +goose Down
ALTER TABLE sandbox.executions
    DROP CONSTRAINT IF EXISTS executions_status_check;
ALTER TABLE sandbox.executions
    ADD CONSTRAINT executions_status_check
    CHECK (status IN ('running', 'success', 'failed', 'timeout'));
