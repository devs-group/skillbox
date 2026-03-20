import { test, expect, MOCK_SKILLS } from "./fixtures"

test.describe("Home Page", () => {
  test("renders hero section with title and search", async ({ mockPage }) => {
    await mockPage.goto("/")
    await expect(mockPage.getByText("SKILLBOX")).toBeVisible()
    await expect(mockPage.getByPlaceholder(/search skills/i)).toBeVisible()
    await expect(mockPage.getByText(/SECTION: SEARCH_SKILLS/)).toBeVisible()
  })

  test("displays skill cards from API", async ({ mockPage }) => {
    await mockPage.goto("/")
    // Wait for loading to finish (skeleton -> cards)
    await expect(mockPage.getByText(MOCK_SKILLS[0].name).first()).toBeVisible()
    // All 6 mock skills should show (limit=12)
    for (const skill of MOCK_SKILLS) {
      await expect(mockPage.getByText(skill.name).first()).toBeVisible()
    }
  })

  test("skill cards have descriptions", async ({ mockPage }) => {
    await mockPage.goto("/")
    await expect(mockPage.getByText(MOCK_SKILLS[0].name).first()).toBeVisible()
    await expect(mockPage.getByText(MOCK_SKILLS[0].description).first()).toBeVisible()
  })

  test("skill cards show copy command button", async ({ mockPage }) => {
    await mockPage.goto("/")
    await expect(mockPage.getByText(MOCK_SKILLS[0].name).first()).toBeVisible()
    await expect(mockPage.getByText(`skillbox add ${MOCK_SKILLS[0].name}`).first()).toBeVisible()
  })

  test("search filters skills", async ({ mockPage }) => {
    await mockPage.goto("/")
    await expect(mockPage.getByText(MOCK_SKILLS[0].name).first()).toBeVisible()

    const searchInput = mockPage.getByPlaceholder(/search skills/i)
    await searchInput.fill("sql")

    // Wait for debounce + fetch
    await expect(mockPage.getByText("sql-optimizer").first()).toBeVisible()
    // Other skills should not be visible
    await expect(mockPage.getByRole("link", { name: "code-review" })).not.toBeVisible()
  })

  test("empty search results show message", async ({ mockPage }) => {
    await mockPage.goto("/")
    await expect(mockPage.getByText(MOCK_SKILLS[0].name).first()).toBeVisible()

    const searchInput = mockPage.getByPlaceholder(/search skills/i)
    await searchInput.fill("nonexistent-xyzzy")

    await expect(mockPage.getByText(/no skills found/i)).toBeVisible()
  })

  test("shows loading skeletons initially", async ({ page }) => {
    // Block the API to see loading state
    await page.route("**/sessions/whoami", (route) =>
      route.fulfill({ status: 401, contentType: "application/json", body: JSON.stringify({}) })
    )
    await page.route("**/v1/marketplace/skills?**", (route) =>
      new Promise((resolve) => setTimeout(resolve, 5000)).then(() =>
        route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ skills: [] }) })
      )
    )
    await page.goto("/")
    // Skeleton divs with animate-pulse class should be present
    const skeletons = page.locator(".animate-pulse")
    await expect(skeletons.first()).toBeVisible()
  })
})
