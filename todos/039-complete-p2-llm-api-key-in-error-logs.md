---
status: complete
priority: p2
issue_id: "039"
tags: [code-review, security, scanner, secret-leakage]
dependencies: []
---

# LLM API Key May Appear in Error Logs

## Problem Statement

`stage_llm.go:269` includes the full API response body in error messages: `fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBody))`. If the Anthropic API echoes the API key in error responses (e.g., invalid key errors), it propagates through the scanner pipeline into structured logs.

## Findings

- `stage_llm.go:269` -- full response body in error
- Security Sentinel: P2-5

## Proposed Solutions

### Option A: Truncate and Sanitize Response Body
Limit logged response body to 200 characters, redact anything matching API key patterns.

- **Effort:** Small | **Risk:** Low

## Acceptance Criteria

- [ ] Error messages from LLM stage truncate response bodies
- [ ] API key patterns are redacted from logged errors
