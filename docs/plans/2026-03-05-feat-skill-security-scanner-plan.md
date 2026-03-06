---
title: "feat: Add Security Scanner for Uploaded Skills"
type: feat
date: 2026-03-05
deepened: 2026-03-06
brainstorm: docs/brainstorms/2026-03-05-skill-security-scanner-brainstorm.md
---

# feat: Add Security Scanner for Uploaded Skills

## Enhancement Summary

**Deepened on:** 2026-03-06
**Sections enhanced:** Architecture, Pipeline Logic, All Phases, Acceptance Criteria, Risk Analysis
**Review agents used:** Security Sentinel, Security Auditor, Architecture Strategist, Performance Oracle, Code Simplicity Reviewer, Pattern Recognition Specialist, Agent-Native Reviewer, Injection Analyst, Deployment Verification Agent

### Key Improvements
1. **Tiered scanning model** — Tier 1 quick-accept for clean uploads, Tier 2 deep scan for flagged content, Tier 3 LLM judgment. Most uploads never leave Tier 1.
2. **ZIP bomb protection** — total decompressed size cap (50MB), entry count limit (500), compression ratio check before scanner even runs.
3. **Memory optimization** — pass `*zip.Reader` through the pipeline instead of `[]byte` to avoid 3x memory amplification from re-parsing.
4. **Infrastructure error contract** — return `(nil, error)` for infra failures, `(result, nil)` for verdicts. Caller distinguishes between "scan says reject" and "scan is broken."
5. **ReDoS prevention** — mandate Go stdlib `regexp` (RE2-based, guaranteed linear time) over `regexp2`.
6. **Prompt injection hardening** — canary tokens, random delimiters, temperature 0, Unicode NFC normalization before matching.
7. **Agent-parseable responses** — structured 422 body with `scan_id` and `categories` array; `POST /v1/skills/validate` dry-run endpoint.
8. **Simplified Phase 1 structure** — flatten `patterns/` and `deps/` sub-packages into stage files; ship external scanners only in Phase 4.

### New Considerations Discovered
- TOCTOU risk: hash ZIP bytes before scan, compare after — reject if modified
- Homoglyph detection in package names (e.g., Cyrillic `а` vs Latin `a`)
- File extension bypass: scan all files regardless of extension, not just `.py`/`.js`/`.sh`
- Tool-call injection patterns in SKILL.md (fake `tool_use` blocks)
- Scanner disable via `SKILLBOX_SCANNER_ENABLED=false` must emit a startup warning log

## Overview

Add a multi-layer security scanner that inspects uploaded skill ZIPs **before** they are stored in MinIO. The scanner uses a **tiered model** with short-circuit optimization: Tier 1 performs fast static checks and accepts clean uploads immediately (~95% of uploads); Tier 2 runs deeper analysis (dependency scanning, prompt injection) on flagged content; Tier 3 invokes LLM for contextual judgment on ambiguous findings. Pluggable external scanners (ClamAV/YARA) run as an optional stage. Any block-level finding causes a hard reject: the skill never reaches storage.

## Problem Statement

Currently, anyone with a valid API key can upload a skill ZIP containing arbitrary Python, Node.js, or Bash code. While skills execute inside sandboxed containers with network deny-all, there is no pre-storage inspection for:

- Malicious code patterns (reverse shells, crypto miners, sandbox escapes)
- Known-malicious or typosquatted dependencies
- Prompt injection payloads in SKILL.md that could manipulate AI agents
- Virus/malware embedded in uploaded files

A compromised API key or a malicious tenant could upload skills that attempt sandbox breakouts, exfiltrate data during the brief execution window, or inject instructions into agent workflows.

## Proposed Solution

Insert a `securityScan()` call into the upload pipeline between `parsedSkill.Validate()` and `registry.Upload()`. The scanner is a new `internal/scanner` package exposing a `Scanner` interface, injected as a dependency into the upload handler.

```
UploadSkill → normalizeSkillZip → validateSkillZip → parsedSkill.Validate()
  → zipBombCheck(zipData)                        ← NEW: pre-scan safety
  → securityScan(ctx, zipReader, parsedSkill)    ← NEW: tiered scanner
  → registry.Upload → store.UpsertSkill
```

### Tiered Scanning Model

```
                    ┌─────────────────┐
                    │   ZIP Upload    │
                    └────────┬────────┘
                             │
                    ┌────────▼────────┐
                    │  ZIP Bomb Check │ (size cap, entry count, ratio)
                    └────────┬────────┘
                             │
               ┌─────────────▼─────────────┐
               │  TIER 1: Quick Scan       │
               │  - Static patterns        │
               │  - Dep blocklist lookup   │
               └─────────┬────────┬────────┘
                         │        │
                    no flags   flags/uncertain
                         │        │
                    ┌────▼────┐   │
                    │ ACCEPT  │   │
                    └─────────┘   │
                         ┌────────▼────────────┐
                         │  TIER 2: Deep Scan  │
                         │  - Typosquat check  │
                         │  - Prompt injection │
                         │  - Unicode analysis │
                         │  - External scanner │
                         └────────┬───────┬────┘
                                  │       │
                             resolved   still ambiguous
                                  │       │
                             ┌────▼────┐  │
                             │ verdict │  │
                             └─────────┘  │
                                  ┌───────▼────────────┐
                                  │  TIER 3: LLM       │
                                  │  - Claude analysis  │
                                  │  - Context judgment │
                                  └───────┬────────────┘
                                          │
                                     ┌────▼────┐
                                     │ verdict │
                                     └─────────┘
```

**Tier 1 (Quick Scan, <100ms):** Runs on every upload. Checks static code patterns (block-level only) and dependency blocklist (exact match lookup). If zero findings → **accept immediately**. Most clean uploads (~95%) stop here.

**Tier 2 (Deep Scan, <500ms):** Triggered when Tier 1 produces flag-level findings or the skill has characteristics requiring deeper analysis (e.g., has SKILL.md with agent instructions, has dependency files). Runs typosquatting detection, prompt injection scanning, Unicode analysis, and optional external scanners (ClamAV/YARA). If all flags resolve cleanly → verdict. If ambiguous flags remain → escalate to Tier 3.

**Tier 3 (LLM Judgment, <10s):** Only triggered when Tier 2 cannot resolve ambiguous findings. Sends entrypoint files + SKILL.md + flag context to Claude for contextual analysis. Opt-in via config (`SKILLBOX_SCANNER_LLM_ENABLED`).

## Technical Approach

### Architecture

```
internal/scanner/
├── scanner.go          — Pipeline struct, New(), Scan() method, Scanner interface
├── result.go           — ScanResult, Finding, Severity types
├── errors.go           — Sentinel errors (ErrBlocked, ErrScanFailed, ErrLLMUnavailable)
├── zipcheck.go         — ZIP bomb protection (pre-scan: size cap, entry count, ratio)
├── stage_patterns.go   — Tier 1: static pattern scanner + dep blocklist (block/flag)
├── stage_deps.go       — Tier 2: typosquatting detection + OSV queries
├── stage_prompt.go     — Tier 2: prompt injection scanner + Unicode analysis
├── stage_llm.go        — Tier 3: LLM deep analysis
├── stage_external.go   — Tier 2: pluggable external scanner interface + noop default
├── clamav.go           — ClamAV clamd client (go-clamd) — Phase 4 only
├── yara.go             — YARA scanner (yargo, pure Go) — Phase 4 only
└── scanner_test.go     — Table-driven tests with in-memory ZIPs
```

**Simplification notes (from Simplicity Reviewer):**
- No `patterns/`, `deps/`, or `external/` sub-packages in Phase 1. Language-specific patterns are compiled regex tables inside `stage_patterns.go`. Split into sub-packages only when `stage_patterns.go` exceeds 500 lines.
- ClamAV/YARA live in `scanner/` directly as `clamav.go` and `yara.go` — added only in Phase 4.
- No-op external scanner is a `nil` check in `stage_external.go`, not a separate file.

### Core Interface

```go
// internal/scanner/scanner.go

// Scanner is the public interface for security scanning.
// Implementations: Pipeline (production), NoopScanner (tests).
//
// Error contract (from Architecture Strategist + Pattern Recognition):
//   - (result, nil): scan completed — check result.Pass for verdict
//   - (nil, error): infrastructure failure — caller should fail closed (reject upload, return 500)
// Never return (result, error) — it's one or the other.
type Scanner interface {
    Scan(ctx context.Context, zr *zip.Reader, s *skill.Skill) (*ScanResult, error)
}
```

**Performance note (from Performance Oracle):** Accept `*zip.Reader` instead of `[]byte`. The upload handler already parses the ZIP for validation — pass the reader through instead of re-parsing. This eliminates 3x memory amplification (raw bytes + zip index + decompressed buffers).

### Result Types

```go
// internal/scanner/result.go

type Severity string
const (
    SeverityBlock Severity = "BLOCK"   // always reject
    SeverityFlag  Severity = "FLAG"    // escalate to deeper tier for judgment
)

type Finding struct {
    Stage       string   // "static_patterns", "dependencies", "prompt_injection", "llm", "external"
    Severity    Severity
    Category    string   // "reverse_shell", "crypto_miner", "typosquat", "prompt_override", etc.
    FilePath    string   // relative path within ZIP
    Description string   // human-readable, no specific regex details
}

type ScanResult struct {
    Pass     bool
    Findings []Finding
    Duration time.Duration
    Tier     int       // 1, 2, or 3 — highest tier reached during scan
}
```

**Simplification (from Simplicity Reviewer):** Dropped `SeverityInfo` — it added complexity without affecting verdicts. If something is worth logging but not acting on, the stage logs it directly via slog; it doesn't need to be a `Finding`.

### ZIP Bomb Protection (Pre-Scan)

```go
// internal/scanner/zipcheck.go
//
// Runs BEFORE the scanner pipeline. This is a cheap safety check
// that prevents resource exhaustion attacks.
//
// Checks (from Security Sentinel + Security Auditor):
//   1. Total decompressed size cap: 50MB (sum of all file UncompressedSize64)
//   2. Entry count limit: 500 files
//   3. Compression ratio: reject if any file ratio > 100:1
//   4. Nested archives: reject if ZIP contains .zip, .tar, .gz, .7z entries
//
// Returns error if any check fails — caller rejects with 422.
func CheckZIPSafety(zr *zip.Reader) error
```

### Pipeline Execution Logic

```go
// internal/scanner/scanner.go

type Pipeline struct {
    tier1   []stage           // quick checks: static patterns + dep blocklist
    tier2   []stage           // deep checks: typosquat, prompt injection, external
    tier3   *llmAnalyzer      // LLM analysis (nil if disabled)
    timeout time.Duration
    logger  *slog.Logger
}

func (p *Pipeline) Scan(ctx context.Context, zr *zip.Reader, s *skill.Skill) (*ScanResult, error) {
    ctx, cancel := context.WithTimeout(ctx, p.timeout)
    defer cancel()

    result := &ScanResult{Pass: true, Tier: 1}
    start := time.Now()

    // --- Tier 1: Quick Scan ---
    flags, err := p.runStages(ctx, p.tier1, zr, s, nil)
    if err != nil {
        return nil, fmt.Errorf("tier 1: %w", err) // infra failure → caller rejects
    }
    result.Findings = append(result.Findings, flags...)

    // Short-circuit: any BLOCK → reject immediately
    if hasBlock(flags) {
        result.Pass = false
        result.Duration = time.Since(start)
        return result, nil
    }

    // Quick accept: no flags at all → accept without deeper scanning
    flagFindings := collectFlags(flags)
    needsTier2 := len(flagFindings) > 0 || s.HasAgentInstructions() || s.HasDependencyFiles()
    if !needsTier2 {
        result.Duration = time.Since(start)
        return result, nil
    }

    // --- Tier 2: Deep Scan ---
    result.Tier = 2
    deepFlags, err := p.runStages(ctx, p.tier2, zr, s, flagFindings)
    if err != nil {
        return nil, fmt.Errorf("tier 2: %w", err)
    }
    result.Findings = append(result.Findings, deepFlags...)

    if hasBlock(deepFlags) {
        result.Pass = false
        result.Duration = time.Since(start)
        return result, nil
    }

    // Resolve: if Tier 2 resolved all flags → accept
    unresolvedFlags := collectFlags(deepFlags)
    allFlags := append(flagFindings, unresolvedFlags...)
    if len(unresolvedFlags) == 0 && !hasBlock(deepFlags) {
        result.Duration = time.Since(start)
        return result, nil
    }

    // --- Tier 3: LLM Judgment ---
    if p.tier3 == nil {
        // LLM disabled but flags remain → fail closed
        result.Pass = false
        result.Findings = append(result.Findings, Finding{
            Stage:       "llm",
            Severity:    SeverityBlock,
            Category:    "unresolved_flags",
            Description: "flags require LLM analysis but LLM is disabled",
        })
        result.Duration = time.Since(start)
        return result, nil
    }

    result.Tier = 3
    llmFindings, err := p.tier3.analyze(ctx, zr, s, allFlags)
    if err != nil {
        return nil, fmt.Errorf("tier 3: %w", err) // LLM infra failure → caller rejects
    }
    result.Findings = append(result.Findings, llmFindings...)

    if hasBlock(llmFindings) {
        result.Pass = false
    }
    result.Duration = time.Since(start)
    return result, nil
}
```

**Error contract (from Architecture Strategist + Pattern Recognition):**
- Infrastructure errors (network failures, ClamAV down, LLM timeout) return `(nil, error)`. The handler responds with 500 and logs an ops alert. This is distinct from "scan found a threat."
- Security verdicts (pass/block/flag) return `(result, nil)`. The handler checks `result.Pass`.
- This fixes the original plan's error-swallowing pattern where infra failures were silently converted to `SeverityBlock` findings — a high-severity deviation from the codebase's error handling conventions.

### Implementation Phases

#### Phase 1: Scanner Foundation + Tier 1 Quick Scan

**Deliverables:**
- `Scanner` interface and `Pipeline` struct with tiered execution
- `ScanResult`, `Finding`, `Severity` types (BLOCK + FLAG only, no INFO)
- Sentinel errors (`ErrBlocked`, `ErrScanFailed`)
- ZIP bomb protection (`CheckZIPSafety`)
- Tier 1 stage: static pattern scanner with block/flag classification + dep blocklist lookup
- Integration into `UploadSkill` handler (pass `*zip.Reader`, not `[]byte`)
- Config: `SKILLBOX_SCANNER_ENABLED` (bool, default `true`) — emit slog.Warn at startup if `false`
- Config: `SKILLBOX_SCANNER_TIMEOUT` (duration, default `30s`)
- Structured slog audit logging for all scan results
- Unit tests with in-memory ZIPs (table-driven)

**Files to create:**
- `internal/scanner/scanner.go` — interface, Pipeline, tiered Scan()
- `internal/scanner/result.go` — ScanResult, Finding, Severity
- `internal/scanner/errors.go` — sentinel errors
- `internal/scanner/zipcheck.go` — ZIP bomb protection
- `internal/scanner/stage_patterns.go` — Tier 1: all pattern tables (Python, Node.js, Bash, common) as compiled regex in one file + dep blocklist exact-match lookup
- `internal/scanner/scanner_test.go` — table-driven tests

**Files to modify:**
- `internal/config/config.go` — add `Scanner*` config fields
- `internal/config/config_test.go` — test new config fields
- `internal/api/handlers/skill.go` — inject scanner call at line 104-107; pass `*zip.Reader`
- `internal/api/router.go` — wire scanner dependency into handler
- `cmd/skillbox-server/main.go` — construct scanner, pass to router

**Research enhancements for Phase 1:**

> **ReDoS prevention (Security Auditor):** Use only Go stdlib `regexp` (RE2-based, guaranteed linear time). Never use `regexp2` or PCRE-compatible engines. Validate all regex patterns with `regexp.MustCompile` at init — panics at startup if any pattern is invalid, which is preferable to silent runtime failures.

> **Regex compilation (Performance Oracle):** Compile all regex patterns once at `Pipeline` construction in a `var patterns = [...]compiledPattern{}` package-level block. Never compile inside `Scan()`.

> **File scanning (Security Auditor):** Scan ALL files in the ZIP regardless of extension. Attackers rename `.py` to `.txt` to bypass extension-based scanning. Use content heuristics (shebang lines, import statements) to detect file type.

> **Binary detection (Performance Oracle):** Use `net/http.DetectContentType()` on the first 512 bytes instead of a manual null-byte check. More reliable and handles more MIME types.

> **Per-file byte cap (Performance Oracle):** Skip files larger than 1MB for regex scanning. Large files are typically data, not code. Log skipped files at slog.Debug level.

**Success criteria:**
- Known malicious patterns (reverse shells, `nc -e`, `curl | bash`, crypto mining imports) are blocked
- Ambiguous patterns (`subprocess.Popen`, `eval()`) are flagged (not blocked) and recorded
- Binary files are skipped (detected via `DetectContentType`, not just null bytes)
- Files > 1MB are skipped for regex scanning
- ZIP bombs are rejected before scanner runs (50MB total decompressed, 500 entry limit, 100:1 ratio)
- Cognitive mode skills (no code files) pass Tier 1 with no findings
- Tier 1 adds <100ms for typical skills
- All scan results logged via slog with: tenant_id, skill_name, version, verdict, tier, categories, duration_ms
- `SKILLBOX_SCANNER_ENABLED=false` emits startup warning
- Infrastructure errors return `(nil, error)`, not wrapped findings

#### Phase 2: Tier 2 Deep Scan (Dependencies + Prompt Injection)

**Deliverables:**
- Tier 2 stage: Dependency deep scanner
  - Parse `requirements.txt`, `package.json`, `pyproject.toml`
  - Typosquatting detection: Levenshtein distance ≤ 1 against top-1000 PyPI/npm packages = block; distance ≤ 2 = flag
  - Homoglyph detection in package names (Cyrillic `а` vs Latin `a`, etc.)
  - `preinstall`/`postinstall` scripts in package.json = block (they execute before network-deny applies)
  - OSV vulnerability query for known CVEs = flag (not block — vulnerable is not malicious)
- Tier 2 stage: Prompt injection scanner
  - Regex patterns for: override instructions, role hijacking, data exfiltration instructions, MCP server references
  - Tool-call injection detection: fake `tool_use` blocks, `<function_call>` patterns in SKILL.md
  - Delimiter injection patterns: `</system>`, `Human:`, `Assistant:` in skill content
  - Invisible Unicode detection (zero-width spaces, RTL overrides, private use area, homoglyphs) — in both SKILL.md and code files
  - Unicode NFC normalization before all pattern matching
  - Risk scoring: max-of-components model (≥ 0.7 = block, 0.4-0.7 = flag, < 0.4 = pass)
- YAML frontmatter injection check: validate SKILL.md YAML for unexpected keys
- Unit tests for both stages

**Files to create:**
- `internal/scanner/stage_deps.go` — typosquatting, homoglyph, OSV, install scripts
- `internal/scanner/stage_prompt.go` — prompt injection, Unicode, tool-call injection
- `internal/scanner/stage_deps_test.go`
- `internal/scanner/stage_prompt_test.go`

**Dependencies:**
- `github.com/google/osv-scanner/pkg/osv` — OSV vulnerability queries
- `github.com/google/osv-scanner/pkg/lockfile` — dependency file parsing

**Research enhancements for Phase 2:**

> **BK-tree for Levenshtein (Performance Oracle):** Use a BK-tree (or precomputed distance table) for the top-1000 package corpus instead of brute-force O(n) Levenshtein against every popular package. BK-tree gives O(log n) lookups for distance queries.

> **Homoglyph detection (Injection Analyst):** Check package names for mixed-script characters. A package name containing both Latin and Cyrillic characters is almost certainly an attack. Use `unicode.Is(unicode.Latin, r)` and `unicode.Is(unicode.Cyrillic, r)` checks.

> **Unicode NFC normalization (Injection Analyst):** Apply `golang.org/x/text/unicode/norm` NFC normalization to all text content before pattern matching. Attackers use decomposed Unicode (NFD) to bypass regex — e.g., `é` as `e` + combining accent bypasses a pattern matching `é`.

> **Max-of-components scoring (Injection Analyst):** For prompt injection risk scoring, use `max(component_scores)` not `mean()`. A single high-confidence injection indicator should not be diluted by benign components.

> **Tool-call injection (Agent-Native Reviewer):** Detect patterns in SKILL.md that mimic Anthropic tool-call format: `<tool_use>`, `<function_call>`, `tool_name:`, `tool_input:`. These can trick agent systems into executing unintended tool calls.

> **OSV query latency (Performance Oracle):** OSV API calls add ~200-500ms. Consider: (a) timeout the OSV query at 2s and treat timeout as flag (not block), (b) cache OSV responses for 1 hour with `sync.Map` keyed by package+version.

**Success criteria:**
- Known-malicious packages from OSSF database are blocked (moved to Tier 1 blocklist)
- Typosquatted names (e.g., `reqeusts` for `requests`) are caught via Levenshtein + homoglyph detection
- `preinstall`/`postinstall` hooks are blocked
- Prompt injection patterns (override instructions, role hijacking) are detected
- Tool-call injection patterns (`<tool_use>`, `<function_call>`) are detected
- Delimiter injection patterns (`</system>`, `Human:`) are detected
- Invisible Unicode characters are flagged in both SKILL.md and code files
- Unicode NFC normalization is applied before all pattern matching
- `pyproject.toml` is parsed alongside `requirements.txt`
- OSV query timeouts do not block the upload (graceful degradation to flag)

#### Phase 3: Tier 3 LLM Deep Analysis

**Deliverables:**
- Tier 3: LLM analysis via Claude API (Haiku by default, configurable)
  - Trigger: runs only when Tier 2 leaves unresolved flag-level findings. Does NOT run on every upload.
  - Input: entrypoint file(s) truncated to first 100KB + SKILL.md + all flag findings with context
  - Structured prompt with JSON output schema: `{ "threat": bool, "confidence": float, "reasoning": string, "categories": []string }`
  - Fail closed: LLM unavailable (timeout, 5xx, 429) → return `(nil, error)`, caller rejects and logs ops alert
  - Prompt hardening (see details below)
- Config: `SKILLBOX_SCANNER_LLM_ENABLED` (bool, default `false` — opt-in)
- Config: `SKILLBOX_SCANNER_LLM_API_KEY` (string, required if LLM enabled — validate at startup, not at first scan)
- Config: `SKILLBOX_SCANNER_LLM_MODEL` (string, default `claude-haiku-4-5-20251001`)
- Config: `SKILLBOX_SCANNER_LLM_TIMEOUT` (duration, default `10s`)
- Semaphore for concurrent LLM calls: max 5 concurrent (prevents 429 storms)
- Unit tests with mock HTTP server simulating Claude API responses

**Files to create:**
- `internal/scanner/stage_llm.go`
- `internal/scanner/stage_llm_test.go`

**Dependencies:**
- `github.com/anthropics/anthropic-sdk-go` — Claude API client

**Research enhancements for Phase 3:**

> **LLM prompt hardening (Injection Analyst + Security Sentinel):**
> 1. **Canary tokens:** Insert a random 8-character token in the system prompt. Ask the model to repeat it in the response. If the canary is missing or altered, the scanned content may have hijacked the prompt → reject.
> 2. **Random delimiters:** Use a randomly generated delimiter (e.g., `===SCAN_abc12345===`) around user content instead of predictable `---` or `` ``` ``. Prevents attackers from closing the fence with a matching delimiter.
> 3. **Temperature 0:** Use `temperature: 0` for deterministic, less manipulable output.
> 4. **Content placement:** Scanned content goes in a `user` message, never in the `system` message. System message contains only the analysis instructions and expected output format.
> 5. **Response validation:** Parse the JSON response strictly. Reject any response that doesn't match the expected schema — a sign the content manipulated the output format.

> **API key validation (Pattern Recognition):** Validate `SKILLBOX_SCANNER_LLM_API_KEY` at startup when `SKILLBOX_SCANNER_LLM_ENABLED=true`. Fail startup (not just first scan) if the key is missing. This follows the existing config validation pattern.

> **Semaphore (Simplicity Reviewer):** Use a simple `make(chan struct{}, maxConcurrent)` semaphore. No need for a configurable semaphore — hardcode the default (5) and make it a config var only if needed later.

**Success criteria:**
- [x] Flagged patterns (e.g., `subprocess.Popen` used for legitimate git commands) are correctly classified as non-malicious by LLM
- [x] Obfuscated malicious code that passes static checks is caught by LLM analysis
- [x] LLM prompt uses canary tokens, random delimiters, temperature 0
- [x] Scanned content is in `user` message only, never `system`
- [x] LLM timeout/failure returns `(nil, error)` — caller rejects with 500
- [x] Concurrent uploads respect the semaphore (max 5 LLM calls)
- [x] API key validated at startup, not first scan

#### Phase 4: Pluggable External Scanners

**Deliverables:**
- External scanner stage (runs in Tier 2 when configured)
  - `ExternalScanner` interface: `Scan(ctx, zr *zip.Reader) ([]Finding, error)`
  - ClamAV implementation via `go-clamd` (Unix socket or TCP)
  - YARA implementation via `yargo` (pure Go, no CGO)
  - When unconfigured: stage is skipped entirely (nil check, not a no-op struct)
  - Fail closed: ClamAV/YARA unavailable → return `(nil, error)`
  - ClamAV runs in Tier 2 (before LLM) — it's fast (~50ms) and catches binary malware that other stages miss
- Config: `SKILLBOX_SCANNER_EXTERNAL_TYPE` (string: `none`, `clamav`, `yara`)
- Config: `SKILLBOX_SCANNER_CLAMAV_ADDRESS` (string, e.g., `unix:/run/clamav/clamd.ctl` or `tcp://127.0.0.1:3310`)
- Config: `SKILLBOX_SCANNER_YARA_RULES_DIR` (string, path to `.yar` rule files)
- Docker Compose update: optional `clamav` sidecar service
- Unit tests with mock clamd server
- `POST /v1/skills/validate` dry-run endpoint (from Agent-Native Reviewer)

**Files to create:**
- `internal/scanner/stage_external.go` — interface + nil-check skip logic
- `internal/scanner/clamav.go` — ClamAV clamd client
- `internal/scanner/yara.go` — YARA scanner
- `internal/scanner/stage_external_test.go`

**Dependencies:**
- `github.com/Lyimmi/go-clamd` — ClamAV daemon client
- `github.com/sansecio/yargo` — Pure Go YARA engine

**Files to modify:**
- `deploy/docker/docker-compose.yml` — optional ClamAV sidecar
- `internal/api/handlers/skill.go` — add `ValidateSkill` handler (dry-run scan, no storage)
- `internal/api/router.go` — register `POST /v1/skills/validate`

**Research enhancements for Phase 4:**

> **Validate endpoint (Agent-Native Reviewer):** Add `POST /v1/skills/validate` that runs the full scanner pipeline but does NOT store the skill. Returns the same 422 body on failure or 200 with scan summary on success. This lets agents pre-flight check skills before committing to upload. Same auth as upload.

> **EICAR test verification (Deployment Verification):** Include an EICAR test file in the test suite to verify ClamAV integration. EICAR is the standard antivirus test pattern — ClamAV must detect it for the integration to be considered working.

> **ClamAV chunk size (Performance Oracle):** Send ZIP content to clamd in 64KB chunks via `INSTREAM`. Default chunk size in go-clamd may be too small, causing excessive syscalls.

**Success criteria:**
- ClamAV detects EICAR test file and known malware samples
- YARA rules match crafted test payloads
- Unconfigured external scanner adds zero latency (nil check, not a no-op call)
- ClamAV/YARA unavailability returns `(nil, error)` — caller rejects with 500
- `POST /v1/skills/validate` performs dry-run scan without storing the skill
- ClamAV runs in Tier 2, before LLM (Tier 3)

## Future Ideas

- **Quarantine bucket**: Upload rejected skills to a separate MinIO bucket (`quarantine`) with metadata (scan_id, categories, tenant_id, timestamp) for forensic analysis, pattern improvement, and false positive recovery. Needs retention policy and strict access controls.

## Key Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Scanning model | 3-tier: quick scan → deep scan → LLM | Most uploads (~95%) accept at Tier 1 (<100ms). Deeper tiers only run when needed. Cost and latency proportional to risk. |
| HTTP status for rejection | `422 Unprocessable Entity` | Structurally valid ZIP, rejected on security policy. Distinct from 400 (malformed), 413 (too large), and 500 (infra failure). |
| Error response body | `{ "error": "security_scan_failed", "scan_id": "<uuid>", "categories": ["<category>", ...] }` | Agent-parseable (array of categories, not just one). `scan_id` for correlation with audit logs. No stage number or specific rule — avoids leaking pipeline internals. |
| Infra error vs verdict | `(nil, error)` vs `(result, nil)` | Caller can distinguish "scan is broken" (→ 500) from "scan says reject" (→ 422). Never mix: no `(result, error)`. |
| Tier 3 trigger | Unresolved flags after Tier 2 | Cost optimization. Clean uploads skip LLM entirely. Only ambiguous findings reach the LLM. |
| External scanner failure | Fail closed: `(nil, error)` | Consistent with LLM failure policy. Availability > permissiveness for a security gate. |
| LLM content cap | 100KB per file, entrypoint + SKILL.md only | Prevents context overflow and controls cost. Larger files rely on static analysis. |
| Scan audit log | slog structured fields (v1); PostgreSQL table deferred to v2 | Keeps v1 simple. Structured slog is queryable with log aggregation. DB table adds migration complexity. |
| `preinstall`/`postinstall` hooks | Block-level | Install scripts run before sandbox network-deny. Cannot be safely deferred to LLM judgment. |
| Typosquatting threshold | Levenshtein ≤ 1 from top-1000 = block; ≤ 2 = flag | Distance 1 has very high signal. Distance 2 has higher FP rate, appropriate for flag-level. |
| Binary file handling | Skip (detect via `DetectContentType`) | Regex on binary content produces meaningless matches. Binary malware is ClamAV/YARA's domain. |
| Re-upload scanning | Always scan, regardless of name/version match | Prevents bypassing scanner via overwrite. Consistent with the principle that no unscanned content enters storage. |
| Scanner in tests | No-op `Scanner` implementation for handler tests | Follows existing DI pattern. Tests that specifically test scanning use the real pipeline with in-memory ZIPs. |
| Config reload | Restart required (v1) | Matches existing config pattern (loaded once at startup). Hot-reload deferred to v2. |
| Regex engine | Go stdlib `regexp` only (RE2) | RE2 guarantees linear-time matching, eliminating ReDoS risk. Never use `regexp2` or PCRE. |
| ZIP input type | `*zip.Reader` (not `[]byte`) | Avoids 3x memory amplification from re-parsing. Upload handler already parses the ZIP. |
| Severity levels | BLOCK + FLAG only | INFO was removed — it added a code path without affecting verdicts. Stages log informational observations directly via slog. |
| Scanner disable logging | Emit `slog.Warn` at startup if `ENABLED=false` | Prevents silent misconfiguration in production. Ops teams notice immediately. |

## Acceptance Criteria

### Functional Requirements

- [x] All uploaded skills pass through the security scanner before storage
- [x] ZIP bomb protection rejects archives exceeding 50MB decompressed, 500 entries, or 100:1 ratio
- [x] Reverse shell patterns (`nc -e`, `/dev/tcp/`, `mkfifo`, `bash -i >& /dev/tcp/`) are blocked
- [x] Piped execution patterns (`curl | bash`, `wget | sh`, `base64 -d | bash`) are blocked
- [x] Known-malicious packages from OSSF blocklist are blocked (Tier 1 exact-match)
- [x] Typosquatted package names are detected (block at distance ≤ 1, flag at ≤ 2) (Tier 2)
- [x] Homoglyph package names (mixed Latin/Cyrillic) are blocked (Tier 2)
- [x] `preinstall`/`postinstall` npm hooks are blocked
- [x] Prompt injection patterns (override instructions, role hijacking) are detected in SKILL.md
- [x] Tool-call injection patterns (`<tool_use>`, `<function_call>`) are detected in SKILL.md
- [x] Delimiter injection patterns (`</system>`, `Human:`, `Assistant:`) are detected
- [x] Invisible Unicode characters are flagged in SKILL.md and code files
- [x] Unicode NFC normalization is applied before all pattern matching
- [x] MCP server references in SKILL.md are flagged
- [x] Ambiguous patterns (`subprocess`, `eval`) are flagged, not blocked, and escalated to Tier 3
- [ ] LLM analysis distinguishes legitimate use of flagged APIs from malicious use
- [ ] LLM unavailability returns `(nil, error)` — caller rejects with 500 (fail closed)
- [ ] ClamAV integration detects EICAR test file and known malware when configured
- [ ] YARA rules match crafted test payloads when configured
- [ ] Unconfigured external scanners add zero latency (nil check, no function call)
- [x] Cognitive-mode skills (SKILL.md only, no code) are scanned by Tier 2 prompt injection and optionally Tier 3
- [x] All files in ZIP are scanned regardless of extension (no extension-based filtering)
- [x] Binary files are skipped for regex scanning (detected via `DetectContentType`)
- [x] Files > 1MB are skipped for regex scanning
- [x] Scan verdicts are logged with structured fields: tenant_id, skill_name, version, verdict, tier, categories, duration_ms, scan_id
- [x] Clean uploads accept at Tier 1 without triggering Tier 2/3
- [ ] `POST /v1/skills/validate` performs dry-run scan without storage (Phase 4)

### Non-Functional Requirements

- [x] Tier 1 (quick scan) completes in <100ms for typical skills
- [x] Tier 2 (deep scan) completes in <500ms for typical skills (<5MB)
- [ ] Tier 3 (LLM) completes in <10s when triggered
- [x] Total scan pipeline respects the configured timeout (default 30s)
- [x] Concurrent uploads are safe (no shared mutable state in scanner)
- [ ] LLM concurrent calls are bounded by semaphore (default max 5)
- [x] All regex patterns compiled once at init, never inside Scan()
- [x] ZIP is read as `*zip.Reader`, not re-parsed from `[]byte`
- [ ] Zero false positives on existing uploaded skills (run against current MinIO contents as validation)
- [x] Go stdlib `regexp` only (RE2) — no `regexp2` or PCRE (ReDoS prevention)

### Quality Gates

- [x] Unit tests for every scan stage with table-driven test cases
- [x] In-memory ZIP construction for test cases (no fixture files)
- [x] Mock scanner for handler tests (interface-based DI)
- [x] Handler returns 422 with structured body (`scan_id`, `categories` array) on security rejection
- [x] Handler returns 500 on scanner infrastructure failure (distinct from 422)
- [x] Handler returns 201 with skill metadata on scan pass
- [x] Config test coverage for all new `SKILLBOX_SCANNER_*` env vars
- [ ] API key validated at startup when LLM is enabled
- [x] `SKILLBOX_SCANNER_ENABLED=false` emits startup warning

## Dependencies & Prerequisites

- **Phase 1:** No external dependencies (pure Go `regexp` + stdlib `archive/zip` + `net/http.DetectContentType`)
- **Phase 2:** `github.com/google/osv-scanner` for dependency parsing and OSV queries; `golang.org/x/text/unicode/norm` for NFC normalization
- **Phase 3:** `github.com/anthropics/anthropic-sdk-go` for Claude API; requires `SKILLBOX_SCANNER_LLM_API_KEY` at startup
- **Phase 4:** `github.com/Lyimmi/go-clamd` for ClamAV; `github.com/sansecio/yargo` for YARA; optional ClamAV sidecar container

Phases are independently deployable. Phase 1 alone provides significant security value — ZIP bomb protection, static pattern detection, and dep blocklist cover the most common attack vectors.

## Risk Analysis & Mitigation

| Risk | Impact | Mitigation |
|------|--------|------------|
| **ZIP bomb / resource exhaustion** (Critical) | OOM or CPU starvation on upload | `CheckZIPSafety` runs before scanner: 50MB decompressed cap, 500 entry limit, 100:1 ratio check. Per-file 1MB cap for regex scanning. |
| **ReDoS via crafted input** (Critical) | Scanner hangs, blocks uploads | Go stdlib `regexp` only (RE2, guaranteed linear time). Never use `regexp2` or PCRE. All patterns validated with `MustCompile` at init. |
| **TOCTOU on ZIP data** (High) | Attacker swaps content between scan and storage | Hash ZIP bytes before scan, compare hash after scan completes, reject if mismatch. Single goroutine per upload. |
| **LLM prompt injection via scanned content** (High) | Scanner returns false negative | Canary tokens, random delimiters, temperature 0, content in `user` message only, JSON response validation. |
| False positives block legitimate skills | Upload failures, user frustration | Block/flag severity split; tiered model (Tier 1 accepts clean uploads); LLM contextual analysis for ambiguous patterns; zero-FP validation against existing skills |
| LLM adds latency to uploads | Slower upload UX | Tier 3 only triggered on unresolved flags (not all); 10s timeout; opt-in via config |
| ClamAV sidecar adds operational complexity | Deployment burden | Pluggable interface; ClamAV is opt-in; nil-check skip when unconfigured |
| Bundled blocklist becomes stale | Misses newly discovered malicious packages | Document update cadence; OSV live query supplements static blocklist |
| Scanner bypass via alternate code paths | Unscanned skills in storage | Scanner is injected at the only upload entry point; no other code path calls `registry.Upload` |
| Scanner disabled in production | Security gate bypassed | `SKILLBOX_SCANNER_ENABLED=false` emits `slog.Warn` at startup; detectable by monitoring |
| File extension bypass | Malicious code in `.txt` or `.dat` files | Scan ALL files regardless of extension; use content heuristics for file type detection |

## Configuration Reference

| Env Var | Type | Default | Phase | Description |
|---------|------|---------|-------|-------------|
| `SKILLBOX_SCANNER_ENABLED` | bool | `true` | 1 | Enable/disable the entire scanner |
| `SKILLBOX_SCANNER_TIMEOUT` | duration | `30s` | 1 | Total scan timeout |
| `SKILLBOX_SCANNER_LLM_ENABLED` | bool | `false` | 3 | Enable LLM deep analysis stage |
| `SKILLBOX_SCANNER_LLM_API_KEY` | string | — | 3 | Anthropic API key (required if LLM enabled) |
| `SKILLBOX_SCANNER_LLM_MODEL` | string | `claude-haiku-4-5-20251001` | 3 | Claude model for analysis |
| `SKILLBOX_SCANNER_LLM_TIMEOUT` | duration | `10s` | 3 | LLM call timeout |
| `SKILLBOX_SCANNER_LLM_MAX_CONCURRENT` | int | `5` | 3 | Max concurrent LLM calls |
| `SKILLBOX_SCANNER_EXTERNAL_TYPE` | string | `none` | 4 | External scanner: `none`, `clamav`, `yara` |
| `SKILLBOX_SCANNER_CLAMAV_ADDRESS` | string | — | 4 | ClamAV address (unix:/path or tcp://host:port) |
| `SKILLBOX_SCANNER_YARA_RULES_DIR` | string | — | 4 | Directory containing .yar rule files |

## Deployment & Rollback (from Deployment Verification Agent)

### Per-Phase Go/No-Go

**Phase 1:**
- [ ] All existing skills in MinIO pass Tier 1 without false positives (run batch validation script)
- [ ] Tier 1 latency <100ms at p99 under load test (50 concurrent uploads)
- [ ] `SKILLBOX_SCANNER_ENABLED=true` in staging for 24h with zero regressions
- [ ] Rollback: set `SKILLBOX_SCANNER_ENABLED=false` and restart — instant, no migration

**Phase 2:**
- [ ] All existing skills pass Tier 2 without false positives
- [ ] OSV API reachable from production network (or timeout gracefully)
- [ ] Rollback: revert to Phase 1 binary — Tier 2 stages simply don't exist

**Phase 3:**
- [ ] LLM API key validated at startup
- [ ] Canary token verification works end-to-end
- [ ] Rollback: set `SKILLBOX_SCANNER_LLM_ENABLED=false` and restart

**Phase 4:**
- [ ] EICAR test file detected by ClamAV
- [ ] `/v1/skills/validate` endpoint returns correct responses
- [ ] Rollback: set `SKILLBOX_SCANNER_EXTERNAL_TYPE=none` and restart

### Load Testing Scenarios

1. **Burst upload:** 50 concurrent skill uploads of 1-5MB ZIPs. Target: <500ms p99 for Tier 1+2.
2. **Large skill:** Single 10MB ZIP with 200+ files. Target: <2s for full Tier 1+2.
3. **LLM concurrency:** 10 concurrent uploads that all trigger Tier 3. Target: semaphore limits to 5 concurrent, queue works correctly.
4. **ZIP bomb:** Upload a 42.zip (recursive ZIP bomb). Target: rejected by `CheckZIPSafety` in <10ms.

## References & Research

### Internal References

- Upload handler: `internal/api/handlers/skill.go:31-135`
- Skill validation: `internal/skill/skill.go:69-144`
- Image allowlist: `internal/runner/security.go:13-32`
- Path validator: `internal/sandbox/path_validator.go:26-70`
- Config loading: `internal/config/config.go:96-228`
- Router wiring: `internal/api/router.go:25`
- Server main: `cmd/skillbox-server/main.go:80`
- Error pattern: `internal/runner/errors.go`
- Test pattern (table-driven): `internal/runner/security_test.go:8-75`
- Test pattern (in-memory ZIP): `internal/registry/loader_test.go:11-53`
- Sentinel error learning: `docs/solutions/runtime-errors/minio-error-sentinels-not-propagated-registry-20260226.md`
- Brainstorm: `docs/brainstorms/2026-03-05-skill-security-scanner-brainstorm.md`

### External References

- OSV Scanner (Go dependency scanning): https://github.com/google/osv-scanner
- OSSF Malicious Packages database: https://github.com/ossf/malicious-packages
- go-clamd (ClamAV Go client): https://github.com/Lyimmi/go-clamd
- Yargo (Pure Go YARA engine): https://sansec.io/research/yargo
- TypoSmart research (typosquatting detection): https://arxiv.org/html/2502.20528v1
- Agent Skills Prompt Injection research: https://arxiv.org/html/2510.26328v1
- OWASP LLM Prompt Injection Prevention: https://cheatsheetseries.owasp.org/cheatsheets/LLM_Prompt_Injection_Prevention_Cheat_Sheet.html
- GuardDog (malicious package detection): https://github.com/DataDog/guarddog
- YARA Forge (curated rules): https://yarahq.github.io/
