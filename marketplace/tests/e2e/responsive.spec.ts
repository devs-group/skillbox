import { test, expect } from "./fixtures"

test.describe("Responsive Design", () => {
  test("home page renders on mobile viewport", async ({ mockPage }) => {
    await mockPage.setViewportSize({ width: 375, height: 812 })
    await mockPage.goto("/")
    await expect(mockPage.getByText("SKILLBOX")).toBeVisible()
    await expect(mockPage.getByText("code-review").first()).toBeVisible()
  })

  test("skills grid adjusts to single column on mobile", async ({ mockPage }) => {
    await mockPage.setViewportSize({ width: 375, height: 812 })
    await mockPage.goto("/skills")
    await expect(mockPage.getByText("code-review").first()).toBeVisible()
    // Content should still be visible (no horizontal overflow)
    const heading = mockPage.getByRole("heading", { name: /skill catalog/i })
    await expect(heading).toBeVisible()
  })

  test("navbar is visible on mobile", async ({ mockPage }) => {
    await mockPage.setViewportSize({ width: 375, height: 812 })
    await mockPage.goto("/")
    await expect(mockPage.getByText("SKILLBOX")).toBeVisible()
  })

  test("login form fits on mobile", async ({ mockPage }) => {
    await mockPage.setViewportSize({ width: 375, height: 812 })
    await mockPage.goto("/auth/login")
    await expect(mockPage.getByText(/welcome back/i)).toBeVisible()
    await expect(mockPage.getByRole("button", { name: /sign in/i })).toBeVisible()
  })

  test("admin layout stacks on mobile", async ({ adminPage }) => {
    await adminPage.setViewportSize({ width: 375, height: 812 })
    await adminPage.goto("/admin")
    await expect(adminPage.getByRole("heading", { name: /admin dashboard/i })).toBeVisible()
    // Navigation and content should both be visible (stacked vertically)
    await expect(adminPage.getByRole("link", { name: /approvals/i })).toBeVisible()
    await expect(adminPage.getByText("PENDING_APPROVALS")).toBeVisible()
  })

  test("home page renders on tablet viewport", async ({ mockPage }) => {
    await mockPage.setViewportSize({ width: 768, height: 1024 })
    await mockPage.goto("/")
    await expect(mockPage.getByText("SKILLBOX")).toBeVisible()
    await expect(mockPage.getByText("code-review").first()).toBeVisible()
  })

  test("skill detail page readable on mobile", async ({ mockPage }) => {
    await mockPage.setViewportSize({ width: 375, height: 812 })
    await mockPage.goto("/skills/code-review")
    await expect(mockPage.getByRole("heading", { name: "code-review" })).toBeVisible()
    await expect(mockPage.getByText("skillbox add code-review")).toBeVisible()
  })
})
