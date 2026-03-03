import Link from "next/link"
import { Github, ArrowRight } from "lucide-react"

export function CTA() {
  return (
    <section className="py-20 md:py-32 border-t border-border relative overflow-hidden">
      {/* Glow */}
      <div className="absolute top-0 left-1/2 -translate-x-1/2 w-[600px] h-[300px] opacity-10" style={{
        background: 'radial-gradient(ellipse, oklch(0.75 0.18 155) 0%, transparent 70%)',
      }} />

      <div className="relative mx-auto max-w-4xl px-6 text-center">
        <h2 className="text-3xl md:text-5xl font-bold text-foreground text-balance">
          Your agents need a sandbox.
          <br />
          <span className="text-primary">Don{"'"}t build one.</span>
        </h2>
        <p className="mt-6 text-lg text-muted-foreground max-w-xl mx-auto text-pretty">
          Start running sandboxed skills in under 5 minutes. Clone the repo, start the stack, push your first skill.
        </p>

        <div className="flex flex-col sm:flex-row items-center justify-center gap-4 mt-10">
          <Link
            href="https://github.com/devs-group/skillbox#quick-start"
            target="_blank"
            rel="noopener noreferrer"
            className="flex items-center gap-2 px-8 py-3.5 text-sm font-medium text-primary-foreground bg-primary rounded-md hover:opacity-90 transition-opacity"
          >
            Quick Start Guide
            <ArrowRight className="w-4 h-4" />
          </Link>
          <Link
            href="https://github.com/devs-group/skillbox"
            target="_blank"
            rel="noopener noreferrer"
            className="flex items-center gap-2 px-8 py-3.5 text-sm font-medium text-foreground bg-secondary border border-border rounded-md hover:bg-secondary/80 transition-colors"
          >
            <Github className="w-4 h-4" />
            Star on GitHub
          </Link>
        </div>

        <div className="mt-12 flex flex-col items-center gap-3">
          <div className="font-mono text-sm text-muted-foreground bg-secondary/50 border border-border rounded-md px-6 py-3">
            <span className="text-primary">$</span>{" "}
            git clone https://github.com/devs-group/skillbox.git && cd skillbox
          </div>
          <div className="font-mono text-sm text-muted-foreground bg-secondary/50 border border-border rounded-md px-6 py-3">
            <span className="text-primary">$</span>{" "}
            docker compose -f deploy/docker/docker-compose.yml up -d
          </div>
        </div>
      </div>
    </section>
  )
}
