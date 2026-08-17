# HadesLogManager

HadesLogManager collects build-job logs from NATS, aggregates them in memory per
job, and forwards them to the Artemis adapter when a job finishes. It also serves
a small HTTP API for inspecting logs and status.

## Where it sits in the build-log flow

Build logs travel from a running job all the way to Artemis. The LogManager is
the hop that turns the per-job NATS log stream into a single HTTP payload for the
adapter. Test results travel a **separate** path and are re-joined inside the
adapter.

```
 build job (pod)                         NATS JetStream                     HTTP
 ┌───────────────┐   operator reads    ┌────────────────────┐          ┌──────────────┐         ┌─────────┐
 │ step-1 clone  │   pod logs via      │ stream:            │  watch   │ HadesLog     │  POST   │ Artemis │
 │ step-2 execute│──► K8s API, parses ─┤ HADES_JOB_LOGS     ├─────────►│ Manager      │ /logs   │ Adapter │──► Artemis
 │ step-3 result │   & publishes       │ subj: hades.logs.<jobID>      │ (aggregator) │         │         │
 └──────┬────────┘                     └────────────────────┘          └──────────────┘         └────▲────┘
        │ step-3 (junit-result-parser) ───────────────────────── POST /adapter/test-results ────────┘
        │                                                                 (test results)
```

### Step by step

0. **Job starts.** The scheduler (operator mode) creates a `BuildJob` CR; the
   HadesOperator reconciles it into a Kubernetes `Job` with one container per step
   (`step-1` clone, `step-2` execute, `step-3` result) and publishes a `running`
   status to NATS.
1. **Operator streams container logs live.** As each container runs, the operator
   **follows** its stdout/stderr from the Kubernetes API (`Follow: true`) and parses
   lines into `buildlogs.Log{JobID, ContainerID, Logs[]}` incrementally, rather than
   reading the whole log once after the container stops. (The Docker executor does the
   same via `ContainerLogs` with `Follow: true`.)
2. **Operator publishes logs to NATS JetStream** on subject `hades.logs.<jobID>`
   (stream `HADES_JOB_LOGS`), emitting many small per-container batches over the
   container's lifetime (flushed on ~50 lines or ~1s). A zero-entry `Log` is published
   when a container starts so a step that produces no output keeps its slot. Status
   changes go to `hades.jobstatus.<status>`.
3. **LogManager watches, keyed off status.** On `running` it starts a durable
   JetStream consumer on `hades.logs.<jobID>` and streams batches into the in-memory
   aggregator, **coalesced per container** (all batches for one `ContainerID` merge
   into a single `Log`, preserving first-seen container order). On `succeeded`/`failed`
   it stops watching, marks the job complete, and forwards the logs.
4. **LogManager forwards to the adapter.** It POSTs the aggregated `[]buildlogs.Log`
   to `ARTEMIS_ADAPTER_URL` (`POST /adapter/logs`).
5. **Adapter stores execution logs.** It takes the execute step (`logs[1]`) as the
   execution logs. Per-container grouping matters here: if all steps were flattened
   into one `Log`, `logs[1]` would not exist and the adapter would report
   "Execution logs missing".
6. **Test results arrive separately.** The job's `junit-result-parser` step POSTs
   a `ResultDTO` to `POST /adapter/test-results`.
7. **Adapter joins and sends to Artemis.** Only once both logs and results exist for
   a job does the adapter merge them and POST the result to Artemis.

### Key points

- Logs are **pulled** by the operator (scraped from the K8s log API), not pushed by
  the build container.
- Logs stream **live**: they are published incrementally as a container produces them,
  not buffered until the step completes. The HadesAPI dashboard tails them over SSE
  (`GET /api/jobs/:id/logs/stream`), backed by its own ephemeral JetStream consumer on
  `hades.logs.<jobID>` (independent of this service).
- The LogManager is triggered by **job status events**, not a direct call.
- Artemis only receives a result when **both** the build logs and the test results
  land, so a job missing the `result` step will run and log fine but never post to
  Artemis.

## HTTP API

| Method | Path                     | Description                                  |
| ------ | ------------------------ | -------------------------------------------- |
| GET    | `/jobs`                  | List known job IDs (active and completed).   |
| GET    | `/jobs/:jobId/logs`      | Aggregated log entries for a job.            |
| GET    | `/jobs/:jobId/status`    | Current build status for a job, or 404.      |
| GET    | `/health`                | Liveness probe.                              |

## Configuration

| Env var                  | Default  | Description                                    |
| ------------------------ | -------- | ---------------------------------------------- |
| `NATS_URL`               | `nats://localhost:4222` | NATS server URL.                |
| `NATS_TLS_ENABLED`       | `false`  | Enable TLS for the NATS connection.            |
| `HADESLOGMANAGER_API_PORT` | `8081` | HTTP API port.                                 |
| `LOG_BATCH_SIZE`         | `100`    | Log entries buffered before a flush.           |
| `LOG_RETENTION`          | `1h`     | How long completed job logs are kept in memory.|
| `MAX_JOB_LOGS`           | `1000`   | Max log entries retained per job.              |
| `ARTEMIS_ADAPTER_URL`    | (unset)  | Adapter endpoint for forwarding logs. If unset, forwarding is skipped. |
| `DEBUG`                  | `false`  | Enable verbose (debug-level) logging.          |
