# hadesoperator
The **Hades Operator** brings Kubernetes-native automation to the [HadesCI](https://github.com/ls1intum/hades) system — a scalable job execution tool built for programming exercises and CI pipelines.

This Operator helps run, schedule, and manage Hades build jobs inside a Kubernetes cluster, using native APIs and cloud-native patterns. It improves scalability, reliability, and simplifies deployment.


## Description
The Hades Operator is designed to:

- Schedule and execute CI build jobs as Kubernetes custom resources.
- Automatically handle retries and failure recovery using native Kubernetes features.
- Improve scalability during high traffic (e.g. student exams with many submissions).
- Collect build logs using for easier debugging and monitoring.
- Simplify deployment and upgrades with Helm.
- Apply fine-grained access control using Kubernetes ServiceAccounts and RBAC.

## Getting Started

### Prerequisites
- go version v1.24.0+
- docker version 17.05+.
- kubectl version v1.14.0+.
- Access to a Kubernetes v1.16.0+ cluster.

### To Deploy on the cluster
**Deploy the Operator to the cluster:**

> **NOTE**: Make sure you are under the root directory.

```sh
  cat ./helm/hades/values.yaml
```
- Modify the value `hadesScheduler.configMode` to be `operator`
- (Optional) If you want to have the operator to have the Cluster wide access, 
  modify `hadesOperator.clusterWide` top be `true`
```sh
  helm upgrade --install hades ./helm/hades -n hades --create-namespace
```
Alternative, run
```sh
  helm upgrade --install hades ./helm/hades -n hades --create-namespace \
      --set hadesScheduler.configMode=operator \
```
> **NOTE**: These commands will install the project within the hades namespace.

**Create instances of your solution**
You can apply the samples (examples) from the config/sample:

```sh
  kubectl apply -k ./HadesScheduler/HadesOperator/config/samples/build_v1_buildjob.yaml
```

## License

Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

