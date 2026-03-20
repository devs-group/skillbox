import { test as base, type Page, type Route } from "@playwright/test"

// ── Mock Data ──────────────────────────────────────────────────────

export const MOCK_SKILLS = [
  {
    name: "code-review",
    description: "Automated code review with AI-powered suggestions",
    version: "1.2.0",
    provider: "claude",
    tags: ["review", "ai"],
  },
  {
    name: "test-generator",
    description: "Generate unit tests from existing code",
    version: "0.9.1",
    provider: "claude",
    tags: ["testing"],
  },
  {
    name: "sql-optimizer",
    description: "Optimize SQL queries for better performance",
    version: "2.0.0",
    provider: "claude",
    tags: ["sql", "performance"],
  },
  {
    name: "api-docs",
    description: "Auto-generate OpenAPI documentation from code",
    version: "1.0.0",
    provider: "claude",
    tags: ["docs", "api"],
  },
  {
    name: "refactor-assistant",
    description: "Suggest and apply code refactoring patterns",
    version: "0.5.0",
    provider: "claude",
    tags: ["refactoring"],
  },
  {
    name: "security-scanner",
    description: "Scan code for common security vulnerabilities",
    version: "3.1.0",
    provider: "claude",
    tags: ["security"],
  },
]

export const MOCK_USERS = [
  { id: "u1", email: "admin@example.com", role: "admin", created_at: "2026-01-15T10:00:00Z" },
  { id: "u2", email: "dev@example.com", role: "consumer", created_at: "2026-02-20T14:30:00Z" },
  { id: "u3", email: "publisher@example.com", role: "publisher", created_at: "2026-03-01T09:00:00Z" },
]

export const MOCK_APPROVALS = [
  { id: "a1", skill_name: "code-review", requester: "dev@example.com", status: "pending", created_at: "2026-03-18T12:00:00Z" },
  { id: "a2", skill_name: "sql-optimizer", requester: "dev@example.com", status: "approved", created_at: "2026-03-17T09:00:00Z", comment: "Looks good" },
  { id: "a3", skill_name: "security-scanner", requester: "publisher@example.com", status: "rejected", created_at: "2026-03-16T15:00:00Z", comment: "Needs review" },
  { id: "a4", skill_name: "test-generator", requester: "dev@example.com", status: "pending", created_at: "2026-03-19T08:00:00Z" },
]

export const MOCK_GROUPS = [
  { id: "g1", name: "Engineering", description: "Core engineering team", members: [{ id: "u2", email: "dev@example.com" }] },
  { id: "g2", name: "Security", description: "Security review team", members: [{ id: "u3", email: "publisher@example.com" }] },
]

// Ory Kratos mock session (authenticated as admin)
export const MOCK_SESSION_ADMIN = {
  id: "session-admin",
  active: true,
  identity: {
    id: "kratos-admin-id",
    traits: { email: "admin@example.com" },
    metadata_public: { role: "admin" },
  },
}

// Ory Kratos mock session (authenticated as consumer)
export const MOCK_SESSION_CONSUMER = {
  id: "session-consumer",
  active: true,
  identity: {
    id: "kratos-consumer-id",
    traits: { email: "dev@example.com" },
    metadata_public: { role: "consumer" },
  },
}

// Ory Kratos login flow
export const MOCK_LOGIN_FLOW = {
  id: "flow-login-123",
  type: "browser",
  ui: {
    action: "http://localhost:4433/self-service/login?flow=flow-login-123",
    method: "POST",
    nodes: [
      {
        type: "input",
        group: "default",
        attributes: { name: "csrf_token", type: "hidden", value: "mock-csrf", required: true, node_type: "input" },
        messages: [],
        meta: {},
      },
      {
        type: "input",
        group: "password",
        attributes: { name: "identifier", type: "text", required: true, node_type: "input" },
        messages: [],
        meta: { label: { id: 1, text: "Email", type: "info" } },
      },
      {
        type: "input",
        group: "password",
        attributes: { name: "password", type: "password", required: true, node_type: "input" },
        messages: [],
        meta: { label: { id: 2, text: "Password", type: "info" } },
      },
      {
        type: "input",
        group: "password",
        attributes: { name: "method", type: "submit", value: "password", node_type: "input" },
        messages: [],
        meta: { label: { id: 3, text: "Sign In", type: "info" } },
      },
    ],
    messages: [],
  },
}

// Ory Kratos registration flow
export const MOCK_REGISTRATION_FLOW = {
  id: "flow-reg-456",
  type: "browser",
  ui: {
    action: "http://localhost:4433/self-service/registration?flow=flow-reg-456",
    method: "POST",
    nodes: [
      {
        type: "input",
        group: "default",
        attributes: { name: "csrf_token", type: "hidden", value: "mock-csrf", required: true, node_type: "input" },
        messages: [],
        meta: {},
      },
      {
        type: "input",
        group: "password",
        attributes: { name: "traits.email", type: "text", required: true, node_type: "input" },
        messages: [],
        meta: { label: { id: 4, text: "Email", type: "info" } },
      },
      {
        type: "input",
        group: "password",
        attributes: { name: "password", type: "password", required: true, node_type: "input" },
        messages: [],
        meta: { label: { id: 5, text: "Password", type: "info" } },
      },
      {
        type: "input",
        group: "password",
        attributes: { name: "method", type: "submit", value: "password", node_type: "input" },
        messages: [],
        meta: { label: { id: 6, text: "Register", type: "info" } },
      },
    ],
    messages: [],
  },
}

// ── Route Mocking Helpers ──────────────────────────────────────────

/** Mock all Skillbox API and Ory Kratos routes for a given page */
export async function mockAllRoutes(page: Page, options?: { session?: typeof MOCK_SESSION_ADMIN | null }) {
  const session = options?.session !== undefined ? options.session : null

  // ── Ory Kratos ──
  await page.route("**/sessions/whoami", (route: Route) => {
    if (session) {
      return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(session) })
    }
    return route.fulfill({ status: 401, contentType: "application/json", body: JSON.stringify({ error: { code: 401, message: "No session" } }) })
  })

  await page.route("**/self-service/login/browser**", (route: Route) => {
    return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(MOCK_LOGIN_FLOW) })
  })

  await page.route("**/self-service/login?flow=**", (route: Route) => {
    if (route.request().method() === "POST") {
      return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ session: MOCK_SESSION_ADMIN }) })
    }
    return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(MOCK_LOGIN_FLOW) })
  })

  await page.route("**/self-service/registration/browser**", (route: Route) => {
    return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(MOCK_REGISTRATION_FLOW) })
  })

  await page.route("**/self-service/registration?flow=**", (route: Route) => {
    if (route.request().method() === "POST") {
      return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ session: MOCK_SESSION_ADMIN, identity: MOCK_SESSION_ADMIN.identity }) })
    }
    return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(MOCK_REGISTRATION_FLOW) })
  })

  await page.route("**/self-service/logout/browser**", (route: Route) => {
    return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ logout_token: "mock-logout-token", logout_url: "/" }) })
  })

  await page.route("**/self-service/logout?token=**", (route: Route) => {
    return route.fulfill({ status: 204 })
  })

  // ── Skillbox API ──

  // Marketplace skills - single handler for both list and detail
  await page.route("**/v1/marketplace/skills**", (route: Route) => {
    const url = route.request().url()
    const afterSkills = url.split("/v1/marketplace/skills")[1] || ""

    // Detail route: has a path segment after /skills/ (e.g., /skills/code-review)
    if (afterSkills.startsWith("/") && afterSkills.length > 1) {
      const name = decodeURIComponent(afterSkills.slice(1).split("?")[0] || "")
      const skill = MOCK_SKILLS.find((s) => s.name === name)
      if (!skill) {
        return route.fulfill({ status: 404, contentType: "application/json", body: JSON.stringify({ error: "not_found", message: "Skill not found" }) })
      }
      return route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ ...skill, approval_status: session ? "approved" : undefined }),
      })
    }

    // List route: /skills or /skills?q=...&limit=...
    const parsedUrl = new URL(url)
    const q = parsedUrl.searchParams.get("q")?.toLowerCase() || ""
    const limit = parseInt(parsedUrl.searchParams.get("limit") || "20")
    const offset = parseInt(parsedUrl.searchParams.get("offset") || "0")

    let filtered = MOCK_SKILLS
    if (q) {
      filtered = MOCK_SKILLS.filter(
        (s) => s.name.toLowerCase().includes(q) || s.description.toLowerCase().includes(q)
      )
    }

    const paged = filtered.slice(offset, offset + limit)
    return route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ skills: paged, total: filtered.length, limit, offset }),
    })
  })

  // Admin stats
  await page.route("**/v1/admin/stats", (route: Route) => {
    return route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ pending_approvals: 2, total_users: 3, total_groups: 2, total_skills: 6 }),
    })
  })

  // Users list - registered BEFORE role update so role update takes priority
  await page.route("**/v1/users", (route: Route) => {
    return route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ users: MOCK_USERS }),
    })
  })

  // User role update - registered AFTER list so it takes priority for /users/*/role
  await page.route("**/v1/users/*/role", (route: Route) => {
    return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ ok: true }) })
  })

  // Approvals list (matches with or without query string) - registered BEFORE specific routes
  await page.route("**/v1/approvals**", (route: Route) => {
    if (route.request().method() === "POST") {
      return route.fulfill({
        status: 201,
        contentType: "application/json",
        body: JSON.stringify({ id: "new-approval", status: "pending" }),
      })
    }
    const url = new URL(route.request().url())
    const status = url.searchParams.get("status")
    let filtered = MOCK_APPROVALS
    if (status) {
      filtered = MOCK_APPROVALS.filter((a) => a.status === status)
    }
    return route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ approvals: filtered }),
    })
  })

  // Approval action (approve/reject) - registered AFTER list so these take priority
  await page.route("**/v1/approvals/*/approve", (route: Route) => {
    return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ ok: true }) })
  })

  await page.route("**/v1/approvals/*/reject", (route: Route) => {
    return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ ok: true }) })
  })

  // Groups list - registered BEFORE detail so detail takes priority
  await page.route("**/v1/groups", (route: Route) => {
    if (route.request().method() === "POST") {
      return route.fulfill({
        status: 201,
        contentType: "application/json",
        body: JSON.stringify({ id: "new-group", name: "New Group", description: "", members: [] }),
      })
    }
    return route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ groups: MOCK_GROUPS }),
    })
  })

  // Group detail - registered AFTER list so it takes priority for /groups/*
  await page.route("**/v1/groups/*", (route: Route) => {
    const url = route.request().url()
    if (url.includes("/members")) {
      if (route.request().method() === "POST") {
        return route.fulfill({ status: 201, contentType: "application/json", body: JSON.stringify({ ok: true }) })
      }
      if (route.request().method() === "DELETE") {
        return route.fulfill({ status: 204 })
      }
    }
    const id = url.split("/v1/groups/")[1]?.split("/")[0] || ""
    const group = MOCK_GROUPS.find((g) => g.id === id) || MOCK_GROUPS[0]
    return route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(group),
    })
  })
}

// ── Custom test fixture with mocking ────────────────────────────────

export const test = base.extend<{
  mockPage: Page
  adminPage: Page
  consumerPage: Page
}>({
  /** Page with no authentication (anonymous visitor) */
  mockPage: async ({ page }, use) => {
    await mockAllRoutes(page, { session: null })
    await use(page)
  },

  /** Page authenticated as admin */
  adminPage: async ({ page }, use) => {
    await mockAllRoutes(page, { session: MOCK_SESSION_ADMIN })
    await use(page)
  },

  /** Page authenticated as consumer (non-admin) */
  consumerPage: async ({ page }, use) => {
    await mockAllRoutes(page, { session: MOCK_SESSION_CONSUMER })
    await use(page)
  },
})

export { expect } from "@playwright/test"
