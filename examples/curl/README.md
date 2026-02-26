# Skillbox curl Examples

Step-by-step examples using only `curl` and `jq`. No SDK required.

## Prerequisites

Start the stack:

```bash
cd deploy/docker
docker compose up -d
```

## 1. Create an API key

```bash
# Generate a key and insert its hash into Postgres
API_KEY="sk-my-dev-key"
KEY_HASH=$(echo -n "$API_KEY" | shasum -a 256 | cut -d' ' -f1)

docker exec docker-postgres-1 psql -U skillbox -d skillbox -c \
  "INSERT INTO sandbox.api_keys (tenant_id, name, key_hash)
   VALUES ('default', 'dev-key', '$KEY_HASH')
   ON CONFLICT DO NOTHING;"

export SKILLBOX_API_KEY="$API_KEY"
```

## 2. Check health

```bash
# Liveness
curl -s http://localhost:8080/health | jq .
# {"status": "ok"}

# Readiness (checks Postgres)
curl -s http://localhost:8080/ready | jq .
# {"status": "ready", "checks": {"postgres": "ok"}}
```

## 3. Upload a skill

```bash
# Package the example skill as a zip
cd examples/skills/data-analysis
zip -r /tmp/data-analysis.zip SKILL.md scripts/ requirements.txt

# Upload it
curl -s -X POST http://localhost:8080/v1/skills \
  -H "Authorization: Bearer $SKILLBOX_API_KEY" \
  -F "file=@/tmp/data-analysis.zip" \
  -F "name=data-analysis" \
  -F "version=1.0.0" | jq .
```

## 4. List skills

```bash
curl -s http://localhost:8080/v1/skills \
  -H "Authorization: Bearer $SKILLBOX_API_KEY" | jq .
```

Output:

```json
[
  {
    "name": "data-analysis",
    "version": "1.0.0",
    "uploaded_at": "2026-02-26T12:27:19.222Z"
  }
]
```

## 5. Run a skill

```bash
curl -s -X POST http://localhost:8080/v1/executions \
  -H "Authorization: Bearer $SKILLBOX_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "skill": "data-analysis",
    "input": {
      "data": [
        {"name": "Alice", "age": 30, "score": 85.5},
        {"name": "Bob", "age": 25, "score": 92.0},
        {"name": "Charlie", "age": 35, "score": 78.3}
      ]
    }
  }' | jq .
```

Output:

```json
{
  "execution_id": "7257c33f-22a4-4ef9-96f9-4caa4f6c66a9",
  "status": "success",
  "output": {
    "row_count": 3,
    "column_count": 3,
    "columns": {
      "age": {
        "count": 3, "mean": 30.0, "median": 30.0,
        "std_dev": 4.0825, "min": 25.0, "max": 35.0,
        "type": "numeric"
      },
      "score": {
        "count": 3, "mean": 85.2667, "median": 85.5,
        "std_dev": 5.5977, "min": 78.3, "max": 92.0,
        "type": "numeric"
      },
      "name": {
        "count": 3, "unique": 3, "type": "categorical"
      }
    }
  },
  "files_list": ["summary.txt"],
  "logs": "Analysis complete: 3 rows, 3 columns\n",
  "duration_ms": 1994,
  "error": null
}
```

## 6. Run with CSV input

```bash
curl -s -X POST http://localhost:8080/v1/executions \
  -H "Authorization: Bearer $SKILLBOX_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "skill": "data-analysis",
    "input": {
      "csv": "name,age,salary\nAlice,30,75000\nBob,25,65000\nCharlie,35,95000"
    }
  }' | jq '.output'
```

## 7. Pin a specific version

```bash
curl -s -X POST http://localhost:8080/v1/executions \
  -H "Authorization: Bearer $SKILLBOX_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "skill": "data-analysis",
    "version": "1.0.0",
    "input": {"data": [{"x": 1}, {"x": 2}]}
  }' | jq '{status, output}'
```

Omitting `version` (or setting it to `"latest"`) automatically resolves to the most recently uploaded version.

## 8. Get execution details

```bash
EXEC_ID="<execution-id-from-step-5>"

# Full execution record
curl -s http://localhost:8080/v1/executions/$EXEC_ID \
  -H "Authorization: Bearer $SKILLBOX_API_KEY" | jq .

# Logs only (plain text)
curl -s http://localhost:8080/v1/executions/$EXEC_ID/logs \
  -H "Authorization: Bearer $SKILLBOX_API_KEY"
```

## 9. Delete a skill

```bash
curl -s -X DELETE http://localhost:8080/v1/skills/data-analysis/1.0.0 \
  -H "Authorization: Bearer $SKILLBOX_API_KEY" -w "\nHTTP %{http_code}\n"
# HTTP 204
```
