# HadesAPI

HadesAPI is the entry point of Hades: a small [Gin](https://gin-gonic.com/) HTTP server that accepts job submissions, validates them, and publishes them to NATS JetStream for the [scheduler](../HadesScheduler/Readme.md) to execute.

## Responsibilities

- Validate an incoming job payload (`name` is required; per-step `memory_limit` must be parseable).
- Assign the job a UUID.
- Publish it to the NATS queue matching its priority (`hades.jobs.{high,medium,low}`).

It does **not** run jobs itself and holds no state.

## Endpoints

| Method | Path | Auth | Description |
| ------ | ---- | ---- | ----------- |
| `GET` | `/ping` | none | Liveness probe (`{"status":"ok","timestamp":...}`). |
| `POST` | `/build` | Basic (when `AUTH_KEY` set) | Validate and enqueue a job. Returns the assigned `job_id`. |

The full request/response contract and the job payload schema are in [docs/api.md](../docs/api.md) and the [published API reference](https://ls1intum.github.io/hades/docs/reference/api).

### Example

```bash
# When AUTH_KEY is set, add:  -u "hades:$AUTH_KEY"
curl -X POST -H "Content-Type: application/json" \
  -d '{"name":"Example","steps":[{"id":1,"name":"hi","image":"alpine:latest","script":"echo hello"}]}' \
  http://localhost:8080/build
```

## Authentication

If `AUTH_KEY` is set, `/build` requires HTTP Basic Auth with username `hades` and the key as the password. If it is empty, the endpoint is open and a warning is logged at startup.

## Priority

`POST /build` accepts an optional `priority` field: `1` = low, `2` = medium, `3+` = high (default `3`). The priority selects the NATS queue and is propagated to executors via job metadata (see `MetadataKeyPriority` in `../shared/prio.go`).

## Configuration

Common variables: `API_PORT` (default `8080`), `AUTH_KEY`, `NATS_URL`. See [docs/configuration.md](../docs/configuration.md) for the complete list.

## Run locally

```bash
make run-api      # from the repository root (auto-starts NATS in Docker)
```
