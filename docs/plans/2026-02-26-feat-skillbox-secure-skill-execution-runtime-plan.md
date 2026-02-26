---
title: "Skillbox: Secure Skill Execution Runtime for AI Agents"
type: feat
date: 2026-02-26
status: draft
source: Agent Skill Execution Dokumentation.docx (PRD v1.0)
---

# Skillbox: Secure Skill Execution Runtime for AI Agents

## Overview

Skillbox is an open-source, self-hostable execution runtime that gives AI agents a single clean API to run sandboxed skill scripts (Python, Node.js, Bash) and receive structured output plus file artifacts. It deploys identically on Docker Compose (local dev) and Kubernetes (production) with zero infrastructure changes between environments.

**Core value proposition:** One API call → sandboxed execution → structured output + file artifacts. Secure by default, not by configuration.

## Problem Statement

Every team building agent infrastructure independently solves the same problem: how to run LLM-generated or tool-defined code securely without giving the execution environment a path into production systems. Existing solutions are either managed-only (E2B, Modal) or require non-K8s orchestrators/KVM hardware. No clean, drop-in OSS option exists for Go-based agent platforms.

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                      Agent / SDK Client                      │
│                  client.Run(ctx, RunRequest)                 │
└──────────────────────────┬──────────────────────────────────┘
                           │ REST / gRPC
┌──────────────────────────▼──────────────────────────────────┐
│                        API Server                            │
│              Go / Gin — Stateless, Horizontally Scalable     │
│  ┌──────────┐ ┌──────────┐ ┌───────────┐ ┌──────────────┐  │
│  │   Auth    │ │  Skill   │ │ Execution │ │   Artifact   │  │
│  │ Middleware│ │ Registry │ │  Handler  │ │   Handler    │  │
│  └──────────┘ └──────────┘ └─────┬─────┘ └──────────────┘  │
└──────────────────────────────────┼──────────────────────────┘
                                   │
┌──────────────────────────────────▼──────────────────────────┐
│                     Docker Runner                            │
│  ┌─────────────────────────────────────────────────────┐    │
│  │  Security: net=none, cap=drop ALL, ro rootfs,       │    │
│  │  PID=128, no-new-privs, user=65534, tmpfs /tmp      │    │
│  └─────────────────────────────────────────────────────┘    │
│              via Socket Proxy (tcp://localhost:2375)         │
└─────────────────────────────────────────────────────────────┘
        │                    │                    │
┌───────▼───────┐  ┌────────▼────────┐  ┌───────▼───────┐
│  PostgreSQL   │  │   MinIO / S3    │  │    Redis      │
│  (metadata)   │  │ (skills+files)  │  │  (cache, opt) │
└───────────────┘  └─────────────────┘  └───────────────┘
```

## Technical Approach

### Project Structure

```
skillbox/
├── cmd/
│   ├── skillbox-server/       # API server binary
│   │   └── main.go
│   └── skillbox/              # CLI tool binary
│       └── main.go
├── internal/
│   ├── api/                   # HTTP/gRPC handlers
│   │   ├── router.go
│   │   ├── middleware/
│   │   │   ├── auth.go
│   │   │   └── tenant.go
│   │   ├── handlers/
│   │   │   ├── execution.go
│   │   │   ├── skill.go
│   │   │   └── health.go
│   │   └── grpc/
│   │       ├── server.go
│   │       └── proto/
│   ├── runner/                # Docker container execution
│   │   ├── runner.go
│   │   ├── container.go
│   │   ├── security.go
│   │   └── cleanup.go
│   ├── registry/              # Skill storage and loading
│   │   ├── registry.go
│   │   ├── loader.go
│   │   └── validator.go
│   ├── store/                 # Database layer
│   │   ├── postgres.go
│   │   ├── migrations/
│   │   │   └── 001_initial.sql
│   │   ├── apikeys.go
│   │   └── executions.go
│   ├── artifacts/             # File artifact handling
│   │   ├── collector.go
│   │   └── presigned.go
│   ├── config/                # Configuration
│   │   └── config.go
│   └── skill/                 # Skill format parsing
│       ├── skill.go
│       └── parser.go
├── sdk/                       # Go SDK (single-file, stdlib-only)
│   └── skillbox.go
├── proto/                     # Protobuf definitions
│   └── skillbox/
│       └── v1/
│           └── skillbox.proto
├── deploy/
│   ├── docker/
│   │   ├── Dockerfile
│   │   ├── Dockerfile.dev
│   │   └── docker-compose.yml
│   └── k8s/
│       ├── base/
│       │   ├── kustomization.yaml
│       │   ├── namespace.yaml
│       │   ├── deployment.yaml
│       │   ├── service.yaml
│       │   ├── configmap.yaml
│       │   ├── networkpolicy.yaml
│       │   └── serviceaccount.yaml
│       └── overlays/
│           ├── dev/
│           │   └── kustomization.yaml
│           └── prod/
│               └── kustomization.yaml
├── examples/
│   ├── skills/
│   │   ├── data-analysis/
│   │   │   ├── SKILL.md
│   │   │   ├── scripts/main.py
│   │   │   └── requirements.txt
│   │   ├── pdf-extract/
│   │   │   ├── SKILL.md
│   │   │   └── scripts/main.py
│   │   └── text-summary/
│   │       ├── SKILL.md
│   │       └── scripts/main.py
│   └── agent-integration/
│       └── main.go
├── docs/
│   ├── SKILL-SPEC.md
│   ├── ARCHITECTURE.md
│   └── API.md
├── scripts/
│   ├── init-minio.sh
│   └── seed-apikey.sh
├── .github/
│   └── workflows/
│       ├── ci.yml
│       └── release.yml
├── go.mod
├── go.sum
├── Makefile
├── LICENSE                    # Apache-2.0
├── README.md
├── CONTRIBUTING.md
├── SECURITY.md
└── .goreleaser.yml
```

### Implementation Phases

---

#### Phase 1: Project Bootstrap + Core Runtime (Foundation)

**Goal:** `curl POST /v1/executions` returns a result from a Docker container.

**Tasks:**

1. **Project initialization**
   - `go mod init github.com/devs-group/skillbox`
   - Set up directory structure as above
   - Configure linting (golangci-lint), formatting (gofmt)
   - Create Makefile with build, test, lint, fmt targets

2. **Configuration system** (`internal/config/config.go`)
   - 12-factor: all config from env vars
   - `SKILLBOX_DB_DSN` — Postgres connection string
   - `SKILLBOX_REDIS_URL` — Redis URL (optional)
   - `SKILLBOX_S3_ENDPOINT`, `SKILLBOX_S3_ACCESS_KEY`, `SKILLBOX_S3_SECRET_KEY`
   - `SKILLBOX_DOCKER_HOST` — Docker socket proxy address (default: tcp://localhost:2375)
   - `SKILLBOX_IMAGE_ALLOWLIST` — comma-separated Docker image allowlist
   - `SKILLBOX_DEFAULT_TIMEOUT` — default execution timeout (default: 120s)
   - `SKILLBOX_API_PORT` — HTTP port (default: 8080)
   - `SKILLBOX_GRPC_PORT` — gRPC port (default: 9090)
   - Optional `~/.skillbox/config.yaml` override for CLI

3. **Database layer** (`internal/store/`)
   - Postgres connection with migration support
   - `001_initial.sql`: `api_keys`, `executions`, `skills_metadata` tables
   - `api_keys`: id, key_hash (SHA-256), tenant_id, name, created_at, revoked_at
   - `executions`: id (UUID), skill_name, skill_version, tenant_id, status, input, output, logs, files_url, files_list, duration_ms, error, created_at
   - Key lookup by SHA-256 hash

4. **Skill format parser** (`internal/skill/`)
   - Parse SKILL.md YAML frontmatter (name, version, description, lang, image, timeout, resources)
   - Validate required fields
   - Resolve default image per language (`python:3.12-slim`, `node:20-slim`, `bash:5`)

5. **Skill registry** (`internal/registry/`)
   - Upload skill zip to MinIO `skills` bucket at `{tenant}/{name}/{version}/skill.zip`
   - Download and unpack skill zip to temp directory
   - List skills per tenant
   - Delete skill version
   - Validate SKILL.md inside zip before accepting

6. **Docker runner** (`internal/runner/`)
   - Create temp workdir: `scripts/` (read-only mount), `out/` (read-write mount)
   - Container creation with basic security (network none, cap drop)
   - Mount `$SANDBOX_INPUT` as env var (JSON string)
   - Mount `$SANDBOX_OUTPUT` pointing to `/sandbox/out/output.json`
   - Mount `$SANDBOX_FILES_DIR` pointing to `/sandbox/out/files/`
   - Mount `$SKILL_INSTRUCTIONS` with SKILL.md body
   - Start container, wait for exit with timeout (Go context)
   - Collect output.json from workdir
   - Force-remove container + delete temp workdir on completion
   - Handle: timeout (kill + cleanup), OOM kill, non-zero exit

7. **REST API** (`internal/api/`)
   - Gin router with health/ready endpoints
   - `POST /v1/executions` — synchronous execution
   - `GET /v1/executions/:id` — fetch result by ID
   - `GET /v1/executions/:id/logs` — get execution logs
   - `POST /v1/skills` — upload skill zip
   - `GET /v1/skills` — list skills
   - `GET /v1/skills/:name/:version` — get skill metadata
   - `DELETE /v1/skills/:name/:version` — delete skill (admin)
   - JSON error responses with consistent format

8. **Docker Compose stack** (`deploy/docker/docker-compose.yml`)
   - Services: api, socket-proxy, postgres, redis, minio, minio-init
   - Socket proxy: tecnativa/docker-socket-proxy with minimal permissions
   - MinIO init: create `skills` and `executions` buckets
   - Postgres init: run migrations
   - API key seeding script
   - Health checks on all services
   - Internal network isolation

**Success criteria:** `docker compose up` → upload a skill → `POST /v1/executions` → get structured JSON result back.

---

#### Phase 2: Security Hardening

**Goal:** No container can reach host network, host filesystem, or Docker daemon directly.

**Tasks:**

1. **Full container hardening** (`internal/runner/security.go`)
   ```go
   // Every container gets ALL of these — non-configurable
   &container.HostConfig{
       NetworkMode:    "none",
       CapDrop:        []string{"ALL"},
       ReadonlyRootfs: true,
       SecurityOpt:    []string{"no-new-privileges:true"},
       PidsLimit:      int64Ptr(128),
       Resources: container.Resources{
           Memory:    memoryLimit,    // from SKILL.md or default
           CPUQuota:  cpuQuota,       // from SKILL.md or default
       },
       Tmpfs: map[string]string{
           "/tmp":  "rw,noexec,nosuid,size=64m",
           "/root": "rw,noexec,nosuid,size=1m",
       },
       User: "65534:65534", // nobody
   }
   ```

2. **Image allowlist enforcement** (`internal/runner/security.go`)
   - Check image against `SKILLBOX_IMAGE_ALLOWLIST` before `ContainerCreate`
   - Default allowlist: `python:3.12-slim, python:3.11-slim, node:20-slim, node:18-slim, bash:5`
   - Reject with clear error if image not in allowlist
   - Log all image validation attempts

3. **Socket proxy configuration**
   - Docker Compose: tecnativa/docker-socket-proxy with only required capabilities
   - `CONTAINERS=1, POST=1` (create/start/stop/remove)
   - `IMAGES=1` (pull allowed images)
   - Everything else disabled: `NETWORKS=0, VOLUMES=0, EXEC=0, SWARM=0`
   - API connects to `tcp://socket-proxy:2375`, never to raw Docker socket

4. **Timeout enforcement**
   - Go context with deadline per execution
   - Default: 120s, per-skill override in SKILL.md (max 10min)
   - On timeout: `ContainerKill` → `ContainerRemove` → cleanup workdir → return timeout error
   - Container wait uses `select` on context.Done() and container exit channel

5. **Resource cleanup guarantees**
   - `defer` chain: container remove → workdir delete → log completion
   - Cleanup runs even on panic (recover + cleanup)
   - Stale container cleanup on API startup (find orphaned containers by label)

**Success criteria:** Security audit — verify no container can reach host network, no capability escalation possible, socket proxy blocks unauthorized Docker API calls.

---

#### Phase 3: Go SDK + File Artifacts

**Goal:** `client.Run()` returns output; `client.DownloadFiles()` extracts files.

**Tasks:**

1. **Go SDK** (`sdk/skillbox.go`)
   - Single file, stdlib-only (net/http, encoding/json, io, os, archive/tar, compress/gzip)
   - `sdk.New(baseURL, apiKey, ...opts) *Client`
   - `client.Run(ctx, RunRequest) (*RunResult, error)`
   - `client.DownloadFiles(ctx, result, destDir) error`
   - `client.RegisterSkill(ctx, zipPath) error`
   - `client.ListSkills(ctx) ([]Skill, error)`
   - `client.Health(ctx) error`
   - Options: `WithTenant(id)`, `WithHTTPClient(hc)`
   - Full error types with status codes
   - Retries with exponential backoff for transient errors

2. **File artifact collection** (`internal/artifacts/`)
   - After container exit, check `/sandbox/out/files/` in workdir
   - If files present: tar + gzip into `files.tar.gz`
   - Upload to MinIO `executions` bucket at `{tenant}/{execution_id}/files.tar.gz`
   - Generate presigned GET URL with 1-hour TTL
   - Return `files_url` and `files_list` (relative paths) in response

3. **Execution history** (`internal/store/executions.go`)
   - Insert execution record at start (status: running)
   - Update on completion (status: success/failed/timeout, output, logs, duration_ms)
   - Query by ID, list by tenant with pagination
   - Prune old executions (configurable retention)

4. **pip/npm install support** (`internal/runner/`)
   - If `requirements.txt` present in skill: run `pip install -r requirements.txt` as init step
   - If `package.json` present: run `npm install` as init step
   - Dependency install runs in a writable tmpfs before rootfs goes read-only
   - Cache strategy: pre-built images with common deps (future optimization)

**Success criteria:** A Go program using the SDK can run a skill, get structured output, and download file artifacts to a local directory.

---

#### Phase 4: Kubernetes Deployment

**Goal:** `kubectl apply -k` works on any standard K8s cluster.

**Tasks:**

1. **Kustomize base** (`deploy/k8s/base/`)
   - `namespace.yaml`: `sandbox` namespace with PSA labels
   - `serviceaccount.yaml`: `sandbox-api` — no cluster-wide perms, automount disabled
   - `deployment.yaml`: 2-container pod (api + socket-proxy sidecar)
   - `service.yaml`: ClusterIP on port 80 (HTTP) and 9090 (gRPC)
   - `configmap.yaml`: non-sensitive config (store endpoint, defaults)
   - `secret.yaml`: placeholder for DB DSN, Redis URL, MinIO creds, API key seed
   - `networkpolicy.yaml`: deny all for sandbox pods; allow API → Postgres/Redis/MinIO

2. **Kustomize overlays**
   - `overlays/dev/`: 1 replica, debug logging, resource requests only
   - `overlays/prod/`: 2+ replicas, resource limits, anti-affinity, PDB

3. **Security context for API pod**
   ```yaml
   securityContext:
     runAsNonRoot: true
     runAsUser: 65534
     readOnlyRootFilesystem: true
     allowPrivilegeEscalation: false
     capabilities:
       drop: ["ALL"]
   ```

4. **Socket proxy sidecar in K8s**
   - Sidecar mounts Docker socket from host
   - API container communicates via localhost:2375
   - Network policy ensures only the sidecar can access Docker daemon

5. **Multi-stage Dockerfile** (`deploy/docker/Dockerfile`)
   - Stage 1: Go build with CGO_ENABLED=0
   - Stage 2: distroless/static or scratch with the binary
   - Non-root user, read-only filesystem

**Success criteria:** Fresh K8s cluster → `kubectl apply -k deploy/k8s/overlays/dev` → system runs → skill execution works.

---

#### Phase 5: CLI Tool

**Goal:** `skillbox skill push` + `skillbox run` works end-to-end.

**Tasks:**

1. **CLI framework** (`cmd/skillbox/`)
   - Cobra-based CLI with root command and subcommands
   - Config: `SKILLBOX_SERVER_URL`, `SKILLBOX_API_KEY` env vars
   - Optional `~/.skillbox/config.yaml` for persistent config
   - Version command with build info

2. **Commands:**
   - `skillbox run <skill> [--input '{}'] [--version latest]` — run skill, print result
   - `skillbox skill package <dir>` — validate and zip a skill directory
   - `skillbox skill push <dir|zip>` — package (if dir) and upload
   - `skillbox skill list` — table of skills with name, version, description
   - `skillbox skill lint <dir>` — validate SKILL.md, check entrypoint, check image
   - `skillbox exec list` — recent executions table
   - `skillbox exec logs <id>` — print execution logs
   - `skillbox server start` — start API server (dev mode)

3. **Output formatting**
   - JSON output by default for `run` (piping-friendly)
   - Table output for `list` commands
   - `--format json|table|yaml` flag
   - Color output for terminal, plain for pipes
   - Progress indicator for execution

**Success criteria:** Developer can manage the full skill lifecycle from the CLI without writing any code.

---

#### Phase 6: Example Skills + Documentation

**Goal:** New developer runs a skill in < 5 minutes from README.

**Tasks:**

1. **Example skills:**
   - **data-analysis** (Python): Takes CSV data as input, runs pandas analysis, outputs summary stats + chart as file artifact
   - **pdf-extract** (Python): Takes PDF URL or base64, extracts text, outputs structured JSON
   - **text-summary** (Python): Takes text input, produces a summary using basic NLP (no external API needed)

2. **README.md** — The front door
   - One-line description + badges
   - "What is Skillbox?" (3 sentences)
   - Quick Start (docker compose up → push skill → run → see result in < 5 min)
   - Architecture diagram (ASCII)
   - Links to detailed docs
   - Contributing + License

3. **docs/SKILL-SPEC.md** — Skill format specification
   - Archive structure
   - SKILL.md frontmatter fields (with examples)
   - I/O contract ($SANDBOX_INPUT, $SANDBOX_OUTPUT, etc.)
   - Language-specific examples (Python, Node.js, Bash)

4. **docs/ARCHITECTURE.md** — System design
   - Component overview
   - Execution flow (numbered steps)
   - Security model (with threat matrix)
   - Deployment modes

5. **docs/API.md** — API reference
   - All REST endpoints with request/response examples
   - Authentication
   - Error codes
   - gRPC service definition

6. **CONTRIBUTING.md** — How to contribute
   - Development setup
   - Running tests
   - Code style
   - PR process

7. **SECURITY.md** — Security policy
   - Reporting vulnerabilities
   - Security model overview
   - Known limitations (shared kernel)

---

#### Phase 7: gRPC API + Polish

**Goal:** Production-quality gRPC API alongside REST, CI pipeline, release automation.

**Tasks:**

1. **Protobuf definitions** (`proto/skillbox/v1/skillbox.proto`)
   - `ExecutionService`: RunSkill, GetExecution, GetExecutionLogs
   - `SkillService`: RegisterSkill, ListSkills, GetSkill, DeleteSkill
   - `HealthService`: Check, Watch

2. **gRPC server** (`internal/api/grpc/`)
   - Mirror of REST functionality
   - Shared business logic layer
   - Streaming logs via server-side stream

3. **CI Pipeline** (`.github/workflows/ci.yml`)
   - Go build + test + lint on PR
   - Integration tests with Docker Compose
   - Security scanning (gosec, trivy)

4. **Release automation** (`.github/workflows/release.yml`, `.goreleaser.yml`)
   - GoReleaser for multi-platform binary builds
   - Docker image push to ghcr.io
   - Changelog generation

5. **API key management**
   - Seed script for initial API key
   - CLI command for key generation
   - Key rotation support

**Success criteria:** Complete, polished OSS project ready for public launch.

---

## Acceptance Criteria

### Functional Requirements

- [ ] `docker compose up` produces working system in < 5 minutes on clean machine
- [ ] Skills can be uploaded via CLI (`skillbox skill push`) and API (`POST /v1/skills`)
- [ ] Skills execute in sandboxed Docker containers with full security hardening
- [ ] Execution returns structured JSON output from skill's output.json
- [ ] File artifacts written by skills are returned as presigned S3/MinIO URLs
- [ ] Go SDK: `client.Run()` returns structured output in < 10 lines of code
- [ ] CLI provides full skill lifecycle management (package, push, lint, run, logs)
- [ ] K8s deployment via `kubectl apply -k` works on standard cluster
- [ ] Multi-tenancy: skills and executions isolated per tenant via API key scoping
- [ ] Image allowlist prevents unauthorized Docker image execution
- [ ] gRPC API mirrors REST functionality for performance-sensitive callers

### Non-Functional Requirements

- [ ] No container can reach host network (NetworkMode: none)
- [ ] No container can access Docker daemon directly (socket proxy only)
- [ ] All containers run as non-root (user 65534:65534)
- [ ] Read-only rootfs on all skill containers
- [ ] PID limit of 128 prevents fork bombs
- [ ] Execution timeout enforced (default 120s, max 10min)
- [ ] API keys SHA-256 hashed before storage
- [ ] Redis optional — system degrades gracefully without it

### Quality Gates

- [ ] Unit test coverage > 80% for core packages (runner, registry, store)
- [ ] Integration tests for full execution flow (upload skill → run → get result)
- [ ] Security tests verifying all hardening controls
- [ ] 3 production-quality example skills with documentation
- [ ] README enables first skill run in < 5 minutes
- [ ] CI pipeline passes on all PRs
- [ ] No critical security issues (gosec clean, trivy clean)

## Dependencies & Prerequisites

| Dependency | Version | Purpose |
|---|---|---|
| Go | 1.22+ | Primary language |
| Docker Engine | 20+ | Container runtime |
| PostgreSQL | 14+ | Metadata, API keys, execution history |
| Redis | 6+ | Optional cache layer |
| MinIO | Latest | S3-compatible storage for skills and artifacts |
| Gin | v1.10+ | HTTP router |
| Cobra | v1.8+ | CLI framework |
| Docker SDK | v27+ | Container management |
| MinIO Go SDK | v7+ | S3 operations |
| gRPC-Go | v1.65+ | gRPC server |
| golang-migrate | v4+ | Database migrations |

## Risk Analysis & Mitigation

| Risk | Impact | Likelihood | Mitigation |
|---|---|---|---|
| Docker socket exposure | Critical | Medium | Socket proxy sidecar, minimal capability grants |
| Container escape | Critical | Low | All hardening controls, non-root, no-new-privileges |
| Resource exhaustion (fork bomb) | High | Medium | PID limit (128), memory limits, CPU quotas |
| Data exfiltration from skill | High | Medium | Network: none on all containers |
| Supply chain attack via image | High | Low | Image allowlist, no arbitrary image pulls |
| MinIO downtime during execution | Medium | Low | Fail execution cleanly, return error |
| Orphaned containers on crash | Medium | Medium | Startup cleanup, container labels for tracking |
| Skill zip path traversal | High | Low | Validate zip entries, reject `../` paths |

## Open Decisions (from PRD)

1. **License:** Apache-2.0 (recommended for patent protection) vs MIT
2. **Redis:** Optional at MVP — degrades gracefully (recommendation: optional)
3. **SKILL.md spec version:** v0 (breaking changes permitted) at launch, upgrade to v1 when stable
4. **Project name validation:** Check npm, PyPI, GitHub for conflicts before launch

## References

- Source PRD: `Agent Skill Execution Dokumentation.docx` (v1.0, Feb 2025)
- tecnativa/docker-socket-proxy: https://github.com/Tecnativa/docker-socket-proxy
- Docker SDK for Go: https://pkg.go.dev/github.com/docker/docker/client
- MinIO Go SDK: https://min.io/docs/minio/linux/developers/go/minio-go.html
