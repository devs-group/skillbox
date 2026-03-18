---
title: "feat: Add security scanner for uploaded skills"
type: feat
date: 2026-03-04
brainstorm: docs/brainstorms/2026-03-04-security-scan-uploaded-skills-brainstorm.md
---

# feat: Add Security Scanner for Uploaded Skills

## Overview

Add a static pattern scanner that inspects every file inside a skill zip archive at upload time. Skills containing malicious code patterns (reverse shells, data exfiltration, privilege escalation, credential harvesting) are hard-rejected with detailed feedback before they ever reach storage.

## Problem Statement

Skillbox accepts arbitrary zip archives containing executable code (Python, Node, Bash). While the runtime sandbox provides strong isolation (network deny, capability drop, read-only rootfs, non-root user), there is no pre-storage inspection of skill contents. Malicious or accidentally dangerous code is stored and potentially executed without any static analysis gate.

## Proposed Solution

A Go package (`internal/scanner`) that:

1. Opens the normalized zip bytes
2. Iterates all entries, skipping binary files
3. Matches each text file line-by-line against a compiled ruleset across 3 categories
4. Collects all findings exhaustively (does not stop at first match)
5. Returns structured findings to the upload handler
6. Handler rejects with 422 + detailed findings if any high/critical severity match exists

Inserted into the upload pipeline between `parsedSkill.Validate()` and `reg.Upload()`.

## Technical Approach

### Architecture

```
internal/scanner/
├── scanner.go       # Scanner struct, ScanZip() method, binary detection
├── rules.go         # Rule definitions with compiled regexes
├── rules_test.go    # Table-driven tests for individual rules
└── scanner_test.go  # Integration tests with makeZip helper
```

**Pipeline placement** in `internal/api/handlers/skill.go`:

```
normalizeSkillZip(zipData)
validateSkillZip(zipData)        → *skill.Skill
parsedSkill.Validate()
─── NEW: scanner.ScanZip(zipData) ───  ← rejects here
reg.Upload(...)
s.UpsertSkill(...)
201 Created
```

### Key Design Decisions

**D1. Fail-closed on scanner error.** If the scanner returns a non-nil error (internal failure, e.g. corrupt zip re-read), the upload is rejected with HTTP 500 and error code `scanner_error`. The security guarantee is never silently bypassed.

**D2. Binary detection: null-byte heuristic + extension allowlist.** A file is binary if the first 512 bytes contain a null (`\x00`) byte. Files with known text extensions (`.py`, `.js`, `.sh`, `.ts`, `.rb`, `.go`, `.rs`, `.c`, `.h`, `.yaml`, `.yml`, `.json`, `.toml`, `.txt`, `.md`, `.cfg`, `.ini`, `.env`, `.bash`, `.zsh`) are always scanned regardless of null-byte check. This prevents evasion via extension renaming while avoiding false classification of UTF-16 text.

**D3. SKILL.md is scanned but only inside fenced code blocks.** Markdown prose is excluded — only content inside triple-backtick fenced code blocks is scanned. This prevents false positives from documentation describing security concepts while still catching executable examples. Files other than `*.md` are scanned fully.

**D4. Exhaustive scan — all files, all rules.** The scanner collects every finding across all files before returning. Developers see all violations in a single upload attempt, enabling them to fix everything at once.

**D5. Per-file scan cap: 1 MB.** Only the first 1 MB of each text file is scanned. Content beyond this threshold is not analyzed. This prevents DoS via large text files causing regex backtracking. Files over 1 MB that pass the scanned portion are allowed through.

**D6. Rule IDs use stable dotted slugs.** Format: `{category}/{name}`, e.g., `network.reverse_shell`, `system.etc_access`. These are stable across versions for API consumers to match on.

**D7. Scanner findings are not persisted in DB in v1.** Findings are returned in the 422 response and logged via `slog.Warn`. No new database table or column.

**D8. No medium-severity warnings in 201 response in v1.** Only high/critical findings trigger rejection. Medium findings are logged server-side but not surfaced to the caller. `SkillSummary` is unchanged.

### Findings Response Schema

On rejection, the handler returns:

```json
{
  "error": "security_violation",
  "message": "skill failed security scan: 3 violation(s) found",
  "details": [
    {
      "file": "main.py",
      "line": 12,
      "rule": "network.reverse_shell",
      "severity": "critical",
      "message": "Reverse shell pattern detected: /dev/tcp connection"
    }
  ]
}
```

Go types:

```go
// internal/scanner/scanner.go

type Finding struct {
    File     string `json:"file"`
    Line     int    `json:"line"`
    Rule     string `json:"rule"`
    Severity string `json:"severity"`
    Message  string `json:"message"`
}

type ScanResult struct {
    Findings []Finding
}

func (r *ScanResult) HasViolations() bool {
    for _, f := range r.Findings {
        if f.Severity == "critical" || f.Severity == "high" {
            return true
        }
    }
    return false
}
```

### Rule Catalog

All rules in v1 are **high** or **critical** severity. Medium-severity rules are deferred.

#### Category: `network` (Exfiltration & Reverse Shells)

| Rule ID | Severity | Pattern (simplified) | Description |
|---|---|---|---|
| `network.reverse_shell_bash` | critical | `bash\s+-i\s+.*>/dev/tcp` | Bash reverse shell via /dev/tcp |
| `network.reverse_shell_nc` | critical | `nc\s+.*-e\s+/bin/(sh\|bash)` | Netcat reverse shell |
| `network.dev_tcp` | critical | `/dev/(tcp\|udp)/` | Direct /dev/tcp or /dev/udp access |
| `network.python_reverse_shell` | critical | `socket\..*connect\(.*\).*subprocess\|pty\.spawn` | Python socket connect-back shell |
| `network.socat_shell` | critical | `socat\s+.*exec:` | Socat-based reverse shell |
| `network.raw_socket` | high | `socket\.socket\(socket\.AF_INET,\s*socket\.SOCK_RAW` | Raw socket creation |
| `network.curl_post_data` | high | `curl\s+.*(-d\|--data\|-F\|--form)\s+.*http` | curl posting data to external host |
| `network.wget_pipe` | high | `wget\s+.*-O\s*-\s*\|` | wget piped to execution |

#### Category: `system` (Tampering & Escalation)

| Rule ID | Severity | Pattern (simplified) | Description |
|---|---|---|---|
| `system.path_traversal` | high | `\.\./\.\./` | Double path traversal (../../) |
| `system.etc_shadow` | critical | `/etc/(shadow\|passwd\|sudoers)` | Access to sensitive system files |
| `system.privilege_escalation` | high | `sudo\s+\|chmod\s+[+]?[ugo]*s\|chown\s+root` | Privilege escalation attempts |
| `system.setuid` | critical | `os\.set(uid\|gid\|euid\|egid)\(0\)` | Setting UID/GID to root |
| `system.proc_self` | high | `/proc/self/(environ\|maps\|cmdline\|fd)` | Process introspection via /proc |
| `system.ptrace` | critical | `ptrace\(PTRACE_ATTACH\|SYS_ptrace` | Process injection via ptrace |
| `system.kernel_module` | critical | `insmod\s+\|modprobe\s+\|rmmod\s+` | Kernel module manipulation |
| `system.ld_preload` | critical | `LD_PRELOAD=\|LD_LIBRARY_PATH=` | Library injection |

#### Category: `data` (Theft & Exfiltration)

| Rule ID | Severity | Pattern (simplified) | Description |
|---|---|---|---|
| `data.env_harvest` | high | `os\.environ\b(?!\.get\()` | Bulk environment variable access (not single `.get()`) |
| `data.cloud_metadata` | critical | `169\.254\.169\.254` | AWS/GCP/Azure metadata endpoint |
| `data.credential_files` | high | `~/?\.(aws\|ssh\|gnupg\|docker)/\|\.kube/config` | Access to credential directories |
| `data.base64_exfil` | high | `base64.*\|\s*(curl\|wget\|nc\|python)` | Base64-encode piped to network tool |
| `data.crypto_mining` | critical | `stratum\+tcp://\|xmrig\|cryptonight\|minerd` | Cryptocurrency mining indicators |
| `data.dns_exfil` | high | `dig\s+.*@\|nslookup\s+.*\.\|host\s+.*\.` combined with variable interpolation | DNS-based data exfiltration |

### Config Changes

Add to `internal/config/config.go`:

```go
// Scanner
ScannerEnabled     bool  // SKILLBOX_SCANNER_ENABLED (default: true)
ScannerMaxFileSize int64 // SKILLBOX_SCANNER_MAX_FILE_SIZE (default: 1048576 = 1MB)
```

Environment variables:

| Env Var | Type | Default | Description |
|---|---|---|---|
| `SKILLBOX_SCANNER_ENABLED` | bool | `true` | Enable/disable the security scanner |
| `SKILLBOX_SCANNER_MAX_FILE_SIZE` | int64 | `1048576` | Max bytes per file to scan (1 MB) |

No per-category env vars in v1. Keep it simple.

### Implementation Phases

#### Phase 1: Scanner Package (`internal/scanner/`)

**Files to create:**

- `internal/scanner/scanner.go` — `Scanner` struct, `ScanZip(data []byte) (*ScanResult, error)`, binary detection, markdown code-block extraction, per-file scan loop
- `internal/scanner/rules.go` — `Rule` struct, compiled regex rules organized by category, `DefaultRules()` function
- `internal/scanner/scanner_test.go` — table-driven tests using `makeZip` helper:
  - Clean skill passes
  - Each rule category triggers correctly
  - Binary files are skipped
  - SKILL.md prose does not trigger, but SKILL.md code blocks do
  - Files over 1 MB are truncated to 1 MB scan
  - Empty files are handled
  - Zip with no text files passes

**Acceptance criteria:**
- [ ] `ScanZip` returns `*ScanResult` with correct findings for each rule
- [ ] Binary detection skips `.pyc`, `.so`, image files, etc.
- [ ] Markdown files only scanned inside fenced code blocks
- [ ] Per-file cap at `ScannerMaxFileSize` bytes
- [ ] All rules have stable slug IDs
- [ ] Tests pass for clean skills, each malicious category, and edge cases

#### Phase 2: Config Integration

**Files to modify:**

- `internal/config/config.go` — add `ScannerEnabled`, `ScannerMaxFileSize` fields and parsing

**Acceptance criteria:**
- [ ] `SKILLBOX_SCANNER_ENABLED` defaults to `true`, parses via `parseBool`
- [ ] `SKILLBOX_SCANNER_MAX_FILE_SIZE` defaults to `1048576`, parses via `ParseInt`
- [ ] Existing config tests still pass
- [ ] New config fields tested with `t.Setenv`

#### Phase 3: Handler Integration

**Files to modify:**

- `internal/api/handlers/skill.go` — call `scanner.ScanZip` after `Validate()`, before `Upload()`
  - If scanner disabled: skip
  - If scanner error: `response.RespondError(c, 500, "scanner_error", ...)`
  - If violations found: `response.RespondErrorWithDetails(c, 422, "security_violation", msg, findings)`
  - Log rejections via `slog.Warn("skill rejected by scanner", "tenant", tenantID, "skill", name, "findings_count", len(findings))`

**Acceptance criteria:**
- [ ] Clean skill uploads succeed (201) as before
- [ ] Malicious skill uploads return 422 with `security_violation` error code and findings array
- [ ] Scanner internal error returns 500 with `scanner_error` code
- [ ] `SKILLBOX_SCANNER_ENABLED=false` bypasses scanner entirely
- [ ] Existing upload tests still pass
- [ ] New integration tests cover rejection and bypass paths

#### Phase 4: Integration Tests

**Files to modify:**

- `internal/api/router_test.go` — add test cases:
  - Upload with malicious content → 422
  - Upload with clean content → 201 (scanner enabled)
  - Upload with scanner disabled → 201 regardless of content

**Acceptance criteria:**
- [ ] End-to-end test: malicious zip → 422 with parseable findings JSON
- [ ] End-to-end test: clean zip → 201 unchanged
- [ ] End-to-end test: scanner disabled → 201 for any content

## Acceptance Criteria

### Functional Requirements

- [ ] `POST /v1/skills` scans zip contents before storage
- [ ] Malicious patterns across all 3 categories are detected
- [ ] Rejected uploads return 422 with `security_violation` and detailed findings
- [ ] Clean uploads are unaffected (201, same response shape)
- [ ] Scanner can be disabled via `SKILLBOX_SCANNER_ENABLED=false`
- [ ] SKILL.md documentation prose does not cause false positives
- [ ] Binary files in zip are skipped

### Non-Functional Requirements

- [ ] Scanner adds < 100ms latency for typical skills (< 5 MB text)
- [ ] No new external dependencies (pure Go, stdlib regex)
- [ ] Fail-closed: scanner errors → 500, never silently bypass

### Quality Gates

- [ ] All existing tests pass
- [ ] New scanner unit tests with > 90% branch coverage on rules
- [ ] Integration tests for all 3 response paths (201, 422, 500)
- [ ] `go vet` and `go test -race` pass

## Known Limitations (v1)

1. **Regex evasion:** Obfuscated payloads (string concatenation, dynamic eval) can bypass static patterns. The sandbox runtime isolation is the second defense layer.
2. **No retroactive scan:** Skills uploaded before scanner deployment are not scanned. Accepted for v1.
3. **No admin override:** No quarantine or force-upload mechanism. Deferred to v2.
4. **No DB persistence of findings:** Findings only in response + logs. Deferred to v2.
5. **No medium-severity surfacing:** Below-threshold findings are logged only. Deferred to v2.

## References

### Internal

- Brainstorm: `docs/brainstorms/2026-03-04-security-scan-uploaded-skills-brainstorm.md`
- Upload handler: `internal/api/handlers/skill.go`
- Existing security: `internal/runner/security.go`
- Config: `internal/config/config.go`
- Error responses: `internal/api/response/errors.go`
- Test patterns: `internal/api/router_test.go`, `internal/api/handlers/normalize_test.go`

### Patterns to Follow

- Error accumulation: `skill.Validate()` pattern (collect all errors, join with "; ")
- Security function: `runner.ValidateImage()` pattern (return `error`, use `fmt.Errorf`)
- Config parsing: `ImageAllowlist` pattern for comma lists, `parseBool` for booleans
- Test style: table-driven subtests with `t.Run`, `strings.Contains` for error matching
- Response format: `response.RespondErrorWithDetails()` for structured error payloads
