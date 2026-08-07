# Hades CI - Ansible role

Deploys Hades onto one or more VMs with Docker Compose. A host is provisioned either as an **API** node or a **scheduler** node (selected via `hades_node_role`); NATS runs alongside the API node for message queuing.

This role is an alternative to the [Helm chart](../../helm/hades/Readme.md) for non-Kubernetes VM deployments. For the application itself, see the top-level [Readme.md](../../Readme.md).

## Requirements

- Docker installed on the target host (the role deploys Hades as containers via Docker Compose).

## Role variables

All variables and their defaults are defined in [`defaults/main.yml`](defaults/main.yml). The most important ones:

| Variable | Default | Description |
| -------- | ------- | ----------- |
| `hades_node_role` | `scheduler` | Which component to install on the host: `api` or `scheduler`. |
| `hades_version` | `latest` | Image tag for both components (overridable per component via `hades_api_version` / `hades_scheduler_version`). |
| `hades_api_port` | `8080` | Port the API listens on. |
| `hades_api_host` | `localhost` | Public hostname of the API node. |
| `hades_api_certificate_fullchain_path` | `""` | Path to the TLS fullchain certificate on the API host (enables HTTPS via nginx). |
| `hades_api_certificate_key_path` | `""` | Path to the TLS private key on the API host. |
| `hades_nats_url` | `nats://localhost:4222` | NATS URL both components connect to. |
| `hades_nats_username` / `hades_nats_password` | `""` | NATS credentials (optional). |
| `hades_nats_tls_enabled` | `false` | Enable TLS for the NATS connection. |
| `hades_scheduler_concurrency` | `1` | Number of jobs the scheduler runs concurrently. |
| `hades_executor` | `docker` | Scheduler executor (`docker` or `k8s`). |
| `hades_debug` | `false` | Verbose logging for both components (`hades_api_debug` / `hades_scheduler_debug` inherit this). |

The corresponding runtime environment variables are documented in [docs/configuration.md](../../docs/configuration.md).

## Example playbook

```yaml
- name: Set up the Hades scheduler
  hosts: hades_dev_scheduler
  roles:
    - role: hades
      vars:
        hades_version: "latest"
        hades_node_role: "scheduler"
        hades_nats_url: "nats://nats.hades.example:4222"
        hades_nats_username: "hades_user"
        hades_nats_password: "nats_password"

- name: Set up the Hades API
  hosts: hades_dev_api
  roles:
    - role: hades
      vars:
        hades_version: "latest"
        hades_node_role: "api"
        hades_api_certificate_fullchain_path: "/var/lib/cert/cert.fullchain.pem"
        hades_api_certificate_key_path: "/var/lib/cert/cert.privkey.pem"
        hades_nats_url: "nats://nats.hades.example:4222"
        hades_nats_username: "hades_user"
        hades_nats_password: "nats_password"
```
