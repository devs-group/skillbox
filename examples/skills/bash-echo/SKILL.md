---
name: bash-echo
version: "1.0.0"
description: Echoes input back as output — tests input/output round-trip
lang: python
image: python:3.12-slim
timeout: 10s
resources:
  memory: 64Mi
  cpu: "0.25"
---

# Echo Skill

Reads JSON input and writes it back as output wrapped in an "echo" field.
Used for testing input/output round-trip fidelity.
