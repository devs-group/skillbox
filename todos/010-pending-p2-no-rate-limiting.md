---
status: pending
priority: p2
issue_id: "010"
tags: [code-review, security, rate-limiting, api]
dependencies: []
---

# Add Rate Limiting to API Endpoints

## Problem Statement

No rate limiting exists on any API endpoint. An attacker with a valid API key could flood the execution endpoint, exhausting sandbox resources and affecting all tenants on the shared infrastructure.

## Findings

- No rate limiting middleware in Gin router setup
- Execution endpoint is the highest-risk target (expensive sandbox operations)
- Upload endpoint is second-highest risk (S3 writes + processing)
- API key auth exists but provides no rate control
- Config has `MaxConcurrentExecs` but this is a global semaphore, not per-tenant

**Affected files:**
- `internal/api/router.go` — middleware setup
- `internal/api/middleware.go` — auth middleware (no rate limiting)

## Proposed Solutions

### Option 1: Per-Tenant Token Bucket via Redis

**Approach:** Use Redis-backed token bucket rate limiter. Config already supports `SKILLBOX_REDIS_URL`. Different limits per endpoint tier (execution=strict, CRUD=relaxed).

**Pros:**
- Distributed rate limiting (works across replicas)
- Per-tenant fairness
- Redis already optional in config

**Cons:**
- Requires Redis for full functionality
- Additional infrastructure dependency

**Effort:** 3-4 hours
**Risk:** Low

---

### Option 2: In-Memory Rate Limiter (golang.org/x/time/rate)

**Approach:** Use stdlib-compatible rate limiter. Per-tenant buckets stored in sync.Map with periodic cleanup.

**Pros:**
- No external dependencies
- Simple implementation
- Good enough for single-instance deployments

**Cons:**
- Not distributed (per-instance only)
- Memory usage grows with tenant count

**Effort:** 2-3 hours
**Risk:** Low

## Recommended Action

*To be filled during triage.*

## Acceptance Criteria

- [ ] Execution endpoint has per-tenant rate limiting
- [ ] Upload endpoint has per-tenant rate limiting
- [ ] Rate limit headers returned (X-RateLimit-Remaining, etc.)
- [ ] 429 Too Many Requests returned when exceeded
- [ ] Rate limits are configurable via environment variables

## Work Log

### 2026-03-03 - Initial Discovery

**By:** Claude Code (code review)

**Actions:**
- Security Sentinel identified missing rate limiting
- Performance Oracle confirmed execution endpoint as highest-risk target
- Config already supports optional Redis URL

**Learnings:**
- Start with in-memory rate limiter, upgrade to Redis when needed
