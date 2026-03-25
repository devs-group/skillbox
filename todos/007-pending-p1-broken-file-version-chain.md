---
status: complete
priority: p1
issue_id: "007"
tags: [code-review, data-integrity, database, files]
dependencies: []
---

# Fix Broken File Version Chain Query

## Problem Statement

The file version chain query uses recursive CTE or self-joins that break for chains deeper than the query's hardcoded depth. Files with many versions may lose history, and the "latest version" resolution may return incorrect results.

## Findings

- `internal/store/file_store.go` has version chain traversal via `parent_id`
- Deep version chains (>10 updates) may not be fully traversed
- `ListFileVersions` query doesn't paginate or limit depth correctly
- `GetLatestVersion` walks the chain — O(n) for n versions
- No index on `parent_id` column — chain traversal is slow

**Affected files:**
- `internal/store/file_store.go` — `ListFileVersions()`, `GetLatestVersion()`
- Database schema — missing index on `files.parent_id`

## Proposed Solutions

### Option 1: Recursive CTE with Proper Termination

**Approach:** Replace chain walking with a proper recursive CTE (`WITH RECURSIVE`) that terminates correctly. Add index on `parent_id`.

**Pros:**
- Single query instead of chain walking
- Database handles recursion efficiently
- Works for any depth

**Cons:**
- PostgreSQL-specific syntax
- Need to add cycle detection

**Effort:** 2-3 hours
**Risk:** Low

---

### Option 2: Denormalize with Version Number Column

**Approach:** Add `version_number` integer column to files table. Increment on each update. Query latest by `ORDER BY version_number DESC LIMIT 1`.

**Pros:**
- Simple queries
- No recursion needed
- O(1) latest version lookup

**Cons:**
- Schema migration needed
- Need to handle concurrent version creation
- Backfill existing data

**Effort:** 3-4 hours
**Risk:** Medium

## Recommended Action

*To be filled during triage.*

## Acceptance Criteria

- [ ] File version chains of arbitrary depth are fully traversable
- [ ] GetLatestVersion returns correct result for deep chains
- [ ] parent_id column has database index
- [ ] Performance acceptable for 100+ version chains
- [ ] Tests cover deep version chain scenarios

## Work Log

### 2026-03-03 - Initial Discovery

**By:** Claude Code (code review)

**Actions:**
- Data Integrity Guardian identified broken version chain traversal
- Performance Oracle noted missing index on parent_id
- Verified chain walking is O(n) with no depth limit protection

**Learnings:**
- Recursive CTEs in PostgreSQL are efficient and handle cycles with UNION vs UNION ALL
