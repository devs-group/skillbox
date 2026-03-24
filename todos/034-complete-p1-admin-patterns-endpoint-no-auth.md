---
status: complete
priority: p1
issue_id: "034"
tags: [code-review, security, scanner, authorization, privilege-escalation]
dependencies: ["022"]
---

# Admin Pattern Endpoints Allow Any Tenant to Rewrite Scanner Rules

## Problem Statement

`PUT /v1/admin/scanner/patterns` and `GET /v1/admin/scanner/patterns` use the same `AuthMiddleware` + `TenantMiddleware` as regular user endpoints. Any authenticated tenant can read the current scanner patterns (revealing what the scanner looks for, enabling evasion) and **replace all custom patterns**, effectively neutering the scanner for ALL tenants.

This is a privilege escalation vulnerability. An attacker with a valid API key can upload patterns that remove all block rules, then upload a malicious skill that passes scanning.

**Escalation of todo 022** (which only covers the stats endpoint). The patterns PUT endpoint is significantly more dangerous.

## Findings

- `router.go:54-56` -- admin routes share the same middleware group as regular `/v1/*` routes
- `handlers/skill.go:ScannerSetPatterns` -- accepts and applies any valid PatternFile with no role check
- `handlers/skill.go:ScannerGetPatterns` -- returns full pattern definitions including regexes
- Security Sentinel: P1-1, Architecture Strategist: 4.3

## Proposed Solutions

### Option A: Add Admin Middleware (Recommended)
Create an `AdminMiddleware` that checks an `is_admin` or `role` field on the API key record. Apply it to the `/v1/admin/*` group.

- **Pros:** Clean separation, reusable for future admin endpoints
- **Cons:** Requires schema change if API keys don't have a role field
- **Effort:** Medium
- **Risk:** Low

### Option B: Separate Admin API with Different Auth
Create a separate `/admin/*` route group with a distinct admin token (e.g., env var `SKILLBOX_ADMIN_TOKEN`).

- **Pros:** Simple, no DB changes needed
- **Cons:** Less flexible, single shared admin token
- **Effort:** Small
- **Risk:** Low

## Acceptance Criteria

- [ ] Only admin-authorized API keys can call PUT /v1/admin/scanner/patterns
- [ ] Only admin-authorized API keys can call GET /v1/admin/scanner/patterns
- [ ] Regular tenant API keys receive 403 on admin endpoints
- [ ] Test coverage for authorization check
