---
sidebar_position: 1
---

# Submitting Jobs

The Hades API exposes a simple REST interface for submitting and monitoring jobs. Every job is a JSON payload that defines a **name** and an ordered list of **steps**.

## API Base URL

When running locally with Docker Compose, the API is available at:

```
http://localhost:8081
```

In production, this will be the domain you configured (e.g., `https://hades.example.com`).

## Job Payload Structure

```json
{
  "name": "string",
  "metadata": {
    "KEY": "value"
  },
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
| `metadata` | ❌ | Key-value pairs injected as environment variables into every step |
| `steps[].id` | ✅ | Numeric step order (must be unique and sequential) |
| `steps[].name` | ✅ | Human-readable step name |
| `steps[].image` | ✅ | Docker image to run this step in |
| `steps[].script` | ✅ | Shell script or command to execute inside the container |

## Hello World Example

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

A successful response returns the job ID:

```json
{
  "id": "7f3a1c2b-..."
}
```

## Multi-Step Job

Steps are executed sequentially. Each step runs in its own container but shares a common `/shared` volume — use it to pass files between steps.

```bash
curl -X POST http://localhost:8081/build \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Multi-Step Example",
    "steps": [
      {
        "id": 1,
        "name": "Setup",
        "image": "alpine:latest",
        "script": "echo Setting up... > /shared/output.txt"
      },
      {
        "id": 2,
        "name": "Process",
        "image": "ubuntu:latest",
        "script": "cat /shared/output.txt && echo Processing... >> /shared/output.txt"
      },
      {
        "id": 3,
        "name": "Finalize",
        "image": "python:3.9-alpine",
        "script": "cat /shared/output.txt && echo Done!"
      }
    ]
  }'
```

## Using Metadata

The `metadata` block allows you to inject global environment variables into every step:

```json
{
  "name": "Job with Metadata",
  "metadata": {
    "GLOBAL": "shared-value",
    "REPO_URL": "https://github.com/example/repo"
  },
  "steps": [
    {
      "id": 1,
      "name": "Print Env",
      "image": "alpine:latest",
      "script": "echo $GLOBAL && echo $REPO_URL"
    }
  ]
}
```

## Checking Job Status

```bash
curl http://localhost:8081/build/{id}/status
```

Returns the current job state, e.g. `pending`, `running`, `success`, or `failed`.

## Retrieving Build Logs

```bash
curl http://localhost:8081/build/{id}/logs
```

Returns a stream of log lines from all steps of the job.

