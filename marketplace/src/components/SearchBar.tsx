"use client"

import { useEffect, useState } from "react"
import { Search } from "lucide-react"

interface SearchBarProps {
  placeholder?: string
  onSearch: (query: string) => void
  debounceMs?: number
}

export function SearchBar({ placeholder = "Search skills...", onSearch, debounceMs = 300 }: SearchBarProps) {
  const [value, setValue] = useState("")

  useEffect(() => {
    const timer = setTimeout(() => {
      onSearch(value)
    }, debounceMs)
    return () => clearTimeout(timer)
  }, [value, debounceMs, onSearch])

  return (
    <div className="relative w-full max-w-lg">
      <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
      <input
        type="search"
        placeholder={placeholder}
        value={value}
        onChange={(e) => setValue(e.target.value)}
        className="w-full h-10 border-2 border-foreground bg-background px-10 text-xs font-mono tracking-wider uppercase placeholder:text-muted-foreground placeholder:normal-case placeholder:tracking-normal focus:outline-none focus:ring-2 focus:ring-[#ea580c] focus:border-[#ea580c] transition-colors"
      />
    </div>
  )
}
