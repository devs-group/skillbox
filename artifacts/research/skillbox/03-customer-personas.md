# Skillbox Customer Personas

**Product:** Skillbox -- Secure skill execution runtime for AI agents
**Category:** AI Infrastructure / Developer Tools
**Date:** February 2026
**Research basis:** Market salary data, developer tool adoption surveys, AI agent framework landscape analysis, and enterprise procurement trends

---

## Table of Contents

1. [Persona 1: AI Agent Engineer at a Startup](#persona-1-alex-chen--senior-ai-agent-engineer)
2. [Persona 2: Platform/DevOps Engineer at a Mid-Size Company](#persona-2-jordan-rivera--staff-platform-engineer)
3. [Persona 3: Enterprise Architect at a Large Corporation](#persona-3-priya-sharma--principal-enterprise-architect)
4. [Persona 4: CTO/Technical Founder Evaluating AI Infrastructure](#persona-4-marcus-okonkwo--cto--co-founder)
5. [Cross-Persona Insights](#cross-persona-insights)

---

## Persona 1: Alex Chen -- Senior AI Agent Engineer

**Title:** Senior AI Agent Engineer
**Company type:** Series A/B AI-native startup (40-120 employees)

### Demographics

| Attribute | Detail |
|---|---|
| Age range | 27-34 |
| Seniority | Senior IC (4-7 years experience) |
| Company size | 40-120 employees |
| Industry vertical | AI/ML SaaS, developer tooling, or vertical AI applications |
| Education | BS/MS in Computer Science or related field |
| Compensation | $140,000-$190,000 base + 0.1-0.5% equity ([ZipRecruiter](https://www.ziprecruiter.com/Salaries/Ai-Agent-Engineer-Salary), [Glassdoor](https://www.glassdoor.com/Salaries/ai-agent-engineer-salary-SRCH_KO0,17.htm)) |
| Location | San Francisco, New York, Seattle, or remote-first |
| Team | 3-8 person AI/agent engineering team |

### Goals

1. **Ship reliable multi-agent systems to production.** Alex is under pressure to move beyond demos. The team needs agents that can perform real tasks -- data analysis, code generation, document processing -- without breaking in production. Every week of delay is runway burned.

2. **Reduce the custom infrastructure burden.** Alex has already built two internal sandboxing solutions that half-work. One uses subprocess calls with timeouts, the other runs containers manually with Docker SDK bindings. Both are brittle and eat time that should go toward agent logic.

3. **Maintain a composable, framework-agnostic stack.** The team uses LangGraph for orchestration today but may switch to CrewAI or a custom solution next quarter. Alex needs execution infrastructure that does not lock the team into a specific agent framework.

### Pain Points

1. **Sandboxing is a DIY nightmare.** Every team builds its own container execution layer. Alex has seen three different approaches in three startups, all with security gaps. There is no standard "run untrusted code safely" primitive for AI agents.

2. **Security is an afterthought in agent frameworks.** LangChain, AutoGen, and CrewAI focus on orchestration and prompt management. None of them provide hardened execution environments. When agents generate and run code, the security story is "trust the model not to do anything bad."

3. **Structured I/O is inconsistent.** Every skill or tool the agent calls returns data in a different format. Some return raw stdout, some return JSON, some return file paths. Alex spends hours writing adapters and parsers instead of building agent capabilities.

4. **No visibility into execution failures.** When a sandboxed script fails, debugging means SSH-ing into containers, tailing logs, and guessing. There is no structured execution log tied back to the agent's decision chain.

5. **Scaling from one agent to many is painful.** The prototype runs one skill at a time on a single Docker host. Moving to concurrent, multi-tenant execution with proper resource limits requires re-architecting everything Alex has built.

### Buying Behavior

- **Discovery:** GitHub trending, Hacker News, Twitter/X AI engineering community, LangChain/LangGraph Discord servers, podcast mentions (Latent Space, Practical AI). Follows key voices like Harrison Chase, Chip Huyen, and Shreya Shankar.
- **Evaluation:** Clones the repo within 10 minutes of discovering it. Runs `docker compose up`, pushes a skill, and evaluates whether the API contract is clean. Reads the source code before reading the docs. Checks GitHub stars, commit frequency, and issue response time.
- **Decision criteria:** Time-to-first-successful-execution under 15 minutes. Clean API design. No vendor lock-in. MIT license. Active maintainers.
- **Purchase trigger:** Free and open source for initial adoption. Would advocate for a commercial support tier if one existed once the team depends on it in production.
- **Influence:** Strong bottom-up influence. Can unilaterally adopt open-source tools. Needs engineering lead sign-off for anything that touches production infrastructure.

### Objections

1. **"We already have a working container execution setup."** Alex has invested time in a custom solution. Switching has a migration cost, and the team may resist ripping out something that "mostly works." _Counter: Skillbox replaces hundreds of lines of brittle glue code with a single API call. The security hardening alone (capability drop, PID limits, network isolation, non-root) would take weeks to replicate correctly._

2. **"I am not sure this project will be maintained long-term."** Startups move fast and cannot afford to depend on an open-source project that goes dormant in six months. _Counter: MIT license means the team can fork and maintain. Go codebase with zero external dependencies is easy to understand and modify. The project's architecture is simple enough to own._

3. **"We need async/streaming execution, not just synchronous."** Some agent workloads require long-running tasks with progress callbacks. _Counter: The synchronous API with timeouts covers 80%+ of agent skill execution. Async patterns can be built on top using the execution status endpoint._

### Messaging Hooks

- "Stop building sandboxing infrastructure. Start building agents."
- "One API call. Sandboxed execution. Structured output. Done."
- "Your agents deserve a runtime as secure as your production services."
- "Works with LangChain, CrewAI, AutoGen, or your custom framework. Skillbox does not care what orchestrates the call."
- "Docker Compose for dev. Kubernetes for prod. Same config."

### Day-in-the-Life Scenario

> It is 9:30 AM. Alex opens the team's Slack and sees a message from the product lead: "Customer X wants the agent to generate charts from their CSV uploads by end of week." Alex has built the agent orchestration layer, but the agent currently cannot run Python scripts safely -- the last attempt at subprocess execution caused a memory leak that took down the staging environment.
>
> Alex remembers seeing Skillbox on Hacker News two weeks ago and bookmarked it. After coffee, Alex clones the repo and runs `docker compose up`. Within 10 minutes the stack is running. Alex writes a quick `data-analysis` skill -- a Python script that reads JSON input, processes a CSV, generates a matplotlib chart, and writes both a JSON summary and a PNG to the output directory. `skillbox skill push` uploads it. A curl command confirms it works.
>
> By lunch, Alex has wired the Skillbox Python SDK into the agent's LangGraph tool executor. The agent can now call `skillbox_data_analysis` like any other tool. The structured JSON output feeds directly back into the agent's reasoning loop, and the chart PNG comes back as a presigned S3 URL that the frontend can render.
>
> At the 2 PM standup, Alex demos the feature. The product lead asks, "What happens if a user uploads a malicious script?" Alex pulls up the security model: network disabled, all capabilities dropped, PID limits, non-root user, 60-second timeout. The team ships it to staging that afternoon.

---

## Persona 2: Jordan Rivera -- Staff Platform Engineer

**Title:** Staff Platform Engineer
**Company type:** Mid-size SaaS company (200-800 employees) with a growing AI initiative

### Demographics

| Attribute | Detail |
|---|---|
| Age range | 30-38 |
| Seniority | Staff/Senior Staff (6-12 years experience) |
| Company size | 200-800 employees |
| Industry vertical | B2B SaaS, fintech, healthtech, or e-commerce |
| Education | BS in Computer Science or self-taught with equivalent experience |
| Compensation | $150,000-$200,000 base + bonus ([Glassdoor](https://www.glassdoor.com/Salaries/devops-engineer-salary-SRCH_KO0,15.htm), [Coursera](https://www.coursera.org/articles/devops-engineer-salary)) |
| Location | Major metro area or remote |
| Team | Internal platform/infrastructure team of 5-15 engineers |

### Goals

1. **Build a self-service internal platform for AI workloads.** Jordan's company just spun up an AI team of 8 engineers. They need to run arbitrary Python and Node.js scripts as part of agent pipelines, and they are currently asking Jordan's team to manually provision Docker containers for each new "skill." Jordan wants to provide a self-service API so the AI team stops filing infrastructure tickets.

2. **Enforce security and resource governance without blocking velocity.** The CISO has flagged that running LLM-generated code in production containers is a compliance risk. Jordan needs to provide sandboxed execution with proper isolation that satisfies security review without slowing down the AI team's iteration speed.

3. **Standardize on Kubernetes-native tooling.** The company runs everything on Kubernetes. Any new infrastructure component must fit into existing GitOps workflows (ArgoCD/Flux), support standard observability (Prometheus, Grafana), and not require special operational procedures.

### Pain Points

1. **The AI team keeps reinventing execution infrastructure.** Three different AI engineers have built three different "run this script in Docker" solutions. None of them handle cleanup properly, two have known security gaps, and Jordan's team is now on-call for all of them.

2. **No multi-tenancy for AI workloads.** The AI team serves multiple internal customers (marketing analytics, customer support automation, sales intelligence). Each needs isolated execution with separate resource limits and access controls. The current setup runs everything under one service account.

3. **Container sprawl and resource leaks.** When agent executions fail or timeout, orphaned containers and temp directories accumulate. Jordan has written three different cron jobs to clean them up, and they still find stale containers during incident reviews.

4. **Security review is a bottleneck.** Every new AI skill that runs arbitrary code triggers a security review. The review cycle takes 2-3 weeks because the security team has to assess each skill's container configuration individually. There is no standardized security baseline they can approve once.

5. **Observability gaps in AI execution pipelines.** When an agent skill fails, there is no correlation between the API request, the container execution, and the application logs. Jordan's team spends hours during incidents stitching together logs from three different systems.

### Buying Behavior

- **Discovery:** CNCF landscape, KubeCon talks, internal Slack channels (#platform-engineering, #devops), The New Stack, Platform Engineering community blog, LinkedIn thought leadership.
- **Evaluation:** Evaluates against a checklist: Kubernetes-native deployment (Kustomize/Helm), 12-factor configuration, health/readiness probes, structured logging, horizontal scalability, and security posture. Runs a proof-of-concept on a staging cluster for 1-2 weeks. Involves the security team early.
- **Decision criteria:** Operational simplicity. "Can I deploy this with our existing GitOps pipeline?" Production-readiness indicators (health checks, graceful shutdown, resource limits). Clear security model that the CISO will approve. Low maintenance burden -- Jordan's team cannot babysit another service.
- **Purchase trigger:** Open source for evaluation and initial deployment. Would push for enterprise support contract ($20K-$50K/year range) once it is running in production, primarily for SLA guarantees and priority security patches.
- **Influence:** Jordan has direct authority over the internal platform stack. Needs sign-off from the VP of Engineering for new production services and from the CISO for anything that runs untrusted code.

### Objections

1. **"We could build this ourselves with existing Kubernetes primitives."** Jordan's team has the skills to build a job runner using Kubernetes Jobs or Pods. The question is whether it is worth the ongoing maintenance. _Counter: Skillbox handles the full lifecycle -- image allowlisting, security hardening, structured I/O, artifact collection, cleanup -- out of the box. Building this from scratch is 3-6 months of platform engineering time. Maintaining it is ongoing._

2. **"This adds another service to our operational surface area."** Jordan's team already operates 30+ internal services. Every new one adds on-call burden. _Counter: Skillbox is a single Go binary with PostgreSQL and MinIO (both of which you already run). It exposes standard health/readiness probes and uses 12-factor configuration. The Docker socket proxy architecture means it does not require privileged access. Operational overhead is minimal._

3. **"How does this handle our compliance requirements (SOC 2, HIPAA)?"** The security team will ask about audit trails, data retention, and network isolation. _Counter: Every execution is logged in PostgreSQL with full input/output/timing metadata. Containers run with network disabled by default, so no data exfiltration is possible. The image allowlist ensures only pre-approved images run. These controls map directly to SOC 2 CC6/CC7 requirements._

### Messaging Hooks

- "Give your AI team a self-service execution platform. Stop being the bottleneck."
- "Security-hardened by default. Pass your next audit without a custom container policy."
- "Deploys on Kubernetes with Kustomize. Fits your existing GitOps workflow."
- "Multi-tenant API keys. Isolate teams, enforce resource limits, maintain visibility."
- "One service replaces three custom script runners and two cleanup cron jobs."

### Day-in-the-Life Scenario

> It is Monday morning. Jordan opens PagerDuty to find two alerts from the weekend: an orphaned container from the AI team's custom script runner consumed 4GB of memory before the OOM killer got it, and a temp directory on the Docker host hit 95% disk usage from failed execution artifacts. Jordan spends the first hour cleaning up.
>
> At the 10 AM platform team standup, the tech lead mentions that the AI team has filed another ticket requesting a "sandboxed Python execution environment for the new customer support agent." This is the fourth such request in two months. Jordan proposes evaluating Skillbox as a standardized execution runtime.
>
> By Wednesday, Jordan has Skillbox running on the staging Kubernetes cluster using the provided Kustomize overlays. The deployment includes namespace isolation, RBAC, NetworkPolicy, and Pod Security Standards -- all provided out of the box. Jordan adds Prometheus scraping for the `/metrics` endpoint and creates a Grafana dashboard for execution latency and failure rates.
>
> On Thursday, Jordan demonstrates the setup to the AI team lead. They create a tenant API key, push a test skill, and run it. The AI lead immediately asks, "Can we have separate API keys for each of our three internal customers?" Jordan shows the multi-tenant configuration. The security team reviews the container hardening (capabilities dropped, network disabled, PID limits, non-root) and approves the security baseline in a single review -- no more per-skill assessments.
>
> By the following Monday, the AI team is self-serving their execution needs through the Skillbox API. Jordan's platform team on-call has not been paged once for an AI execution issue.

---

## Persona 3: Priya Sharma -- Principal Enterprise Architect

**Title:** Principal Enterprise Architect
**Company type:** Large enterprise or regulated corporation (5,000-50,000+ employees)

### Demographics

| Attribute | Detail |
|---|---|
| Age range | 38-50 |
| Seniority | Principal/Distinguished (15-25 years experience) |
| Company size | 5,000-50,000+ employees |
| Industry vertical | Financial services, healthcare, insurance, or government |
| Education | MS in Computer Science or MBA with technical background; TOGAF or equivalent certification |
| Compensation | $200,000-$275,000 base + bonus ([Glassdoor](https://www.glassdoor.com/Salaries/enterprise-architect-salary-SRCH_KO0,20.htm), [ZipRecruiter](https://www.ziprecruiter.com/Salaries/Enterprise-Architect-Salary)) |
| Location | Corporate headquarters city (NYC, Chicago, Charlotte, Boston) or hybrid |
| Team | Architecture review board of 5-10; influences 50-200+ engineers across multiple business units |

### Goals

1. **Define a secure, enterprise-approved AI execution architecture.** Priya's company has 15+ teams experimenting with AI agents, each building their own execution infrastructure. The CTO has asked Priya to define a reference architecture that all teams must follow. It needs to satisfy compliance (SOC 2, HIPAA, PCI-DSS), security, and operational requirements.

2. **Prevent vendor lock-in while enabling AI adoption at scale.** The company already spends $40M+ annually on cloud and infrastructure vendors. Priya has seen what happens when teams adopt proprietary services without an exit strategy. The AI execution layer must be open, portable, and self-hosted.

3. **Reduce architectural fragmentation across business units.** The wealth management division runs Python scripts in Lambda, the insurance claims team uses a custom Kubernetes job runner, and the retail banking team is evaluating a commercial AI sandbox vendor at $500K/year. Priya needs to consolidate these into a single platform.

### Pain Points

1. **No enterprise-grade standard for AI agent execution.** The market is full of AI orchestration frameworks, but none of them address the "run untrusted code safely in a regulated environment" problem. Priya's architecture review board has rejected three proposals in the past six months because none could demonstrate adequate security controls.

2. **Compliance review cycles are killing AI velocity.** Every new AI workload that executes generated code requires a full security and compliance review: threat modeling, penetration testing, and audit trail verification. This takes 6-12 weeks per workload. Teams are either waiting in the queue or bypassing the process entirely.

3. **Shadow IT in AI tooling.** Development teams are spinning up unapproved AI execution environments because the approved process is too slow. Priya has discovered three teams running containers with `--privileged` mode in production because their custom sandboxing required it. This is a compliance violation waiting to be found.

4. **Audit trail requirements are unmet.** Regulators require a complete audit trail of what code was executed, with what inputs, producing what outputs, by which user/agent, at what time. None of the existing ad-hoc solutions provide this. During the last audit, the team spent two weeks manually reconstructing execution logs.

5. **Vendor risk concentration.** The CTO has flagged that relying on a single commercial AI platform for execution creates unacceptable vendor risk. If the vendor raises prices, changes terms, or shuts down, 15+ teams are disrupted. Priya needs an open-source, self-hosted solution that the company can operate independently.

### Buying Behavior

- **Discovery:** Gartner and Forrester reports on AI infrastructure, ThoughtWorks Technology Radar, architecture conference presentations (QCon, O'Reilly Software Architecture), vendor briefings, peer CTO/architect network conversations.
- **Evaluation:** Formal evaluation process with weighted criteria: security model (30%), compliance readiness (25%), operational maturity (20%), total cost of ownership (15%), ecosystem/extensibility (10%). Requires a proof-of-concept in a controlled environment, security review by the AppSec team, and architecture review board approval.
- **Decision criteria:** Can the security team approve this once for all teams? Does it produce audit-compliant execution logs? Is it self-hosted with no external dependencies? Does it run on our existing Kubernetes infrastructure? Is the license enterprise-friendly (MIT or Apache 2.0)?
- **Purchase trigger:** Would champion a commercial support contract ($100K-$250K/year) to get SLAs, dedicated support, priority security patches, and compliance documentation (SOC 2 Type II report). Needs a vendor entity the procurement team can sign a contract with.
- **Influence:** Priya does not buy software directly. She defines architectural standards and approved technology lists. Once Skillbox is on the approved list, 15+ teams will adopt it. Procurement handles the commercial relationship. The CISO has veto power over any security-sensitive technology.

### Objections

1. **"There is no commercial entity behind this for enterprise support."** Large enterprises require a vendor they can hold contractable for SLAs, security incident response, and indemnification. An open-source project maintained by a small team does not satisfy procurement requirements. _Counter: The MIT license allows the internal platform team to own and operate the software independently. For enterprises that need commercial support, devs group (the maintainers) offers enterprise support tiers. The Go codebase is small and auditable -- the company's own engineers can review and maintain it._

2. **"The project is too young for enterprise adoption."** Priya's architecture review board has a maturity criterion: projects must demonstrate production stability, a track record of security patch responsiveness, and community adoption. _Counter: Position Skillbox as a "reference implementation" that the internal platform team adapts and hardens. The security model (capability drop, network isolation, non-root, socket proxy) follows Docker and Kubernetes security best practices that the team already understands. The architecture is simple enough to audit in a single day._

3. **"We need integration with our existing identity and access management (IAM) stack."** The enterprise uses Okta/Azure AD for authentication and a custom RBAC system for authorization. Skillbox's API key model is too simple. _Counter: The API sits behind the company's existing API gateway (Kong, Istio, etc.), which handles enterprise SSO. Skillbox's multi-tenant API keys map to teams/business units. For deeper RBAC integration, the API is simple enough to wrap with an internal authorization layer._

### Messaging Hooks

- "One security review. All teams covered. Ship your next AI audit in days, not months."
- "Self-hosted, MIT-licensed, no external dependencies. Your data never leaves your infrastructure."
- "Replace 15 ad-hoc script runners with one enterprise-approved execution platform."
- "Complete audit trail: who ran what, with what inputs, producing what outputs, when."
- "Runs on your existing Kubernetes cluster. No new infrastructure to procure."

### Day-in-the-Life Scenario

> It is the quarterly architecture review board meeting. Priya presents the findings from her AI execution infrastructure assessment. Three business units have built custom solutions: Lambda-based script execution (no audit trail), a Kubernetes job runner (no network isolation), and a commercial sandbox vendor (costs $500K/year, creates vendor lock-in, sends data to third-party infrastructure).
>
> Priya proposes Skillbox as the standardized execution layer. She walks through the security model table: network isolation via `NetworkMode: none`, all capabilities dropped, PID limits, non-root user, socket proxy for Docker daemon access, and image allowlisting. The CISO, who has been blocking AI execution proposals for six months, reviews the model and says, "This is the first proposal that addresses all our threat categories. I can approve this as a baseline."
>
> Priya's team works with the central platform team to deploy Skillbox on the company's shared Kubernetes cluster. They configure tenant API keys for each business unit, set up audit log forwarding to Splunk, and create a Grafana dashboard for execution monitoring. The architecture review board approves Skillbox for the internal "Approved Technology" list.
>
> Over the next quarter, the wealth management, insurance claims, and retail banking teams migrate their AI execution workloads to Skillbox. The compliance team completes a single review that covers all three deployments. The $500K/year commercial vendor contract is not renewed. During the next SOC 2 audit, the auditor reviews the centralized execution logs and closes the AI workload control in under an hour.

---

## Persona 4: Marcus Okonkwo -- CTO & Co-Founder

**Title:** CTO & Co-Founder
**Company type:** Seed/Series A AI startup (5-30 employees)

### Demographics

| Attribute | Detail |
|---|---|
| Age range | 30-42 |
| Seniority | Co-Founder / Executive (8-18 years experience) |
| Company size | 5-30 employees |
| Industry vertical | Vertical AI applications, AI-powered automation, or developer tools |
| Education | BS/MS in Computer Science; may have PhD in ML/AI |
| Compensation | $120,000-$180,000 base + 10-30% equity (pre-dilution) |
| Location | San Francisco, New York, London, or fully remote |
| Team | Directly manages 3-12 engineers; writes code daily |

### Goals

1. **Ship the core product fast with limited engineering resources.** Marcus has 6 engineers, 18 months of runway, and needs to reach product-market fit. Every hour spent on infrastructure that is not the core product is an existential risk. Marcus needs to buy or adopt, not build, every non-differentiating component.

2. **Build a product architecture that scales to enterprise customers.** The first customers are startups and mid-market companies, but the Series A pitch deck has "enterprise" on slide 7. The architecture needs to be secure enough and auditable enough that a future Fortune 500 customer's security team will not reject it.

3. **Attract and retain top AI engineering talent.** The best AI engineers want to work on interesting agent problems, not on container orchestration and sandboxing plumbing. Marcus needs a modern, well-tooled stack that signals engineering sophistication and lets the team focus on differentiated work.

### Pain Points

1. **The build-vs-buy tradeoff is acute at the early stage.** Marcus has considered building a custom execution sandbox three times. Each time, the estimate comes back at 4-8 weeks of one senior engineer's time -- time that could go toward features that directly drive revenue. But the alternative (running agent-generated code unsandboxed) is a security liability that could end the company.

2. **Security incidents at an early stage are company-ending.** If an AI agent running on Marcus's platform exfiltrates customer data or executes a malicious script, the resulting breach would destroy customer trust and likely kill fundraising. Marcus needs production-grade security from day one, not as a future project.

3. **The team is too small to specialize in infrastructure.** Marcus does not have a dedicated platform team. The same engineers who build agent logic also manage Docker deployments and debug container networking issues. Infrastructure work is context-switching overhead that slows feature development.

4. **Investor and customer due diligence asks hard questions about security.** During the Series A due diligence, the lead investor's technical advisor asked, "How do you isolate agent code execution from your core infrastructure?" Marcus's answer -- "We use Docker containers with some security flags" -- did not inspire confidence. He needs a more rigorous answer.

5. **Framework churn in the AI ecosystem is relentless.** The team started with LangChain, evaluated CrewAI, and is now considering building a custom orchestration layer. The execution runtime should be stable and framework-agnostic so that swapping the orchestration layer does not require re-architecting the sandbox.

### Buying Behavior

- **Discovery:** Hacker News, Twitter/X AI engineering community, Y Combinator internal forums, investor portfolio company network, direct outreach from developer tool companies, GitHub Explore.
- **Evaluation:** Marcus evaluates tools in under 30 minutes. Reads the README. If the quick start works in 10 minutes, evaluates the architecture. Checks the license (must be MIT or Apache 2.0). Reviews the security model. Makes a decision by end of day. If the tool requires a sales call or a demo, Marcus moves on.
- **Decision criteria:** Time-to-value under 30 minutes. No sales process. MIT license. Self-hosted (no data leaving the infrastructure). Docker Compose for dev, Kubernetes for prod. Clean API that the team can integrate in a day. Actively maintained.
- **Purchase trigger:** Adopts open-source immediately. Would pay $1K-$5K/month for a managed/enterprise tier that saves the team operational overhead, especially if it includes monitoring, auto-scaling, and compliance documentation that impresses investors.
- **Influence:** Marcus has unilateral authority over technology decisions. Consults the engineering team on implementation details but makes the architectural call.

### Objections

1. **"This is another dependency, and dependencies at our stage are risky."** Marcus has been burned by open-source projects that go unmaintained. Adding a dependency on an early-stage project for a critical security function feels risky. _Counter: Skillbox is a single Go binary with no external dependencies beyond PostgreSQL and S3 (MinIO). The MIT license means Marcus's team owns the code forever. The architecture is intentionally simple -- the entire codebase is auditable in a day. If the project goes dormant, the team can maintain their fork with minimal effort._

2. **"I can get 80% of this with a shell script and Docker."** Marcus has seen engineers wire up Docker execution in a weekend. The question is whether the remaining 20% (security hardening, structured I/O, artifact management, cleanup, multi-tenancy) is worth adopting a new tool. _Counter: The 80% solution is what gets startups breached. The missing 20% is network isolation, capability drops, PID limits, image allowlisting, proper cleanup, and audit trails. Building that correctly takes 4-8 weeks, not a weekend. Skillbox gives you the 100% solution in 10 minutes._

3. **"We need to move fast, and adding infrastructure slows us down."** Every new service in the stack is something to deploy, monitor, and debug. Marcus wants fewer moving parts, not more. _Counter: Skillbox replaces moving parts -- it consolidates script execution, sandboxing, artifact management, and security hardening into a single service. The team is currently maintaining ad-hoc versions of all these functions. Skillbox reduces net operational complexity._

### Messaging Hooks

- "Your agents need a sandbox. You should not build one."
- "10 minutes from git clone to production-grade sandboxed execution."
- "MIT license. Self-hosted. No vendor lock-in. No data leaves your infrastructure."
- "Impress your next investor's technical diligence with a real security model."
- "Let your engineers build the product, not the plumbing."

### Day-in-the-Life Scenario

> It is 7 AM. Marcus is reviewing the Series A term sheet feedback. The lead investor's technical advisor flagged one concern: "The application allows AI agents to execute arbitrary code on shared infrastructure. What isolation guarantees exist?" Marcus's current answer is a Docker container with a 30-second timeout. He knows this is inadequate.
>
> Over breakfast, Marcus opens Hacker News and sees a Show HN post about Skillbox. He reads the README in 5 minutes, focusing on the security model table. Network isolation, capability drop, PID limits, non-root user, socket proxy, image allowlist -- this is the answer to the investor's question.
>
> By 9 AM, Marcus has run `docker compose up` and pushed a test skill. By 10 AM, he has wired the Skillbox Python SDK into the team's agent framework. The integration is 12 lines of code. He sends a Slack message to the engineering team: "I just replaced our custom Docker execution layer with Skillbox. Our sandbox now has proper security hardening. Please review the diff and the architecture doc."
>
> At noon, Marcus replies to the investor's technical advisor with a detailed description of the Skillbox security model, including the specific container hardening controls and the architecture diagram. The advisor responds: "This is significantly more rigorous than what we typically see at this stage. Approved."
>
> That afternoon, Marcus updates the technical architecture document in the data room. The section on "Agent Execution Security" is no longer a single paragraph -- it is a full page describing defense-in-depth controls with references to OWASP and CIS Docker benchmarks. The team ships two new agent skills before end of day using the time they saved not debugging container cleanup issues.

---

## Cross-Persona Insights

### Adoption Pathway

The typical Skillbox adoption follows a predictable pattern across all four personas:

```
Discovery (GitHub/HN/Twitter)
    --> Quick Start (<15 min)
        --> First Skill Execution
            --> Integration into Agent Framework
                --> Production Deployment
                    --> Team/Org-Wide Adoption
                        --> Enterprise Support Inquiry
```

### Decision Influence Map

| Persona | Decision Authority | Budget Authority | Gatekeeper |
|---|---|---|---|
| Alex (AI Engineer) | Can adopt OSS unilaterally | No direct budget | Eng Lead approves production use |
| Jordan (Platform Eng) | Owns platform stack decisions | Can justify $20K-$50K/yr support | CISO has veto on security tools |
| Priya (Enterprise Arch) | Defines approved tech list | Champions $100K-$250K/yr contracts | Architecture review board, CISO, procurement |
| Marcus (CTO/Founder) | Unilateral authority | Full budget control | Investor technical diligence |

### Common Themes Across All Personas

1. **Security is the primary value driver.** Every persona's top pain point relates to insecure or inadequately sandboxed code execution. Skillbox's security-by-default model (network disabled, capabilities dropped, non-root, PID limits, image allowlist) is the single most compelling feature.

2. **Time saved on infrastructure is the secondary driver.** All four personas are spending significant engineering time building, maintaining, or debugging custom execution infrastructure. Skillbox's value proposition is not just "better sandboxing" -- it is "hours per week returned to the team."

3. **Open source and self-hosted are table stakes.** No persona expressed willingness to send execution data to a third-party SaaS. Self-hosted, MIT-licensed, and no external dependencies are baseline requirements, not differentiators.

4. **Framework agnosticism matters.** The AI agent framework landscape is volatile. All personas need an execution runtime that works with LangChain, CrewAI, AutoGen, or custom frameworks. Skillbox's clean REST API and SDK approach satisfies this requirement.

5. **The evaluation window is short.** Technical buyers give new tools 10-30 minutes. If the quick start does not work, they move on. The `docker compose up` experience and time-to-first-execution are critical adoption metrics.

### Pricing Sensitivity by Persona

| Persona | Open Source Adoption | Willingness to Pay | Budget Range |
|---|---|---|---|
| Alex (AI Engineer) | Yes, immediate | Would advocate internally | Influenced by team lead |
| Jordan (Platform Eng) | Yes, for evaluation | Yes, for support SLA | $20K-$50K/year |
| Priya (Enterprise Arch) | Yes, as reference implementation | Yes, for commercial support | $100K-$250K/year |
| Marcus (CTO/Founder) | Yes, immediate | Yes, for managed/enterprise tier | $12K-$60K/year |

### Competitive Positioning by Persona

| Persona | Primary Alternative | Skillbox Advantage |
|---|---|---|
| Alex (AI Engineer) | Custom Docker SDK wrapper | Security hardening, structured I/O, artifact management |
| Jordan (Platform Eng) | Kubernetes Jobs with custom controller | Out-of-box security baseline, multi-tenancy, audit trail |
| Priya (Enterprise Arch) | Commercial AI sandbox vendor ($500K/yr) | Self-hosted, MIT license, 10x cost reduction, no vendor lock-in |
| Marcus (CTO/Founder) | Quick-and-dirty subprocess/Docker script | Production-grade security from day one, investor confidence |

---

## Sources

- [Coursera: AI Engineer Salary Guide 2026](https://www.coursera.org/articles/ai-engineer-salary)
- [ZipRecruiter: AI Agent Engineer Salary](https://www.ziprecruiter.com/Salaries/Ai-Agent-Engineer-Salary)
- [Glassdoor: AI Agent Engineer Salary](https://www.glassdoor.com/Salaries/ai-agent-engineer-salary-SRCH_KO0,17.htm)
- [Glassdoor: DevOps Engineer Salary](https://www.glassdoor.com/Salaries/devops-engineer-salary-SRCH_KO0,15.htm)
- [Coursera: DevOps Engineer Salary 2026](https://www.coursera.org/articles/devops-engineer-salary)
- [Platform Engineering: Being a Platform Engineer in 2026](https://platformengineering.org/blog/being-a-platform-engineer-in-2026)
- [Glassdoor: Enterprise Architect Salary](https://www.glassdoor.com/Salaries/enterprise-architect-salary-SRCH_KO0,20.htm)
- [ZipRecruiter: Enterprise Architect Salary](https://www.ziprecruiter.com/Salaries/Enterprise-Architect-Salary)
- [Glassdoor: Senior Enterprise Architect Salary](https://www.glassdoor.com/Salaries/senior-enterprise-architect-salary-SRCH_KO0,27.htm)
- [Bain Capital Ventures: VC Insights 2025](https://baincapitalventures.com/insight/vc-insights-2025-ai-trends-startup-growth-and-2026-predictions/)
- [Lloydson: AI Infrastructure in 2026](https://www.lloydson.com/insights/ai-infrastructure-trends-2026-ceo-guide)
- [Harness: CTO Predictions for 2026](https://www.harness.io/blog/cto-predictions-for-2026-durkin)
- [The New Stack: AI Engineering Trends in 2025](https://thenewstack.io/ai-engineering-trends-in-2025-agents-mcp-and-vibe-coding/)
- [Langfuse Blog: Comparing Open-Source AI Agent Frameworks](https://langfuse.com/blog/2025-03-19-ai-agent-comparison)
- [Getmaxim: Best AI Agent Frameworks 2025](https://www.getmaxim.ai/articles/top-5-ai-agent-frameworks-in-2025-a-practical-guide-for-ai-builders/)
- [Catchy Agency: What 202 Open Source Developers Taught Us About Tool Adoption](https://www.catchyagency.com/post/what-202-open-source-developers-taught-us-about-tool-adoption)
- [Product Marketing Alliance: Open Source to PLG](https://www.productmarketingalliance.com/developer-marketing/open-source-to-plg/)
- [The New Stack: Open Source 2025 Trends](https://thenewstack.io/open-source-inside-2025s-4-biggest-trends/)
