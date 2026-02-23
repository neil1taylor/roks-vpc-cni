# Installation

The VPC Network Operator is deployed using a Helm chart that installs all three components: the operator, the BFF service, and the OpenShift Console plugin.

---

## Step 1: Create the Namespace

```bash
oc create namespace roks-vpc-network-operator
```

## Step 2: Create the API Key Secret

Store the IBM Cloud API key as a Kubernetes Secret. This key is used by the operator and BFF service to authenticate with the VPC API.

```bash
oc create secret generic roks-vpc-network-operator-credentials \
  --namespace roks-vpc-network-operator \
  --from-literal=IBMCLOUD_API_KEY=<your-api-key>
```

## Step 3: Install the Helm Chart

```bash
helm upgrade --install vpc-network-operator \
  deploy/helm/roks-vpc-network-operator \
  --namespace roks-vpc-network-operator \
  --set vpc.region=us-south \
  --set vpc.resourceGroupID=<your-resource-group-id> \
  --set cluster.id=<your-cluster-id>
```

### Required Values

| Value | Description | Example |
|-------|-------------|---------|
| `vpc.region` | IBM Cloud VPC region | `us-south` |
| `vpc.resourceGroupID` | Resource group ID for VPC resources | `abc123def456` |
| `cluster.id` | ROKS cluster ID (used for tagging VPC resources) | `c1a2b3c4d5` |

### Optional Values

| Value | Default | Description |
|-------|---------|-------------|
| `credentials.secretName` | `roks-vpc-network-operator-credentials` | Name of the API key Secret |
| `credentials.secretKey` | `IBMCLOUD_API_KEY` | Key within the Secret |
| `replicaCount` | `1` | Operator pod replicas |
| `bff.enabled` | `true` | Deploy the BFF service |
| `bff.replicas` | `2` | BFF service replicas |
| `bff.clusterMode` | `roks` | `roks` or `unmanaged` |
| `consolePlugin.enabled` | `true` | Deploy the console plugin |
| `consolePlugin.replicas` | `2` | Console plugin replicas |
| `gc.interval` | `10m` | Orphan GC scan interval |
| `gc.gracePeriod` | `15m` | Grace period before orphan deletion |
| `leaderElection.enabled` | `true` | Enable leader election |

See [Helm Values Reference](../reference/helm-values.md) for the complete list of configurable values.

## Step 4: Verify the Installation

Check that all pods are running:

```bash
oc get pods -n roks-vpc-network-operator
```

Expected output:

```
NAME                                          READY   STATUS    RESTARTS   AGE
vpc-network-operator-manager-abc123-xyz       1/1     Running   0          60s
vpc-network-bff-def456-uvw                    1/1     Running   0          60s
vpc-network-bff-def456-rst                    1/1     Running   0          60s
vpc-network-console-plugin-ghi789-opq         1/1     Running   0          60s
vpc-network-console-plugin-ghi789-lmn         1/1     Running   0          60s
```

Verify the CRDs are installed:

```bash
oc get crd | grep vpc.roks.ibm.com
```

Expected output:

```
floatingips.vpc.roks.ibm.com              2026-02-23T00:00:00Z
virtualnetworkinterfaces.vpc.roks.ibm.com 2026-02-23T00:00:00Z
vlanattachments.vpc.roks.ibm.com          2026-02-23T00:00:00Z
vpcsubnets.vpc.roks.ibm.com               2026-02-23T00:00:00Z
```

Verify the webhook is registered:

```bash
oc get mutatingwebhookconfigurations | grep roks-vpc
```

Check the operator logs:

```bash
oc logs -n roks-vpc-network-operator deployment/vpc-network-operator-manager
```

## Step 5: Enable the Console Plugin

If the console plugin was deployed (`consolePlugin.enabled: true`), enable it in the OpenShift Console:

```bash
oc patch consoles.operator.openshift.io cluster \
  --type=merge \
  --patch '{"spec":{"plugins":["vpc-network-console-plugin"]}}'
```

After a few moments, the VPC Networking pages will appear under the **Networking** section in the OpenShift Console.

---

## Unmanaged Cluster Mode

For self-managed OpenShift clusters (not ROKS), set the cluster mode to `unmanaged`:

```bash
helm upgrade --install vpc-network-operator \
  deploy/helm/roks-vpc-network-operator \
  --namespace roks-vpc-network-operator \
  --set vpc.region=us-south \
  --set vpc.resourceGroupID=<your-resource-group-id> \
  --set cluster.id=<your-cluster-id> \
  --set bff.clusterMode=unmanaged
```

In unmanaged mode, the operator manages VNIs and VLAN attachments directly via the VPC API. In ROKS mode, the ROKS platform manages these resources and the operator syncs status only.

See [Dual Cluster Mode](../architecture/dual-cluster-mode.md) for details on the differences.

---

## Upgrading

See [Upgrading](../admin-guide/upgrading.md) for upgrade procedures.

## Uninstalling

See [Uninstalling](../admin-guide/uninstalling.md) for removal and cleanup instructions.

## Next Steps

- [Quick Start](quick-start.md) — Create your first CUDN and deploy a VM
- [Configuration](../admin-guide/configuration.md) — Full configuration reference
- [VPC Prerequisites](../admin-guide/vpc-prerequisites.md) — Set up VPC resources before creating networks
