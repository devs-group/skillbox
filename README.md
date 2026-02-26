# Skillbox

**Secure skill execution runtime for AI agents.**

Skillbox gives AI agents a single clean API to run sandboxed skill scripts (Python, Node.js, Bash) and receive structured output plus file artifacts in return. It deploys identically on Docker Compose for local development and Kubernetes for production.

```go
client := skillbox.New("http://localhost:8080", os.Getenv("SKILLBOX_API_KEY"))

result, err := client.Run(ctx, skillbox.RunRequest{
    Skill: "data-analysis",
    Input: json.RawMessage(`{"data": [1, 2, 3, 4, 5]}`),
})

fmt.Println(result.Status)       // "success"
fmt.Println(string(result.Output)) // {"row_count": 5, ...}
```

## Features

- **Secure by default** — Every container runs with: network disabled, all capabilities dropped, PID limits, non-root user, no-new-privileges
- **Docker socket proxy** — API never touches the Docker daemon directly
- **Structured I/O** — Skills read JSON input, write JSON output, and produce file artifacts
- **File artifacts** — Skills write files → runtime tars them → presigned S3 URL returned
- **Go SDK** — Single-file, stdlib-only, idiomatic Go client
- **Python SDK** — Single-file, stdlib-only Python client
- **LangChain-ready** — Skills map 1:1 to LangChain tools with full introspection via `get_skill`
- **CLI tool** — Package, push, lint, run, and manage skills from the command line
- **Self-hosted** — Runs on Docker Compose (dev) or Kubernetes (prod)
- **Multi-tenant** — API keys scoped to tenants, skills and executions isolated
- **Image allowlist** — Only pre-approved Docker images can execute

## Quick Start

**Prerequisites:** Docker and Docker Compose installed.

### 1. Start the stack

```bash
git clone https://github.com/devs-group/skillbox.git
cd skillbox
docker compose -f deploy/docker/docker-compose.yml up -d
```

This starts: API server, PostgreSQL, Redis, MinIO, and Docker socket proxy.

### 2. Create an API key

```bash
bash scripts/seed-apikey.sh
export SKILLBOX_API_KEY=sk-...  # from the script output
```

### 3. Push an example skill

```bash
# Install the CLI
go install github.com/devs-group/skillbox/cmd/skillbox@latest

# Push the data-analysis skill
skillbox skill push examples/skills/data-analysis --server http://localhost:8080
```

### 4. Run it

```bash
skillbox run data-analysis --input '{"data": [{"name": "Alice", "age": 30}, {"name": "Bob", "age": 25}]}'
```

Or with curl:

```bash
curl -s http://localhost:8080/v1/executions \
  -H "Authorization: Bearer $SKILLBOX_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"skill": "data-analysis", "input": {"data": [{"name": "Alice", "age": 30}]}}' | jq .
```

## Architecture

```
Agent → REST/gRPC API → Skill Registry (MinIO) → Docker Runner → Container (sandboxed) → Output + Files
                ↕                                       ↕
           PostgreSQL                          Socket Proxy → Docker Daemon
```

Every execution:
1. API authenticates request via Bearer token (SHA-256 hashed in Postgres)
2. Skill loaded from MinIO (versioned zip: SKILL.md + scripts)
3. Image validated against allowlist
4. Container created with full security hardening
5. Skill reads `$SANDBOX_INPUT`, writes `output.json` + files to `/sandbox/out/`
6. Output and file artifacts returned to caller
7. Container force-removed and temp directory cleaned up

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for details.

## Security Model

Security is enforced by the runtime — not configurable away by callers:

| Control | Implementation | Threat Mitigated |
|---|---|---|
| Network isolation | `NetworkMode: none` | Data exfiltration, SSRF |
| Capability drop | `CapDrop: ["ALL"]` | Privilege escalation |
| PID limit | `PidsLimit: 128` | Fork bombs |
| No-new-privileges | `no-new-privileges:true` | setuid/setgid escalation |
| Non-root user | `User: 65534:65534` | Container escape |
| Socket proxy | tecnativa/docker-socket-proxy | Host escape via Docker socket |
| Image allowlist | Checked before ContainerCreate | Supply-chain attack |
| Timeout | Go context cancellation | Resource exhaustion |

## Skill Format

A skill is a zip archive containing `SKILL.md` + scripts:

```
my-skill/
├── SKILL.md              # YAML frontmatter + instructions
├── scripts/
│   └── main.py           # Entrypoint
└── requirements.txt      # Optional: Python deps
```

**SKILL.md example:**

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

See [docs/SKILL-SPEC.md](docs/SKILL-SPEC.md) for the full specification.

## Go SDK

The SDK is a single file with zero dependencies beyond the Go standard library:

```bash
go get github.com/devs-group/skillbox/sdks/go
```

```go
import skillbox "github.com/devs-group/skillbox/sdks/go"

client := skillbox.New("http://localhost:8080", "sk-your-key",
    skillbox.WithTenant("my-team"),
)

// Run a skill
result, err := client.Run(ctx, skillbox.RunRequest{
    Skill:   "text-summary",
    Input:   json.RawMessage(`{"text": "Long text here...", "max_sentences": 3}`),
})

// Download file artifacts
if result.HasFiles() {
    err = client.DownloadFiles(ctx, result, "./output")
}
```

## Python SDK

The SDK is a single file with zero dependencies beyond the Python standard library:

```bash
# Copy into your project
curl -O https://raw.githubusercontent.com/devs-group/skillbox/main/sdks/python/skillbox.py
```

```python
from skillbox import Client

client = Client("http://localhost:8080", "sk-your-key", tenant_id="my-team")

# Run a skill
result = client.run("text-summary", input={"text": "Long text here...", "max_sentences": 3})

print(result.status)  # "success"
print(result.output)  # {"summary": "...", "sentence_count": 2}

# Download file artifacts
if result.has_files:
    client.download_files(result, "./output")
```

## LangChain Integration

Skillbox skills map directly to LangChain tools. Each skill becomes a callable tool that an agent can discover, inspect, and execute.

### Skill Discovery + Execution

```python
from skillbox import Client

client = Client("http://localhost:8080", "sk-your-key")

# List skills WITH descriptions — agent can decide which to use
for skill in client.list_skills():
    print(f"{skill.name}: {skill.description}")

# Load full instructions before executing (like Claude Code's Skill tool)
detail = client.get_skill("data-analysis", "1.0.0")
print(detail.instructions)  # Full SKILL.md body

# Execute
result = client.run("data-analysis", input={"data": [1, 2, 3]})
```

### Custom LangChain Tool

```python
from langchain_core.tools import BaseTool
from pydantic import BaseModel, Field
from skillbox import Client, APIError


class SkillboxInput(BaseModel):
    input: dict = Field(default_factory=dict, description="JSON input for the skill")


class SkillboxTool(BaseTool):
    name: str
    description: str
    args_schema: type[BaseModel] = SkillboxInput
    client: Client
    skill_name: str
    skill_version: str = ""

    def _run(self, input: dict = {}) -> str:
        result = self.client.run(
            self.skill_name,
            version=self.skill_version,
            input=input,
        )
        if result.error:
            return f"Error: {result.error}"
        return json.dumps(result.output, indent=2)
```

### Build a Toolkit from All Registered Skills

```python
import json
from skillbox import Client


def build_skillbox_toolkit(base_url: str, api_key: str) -> list[BaseTool]:
    client = Client(base_url, api_key)
    tools = []

    for skill in client.list_skills():
        # Fetch full instructions for the tool description
        detail = client.get_skill(skill.name, skill.version)

        tools.append(SkillboxTool(
            name=f"skillbox_{skill.name.replace('-', '_')}",
            description=f"{skill.description}\n\n{detail.instructions}",
            client=client,
            skill_name=skill.name,
            skill_version=skill.version,
        ))

    return tools
```

### Wire into a LangChain Agent

```python
from langchain_anthropic import ChatAnthropic
from langgraph.prebuilt import create_react_agent

tools = build_skillbox_toolkit("http://localhost:8080", "sk-your-key")
agent = create_react_agent(ChatAnthropic(model="claude-sonnet-4-6"), tools)

result = agent.invoke({
    "messages": [{"role": "user", "content": "Analyze this data: name,age\nAlice,30\nBob,25"}]
})
```

The agent sees tools like `skillbox_data_analysis` and `skillbox_text_summary`, reads their descriptions (pulled from SKILL.md instructions), picks the right one, calls it with structured input, and gets structured output back.

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

## Examples

### Run with curl

```bash
# Upload a skill
cd examples/skills/data-analysis
zip -r /tmp/data-analysis.zip SKILL.md scripts/ requirements.txt
curl -X POST http://localhost:8080/v1/skills \
  -H "Authorization: Bearer $SKILLBOX_API_KEY" \
  -F "file=@/tmp/data-analysis.zip" -F "name=data-analysis" -F "version=1.0.0"

# Execute it
curl -s -X POST http://localhost:8080/v1/executions \
  -H "Authorization: Bearer $SKILLBOX_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "skill": "data-analysis",
    "input": {
      "data": [
        {"name": "Alice", "age": 30, "score": 85.5},
        {"name": "Bob", "age": 25, "score": 92.0}
      ]
    }
  }' | jq .
```

### Run with the Go SDK

```go
client := skillbox.New("http://localhost:8080", os.Getenv("SKILLBOX_API_KEY"))

result, _ := client.Run(ctx, skillbox.RunRequest{
    Skill: "data-analysis",
    Input: json.RawMessage(`{
        "data": [{"name": "Alice", "age": 30}, {"name": "Bob", "age": 25}]
    }`),
})

fmt.Println(result.Status)         // "success"
fmt.Println(string(result.Output)) // {"row_count": 2, "columns": {...}}

if result.HasFiles() {
    client.DownloadFiles(ctx, result, "./output")
}
```

### Run with the Python SDK

```python
from skillbox import Client

client = Client("http://localhost:8080", os.environ["SKILLBOX_API_KEY"])

result = client.run("data-analysis", input={
    "data": [{"name": "Alice", "age": 30}, {"name": "Bob", "age": 25}]
})

print(result.status)   # "success"
print(result.output)   # {"row_count": 2, "columns": {...}}

if result.has_files:
    client.download_files(result, "./output")
```

### Run all examples with Docker Compose

```bash
docker compose -f examples/docker-compose.yml up
```

This starts the full stack, seeds an API key, uploads example skills, and runs them — see the test runner logs for results.

### Write your own skill

See [examples/write-your-first-skill/](examples/write-your-first-skill/) for a step-by-step tutorial.

### More examples

| Example | Description |
|---|---|
| [examples/curl/](examples/curl/) | Step-by-step curl + jq walkthrough |
| [examples/python/](examples/python/) | Python integration (stdlib only) |
| [examples/agent-integration/](examples/agent-integration/) | Full Go agent using the SDK |
| [examples/skills/data-analysis/](examples/skills/data-analysis/) | CSV/JSON statistics with chart artifacts |
| [examples/skills/text-summary/](examples/skills/text-summary/) | Extractive text summarization |
| [examples/skills/word-counter/](examples/skills/word-counter/) | Word frequency counting |

## Deployment

### Docker Compose (Development)

```bash
docker compose -f deploy/docker/docker-compose.yml up
```

### Kubernetes (Production)

```bash
kubectl apply -k deploy/k8s/overlays/prod
```

Kustomize overlays for dev and prod environments. Includes namespace, RBAC, NetworkPolicy, and Pod Security Standards.

## API

| Method | Path | Description |
|---|---|---|
| POST | /v1/executions | Run a skill synchronously |
| GET | /v1/executions/:id | Get execution result |
| GET | /v1/executions/:id/logs | Get execution logs |
| POST | /v1/skills | Upload a skill zip |
| GET | /v1/skills | List skills (with descriptions) |
| GET | /v1/skills/:name/:version | Get skill metadata + instructions |
| DELETE | /v1/skills/:name/:version | Delete a skill |
| GET | /health | Liveness probe |
| GET | /ready | Readiness probe |

See [docs/API.md](docs/API.md) for the full reference.

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

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

MIT. See [LICENSE](LICENSE).

---

Built and maintained by [devs group](https://devs-group.com) · Kreuzlingen, Switzerland
