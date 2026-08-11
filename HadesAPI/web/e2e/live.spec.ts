import { test, expect, submitJob } from "./fixtures";

test.describe("live updates (SSE)", () => {
  test("a newly submitted job appears without a reload", async ({
    authedPage,
    request,
  }) => {
    await authedPage.goto("/jobs");
    // The header shows a "Live" indicator once the SSE stream connects.
    await expect(authedPage.getByText("Live")).toBeVisible();

    const name = `live-${Date.now()}`;
    await submitJob(request, { name });

    // The row must appear via the SSE push. The list's fallback poll is 15s, so a
    // match within the default 10s expect timeout demonstrates the live channel.
    await expect(
      authedPage.getByRole("row", { name: new RegExp(name) }),
    ).toBeVisible();
  });
});
