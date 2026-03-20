"use client"

import { useState } from "react"
import { Check, Copy } from "lucide-react"
import { toast } from "sonner"

export function CopyCommand({ name }: { name: string }) {
  const [copied, setCopied] = useState(false)
  const command = `skillbox add ${name}`

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(command)
      setCopied(true)
      toast.success("Copied to clipboard")
      setTimeout(() => setCopied(false), 2000)
    } catch {
      toast.error("Failed to copy")
    }
  }

  return (
    <div className="flex items-center gap-0 w-full border border-foreground/30">
      <code className="flex-1 text-[10px] bg-foreground text-background px-3 py-2 font-mono truncate tracking-wider">
        {command}
      </code>
      <button
        onClick={handleCopy}
        className="shrink-0 h-8 w-8 flex items-center justify-center bg-[#ea580c] text-background hover:bg-[#ea580c]/80 transition-colors"
      >
        {copied ? <Check size={12} strokeWidth={2} /> : <Copy size={12} strokeWidth={2} />}
      </button>
    </div>
  )
}
