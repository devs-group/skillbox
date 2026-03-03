-- +goose Up
CREATE TABLE IF NOT EXISTS sandbox.files (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    TEXT NOT NULL,
    session_id   TEXT,
    execution_id UUID REFERENCES sandbox.executions(id),
    name         TEXT NOT NULL,
    content_type TEXT NOT NULL DEFAULT 'application/octet-stream',
    size_bytes   BIGINT NOT NULL DEFAULT 0,
    s3_key       TEXT NOT NULL,
    version      INTEGER NOT NULL DEFAULT 1,
    parent_id    UUID REFERENCES sandbox.files(id),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_files_tenant ON sandbox.files(tenant_id);
CREATE INDEX idx_files_execution ON sandbox.files(execution_id);
CREATE INDEX idx_files_session ON sandbox.files(session_id);
CREATE INDEX idx_files_parent ON sandbox.files(parent_id);

-- +goose Down
DROP TABLE IF EXISTS sandbox.files;
