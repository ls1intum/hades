# Architecture Diagrams

Editable [draw.io](https://www.drawio.com/) sources for the Hades architecture. Open them at [app.diagrams.net](https://app.diagrams.net/) or with the draw.io desktop app.

| File | Shows |
| ---- | ----- |
| `HadesCI-Components.drawio` | The overall component layout (API, NATS, scheduler, executors). |
| `HadesCI-Sequence-Diagram.drawio` | The end-to-end sequence of a job from submission to execution. |
| `HadesCI-Job-States.drawio` | The job lifecycle state machine (labelled `Pending`, `Active`, `Retry`, `Completed`, `Failed`). |
| `HadesCI-Logging-Components.drawio` | The logging component layout (Gateway, Queue, Scheduler, Docker Executor, Fluent Bit, and the logging services). |
| `HadesCI-Stripped-Logging-Components.drawio` | A simplified view of the logging components. |

> **Note:** these are the original design sources. Some labels predate the current terminology - the runtime status enum is `Queued` / `Running` / `Succeeded` / `Failed` / `Stopped` (see `shared/buildstatus`), and the current build-log flow is operator → NATS → Log Manager → Artemis adapter (see [HadesLogManager/Readme.md](../../HadesLogManager/Readme.md)).

A rendered high-level architecture diagram (ASCII) lives in the top-level [Readme.md](../../Readme.md#high-level-architecture-diagram), and the build-log flow is drawn in [HadesLogManager/Readme.md](../../HadesLogManager/Readme.md).

To export an image (SVG/PNG) for embedding, use draw.io: **File → Export as**, or the CLI:

```bash
drawio --export --format svg --output HadesCI-Components.svg HadesCI-Components.drawio
```
