# Skill Security Scanner

**Date:** 2026-03-05
**Status:** Brainstorm complete
**TODO ref:** P1 — "Security scanner for uploaded skills"

## What We're Building

A multi-layer security scanner that inspects uploaded skill ZIPs **before** they are stored in the registry. The scanner runs as a pipeline of sequential stages, each progressively deeper. If any stage flags a threat, the upload is **hard rejected** — the skill never reaches MinIO or the database.

The scanner covers five threat categories:
1. **Malicious code patterns** — reverse shells, crypto miners, sandbox escapes, dangerous system calls
2. **Dependency threats** — known-malicious packages, typosquatting in requirements.txt / package.json
3. **Prompt injection** — hidden instructions in SKILL.md, MCP server manipulation, tool hijacking
4. **Obfuscated / novel attacks** — LLM-assisted semantic analysis catches what regex cannot
5. **Virus / malware** — pluggable interface for ClamAV, YARA rules, or other engines

## Why This Approach

**Multi-Layer Pipeline (Approach A)** was chosen over single-pass or LLM-first alternatives because:

- **Fast rejection:** Static pattern checks reject obvious threats in <100ms without LLM cost
- **Defense in depth:** Each layer catches different threat classes; bypassing all layers is harder than bypassing one
- **Cost efficiency:** LLM analysis only runs on skills that pass all static checks
- **Extensibility:** Pluggable scanner interface lets us add ClamAV, YARA, or custom engines without changing core logic
- **YAGNI-compliant:** Ships with built-in heuristics (no extra infra), external engines are opt-in

### Alternatives Considered

| Approach | Why Not |
|----------|---------|
| Single-Pass (B) | No short-circuiting; LLM runs on every upload; harder to extend |
| LLM-First (C) | Expensive per upload; latency on every request; LLM can be bypassed with encoding tricks |

## Key Decisions

1. **Hard reject on failure** — malicious skills are never stored, never executed. Upload returns an error with the failing stage and a general category (e.g., "blocked: malicious code pattern detected") but not the specific regex or rule that triggered, to avoid teaching attackers to evade.

2. **Scan before storage** — the scanner runs after ZIP validation (path traversal, SKILL.md parsing) but before `registry.Upload()`. Malicious content never touches MinIO.

3. **Hybrid analysis** — rule-based static checks first, LLM analysis second. LLM is the last stage and only runs if all prior stages pass.

4. **Pluggable external scanners** — define a `Scanner` interface so ClamAV, YARA, or future engines can be added without touching the pipeline core. Ship with built-in heuristics only.

5. **Pipeline architecture** — scanners run sequentially with short-circuit on first failure. Each scanner returns a verdict (pass/fail) with findings.

## Scanner Pipeline Stages

### Stage 1: Static Pattern Scanner
- Regex-based detection of dangerous patterns across all files in the ZIP
- Patterns are categorized by severity: **block** (always reject) vs. **flag** (escalate to LLM stage for contextual judgment)
- **Block (always reject):** reverse shell patterns (`nc -e`, `/dev/tcp/`, `mkfifo`), `curl | bash` / `wget | sh` piped execution, crypto mining imports, base64-decoded execution (`base64 -d | bash`), sandbox escape attempts
- **Flag (escalate to LLM):** `subprocess.Popen`, `os.system`, `child_process`, `eval()`, `exec()`, `socket.connect`, `net.connect`, `Function()` constructor — these are suspicious but may be legitimate depending on context
- **All languages:** Base64 encoded blobs above threshold size, obfuscated variable names, IP address literals, hardcoded URLs to non-allowlisted domains

### Stage 2: Dependency Scanner
- Parse `requirements.txt` and `package.json`
- Check against known-malicious package database (e.g., maintained blocklist)
- Typosquatting detection (Levenshtein distance against popular package names)
- Flag packages with suspicious install scripts (`preinstall`, `postinstall` in package.json)

### Stage 3: Prompt Injection Scanner
- Analyze SKILL.md for hidden instructions, role overrides, system prompt manipulation
- Detect MCP server references (`mcp://`, tool_use patterns, server connection strings)
- Flag attempts to override sandbox restrictions or escalate permissions
- Check for invisible Unicode characters, zero-width spaces, RTL overrides used to hide content

### Stage 4: LLM Deep Analysis
- Send entrypoint files + SKILL.md to Claude (Haiku for cost efficiency)
- Structured prompt asking for threat assessment across categories
- Also receives any "flag" findings from Stage 1 for contextual judgment (e.g., "this skill uses subprocess — is the usage malicious or legitimate?")
- Returns pass/fail with confidence score and reasoning
- Only runs if stages 1-3 have no **block**-level findings (cost optimization; may still run if there are **flag**-level findings that need contextual review)
- **If LLM is unavailable:** fail closed — reject the upload. Log the failure for ops alerting.

### Stage 5: External Scanner (Pluggable)
- `Scanner` interface: `Scan(ctx, zipBytes) -> ScanResult`
- Built-in: no-op (passes through) — so this stage adds zero latency when no external engine is configured
- Optional: ClamAV client (connects to clamd socket/TCP)
- Optional: YARA rule engine
- Configured via environment variable or config file
- Note: when ClamAV is enabled, consider moving it earlier in the pipeline (before LLM) since it's fast (~50ms) and catches binary malware that other stages miss

## Integration Point

```
Current pipeline:
  UploadSkill → normalizeSkillZip → validateSkillZip → registry.Upload → store.UpsertSkill

New pipeline:
  UploadSkill → normalizeSkillZip → validateSkillZip → securityScan → registry.Upload → store.UpsertSkill
                                                        ^^^^^^^^^^^^
                                                        NEW: inserted here
```

The scanner receives the normalized, validated ZIP bytes and the parsed skill metadata. It returns a `ScanResult` with pass/fail and a list of findings.

## Open Questions

1. **Malicious package database** — maintain our own blocklist or use an existing feed (e.g., OSV, Snyk advisories)?
2. **LLM model choice** — Haiku for speed/cost, or Sonnet for better detection? Configurable?
3. **Scan result logging** — how much detail to log for security audit trail vs. information leakage?
4. **Rate limiting** — should the scanner have its own rate limit separate from the upload endpoint?
5. **Bypass for trusted tenants** — out of scope for v1. All tenants go through the full pipeline.

## Success Criteria

- All uploaded skills pass through the scanner before storage
- Known malicious patterns (reverse shells, crypto miners, prompt injections) are caught and rejected
- Scanner adds <500ms latency for rule-based checks (LLM stage may add 1-3s)
- Pluggable interface allows adding ClamAV without code changes to the pipeline
- Scan verdicts are logged for security audit
- Zero false positives on the existing skill set
