# AGENTS.md

Quick orientation for AI coding agents working on Hades. Pair this with `Readme.md` for user-facing docs; this file focuses on internal layout and gotchas.

## What this repo is

Hades is a job scheduler for containerized CI workloads (originally for programming-exercise grading at TUM). A user POSTs a multi-step job to `HadesAPI`, the API enqueues it on NATS JetStream, and `HadesScheduler` consumes it and runs each step in a container - either via the Docker daemon (local dev) or Kubernetes (production, via the `HadesOperator` and a `BuildJob` CRD). `HadesLogManager` is an in-memory aggregator that subscribes to log/status events on NATS and exposes them over HTTP.

## Repo layout

Go workspace (`go.work`, Go 1.25) with five modules:

| Module                              | Binary / role                                                                                                                                                                       |
| ----------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `HadesAPI/`                         | Gin HTTP server. `POST /build` validates a payload, assigns a UUID, publishes to NATS by priority. `GET /ping` health check. Optional Basic Auth via `AUTH_KEY`.                    |
| `HadesScheduler/`                   | NATS consumer. Reads `HADES_EXECUTOR` and dispatches to either the `docker/` or `k8s/` package.                                                                                     |
| `HadesScheduler/docker/`            | Runs each step as a Docker container; shares state between steps via a per-job named volume `shared-<uuid>`.                                                                        |
| `HadesScheduler/k8s/`               | Three sub-modes selected by `K8S_CONFIG_MODE`: `kubeconfig`, `serviceaccount` (legacy: builds a `batchv1.Job` directly), or `operator` (creates a `BuildJob` CR via dynamic client). |
| `HadesScheduler/HadesOperator/`     | Standalone kubebuilder operator. Watches `BuildJob` CRs (`build.hades.tum.de/v1`) and reconciles them into `batchv1.Job`s with one initContainer per step plus a finalizer pod.     |
| `HadesLogManager/`                  | Subscribes to `hades.jobstatus.*` and `hades.joblog.*` on NATS, aggregates logs in-memory (`sync.Map`), exposes `GET /jobs`, `/jobs/:id/logs`, `/jobs/:id/status` on port 8081.      |
| `shared/`                           | Cross-module: `payload` (DTOs), `nats` (publisher/consumer/connection), `buildlogs` (log types + `LogPublisher`/`LogAggregator` interfaces), `buildstatus` (job status enum + subjects), `utils` (env config loader, memory-limit parsing), `prio.go` (priority enum: high/medium/low ←→ ints). |

## Key contracts

- **Queue subjects:** `hades.jobs.{high,medium,low}` (priority-bucketed JetStream subjects).
- **Status subjects:** `hades.jobstatus.{Queued,Running,Succeeded,Failed,Stopped}` (payload = job ID string).
- **Log subjects:** see `shared/buildlogs/buildlog_stream.go`.
- **Job DTO:** `shared/payload/payload.go` (`QueuePayload` with ordered `Step`s; each step has `image`, optional `script`, `metadata` env vars, `cpu_limit`, `memory_limit`).
- **CRD:** `BuildJob` in `HadesScheduler/HadesOperator/api/v1/buildjob_types.go`. **Important:** if you change `BuildJobSpec`, run `make -C HadesScheduler/HadesOperator manifests generate` and commit `helm/hades/crds/build.hades.tum.de_buildjobs.yaml` and `zz_generated.deepcopy.go` - the `verify-crd.yml` GitHub workflow will fail otherwise. The `BuildJobSpec` is intentionally duplicated from `shared/payload`; keep them in sync manually.

## Build / test

```fish
# whole workspace
go build ./...
go test ./...
go work sync

# operator (kubebuilder targets)
make -C HadesScheduler/HadesOperator manifests generate fmt vet test

# helm chart
helm dependency build ./helm/hades
helm lint ./helm/hades
helm template hades ./helm/hades -n hades
```
## Running locally

The top-level `Makefile` wraps the common workflows (`make help` lists every target):

- **CLI mode:** `make run` runs `HadesAPI`, `HadesScheduler`, and `HadesLogManager` via `go run` and auto-starts NATS in Docker.
- **Docker mode:** `make docker-run` brings up `hadesAPI` (8081→8080), `hadesScheduler` (docker executor), and `nats` via `compose.yml`. API requests: see `bruno/api/*.bru`.
- **K8s mode:** `helm upgrade --install hades ./helm/hades -n hades --create-namespace`. The chart deploys API, scheduler (configMode=operator), operator, and embedded NATS JetStream.

`CODE_REVIEW.md` lists historical issues - several have already been fixed (graceful shutdown in API/scheduler, namespace `AlreadyExists` handling, `Kubeconfig` field exported). Re-verify against current code before acting on any item there.

## Conventions

- Logging: `log/slog` everywhere. Set `DEBUG=true` for debug level. Operator uses `sigs.k8s.io/controller-runtime` `zap` logger.
- Config: `caarlos0/env/v11` + `joho/godotenv`. Each binary has its own `Config` struct that calls `utils.LoadConfig`. Field tags must be exported; reflection silently ignores lowercase fields.
- Errors: prefer wrapping (`fmt.Errorf("...: %w", err)`) and returning to the caller; avoid `log.Fatal` in libraries.
- Tests: `HadesAPI/router_test.go` spins up NATS via `testcontainers-go`; expect Docker to be running locally.

## Things to avoid

- Don't bypass the operator path by writing a new direct-K8s scheduler; the operator is the strategic direction.
- Don't add fields to `BuildJobSpec` without regenerating the CRD (CI will block the PR).
- Don't introduce package-level mutable globals for dependencies - the codebase has been moving away from them (see `setupRouter` taking `JobPublisher` as a parameter).
- Don't assume `HadesLogManager` is part of the deployed system when reasoning about production end-to-end flows; it is currently CLI-only (no Dockerfile / helm template).
