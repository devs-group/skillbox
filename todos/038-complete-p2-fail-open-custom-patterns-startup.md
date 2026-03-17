---
status: complete
priority: p2
issue_id: "038"
tags: [code-review, security, scanner, fail-open]
dependencies: []
---

# Fail-Open When Custom Pattern File Cannot Be Loaded at Startup

## Problem Statement

When `SKILLBOX_SCANNER_PATTERNS_FILE` is configured but the file cannot be read/parsed, the scanner silently falls back to embedded defaults with only a warning log (`scanner.go:53-57`). If the file contains critical blocklist additions for newly discovered malicious packages and becomes unreadable, the scanner runs without those patterns, allowing known-malicious packages through.

This is a fail-open behavior in a security-critical path.

## Findings

- `scanner.go:52-57` -- `loadPatterns` error triggers silent fallback
- Security Sentinel: P2-4

## Proposed Solutions

### Option A: Fatal When Explicitly Configured (Recommended)
If `customPatternsFile != ""` and loading fails, return an error from `New()` instead of falling back.

- **Effort:** Small | **Risk:** Low

## Acceptance Criteria

- [ ] Startup fails if a configured custom patterns file cannot be loaded
- [ ] Missing/empty custom file config still allows startup with embedded defaults
