-- +goose Up
-- Add composite index for efficient "latest version" resolution queries.
-- Covers: SELECT ... WHERE tenant_id = $1 AND name = $2 ORDER BY version DESC LIMIT 1
CREATE INDEX idx_skills_latest_version ON sandbox.skills(tenant_id, name, version DESC);

-- +goose Down
DROP INDEX IF EXISTS sandbox.idx_skills_latest_version;
