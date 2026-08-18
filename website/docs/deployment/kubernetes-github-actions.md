---
title: Kubernetes (GitHub Actions)
sidebar_position: 4
---

# Deploying to Kubernetes with GitHub Actions

Hades ships two manual GitHub Actions workflows that install/upgrade the Helm chart on a
Kubernetes cluster. The default setup targets **one cluster with two namespaces**:
`hades` (production) and `hades-test` (test).

| Workflow | Environment | Namespace | Version rule |
|---|---|---|---|
| **Deploy to Kubernetes (prod)** | `k8s-prod` | `hades` | `latest` or a **published GitHub Release** tag (validated) |
| **Deploy to Kubernetes (test)** | `k8s-test` | `hades-test` | **any** image tag (`latest`, a release tag, `pr-N`, a branch/sha tag) |

Both are `workflow_dispatch` only - nothing deploys automatically. They call the reusable
`deploy-k8s.yml`, which does the actual work.

## How a deploy runs

1. **Checkout** the chart source. Prod checks out the **release tag** (when the version is
   not `latest`) so the chart templates and CRDs match the released images; test uses the
   ref the workflow was run from.
2. **Configure kubeconfig** from the environment's `KUBE_CONFIG` secret.
3. **Ensure the namespace** exists.
4. **Create/update the app Secrets** (`hades-auth`, `hades-dashboard`) from the
   environment secrets. The first deploy creates them; later deploys keep them in sync.
5. **Apply the CRDs** with a server-side apply. Helm never upgrades CRDs after the first
   install, and server-side apply avoids the client-side annotation size limit on the
   `BuildJob` CRD.
6. **`helm upgrade --install --atomic`** with the base `values.yaml` plus the
   environment values file, pinning all four component image tags to the chosen version.
   `--atomic` rolls the release back on failure.

The `version` input maps directly to the container image tag (this repo tags images with
the release tag verbatim, with no `v` prefix).

## Selecting the version

- **Test**: type any image tag. Handy tags: `latest` (current `main`), `pr-123` (a PR
  build), or a release tag.
- **Prod**: type `latest` or an existing release tag (for example `1.0.0`). A bogus value
  fails the `validate` job before the cluster is touched. To deploy a specific release,
  also set **"Use workflow from"** to that release's tag so the chart matches the images.

## Per-environment configuration

Non-secret differences live in committed values files, layered on top of `values.yaml`:

- `helm/hades/values-prod.yaml` - prod ingress host and Secret wiring.
- `helm/hades/values-test.yaml` - test ingress host, Secret wiring, and the
  `nats.host` override for the `hades-test` namespace.

Only credentials are stored as GitHub secrets.

## Required secrets

Add these to **each** GitHub environment (`k8s-prod` and `k8s-test`):

| Secret | Purpose |
|---|---|
| `KUBE_CONFIG` | base64-encoded kubeconfig for the target cluster |
| `AUTH_KEY` | protects the API `/build` endpoint (Secret `hades-auth`) |
| `DASHBOARD_USERNAME` | dashboard login user (Secret `hades-dashboard`) |
| `DASHBOARD_PASSWORD_HASH` | bcrypt hash of the dashboard password |
| `DASHBOARD_SESSION_SECRET` | dashboard session signing secret (>=32 chars) |

Encode the kubeconfig with `base64 -w0 < kubeconfig` (or `base64 < kubeconfig | tr -d '\n'`
on macOS) and paste the result as `KUBE_CONFIG`.

## Production approval

The `k8s-prod` environment uses a **required-reviewer** protection rule, so a production
deploy pauses for manual approval before it runs. TLS certificates are issued in-cluster
by cert-manager (the `letsencrypt-prod` cluster-issuer), so no ACME secrets are needed in
the workflow.

## Troubleshooting

When a deploy fails, the reusable workflow prints pods, events, and the logs/description
of any not-ready pod. Because `--atomic` rolls back (and deletes the release on a failed
first install), check that diagnostics step in the run log for the real cause (commonly
`ImagePullBackOff` for a nonexistent tag, or a missing secret).
