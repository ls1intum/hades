---
sidebar_position: 2
title: Configuration Reference
---

# Configuration Reference

Hades is configured entirely through environment variables (loaded via `caarlos0/env`, plus an optional `.env` file for local runs). This is the single source of truth for every variable each component reads. A blank **Default** means the variable is optional and unset by default.

:::tip
`.env.example` at the repository root is a ready-to-copy template covering the common variables.
:::

## Global (all components)

| Variable | Default | Description |
| -------- | ------- | ----------- |
| `DEBUG` | `false` | Set to `true` for verbose (debug-level) logging. Read by every binary. |

## NATS connection (all components)

| Variable | Default | Description |
| -------- | ------- | ----------- |
| `NATS_URL` | `nats://localhost:4222` | NATS server URL. |
| `NATS_USERNAME` | | NATS username (optional). |
| `NATS_PASSWORD` | | NATS password (optional). |
| `NATS_TLS_ENABLED` | `false` | Enable TLS for the NATS connection. |

## HadesAPI

| Variable | Default | Description |
| -------- | ------- | ----------- |
| `API_PORT` | `8080` | Port the HTTP API listens on. |
| `AUTH_KEY` | | HTTP Basic Auth key for the `hades` user. Empty disables auth. |

:::note Reserved (not yet implemented)
`PROMETHEUS_ADDRESS`, `RETENTION_IN_MIN`, `MAX_RETRIES`, and `TIMEOUT_IN_MIN` appear in `.env.example` but are **not read by any component today**. They are placeholders for planned features.
:::

## HadesScheduler

| Variable | Default | Description |
| -------- | ------- | ----------- |
| `CONCURRENCY` | `1` | Number of jobs processed concurrently. |
| `HADES_EXECUTOR` | `docker` | Execution platform: `docker` or `k8s`. |

Depending on `HADES_EXECUTOR`, the Docker or Kubernetes variables below also apply.

### Docker executor (`HADES_EXECUTOR=docker`)

| Variable | Default | Description |
| -------- | ------- | ----------- |
| `DOCKER_HOST` | `unix:///var/run/docker.sock` | Docker daemon endpoint. |
| `DOCKER_CONTAINER_AUTOREMOVE` | `false` | Auto-remove step containers after they exit. |
| `DOCKER_SCRIPT_EXECUTOR` | `/bin/bash -c` | Shell used to run each step's `script`. |
| `DOCKER_CPU_LIMIT` | | Default CPU limit (whole CPUs) when a step sets none. |
| `DOCKER_MEMORY_LIMIT` | | Default memory limit (e.g. `4g`) when a step sets none. |

### Kubernetes executor (`HADES_EXECUTOR=k8s`)

| Variable | Default | Description |
| -------- | ------- | ----------- |
| `K8S_CONFIG_MODE` | `kubeconfig` | Client mode: `kubeconfig`, `serviceaccount` (legacy), or `operator` (create `BuildJob` CRs). **Deployments default to `operator`.** |
| `K8S_NAMESPACE` | `hades-executor` | Namespace jobs are scheduled into. |
| `KUBECONFIG` | | Path to a kubeconfig file (only in `kubeconfig` mode). |
| `BUILDJOB_GROUP` | `build.hades.tum.de` | API group of the `BuildJob` CRD. |
| `BUILDJOB_VERSION` | `v1` | API version of the `BuildJob` CRD. |
| `BUILDJOB_RESOURCE` | `buildjobs` | Plural resource name of the `BuildJob` CRD. |

## HadesOperator

Only deployed when the scheduler runs in `operator` mode.

| Variable | Default | Description |
| -------- | ------- | ----------- |
| `WATCH_NAMESPACE` | | Namespace the operator watches (empty = all namespaces, subject to RBAC). |
| `DELETE_ON_COMPLETE` | `true` | Delete the `BuildJob` CR (and its `Job`) once it finishes. |
| `MAX_PARALLELISM` | `100` | Maximum concurrent Jobs the operator admits; excess are suspended. |
| `DEV_MODE` | `false` | Enable the controller-runtime development logger. |

The operator also accepts standard controller-runtime flags: `--health-probe-bind-address` (default `:8083`), `--leader-elect`, and the metrics flags.

## HadesLogManager

Deployed by the Helm chart (`hades-log-manager`); also run locally via `make run`.

| Variable | Default | Description |
| -------- | ------- | ----------- |
| `HADESLOGMANAGER_API_PORT` | `8081` | HTTP API port. |
| `LOG_BATCH_SIZE` | `100` | Log entries buffered before a flush. |
| `LOG_RETENTION` | `1h` | How long completed-job logs are kept in memory (Go duration). |
| `MAX_JOB_LOGS` | `1000` | Max log entries retained per job. |

Log forwarding is configured per job, not globally: set an optional `callback_url` (an absolute `http`/`https` URL with a host) on the build request and the Log Manager forwards that job's aggregated logs there. If omitted, the job's logs are not forwarded.
