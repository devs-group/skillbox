# Contributing to Skillbox

Thank you for your interest in contributing to Skillbox! This guide will help you get started.

## Development Setup

### Prerequisites

- Go 1.22 or later
- Docker Engine 20+
- Docker Compose v2
- Make

### Getting Started

```bash
git clone https://github.com/devs-group/skillbox.git
cd skillbox

# Start dependencies (Postgres, Redis, MinIO, Docker socket proxy)
make dev

# In another terminal, seed an API key
make seed

# Build the CLI
make build-all

# Run tests
make test
```

### Project Structure

```
cmd/skillbox-server/    Server binary
cmd/skillbox/           CLI binary
internal/api/           HTTP handlers, middleware, router
internal/runner/        Docker container execution engine
internal/registry/      Skill storage and loading (MinIO/S3)
internal/store/         Database layer (PostgreSQL)
internal/artifacts/     File artifact collection
internal/config/        Configuration
internal/skill/         SKILL.md format parser
sdks/go/                Go SDK (single-file, stdlib-only)
deploy/docker/          Docker Compose stack
deploy/k8s/             Kubernetes manifests
examples/skills/        Example skills
docs/                   Documentation
```

## Development Workflow

1. Fork the repository
2. Create a feature branch: `git checkout -b feat/your-feature`
3. Make your changes
4. Run checks: `make lint test`
5. Commit with a descriptive message
6. Push and open a Pull Request

### Commit Messages

Use conventional commit format:

```
feat: add async execution mode
fix: handle nil output in runner
docs: update SKILL.md spec with resources field
refactor: extract container config into security.go
test: add integration tests for skill upload
```

### Code Style

- Run `make fmt` before committing
- Run `make lint` to check for issues
- Follow standard Go conventions
- Keep the SDK dependency-free (stdlib only)
- Prefer explicit error handling over panics

## Running Tests

```bash
# Unit tests
make test

# With coverage
make test-cover

# Integration tests (requires Docker)
make test-integration
```

## Adding a New Skill

1. Create a directory under `examples/skills/`
2. Add a `SKILL.md` with valid frontmatter
3. Add an entrypoint in `scripts/` (main.py, main.js, or main.sh)
4. Test locally: `skillbox skill lint ./examples/skills/your-skill`
5. Push: `skillbox skill push ./examples/skills/your-skill`
6. Run: `skillbox run your-skill --input '{}'`

See [docs/SKILL-SPEC.md](docs/SKILL-SPEC.md) for the full specification.

## Reporting Issues

- Use GitHub Issues for bugs and feature requests
- Include reproduction steps for bugs
- Include your environment (OS, Docker version, Go version)

## Security

See [SECURITY.md](SECURITY.md) for reporting security vulnerabilities.

## License

By contributing, you agree that your contributions will be licensed under the Apache-2.0 License.
