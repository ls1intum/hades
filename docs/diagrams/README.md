# Architecture Diagrams

Editable [draw.io](https://www.drawio.com/) sources for the Hades architecture. Open them at [app.diagrams.net](https://app.diagrams.net/) or with the draw.io desktop app.

| File | Shows |
| ---- | ----- |
| `HadesCI-Components.drawio` | The overall component layout (API, NATS, scheduler, executors). |
| `HadesCI-Sequence-Diagram.drawio` | The end-to-end sequence of a job from submission to execution. |
| `HadesCI-Job-States.drawio` | The job status state machine (Queued → Running → Succeeded/Failed/Stopped). |
| `HadesCI-Logging-Components.drawio` | The build-log flow (operator → NATS → Log Manager → Artemis adapter). |
| `HadesCI-Stripped-Logging-Components.drawio` | A simplified view of the logging flow. |

A rendered high-level architecture diagram (ASCII) lives in the top-level [Readme.md](../../Readme.md#high-level-architecture-diagram), and the build-log flow is drawn in [HadesLogManager/Readme.md](../../HadesLogManager/Readme.md).

To export an image (SVG/PNG) for embedding, use draw.io: **File → Export as**, or the CLI:

```bash
drawio --export --format svg --output HadesCI-Components.svg HadesCI-Components.drawio
```
