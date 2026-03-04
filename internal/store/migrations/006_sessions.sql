-- +goose Up
CREATE TABLE IF NOT EXISTS sandbox.sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id TEXT NOT NULL,
    external_id TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_accessed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(tenant_id, external_id)
);

CREATE INDEX idx_sessions_tenant ON sandbox.sessions(tenant_id);
CREATE INDEX idx_sessions_external ON sandbox.sessions(tenant_id, external_id);

-- +goose Down
DROP TABLE IF EXISTS sandbox.sessions;
