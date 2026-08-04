# Hades Helm Chart

Deploy the **Hades** build system (API, Scheduler, Operator, and NATS broker) into any Kubernetes cluster using Helm.

---

## Contents

* [Overview](#overview)
* [Prerequisites](#prerequisites)
* [Quick Start](#quick-start)
* [Configuration](#configuration)

    * [Values Reference](#values-reference)
* [Upgrade & Rollback](#upgrade--rollback)
* [Uninstall](#uninstall)
* [Development](#development)

---

## Overview

This chart bundles the core services of Hades:

| Component            | Description                                                                                              |
|----------------------|----------------------------------------------------------------------------------------------------------|
| **hades-api**        | Processes and validates the request and produces the build request as NATS events                        |
| **hades-scheduler**  | Consumes NATS events and translates each into a `BuildJob` custom resource                               |
| **hades-operator**   | Watches `BuildJob` CRs and reconciles them into Kubernetes `batchv1.Job`s with one container per step    |
| **hades-nats**       | Embedded [NATS JetStream](https://nats.io) message broker (sub-chart)                                    |

The Scheduler operates in **`operator` mode** by default (`hadesScheduler.configMode: operator` in `values.yaml`). In this mode the scheduler does not create Pods directly; instead it creates `BuildJob` CRs, which the operator reconciles into Kubernetes Jobs.

> The chart also still supports `serviceaccount` mode (legacy direct scheduling) by setting `hadesScheduler.configMode: serviceaccount`. The `kubeconfig` mode is not intended for in-cluster deployment.

---

## Prerequisites

* Kubernetes **v1.25+**
* Helm **v3.12+**

---

## Quick Start

1. Install the NATS sub-chart (if not already installed)
    ```bash
    helm repo add nats https://nats-io.github.io/k8s/helm/charts
    helm dependency build ./helm/hades/
    ```

2. Adjust the values in `values.yaml` as needed. (e.g., the hostname)

      ```bash
      cat ./helm/hades/values.yaml
      ```

3. Install the chart (default: operator mode)
    ```bash
    helm upgrade --install hades ./helm/hades -n hades --create-namespace
    ```
   or if you prefer to use the `--set` flag to override values directly in the command line, you can do so like this:

    ```bash
    helm upgrade --install hades ./helm/hades -n hades --create-namespace \
      --set ingress.host=hades.example.com \
      --set ingress.tls.secretName=my-secret
    ```

> In the above command:
> 
> The first "hades" is the Helm release name, i.e., the name Helm will use to track this deployment. You can change this to any name (e.g., hades-dev, ci-release). 
> 
>The second "hades" after -n is the Kubernetes namespace where the resources will be deployed. This namespace will be created automatically if it does not exist using --create-namespace

4. Tail the Scheduler logs to verify connectivity
    ```bash
    kubectl -n hades logs deploy/hades-scheduler -f
    ```
> You maybe have to wait a few seconds until the NATS broker is set up.

Expected healthy log lines:

```text
INFO Connected to NATS server url=nats://hades-nats.hades.svc:4222
INFO Started HadesScheduler in Kubernetes mode
INFO Using operator mode (dynamic client)
```

---

## Configuration

All user-configurable options live in **`values.yaml`**. The default mode is `operator`.

### Values Reference

```yaml
# NATS broker
nats:
  host: "hades-nats.hades.svc.cluster.local"
  port: 4222

# Scheduler
hadesScheduler:
  replicaCount: 1
  executor: k8s
  resources:
    limits:
      cpu: 500m
      memory: 512Mi
    requests:
      cpu: 100m
      memory: 256Mi
  service:
    targetPort: 8080
  configMode: operator  # one of: operator (default), serviceaccount

# Operator (only deployed when scheduler runs in `operator` mode)
hadesOperator:
  replicaCount: 1
  clusterWide: false        # set true to grant cluster-wide RBAC instead of namespace-scoped
  DeleteOnComplete: true    # delete BuildJob CRs once their batchv1.Job finishes
  maxParallelism: "100"
```

---

## Upgrade

```bash
# Upgrade in place
helm upgrade hades ./helm/hades -n hades
```

---

## Uninstall

```bash
helm uninstall hades -n hades
# Optional: delete namespace and any leftover ConfigMaps or Secrets
kubectl delete namespace hades
```

---

## Development

```bash
# Lint the chart
helm lint ./helm/hades

# Render templates without deploying
helm template hades ./helm/hades -n hades
```