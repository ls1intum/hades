import { defineConfig, devices } from "@playwright/test";

const PORT = Number(process.env.E2E_API_PORT || 8099);
const BASE_URL = `http://localhost:${PORT}`;

// End-to-end tests drive the real stack: a NATS container + HadesAPI serving the
// embedded SPA (see e2e/serve.sh). The API sets Secure session cookies, which
// Chromium accepts over http://localhost (a trusted secure context).
export default defineConfig({
  testDir: "./e2e",
  testMatch: "**/*.spec.ts",
  fullyParallel: false,
  workers: 1,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [["github"], ["list"]] : "list",
  globalTeardown: "./e2e/global-teardown.ts",
  timeout: 30_000,
  expect: { timeout: 10_000 },
  use: {
    baseURL: BASE_URL,
    trace: "on-first-retry",
    video: "retain-on-failure",
  },
  projects: [
    { name: "chromium", use: { ...devices["Desktop Chrome"] } },
  ],
  webServer: {
    command: "bash ./e2e/serve.sh",
    url: `${BASE_URL}/ping`,
    reuseExistingServer: !process.env.CI,
    timeout: 180_000,
    stdout: "pipe",
    stderr: "pipe",
  },
});
