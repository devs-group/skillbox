# TODO

## P0 — Critical

- [ ] Add rate limiting — no rate limiting on any endpoint currently
- [ ] Add HTTP server `WriteTimeout` and `ReadTimeout` (slowloris risk)
- [ ] Remove gRPC port 9090 from Helm/k8s manifests — no gRPC server exists, port is never started
- [ ] API key management endpoints — keys can only be created via direct DB access (`seed-apikey.sh`)

## P1 — High

- [ ] Expand test suite — 8 packages have zero test coverage (execution, sandbox, sessions, skill handlers, artifacts, session_manager, path_validator, store/executions, store/sessions, store/apikeys)
- [ ] Add integration tests that run full skill execution end-to-end
- [ ] Local Kubernetes setup with Kind for Helm chart testing
- [ ] Wrap multi-step writes in transactions — skill upload + metadata insert not atomic despite `RunInTx` existing
- [ ] Migrate runner logging from `log.Printf` to `slog` — 25+ calls bypass structured JSON logging
- [ ] Add `GET /v1/executions` (list) endpoint — store method exists, no HTTP handler
- [ ] Complete Python SDK — missing ~60% of API surface (no sandbox, sessions, delete_skill, upload_file)
- [ ] Async execution mode — HTTP request blocks for up to 10 minutes per execution
- [ ] Security scanner for uploaded skills — scan ZIPs for suspicious patterns before making available

## P2 — Medium

- [ ] Admin UI — lightweight web UI for managing skills (list, upload, delete, view executions, logs)
- [ ] Improve CLI UX — interactive skill management, better output formatting, `skill get/delete`, `exec list/get`, `file` commands. Migrate from Cobra to urfave/cli v3 (team convention)

- [ ] Add Prometheus `/metrics` endpoint and basic counters (executions, errors, latency)
- [ ] Wire up Redis for API key caching — config exists (`SKILLBOX_REDIS_URL`) but is unused
- [ ] Fix `ListFileVersions` — only traverses one level of parent chain, loses history for v3+
- [ ] Add security response headers (`X-Content-Type-Options`, `Strict-Transport-Security`, etc.)
- [ ] Run Python SDK tests in CI
- [ ] Add Helm chart linting to CI
- [ ] Expand CLI — ~65% of API surface unreachable (no file, session, sandbox, exec list commands)
- [ ] Add upload size limit for Files API — `handlers/files.go` Upload() missing `MaxBytesReader`
- [ ] Add pagination metadata to list responses (total count, page info)

## P3 — Nice to Have

- [ ] Skill Creator Skill — a built-in skill that generates new skills (like Claude Code or OpenClaw)
- [ ] Remove dead code — `ErrNotImplemented`, `InsertExecution`, `ValidateKey` are unused
- [ ] Separate Go SDK into its own module — currently shares root `go.mod`, pulling all server deps
- [ ] Add distributed tracing (OpenTelemetry / OTLP)
- [ ] `/ready` endpoint should check MinIO and OpenSandbox, not just Postgres
