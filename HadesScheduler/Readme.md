# HadesScheduler

HadesScheduler consumes queued jobs from NATS JetStream and executes each job's steps in containers. It is the component that actually runs work; the [API](../HadesAPI/Readme.md) only enqueues it.

## How it works

The scheduler subscribes to the priority-bucketed job subjects (`hades.jobs.{high,medium,low}`), pulls jobs up to `CONCURRENCY` at a time, and dispatches each to an **executor** chosen by the `HADES_EXECUTOR` environment variable.

```text
NATS (hades.jobs.*) ──► HadesScheduler ──► executor ──► containers (one per step)
```

## Executors

### Docker (`HADES_EXECUTOR=docker`) - local development

Runs each step as a Docker container on the local daemon. Steps of a job share a per-job named volume `shared-<uuid>`, so a file written by one step is visible to later steps. The volume is created before the first step and removed after the last one. A step marked `continue_on_error: true` does not fail the job if it exits non-zero. Package: [`docker/`](docker).

### Kubernetes (`HADES_EXECUTOR=k8s`) - production

The Kubernetes path has three modes, selected by `K8S_CONFIG_MODE`:

| Mode | Behavior |
| ---- | -------- |
| `operator` (**recommended, deployment default**) | Creates a `BuildJob` custom resource; the [HadesOperator](HadesOperator/Readme.md) reconciles it into a Kubernetes `Job`. |
| `serviceaccount` (legacy) | Builds a `batchv1.Job` directly from within the cluster. |
| `kubeconfig` | Like `serviceaccount` but authenticates via a kubeconfig file (`KUBECONFIG`); intended for out-of-cluster use, not in-cluster deployment. |

> The scheduler's own default for `K8S_CONFIG_MODE` is `kubeconfig`, but every deployment (Helm, `.env.example`) sets `operator`. Package: [`k8s/`](k8s).

## Configuration

Common variables: `HADES_EXECUTOR` (default `docker`), `CONCURRENCY` (default `1`), `NATS_URL`, and the executor-specific `DOCKER_*` or `K8S_*` variables. See [docs/configuration.md](../docs/configuration.md) for the complete list.

## Run locally

```bash
make run-scheduler   # from the repository root (auto-starts NATS in Docker)
```
