---
status: pending
priority: p3
issue_id: "015"
tags: [code-review, quality, dead-code, cleanup]
dependencies: []
---

# Remove Dead Code (~250 LOC)

## Problem Statement

The codebase contains approximately 250 lines of dead code — unused error sentinels, duplicate functions, and unreachable code paths. This increases cognitive load for developers and maintenance burden.

## Findings

- `ErrNotImplemented` — defined but never returned or checked
- `ErrTimeout` — defined but never returned (OpenSandbox handles timeouts)
- `ValidateKey` in api_key_store.go — duplicate of `GetAPIKeyByHash`, never called
- `InsertExecution` — duplicate of `CreateExecution`, never called
- `ErrImageNotAllowed` — defined and checked in handler but never returned by any function
- Duplicate `detectContentType` implementations in both skill_handler.go and files_handler.go

**Affected files:**
- `internal/runner/runner.go` — unused error sentinels
- `internal/store/api_key_store.go` — duplicate ValidateKey
- `internal/store/execution_store.go` — duplicate InsertExecution
- `internal/api/skill_handler.go` — duplicate detectContentType
- `internal/api/files_handler.go` — duplicate detectContentType

## Proposed Solutions

### Option 1: Remove All Dead Code in One Pass

**Approach:** Delete all identified dead code. Consolidate duplicates into shared utility functions.

**Pros:**
- Clean, simple
- Reduces ~250 LOC
- Single PR

**Cons:**
- Need to verify nothing uses the "dead" code at runtime

**Effort:** 1-2 hours
**Risk:** Low

## Recommended Action

*To be filled during triage.*

## Acceptance Criteria

- [ ] All identified dead code removed
- [ ] Duplicate detectContentType consolidated into one function
- [ ] All tests still pass
- [ ] No runtime references to removed code

## Work Log

### 2026-03-03 - Initial Discovery

**By:** Claude Code (code review)

**Actions:**
- Pattern Recognition Specialist found duplicate functions and dead error sentinels
- Code Simplicity Reviewer estimated ~250 LOC removable
- Verified no runtime usage of dead code via grep

**Learnings:**
- Dead code accumulates quickly in a young codebase
