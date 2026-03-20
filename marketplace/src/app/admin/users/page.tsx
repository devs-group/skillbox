"use client"

import { useEffect, useState, useCallback } from "react"
import { apiFetch } from "@/lib/api"
import { toast } from "sonner"

interface User {
  id: string
  email: string
  role: string
  created_at: string
}

const roles = ["viewer", "publisher", "admin"]

export default function UsersPage() {
  const [users, setUsers] = useState<User[]>([])
  const [loading, setLoading] = useState(true)

  const fetchUsers = useCallback(async () => {
    setLoading(true)
    try {
      const data = await apiFetch<{ users: User[] }>("/v1/users")
      setUsers(data.users ?? [])
    } catch {
      setUsers([])
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchUsers()
  }, [fetchUsers])

  const handleRoleChange = async (userId: string, newRole: string) => {
    try {
      await apiFetch(`/v1/users/${userId}/role`, {
        method: "PUT",
        body: JSON.stringify({ role: newRole }),
      })
      toast.success("Role updated")
      setUsers((prev) =>
        prev.map((u) => (u.id === userId ? { ...u, role: newRole } : u))
      )
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to update role")
    }
  }

  return (
    <div>
      <div className="flex items-center gap-4 mb-6">
        <span className="text-[10px] tracking-[0.2em] uppercase text-muted-foreground font-mono">
          {"// USER_MANAGEMENT"}
        </span>
        <div className="flex-1 border-t border-border" />
      </div>

      {loading ? (
        <div className="space-y-0">
          {Array.from({ length: 5 }).map((_, i) => (
            <div key={i} className="h-12 bg-muted/30 animate-pulse border border-foreground/10" />
          ))}
        </div>
      ) : users.length === 0 ? (
        <div className="border-2 border-foreground p-8 text-center">
          <p className="text-xs font-mono tracking-widest uppercase text-muted-foreground">
            No users found.
          </p>
        </div>
      ) : (
        <div className="border-2 border-foreground">
          <div className="grid grid-cols-4 gap-2 px-4 py-2 border-b-2 border-foreground bg-muted/30">
            <span className="text-[9px] tracking-[0.15em] uppercase text-muted-foreground font-mono">Email</span>
            <span className="text-[9px] tracking-[0.15em] uppercase text-muted-foreground font-mono">Role</span>
            <span className="text-[9px] tracking-[0.15em] uppercase text-muted-foreground font-mono">Joined</span>
            <span className="text-[9px] tracking-[0.15em] uppercase text-muted-foreground font-mono text-right">Change Role</span>
          </div>
          {users.map((user) => (
            <div
              key={user.id}
              className="grid grid-cols-4 gap-2 px-4 py-3 border-b border-foreground/10 last:border-b-0 items-center"
            >
              <span className="text-xs font-mono font-bold truncate">{user.email}</span>
              <span className={`text-[9px] tracking-[0.15em] uppercase font-mono px-2 py-0.5 w-fit ${
                user.role === "admin"
                  ? "bg-[#ea580c] text-background"
                  : "border border-foreground/30 text-muted-foreground"
              }`}>
                {user.role}
              </span>
              <span className="text-[10px] font-mono text-muted-foreground">
                {new Date(user.created_at).toLocaleDateString()}
              </span>
              <div className="flex justify-end">
                <select
                  value={user.role}
                  onChange={(e) => handleRoleChange(user.id, e.target.value)}
                  className="border-2 border-foreground bg-background px-2 py-1 text-[10px] font-mono tracking-widest uppercase focus:outline-none focus:border-[#ea580c] cursor-pointer"
                >
                  {roles.map((role) => (
                    <option key={role} value={role}>
                      {role}
                    </option>
                  ))}
                </select>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
