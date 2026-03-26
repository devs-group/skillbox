---
status: complete
priority: p1
issue_id: "035"
tags: [code-review, security, scanner, denial-of-service]
dependencies: ["034"]
---

# No Limits on Custom Pattern Count or Length (DoS Vector)

## Problem Statement

When custom patterns are uploaded via `PUT /v1/admin/scanner/patterns`, there is no validation of pattern count or regex string length. An attacker (or misconfigured admin) can upload a `PatternFile` with millions of entries or extremely long regexes, causing:

1. Memory exhaustion during pattern compilation
2. Extreme scan latency (every file scanned against every pattern)
3. Effective denial of service for all skill uploads

While Go's RE2 engine prevents catastrophic backtracking, the combinatorial cost of N patterns x M files is still a practical DoS.

## Findings

- `pattern_loader.go:132-147` -- `compilePatternFile` compiles all patterns without limits
- `handlers/skill.go:526-580` -- `ScannerSetPatterns` passes data directly to `ParsePatternData` with no size checks
- Security Sentinel: P1-2

## Proposed Solutions

### Option A: Enforce Limits in Handler (Recommended)
Add validation in `ScannerSetPatterns` before calling `SetCustomPatterns`:
- Max 100 patterns per category (block/flag)
- Max 1024 characters per regex string
- Max 500 blocklist/popular packages
- Max 1MB total request body

- **Pros:** Simple, catches abuse early
- **Cons:** Limits may need tuning
- **Effort:** Small
- **Risk:** Low

### Option B: Compile with Timeout
Run `compilePatternFile` in a goroutine with a context timeout (e.g., 5 seconds).

- **Pros:** Catches pathological patterns regardless of count
- **Cons:** More complex, doesn't prevent memory exhaustion
- **Effort:** Medium
- **Risk:** Low

## Acceptance Criteria

- [ ] Pattern count per category is capped (configurable, default 100)
- [ ] Regex string length is capped (configurable, default 1024 chars)
- [ ] Package list lengths are capped
- [ ] Request body size is limited
- [ ] Tests for limit enforcement
