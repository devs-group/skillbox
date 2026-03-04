# Skillbox: DeerFlow Alignment — Session Workspace + Cognitive Skills

**Date**: 2026-03-04
**Status**: Planned
**Goal**: Make Skillbox work like DeerFlow's sandbox system — persistent session workspaces, cognitive skill mode, and sandbox shell tools — so VectorChat agents become autonomous programmers rather than skill dispatchers.

---

## Context

DeerFlow gives agents a persistent per-thread filesystem (`/mnt/user-data/{workspace,uploads,outputs}`) that survives across turns. Agents have `bash`, `read_file`, `write_file`, `ls` tools to freely read/write/execute. Skills are instructions the agent follows, not just executable programs.

Skillbox currently destroys sandboxes after each execution. There's no persistent workspace. Skills are atomic black-box programs. This plan adds the missing primitives so VectorChat can achieve DeerFlow-like behavior.

---

## Changes Overview

### Phase 1: Session Workspace (persistent file storage across executions)
### Phase 2: Sandbox Shell API (bash/read/write/ls via sandbox)
### Phase 3: Cognitive Skill Mode (skills as agent instructions)
### Phase 4: SDK Updates (expose new features to VectorChat)

---

## Phase 1: Session Workspace

### Problem
Every execution creates a fresh sandbox that's destroyed. The agent can't iterate on files or build upon previous work within a conversation.

### Solution
Add session-scoped file storage in MinIO. Files written to `/sandbox/out/session/` persist and are auto-mounted on subsequent executions in the same session.

### 1.1 Database: Add `sandbox.sessions` table

**File**: `internal/store/migrations/006_sessions.sql`

```sql
CREATE TABLE IF NOT EXISTS sandbox.sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id TEXT NOT NULL,
    external_id TEXT NOT NULL,  -- VectorChat session UUID
    created_at TIMESTAMPTZ DEFAULT NOW(),
    last_accessed_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(tenant_id, external_id)
);

CREATE INDEX idx_sessions_tenant ON sandbox.sessions(tenant_id);
CREATE INDEX idx_sessions_external ON sandbox.sessions(tenant_id, external_id);
```

### 1.2 Store: Add session + session file queries

**File**: `internal/store/sessions.go` (NEW)

```go
// GetOrCreateSession finds or creates a session by external ID
func (s *Store) GetOrCreateSession(ctx context.Context, tenantID, externalID string) (*Session, error)

// TouchSession updates last_accessed_at
func (s *Store) TouchSession(ctx context.Context, sessionID string) error

// ListSessionFiles returns files linked to a session (via session_id on sandbox.files)
func (s *Store) ListSessionFiles(ctx context.Context, tenantID, sessionID string) ([]*File, error)
```

### 1.3 Runner: Mount session files + persist session outputs

**File**: `internal/runner/runner.go` — modify `Run()` method

Changes to `RunRequest`:
```go
type RunRequest struct {
    // ... existing fields ...
    SessionID  string `json:"session_id,omitempty"` // NEW: session external ID
}
```

In `Run()`, after sandbox creation and before execution:
1. If `req.SessionID != ""`:
   - Call `store.GetOrCreateSession(tenantID, req.SessionID)`
   - Call `store.ListSessionFiles(tenantID, dbSession.ID)` to get prior files
   - Download each session file from MinIO and upload to sandbox at `/sandbox/session/{filename}`
2. Add env var: `SANDBOX_SESSION_DIR=/sandbox/session/`
3. Create placeholder: `/sandbox/out/session/.keep`

After execution, before cleanup:
1. Search for files in `/sandbox/out/session/`
2. For each file found:
   - Upload to MinIO at `{tenantID}/sessions/{dbSessionID}/{filename}`
   - Upsert file record with `session_id = dbSessionID` (overwrite if name exists)
3. Call `store.TouchSession(dbSessionID)`

### 1.4 API: Session file endpoints

**File**: `internal/api/handler_sessions.go` (NEW)

```
GET  /v1/sessions/:external_id/files           → List files in session workspace
GET  /v1/sessions/:external_id/files/:filename  → Download specific session file
DELETE /v1/sessions/:external_id/files/:filename → Remove session file
DELETE /v1/sessions/:external_id                 → Delete session and all files
```

**File**: `internal/api/router.go` — register new routes under v1 group

### 1.5 MinIO Key Pattern

```
Session files:  {tenantID}/sessions/{dbSessionID}/{filename}
```

Use the existing `S3BucketExecs` bucket (or add a new `S3BucketSessions` config field — prefer reusing existing bucket with prefix).

---

## Phase 2: Sandbox Shell API

### Problem
VectorChat's agent needs `bash`, `read_file`, `write_file`, `ls` tools that execute in an isolated sandbox with session workspace mounted. Currently agents can only dispatch entire skill executions.

### Solution
Add a `/v1/sandbox` endpoint group that manages long-lived sandbox sessions. The sandbox persists for the session's lifetime (or a configurable TTL), enabling multi-turn command execution.

### 2.1 Sandbox Session Manager

**File**: `internal/sandbox/session_manager.go` (NEW)

```go
type SessionManager struct {
    client    *Client
    store     *store.Store
    artifacts *artifacts.Collector
    config    *config.Config

    mu       sync.Mutex
    sessions map[string]*ManagedSandbox // keyed by "{tenantID}:{sessionExternalID}"
}

type ManagedSandbox struct {
    SandboxID  string
    ExecDURL   string
    SessionID  string    // DB session ID
    TenantID   string
    ExternalID string    // VectorChat session UUID
    CreatedAt  time.Time
    LastUsedAt time.Time
    Image      string
}

// GetOrCreate returns existing sandbox or creates one with session files mounted
func (m *SessionManager) GetOrCreate(ctx context.Context, tenantID, externalID string, opts SandboxSessionOpts) (*ManagedSandbox, error)

// Execute runs a command in the session's sandbox
func (m *SessionManager) Execute(ctx context.Context, key string, command, workdir string, timeout int) (*CommandResult, error)

// ReadFile reads file content from sandbox
func (m *SessionManager) ReadFile(ctx context.Context, key string, path string) ([]byte, error)

// WriteFile writes content to sandbox
func (m *SessionManager) WriteFile(ctx context.Context, key string, path, content string) error

// ListDir lists directory contents in sandbox
func (m *SessionManager) ListDir(ctx context.Context, key string, path string, maxDepth int) ([]FileInfo, error)

// SyncSessionFiles persists /sandbox/out/session/ files back to MinIO
func (m *SessionManager) SyncSessionFiles(ctx context.Context, key string) error

// Cleanup destroys idle sandboxes (called by background goroutine)
func (m *SessionManager) Cleanup(ctx context.Context, maxIdle time.Duration)

// Shutdown destroys all managed sandboxes
func (m *SessionManager) Shutdown(ctx context.Context)
```

`SandboxSessionOpts`:
```go
type SandboxSessionOpts struct {
    Image   string // default: python:3.12-slim
    Memory  string // default: config.DefaultMemory
    CPU     string // default: config.DefaultCPU
    Timeout int    // sandbox TTL in seconds
}
```

### 2.2 API: Sandbox shell endpoints

**File**: `internal/api/handler_sandbox.go` (NEW)

```
POST /v1/sandbox/execute     → Execute bash command in session sandbox
POST /v1/sandbox/read-file   → Read file from session sandbox
POST /v1/sandbox/write-file  → Write file to session sandbox
POST /v1/sandbox/list-dir    → List directory in session sandbox
POST /v1/sandbox/sync        → Persist session files to MinIO
DELETE /v1/sandbox/:session  → Destroy session sandbox
```

All endpoints require `X-Session-ID` header (maps to VectorChat session UUID).

Request/Response:

```go
// POST /v1/sandbox/execute
type SandboxExecRequest struct {
    Command    string `json:"command"`
    WorkDir    string `json:"workdir,omitempty"`  // default: /sandbox/session
    TimeoutMs  int    `json:"timeout_ms,omitempty"` // default: 30000
}
type SandboxExecResponse struct {
    Stdout   string `json:"stdout"`
    Stderr   string `json:"stderr"`
    ExitCode int    `json:"exit_code"`
}

// POST /v1/sandbox/read-file
type SandboxReadRequest struct {
    Path      string `json:"path"`
    StartLine *int   `json:"start_line,omitempty"`
    EndLine   *int   `json:"end_line,omitempty"`
}
type SandboxReadResponse struct {
    Content string `json:"content"`
    Size    int64  `json:"size"`
}

// POST /v1/sandbox/write-file
type SandboxWriteRequest struct {
    Path    string `json:"path"`
    Content string `json:"content"`
    Append  bool   `json:"append,omitempty"`
}

// POST /v1/sandbox/list-dir
type SandboxListRequest struct {
    Path     string `json:"path"`
    MaxDepth int    `json:"max_depth,omitempty"` // default: 2
}
type SandboxListResponse struct {
    Entries []SandboxDirEntry `json:"entries"`
}
type SandboxDirEntry struct {
    Path  string `json:"path"`
    IsDir bool   `json:"is_dir"`
    Size  int64  `json:"size"`
}
```

### 2.3 Path Security

All sandbox shell endpoints validate paths:
- Must be absolute paths starting with `/sandbox/`
- No `..` components allowed
- Allowed prefixes: `/sandbox/session/`, `/sandbox/scripts/`, `/sandbox/input/`, `/sandbox/out/`
- `/sandbox/session/` is read-write (workspace)
- `/sandbox/scripts/` is read-only (skill code)
- `/sandbox/input/` is read-only (input files)
- `/sandbox/out/` is write-only (outputs)

**File**: `internal/sandbox/path_validator.go` (NEW)

```go
func ValidateSandboxPath(path string, mode PathMode) error
// PathMode: Read, Write, ReadWrite
// Rejects: "..", relative paths, paths outside /sandbox/
```

### 2.4 Config additions

**File**: `internal/config/config.go`

```go
// New fields:
SandboxSessionTTL    time.Duration // default: 30m (idle TTL for session sandboxes)
SandboxSessionImage  string        // default: "python:3.12-slim"
MaxSessionSandboxes  int           // default: 20 (per server instance)
```

### 2.5 Main wiring

**File**: `cmd/skillbox-server/main.go`

- Create `SessionManager` after sandbox client init
- Register sandbox routes
- Start background cleanup goroutine (every 5 minutes, kill sandboxes idle > TTL)
- Add `SessionManager.Shutdown()` to graceful shutdown

---

## Phase 3: Cognitive Skill Mode

### Problem
All Skillbox skills are executable programs. DeerFlow skills are agent instructions. We need a mode where the SKILL.md body serves as instructions for the agent, and the skill's Python/JS files are libraries the agent can import.

### Solution
Add `mode` field to SKILL.md. When `mode: cognitive`, the agent reads instructions and uses sandbox tools to execute code using the skill's library files.

### 3.1 Skill model update

**File**: `internal/skill/skill.go`

Add `Mode` field:
```go
type Skill struct {
    // ... existing fields ...
    Mode string // "executable" (default) or "cognitive"
}

// In ParseSkillMD():
// Parse mode from frontmatter, default to "executable"
```

SKILL.md format:
```yaml
---
name: data-analysis
version: 1.0.0
description: Analyze datasets with SQL and Python
mode: cognitive
lang: python
---
# Data Analysis Skill

## Instructions
When the user wants to analyze data:
1. Read uploaded files from /sandbox/session/
2. Use pandas/duckdb to query data
3. Write results to /sandbox/out/session/

## Available Utilities
- `core/loader.py` — load CSV/Excel/JSON files
- `core/query.py` — run SQL queries via DuckDB
```

### 3.2 API: Expose mode in skill detail

**File**: `internal/api/handler_skills.go`

Add `mode` to skill detail response:
```go
type SkillDetailResponse struct {
    // ... existing fields ...
    Mode string `json:"mode"` // "executable" or "cognitive"
}
```

### 3.3 Runner: Handle cognitive skills

When `mode == "cognitive"`, the runner should NOT auto-execute. Instead:
- The skill files are made available at `/sandbox/scripts/` in the session sandbox
- The `RunRequest` with a cognitive skill just mounts the skill files — execution happens via sandbox shell API calls from VectorChat

Add to `RunRequest`:
```go
type RunRequest struct {
    // ... existing fields ...
    MountOnly bool `json:"mount_only,omitempty"` // NEW: mount skill files without executing
}
```

When `MountOnly == true`:
1. Create execution record
2. Load skill from registry
3. Upload skill files to sandbox at `/sandbox/scripts/`
4. Do NOT execute any command
5. Return immediately with status "mounted"
6. Sandbox stays alive (managed by SessionManager)

### 3.4 SDK update for mode

**File**: `sdks/go/skillbox.go`

Add `Mode` to `SkillDetail`:
```go
type SkillDetail struct {
    // ... existing fields ...
    Mode string `json:"mode"` // "executable" or "cognitive"
}
```

---

## Phase 4: SDK Updates

### 4.1 New SDK methods

**File**: `sdks/go/skillbox.go`

```go
// Session workspace
func (c *Client) ListSessionFiles(ctx context.Context, sessionID string) ([]FileInfo, error)
func (c *Client) GetSessionFile(ctx context.Context, sessionID, filename string) (io.ReadCloser, error)
func (c *Client) DeleteSessionFile(ctx context.Context, sessionID, filename string) error

// Sandbox shell
func (c *Client) SandboxExecute(ctx context.Context, sessionID string, req SandboxExecRequest) (*SandboxExecResponse, error)
func (c *Client) SandboxReadFile(ctx context.Context, sessionID string, path string) (string, error)
func (c *Client) SandboxWriteFile(ctx context.Context, sessionID string, path, content string, append bool) error
func (c *Client) SandboxListDir(ctx context.Context, sessionID string, path string, maxDepth int) ([]SandboxDirEntry, error)
func (c *Client) SandboxSync(ctx context.Context, sessionID string) error
func (c *Client) SandboxDestroy(ctx context.Context, sessionID string) error
```

### 4.2 RunRequest update

```go
type RunRequest struct {
    // ... existing fields ...
    SessionID string `json:"session_id,omitempty"`
    MountOnly bool   `json:"mount_only,omitempty"`
}
```

---

## File Change Summary

### New Files
| File | Purpose |
|------|---------|
| `internal/store/migrations/006_sessions.sql` | Sessions table |
| `internal/store/sessions.go` | Session DB queries |
| `internal/sandbox/session_manager.go` | Long-lived sandbox management |
| `internal/sandbox/path_validator.go` | Path security validation |
| `internal/api/handler_sessions.go` | Session file API endpoints |
| `internal/api/handler_sandbox.go` | Sandbox shell API endpoints |

### Modified Files
| File | Changes |
|------|---------|
| `internal/runner/runner.go` | Add SessionID to RunRequest, mount/persist session files |
| `internal/skill/skill.go` | Add Mode field, parse from frontmatter |
| `internal/config/config.go` | Add session sandbox config fields |
| `internal/api/router.go` | Register new route groups |
| `internal/api/handler_skills.go` | Expose mode in detail response |
| `internal/api/handler_executions.go` | Support mount_only mode |
| `cmd/skillbox-server/main.go` | Wire SessionManager, background cleanup |
| `sdks/go/skillbox.go` | Add session, sandbox, and mode SDK methods |

---

## Verification

### Unit Tests
- `internal/store/sessions_test.go` — session CRUD
- `internal/sandbox/session_manager_test.go` — sandbox lifecycle
- `internal/sandbox/path_validator_test.go` — path traversal prevention
- `internal/skill/skill_test.go` — extend for mode parsing
- `internal/runner/runner_test.go` — extend for session file mounting

### Integration Tests
1. **Session persistence**: Execute skill A in session S, verify files persist, execute skill B in same session S, verify skill B can read skill A's outputs
2. **Sandbox shell**: Create sandbox for session, execute bash command, read/write files, verify persistence after sync
3. **Cognitive skill**: Upload skill with `mode: cognitive`, call with `mount_only: true`, use sandbox shell to execute code from skill scripts
4. **Path security**: Verify `../` traversal blocked, verify read-only paths can't be written

### Manual E2E Test
1. Start Skillbox with OpenSandbox
2. Upload a data-analysis skill with `mode: cognitive`
3. Call `POST /v1/sandbox/execute` with `X-Session-ID: test-session` to run Python code
4. Call `POST /v1/sandbox/read-file` to inspect results
5. Call `POST /v1/sandbox/sync` to persist workspace files
6. Call `GET /v1/sessions/test-session/files` to verify persistence
7. Call `POST /v1/sandbox/execute` again — verify prior workspace files still available

---

## Dependencies

- OpenSandbox must support long-lived sandboxes (current TTL is configurable, max 86400s)
- MinIO must be available for session file storage
- No new Go dependencies required (all existing: minio-go, pgx, gin)

## Parallel Execution Notes

This plan can be executed by 4 parallel agents:
- **Agent 1**: Phase 1 (store/migrations + session files)
- **Agent 2**: Phase 2 (sandbox session manager + shell API)
- **Agent 3**: Phase 3 (cognitive skill mode)
- **Agent 4**: Phase 4 (SDK updates)

Dependency: Agent 2 depends on Agent 1 (sessions table). Agent 3 is independent. Agent 4 depends on all others for types.
