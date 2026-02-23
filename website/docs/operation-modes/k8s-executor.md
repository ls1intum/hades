---
sidebar_position: 2
---

# Kubernetes Executor *(Deprecated)*

:::warning Deprecated
The Kubernetes executor is the **legacy** integration mode and is no longer recommended for new deployments. It will be removed in a future release.

Please migrate to the **[Hades Operator](./k8s-operator)**, which offers superior scalability, automatic retries, and fine-grained RBAC integration.
:::

## Overview

The Kubernetes executor was the original way to run Hades jobs on a Kubernetes cluster. It directly creates Kubernetes `Job` and `Pod` resources using the `kubectl` API, without a controller loop.

## Configuration

Set the executor mode in your environment:

```env
HADES_EXECUTOR=kubernetes
```

The Scheduler must have access to a Kubernetes cluster, configured either via an in-cluster `ServiceAccount` or a `kubeconfig` file.

## Limitations

- No controller pattern — there is no reconciliation loop to handle failures.
- No CRD integration — jobs are plain Kubernetes `Job` resources with no custom status tracking.
- Limited RBAC granularity compared to the Operator.
- Not supported by the current Helm chart.

## Migration

To migrate to the Hades Operator:

1. Deploy Hades with Helm (see [Helm Chart Installation](../installation/helm-chart)).
2. The Helm chart defaults to `serviceaccount` mode with the Operator, so no further configuration is required.
3. Remove any references to `HADES_EXECUTOR=kubernetes` from your environment.

