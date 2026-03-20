import { test, expect, MOCK_APPROVALS } from "./fixtures"

test.describe("Admin Approvals Page", () => {
  test("displays approval requests table", async ({ adminPage }) => {
    await adminPage.goto("/admin/approvals")
    await expect(adminPage.getByText(/APPROVAL_REQUESTS/)).toBeVisible()

    // Check table headers
    await expect(adminPage.getByText("Skill", { exact: true }).first()).toBeVisible()
    await expect(adminPage.getByText("Requester", { exact: true }).first()).toBeVisible()
    await expect(adminPage.getByText("Status", { exact: true }).first()).toBeVisible()
  })

  test("shows all approvals by default", async ({ adminPage }) => {
    await adminPage.goto("/admin/approvals")
    for (const approval of MOCK_APPROVALS) {
      await expect(adminPage.getByText(approval.skill_name).first()).toBeVisible()
    }
  })

  test("filter tabs work - pending", async ({ adminPage }) => {
    await adminPage.goto("/admin/approvals")
    await expect(adminPage.getByText("code-review").first()).toBeVisible()

    // Click pending filter button
    await adminPage.getByRole("button", { name: /^pending$/i }).click()

    // Should show only pending approvals
    await expect(adminPage.getByText("code-review").first()).toBeVisible()
    await expect(adminPage.getByText("test-generator").first()).toBeVisible()
  })

  test("filter tabs work - approved", async ({ adminPage }) => {
    await adminPage.goto("/admin/approvals")
    await adminPage.getByRole("button", { name: /^approved$/i }).click()

    await expect(adminPage.getByText("sql-optimizer").first()).toBeVisible()
  })

  test("filter tabs work - rejected", async ({ adminPage }) => {
    await adminPage.goto("/admin/approvals")
    await adminPage.getByRole("button", { name: /^rejected$/i }).click()

    await expect(adminPage.getByText("security-scanner").first()).toBeVisible()
  })

  test("pending approvals show approve and reject buttons", async ({ adminPage }) => {
    await adminPage.goto("/admin/approvals")
    await adminPage.getByRole("button", { name: /^pending$/i }).click()

    // Pending rows should have Approve and Reject buttons
    const approveButtons = adminPage.getByRole("button", { name: /^approve$/i })
    const rejectButtons = adminPage.getByRole("button", { name: /^reject$/i })
    await expect(approveButtons.first()).toBeVisible()
    await expect(rejectButtons.first()).toBeVisible()
  })

  test("approved/rejected items do not show action buttons", async ({ adminPage }) => {
    await adminPage.goto("/admin/approvals")
    await adminPage.getByRole("button", { name: /^approved$/i }).click()

    await expect(adminPage.getByText("sql-optimizer").first()).toBeVisible()
    // No approve/reject action buttons for already-processed items
    // The only "approved" button visible is the filter button itself
    const approveActionButtons = adminPage.locator("button").filter({ hasText: /^Approve$/ })
    await expect(approveActionButtons).toHaveCount(0)
  })

  test("clicking approve opens dialog", async ({ adminPage }) => {
    await adminPage.goto("/admin/approvals")
    await adminPage.getByRole("button", { name: /^pending$/i }).click()
    await expect(adminPage.getByRole("button", { name: /^approve$/i }).first()).toBeVisible()

    await adminPage.getByRole("button", { name: /^approve$/i }).first().click()

    // Dialog should open
    await expect(adminPage.getByText(/APPROVE REQUEST/)).toBeVisible()
    await expect(adminPage.getByPlaceholder(/optional comment/i)).toBeVisible()
    await expect(adminPage.getByRole("button", { name: /cancel/i })).toBeVisible()
  })

  test("clicking reject opens dialog", async ({ adminPage }) => {
    await adminPage.goto("/admin/approvals")
    await adminPage.getByRole("button", { name: /^pending$/i }).click()

    await adminPage.getByRole("button", { name: /^reject$/i }).first().click()

    // Dialog should open
    await expect(adminPage.getByText(/REJECT REQUEST/)).toBeVisible()
    await expect(adminPage.getByPlaceholder(/optional comment/i)).toBeVisible()
  })

  test("can submit approval with comment", async ({ adminPage }) => {
    await adminPage.goto("/admin/approvals")
    await adminPage.getByRole("button", { name: /^pending$/i }).click()

    await adminPage.getByRole("button", { name: /^approve$/i }).first().click()
    await expect(adminPage.getByText(/APPROVE REQUEST/)).toBeVisible()

    await adminPage.getByPlaceholder(/optional comment/i).fill("Looks good, approved!")

    // Find the approve button inside the dialog overlay
    const dialogOverlay = adminPage.locator(".fixed.inset-0")
    await dialogOverlay.getByRole("button", { name: /^approve$/i }).click()

    // Toast should appear
    await expect(adminPage.getByText(/approved/i).first()).toBeVisible()
  })

  test("can cancel dialog without action", async ({ adminPage }) => {
    await adminPage.goto("/admin/approvals")
    await adminPage.getByRole("button", { name: /^pending$/i }).click()

    await adminPage.getByRole("button", { name: /^approve$/i }).first().click()
    await expect(adminPage.getByText(/APPROVE REQUEST/)).toBeVisible()

    await adminPage.getByRole("button", { name: /cancel/i }).click()

    // Dialog should close
    await expect(adminPage.getByText(/APPROVE REQUEST/)).not.toBeVisible()
  })

  test("shows empty state when no approvals match filter", async ({ page }) => {
    // Mock with empty results
    await page.route("**/sessions/whoami", (route) => {
      const { MOCK_SESSION_ADMIN } = require("./fixtures")
      return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(MOCK_SESSION_ADMIN) })
    })
    await page.route("**/v1/approvals**", (route) =>
      route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ approvals: [] }) })
    )

    await page.goto("/admin/approvals")
    await expect(page.getByText(/no approval requests found/i)).toBeVisible()
  })
})
