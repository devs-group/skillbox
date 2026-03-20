---
date: 2026-03-19
topic: enterprise-skill-registry
---

# Enterprise Skill Registry

## Problem Frame

Developers in enterprise environments need a governed way to discover, request, and install AI coding skills (e.g., Claude Code skills) from both a public marketplace and internal repositories. Today, Skillbox has no user identity, no approval workflows, and no CLI-based installation flow. Enterprise teams cannot adopt Skillbox because there is no SSO, no governance process for which skills are allowed, and no way to browse and install skills without manual file operations.

**Who is affected:**
- **Developers** who want to find and install skills quickly via CLI
- **Admins** who need to control which public skills enter the enterprise environment
- **Enterprise IT** who require SSO integration (Entra ID, Google) and audit capability

## Requirements

### Authentication & Identity

- R1. **Ory Kratos + Hydra for identity and OAuth2.** Kratos handles user registration, login, and identity management. Hydra provides OAuth2/OIDC tokens for CLI and API authentication. Both self-hosted, fully local-testable via Docker Compose without external dependencies.
- R2. **Username/password login in V1.** Users can register and log in with email + password via Kratos. Social/enterprise SSO (Entra ID, Google) is architecturally supported but configured later per customer.
- R3. **CLI authentication via Device Authorization Grant (RFC 8628).** `skillbox login` initiates device code flow through Hydra. User opens browser, enters code, authenticates via Kratos, CLI receives tokens. Tokens stored locally in `~/.config/skillbox/credentials.json`.
- R4. **Go backend validates Hydra-issued tokens.** Existing `AuthMiddleware` extended to accept both legacy API keys (for machine-to-machine/CI) and Hydra JWT tokens. JWT tokens carry `sub` (Kratos identity ID), `tenant_id`, and `groups` claims. Both auth paths set the same context keys so all handlers work unchanged.
- R5. **User-to-tenant mapping.** Each Kratos identity belongs to exactly one tenant. Mapping stored in Skillbox DB (not in Kratos). First user in a tenant gets `admin` role automatically.

### User & Group Model

- R6. **Users table.** Fields: `id`, `kratos_identity_id`, `tenant_id`, `email`, `display_name`, `role` (admin/publisher/consumer), `created_at`, `updated_at`. Populated on first successful login via Kratos identity data.
- R7. **Groups table (schema from day one).** Fields: `id`, `tenant_id`, `name`, `description`, `external_id` (for future OAuth group sync), `created_at`. Groups are tenant-scoped. Admins can create/manage groups via Admin UI in V1. OAuth provider sync is deferred.
- R8. **User-group membership.** Many-to-many `user_groups` table. In V1, managed manually by admins via UI. In future, syncable from Entra ID/Google groups.

### Marketplace UI (Separate Next.js App)

- R9. **Separate Next.js app with shadcn/ui.** New app (not integrated into landing-page or docs-site). Uses same shadcn/ui component library. Contains: login/registration pages, marketplace browse, admin dashboard.
- R10. **Auth pages rendered by the app, driven by Kratos flows.** Login, registration, recovery, and verification pages fetch Kratos flow objects and render form fields. Uses `@ory/client` SDK. Consent screen for Hydra OAuth2 flows also in this app.
- R11. **Marketplace browse page.** Lists all skills available in the public registry (synced from Git). Search by name/description. Each skill card shows: name, description, provider compatibility, and a copy-to-clipboard `skillbox add <skill_name>` command.
- R12. **Admin dashboard.** Shows pending approval requests. Admin can approve (add to enterprise registry) or reject with comment. Shows list of approved/rejected skills. Basic user management (view users, assign roles). Group management (create/edit groups, assign users).
- R13. **Approval notification via DB.** When a developer requests a public skill, an `approval_requests` record is created. Admin UI shows a badge/counter for pending requests. No email or webhook in V1.

### CLI Installation Flow

- R14. **`skillbox login` command.** Initiates Device Authorization Grant via Hydra. Shows verification URL and user code. Polls for token. On success, stores access token + refresh token in `~/.config/skillbox/credentials.json`. Shows "Logged in as <email>" confirmation.
- R15. **`skillbox add <skill_name>` command.** Main installation command. Flow:
  1. Check if logged in (token exists and valid). If not, prompt to run `skillbox login`.
  2. Resolve skill from registry API (name lookup).
  3. If skill is already approved for user's tenant → proceed to install.
  4. If skill is from public registry and not yet approved → create approval request via API, show "Pending approval. Run `skillbox add <skill_name>` again after admin approves." and exit.
  5. If approved → download skill archive, extract SKILL.md and assets.
  6. Ask which provider to install for (V1: Claude Code only, auto-detected).
  7. Copy skill files to correct path (Claude Code: `.claude/skills/<skill_name>/` in project or `~/.claude/skills/<skill_name>/` globally).
  8. Show success message with usage hint.
- R16. **`skillbox list` command.** Show installed skills with source, version, and installed-at timestamp. Read from local lock file.
- R17. **`skillbox remove <skill_name>` command.** Remove installed skill files and update lock file.
- R18. **Lock file at `~/.config/skillbox/skill-lock.json`.** Tracks installed skills with source, install path, installed-at, and content hash for update detection.
- R19. **Provider path for Claude Code.** Project scope: `.claude/skills/<skill_name>/SKILL.md`. Global scope: `~/.claude/skills/<skill_name>/SKILL.md`. Default to project scope, `--global` flag for global.

### Approval Workflow

- R20. **Approval requests table.** Fields: `id`, `tenant_id`, `user_id`, `skill_name`, `skill_version`, `source` (public/marketplace), `status` (pending/approved/rejected), `reviewed_by`, `review_comment`, `created_at`, `reviewed_at`.
- R21. **Enterprise skills bypass approval.** Skills uploaded directly to the tenant's own registry (via existing `skill push`) are immediately available to all tenant users without approval.
- R22. **Public skills require admin approval.** When a user requests a public marketplace skill, it enters `pending` status. Admin approves → skill becomes available to all users in the tenant (added to tenant's approved skills list). Admin can also approve only for the requesting user (future: per-group).
- R23. **Security scan on approval.** When admin approves a public skill, the existing security scanner pipeline runs before the skill is added to the tenant registry. If scan fails, admin is notified and must explicitly override or reject.

### Infrastructure & Dev Setup

- R24. **Docker Compose for local development.** Single `docker-compose up` starts: PostgreSQL, Ory Kratos, Ory Hydra, MailSlurper (dev email), Skillbox API server, Marketplace UI (Next.js). Everything works locally without internet or external OAuth providers.
- R25. **Database migrations.** New tables (users, groups, user_groups, approval_requests) added via SQL migrations. Existing tables (skills, executions, api_keys) unchanged. Groups schema included from day one even if group features are minimal in V1.

## Success Criteria

- A developer can register, log in via the Marketplace UI, browse skills, copy `skillbox add <name>`, authenticate via CLI, and install an approved skill to their Claude Code setup — end-to-end in under 5 minutes
- An admin can see pending approval requests in the Admin UI and approve/reject them
- The entire stack runs locally via `docker-compose up` with zero external dependencies
- Existing API-key-based auth continues to work unchanged for CI/CD and SDK usage
- Entra ID can be added later by editing Kratos config (no code changes needed)

## Scope Boundaries

**In scope (V1):**
- Ory Kratos + Hydra self-hosted with Docker Compose
- Email/password registration and login
- Device Authorization Grant for CLI
- Marketplace browse UI with skill search
- Admin UI for approval management and basic user/group management
- CLI commands: `login`, `add`, `list`, `remove`
- Claude Code as only provider
- Approval workflow (public skills require admin approval)
- Groups schema (tables created, basic CRUD via admin UI)

**Out of scope (V1):**
- Entra ID / Google SSO configuration (architecture supports it, not configured)
- SCIM group sync from OAuth providers
- Email/Slack/webhook notifications for approvals
- Multiple provider support (Cursor, Copilot, Windsurf)
- Skill update/upgrade detection
- Skill dependency resolution
- Per-group skill assignment (schema ready, logic deferred)
- Execution layer / cloud sandbox for skill testing
- Permission annotations on tools (Read/Write/Admin)
- Central command allowlist configuration
- `skillbox search` / `skillbox find` interactive search
- Audit log (separate workstream)
- Rate limiting (separate P0 workstream)

## Key Decisions

- **Ory Kratos + Hydra (not Auth0, Keycloak, or Supabase Auth):** Open source, self-hosted, locally testable without internet. Kratos handles identity, Hydra provides OAuth2/OIDC. Entra ID/Google added later as upstream Kratos OIDC providers — no code changes, only config.
- **Separate Next.js app for Marketplace UI:** Decoupled from landing page and docs site. Independent deployment, own concerns. Shares shadcn/ui design system.
- **Device Authorization Grant for CLI auth:** No localhost HTTP server needed in CLI. User authenticates in their normal browser. Works in SSH sessions and headless environments.
- **Claude Code only in V1:** Simplifies provider logic. Skills go to `.claude/skills/`. Other providers added later with same pattern as vercel-labs/skills.
- **Groups schema from day one:** `groups` and `user_groups` tables created in initial migration. Minimal CRUD in admin UI. Full group-based skill access control deferred but schema is ready.
- **Approval = DB entry + Admin UI poll:** No push notifications in V1. Admin checks dashboard. Simple and no external dependencies.
- **CLI exits on pending approval:** No polling or blocking wait. Developer runs `skillbox add` again after approval. Clean, simple UX.

## Dependencies / Assumptions

- Ory Kratos v1.3+ and Hydra v2.3+ (both support Device Authorization Grant since Ory OSS v25.4.0)
- Docker and Docker Compose available for local development
- Existing PostgreSQL instance extended with new tables (or Ory services use separate databases)
- Public skill marketplace data already synced from Git to Skillbox DB (existing functionality)
- The `@ory/client` npm package available for Next.js Kratos integration
- shadcn/ui and Next.js patterns from landing-page can be reused (copy component configs)

## Outstanding Questions

### Resolve Before Planning
_(None — all blocking product decisions resolved)_

### Deferred to Planning
_(All resolved during planning — see `docs/plans/2026-03-19-001-feat-enterprise-skill-registry-plan.md` "Deferred Questions Resolved" table)_

- ~~[Affects R1] Ory version pinning~~ → Kratos v1.3.0, Hydra v2.3.0
- ~~[Affects R4] JWT vs API-key detection~~ → `eyJ` prefix + 2 dot check
- ~~[Affects R5] User-to-tenant mapping~~ → Skillbox `users` table + invite codes
- ~~[Affects R9] Marketplace app location~~ → `marketplace/` at project root
- ~~[Affects R10] Kratos UI rendering~~ → Manual rendering with shadcn (no `@ory/elements`)
- ~~[Affects R15] Skill resolution~~ → CLI calls Skillbox API (server-side approval check)
- ~~[Affects R19] Claude Code skill path~~ → `.claude/skills/<name>/SKILL.md`
- ~~[Affects R24] Ory database setup~~ → Same Postgres, separate databases (`skillbox_kratos`, `skillbox_hydra`)

## Next Steps

→ Plan: `docs/plans/2026-03-19-001-feat-enterprise-skill-registry-plan.md`
→ Phase 1 (Infrastructure & Auth Foundation) implemented 2026-03-19
