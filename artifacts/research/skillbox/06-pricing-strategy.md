# Skillbox Pricing Strategy

**Product:** Skillbox -- Secure skill execution runtime for AI agents
**Category:** AI Infrastructure / Developer Tools
**Date:** February 26, 2026
**Analyst framework:** Competitive benchmarking, value-based pricing, open-source monetization best practices
**Confidence level:** Medium-High (based on public competitor pricing data, market sizing from prior analyses, and open-source business model precedents)

---

## Executive Summary

Skillbox is an open-source (MIT), self-hosted, Docker-native skill execution runtime for AI agents. It has no commercial offering today. This document recommends a monetization strategy that preserves Skillbox's open-source identity while building a sustainable revenue engine across three vectors: **(1) a managed cloud offering** with usage-based pricing, **(2) an enterprise self-hosted tier** with premium features and support, and **(3) a team/professional tier** bridging the gap.

The recommended pricing architecture follows the "open core + managed cloud" hybrid model proven by GitLab, Supabase, and PostHog. The core open-source product remains fully functional under MIT. Revenue comes from operational convenience (managed cloud), collaboration and governance features (team tier), and compliance/security hardening (enterprise tier).

Key findings:
- The competitive price band for AI agent sandboxes is **$0.05-$0.15/hr per sandbox** for usage-based cloud, **$0-$150/mo** for developer plans, and **$3,000-$10,000+/mo** for enterprise.
- Skillbox's self-hosted positioning creates unique pricing leverage: customers who need data sovereignty will pay a premium for on-prem enterprise features that competitors simply do not offer.
- Conservative Year 1 revenue target: **$180K-$360K ARR**. Year 3 target under the recommended scenario: **$2.4M-$4.8M ARR**.
- The MIT license should be preserved. License changes (BSL, SSPL) destroy more value than they protect, as proven by the HashiCorp and Redis controversies.

---

## Table of Contents

1. [Pricing Model Assessment](#1-pricing-model-assessment)
2. [Competitive Pricing Benchmark](#2-competitive-pricing-benchmark)
3. [Value-Based Pricing Analysis](#3-value-based-pricing-analysis)
4. [Recommended Pricing Tiers](#4-recommended-pricing-tiers)
5. [Price Sensitivity Analysis](#5-price-sensitivity-analysis)
6. [Open Source Monetization Strategy](#6-open-source-monetization-strategy)
7. [Revenue Projections](#7-revenue-projections)
8. [Implementation Roadmap](#8-implementation-roadmap)
9. [Sources](#9-sources)

---

## 1. Pricing Model Assessment

### 1.1 Model Comparison Matrix

| Model | Description | Pros | Cons | Fit for Skillbox |
|-------|-------------|------|------|-----------------|
| **Open Core** | Free community edition + paid features in proprietary tiers | Proven at scale (GitLab, Supabase); preserves OSS community; clear upgrade path | Requires careful feature gating; community may resent paywalled features | **High** -- natural fit for self-hosted product |
| **Usage-Based (Cloud)** | Pay-per-second/minute of sandbox execution time | Aligns cost with value; low barrier to entry; scales with customer growth | Requires managed infrastructure investment; unpredictable revenue; complex billing | **High** -- essential for managed cloud offering |
| **Enterprise License** | Annual contract with SLA, support, and premium features | High ACV; predictable revenue; aligns with enterprise procurement | Long sales cycles; requires sales team; high customer acquisition cost | **Medium-High** -- important for Year 2+ |
| **Managed Cloud (PaaS)** | Fully hosted Skillbox with operational guarantees | Highest convenience value; recurring revenue; controls the full experience | Significant infrastructure cost; competes with self-hosted value prop | **High** -- primary revenue driver for non-enterprise |
| **Support-Only** | Free product, paid support/consulting | Low engineering overhead; works with pure OSS | Low margins; does not scale; creates perverse incentive (complex product = more support) | **Low** -- supplementary only |
| **Dual License (AGPL + Commercial)** | AGPL for open source, commercial license for proprietary use | Protects against cloud providers; strong copyleft leverage | Alienates contributors; legally complex; recent controversies (MongoDB, Redis) | **Low** -- MIT is a strategic asset, do not change |

### 1.2 Recommended Model: Hybrid Open Core + Managed Cloud + Enterprise

The recommended approach combines three revenue streams:

1. **Managed Cloud (Skillbox Cloud):** Usage-based pricing for developers who want zero-ops sandbox execution. This is the primary growth engine and the easiest path to initial revenue.

2. **Open Core (Self-Hosted Tiers):** The MIT-licensed core remains free. A "Team" tier adds collaboration, observability, and governance features. An "Enterprise" tier adds SSO/SAML, audit logging, RBAC, compliance certifications, and dedicated support.

3. **Enterprise Contracts:** Annual agreements for large organizations requiring custom deployment, SLAs, and compliance guarantees.

This mirrors the proven playbook of Supabase (free self-host + paid cloud at $25-$599/mo + enterprise), GitLab (free + Premium $29/user/mo + Ultimate $99/user/mo), and PostHog (free self-host + usage-based cloud + enterprise).

### 1.3 Competitor Model Comparison

| Company | Primary Model | Self-Hosted Free? | Cloud Tier | Enterprise |
|---------|--------------|-------------------|------------|------------|
| **E2B** | Usage-based cloud | No (experimental) | $0-$150+/mo + usage | $3,000/mo min |
| **Modal** | Usage-based cloud | No | $0-usage + $30/mo credit | Custom |
| **Daytona** | Usage-based cloud + OSS | Yes (open source) | $0 + $200 free credit | Custom |
| **Fly.io** | Usage-based cloud | No | Pay-as-you-go | Custom |
| **GitLab** | Open core | Yes | Free-$99/user/mo | Custom |
| **Supabase** | Open core + cloud | Yes | $0-$599/mo | Custom |
| **PostHog** | Usage-based + OSS | Yes (MIT) | Free + usage-based | Custom |
| **Skillbox (proposed)** | Open core + cloud | Yes (MIT) | $0-$149/mo + usage | $2,500/mo min |

---

## 2. Competitive Pricing Benchmark

### 2.1 Direct Competitor Pricing Table

| Feature / Metric | E2B | Daytona | Modal | Fly.io | CodeSandbox SDK |
|-----------------|-----|---------|-------|--------|-----------------|
| **Free Tier** | $100 one-time credit | $200 free credit | $30/mo credit | Limited free VMs | 40 hrs/mo (Pico VMs) |
| **Base Sandbox Cost** | ~$0.05/hr (1 vCPU) | ~$0.067/hr (1 vCPU, 1 GiB) | ~$0.047/hr (1 CPU core) | ~$0.003/hr (shared 256MB) | $0.15/hr (VM) |
| **Pro/Team Plan** | $150/mo + usage | Not disclosed | $0 platform + usage | Pay-as-you-go | $15/mo + credits |
| **Enterprise Minimum** | $3,000/mo | Custom | Custom | Custom | Custom |
| **Billing Granularity** | Per second | Per second | Per second | Per second | Per hour (credits) |
| **Max Session Duration** | 1hr (free) / 24hr (pro) | Not disclosed | No hard limit | No hard limit | No hard limit |
| **Concurrent Sandboxes** | 20 (free) / higher (pro) | Not disclosed | Based on plan | Based on plan | Unlimited |
| **Storage Included** | 10GB (free) / 20GB (pro) | Included (archived pricing) | Ephemeral | $0.15/GB/mo | Included |
| **Self-Hosted Option** | Experimental only | Yes (open source) | No | No | No |
| **Isolation Model** | Firecracker microVMs | OCI/Docker containers | Containers (gVisor) | Firecracker microVMs | MicroVMs |

Sources: [E2B Pricing](https://e2b.dev/pricing), [E2B Pricing Estimator](https://pricing.e2b.dev/), [Modal Pricing](https://modal.com/pricing), [Fly.io Pricing](https://fly.io/pricing/), [Daytona Pricing](https://www.daytona.io/pricing), [CodeSandbox SDK Pricing](https://codesandbox.io/docs/sdk/pricing)

### 2.2 Open-Core Pricing Benchmarks

| Company | Free Tier | Mid Tier | Enterprise Tier | Pricing Metric |
|---------|-----------|----------|-----------------|----------------|
| **GitLab** | Free (5 users, 400 CI min) | Premium: $29/user/mo | Ultimate: $99/user/mo | Per user/month |
| **Supabase** | Free (500MB, 50K MAUs) | Pro: $25/mo + usage | Team: $599/mo; Enterprise: custom | Per project/month + usage |
| **PostHog** | 1M events free/mo | Usage-based (~$150-900/mo typical) | Custom | Per event/recording |
| **HashiCorp (pre-BSL)** | Free (open source) | Cloud: usage-based | Enterprise: custom | Per resource/hour |

Sources: [GitLab Pricing](https://about.gitlab.com/pricing/), [Supabase Pricing](https://supabase.com/pricing), [PostHog Pricing](https://userorbit.com/blog/posthog-pricing-guide), [Spendflo GitLab Guide](https://www.spendflo.com/blog/gitlab-pricing-guide)

### 2.3 Key Pricing Insights

1. **Per-second billing is table stakes.** Every direct competitor bills per second of sandbox runtime. Skillbox Cloud must match this.

2. **Free tiers are generous but time-limited.** E2B gives $100 once; Modal gives $30/month; Daytona gives $200 once. The pattern: enough to build a prototype, not enough for production.

3. **The $150/mo price point is a natural anchor.** E2B's Pro plan at $150/mo establishes a market reference. Skillbox can position below this for cloud (since the self-hosted option cannibalizes some cloud demand) or at parity with more included value.

4. **Enterprise starts at $3,000/mo.** E2B's minimum enterprise commitment sets the floor. Given Skillbox's self-hosted and compliance advantages, the enterprise tier can command $2,500-$5,000/mo.

5. **Self-hosted pricing is the gap in the market.** No direct competitor offers a production-grade, commercially supported self-hosted option. This is Skillbox's pricing moat.

---

## 3. Value-Based Pricing Analysis

### 3.1 Value Quantification by Persona

#### Persona 1: AI Agent Engineer at a Startup (Alex Chen)

| Value Driver | Quantification | Assumptions |
|-------------|---------------|-------------|
| **Engineering time saved** | $15,000-$30,000/year | 2-4 weeks of senior engineer time ($75-$95/hr) building and maintaining custom sandbox infrastructure, redirected to core product work |
| **Reduced security incidents** | $5,000-$50,000/year | 1-3 security incidents per year from unprotected code execution; average incident cost for startups is $8,000-$18,000 (IBM Cost of a Data Breach) |
| **Faster time-to-market** | $20,000-$80,000/year | 2-6 week acceleration in shipping agent features; valued at burn-rate opportunity cost ($10K-$15K/week for a Series A startup) |
| **Total value delivered** | **$40,000-$160,000/year** | |
| **Willingness to pay** | **$1,200-$3,600/year** | 3-5% of value captured; aligned with developer tooling benchmarks ($100-$300/mo) |

#### Persona 2: Platform Engineer at a Mid-Market Company (Jordan Rivera)

| Value Driver | Quantification | Assumptions |
|-------------|---------------|-------------|
| **Infrastructure consolidation** | $25,000-$60,000/year | Replaces 2-3 internal tools/scripts for sandboxed execution; reduces maintenance burden by 1 FTE-equivalent quarter |
| **Standardized execution layer** | $30,000-$75,000/year | Eliminates per-team custom sandboxing; 5-10 teams each saving 2-4 weeks/year |
| **Compliance acceleration** | $50,000-$150,000/year | Reduces SOC2/ISO27001 audit scope for AI workloads; avoids 1-2 months of compliance engineering |
| **Reduced on-call burden** | $10,000-$25,000/year | Fewer sandbox-related incidents; 50% reduction in related pages |
| **Total value delivered** | **$115,000-$310,000/year** | |
| **Willingness to pay** | **$6,000-$24,000/year** | 5-8% of value captured; aligned with platform tooling budgets ($500-$2,000/mo) |

#### Persona 3: Enterprise Architect at a Large Corporation (Priya Sharma)

| Value Driver | Quantification | Assumptions |
|-------------|---------------|-------------|
| **Regulatory compliance (EU AI Act, GDPR)** | $200,000-$1,000,000/year | Avoidance of fines (up to EUR 35M or 7% of revenue); audit readiness across AI workloads |
| **Data sovereignty** | $100,000-$500,000/year | On-prem execution eliminates data exfiltration risk; satisfies data localization requirements without architectural compromises |
| **Vendor consolidation** | $50,000-$200,000/year | Replaces multiple point solutions for sandbox execution across business units |
| **Risk reduction** | $100,000-$300,000/year | Structured execution with audit trails reduces liability exposure for AI-generated actions |
| **Total value delivered** | **$450,000-$2,000,000/year** | |
| **Willingness to pay** | **$30,000-$120,000/year** | 5-7% of value captured; aligned with enterprise infrastructure licensing ($2,500-$10,000/mo) |

### 3.2 Value-Price Ratio Summary

| Persona | Value Delivered | Recommended Price | Value Capture Rate | Price/Month |
|---------|----------------|-------------------|-------------------|-------------|
| Startup Engineer | $40K-$160K/yr | $1,200-$3,600/yr | 3-5% | $100-$300 |
| Platform Engineer | $115K-$310K/yr | $6K-$24K/yr | 5-8% | $500-$2,000 |
| Enterprise Architect | $450K-$2M/yr | $30K-$120K/yr | 5-7% | $2,500-$10,000 |

The 3-8% value capture rate is consistent with developer infrastructure pricing norms. Developer tools rarely capture more than 10% of the value they create -- the leverage comes from volume and expansion revenue.

---

## 4. Recommended Pricing Tiers

### 4.1 Tier Architecture

```
                    Community         Pro              Team             Enterprise
                    (Free/OSS)        ($49/mo)         ($149/mo)        ($2,500/mo min)
                    ─────────────     ─────────────    ─────────────    ─────────────
Target              Individual        Small team       Growth team      Large org
                    devs, eval        shipping to      scaling agents   compliance,
                                      production                        multi-tenant

Deployment          Self-hosted       Cloud or         Cloud or         Self-hosted
                    only              self-hosted      self-hosted      (managed optional)

Cloud Credits       --                $50/mo incl.     $200/mo incl.    Custom allocation
```

### 4.2 Detailed Tier Breakdown

#### Tier 1: Community (Free, MIT License)

**Target:** Individual developers, students, evaluation, open-source community.

| Feature | Included |
|---------|----------|
| Core runtime engine | Full (all runtimes: Python, Node.js, Bash) |
| SKILL.md format & catalog | Full |
| Docker-native execution | Full |
| Security hardening (capability drop, PID limits, non-root, network isolation) | Full |
| Structured JSON I/O + file artifacts | Full |
| MCP server integration | Full |
| REST API | Full |
| CLI tools | Full |
| Community support (GitHub Issues, Discord) | Yes |
| Concurrent sandboxes | Unlimited (self-hosted, resource-limited) |
| Telemetry & basic logs | Basic (stdout/stderr) |

**Rationale:** The entire core product remains free and fully functional. This is non-negotiable for maintaining community trust and the bottom-up adoption funnel that open-source products depend on. There is no artificial crippling of the free tier.

---

#### Tier 2: Pro ($49/month + usage overages)

**Target:** Small teams (2-5 engineers) shipping AI agents to production. Startup engineers who need operational convenience.

| Feature | Included |
|---------|----------|
| Everything in Community | Yes |
| **Skillbox Cloud access** | Yes |
| Included cloud compute | $50/mo (~1,000 sandbox-hours at $0.05/hr) |
| Overage rate | $0.045/hr per 1 vCPU sandbox (10% below E2B) |
| Cloud sandbox cold start | <500ms |
| Maximum session duration | 4 hours |
| Concurrent cloud sandboxes | 50 |
| Execution dashboard (web UI) | Yes |
| Structured execution logs | 7-day retention |
| Webhook notifications | Yes |
| Email support (48hr SLA) | Yes |
| SDK (Python, TypeScript) | Yes |
| LangChain / LangGraph integration | Yes |

**Why $49/mo:** Positioned deliberately below E2B Pro ($150/mo) and Supabase Pro ($25/mo, but different product category). The $49 price point is psychologically below the $50 threshold, easy to expense on a corporate card without procurement approval, and competitive enough to win budget-conscious startups. Included compute credits ($50/mo) mean light users effectively pay nothing beyond the platform fee.

---

#### Tier 3: Team ($149/month + usage overages)

**Target:** Growth-stage teams (5-20 engineers) with multiple agent deployments and need for collaboration, governance, and observability.

| Feature | Included |
|---------|----------|
| Everything in Pro | Yes |
| Included cloud compute | $200/mo (~4,000 sandbox-hours) |
| Overage rate | $0.040/hr per 1 vCPU sandbox (volume discount) |
| Maximum session duration | 24 hours |
| Concurrent cloud sandboxes | 200 |
| **Team workspace** | Multi-user with roles (admin, developer, viewer) |
| **Skill registry** | Private skill catalog with versioning and access controls |
| **Execution analytics** | Dashboard with usage trends, error rates, latency percentiles |
| **Structured execution logs** | 30-day retention + export |
| **Resource quotas & budgets** | Per-team and per-project spending limits |
| **Priority support** | Email (24hr SLA) + shared Slack channel |
| Self-hosted license key | Yes (unlocks Team features on self-hosted) |

**Why $149/mo:** Matches the E2B Pro anchor price. At this tier, the customer is paying for collaboration and governance features that do not exist in the open-source core. The $200/mo compute inclusion means most mid-market teams' usage is covered in the base price, with overages only for heavy workloads.

---

#### Tier 4: Enterprise ($2,500/month minimum, annual contract)

**Target:** Large organizations (500+ employees) with compliance requirements, multi-tenant deployments, and need for self-hosted production-grade infrastructure.

| Feature | Included |
|---------|----------|
| Everything in Team | Yes |
| **SSO / SAML / OIDC** | Yes |
| **RBAC with custom policies** | Yes (fine-grained, per-skill permissions) |
| **Audit logging** | Immutable, exportable, SIEM-compatible |
| **Air-gapped deployment support** | Yes (offline skill installation, no outbound telemetry) |
| **Custom sandbox images** | Bring your own base images with pre-installed dependencies |
| **Multi-cluster orchestration** | Kubernetes operator for multi-region / multi-cluster |
| **Compliance packages** | SOC2 Type II report, GDPR DPA, EU AI Act compliance documentation |
| **SLA** | 99.9% uptime (cloud) or 4hr response time (self-hosted) |
| **Dedicated support** | Named account engineer, private Slack, quarterly business review |
| **Volume pricing** | Custom compute rates (typically 30-50% below list price) |
| **Professional services** | Architecture review, migration assistance, custom skill development |
| Self-hosted license key | Yes (unlocks Enterprise features on self-hosted) |

**Why $2,500/mo minimum:** Slightly below E2B's $3,000/mo enterprise minimum, reflecting Skillbox's earlier market position. The $30,000/yr annual commitment is modest by enterprise standards -- large organizations routinely spend $50K-$500K/year on developer infrastructure. The self-hosted compliance package is the unique value differentiator: no competitor offers a production-grade, commercially supported, fully on-premises AI agent execution runtime with compliance documentation.

### 4.3 Feature Gate Philosophy

The feature gates follow GitLab's "buyer-based open core" principle:

- **Individual developer features** stay in Community (free). Anything a solo developer needs to build and run agent skills locally is free forever.
- **Team collaboration features** are in Pro/Team. Multi-user workspaces, shared skill registries, execution analytics, and team roles require coordination features that solo developers do not need.
- **Enterprise governance features** are in Enterprise. SSO, RBAC, audit logging, air-gapped deployment, and compliance documentation are valuable specifically to organizations with regulatory requirements.
- **Operational convenience** is in Cloud tiers. Self-hosting is always free. Paying for cloud means paying for uptime, cold-start optimization, and zero-ops execution.

This approach ensures the open-source community never feels that essential functionality is being withheld. The paid features serve genuinely different buyer needs at different organizational scales.

### 4.4 Cloud Usage Pricing Detail

| Resource | Rate | Billing |
|----------|------|---------|
| 1 vCPU sandbox | $0.045/hr (Pro) / $0.040/hr (Team) / Custom (Enterprise) | Per second |
| Additional vCPU | +$0.035/hr per vCPU | Per second |
| Memory (above 512MB default) | $0.005/hr per 512MB | Per second |
| Persistent storage | $0.10/GB/month | Monthly |
| Network egress | First 10GB free; $0.01/GB after | Monthly |
| Artifact storage | First 5GB free; $0.08/GB/month after | Monthly |

**Benchmark comparison:** These rates are 10-25% below E2B's published pricing and competitive with Modal's sandbox pricing ($0.14/hr for 1 core at production multipliers). The lower pricing is justified by Skillbox's smaller infrastructure footprint (newer, optimized platform) and the strategic need to win early market share.

---

## 5. Price Sensitivity Analysis

### 5.1 Price Floor (Minimum Viable Pricing)

| Component | Floor Rationale |
|-----------|----------------|
| **Cloud compute** | $0.03/hr per 1 vCPU | Below this, infrastructure costs (EC2/GCE + orchestration overhead) make the unit economics negative. Assuming 40-50% gross margin target, the cost floor is ~$0.015-$0.02/hr, yielding a minimum viable price of $0.03/hr. |
| **Pro tier** | $29/mo | Below $29/mo, the tier fails to cover customer acquisition cost (CAC) within a reasonable payback period. At $29/mo with 12-month average lifetime, LTV = $348 -- insufficient for sustainable growth. |
| **Team tier** | $99/mo | Below $99/mo, the tier is indistinguishable from Pro and does not justify the collaboration features investment. |
| **Enterprise tier** | $1,500/mo | Below $1,500/mo ($18K/yr), the enterprise sales motion (sales engineer, legal review, security questionnaire) is unprofitable. Minimum enterprise deal size should cover 2-3 months of sales cycle cost. |

### 5.2 Price Ceiling (Maximum Defensible Pricing)

| Component | Ceiling Rationale |
|-----------|-------------------|
| **Cloud compute** | $0.06/hr per 1 vCPU | Above E2B's $0.05/hr, Skillbox loses on price in head-to-head comparisons. As a newer entrant without E2B's enterprise traction, pricing above the market leader is unjustifiable unless accompanied by demonstrably superior features. |
| **Pro tier** | $79/mo | Above $79/mo, budget-conscious startups will default to free self-hosted + DIY, or choose E2B's $150/mo Pro which has stronger brand recognition. The $49-$79 range maximizes conversion. |
| **Team tier** | $249/mo | Above $249/mo, mid-market teams will evaluate E2B Enterprise ($3,000/mo) or build internal tooling. The $149-$249 range captures teams that want more than Pro but are not ready for enterprise procurement. |
| **Enterprise tier** | $10,000/mo | Above $10,000/mo ($120K/yr), Skillbox enters territory where customers demand extensive proof of value, multi-quarter pilots, and reference customers that a new product may not yet have. $2,500-$10,000/mo captures the sweet spot for Year 1-2. |

### 5.3 Price Sensitivity by Persona

```
                    Floor           Recommended         Ceiling
                    ──────          ──────────          ────────
Startup Engineer    $29/mo          $49/mo              $79/mo
Platform Engineer   $99/mo          $149/mo             $249/mo
Enterprise          $1,500/mo       $2,500/mo           $10,000/mo
Cloud Usage         $0.03/hr        $0.045/hr           $0.06/hr
```

### 5.4 Elasticity Observations

- **Startups** are highly price-elastic. A $20/mo difference can swing adoption decisions. The free tier and low Pro entry point are critical for this segment.
- **Mid-market** is moderately elastic. Teams at this stage care more about feature completeness and reliability than saving $50/mo. The Team tier should emphasize value, not price.
- **Enterprise** is inelastic on price, elastic on trust. A $5,000/mo vs $10,000/mo difference matters less than SOC2 compliance, named support, and reference customers. Enterprise pricing should be value-anchored, not cost-anchored.

---

## 6. Open Source Monetization Strategy

### 6.1 Lessons from the Industry

#### What Went Wrong: HashiCorp BSL (2023)

HashiCorp switched from MPL to BSL in August 2023 after struggling to monetize its user base despite technical market leadership. The result was immediate community backlash, the creation of the OpenTF fork (now OpenTofu under the Linux Foundation), and erosion of community goodwill that HashiCorp spent a decade building. IBM subsequently acquired HashiCorp for $6.4B -- arguably at a discount driven partly by the community fracture.

**Lesson:** License changes are a one-way door. Once trust is broken, it cannot be rebuilt. The community penalty exceeds any revenue protection gained. ([The New Stack](https://thenewstack.io/hashicorp-abandons-open-source-for-business-source-license/), [Runtime News](https://www.runtime.news/as-hashicorp-adopts-the-bsl-an-era-of-open-source-software-might-be-ending/))

#### What Went Wrong: Redis SSPL (2024) and Reversal (2025)

Redis switched from BSD to SSPL/RSAL in March 2024 to prevent cloud providers from offering managed Redis without contributing back. The Linux Foundation backed a fork (Valkey), and within a year, 83% of large enterprise Redis users had adopted or were testing Valkey. Redis reversed course in 2025, moving to AGPLv3 -- but the damage was done. The community had already migrated.

**Lesson:** If you are going to make a license change, do it before your project has massive adoption. For an early-stage project like Skillbox, the MIT license is an adoption accelerant, not a liability. Protect it. ([InfoQ](https://www.infoq.com/news/2024/03/redis-license-open-source/), [The Register](https://www.theregister.com/2025/05/01/redis_returns_to_open_source/))

#### What Went Right: Supabase

Supabase kept its core open source (MIT/Apache 2.0), built a managed cloud platform as the primary revenue driver, and reached 1.7M+ developers with clear tier separation. The self-hosted option drives adoption; the cloud option drives revenue. The Team tier at $599/mo captures mid-market without enterprise friction.

**Lesson:** Managed cloud is the natural monetization layer for open-source infrastructure. Self-hosted users become cloud customers when they scale. ([Supabase Pricing](https://supabase.com/pricing))

#### What Went Right: PostHog

PostHog kept MIT licensing, offered generous free tiers (1M events/month free), and built usage-based pricing that scales naturally. 98% of customers use PostHog for free. The 2% that pay generate $20M+ ARR. PostHog's philosophy: make the free product so good that word-of-mouth drives enterprise inbound.

**Lesson:** A massive free user base is a marketing engine, not a cost center. Generous free tiers create community advocates who drive enterprise sales. ([PostHog GitHub](https://github.com/PostHog/posthog), [Sacra](https://sacra.com/c/posthog/))

### 6.2 Skillbox Monetization Principles

Based on the above analysis, Skillbox should follow six principles:

1. **Never change the license.** MIT stays MIT. The license is a competitive moat against proprietary vendors and a trust signal for the community. If the business model requires license protection to work, the business model is wrong.

2. **Monetize operations, not code.** The open-source code is the product. The commercial offering sells operational convenience (managed cloud), organizational features (collaboration, governance), and trust (compliance, SLA, support). Customers pay to not have to operate Skillbox themselves, not to access the core functionality.

3. **Keep the free tier genuinely useful.** A solo developer should be able to run Skillbox in production on their own infrastructure indefinitely, for free, without hitting artificial limitations. The free tier is the top of the funnel and the community's reason to advocate.

4. **Gate on buyer role, not product capability.** Following GitLab's buyer-based open core model: individual developer features are free, team features are Team-tier, governance features are Enterprise-tier. This aligns price with the organizational buyer who controls the budget.

5. **Make cloud the path of least resistance.** The self-hosted experience should be excellent, but the cloud experience should be magical. Sub-second cold starts, zero configuration, automatic scaling, built-in observability -- the cloud tier should make self-hosting feel like unnecessary work for any team without a regulatory mandate.

6. **Build community-first, revenue-second.** Year 1 focus is adoption: GitHub stars, Discord members, skill catalog contributions, blog posts, conference talks. Revenue begins with self-serve cloud in Quarter 2-3. Enterprise sales begin only after achieving product-market fit signals (>1,000 GitHub stars, >100 weekly active self-hosted deployments, >10 community-contributed skills).

### 6.3 Community-Friendly Feature Gates

| Category | Community (Free) | Pro | Team | Enterprise |
|----------|------------------|-----|------|------------|
| Runtime execution | All runtimes | All runtimes | All runtimes | All runtimes |
| Security hardening | Full | Full | Full | Full |
| API & CLI | Full | Full | Full | Full |
| MCP integration | Full | Full | Full | Full |
| Skill catalog (local) | Full | Full | Full | Full |
| Cloud execution | -- | Yes | Yes | Yes |
| Web dashboard | -- | Basic | Advanced | Full |
| Execution logs | Basic (stdout) | 7-day structured | 30-day + export | Unlimited + SIEM |
| Team workspaces | -- | -- | Yes | Yes |
| Private skill registry | -- | -- | Yes | Yes |
| Usage analytics | -- | -- | Yes | Yes |
| SSO/SAML/OIDC | -- | -- | -- | Yes |
| RBAC | -- | -- | -- | Yes |
| Audit logging | -- | -- | -- | Yes |
| Air-gapped deployment | -- | -- | -- | Yes |
| Compliance docs | -- | -- | -- | Yes |
| Dedicated support | -- | -- | -- | Yes |

---

## 7. Revenue Projections

### 7.1 Assumptions

| Parameter | Value | Basis |
|-----------|-------|-------|
| Cloud launch | Q3 2026 (Month 7) | 6 months to build managed platform |
| Enterprise launch | Q1 2027 (Month 13) | 12 months to build enterprise features |
| Self-serve conversion rate | 2-5% of active self-hosted users | PostHog benchmark (2% pay); Supabase anecdotal |
| Enterprise ACV | $42,000/yr ($3,500/mo avg) | Midpoint of $2,500-$5,000/mo range |
| Monthly cloud churn | 5-8% (Pro), 3-5% (Team) | SaaS benchmarks for developer tools |
| Enterprise annual churn | 10-15% | Infrastructure tool benchmark |
| GitHub star growth | 500 by M6, 2,000 by M12, 5,000 by M18 | Comparable to Daytona trajectory |
| Self-hosted weekly active deployments | 50 by M6, 200 by M12, 500 by M18 | Conservative; Skillbox has no managed cloud yet |

### 7.2 Scenario A: Conservative (Organic Growth, No Fundraise)

Minimal marketing spend. Community-driven growth. Cloud launches Q3 2026.

| Period | Pro Customers | Team Customers | Enterprise Customers | MRR | ARR |
|--------|--------------|----------------|---------------------|-----|-----|
| M6 (Aug 2026) | 5 | 0 | 0 | $245 | $2,940 |
| M9 (Nov 2026) | 15 | 3 | 0 | $1,182 | $14,184 |
| M12 (Feb 2027) | 30 | 8 | 1 | $4,162 | $49,944 |
| M18 (Aug 2027) | 60 | 20 | 3 | $11,440 | $137,280 |
| M24 (Feb 2028) | 100 | 40 | 6 | $22,860 | $274,320 |
| M30 (Aug 2028) | 150 | 65 | 10 | $37,135 | $445,620 |
| M36 (Feb 2029) | 200 | 90 | 15 | $52,650 | $631,800 |

*MRR calculation: Pro x $49 + Team x $149 + Enterprise x $2,500. Excludes usage overages (estimated at 20-40% additional revenue at scale).*

**Year 1 ARR (Feb 2027):** ~$50K
**Year 2 ARR (Feb 2028):** ~$274K
**Year 3 ARR (Feb 2029):** ~$632K (+$125K-$250K usage overages = **~$757K-$882K total**)

### 7.3 Scenario B: Moderate (Seed Funding, Active Marketing)

$1-2M seed funding. Developer advocate hire. Content marketing, conference sponsorships. Cloud launches Q2 2026 (accelerated).

| Period | Pro Customers | Team Customers | Enterprise Customers | MRR | ARR |
|--------|--------------|----------------|---------------------|-----|-----|
| M6 (Aug 2026) | 20 | 5 | 0 | $1,725 | $20,700 |
| M9 (Nov 2026) | 50 | 15 | 2 | $9,685 | $116,220 |
| M12 (Feb 2027) | 100 | 30 | 4 | $19,370 | $232,440 |
| M18 (Aug 2027) | 200 | 70 | 10 | $45,230 | $542,760 |
| M24 (Feb 2028) | 350 | 130 | 20 | $87,520 | $1,050,240 |
| M30 (Aug 2028) | 500 | 200 | 35 | $142,300 | $1,707,600 |
| M36 (Feb 2029) | 700 | 300 | 50 | $203,800 | $2,445,600 |

**Year 1 ARR:** ~$232K
**Year 2 ARR:** ~$1.05M
**Year 3 ARR:** ~$2.45M (+$490K-$980K usage overages = **~$2.94M-$3.43M total**)

### 7.4 Scenario C: Aggressive (Series A, Product-Led Growth Engine)

$5-10M Series A. Hire sales team (2 AEs + SE). Significant cloud infrastructure investment. Product-led growth loops (skill marketplace, community integrations).

| Period | Pro Customers | Team Customers | Enterprise Customers | MRR | ARR |
|--------|--------------|----------------|---------------------|-----|-----|
| M6 (Aug 2026) | 50 | 15 | 2 | $12,685 | $152,220 |
| M9 (Nov 2026) | 120 | 40 | 5 | $24,340 | $292,080 |
| M12 (Feb 2027) | 250 | 80 | 12 | $54,170 | $650,040 |
| M18 (Aug 2027) | 500 | 200 | 30 | $129,300 | $1,551,600 |
| M24 (Feb 2028) | 900 | 400 | 60 | $253,700 | $3,044,400 |
| M30 (Aug 2028) | 1,400 | 650 | 100 | $415,450 | $4,985,400 |
| M36 (Feb 2029) | 2,000 | 1,000 | 150 | $622,000 | $7,464,000 |

**Year 1 ARR:** ~$650K
**Year 2 ARR:** ~$3.04M
**Year 3 ARR:** ~$7.46M (+$1.5M-$3.0M usage overages = **~$8.96M-$10.46M total**)

### 7.5 Revenue Projection Summary

| Metric | Scenario A (Conservative) | Scenario B (Moderate) | Scenario C (Aggressive) |
|--------|--------------------------|----------------------|------------------------|
| **Year 1 ARR** | ~$50K | ~$232K | ~$650K |
| **Year 2 ARR** | ~$274K | ~$1.05M | ~$3.04M |
| **Year 3 ARR** | ~$757K-$882K | ~$2.94M-$3.43M | ~$8.96M-$10.46M |
| Investment needed | $0 (bootstrapped) | $1-2M seed | $5-10M Series A |
| Team size (Y3) | 3-5 | 8-15 | 25-40 |
| Key risk | Slow growth; market passes you by | Execution speed vs. well-funded competitors | Burn rate vs. enterprise sales cycle length |

### 7.6 Revenue Mix (Steady State, Year 3)

Under Scenario B, the projected revenue mix at Year 3 is:

| Stream | % of Revenue | ARR Contribution |
|--------|-------------|-----------------|
| Enterprise contracts | 51% | ~$1.5M |
| Team tier subscriptions | 18% | ~$536K |
| Pro tier subscriptions | 14% | ~$412K |
| Usage overages (cloud) | 17% | ~$490K |

This mix is consistent with open-core benchmarks: GitLab generates ~75% of revenue from Premium/Ultimate tiers (analogous to Team/Enterprise), and Supabase's paid cloud revenue is dominated by Team ($599/mo) and Enterprise tiers.

---

## 8. Implementation Roadmap

### Phase 1: Foundation (Months 1-6, Q1-Q2 2026)

**Focus:** Community growth + cloud infrastructure development.

| Action | Timeline | Cost |
|--------|----------|------|
| Keep core MIT, build community (Discord, GitHub) | Ongoing | $0 (time only) |
| Build cloud execution backend (multi-tenant, usage metering) | M1-M5 | Engineering time |
| Build billing system (Stripe integration, usage tracking) | M3-M5 | Engineering time |
| Build basic web dashboard (execution logs, usage) | M4-M6 | Engineering time |
| Publish pricing page (announce tiers, waitlist for cloud) | M4 | $0 |
| Launch Pro tier (cloud, self-serve) | M6 | Cloud infrastructure (~$2K-$5K/mo initial) |

### Phase 2: Growth (Months 7-12, Q3-Q4 2026)

**Focus:** Cloud growth + Team tier + enterprise pipeline.

| Action | Timeline | Cost |
|--------|----------|------|
| Launch Team tier (workspace, analytics, skill registry) | M7 | Engineering time |
| Build self-hosted license key system | M8 | Engineering time |
| Hire developer advocate | M7 | ~$130K/yr (Switzerland-adjusted) |
| Begin enterprise feature development (SSO, RBAC, audit) | M9 | Engineering time |
| Attend/sponsor 2-3 AI infrastructure conferences | M8-M12 | $15K-$30K |
| First enterprise design partner (free, co-development) | M9 | $0 (value exchange) |

### Phase 3: Enterprise (Months 13-18, Q1-Q2 2027)

**Focus:** Enterprise launch + first enterprise revenue.

| Action | Timeline | Cost |
|--------|----------|------|
| Launch Enterprise tier | M13 | Engineering time |
| Hire first account executive (enterprise sales) | M13 | ~$150K/yr base + commission |
| SOC2 Type II certification | M12-M15 | $30K-$50K |
| Publish EU AI Act compliance documentation | M14 | Legal + engineering time |
| First paying enterprise customer | M14-M16 | -- |
| Expand cloud regions (EU, APAC) | M15-M18 | +$5K-$10K/mo infrastructure |

### Phase 4: Scale (Months 19-36, Q3 2027 - Q1 2029)

**Focus:** Revenue scaling + product expansion.

| Action | Timeline | Cost |
|--------|----------|------|
| Skill marketplace (community-contributed skills, potential revenue share) | M20 | Engineering time |
| Volume discount program for large deployments | M22 | Sales operations |
| Evaluate additional compliance certs (ISO 27001, HIPAA) | M24 | $50K-$100K each |
| Geographic expansion (dedicated sales in EU) | M24-M30 | Hire + operations |
| Potential Series A fundraise (if Scenario B/C trajectory) | M18-M24 | -- |

---

## 9. Sources

### Competitor Pricing
- [E2B Pricing](https://e2b.dev/pricing)
- [E2B Workload Pricing Estimator](https://pricing.e2b.dev/)
- [Modal Pricing](https://modal.com/pricing)
- [Modal AI Pricing Guide (Eesel)](https://www.eesel.ai/blog/modal-ai-pricing)
- [Fly.io Pricing](https://fly.io/pricing/)
- [Fly.io Resource Pricing Docs](https://fly.io/docs/about/pricing/)
- [Daytona Pricing](https://www.daytona.io/pricing)
- [CodeSandbox SDK Pricing](https://codesandbox.io/docs/sdk/pricing)

### Open-Core / OSS Business Models
- [GitLab Pricing](https://about.gitlab.com/pricing/)
- [GitLab Pricing Guide (Spendflo)](https://www.spendflo.com/blog/gitlab-pricing-guide)
- [Supabase Pricing](https://supabase.com/pricing)
- [Supabase Pricing 2026 (Metacto)](https://www.metacto.com/blogs/the-true-cost-of-supabase-a-comprehensive-guide-to-pricing-integration-and-maintenance)
- [PostHog Pricing Guide (Userorbit)](https://userorbit.com/blog/posthog-pricing-guide)
- [PostHog Revenue (Sacra)](https://sacra.com/c/posthog/)

### Open Source Monetization Strategy
- [How to Monetize Open Source Software (Reo.dev)](https://www.reo.dev/blog/monetize-open-source-software)
- [Open Source Business Models (Generative Value)](https://www.generativevalue.com/p/open-source-business-models-notes)
- [Open Source Playbook (Work-Bench)](https://www.work-bench.com/playbooks/open-source-playbook-proven-monetization-strategies)

### License Controversies
- [HashiCorp Adopts BSL (HashiCorp Blog)](https://www.hashicorp.com/en/blog/hashicorp-adopts-business-source-license)
- [HashiCorp BSL Analysis (The New Stack)](https://thenewstack.io/hashicorp-abandons-open-source-for-business-source-license/)
- [Era of Open Source Ending (Runtime News)](https://www.runtime.news/as-hashicorp-adopts-the-bsl-an-era-of-open-source-software-might-be-ending/)
- [Redis Switches to SSPL (InfoQ)](https://www.infoq.com/news/2024/03/redis-license-open-source/)
- [Redis Returns to Open Source (The Register)](https://www.theregister.com/2025/05/01/redis_returns_to_open_source/)
- [Redis Re-relicensing Explained (Dirk Riehle)](https://dirkriehle.com/2025/05/03/re-relicensing-to-open-source-explained/)

### Market Data
- [AI Agents Market (Grand View Research)](https://www.grandviewresearch.com/industry-analysis/ai-agents-market-report)
- [Agentic AI Market (Precedence Research)](https://www.precedenceresearch.com/agentic-ai-market)
- [AI Infrastructure Market (Coherent Market Insights)](https://www.coherentmarketinsights.com/industry-reports/ai-infrastructure-market)
- [SaaS Growth Report 2025 (ChartMogul)](https://chartmogul.com/reports/saas-growth-the-odds-of-making-it/)
- [Hottest Open Source Startups 2024 (TechCrunch)](https://techcrunch.com/2025/03/22/the-20-hottest-open-source-startups-of-2024/)
- [AI Sandbox Benchmark 2026 (Superagent)](https://www.superagent.sh/blog/ai-code-sandbox-benchmark-2026)
- [Best Sandbox Runners 2026 (Better Stack)](https://betterstack.com/community/comparisons/best-sandbox-runners/)

### Pricing Strategy References
- [Fly.io Pricing Analysis (SaaS Price Pulse)](https://www.saaspricepulse.com/tools/flyio)
- [Fly.io Pricing Guide (Withorb)](https://www.withorb.com/blog/flyio-pricing)
- [Supabase Pricing Breakdown (Flexprice)](https://flexprice.io/blog/supabase-pricing-breakdown)
- [5 Strategies for Monetizing OSS (Wingback)](https://www.wingback.com/blog/5-proven-strategies-for-monetizing-open-source-software)
