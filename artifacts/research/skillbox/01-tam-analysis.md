# Skillbox TAM/SAM/SOM Market Sizing Analysis

**Date:** February 2026
**Product:** Skillbox -- Secure Skill Execution Runtime for AI Agents
**Analyst Framework:** Top-down market sizing with bottom-up validation
**Confidence Level:** Medium-High (based on triangulated public data)

---

## Executive Summary

Skillbox operates at the intersection of three rapidly converging markets: AI infrastructure, AI agent platforms, and developer tools. The product -- a self-hosted, Docker-native, Kubernetes-ready sandboxed code execution runtime for AI agents -- addresses a critical gap in the AI agent stack: secure, structured execution of arbitrary code (Python, Node.js, Bash) with JSON output and file artifact support.

The total addressable market for AI infrastructure is projected at $75-100B in 2026, but Skillbox targets a specific niche: **sandboxed code execution for AI agents**. This niche is nascent but growing explosively, validated by E2B's trajectory ($1.5M revenue in mid-2025, "seven figures" in new monthly business by late 2025, and adoption by 88% of the Fortune 100). Skillbox differentiates through self-hosting, data sovereignty, and air-gapped deployment -- a positioning that aligns with regulatory tailwinds from the EU AI Act, GDPR, and increasing enterprise demand for on-premises AI infrastructure.

---

## 1. Total Addressable Market (TAM)

### 1.1 Market Definition

The TAM represents the total global revenue opportunity if Skillbox captured 100% of the market for AI infrastructure software related to agent execution, sandboxing, and developer tooling for AI systems.

We define Skillbox's TAM as the intersection of three market categories:

| Market Category | 2025 Size | 2026 Size | 2030 Projected | CAGR | Source |
|---|---|---|---|---|---|
| AI Infrastructure (broad) | $58.8-158.3B | $75.4-101.2B | $205-418B | 19-25% | [Fortune Business Insights](https://www.fortunebusinessinsights.com/ai-infrastructure-market-110456), [BCC Research](https://www.bccresearch.com/market-research/artificial-intelligence-technology/ai-infrastructure-market.html), [Mordor Intelligence](https://www.mordorintelligence.com/industry-reports/ai-infrastructure-market) |
| AI Agents Market | $7.6B | $10.9B | $52.6B | 45-50% | [Grand View Research](https://www.grandviewresearch.com/industry-analysis/ai-agents-market-report), [Markets and Markets](https://www.marketsandmarkets.com/Market-Reports/ai-agents-market-15761548.html) |
| AI Developer Tools | $4.5B | ~$5.3B | $10.0B | 17.3% | [Virtue Market Research](https://virtuemarketresearch.com/report/ai-developer-tools-market) |
| Software Dev Tools (broad) | $7.5B | $8.8B | $13.7B | 16.4% | [Mordor Intelligence](https://www.mordorintelligence.com/industry-reports/software-development-tools-market) |
| Container & K8s Security | $2.3-7.6B | $2.8-9.3B | $7.5-14.0B | 21-27% | [IMARC Group](https://www.imarcgroup.com/container-kubernetes-security-market), [Straits Research](https://straitsresearch.com/report/container-and-kubernetes-security-market) |

### 1.2 TAM Calculation

Skillbox's TAM sits at the intersection of "AI agent runtime infrastructure" and "secure code execution platforms." To size this, we use a composite approach:

**Approach A: AI Agent Infrastructure Spend (Top-Down)**

The AI agents market is projected at $10.9B in 2026. Infrastructure and tooling (execution runtimes, orchestration, monitoring) typically represents 25-35% of platform spend in enterprise software markets.

> TAM (A) = $10.9B x 30% = **$3.3B** (2026)

**Approach B: Developer Tooling for AI (Bottom-Up)**

- Global developer population: ~47 million ([SlashData, 2025](https://www.slashdata.co/post/global-developer-population-trends-2025-how-many-developers-are-there))
- Professional developers: ~36.5 million
- Developers working with AI/ML: estimated 15-20% = 5.5-7.3 million
- Developers building AI agent systems: estimated 5-8% of AI/ML developers = 275K-585K
- Average annual tooling spend per developer seat: $1,200-$3,600/year (based on E2B Pro at $150/mo = $1,800/yr; enterprise tiers at $3,000+/yr)

> TAM (B) = 430K developers (midpoint) x $2,400/yr (midpoint) = **$1.03B** (2026)

**Approach C: Sandboxed Execution as % of Container Security Market**

The container and Kubernetes security market is $2.8B+ in 2026. Runtime protection is a major segment (~$1.8B in 2024). Sandboxed execution for AI agents represents a growing sub-segment estimated at 15-25% of runtime protection.

> TAM (C) = $2.2B (2026 runtime protection estimate) x 20% = **$440M** (2026)

### 1.3 TAM Summary

| Method | 2026 TAM Estimate | Notes |
|---|---|---|
| AI Agent Infrastructure (Top-Down) | $3.3B | Broad; includes orchestration, monitoring, execution |
| Developer Tooling (Bottom-Up) | $1.0B | Focused on execution runtime seats |
| Container Security Sub-Segment | $440M | Narrowest; runtime sandboxing only |
| **Blended TAM Estimate** | **$1.0 - $1.5B** | Weighted toward bottom-up as most relevant |

We adopt a **blended TAM of $1.2B for 2026**, representing the global market for secure code execution runtimes specifically serving AI agent workloads, inclusive of both cloud-hosted and self-hosted deployment models.

---

## 2. Serviceable Addressable Market (SAM)

### 2.1 Filtering Criteria

SAM narrows the TAM to the portion Skillbox can realistically serve given its product architecture, go-to-market, and positioning:

| Filter | Rationale | TAM Reduction |
|---|---|---|
| **Self-hosted / on-prem requirement** | Skillbox is Docker-native and self-hosted. On-premises held 57.46% of AI infrastructure spending in 2025 ([Precedence Research](https://www.precedenceresearch.com/artificial-intelligence-infrastructure-market)). Not all self-hosted buyers need sandboxing, but this is Skillbox's core differentiator vs. E2B/Modal. | Focus on ~40% of TAM |
| **Geographic fit** | Strong fit for EU (GDPR, EU AI Act, data sovereignty), US (AI hub), and air-gapped enterprises globally. EU + US represent ~75% of AI infrastructure spend. | Keep ~75% |
| **Customer segment** | Target: AI/ML engineers building agent systems, DevOps teams, enterprises with compliance needs. Excludes hobbyist/individual developers, pure-cloud-native teams satisfied with E2B. | Keep ~60% |
| **Product fit** | Must need sandboxed multi-language execution (Python, Node.js, Bash) with structured JSON output. Excludes teams using simple Docker exec, or teams needing only Python notebooks. | Keep ~50% |

### 2.2 SAM Calculation

Starting from the $1.2B blended TAM:

> SAM = $1.2B x 40% (self-hosted preference) x 75% (US + EU geography) x 60% (enterprise/pro segment) x 50% (product fit)

> **SAM = $108M (2026)**

### 2.3 SAM Validation: Bottom-Up

- Enterprises actively deploying AI agents in production: ~13.2% of organizations as of December 2025 ([Multimodal.dev](https://www.multimodal.dev/post/agentic-ai-statistics)), trending toward 25%+ by end of 2026
- Fortune 500 companies: 500 firms; ~60% using agent frameworks like CrewAI ([Arsum](https://arsum.com/blog/posts/ai-agent-frameworks/))
- Estimated enterprises needing self-hosted sandboxed execution: 2,000-5,000 globally
- Average contract value for infrastructure tooling: $20,000-$50,000/year
- Mid-market / scale-up companies (1,000-10,000 employees) with AI teams: ~10,000 globally
- Average spend: $5,000-$15,000/year

> Bottom-up SAM = (3,500 enterprises x $35K) + (10,000 mid-market x $10K) = $122.5M + $100M = **$222.5M**

The bottom-up estimate is higher because it includes some cloud-deployed scenarios. Adjusting for the self-hosted filter, we converge on:

> **SAM = $100-150M (2026)**

### 2.4 SAM by Segment

| Segment | Description | Estimated SAM | % of Total |
|---|---|---|---|
| Regulated Enterprise | Finance, healthcare, government, defense -- need air-gapped/on-prem | $50-60M | ~45% |
| EU Data Sovereignty | Companies requiring GDPR/AI Act compliant self-hosted AI infra | $25-35M | ~25% |
| AI-Native Startups | Startups building AI agent products, prefer OSS + self-hosted | $15-25M | ~15% |
| DevOps / Platform Teams | Teams standardizing agent execution across orgs | $10-20M | ~15% |

---

## 3. Serviceable Obtainable Market (SOM)

### 3.1 Assumptions

SOM estimates what Skillbox can realistically capture in its current stage:

| Factor | Assumption | Rationale |
|---|---|---|
| Company stage | Pre-seed / early-stage OSS project | No significant revenue yet |
| Team size | Small (1-5) | Typical for OSS infrastructure projects |
| Go-to-market | Open-source led, bottom-up adoption | Similar to E2B's early trajectory |
| Competitive landscape | E2B ($21M Series A, $1.5M+ revenue), Modal, Northflank, Cloudflare, Daytona | Crowded but fast-growing |
| Differentiation | Self-hosted, data sovereignty, air-gapped support, Docker-native | Unique positioning vs cloud-only competitors |
| Sales cycle | 1-3 months for mid-market, 6-12 months for enterprise | Standard for infrastructure software |

### 3.2 SOM Calculation

**Year 1 (2026):**
- OSS adoption: 500-2,000 active installations
- Paying customers (enterprise support / premium features): 10-30
- Average revenue per customer: $8,000-$15,000/year
- Estimated revenue: $80K-$450K

> **Year 1 SOM = $150K-$450K** (0.1-0.4% of SAM)

**Year 2 (2027):**
- Growing OSS community, LangChain/CrewAI integrations driving adoption
- Paying customers: 50-150
- Average revenue per customer: $12,000-$25,000/year (mix shifting toward enterprise)
- Estimated revenue: $600K-$3.75M

> **Year 2 SOM = $1M-$3.5M** (0.7-2.5% of SAM)

**Year 3 (2028):**
- Established OSS brand, enterprise pipeline maturing
- Paying customers: 150-500
- Average revenue per customer: $18,000-$40,000/year
- Estimated revenue: $2.7M-$20M

> **Year 3 SOM = $5M-$15M** (2.5-7.5% of SAM)

### 3.3 SOM Benchmark: E2B Trajectory

E2B provides a useful benchmark as the closest comparable:

| Metric | E2B | Skillbox (Projected) |
|---|---|---|
| Revenue at ~18 months | $1.5M (June 2025) | $150K-$450K (conservative, self-hosted) |
| Funding | $35M total (Seed + Series A) | Pre-seed / bootstrapped |
| Go-to-market | Cloud-hosted SaaS + OSS | Self-hosted OSS + enterprise |
| Fortune 100 adoption | 88% signed up | N/A (different target) |
| Team size | 14 people | 1-5 people |

E2B's revenue growth accelerated dramatically after its Series A -- adding "seven figures" monthly by late 2025 ([VentureBeat](https://venturebeat.com/ai/how-e2b-became-essential-to-88-of-fortune-100-companies-and-raised-21-million/)). Skillbox's self-hosted model will likely show slower initial revenue (no usage-based cloud billing) but potentially stronger enterprise ASP and retention.

### 3.4 SOM Summary Table

| Year | Low Estimate | Mid Estimate | High Estimate | % of SAM |
|---|---|---|---|---|
| 2026 (Year 1) | $150K | $250K | $450K | 0.1-0.4% |
| 2027 (Year 2) | $1.0M | $2.0M | $3.5M | 0.7-2.5% |
| 2028 (Year 3) | $5.0M | $8.0M | $15.0M | 2.5-7.5% |

---

## 4. Growth Projections

### 4.1 Three-Year Outlook (2026-2028)

| Metric | 2026 | 2027 | 2028 | CAGR |
|---|---|---|---|---|
| AI Agents Market | $10.9B | $16.3B | $24.3B | ~49% |
| AI Infrastructure Market | $75.4B | $90.5B | $108.6B | ~20% |
| Sandboxed Execution Niche (est.) | $1.2B | $1.9B | $3.0B | ~58% |
| Container & K8s Security | $2.8B | $3.4B | $4.2B | ~22% |

### 4.2 Five-Year Outlook (2026-2030)

| Metric | 2026 | 2028 | 2030 | CAGR |
|---|---|---|---|---|
| AI Agents Market | $10.9B | $24.3B | $52.6B | 48.3% |
| AI Infrastructure Market | $75.4B | $108.6B | $205-418B | 19-25% |
| Sandboxed Execution Niche (est.) | $1.2B | $3.0B | $7.5B | ~58% |
| Skillbox SAM | $120M | $280M | $650M | ~52% |

### 4.3 Growth Rate Sources

| Market | CAGR | Period | Source |
|---|---|---|---|
| AI Agents | 45-50% | 2025-2030 | [Grand View Research](https://www.grandviewresearch.com/industry-analysis/ai-agents-market-report), [Markets and Markets](https://www.marketsandmarkets.com/Market-Reports/ai-agents-market-15761548.html) |
| AI Infrastructure | 19-25% | 2025-2030 | [BCC Research](https://www.bccresearch.com/market-research/artificial-intelligence-technology/ai-infrastructure-market.html), [Markets and Markets](https://www.marketsandmarkets.com/Market-Reports/ai-infrastructure-market-38254348.html) |
| AI Developer Tools | 17.3% | 2025-2030 | [Virtue Market Research](https://virtuemarketresearch.com/report/ai-developer-tools-market) |
| Container & K8s Security | 21-27% | 2025-2033 | [IMARC Group](https://www.imarcgroup.com/container-kubernetes-security-market), [Straits Research](https://straitsresearch.com/report/container-and-kubernetes-security-market) |
| Sovereign Cloud | ~27% | 2025-2032 | [AI Barcelona](https://www.aibarcelona.org/2026/01/sovereign-cloud-europe-hyperscalers-ai-infrastructure.html) |

---

## 5. Market Drivers

### Top 5 Factors Driving Market Growth

#### 1. Explosive Growth in AI Agent Adoption

AI agent production deployments nearly doubled in four months (7.2% to 13.2% of organizations, August to December 2025). Gartner forecasts 33% of enterprise software will incorporate agentic AI by 2028, up from <1% in 2024. Every AI agent that needs to execute code requires a secure runtime, directly expanding Skillbox's market.

- **Source:** [Multimodal.dev AI Agent Statistics](https://www.multimodal.dev/post/agentic-ai-statistics), [Warmly AI Agent Statistics](https://www.warmly.ai/p/blog/ai-agents-statistics)

#### 2. Regulatory Tailwinds: EU AI Act, GDPR, and Data Sovereignty

The EU AI Act reaches full implementation in August 2026. 61% of Western European CIOs plan to increase reliance on local cloud/AI providers. 52% of Western European enterprises are accelerating investment in data sovereignty initiatives. Self-hosted, air-gapped solutions like Skillbox directly address this demand -- a segment underserved by cloud-only competitors like E2B and Modal.

- **Source:** [Gartner 2025 CIO Survey via AI Barcelona](https://www.aibarcelona.org/2026/01/sovereign-cloud-europe-hyperscalers-ai-infrastructure.html), [Secure Privacy](https://secureprivacy.ai/blog/data-privacy-trends-2026)

#### 3. Maturation of AI Agent Frameworks

The AI agent framework ecosystem has matured rapidly: LangChain has 47M+ PyPI downloads, CrewAI powers agents for 60% of Fortune 500 companies, and agent framework repos with 1,000+ GitHub stars grew 535% (14 to 89) from 2024 to 2025. This creates a large and growing base of developers who need execution runtimes for their agents.

- **Source:** [Arsum AI Agent Frameworks](https://arsum.com/blog/posts/ai-agent-frameworks/), [AlphaMatch](https://www.alphamatch.ai/blog/top-agentic-ai-frameworks-2026)

#### 4. Enterprise Demand for On-Premises AI Infrastructure

On-premises architectures held 57.46% of AI infrastructure market share in 2025, driven by data-residency mandates and sector-specific regulations (HIPAA, PCI-DSS, ITAR). Regulated industries (finance, healthcare, defense, government) represent a large and underserved market for self-hosted AI agent execution runtimes.

- **Source:** [Precedence Research](https://www.precedenceresearch.com/artificial-intelligence-infrastructure-market), [Mordor Intelligence](https://www.mordorintelligence.com/industry-reports/ai-infrastructure-market)

#### 5. Security Concerns with AI-Generated Code Execution

As AI agents generate and execute increasingly complex code, the security implications are significant. Container runtime protection tool deployments grew 48% globally. The need to sandbox untrusted AI-generated code in isolated environments is becoming a core infrastructure requirement, not a nice-to-have. Cursor alone produces nearly a billion lines of accepted code daily, underscoring the scale of code execution that needs sandboxing.

- **Source:** [IMARC Group](https://www.imarcgroup.com/container-kubernetes-security-market), [Northflank Sandbox Comparison](https://northflank.com/blog/best-code-execution-sandbox-for-ai-agents)

---

## 6. Competitive Landscape

### Direct Competitors

| Company | Model | Funding | Key Differentiator | Weakness vs. Skillbox |
|---|---|---|---|---|
| **E2B** | Cloud SaaS + OSS | $35M (Series A, Jul 2025) | Firecracker microVMs, 88% Fortune 100 adoption | Cloud-only; no self-hosted option for air-gapped/sovereign needs |
| **Modal** | Cloud SaaS | $113M+ | Python-centric ML workflows, massive autoscaling | No BYOC or on-prem; vendor lock-in |
| **Northflank** | Cloud + BYOC | Undisclosed | Kata Containers/gVisor, 2M+ monthly workloads | Broader platform, not agent-specific |
| **Daytona** | OSS + Cloud | $10M+ | Fastest cold starts (sub-90ms) | Docker containers = weaker isolation |
| **Cloudflare Sandbox** | Cloud SaaS | N/A (Cloudflare division) | Workers integration, global edge | Tied to Cloudflare ecosystem |

### Skillbox Competitive Positioning

Skillbox's unique positioning is the intersection of:
1. **Self-hosted / air-gapped** -- No cloud dependency
2. **Multi-language** -- Python, Node.js, Bash in a single runtime
3. **Structured output** -- JSON + file artifacts (not just stdout)
4. **Docker-native + K8s-ready** -- Fits existing enterprise infrastructure
5. **AI-agent-specific** -- Purpose-built for agent workflows

This positions Skillbox in a defensible niche that cloud-only competitors cannot easily address without fundamentally changing their architecture.

---

## 7. Risks and Caveats

| Risk | Impact | Mitigation |
|---|---|---|
| Market timing | The sandboxed execution market is nascent; adoption could be slower than projected | Strong OSS community building reduces dependency on paid conversion timing |
| E2B dominance | E2B's funding and Fortune 100 adoption create network effects | Skillbox targets a different buyer (self-hosted/sovereign) -- complementary, not head-to-head |
| Build vs. buy | Some enterprises may build internal sandboxing solutions | Complexity of multi-language sandboxing + security makes buy more attractive over time |
| Cloud shift | If on-prem AI infra share declines faster than expected, SAM shrinks | Hybrid deployment support can bridge both models |
| Regulatory uncertainty | Changes to AI regulation could affect market dynamics | Self-hosted positioning is inherently regulation-friendly |

---

## 8. Data Sources

All market sizing figures are sourced from publicly available industry reports and press releases:

| Source | Used For | URL |
|---|---|---|
| Grand View Research | AI Agents market size, Container security | [Link](https://www.grandviewresearch.com/industry-analysis/ai-agents-market-report) |
| Markets and Markets | AI Agents, AI Infrastructure | [Link](https://www.marketsandmarkets.com/Market-Reports/ai-agents-market-15761548.html) |
| Fortune Business Insights | AI Infrastructure, Agentic AI | [Link](https://www.fortunebusinessinsights.com/ai-infrastructure-market-110456) |
| BCC Research | AI Infrastructure | [Link](https://www.bccresearch.com/market-research/artificial-intelligence-technology/ai-infrastructure-market.html) |
| Mordor Intelligence | AI Infrastructure, Dev Tools | [Link](https://www.mordorintelligence.com/industry-reports/ai-infrastructure-market) |
| Precedence Research | AI Infrastructure (on-prem share), Agentic AI | [Link](https://www.precedenceresearch.com/artificial-intelligence-infrastructure-market) |
| Virtue Market Research | AI Developer Tools | [Link](https://virtuemarketresearch.com/report/ai-developer-tools-market) |
| IMARC Group | Container & K8s Security | [Link](https://www.imarcgroup.com/container-kubernetes-security-market) |
| Straits Research | Container & K8s Security | [Link](https://straitsresearch.com/report/container-and-kubernetes-security-market) |
| SlashData | Global developer population | [Link](https://www.slashdata.co/post/global-developer-population-trends-2025-how-many-developers-are-there) |
| VentureBeat | E2B funding and adoption | [Link](https://venturebeat.com/ai/how-e2b-became-essential-to-88-of-fortune-100-companies-and-raised-21-million/) |
| GetLatka | E2B revenue data | [Link](https://getlatka.com/companies/e2b.dev) |
| Multimodal.dev | AI Agent deployment statistics | [Link](https://www.multimodal.dev/post/agentic-ai-statistics) |
| AI Barcelona | Sovereign cloud market | [Link](https://www.aibarcelona.org/2026/01/sovereign-cloud-europe-hyperscalers-ai-infrastructure.html) |
| Arsum | AI Agent frameworks comparison | [Link](https://arsum.com/blog/posts/ai-agent-frameworks/) |
| Deloitte | Enterprise AI adoption | [Link](https://www.deloitte.com/global/en/issues/generative-ai/state-of-ai-in-enterprise.html) |
| Warmly | AI Agent adoption statistics | [Link](https://www.warmly.ai/p/blog/ai-agents-statistics) |
| Northflank | Sandbox platform comparison | [Link](https://northflank.com/blog/best-code-execution-sandbox-for-ai-agents) |

---

## Appendix: Key Metrics at a Glance

| Metric | Value |
|---|---|
| **TAM (2026)** | **$1.2B** |
| **SAM (2026)** | **$100-150M** |
| **SOM Year 1 (2026)** | **$150K-$450K** |
| **SOM Year 3 (2028)** | **$5M-$15M** |
| **AI Agents Market CAGR** | **45-50%** |
| **Sandboxed Execution Niche CAGR (est.)** | **~58%** |
| **On-Prem AI Infra Share (2025)** | **57.46%** |
| **EU CIOs Increasing Local AI Infra** | **61%** |
| **AI Agent Production Deployment Rate** | **13.2% (Dec 2025), doubling every ~4 months** |
| **Global Developer Population** | **47M total, ~36.5M professional** |
| **E2B Benchmark Revenue** | **$1.5M at 18 months (cloud SaaS model)** |

---

*This analysis was prepared using publicly available market research data as of February 2026. All projections involve uncertainty and should be updated quarterly as new data becomes available. The sandboxed code execution market for AI agents is nascent and evolving rapidly; estimates for this specific niche are derived from adjacent market data and analyst judgment rather than direct market reports.*
