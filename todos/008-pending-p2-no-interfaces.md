---
status: pending
priority: p2
issue_id: "008"
tags: [code-review, architecture, testing, interfaces]
dependencies: []
---

# Extract Interfaces for Core Components

## Problem Statement

The codebase has no interfaces for core components (store, runner, S3 client). This makes unit testing impossible without a real database/S3 instance and prevents mocking in handler tests. It's the single biggest architectural gap.

## Findings

- `internal/api/` handlers accept concrete `*store.SkillStore`, `*store.FileStore` directly
- `internal/runner/runner.go` directly depends on concrete store and S3 implementations
- No test files exist that use mocks
- Integration tests require full infrastructure (PostgreSQL, MinIO)
- Adding new storage backends (e.g., switching from MinIO to GCS) requires modifying every consumer

**Affected files:**
- `internal/store/skill_store.go` — needs interface extraction
- `internal/store/file_store.go` — needs interface extraction
- `internal/store/s3_store.go` — needs interface extraction
- `internal/runner/runner.go` — should accept interfaces
- `internal/api/*.go` — all handlers should accept interfaces

## Proposed Solutions

### Option 1: Extract Interfaces at Consumer Site (Go Idiom)

**Approach:** Define small interfaces where they're consumed (in api/ and runner/ packages). Each consumer defines only the methods it needs.

**Pros:**
- Idiomatic Go (accept interfaces, return structs)
- Minimal interfaces per consumer
- Easy to mock in tests
- No circular dependencies

**Cons:**
- Multiple interface definitions (some overlap)
- More files to maintain

**Effort:** 4-6 hours
**Risk:** Low

---

### Option 2: Central Interface Definitions

**Approach:** Define interfaces in a `domain` or `port` package. All implementations satisfy these interfaces.

**Pros:**
- Single source of truth
- Clean hexagonal architecture
- Easy to discover all contracts

**Cons:**
- Larger interfaces than needed per consumer
- May pull in unnecessary dependencies
- Less idiomatic Go

**Effort:** 6-8 hours
**Risk:** Medium

## Recommended Action

*To be filled during triage.*

## Acceptance Criteria

- [ ] Core store operations have interface definitions
- [ ] S3 operations have interface definition
- [ ] Runner accepts interfaces instead of concrete types
- [ ] At least one handler has mock-based unit tests
- [ ] Test coverage increases measurably

## Work Log

### 2026-03-03 - Initial Discovery

**By:** Claude Code (code review)

**Actions:**
- Architecture Strategist identified lack of interfaces as biggest architectural gap
- Pattern Recognition confirmed no mock-based tests exist
- Code Simplicity reviewer noted tight coupling throughout

**Learnings:**
- Go idiom: define interfaces at the consumer site, not the provider
