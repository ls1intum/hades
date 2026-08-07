---
sidebar_position: 1
---

# What is Hades?

**Hades** is a scalable, container-native job scheduler built for executing containerized workloads reliably and at scale. It was designed with simplicity and extensibility in mind - from local development all the way to production Kubernetes clusters. It originated as the CI system for programming-exercise grading at TUM.

## Core Design Goals

| Goal | Description |
|---|---|
| **Simplicity** | Focuses on delivering the essentials needed to execute containerized jobs without unnecessary complexity. |
| **Scalability** | Capable of queuing and executing a large number of jobs in parallel, making it ideal for high-traffic scenarios such as student exam submissions. |
| **Container-Based Isolation** | Every job step runs inside its own container, ensuring consistent execution environments and strong security boundaries between workloads. |
| **Kubernetes Native** | First-class support for Kubernetes via the Hades Operator, leveraging CRDs and cloud-native patterns for production deployments. |
| **Extensibility** | Designed to plug into different execution backends (Docker, Kubernetes, Kubernetes Operator) with minimal configuration changes. |

## Architecture

A user submits a multi-step job to the **API**, which enqueues it on **NATS JetStream**. The **Scheduler** consumes it and runs each step in a container, either via the Docker daemon (local development) or Kubernetes (production, via the **Hades Operator** and a `BuildJob` custom resource). The **Log Manager** aggregates per-job logs from NATS for inspection.

```
┌─────────┐   jobs    ┌─────────┐   jobs   ┌───────────────┐
│  API    │──────────▶│  NATS   │─────────▶│  Scheduler    │
└─────────┘           │ Queue   │          └───────┬───────┘
                      └────┬────┘                  │
               status │    ▲ logs                  ▼
                      ▼    │            ┌───────────┴───────────┐
                ┌──────────┴──┐         ▼                       ▼
                │    Log      │   ┌─────────────┐      ┌──────────────────┐
                │   Manager   │   │   Docker    │      │  Kubernetes /    │
                │  (HTTP API) │   │  Executor   │      │  Hades Operator  │
                └─────────────┘   └─────────────┘      └──────────────────┘
```

### Components

- **API** - The main entry point. Accepts job submissions, validates payloads, assigns a UUID, and publishes build events to the NATS queue by priority.
- **NATS (JetStream)** - The message broker that decouples job submission from execution, enabling reliable async processing and back-pressure.
- **Scheduler** - Consumes events from NATS and dispatches jobs to the configured executor backend.
- **Hades Operator** - A Kubernetes controller that reconciles `BuildJob` custom resources into Kubernetes `Job`s (production execution path).
- **Log Manager** - Subscribes to job status and log events on NATS and exposes aggregated per-job logs over an HTTP API. Local-development aid.

### Executor Backends

Hades supports three execution modes:

| Mode | Use Case |
|---|---|
| **Docker** | Local development and single-host deployments |
| **Kubernetes Executor** *(deprecated)* | Legacy direct Kubernetes integration |
| **Hades Operator** *(recommended)* | Production Kubernetes - uses CRDs and a native controller pattern |

## How a Job Works

1. **Submit** - A job (with one or more steps) is `POST`ed to the API.
2. **Queue** - The API publishes the job to NATS JetStream on the queue matching its priority.
3. **Schedule** - The Scheduler picks up the event and dispatches it to the active executor.
4. **Execute** - Each step runs in its own container. Steps share data via a common `/shared` volume.
5. **Complete** - Status transitions and logs are published back to NATS and are accessible via the Log Manager.

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

Each step specifies the container image to use and the script to run inside it. See the [API reference](./reference/api) for the full payload schema.

## What's Next?

- **[Installation](./installation/docker-mode)** - Get Hades running in Docker or Kubernetes.
- **[Usage Guide](./usage/submitting-jobs)** - Learn how to submit and monitor jobs.
- **[Operation Modes](./operation-modes/docker)** - Understand the different executor backends in depth.
- **[Helm Chart](./deployment/helm)** - Deploy Hades to Kubernetes with Helm.
- **[Traefik Deployment](./deployment/traefik)** - Expose Hades securely with automatic TLS via Traefik.
