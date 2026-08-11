import { describe, expect, it } from "vitest";
import { formatDuration, relativeTime } from "./utils";

describe("formatDuration", () => {
  it("never renders a 60s remainder", () => {
    // 119.6s would naively round the remainder to 60s.
    expect(formatDuration(119_600)).toBe("2m 0s");
  });
  it("formats sub-second and seconds", () => {
    expect(formatDuration(500)).toBe("500ms");
    expect(formatDuration(1500)).toBe("1.5s");
  });
  it("returns - for nullish", () => {
    expect(formatDuration(null)).toBe("-");
    expect(formatDuration(undefined)).toBe("-");
  });
});

describe("relativeTime", () => {
  it("handles future timestamps as 'just now'", () => {
    const future = new Date(Date.now() + 5000).toISOString();
    expect(relativeTime(future)).toBe("just now");
  });
  it("returns - for invalid input", () => {
    expect(relativeTime(null)).toBe("-");
    expect(relativeTime("not-a-date")).toBe("-");
  });
});
