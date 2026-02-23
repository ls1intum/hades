---
sidebar_position: 1
---

# What is Hades?

**Hades** is a scalable, container-native job scheduler built for executing containerized workloads reliably and at scale. It was designed with simplicity and extensibility in mind — from local development all the way to production Kubernetes clusters.

## Core Design Goals

| Goal | Description |
|---|---|
| **Simplicity** | Focuses on delivering the essentials needed to execute containerized jobs without unnecessary complexity. |
| **Scalability** | Capable of queuing and executing a large number of jobs in parallel, making it ideal for high-traffic scenarios such as student exam submissions. |
| **Container-Based Isolation** | Every job step runs inside its own container, ensuring consistent execution environments and strong security boundaries between workloads. |
| **Kubernetes Native** | First-class support for Kubernetes via the Hades Operator, leveraging CRDs and cloud-native patterns for production deployments. |
| **Extensibility** | Designed to plug into different execution backends (Docker, Kubernetes, Kubernetes Operator) with minimal configuration changes. |

## Architecture

Hades is composed of three core services that work together:

```
┌─────────┐         ┌─────────┐          ┌───────────────┐
│         │         │         │          │               │
│  API    │────────▶│  NATS   │─────────▶│  Scheduler    │
│         │         │ Queue   │          │               │
└─────────┘         └─────────┘          └───────┬───────┘
                                                 │
                                                 ▼
                        ┌────────────────────────┴───────────────────────┐
                        │                                                │
                        ▼                                                ▼
                 ┌─────────────┐                               ┌─────────────────┐
                 │             │                               │                 │
                 │   Docker    │                               │   Kubernetes    │
                 │  Executor   │                               │    Executor     │
                 │             │                               │                 │
                 └─────────────┘                               └─────────────────┘
```

### Components

- **API** — The main entry point. Accepts job submissions, validates payloads, and publishes build events to the NATS queue.
- **NATS (JetStream)** — The message broker that decouples job submission from execution, enabling reliable async processing and back-pressure.
- **Scheduler** — Consumes events from NATS and dispatches jobs to the configured executor backend.

### Executor Backends

Hades supports three execution modes:

| Mode | Use Case |
|---|---|
| **Docker** | Local development and single-host deployments |
| **Kubernetes Executor** *(deprecated)* | Legacy Kubernetes integration |
| **Hades Operator** *(recommended)* | Production Kubernetes — uses CRDs and a native controller pattern |

## How a Job Works

1. **Submit** — A job (with one or more steps) is `POST`ed to the API.
2. **Queue** — The API publishes the job to NATS JetStream.
3. **Schedule** — The Scheduler picks up the event and dispatches it to the active executor.
4. **Execute** — Each step runs in its own container. Steps share data via a common volume.
5. **Complete** — Results and logs are stored and accessible via the API.

## Job Format

A job is a JSON document that defines a name and a list of ordered steps:

```json
{
  "name": "Example Job",
  "steps": [
    {
      "id": 1,
      "name": "Hello World",
      "image": "alpine:latest",
      "script": "echo 'Hello, Hades!'"
    }
  ]
}
```

Each step specifies the container image to use and the script to run inside it.

## What's Next?

- **[Installation](./installation/docker-mode)** — Get Hades running in Docker or Kubernetes.
- **[Usage Guide](./usage/submitting-jobs)** — Learn how to submit and monitor jobs.
- **[Operation Modes](./operation-modes/docker)** — Understand the different executor backends in depth.
- **[Helm Chart](./deployment/helm)** — Deploy Hades to Kubernetes with Helm.
- **[Traefik Deployment](./deployment/traefik)** — Expose Hades securely with automatic TLS via Traefik.
