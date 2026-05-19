-- +goose Up
CREATE TABLE sandbox.tenant_blocked_skills (
    tenant_id TEXT NOT NULL,
    name TEXT NOT NULL,
    blocked_by TEXT,
    reason TEXT,
    blocked_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, name)
);

-- +goose Down
DROP TABLE IF EXISTS sandbox.tenant_blocked_skills;
