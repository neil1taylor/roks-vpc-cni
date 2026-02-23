# Configuration

This document describes all configuration options for the VPC Network Operator, including environment variables, Helm values, secrets, and tuning parameters.

## Environment Variables

The operator reads the following environment variables at startup. When deployed via the Helm chart, these are set automatically from Helm values.

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `IBMCLOUD_API_KEY` | Yes | (none) | IBM Cloud API key with VPC Infrastructure Services Editor and IP Spoofing Operator roles. Injected from the credentials secret. |
| `VPC_REGION` | Yes | `us-south` | IBM Cloud VPC region (e.g., `us-south`, `eu-de`, `jp-tok`). |
| `CLUSTER_ID` | Yes | (none) | The ROKS cluster ID. Used for tagging VPC resources and correlating them to the cluster. |
| `RESOURCE_GROUP_ID` | Yes | (none) | IBM Cloud resource group ID under which VPC resources are created. |
| `CLUSTER_MODE` | No | `unmanaged` | Cluster mode: `roks` or `unmanaged`. Controls which API backend is used for VNI and VLAN attachment operations. |

## Helm Values

The Helm chart is located at `deploy/helm/roks-vpc-network-operator/`. Below is a complete reference of every configurable value.

### Operator

| Helm Path | Default | Description |
|-----------|---------|-------------|
| `replicaCount` | `1` | Number of operator pod replicas. Only one replica holds the leader lease at a time. |
| `image.repository` | `icr.io/roks/vpc-network-operator` | Container image repository for the operator. |
| `image.tag` | `latest` | Container image tag for the operator. |
| `image.pullPolicy` | `IfNotPresent` | Image pull policy (`Always`, `IfNotPresent`, `Never`). |
| `resources.requests.cpu` | `100m` | CPU request for the operator pod. |
| `resources.requests.memory` | `128Mi` | Memory request for the operator pod. |
| `resources.limits.cpu` | `200m` | CPU limit for the operator pod. |
| `resources.limits.memory` | `256Mi` | Memory limit for the operator pod. |

### VPC and Cluster

| Helm Path | Default | Description |
|-----------|---------|-------------|
| `vpc.region` | `us-south` | IBM Cloud VPC region. Maps to the `VPC_REGION` environment variable. |
| `vpc.resourceGroupID` | `""` | IBM Cloud resource group ID. Maps to the `RESOURCE_GROUP_ID` environment variable. Must be set before install. |
| `cluster.id` | `""` | ROKS cluster ID. Maps to the `CLUSTER_ID` environment variable. Must be set before install. |

### Credentials

| Helm Path | Default | Description |
|-----------|---------|-------------|
| `credentials.secretName` | `roks-vpc-network-operator-credentials` | Name of the Kubernetes Secret containing the IBM Cloud API key. |
| `credentials.secretKey` | `IBMCLOUD_API_KEY` | Key within the Secret that holds the API key value. |

### Webhook

| Helm Path | Default | Description |
|-----------|---------|-------------|
| `webhook.port` | `9443` | Port on which the mutating admission webhook listens. |
| `webhook.certDir` | `/tmp/k8s-webhook-server/serving-certs` | Directory where TLS certificates for the webhook are mounted. |

### Leader Election

| Helm Path | Default | Description |
|-----------|---------|-------------|
| `leaderElection.enabled` | `true` | Enable leader election for HA deployments. When running multiple replicas, only one replica actively reconciles at a time. |

### Orphan Garbage Collection

| Helm Path | Default | Description |
|-----------|---------|-------------|
| `gc.interval` | `10m` | How frequently the orphan GC loop runs. |
| `gc.gracePeriod` | `15m` | How long a VPC resource must be orphaned (no matching Kubernetes object) before the GC deletes it. |

### BFF Service

| Helm Path | Default | Description |
|-----------|---------|-------------|
| `bff.enabled` | `true` | Deploy the BFF (Backend-for-Frontend) service. Required for the console plugin. |
| `bff.image.repository` | `icr.io/roks/vpc-network-bff` | Container image repository for the BFF. |
| `bff.image.tag` | `latest` | Container image tag for the BFF. |
| `bff.image.pullPolicy` | `IfNotPresent` | Image pull policy for the BFF. |
| `bff.replicas` | `2` | Number of BFF pod replicas. |
| `bff.port` | `8443` | Port on which the BFF listens. |
| `bff.clusterMode` | `roks` | Cluster mode for the BFF service. |
| `bff.apiKeySecretName` | `ibm-vpc-api-key` | Name of the Secret containing the IBM Cloud API key for the BFF. |
| `bff.csiMountPath` | `/var/run/ibm-vpc-credentials` | Path where the API key secret is mounted via CSI volume. |
| `bff.logLevel` | `info` | Log level for the BFF service (`debug`, `info`, `warn`, `error`). |
| `bff.resources.requests.cpu` | `100m` | CPU request for BFF pods. |
| `bff.resources.requests.memory` | `256Mi` | Memory request for BFF pods. |
| `bff.resources.limits.cpu` | `500m` | CPU limit for BFF pods. |
| `bff.resources.limits.memory` | `512Mi` | Memory limit for BFF pods. |

### Console Plugin

| Helm Path | Default | Description |
|-----------|---------|-------------|
| `consolePlugin.enabled` | `true` | Deploy the OpenShift Console dynamic plugin. Requires the BFF to also be enabled. |
| `consolePlugin.image.repository` | `icr.io/roks/vpc-network-console-plugin` | Container image repository for the console plugin. |
| `consolePlugin.image.tag` | `latest` | Container image tag for the console plugin. |
| `consolePlugin.image.pullPolicy` | `IfNotPresent` | Image pull policy for the console plugin. |
| `consolePlugin.replicas` | `2` | Number of console plugin pod replicas. |
| `consolePlugin.port` | `9443` | Port on which the console plugin serves content. |
| `consolePlugin.resources.requests.cpu` | `50m` | CPU request for console plugin pods. |
| `consolePlugin.resources.requests.memory` | `128Mi` | Memory request for console plugin pods. |
| `consolePlugin.resources.limits.cpu` | `200m` | CPU limit for console plugin pods. |
| `consolePlugin.resources.limits.memory` | `256Mi` | Memory limit for console plugin pods. |

### Plugin RBAC

| Helm Path | Default | Description |
|-----------|---------|-------------|
| `pluginRbac.createAdminBinding` | `true` | Create a ClusterRoleBinding granting admin users access to VPC networking resources through the console plugin. |
| `pluginRbac.createDeveloperBinding` | `false` | Create RoleBindings for developer access scoped to specific namespaces. |
| `pluginRbac.developerNamespaces` | `[]` | List of namespaces where developer RoleBindings are created when `createDeveloperBinding` is `true`. |

## API Key Secret

The operator authenticates to IBM Cloud using an API key stored in a Kubernetes Secret. Create the secret before installing the Helm chart:

```bash
kubectl create namespace roks-vpc-network-operator

kubectl create secret generic roks-vpc-network-operator-credentials \
  --namespace roks-vpc-network-operator \
  --from-literal=IBMCLOUD_API_KEY=<your-api-key>
```

The secret name and key are configurable via the `credentials.secretName` and `credentials.secretKey` Helm values. The API key must belong to a Service ID with the following IAM roles:

- **VPC Infrastructure Services: Editor** -- create, update, and delete VPC subnets, virtual network interfaces, VLAN attachments, and floating IPs.
- **VPC Infrastructure Services: IP Spoofing Operator** -- required to create VNIs with `allow_ip_spoofing` enabled.

See [VPC Prerequisites](vpc-prerequisites.md) for step-by-step instructions on creating the Service ID and API key.

## Cluster Mode

The `CLUSTER_MODE` environment variable (set via `bff.clusterMode` for the BFF) controls how the operator manages VNIs and VLAN attachments:

### Unmanaged Mode (`CLUSTER_MODE=unmanaged`)

- Default mode.
- The operator calls the IBM Cloud VPC API directly to create and manage Virtual Network Interfaces and VLAN Attachments.
- Suitable for self-managed (non-ROKS) OpenShift clusters on IBM Cloud bare metal servers.

### ROKS Mode (`CLUSTER_MODE=roks`)

- The operator delegates VNI and VLAN attachment operations to the ROKS platform API.
- Intended for managed ROKS clusters where the platform provides these operations natively.
- Other resources (VPC subnets, floating IPs) are still managed via the VPC API directly.

Set the mode in your Helm values:

```yaml
bff:
  clusterMode: "roks"    # or "unmanaged"
```

## Orphan GC Configuration

The orphan garbage collector runs as a periodic loop inside the operator. It detects VPC resources (subnets, VNIs, VLAN attachments, floating IPs) that are tagged with the cluster ID but have no corresponding Kubernetes object.

| Setting | Helm Path | Default | Description |
|---------|-----------|---------|-------------|
| Interval | `gc.interval` | `10m` | How often the GC loop executes. Shorter intervals detect orphans faster but increase VPC API calls. |
| Grace Period | `gc.gracePeriod` | `15m` | Minimum age of an orphaned resource before deletion. Prevents deletion of resources that are still being reconciled. |

For clusters with high VM churn, consider reducing the interval to `5m`. For clusters where VPC API rate limits are a concern, increase the interval to `15m` or `20m`.

```yaml
gc:
  interval: 5m
  gracePeriod: 15m
```

## Resource Limits

The default resource requests and limits are sized for typical workloads. Adjust them based on the number of VMs and networks in your cluster.

### Operator

The operator runs reconciliation loops for CUDNs, nodes, VMs, and CRDs. Memory usage scales with the number of watched objects.

| Cluster Size | CPU Request | CPU Limit | Memory Request | Memory Limit |
|-------------|-------------|-----------|----------------|--------------|
| Small (< 50 VMs) | 100m | 200m | 128Mi | 256Mi |
| Medium (50-200 VMs) | 200m | 500m | 256Mi | 512Mi |
| Large (200+ VMs) | 500m | 1000m | 512Mi | 1Gi |

### BFF Service

The BFF aggregates data from the VPC API and Kubernetes API. Memory usage scales with the number of VPC resources queried.

| Cluster Size | CPU Request | CPU Limit | Memory Request | Memory Limit |
|-------------|-------------|-----------|----------------|--------------|
| Small (< 50 VMs) | 100m | 500m | 256Mi | 512Mi |
| Large (200+ VMs) | 250m | 1000m | 512Mi | 1Gi |

### Console Plugin

The console plugin is a static file server and requires minimal resources. The defaults are suitable for most deployments.

---

**See also:**

- [VPC Prerequisites](vpc-prerequisites.md) -- IBM Cloud resources to prepare before installation
- [RBAC and Access Control](rbac.md) -- permissions and role configuration
- [Upgrading](upgrading.md) -- changing configuration during upgrades
