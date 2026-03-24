---
status: complete
priority: p2
issue_id: "022"
tags: [code-review, security, scanner, authorization]
dependencies: []
---

# Add Admin Authorization to Scanner Stats Endpoint

## Problem Statement

`GET /v1/admin/scanner/stats` uses the same `AuthMiddleware` + `TenantMiddleware` as regular user endpoints. Any authenticated tenant can access scanner metrics (total scans, blocked scans, failure rates, block category distributions), exposing operational intelligence that helps attackers craft evasion payloads.

## Findings

- `router.go:54` — no admin-only middleware
- Metrics expose: which categories are caught, whether external scanners are active, block rates
- This is an information disclosure vulnerability

**Affected files:**
- `internal/api/router.go:54`

## Proposed Solutions

### Option A: Add admin-only middleware (Recommended)

Add an `admin` flag to the API key record and create `AdminMiddleware` that checks it.

- **Effort:** Medium
- **Risk:** Low

### Option B: Move stats to a separate admin port

Run admin endpoints on a different port (e.g., :9090) not exposed publicly.

- **Effort:** Medium
- **Risk:** Low

## Acceptance Criteria

- [ ] Scanner stats endpoint is only accessible to admin API keys
- [ ] Regular tenants get 403 Forbidden
- [ ] Admin auth tested

## Work Log

| Date | Action | Learnings |
|------|--------|-----------|
| 2026-03-06 | Created from code review | Flagged by security-sentinel |
