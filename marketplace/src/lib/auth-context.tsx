"use client"

import { createContext, useContext, useEffect, useState, useCallback } from "react"
import type { Session, Identity } from "@ory/client"
import { ory } from "./ory"

interface AuthState {
  session: Session | null
  identity: Identity | null
  loading: boolean
  isAdmin: boolean
  logout: () => Promise<void>
  refresh: () => Promise<void>
}

const AuthContext = createContext<AuthState>({
  session: null,
  identity: null,
  loading: true,
  isAdmin: false,
  logout: async () => {},
  refresh: async () => {},
})

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [session, setSession] = useState<Session | null>(null)
  const [loading, setLoading] = useState(true)

  const refresh = useCallback(async () => {
    try {
      const { data } = await ory.toSession()
      setSession(data)
    } catch {
      setSession(null)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    refresh()
  }, [refresh])

  const logout = useCallback(async () => {
    try {
      const { data } = await ory.createBrowserLogoutFlow()
      await ory.updateLogoutFlow({ token: data.logout_token })
      setSession(null)
      window.location.href = "/"
    } catch {
      // already logged out
      setSession(null)
    }
  }, [])

  const identity = session?.identity ?? null
  const metaPublic = (identity?.metadata_public as Record<string, unknown>) ?? {}
  const isAdmin = metaPublic?.role === "admin"

  return (
    <AuthContext.Provider value={{ session, identity, loading, isAdmin, logout, refresh }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  return useContext(AuthContext)
}
