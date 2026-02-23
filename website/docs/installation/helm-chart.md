---
sidebar_position: 2
---

# Kubernetes via Helm (Production)

The recommended way to deploy Hades in production is with **Helm**. The Hades Helm chart bundles the API, Scheduler, and NATS broker and is configured to use the **Hades Operator** executor — the modern, Kubernetes-native execution mode.

## Prerequisites

- Kubernetes **v1.25+**
- [Helm](https://helm.sh/docs/intro/install/) **v3.12+**
- `kubectl` configured to point to your target cluster

## Install

### 1. Add the NATS Sub-chart Dependency

```bash
helm repo add nats https://nats-io.github.io/k8s/helm/charts
helm dependency build ./helm/hades/
```

### 2. Review Default Values

```bash
cat ./helm/hades/values.yaml
```

At minimum, update the `ingress.host` to match your domain.

### 3. Install the Chart

```bash
helm upgrade --install hades ./helm/hades -n hades --create-namespace
```

Or override values inline:

```bash
helm upgrade --install hades ./helm/hades -n hades --create-namespace \
  --set ingress.host=hades.example.com \
  --set tls.secretName=my-tls-secret
```

> The first `hades` is the Helm **release name** (can be anything). The `-n hades` flag specifies the **namespace**.

### 4. Verify Connectivity

Tail the Scheduler logs to confirm it connected to NATS and started correctly:

```bash
kubectl -n hades logs deploy/hades-scheduler -f
```

Expected healthy output:

```
INFO Connected to NATS server url=nats://hades-nats.hades.svc:4222
INFO Started HadesScheduler in Kubernetes mode
INFO Using service account for Kubernetes access
```

## Upgrade

```bash
helm upgrade hades ./helm/hades -n hades
```

## Uninstall

```bash
helm uninstall hades -n hades
# Optionally remove the namespace and any leftover resources
kubectl delete namespace hades
```

## Next Steps

- Learn about the [Hades Operator execution mode](../operation-modes/k8s-operator).
- Expose the API with TLS using [Traefik](../deployment/traefik).

