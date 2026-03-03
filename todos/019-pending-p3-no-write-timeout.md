---
status: pending
priority: p3
issue_id: "019"
tags: [code-review, security, http, timeout]
dependencies: []
---

# Add HTTP Server Write Timeout

## Problem Statement

The HTTP server is created without a `WriteTimeout`, making it vulnerable to slowloris-style attacks where a client reads the response very slowly, tying up server resources indefinitely.

## Findings

- `cmd/skillbox-server/main.go` uses Gin's default server setup
- No explicit `http.Server` with timeout configuration
- `ReadTimeout` may also be missing
- Without `WriteTimeout`, a slow client can hold a connection open forever
- Combined with synchronous execution (issue 005), this is amplified

**Affected files:**
- `cmd/skillbox-server/main.go` — server startup

## Proposed Solutions

### Option 1: Configure http.Server Timeouts

**Approach:** Create explicit `http.Server` with `ReadTimeout`, `WriteTimeout`, and `IdleTimeout`.

```go
srv := &http.Server{
    Addr:         ":" + cfg.APIPort,
    Handler:      router,
    ReadTimeout:  30 * time.Second,
    WriteTimeout: cfg.MaxTimeout + 30*time.Second,
    IdleTimeout:  120 * time.Second,
}
```

**Pros:**
- Standard Go best practice
- Protects against slowloris
- Simple implementation

**Cons:**
- WriteTimeout must be longer than MaxTimeout for execution responses
- Need to consider streaming responses

**Effort:** 15 minutes
**Risk:** Low

## Recommended Action

*To be filled during triage.*

## Acceptance Criteria

- [ ] HTTP server has ReadTimeout, WriteTimeout, and IdleTimeout configured
- [ ] WriteTimeout accommodates longest possible execution
- [ ] Slowloris attack no longer ties up connections indefinitely

## Work Log

### 2026-03-03 - Initial Discovery

**By:** Claude Code (code review)

**Actions:**
- Security Sentinel identified missing WriteTimeout
- Performance Oracle noted interaction with synchronous execution design

**Learnings:**
- WriteTimeout must exceed MaxTimeout for execution endpoint
