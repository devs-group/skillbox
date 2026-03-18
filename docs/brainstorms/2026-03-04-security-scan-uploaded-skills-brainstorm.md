# Brainstorm: Security Scanning for Uploaded Skills

**Date:** 2026-03-04
**Status:** Ready for planning

## What We're Building

A static pattern scanner that inspects every file inside a skill zip archive at upload time, detecting malicious code patterns across three categories: network/exfiltration, system tampering, and data theft. Skills that fail the scan are hard-rejected with detailed feedback before they ever reach storage.

## Why This Approach

- **Static pattern scanner** was chosen over sandboxed dry-run analysis (too slow/complex) and external scanner integration (operational overhead, licensing).
- The existing sandbox already provides strong runtime isolation — the scanner's role is to catch obvious malicious intent *before storage*, acting as a first line of defense.
- Fits naturally into the existing upload pipeline between `validateSkillZip()` and `registry.Upload()`.
- Fast (milliseconds), no external dependencies, easy to extend with new rules.

## Key Decisions

1. **Scan timing:** Blocking at upload time — malicious skills never reach S3/MinIO.
2. **On detection:** Hard reject (400/422) — skill is not stored at all.
3. **Feedback:** Detailed — uploader sees which file and pattern triggered the rejection.
4. **Detection scope:** All categories (network exfiltration, system tampering, data theft).
5. **Implementation:** Pure Go, regex/pattern-based, built into the upload handler pipeline.

## Detection Categories

### Network & Exfiltration
- Reverse shells (bash -i, /dev/tcp, nc -e, python socket connect-back)
- Outbound HTTP/DNS to hardcoded external hosts
- curl/wget to non-localhost targets
- Raw socket creation

### System Tampering
- Path traversal / filesystem escape (`../../`, writing outside /sandbox/)
- Privilege escalation (sudo, chmod +s, setuid)
- Modifying system binaries or config (/etc/passwd, /etc/shadow)
- Process injection, ptrace, /proc/self

### Data Theft
- Reading environment variables to harvest secrets (os.environ iteration)
- Cloud metadata endpoint access (169.254.169.254)
- Credential file access (~/.aws, ~/.ssh, /etc/shadow)
- Encoding/exfiltrating data (base64 piped to network calls)

## Architecture Sketch

```
Upload Request
  → AuthMiddleware
  → readZipBytes (size limit)
  → normalizeSkillZip
  → validateSkillZip (zip-slip, SKILL.md presence)
  → **scanSkillZip** (NEW — pattern scanner)     ← rejects here if malicious
  → registry.Upload (S3)
  → store.UpsertSkill (Postgres)
  → 201 Created
```

New package: `internal/security/scanner.go`

```
SecurityScanner
  ├── ScanZip(zipBytes) → ScanResult
  ├── rules []ScanRule
  │     ├── Category (network | system | data_theft)
  │     ├── Severity (critical | high | medium)
  │     ├── Pattern (compiled regex)
  │     ├── Description (human-readable)
  │     └── FileTypes (which extensions to check)
  └── ScanResult
        ├── Clean bool
        ├── Findings []Finding
        │     ├── Rule, File, Line, Match
        └── Summary string
```

## Open Questions

1. **False positive handling:** Should there be an admin override to force-upload a flagged skill? (Deferred — not in v1.)
2. **Rule updates:** Should rules be configurable via env/config, or hardcoded? (Start hardcoded, make configurable later if needed.)
3. **Binary files:** Should we scan binary files or skip them? (Skip — focus on text/script files where patterns are meaningful.)
4. **Obfuscation:** Base64-encoded payloads, string concatenation tricks — how aggressively to detect? (v1: detect common obfuscation patterns like base64-decode-pipe-exec; don't try to deobfuscate.)

## Out of Scope (YAGNI)

- Runtime behavioral analysis (sandbox already handles isolation)
- Dependency vulnerability scanning (separate feature)
- ML-based detection
- Admin review/approval workflow for quarantined skills
