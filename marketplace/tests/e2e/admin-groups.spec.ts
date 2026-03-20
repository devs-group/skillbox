import { test, expect, MOCK_GROUPS } from "./fixtures"

test.describe("Admin Groups Page", () => {
  test("displays groups heading and create button", async ({ adminPage }) => {
    await adminPage.goto("/admin/groups")
    await expect(adminPage.getByText(/GROUP_MANAGEMENT/)).toBeVisible()
    await expect(adminPage.getByRole("button", { name: /create group/i })).toBeVisible()
  })

  test("shows group cards", async ({ adminPage }) => {
    await adminPage.goto("/admin/groups")
    for (const group of MOCK_GROUPS) {
      await expect(adminPage.getByText(group.name, { exact: true })).toBeVisible()
      await expect(adminPage.getByText(group.description)).toBeVisible()
    }
  })

  test("group cards show member count", async ({ adminPage }) => {
    await adminPage.goto("/admin/groups")
    // Each group has 1 member in mock data - member count badge should be visible
    const badges = adminPage.locator(".flex.items-center.gap-1")
    await expect(badges.first()).toBeVisible()
  })

  test("clicking create group opens dialog", async ({ adminPage }) => {
    await adminPage.goto("/admin/groups")
    await adminPage.getByRole("button", { name: /create group/i }).click()

    await expect(adminPage.getByText("CREATE_GROUP")).toBeVisible()
    await expect(adminPage.getByPlaceholder(/group name/i)).toBeVisible()
    await expect(adminPage.getByPlaceholder(/optional description/i)).toBeVisible()
    await expect(adminPage.getByRole("button", { name: /^create$/i })).toBeVisible()
    await expect(adminPage.getByRole("button", { name: /cancel/i })).toBeVisible()
  })

  test("create button disabled when name is empty", async ({ adminPage }) => {
    await adminPage.goto("/admin/groups")
    await adminPage.getByRole("button", { name: /create group/i }).click()

    const createBtn = adminPage.locator(".fixed.inset-0").getByRole("button", { name: /^create$/i })
    await expect(createBtn).toBeDisabled()
  })

  test("can fill and submit create group form", async ({ adminPage }) => {
    await adminPage.goto("/admin/groups")
    await adminPage.getByRole("button", { name: /create group/i }).click()

    await adminPage.getByPlaceholder(/group name/i).fill("DevOps Team")
    await adminPage.getByPlaceholder(/optional description/i).fill("Infrastructure and deployment")

    const createBtn = adminPage.locator(".fixed.inset-0").getByRole("button", { name: /^create$/i })
    await expect(createBtn).toBeEnabled()
    await createBtn.click()

    // Toast should appear
    await expect(adminPage.getByText(/group created/i)).toBeVisible()
  })

  test("can cancel create dialog", async ({ adminPage }) => {
    await adminPage.goto("/admin/groups")
    await adminPage.getByRole("button", { name: /create group/i }).click()
    await expect(adminPage.getByText("CREATE_GROUP")).toBeVisible()

    await adminPage.getByRole("button", { name: /cancel/i }).click()
    await expect(adminPage.getByText("CREATE_GROUP")).not.toBeVisible()
  })

  test("clicking a group card opens detail dialog", async ({ adminPage }) => {
    await adminPage.goto("/admin/groups")

    // Click on the first group card
    await adminPage.getByText(MOCK_GROUPS[0].name, { exact: true }).click()

    // Detail dialog should show
    const dialogOverlay = adminPage.locator(".fixed.inset-0")
    await expect(dialogOverlay.getByText(MOCK_GROUPS[0].name, { exact: true })).toBeVisible()
    await expect(dialogOverlay.getByText(MOCK_GROUPS[0].description)).toBeVisible()
    // Member input should be visible
    await expect(dialogOverlay.getByPlaceholder(/add member by email/i)).toBeVisible()
  })

  test("group detail shows members", async ({ adminPage }) => {
    await adminPage.goto("/admin/groups")
    await adminPage.getByText(MOCK_GROUPS[0].name, { exact: true }).click()

    // Should show the member email
    await expect(adminPage.getByText(MOCK_GROUPS[0].members[0].email)).toBeVisible()
  })

  test("can type member email to add", async ({ adminPage }) => {
    await adminPage.goto("/admin/groups")
    await adminPage.getByText(MOCK_GROUPS[0].name, { exact: true }).click()

    const input = adminPage.getByPlaceholder(/add member by email/i)
    await input.fill("newmember@example.com")
    await expect(input).toHaveValue("newmember@example.com")
  })

  test("add member button disabled when email empty", async ({ adminPage }) => {
    await adminPage.goto("/admin/groups")
    await adminPage.getByText(MOCK_GROUPS[0].name, { exact: true }).click()

    const dialogOverlay = adminPage.locator(".fixed.inset-0")
    const addBtn = dialogOverlay.getByRole("button", { name: /^add$/i })
    await expect(addBtn).toBeDisabled()
  })

  test("remove member button is visible", async ({ adminPage }) => {
    await adminPage.goto("/admin/groups")
    await adminPage.getByText(MOCK_GROUPS[0].name, { exact: true }).click()

    // There should be a trash/remove icon button
    await expect(adminPage.getByText(MOCK_GROUPS[0].members[0].email)).toBeVisible()
    const removeBtn = adminPage.locator(".fixed.inset-0 button").filter({ has: adminPage.locator("svg") })
    const count = await removeBtn.count()
    expect(count).toBeGreaterThanOrEqual(1)
  })

  test("empty groups state", async ({ page }) => {
    await page.route("**/sessions/whoami", (route) => {
      const { MOCK_SESSION_ADMIN } = require("./fixtures")
      return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(MOCK_SESSION_ADMIN) })
    })
    await page.route("**/v1/groups", (route) =>
      route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ groups: [] }) })
    )

    await page.goto("/admin/groups")
    await expect(page.getByText(/no groups yet/i)).toBeVisible()
  })
})
