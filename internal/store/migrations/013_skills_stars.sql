-- +goose Up
-- Migration: Track GitHub stars per skill so the marketplace can sort
-- by popularity (descending). Defaults to 0 for skills uploaded outside
-- the GitHub marketplace flow.

ALTER TABLE sandbox.skills
    ADD COLUMN IF NOT EXISTS stars INTEGER NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_skills_marketplace_stars
    ON sandbox.skills (tenant_id, stars DESC, name);

-- +goose Down
DROP INDEX IF EXISTS idx_skills_marketplace_stars;
ALTER TABLE sandbox.skills DROP COLUMN IF EXISTS stars;
