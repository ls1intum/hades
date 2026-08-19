import { describe, expect, it } from "vitest";
import { formatVersion } from "./layout";

describe("formatVersion", () => {
  it("prefixes v for numeric release tags", () => {
    expect(formatVersion("1.0.0")).toBe("v1.0.0");
    expect(formatVersion("1.0.0-rc1")).toBe("v1.0.0-rc1");
  });

  it("leaves non-release tags untouched", () => {
    expect(formatVersion("latest")).toBe("latest");
    expect(formatVersion("dev")).toBe("dev");
    expect(formatVersion("v1.0.0")).toBe("v1.0.0");
    expect(formatVersion("abc123")).toBe("abc123");
    expect(formatVersion("pr-494")).toBe("pr-494");
  });

  it("does not prefix a numeric-leading commit SHA", () => {
    expect(formatVersion("4636d0f")).toBe("4636d0f");
    expect(formatVersion("0badc0de")).toBe("0badc0de");
  });
});
