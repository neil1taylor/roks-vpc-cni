# Dual Cluster Mode

The VPC Network Operator supports two deployment modes, controlled by the `CLUSTER_MODE` environment variable (set via `bff.clusterMode` in Helm values).

---

## Mode Overview

| Aspect | ROKS Mode (`roks`) | Unmanaged Mode (`unmanaged`) |
|--------|-------------------|------------------------------|
| **VNI management** | ROKS platform manages VNIs | Operator manages VNIs via VPC API |
| **VLAN attachment management** | ROKS platform manages VLAN attachments | Operator manages via VPC API |
| **Subnet management** | Operator manages via VPC API | Operator manages via VPC API |
| **Security group management** | Operator attaches; admin manages rules | Operator attaches; admin manages rules |
| **Floating IP management** | Operator manages via VPC API | Operator manages via VPC API |
| **API key required** | Yes (for subnet, FIP, SG, ACL operations) | Yes (for all VPC operations) |
| **IAM roles** | VPC Editor + IP Spoofing Operator | VPC Editor + IP Spoofing Operator |

---

## ROKS Mode

In ROKS mode (`CLUSTER_MODE=roks`), the ROKS platform is responsible for managing VNIs and VLAN attachments as part of its built-in bare metal networking support. The operator:

- **Creates and manages** VPC subnets, floating IPs, security groups, and ACLs as usual
- **Syncs status** of VNIs and VLAN attachments from the ROKS API into CRD status fields
- **Does not create or delete** VNIs and VLAN attachments directly

### VNI Reconciler in ROKS Mode

When the ROKS API is available, the VNI Reconciler:
1. Reads VNI information from the ROKS platform API
2. Populates the `VirtualNetworkInterface` CRD status (VNI ID, MAC, IP)
3. Sets `syncStatus: Synced`

When the ROKS API is not yet available (stub phase):
1. Sets `syncStatus: Pending`
2. Sets message: "VNI is managed by ROKS platform. ROKS API integration pending."
3. Requeues every 5 minutes to check availability

### VLANAttachment Reconciler in ROKS Mode

Same pattern as VNI — read-only sync from ROKS platform, `Pending` status until API is available.

---

## Unmanaged Mode

In unmanaged mode (`CLUSTER_MODE=unmanaged`), the operator has full control over all VPC resources:

- **Creates** VNIs via `POST /v1/virtual_network_interfaces` with required settings (`allow_ip_spoofing`, `enable_infrastructure_nat`, `auto_delete`)
- **Creates** VLAN attachments via `POST /v1/bare_metal_servers/{id}/network_attachments`
- **Deletes** these resources via the corresponding VPC API endpoints
- **Full lifecycle** with finalizers preventing orphaned resources

This mode is used for:
- Self-managed OpenShift clusters on IBM Cloud VPC
- Development and testing environments
- Clusters where the ROKS platform does not yet support automated VNI/VLAN management

---

## Feature Flags

The BFF's `/api/v1/cluster-info` endpoint returns feature flags based on the cluster mode:

```json
// ROKS mode
{
  "clusterMode": "roks",
  "features": {
    "vniManagement": false,
    "vlanAttachmentManagement": false,
    "subnetManagement": true,
    "securityGroupManagement": true,
    "networkACLManagement": true,
    "floatingIPManagement": true,
    "roksAPIAvailable": false
  }
}

// Unmanaged mode
{
  "clusterMode": "unmanaged",
  "features": {
    "vniManagement": true,
    "vlanAttachmentManagement": true,
    "subnetManagement": true,
    "securityGroupManagement": true,
    "networkACLManagement": true,
    "floatingIPManagement": true,
    "roksAPIAvailable": false
  }
}
```

The console plugin uses these flags to show or hide create/delete actions on VNI and VLAN Attachment pages.

---

## Configuring the Mode

### Via Helm

```bash
# ROKS mode (default)
helm upgrade --install vpc-network-operator \
  deploy/helm/roks-vpc-network-operator \
  --set bff.clusterMode=roks

# Unmanaged mode
helm upgrade --install vpc-network-operator \
  deploy/helm/roks-vpc-network-operator \
  --set bff.clusterMode=unmanaged
```

### Via Environment Variable

The operator manager reads `CLUSTER_MODE` directly:

```yaml
env:
- name: CLUSTER_MODE
  value: "unmanaged"
```

---

## ROKS API Client

The ROKS client (`pkg/roks/`) is a separate API client for the IBM Cloud ROKS platform API. It provides:

- `IsAvailable(ctx)` — checks if the ROKS API is reachable
- `GetVNIByVM(ctx, namespace, name)` — retrieves VNI details for a VM (planned)
- `ListVLANAttachmentsByNode(ctx, nodeName)` — lists VLAN attachments for a node (planned)

The ROKS API client is currently a stub, awaiting the ROKS platform API for VNI/VLAN management to become available. When it does, the operator will automatically start syncing status in ROKS mode.

---

## Migration Between Modes

Changing the cluster mode after initial deployment is not recommended, as it changes which component manages VNI and VLAN attachment lifecycle. If you need to change modes:

1. Delete all VMs (ensures VNI cleanup)
2. Delete all CUDNs (ensures VLAN attachment cleanup)
3. Update the Helm value and upgrade
4. Recreate CUDNs and VMs

---

## Next Steps

- [Architecture Overview](overview.md) — System-level architecture
- [Operator Internals](operator-internals.md) — Reconciler deep dive
- [Configuration](../admin-guide/configuration.md) — All Helm values and env vars
