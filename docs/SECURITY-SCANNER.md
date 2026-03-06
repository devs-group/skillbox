# Security Scanner — Implementation Guide

The security scanner inspects uploaded skill ZIPs **before** they reach MinIO storage. It uses a tiered model where most clean uploads are accepted in under 1ms, while suspicious content is escalated through progressively deeper analysis.

## Architecture Overview

```
UploadSkill → normalizeSkillZip → validateSkillZip → parsedSkill.Validate()
  → CheckZIPSafety(zipData)                        ← ZIP bomb protection
  → scanner.Scan(ctx, zipReader, parsedSkill)       ← tiered scanning
  → registry.Upload → store.UpsertSkill
```

The scanner sits in `internal/scanner/` and exposes a single `Scanner` interface:

```go
type Scanner interface {
    Scan(ctx context.Context, zr *zip.Reader, s *skill.Skill) (*ScanResult, error)
}
```

**Error contract:**
- `(result, nil)` — scan completed. Check `result.Pass` for the verdict.
- `(nil, error)` — infrastructure failure. Caller rejects the upload (fail closed) and returns HTTP 500.
- Never returns `(result, error)`.

## Tiered Scanning Model

```
                    ┌─────────────────┐
                    │   ZIP Upload    │
                    └────────┬────────┘
                             │
                    ┌────────▼────────┐
                    │  ZIP Bomb Check │  50MB cap, 500 entries, 100:1 ratio
                    └────────┬────────┘
                             │
               ┌─────────────▼─────────────┐
               │  TIER 1: Quick Scan       │  < 100ms
               │  - Static regex patterns  │
               │  - Dependency blocklist   │
               └─────────┬────────┬────────┘
                         │        │
                    no flags   flags or dep files present
                         │        │
                    ┌────▼────┐   │
                    │ ACCEPT  │   │
                    └─────────┘   │
                         ┌────────▼────────────┐
                         │  TIER 2: Deep Scan  │  < 500ms
                         │  - Typosquat check  │
                         │  - Prompt injection │
                         │  - Homoglyph detect │
                         │  - Install hooks    │
                         └────────┬───────┬────┘
                                  │       │
                             resolved   unresolved flags
                                  │       │
                             ┌────▼────┐  │
                             │ verdict │  │
                             └─────────┘  │
                                  ┌───────▼────────────┐
                                  │  TIER 3: LLM       │  < 10s (opt-in)
                                  │  - Claude analysis  │
                                  │  - Canary tokens   │
                                  │  - Prompt hardening│
                                  └───────┬────────────┘
                                          │
                                     ┌────▼────┐
                                     │ verdict │
                                     └─────────┘
```

**Short-circuit optimization:** ~95% of uploads contain no suspicious patterns and are accepted at Tier 1 without ever reaching Tier 2 or 3. Tier 2 runs only when Tier 1 produces FLAG findings or when dependency manifests (`requirements.txt`, `package.json`, `pyproject.toml`) are present. Tier 3 runs only when Tier 2 leaves unresolved FLAG findings and LLM analysis is enabled.

## File Structure

```
internal/scanner/
├── scanner.go               # Pipeline struct, Scan() method, tier orchestration
├── result.go                # Finding, ScanResult, Severity types
├── errors.go                # ErrBlocked, ErrScanFailed sentinel errors
├── metrics.go               # Atomic scan counters (pass/block/fail, per-tier, per-category)
├── zipcheck.go              # CheckZIPSafety — zip bomb protection
├── stage_patterns.go        # Tier 1: static regex patterns + dep blocklist
├── corpus.go                # ~200 popular PyPI/npm package names for typosquat detection
├── stage_deps.go            # Tier 2: typosquatting, homoglyphs, install hooks
├── stage_deps_test.go       # Tier 2 deps tests (12 cases)
├── stage_prompt.go          # Tier 2: prompt injection, delimiter injection, invisible Unicode
├── stage_prompt_test.go     # Tier 2 prompt tests (17 cases)
├── stage_external.go        # Tier 2: pluggable external scanner interface + stage wrapper
├── clamav.go                # ClamAV clamd client (native INSTREAM protocol)
├── yara.go                  # YARA scanner (shells out to yara binary)
├── stage_external_test.go   # External scanner tests (ClamAV, YARA, metrics — 20 cases)
├── stage_llm.go             # Tier 3: LLM deep analysis via Claude API
├── stage_llm_test.go        # Tier 3 LLM tests (13 cases, mock HTTP server)
└── scanner_test.go          # Integration tests + ZIP safety tests
```

## Findings Model

Every scan observation is a `Finding`:

```go
type Finding struct {
    Stage       string   // "static_patterns", "dependencies", "prompt_injection", "llm_analysis"
    Severity    Severity // "BLOCK" or "FLAG"
    Category    string   // e.g. "reverse_shell", "typosquat_package", "llm_threat"
    FilePath    string   // relative path within the ZIP
    Description string   // human-readable explanation
}
```

- **BLOCK** — immediate rejection. The skill is never stored.
- **FLAG** — suspicious but possibly legitimate. Escalated to a deeper tier for contextual judgment.

## Tier 1: Quick Scan (`stage_patterns.go`)

Runs compiled regexes against every text file in the ZIP. Scans all file extensions (not just `.py`/`.js`/`.sh`), skips binary files and files over 1MB.

### Block Patterns (always reject)

| Category | Examples |
|----------|----------|
| `reverse_shell` | `nc -e /bin/sh`, `/dev/tcp/`, `mkfifo` + `nc`, `socat exec` |
| `piped_execution` | `curl \| bash`, `wget \| sh`, `base64 -d \| bash` |
| `crypto_miner` | `import xmrig`, `stratum+tcp://`, `coinhive` |
| `sandbox_escape` | `nsenter`, `unshare --mount`, `/proc/*/ns/` |
| `destructive_command` | `rm -rf /` |
| `fork_bomb` | `:(){ :\|:& };:` |
| `obfuscation` | Base64 blob > 256 chars |
| `malicious_package` | Known-malicious packages from OSSF/npm advisories (exact match) |

### Flag Patterns (escalate to Tier 2)

| Category | Examples |
|----------|----------|
| `process_execution` | `subprocess.Popen`, `os.system`, `child_process`, `execSync` |
| `dynamic_execution` | `eval()`, `exec()`, `Function()`, `compile()` |
| `network_access` | `socket.connect`, `requests.get`, `urllib.request` |
| `sensitive_file_access` | `/etc/passwd`, `/etc/shadow`, `~/.ssh/`, `.env` |
| `hardcoded_ip` | Any IP address literal |

### Dependency Blocklist

The `blocklistPackages` map in `stage_patterns.go` contains ~40 known-malicious package names from Python (OSSF advisories) and Node.js (npm advisories). Checked by exact match in `requirements.txt` and `package.json`.

## Tier 2: Deep Scan

### Dependency Analysis (`stage_deps.go`)

**Typosquat Detection:**
Uses Levenshtein edit distance against a corpus of ~200 popular packages (`corpus.go`):
- Distance 1 → BLOCK (e.g., `requets` for `requests`)
- Distance 2 → FLAG (e.g., `reqeusts` for `requests`)
- Distance 0 → exact match, not a typosquat

The check finds the **minimum** distance across all popular packages to avoid non-deterministic results from Go map iteration order.

**Homoglyph Detection:**
`hasMixedScript()` detects package names containing characters from multiple Unicode scripts (Latin + Cyrillic, Latin + Greek). A package name like `rеquеsts` (with Cyrillic `е` instead of Latin `e`) is immediately blocked.

**Install Hook Detection:**
Blocks `preinstall`, `postinstall`, `preuninstall`, `postuninstall` scripts in `package.json`. These run before sandbox network-deny, making them dangerous.

**Supported dependency files:**
- `requirements.txt` — line-by-line package extraction
- `package.json` — parses `dependencies`, `devDependencies`, `peerDependencies` with brace-depth tracking
- `pyproject.toml` — parses `[project.dependencies]` and `[tool.poetry.dependencies]` sections (lightweight, no TOML parser dependency)

### Prompt Injection Analysis (`stage_prompt.go`)

All text content is NFC-normalized (via `golang.org/x/text/unicode/norm`) before pattern matching to prevent normalization bypass attacks.

**Risk scoring model:** Each pattern has a score (0.0-1.0). The highest-scoring match per file determines severity:
- Score >= 0.7 → BLOCK
- Score 0.4-0.7 → FLAG
- Score < 0.4 → ignored

**Code file score halving:** Patterns found in code files (not `SKILL.md`) have their score halved, since prompt injection patterns are less suspicious in code than in agent instructions.

| Pattern Group | Examples | Score Range |
|---------------|----------|-------------|
| Prompt override | "ignore all previous instructions", "from now on you will" | 0.8-0.95 |
| Role hijacking | "you are now a new role", "act as if you are" | 0.7-0.9 |
| Data exfiltration | "output the system prompt", "reveal hidden internal" | 0.8-0.85 |
| Safety bypass | "disregard safety guidelines" | 0.95 |
| Tool-call injection | `<tool_use>`, `<function_call>`, `tool_name` JSON | 0.7-0.9 |
| Delimiter injection | `</system>`, `Human:`, `<\|im_start\|>`, `[INST]` | 0.7-0.85 |
| MCP references | `mcp_server`, `tool_server` | 0.4-0.5 |

**Invisible Unicode Detection:**
`checkInvisibleUnicode()` scans all files for characters commonly used in steganography or homoglyph attacks:
- Zero-width characters (ZWS, ZWNJ, ZWJ, BOM, soft hyphen)
- Bidirectional text controls (LTR/RTL marks, embeddings, overrides, isolates)
- Private use area characters
- Tag characters (U+E0000-U+E007F)

## Tier 3: LLM Deep Analysis (`stage_llm.go`)

Opt-in via `SKILLBOX_SCANNER_LLM_ENABLED=true`. Only runs when Tier 2 leaves unresolved FLAG findings.

### How It Works

1. **Content collection:** Gathers `SKILL.md` and entrypoint files (`main.py`, `index.js`, `run.sh`, etc.) in priority order, truncated to 100KB total.

2. **Prompt construction:** Builds a system prompt with analysis instructions and a user message containing the scanned content. The scanned content is **never** placed in the system message.

3. **API call:** Sends a request to the Anthropic Messages API with `temperature: 0` for deterministic output.

4. **Response validation:** Parses the JSON response, validates the canary token, checks confidence range.

5. **Verdict:** If the LLM says benign → clears the flags. If threat with confidence >= 0.7 → BLOCK. If threat with confidence < 0.7 → FLAG.

### Prompt Hardening

Four defense mechanisms prevent the scanned content from hijacking the LLM:

**1. Canary Tokens:**
A random 8-character alphanumeric token is inserted in the system prompt. The model must echo it in the `"canary"` field of its JSON response. If the canary is missing or altered, the scanned content likely manipulated the prompt → the scanner returns an error (fail closed).

**2. Random Delimiters:**
Scanned content is wrapped in random delimiters like `===SCAN_a7b3k9m2x1p4===` instead of predictable `---` or triple backticks. This prevents attackers from closing the fence with a matching delimiter.

**3. Temperature 0:**
Deterministic output reduces the LLM's susceptibility to manipulation.

**4. Content Placement:**
Scanned content goes in the `user` message only. The `system` message contains only analysis instructions and the expected JSON schema.

### Concurrency Control

A buffered channel semaphore limits concurrent LLM API calls (default: 5). This prevents 429 rate limit storms during burst uploads.

```go
semaphore: make(chan struct{}, cfg.MaxConcurrent)
```

### LLM Response Schema

```json
{
  "canary": "<echoed-canary-token>",
  "threat": true,
  "confidence": 0.95,
  "reasoning": "Code obfuscates a reverse shell using base64 encoding",
  "categories": ["malicious_code", "obfuscation"]
}
```

Valid categories: `malicious_code`, `data_exfiltration`, `prompt_injection`, `obfuscation`, `dependency_risk`, `sandbox_escape`, `legitimate_usage`.

### Fail-Closed Behavior

Any LLM error results in `(nil, error)`, which the upload handler treats as an infrastructure failure (HTTP 500, upload rejected):

- API timeout
- HTTP 5xx from Anthropic
- HTTP 429 rate limit
- Malformed JSON response
- Canary mismatch (possible prompt hijacking)
- Confidence out of range [0, 1]

## ZIP Bomb Protection (`zipcheck.go`)

Runs **before** the scanner pipeline. Checks:

| Check | Limit | Rationale |
|-------|-------|-----------|
| Total decompressed size | 50 MB | Prevents memory exhaustion |
| Entry count | 500 files | Prevents file descriptor exhaustion |
| Compression ratio | 100:1 per file | Detects zip bomb quines |
| Nested archives | Rejected | `.zip`, `.tar`, `.gz`, `.7z`, etc. inside ZIP |

## Configuration

All config is via environment variables, following 12-factor methodology.

| Variable | Type | Default | Phase | Description |
|----------|------|---------|-------|-------------|
| `SKILLBOX_SCANNER_ENABLED` | bool | `true` | 1 | Enable/disable the entire scanner |
| `SKILLBOX_SCANNER_TIMEOUT` | duration | `30s` | 1 | Total scan timeout (all tiers) |
| `SKILLBOX_SCANNER_LLM_ENABLED` | bool | `false` | 3 | Enable LLM deep analysis (Tier 3) |
| `SKILLBOX_SCANNER_LLM_API_KEY` | string | — | 3 | Anthropic API key (required if LLM enabled) |
| `SKILLBOX_SCANNER_LLM_MODEL` | string | `claude-haiku-4-5-20251001` | 3 | Claude model for analysis |
| `SKILLBOX_SCANNER_LLM_TIMEOUT` | duration | `10s` | 3 | Per-LLM-call timeout |
| `SKILLBOX_SCANNER_LLM_MAX_CONCURRENT` | int | `5` | 3 | Max concurrent LLM API calls |
| `SKILLBOX_SCANNER_EXTERNAL_TYPE` | string | `none` | 4 | External scanner: `none`, `clamav`, `yara` |
| `SKILLBOX_SCANNER_CLAMAV_ADDRESS` | string | `tcp://127.0.0.1:3310` | 4 | ClamAV clamd address (TCP or Unix socket) |
| `SKILLBOX_SCANNER_YARA_RULES_DIR` | string | — | 4 | Directory containing `.yar`/`.yara` rule files |

**Startup validation:**
- `SKILLBOX_SCANNER_ENABLED=false` emits `slog.Warn` at startup
- `SKILLBOX_SCANNER_LLM_ENABLED=true` without `SKILLBOX_SCANNER_LLM_API_KEY` fails startup

## HTTP Response on Rejection

When a skill is blocked, the upload handler returns HTTP 422 with a structured body:

```json
{
  "error": "security_scan_failed",
  "message": "upload rejected by security scan",
  "details": {
    "scan_id": "a1b2c3d4-...",
    "categories": ["reverse_shell", "piped_execution"]
  }
}
```

Infrastructure failures (scanner unavailable) return HTTP 500.

## Pluggable External Scanners

External scanners (ClamAV, YARA) run in Tier 2 alongside dependency and prompt analysis. When unconfigured (`SKILLBOX_SCANNER_EXTERNAL_TYPE=none`, the default), the external stage is not added to the pipeline — zero overhead via nil check, not a no-op struct.

### ClamAV (`clamav.go`)

Native implementation of the ClamAV `clamd` INSTREAM protocol — no external Go dependency needed.

**Protocol:** Connect via TCP or Unix socket → send `zINSTREAM\0` → send data in 64KB chunks (4-byte big-endian length + data) → send 4 zero bytes → read verdict.

**Responses:**
- `stream: OK` — clean file
- `stream: <virus> FOUND` — malware detected → BLOCK
- `stream: <msg> ERROR` — scan error → fail closed

**Address formats:**
- `tcp://127.0.0.1:3310` — TCP connection
- `unix:/run/clamav/clamd.ctl` — Unix socket

### YARA (`yara.go`)

Shells out to the system `yara` binary — no CGO, no Go YARA bindings needed.

Loads all `.yar`/`.yara` files from the configured rules directory at startup. For each file in the ZIP, writes to a temp file and runs `yara --no-warnings <rule> <file>`. Any match → BLOCK.

**Startup validation:**
- Rules directory must exist and contain at least one `.yar`/`.yara` file
- `yara` binary must be in `$PATH`

### ExternalScanner Interface

```go
type ExternalScanner interface {
    ScanFile(ctx context.Context, filePath string, data []byte) ([]Finding, error)
    Name() string
}
```

## Dry-Run Validation Endpoint

`POST /v1/skills/validate` runs the full scanner pipeline without storing the skill. Same auth as upload. Useful for agents to pre-flight check skills.

- **200 OK** — scan passed, returns `{ "valid": true, "skill_name": "...", "scan_tier": 2, "duration_ms": 45 }`
- **422 Unprocessable Entity** — scan failed, same body as upload rejection

## Scanner Metrics

The pipeline tracks atomic counters for monitoring:

```
GET /v1/admin/scanner/stats
```

Response:
```json
{
  "total_scans": 1542,
  "passed_scans": 1489,
  "blocked_scans": 48,
  "failed_scans": 5,
  "tier1_scans": 1400,
  "tier2_scans": 137,
  "tier3_scans": 5,
  "avg_duration_ms": 12.5,
  "max_duration_ms": 8450.0,
  "block_categories": {
    "reverse_shell": 12,
    "typosquat_package": 8,
    "piped_execution": 6,
    "malware_detected": 3,
    "prompt_injection": 19
  }
}
```

Categories track only BLOCK findings (not FLAGS). Duplicate categories in a single scan are counted once.

## Wiring into the Server

In `cmd/skillbox-server/main.go`:

```go
var sc scanner.Scanner
if cfg.ScannerEnabled {
    var llmCfg *scanner.LLMConfig
    if cfg.ScannerLLMEnabled {
        llmCfg = &scanner.LLMConfig{
            APIKey:        cfg.ScannerLLMAPIKey,
            Model:         cfg.ScannerLLMModel,
            Timeout:       cfg.ScannerLLMTimeout,
            MaxConcurrent: cfg.ScannerLLMMaxConcurrent,
        }
    }
    // Configure external scanner (ClamAV/YARA).
    var ext scanner.ExternalScanner
    switch cfg.ScannerExternalType {
    case "clamav":
        ext, _ = scanner.NewClamAVScanner(cfg.ScannerClamAVAddress)
    case "yara":
        ext, _ = scanner.NewYARAScanner(cfg.ScannerYARARulesDir)
    }

    pipeline = scanner.New(cfg.ScannerTimeout, slog.Default(), llmCfg, ext)
    sc = pipeline
} else {
    sc = &scanner.NoopScanner{}
}
```

The `NoopScanner` is used when scanning is disabled, for tests, and as a fallback.
The `pipeline` variable (non-nil when scanner is enabled) is passed to the router for the metrics endpoint.

## Testing

### Test Skill Generator

`scripts/gen-test-skills/main.go` generates 20 test skill ZIPs covering all detection categories:

```bash
go run ./scripts/gen-test-skills
# Output: scripts/gen-test-skills/out/*.zip
```

### Upload Script

`scripts/gen-test-skills/upload-all.sh` uploads all test ZIPs and verifies expected HTTP status codes:

```bash
./scripts/gen-test-skills/upload-all.sh [BASE_URL] [API_KEY]
# Defaults: http://localhost:8080, test-api-key
```

Expected results:
- 9 BLOCK patterns → 422
- 4 FLAG patterns → 201 (flagged but pass)
- 2 dependency deep scan → 422
- 3 prompt injection → 422
- 1 invisible unicode → 201 (flagged only)
- 1 clean skill → 201

### Unit Tests

```bash
go test ./internal/scanner/ -v
```

- **scanner_test.go**: 22 integration tests (clean skills, reverse shells, crypto miners, fork bombs, blocklisted packages, context cancellation, binary/large file skipping)
- **stage_deps_test.go**: 12 tests (typosquatting at various distances, homoglyphs, install hooks, pyproject.toml, clean deps)
- **stage_prompt_test.go**: 17 tests (all injection categories, score halving, invisible Unicode)
- **stage_llm_test.go**: 13 tests (benign/threat detection, canary validation, API errors, rate limiting, timeouts, semaphore, markdown-wrapped JSON)
- **stage_external_test.go**: 20 tests (mock external scanner, ClamAV INSTREAM protocol with mock clamd server, EICAR detection, fail-closed, address parsing, YARA binary/dir validation, metrics tracking, pipeline integration)

LLM tests use `httptest.Server` to mock the Anthropic Messages API. ClamAV tests use a mock TCP server that speaks the INSTREAM protocol. No real API calls or daemon connections are made.

## Rollback

Instant rollback with no migration:

- **Disable all scanning:** `SKILLBOX_SCANNER_ENABLED=false` and restart
- **Disable LLM only:** `SKILLBOX_SCANNER_LLM_ENABLED=false` and restart (Tier 1+2 continue running)
- **Disable external scanner:** `SKILLBOX_SCANNER_EXTERNAL_TYPE=none` and restart (Tier 1+2 without ClamAV/YARA)

## Future Ideas

- **Quarantine bucket:** Store rejected skills in a separate MinIO bucket for post-mortem analysis
- **Phase 4:** Pluggable external scanners (ClamAV, YARA)
- **`POST /v1/skills/validate`:** Dry-run endpoint that runs the scanner without storing the skill
