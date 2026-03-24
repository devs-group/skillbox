---
status: complete
priority: p2
issue_id: "027"
tags: [code-review, security, scanner, information-disclosure]
dependencies: []
---

# Reduce Scan Detail in Client Error Responses to Prevent Evasion

## Problem Statement

When a scan blocks an upload, the response includes exact category names (`reverse_shell`, `typosquat_package`, `malware_detected`). This tells an attacker exactly which detection rule triggered, enabling iterative evasion: submit → see which rule caught it → modify → resubmit. Similarly, internal error details (`err.Error()`) are leaked to clients.

## Findings

- `skill.go:146-166` — returns `"categories": categories` in error response
- `skill.go:49,69,112,173` — raw `err.Error()` in responses leaks internal details
- Validate endpoint (`POST /v1/skills/validate`) makes evasion probing even easier

**Affected files:**
- `internal/api/handlers/skill.go:146-166` — category exposure
- `internal/api/handlers/skill.go:49,69,112,173` — error detail leakage

## Proposed Solutions

### Option A: Return only scan_id, log details server-side (Recommended)

Return generic "upload rejected by security scan" with `scan_id`. Store detailed findings server-side, retrievable only by admins. Wrap all `err.Error()` in generic messages.

- **Effort:** Small
- **Risk:** Low — changes API response shape

## Acceptance Criteria

- [ ] Block responses contain scan_id but not category names
- [ ] Error responses use generic messages without err.Error()
- [ ] Detailed findings logged server-side with scan_id
- [ ] Admin can retrieve details via scan_id (future endpoint)

## Work Log

| Date | Action | Learnings |
|------|--------|-----------|
| 2026-03-06 | Created from code review | Flagged by security-sentinel |
