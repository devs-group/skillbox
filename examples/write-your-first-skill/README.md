# Write Your First Skill

This guide walks through creating, testing, and deploying a custom Skillbox skill from scratch.

## What we're building

A **word-counter** skill that takes text input and returns word frequency statistics.

## Step 1: Create the directory structure

```
word-counter/
├── SKILL.md
├── scripts/
│   └── main.py
└── requirements.txt    (optional — only if you need pip packages)
```

```bash
mkdir -p word-counter/scripts
```

## Step 2: Write SKILL.md

The `SKILL.md` file defines your skill's metadata in YAML frontmatter and provides instructions in markdown.

```bash
cat > word-counter/SKILL.md << 'EOF'
---
name: word-counter
version: "1.0.0"
description: Count word frequencies in text
lang: python
image: python:3.12-slim
timeout: 30s
resources:
  memory: 128Mi
  cpu: "0.5"
---

# Word Counter Skill

Count word frequencies in input text and return the top N most common words.

## Input

- `text` (string, required): The text to analyze
- `top_n` (integer, optional): Number of top words to return (default: 10)

## Output

- `total_words`: Total word count
- `unique_words`: Number of unique words
- `top_words`: Array of `{word, count}` objects
EOF
```

## Step 3: Write the entrypoint script

Every skill reads input from `$SANDBOX_INPUT` (JSON string) and writes output to `$SANDBOX_OUTPUT` (file path).

```bash
cat > word-counter/scripts/main.py << 'PYEOF'
import json
import os
import re
from collections import Counter

def main():
    # Read input from environment variable
    raw = os.environ.get("SANDBOX_INPUT", "{}")
    input_data = json.loads(raw)

    text = input_data.get("text", "")
    top_n = input_data.get("top_n", 10)

    # Tokenize and count
    words = re.findall(r'\b[a-z]+\b', text.lower())
    counter = Counter(words)

    # Build result
    result = {
        "total_words": len(words),
        "unique_words": len(counter),
        "top_words": [
            {"word": w, "count": c}
            for w, c in counter.most_common(top_n)
        ],
    }

    # Write output
    output_path = os.environ.get("SANDBOX_OUTPUT", "/sandbox/out/output.json")
    os.makedirs(os.path.dirname(output_path), exist_ok=True)
    with open(output_path, "w") as f:
        json.dump(result, f, indent=2)

    print(f"Counted {len(words)} words, {len(counter)} unique")

if __name__ == "__main__":
    main()
PYEOF
```

## Step 4: Lint the skill

```bash
skillbox skill lint word-counter
```

Expected output:

```
Linting word-counter
  PASS  name
  PASS  version
  PASS  description
  PASS  entrypoint
  PASS  image
All checks passed
```

## Step 5: Push to the server

```bash
# Push directly from directory (packages + uploads automatically)
skillbox skill push word-counter
```

Or package and push separately:

```bash
skillbox skill package word-counter
# -> word-counter-1.0.0.zip

skillbox skill push word-counter-1.0.0.zip
```

## Step 6: Run it

With the CLI:

```bash
skillbox run word-counter --input '{
  "text": "the quick brown fox jumps over the lazy dog the fox",
  "top_n": 5
}'
```

With curl:

```bash
curl -s -X POST http://localhost:8080/v1/executions \
  -H "Authorization: Bearer $SKILLBOX_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "skill": "word-counter",
    "input": {
      "text": "the quick brown fox jumps over the lazy dog the fox",
      "top_n": 5
    }
  }' | jq .
```

Expected output:

```json
{
  "status": "success",
  "output": {
    "total_words": 11,
    "unique_words": 8,
    "top_words": [
      {"word": "the", "count": 3},
      {"word": "fox", "count": 2},
      {"word": "quick", "count": 1},
      {"word": "brown", "count": 1},
      {"word": "jumps", "count": 1}
    ]
  }
}
```

## Step 7: Produce file artifacts (optional)

Skills can write files to `$SANDBOX_FILES_DIR`. These get collected, archived as `.tar.gz`, uploaded to S3, and returned as a presigned URL.

Add this to your `main.py`:

```python
# Write a report file
files_dir = os.environ.get("SANDBOX_FILES_DIR", "/sandbox/out/files/")
os.makedirs(files_dir, exist_ok=True)

report_path = os.path.join(files_dir, "report.txt")
with open(report_path, "w") as f:
    f.write(f"Word Count Report\n")
    f.write(f"=================\n\n")
    f.write(f"Total words: {len(words)}\n")
    for w, c in counter.most_common(top_n):
        f.write(f"  {w}: {c}\n")
```

The execution response will include:

```json
{
  "files_url": "https://minio:9000/executions/.../files.tar.gz?...",
  "files_list": ["report.txt"]
}
```

## Step 8: Use from Go

```go
client := skillbox.New("http://localhost:8080", os.Getenv("SKILLBOX_API_KEY"))

result, err := client.Run(ctx, skillbox.RunRequest{
    Skill: "word-counter",
    Input: json.RawMessage(`{"text": "hello world hello", "top_n": 5}`),
})

fmt.Println(result.Status)       // "success"
fmt.Println(string(result.Output)) // {"total_words": 3, ...}

// Download file artifacts
if result.HasFiles() {
    client.DownloadFiles(ctx, result, "./output")
}
```

## Skill contract summary

| Environment Variable | Description |
|---|---|
| `SANDBOX_INPUT` | JSON string with input data |
| `SANDBOX_OUTPUT` | Path to write `output.json` |
| `SANDBOX_FILES_DIR` | Directory for file artifacts |
| `SKILL_INSTRUCTIONS` | Markdown instructions from SKILL.md |

Your script:
1. Reads from `SANDBOX_INPUT`
2. Does its work
3. Writes JSON to `SANDBOX_OUTPUT`
4. Optionally writes files to `SANDBOX_FILES_DIR`
5. Prints logs to stdout/stderr
