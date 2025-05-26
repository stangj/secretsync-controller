# secretsync-controller
// TODO(user): Add simple overview of use/purpose

## Description
// TODO(user): An in-depth paragraph about your project and overview of use

## Getting Started

### Prerequisites
- go version v1.24.0+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.

### To Deploy on the cluster
**Build and push your image to the location specified by `IMG`:**

```sh
make docker-build docker-push IMG=<some-registry>/secretsync-controller:tag
```

**NOTE:** This image ought to be published in the personal registry you specified.
And it is required to have access to pull the image from the working environment.
Make sure you have the proper permission to the registry if the above commands don’t work.

**Install the CRDs into the cluster:**

```sh
make install
```

**Deploy the Manager to the cluster with the image specified by `IMG`:**

```sh
make deploy IMG=<some-registry>/secretsync-controller:tag
```

> **NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin
privileges or be logged in as admin.

**Create instances of your solution**
You can apply the samples (examples) from the config/sample:

```sh
kubectl apply -k config/samples/
```

>**NOTE**: Ensure that the samples has default values to test it out.

### To Uninstall
**Delete the instances (CRs) from the cluster:**

```sh
kubectl delete -k config/samples/
```

**Delete the APIs(CRDs) from the cluster:**

```sh
make uninstall
```

**UnDeploy the controller from the cluster:**

```sh
make undeploy
```

## Project Distribution

Following the options to release and provide this solution to the users.

### By providing a bundle with all YAML files

1. Build the installer for the image built and published in the registry:

```sh
make build-installer IMG=<some-registry>/secretsync-controller:tag
```

**NOTE:** The makefile target mentioned above generates an 'install.yaml'
file in the dist directory. This file contains all the resources built
with Kustomize, which are necessary to install this project without its
dependencies.

2. Using the installer

Users can just run 'kubectl apply -f <URL for YAML BUNDLE>' to install
the project, i.e.:

```sh
kubectl apply -f https://raw.githubusercontent.com/<org>/secretsync-controller/<tag or branch>/dist/install.yaml
```

### By providing a Helm Chart

1. Build the chart using the optional helm plugin

```sh
kubebuilder edit --plugins=helm/v1-alpha
```

2. See that a chart was generated under 'dist/chart', and users
can obtain this solution from there.

**NOTE:** If you change the project, you need to update the Helm Chart
using the same command above to sync the latest changes. Furthermore,
if you create webhooks, you need to use the above command with
the '--force' flag and manually ensure that any custom configuration
previously added to 'dist/chart/values.yaml' or 'dist/chart/manager/manager.yaml'
is manually re-applied afterwards.

## Contributing
// TODO(user): Add detailed information on how you would like others to contribute to this project

**NOTE:** Run `make help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

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
## 测试部署到集群
```bash
生成本地yaml并部署到k8s集群
# make manifests
# make install
# kubectl get crd | grep stangj
secretsyncs.sync.stangj.com    2025-05-25T08:11:13Z
本地启动controller
# go run cmd/main.go
2025-05-25T17:37:20+08:00	INFO	setup	starting manager
2025-05-25T17:37:20+08:00	INFO	starting server	{"name": "health probe", "addr": "[::]:8081"}
2025-05-25T17:37:20+08:00	INFO	Starting EventSource	{"controller": "secretsync", "controllerGroup": "sync.stangj.com", "controllerKind": "Secretsync", "source": "kind source: *v1.Namespace"}
2025-05-25T17:37:20+08:00	INFO	Starting EventSource	{"controller": "secretsync", "controllerGroup": "sync.stangj.com", "controllerKind": "Secretsync", "source": "kind source: *v1.Secretsync"}
2025-05-25T17:37:20+08:00	INFO	Starting EventSource	{"controller": "secretsync", "controllerGroup": "sync.stangj.com", "controllerKind": "Secretsync", "source": "kind source: *v1.Secret"}
2025-05-25T17:37:20+08:00	INFO	Starting Controller	{"controller": "secretsync", "controllerGroup": "sync.stangj.com", "controllerKind": "Secretsync"}
2025-05-25T17:37:20+08:00	INFO	Starting workers	{"controller": "secretsync", "controllerGroup": "sync.stangj.com", "controllerKind": "Secretsync", "worker count": 1}
....
编写SecretSync进行同步
# kubectl create ns cert-sync
# cat sync-Secret.yaml 
apiVersion: v1
kind: Secret
metadata:
  namespace: cert-sync
  name: tls
stringData:
  tls.crt: "FAKE-CERT1"
  tls.key: "FAKE-KEY2"
type: kubernetes.io/tls
# kubectl apply -f sync-Secret.yaml
# kubectl get secrets -n cert-sync -oyaml
apiVersion: v1
items:
- apiVersion: v1
  data:
    tls.crt: RkFLRS1DRVJUMQ==
    tls.key: RkFLRS1LRVky
  kind: Secret
  metadata:
    annotations:
      kubectl.kubernetes.io/last-applied-configuration: |
        {"apiVersion":"v1","kind":"Secret","metadata":{"annotations":{},"name":"tls","namespace":"cert-sync"},"stringData":{"tls.crt":"FAKE-CERT1","tls.key":"FAKE-KEY2"},"type":"kubernetes.io/tls"}
    creationTimestamp: "2025-05-25T09:40:02Z"
    name: tls
    namespace: cert-sync
    resourceVersion: "24453"
    uid: d0708bf8-16c7-4de2-82d0-44da794f1a9e
  type: kubernetes.io/tls
kind: List
metadata:
  resourceVersion: ""

创建数据同步
创建要同步的名称空间
# kubectl create ns dest1-sync
# kubectl create ns dest2-sync
创建yaml指定同步的名称空间
# vim sync-tls.yaml
apiVersion: sync.stangj.com/v1
kind: Secretsync
metadata:
  name: sync-tls
spec:
  sourceNamespace: cert-sync
  sourceSecretName: tls
  targetNamespaceSelector:
    matchLabels:
      secret-sync: "enabled"
# kubectl apply -f sync-dest1.yaml 
# kubectl get secretsync
NAME         AGE
sync-dest1   33s
给要同步的名称空间打标签 secret-sync: "enabled
# kubectl label namespaces dest1-sync secret-sync=enabled
# kubectl get namespaces dest1-sync --show-labels
NAME         STATUS   AGE    LABELS
dest1-sync   Active   9m1s   kubernetes.io/metadata.name=dest1-sync,secret-sync=enabled
查看打上标签的名称空间有没有同步secret
# kubectl get secrets -n dest1-sync 
NAME   TYPE                DATA   AGE
tls    kubernetes.io/tls   2      2m45s
# kubectl get secrets -n dest1-sync -oyaml 
apiVersion: v1
items:
- apiVersion: v1
  data:
    tls.crt: RkFLRS1DRVJUMQ==
    tls.key: RkFLRS1LRVky
  kind: Secret
  metadata:
    creationTimestamp: "2025-05-25T09:50:52Z"
    name: tls
    namespace: dest1-sync
    resourceVersion: "25461"
    uid: 5abad270-62fa-40a4-afc9-e64003bd8b5b
  type: kubernetes.io/tls
kind: List
metadata:
  resourceVersion: ""
验证修改数据是否可以同步
# cat sync-Secret.yaml
apiVersion: v1
kind: Secret
metadata:
  namespace: cert-sync
  name: tls
stringData:
  tls.crt: "FAKE-CERT2"
  tls.key: "FAKE-KEY3"
type: kubernetes.io/tls

# kubectl apply -f sync-Secret.yaml
# kubectl get secrets -n cert-sync -oyaml 
apiVersion: v1
items:
- apiVersion: v1
  data:
    tls.crt: RkFLRS1DRVJUMg==
    tls.key: RkFLRS1LRVkz
  kind: Secret
  metadata:
    annotations:
      kubectl.kubernetes.io/last-applied-configuration: |
        {"apiVersion":"v1","kind":"Secret","metadata":{"annotations":{},"name":"tls","namespace":"cert-sync"},"stringData":{"tls.crt":"FAKE-CERT2","tls.key":"FAKE-KEY3"},"type":"kubernetes.io/tls"}
    creationTimestamp: "2025-05-25T09:40:02Z"
    name: tls
    namespace: cert-sync
    resourceVersion: "26030"
    uid: d0708bf8-16c7-4de2-82d0-44da794f1a9e
  type: kubernetes.io/tls
kind: List
metadata:
  resourceVersion: ""

验证同步的名称空间是否已经同步
# kubectl get secrets -n dest1-sync -oyaml 
apiVersion: v1
items:
- apiVersion: v1
  data:
    tls.crt: RkFLRS1DRVJUMg==
    tls.key: RkFLRS1LRVkz
  kind: Secret
  metadata:
    creationTimestamp: "2025-05-25T09:50:52Z"
    name: tls
    namespace: dest1-sync
    resourceVersion: "26031"
    uid: 5abad270-62fa-40a4-afc9-e64003bd8b5b
  type: kubernetes.io/tls
kind: List
metadata:
  resourceVersion: ""
到这里通过标签的方法已经实现了

 kubectl delele -f sync-dest1.yaml 就不会在同步了 但是目的名称空间的Secret并没有删除
```
## 新增加 目的名称空间功能
```bash
apiVersion: sync.stangj.com/v1
kind: Secretsync
metadata:
  name: sync-tls
  namespace: cert-sync
spec:
  sourceNamespace: cert-sync
  sourceSecretName: tls
  targetNamespaces:
    - dest2-sync
    - dest1-sync
  targetNamespaceSelector:
    matchLabels:
      secret-sync: "enabled"

查看同步状态
# kubectl get secretsync -n cert-sync sync-dest1 -o jsonpath='{.status}' ;echo 
{"lastSyncTime":"2025-05-26T13:35:07Z","syncedNamespaces":["dest2-sync","dest1-sync"]}
```
