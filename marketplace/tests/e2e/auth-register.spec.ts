import { test, expect } from "./fixtures"

test.describe("Registration Page", () => {
  test("renders registration form", async ({ mockPage }) => {
    await mockPage.goto("/auth/register")
    await expect(mockPage.getByText(/create an account/i)).toBeVisible()
    await expect(mockPage.getByText(/sign up to get started/i)).toBeVisible()
    await expect(mockPage.getByLabel(/email/i)).toBeVisible()
    await expect(mockPage.getByLabel(/password/i)).toBeVisible()
    await expect(mockPage.getByRole("button", { name: /register/i })).toBeVisible()
  })

  test("has link to login page", async ({ mockPage }) => {
    await mockPage.goto("/auth/register")
    const loginLink = mockPage.getByRole("link", { name: /sign in/i })
    await expect(loginLink).toBeVisible()
    await expect(loginLink).toHaveAttribute("href", "/auth/login")
  })

  test("can fill registration form", async ({ mockPage }) => {
    await mockPage.goto("/auth/register")
    await expect(mockPage.getByLabel(/email/i)).toBeVisible()

    await mockPage.getByLabel(/email/i).fill("newuser@example.com")
    await mockPage.getByLabel(/password/i).fill("Str0ngP@ssword!")

    await expect(mockPage.getByLabel(/email/i)).toHaveValue("newuser@example.com")
    await expect(mockPage.getByLabel(/password/i)).toHaveValue("Str0ngP@ssword!")
  })

  test("shows error on flow initialization failure", async ({ page }) => {
    await page.route("**/sessions/whoami", (route) =>
      route.fulfill({ status: 401, contentType: "application/json", body: JSON.stringify({}) })
    )
    await page.route("**/self-service/registration/browser**", (route) =>
      route.fulfill({ status: 500, contentType: "application/json", body: JSON.stringify({ error: "server_error" }) })
    )

    await page.goto("/auth/register")
    await expect(page.getByText(/failed to initialize registration flow/i)).toBeVisible()
  })

  test("shows error on registration failure (weak password)", async ({ page }) => {
    await page.route("**/sessions/whoami", (route) =>
      route.fulfill({ status: 401, contentType: "application/json", body: JSON.stringify({}) })
    )
    const { MOCK_REGISTRATION_FLOW } = require("./fixtures")
    await page.route("**/self-service/registration/browser**", (route) =>
      route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(MOCK_REGISTRATION_FLOW) })
    )
    await page.route("**/self-service/registration?flow=**", (route) => {
      if (route.request().method() === "POST") {
        return route.fulfill({
          status: 400,
          contentType: "application/json",
          body: JSON.stringify({
            id: "flow-reg-456",
            ui: {
              nodes: [],
              messages: [{ id: 1, text: "Password must be at least 8 characters", type: "error" }],
            },
          }),
        })
      }
      return route.continue()
    })

    await page.goto("/auth/register")
    await expect(page.getByLabel(/email/i)).toBeVisible()
    await page.getByLabel(/email/i).fill("user@example.com")
    await page.getByLabel(/password/i).fill("short")
    await page.getByRole("button", { name: /register/i }).click()

    await expect(page.getByText(/password must be/i)).toBeVisible()
  })
})
