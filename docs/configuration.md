# Configuration Reference

Hades is configured entirely through environment variables (loaded via `caarlos0/env` and, for local runs, an optional `.env` file read by `joho/godotenv`). This page is the single source of truth for every variable each component reads; component READMEs and the top-level `Readme.md` link here instead of duplicating tables.

Each table is derived directly from the `Config` structs in the code, cited per section. A blank **Default** means the variable is optional and unset by default.

> **Tip:** `.env.example` at the repository root is a ready-to-copy template covering the common variables.

## Global (all components)

| Variable | Default | Description | Source |
| -------- | ------- | ----------- | ------ |
| `DEBUG` | `false` | Set to `true` for verbose (debug-level) logging. Read by every binary. | `shared/utils/logging.go` |

## NATS connection (all components)

Every component connects to NATS with the same `ConnectionConfig`.

| Variable | Default | Description | Source |
| -------- | ------- | ----------- | ------ |
| `NATS_URL` | `nats://localhost:4222` | NATS server URL. | `shared/nats/connection.go` |
| `NATS_USERNAME` | | NATS username (optional). | `shared/nats/connection.go` |
| `NATS_PASSWORD` | | NATS password (optional). | `shared/nats/connection.go` |
| `NATS_TLS_ENABLED` | `false` | Enable TLS for the NATS connection. | `shared/nats/connection.go` |

## HadesAPI

| Variable | Default | Description | Source |
| -------- | ------- | ----------- | ------ |
| `API_PORT` | `8080` | Port the HTTP API listens on. | `HadesAPI/main.go` (`HadesAPIConfig`) |
| `AUTH_KEY` | | HTTP Basic Auth key for the `hades` user. Empty disables auth (a warning is logged). | `HadesAPI/main.go` (`HadesAPIConfig`) |

Plus the [NATS](#nats-connection-all-components) and [global](#global-all-components) variables.

> **Reserved (not yet implemented):** `PROMETHEUS_ADDRESS`, `RETENTION_IN_MIN`, `MAX_RETRIES`, and `TIMEOUT_IN_MIN` appear in `.env.example` and some compose files but are **not read by any component today**. They are placeholders for planned features and currently have no effect.

## HadesScheduler

| Variable | Default | Description | Source |
| -------- | ------- | ----------- | ------ |
| `CONCURRENCY` | `1` | Number of jobs processed concurrently. | `HadesScheduler/main.go` (`HadesSchedulerConfig`) |
| `HADES_EXECUTOR` | `docker` | Execution platform: `docker` or `k8s`. | `shared/utils/config.go` (`ExecutorConfig`) |

Plus the [NATS](#nats-connection-all-components) and [global](#global-all-components) variables, and - depending on `HADES_EXECUTOR` - the Docker or Kubernetes variables below.

### Docker executor (`HADES_EXECUTOR=docker`)

| Variable | Default | Description | Source |
| -------- | ------- | ----------- | ------ |
| `DOCKER_HOST` | `unix:///var/run/docker.sock` | Docker daemon endpoint. | `HadesScheduler/docker/env.go` (`EnvConfig`) |
| `DOCKER_CONTAINER_AUTOREMOVE` | `false` | Auto-remove step containers after they exit. Keep `false` to retain logs post-run. | `HadesScheduler/docker/env.go` |
| `DOCKER_SCRIPT_EXECUTOR` | `/bin/bash -c` | Shell used to run each step's `script`. | `HadesScheduler/docker/env.go` |
| `DOCKER_CPU_LIMIT` | | Default CPU limit (whole CPUs, e.g. `6`) when a step sets none. | `HadesScheduler/docker/env.go` |
| `DOCKER_MEMORY_LIMIT` | | Default memory limit (e.g. `4g`) when a step sets none. | `HadesScheduler/docker/env.go` |

### Kubernetes executor (`HADES_EXECUTOR=k8s`)

| Variable | Default | Description | Source |
| -------- | ------- | ----------- | ------ |
| `K8S_NAMESPACE` | `hades-executor` | Namespace jobs are scheduled into. | `HadesScheduler/k8s/k8s.go` |
| `BUILDJOB_GROUP` | `build.hades.tum.de` | API group of the `BuildJob` CRD. | `HadesScheduler/k8s/k8s.go` (`BuildJobGVRConfig`) |
| `BUILDJOB_VERSION` | `v1` | API version of the `BuildJob` CRD. | `HadesScheduler/k8s/k8s.go` |
| `BUILDJOB_RESOURCE` | `buildjobs` | Plural resource name of the `BuildJob` CRD. | `HadesScheduler/k8s/k8s.go` |

## HadesOperator

Only deployed when the scheduler runs in `operator` mode.

| Variable | Default | Description | Source |
| -------- | ------- | ----------- | ------ |
| `WATCH_NAMESPACE` | | Namespace the operator watches for `BuildJob` CRs. Empty means all namespaces (subject to RBAC). | `HadesScheduler/HadesOperator/cmd/main.go` (`NSConfig`) |
| `DELETE_ON_COMPLETE` | `true` | Delete the `BuildJob` CR (and its `Job`) once it finishes. Set `false` to retain them for debugging. | `HadesScheduler/HadesOperator/cmd/main.go` (`OperatorConfig`) |
| `MAX_PARALLELISM` | `100` | Maximum number of `Job`s the operator admits concurrently; excess jobs are suspended. | `HadesScheduler/HadesOperator/cmd/main.go` (`OperatorConfig`) |
| `DEV_MODE` | `false` | Enable the controller-runtime development logger. | `HadesScheduler/HadesOperator/cmd/main.go` |

Plus the [NATS](#nats-connection-all-components) variables (the operator publishes status/log events). The operator also accepts standard controller-runtime **flags**: `--health-probe-bind-address` (default `:8083`), `--leader-elect`, and the metrics flags bound via `opts.BindFlags`.

## HadesLogManager

Deployed by the Helm chart (`hades-log-manager`); also run locally via `make run`.

| Variable | Default | Description | Source |
| -------- | ------- | ----------- | ------ |
| `HADESLOGMANAGER_API_PORT` | `8081` | HTTP API port. | `HadesLogManager/main.go` (`HadesLogManagerConfig`) |
| `LOG_BATCH_SIZE` | `100` | Log entries buffered before a flush. | `HadesLogManager/processor.go` (`AggregatorConfig`) |
| `LOG_RETENTION` | `1h` | How long completed-job logs are kept in memory (Go duration). | `HadesLogManager/processor.go` |
| `MAX_JOB_LOGS` | `1000` | Max log entries retained per job. | `HadesLogManager/processor.go` |

Log forwarding is configured per job, not globally: set an optional `callback_url` (an absolute `http`/`https` URL with a host) on the build request and the Log Manager forwards that job's aggregated logs there. If omitted, the job's logs are not forwarded.

Plus the [NATS](#nats-connection-all-components) and [global](#global-all-components) variables.
