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

### Running with Docker Compose

Update the `.env` file with the path to your kubeconfig file

Start hadesci with docker-compose:

```bash
docker-compose -f docker-compose.yml -f docker-compose.k8s.yml up -d
```

## Development

### Prerequisites

To get started with HadesCI development, make sure you have the following prerequisites installed:

- [Docker](https://docs.docker.com/get-docker/)
- [Docker Compose](https://docs.docker.com/compose/install/)
- [Go](https://golang.org/doc/install)
- [Kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/)
- [Minikube](https://minikube.sigs.k8s.io/docs/start/)

### Running Locally

To run HadesCI locally, follow these steps:

1. Start a local Kubernetes cluster using Minikube:

   ```bash
   minikube start
   ```

2. Run HadesCI with Docker Compose:

   ```bash
   docker-compose up
   ```

With these steps, you'll have a local instance of HadesCI up and running, ready for development and testing. Enjoy coding and building scalable CI solutions with HadesCI!
