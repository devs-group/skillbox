---
name: multi-file-output
version: "1.0.0"
description: Produces multiple file artifacts — tests artifact collection
lang: python
image: python:3.12-slim
timeout: 15s
resources:
  memory: 64Mi
  cpu: "0.25"
---

# Multi-File Output Skill

Writes multiple files (text, CSV, nested dirs) to test artifact collection.
