"use client"

import Link from "next/link"
import { useAuth } from "@/lib/auth-context"
import { Cpu, Shield } from "lucide-react"
import { ThemeToggle } from "@/components/theme-toggle"

export function Navbar() {
  const { session, isAdmin, loading, logout } = useAuth()

  return (
    <div className="w-full px-4 pt-4 lg:px-6 lg:pt-6">
      <nav className="w-full border border-foreground/20 bg-background/80 backdrop-blur-sm px-6 py-3 lg:px-8">
        <div className="flex items-center justify-between">
          <Link href="/" className="flex items-center gap-3">
            <Cpu size={16} strokeWidth={1.5} />
            <span className="text-xs font-mono tracking-[0.15em] uppercase font-bold">
              SKILLBOX
            </span>
          </Link>

          <div className="hidden md:flex items-center gap-8">
            {[
              { label: "Browse", href: "/skills" },
              { label: "GitHub", href: "/github" },
              ...(isAdmin
                ? [{ label: "Admin", href: "/admin" }]
                : []),
            ].map((link) => (
              <Link
                key={link.label}
                href={link.href}
                className="text-xs font-mono tracking-widest uppercase text-muted-foreground hover:text-foreground transition-colors duration-200 flex items-center gap-1"
              >
                {link.label === "Admin" && <Shield size={12} strokeWidth={1.5} />}
                {link.label}
              </Link>
            ))}
          </div>

          <div className="flex items-center gap-4">
            <ThemeToggle />
            {loading ? null : session ? (
              <button
                onClick={logout}
                className="text-xs font-mono tracking-widest uppercase text-muted-foreground hover:text-foreground transition-colors duration-200"
              >
                Logout
              </button>
            ) : (
              <Link href="/auth/login">
                <span className="inline-block bg-foreground text-background px-4 py-2 text-xs font-mono tracking-widest uppercase hover:bg-foreground/80 transition-colors">
                  Login
                </span>
              </Link>
            )}
          </div>
        </div>
      </nav>
    </div>
  )
}
