import { emitUnauthorized } from "./auth-events";
import type { JobDetail, JobSummary, LogsResponse, Metrics } from "./types";

/** ApiError carries the HTTP status so callers can special-case 401. */
export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

// Endpoints where a 401 is an expected, local outcome (wrong password / the
// initial "am I logged in?" probe) rather than a session that just expired.
const NON_SESSION_401 = new Set(["/api/login", "/api/session"]);

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    credentials: "same-origin",
    headers: { "Content-Type": "application/json" },
    ...init,
  });
  if (!res.ok) {
    // A 401 on an authenticated request means the session expired/was revoked;
    // signal the app to clear auth and redirect to login.
    if (res.status === 401 && !NON_SESSION_401.has(path)) {
      emitUnauthorized();
    }
    let message = res.statusText;
    try {
      const body = await res.json();
      if (body?.error) message = body.error;
    } catch {
      // non-JSON error body; keep statusText
    }
    throw new ApiError(res.status, message);
  }
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

export const api = {
  login: (username: string, password: string) =>
    request<{ username: string; version?: string }>("/api/login", {
      method: "POST",
      body: JSON.stringify({ username, password }),
    }),

  logout: () => request<void>("/api/logout", { method: "POST" }),

  session: () => request<{ username: string; version?: string }>("/api/session"),

  jobs: (status?: string) =>
    request<{ jobs: JobSummary[] }>(
      "/api/jobs" + (status ? `?status=${encodeURIComponent(status)}` : ""),
    ).then((r) => r.jobs ?? []),

  job: (id: string) => request<JobDetail>(`/api/jobs/${encodeURIComponent(id)}`),

  logs: (id: string) =>
    request<LogsResponse>(`/api/jobs/${encodeURIComponent(id)}/logs`),

  metrics: () => request<Metrics>("/api/metrics"),
};
