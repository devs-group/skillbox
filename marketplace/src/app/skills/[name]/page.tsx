"use client"

import { useEffect, useState, use } from "react"
import { ArrowLeft, Package } from "lucide-react"
import Link from "next/link"
import { CopyCommand } from "@/components/CopyCommand"
import { ApprovalBadge } from "@/components/ApprovalBadge"
import { apiFetch } from "@/lib/api"
import { useAuth } from "@/lib/auth-context"
import type { Skill } from "@/components/SkillCard"

interface SkillDetail extends Skill {
  approval_status?: string
}

function BlinkDot() {
  return <span className="inline-block h-2 w-2 bg-[#ea580c] animate-blink" />
}

export default function SkillDetailPage({
  params,
}: {
  params: Promise<{ name: string }>
}) {
  const { name } = use(params)
  const { session } = useAuth()
  const [skill, setSkill] = useState<SkillDetail | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState("")

  useEffect(() => {
    async function load() {
      try {
        const data = await apiFetch<SkillDetail>(
          `/v1/marketplace/skills/${encodeURIComponent(name)}`
        )
        setSkill(data)
      } catch (err) {
        setError(err instanceof Error ? err.message : "Failed to load skill")
      } finally {
        setLoading(false)
      }
    }
    load()
  }, [name])

  if (loading) {
    return (
      <div className="min-h-screen dot-grid-bg">
        <div className="w-full px-6 py-12 lg:px-12 max-w-3xl mx-auto">
          <div className="h-8 w-48 bg-muted animate-pulse mb-4" />
          <div className="h-4 w-96 bg-muted animate-pulse mb-2" />
          <div className="h-4 w-72 bg-muted animate-pulse" />
        </div>
      </div>
    )
  }

  if (error || !skill) {
    return (
      <div className="min-h-screen dot-grid-bg">
        <div className="w-full px-6 py-12 lg:px-12 text-center">
          <div className="border-2 border-foreground p-12">
            <p className="text-xs font-mono tracking-widest uppercase text-destructive">
              {error || "Skill not found"}
            </p>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="min-h-screen dot-grid-bg">
      <div className="w-full px-6 py-12 lg:px-12 max-w-3xl mx-auto">
        {/* Back link */}
        <div className="mb-8">
          <Link
            href="/skills"
            className="text-[10px] font-mono tracking-widest uppercase text-muted-foreground hover:text-foreground transition-colors inline-flex items-center gap-2"
          >
            <ArrowLeft size={12} />
            Back to catalog
          </Link>
        </div>

        {/* Section label */}
        <div className="flex items-center gap-4 mb-8">
          <span className="text-[10px] tracking-[0.2em] uppercase text-muted-foreground font-mono">
            {"// SKILL: DETAIL_VIEW"}
          </span>
          <div className="flex-1 border-t border-border" />
          <BlinkDot />
        </div>

        {/* Skill detail card */}
        <div className="border-2 border-foreground">
          {/* Header bar */}
          <div className="flex items-center justify-between px-5 py-3 border-b-2 border-foreground">
            <div className="flex items-center gap-3">
              <Package size={14} strokeWidth={1.5} />
              <span className="text-[10px] tracking-[0.2em] uppercase text-muted-foreground font-mono">
                {skill.provider ?? "community"}
              </span>
            </div>
            <div className="flex items-center gap-3">
              {skill.version && (
                <span className="text-[10px] tracking-[0.2em] uppercase text-muted-foreground font-mono">
                  v{skill.version}
                </span>
              )}
              {session && skill.approval_status && (
                <ApprovalBadge status={skill.approval_status} />
              )}
            </div>
          </div>

          {/* Body */}
          <div className="px-5 py-6">
            <h1 className="text-2xl lg:text-3xl font-mono font-bold tracking-tight uppercase mb-4">
              {skill.name}
            </h1>
            <p className="text-xs lg:text-sm font-mono text-muted-foreground leading-relaxed mb-6">
              {skill.description}
            </p>

            {skill.tags && skill.tags.length > 0 && (
              <div className="flex flex-wrap gap-2 mb-6">
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

          {/* Install section */}
          <div className="px-5 py-4 border-t-2 border-foreground">
            <span className="text-[10px] tracking-[0.2em] uppercase text-muted-foreground font-mono block mb-3">
              Install this skill
            </span>
            <CopyCommand name={skill.name} />
          </div>
        </div>
      </div>
    </div>
  )
}
