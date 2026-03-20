const statusConfig: Record<string, { label: string; color: string }> = {
  approved: { label: "APPROVED", color: "bg-[#ea580c] text-background" },
  pending: { label: "PENDING", color: "bg-muted text-muted-foreground border border-foreground/30" },
  rejected: { label: "REJECTED", color: "bg-destructive/10 text-destructive" },
}

export function ApprovalBadge({ status }: { status: string }) {
  const config = statusConfig[status] ?? { label: status.toUpperCase(), color: "border border-foreground/30 text-foreground" }
  return (
    <span className={`text-[9px] tracking-[0.15em] uppercase font-mono px-2 py-0.5 ${config.color}`}>
      {config.label}
    </span>
  )
}
