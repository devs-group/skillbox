"use client"

import Link from "next/link"
import { CopyCommand } from "./CopyCommand"

export interface Skill {
  name: string
  description: string
  version?: string
  provider?: string
  tags?: string[]
}

export function SkillCard({ skill }: { skill: Skill }) {
  return (
    <div className="flex flex-col h-full border-2 border-foreground bg-background">
      {/* Card header bar */}
      <div className="flex items-center justify-between px-4 py-2 border-b-2 border-foreground">
        <span className="text-[10px] tracking-[0.2em] uppercase text-muted-foreground font-mono truncate">
          {skill.provider ?? "community"}
        </span>
        {skill.version && (
          <span className="text-[10px] tracking-[0.2em] uppercase text-muted-foreground font-mono">
            v{skill.version}
          </span>
        )}
      </div>

      {/* Card body */}
      <div className="flex-1 flex flex-col px-4 py-4">
        <Link
          href={`/skills/${encodeURIComponent(skill.name)}`}
          className="text-sm font-mono font-bold tracking-tight uppercase hover:text-[#ea580c] transition-colors duration-200 mb-2"
        >
          {skill.name}
        </Link>
        <p className="text-xs font-mono text-muted-foreground leading-relaxed line-clamp-3 mb-3">
          {skill.description}
        </p>
        {skill.tags && skill.tags.length > 0 && (
          <div className="flex flex-wrap gap-1 mt-auto">
            {skill.tags.map((tag) => (
              <span
                key={tag}
                className="text-[9px] tracking-[0.15em] uppercase font-mono border border-foreground/30 px-2 py-0.5 text-muted-foreground"
              >
                {tag}
              </span>
            ))}
          </div>
        )}
      </div>

      {/* Card footer */}
      <div className="px-4 py-3 border-t-2 border-foreground bg-muted/30">
        <CopyCommand name={skill.name} />
      </div>
    </div>
  )
}
