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

  - **Kubernetes Executor**: Intended for production use, the Kubernetes executor executes jobs within a Kubernetes cluster, providing improved scalability, reliability, and resource utilization.

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

2. Copy the `.env.example` file to `.env`:

   ```fish
   cp .env.example .env
   ```

   The default configuration uses Docker as the executor, so no changes are necessary for local testing.

3. Start the Hades services:

   ```fish
   docker compose up -d
   ```

### Running in Kubernetes Mode

For production deployments or testing with Kubernetes:

1. Ensure you have a running Kubernetes cluster and a valid kubeconfig file.

2. Copy the `.env.example` file to `.env` and update the configuration:

   ```fish
   cp .env.example .env
   ```

3. Change the `HADES_EXECUTOR` variable to `kubernetes` in your `.env` file.

4. Adjust the Kubeconfig volume mount in `docker-compose.k8s.yml` to point to your kubeconfig file.

5. Start Hades in Kubernetes mode:

   ```fish
   docker compose -f compose.yml -f docker-compose.k8s.yml up -d
   ```

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

## Configuration Options

Hades can be configured through environment variables or a `.env` file:

| Variable | Description | Default |
|----------|-------------|---------|
| `HADES_EXECUTOR` | Execution platform: `docker` or `kubernetes` | `docker` |
| `CONCURRENCY` | Number of jobs to process concurrently | `1` |
| `HADESAPI_API_PORT` | Port for the Hades API | `8080` |

## Deployment

### Ansible Deployment

Hades includes Ansible playbooks for automated deployment.
See the `ansible/hades/README.md` file for more details.

## High Level Architecture Diagram

```
┌─────────┐         ┌─────────┐          ┌───────────────┐
│         │         │         │          │               │
│  API    │────────▶│  NATS   │─────────▶│  Scheduler    │
│         │         │ Queue   │          │               │
└─────────┘         └─────────┘          └───────┬───────┘
                                                 │
                                                 ▼
                        ┌────────────────────────┴───────────────────────┐
                        │                                                │
                        ▼                                                ▼
                 ┌─────────────┐                               ┌─────────────────┐
                 │             │                               │                 │
                 │   Docker    │                               │   Kubernetes    │
                 │  Executor   │                               │    Executor     │
                 │             │                               │                 │
                 └─────────────┘                               └─────────────────┘
```


## Acknowledgments

- Special thanks to all contributors who have helped shape Hades
- Inspired by the need for a lightweight, scalable job execution system in educational environments
- Built with Go, Docker, Kubernetes, and NATS
