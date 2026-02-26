---
name: exit-nonzero
version: "1.0.0"
description: Exits with a non-zero code — tests failure handling
lang: python
image: python:3.12-slim
timeout: 10s
resources:
  memory: 64Mi
  cpu: "0.25"
---

# Exit Non-Zero Skill

Deliberately exits with a non-zero exit code to test error handling.
