---
name: pdf-extract
version: "1.0.0"
description: Extract text content from a base64-encoded PDF
lang: python
image: python:3.12-slim
timeout: 60s
resources:
  memory: 512Mi
  cpu: "0.5"
---

# PDF Text Extraction Skill

Extract text content from PDF documents.

## Input

Provide a JSON object with either:
- `pdf_base64`: Base64-encoded PDF content
- `text`: Plain text to process (fallback for testing)

## Output

Returns a JSON object with:
- `pages`: Array of objects with `page_number` and `text` fields
- `total_pages`: Number of pages extracted
- `total_characters`: Total character count
