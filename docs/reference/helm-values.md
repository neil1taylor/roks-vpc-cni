# Helm Values Reference

Complete reference for all configurable values in the VPC Network Operator Helm chart (`deploy/helm/roks-vpc-network-operator/values.yaml`).

---

## Operator

| Value | Type | Default | Description |
|-------|------|---------|-------------|
| `replicaCount` | integer | `1` | Number of operator manager pod replicas |
| `image.repository` | string | `icr.io/roks/vpc-network-operator` | Operator container image repository |
| `image.tag` | string | `latest` | Operator container image tag |
| `image.pullPolicy` | string | `IfNotPresent` | Image pull policy (`Always`, `IfNotPresent`, `Never`) |

## VPC Configuration

| Value | Type | Default | Description |
|-------|------|---------|-------------|
| `vpc.region` | string | `us-south` | IBM Cloud VPC region (e.g., `us-south`, `eu-de`, `jp-tok`) |
| `vpc.resourceGroupID` | string | `""` | IBM Cloud resource group ID for created VPC resources |

## Cluster Identity

| Value | Type | Default | Description |
|-------|------|---------|-------------|
| `cluster.id` | string | `""` | ROKS cluster ID, used for tagging all created VPC resources |

## Credentials

| Value | Type | Default | Description |
|-------|------|---------|-------------|
| `credentials.secretName` | string | `roks-vpc-network-operator-credentials` | Name of the Kubernetes Secret containing the IBM Cloud API key |
| `credentials.secretKey` | string | `IBMCLOUD_API_KEY` | Key within the Secret that holds the API key value |

## Webhook

| Value | Type | Default | Description |
|-------|------|---------|-------------|
| `webhook.port` | integer | `9443` | Port for the webhook server |
| `webhook.certDir` | string | `/tmp/k8s-webhook-server/serving-certs` | Directory for webhook TLS certificates |

## Resource Limits (Operator)

| Value | Type | Default | Description |
|-------|------|---------|-------------|
| `resources.requests.cpu` | string | `100m` | CPU request for operator pod |
| `resources.requests.memory` | string | `128Mi` | Memory request for operator pod |
| `resources.limits.cpu` | string | `200m` | CPU limit for operator pod |
| `resources.limits.memory` | string | `256Mi` | Memory limit for operator pod |

## Leader Election

| Value | Type | Default | Description |
|-------|------|---------|-------------|
| `leaderElection.enabled` | boolean | `true` | Enable leader election for HA deployments |

## Orphan GC

| Value | Type | Default | Description |
|-------|------|---------|-------------|
| `gc.interval` | duration | `10m` | Interval between orphan GC scans |
| `gc.gracePeriod` | duration | `15m` | Grace period before deleting orphaned VPC resources |

## BFF Service

| Value | Type | Default | Description |
|-------|------|---------|-------------|
| `bff.enabled` | boolean | `true` | Deploy the BFF service |
| `bff.image.repository` | string | `icr.io/roks/vpc-network-bff` | BFF container image repository |
| `bff.image.tag` | string | `latest` | BFF container image tag |
| `bff.image.pullPolicy` | string | `IfNotPresent` | Image pull policy |
| `bff.replicas` | integer | `2` | Number of BFF pod replicas |
| `bff.port` | integer | `8443` | BFF service listen port |
| `bff.clusterMode` | string | `roks` | Cluster mode: `roks` or `unmanaged` |
| `bff.apiKeySecretName` | string | `ibm-vpc-api-key` | Secret name for BFF's VPC API key |
| `bff.csiMountPath` | string | `/var/run/ibm-vpc-credentials` | Path for CSI-mounted credentials |
| `bff.logLevel` | string | `info` | Log level (`debug`, `info`, `warn`, `error`) |
| `bff.resources.requests.cpu` | string | `100m` | CPU request for BFF pods |
| `bff.resources.requests.memory` | string | `256Mi` | Memory request for BFF pods |
| `bff.resources.limits.cpu` | string | `500m` | CPU limit for BFF pods |
| `bff.resources.limits.memory` | string | `512Mi` | Memory limit for BFF pods |

## Console Plugin

| Value | Type | Default | Description |
|-------|------|---------|-------------|
| `consolePlugin.enabled` | boolean | `true` | Deploy the OpenShift Console plugin |
| `consolePlugin.image.repository` | string | `icr.io/roks/vpc-network-console-plugin` | Console plugin image repository |
| `consolePlugin.image.tag` | string | `latest` | Console plugin image tag |
| `consolePlugin.image.pullPolicy` | string | `IfNotPresent` | Image pull policy |
| `consolePlugin.replicas` | integer | `2` | Number of console plugin pod replicas |
| `consolePlugin.port` | integer | `9443` | Console plugin service port |
| `consolePlugin.resources.requests.cpu` | string | `50m` | CPU request for console plugin pods |
| `consolePlugin.resources.requests.memory` | string | `128Mi` | Memory request for console plugin pods |
| `consolePlugin.resources.limits.cpu` | string | `200m` | CPU limit for console plugin pods |
| `consolePlugin.resources.limits.memory` | string | `256Mi` | Memory limit for console plugin pods |

## Plugin RBAC

| Value | Type | Default | Description |
|-------|------|---------|-------------|
| `pluginRbac.createAdminBinding` | boolean | `true` | Create ClusterRoleBinding for admin access to console plugin |
| `pluginRbac.createDeveloperBinding` | boolean | `false` | Create RoleBindings for developer access |
| `pluginRbac.developerNamespaces` | list | `[]` | Namespaces where developer RoleBindings are created |

---

## Example: Minimal Installation

```yaml
vpc:
  region: "us-south"
  resourceGroupID: "abc123"
cluster:
  id: "my-cluster-id"
```

## Example: Full Customization

```yaml
replicaCount: 1
image:
  repository: icr.io/roks/vpc-network-operator
  tag: v1.0.0
  pullPolicy: Always
vpc:
  region: "eu-de"
  resourceGroupID: "rg-production"
cluster:
  id: "prod-cluster-01"
credentials:
  secretName: my-vpc-credentials
  secretKey: API_KEY
webhook:
  port: 9443
resources:
  requests:
    cpu: 200m
    memory: 256Mi
  limits:
    cpu: 500m
    memory: 512Mi
leaderElection:
  enabled: true
gc:
  interval: 5m
  gracePeriod: 10m
bff:
  enabled: true
  replicas: 3
  clusterMode: "unmanaged"
  logLevel: "debug"
consolePlugin:
  enabled: true
  replicas: 3
pluginRbac:
  createAdminBinding: true
  createDeveloperBinding: true
  developerNamespaces:
    - development
    - staging
```
