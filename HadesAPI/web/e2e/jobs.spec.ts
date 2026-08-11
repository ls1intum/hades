import { test, expect, submitJob } from "./fixtures";

test.describe("jobs list", () => {
  test("shows a submitted job as Queued", async ({ authedPage, request }) => {
    const name = `list-${Date.now()}`;
    await submitJob(request, { name });

    await authedPage.goto("/jobs");
    const row = authedPage.getByRole("row", { name: new RegExp(name) });
    await expect(row).toBeVisible();
    await expect(row.getByText("Queued")).toBeVisible();
    await expect(row.getByText("high")).toBeVisible();
  });

  test("filters by status", async ({ authedPage, request }) => {
    const name = `filter-${Date.now()}`;
    await submitJob(request, { name });
    await authedPage.goto("/jobs");

    await expect(authedPage.getByRole("row", { name: new RegExp(name) })).toBeVisible();

    // Queued filter keeps it; Succeeded filter (no scheduler in this stack) hides it.
    await authedPage.getByRole("button", { name: "Queued", exact: true }).click();
    await expect(authedPage.getByRole("row", { name: new RegExp(name) })).toBeVisible();

    await authedPage.getByRole("button", { name: "Succeeded", exact: true }).click();
    await expect(authedPage.getByRole("row", { name: new RegExp(name) })).toHaveCount(0);
  });

  test("navigates from a row to the detail view", async ({ authedPage, request }) => {
    const name = `nav-${Date.now()}`;
    const id = await submitJob(request, { name });
    await authedPage.goto("/jobs");
    await authedPage.getByRole("row", { name: new RegExp(name) }).click();
    await expect(authedPage).toHaveURL(new RegExp(`/jobs/${id}`));
    await expect(authedPage.getByRole("heading", { name })).toBeVisible();
  });
});
