"use client"

import { useEffect, useState, useCallback } from "react"
import { apiFetch } from "@/lib/api"
import { Plus, Users, Trash2 } from "lucide-react"
import { toast } from "sonner"

interface GroupMember {
  id: string
  email: string
}

interface Group {
  id: string
  name: string
  description: string
  members: GroupMember[]
}

export default function GroupsPage() {
  const [groups, setGroups] = useState<Group[]>([])
  const [loading, setLoading] = useState(true)
  const [createOpen, setCreateOpen] = useState(false)
  const [newName, setNewName] = useState("")
  const [newDescription, setNewDescription] = useState("")
  const [creating, setCreating] = useState(false)
  const [selectedGroup, setSelectedGroup] = useState<Group | null>(null)
  const [memberEmail, setMemberEmail] = useState("")

  const fetchGroups = useCallback(async () => {
    setLoading(true)
    try {
      const data = await apiFetch<{ groups: Group[] }>("/v1/groups")
      setGroups(data.groups ?? [])
    } catch {
      setGroups([])
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchGroups()
  }, [fetchGroups])

  const handleCreate = async () => {
    if (!newName.trim()) return
    setCreating(true)
    try {
      await apiFetch("/v1/groups", {
        method: "POST",
        body: JSON.stringify({ name: newName, description: newDescription }),
      })
      toast.success("Group created")
      setCreateOpen(false)
      setNewName("")
      setNewDescription("")
      fetchGroups()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to create group")
    } finally {
      setCreating(false)
    }
  }

  const handleAddMember = async () => {
    if (!selectedGroup || !memberEmail.trim()) return
    try {
      await apiFetch(`/v1/groups/${selectedGroup.id}/members`, {
        method: "POST",
        body: JSON.stringify({ email: memberEmail }),
      })
      toast.success("Member added")
      setMemberEmail("")
      fetchGroups()
      const updated = await apiFetch<Group>(`/v1/groups/${selectedGroup.id}`)
      setSelectedGroup(updated)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to add member")
    }
  }

  const handleRemoveMember = async (memberId: string) => {
    if (!selectedGroup) return
    try {
      await apiFetch(`/v1/groups/${selectedGroup.id}/members/${memberId}`, {
        method: "DELETE",
      })
      toast.success("Member removed")
      setSelectedGroup((g) =>
        g ? { ...g, members: g.members.filter((m) => m.id !== memberId) } : null
      )
      fetchGroups()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to remove member")
    }
  }

  return (
    <div>
      <div className="flex items-center gap-4 mb-6">
        <span className="text-[10px] tracking-[0.2em] uppercase text-muted-foreground font-mono">
          {"// GROUP_MANAGEMENT"}
        </span>
        <div className="flex-1 border-t border-border" />
        <button
          onClick={() => setCreateOpen(true)}
          className="flex items-center gap-2 bg-foreground text-background px-4 py-2 text-[10px] font-mono tracking-widest uppercase hover:bg-foreground/80 transition-colors"
        >
          <Plus size={12} />
          Create Group
        </button>
      </div>

      {loading ? (
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-0">
          {Array.from({ length: 4 }).map((_, i) => (
            <div key={i} className="h-32 bg-muted/30 animate-pulse border border-foreground/10" />
          ))}
        </div>
      ) : groups.length === 0 ? (
        <div className="border-2 border-foreground p-8 text-center">
          <p className="text-xs font-mono tracking-widest uppercase text-muted-foreground">
            No groups yet. Create one to get started.
          </p>
        </div>
      ) : (
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-0">
          {groups.map((group) => (
            <div
              key={group.id}
              onClick={() => setSelectedGroup(group)}
              className="border-2 border-foreground p-4 cursor-pointer hover:bg-foreground/5 transition-colors"
            >
              <div className="flex items-center justify-between mb-2">
                <span className="text-sm font-mono font-bold tracking-tight uppercase">
                  {group.name}
                </span>
                <span className="flex items-center gap-1 text-[9px] tracking-[0.15em] uppercase font-mono border border-foreground/30 px-2 py-0.5 text-muted-foreground">
                  <Users size={10} />
                  {group.members?.length ?? 0}
                </span>
              </div>
              <p className="text-xs font-mono text-muted-foreground leading-relaxed line-clamp-2">
                {group.description || "No description"}
              </p>
            </div>
          ))}
        </div>
      )}

      {/* Create Group Dialog */}
      {createOpen && (
        <div className="fixed inset-0 bg-foreground/50 flex items-center justify-center z-50 p-4">
          <div className="bg-background border-2 border-foreground w-full max-w-md">
            <div className="flex items-center justify-between px-5 py-3 border-b-2 border-foreground">
              <span className="text-[10px] tracking-[0.2em] uppercase font-mono">
                CREATE_GROUP
              </span>
            </div>
            <div className="px-5 py-4 flex flex-col gap-4">
              <div className="flex flex-col gap-2">
                <label className="text-[10px] tracking-[0.2em] uppercase text-muted-foreground font-mono">
                  Name
                </label>
                <input
                  value={newName}
                  onChange={(e) => setNewName(e.target.value)}
                  placeholder="Group name"
                  className="border-2 border-foreground bg-background px-3 py-2 text-xs font-mono focus:outline-none focus:border-[#ea580c]"
                />
              </div>
              <div className="flex flex-col gap-2">
                <label className="text-[10px] tracking-[0.2em] uppercase text-muted-foreground font-mono">
                  Description
                </label>
                <textarea
                  value={newDescription}
                  onChange={(e) => setNewDescription(e.target.value)}
                  placeholder="Optional description"
                  className="border-2 border-foreground bg-background px-3 py-2 text-xs font-mono focus:outline-none focus:border-[#ea580c] min-h-[60px]"
                />
              </div>
            </div>
            <div className="flex items-center justify-end gap-2 px-5 py-3 border-t-2 border-foreground">
              <button
                onClick={() => setCreateOpen(false)}
                className="border border-foreground px-4 py-2 text-[10px] font-mono tracking-widest uppercase hover:bg-muted transition-colors"
              >
                Cancel
              </button>
              <button
                onClick={handleCreate}
                disabled={creating || !newName.trim()}
                className="bg-foreground text-background px-4 py-2 text-[10px] font-mono tracking-widest uppercase disabled:opacity-50 hover:bg-foreground/80 transition-colors"
              >
                {creating ? "Creating..." : "Create"}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Group Detail Dialog */}
      {selectedGroup && (
        <div className="fixed inset-0 bg-foreground/50 flex items-center justify-center z-50 p-4">
          <div className="bg-background border-2 border-foreground w-full max-w-lg">
            <div className="flex items-center justify-between px-5 py-3 border-b-2 border-foreground">
              <span className="text-sm font-mono font-bold tracking-tight uppercase">
                {selectedGroup.name}
              </span>
              <button
                onClick={() => setSelectedGroup(null)}
                className="text-xs font-mono text-muted-foreground hover:text-foreground"
              >
                CLOSE
              </button>
            </div>
            <div className="px-5 py-4 space-y-4">
              <p className="text-xs font-mono text-muted-foreground leading-relaxed">
                {selectedGroup.description || "No description"}
              </p>

              <div className="flex items-center gap-0">
                <input
                  placeholder="Add member by email"
                  value={memberEmail}
                  onChange={(e) => setMemberEmail(e.target.value)}
                  onKeyDown={(e) => e.key === "Enter" && handleAddMember()}
                  className="flex-1 border-2 border-foreground bg-background px-3 py-2 text-xs font-mono focus:outline-none focus:border-[#ea580c]"
                />
                <button
                  onClick={handleAddMember}
                  disabled={!memberEmail.trim()}
                  className="bg-foreground text-background px-4 py-2 text-[10px] font-mono tracking-widest uppercase disabled:opacity-50 border-2 border-foreground border-l-0"
                >
                  Add
                </button>
              </div>

              {selectedGroup.members?.length > 0 ? (
                <div className="border-2 border-foreground">
                  <div className="grid grid-cols-[1fr_40px] gap-2 px-4 py-2 border-b-2 border-foreground bg-muted/30">
                    <span className="text-[9px] tracking-[0.15em] uppercase text-muted-foreground font-mono">Member</span>
                    <span />
                  </div>
                  {selectedGroup.members.map((member) => (
                    <div
                      key={member.id}
                      className="grid grid-cols-[1fr_40px] gap-2 px-4 py-2 border-b border-foreground/10 last:border-b-0 items-center"
                    >
                      <span className="text-xs font-mono">{member.email}</span>
                      <button
                        onClick={() => handleRemoveMember(member.id)}
                        className="h-7 w-7 flex items-center justify-center text-destructive hover:bg-destructive/10 transition-colors"
                      >
                        <Trash2 size={12} />
                      </button>
                    </div>
                  ))}
                </div>
              ) : (
                <div className="border border-foreground/20 p-4 text-center">
                  <p className="text-xs font-mono text-muted-foreground tracking-widest uppercase">
                    No members yet.
                  </p>
                </div>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
