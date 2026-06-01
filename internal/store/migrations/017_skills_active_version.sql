-- +goose Up
-- Add an explicit per-(tenant, name) active version pointer.
-- Resolution previously relied on uploaded_at DESC; the switcher needs a
-- deliberately chosen active version, with at most one active per skill.
ALTER TABLE sandbox.skills
    ADD COLUMN is_active BOOLEAN NOT NULL DEFAULT false;

-- Enforce at most one active version per (tenant, name).
CREATE UNIQUE INDEX idx_skills_one_active
    ON sandbox.skills (tenant_id, name)
    WHERE is_active;

-- +goose Down
DROP INDEX IF EXISTS sandbox.idx_skills_one_active;

ALTER TABLE sandbox.skills
    DROP COLUMN IF EXISTS is_active;
