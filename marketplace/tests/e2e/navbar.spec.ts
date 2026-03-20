import { test, expect } from "./fixtures"

test.describe("Navbar", () => {
  test("shows logo and browse link", async ({ mockPage }) => {
    await mockPage.goto("/")
    await expect(mockPage.getByText("SKILLBOX")).toBeVisible()
    await expect(mockPage.getByRole("link", { name: /browse/i })).toBeVisible()
  })

  test("anonymous user sees login button", async ({ mockPage }) => {
    await mockPage.goto("/")
    await expect(mockPage.getByRole("link", { name: /login/i })).toBeVisible()
    await expect(mockPage.getByRole("button", { name: /logout/i })).not.toBeVisible()
  })

  test("anonymous user does not see admin link", async ({ mockPage }) => {
    await mockPage.goto("/")
    await expect(mockPage.getByText(MOCK_SKILLS_PLACEHOLDER)).not.toBeVisible()
    // Admin link should not exist
    await expect(mockPage.getByRole("link", { name: /admin/i })).not.toBeVisible()
  })

  test("authenticated admin sees logout and admin link", async ({ adminPage }) => {
    await adminPage.goto("/")
    await expect(adminPage.getByRole("button", { name: /logout/i })).toBeVisible()
    await expect(adminPage.getByRole("link", { name: /admin/i })).toBeVisible()
    // Login button should not be shown
    await expect(adminPage.getByRole("link", { name: /^login$/i })).not.toBeVisible()
  })

  test("authenticated consumer sees logout but not admin link", async ({ consumerPage }) => {
    await consumerPage.goto("/")
    await expect(consumerPage.getByRole("button", { name: /logout/i })).toBeVisible()
    await expect(consumerPage.getByRole("link", { name: /admin/i })).not.toBeVisible()
  })

  test("browse link navigates to skills page", async ({ mockPage }) => {
    await mockPage.goto("/")
    await mockPage.getByRole("link", { name: /browse/i }).click()
    await expect(mockPage).toHaveURL("/skills")
  })

  test("login link navigates to login page", async ({ mockPage }) => {
    await mockPage.goto("/")
    await mockPage.getByRole("link", { name: /login/i }).click()
    await expect(mockPage).toHaveURL("/auth/login")
  })
})

// Placeholder to prevent lint errors — not used
const MOCK_SKILLS_PLACEHOLDER = "__never_visible__"
