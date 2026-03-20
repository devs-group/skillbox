"use client"

import { useState, useCallback } from "react"
import { SearchBar } from "@/components/SearchBar"
import { apiFetch } from "@/lib/api"
import Link from "next/link"

interface GitHubSkill {
  name: string
  description: string
  repo_owner: string
  repo_name: string
  file_path: string
  stars: number
  html_url: string
}

interface SearchResponse {
  results: GitHubSkill[]
  total_count: number
  has_more: boolean
}

export default function GitHubSearchPage() {
  const [results, setResults] = useState<GitHubSkill[]>([])
  const [loading, setLoading] = useState(false)
  const [searched, setSearched] = useState(false)
  const [totalCount, setTotalCount] = useState(0)
  const [hasMore, setHasMore] = useState(false)
  const [page, setPage] = useState(1)
  const [query, setQuery] = useState("")

  const search = useCallback(async (q: string, p = 1) => {
    if (!q || q.length < 2) return
    setLoading(true)
    setSearched(true)
    setQuery(q)
    setPage(p)
    try {
      const params = new URLSearchParams({ q, page: String(p) })
      const data = await apiFetch<SearchResponse>(`/v1/github/search?${params}`)
      if (p === 1) {
        setResults(data.results ?? [])
      } else {
        setResults((prev) => [...prev, ...(data.results ?? [])])
      }
      setTotalCount(data.total_count)
      setHasMore(data.has_more)
    } catch {
      if (p === 1) setResults([])
    } finally {
      setLoading(false)
    }
  }, [])

  return (
    <div className="container mx-auto px-4 py-8">
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4 mb-8">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">GitHub Skills</h1>
          <p className="text-muted-foreground mt-1">Discover skills from public GitHub repositories</p>
        </div>
        <SearchBar placeholder="Search GitHub for skills..." onSearch={(q: string) => search(q, 1)} debounceMs={500} />
      </div>

      {!searched ? (
        <div className="text-center py-16">
          <p className="text-muted-foreground">Search GitHub for skills containing SKILL.md files</p>
          <p className="text-sm text-muted-foreground mt-2">Try: &quot;code review&quot;, &quot;testing&quot;, &quot;refactoring&quot;</p>
        </div>
      ) : loading && results.length === 0 ? (
        <div className="space-y-4">
          {Array.from({ length: 6 }).map((_, i) => (
            <div key={i} className="h-20 rounded-lg bg-muted animate-pulse" />
          ))}
        </div>
      ) : results.length === 0 ? (
        <p className="text-center text-muted-foreground py-12">No skills found on GitHub.</p>
      ) : (
        <div className="space-y-3">
          {totalCount > 0 && (
            <p className="text-sm text-muted-foreground mb-4">{totalCount} results found</p>
          )}
          {results.map((skill, i) => (
            <div key={`${skill.repo_owner}/${skill.repo_name}/${skill.file_path}-${i}`}
              className="border rounded-lg p-4 hover:border-primary/50 transition-colors">
              <div className="flex items-start justify-between gap-4">
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2 mb-1">
                    <h3 className="font-semibold text-base">{skill.name || skill.repo_name}</h3>
                    <span className="text-xs text-muted-foreground flex items-center gap-1">
                      ★ {skill.stars}
                    </span>
                  </div>
                  <p className="text-sm text-muted-foreground mb-2">
                    {skill.repo_owner}/{skill.repo_name}
                  </p>
                  {skill.description && (
                    <p className="text-sm text-muted-foreground line-clamp-2">{skill.description}</p>
                  )}
                </div>
                <div className="flex items-center gap-2 shrink-0">
                  <Link
                    href={`/github/${encodeURIComponent(skill.repo_owner)}/${encodeURIComponent(skill.repo_name)}?path=${encodeURIComponent(skill.file_path)}`}
                    className="inline-flex items-center px-3 py-1.5 rounded-md border text-sm font-medium hover:bg-accent transition-colors"
                  >
                    Preview
                  </Link>
                  <a
                    href={skill.html_url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="inline-flex items-center px-3 py-1.5 rounded-md border text-sm text-muted-foreground hover:bg-accent transition-colors"
                  >
                    GitHub ↗
                  </a>
                </div>
              </div>
            </div>
          ))}
          {hasMore && (
            <div className="flex justify-center mt-6">
              <button
                onClick={() => search(query, page + 1)}
                disabled={loading}
                className="px-4 py-2 rounded-md border text-sm font-medium hover:bg-accent disabled:opacity-50 transition-colors"
              >
                {loading ? "Loading..." : "Load More"}
              </button>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
