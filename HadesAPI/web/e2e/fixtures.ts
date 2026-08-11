import {
  test as base,
  expect,
  type APIRequestContext,
  type Page,
} from "@playwright/test";

export const CREDENTIALS = { username: "admin", password: "test-password" };

export interface JobOverrides {
  name?: string;
  priority?: number;
  metadata?: Record<string, string>;
  script?: string;
}

/**
 * submitJob enqueues a job via the unauthenticated POST /build endpoint and
 * returns its id. By default the job carries a visible key plus two
 * secret-bearing values so redaction can be asserted.
 */
export async function submitJob(
  request: APIRequestContext,
  overrides: JobOverrides = {},
): Promise<string> {
  const job = {
    name: overrides.name ?? "e2e-job",
    priority: overrides.priority ?? 3,
    metadata: {
      REPO_URL: "https://github.com/org/repo.git",
      GIT_PASSWORD: "supersecret-value",
      DATABASE_URL: "postgres://dbuser:dbpass@db:5432/app",
      ...(overrides.metadata ?? {}),
    },
    steps: [
      {
        id: 1,
        name: "checkout",
        image: "alpine:latest",
        script: overrides.script ?? "echo 'building the thing'",
      },
    ],
  };
  const res = await request.post("/build", { data: job });
  expect(res.ok(), `POST /build failed: ${res.status()}`).toBeTruthy();
  const body = await res.json();
  return body.job_id as string;
}

/** login drives the login form and waits for the dashboard to load. */
export async function login(page: Page): Promise<void> {
  await page.goto("/login");
  await page.getByLabel(/username/i).fill(CREDENTIALS.username);
  await page.getByLabel(/password/i).fill(CREDENTIALS.password);
  await page.getByRole("button", { name: /sign in/i }).click();
  await expect(page.getByRole("heading", { name: "Overview" })).toBeVisible();
}

/** authedPage is a Page fixture that is already logged in. */
export const test = base.extend<{ authedPage: Page }>({
  authedPage: async ({ page }, use) => {
    await login(page);
    await use(page);
  },
});

export { expect };
