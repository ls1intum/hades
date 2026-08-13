# Dependabot security updates (2026-08-11)

Resolves the 18 open Dependabot alerts on `ls1intum/hades`. 16 are fixed; 2 have
no patched release in existence and are documented below.

## Go: migrate off `github.com/docker/docker`

Five alerts (#73, #75, #99, #100, #101) targeted `github.com/docker/docker`
v28.5.2 in `HadesScheduler`.

A version bump was not possible. That module path is frozen at v28.5.2 - Docker
Engine 29+ ships as `github.com/moby/moby/v2` (daemon) and
`github.com/moby/moby/client` (client), so the advisories have no fixed version
on the old path. Three of the five have no fixed version anywhere.

`testcontainers-go` v0.44.0, already a direct dependency, had migrated to
`github.com/moby/moby/client` v0.5.0 + `github.com/moby/moby/api` v1.55.0. So
`HadesScheduler` was the last consumer holding the legacy module in the tree.

Migrated the five files in `HadesScheduler/docker` to the new modules:

| Old | New |
|---|---|
| `github.com/docker/docker/client` | `github.com/moby/moby/client` |
| `github.com/docker/docker/api/types/...` | `github.com/moby/moby/api/types/...` |
| `github.com/docker/docker/pkg/jsonmessage` | `github.com/moby/moby/client/pkg/jsonmessage` |
| `github.com/docker/docker/pkg/stdcopy` | `github.com/moby/moby/api/pkg/stdcopy` |

API changes the new client required:

- Per-call options moved from the `api/types/*` packages onto the client
  package: `container.LogsOptions` → `client.ContainerLogsOptions`,
  `container.RemoveOptions` → `client.ContainerRemoveOptions`,
  `image.PullOptions` → `client.ImagePullOptions`,
  `container.StartOptions` → `client.ContainerStartOptions`,
  `volume.CreateOptions` → `client.VolumeCreateOptions`.
- `ContainerCreate` takes a single `client.ContainerCreateOptions` struct
  instead of five positional arguments.
- `ContainerRemove`, `ContainerStart`, and `VolumeRemove` now return a result
  struct alongside the error. `VolumeRemove`'s `force bool` became
  `client.VolumeRemoveOptions{Force: ...}`.
- `ContainerWait` returns one `ContainerWaitResult` struct carrying the
  `Result` and `Error` channels, instead of returning two channels.
- `NewClientWithOpts` is deprecated in favour of `New`, which negotiates the API
  version by default - so `WithAPIVersionNegotiation()` (now a no-op) was
  dropped.

Because the helper functions took a parameter named `client`, which now shadows
the package that holds the options types, those parameters were renamed to
`cli` (already the house style in `step.go` and `job.go`).

### Test coverage

`HadesScheduler/docker` had no tests, and every Docker API call in it changed.
Added `HadesScheduler/docker/scheduler_test.go`, which skips when no daemon is
reachable and otherwise runs real jobs end to end:

- a two-step job asserting the per-job shared volume carries state between steps
  (covers volume create/remove, image pull, container create/start/wait/logs/
  remove, and stdcopy demultiplexing),
- a failing step asserting a non-zero exit aborts the job and that stderr is
  still captured,
- a missing image asserting in-band pull errors surface rather than being
  silently drained.

## npm: `HadesAPI/web`

Alerts #109-#113. `vite` 6.0.5 → 6.4.3, `vitest` 2.1.8 → 3.2.7 (major),
`esbuild` → 0.25.12 transitively. `npm audit` is clean.

The stale `esbuild@0.21.5` entry in the `allowScripts` block was removed; that
version only existed under vitest 2.

## npm: `website`

Alerts #64, #102-#108, resolved via the `resolutions` block:

| Package | Before | After | Note |
|---|---|---|---|
| `webpack` | 5.97.1 (pinned) | 5.109.2 | |
| `webpackbar` | 6.0.1 | 7.0.0 | required by the webpack bump |
| `serialize-javascript` | 6.0.2 | 7.1.0 | |
| `uuid` | 8.3.2 | 11.1.1 | only consumer is `sockjs` (dev server) |
| `js-yaml` (v4 branch) | 4.3.0 | 4.3.1 | scoped so `gray-matter` keeps v3 |

Two things worth recording:

- The `webpack` pin at 5.97.1 was load-bearing. Docusaurus 3.9 bundles
  webpackbar 6, which passes `name`/`color`/`reporters`/`reporter` to webpack's
  `ProgressPlugin`; webpack >= 5.98 tightened that schema and rejects them, so
  the build fails. Resolving `webpackbar` to 7 is what unblocks the bump.
- The js-yaml resolution is scoped to `**/@redocly/openapi-core/js-yaml`, not
  global. `gray-matter` (Docusaurus frontmatter parsing) needs js-yaml v3 and
  calls `safeLoad`, which v4 removed.

Upgrading Docusaurus to 3.10.2 was tried first and reverted: 3.10 makes
`future.v4: true` imply the rspack-based "faster" bundler, which needs a new
`@docusaurus/faster` dependency. That is a bundler swap, not a security fix.

## Not fixed

| Alert | Package | Why |
|---|---|---|
| #107, #108 | `image-size` <= 2.0.2 | No patched release exists at any version. Reached only via `@docusaurus/mdx-loader` at docs build time, measuring images committed to this repo - the input is not attacker-controlled. Revisit when upstream publishes a fix. |

`golang.org/x/crypto/openpgp` also shows in `govulncheck` (GO-2026-5932,
unmaintained package) but is not a Dependabot alert and no Hades code path
reaches it - govulncheck reports "your code is affected by 0 vulnerabilities".

## Tooling fix

`make vuln` was broken two ways: it invoked a bare `govulncheck` that is not on
`PATH` after `go install`, and ran it from the workspace root where there is no
`go.mod`. It now resolves the binary from `GOPATH/bin` and iterates per module,
mirroring the existing `make lint` target.

## Verification

- `make build`, `make lint`, `make test` - pass.
- `make vuln` - no vulnerabilities in any module.
- Dashboard: `npm test` (22 unit tests), `npm run lint`, `npm run build`,
  `npm run test:e2e` (11 Playwright tests) - pass.
- Website: `yarn typecheck`, `yarn build`, and a dev-server smoke test
  (`yarn start`, HTTP 200) - pass.
