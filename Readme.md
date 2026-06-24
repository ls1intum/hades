# Hades: A Scalable Job Scheduler for Container Workloads

Welcome to Hades, a robust job scheduler designed with scalability in mind. Hades' primary mission is to provide a straightforward, scalable, and adaptable solution for executing containerized workloads in various environments, from educational programming courses to research computing clusters.

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## Design Goals

Hades embodies several core design principles:

- **Simplicity**: Hades focuses on delivering just the essentials required to execute containerized jobs efficiently, without unnecessary complexity.

- **Scalability**: Hades has scalability at its core, capable of queuing and executing a vast number of jobs in parallel, making it ideal for large-scale operations.

- **Container-Based**: Hades executes jobs within containers, ensuring a high level of isolation and security between workloads.

- **Kubernetes Native**: As a Kubernetes-native solution, Hades leverages the power and flexibility of Kubernetes as its primary execution platform for production workloads.

- **Extensibility**: Hades is designed to be highly extensible, allowing for easy integration with other execution platforms and workflow systems as needed.

## Architecture

Hades is built upon the following key components:

- **API**: Serving as the main entry point, the API handles all incoming job requests and provides status information.

- **Queue**: Using NATS as a message queue, this component is responsible for managing the queue of jobs, ensuring efficient scheduling and reliable delivery.

- **Scheduler**: The scheduler orchestrates the execution of jobs, coordinating with the executor components to run each job step in the appropriate environment.

  - **Docker Executor**: Designed for local development, the Docker executor is responsible for running jobs within Docker containers on a single host.

  - **Hades Operator (Recommended)**: The modern, production-ready standard for Kubernetes. It implements a Kubernetes-native controller pattern using Custom Resource Definitions (CRDs). This mode offers superior scalability, automatic retries, and fine-grained RBAC integration.

  - **Kubernetes Executor (Deprecated)**: The legacy Kubernetes execution mode.

- **Log Manager** *(local development only)*: Subscribes to job status and log events on NATS, aggregates per-job logs in memory, and exposes them through an HTTP API (`GET /jobs`, `/jobs/:id/logs`, `/jobs/:id/status`, default port `8081`). Run via `make run` for local workflows; not currently part of the Docker compose stack or the production Helm deployment.

## How It Works

Hades processes jobs through a sequence of well-defined steps:

1. **Job Submission**: Jobs are submitted to the API, defining a series of steps to execute.
2. **Queuing**: The job is queued in NATS for asynchronous processing.
3. **Scheduling**: The scheduler picks up the job and schedules it on the appropriate executor.
4. **Execution**: Each step of the job runs in its own container, with steps sharing data through a common volume.
5. **Completion**: Upon completion, results are stored and made available through the API.

## Getting Started

### Prerequisites

- [Docker](https://docs.docker.com/get-docker/) and [Docker Compose](https://docs.docker.com/compose/install/) for local development
- [Kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/) and a Kubernetes cluster for production deployment
- [Minikube](https://minikube.sigs.k8s.io/docs/start/) for local Kubernetes testing (optional)

### Running in Docker Mode

To run Hades in Docker mode for local development:

1. Clone the repository:

   ```fish
   git clone https://github.com/yourusername/Hades.git
   cd Hades
   ```

2. Copy the `.env.example` file to `.env` (the default configuration uses Docker as the executor, so no changes are necessary for local testing):

   ```fish
   cp .env.example .env
   ```

3. Start the Hades services:

   - **All components in the CLI** (NATS still runs in Docker):

     ```fish
     make run
     ```

     This launches `HadesAPI`, `HadesScheduler`, and `HadesLogManager` via `go run` and streams their logs to the terminal. Press `Ctrl-C` to stop them; run `make docker-stop` to also shut NATS down.

   - **Full stack in Docker**:

     ```fish
     make docker-run
     ```

     Use `make docker-logs` to follow the output and `make docker-stop` to tear the stack down.

### Running in Kubernetes Mode

For production deployments, Hades is designed to run natively within a Kubernetes cluster using **Helm**. This is the recommended way to achieve full scalability and reliability.

1. **Prerequisites**:
   - A Kubernetes cluster (v1.25+)
   - [Helm](https://helm.sh/docs/intro/install/) (v3.12+) installed locally.

2. **Deployment**: We provide a comprehensive Helm Chart that packages the API, Scheduler, and NATS broker. The scheduler uses a `ServiceAccount` to manage job lifecycles within the cluster.
   
   ```fish
   # Quick install
   helm repo add nats https://nats-io.github.io/k8s/helm/charts
   helm dependency build ./helm/hades/
   helm upgrade --install hades ./helm/hades -n hades --create-namespace
   ```

3. **Detailed Documentation**: For advanced configuration (Ingress, TLS, resource limits) and step-by-step setup, please refer to the: [Hades Helm Chart Guide](./helm/hades/Readme.md)

## Usage Examples

### Creating a Simple Job

Here's an example of submitting a basic job to Hades:

```json
{
  "name": "Example Job",
  "metadata": {
    "GLOBAL": "test"
  },
  "steps": [
    {
      "id": 1,
      "name": "Hello World",
      "image": "alpine:latest",
      "script": "echo 'Hello, Hades!'"
    }
  ]
}
```

Submit this job using:

```fish
curl -X POST -H "Content-Type: application/json" -d @job.json http://localhost:8080/build
```

### Multi-Step Job Example

For more complex workflows, you can define multi-step jobs where each step runs in a different container:

```json
{
  "name": "Multi-Step Example",
  "steps": [
    {
      "id": 1,
      "name": "Step 1",
      "image": "alpine:latest",
      "script": "echo 'Setting up environment...' > /shared/output.txt"
    },
    {
      "id": 2,
      "name": "Step 2",
      "image": "ubuntu:latest",
      "script": "cat /shared/output.txt && echo 'Processing data...' >> /shared/output.txt"
    },
    {
      "id": 3,
      "name": "Step 3",
      "image": "python:3.9-alpine",
      "script": "cat /shared/output.txt && echo 'Finalizing...' >> /shared/output.txt && cat /shared/output.txt"
    }
  ]
}
```

### Running LLM / Claude Code Jobs

Hades can run any containerized LLM agent as a regular job step. The Claude Code CLI is
a natural fit: pass a prompt via `metadata` (which becomes a container env var) and the
model's answer comes back through the standard log channel.

#### Oneshot prompt

Submit a single-step job that runs `claude -p "$CLAUDE_PROMPT"` and wraps its output in
sentinels so you can extract it cleanly from the logs:

```json
{
  "name": "Claude Code Oneshot",
  "priority": 3,
  "steps": [
    {
      "id": 1,
      "name": "claude",
      "image": "ghcr.io/your-org/claude-code:latest",
      "metadata": {
        "ANTHROPIC_API_KEY": "<your-key>",
        "CLAUDE_PROMPT": "Write a haiku about distributed systems"
      },
      "script": "echo '===HADES_RESULT_START==='; claude -p \"$CLAUDE_PROMPT\" --output-format json --dangerously-skip-permissions; echo '===HADES_RESULT_END==='"
    }
  ]
}
```

After the job succeeds, retrieve the answer:

```fish
# make run mode (HadesLogManager on :8081)
curl http://localhost:8081/jobs/<job_id>/logs

# make docker-run mode (HadesLogManager on :8082)
curl http://localhost:8082/jobs/<job_id>/logs
```

The response is a JSON array of log entries. The model's answer appears as `message`
lines between the `===HADES_RESULT_START===` / `===HADES_RESULT_END===` markers.

Using `--output-format json` keeps the answer on one line, which is important because the
log parser strips a leading RFC3339 timestamp from each line and trims whitespace -
multi-line output survives, but formatting may be normalised.

#### Agentic "implement this feature" flow

Multi-step jobs share a `/shared` Docker volume, so you can chain a clone step with an
implement step:

1. **Step 1 - Clone**: use `hades-clone-container` to clone the target repo into `/shared/repo`.
2. **Step 2 - Implement**: run Claude Code against `/shared/repo`, commit the changes, and
   push the branch back.

```json
{
  "name": "Claude Code Implement Feature",
  "priority": 3,
  "steps": [
    {
      "id": 1,
      "name": "Clone",
      "image": "ghcr.io/ls1intum/hades/hades-clone-container:latest",
      "metadata": {
        "REPOSITORY_DIR": "/shared",
        "HADES_REPO_URL": "https://github.com/your-org/your-repo.git",
        "HADES_REPO_USERNAME": "x-access-token",
        "HADES_REPO_PASSWORD": "<git-pat>",
        "HADES_REPO_PATH": "./repo",
        "HADES_REPO_ORDER": "1"
      }
    },
    {
      "id": 2,
      "name": "Implement",
      "image": "ghcr.io/your-org/claude-code:latest",
      "metadata": {
        "ANTHROPIC_API_KEY": "<your-key>",
        "CLAUDE_PROMPT": "Add a /healthz endpoint that returns HTTP 200 and {\"status\":\"ok\"}.",
        "GIT_TOKEN": "<git-pat>",
        "REPO_URL": "https://github.com/your-org/your-repo.git"
      },
      "script": "set -e; cd /shared/repo; git config user.email hades@local; git config user.name Hades; git checkout -b hades/feature; echo '===HADES_RESULT_START==='; claude -p \"$CLAUDE_PROMPT\" --allowedTools 'Edit,Write,Read,Bash' --output-format json; echo '===HADES_RESULT_END==='; git add -A; git diff --cached --quiet || git commit -m 'feat: implement via Claude Code'; git push \"https://x-access-token:${GIT_TOKEN}@${REPO_URL#https://}\" hades/feature"
    }
  ]
}
```

The "result" is the pushed `hades/feature` branch plus Claude's JSON summary in the logs.

**Key points for Claude Code containers:**

- The image must contain the `claude` CLI and a POSIX shell (`/bin/bash` by default; override
  with `DOCKER_SCRIPT_EXECUTOR` if needed).
- Use `--allowedTools 'Edit,Write,Read,Bash'` for the implement step - headless Claude Code
  cannot edit files or run git commands without explicit tool permission (or
  `--dangerously-skip-permissions` for a fully open sandbox).
- Pass secrets (`ANTHROPIC_API_KEY`, `GIT_TOKEN`) via `metadata` - the API strips metadata
  values from its own logs automatically (`SafePayloadFormat`), but the values are present
  as container env vars, so avoid scripts that print the full environment.
- The `/shared` volume is **purged after the job completes** - any work that should survive
  (e.g. a git push) must happen within the job itself.
- The log manager only captures logs for jobs it observed transition to `Running` while
  it was alive. Logs persist for 24 hours in NATS JetStream and can be re-read directly
  via `hades.logs.<jobID>` even if the log manager was restarted.
- Containers are **not streamed live** - all container output is published after the
  step exits, so you see the full answer at once once the job is `Succeeded`.

Ready-to-run Bruno requests for both flows live in `bruno/api/`:
- `Create Build Job (Claude Code Oneshot).bru`
- `Create Build Job (Claude Code Implement & Push).bru`

## Configuration Options

Hades can be configured through environment variables or a `.env` file:

| Variable | Description | Default |
|----------|-------------|---------|
| `HADES_EXECUTOR` | Execution platform: `docker` or `k8s` | `docker` |
| `CONCURRENCY` | Number of jobs to process concurrently | `1` |
| `API_PORT` | Port for the Hades API | `8080` |

## Development Workflow

A top-level [`Makefile`](./Makefile) wraps the most common development tasks. Run `make help` to see every target.

| Target | Purpose |
| ------ | ------- |
| `make run` | Run `HadesAPI`, `HadesScheduler`, and `HadesLogManager` locally via `go run` (NATS auto-starts in Docker). |
| `make run-api` / `make run-scheduler` / `make run-logmanager` / `make run-operator` | Run a single component locally via `go run`. |
| `make docker-run` / `make docker-stop` / `make docker-logs` | Start, stop, or tail the full docker compose stack. |
| `make docker-run-api` / `make docker-run-scheduler` / `make docker-run-nats` | Start an individual service via docker compose. |
| `make build` | Compile every Go module in the workspace. |
| `make docker-build` | Build all Hades container images. |
| `make test` | Run unit tests across every Go module. |
| `make test-race` | Same as `make test` with the race detector. |
| `make cover` | Generate and open the HadesAPI coverage report. |
| `make test-operator` / `make test-operator-e2e` | Run HadesOperator envtest unit tests, or Kind-based e2e tests. |
| `make fmt` / `make lint` | Format code with `gofmt` or run `go vet`. |
| `make vuln` | Run `govulncheck` (auto-installs it on first use). |
| `make deps-check` / `make deps-update` / `make deps-tidy` | List outdated direct dependencies, bump them, or run `go mod tidy` across all modules. |
| `make helm-deps` | Refresh the Helm chart subchart lock file. |
| `make ci` | Mirror the CI run locally (`lint` + `test`). |

Tests live alongside the code in each module, and CI (`.github/workflows/ci.yml`) currently runs the `shared` and `HadesAPI` suites on every push and pull request.
The HadesOperator e2e target requires [Kind](https://kind.sigs.k8s.io/) to be installed locally.

## Deployment

### Deploy into a VM

For production deployments in a VM:
1. Ensure you have Docker installed in the VM
2. Copy the `.env.example` file to `.env` and update the configuration:

   ```fish
   cp .env.example .env
   ```
3. Change the `LETSENCRYPT_EMAIL` variable to your email address in your `.env` file.
4. Change the `HADES_API_HOST` variable to domain name or your IP address in your `.env` file.
5. Create Traefik configuration files

    ```fish
    touch traefik/acme.json
    chmod 600 traefik/acme.json
    ```
6. Deploy Hades:
   ```fish
   docker compose -f compose.yml -f docker-compose.deploy.yml up -d
   ```

### Ansible Deployment

Hades includes Ansible playbooks for automated deployment.
See the `ansible/hades/README.md` file for more details.

## Dependency Management

Hades uses [Renovate](https://docs.renovatebot.com/) (configured in `renovate.json`) to open automated PRs for dependency updates across Go modules, Helm charts, Docker base images, and GitHub Actions.
Prefer merging Renovate PRs whenever possible so lock files and changelog links stay consistent.

For manual checks (for example before cutting a release), the workspace is wired up through the top-level Makefile:

```fish
make deps-check     # list outdated direct dependencies in every Go module
make deps-update    # bump direct deps in every module and run go mod tidy
make helm-deps      # refresh helm/hades/Chart.lock
make vuln           # run govulncheck across every module
```

After running `make deps-update`, verify the workspace still builds and tests pass:

```fish
make build
make test
```

Major-version upgrades (for example `sigs.k8s.io/controller-runtime` v0.22 -> v0.24, or any `/v2`, `/v3` import path bump) often contain breaking API changes and should be reviewed one module at a time rather than via a blanket `make deps-update`.

Docker base images in the per-component `Dockerfile`s are tracked by Renovate; for a manual bump, look up the latest tag on the relevant registry and edit the `FROM` line.

## High-Level Architecture Diagram

```
┌─────────┐         ┌─────────┐          ┌───────────────┐
│         │ jobs    │         │  jobs    │               │
│  API    │────────▶│  NATS   │─────────▶│  Scheduler    │
│         │         │ Queue   │          │               │
└─────────┘         └────┬────┘          └───────┬───────┘
                         ▲                       │
                  status │ logs                  ▼
                         │            ┌──────────┴──────────┐
                  ┌──────┴──────┐     │                     │
                  │             │     ▼                     ▼
                  │    Log      │  ┌─────────────┐    ┌─────────────────┐
                  │   Manager   │  │   Docker    │    │   Kubernetes    │
                  │  (HTTP API) │  │  Executor   │    │  / Operator     │
                  │             │  └─────────────┘    └─────────────────┘
                  └─────────────┘
```


## Acknowledgments

- Special thanks to all contributors who have helped shape Hades
- Inspired by the need for a lightweight, scalable job execution system in educational environments
- Built with Go, Docker, Kubernetes, and NATS
