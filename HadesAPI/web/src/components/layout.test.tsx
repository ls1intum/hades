import { describe, expect, it } from "vitest";
import { formatVersion } from "./layout";

describe("formatVersion", () => {
  it("prefixes v for numeric release tags", () => {
    expect(formatVersion("1.0.0")).toBe("v1.0.0");
    expect(formatVersion("1.0.0-rc1")).toBe("v1.0.0-rc1");
  });

  it("leaves non-numeric tags untouched", () => {
    expect(formatVersion("latest")).toBe("latest");
    expect(formatVersion("dev")).toBe("dev");
    expect(formatVersion("v1.0.0")).toBe("v1.0.0");
    expect(formatVersion("abc123")).toBe("abc123");
  });
});
