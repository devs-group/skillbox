"use client"

import { useEffect, useState } from "react"
import { apiFetch } from "@/lib/api"
import { CheckCircle, Users, FolderTree, Clock } from "lucide-react"

interface DashboardStats {
  pending_approvals: number
  total_users: number
  total_groups: number
  total_skills: number
}

export default function AdminDashboardPage() {
  const [stats, setStats] = useState<DashboardStats>({
    pending_approvals: 0,
    total_users: 0,
    total_groups: 0,
    total_skills: 0,
  })

  useEffect(() => {
    async function loadStats() {
      try {
        const data = await apiFetch<DashboardStats>("/v1/admin/stats")
        setStats(data)
      } catch {
        // Stats endpoint may not exist yet; keep defaults
      }
    }
    loadStats()
  }, [])

  const cards = [
    { label: "PENDING_APPROVALS", value: stats.pending_approvals, icon: Clock },
    { label: "TOTAL_USERS", value: stats.total_users, icon: Users },
    { label: "TOTAL_GROUPS", value: stats.total_groups, icon: FolderTree },
    { label: "TOTAL_SKILLS", value: stats.total_skills, icon: CheckCircle },
  ]

  return (
    <div>
      <div className="flex items-center gap-4 mb-6">
        <span className="text-[10px] tracking-[0.2em] uppercase text-muted-foreground font-mono">
          {"// OVERVIEW"}
        </span>
        <div className="flex-1 border-t border-border" />
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-0">
        {cards.map((card) => {
          const Icon = card.icon
          return (
            <div
              key={card.label}
              className="flex flex-col border-2 border-foreground px-4 py-4"
            >
              <div className="flex items-center justify-between mb-3">
                <span className="text-[9px] tracking-[0.15em] uppercase text-muted-foreground font-mono">
                  {card.label}
                </span>
                <Icon size={14} strokeWidth={1.5} className="text-muted-foreground" />
              </div>
              <span className="text-2xl lg:text-3xl font-mono font-bold tracking-tight">
                {card.value}
              </span>
            </div>
          )
        })}
      </div>
    </div>
  )
}
