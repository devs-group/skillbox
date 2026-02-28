-- +goose Up
CREATE SCHEMA IF NOT EXISTS sandbox;

CREATE TABLE sandbox.api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key_hash TEXT NOT NULL UNIQUE,  -- SHA-256 hex
    tenant_id TEXT NOT NULL,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at TIMESTAMPTZ
);
CREATE INDEX idx_api_keys_hash ON sandbox.api_keys(key_hash);
CREATE INDEX idx_api_keys_tenant ON sandbox.api_keys(tenant_id);

CREATE TABLE sandbox.executions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    skill_name TEXT NOT NULL,
    skill_version TEXT NOT NULL,
    tenant_id TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'running',  -- running, success, failed, timeout
    input JSONB,
    output JSONB,
    logs TEXT,
    files_url TEXT,
    files_list TEXT[],
    duration_ms BIGINT,
    error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ
);
CREATE INDEX idx_executions_tenant ON sandbox.executions(tenant_id);
CREATE INDEX idx_executions_status ON sandbox.executions(status);

-- +goose Down
DROP TABLE IF EXISTS sandbox.executions;
DROP TABLE IF EXISTS sandbox.api_keys;
DROP SCHEMA IF EXISTS sandbox;
