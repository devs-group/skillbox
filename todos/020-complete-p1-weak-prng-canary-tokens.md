---
status: complete
priority: p1
issue_id: "020"
tags: [code-review, security, scanner, cryptography]
dependencies: []
---

# Replace math/rand with crypto/rand for LLM Canary Tokens

## Problem Statement

The LLM anti-hijacking canary token is generated using `math/rand`, which is not cryptographically secure. If an attacker can predict or brute-force the 8-character canary (36^8 ≈ 2.8 trillion), they can craft a SKILL.md that manipulates the LLM into returning a valid canary, defeating the entire prompt injection defense. Flagged by all 4 review agents (security, architecture, performance, data integrity).

## Findings

- `math/rand` is deterministic and predictable even with Go 1.20+ auto-seeding
- Canary is only 8 chars from 36-char alphabet — adequate entropy only with crypto PRNG
- `math/rand.Intn` uses a global mutex-protected source — contention under concurrent Tier 3 scans
- The canary token is the sole defense against LLM prompt hijacking

**Affected files:**
- `internal/scanner/stage_llm.go:351-358` — `randomAlphanumeric()` function

## Proposed Solutions

### Option A: Replace with crypto/rand (Recommended)

```go
import "crypto/rand"

func randomAlphanumeric(n int) string {
    const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
    b := make([]byte, n)
    if _, err := rand.Read(b); err != nil {
        panic("crypto/rand unavailable: " + err.Error())
    }
    for i := range b {
        b[i] = chars[b[i]%byte(len(chars))]
    }
    return string(b)
}
```

- **Pros:** Cryptographically secure, eliminates global lock contention, trivial change
- **Cons:** None meaningful
- **Effort:** Small (5 min)
- **Risk:** None

### Option B: Increase canary length + crypto/rand

Same as Option A but also increase canary from 8 to 16 characters (~83 bits entropy).

- **Pros:** Makes brute-force completely infeasible
- **Cons:** Slightly longer prompt
- **Effort:** Small
- **Risk:** None

## Acceptance Criteria

- [ ] `randomAlphanumeric` uses `crypto/rand` instead of `math/rand`
- [ ] `math/rand` import removed from `stage_llm.go`
- [ ] All existing LLM tests still pass
- [ ] Consider increasing canary length to 16 characters

## Work Log

| Date | Action | Learnings |
|------|--------|-----------|
| 2026-03-06 | Created from code review | Flagged by security-sentinel, architecture-strategist, performance-oracle, data-integrity-guardian |
