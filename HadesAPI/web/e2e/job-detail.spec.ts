import { test, expect, submitJob } from "./fixtures";

test.describe("job detail", () => {
  test("redacts secret metadata but shows innocuous values", async ({
    authedPage,
    request,
  }) => {
    const id = await submitJob(request, { name: `detail-${Date.now()}` });
    await authedPage.goto(`/jobs/${id}`);

    // Metadata tab (default is Steps) - switch to Metadata.
    await authedPage.getByRole("tab", { name: /metadata/i }).click();

    // Innocuous value is visible.
    await expect(
      authedPage.getByText("https://github.com/org/repo.git"),
    ).toBeVisible();

    // Secret keys are shown, but their values are redacted, never leaked.
    await expect(authedPage.getByText("GIT_PASSWORD")).toBeVisible();
    await expect(authedPage.getByText("DATABASE_URL")).toBeVisible();
    await expect(authedPage.getByText("supersecret-value")).toHaveCount(0);
    await expect(authedPage.getByText("dbuser:dbpass")).toHaveCount(0);
    await expect(authedPage.getByText(/redacted/i).first()).toBeVisible();
  });

  test("shows step image and script", async ({ authedPage, request }) => {
    const id = await submitJob(request, {
      name: `steps-${Date.now()}`,
      script: "echo hello-from-e2e",
    });
    await authedPage.goto(`/jobs/${id}`);

    await expect(authedPage.getByText("checkout")).toBeVisible();
    await expect(authedPage.getByText("alpine:latest").first()).toBeVisible();
    await expect(authedPage.getByText("echo hello-from-e2e")).toBeVisible();
  });

  test("logs tab degrades gracefully when the log service is down", async ({
    authedPage,
    request,
  }) => {
    const id = await submitJob(request, { name: `logs-${Date.now()}` });
    await authedPage.goto(`/jobs/${id}`);
    await authedPage.getByRole("tab", { name: /logs/i }).click();

    // No HadesLogManager runs in this stack, so the proxy returns 503 and the
    // viewer degrades to an "unavailable" message instead of crashing.
    await expect(
      authedPage.getByText(/logs are currently unavailable/i),
    ).toBeVisible();
  });
});
