# Hades Documentation

Start here. This index routes you to the right material by audience. For a project overview, design goals, and the high-level architecture diagram, see the top-level [Readme.md](../Readme.md).

## For users - submitting and running jobs

- [Getting Started](../Readme.md#getting-started) - run Hades locally with Docker or the CLI.
- [API Reference](api.md) - `POST /build`, the job payload schema, priorities, and the Log Manager endpoints.
- [Usage Examples](../Readme.md#usage-examples) - single- and multi-step job definitions.
- Ready-made requests: the [Bruno collection](../bruno) (`bruno/api`, `bruno/HadesLogManager`).

## For administrators - deploying and operating

- [Helm Chart Guide](../helm/hades/Readme.md) - the recommended Kubernetes deployment (API, scheduler, operator, NATS).
- [Configuration Reference](configuration.md) - every environment variable, per component, with defaults.
- [VM / Docker Compose deployment](../Readme.md#deployment) - Traefik-based VM setup and chart release process.
- [Ansible role](../ansible/hades/README.md) - automated VM provisioning.

## For developers - contributing

- [AGENTS.md](../AGENTS.md) - the architecture reference: module layout, key contracts (NATS subjects, DTOs, the `BuildJob` CRD), conventions, and gotchas.
- [CONTRIBUTING.md](../CONTRIBUTING.md) - dev setup, tests, and the CRD-regeneration rule.
- Component guides: [HadesAPI](../HadesAPI/Readme.md) · [HadesScheduler](../HadesScheduler/Readme.md) · [HadesOperator](../HadesScheduler/HadesOperator/Readme.md) · [HadesLogManager](../HadesLogManager/Readme.md)
- [Architecture diagrams](diagrams) - editable draw.io sources.

## Reference

- [Configuration Reference](configuration.md)
- [API Reference](api.md)
- [Architecture diagrams](diagrams)
