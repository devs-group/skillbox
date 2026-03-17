---
status: complete
priority: p2
issue_id: "023"
tags: [code-review, security, scanner, error-handling]
dependencies: []
---

# Fix Fail-Open readZipFileContent Error Handling Inconsistency

## Problem Statement

When `readZipFileContent` fails, stages handle it inconsistently. The pattern stage (Tier 1) returns an error (fail-closed), but the deps stage, prompt stage, and external stage silently `continue` (fail-open). A corrupted dependency file could bypass scanning entirely. This breaks the documented fail-closed contract.

## Findings

- `stage_patterns.go:204-207` — returns error (CORRECT, fail-closed)
- `stage_deps.go:48` — `continue` on error (WRONG, fail-open)
- `stage_prompt.go:106` — `continue` on error (WRONG, fail-open)
- `stage_external.go:57-59` — `continue` on error (WRONG, fail-open)

**Affected files:**
- `internal/scanner/stage_deps.go:48`
- `internal/scanner/stage_prompt.go:106`
- `internal/scanner/stage_external.go:57-59`

## Proposed Solutions

### Option A: Fail closed everywhere (Recommended)

Change all `continue` to `return nil, fmt.Errorf(...)` to match the pattern stage.

- **Effort:** Small
- **Risk:** Low — may increase false rejections on corrupted ZIPs, but that's the correct security tradeoff

## Acceptance Criteria

- [ ] All stages return error when readZipFileContent fails
- [ ] Behavior is consistent with pattern stage (fail-closed)
- [ ] Tests verify fail-closed behavior

## Work Log

| Date | Action | Learnings |
|------|--------|-----------|
| 2026-03-06 | Created from code review | Flagged by architecture-strategist, data-integrity-guardian |
