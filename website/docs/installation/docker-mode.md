---
sidebar_position: 1
---

# Docker Mode (Local Development)

Docker mode is the fastest way to get Hades running on your local machine. It uses Docker Compose to spin up all three services — the API, Scheduler, and NATS broker — on a single host.

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) v20.10+
- [Docker Compose](https://docs.docker.com/compose/install/) v2.0+

## Setup

### 1. Clone the Repository

```bash
git clone https://github.com/ls1intum/hades.git
cd hades
```

### 2. Configure Environment Variables

Copy the example environment file and review the defaults:

```bash
cp .env.example .env
```

The default configuration uses `docker` as the executor, so no further changes are required for local testing.

Key variables:

| Variable | Default | Description |
|---|---|---|
| `HADES_EXECUTOR` | `docker` | Execution backend. Use `docker` for local mode. |
| `CONCURRENCY` | `1` | Number of jobs processed concurrently. |
| `API_PORT` | `8080` | Port the Hades API listens on. |

### 3. Start the Services

```bash
docker compose up -d
```

This starts:
- **hadesAPI** on port `8081` (mapped from internal `8080`)
- **hadesScheduler** connected to Docker socket
- **nats** on ports `4222` (client) and `8222` (monitoring)

### 4. Verify the Stack Is Running

```bash
docker compose ps
```

You should see all three services in a healthy/running state. You can also open the NATS monitoring dashboard at [http://localhost:8222](http://localhost:8222).

## Submit Your First Job

```bash
curl -X POST http://localhost:8081/build \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Hello World",
    "steps": [
      {
        "id": 1,
        "name": "Say Hello",
        "image": "alpine:latest",
        "script": "echo Hello from Hades!"
      }
    ]
  }'
```

## Stopping the Stack

```bash
docker compose down
```

## Next Steps

- Learn how to submit more complex jobs in the [Usage Guide](../usage/submitting-jobs).
- Ready for production? Follow the [Helm Chart installation guide](./helm-chart).

