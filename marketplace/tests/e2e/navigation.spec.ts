import { test, expect } from "./fixtures"

test.describe("Navigation & Routing", () => {
  test("home page loads at /", async ({ mockPage }) => {
    await mockPage.goto("/")
    await expect(mockPage.getByText("SKILLBOX")).toBeVisible()
  })

  test("skills page loads at /skills", async ({ mockPage }) => {
    await mockPage.goto("/skills")
    await expect(mockPage.getByRole("heading", { name: /skill catalog/i })).toBeVisible()
  })

  test("login page loads at /auth/login", async ({ mockPage }) => {
    await mockPage.goto("/auth/login")
    await expect(mockPage.getByText(/welcome back/i)).toBeVisible()
  })

  test("register page loads at /auth/register", async ({ mockPage }) => {
    await mockPage.goto("/auth/register")
    await expect(mockPage.getByText(/create an account/i)).toBeVisible()
  })

  test("navigate from home to skills via card click", async ({ mockPage }) => {
    await mockPage.goto("/")
    await expect(mockPage.getByText("code-review").first()).toBeVisible()
    await mockPage.getByRole("link", { name: "code-review" }).first().click()
    await expect(mockPage).toHaveURL(/\/skills\/code-review/)
  })

  test("navigate from login to register", async ({ mockPage }) => {
    await mockPage.goto("/auth/login")
    await mockPage.getByRole("link", { name: /register/i }).click()
    await expect(mockPage).toHaveURL("/auth/register")
  })

  test("navigate from register to login", async ({ mockPage }) => {
    await mockPage.goto("/auth/register")
    await mockPage.getByRole("link", { name: /sign in/i }).click()
    await expect(mockPage).toHaveURL("/auth/login")
  })

  test("admin navigation between sub-pages", async ({ adminPage }) => {
    await adminPage.goto("/admin")
    await expect(adminPage.getByRole("heading", { name: /admin dashboard/i })).toBeVisible()

    // Navigate through all admin pages
    await adminPage.getByRole("link", { name: /approvals/i }).click()
    await expect(adminPage).toHaveURL("/admin/approvals")
    await expect(adminPage.getByText(/APPROVAL_REQUESTS/)).toBeVisible()

    await adminPage.getByRole("link", { name: /users/i }).click()
    await expect(adminPage).toHaveURL("/admin/users")
    await expect(adminPage.getByText(/USER_MANAGEMENT/)).toBeVisible()

    await adminPage.getByRole("link", { name: /groups/i }).click()
    await expect(adminPage).toHaveURL("/admin/groups")
    await expect(adminPage.getByText(/GROUP_MANAGEMENT/)).toBeVisible()

    await adminPage.getByRole("link", { name: /dashboard/i }).click()
    await expect(adminPage).toHaveURL("/admin")
    await expect(adminPage.getByText(/OVERVIEW/)).toBeVisible()
  })

  test("admin nav highlights active page", async ({ adminPage }) => {
    await adminPage.goto("/admin/approvals")
    // The active link should have a different style (bg-accent class)
    const approvalsLink = adminPage.getByRole("link", { name: /approvals/i }).first()
    await expect(approvalsLink).toHaveClass(/bg-foreground/)
  })
})
