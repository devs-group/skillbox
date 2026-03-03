import Link from "next/link"
import { Github, Star, ArrowRight } from "lucide-react"

export function Hero() {
  return (
    <section className="relative pt-32 pb-20 md:pt-44 md:pb-32 overflow-hidden">
      {/* Background grid */}
      <div className="absolute inset-0 opacity-[0.03]" style={{
        backgroundImage: `linear-gradient(rgba(255,255,255,0.1) 1px, transparent 1px), linear-gradient(90deg, rgba(255,255,255,0.1) 1px, transparent 1px)`,
        backgroundSize: '60px 60px',
      }} />

      {/* Glow effect */}
      <div className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[600px] h-[600px] rounded-full opacity-10" style={{
        background: 'radial-gradient(circle, oklch(0.75 0.18 155) 0%, transparent 70%)',
      }} />

      <div className="relative mx-auto max-w-6xl px-6">
        {/* Badge */}
        <div className="flex justify-center mb-8">
          <Link
            href="https://github.com/devs-group/skillbox"
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-2 px-4 py-1.5 rounded-full border border-border bg-secondary/50 text-sm text-muted-foreground hover:border-primary/40 hover:text-foreground transition-all"
          >
            <Star className="w-3.5 h-3.5 text-primary" />
            <span className="font-mono">Open Source</span>
            <span className="text-border">|</span>
            <span className="font-mono">MIT License</span>
            <ArrowRight className="w-3.5 h-3.5" />
          </Link>
        </div>

        {/* Headline */}
        <h1 className="text-center text-4xl md:text-6xl lg:text-7xl font-bold tracking-tight text-balance leading-[1.1]">
          <span className="text-foreground">The execution runtime</span>
          <br />
          <span className="text-foreground">for </span>
          <span className="text-primary">AI agents</span>
        </h1>

        {/* Subheadline */}
        <p className="mx-auto mt-6 max-w-2xl text-center text-lg md:text-xl text-muted-foreground text-pretty leading-relaxed">
          Your agents need a sandbox. Don{"'"}t build one. Skillbox gives AI agents a single API to run sandboxed scripts and receive structured JSON output. Self-hosted, open source, secure by default.
        </p>

        {/* CTAs */}
        <div className="flex flex-col sm:flex-row items-center justify-center gap-4 mt-10">
          <Link
            href="https://github.com/devs-group/skillbox#quick-start"
            target="_blank"
            rel="noopener noreferrer"
            className="flex items-center gap-2 px-6 py-3 text-sm font-medium text-primary-foreground bg-primary rounded-md hover:opacity-90 transition-opacity"
          >
            Get Started
            <ArrowRight className="w-4 h-4" />
          </Link>
          <Link
            href="https://github.com/devs-group/skillbox"
            target="_blank"
            rel="noopener noreferrer"
            className="flex items-center gap-2 px-6 py-3 text-sm font-medium text-foreground bg-secondary border border-border rounded-md hover:bg-secondary/80 transition-colors"
          >
            <Github className="w-4 h-4" />
            View on GitHub
          </Link>
        </div>

        {/* Install command */}
        <div className="flex justify-center mt-8">
          <div className="flex items-center gap-3 px-5 py-2.5 rounded-md bg-secondary/70 border border-border font-mono text-sm">
            <span className="text-primary">$</span>
            <span className="text-muted-foreground">go install github.com/devs-group/skillbox/cmd/skillbox@latest</span>
          </div>
        </div>
      </div>
    </section>
  )
}
