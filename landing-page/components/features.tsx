import { Shield, Boxes, Braces, Link2, Server, Users, Terminal, FileOutput, Database } from "lucide-react"

const features = [
  {
    icon: Shield,
    title: "Secure by default",
    description: "OpenSandbox isolation with 6 layers of hardening. Network disabled, resource limits enforced, image allowlist, timeout enforcement, env var blocking, API-based lifecycle. Not configurable away by callers.",
  },
  {
    icon: Boxes,
    title: "Skills registry",
    description: "Push skills like you push packages — versioned, discoverable, introspectable. Agents browse the registry, inspect capabilities, and choose the right skill before executing. Think npm, but for agent capabilities.",
  },
  {
    icon: Braces,
    title: "Structured I/O",
    description: "Skills read JSON input, write JSON output, and produce file artifacts. No stdout parsing. Your agents get clean data, every time.",
  },
  {
    icon: Link2,
    title: "LangChain-ready",
    description: "Skills map 1:1 to LangChain tools. get_skill returns descriptions for tool selection. Build agent toolkits in one function call.",
  },
  {
    icon: Server,
    title: "Self-hosted",
    description: "Docker Compose with OpenSandbox for dev. Kubernetes + Helm for prod. Air-gapped? Works offline. Your infrastructure, your data, your rules. GDPR/EU AI Act compliant.",
  },
  {
    icon: Users,
    title: "Multi-tenant",
    description: "API keys scoped to tenants, skills and executions isolated. Ship to multiple teams from a single Skillbox deployment.",
  },
  {
    icon: Terminal,
    title: "Zero-dep SDKs",
    description: "Go and Python clients use only the standard library. No dependency conflicts, no transitive hell. Just import and go.",
  },
  {
    icon: FileOutput,
    title: "File artifacts",
    description: "Skills produce files. Runtime tars them. Presigned S3 URLs returned. Files persist across sessions with full versioning support.",
  },
  {
    icon: Database,
    title: "12-factor config",
    description: "All configuration via environment variables. Stateless API, horizontally scalable behind a load balancer. Production-ready from day one.",
  },
]

export function Features() {
  return (
    <section id="features" className="py-20 md:py-32 border-t border-border">
      <div className="mx-auto max-w-6xl px-6">
        <div className="text-center mb-16">
          <p className="font-mono text-sm text-primary mb-3 tracking-wider uppercase">Features</p>
          <h2 className="text-3xl md:text-5xl font-bold text-foreground text-balance">
            Everything your agents need
          </h2>
          <p className="mt-4 text-lg text-muted-foreground max-w-2xl mx-auto text-pretty">
            Built for teams that take execution security seriously. No shortcuts, no compromises.
          </p>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-px bg-border rounded-lg overflow-hidden">
          {features.map((feature) => (
            <div key={feature.title} className="bg-card p-8 hover:bg-secondary/30 transition-colors">
              <feature.icon className="w-5 h-5 text-primary mb-4" />
              <h3 className="text-base font-semibold text-foreground mb-2">{feature.title}</h3>
              <p className="text-sm text-muted-foreground leading-relaxed">{feature.description}</p>
            </div>
          ))}
        </div>
      </div>
    </section>
  )
}
