# VLANAttachment

The `VLANAttachment` custom resource represents an IBM Cloud VPC VLAN interface attachment on a bare metal server, managed by the ROKS VPC Network Operator. VLAN attachments provide the Layer 2 bridge between the OVN LocalNet overlay on a bare metal node and the VPC subnet, enabling KubeVirt VMs to communicate over VPC networking. They are typically created automatically by the CUDN Reconciler and Node Reconciler.

## API Information

| Field | Value |
|-------|-------|
| Group | `vpc.roks.ibm.com` |
| Version | `v1alpha1` |
| Kind | `VLANAttachment` |
| Short Name | `vla` |
| Scope | Namespaced |

## Spec

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `bmServerID` | `string` | Yes | - | IBM Cloud bare metal server ID to attach the VLAN interface to. Must not be empty. |
| `vlanID` | `int64` | Yes | - | VLAN ID for the attachment. Must be between 1 and 4094 inclusive. This ID corresponds to the VLAN configured on the OVN LocalNet CUDN. |
| `subnetRef` | `string` | Yes | - | Name of the `VPCSubnet` CR that this VLAN attachment is associated with. |
| `subnetID` | `string` | No | `""` | Direct VPC subnet ID. If provided, takes precedence over resolving the subnet from `subnetRef`. |
| `allowToFloat` | `bool` | No | `true` | Allows the VLAN interface to float to other bare metal servers. Must be `true` for live migration support. |
| `nodeName` | `string` | No | `""` | Kubernetes node name of the bare metal server. Used for display and correlation purposes. |

## Status

| Field | Type | Description |
|-------|------|-------------|
| `syncStatus` | `string` | Current synchronization state. One of: `Synced`, `Pending`, `Failed`. |
| `attachmentID` | `string` | The IBM Cloud VPC VLAN attachment ID assigned after creation. |
| `attachmentStatus` | `string` | The status of the VLAN attachment as reported by the VPC API. One of: `attached`, `pending`, `detached`, `failed`. |
| `lastSyncTime` | `*metav1.Time` | Timestamp of the last successful reconciliation with the VPC API. |
| `message` | `string` | Human-readable message providing details about the current state or last error. |
| `conditions` | `[]metav1.Condition` | Standard Kubernetes conditions representing the detailed state of the resource. |

## kubectl Output Columns

When you run `kubectl get vlanattachments` (or `kubectl get vla`), the following columns are displayed:

| Column | Source | Priority | Description |
|--------|--------|----------|-------------|
| BM Server | `spec.bmServerID` | 0 | The bare metal server ID. |
| VLAN | `spec.vlanID` | 0 | The VLAN ID. |
| Node | `spec.nodeName` | 0 | The Kubernetes node name. |
| Status | `status.attachmentStatus` | 0 | The VPC attachment status. |
| Sync | `status.syncStatus` | 0 | Current sync status. |
| Age | `metadata.creationTimestamp` | 0 | Time since the resource was created. |

## Example

```yaml
apiVersion: vpc.roks.ibm.com/v1alpha1
kind: VLANAttachment
metadata:
  name: node1-vlan100
  namespace: default
spec:
  bmServerID: "0717-a1b2c3d4-5678-90ab-cdef-1234567890ab"
  vlanID: 100
  subnetRef: "my-app-subnet"
  allowToFloat: true
  nodeName: "worker-bm-01"
```

## Usage

### Automatic Creation via CUDN and Node Reconcilers

VLAN attachments are created through two automated paths:

1. **CUDN Reconciler**: When a new `ClusterUserDefinedNetwork` with LocalNet topology is created, the CUDN Reconciler creates a `VLANAttachment` for every bare metal node in the cluster, associating each node's bare metal server with the CUDN's VLAN ID and VPC subnet.

2. **Node Reconciler**: When a new bare metal node joins the cluster, the Node Reconciler creates `VLANAttachment` CRs for all existing CUDNs, ensuring the new node has connectivity to all configured VPC subnets.

### Manual Creation

You can create a `VLANAttachment` directly for scenarios such as attaching a specific bare metal server to a VLAN outside of the automated CUDN workflow.

### Dual Cluster Mode

The VLANAttachment Reconciler operates in two modes controlled by the `CLUSTER_MODE` environment variable:

- **`unmanaged`** (default): VLAN attachments are created directly via the IBM Cloud VPC API using the `InterfaceType: "vlan"` parameter.
- **`roks`**: VLAN attachments are created via the ROKS platform API (stub, awaiting API availability).

### Important Constraints

- `allowToFloat` must be `true` to support KubeVirt live migration, where a VM may move between bare metal nodes.
- The `vlanID` must match the VLAN ID configured on the corresponding OVN LocalNet `ClusterUserDefinedNetwork`.
- Each combination of bare metal server and VLAN ID should have exactly one `VLANAttachment`.

### Related Resources

- [VPCSubnet](vpcsubnet.md) -- The VLAN attachment bridges a bare metal server to the VPC subnet referenced by `spec.subnetRef`.
- [VirtualNetworkInterface](virtualnetworkinterface.md) -- VNIs use the L2 path established by VLAN attachments to reach the VPC network.
- [FloatingIP](floatingip.md) -- Floating IPs provide public access to VNIs that are reachable through VLAN attachments.
