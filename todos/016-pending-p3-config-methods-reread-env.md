---
status: pending
priority: p3
issue_id: "016"
tags: [code-review, quality, config, bug]
dependencies: []
---

# Fix Config Methods That Re-Read Environment Variables

## Problem Statement

`Config.DefaultMemoryStr()` and `Config.DefaultCPUStr()` re-read environment variables at call time instead of returning the values parsed during `Config.Load()`. This means the config struct's parsed values and these methods can disagree if env vars change between startup and call time.

## Findings

- `DefaultMemoryStr()` calls `envOrDefault("SKILLBOX_DEFAULT_MEMORY", "256Mi")` — re-reads env
- `DefaultCPUStr()` calls `envOrDefault("SKILLBOX_DEFAULT_CPU", "500m")` — re-reads env
- `Config.DefaultMemory` (int64 bytes) is parsed from the same env var during Load()
- If env changes at runtime, `DefaultMemory` and `DefaultMemoryStr()` return different values
- These methods are used to pass resource strings to OpenSandbox

**Affected files:**
- `internal/config/config.go:52-60` — DefaultMemoryStr, DefaultCPUStr methods

## Proposed Solutions

### Option 1: Store Raw Strings During Load

**Approach:** Add `defaultMemoryRaw` and `defaultCPURaw` string fields to Config. Set during Load(). Return from methods.

**Pros:**
- Simple fix
- Consistent values
- No behavior change at startup

**Cons:**
- Two fields for same concept (parsed + raw)

**Effort:** 15 minutes
**Risk:** Low

## Recommended Action

*To be filled during triage.*

## Acceptance Criteria

- [ ] DefaultMemoryStr() returns value consistent with DefaultMemory
- [ ] DefaultCPUStr() returns value consistent with DefaultCPU
- [ ] No environment re-reads after Load()

## Work Log

### 2026-03-03 - Initial Discovery

**By:** Claude Code (code review)

**Actions:**
- Code Simplicity Reviewer identified env re-reading pattern
- Verified the values are used for OpenSandbox resource requests

**Learnings:**
- Config should be immutable after Load() — env should only be read once
