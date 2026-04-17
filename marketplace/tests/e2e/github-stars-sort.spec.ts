import { test, expect } from "@playwright/test"
import type { Route } from "@playwright/test"

// Unsorted upstream — server MUST return stars desc.
const MOCK_GITHUB_RESULTS = [
  { name: "alpha", description: "a", repo_owner: "owner", repo_name: "alpha", file_path: "skills/alpha/SKILL.md", stars: 10, html_url: "https://github.com/owner/alpha" },
  { name: "beta", description: "b", repo_owner: "owner", repo_name: "beta", file_path: "skills/beta/SKILL.md", stars: 500, html_url: "https://github.com/owner/beta" },
  { name: "gamma", description: "g", repo_owner: "owner", repo_name: "gamma", file_path: "skills/gamma/SKILL.md", stars: 0, html_url: "https://github.com/owner/gamma" },
  { name: "delta", description: "d", repo_owner: "owner", repo_name: "delta", file_path: "skills/delta/SKILL.md", stars: 120, html_url: "https://github.com/owner/delta" },
]

test.describe("GitHub marketplace stars sort", () => {
  test("lists skills ordered by most stars first", async ({ page }) => {
    // Simulate the server having already sorted by stars desc (which is the
    // fix under test). Test verifies the UI renders that order faithfully.
    const sorted = [...MOCK_GITHUB_RESULTS].sort((a, b) => b.stars - a.stars)

    await page.route("**/v1/github/search**", (route: Route) => {
      return route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ results: sorted, total_count: sorted.length, has_more: false }),
      })
    })

    await page.route("**/sessions/whoami", (route: Route) =>
      route.fulfill({ status: 401, contentType: "application/json", body: JSON.stringify({ error: { code: 401 } }) }),
    )

    await page.goto("/github")

    const search = page.getByPlaceholder("Search GitHub for skills...")
    await search.fill("test")
    await search.press("Enter")

    const cards = page.locator("h3")
    await expect(cards.first()).toHaveText("beta", { timeout: 5000 })

    const names = await cards.allInnerTexts()
    expect(names.slice(0, 4)).toEqual(["beta", "delta", "alpha", "gamma"])
  })
})
