# VPCSubnet

The `VPCSubnet` custom resource represents an IBM Cloud VPC subnet managed by the ROKS VPC Network Operator. Each `VPCSubnet` maps one-to-one to a VPC subnet in IBM Cloud and tracks its lifecycle, including creation, status synchronization, and deletion. VPCSubnets are typically created automatically by the CUDN Reconciler when a `ClusterUserDefinedNetwork` with OVN LocalNet topology is provisioned, but they can also be created manually.

## API Information

| Field | Value |
|-------|-------|
| Group | `vpc.roks.ibm.com` |
| Version | `v1alpha1` |
| Kind | `VPCSubnet` |
| Short Name | `vsn` |
| Scope | Namespaced |

## Spec

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `vpcID` | `string` | Yes | - | IBM Cloud VPC ID that the subnet belongs to. Must not be empty. |
| `zone` | `string` | Yes | - | VPC availability zone for the subnet, e.g. `us-south-1`, `eu-de-2`. Must not be empty. |
| `ipv4CIDRBlock` | `string` | Yes | - | IPv4 CIDR block for the subnet. Must match the pattern `^\d+\.\d+\.\d+\.\d+/\d+$`. |
| `aclID` | `string` | No | `""` | Network ACL ID to associate with the subnet. If omitted, the VPC default ACL is used. |
| `resourceGroupID` | `string` | No | `""` | IBM Cloud resource group ID for the subnet. If omitted, the VPC's resource group is used. |
| `securityGroupIDs` | `[]string` | No | `[]` | List of security group IDs to apply to VNIs created in this subnet. |
| `vlanID` | `*int64` | No | `nil` | VLAN ID used for OVN LocalNet bridging. Set automatically when the VPCSubnet is created from a CUDN. |
| `clusterID` | `string` | No | `""` | ROKS cluster ID used for tagging the VPC subnet resource in IBM Cloud. |
| `cudnName` | `string` | No | `""` | Name of the associated `ClusterUserDefinedNetwork` that triggered creation of this VPCSubnet. |

## Status

| Field | Type | Description |
|-------|------|-------------|
| `syncStatus` | `string` | Current synchronization state. One of: `Synced`, `Pending`, `Failed`. |
| `subnetID` | `string` | The IBM Cloud VPC subnet ID assigned after creation. |
| `vpcSubnetStatus` | `string` | The status reported by the VPC API for this subnet (e.g. `available`, `pending`, `deleting`). |
| `availableIPv4` | `int64` | Number of available IPv4 addresses remaining in the subnet. |
| `lastSyncTime` | `*metav1.Time` | Timestamp of the last successful reconciliation with the VPC API. |
| `message` | `string` | Human-readable message providing details about the current state or last error. |
| `conditions` | `[]metav1.Condition` | Standard Kubernetes conditions representing the detailed state of the resource. |

## kubectl Output Columns

When you run `kubectl get vpcsubnets` (or `kubectl get vsn`), the following columns are displayed:

| Column | Source | Priority | Description |
|--------|--------|----------|-------------|
| VPC | `spec.vpcID` | 0 | The VPC ID the subnet belongs to. |
| Zone | `spec.zone` | 0 | The availability zone. |
| CIDR | `spec.ipv4CIDRBlock` | 0 | The IPv4 CIDR block. |
| Sync | `status.syncStatus` | 0 | Current sync status. |
| Subnet ID | `status.subnetID` | 1 | The VPC subnet ID (hidden by default, shown with `-o wide`). |
| Age | `metadata.creationTimestamp` | 0 | Time since the resource was created. |

## Example

```yaml
apiVersion: vpc.roks.ibm.com/v1alpha1
kind: VPCSubnet
metadata:
  name: my-app-subnet
  namespace: default
spec:
  vpcID: "r006-a1b2c3d4-5678-90ab-cdef-1234567890ab"
  zone: "us-south-1"
  ipv4CIDRBlock: "10.240.0.0/24"
  aclID: "r006-abcd1234-abcd-1234-abcd-abcd1234abcd"
  resourceGroupID: "abcdef01234567890abcdef012345678"
  securityGroupIDs:
    - "r006-sg-aaaa-1111"
    - "r006-sg-bbbb-2222"
  vlanID: 100
  clusterID: "c1a2b3c4d5e6f7g8h9"
  cudnName: "my-localnet-cudn"
```

## Usage

### Automatic Creation via CUDN Reconciler

The most common way a `VPCSubnet` is created is automatically by the CUDN Reconciler. When a `ClusterUserDefinedNetwork` with LocalNet topology is created and annotated with VPC networking parameters, the operator creates a corresponding `VPCSubnet` CR, which the VPCSubnet Reconciler then provisions in IBM Cloud.

### Manual Creation

You can create a `VPCSubnet` directly to manage a VPC subnet outside of the CUDN workflow. This is useful for pre-provisioning subnets or integrating with existing VPC infrastructure.

### Lifecycle

1. The VPCSubnet Reconciler detects a new `VPCSubnet` CR.
2. It checks for an existing VPC subnet using resource tags (cluster ID + namespace + name) to ensure idempotency.
3. If no matching subnet exists, it creates one via the VPC API.
4. The reconciler periodically syncs the status, updating `subnetID`, `vpcSubnetStatus`, and `availableIPv4`.
5. On deletion, the reconciler removes the VPC subnet from IBM Cloud before allowing the CR to be garbage collected.

### Related Resources

- [VirtualNetworkInterface](virtualnetworkinterface.md) -- VNIs are created within a VPCSubnet and reference it via `spec.subnetRef`.
- [VLANAttachment](vlanattachment.md) -- VLAN attachments reference a VPCSubnet to determine subnet placement.
- [FloatingIP](floatingip.md) -- Floating IPs are bound to VNIs that reside in a VPCSubnet.
