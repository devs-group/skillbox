import { test, expect } from "./fixtures"

test.describe("Error States & Edge Cases", () => {
  test("home page handles API failure gracefully", async ({ page }) => {
    await page.route("**/sessions/whoami", (route) =>
      route.fulfill({ status: 401, contentType: "application/json", body: JSON.stringify({}) })
    )
    await page.route("**/v1/marketplace/skills?**", (route) =>
      route.fulfill({ status: 500, contentType: "application/json", body: JSON.stringify({ error: "internal_error" }) })
    )

    await page.goto("/")
    // Should show empty state, not crash
    await expect(page.getByText(/no skills found/i)).toBeVisible()
  })

  test("skill detail handles 404 gracefully", async ({ mockPage }) => {
    await mockPage.goto("/skills/this-skill-does-not-exist")
    await expect(mockPage.getByText(/skill not found|failed to load/i)).toBeVisible()
  })

  test("admin dashboard handles stats API failure", async ({ page }) => {
    await page.route("**/sessions/whoami", (route) => {
      const { MOCK_SESSION_ADMIN } = require("./fixtures")
      return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(MOCK_SESSION_ADMIN) })
    })
    await page.route("**/v1/admin/stats", (route) =>
      route.fulfill({ status: 500, contentType: "application/json", body: JSON.stringify({}) })
    )

    await page.goto("/admin")
    // Should show default zeros, not crash
    await expect(page.getByText("PENDING_APPROVALS")).toBeVisible()
    await expect(page.getByText("0").first()).toBeVisible()
  })

  test("approvals page handles API failure", async ({ page }) => {
    await page.route("**/sessions/whoami", (route) => {
      const { MOCK_SESSION_ADMIN } = require("./fixtures")
      return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(MOCK_SESSION_ADMIN) })
    })
    await page.route("**/v1/approvals**", (route) =>
      route.fulfill({ status: 500, contentType: "application/json", body: JSON.stringify({}) })
    )

    await page.goto("/admin/approvals")
    await expect(page.getByText(/no approval requests found/i)).toBeVisible()
  })

  test("users page handles API failure", async ({ page }) => {
    await page.route("**/sessions/whoami", (route) => {
      const { MOCK_SESSION_ADMIN } = require("./fixtures")
      return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(MOCK_SESSION_ADMIN) })
    })
    await page.route("**/v1/users", (route) =>
      route.fulfill({ status: 500, contentType: "application/json", body: JSON.stringify({}) })
    )

    await page.goto("/admin/users")
    await expect(page.getByText(/no users found/i)).toBeVisible()
  })

  test("groups page handles API failure", async ({ page }) => {
    await page.route("**/sessions/whoami", (route) => {
      const { MOCK_SESSION_ADMIN } = require("./fixtures")
      return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(MOCK_SESSION_ADMIN) })
    })
    await page.route("**/v1/groups", (route) =>
      route.fulfill({ status: 500, contentType: "application/json", body: JSON.stringify({}) })
    )

    await page.goto("/admin/groups")
    await expect(page.getByText(/no groups yet/i)).toBeVisible()
  })

  test("search with special characters does not crash", async ({ mockPage }) => {
    await mockPage.goto("/skills")
    await expect(mockPage.getByRole("link", { name: "code-review" })).toBeVisible()

    await mockPage.getByPlaceholder(/search skills/i).fill("<script>alert('xss')</script>")
    // Should show no results, not crash
    await expect(mockPage.getByText(/no skills found/i)).toBeVisible()
  })

  test("very long skill name renders without breaking layout", async ({ page }) => {
    await page.route("**/sessions/whoami", (route) =>
      route.fulfill({ status: 401, contentType: "application/json", body: JSON.stringify({}) })
    )
    await page.route("**/v1/marketplace/skills/*", (route) =>
      route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          name: "a-very-long-skill-name-that-might-cause-layout-issues-in-the-ui-if-not-handled-properly",
          description: "A skill with an extremely long name for testing overflow behavior",
          version: "1.0.0",
        }),
      })
    )

    await page.goto("/skills/a-very-long-skill-name-that-might-cause-layout-issues-in-the-ui-if-not-handled-properly")
    // Page should render without errors
    await expect(page.getByText(/a-very-long-skill-name/i).first()).toBeVisible()
  })
})
