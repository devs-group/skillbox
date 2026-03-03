---
status: pending
priority: p1
issue_id: "003"
tags: [code-review, security, dos, sandbox]
dependencies: []
---

# Enforce Server-Side Resource Limits for Sandbox Execution

## Problem Statement

SKILL.md metadata allows callers to specify `memory`, `cpu`, and `timeout` values that are passed directly to OpenSandbox without server-side caps. A malicious skill could request unlimited memory/CPU, causing resource exhaustion on the sandbox infrastructure.

## Findings

- `internal/runner/runner.go` reads resource limits from SKILL.md metadata
- Config has `DefaultMemory`, `DefaultCPU`, `MaxTimeout` but no `MaxMemory` or `MaxCPU`
- User-provided values override defaults without capping
- `MaxTimeout` exists but is only checked against `DefaultTimeout`, not against per-request values
- A skill requesting `memory: 64Gi` and `cpu: 32` would be honored

**Affected files:**
- `internal/runner/runner.go:Execute()` — resource limit application
- `internal/config/config.go` — missing MaxMemory/MaxCPU config
- `internal/api/execution_handler.go` — no validation of requested resources

## Proposed Solutions

### Option 1: Add Max Resource Caps in Config

**Approach:** Add `SKILLBOX_MAX_MEMORY` and `SKILLBOX_MAX_CPU` config values. Clamp all user-provided values to these caps in the runner before passing to OpenSandbox.

**Pros:**
- Simple, effective defense
- Configurable per deployment
- Matches existing MaxTimeout pattern

**Cons:**
- Requires redeployment to change limits

**Effort:** 1-2 hours
**Risk:** Low

## Recommended Action

*To be filled during triage.*

## Acceptance Criteria

- [ ] MaxMemory and MaxCPU config values added
- [ ] User-requested resources clamped to server-side maximums
- [ ] Warning logged when user values are clamped
- [ ] Tests verify clamping behavior

## Work Log

### 2026-03-03 - Initial Discovery

**By:** Claude Code (code review)

**Actions:**
- Security Sentinel identified unbounded resource limits from SKILL.md
- Performance Oracle confirmed no server-side caps exist
- Config analysis shows MaxTimeout exists but MaxMemory/MaxCPU do not

**Learnings:**
- Follow the same pattern as MaxTimeout for consistency
