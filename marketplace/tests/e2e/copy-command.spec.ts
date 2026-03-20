import { test, expect } from "./fixtures"

test.describe("Copy Command Component", () => {
  test("displays skillbox add command on home page", async ({ mockPage }) => {
    await mockPage.goto("/")
    await expect(mockPage.getByRole("link", { name: "code-review" })).toBeVisible()

    // Check the code element contains the command
    const commands = mockPage.locator("code")
    const texts = await commands.allTextContents()
    const hasAddCommand = texts.some((t) => t.includes("skillbox add"))
    expect(hasAddCommand).toBe(true)
  })

  test("displays skillbox add command on skill detail", async ({ mockPage }) => {
    await mockPage.goto("/skills/code-review")
    await expect(mockPage.getByText("skillbox add code-review")).toBeVisible()
  })

  test("copy button exists next to command", async ({ mockPage }) => {
    await mockPage.goto("/skills/code-review")
    await expect(mockPage.getByText("skillbox add code-review")).toBeVisible()

    // There should be a copy button (ghost variant with icon)
    const copyBtn = mockPage.locator("button").filter({ has: mockPage.locator("svg") })
    const count = await copyBtn.count()
    expect(count).toBeGreaterThanOrEqual(1)
  })

  test("displays commands for all skills in browse", async ({ mockPage }) => {
    await mockPage.goto("/skills")
    await expect(mockPage.getByRole("link", { name: "code-review" })).toBeVisible()

    // Each skill card should have its own command
    await expect(mockPage.getByText("skillbox add code-review").first()).toBeVisible()
    await expect(mockPage.getByText("skillbox add test-generator").first()).toBeVisible()
    await expect(mockPage.getByText("skillbox add sql-optimizer").first()).toBeVisible()
  })
})
