import Link from "next/link"
import { Network, Cpu, FileKey, Timer, KeyRound, Box } from "lucide-react"

const controls = [
  { icon: Network, label: "Network isolation", detail: "defaultAction: deny", threat: "Data exfiltration, SSRF" },
  { icon: Cpu, label: "Resource limits", detail: "CPU + memory caps, server-side clamping", threat: "Fork bombs, resource exhaustion" },
  { icon: FileKey, label: "Image allowlist", detail: "Validated before sandbox creation", threat: "Supply-chain attack" },
  { icon: Timer, label: "Timeout enforcement", detail: "Context cancellation + sandbox TTL", threat: "Resource exhaustion" },
  { icon: KeyRound, label: "Env var blocking", detail: "LD_PRELOAD, PYTHONPATH, NODE_OPTIONS filtered", threat: "Library injection" },
  { icon: Box, label: "Sandbox lifecycle", detail: "OpenSandbox API, no Docker socket", threat: "Host escape" },
]

export function Security() {
  return (
    <section id="security" className="py-20 md:py-32 border-t border-border">
      <div className="mx-auto max-w-6xl px-6">
        <div className="text-center mb-16">
          <p className="font-mono text-sm text-primary mb-3 tracking-wider uppercase">Security</p>
          <h2 className="text-3xl md:text-5xl font-bold text-foreground text-balance">
            6 layers of hardening
          </h2>
          <p className="mt-4 text-lg text-muted-foreground max-w-2xl mx-auto text-pretty">
            Powered by{" "}
            <Link href="https://github.com/alibaba/OpenSandbox" target="_blank" rel="noopener noreferrer" className="text-primary hover:underline underline-offset-4">
              OpenSandbox
            </Link>
            . Security is enforced by the runtime — not configurable away by callers. Every layer is mandatory.
          </p>
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
          {controls.map((control) => (
            <div
              key={control.label}
              className="flex items-start gap-4 p-5 rounded-lg border border-border bg-card hover:border-primary/30 transition-colors"
            >
              <div className="flex items-center justify-center w-9 h-9 rounded-md bg-primary/10 shrink-0">
                <control.icon className="w-4 h-4 text-primary" />
              </div>
              <div className="min-w-0">
                <h3 className="text-sm font-semibold text-foreground">{control.label}</h3>
                <p className="text-xs font-mono text-primary/80 mt-0.5">{control.detail}</p>
                <p className="text-xs text-muted-foreground mt-1">Prevents: {control.threat}</p>
              </div>
            </div>
          ))}
        </div>
      </div>
    </section>
  )
}
