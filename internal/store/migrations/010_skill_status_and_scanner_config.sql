-- +goose Up
-- Migration: Add skill lifecycle status and per-tenant scanner configuration.
-- Skills now track their scan/approval status. A background scan worker
-- processes pending skills and transitions them through the lifecycle.

-- Add status lifecycle fields to skills table.
-- Existing skills default to 'available' for backward compatibility.
ALTER TABLE sandbox.skills
    ADD COLUMN status TEXT NOT NULL DEFAULT 'available',
    ADD COLUMN scan_result JSONB,
    ADD COLUMN scanned_at TIMESTAMPTZ,
    ADD COLUMN reviewed_by TEXT,
    ADD COLUMN reviewed_at TIMESTAMPTZ;

ALTER TABLE sandbox.skills
    ADD CONSTRAINT skills_status_check
    CHECK (status IN ('pending', 'scanning', 'review', 'available', 'declined', 'quarantined'));

-- Index for worker polling: quickly find skills needing processing.
CREATE INDEX idx_skills_status ON sandbox.skills (status)
    WHERE status IN ('pending', 'scanning');

-- Per-tenant scanner configuration.
CREATE TABLE sandbox.scanner_config (
    tenant_id        TEXT PRIMARY KEY,
    approval_policy  TEXT NOT NULL DEFAULT 'auto'
        CHECK (approval_policy IN ('auto', 'always', 'none')),
    tier1_enabled    BOOLEAN NOT NULL DEFAULT true,
    tier2_enabled    BOOLEAN NOT NULL DEFAULT true,
    tier3_enabled    BOOLEAN NOT NULL DEFAULT false,
    tier3_api_key    TEXT,
    tier3_model      TEXT NOT NULL DEFAULT 'claude-sonnet-4-5-20250514',
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS sandbox.scanner_config;

ALTER TABLE sandbox.skills
    DROP CONSTRAINT IF EXISTS skills_status_check;

DROP INDEX IF EXISTS sandbox.idx_skills_status;

ALTER TABLE sandbox.skills
    DROP COLUMN IF EXISTS status,
    DROP COLUMN IF EXISTS scan_result,
    DROP COLUMN IF EXISTS scanned_at,
    DROP COLUMN IF EXISTS reviewed_by,
    DROP COLUMN IF EXISTS reviewed_at;
