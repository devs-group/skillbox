---
status: complete
priority: p1
issue_id: "002"
tags: [code-review, security, s3, injection]
dependencies: []
---

# Sanitize S3 Object Keys to Prevent Path Traversal

## Problem Statement

Skill names and version strings are used directly in S3 object key construction without sanitization. An attacker with a valid API key could craft skill names like `../../other-tenant/secret` to read or overwrite other tenants' data in the shared S3 bucket.

## Findings

- `internal/store/s3_store.go` constructs keys like `fmt.Sprintf("%s/%s/%s", tenantID, skillName, version)`
- No validation or sanitization of `skillName` or `version` components
- Characters like `../`, `/`, null bytes are not stripped
- S3 treats `/` as a path delimiter, enabling traversal
- The tenant isolation boundary is purely based on S3 key prefix — traversal bypasses it

**Affected files:**
- `internal/store/s3_store.go` — `Upload()`, `Download()`, `Delete()` methods
- `internal/api/skill_handler.go` — accepts user-provided skill names
- `internal/api/files_handler.go` — accepts user-provided file paths

## Proposed Solutions

### Option 1: Strict Allowlist Validation

**Approach:** Validate skill names, versions, and file paths against a strict regex (`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`). Reject anything containing `..`, `/`, `\`, null bytes, or other special characters at the API handler level.

**Pros:**
- Simple to implement and understand
- Defense in depth — blocks bad input at the boundary
- Easy to test

**Cons:**
- May restrict legitimate use cases (e.g., scoped names like `@org/pkg`)

**Effort:** 2-3 hours
**Risk:** Low

---

### Option 2: URL-Encode S3 Keys

**Approach:** URL-encode all user-provided components before constructing S3 keys. This neutralizes special characters while preserving arbitrary names.

**Pros:**
- Doesn't restrict naming
- Standard encoding approach

**Cons:**
- Makes S3 keys harder to read/debug
- Need to decode on retrieval
- Doesn't address the root trust issue

**Effort:** 2 hours
**Risk:** Medium

## Recommended Action

*To be filled during triage.*

## Acceptance Criteria

- [ ] Skill names containing `..` or `/` are rejected with 400
- [ ] Version strings containing path traversal characters are rejected
- [ ] File paths are validated before S3 key construction
- [ ] Unit tests cover traversal attempts
- [ ] Existing valid names continue to work

## Work Log

### 2026-03-03 - Initial Discovery

**By:** Claude Code (code review)

**Actions:**
- Security Sentinel agent identified unsanitized S3 key construction
- Verified no input validation exists on skill name, version, or file path
- Confirmed tenant isolation relies solely on S3 key prefix

**Learnings:**
- This is a standard path traversal vulnerability pattern
- Should be fixed at the API boundary with strict validation
