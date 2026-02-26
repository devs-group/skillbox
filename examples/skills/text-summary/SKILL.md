---
name: text-summary
version: "1.0.0"
description: Produce a summary of input text using extractive summarization
lang: python
image: python:3.12-slim
timeout: 30s
resources:
  memory: 128Mi
  cpu: "0.5"
---

# Text Summary Skill

Produce a concise summary of input text using extractive summarization.

## Input

Provide a JSON object with:
- `text`: The text to summarize
- `max_sentences`: Maximum number of sentences in the summary (default: 3)

## Output

Returns a JSON object with:
- `summary`: The extracted summary text
- `sentence_count`: Number of sentences in the summary
- `compression_ratio`: Ratio of summary length to original length
