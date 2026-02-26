---
name: slow-skill
version: "1.0.0"
description: Sleeps for a configurable duration — tests timeout enforcement
lang: python
image: python:3.12-slim
timeout: 5s
resources:
  memory: 64Mi
  cpu: "0.25"
---

# Slow Skill

Sleeps for a configurable number of seconds. Used to test timeout enforcement.
