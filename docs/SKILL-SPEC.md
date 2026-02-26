# Skill Format Specification

**Version: v0** (breaking changes permitted before v1.0 release)

A Skill is a versioned zip archive containing a `SKILL.md` definition plus supporting scripts. This format is the stable contract between skill authors and the runtime.

## Archive Structure

```
my-skill/
├── SKILL.md              # Required: YAML frontmatter + instructions
├── scripts/
│   └── main.py           # Required: Primary entrypoint
├── requirements.txt      # Optional: Python dependencies
├── package.json          # Optional: Node.js dependencies
└── references/           # Optional: Supplemental documents
```

### Required Files

| Path | Description |
|---|---|
| `SKILL.md` | YAML frontmatter defining the skill metadata, plus natural-language instructions |
| `scripts/main.py` | Primary entrypoint. Also accepted: `main.sh`, `main.js`, `run.py` |

### Optional Files

| Path | Description |
|---|---|
| `requirements.txt` | Python dependencies. Installed with `pip install -r` before execution |
| `package.json` | Node.js dependencies. Installed with `npm install` before execution |
| `references/` | Supplemental documents injected into skill context |

## SKILL.md Format

The SKILL.md file consists of YAML frontmatter (between `---` delimiters) followed by a markdown body containing instructions.

### Frontmatter Fields

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `name` | string | Yes | — | Unique lowercase-hyphen identifier (e.g., `energy-report-generator`) |
| `version` | string | Yes | — | Semver string (e.g., `1.0.0`). `latest` accepted as alias by the API |
| `description` | string | Yes | — | One-line description shown in CLI and registry |
| `lang` | enum | No | `python` | `python`, `node`, or `bash` |
| `image` | string | No | Per-language default | Docker image. Must appear in server image allowlist |
| `timeout` | duration | No | Server default (120s) | Per-skill timeout override. Max: 10 minutes |
| `resources.cpu` | string | No | Server default (0.5) | CPU limit (e.g., `0.5`, `1`, `2`) |
| `resources.memory` | string | No | Server default (256Mi) | Memory limit (e.g., `128Mi`, `512Mi`, `1Gi`) |

### Default Images

| Language | Default Image |
|---|---|
| `python` | `python:3.12-slim` |
| `node` | `node:20-slim` |
| `bash` | `bash:5` |

### Example

```yaml
---
name: data-analysis
version: "1.0.0"
description: Analyze CSV data and produce summary statistics
lang: python
image: python:3.12-slim
timeout: 60s
resources:
  memory: 256Mi
  cpu: "0.5"
---

# Data Analysis Skill

Analyze tabular data and produce descriptive statistics per column.

## Input

Provide a JSON object with a `data` field containing an array of records.

## Output

Returns summary statistics for each column including mean, median,
standard deviation, and min/max values.
```

## I/O Contract

Every skill must honour the following contract regardless of language:

### Environment Variables

| Variable | Description |
|---|---|
| `$SANDBOX_INPUT` | JSON string containing the execution input. Mirrors `/sandbox/input.json` |
| `$SANDBOX_OUTPUT` | Path to write `output.json` (default: `/sandbox/out/output.json`) |
| `$SANDBOX_FILES_DIR` | Directory to write file artifacts (default: `/sandbox/out/files/`) |
| `$SKILL_INSTRUCTIONS` | Full text of the SKILL.md body (markdown content after frontmatter) |

### Reading Input

```python
import json, os

input_data = json.loads(os.environ.get("SANDBOX_INPUT", "{}"))
```

```javascript
const input = JSON.parse(process.env.SANDBOX_INPUT || "{}");
```

```bash
INPUT=$(echo "$SANDBOX_INPUT" | jq '.')
```

### Writing Output

Write a valid JSON file to the path in `$SANDBOX_OUTPUT`:

```python
import json, os

output_path = os.environ.get("SANDBOX_OUTPUT", "/sandbox/out/output.json")
os.makedirs(os.path.dirname(output_path), exist_ok=True)
with open(output_path, "w") as f:
    json.dump({"result": "success", "data": [1, 2, 3]}, f)
```

### Writing File Artifacts

Write any files to `$SANDBOX_FILES_DIR`. They will be tarred, uploaded to S3/MinIO, and returned as a presigned URL:

```python
import os

files_dir = os.environ.get("SANDBOX_FILES_DIR", "/sandbox/out/files/")
os.makedirs(files_dir, exist_ok=True)

with open(os.path.join(files_dir, "report.pdf"), "wb") as f:
    f.write(pdf_bytes)
```

### Signaling Failure

Exit with a non-zero code. Stdout and stderr are captured as execution logs:

```python
import sys

if error_condition:
    print("Error: something went wrong", file=sys.stderr)
    sys.exit(1)
```

## Filesystem Layout Inside Container

```
/sandbox/
├── scripts/            # Read-only mount: skill scripts
│   └── main.py
├── input.json          # Read-only mount: input data
└── out/                # Read-write mount
    ├── output.json     # Skill writes structured output here
    └── files/          # Skill writes file artifacts here
```

Additional writable areas (tmpfs, noexec):
- `/tmp` — 64MB tmpfs for temporary files
- `/root` — 1MB tmpfs

## Packaging

Package a skill directory into a zip archive:

```bash
# Using the CLI
skillbox skill package ./my-skill

# Using zip directly
cd my-skill && zip -r ../my-skill-1.0.0.zip .
```

### Validation

Before uploading, validate your skill:

```bash
skillbox skill lint ./my-skill
```

Checks performed:
- SKILL.md exists and has valid YAML frontmatter
- Required fields (name, version, description) are present
- Entrypoint script exists
- Image is in the default allowlist
