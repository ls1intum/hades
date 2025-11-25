# Hades Operator Makefile & CRD Verification Workflow

This document explains what the `HadesScheduler/HadesOperator/Makefile` is used for and why the **“Verify CRD & Deepcopy up-to-date”** GitHub Actions workflow exists.

## 1. Purpose of the Makefile

The Makefile in `HadesScheduler/HadesOperator` provides a structured set of commands to automate the core development workflow of the Hades Operator. It is responsible for:

- **Generating Kubernetes CRDs** from Go type definitions.
- **Generating DeepCopy code** (`zz_generated.deepcopy.go`) required by Kubernetes API machinery.
- **Downloading required CLI tools** such as `controller-gen`, `kustomize`, `golangci-lint`, and envtest utilities.
- **Managing Docker image builds** for the operator.
- **Installing, deploying, and uninstalling** the operator and its CRDs into a Kubernetes cluster.

### Key Makefile Targets

- `make manifests`  
  Generates CRDs and writes them into `helm/hades/crds`.  
  This ensures the Helm chart always contains the latest CRD definition.

- `make generate`  
  Generates DeepCopy methods and other Kubernetes boilerplate code using `controller-gen`.

- `make build`  
  Builds the operator binary after refreshing CRDs, deepcopy files, formatting, and linting.

- `make docker-build` / `make docker-push`  
  Builds and pushes the operator's Docker image to the registry.

- `make deploy`  
  Deploys the operator and CRDs to the current Kubernetes context manually, another option besides Helm.

---

## 2. Why Generated Files Must Be Updated Manually

The Hades Operator defines the `BuildJob` API in: `HadesScheduler/HadesOperator/api/v1/buildjob_types.go`


Whenever this file changes (e.g., adding fields, modifying JSON tags, changing validations), two other files must be regenerated:

- `HadesScheduler/HadesOperator/api/v1/zz_generated.deepcopy.go`
- `helm/hades/crds/build.hades.tum.de_buildjobs.yaml`

To regenerate them, run:

```sh
make -C HadesScheduler/HadesOperator manifests generate
```

If this command is not run, or the regenerated files are not committed, the code will compile, but the CRD and the deepcopy logic will be out of sync, which could lead to:

- invalid API behavior, 
- missing fields in clusters, 
- incorrect Helm installations, 
- runtime errors in the operator.

---

## 3. Purpose of the “Verify CRD & Deepcopy up-to-date” GitHub Action

We introduce a dedicated GitHub Actions workflow to ensure that:

- The committed CRD YAML matches the Go API definition. 
- The committed DeepCopy file is correctly generated. 
- Contributors do not forget to run code generation before committing.

### What the Workflow Does

1. Hashes the existing CRD and deepcopy files. 
2. Runs the regeneration commands: `make -C HadesScheduler/HadesOperator manifests generate`
3. Hashes the regenerated files. 
4. Compares before/after values: 
   - If the hashes changed → the workflow fails. 
   - This signals that the contributor forgot to regenerate them. 
5. Prints clear error messages instructing the user to regenerate and commit the updated files.

### Why This Pipeline Is Important

- Prevents API/CRD drift between Go code and YAML definitions. 
- Ensures the Helm chart always contains the correct CRD. 
- Prevents broken deployments caused by outdated generated files. 
- Enforces consistent, reproducible code generation across all contributors. 
- Protects downstream consumers who rely on an accurate CRD.