---
date: 2026-03-19
topic: enterprise-skill-registry
focus: Meeting notes — Enterprise Skill Registry with Ory auth, CLI, approval workflows
---

# Ideation: Enterprise Skill Registry

## Codebase Context

- **Stack:** Go 1.25 + Gin API, Cobra CLI, PostgreSQL, MinIO/S3, Docker Sandbox
- **Auth (before):** API-key only (SHA-256 hashed, bound to TenantID), no OAuth/user/group concept
- **Sandbox:** Fully hardened Docker (6-layer, network:none, non-configurable)
- **Scanner Pipeline:** Upload pipeline with normalize → validate → scan → upload
- **Critical Gaps:** No rate limiting, no HTTP timeouts, no async execution, 8 packages without tests
- **Learnings:** Sentinel error translation required at package boundaries; ContextKeyTenantID contract must be preserved

## Ranked Ideas

### 1. Enterprise Identity Foundation (Auth Provider Abstraction + CLI OAuth + Dual-Mode Middleware)
**Description:** Ory Kratos (identity) + Hydra (OAuth2/OIDC), dual-mode auth middleware (JWT + API-key), `skillbox login` with Device Authorization Grant (RFC 8628). Extends existing context key contract.
**Rationale:** Without user identity, nothing in the enterprise vision works — no approvals, no groups, no audit. Auth middleware is the single integration point.
**Downsides:** JWT validation latency (mitigated by Hydra introspection caching). Entra ID claims require provider-specific mapping.
**Confidence:** 95%
**Complexity:** Medium-High
**Status:** Explored — brainstormed, planned, Phase 1 implemented (2026-03-19)

### 2. Intra-Tenant User & Group Model
**Description:** `users` table (linked to Kratos identity + tenant), `groups` table (tenant-scoped, external_id for future OAuth sync), `user_groups` mapping. Roles: admin/publisher/consumer. Invite code system for tenant assignment.
**Rationale:** Meeting decision: "Datenschema fur Gruppen bereits bei Planung berucksichtigen." Foundation for approval workflows and RBAC.
**Downsides:** Schema migration on existing tenants. Group sync from OAuth requires future work.
**Confidence:** 92%
**Complexity:** Medium
**Status:** Explored — brainstormed, planned, Phase 1 implemented (2026-03-19)

### 3. Skill Visibility + Intelligent Approval Pipeline
**Description:** `visibility` field (private/enterprise/public), `status` field (pending/approved/rejected). Enterprise skills bypass approval. Public skills require admin approval. Policy-as-code auto-approval for future. Quarantine namespace instead of hard-reject.
**Rationale:** Core meeting requirement. Scanner pipeline has clear insertion points.
**Downsides:** State machine complexity. Policy engine design needed for auto-approve.
**Confidence:** 88%
**Complexity:** High
**Status:** Explored — brainstormed, planned, implementation pending (Phase 2)

### 4. Immutable Audit Log
**Description:** `audit_events` table (append-only): actor, tenant, action, resource, timestamp, outcome. Query API with filters. Optional SIEM webhook.
**Rationale:** Compliance blocker for regulated industries. Currently only `log.Printf` to stdout.
**Downsides:** Table growth requires retention policy. Event schema must be stable.
**Confidence:** 90%
**Complexity:** Low-Medium
**Status:** Unexplored — deferred to separate workstream

### 5. Testability Foundation (Store Interface + Integration Test Harness)
**Description:** Extract `store.Querier` interface. Shared test harness with in-process server, test DB, pre-provisioned fixtures.
**Rationale:** 8 packages with zero tests. Code comments say "Skipping until store accepts interface."
**Downsides:** Refactoring effort. Requires discipline to use harness consistently.
**Confidence:** 85%
**Complexity:** Medium
**Status:** Unexplored

### 6. Rate Limiting + HTTP Timeouts (P0 Security Hardening)
**Description:** Tenant-scoped token-bucket rate limiter (Redis-backed). HTTP ReadTimeout/WriteTimeout/IdleTimeout. Graceful shutdown.
**Rationale:** P0 gaps that must be fixed before OAuth rollout. Redis already in config but unused.
**Downsides:** Rate-limit config per tier requires tenant-tier concept.
**Confidence:** 93%
**Complexity:** Medium
**Status:** Unexplored — deferred to separate P0 workstream

### 7. Tiered Sandbox Profiles with Network Policies
**Description:** Named sandbox profiles (default/high-memory/trusted/cloud). Skills declare egress rules in SKILL.md. Admin allowlist cap. Cloud execution as default for enterprise.
**Rationale:** Meeting: "Execution Layer fur interne Tests" and "Skills auf Cloud statt lokal." NetworkPolicy structs exist but are unused.
**Downsides:** Network config increases attack surface. Cloud execution requires separate deployment.
**Confidence:** 78%
**Complexity:** High
**Status:** Unexplored

## Rejection Summary

| # | Idea | Reason Rejected |
|---|------|-----------------|
| 1 | `skillbox init` Scaffold | Nice-to-have DX, not strategic for registry MVP |
| 2 | CLI Cobra to urfave Migration | Team convention, no enterprise value |
| 3 | SemVer Enforcement | Good practice, not MVP-critical |
| 4 | SCIM Group Provisioning | Over-engineered for MVP |
| 5 | Git PR-Based Approval | Adds complexity; meeting wants UI |
| 6 | Content-Addressable Skill Mesh | Too disruptive for MVP |
| 7 | Federated Skill Resolution | Premature for single deployment |
| 8 | Persistent Warm Pools | Performance optimization, not governance |
| 9 | Skill Composition/Chaining | Scope creep toward workflow engine |
| 10 | Scoped Tokens Instead of Groups | Meeting explicitly decided on groups |
| 11 | OCI Artifacts Instead of Zip | Large migration, unclear near-term benefit |
| 12 | Fused Normalize+Scan | Micro-optimization |
| 13 | Auto-Derived Permissions | Increases upload complexity |
| 14 | Auto-Generated Allowlists | Too complex for MVP |
| 15 | Skills as Inline Functions | Partially implemented via /v1/skills/from-fields |
| 16 | Cloud-First Default | Vision, not MVP — folded into Sandbox Profiles |
| 17 | OpenAPI Contract Tests | Good practice, not registry-specific |
| 18 | Async Execution | Important but separate workstream |
| 19 | Execution Cost Tracking | Later analytics phase |
| 20 | Dependency Pinning/Supply Chain | Separate security hardening |
| 21 | Skill Search/Discovery | Important but after auth/governance foundation |
| 22 | Capability-Based Sandbox Instead of Approval | Too radical; meeting wants approval |
| 23 | Composable Middleware | Implementation detail |
| 24 | Unified Error Catalog | Code quality, not feature |
| 25 | API Key Self-Service | Subsumed by auth provider work |

## Session Log
- 2026-03-19: Initial ideation — 48 candidates generated (6 agents), 7 survived
- 2026-03-19: Ideas #1 and #2 explored via /ce:brainstorm → requirements doc
- 2026-03-19: Plan created via /ce:plan → 5-phase implementation plan
- 2026-03-19: Phase 1 implemented — Ory configs, Docker Compose, DB migration, dual-mode auth middleware, store layer (users, groups, approvals, invites)
