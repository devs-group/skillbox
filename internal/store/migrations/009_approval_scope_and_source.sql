-- +goose Up
ALTER TABLE sandbox.approval_requests ADD COLUMN source TEXT NOT NULL DEFAULT 'marketplace';
ALTER TABLE sandbox.approval_requests ADD COLUMN source_url TEXT;
ALTER TABLE sandbox.approval_requests ADD COLUMN approval_scope TEXT NOT NULL DEFAULT 'global' CHECK (approval_scope IN ('user', 'global'));

ALTER TABLE sandbox.tenant_approved_skills ADD COLUMN approval_scope TEXT NOT NULL DEFAULT 'global' CHECK (approval_scope IN ('user', 'global'));
ALTER TABLE sandbox.tenant_approved_skills ADD COLUMN approved_for_user UUID REFERENCES sandbox.users(id);

-- +goose Down
ALTER TABLE sandbox.tenant_approved_skills DROP COLUMN approved_for_user;
ALTER TABLE sandbox.tenant_approved_skills DROP COLUMN approval_scope;
ALTER TABLE sandbox.approval_requests DROP COLUMN approval_scope;
ALTER TABLE sandbox.approval_requests DROP COLUMN source_url;
ALTER TABLE sandbox.approval_requests DROP COLUMN source;
