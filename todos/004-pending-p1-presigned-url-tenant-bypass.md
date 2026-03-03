---
status: pending
priority: p1
issue_id: "004"
tags: [code-review, security, tenant-isolation, s3]
dependencies: []
---

# Fix Presigned URL Tenant Isolation Bypass

## Problem Statement

Presigned S3 URLs for artifacts are generated based on execution ID without verifying that the requesting tenant owns that execution. Any tenant with a valid API key could guess/enumerate execution IDs to access other tenants' artifacts.

## Findings

- `internal/api/execution_handler.go` generates presigned URLs for artifact download
- Execution ID is the only lookup key — no tenant_id check in the query
- `internal/store/execution_store.go` queries: `SELECT * FROM executions WHERE id = $1` — no tenant filter
- Presigned URLs have a TTL but once generated, anyone with the URL can download
- Execution IDs appear to be UUIDs (hard to guess but not impossible)

**Affected files:**
- `internal/api/execution_handler.go` — presigned URL generation
- `internal/store/execution_store.go` — execution queries missing tenant filter
- `internal/store/s3_store.go` — presigned URL generation

## Proposed Solutions

### Option 1: Add Tenant Filter to All Execution Queries

**Approach:** Add `AND tenant_id = $2` to all execution store queries. Pass tenant_id from the authenticated request context.

**Pros:**
- Simple, effective fix
- Consistent with how skill queries should work
- Defense in depth

**Cons:**
- Need to audit all execution queries

**Effort:** 1-2 hours
**Risk:** Low

## Recommended Action

*To be filled during triage.*

## Acceptance Criteria

- [ ] All execution queries include tenant_id filter
- [ ] Presigned URL generation verifies tenant ownership
- [ ] Returns 404 (not 403) for cross-tenant access attempts
- [ ] Tests verify tenant isolation

## Work Log

### 2026-03-03 - Initial Discovery

**By:** Claude Code (code review)

**Actions:**
- Security Sentinel identified presigned URL tenant bypass
- Data Integrity Guardian confirmed missing tenant filters in execution queries
- Verified execution_store.go queries lack tenant_id WHERE clause

**Learnings:**
- Return 404 instead of 403 to avoid information leakage
