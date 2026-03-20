-- +goose Up

CREATE TABLE sandbox.users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    kratos_identity_id TEXT NOT NULL UNIQUE,
    tenant_id TEXT NOT NULL,
    email TEXT NOT NULL,
    display_name TEXT NOT NULL DEFAULT '',
    role TEXT NOT NULL DEFAULT 'consumer' CHECK (role IN ('admin', 'publisher', 'consumer')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_users_tenant_id ON sandbox.users(tenant_id);
CREATE INDEX idx_users_email ON sandbox.users(email);

CREATE TABLE sandbox.groups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    external_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(tenant_id, name)
);
CREATE INDEX idx_groups_tenant_id ON sandbox.groups(tenant_id);

CREATE TABLE sandbox.user_groups (
    user_id UUID NOT NULL REFERENCES sandbox.users(id) ON DELETE CASCADE,
    group_id UUID NOT NULL REFERENCES sandbox.groups(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, group_id)
);

CREATE TABLE sandbox.invite_codes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id TEXT NOT NULL,
    code TEXT NOT NULL UNIQUE,
    created_by UUID REFERENCES sandbox.users(id),
    used_by UUID REFERENCES sandbox.users(id),
    used_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_invite_codes_code ON sandbox.invite_codes(code);

CREATE TABLE sandbox.approval_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id TEXT NOT NULL,
    user_id UUID NOT NULL REFERENCES sandbox.users(id),
    skill_name TEXT NOT NULL,
    skill_version TEXT NOT NULL DEFAULT 'latest',
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected')),
    also_requested_by JSONB NOT NULL DEFAULT '[]',
    reviewed_by UUID REFERENCES sandbox.users(id),
    review_comment TEXT,
    scan_result TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reviewed_at TIMESTAMPTZ,
    UNIQUE(tenant_id, skill_name, skill_version)
);
CREATE INDEX idx_approval_requests_tenant_status ON sandbox.approval_requests(tenant_id, status);

CREATE TABLE sandbox.tenant_approved_skills (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id TEXT NOT NULL,
    skill_name TEXT NOT NULL,
    skill_version TEXT NOT NULL,
    approved_by UUID NOT NULL REFERENCES sandbox.users(id),
    approved_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(tenant_id, skill_name, skill_version)
);
CREATE INDEX idx_tenant_approved_skills_tenant ON sandbox.tenant_approved_skills(tenant_id);

-- +goose Down

DROP TABLE IF EXISTS sandbox.tenant_approved_skills;
DROP TABLE IF EXISTS sandbox.approval_requests;
DROP TABLE IF EXISTS sandbox.invite_codes;
DROP TABLE IF EXISTS sandbox.user_groups;
DROP TABLE IF EXISTS sandbox.groups;
DROP TABLE IF EXISTS sandbox.users;
