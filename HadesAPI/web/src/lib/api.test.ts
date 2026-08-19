import { afterEach, describe, expect, it, vi } from "vitest";
import { api, ApiError } from "./api";
import { onUnauthorized } from "./auth-events";

function res(status: number, body: unknown): Response {
  return {
    ok: status >= 200 && status < 300,
    status,
    statusText: "",
    json: async () => body,
  } as Response;
}

describe("api 401 handling", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it("emits unauthorized on a 401 from an authenticated request", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => res(401, { error: "nope" })));
    let fired = false;
    const off = onUnauthorized(() => (fired = true));
    await expect(api.jobs()).rejects.toBeInstanceOf(ApiError);
    off();
    expect(fired).toBe(true);
  });

  it("does NOT emit unauthorized on a 401 from /api/login (wrong password)", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => res(401, { error: "invalid" })));
    let fired = false;
    const off = onUnauthorized(() => (fired = true));
    await expect(api.login("u", "bad")).rejects.toBeInstanceOf(ApiError);
    off();
    expect(fired).toBe(false);
  });

  it("does NOT emit unauthorized on the initial /api/session probe", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => res(401, {})));
    let fired = false;
    const off = onUnauthorized(() => (fired = true));
    await expect(api.session()).rejects.toBeInstanceOf(ApiError);
    off();
    expect(fired).toBe(false);
  });
});

describe("api version", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it("parses version from the session response", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => res(200, { username: "admin", version: "1.0.0" })),
    );
    await expect(api.session()).resolves.toEqual({
      username: "admin",
      version: "1.0.0",
    });
  });

  it("parses version from the login response", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => res(200, { username: "admin", version: "latest" })),
    );
    await expect(api.login("admin", "pw")).resolves.toEqual({
      username: "admin",
      version: "latest",
    });
  });
});
