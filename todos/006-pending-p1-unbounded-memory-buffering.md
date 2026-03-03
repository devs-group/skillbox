---
status: pending
priority: p1
issue_id: "006"
tags: [code-review, performance, security, oom]
dependencies: []
---

# Add Streaming and Size Limits for Zip/Artifact Processing

## Problem Statement

Zip archive uploads and artifact downloads are fully buffered in memory before processing. A large skill package (up to MaxSkillSize=50MB default) or artifact could cause OOM on the server, especially under concurrent load.

## Findings

- `internal/api/skill_handler.go` reads entire multipart body into memory via `c.Request.Body`
- `internal/runner/runner.go` buffers complete execution output in memory
- `internal/store/s3_store.go` `Upload()` accepts `io.Reader` but callers often pass fully-buffered data
- With 10 concurrent 50MB uploads = 500MB memory pressure minimum
- No streaming decompression — entire zip is in memory during extraction
- Zip bomb protection exists (max files/size) but only after full buffer is in memory

**Affected files:**
- `internal/api/skill_handler.go` — upload handling
- `internal/runner/runner.go` — output buffering
- `internal/store/s3_store.go` — upload/download methods

## Proposed Solutions

### Option 1: Streaming Multipart Processing

**Approach:** Use `multipart.Reader` to stream file parts directly to S3 or temp files without full buffering. Add `MaxBytesReader` wrapper on request body.

**Pros:**
- Constant memory usage regardless of file size
- Protects against OOM
- Better resource utilization

**Cons:**
- More complex upload handling
- Need to handle partial failures (cleanup temp files)

**Effort:** 4-6 hours
**Risk:** Medium

---

### Option 2: Temp File Spooling

**Approach:** Write uploads to temp files first, then process from disk. Simpler than full streaming but still prevents OOM.

**Pros:**
- Simpler than full streaming
- Leverages OS page cache
- Easy to implement

**Cons:**
- Disk I/O overhead
- Need temp file cleanup
- Still need MaxBytesReader for DoS protection

**Effort:** 2-3 hours
**Risk:** Low

## Recommended Action

*To be filled during triage.*

## Acceptance Criteria

- [ ] Upload handler has MaxBytesReader limit
- [ ] Large uploads don't cause OOM
- [ ] Concurrent uploads stay within memory budget
- [ ] Zip extraction streams from disk or uses bounded buffer
- [ ] Load test with 10 concurrent 50MB uploads passes

## Work Log

### 2026-03-03 - Initial Discovery

**By:** Claude Code (code review)

**Actions:**
- Performance Oracle identified unbounded in-memory buffering
- Security Sentinel flagged OOM potential as DoS vector
- Verified multipart handling reads entire body into memory

**Learnings:**
- `http.MaxBytesReader` is the simplest first defense
- Streaming to S3 via `io.Pipe` would be ideal but more complex
