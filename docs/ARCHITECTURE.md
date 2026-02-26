# Architecture

## System Overview

Skillbox is a stateless API server that orchestrates sandboxed Docker container executions. It stores skill archives and file artifacts in S3-compatible storage (MinIO), execution metadata in PostgreSQL, and communicates with the Docker daemon exclusively through a socket proxy sidecar.

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

## Components

### API Server

- **Technology**: Go + Gin HTTP framework
- **Responsibility**: Route requests, authenticate, dispatch to runner
- **Scaling**: Stateless — horizontally scalable behind a load balancer
- **Ports**: 8080 (HTTP/REST), 9090 (gRPC)

### Skill Registry

- **Backend**: MinIO / S3-compatible storage
- **Storage pattern**: `{tenant}/{skill-name}/{version}/skill.zip`
- **Buckets**: `skills` (archives), `executions` (file artifacts)
- **Validation**: Zip integrity, SKILL.md presence, entrypoint existence

### Docker Runner

- **Interface**: Docker SDK for Go, connecting via socket proxy
- **Lifecycle**: Create container → Start → Wait (with timeout) → Collect output → Remove
- **Isolation**: Every container gets full security hardening (see Security Model)
- **Cleanup**: Defer chain ensures container removal even on panic

### PostgreSQL

- **Schema**: `sandbox` (isolated from other applications)
- **Tables**: `api_keys` (SHA-256 hashed), `executions` (history + results)
- **Version**: 14+

### Redis (Optional)

- **Purpose**: Skill cache, rate limits (future)
- **Degradation**: System operates without Redis — adds conditional code path

## Execution Flow

```
1. Agent sends POST /v1/executions
   { "skill": "data-analysis", "input": {"data": [...]} }

2. Auth middleware extracts Bearer token
   → SHA-256 hash → lookup in api_keys table
   → Sets tenant_id in request context

3. Handler validates request, creates execution record
   → Status: "running", stores in Postgres

4. Skill registry loads skill
   → Downloads skill.zip from MinIO
   → Extracts to temp directory
   → Parses SKILL.md, validates entrypoint

5. Runner validates image against allowlist
   → Rejects if image not in SKILLBOX_IMAGE_ALLOWLIST

6. Runner creates temp workdir
   → workdir/scripts/ (read-only: skill scripts)
   → workdir/out/ (read-write: output + files)
   → workdir/input.json (read-only: input data)

7. Docker container created via socket proxy
   → Full hardening applied (see Security Model)
   → Env vars set: SANDBOX_INPUT, SANDBOX_OUTPUT, etc.
   → Container started

8. Runner waits for container exit
   → select on context.Done() (timeout) and container exit
   → If timeout: kill container, status = "timeout"

9. Output collection
   → Read workdir/out/output.json → parse JSON → result.Output
   → If workdir/out/files/ has files:
     → tar.gz archive → upload to MinIO
     → Generate presigned URL (1 hour TTL)
     → result.FilesURL, result.FilesList

10. Cleanup
    → Force-remove container
    → Delete temp workdir
    → Update execution record in Postgres

11. Response returned to agent
    { "execution_id": "...", "status": "success",
      "output": {...}, "files_url": "...", "duration_ms": 1234 }
```

## Security Model

Security is enforced by the runtime — not configurable by callers. Every execution gets all controls:

| Control | Implementation | Threat Mitigated |
|---|---|---|
| Image allowlist | Checked before ContainerCreate | Supply-chain attack, malicious image |
| Network isolation | `NetworkMode: "none"` | Data exfiltration, SSRF |
| Capability drop | `CapDrop: ["ALL"]` | Privilege escalation |
| Read-only rootfs | `ReadonlyRootfs: true` | Filesystem tampering |
| PID limit | `PidsLimit: 128` | Fork bombs, resource exhaustion |
| No-new-privileges | `SecurityOpt: ["no-new-privileges:true"]` | setuid/setgid escalation |
| Non-root user | `User: "65534:65534"` (nobody) | Container escape techniques |
| Socket proxy | tecnativa/docker-socket-proxy | API compromise → host escape |
| tmpfs scratch | `/tmp`, `/root` as noexec tmpfs | Binary execution in writable areas |
| Timeout | Go context with deadline | Infinite loops, resource hogging |
| Memory limit | Container Resources.Memory | Memory exhaustion |
| CPU limit | Container Resources.CPUQuota | CPU exhaustion |

### Docker Socket Proxy

The API server never connects directly to the Docker socket. Instead, a sidecar runs [tecnativa/docker-socket-proxy](https://github.com/Tecnativa/docker-socket-proxy) with minimal permissions:

| Capability | Enabled | Reason |
|---|---|---|
| CONTAINERS | Yes | Create, start, stop, remove containers |
| IMAGES | Yes | Pull allowed images |
| POST | Yes | Required for container lifecycle |
| Everything else | No | Networks, volumes, exec, swarm — all disabled |

### Residual Risk

The host Linux kernel is shared with all containers. This is the fundamental limitation of Docker isolation. For genuinely untrusted third-party code, gVisor or Kata Containers can be enabled as a Kubernetes RuntimeClass with zero changes to Skillbox.

## Deployment Modes

### Docker Compose (Development)

All services in a single compose file:
- API server
- Docker socket proxy
- PostgreSQL
- Redis
- MinIO + bucket initialization

### Kubernetes (Production)

Kustomize base + overlays:
- **Namespace**: `sandbox` with Pod Security Standards
- **Deployment**: 2-container pod (API + socket proxy sidecar)
- **NetworkPolicy**: Default deny, explicit allow for API → data services
- **RBAC**: Minimal ServiceAccount, no cluster-wide permissions
