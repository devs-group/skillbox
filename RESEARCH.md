# Skillbox — Strategic Research & Investment Brief

> **The self-hosted execution runtime for the AI agent era.**
> Your infrastructure. Your data. Your rules.

---

## Executive Summary

The AI agent market is projected to reach **$10.9B in 2026** and **$52.6B by 2030** (45-50% CAGR). Every AI agent that writes code, analyzes data, or processes files needs one thing: a secure place to run it. This is the sandbox problem — and it's becoming the bottleneck of the entire AI agent stack.

**E2B raised $43.8M. Modal raised $80M. Daytona raised $31M.** Together, over **$289M in venture capital** has poured into this single layer of AI infrastructure. But every funded player is **cloud-only**. For enterprises with data sovereignty requirements, air-gapped environments, or existing Kubernetes clusters — the options are: build it yourself, or use Skillbox.

Skillbox is the **only serious open-source, self-hosted skill execution runtime** for AI agents. It gives agents a single API to run sandboxed scripts (Python, Node.js, Bash) and receive structured JSON output + file artifacts. It's built in Go, deploys on Docker Compose or Kubernetes, and enforces defense-in-depth security that can't be configured away by callers. The unique SKILL.md format creates a **discoverable, versioned, introspectable catalog** of agent capabilities — a fundamentally different abstraction from raw code execution.

---

## The Problem

Every AI agent that does useful work needs to execute code. But executing arbitrary code is dangerous. The industry faces three compounding problems:

### 1. The Security Gap
**41.7% of AI agent skills have vulnerabilities** (OWASP 2025). Most teams either skip sandboxing entirely ("we'll fix it later") or bolt on half-measures that don't survive a real security audit. The gap between "agent wants to run code" and "code runs safely" is where breaches happen.

### 2. The Sovereignty Problem
The EU AI Act takes effect **August 2026**. GDPR already restricts cross-border data transfers. Healthcare has HIPAA. Finance has SOC2/PCI-DSS. Government has FedRAMP. **57.46% of enterprise AI infrastructure** is deployed on-premises (Precedence Research). Yet every funded sandbox runtime is cloud-only.

### 3. The Integration Mess
Every AI team builds its own "run this script in Docker" wrapper. Three engineers, three different implementations, none secure, none maintained. There's no standard way for an agent to discover what skills are available, understand their interfaces, and execute them safely.

---

## The Solution

Skillbox solves all three:

```
Your Agent → Skillbox API → Skill Registry → Sandboxed Container → Structured Output + Files
```

### Security That Can't Be Turned Off

| Control | How | Threat Blocked |
|---|---|---|
| Network disabled | `NetworkMode: none` | Data exfiltration, SSRF |
| All capabilities dropped | `CapDrop: ["ALL"]` | Privilege escalation |
| Read-only filesystem | `ReadonlyRootfs: true` | Filesystem tampering |
| PID limits | `PidsLimit: 128` | Fork bombs |
| Non-root execution | `User: 65534:65534` | Container escape |
| Image allowlist | Checked before creation | Supply-chain attacks |
| Socket proxy | Sidecar with minimal perms | Host escape |
| Timeout enforcement | Go context cancellation | Resource exhaustion |

Security is enforced by the runtime. Callers can't weaken it. This is a deliberate design decision — the one that matters most.

### The Skill Catalog Abstraction

This is Skillbox's unique moat. While competitors give you `sandbox.run_code("print('hello')")`, Skillbox gives you:

```python
# Discover available capabilities
for skill in client.list_skills():
    print(f"{skill.name}: {skill.description}")

# Inspect a skill's full interface before executing
detail = client.get_skill("data-analysis", "1.0.0")
print(detail.instructions)  # Full documentation

# Execute with structured I/O
result = client.run("data-analysis", input={"data": [1, 2, 3]})
print(result.output)  # Structured JSON, not raw stdout
```

Each skill is a **versioned, self-describing unit** with YAML metadata for machines and markdown instructions for LLMs. This maps directly to LangChain tools, OpenAI function calling, and Claude tool use. Agents don't just execute code — they discover, understand, and choose the right capability.

### Deploy Anywhere

```bash
# Development (Docker Compose)
docker compose -f deploy/docker/docker-compose.yml up -d

# Production (Kubernetes)
kubectl apply -k deploy/k8s/overlays/prod

# Helm
helm install skillbox deploy/helm/skillbox/
```

Same code, same security, same API. Air-gapped? Works offline. EU data center? Data never leaves. Your Kubernetes cluster? Slots right in.

---

## Market Opportunity

### Total Addressable Market: $1.2B (2026)

| Segment | 2026 Size | 2030 Projected | CAGR |
|---|---|---|---|
| AI Agent Platforms | $10.9B | $52.6B | 45-50% |
| AI Infrastructure (broad) | $75-101B | $205-418B | 19-25% |
| AI Developer Tools | $5.3B | $10.0B | 17.3% |
| Sandboxed Execution (our niche) | **$1.0-1.5B** | **$4.5-6.0B** | **~45%** |

*Sources: Grand View Research, Markets and Markets, Fortune Business Insights, Mordor Intelligence*

### Serviceable Addressable Market: $100-150M

Filtered by self-hosted preference (57% of enterprises), US+EU geography, enterprise/professional segment, and multi-language execution fit.

### Revenue Trajectory

| Scenario | Year 1 | Year 2 | Year 3 |
|---|---|---|---|
| Bootstrapped | $50-180K | $274K-1M | $757K-2.9M |
| Seed ($1-2M) | $232K | $1.05M | $2.9-3.4M |
| Series A ($5-10M) | $650K | $3.04M | $8.9-10.5M |

---

## Competitive Landscape

### $289M+ Has Entered the Chat

| Company | Raised | Model | Self-Hosted? | Skill Catalog? |
|---|---|---|---|---|
| **E2B** | $43.8M | Cloud microVMs | Experimental | No |
| **Modal** | $80M+ | Serverless GPU/CPU | No | No |
| **Daytona** | $31M | Cloud containers | OSS (limited) | No |
| **Fly.io** | $120M+ | Global VMs | No | No |
| **Cloudflare** | Public | Edge isolates | No | No |
| **Skillbox** | Bootstrapped | **Self-hosted containers** | **Yes (MIT)** | **Yes** |

### Positioning Matrix

```
                    Structured Skills (Catalog)
                           ▲
                           │
                  Skillbox │
                     ★     │
                           │
   Self-Hosted ◄───────────┼───────────► Cloud-Only
                           │
                           │  E2B  Modal  Daytona
                           │   ●     ●      ●
                           │
                    Raw Code Execution
```

**Skillbox occupies the only uncontested quadrant.** Every funded competitor is in the lower-right: cloud-hosted raw execution. Nobody else offers self-hosted + structured skills.

### Why This Position Wins

1. **Regulatory tailwinds**: EU AI Act (August 2026), GDPR, HIPAA, FedRAMP all push workloads on-premises
2. **Enterprise preference**: 57% of enterprise AI infra is already on-prem
3. **Cloud repatriation**: 86% of CIOs planning to bring workloads back (Gartner 2025)
4. **Higher ASP**: Self-hosted enterprise contracts command 2-5x the annual value of cloud subscriptions
5. **Defensible moat**: The SKILL.md ecosystem creates network effects that raw execution can't replicate

---

## Customer Personas

### Who Buys This

| Persona | Company Size | Pain Point | Willingness to Pay |
|---|---|---|---|
| **AI Agent Engineer** | Startup (10-100) | "I need a sandbox that works, not one I have to build" | $1.2-3.6K/yr |
| **Platform Engineer** | Mid-market (200-800) | "Three teams built three sandbox wrappers. None are secure" | $6-24K/yr |
| **Enterprise Architect** | Large corp (5,000+) | "We need one security review to cover all AI agent teams" | $30-120K/yr |
| **CTO / Founder** | Startup (5-30) | "Every hour my team spends on infra is existential risk" | $1.2-3.6K/yr |

### Value Quantification

- **AI Agent Engineer**: Saves 2-4 weeks of building sandbox infrastructure = $8K-16K in engineering time
- **Platform Engineer**: Eliminates 3+ redundant sandbox implementations = $40K-160K in reduced engineering
- **Enterprise Architect**: One compliance review instead of per-team reviews = $450K-2M in avoided cost

---

## Business Model

### Open Core + Managed Cloud + Enterprise

| Tier | Price | Target | Key Features |
|---|---|---|---|
| **Community** | Free (MIT) | Everyone | Full core product, unlimited self-hosted |
| **Pro** | $49/mo + usage | Individual devs | Managed cloud, included compute, email support |
| **Team** | $149/mo + usage | Small teams | Multi-user workspace, skill analytics, registry |
| **Enterprise** | $2,500+/mo | Large orgs | SSO/SAML, RBAC, audit logs, air-gap support, SLA |

**Cloud compute**: $0.04/hr per vCPU (10-25% below E2B's $0.05/hr)

### Why MIT License Stays

The MIT license is a strategic asset, not a liability. Lessons from the industry:
- **HashiCorp** switched to BSL → OpenTofu fork, mass defection, ultimately sold to IBM
- **Redis** switched to SSPL → Valkey adopted by 83% of large users within months
- **Supabase** stayed MIT → $116M ARR, $2B valuation
- **PostHog** stayed MIT → $20M+ ARR, strongest developer community in analytics

**We will never change the MIT license.** Revenue comes from operations, collaboration, and compliance features — not from restricting the code.

---

## Go-To-Market Strategy

### Positioning

> Skillbox is the self-hosted, open-source execution runtime for AI agents — with security enforced by the runtime, not by trust. Unlike cloud-only sandboxes (E2B, Modal), Skillbox runs on your infrastructure, keeps data in your network, and gives your agents a discoverable catalog of versioned, tested capabilities instead of raw code execution.

### Phase 1: Community Seeding (Q1 2026)
- Show HN launch
- 6 technical blog posts ("How to give your AI agent safe code execution in 5 minutes")
- Discord community setup
- Target: **500 GitHub stars, 100 Discord members**

### Phase 2: Framework Integrations (Q2 2026)
- `skillbox-langchain` on PyPI
- CrewAI tool integration
- MCP server for Claude
- TypeScript SDK
- Conference talks (Interrupt, AI Engineer Summit)
- Target: **2,000 stars, 10 production deployments**

### Phase 3: Cloud & Revenue (Q3-Q4 2026)
- Skillbox Cloud beta
- Enterprise design partners (3-5 companies)
- First revenue
- Target: **5,000 stars, $10K MRR**

### Channel Strategy (Ranked by ROI)

| Tier | Channels | Cost |
|---|---|---|
| **Tier 1** (free, high ROI) | GitHub, HN, Twitter/X, AI framework Discords, technical blog |  $0 |
| **Tier 2** (low cost) | Dev.to, Reddit (r/selfhosted, r/LocalLLaMA), YouTube, Product Hunt, conferences | $500-2K/mo |
| **Tier 3** (defer) | Podcasts, paid ads, enterprise outreach | Later |

---

## Technology

### Architecture

```
┌───────────────────────────────────────────────┐
│              Agent / SDK Client                │
│          client.Run(ctx, RunRequest)           │
└───────────────────┬───────────────────────────┘
                    │ REST API
┌───────────────────▼───────────────────────────┐
│              API Server (Go / Gin)             │
│     Stateless · Horizontally Scalable          │
│  ┌──────┐ ┌────────┐ ┌─────────┐ ┌────────┐  │
│  │ Auth │ │Registry│ │ Runner  │ │Artifact│  │
│  └──────┘ └────────┘ └────┬────┘ └────────┘  │
└────────────────────────────┼──────────────────┘
                             │
┌────────────────────────────▼──────────────────┐
│        Sandboxed Container (hardened)          │
│  net=none · cap=drop ALL · ro rootfs           │
│  PID=128 · no-new-privs · user=nobody          │
└───────────────────────────────────────────────┘
     │              │              │
┌────▼────┐  ┌──────▼──────┐  ┌───▼───┐
│Postgres │  │ MinIO / S3  │  │ Redis │
│metadata │  │skills+files │  │ cache │
└─────────┘  └─────────────┘  └───────┘
```

### Tech Stack

| Component | Technology | Why |
|---|---|---|
| API Server | Go + Gin | Single binary, fast, stdlib-only SDKs |
| Container Execution | Docker SDK for Go | Universal runtime, Kubernetes-native |
| Skill Storage | MinIO / S3-compatible | Versioned, tenant-isolated, presigned URLs |
| Metadata | PostgreSQL | ACID, migrations, battle-tested |
| Cache | Redis (optional) | Graceful degradation if absent |
| Security | Docker socket proxy | API never touches daemon directly |
| Deployment | Docker Compose / Kubernetes / Helm | Dev ↔ Prod parity |

### What Makes It Hard to Replicate

1. **Defense-in-depth security** — 11 layers of container hardening, not configurable by callers
2. **SKILL.md format** — A protocol for agent capabilities, not just a runtime for code
3. **Zero-dependency SDKs** — Go and Python clients use only stdlib, no version conflicts
4. **Multi-tenant isolation** — API keys scoped to tenants, skills and executions fully isolated
5. **Socket proxy pattern** — API server never has direct Docker daemon access

---

## Traction & Validation

### Market Validation Signals

- **E2B's trajectory**: $1.5M revenue at 18 months with 15 employees. 88% of Fortune 100 adopted. Proves the market exists and is willing to pay.
- **$289M in competitor funding**: VCs have validated the market thesis at scale.
- **EU AI Act deadline**: August 2026. Enterprises are actively seeking compliant agent infrastructure right now.
- **Agent framework explosion**: LangChain (47M+ downloads), CrewAI (powering 60% of Fortune 500), AutoGen, OpenAI Agents SDK — all need execution backends.
- **Cloud repatriation**: 86% of CIOs planning to bring workloads back on-premises (Gartner 2025).

### What's Built

- Full API server with 11-layer security hardening
- Go SDK (single-file, stdlib-only)
- Python SDK (single-file, stdlib-only)
- LangChain integration
- CLI tool (push, lint, run, package skills)
- Docker Compose deployment
- Kubernetes manifests + Helm chart
- CI/CD with GoReleaser
- Comprehensive E2E test suite
- Example skills (data analysis, text summary, word counting, PDF extraction)

---

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Well-funded competitor adds self-hosting | Medium | High | Ship faster, build SKILL.md ecosystem moat, community lock-in |
| Hyperscaler enters (AWS/GCP sandbox service) | Medium | Severe | Open-source positioning, multi-cloud, avoid vendor lock-in |
| AI agent market hype correction | Medium | Medium | Focus on production use cases, not demos |
| Docker isolation limitations (vs microVMs) | Low | Medium | Support gVisor/Kata as optional Kubernetes RuntimeClass |
| Open-source sustainability (burnout, funding) | Medium | High | Dual-track: community + commercial. Seed funding in 2026. |

### The 12-18 Month Window

The competitive analysis reveals a clear finding: **Skillbox has the right architecture at the right time, but the window to convert differentiation into market position is 12-18 months** before funded competitors close the self-hosted gap. E2B's self-hosted offering is still "experimental." Daytona's open-source is limited. This window won't last.

---

## Financial Summary

### Unit Economics (Target)

| Metric | Value | Benchmark |
|---|---|---|
| Gross margin (cloud) | 65-75% | Industry: 70-80% |
| Gross margin (enterprise) | 85-90% | Industry: 80-90% |
| CAC (PLG/OSS) | $500-2,000 | Industry: $1,000-5,000 |
| LTV (Pro) | $2,500-4,000 | 3-year lifetime |
| LTV (Enterprise) | $75,000-300,000 | 3-year contract |
| LTV:CAC (Enterprise) | 15-30x | Healthy: >3x |
| Payback period | 2-6 months | Industry: 12-18 months |

### Why This Is Capital-Efficient

1. **OSS community does the top-of-funnel**: No paid acquisition needed for developer awareness
2. **Self-hosted = no infrastructure cost for free tier**: Unlike cloud-first competitors, free users don't burn compute
3. **Enterprise contracts are high-ASP, low-volume**: 10-30 enterprise customers = $250K-$900K ARR
4. **Swiss engineering reputation**: Trust signal for EU enterprise buyers, no additional marketing needed
5. **MIT license attracts contributors**: Community builds skills, integrations, and docs for free

---

## The Ask

### What We Need

| Phase | Capital | Use | Timeline |
|---|---|---|---|
| **Now** | $0 (bootstrapped) | Community, DX, first 500 stars | Q1 2026 |
| **Seed** | $1-2M | Cloud MVP, 2-3 engineers, first enterprise deals | Q2-Q3 2026 |
| **Series A** | $5-10M | Scale cloud, sales team, framework integrations | 2027 |

### What Investors Get

- **The only open-source self-hosted play** in a $1.2B market growing 45%+ annually
- **Regulatory tailwinds that accelerate** (EU AI Act, GDPR, data sovereignty)
- **Enterprise pricing power** ($30K-120K/yr per customer) with OSS distribution economics
- **A 12-18 month head start** on the self-hosted + structured skills positioning
- **Capital-efficient model**: No free-tier compute costs, high-ASP enterprise contracts
- **Swiss-based team**: EU credibility, engineering quality, data protection trust

---

## Key Metrics to Watch

| Metric | Month 3 | Month 6 | Month 12 |
|---|---|---|---|
| GitHub Stars | 500 | 2,000 | 5,000 |
| Discord Members | 100 | 500 | 2,000 |
| Production Deployments | 3 | 10 | 50 |
| Enterprise Pilots | 0 | 3 | 10 |
| MRR | $0 | $2K | $10K |
| Community Skills Published | 5 | 20 | 100 |
| Framework Integrations | 1 | 4 | 6+ |

---

## One-Liner for Every Audience

| Audience | Message |
|---|---|
| **AI Engineer** | Stop building sandbox infrastructure. Start building agents. |
| **Platform Engineer** | Give your AI team a self-service execution platform. Stop being the bottleneck. |
| **Enterprise Architect** | One security review. All teams covered. |
| **CTO / Founder** | Your agents need a sandbox. Don't build one. |
| **Investor** | The self-hosted E2B, in a $1.2B market that's 100% cloud-only today. |
| **Open Source Developer** | The skill runtime your agent framework is missing. MIT licensed. |

---

## Appendix: Research Methodology

This document is synthesized from 8 independent McKinsey-grade analyses conducted on February 26, 2026:

1. **TAM/SAM/SOM Market Sizing** — Top-down and bottom-up triangulation
2. **Competitive Intelligence** — 12 competitors profiled, $289M+ in funding mapped
3. **Customer Personas** — 4 personas with value quantification
4. **Market Trends** — PESTEL analysis with 60+ cited sources
5. **SWOT + Porter's Five Forces** — Strategic framework analysis
6. **Pricing Strategy** — Competitive benchmarking, value-based analysis, tier design
7. **Go-To-Market Strategy** — Channel analysis, launch plan, partnership mapping
8. **Customer Journey** — 7-stage funnel mapping with conversion benchmarks

Full analysis files available in `artifacts/research/skillbox/`.

Sources include: Grand View Research, Gartner, Forrester, Mordor Intelligence, Crunchbase, Precedence Research, Fortune Business Insights, OWASP, EU AI Act documentation, and direct competitor data from E2B, Modal, Daytona, and others.

---

*Built by [devs group](https://devs-group.com) · Kreuzlingen, Switzerland*
*MIT License · Open Source · Production Ready*
