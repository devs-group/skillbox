# Brainstorm: Fumadocs Documentation Site for Skillbox

**Date:** 2026-03-04
**Status:** Decided

## What We're Building

A developer-friendly documentation site for Skillbox using [Fumadocs](https://fumadocs.dev), living in `docs-site/` at the repository root. The site will serve both **AI/agent developers** (building skills, integrating SDKs) and **platform operators** (deploying, configuring, managing Skillbox).

### Key characteristics:
- **Modern dark theme** with accent colors (Vercel/Stripe docs vibe)
- **OpenAPI/Swagger viewer** auto-generated from Go code annotations (swaggo)
- **Progressive structure**: Getting Started -> Concepts -> Guides -> API Reference
- **Full content launch**: all sections populated from day one

## Why This Approach

### Fumadocs over alternatives (Docusaurus, Nextra, GitBook)
- Built on Next.js App Router — same stack as the existing landing page
- First-class MDX support with excellent code block rendering
- Built-in OpenAPI integration via `fumadocs-openapi`
- Search built-in (no Algolia dependency needed)
- Actively maintained, modern design system
- Easy to customize with Tailwind CSS

### `docs-site/` as location
- Clean separation from `landing-page/` (marketing ≠ docs)
- Independent build/deploy pipeline
- Can share design tokens or components later if needed
- Keeps `docs/` for raw markdown reference docs and plans

### Auto-generated OpenAPI (swaggo)
- Stays in sync with code as endpoints evolve
- Annotations live next to handler code — single source of truth
- Well-supported in the Go ecosystem
- `fumadocs-openapi` can consume the generated spec directly

## Key Decisions

1. **Location**: `docs-site/` at repo root
2. **Framework**: Fumadocs (Next.js-based)
3. **Theme**: Modern dark with accent colors
4. **Audience**: Both developers and operators, balanced
5. **API docs**: OpenAPI 3.x spec auto-generated via swaggo annotations in Go handlers, rendered with `fumadocs-openapi`
6. **Content scope**: Full launch — all sections populated
7. **Structure**: Getting Started -> Concepts -> Guides -> API Reference

## Documentation Structure

### 1. Getting Started
- **Introduction** — What is Skillbox, why it exists, key value props
- **Quick Start** — Docker Compose up, push first skill, run it (5 min)
- **Installation** — CLI, server, SDK setup

### 2. Concepts
- **Skills** — What they are, SKILL.md format, versioning, the registry
- **Execution Model** — How skills run in sandboxes, lifecycle, I/O contract
- **Cognitive Mode** — Library-style skills, code generation, LLM-driven execution
- **Sessions & Workspaces** — Persistent sandboxes, file management, session lifecycle
- **Sandboxes** — OpenSandbox integration, security model, resource limits
- **Authentication** — API keys, tenant isolation, security model

### 3. Guides
- **Write Your First Skill** — Step-by-step skill authoring tutorial
- **Skill Authoring Deep Dive** — Advanced patterns, multi-file skills, references
- **Agent Integration** — LangChain tools, Go SDK, Python SDK usage
- **Deploy to Production** — Docker Compose, Kubernetes, Helm chart
- **Configuration Reference** — All environment variables and their effects
- **File Management** — Upload, version, and manage files via the API
- **Session Workflows** — Using persistent sandbox sessions for multi-step tasks

### 4. API Reference
- Auto-generated from OpenAPI spec via `fumadocs-openapi`
- Grouped by resource: Executions, Skills, Files, Sessions, Sandbox, Health
- Request/response examples for every endpoint
- Authentication details

### 5. SDKs
- **Go SDK** — Installation, usage, examples
- **Python SDK** — Installation, usage, examples

## Design Direction

- Dark background (#09090b or similar) with light text
- Accent color from Skillbox branding (blue/purple gradient)
- Syntax highlighting with dark theme (e.g., Shiki with tokyo-night)
- Sidebar navigation with section grouping
- Search (Fumadocs built-in)
- Breadcrumbs, table of contents on each page
- Copy buttons on all code blocks
- Responsive / mobile-friendly

## Open Questions

- Should the docs site be deployed alongside the landing page (same domain, `/docs` path) or on a separate subdomain (`docs.skillbox.dev`)?
- Do we want a versioned docs system (v1, v2) from the start, or add it later?
- Should we include a changelog/release notes section?

## Technical Implementation Notes

- **Fumadocs setup**: `create-fumadocs-app` or manual setup with `fumadocs-core` + `fumadocs-ui` + `fumadocs-openapi`
- **swaggo annotations**: Add `@Summary`, `@Description`, `@Tags`, `@Param`, `@Success`, `@Failure` comments to each handler in `internal/api/handlers/`
- **OpenAPI generation**: `swag init` produces `docs/swagger.json` -> consumed by fumadocs-openapi
- **Content source**: MDX files in `docs-site/content/docs/`
- **Build**: `next build` in `docs-site/`
- **Deploy**: Vercel, Cloudflare Pages, or static export
