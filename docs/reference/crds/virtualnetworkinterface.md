# VirtualNetworkInterface

The `VirtualNetworkInterface` custom resource represents an IBM Cloud VPC Virtual Network Interface (VNI) managed by the ROKS VPC Network Operator. A VNI provides a network identity (MAC address, primary IP) for a KubeVirt virtual machine on a VPC subnet. VNIs are typically created automatically by the VM Webhook during VM admission, but can also be created manually for advanced use cases.

## API Information

| Field | Value |
|-------|-------|
| Group | `vpc.roks.ibm.com` |
| Version | `v1alpha1` |
| Kind | `VirtualNetworkInterface` |
| Short Name | `vni` |
| Scope | Namespaced |

## Spec

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `subnetRef` | `string` | Yes | - | Name of the `VPCSubnet` CR in the same namespace that this VNI should be created in. Must not be empty. |
| `subnetID` | `string` | No | `""` | Direct VPC subnet ID. If provided, takes precedence over resolving the subnet from `subnetRef`. Useful for referencing subnets not managed by the operator. |
| `securityGroupIDs` | `[]string` | No | `[]` | List of VPC security group IDs to attach to this VNI. If omitted, the security groups from the referenced VPCSubnet are used. |
| `allowIPSpoofing` | `bool` | No | `true` | Enables IP spoofing on the VNI. Required to be `true` for OVN LocalNet networking to function correctly. |
| `enableInfrastructureNat` | `bool` | No | `false` | Controls whether infrastructure NAT is enabled. Must be `false` for bare metal VNI operation. |
| `autoDelete` | `bool` | No | `false` | Controls whether the VPC VNI is automatically deleted when its target is removed. Must be `false` so the operator manages the lifecycle. |
| `vmRef` | `*VMReference` | No | `nil` | Reference to the KubeVirt `VirtualMachine` that owns this VNI. Contains `namespace` and `name` fields. |
| `clusterID` | `string` | No | `""` | ROKS cluster ID used for tagging the VNI resource in IBM Cloud. |

### VMReference

| Field | Type | Description |
|-------|------|-------------|
| `namespace` | `string` | Namespace of the referenced VirtualMachine. |
| `name` | `string` | Name of the referenced VirtualMachine. |

## Status

| Field | Type | Description |
|-------|------|-------------|
| `syncStatus` | `string` | Current synchronization state. One of: `Synced`, `Pending`, `Failed`. |
| `vniID` | `string` | The IBM Cloud VPC VNI ID assigned after creation. |
| `macAddress` | `string` | The MAC address assigned to the VNI by the VPC API. Injected into the VM spec by the webhook. |
| `primaryIPv4` | `string` | The primary IPv4 address assigned to the VNI. Injected into the VM spec by the webhook. |
| `reservedIPID` | `string` | The VPC reserved IP resource ID associated with the VNI's primary IP. |
| `lastSyncTime` | `*metav1.Time` | Timestamp of the last successful reconciliation with the VPC API. |
| `message` | `string` | Human-readable message providing details about the current state or last error. |
| `conditions` | `[]metav1.Condition` | Standard Kubernetes conditions representing the detailed state of the resource. |

## kubectl Output Columns

When you run `kubectl get virtualnetworkinterfaces` (or `kubectl get vni`), the following columns are displayed:

| Column | Source | Priority | Description |
|--------|--------|----------|-------------|
| VNI ID | `status.vniID` | 0 | The VPC Virtual Network Interface ID. |
| MAC | `status.macAddress` | 0 | The assigned MAC address. |
| IP | `status.primaryIPv4` | 0 | The primary IPv4 address. |
| Subnet | `spec.subnetRef` | 0 | The referenced VPCSubnet CR name. |
| Sync | `status.syncStatus` | 0 | Current sync status. |
| Age | `metadata.creationTimestamp` | 0 | Time since the resource was created. |

## Example

```yaml
apiVersion: vpc.roks.ibm.com/v1alpha1
kind: VirtualNetworkInterface
metadata:
  name: my-vm-eth0
  namespace: default
spec:
  subnetRef: "my-app-subnet"
  securityGroupIDs:
    - "r006-sg-aaaa-1111"
  allowIPSpoofing: true
  enableInfrastructureNat: false
  autoDelete: false
  vmRef:
    namespace: "default"
    name: "my-vm"
  clusterID: "c1a2b3c4d5e6f7g8h9"
```

## Usage

### Automatic Creation via VM Webhook

The primary creation path for VNIs is through the mutating webhook. When a KubeVirt `VirtualMachine` is submitted for creation:

1. The VM Webhook intercepts the admission request.
2. It creates a `VirtualNetworkInterface` CR referencing the appropriate `VPCSubnet`.
3. The VNI Reconciler provisions the VNI in IBM Cloud via the VPC API.
4. Once the VNI is synced, the webhook injects the assigned MAC address and primary IP into the VM spec.

### Manual Creation

You can create a `VirtualNetworkInterface` directly for scenarios such as pre-allocating network identities or debugging.

### Dual Cluster Mode

The VNI Reconciler operates in two modes controlled by the `CLUSTER_MODE` environment variable:

- **`unmanaged`** (default): VNIs are created directly via the IBM Cloud VPC API.
- **`roks`**: VNIs are created via the ROKS platform API (stub, awaiting API availability).

### Important Constraints

- `allowIPSpoofing` must be `true` for OVN LocalNet bridging to work.
- `enableInfrastructureNat` must be `false` for bare metal operation.
- `autoDelete` must be `false` so the operator retains lifecycle control.

### Related Resources

- [VPCSubnet](vpcsubnet.md) -- The VNI is created within a VPC subnet referenced by `spec.subnetRef`.
- [FloatingIP](floatingip.md) -- A floating IP can be bound to a VNI to provide public connectivity.
- [VLANAttachment](vlanattachment.md) -- VLAN attachments provide the L2 bridging that allows VNIs to reach the VPC network.
