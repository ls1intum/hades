---
sidebar_position: 3
---

# Hades Operator *(Recommended)*

The **Hades Operator** is the production-grade execution mode for Kubernetes. It implements the [Kubernetes Operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/) using **Custom Resource Definitions (CRDs)**, giving Hades deep, native integration with the Kubernetes control plane.

## Why the Operator?

| Feature | Docker Executor | K8s Executor *(deprecated)* | Hades Operator |
|---|---|---|---|
| Kubernetes native | ❌ | Partial | ✅ |
| CRD-based job tracking | ❌ | ❌ | ✅ |
| Automatic retries | ❌ | ❌ | ✅ |
| Fine-grained RBAC | ❌ | Limited | ✅ |
| Reconciliation loop | ❌ | ❌ | ✅ |
| Helm chart support | ✅ | ❌ | ✅ |

## Architecture

The Operator introduces a `BuildJob` Custom Resource Definition (CRD). When the Scheduler receives a job, it creates a `BuildJob` resource. The Operator's controller loop watches for `BuildJob` resources and:

1. Creates a Kubernetes `Job` for each step.
2. Monitors Pod status and updates the `BuildJob` status accordingly.
3. Automatically retries failed steps based on the configured policy.
4. Cleans up completed resources.

```
NATS ──▶ Scheduler ──▶ BuildJob (CRD) ──▶ Operator Controller ──▶ Pod/Job
                                                │
                                                ▼
                                        Status & Logs
```

## Configuration

The Operator mode is enabled by default when deploying with the Helm chart. The Scheduler must be configured with:

```yaml
# helm/hades/values.yaml
hadesScheduler:
  executor: k8s
  configMode: serviceaccount
```

The `serviceaccount` config mode is the only supported mode. The Scheduler uses the pod's in-cluster `ServiceAccount` to authenticate with the Kubernetes API — no `kubeconfig` file is needed.

## RBAC

The Helm chart automatically creates a `ServiceAccount`, `Role`, and `RoleBinding` that grant the Scheduler the minimum permissions required to manage `BuildJob` resources and monitor Pods within the `hades` namespace.

If you need cluster-wide access (e.g., to schedule jobs in multiple namespaces), set:

```yaml
hadesOperator:
  clusterWide: true
```

This switches from a `Role`/`RoleBinding` to a `ClusterRole`/`ClusterRoleBinding`.

## CRD Maintenance

The `BuildJob` CRD is defined in Go at:

```
HadesScheduler/HadesOperator/api/v1/buildjob_types.go
```

Whenever this file is modified, the generated files must be regenerated:

```bash
make -C HadesScheduler/HadesOperator manifests generate
```

This updates:
- `HadesScheduler/HadesOperator/api/v1/zz_generated.deepcopy.go`
- `helm/hades/crds/build.hades.tum.de_buildjobs.yaml`

A CI workflow verifies that committed generated files match the Go source on every pull request.

## Submitting a Test Job

Apply the sample `BuildJob` manifest directly:

```bash
kubectl apply -f ./HadesScheduler/HadesOperator/config/samples/build_v1_buildjob.yaml
```

Monitor its progress:

```bash
kubectl -n hades get buildjobs
kubectl -n hades describe buildjob <name>
```

