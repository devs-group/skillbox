---
status: pending
priority: p1
issue_id: "005"
tags: [code-review, performance, architecture, scalability]
dependencies: []
---

# Make Execution Handler Asynchronous

## Problem Statement

The execution handler blocks the HTTP request goroutine for the entire sandbox execution duration (up to 10 minutes with MaxTimeout). This means each concurrent execution ties up an HTTP connection and goroutine, limiting throughput to `MaxConcurrentExecs` simultaneous HTTP connections for execution requests.

## Findings

- `internal/api/execution_handler.go` calls `runner.Execute()` synchronously
- `runner.Execute()` blocks until sandbox completes (up to MaxTimeout)
- HTTP response is only sent after execution finishes
- With default MaxConcurrentExecs=10 and MaxTimeout=10m, only 10 concurrent executions possible
- Gin's default goroutine-per-request model means these goroutines are blocked for the full duration
- The API already returns execution IDs — could easily support async polling

**Affected files:**
- `internal/api/execution_handler.go` — synchronous Execute call
- `internal/runner/runner.go` — Execute method blocks
- `internal/store/execution_store.go` — execution status tracking

## Proposed Solutions

### Option 1: Fire-and-Forget with Status Polling

**Approach:** Return 202 Accepted with execution ID immediately. Run execution in background goroutine. Add GET /executions/{id}/status endpoint for polling.

**Pros:**
- Frees HTTP connections immediately
- Familiar REST pattern
- Client can poll at their own pace
- Simple to implement

**Cons:**
- Clients need to implement polling
- Slight increase in API calls

**Effort:** 4-6 hours
**Risk:** Medium (API behavior change)

---

### Option 2: Server-Sent Events (SSE) Streaming

**Approach:** Return 200 with SSE stream. Send status updates and final result over the stream.

**Pros:**
- Real-time updates
- Single connection
- No polling overhead

**Cons:**
- More complex client implementation
- Proxy/load balancer considerations
- Harder to implement correctly

**Effort:** 8-12 hours
**Risk:** High

## Recommended Action

*To be filled during triage.*

## Acceptance Criteria

- [ ] Execution requests return within seconds regardless of sandbox duration
- [ ] Execution status is queryable via API
- [ ] Existing SDK clients updated for new async flow
- [ ] Backward compatibility considered (or documented as breaking change)

## Work Log

### 2026-03-03 - Initial Discovery

**By:** Claude Code (code review)

**Actions:**
- Performance Oracle identified synchronous blocking as critical scalability bottleneck
- Architecture Strategist confirmed this limits throughput to MaxConcurrentExecs
- Noted the API already returns execution IDs, making async transition natural

**Learnings:**
- The semaphore in runner.go already handles concurrency limiting — async model would work with it
