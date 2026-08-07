---
sidebar_position: 1
---

# Submitting Jobs

The Hades API exposes a small REST interface for submitting jobs. Every job is a JSON payload that defines a **name** and an ordered list of **steps**. For the full machine-readable contract, see the [API reference](../reference/api).

## API Base URL

When running locally with Docker Compose, the API is available at:

```
http://localhost:8081
```

(The API listens on `8080` inside the container and is published on `8081` by `compose.yml`. With `make run` it is on `8080`.) In production this is the domain you configured (e.g. `https://hades.example.com`).

## Authentication

If the API is started with a non-empty `AUTH_KEY`, `POST /build` requires HTTP Basic Auth with username `hades` and the key as the password. If `AUTH_KEY` is empty, the endpoint is open.

## Job Payload Structure

```json
{
  "name": "string",
  "priority": 3,
  "metadata": { "KEY": "value" },
  "steps": [
    {
      "id": 1,
      "name": "string",
      "image": "docker-image:tag",
      "script": "shell command or script"
    }
  ]
}
```

| Field | Required | Description |
|---|---|---|
| `name` | ✅ | Human-readable job name |
| `priority` | ❌ | `1` = low, `2` = medium, `3+` = high (default `3`); selects the NATS queue |
| `metadata` | ❌ | Key-value pairs injected as environment variables into every step |
| `steps[].id` | ✅ | Numeric step order (starts at 1) |
| `steps[].name` | ✅ | Human-readable step name |
| `steps[].image` | ✅ | Container image to run this step in |
| `steps[].script` | ❌ | Shell script to execute inside the container |
| `steps[].continue_on_error` | ❌ | Continue with the next step if this one fails |
| `steps[].cpu_limit` | ❌ | CPU limit in millicores (e.g. `1000` = 1 core) |
| `steps[].memory_limit` | ❌ | Memory limit, e.g. `512M`, `2G` |

## Hello World Example

```bash
curl -X POST http://localhost:8081/build \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Hello World",
    "steps": [
      { "id": 1, "name": "Say Hello", "image": "alpine:latest", "script": "echo Hello from Hades!" }
    ]
  }'
```

A successful response returns the assigned job ID:

```json
{ "message": "Successfully enqueued job", "job_id": "7f3a1c2b-..." }
```

## Multi-Step Job

Steps run sequentially. Each step runs in its own container but shares a common `/shared` volume - use it to pass files between steps.

```bash
curl -X POST http://localhost:8081/build \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Multi-Step Example",
    "steps": [
      { "id": 1, "name": "Setup",    "image": "alpine:latest",     "script": "echo Setting up... > /shared/output.txt" },
      { "id": 2, "name": "Process",  "image": "ubuntu:latest",     "script": "cat /shared/output.txt && echo Processing... >> /shared/output.txt" },
      { "id": 3, "name": "Finalize", "image": "python:3.9-alpine", "script": "cat /shared/output.txt && echo Done!" }
    ]
  }'
```

## Monitoring Jobs (Log Manager)

Status and logs are served by the **Log Manager**, a separate service (default port `8081` when run via `make run`), **not** by the API. Its endpoints:

| Method & path | Description |
|---|---|
| `GET /jobs` | List all known job IDs |
| `GET /jobs/{id}/status` | Current build status (`Queued`, `Running`, `Succeeded`, `Failed`, `Stopped`) |
| `GET /jobs/{id}/logs` | Aggregated log entries for the job |
| `GET /health` | Liveness probe |

```bash
curl http://localhost:8081/jobs/<id>/status
curl http://localhost:8081/jobs/<id>/logs
```

:::note
The Log Manager is a local-development aid and is not part of the production Helm deployment. See the [API Reference](../reference/api) for the interactive Log Manager OpenAPI spec.
:::
