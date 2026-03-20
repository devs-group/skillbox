"use client"

import { useEffect, useState, use } from "react"
import { useSearchParams } from "next/navigation"
import { apiFetch } from "@/lib/api"
import { CopyCommand } from "@/components/CopyCommand"
import { useAuth } from "@/lib/auth-context"

interface PreviewData {
  name: string
  description: string
  version: string
  lang: string
  instructions: string
  repo_owner: string
  repo_name: string
  file_path: string
  files: { path: string; size: number }[]
}

export default function GitHubPreviewPage({
  params,
}: {
  params: Promise<{ owner: string; repo: string }>
}) {
  const { owner, repo } = use(params)
  const searchParams = useSearchParams()
  const filePath = searchParams.get("path") || "SKILL.md"
  const { session } = useAuth()
  const [preview, setPreview] = useState<PreviewData | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState("")
  const [installing, setInstalling] = useState(false)
  const [installed, setInstalled] = useState(false)

  useEffect(() => {
    async function load() {
      try {
        const qs = new URLSearchParams({ owner, repo, path: filePath })
        const data = await apiFetch<PreviewData>(`/v1/github/preview?${qs}`)
        setPreview(data)
      } catch (err) {
        setError(err instanceof Error ? err.message : "Failed to load preview")
      } finally {
        setLoading(false)
      }
    }
    load()
  }, [owner, repo, filePath])

  const handleInstall = async () => {
    if (!preview) return
    setInstalling(true)
    try {
      await apiFetch("/v1/github/install", {
        method: "POST",
        body: JSON.stringify({
          repo_owner: owner,
          repo_name: repo,
          file_path: filePath,
        }),
      })
      setInstalled(true)
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to install")
    } finally {
      setInstalling(false)
    }
  }

  if (loading) {
    return (
      <div className="container mx-auto px-4 py-12">
        <div className="h-8 w-64 bg-muted animate-pulse rounded mb-4" />
        <div className="h-4 w-96 bg-muted animate-pulse rounded mb-2" />
        <div className="h-64 bg-muted animate-pulse rounded" />
      </div>
    )
  }

  if (error || !preview) {
    return (
      <div className="container mx-auto px-4 py-12 text-center">
        <p className="text-destructive">{error || "Skill not found"}</p>
      </div>
    )
  }

  return (
    <div className="container mx-auto px-4 py-12 max-w-3xl">
      <div className="mb-6">
        <h1 className="text-3xl font-bold tracking-tight mb-2">{preview.name || repo}</h1>
        <p className="text-sm text-muted-foreground">
          {owner}/{repo} {preview.version && `\u2022 v${preview.version}`} {preview.lang && `\u2022 ${preview.lang}`}
        </p>
      </div>

      {preview.description && (
        <p className="text-muted-foreground mb-6">{preview.description}</p>
      )}

      {/* Install section */}
      <div className="border rounded-lg p-4 mb-6">
        <div className="flex items-center justify-between gap-4">
          <div>
            <p className="text-sm font-medium mb-1">Install this skill</p>
            <CopyCommand name={preview.name || repo} />
          </div>
          {session && (
            <button
              onClick={handleInstall}
              disabled={installing || installed}
              className="px-4 py-2 rounded-md bg-primary text-primary-foreground text-sm font-medium disabled:opacity-50 transition-colors"
            >
              {installed ? "Installed!" : installing ? "Installing..." : "Install to Registry"}
            </button>
          )}
        </div>
      </div>

      {/* Files */}
      {preview.files && preview.files.length > 0 && (
        <div className="border rounded-lg p-4 mb-6">
          <p className="text-sm font-medium mb-3">Files ({preview.files.length})</p>
          <div className="space-y-1">
            {preview.files.map((f) => (
              <div key={f.path} className="flex items-center justify-between text-sm py-1">
                <code className="text-xs bg-muted px-2 py-0.5 rounded">{f.path}</code>
                <span className="text-xs text-muted-foreground">{(f.size / 1024).toFixed(1)} KB</span>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Instructions */}
      {preview.instructions && (
        <div className="border rounded-lg p-4">
          <p className="text-sm font-medium mb-3">Instructions</p>
          <div className="prose prose-sm max-w-none text-muted-foreground whitespace-pre-wrap">
            {preview.instructions}
          </div>
        </div>
      )}
    </div>
  )
}
