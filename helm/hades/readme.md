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

```bash
# 1. Create target namespace if the namespace does not exist
kubectl create namespace hades

# 2. Install the chart using serviceaccount mode (default)
helm upgrade --install hades ./helm/hades -n hades --create-namespace

# 3. Tail the Scheduler logs to verify connectivity
kubectl -n hades logs deploy/hades-scheduler -f
```

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