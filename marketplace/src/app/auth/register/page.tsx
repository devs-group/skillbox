"use client"

import { useEffect, useState } from "react"
import { useRouter } from "next/navigation"
import type { RegistrationFlow, UiNode, UiNodeInputAttributes } from "@ory/client"
import { ory } from "@/lib/ory"
import { useAuth } from "@/lib/auth-context"
import Link from "next/link"

function isInputNode(node: UiNode): node is UiNode & { attributes: UiNodeInputAttributes } {
  return node.type === "input"
}

export default function RegisterPage() {
  const router = useRouter()
  const { refresh } = useAuth()
  const [flow, setFlow] = useState<RegistrationFlow | null>(null)
  const [values, setValues] = useState<Record<string, string>>({})
  const [error, setError] = useState("")
  const [submitting, setSubmitting] = useState(false)

  useEffect(() => {
    ory
      .createBrowserRegistrationFlow()
      .then(({ data }) => {
        setFlow(data)
        const defaults: Record<string, string> = {}
        data.ui.nodes.forEach((node) => {
          if (isInputNode(node)) {
            const attrs = node.attributes
            if (attrs.value) defaults[attrs.name] = String(attrs.value)
          }
        })
        setValues(defaults)
      })
      .catch(() => setError("Failed to initialize registration flow"))
  }, [])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!flow) return
    setSubmitting(true)
    setError("")
    try {
      await ory.updateRegistrationFlow({
        flow: flow.id,
        updateRegistrationFlowBody: {
          method: "password",
          password: values.password ?? "",
          traits: {
            email: values["traits.email"] ?? "",
            name: {
              first: values["traits.name.first"] ?? "",
              last: values["traits.name.last"] ?? "",
            },
          },
          csrf_token: values.csrf_token,
        },
      })
      await refresh()
      router.push("/")
    } catch (err: unknown) {
      const oryErr = err as { response?: { data?: RegistrationFlow } }
      if (oryErr.response?.data) {
        setFlow(oryErr.response.data)
        const msgs = oryErr.response.data.ui.messages
        if (msgs && msgs.length > 0) {
          setError(msgs.map((m) => m.text).join(". "))
        } else {
          setError("Registration failed")
        }
      } else {
        setError("Registration failed. Please try again.")
      }
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="min-h-[calc(100vh-5rem)] dot-grid-bg flex items-center justify-center px-4">
      <div className="w-full max-w-md border-2 border-foreground bg-background">
        {/* Header bar */}
        <div className="flex items-center justify-between px-5 py-3 border-b-2 border-foreground">
          <span className="text-[10px] tracking-[0.2em] uppercase text-muted-foreground font-mono">
            AUTH: REGISTER
          </span>
          <span className="inline-block h-2 w-2 bg-[#ea580c] animate-blink" />
        </div>

        <div className="px-5 py-6">
          <h1 className="text-xl font-mono font-bold tracking-tight uppercase mb-1">
            Create an account
          </h1>
          <p className="text-xs font-mono text-muted-foreground mb-6">
            Sign up to get started
          </p>

          {error && (
            <div className="border border-destructive/50 bg-destructive/10 px-3 py-2 mb-4">
              <p className="text-xs font-mono text-destructive">{error}</p>
            </div>
          )}

          <form onSubmit={handleSubmit} className="flex flex-col gap-4">
            {flow?.ui.nodes.filter(isInputNode).map((node) => {
              const attrs = node.attributes
              if (attrs.type === "hidden" || attrs.type === "submit") return null
              return (
                <div key={attrs.name} className="flex flex-col gap-2">
                  <label
                    htmlFor={attrs.name}
                    className="text-[10px] tracking-[0.2em] uppercase text-muted-foreground font-mono"
                  >
                    {node.meta.label?.text ?? attrs.name}
                  </label>
                  <input
                    id={attrs.name}
                    name={attrs.name}
                    type={attrs.type === "password" ? "password" : attrs.type === "email" ? "email" : "text"}
                    required={attrs.required}
                    value={values[attrs.name] ?? ""}
                    onChange={(e) =>
                      setValues((v) => ({ ...v, [attrs.name]: e.target.value }))
                    }
                    className="border-2 border-foreground bg-background px-3 py-2 text-xs font-mono focus:outline-none focus:border-[#ea580c] transition-colors"
                  />
                </div>
              )
            })}
            <div className="flex flex-col gap-2">
              <label
                htmlFor="password"
                className="text-[10px] tracking-[0.2em] uppercase text-muted-foreground font-mono"
              >
                Password
              </label>
              <input
                id="password"
                name="password"
                type="password"
                required
                value={values.password ?? ""}
                onChange={(e) =>
                  setValues((v) => ({ ...v, password: e.target.value }))
                }
                className="border-2 border-foreground bg-background px-3 py-2 text-xs font-mono focus:outline-none focus:border-[#ea580c] transition-colors"
              />
            </div>
            <button
              type="submit"
              disabled={submitting || !flow}
              className="w-full bg-foreground text-background py-2.5 text-xs font-mono tracking-widest uppercase disabled:opacity-50 hover:bg-foreground/80 transition-colors"
            >
              {submitting ? "Creating account..." : "Register"}
            </button>
          </form>
        </div>

        <div className="px-5 py-3 border-t-2 border-foreground text-center">
          <p className="text-[10px] font-mono text-muted-foreground tracking-wider">
            Already have an account?{" "}
            <Link href="/auth/login" className="text-[#ea580c] hover:underline uppercase">
              Sign in
            </Link>
          </p>
        </div>
      </div>
    </div>
  )
}
