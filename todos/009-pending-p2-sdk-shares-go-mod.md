---
status: pending
priority: p2
issue_id: "009"
tags: [code-review, architecture, sdk, dependencies]
dependencies: []
---

# Separate SDK into Independent Go Module

## Problem Statement

The Go SDK at `sdks/go/skillbox.go` shares the root `go.mod` with the server. Consumers importing the SDK pull in all server dependencies (PostgreSQL driver, MinIO client, Gin, etc.) even though the SDK is stdlib-only.

## Findings

- `sdks/go/skillbox.go` has zero imports outside stdlib — excellent design
- But `go.mod` is at repo root, so `go get github.com/devs-group/skillbox` pulls all transitive deps
- Vectorchat's go.mod shows many unnecessary indirect deps from skillbox
- The SDK is consumed by external projects (vectorchat) — this is a real issue, not theoretical

**Affected files:**
- `go.mod` — root module
- `sdks/go/skillbox.go` — SDK code
- `sdks/go/go.mod` — needs to be created

## Proposed Solutions

### Option 1: Separate go.mod for SDK

**Approach:** Add `sdks/go/go.mod` with module path `github.com/devs-group/skillbox/sdks/go`. SDK becomes independently importable with zero deps.

**Pros:**
- Consumers get only stdlib dependencies
- Clean separation of concerns
- Standard Go multi-module pattern

**Cons:**
- Need to manage two go.mod files
- Versioning becomes slightly more complex
- Need to update import paths in consumers

**Effort:** 1-2 hours
**Risk:** Low (but breaking change for consumers)

## Recommended Action

*To be filled during triage.*

## Acceptance Criteria

- [ ] SDK has its own go.mod with zero external dependencies
- [ ] `go get github.com/devs-group/skillbox/sdks/go` works
- [ ] SDK tests pass independently
- [ ] Consumer (vectorchat) updated to use new import path
- [ ] Release notes document the import path change

## Work Log

### 2026-03-03 - Initial Discovery

**By:** Claude Code (code review)

**Actions:**
- Architecture Strategist identified shared go.mod as architectural issue
- Verified SDK has zero external imports (stdlib only)
- Confirmed vectorchat pulls unnecessary deps via shared module

**Learnings:**
- Go multi-module repos are well-supported since Go 1.18+
