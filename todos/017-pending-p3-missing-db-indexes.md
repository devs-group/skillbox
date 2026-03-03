---
status: pending
priority: p3
issue_id: "017"
tags: [code-review, performance, database, indexes]
dependencies: []
---

# Add Missing Database Indexes

## Problem Statement

Several frequently-queried columns lack database indexes, causing full table scans under load. This will become a performance bottleneck as data grows.

## Findings

- `files.parent_id` — no index (used in version chain traversal)
- `files.skill_id` — no index (used in listing files per skill)
- `executions.skill_id` — no index (used in execution history)
- `executions.tenant_id` — no index (used in tenant filtering)
- `skills.tenant_id` — may or may not have index (needs verification)
- `api_keys.key_hash` — likely indexed as unique constraint, but should verify

**Affected files:**
- Database migration files
- `internal/store/file_store.go` — queries that would benefit
- `internal/store/execution_store.go` — queries that would benefit

## Proposed Solutions

### Option 1: Add Composite Indexes via Migration

**Approach:** Create a new goose migration adding composite indexes on the most-queried columns.

```sql
CREATE INDEX idx_files_skill_id ON files(skill_id);
CREATE INDEX idx_files_parent_id ON files(parent_id);
CREATE INDEX idx_executions_skill_id_tenant_id ON executions(skill_id, tenant_id);
CREATE INDEX idx_skills_tenant_id ON skills(tenant_id);
```

**Pros:**
- Significant query performance improvement
- Standard database optimization
- Non-destructive (additive only)

**Cons:**
- Index maintenance overhead on writes
- Migration needed

**Effort:** 30 minutes
**Risk:** Low

## Recommended Action

*To be filled during triage.*

## Acceptance Criteria

- [ ] All frequently-queried FK columns have indexes
- [ ] Migration runs successfully
- [ ] EXPLAIN plans show index usage for key queries
- [ ] Write performance not significantly impacted

## Work Log

### 2026-03-03 - Initial Discovery

**By:** Claude Code (code review)

**Actions:**
- Performance Oracle identified missing indexes on FK columns
- Data Integrity Guardian confirmed no FK cascade constraints exist
- Verified queries that would benefit from indexing

**Learnings:**
- Composite indexes (tenant_id, skill_id) are better than individual indexes for multi-tenant queries
