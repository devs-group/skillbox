---
status: pending
priority: p2
issue_id: "011"
tags: [code-review, security, tenant-isolation, database]
dependencies: []
---

# Add Tenant ID Filters to All SQL Queries

## Problem Statement

Several SQL queries in the store layer are missing `tenant_id` filters. This means data from all tenants is accessible if an execution ID, file ID, or skill ID can be guessed or enumerated.

## Findings

- `GetExecution` — no tenant_id filter (see also issue 004)
- `ListFileVersions` — filters by file_id but not tenant_id
- `GetFile` — filters by file_id but not tenant_id
- `DeleteFile` — filters by file_id but not tenant_id
- Pattern: primary key lookups skip tenant_id, list queries include it
- Tenant ID is available in request context from auth middleware

**Affected files:**
- `internal/store/execution_store.go` — execution queries
- `internal/store/file_store.go` — file queries
- `internal/store/skill_store.go` — some skill queries

## Proposed Solutions

### Option 1: Add Tenant Filter to All Queries

**Approach:** Audit every SQL query and add `AND tenant_id = $N` where missing. Add tenant_id as required parameter to all store method signatures.

**Pros:**
- Complete fix
- Simple to implement
- Easy to verify

**Cons:**
- Need to touch many methods
- May break existing tests

**Effort:** 2-3 hours
**Risk:** Low

## Recommended Action

*To be filled during triage.*

## Acceptance Criteria

- [ ] Every store query includes tenant_id filter
- [ ] No cross-tenant data access possible via any API endpoint
- [ ] Tests verify tenant isolation for each entity type
- [ ] SQL review confirms no missing filters

## Work Log

### 2026-03-03 - Initial Discovery

**By:** Claude Code (code review)

**Actions:**
- Data Integrity Guardian found tenant_id missing from multiple queries
- Security Sentinel confirmed cross-tenant access risk
- Audited all store files for tenant_id usage patterns

**Learnings:**
- Primary key lookups commonly skip tenant checks — this is the gap pattern
