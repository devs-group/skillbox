-- Migration: Add skills metadata table for fast listing with descriptions.
-- Previously, ListSkills iterated MinIO objects which only gave name/version.
-- This table caches skill metadata on upload so listings include description.

CREATE TABLE sandbox.skills (
    tenant_id   TEXT NOT NULL,
    name        TEXT NOT NULL,
    version     TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    lang        TEXT NOT NULL DEFAULT 'python',
    uploaded_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, name, version)
);

CREATE INDEX idx_skills_tenant ON sandbox.skills(tenant_id);
CREATE INDEX idx_skills_name ON sandbox.skills(tenant_id, name);
