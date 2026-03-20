import { test, expect, MOCK_SKILLS } from "./fixtures"

test.describe("Skills Browse Page", () => {
  test("renders browse heading and search", async ({ mockPage }) => {
    await mockPage.goto("/skills")
    await expect(mockPage.getByRole("heading", { name: /skill catalog/i })).toBeVisible()
    await expect(mockPage.getByPlaceholder(/search skills/i)).toBeVisible()
  })

  test("displays all skills in grid", async ({ mockPage }) => {
    await mockPage.goto("/skills")
    for (const skill of MOCK_SKILLS) {
      await expect(mockPage.getByText(skill.name).first()).toBeVisible()
    }
  })

  test("search filters by name", async ({ mockPage }) => {
    await mockPage.goto("/skills")
    await expect(mockPage.getByRole("link", { name: "code-review" })).toBeVisible()

    await mockPage.getByPlaceholder(/search skills/i).fill("security")
    await expect(mockPage.getByText("security-scanner").first()).toBeVisible()
    await expect(mockPage.getByRole("link", { name: "code-review" })).not.toBeVisible()
  })

  test("search filters by description", async ({ mockPage }) => {
    await mockPage.goto("/skills")
    await expect(mockPage.getByRole("link", { name: "code-review" })).toBeVisible()

    await mockPage.getByPlaceholder(/search skills/i).fill("unit tests")
    await expect(mockPage.getByText("test-generator").first()).toBeVisible()
    await expect(mockPage.getByRole("link", { name: "sql-optimizer" })).not.toBeVisible()
  })

  test("empty search results", async ({ mockPage }) => {
    await mockPage.goto("/skills")
    await expect(mockPage.getByText(MOCK_SKILLS[0].name).first()).toBeVisible()

    await mockPage.getByPlaceholder(/search skills/i).fill("zzzzz-nothing")
    await expect(mockPage.getByText(/no skills found/i)).toBeVisible()
  })

  test("skill card links to detail page", async ({ mockPage }) => {
    await mockPage.goto("/skills")
    await expect(mockPage.getByRole("link", { name: "code-review" })).toBeVisible()

    await mockPage.getByRole("link", { name: "code-review" }).click()
    await expect(mockPage).toHaveURL(/\/skills\/code-review/)
  })

  test("each skill card shows copy command", async ({ mockPage }) => {
    await mockPage.goto("/skills")
    await expect(mockPage.getByRole("link", { name: "code-review" })).toBeVisible()

    const commands = mockPage.locator("code")
    const count = await commands.count()
    expect(count).toBeGreaterThanOrEqual(MOCK_SKILLS.length)
  })

  test("provider badges are shown", async ({ mockPage }) => {
    await mockPage.goto("/skills")
    await expect(mockPage.getByRole("link", { name: "code-review" })).toBeVisible()

    // All skills have provider "claude"
    const badges = mockPage.getByText("claude")
    const count = await badges.count()
    expect(count).toBeGreaterThanOrEqual(1)
  })
})
