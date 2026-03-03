---
status: pending
priority: p2
issue_id: "013"
tags: [code-review, observability, logging, patterns]
dependencies: []
---

# Migrate All Logging to slog

## Problem Statement

The codebase uses two logging systems: `log.Printf` (17 call sites, mostly in runner.go) and `log/slog` (used elsewhere). This creates inconsistent log output — `log.Printf` lacks structured fields, levels, and timestamps that `slog` provides, making production debugging harder.

## Findings

- 17 `log.Printf` call sites in `internal/runner/runner.go`
- `slog` is properly configured in `cmd/skillbox-server/main.go`
- Other packages use `slog` correctly with structured fields
- `log.Printf` output bypasses slog handlers (different format, no JSON output)
- Runner is the most critical component to have good logging (execution orchestration)

**Affected files:**
- `internal/runner/runner.go` — 17 log.Printf sites to migrate

## Proposed Solutions

### Option 1: Direct Migration to slog

**Approach:** Replace all `log.Printf` calls with equivalent `slog.Info`/`slog.Error`/`slog.Debug` calls. Add structured fields (execution_id, skill_name, tenant_id).

**Pros:**
- Simple, mechanical change
- Adds structured context to logs
- Consistent output format

**Cons:**
- Need to decide appropriate log levels for each call

**Effort:** 1-2 hours
**Risk:** Low

## Recommended Action

*To be filled during triage.*

## Acceptance Criteria

- [ ] Zero `log.Printf` calls remain in the codebase
- [ ] All log lines include structured fields (execution_id, skill_name where applicable)
- [ ] Log levels are appropriate (Info for normal, Error for failures, Debug for verbose)
- [ ] JSON log output works in production mode

## Work Log

### 2026-03-03 - Initial Discovery

**By:** Claude Code (code review)

**Actions:**
- Pattern Recognition Specialist found 17 log.Printf sites in runner.go
- Confirmed slog is the intended logging framework
- Architecture Strategist noted this is a consistency issue

**Learnings:**
- runner.go is the most critical package to have structured logging
