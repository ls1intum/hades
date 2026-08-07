# Hades Dashboard (web UI)

A React + TypeScript single-page app (Vite, Tailwind v4, shadcn/ui) that provides
a secured, live operator dashboard for Hades: job list, job detail with redacted
metadata and logs, and system metrics.

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
make ui-test    # vitest
npm run lint    # tsc --noEmit
```

## Layout

- `src/lib/` - API client (`api.ts`), shared types, helpers.
- `src/hooks/useStream.ts` - SSE subscription feeding the React Query cache.
- `src/context/auth.tsx` - session state (login/logout via `/api/*`).
- `src/components/ui/` - shadcn-style primitives (button, card, table, tabs, ...).
- `src/components/` - dashboard components (layout, jobs table, metrics, logs, metadata).
- `src/pages/` - login, overview, jobs, job detail.

## Security notes

- Secret metadata is redacted **server-side**; the client only ever renders the
  mask token and shows it as a "redacted" chip.
- Job **logs** and step **scripts** are shown verbatim and are **not** scrubbed -
  the logs view carries a visible warning to that effect.
