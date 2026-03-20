import { test, expect, MOCK_SKILLS } from "./fixtures"

test.describe("Skill Detail Page", () => {
  test("displays skill name and description", async ({ mockPage }) => {
    await mockPage.goto("/skills/code-review")
    await expect(mockPage.getByRole("heading", { name: "code-review" })).toBeVisible()
    await expect(mockPage.getByText(MOCK_SKILLS[0].description)).toBeVisible()
  })

  test("shows version badge", async ({ mockPage }) => {
    await mockPage.goto("/skills/code-review")
    await expect(mockPage.getByText("v1.2.0")).toBeVisible()
  })

  test("shows install command with copy button", async ({ mockPage }) => {
    await mockPage.goto("/skills/code-review")
    await expect(mockPage.getByText("skillbox add code-review")).toBeVisible()
    await expect(mockPage.getByText(/install this skill/i)).toBeVisible()
  })

  test("shows 404 for unknown skill", async ({ mockPage }) => {
    await mockPage.goto("/skills/nonexistent-skill")
    await expect(mockPage.getByText(/skill not found|failed to load/i)).toBeVisible()
  })

  test("anonymous user does not see approval badge", async ({ mockPage }) => {
    await mockPage.goto("/skills/code-review")
    await expect(mockPage.getByRole("heading", { name: "code-review" })).toBeVisible()
    // ApprovalBadge should not be visible for anonymous
    await expect(mockPage.getByText("Approved")).not.toBeVisible()
  })

  test("authenticated user sees approval status", async ({ adminPage }) => {
    await adminPage.goto("/skills/code-review")
    await expect(adminPage.getByRole("heading", { name: "code-review" })).toBeVisible()
    // Session is admin, so approval_status is "approved"
    await expect(adminPage.getByText("Approved")).toBeVisible()
  })

  test("navigates between skills", async ({ mockPage }) => {
    await mockPage.goto("/skills/code-review")
    await expect(mockPage.getByRole("heading", { name: "code-review" })).toBeVisible()

    // Go to a different skill
    await mockPage.goto("/skills/sql-optimizer")
    await expect(mockPage.getByRole("heading", { name: "sql-optimizer" })).toBeVisible()
    await expect(mockPage.getByText(MOCK_SKILLS[2].description)).toBeVisible()
  })
})
