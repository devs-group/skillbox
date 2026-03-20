"use client"

import { useEffect, useState, useCallback } from "react"
import { apiFetch } from "@/lib/api"
import { ApprovalBadge } from "@/components/ApprovalBadge"
import { toast } from "sonner"

interface Approval {
  id: string
  skill_name: string
  requester: string
  status: string
  created_at: string
  comment?: string
}

type StatusFilter = "all" | "pending" | "approved" | "rejected"

export default function ApprovalsPage() {
  const [approvals, setApprovals] = useState<Approval[]>([])
  const [loading, setLoading] = useState(true)
  const [filter, setFilter] = useState<StatusFilter>("all")
  const [actionDialog, setActionDialog] = useState<{
    approval: Approval
    action: "approve" | "reject"
  } | null>(null)
  const [comment, setComment] = useState("")
  const [submitting, setSubmitting] = useState(false)

  const fetchApprovals = useCallback(async () => {
    setLoading(true)
    try {
      const params = new URLSearchParams()
      if (filter !== "all") params.set("status", filter)
      const data = await apiFetch<{ approvals: Approval[] }>(
        `/v1/approvals?${params}`
      )
      setApprovals(data.approvals ?? [])
    } catch {
      setApprovals([])
    } finally {
      setLoading(false)
    }
  }, [filter])

  useEffect(() => {
    fetchApprovals()
  }, [fetchApprovals])

  const handleAction = async () => {
    if (!actionDialog) return
    setSubmitting(true)
    try {
      await apiFetch(`/v1/approvals/${actionDialog.approval.id}/${actionDialog.action}`, {
        method: "POST",
        body: JSON.stringify({ comment }),
      })
      toast.success(
        actionDialog.action === "approve" ? "Approved" : "Rejected"
      )
      setActionDialog(null)
      setComment("")
      fetchApprovals()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Action failed")
    } finally {
      setSubmitting(false)
    }
  }

  const filters: StatusFilter[] = ["all", "pending", "approved", "rejected"]

  return (
    <div>
      <div className="flex items-center gap-4 mb-6">
        <span className="text-[10px] tracking-[0.2em] uppercase text-muted-foreground font-mono">
          {"// APPROVAL_REQUESTS"}
        </span>
        <div className="flex-1 border-t border-border" />
      </div>

      {/* Filter tabs */}
      <div className="flex gap-0 mb-6 border-2 border-foreground w-fit">
        {filters.map((f) => (
          <button
            key={f}
            onClick={() => setFilter(f)}
            className={`px-4 py-2 text-[10px] font-mono tracking-widest uppercase transition-colors border-r border-foreground/20 last:border-r-0 ${
              filter === f
                ? "bg-foreground text-background"
                : "text-muted-foreground hover:bg-foreground/5 hover:text-foreground"
            }`}
          >
            {f}
          </button>
        ))}
      </div>

      {loading ? (
        <div className="space-y-0">
          {Array.from({ length: 5 }).map((_, i) => (
            <div key={i} className="h-12 bg-muted/30 animate-pulse border border-foreground/10" />
          ))}
        </div>
      ) : approvals.length === 0 ? (
        <div className="border-2 border-foreground p-8 text-center">
          <p className="text-xs font-mono tracking-widest uppercase text-muted-foreground">
            No approval requests found.
          </p>
        </div>
      ) : (
        <div className="border-2 border-foreground">
          {/* Table header */}
          <div className="grid grid-cols-5 gap-2 px-4 py-2 border-b-2 border-foreground bg-muted/30">
            <span className="text-[9px] tracking-[0.15em] uppercase text-muted-foreground font-mono">Skill</span>
            <span className="text-[9px] tracking-[0.15em] uppercase text-muted-foreground font-mono">Requester</span>
            <span className="text-[9px] tracking-[0.15em] uppercase text-muted-foreground font-mono">Status</span>
            <span className="text-[9px] tracking-[0.15em] uppercase text-muted-foreground font-mono">Requested</span>
            <span className="text-[9px] tracking-[0.15em] uppercase text-muted-foreground font-mono text-right">Actions</span>
          </div>
          {approvals.map((a) => (
            <div
              key={a.id}
              className="grid grid-cols-5 gap-2 px-4 py-3 border-b border-foreground/10 last:border-b-0 items-center"
            >
              <span className="text-xs font-mono font-bold truncate">{a.skill_name}</span>
              <span className="text-xs font-mono text-muted-foreground truncate">{a.requester}</span>
              <div><ApprovalBadge status={a.status} /></div>
              <span className="text-[10px] font-mono text-muted-foreground">
                {new Date(a.created_at).toLocaleDateString()}
              </span>
              <div className="flex gap-2 justify-end">
                {a.status === "pending" && (
                  <>
                    <button
                      onClick={() => setActionDialog({ approval: a, action: "approve" })}
                      className="bg-foreground text-background px-3 py-1 text-[10px] font-mono tracking-widest uppercase hover:bg-foreground/80 transition-colors"
                    >
                      Approve
                    </button>
                    <button
                      onClick={() => setActionDialog({ approval: a, action: "reject" })}
                      className="bg-destructive/10 text-destructive px-3 py-1 text-[10px] font-mono tracking-widest uppercase hover:bg-destructive/20 transition-colors"
                    >
                      Reject
                    </button>
                  </>
                )}
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Action dialog overlay */}
      {actionDialog && (
        <div className="fixed inset-0 bg-foreground/50 flex items-center justify-center z-50 p-4">
          <div className="bg-background border-2 border-foreground w-full max-w-md">
            <div className="flex items-center justify-between px-5 py-3 border-b-2 border-foreground">
              <span className="text-[10px] tracking-[0.2em] uppercase font-mono">
                {actionDialog.action === "approve" ? "APPROVE" : "REJECT"} REQUEST
              </span>
            </div>
            <div className="px-5 py-4">
              <p className="text-xs font-mono text-muted-foreground mb-4">
                {actionDialog.action === "approve" ? "Approve" : "Reject"}{" "}
                <strong className="text-foreground">{actionDialog.approval.skill_name}</strong> requested by{" "}
                <strong className="text-foreground">{actionDialog.approval.requester}</strong>?
              </p>
              <textarea
                placeholder="Optional comment..."
                value={comment}
                onChange={(e) => setComment(e.target.value)}
                className="w-full border-2 border-foreground bg-background px-3 py-2 text-xs font-mono focus:outline-none focus:border-[#ea580c] min-h-[80px]"
              />
            </div>
            <div className="flex items-center justify-end gap-2 px-5 py-3 border-t-2 border-foreground">
              <button
                onClick={() => { setActionDialog(null); setComment("") }}
                className="border border-foreground px-4 py-2 text-[10px] font-mono tracking-widest uppercase hover:bg-muted transition-colors"
              >
                Cancel
              </button>
              <button
                onClick={handleAction}
                disabled={submitting}
                className={`px-4 py-2 text-[10px] font-mono tracking-widest uppercase disabled:opacity-50 ${
                  actionDialog.action === "reject"
                    ? "bg-destructive/10 text-destructive hover:bg-destructive/20"
                    : "bg-foreground text-background hover:bg-foreground/80"
                } transition-colors`}
              >
                {submitting
                  ? "Processing..."
                  : actionDialog.action === "approve"
                    ? "Approve"
                    : "Reject"}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
