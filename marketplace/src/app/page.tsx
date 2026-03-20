"use client"

import { useEffect, useState, useCallback } from "react"
import { Terminal } from "lucide-react"
import { SearchBar } from "@/components/SearchBar"
import { SkillCard, type Skill } from "@/components/SkillCard"
import { apiFetch } from "@/lib/api"

function BlinkDot() {
  return <span className="inline-block h-2 w-2 bg-[#ea580c] animate-blink" />
}

export default function HomePage() {
  const [skills, setSkills] = useState<Skill[]>([])
  const [loading, setLoading] = useState(true)

  const fetchSkills = useCallback(async (query = "") => {
    setLoading(true)
    try {
      const params = new URLSearchParams({ limit: "12" })
      if (query) params.set("q", query)
      const data = await apiFetch<{ skills: Skill[] }>(
        `/v1/marketplace/skills?${params}`
      )
      setSkills(data.skills ?? [])
    } catch {
      setSkills([])
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchSkills()
  }, [fetchSkills])

  return (
    <div className="min-h-screen dot-grid-bg">
      {/* Search Section */}
      <section className="w-full px-6 pt-12 pb-8 lg:px-12">
        <div className="flex items-center gap-4 mb-8">
          <span className="text-[10px] tracking-[0.2em] uppercase text-muted-foreground font-mono">
            {"// SECTION: SEARCH_SKILLS"}
          </span>
          <div className="flex-1 border-t border-border" />
          <BlinkDot />
          <span className="text-[10px] tracking-[0.2em] uppercase text-muted-foreground font-mono">
            001
          </span>
        </div>

        <div className="flex justify-center mb-4">
          <SearchBar placeholder="Search skills..." onSearch={fetchSkills} />
        </div>
      </section>

      {/* Skills Grid */}
      <section className="w-full px-6 pb-20 lg:px-12">
        <div className="flex items-center gap-4 mb-8">
          <span className="text-[10px] tracking-[0.2em] uppercase text-muted-foreground font-mono">
            {"// SECTION: SKILLS"}
          </span>
          <div className="flex-1 border-t border-border" />
          <span className="text-[10px] tracking-[0.2em] uppercase text-muted-foreground font-mono">
            002
          </span>
        </div>

        {loading ? (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-0 border-2 border-foreground">
            {Array.from({ length: 6 }).map((_, i) => (
              <div key={i} className="h-48 bg-muted/30 animate-pulse border border-foreground/10" />
            ))}
          </div>
        ) : skills.length === 0 ? (
          <div className="border-2 border-foreground p-12 text-center">
            <Terminal size={24} className="mx-auto mb-4 text-muted-foreground" />
            <p className="text-xs font-mono tracking-widest uppercase text-muted-foreground">
              No skills found. Try a different search term.
            </p>
          </div>
        ) : (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-0">
            {skills.map((skill) => (
              <SkillCard key={skill.name} skill={skill} />
            ))}
          </div>
        )}
      </section>
    </div>
  )
}
