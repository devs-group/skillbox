---
status: complete
priority: p2
issue_id: "040"
tags: [code-review, security, scanner, path-traversal]
dependencies: []
---

# ZIP Path Traversal Check Missing Absolute Path Validation

## Problem Statement

`zipcheck.go:51` checks for `..` but not absolute paths (`/etc/passwd`, `C:\Windows`). While Go's `archive/zip` typically strips leading slashes, a crafted ZIP could contain absolute path entries.

## Findings

- `zipcheck.go:51` -- only `strings.Contains(f.Name, "..")`
- `handlers/skill.go:213` -- same pattern
- Security Sentinel: P2-2

## Proposed Solutions

### Option A: Add Absolute Path Check
```go
if strings.HasPrefix(f.Name, "/") || strings.HasPrefix(f.Name, "\\") {
    return fmt.Errorf("zip contains absolute path: %s", f.Name)
}
```
Also apply `filepath.Clean` and verify result stays within root.

- **Effort:** Small | **Risk:** Low

## Acceptance Criteria

- [ ] Absolute paths rejected in ZIP entries
- [ ] Backslash paths rejected
- [ ] Test coverage for both cases
