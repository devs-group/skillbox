"use client"

import { useEffect, useState, useCallback } from "react"
import { Terminal } from "lucide-react"
import { SearchBar } from "@/components/SearchBar"
import { SkillCard, type Skill } from "@/components/SkillCard"
import { apiFetch } from "@/lib/api"

const PAGE_SIZE = 20

function BlinkDot() {
  return <span className="inline-block h-2 w-2 bg-[#ea580c] animate-blink" />
}

export default function SkillsPage() {
  const [skills, setSkills] = useState<Skill[]>([])
  const [loading, setLoading] = useState(true)
  const [query, setQuery] = useState("")
  const [offset, setOffset] = useState(0)
  const [hasMore, setHasMore] = useState(false)

  const fetchSkills = useCallback(async (q: string, off: number) => {
    setLoading(true)
    try {
      const params = new URLSearchParams({
        limit: String(PAGE_SIZE),
        offset: String(off),
      })
      if (q) params.set("q", q)
      const data = await apiFetch<{ skills: Skill[]; total?: number }>(
        `/v1/marketplace/skills?${params}`
      )
      const fetched = data.skills ?? []
      setSkills(off === 0 ? fetched : (prev) => [...prev, ...fetched])
      setHasMore(fetched.length === PAGE_SIZE)
    } catch {
      if (off === 0) setSkills([])
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchSkills(query, 0)
    setOffset(0)
  }, [query, fetchSkills])

  const loadMore = () => {
    const next = offset + PAGE_SIZE
    setOffset(next)
    fetchSkills(query, next)
  }

  const handleSearch = useCallback((q: string) => {
    setQuery(q)
  }, [])

  return (
    <div className="min-h-screen dot-grid-bg">
      <div className="w-full px-6 py-12 lg:px-12">
        {/* Section label */}
        <div className="flex items-center gap-4 mb-8">
          <span className="text-[10px] tracking-[0.2em] uppercase text-muted-foreground font-mono">
            {"// SECTION: BROWSE_SKILLS"}
          </span>
          <div className="flex-1 border-t border-border" />
          <BlinkDot />
          <span className="text-[10px] tracking-[0.2em] uppercase text-muted-foreground font-mono">
            001
          </span>
        </div>

        {/* Header + Search */}
        <div className="flex flex-col lg:flex-row lg:items-end lg:justify-between gap-6 mb-12">
          <div className="flex flex-col gap-3">
            <h1 className="text-2xl lg:text-3xl font-mono font-bold tracking-tight uppercase text-foreground">
              Skill Catalog
            </h1>
            <p className="text-xs lg:text-sm font-mono text-muted-foreground leading-relaxed max-w-md">
              Browse all available skills. Install any skill with a single command.
            </p>
          </div>
          <SearchBar onSearch={handleSearch} />
        </div>

        {/* Skills Grid */}
        {loading && skills.length === 0 ? (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-0">
            {Array.from({ length: 8 }).map((_, i) => (
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
          <>
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-0">
              {skills.map((skill) => (
                <SkillCard key={skill.name} skill={skill} />
              ))}
            </div>
            {hasMore && (
              <div className="flex justify-center mt-8">
                <button
                  onClick={loadMore}
                  disabled={loading}
                  className="bg-foreground text-background px-6 py-2.5 text-xs font-mono tracking-widest uppercase disabled:opacity-50 hover:bg-foreground/80 transition-colors"
                >
                  {loading ? "Loading..." : "Load More"}
                </button>
              </div>
            )}
          </>
        )}
      </div>
    </div>
  )
}
