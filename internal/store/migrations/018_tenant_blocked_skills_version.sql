-- +goose Up
ALTER TABLE sandbox.tenant_blocked_skills ADD COLUMN version TEXT NOT NULL DEFAULT '';
ALTER TABLE sandbox.tenant_blocked_skills DROP CONSTRAINT tenant_blocked_skills_pkey;
ALTER TABLE sandbox.tenant_blocked_skills ADD PRIMARY KEY (tenant_id, name, version);

-- +goose Down
ALTER TABLE sandbox.tenant_blocked_skills DROP CONSTRAINT tenant_blocked_skills_pkey;
DELETE FROM sandbox.tenant_blocked_skills a USING sandbox.tenant_blocked_skills b
  WHERE a.tenant_id = b.tenant_id AND a.name = b.name AND a.ctid > b.ctid;
ALTER TABLE sandbox.tenant_blocked_skills ADD PRIMARY KEY (tenant_id, name);
ALTER TABLE sandbox.tenant_blocked_skills DROP COLUMN version;
