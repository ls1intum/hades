---
sidebar_position: 1
---

# Helm Chart Reference

This page covers the complete configuration reference for the Hades Helm chart, including all available values, upgrade procedures, and development utilities.

## Chart Overview

The Hades Helm chart bundles three services:

| Component | Description |
|---|---|
| **hades-api** | Processes and validates job requests; publishes build events to NATS |
| **hades-scheduler** | Consumes NATS events and dispatches jobs to the Kubernetes executor |
| **hades-nats** | Embedded [NATS JetStream](https://nats.io) message broker (sub-chart) |

The Scheduler runs in `serviceaccount` mode by default, using in-cluster credentials to authenticate with the Kubernetes API.

:::info
The previously supported `kubeconfig` mode has been removed. Only `serviceaccount` mode is supported.
:::

## Prerequisites

- Kubernetes **v1.25+**
- Helm **v3.12+**
- `kubectl` configured to point to your target cluster

## Installation

### 1. Add the NATS Dependency

```bash
helm repo add nats https://nats-io.github.io/k8s/helm/charts
helm dependency build ./helm/hades/
```

### 2. Review and Edit Values

```bash
cat ./helm/hades/values.yaml
```

At minimum, update `ingress.host` to match your domain.

### 3. Deploy

```bash
helm upgrade --install hades ./helm/hades -n hades --create-namespace
```

To override specific values on the command line:

```bash
helm upgrade --install hades ./helm/hades -n hades --create-namespace \
  --set ingress.host=hades.example.com \
  --set ingress.tls.secretName=my-tls-secret
```

:::tip Release name vs namespace
The first `hades` in the command above is the **Helm release name** — you can change it to anything (e.g. `hades-dev`, `ci-prod`). The `-n hades` flag sets the **Kubernetes namespace**, which is created automatically when `--create-namespace` is provided.
:::

### 4. Verify the Deployment

```bash
kubectl -n hades logs deploy/hades-scheduler -f
```

Expected healthy output:

```
INFO Connected to NATS server url=nats://hades-nats.hades.svc:4222
INFO Started HadesScheduler in Kubernetes mode
INFO Using service account for Kubernetes access
```

## Values Reference

### API (`hadesApi`)

```yaml
hadesApi:
  image:
    repository: ghcr.io/ls1intum/hades/hades-api
    tag: latest
    pullPolicy: Always
  service:
    type: ClusterIP
    port: 8080
    targetPort: 8080
  replicaCount: 1
  resources:
    limits:
      cpu: "500m"
      memory: "512Mi"
    requests:
      cpu: "200m"
      memory: "256Mi"
```

### Scheduler (`hadesScheduler`)

```yaml
hadesScheduler:
  image:
    repository: ghcr.io/ls1intum/hades/hades-scheduler
    tag: latest
    pullPolicy: Always
  replicaCount: 1
  executor: "k8s"           # Use "k8s" for Kubernetes / Operator mode
  configMode: "operator"    # "serviceaccount" is the only supported value
  resources:
    limits:
      cpu: "500m"
      memory: "512Mi"
    requests:
      cpu: "200m"
      memory: "256Mi"
```

### Operator (`hadesOperator`)

```yaml
hadesOperator:
  image:
    repository: ghcr.io/ls1intum/hades/hades-operator
    tag: latest
    pullPolicy: Always
  replicaCount: 1
  clusterWide: false        # Set to true for cross-namespace job scheduling
  leaderElection:
    enabled: true
  targetNamespace: ""       # Leave empty to use the release namespace
  DeleteOnComplete: true    # Remove completed BuildJob resources automatically
  maxParallelism: "100"
  resources:
    limits:
      cpu: "500m"
      memory: "512Mi"
    requests:
      cpu: "200m"
      memory: "256Mi"
```

### RBAC

```yaml
rbac:
  hadesScheduler:
    serviceAccountName: hades-scheduler
    role:
      name: hades-scheduler-role
  hadesOperator:
    serviceAccountName: hades-operator
    role:
      name: hades-operator-role
```

### Ingress

```yaml
ingress:
  enabled: true
  className: "nginx"
  host: hades.example.com
  tls:
    enabled: true
    secretName: hades-tls
  annotations:
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
```

### NATS (sub-chart)

```yaml
nats:
  config:
    jetstream:
      enabled: true
      memoryStore:
        enabled: true
        maxSize: 1Gi
  host: "hades-nats.hades.svc.cluster.local"
  port: 4222
```

## Cluster-Wide Access

By default the Operator is scoped to the Hades namespace. To schedule jobs across multiple namespaces:

```bash
helm upgrade hades ./helm/hades -n hades \
  --set hadesOperator.clusterWide=true
```

This switches the `Role`/`RoleBinding` to a `ClusterRole`/`ClusterRoleBinding`.

## Upgrade

```bash
helm upgrade hades ./helm/hades -n hades
```

## Rollback

```bash
# List revision history
helm history hades -n hades

# Roll back to a previous revision
helm rollback hades <revision> -n hades
```

## Uninstall

```bash
helm uninstall hades -n hades

# Optional: delete the namespace and any remaining resources
kubectl delete namespace hades
```

## Development Utilities

```bash
# Lint the chart for common issues
helm lint ./helm/hades

# Render templates without deploying (dry run)
helm template hades ./helm/hades -n hades
```

## CRD Maintenance

The `BuildJob` CRD is generated from Go source code. After editing `HadesScheduler/HadesOperator/api/v1/buildjob_types.go`, regenerate the manifests:

```bash
make -C HadesScheduler/HadesOperator manifests generate
```

This updates both the deep-copy helper code and the CRD YAML at `helm/hades/crds/build.hades.tum.de_buildjobs.yaml`. A CI check verifies that committed generated files are always in sync with the Go source.

See the [Makefile documentation](https://github.com/ls1intum/hades/blob/main/HadesScheduler/HadesOperator/makefile-explanation.md) for the full list of available `make` targets.

