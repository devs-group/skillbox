---
status: pending
priority: p2
issue_id: "014"
tags: [code-review, performance, database, caching]
dependencies: []
---

# Cache API Key Authentication

## Problem Statement

Every API request triggers a database query to validate the API key hash. Under load, this creates unnecessary database pressure and adds latency to every request. The config already supports an optional Redis URL that could be used for caching.

## Findings

- `internal/api/middleware.go` calls `store.GetAPIKeyByHash()` on every request
- API keys are immutable (hash never changes) — perfect cache candidates
- With 100 req/s, that's 100 unnecessary DB queries per second
- Config has `SKILLBOX_REDIS_URL` (optional) but it's unused
- No in-memory cache exists either

**Affected files:**
- `internal/api/middleware.go` — auth middleware
- `internal/store/api_key_store.go` — key lookup

## Proposed Solutions

### Option 1: In-Memory LRU Cache with TTL

**Approach:** Add a sync.Map or LRU cache in the auth middleware. Cache key_hash → tenant_id with 5-minute TTL. No external dependencies.

**Pros:**
- Zero additional dependencies
- Very fast (in-process)
- Simple implementation

**Cons:**
- Per-instance cache (not shared across replicas)
- TTL delay for key revocation

**Effort:** 1-2 hours
**Risk:** Low

---

### Option 2: Redis Cache

**Approach:** Use the existing Redis URL config for distributed caching. Cache API key lookups with configurable TTL.

**Pros:**
- Shared across replicas
- Consistent cache
- Redis already in config

**Cons:**
- Network roundtrip (faster than DB but not free)
- Requires Redis infrastructure

**Effort:** 2-3 hours
**Risk:** Low

## Recommended Action

*To be filled during triage.*

## Acceptance Criteria

- [ ] API key validation hits DB at most once per TTL period
- [ ] Cache invalidation works when keys are revoked
- [ ] Performance benchmark shows reduced DB load
- [ ] Graceful degradation when cache unavailable

## Work Log

### 2026-03-03 - Initial Discovery

**By:** Claude Code (code review)

**Actions:**
- Performance Oracle identified per-request DB hit for auth
- Config analysis found unused SKILLBOX_REDIS_URL
- Confirmed API keys are immutable — ideal for caching

**Learnings:**
- Start with in-memory cache, optionally upgrade to Redis
