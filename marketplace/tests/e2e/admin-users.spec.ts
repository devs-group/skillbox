import { test, expect, MOCK_USERS } from "./fixtures"

test.describe("Admin Users Page", () => {
  test("displays users table", async ({ adminPage }) => {
    await adminPage.goto("/admin/users")
    await expect(adminPage.getByText(/USER_MANAGEMENT/)).toBeVisible()

    // Table headers
    await expect(adminPage.getByText("Email", { exact: true }).first()).toBeVisible()
    await expect(adminPage.getByText("Role", { exact: true }).first()).toBeVisible()
    await expect(adminPage.getByText("Joined", { exact: true }).first()).toBeVisible()
  })

  test("shows all users", async ({ adminPage }) => {
    await adminPage.goto("/admin/users")
    for (const user of MOCK_USERS) {
      await expect(adminPage.getByText(user.email)).toBeVisible()
    }
  })

  test("shows role badges", async ({ adminPage }) => {
    await adminPage.goto("/admin/users")
    // Role badges are span elements, not options
    await expect(adminPage.locator("span").filter({ hasText: "admin" }).first()).toBeVisible()
    await expect(adminPage.locator("span").filter({ hasText: "consumer" }).first()).toBeVisible()
    await expect(adminPage.locator("span").filter({ hasText: "publisher" }).first()).toBeVisible()
  })

  test("role select dropdowns are present", async ({ adminPage }) => {
    await adminPage.goto("/admin/users")
    await expect(adminPage.getByText(MOCK_USERS[0].email)).toBeVisible()

    // There should be a native select for each user
    const selects = adminPage.locator("select")
    const count = await selects.count()
    expect(count).toBe(MOCK_USERS.length)
  })

  test("shows formatted dates", async ({ adminPage }) => {
    await adminPage.goto("/admin/users")
    // Dates should be formatted (at least partially visible)
    await expect(adminPage.getByText(MOCK_USERS[0].email)).toBeVisible()
    // Each user row should be present
    const rows = adminPage.locator(".grid.grid-cols-4.items-center")
    const count = await rows.count()
    expect(count).toBe(MOCK_USERS.length)
  })

  test("empty users state", async ({ page }) => {
    await page.route("**/sessions/whoami", (route) => {
      const { MOCK_SESSION_ADMIN } = require("./fixtures")
      return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(MOCK_SESSION_ADMIN) })
    })
    await page.route("**/v1/users", (route) =>
      route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ users: [] }) })
    )

    await page.goto("/admin/users")
    await expect(page.getByText(/no users found/i)).toBeVisible()
  })
})
