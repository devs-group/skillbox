"use client"

import { useEffect } from "react"
import { useRouter } from "next/navigation"
import { useAuth } from "@/lib/auth-context"
import { AdminNav } from "@/components/AdminNav"

export default function AdminLayout({ children }: { children: React.ReactNode }) {
  const { isAdmin, loading } = useAuth()
  const router = useRouter()

  useEffect(() => {
    if (!loading && !isAdmin) {
      router.push("/auth/login")
    }
  }, [loading, isAdmin, router])

  if (loading) {
    return (
      <div className="min-h-screen dot-grid-bg">
        <div className="w-full px-6 py-12 lg:px-12">
          <div className="h-8 w-48 bg-muted animate-pulse" />
        </div>
      </div>
    )
  }

  if (!isAdmin) return null

  return (
    <div className="min-h-screen dot-grid-bg">
      <div className="w-full px-6 py-12 lg:px-12">
        {/* Section label */}
        <div className="flex items-center gap-4 mb-8">
          <span className="text-[10px] tracking-[0.2em] uppercase text-muted-foreground font-mono">
            {"// SECTION: ADMIN_DASHBOARD"}
          </span>
          <div className="flex-1 border-t border-border" />
          <span className="inline-block h-2 w-2 bg-[#ea580c] animate-blink" />
          <span className="text-[10px] tracking-[0.2em] uppercase text-muted-foreground font-mono">
            SYS
          </span>
        </div>

        <h1 className="text-2xl lg:text-3xl font-mono font-bold tracking-tight uppercase mb-8">
          Admin Dashboard
        </h1>

        <div className="flex flex-col md:flex-row gap-8">
          <aside className="w-full md:w-56 shrink-0">
            <AdminNav />
          </aside>
          <div className="flex-1 min-w-0">{children}</div>
        </div>
      </div>
    </div>
  )
}
