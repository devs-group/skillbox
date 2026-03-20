import { test, expect } from "./fixtures"

test.describe("Login Page", () => {
  test("renders login form with email and password fields", async ({ mockPage }) => {
    await mockPage.goto("/auth/login")
    await expect(mockPage.getByText(/welcome back/i)).toBeVisible()
    await expect(mockPage.getByText(/sign in to your account/i)).toBeVisible()
    await expect(mockPage.getByLabel(/email/i)).toBeVisible()
    await expect(mockPage.getByLabel(/password/i)).toBeVisible()
    await expect(mockPage.getByRole("button", { name: /sign in/i })).toBeVisible()
  })

  test("has link to registration page", async ({ mockPage }) => {
    await mockPage.goto("/auth/login")
    const registerLink = mockPage.getByRole("link", { name: /register/i })
    await expect(registerLink).toBeVisible()
    await expect(registerLink).toHaveAttribute("href", "/auth/register")
  })

  test("submit button is enabled when flow is loaded", async ({ mockPage }) => {
    await mockPage.goto("/auth/login")
    await expect(mockPage.getByLabel(/email/i)).toBeVisible()
    const submitBtn = mockPage.getByRole("button", { name: /sign in/i })
    await expect(submitBtn).toBeEnabled()
  })

  test("can fill in email and password", async ({ mockPage }) => {
    await mockPage.goto("/auth/login")
    await expect(mockPage.getByLabel(/email/i)).toBeVisible()

    await mockPage.getByLabel(/email/i).fill("admin@example.com")
    await mockPage.getByLabel(/password/i).fill("securepassword")

    await expect(mockPage.getByLabel(/email/i)).toHaveValue("admin@example.com")
    await expect(mockPage.getByLabel(/password/i)).toHaveValue("securepassword")
  })

  test("shows error on login flow initialization failure", async ({ page }) => {
    // Mock session as unauthenticated
    await page.route("**/sessions/whoami", (route) =>
      route.fulfill({ status: 401, contentType: "application/json", body: JSON.stringify({}) })
    )
    // Mock login flow to fail
    await page.route("**/self-service/login/browser**", (route) =>
      route.fulfill({ status: 500, contentType: "application/json", body: JSON.stringify({ error: "server_error" }) })
    )

    await page.goto("/auth/login")
    await expect(page.getByText(/failed to initialize login flow/i)).toBeVisible()
  })

  test("shows error on invalid credentials", async ({ page }) => {
    // Mock session as unauthenticated
    await page.route("**/sessions/whoami", (route) =>
      route.fulfill({ status: 401, contentType: "application/json", body: JSON.stringify({}) })
    )
    // Mock login flow success
    await page.route("**/self-service/login/browser**", (route) => {
      const { MOCK_LOGIN_FLOW } = require("./fixtures")
      return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(MOCK_LOGIN_FLOW) })
    })
    // Mock login submit to return error
    await page.route("**/self-service/login?flow=**", (route) => {
      if (route.request().method() === "POST") {
        return route.fulfill({
          status: 400,
          contentType: "application/json",
          body: JSON.stringify({
            id: "flow-login-123",
            ui: {
              nodes: [],
              messages: [{ id: 1, text: "The provided credentials are invalid", type: "error" }],
            },
          }),
        })
      }
      return route.continue()
    })

    await page.goto("/auth/login")
    await expect(page.getByLabel(/email/i)).toBeVisible()
    await page.getByLabel(/email/i).fill("bad@example.com")
    await page.getByLabel(/password/i).fill("wrong")
    await page.getByRole("button", { name: /sign in/i }).click()

    await expect(page.getByText(/invalid/i)).toBeVisible()
  })
})
