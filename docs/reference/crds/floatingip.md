# FloatingIP

The `FloatingIP` custom resource represents an IBM Cloud VPC Floating IP managed by the ROKS VPC Network Operator. A Floating IP provides a public IPv4 address that can be bound to a Virtual Network Interface (VNI), enabling inbound connectivity from the internet to a KubeVirt virtual machine. FloatingIPs can be created manually or provisioned automatically based on VM annotations.

## API Information

| Field | Value |
|-------|-------|
| Group | `vpc.roks.ibm.com` |
| Version | `v1alpha1` |
| Kind | `FloatingIP` |
| Short Name | `fip` |
| Scope | Namespaced |

## Spec

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `zone` | `string` | Yes | - | VPC availability zone for the floating IP, e.g. `us-south-1`. Must be the same zone as the target VNI's subnet. Must not be empty. |
| `vniRef` | `string` | No | `""` | Name of the `VirtualNetworkInterface` CR in the same namespace to bind this floating IP to. Mutually informative with `vniID`; if both are provided, `vniID` takes precedence. |
| `vniID` | `string` | No | `""` | Direct VPC VNI ID to bind this floating IP to. If provided, takes precedence over resolving the VNI from `vniRef`. |
| `name` | `string` | No | `""` | Desired name for the floating IP resource in IBM Cloud VPC. If omitted, a name is generated based on cluster ID, namespace, and CR name. |

## Status

| Field | Type | Description |
|-------|------|-------------|
| `syncStatus` | `string` | Current synchronization state. One of: `Synced`, `Pending`, `Failed`. |
| `fipID` | `string` | The IBM Cloud VPC Floating IP ID assigned after creation. |
| `address` | `string` | The public IPv4 address assigned to this floating IP. |
| `targetVNIID` | `string` | The VPC VNI ID that this floating IP is currently bound to. |
| `lastSyncTime` | `*metav1.Time` | Timestamp of the last successful reconciliation with the VPC API. |
| `message` | `string` | Human-readable message providing details about the current state or last error. |
| `conditions` | `[]metav1.Condition` | Standard Kubernetes conditions representing the detailed state of the resource. |

## kubectl Output Columns

When you run `kubectl get floatingips` (or `kubectl get fip`), the following columns are displayed:

| Column | Source | Priority | Description |
|--------|--------|----------|-------------|
| Address | `status.address` | 0 | The public IPv4 address. |
| Zone | `spec.zone` | 0 | The VPC availability zone. |
| VNI | `spec.vniRef` | 0 | The referenced VNI CR name. |
| Sync | `status.syncStatus` | 0 | Current sync status. |
| Age | `metadata.creationTimestamp` | 0 | Time since the resource was created. |

## Example

```yaml
apiVersion: vpc.roks.ibm.com/v1alpha1
kind: FloatingIP
metadata:
  name: my-vm-public-ip
  namespace: default
spec:
  zone: "us-south-1"
  vniRef: "my-vm-eth0"
  name: "my-cluster-default-my-vm-fip"
```

## Usage

### Annotation-Driven Creation

The most common creation path is through VM annotations. When a KubeVirt `VirtualMachine` is annotated with a floating IP request annotation (`vpc.roks.ibm.com/floating-ip`), the VM Reconciler creates a corresponding `FloatingIP` CR that targets the VM's VNI.

### Manual Creation

You can create a `FloatingIP` directly to assign a public IP to any VNI:

1. Identify or create the `VirtualNetworkInterface` CR you want to expose.
2. Create a `FloatingIP` CR referencing it via `vniRef` or `vniID`.
3. The FloatingIP Reconciler provisions the floating IP in IBM Cloud and binds it to the VNI.

### Lifecycle

1. The FloatingIP Reconciler detects a new `FloatingIP` CR.
2. It checks for an existing floating IP using resource tags (cluster ID + namespace + name) to ensure idempotency.
3. If no matching floating IP exists, it creates one via the VPC API in the specified zone.
4. If a `vniRef` or `vniID` is specified, the reconciler binds the floating IP to the target VNI.
5. The reconciler periodically syncs the status, updating `fipID`, `address`, and `targetVNIID`.
6. On deletion, the reconciler unbinds and deletes the floating IP from IBM Cloud.

### Cleanup

When a KubeVirt `VirtualMachine` is deleted, the VM Reconciler handles cleanup of associated `FloatingIP` CRs. The Orphan GC also periodically checks for floating IPs in IBM Cloud that no longer have a corresponding CR, deleting them after a 15-minute grace period.

### Related Resources

- [VirtualNetworkInterface](virtualnetworkinterface.md) -- The floating IP is bound to a VNI referenced by `spec.vniRef` or `spec.vniID`.
- [VPCSubnet](vpcsubnet.md) -- The floating IP's zone must match the zone of the subnet containing the target VNI.
- [VLANAttachment](vlanattachment.md) -- VLAN attachments provide the underlying L2 connectivity that the VNI (and by extension, the floating IP) depends on.
