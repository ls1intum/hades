import { test, expect, submitJob } from "./fixtures";

test.describe("overview / metrics", () => {
  test("shows metric tiles and the status chart", async ({
    authedPage,
    request,
  }) => {
    await submitJob(request, { name: `overview-${Date.now()}` });
    await authedPage.goto("/");

    await expect(authedPage.getByRole("heading", { name: "Overview" })).toBeVisible();

    // Stat tiles (labels are unique on the page).
    await expect(authedPage.getByText("Throughput")).toBeVisible();
    await expect(authedPage.getByText("Avg duration")).toBeVisible();
    await expect(authedPage.getByText("Jobs by status")).toBeVisible();

    // Recent jobs section renders.
    await expect(
      authedPage.getByRole("heading", { name: "Recent jobs" }),
    ).toBeVisible();
  });
});
