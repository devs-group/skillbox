# Brainstorm: Registry-Level Scanner Pipeline & Approval Workflow

**Date:** 2026-03-25
**Status:** Ready for planning
**Participants:** Yannick, Claude

## What We're Building

A registry-level scan gate that ensures **every** skill ingestion path (zip upload, from-fields, GitHub install, future sources) passes through the security scanner before becoming available. Skills land in a `pending` state, get scanned asynchronously by a background worker, and are promoted to `available`, sent to `admin review`, or moved to `quarantine` based on scan results and admin-configured approval policy.

### Core Capabilities

1. **Unified ingestion gate** — Replace `reg.Upload()` with `reg.Submit()`. All handlers write to a `pending/` prefix in MinIO. No path can bypass scanning.

2. **Async scan worker** — A background goroutine consumes pending skills (via Go channel or DB polling), runs the tiered scanner, and updates skill status.

3. **Three-bucket model:**
   - `pending/` — newly submitted, awaiting scan
   - `available/` — passed scan + approval, ready for execution
   - `quarantine/` — declined or BLOCK'd, kept indefinitely for threat analysis

4. **Admin-configurable approval policy** — Admin chooses one of:
   - `auto` — clean scans auto-promote; flagged skills need admin review
   - `always` — every skill requires admin sign-off regardless of scan result
   - `none` — all scans auto-promote (trust the scanner, no manual gate)

5. **Configurable scanner stages** — Each tier independently toggleable via API/env config:
   - Tier 1 (patterns): always on by default
   - Tier 2 (deps, prompt injection, security): always on by default
   - Tier 3 (LLM deep analysis): off by default, requires API key to enable

6. **Quarantine for threat intelligence** — Declined/blocked skills stay in quarantine indefinitely. Admin can inspect, export, or use them to improve scan patterns.

7. **Fix and resubmit** — Declined skills are immutable. Uploader must submit a new version. Clean audit trail.

8. **Status via API** — `GET /v1/skills/:name/:version` returns status (`pending`, `scanning`, `review`, `available`, `declined`, `quarantined`). CLI and UI poll this. No push notifications in v1.

## Why This Approach

**Registry-level gate over event-driven workers** because:
- Zero new infrastructure — just a goroutine with a channel, no Redis/NATS needed
- Impossible to bypass — `reg.Upload()` (direct to available) ceases to exist
- Can evolve to event-driven later if we need multi-server workers

**Async over synchronous** because:
- Tier 3 LLM analysis can take 5-30s — too slow for a blocking HTTP request
- Users get instant feedback ("submitted, pending scan") instead of waiting
- Scanner failures don't break uploads

**Reference:** JFrog's AI skill registry uses a similar two-phase model (rapid assessment + deep verification) with attestation-based evidence. Azure ACR has the strongest quarantine model (auto-quarantine → scan → promote/reject). We're taking the simplest version of this pattern.

## Key Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Scan gate location | Registry-level (`reg.Submit()`) | Can't be bypassed, no new infra |
| Processing model | Async background goroutine | LLM scans too slow for sync |
| Approval policy | Admin-configurable (auto/always/none) | Flexibility without complexity |
| Quarantine retention | Indefinite | Enables threat intelligence |
| Resubmit model | New version required | Clean audit trail |
| Notification | API polling only (v1) | Simple, webhooks can come later |
| Scanner stage config | API + env vars, per-tier toggle | Admin controls cost/coverage tradeoff |

## Affected Ingestion Paths

| Path | Current | After |
|------|---------|-------|
| `POST /v1/skills` (zip upload) | `reg.Upload()` + inline scan | `reg.Submit()` → async scan |
| `POST /v1/skills/from-fields` | `reg.Upload()`, **no scan** | `reg.Submit()` → async scan |
| `POST /v1/github/install` | `reg.Upload()`, **no scan** | `reg.Submit()` → async scan |
| `POST /v1/skills/validate` | Scan only, no storage | Unchanged (dry-run) |

## State Transitions

```
submit ──→ [pending] ──→ [scanning] ──→ scan complete
                                            │
                          ┌─────────────────┼─────────────────┐
                          │                 │                 │
                       BLOCK'd           FLAG'd            CLEAN
                          │                 │                 │
                          ▼                 ▼                 ▼
                    [quarantined]    approval_policy?    approval_policy?
                    (threat intel)      │      │            │      │
                                     always  auto        always  auto/none
                                       │      │            │      │
                                       ▼      ▼            ▼      ▼
                                  [review]  [review]   [review]  [available]
                                       │
                              ┌────────┴────────┐
                              │                 │
                           approve           decline
                              │                 │
                              ▼                 ▼
                         [available]        [declined]
```

**Status definitions:**
- `pending` — submitted, queued for scan
- `scanning` — scan worker has picked it up (transient)
- `available` — passed scan + approval, executable
- `declined` — admin manually rejected (human judgment — low quality, policy violation, etc.)
- `quarantined` — scanner BLOCK'd with high-confidence malicious findings (automated, kept for threat analysis)
- `review` — awaiting admin decision (flagged by scanner or policy requires manual approval)

**Key rules:**
- Scanner BLOCKs → quarantine (automated, no admin needed)
- Admin declines → declined (separate from quarantine — different intent)
- Both `declined` and `quarantined` are terminal. New version required to resubmit.
- Only `available` skills can be executed by the runner.

## Data Model Changes

### Skill status (new fields on `sandbox.skills` table)
```
status: pending | scanning | review | available | declined | quarantined
scan_result: JSONB (nullable) — full ScanResult when scan completes
scanned_at: timestamp (nullable)
reviewed_by: text (nullable) — admin who approved/declined
reviewed_at: timestamp (nullable)
```

### Scanner config (per-tenant, `sandbox.scanner_config` table)
```
tenant_id: text (FK, unique)
approval_policy: auto | always | none (default: auto)
tier1_enabled: bool (default true)
tier2_enabled: bool (default true)
tier3_enabled: bool (default false)
tier3_api_key: text (encrypted, nullable)
tier3_model: text (default "claude-sonnet-4-5-20250514")
```

### MinIO bucket layout
```
{tenant}/{skill}/{version}/skill.zip          → available (promoted)
{tenant}/.pending/{skill}/{version}/skill.zip → awaiting scan
{tenant}/.quarantine/{skill}/{version}/skill.zip → blocked/declined
```

## Resolved Questions

1. **Runner execution gate** — Yes, fail-closed. Runner refuses to execute skills not in `available` status.
2. **Rate limiting resubmits** — No. The scanner catches bad skills; rate limiting adds complexity for a rare edge case.
3. **Bulk admin operations** — No, single skill per API call for now. Batch endpoint can come later.
4. **Scan result visibility** — Non-admin users see status + remediation hints only. Full findings (patterns, line numbers) are admin-only to avoid leaking detection logic.

## Out of Scope (for now)

- Continuous re-scanning of already-available skills (JFrog Xray-style)
- Webhook/push notifications for status changes
- Agent-based auto-fix of flagged skills
- Multi-server worker scaling
- OSSF malicious packages feed integration (already stubbed in scanner)
