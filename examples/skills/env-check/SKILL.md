---
name: env-check
version: "1.0.0"
description: Reports sandbox environment variables — tests env injection
lang: python
image: python:3.12-slim
timeout: 10s
resources:
  memory: 64Mi
  cpu: "0.25"
---

# Environment Check Skill

Reports which SANDBOX_* env vars are set and their values.
Also reports any custom env vars passed via the execution request.
