---
status: complete
priority: p1
issue_id: "001"
tags: [code-review, data-integrity, database, security]
dependencies: []
---

# Add Transaction Boundaries to Store Layer

## Problem Statement

The entire store layer has zero transaction boundaries. Multi-step operations (e.g., creating a skill version + uploading files + updating metadata) can leave the database in an inconsistent state if any step fails. This is the most critical data integrity issue in the codebase.

## Findings

- No `BEGIN`/`COMMIT`/`ROLLBACK` anywhere in the store package
- `CreateSkillVersion` + file uploads are not atomic — a failed upload leaves orphaned version records
- `DeleteSkill` doesn't cascade properly — files and versions can be orphaned
- `UpdateFile` with parent_id chain mutation is not atomic
- All store methods accept `*sql.DB` directly, making transaction injection impossible without refactoring

**Affected files:**
- `internal/store/skill_store.go` — all write methods
- `internal/store/file_store.go` — all write methods
- `internal/store/api_key_store.go` — write methods
- `internal/runner/runner.go` — orchestrates multi-step operations

## Proposed Solutions

### Option 1: Add Transaction Support via Context Pattern

**Approach:** Refactor store to accept `Querier` interface (wrapping both `*sql.DB` and `*sql.Tx`), add `WithTx()` method to store, wrap multi-step operations in transactions at the runner/handler level.

**Pros:**
- Clean, idiomatic Go pattern
- Backward compatible (non-transactional calls still work)
- Enables nested transaction support later

**Cons:**
- Requires touching every store method signature
- Moderate refactoring effort

**Effort:** 6-8 hours
**Risk:** Medium (large surface area change)

---

### Option 2: Transaction Wrapper Functions

**Approach:** Add `RunInTx(ctx, fn)` helper that creates a transaction and passes it to a closure. Only wrap the critical multi-step operations.

**Pros:**
- Less invasive than full refactor
- Can be done incrementally
- Focuses on highest-risk operations first

**Cons:**
- Doesn't enforce transactions everywhere
- May miss edge cases

**Effort:** 3-4 hours
**Risk:** Low

## Recommended Action

*To be filled during triage.*

## Acceptance Criteria

- [ ] Multi-step skill creation (version + files) is atomic
- [ ] Failed file uploads roll back the version record
- [ ] DeleteSkill removes all associated data atomically
- [ ] File updates with parent_id changes are atomic
- [ ] Integration tests verify rollback on failure

## Work Log

### 2026-03-03 - Initial Discovery

**By:** Claude Code (code review)

**Actions:**
- Data Integrity Guardian agent identified zero transaction usage across entire store layer
- Verified by searching for BEGIN/COMMIT/ROLLBACK patterns — none found
- Identified 4 critical multi-step operations that need transactional wrapping

**Learnings:**
- Store methods accept `*sql.DB` directly, preventing easy transaction injection
- The `Querier` interface pattern (used in sqlc-generated code) would be the cleanest approach
