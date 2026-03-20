import { test, expect } from "./fixtures"

test.describe("Admin Dashboard", () => {
  test("redirects unauthenticated users to login", async ({ mockPage }) => {
    await mockPage.goto("/admin")
    // Admin layout checks isAdmin and redirects to /auth/login
    await expect(mockPage).toHaveURL(/\/auth\/login/)
  })

  test("redirects non-admin users to login", async ({ consumerPage }) => {
    await consumerPage.goto("/admin")
    await expect(consumerPage).toHaveURL(/\/auth\/login/)
  })

  test("admin can access dashboard", async ({ adminPage }) => {
    await adminPage.goto("/admin")
    await expect(adminPage.getByRole("heading", { name: /admin dashboard/i })).toBeVisible()
    await expect(adminPage.getByText(/OVERVIEW/)).toBeVisible()
  })

  test("shows stats cards", async ({ adminPage }) => {
    await adminPage.goto("/admin")
    await expect(adminPage.getByText("PENDING_APPROVALS")).toBeVisible()
    await expect(adminPage.getByText("TOTAL_USERS")).toBeVisible()
    await expect(adminPage.getByText("TOTAL_GROUPS")).toBeVisible()
    await expect(adminPage.getByText("TOTAL_SKILLS")).toBeVisible()
  })

  test("stats cards show numeric values", async ({ adminPage }) => {
    await adminPage.goto("/admin")
    // From mock: pending=2, users=3, groups=2, skills=6
    await expect(adminPage.getByText("2").first()).toBeVisible()
    await expect(adminPage.getByText("3")).toBeVisible()
    await expect(adminPage.getByText("6")).toBeVisible()
  })

  test("admin sidebar navigation is visible", async ({ adminPage }) => {
    await adminPage.goto("/admin")
    await expect(adminPage.getByRole("link", { name: /dashboard/i })).toBeVisible()
    await expect(adminPage.getByRole("link", { name: /approvals/i })).toBeVisible()
    await expect(adminPage.getByRole("link", { name: /users/i })).toBeVisible()
    await expect(adminPage.getByRole("link", { name: /groups/i })).toBeVisible()
  })

  test("sidebar links navigate correctly", async ({ adminPage }) => {
    await adminPage.goto("/admin")
    await adminPage.getByRole("link", { name: /approvals/i }).click()
    await expect(adminPage).toHaveURL("/admin/approvals")

    await adminPage.getByRole("link", { name: /users/i }).click()
    await expect(adminPage).toHaveURL("/admin/users")

    await adminPage.getByRole("link", { name: /groups/i }).click()
    await expect(adminPage).toHaveURL("/admin/groups")

    await adminPage.getByRole("link", { name: /dashboard/i }).click()
    await expect(adminPage).toHaveURL("/admin")
  })
})
