---
sidebar_position: 2
---

# Kubernetes Executor *(Deprecated)*

:::warning Deprecated
The Kubernetes executor is the **legacy** integration mode and is no longer recommended for new deployments. Please migrate to the **[Hades Operator](./k8s-operator)**, which offers superior scalability, a reconciliation loop, and fine-grained RBAC integration.
:::

## Overview

The Kubernetes executor was the original way to run Hades jobs on a Kubernetes cluster. In this mode the scheduler directly creates a Kubernetes `Job` (using the client-go API) instead of delegating to the Operator via a `BuildJob` custom resource. There is no controller loop.

## Configuration

This is the `serviceaccount` config mode of the Kubernetes executor:

```yaml
# helm/hades/values.yaml
hadesScheduler:
  executor: k8s
  configMode: serviceaccount   # legacy direct scheduling
```

Equivalently, via environment variables: `HADES_EXECUTOR=k8s` with `K8S_CONFIG_MODE=serviceaccount`. The scheduler must have access to a Kubernetes cluster, configured either via an in-cluster `ServiceAccount` (`serviceaccount`) or a `kubeconfig` file (`kubeconfig`, out-of-cluster use only).

## Limitations

- No controller pattern - there is no reconciliation loop to handle failures.
- No CRD integration - jobs are plain Kubernetes `Job` resources with no custom status tracking.
- Limited RBAC granularity compared to the Operator.

## Migration

To migrate to the Hades Operator:

1. Deploy Hades with Helm (see [Helm Chart Installation](../installation/helm-chart)).
2. The Helm chart defaults to `hadesScheduler.configMode: operator`, so no further configuration is required.
3. Remove any override that sets `configMode: serviceaccount`.
