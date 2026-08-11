import { test, expect, login, CREDENTIALS } from "./fixtures";

test.describe("authentication", () => {
  test("redirects unauthenticated visitors to the login page", async ({ page }) => {
    await page.goto("/");
    await expect(page).toHaveURL(/\/login$/);
    await expect(page.getByRole("button", { name: /sign in/i })).toBeVisible();
  });

  test("rejects invalid credentials with an error", async ({ page }) => {
    await page.goto("/login");
    await page.getByLabel(/username/i).fill(CREDENTIALS.username);
    await page.getByLabel(/password/i).fill("wrong-password");
    await page.getByRole("button", { name: /sign in/i }).click();
    await expect(page.getByRole("alert")).toContainText(/invalid/i);
    await expect(page).toHaveURL(/\/login$/);
  });

  test("logs in and out", async ({ page }) => {
    await login(page);
    await expect(page.getByText(CREDENTIALS.username)).toBeVisible();

    await page.getByRole("button", { name: /log out/i }).click();
    await expect(page).toHaveURL(/\/login$/);

    // Session is cleared: navigating back to the app returns to login.
    await page.goto("/jobs");
    await expect(page).toHaveURL(/\/login$/);
  });
});
