# HadesLogManager

HadesLogManager collects build-job logs from NATS, aggregates them in memory per
job, and forwards them to each job's own callback URL when the job finishes. It
also serves a small HTTP API for inspecting logs and status.

Where the logs go is configured **per job**: a build request may set a
`callback_url` field (see the main Readme's payload examples). The LogManager
resolves that URL by looking the job up in the `HADES_JOBS` JetStream KV store
(the same bucket HadesAPI writes and the scheduler consumes) at forward time. A
job with no `callback_url` is simply not forwarded.

## Where it sits in the build-log flow

Build logs travel from a running job to the URL the job asked for (typically the
Artemis adapter). The LogManager is the hop that turns the per-job NATS log
stream into a single HTTP payload for that endpoint. Test results travel a
**separate** path and are re-joined inside the adapter.

```
 build job (pod)                         NATS JetStream                     HTTP
 ┌───────────────┐   operator reads    ┌────────────────────┐          ┌──────────────┐         ┌──────────────┐
 │ step-1 clone  │   pod logs via      │ stream:            │  watch   │ HadesLog     │  POST   │ job          │
 │ step-2 execute│──► K8s API, parses ─┤ HADES_JOB_LOGS     ├─────────►│ Manager      │ /logs   │ callback_url │──► Artemis
 │ step-3 result │   & publishes       │ subj: hades.logs.<jobID>      │ (aggregator) │         │ (adapter)    │
 └──────┬────────┘                     └────────────────────┘          └──────────────┘         └──────▲───────┘
        │ step-3 (junit-result-parser) ───────────────────────── POST /adapter/test-results ──────────┘
        │                                                                 (test results)
```

### Step by step

0. **Job starts.** The scheduler (operator mode) creates a `BuildJob` CR; the
   HadesOperator reconciles it into a Kubernetes `Job` with one container per step
   (`step-1` clone, `step-2` execute, `step-3` result) and publishes a `running`
   status to NATS.
1. **Operator captures container logs.** As the pod runs, the operator reads each
   container's stdout/stderr from the Kubernetes API, parses them into a
   `buildlogs.Log{JobID, ContainerID, Logs[]}` (one per container).
2. **Operator publishes logs to NATS JetStream** on subject `hades.logs.<jobID>`
   (stream `HADES_JOB_LOGS`). Status changes go to `hades.jobstatus.<status>`.
3. **LogManager watches, keyed off status.** On `running` it starts a durable
   JetStream consumer on `hades.logs.<jobID>` and streams batches into the
   in-memory aggregator, **grouped per container**. On `succeeded`/`failed` it
   stops watching, marks the job complete, and forwards the logs.
4. **LogManager forwards to the job's callback URL.** It resolves the job's
   `callback_url` from the `HADES_JOBS` KV store and POSTs the aggregated
   `[]buildlogs.Log` there (typically the adapter's `POST /adapter/logs`). Jobs
   without a `callback_url` are not forwarded.
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
- The LogManager is triggered by **job status events**, not a direct call.
- The forwarding destination is **per job** (`callback_url`), resolved from the
  `HADES_JOBS` KV store at forward time. A job without a `callback_url` runs and
  logs fine but its logs are never forwarded.
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
| `DEBUG`                  | `false`  | Enable verbose (debug-level) logging.          |

The forwarding destination is no longer a global env var. Each job sets its own
`callback_url` in the build request; the LogManager reads it from the
`HADES_JOBS` JetStream KV store when forwarding. Jobs without a `callback_url`
are not forwarded.
