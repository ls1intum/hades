# Hades Dashboard (web UI)

A React + TypeScript single-page app (Vite, Tailwind v4, shadcn/ui) that provides
a secured, live operator dashboard for Hades: job list, job detail with redacted
metadata and logs, and system metrics. The header shows the deployed version,
which the API returns in the `/api/session` and `/api/login` responses from the
`HADES_VERSION` env var (set from the deployed container image tag; `dev` locally).

It is **embedded into the `HadesAPI` binary** (`//go:embed` of `./dist`) and
served from the API origin, so in production the app talks to the same host over
`/api/*` with a session cookie - no separate service and no CORS.

## Develop

Run the API (and NATS) with the dashboard enabled, then start the Vite dev
server, which proxies `/api` to the API:

```fish
# terminal 1: backend (dashboard env set - see repo .env.example)
make run

# terminal 2: SPA dev server on http://localhost:5173
make ui-dev
```

Point the proxy at a different API with `HADES_API_URL=http://host:8080 npm run dev`.

## Build

```fish
make ui-build   # npm ci && vite build -> ./dist
```

`./dist` is embedded by the Go build. A placeholder `dist/index.html` is committed
so the Go module compiles before a UI build; the real assets under `dist/` are
git-ignored and produced by the build (the Docker image builds them in a Node
stage automatically).

## Test / typecheck

```fish
make ui-test    # vitest (component/unit tests)
npm run lint    # tsc --noEmit
```

## End-to-end tests (Playwright)

`e2e/` contains a Playwright suite that exercises the **whole stack** in a real
browser: `e2e/serve.sh` boots a NATS container and `HadesAPI` (with the dashboard
enabled and the SPA embedded), and the specs cover login/logout, the jobs list,
job detail with secret redaction, live SSE updates, the logs graceful-degradation
path, and the metrics overview.

```fish
make ui-e2e                       # installs the browser, boots the stack, runs the suite
# or, from HadesAPI/web:
npx playwright install chromium
npm run test:e2e                  # headless
npm run test:e2e:ui               # interactive UI mode
```

Requirements: Docker (for the NATS container), Go, and Node. The suite uses a
fixed test login (`admin` / `test-password`); the API is started on port `8099`
and NATS on `4223` so it does not collide with a local dev stack. It runs in CI
via `.github/workflows/e2e.yml`.

## Layout

- `src/lib/` - API client (`api.ts`), shared types, helpers.
- `src/hooks/useStream.ts` - SSE subscription feeding the React Query cache.
- `src/context/auth.tsx` - session state (login/logout via `/api/*`).
- `src/components/ui/` - shadcn-style primitives (button, card, table, tabs, ...).
- `src/components/` - dashboard components (layout, jobs table, metrics, logs, metadata).
- `src/pages/` - login, overview, jobs, job detail.

## Security notes

- Secret metadata **and** step scripts are redacted **server-side** (key- and
  value-heuristics); the client only ever renders the mask token, shown as a
  "redacted" chip.
- Job **logs** are proxied and shown verbatim - the logs view carries a visible
  warning that they may contain secrets Hades does not scrub.
- All `/api/*` calls are same-origin with a `HttpOnly; Secure; SameSite=Strict`
  session cookie. A 401 on any authenticated request clears auth and redirects to
  the login page (see `src/lib/api.ts` + `src/lib/auth-events.ts`).
- The app is served under a strict `Content-Security-Policy` (same-origin scripts,
  no inline JS); keep new code free of inline `<script>` and remote asset loads.
