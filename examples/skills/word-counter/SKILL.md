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
