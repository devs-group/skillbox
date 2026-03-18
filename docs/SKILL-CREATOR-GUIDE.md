# Skill Creator Guide

How to implement a skill creator on top of Skillbox — enabling users and agents to author, store, and invoke prompt-based skills through your application.

## Background

Skillbox ships two kinds of skills:

| | Executable skills | Prompt skills |
|---|---|---|
| **What they are** | Sandboxed code (Python/Node/Bash) | LLM instruction templates |
| **Who runs them** | Skillbox runner (OpenSandbox) | Your application (injected into LLM context) |
| **Entrypoint** | `main.py`, `main.js`, `main.sh` | None — the markdown body *is* the skill |
| **SKILL.md `lang`** | `python`, `node`, `bash` | Omitted or empty |

Executable skills already work end-to-end. This guide covers **prompt skills** — the Claude Code / OpenClaw pattern where a skill is a set of instructions that shape agent behavior at runtime.

## Architecture

```
+---------------------+       +---------------------+
|   Your Application  |       |      Skillbox       |
|                     |       |                     |
|  Skill Creator UI   |       |  Registry (MinIO)   |
|  LLM Integration    | ----> |  Catalog API        |
|  Prompt Injection   |       |  Metadata (Postgres) |
|  Execution Context  |       |  Versioning         |
+---------------------+       +---------------------+
```

**Skillbox** handles storage, versioning, catalog, and discovery — the same primitives it uses for executable skills. **Your application** handles the creator UX, LLM integration, and runtime prompt injection. The bridge is the existing `POST /v1/skills` and `GET /v1/skills/:name/:version` API.

## SKILL.md Format for Prompt Skills

Prompt skills use the same SKILL.md frontmatter. Omit `lang` (or leave it empty) to signal that this skill has no executable entrypoint:

```yaml
---
name: code-reviewer
version: "1.0.0"
description: Review code changes for correctness, security, and style
---

You are a senior code reviewer. When presented with a diff or code snippet:

1. Check for correctness — does the code do what it claims?
2. Check for security — SQL injection, XSS, command injection, hardcoded secrets
3. Check for style — naming conventions, dead code, unnecessary complexity
4. Check for tests — are edge cases covered?

## Output Format

Respond with a structured review:
- **Summary**: One sentence overall assessment
- **Issues**: Bulleted list, each with severity (critical/warning/info)
- **Suggestion**: Concrete code fix if applicable
```

The zip archive for a prompt skill contains only SKILL.md — no scripts directory, no entrypoint:

```
code-reviewer/
└── SKILL.md
```

## Step 1: Store Prompt Skills via the Skillbox API

**Option A: Structured fields (recommended)**

Use `POST /v1/skills/from-fields` to create skills from JSON — no zip packaging needed:

```bash
curl -X POST http://localhost:8080/v1/skills/from-fields \
  -H "Authorization: Bearer $SKILLBOX_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "code-reviewer",
    "version": "1.0.0",
    "description": "Review code changes for correctness, security, and style",
    "instructions": "You are a senior code reviewer..."
  }'
```

Or with the Go SDK:

```go
client := skillbox.New("http://localhost:8080", "sk-your-key")
result, err := client.UpsertSkillFromFields(ctx, skillbox.CreateFromFieldsRequest{
    Name:        "code-reviewer",
    Version:     "1.0.0",
    Description: "Review code changes for correctness, security, and style",
    Instructions: "You are a senior code reviewer...",
})
```

Upsert semantics: calling again with the same name replaces the skill.

**Option B: Zip upload**

Package and upload the skill as a zip archive (useful for multi-file skills):

```bash
# Package
cd code-reviewer && zip -r ../code-reviewer-1.0.0.zip .

# Upload
curl -X POST http://localhost:8080/v1/skills \
  -H "Authorization: Bearer $SKILLBOX_API_KEY" \
  -H "Content-Type: application/zip" \
  --data-binary @code-reviewer-1.0.0.zip
```

Or with the Python SDK:

```python
from skillbox import Client

client = Client("http://localhost:8080", "sk-your-key")

# Upload from a directory
client.upload_skill("./code-reviewer")
```

## Step 2: Discover and Fetch at Runtime

Your application lists available prompt skills and fetches their instructions:

```python
# List all skills — descriptions help the agent choose
skills = client.list_skills()
prompt_skills = [s for s in skills if not s.lang]

# Fetch the full instructions for one skill
skill = client.get_skill("code-reviewer", "latest")
instructions = skill.instructions  # The markdown body from SKILL.md
```

Or via REST:

```bash
# List
curl http://localhost:8080/v1/skills \
  -H "Authorization: Bearer $SKILLBOX_API_KEY"

# Get instructions
curl http://localhost:8080/v1/skills/code-reviewer/latest \
  -H "Authorization: Bearer $SKILLBOX_API_KEY" | jq -r .instructions
```

## Step 3: Inject Into LLM Context

The simplest integration is system-prompt injection. When the agent selects a skill, fetch its instructions and prepend them to the conversation:

```python
import anthropic

client = anthropic.Anthropic()
skillbox = SkillboxClient("http://localhost:8080", "sk-your-key")

def run_with_skill(skill_name: str, user_message: str) -> str:
    # Fetch skill instructions from Skillbox
    skill = skillbox.get_skill(skill_name, "latest")

    response = client.messages.create(
        model="claude-sonnet-4-6",
        system=skill.instructions,
        messages=[{"role": "user", "content": user_message}],
    )
    return response.content[0].text
```

For agent frameworks that support tool selection, expose prompt skills as tools:

```python
# LangChain example
from langchain_core.tools import StructuredTool

def build_prompt_skill_tool(skill_meta):
    """Turn a Skillbox prompt skill into a LangChain tool."""

    def invoke(user_input: str) -> str:
        skill = skillbox.get_skill(skill_meta.name, "latest")
        response = llm.invoke([
            SystemMessage(content=skill.instructions),
            HumanMessage(content=user_input),
        ])
        return response.content

    return StructuredTool.from_function(
        func=invoke,
        name=skill_meta.name.replace("-", "_"),
        description=skill_meta.description,
    )

# Build tools from all prompt skills
tools = [build_prompt_skill_tool(s) for s in prompt_skills]
agent = create_react_agent(llm, tools)
```

## Step 4: Build the Skill Creator

The skill creator is where users author and iterate on prompt skills. This is entirely application-layer logic — Skillbox just stores the result.

### Minimal creator flow

```
User writes instructions
        ↓
Application validates SKILL.md frontmatter
        ↓
User tests against sample inputs (application calls LLM)
        ↓
Application packages zip and uploads to Skillbox
        ↓
Skill is live in the catalog
```

### Example: API-driven creator

```python
import requests

def create_prompt_skill(
    name: str,
    version: str,
    description: str,
    instructions: str,
) -> dict:
    """Create a prompt skill via the structured fields API."""

    return requests.post(
        f"{SKILLBOX_URL}/v1/skills/from-fields",
        headers={"Authorization": f"Bearer {SKILLBOX_API_KEY}"},
        json={
            "name": name,
            "version": version,
            "description": description,
            "instructions": instructions,
        },
    ).json()
```

Or with the Go SDK:

```go
result, err := client.UpsertSkillFromFields(ctx, skillbox.CreateFromFieldsRequest{
    Name:         name,
    Description:  description,
    Instructions: instructions,
    Version:      version,
})
```

### Example: LLM-assisted creator

Let users describe what they want and have an LLM draft the skill instructions:

```python
def draft_skill(user_description: str) -> str:
    """Use an LLM to draft skill instructions from a description."""

    response = client.messages.create(
        model="claude-sonnet-4-6",
        system="""You are a skill author. Given a description of what the skill
should do, write the SKILL.md body (markdown instructions only, not the
frontmatter). Be specific about input format, output format, and edge cases.
Keep instructions concise — under 500 words.""",
        messages=[{"role": "user", "content": user_description}],
    )
    return response.content[0].text
```

### Example: Test before publish

```python
def test_prompt_skill(instructions: str, test_input: str) -> str:
    """Run a prompt skill against a test input without publishing it."""

    response = client.messages.create(
        model="claude-sonnet-4-6",
        system=instructions,
        messages=[{"role": "user", "content": test_input}],
    )
    return response.content[0].text


# Iterate
instructions = draft_skill("A skill that translates technical jargon to plain English")
result = test_prompt_skill(instructions, "The API returns a 429 when the rate limit is exceeded")
print(result)

# Satisfied? Publish.
create_prompt_skill(
    name="jargon-translator",
    version="1.0.0",
    description="Translate technical jargon to plain English",
    instructions=instructions,
)
```

## Step 5: Distinguish Skill Types at Runtime

Since prompt skills omit the `lang` field, your application can distinguish them:

```python
def is_prompt_skill(skill) -> bool:
    """Prompt skills have no lang set."""
    return not skill.lang

def handle_skill(skill_name: str, input_data: dict) -> dict:
    skill = skillbox.get_skill(skill_name, "latest")

    if is_prompt_skill(skill):
        # Inject into LLM context
        return run_with_llm(skill.instructions, input_data)
    else:
        # Execute in sandbox
        return skillbox.run(skill_name, input=input_data)
```

This gives you a single catalog with a unified discovery API, but two execution paths.

## Full Lifecycle

```
Author                     Skillbox                  Application (runtime)
  │                           │                           │
  │  1. Write SKILL.md        │                           │
  │  (or use LLM draft)       │                           │
  │                           │                           │
  │  2. Test locally ─────────┼───────────────────────────┤ LLM call
  │     (iterate)             │                           │
  │                           │                           │
  │  3. Publish ─────────────┤ POST /v1/skills/from-fields │
  │     (or zip upload)      │  (or POST /v1/skills)      │
  │                           │  → store zip in MinIO     │
  │                           │  → index in Postgres      │
  │                           │                           │
  │                           │  4. Agent discovers ──────┤ GET /v1/skills
  │                           │                           │  (list catalog)
  │                           │                           │
  │                           │  5. Agent fetches ────────┤ GET /v1/skills/:name/latest
  │                           │                           │  (get instructions)
  │                           │                           │
  │                           │                           │  6. Inject instructions
  │                           │                           │     into LLM context
  │                           │                           │
  │                           │                           │  7. Return result to user
```

## What Stays in Skillbox vs. the Application

| Concern | Where it lives | Why |
|---|---|---|
| Skill storage & versioning | Skillbox | Already built, multi-tenant, S3-backed |
| Catalog & discovery API | Skillbox | `GET /v1/skills` returns descriptions for tool selection |
| SKILL.md parsing & validation | Skillbox | Frontmatter + body extraction already works |
| Skill creator UX | Application | Needs user-facing UI, LLM calls for drafting |
| Prompt testing | Application | Requires LLM integration |
| LLM context injection | Application | Application owns the agent / LLM pipeline |
| Executable skill running | Skillbox | Sandboxed container execution |

## Tips

- **Version your prompt skills** the same way you version executable skills. When you change instructions, bump the version. This lets you A/B test different prompt versions.
- **Use descriptions wisely.** The `description` field in frontmatter is what agents see in the catalog. Write it for LLM consumption — be specific about what the skill does and when to use it.
- **Keep instructions focused.** One skill = one capability. A 2000-word instruction set that tries to do everything will perform worse than three 300-word focused skills.
- **Cache aggressively.** Skill instructions change infrequently. Cache `GET /v1/skills/:name/:version` responses in your application to avoid round-trips on every invocation.
- **Combine both types.** An agent can use prompt skills for reasoning tasks and executable skills for computation — all discovered from the same catalog.
