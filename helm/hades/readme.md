# Hades Helm Chart

Deploy the **Hades** build system (API, Scheduler, and NATS broker) into any Kubernetes cluster using Helm.

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

| Component           | Description                                                                      |
|---------------------|----------------------------------------------------------------------------------|
| **hades-api**       | Processes and validates the request and produce the build request as NATS events |
| **hades-scheduler** | Consumes NATS events and spawns per-exercise runner Pods                         |
| **hades-nats**      | Embedded [NATS JetStream](https://nats.io) message broker (sub-chart)            |

The Scheduler operates in **`serviceaccount` mode** by default, using Kubernetes-native in-cluster credentials to authenticate and create jobs.

> ⚠️ The previously supported `kubeconfig` mode is not implemented in this version.

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

3. Install the chart using serviceaccount mode (default)
    ```bash
    helm upgrade --install hades ./helm/hades -n hades --create-namespace
    ```
   or if you prefer to use the `--set` flag to override values directly in the command line, you can do so like this:

    ```bash
    helm upgrade --install hades ./helm/hades -n hades --create-namespace \
      --set ingress.host=hades.example.com \
      --set tls.secretName=my-secrect
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

```
INFO Connected to NATS server url=nats://hades-nats.hades.svc:4222
INFO Started HadesScheduler in Kubernetes mode
INFO Using service account for Kubernetes access
```

---

## Configuration

All user-configurable options live in **`values.yaml`**. The default mode is `serviceaccount`.

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
  configMode: serviceaccount  # ← required value, "serviceaccount" is the only supported mode currently
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