---
sidebar_position: 1
---

# Docker Executor

The **Docker executor** is the default execution mode for local development and single-host deployments. It uses the Docker daemon on the host machine to run job steps as individual containers.

## When to Use

- Local development and testing
- Single-machine CI pipelines
- Demos and evaluation environments

## How It Works

When the Scheduler receives a job in Docker mode:

1. For each step, it pulls the specified Docker image (if not already cached).
2. It creates a container from that image, injecting the step's script and environment variables.
3. A shared Docker volume is mounted at `/shared` across all steps of the same job.
4. Steps are executed sequentially. The exit code of each container determines success or failure.
5. Logs are collected from the container's stdout/stderr and forwarded to the log manager.

## Configuration

Set the following environment variable in your `.env` file (or `compose.yml`):

```env
HADES_EXECUTOR=docker
```

This is the default value, so no changes are needed when using the provided `compose.yml`.

## Resource Requirements

| Requirement | Detail |
|---|---|
| Docker socket | Mounted at `/var/run/docker.sock` — the Scheduler must have access to it. |
| Network | The Scheduler and all job containers share the `hades` Docker network. |
| Volume | A named or anonymous volume is created per job for the `/shared` directory. |

## Limitations

- **Single host only** — all containers run on the machine where the Scheduler is running.
- **No auto-retry** — failed steps are not automatically retried; the job is marked as failed.
- **No resource quotas** — containers are not constrained by CPU or memory limits by default.

For production workloads requiring scalability, retry logic, and RBAC, use the [Hades Operator](./k8s-operator) instead.

