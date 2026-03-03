---
status: pending
priority: p3
issue_id: "018"
tags: [code-review, security, http, headers]
dependencies: []
---

# Add Security Response Headers

## Problem Statement

The API responses lack standard security headers. While skillbox is primarily an API service (not serving HTML), security headers are still best practice and may be required for compliance.

## Findings

- No `X-Content-Type-Options: nosniff` header
- No `X-Frame-Options` header
- No `Strict-Transport-Security` header
- No `Content-Security-Policy` header
- Gin does not add these by default
- API responses include error messages that could be parsed by browsers

**Affected files:**
- `internal/api/router.go` — middleware setup

## Proposed Solutions

### Option 1: Add Security Headers Middleware

**Approach:** Add a simple Gin middleware that sets standard security headers on all responses.

**Pros:**
- 10-line middleware
- Industry standard practice
- No behavioral changes

**Cons:**
- Minimal impact for API-only service

**Effort:** 15 minutes
**Risk:** Low

## Recommended Action

*To be filled during triage.*

## Acceptance Criteria

- [ ] X-Content-Type-Options: nosniff on all responses
- [ ] X-Frame-Options: DENY on all responses
- [ ] Security headers verifiable via curl

## Work Log

### 2026-03-03 - Initial Discovery

**By:** Claude Code (code review)

**Actions:**
- Security Sentinel noted missing security headers
- Classified as low priority since service is API-only

**Learnings:**
- Even API services benefit from security headers (defense in depth)
