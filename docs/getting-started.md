# Getting started

This document will help you provision a management cluster and a workload cluster.

## Setup a management cluster

### Provision the cluster

You can use any existing Kubernetes cluster as a management cluster. If you don't
have one, you can use one of the following methods to provision a cluster. At the
end of this section, you must have the kubeconfig of your future management cluster.

#### Method 1: Create a Scaleway Kapsule cluster

Follow this documentation to create a Scaleway Kapsule cluster: [Kubernetes - Quickstart](https://www.scaleway.com/en/docs/containers/kubernetes/quickstart/)

Make sure the `KUBECONFIG` environment variable points to the cluster's kubeconfig:

```console
export KUBECONFIG=/path/to/your/kubeconfig
```

#### Method 2: Create a cluster in Docker with kind

1. Follow this documentation to install Docker: [Install Docker Engine](https://docs.docker.com/engine/install/)
2. Follow this documentation to install kind: [Quick Start](https://kind.sigs.k8s.io/docs/user/quick-start/)
3. Create a kind cluster:

   ```console
   $ kind create cluster
   Creating cluster "kind" ...
   ✓ Ensuring node image (kindest/node:v1.27.3) 🖼
   ✓ Preparing nodes 📦
   ✓ Writing configuration 📜
   ✓ Starting control-plane 🕹️
   ✓ Installing CNI 🔌
   ✓ Installing StorageClass 💾
   Set kubectl context to "kind-kind"
   You can now use your cluster with:

   kubectl cluster-info --context kind-kind

   Have a question, bug, or feature request? Let us know! https://kind.sigs.k8s.io/#community 🙂
   ```

4. Get the kubeconfig:

   ```console
   kind get kubeconfig > mgmt.yaml
   export KUBECONFIG=mgmt.yaml
   ```

### Install cluster API and the Scaleway provider

1. Follow these instructions to install the `clusterctl` command-line tool: [Install clusterctl](https://cluster-api.sigs.k8s.io/user/quick-start#install-clusterctl)
2. Add `scaleway` to the provider repositories by following this [documentation](https://cluster-api.sigs.k8s.io/clusterctl/configuration#provider-repositories).
   You should have the following config:

   ```bash
   $ cat $HOME/.config/cluster-api/clusterctl.yaml
   providers:
   - name: "scaleway"
      url: "https://github.com/Tomy2e/cluster-api-provider-scaleway/releases/latest/infrastructure-components.yaml"
      type: "InfrastructureProvider"
   ```

3. Initialize the management cluster:

   ```console
   $ clusterctl init --infrastructure scaleway
   Fetching providers
   Installing cert-manager Version="v1.14.4"
   Waiting for cert-manager to be available...
   Installing Provider="cluster-api" Version="v1.7.0" TargetNamespace="capi-system"
   Installing Provider="bootstrap-kubeadm" Version="v1.7.0" TargetNamespace="capi-kubeadm-bootstrap-system"
   Installing Provider="control-plane-kubeadm" Version="v1.7.0" TargetNamespace="capi-kubeadm-control-plane-system"
   Installing Provider="infrastructure-scaleway" Version="v0.0.1" TargetNamespace="cluster-api-provider-scaleway-system"

   Your management cluster has been initialized successfully!

   You can now create your first workload cluster by running the following:

   clusterctl generate cluster [name] --kubernetes-version [version] | kubectl apply -f -
   ```

### Create a basic worklow cluster

1. Replace the placeholder values and set the following environment variables:

   ```bash
   export CLUSTER_NAME="my-cluster"
   export KUBERNETES_VERSION="1.30.0"
   export SCW_REGION="fr-par"
   export SCW_ACCESS_KEY="CHANGE THIS"
   export SCW_SECRET_KEY="CHANGE THIS"
   export SCW_PROJECT_ID="CHANGE THIS"
   ```

2. Generate the cluster manifests:

   ```bash
   clusterctl generate cluster ${CLUSTER_NAME} > my-cluster.yaml
   ```

3. Review and edit the `my-cluster.yaml` file as needed.
4. Apply the `my-cluster.yaml` file to create the workflow cluster.
5. Wait for the cluster to be ready.

   ```bash
   $ watch clusterctl describe cluster ${CLUSTER_NAME}
   NAME                                                                          READY  SEVERITY  REASON  SINCE  MESSAGE
   Cluster/my-cluster                                                            True                     4m14s
   ├─ClusterInfrastructure - ScalewayCluster/my-cluster
   ├─ControlPlane - KubeadmControlPlane/my-cluster-control-plane                 True                     4m14s
   │ └─Machine/my-cluster-control-plane-4rrb7                                    True                     6m4s
   │   └─MachineInfrastructure - ScalewayMachine/my-cluster-control-plane-4rrb7
   └─Workers
   └─MachineDeployment/my-cluster-md-0                                         True                     73s
      └─Machine/my-cluster-md-0-22sk4-6jwk2                                     True                     3m51s
         └─MachineInfrastructure - ScalewayMachine/my-cluster-md-0-22sk4-6jwk2
   ```

6. Fetch the kubeconfig of the cluster.

   ```bash
   clusterctl get kubeconfig ${CLUSTER_NAME} > kubeconfig.yaml
   export KUBECONFIG=kubeconfig.yaml
   ```

7. List nodes.

   ```bash
   $ kubectl get nodes
   NAME                                  STATUS   ROLES           AGE     VERSION
   caps-my-cluster-control-plane-4rrb7   Ready    control-plane   6m18s   v1.30.0
   caps-my-cluster-md-0-22sk4-6jwk2      Ready    <none>          3m18s   v1.30.0
   ```

### Setup the workflow cluster

The workload cluster is ready to use. You should now:

- Install a CNI plugin
- (Optional) Install the Scaleway CSI driver to manage block volumes and snapshots.
- (Optional) Install the Scaleway CCM to manage LoadBalancers
