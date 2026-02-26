# Skillbox

**The self-hosted execution runtime for AI agents.**

> Your agents need a sandbox. Don't build one.

Skillbox gives AI agents a single API to run sandboxed skill scripts (Python, Node.js, Bash) and receive structured JSON output + file artifacts. Self-hosted, open source, secure by default.

```python
from skillbox import Client

client = Client("http://localhost:8080", "sk-your-key")

# Discover what skills are available
for skill in client.list_skills():
    print(f"{skill.name}: {skill.description}")

# Run a skill — structured in, structured out
result = client.run("data-analysis", input={"data": [1, 2, 3, 4, 5]})
print(result.output)  # {"row_count": 5, "mean": 3.0, ...}
```

## Why Skillbox

**Every AI agent that does useful work needs to execute code.** But executing arbitrary code is dangerous. Most teams either skip sandboxing ("we'll fix it later") or build their own broken wrapper. Skillbox is the missing piece:

| Problem | How Skillbox Solves It |
|---|---|
| **"We need sandboxing but E2B/Modal are cloud-only"** | Self-hosted. Your infrastructure, your data, your rules. |
| **"Three teams built three sandbox wrappers"** | One runtime. One API. One security review. |
| **"Our agents don't know what tools are available"** | Skill catalog — agents discover, inspect, and choose capabilities. |
| **"We need GDPR/EU AI Act compliance"** | Data never leaves your network. MIT license. |
| **"Docker is insecure for running untrusted code"** | 11 layers of hardening, enforced by the runtime, not configurable by callers. |

### Compared to Alternatives

| | Skillbox | E2B | Modal | Daytona |
|---|---|---|---|---|
| Self-hosted | **Yes (MIT)** | Experimental | No | Limited |
| Skill catalog | **Yes (SKILL.md)** | No | No | No |
| Structured I/O | **JSON in → JSON out** | Raw stdout | Raw stdout | Raw stdout |
| Agent introspection | **list + get_skill** | No | No | No |
| LangChain-native | **1:1 tool mapping** | Manual | Manual | Manual |
| Network disabled | **Always** | Optional | No | No |
| Zero-dep SDK | **Go + Python** | Python | Python | REST only |
| License | **MIT** | Apache-2.0 | Proprietary | Apache-2.0 |

## Features

- **Secure by default** — Network disabled, all capabilities dropped, read-only rootfs, PID limits, non-root user, no-new-privileges, image allowlist, socket proxy. 11 layers. Not optional.
- **Skill catalog** — Skills are versioned, discoverable, introspectable units with YAML metadata + markdown instructions. Agents understand what's available before executing.
- **Structured I/O** — Skills read JSON input, write JSON output, and produce file artifacts. No stdout parsing.
- **LangChain-ready** — Skills map 1:1 to LangChain tools. `get_skill` returns descriptions for tool selection.
- **Self-hosted** — Docker Compose (dev), Kubernetes (prod), Helm chart. Air-gapped? Works offline.
- **Multi-tenant** — API keys scoped to tenants, skills and executions isolated.
- **Zero-dep SDKs** — Go and Python clients use only the standard library. No dependency conflicts.
- **CLI** — Push, lint, run, package, and manage skills from the terminal.
- **File artifacts** — Skills write files, runtime tars them, presigned S3 URL returned.
- **12-factor config** — All configuration via environment variables.

## Quick Start

**Prerequisites:** Docker and Docker Compose.

```bash
# 1. Start the stack
git clone https://github.com/devs-group/skillbox.git && cd skillbox
docker compose -f deploy/docker/docker-compose.yml up -d

# 2. Create an API key
bash scripts/seed-apikey.sh
export SKILLBOX_API_KEY=sk-...  # from the script output

# 3. Install the CLI and push a skill
go install github.com/devs-group/skillbox/cmd/skillbox@latest
skillbox skill push examples/skills/data-analysis --server http://localhost:8080

# 4. Run it
skillbox run data-analysis --input '{"data": [{"name": "Alice", "age": 30}, {"name": "Bob", "age": 25}]}'
```

Or with curl:

```bash
curl -s http://localhost:8080/v1/executions \
  -H "Authorization: Bearer $SKILLBOX_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"skill": "data-analysis", "input": {"data": [{"name": "Alice", "age": 30}]}}' | jq .
```

## Security Model

Security is enforced by the runtime — **not configurable away by callers**:

| Control | Implementation | Threat Mitigated |
|---|---|---|
| Network isolation | `NetworkMode: none` | Data exfiltration, SSRF |
| Capability drop | `CapDrop: ["ALL"]` | Privilege escalation |
| Read-only rootfs | `ReadonlyRootfs: true` | Filesystem tampering |
| PID limit | `PidsLimit: 128` | Fork bombs |
| No-new-privileges | `no-new-privileges:true` | setuid/setgid escalation |
| Non-root user | `User: 65534:65534` | Container escape |
| Socket proxy | tecnativa/docker-socket-proxy | Host escape via Docker socket |
| Image allowlist | Checked before ContainerCreate | Supply-chain attack |
| Timeout | Go context cancellation | Resource exhaustion |
| tmpfs noexec | `/tmp`, `/root` as noexec | Binary execution in writable areas |
| Env var blocking | `LD_PRELOAD`, `PYTHONPATH` blocked | Library injection |

For genuinely untrusted code, gVisor or Kata Containers can be enabled as a Kubernetes RuntimeClass with zero changes to Skillbox.

## Skill Format

A skill is a zip archive containing `SKILL.md` + scripts:

```
my-skill/
├── SKILL.md              # YAML frontmatter + instructions
├── scripts/
│   └── main.py           # Entrypoint
└── requirements.txt      # Optional: Python deps
```

```yaml
---
name: data-analysis
version: "1.0.0"
description: Analyze CSV data and produce summary statistics
lang: python
timeout: 60s
resources:
  memory: 256Mi
  cpu: "0.5"
---

# Data Analysis Skill

Analyze data and produce summary statistics with charts.
```

The YAML frontmatter is machine-readable (for SDKs and API). The markdown body is LLM-readable (for agent tool selection). This dual format is what makes Skillbox skills work as LangChain tools out of the box.

See [docs/SKILL-SPEC.md](docs/SKILL-SPEC.md) for the full specification.

## SDKs

### Go

Single file, zero dependencies beyond the Go standard library:

```bash
go get github.com/devs-group/skillbox/sdks/go
```

```go
import skillbox "github.com/devs-group/skillbox/sdks/go"

client := skillbox.New("http://localhost:8080", "sk-your-key",
    skillbox.WithTenant("my-team"),
)

result, err := client.Run(ctx, skillbox.RunRequest{
    Skill:   "text-summary",
    Input:   json.RawMessage(`{"text": "Long text here...", "max_sentences": 3}`),
})

if result.HasFiles() {
    err = client.DownloadFiles(ctx, result, "./output")
}
```

### Python

Single file, zero dependencies beyond the Python standard library:

```python
from skillbox import Client

client = Client("http://localhost:8080", "sk-your-key", tenant_id="my-team")

result = client.run("text-summary", input={"text": "Long text here...", "max_sentences": 3})
print(result.output)  # {"summary": "...", "sentence_count": 2}

if result.has_files:
    client.download_files(result, "./output")
```

## LangChain Integration

Skillbox skills map directly to LangChain tools. Each skill becomes a callable tool that an agent can discover, inspect, and execute:

```python
from langchain_anthropic import ChatAnthropic
from langgraph.prebuilt import create_react_agent

# Build tools from all registered skills
tools = build_skillbox_toolkit("http://localhost:8080", "sk-your-key")

# Agent sees tools like skillbox_data_analysis, reads their descriptions,
# picks the right one, calls it with structured input, gets structured output
agent = create_react_agent(ChatAnthropic(model="claude-sonnet-4-6"), tools)
result = agent.invoke({
    "messages": [{"role": "user", "content": "Analyze this data: name,age\nAlice,30\nBob,25"}]
})
```

See the [full LangChain integration guide](docs/API.md) for `SkillboxTool`, `SkillboxToolkit`, and custom tool examples.

## Architecture

```
Agent → REST API → Skill Registry (MinIO) → Docker Runner → Container (sandboxed) → Output + Files
              ↕                                     ↕
         PostgreSQL                        Socket Proxy → Docker Daemon
```

Every execution: authenticate → load skill → validate image → create hardened container → run → collect output + files → cleanup. Stateless API, horizontally scalable behind a load balancer.

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for the full deep-dive.

## CLI

```bash
skillbox run <skill> [--input '{}'] [--version latest]
skillbox skill push <dir|zip>
skillbox skill list
skillbox skill lint <dir>
skillbox skill package <dir>
skillbox exec logs <id>
skillbox health
skillbox version
```

## Deployment

### Docker Compose (Development)

```bash
docker compose -f deploy/docker/docker-compose.yml up
```

### Kubernetes (Production)

```bash
kubectl apply -k deploy/k8s/overlays/prod
```

### Helm

```bash
helm install skillbox deploy/helm/skillbox/
```

Kustomize overlays for dev and prod environments. Includes namespace, RBAC, NetworkPolicy, and Pod Security Standards.

## API

| Method | Path | Description |
|---|---|---|
| POST | /v1/executions | Run a skill |
| GET | /v1/executions/:id | Get execution result |
| GET | /v1/executions/:id/logs | Get execution logs |
| POST | /v1/skills | Upload a skill zip |
| GET | /v1/skills | List skills (with descriptions) |
| GET | /v1/skills/:name/:version | Get skill metadata + instructions |
| DELETE | /v1/skills/:name/:version | Delete a skill |
| GET | /health | Liveness probe |
| GET | /ready | Readiness probe |

See [docs/API.md](docs/API.md) for the full reference.

## Examples

| Example | Description |
|---|---|
| [examples/skills/data-analysis/](examples/skills/data-analysis/) | CSV/JSON statistics with chart artifacts |
| [examples/skills/text-summary/](examples/skills/text-summary/) | Extractive text summarization |
| [examples/skills/word-counter/](examples/skills/word-counter/) | Word frequency counting |
| [examples/curl/](examples/curl/) | Step-by-step curl + jq walkthrough |
| [examples/python/](examples/python/) | Python integration (stdlib only) |
| [examples/agent-integration/](examples/agent-integration/) | Full Go agent using the SDK |
| [examples/write-your-first-skill/](examples/write-your-first-skill/) | Build your first skill (tutorial) |

Run all examples at once:

```bash
docker compose -f examples/docker-compose.yml up
```

## Configuration

All configuration via environment variables (12-factor):

| Variable | Default | Description |
|---|---|---|
| `SKILLBOX_DB_DSN` | *required* | PostgreSQL connection string |
| `SKILLBOX_S3_ENDPOINT` | *required* | MinIO/S3 endpoint |
| `SKILLBOX_S3_ACCESS_KEY` | *required* | S3 access key |
| `SKILLBOX_S3_SECRET_KEY` | *required* | S3 secret key |
| `SKILLBOX_DOCKER_HOST` | tcp://localhost:2375 | Docker socket proxy address |
| `SKILLBOX_IMAGE_ALLOWLIST` | python:3.12-slim,... | Allowed Docker images |
| `SKILLBOX_DEFAULT_TIMEOUT` | 120s | Default execution timeout |
| `SKILLBOX_API_PORT` | 8080 | HTTP port |
| `SKILLBOX_REDIS_URL` | *(optional)* | Redis URL for caching |

## Contributing

We welcome contributions! See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, coding guidelines, and how to add new skills.

## License

MIT. See [LICENSE](LICENSE).

---

Built and maintained by [devs group](https://devs-group.com) · Kreuzlingen, Switzerland
