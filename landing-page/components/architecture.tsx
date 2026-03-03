import Link from "next/link"

export function Architecture() {
  return (
    <section className="py-20 md:py-32 border-t border-border">
      <div className="mx-auto max-w-6xl px-6">
        <div className="text-center mb-16">
          <p className="font-mono text-sm text-primary mb-3 tracking-wider uppercase">Architecture</p>
          <h2 className="text-3xl md:text-5xl font-bold text-foreground text-balance">
            Simple. Stateless. Scalable.
          </h2>
          <p className="mt-4 text-lg text-muted-foreground max-w-2xl mx-auto text-pretty">
            Every execution: authenticate, load skill, validate image, create hardened sandbox, run, collect output, cleanup.
          </p>
        </div>

        <div className="rounded-lg border border-border bg-card p-6 md:p-10 overflow-x-auto">
          <pre className="font-mono text-sm md:text-base text-center text-muted-foreground leading-loose whitespace-pre">
{`┌─────────┐     ┌──────────┐     ┌────────────────┐     ┌───────────────┐     ┌──────────────┐
│  Agent  │────▶│ REST API │────▶│ Skill Registry │────▶│ OpenSandbox   │────▶│   Sandbox    │
│         │     │          │     │    (MinIO)     │     │   Runner      │     │  (hardened)  │
└─────────┘     └────┬─────┘     └────────────────┘     └───────┬───────┘     └──────┬───────┘
                     │                                          │                    │
                     ▼                                          ▼                    ▼
                ┌──────────┐                            ┌───────────────┐     ┌──────────────┐
                │PostgreSQL│                            │ OpenSandbox   │     │   Output +   │
                │          │                            │     API       │     │    Files     │
                └──────────┘                            └───────────────┘     └──────────────┘`}
          </pre>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mt-8">
          <div className="rounded-lg border border-border bg-card p-6">
            <h3 className="font-mono text-sm font-semibold text-primary mb-2">Deploy anywhere</h3>
            <p className="text-sm text-muted-foreground">Docker Compose for dev. Kubernetes + Helm for prod. Kustomize overlays for both environments.</p>
          </div>
          <div className="rounded-lg border border-border bg-card p-6">
            <h3 className="font-mono text-sm font-semibold text-primary mb-2">Scale horizontally</h3>
            <p className="text-sm text-muted-foreground">Stateless API behind a load balancer. PostgreSQL for state. MinIO/S3 for artifacts. Redis for optional caching.</p>
          </div>
          <div className="rounded-lg border border-border bg-card p-6">
            <h3 className="font-mono text-sm font-semibold text-primary mb-2">No Docker socket</h3>
            <p className="text-sm text-muted-foreground"><Link href="https://github.com/alibaba/OpenSandbox" target="_blank" rel="noopener noreferrer" className="text-primary hover:underline underline-offset-4">OpenSandbox</Link> manages container lifecycle directly. No Docker socket proxy required. Reduced attack surface.</p>
          </div>
        </div>
      </div>
    </section>
  )
}
