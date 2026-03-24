---
status: complete
priority: p2
issue_id: "028"
tags: [code-review, security, scanner]
dependencies: []
---

# Fix Files >1MB Silently Skipping Pattern Scanning

## Problem Statement

Files larger than 1MB are silently skipped during pattern scanning. An attacker could pad a malicious script with comments/whitespace to exceed 1MB and evade all static pattern detection. The test `TestScan_LargeFileSkipped` validates this behavior, meaning it's intentional but exploitable.

## Findings

- `stage_patterns.go:194-200` — files >1MB skipped with only Debug log
- `stage_patterns.go:362-375` — `readZipFileContent` reads up to 1MB+1 but doesn't check truncation

**Affected files:**
- `internal/scanner/stage_patterns.go:194-200`

## Proposed Solutions

### Option A: Scan first 1MB instead of skipping (Recommended)

Read up to 1MB of each file and scan that. Most malicious patterns appear early in files.

- **Effort:** Small
- **Risk:** Low

### Option B: Emit FLAG finding for oversized files

Escalate oversized files to deeper tiers for scrutiny.

- **Effort:** Small
- **Risk:** Low

## Acceptance Criteria

- [ ] Files >1MB are partially scanned (first 1MB) OR flagged for escalation
- [ ] No silent skip of scannable content
- [ ] Tests updated

## Work Log

| Date | Action | Learnings |
|------|--------|-----------|
| 2026-03-06 | Created from code review | Flagged by security-sentinel |
