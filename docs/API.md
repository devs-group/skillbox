# API Reference

All API requests require authentication via Bearer token unless otherwise noted.

**Base URL**: `http://localhost:8080` (configurable via `SKILLBOX_API_PORT`)

## Authentication

Include the API key in the Authorization header:

```
Authorization: Bearer sk-your-api-key-here
```

API keys are SHA-256 hashed before storage. Each key is scoped to a tenant.

## Endpoints

### Health

#### GET /health

Liveness probe. Returns 200 when the process is running. No authentication required.

**Response**: `200 OK`
```json
{
  "status": "ok"
}
```

#### GET /ready

Readiness probe. Returns 200 when all dependencies are connected. No authentication required.

**Response**: `200 OK`
```json
{
  "status": "ready"
}
```

**Response**: `503 Service Unavailable`
```json
{
  "status": "not_ready",
  "checks": {
    "postgres": "connection refused"
  }
}
```

---

### Executions

#### POST /v1/executions

Run a skill synchronously. Blocks until the execution completes or times out.

**Request**:
```json
{
  "skill": "data-analysis",
  "version": "1.0.0",
  "input": {
    "data": [
      {"name": "Alice", "age": 30},
      {"name": "Bob", "age": 25}
    ]
  },
  "env": {
    "CUSTOM_VAR": "value"
  }
}
```

| Field | Type | Required | Description |
|---|---|---|---|
| `skill` | string | Yes | Skill name as registered in the registry |
| `version` | string | No | Version to run. Defaults to `latest` |
| `input` | object | No | Arbitrary JSON passed as `$SANDBOX_INPUT` |
| `env` | map | No | Extra env vars injected into the container |

**Response**: `200 OK`
```json
{
  "execution_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "success",
  "output": {
    "row_count": 2,
    "column_count": 2,
    "columns": {}
  },
  "files_url": "http://minio:9000/executions/.../files.tar.gz?...",
  "files_list": ["summary.txt"],
  "logs": "Analysis complete: 2 rows, 2 columns\n",
  "duration_ms": 1234,
  "error": null
}
```

| Field | Type | Description |
|---|---|---|
| `execution_id` | UUID | Unique identifier for this execution |
| `status` | string | `success`, `failed`, or `timeout` |
| `output` | object | Parsed JSON from the skill's output.json. Null if not written |
| `files_url` | string | Presigned URL for files.tar.gz (1-hour TTL). Null if no files |
| `files_list` | string[] | Relative paths of files in the archive |
| `logs` | string | Combined stdout and stderr from the container |
| `duration_ms` | int | Wall-clock execution time in milliseconds |
| `error` | string | Error message when status is `failed` or `timeout` |

#### GET /v1/executions/:id

Fetch the result of a completed execution.

**Response**: `200 OK` — Same format as POST response.

**Response**: `404 Not Found`
```json
{
  "error": "not_found",
  "message": "execution not found"
}
```

#### GET /v1/executions/:id/logs

Fetch execution logs as plain text.

**Response**: `200 OK` (Content-Type: text/plain)
```
Analysis complete: 2 rows, 2 columns
Chart written to /sandbox/out/files/summary.txt
```

---

### Skills

#### POST /v1/skills

Upload a skill zip archive.

**Request**: Content-Type: `application/zip`

Send the raw zip data in the request body. The zip must contain a valid SKILL.md file.

**Response**: `201 Created`
```json
{
  "name": "data-analysis",
  "version": "1.0.0",
  "description": "Analyze CSV data and produce summary statistics"
}
```

#### GET /v1/skills

List all skills available to the calling tenant.

**Response**: `200 OK`
```json
[
  {
    "name": "data-analysis",
    "version": "1.0.0",
    "description": "Analyze CSV data and produce summary statistics"
  },
  {
    "name": "text-summary",
    "version": "1.0.0",
    "description": "Produce a summary of input text"
  }
]
```

#### GET /v1/skills/:name/:version

Fetch skill metadata and SKILL.md content.

**Response**: `200 OK`
```json
{
  "name": "data-analysis",
  "version": "1.0.0",
  "description": "Analyze CSV data and produce summary statistics",
  "lang": "python",
  "content": "# Data Analysis Skill\n\nAnalyze data and..."
}
```

#### DELETE /v1/skills/:name/:version

Delete a skill version.

**Response**: `204 No Content`

---

## Error Format

All errors return a consistent JSON structure:

```json
{
  "error": "error_code",
  "message": "Human-readable description"
}
```

### Error Codes

| HTTP Status | Error Code | Description |
|---|---|---|
| 400 | `bad_request` | Invalid request body or parameters |
| 401 | `unauthorized` | Missing or invalid API key |
| 403 | `forbidden` | Tenant mismatch or insufficient permissions |
| 404 | `not_found` | Resource not found |
| 413 | `payload_too_large` | Skill zip exceeds size limit |
| 422 | `invalid_skill` | Skill validation failed |
| 500 | `internal_error` | Unexpected server error |
| 503 | `service_unavailable` | Dependency not ready |

## gRPC API

The gRPC API mirrors the REST endpoints on port 9090. Service definitions:

- `ExecutionService.RunSkill` — mirrors POST /v1/executions
- `ExecutionService.GetExecution` — mirrors GET /v1/executions/:id
- `ExecutionService.GetExecutionLogs` — mirrors GET /v1/executions/:id/logs
- `SkillService.ListSkills` — mirrors GET /v1/skills
- `SkillService.GetSkill` — mirrors GET /v1/skills/:name/:version
- `SkillService.DeleteSkill` — mirrors DELETE /v1/skills/:name/:version
- `HealthService.Check` — mirrors GET /health

See `proto/skillbox/v1/skillbox.proto` for the full protobuf definitions.
