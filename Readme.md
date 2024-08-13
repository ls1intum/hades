# HadesCI: Scalable Continuous Integration for Programming Exercises

Welcome to HadesCI, a robust Continuous Integration (CI) tool designed with scalability in mind. HadesCI's primary mission is to provide a straightforward, scalable, and adaptable CI solution tailored for executing build jobs in large-scale programming courses.

## Design Goals

HadesCI embodies several core design principles:

- **Simplicity**: Unlike many other CI implementations, HadesCI focuses on delivering just the essentials required to execute build jobs efficiently.

- **Scalability**: HadesCI has scalability at its core, capable of queuing and executing a vast number of build jobs in parallel, making it ideal for large-scale operations.

- **Container-Based**: HadesCI executes build jobs within containers, ensuring a high level of isolation and security between jobs.

- **Kubernetes Native**: As a Kubernetes-native solution, HadesCI leverages the power and flexibility of Kubernetes as its primary execution platform.

- **Extensibility**: HadesCI is designed to be highly extensible, allowing for easy integration with other build execution platforms as needed.

## Architecture

HadesCI is built upon the following key components:

- **API**: Serving as the main entry point, the API handles all incoming requests

- **Queue**: The queue component is responsible for managing the queue of build jobs, ensuring efficient scheduling.

- **Scheduler**: The scheduler orchestrates the execution of build jobs, coordinating with the executor components.

  - **Docker Executor**: Designed for local development, the Docker executor is responsible for running build jobs within Docker containers.

  - **Kubernetes Executor**: Intended for production use, the Kubernetes executor executes build jobs within a Kubernetes cluster.

## Getting Started

### Prerequisites

- You need a kubeconfig file to connect to a Kubernetes cluster.
- [Docker](https://docs.docker.com/get-docker/)
- [Docker Compose](https://docs.docker.com/compose/install/)
- [Kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/)
- [Minikube](https://minikube.sigs.k8s.io/docs/start/)

### Running in Docker Mode

To run HadesCI in Docker mode, follow these steps:

1. Copy the `.env.example` file to `.env`.
   - The default is to use docker as the executor!
   - Changes should not be necessary for a local test.
2. Start the HadesCI services using Docker Compose

```bash
docker-compose -f docker-compose.yml up -d
```

### Running in Kubernetes Mode

> We assume that you have a Kubernetes cluster running and a kubeconfig file to connect to it.

To run HadesCI in Kubernetes mode, follow these steps:

1. Copy the `.env.example` file to `.env`.
   - Change the `HADES_EXECUTOR` variable to `kubernetes`.
2. Adjust the Kubeconfig volume mount in the `docker-compose.k8s.yml` file.
3. Start the HadesCI services using Docker Compose

```bash
docker-compose -f docker-compose.yml -f docker-compose.k8s.yml up -d
```
