---
status: pending
priority: p2
issue_id: "012"
tags: [code-review, agent-native, api, completeness]
dependencies: []
---

# Add Missing API Endpoints for Agent Parity

## Problem Statement

Several operations that agents need are missing from the API, forcing workarounds or making certain workflows impossible for automated consumers. The agent-native review scored 22/28 capabilities as fully accessible.

## Findings

- **ListExecutions** — no endpoint to list past executions (agents can't review history)
- **DeleteSkill** — exists in server but missing from Go and TypeScript SDKs
- **API Key Management** — no CRUD endpoints (keys can only be created via direct DB access)
- **Execution Logs Streaming** — no way to get real-time execution output
- **Bulk Operations** — no batch skill upload or execution

**Affected files:**
- `internal/api/router.go` — route registration
- `internal/api/execution_handler.go` — missing list endpoint
- `sdks/go/skillbox.go` — missing DeleteSkill method
- `sdks/typescript/` — missing DeleteSkill method

## Proposed Solutions

### Option 1: Add Critical Missing Endpoints

**Approach:** Add ListExecutions and SDK DeleteSkill first (highest impact). API key management and streaming in follow-up.

**Pros:**
- Quick wins for agent usability
- Incremental approach
- Low risk

**Cons:**
- Doesn't solve everything at once

**Effort:** 3-4 hours (ListExecutions + SDK updates)
**Risk:** Low

## Recommended Action

*To be filled during triage.*

## Acceptance Criteria

- [ ] GET /executions endpoint exists with tenant filtering and pagination
- [ ] DeleteSkill available in Go SDK
- [ ] DeleteSkill available in TypeScript SDK
- [ ] API documentation updated

## Work Log

### 2026-03-03 - Initial Discovery

**By:** Claude Code (code review)

**Actions:**
- Agent-Native Reviewer scored 22/28 capabilities as accessible
- Identified ListExecutions as most impactful missing endpoint
- Confirmed DeleteSkill exists in API but not in SDKs

**Learnings:**
- Agent parity is important for programmatic consumers (like vectorchat)
