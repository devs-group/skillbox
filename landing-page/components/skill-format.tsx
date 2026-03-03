export function SkillFormat() {
  return (
    <section className="py-20 md:py-32 border-t border-border">
      <div className="mx-auto max-w-6xl px-6">
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-12 items-center">
          <div>
            <p className="font-mono text-sm text-primary mb-3 tracking-wider uppercase">Skill Spec</p>
            <h2 className="text-3xl md:text-4xl font-bold text-foreground text-balance">
              Skills are just files
            </h2>
            <p className="mt-4 text-lg text-muted-foreground leading-relaxed text-pretty">
              A skill is a zip archive with a <code className="font-mono text-primary text-sm bg-primary/10 px-1.5 py-0.5 rounded">SKILL.md</code> file and your scripts. YAML frontmatter for machines, markdown body for LLMs. Push with the CLI, discover via the API.
            </p>

            <div className="mt-8 space-y-3">
              <div className="flex items-center gap-3">
                <div className="w-1.5 h-1.5 rounded-full bg-primary" />
                <span className="text-sm text-foreground">YAML metadata for SDKs and API</span>
              </div>
              <div className="flex items-center gap-3">
                <div className="w-1.5 h-1.5 rounded-full bg-primary" />
                <span className="text-sm text-foreground">Markdown body for LLM tool selection</span>
              </div>
              <div className="flex items-center gap-3">
                <div className="w-1.5 h-1.5 rounded-full bg-primary" />
                <span className="text-sm text-foreground">Versioned. Lintable. Packageable.</span>
              </div>
              <div className="flex items-center gap-3">
                <div className="w-1.5 h-1.5 rounded-full bg-primary" />
                <span className="text-sm text-foreground">Push with one CLI command</span>
              </div>
            </div>
          </div>

          <div className="rounded-lg border border-border bg-card overflow-hidden">
            <div className="flex items-center gap-2 px-4 py-2.5 border-b border-border bg-secondary/30">
              <div className="flex gap-1.5">
                <div className="w-2.5 h-2.5 rounded-full bg-muted-foreground/20" />
                <div className="w-2.5 h-2.5 rounded-full bg-muted-foreground/20" />
                <div className="w-2.5 h-2.5 rounded-full bg-muted-foreground/20" />
              </div>
              <span className="font-mono text-xs text-muted-foreground ml-2">my-skill/</span>
            </div>
            <div className="p-5 font-mono text-sm space-y-1 text-muted-foreground">
              <div className="flex items-center gap-2">
                <span className="text-primary/60">{'├──'}</span>
                <span className="text-foreground font-medium">SKILL.md</span>
              </div>
              <div className="flex items-center gap-2">
                <span className="text-primary/60">{'├──'}</span>
                <span>scripts/</span>
              </div>
              <div className="flex items-center gap-2">
                <span className="text-primary/60">{'│   └──'}</span>
                <span>main.py</span>
              </div>
              <div className="flex items-center gap-2">
                <span className="text-primary/60">{'└──'}</span>
                <span>requirements.txt</span>
              </div>

              <div className="mt-6 pt-4 border-t border-border">
                <p className="text-xs text-muted-foreground/50 mb-3">{'# SKILL.md'}</p>
                <pre className="text-xs leading-5">{`---
name: data-analysis
version: "1.0.0"
description: Analyze CSV data
lang: python
timeout: 60s
resources:
  memory: 256Mi
  cpu: "0.5"
---

# Data Analysis Skill

Analyze data and produce summary
statistics with charts.`}</pre>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>
  )
}
