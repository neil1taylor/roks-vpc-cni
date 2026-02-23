# Annotations Reference

The VPC Network Operator uses Kubernetes annotations to configure and track VPC resource lifecycle. All annotations share the prefix `vpc.roks.ibm.com/`. Some annotations are **admin-provided** (you set them), while others are **operator-managed** (the operator writes them and you should treat them as read-only).

---

## CUDN Annotations

ClusterUserDefinedNetwork (CUDN) annotations control how the operator provisions VPC subnets, VLAN attachments, and related networking resources for OVN LocalNet topologies.

### Admin-Provided (Required)

| Annotation | Required | Type | Description | Example |
|---|---|---|---|---|
| `vpc.roks.ibm.com/zone` | Yes | `string` | The IBM Cloud VPC zone where the subnet will be created. Must match the zone of your bare metal workers. | `"us-south-1"` |
| `vpc.roks.ibm.com/cidr` | Yes | `string` | IPv4 CIDR block for the VPC subnet. Must not overlap with existing subnets in the VPC. | `"10.240.64.0/24"` |
| `vpc.roks.ibm.com/vpc-id` | Yes | `string` | The ID of the IBM Cloud VPC in which to create the subnet. | `"r006-abc12345-..."` |
| `vpc.roks.ibm.com/vlan-id` | Yes | `string` | VLAN ID used for OVN LocalNet segmentation. Each CUDN must use a unique VLAN ID within the cluster. | `"100"` |
| `vpc.roks.ibm.com/security-group-ids` | Yes | `string` | Comma-separated list of VPC Security Group IDs to attach to virtual network interfaces created under this CUDN. | `"r006-sg1,r006-sg2"` |
| `vpc.roks.ibm.com/acl-id` | Yes | `string` | Network ACL ID to associate with the VPC subnet. Controls inbound and outbound traffic at the subnet level. | `"r006-acl-5678..."` |

### Operator-Managed (Read-Only)

These annotations are written by the operator during reconciliation. Do not modify them manually.

| Annotation | Type | Description | Example |
|---|---|---|---|
| `vpc.roks.ibm.com/subnet-id` | `string` | The ID of the VPC subnet created by the operator. | `"02h7-abcd1234-..."` |
| `vpc.roks.ibm.com/subnet-status` | `string` | Current status of the VPC subnet. One of `"active"`, `"pending"`, or `"error"`. | `"active"` |
| `vpc.roks.ibm.com/vlan-attachments` | `string` | Mapping of bare metal node names to their VLAN attachment IDs. Format: `"node1:att-id-1,node2:att-id-2"`. | `"bm-worker-1:0727-aaa...,bm-worker-2:0727-bbb..."` |

---

## VirtualMachine Annotations

VirtualMachine annotations control per-VM VPC resource provisioning. The mutating webhook reads admin-provided annotations at VM creation time, provisions VPC resources, and writes operator-managed annotations back onto the VM.

### Admin-Provided (Optional)

| Annotation | Required | Type | Description | Example |
|---|---|---|---|---|
| `vpc.roks.ibm.com/fip` | No | `string` | Set to `"true"` to request a floating IP for this VM. The operator will allocate a public IP and attach it to the VM's virtual network interface. | `"true"` |

### Operator-Managed (Read-Only)

These annotations are written by the mutating webhook during VM creation and updated by the VM reconciler during the VM lifecycle. Do not modify them manually.

| Annotation | Type | Description | Example |
|---|---|---|---|
| `vpc.roks.ibm.com/vni-id` | `string` | The ID of the Virtual Network Interface (VNI) created for this VM. | `"02h7-vni-1234..."` |
| `vpc.roks.ibm.com/mac-address` | `string` | The MAC address assigned by the VPC API to the VNI. Injected into the VM spec so OVN LocalNet traffic is correctly forwarded. | `"fa:16:3e:aa:bb:cc"` |
| `vpc.roks.ibm.com/reserved-ip` | `string` | The private IPv4 address reserved on the VPC subnet for this VM. | `"10.240.64.10"` |
| `vpc.roks.ibm.com/reserved-ip-id` | `string` | The VPC resource ID of the reserved IP. Used for cleanup on VM deletion. | `"02h7-rip-5678..."` |
| `vpc.roks.ibm.com/fip-id` | `string` | The ID of the floating IP resource (present only if `fip: "true"` was requested). | `"r006-fip-9012..."` |
| `vpc.roks.ibm.com/fip-address` | `string` | The public IPv4 floating IP address attached to the VM's VNI (present only if `fip: "true"` was requested). | `"169.48.100.50"` |

---

## Example: Fully Annotated CUDN

The following shows a CUDN after the operator has successfully reconciled it. Admin-provided annotations are marked with comments.

```yaml
apiVersion: k8s.ovn.org/v1
kind: ClusterUserDefinedNetwork
metadata:
  name: tenant-network
  annotations:
    # Admin-provided
    vpc.roks.ibm.com/zone: "us-south-1"
    vpc.roks.ibm.com/cidr: "10.240.64.0/24"
    vpc.roks.ibm.com/vpc-id: "r006-abc12345-6789-def0-1234-567890abcdef"
    vpc.roks.ibm.com/vlan-id: "100"
    vpc.roks.ibm.com/security-group-ids: "r006-sg-aaa,r006-sg-bbb"
    vpc.roks.ibm.com/acl-id: "r006-acl-ccc"
    # Operator-managed (read-only)
    vpc.roks.ibm.com/subnet-id: "02h7-subnet-ddd"
    vpc.roks.ibm.com/subnet-status: "active"
    vpc.roks.ibm.com/vlan-attachments: "bm-worker-1:0727-att-eee,bm-worker-2:0727-att-fff"
spec:
  topology: LocalNet
  network:
    localNet:
      subnets:
        - cidr: "10.240.64.0/24"
```

## Example: VM After Webhook Processing

The following shows a VirtualMachine after the mutating webhook has created VPC resources and injected networking details.

```yaml
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: my-vm
  namespace: tenant-ns
  annotations:
    # Admin-provided
    vpc.roks.ibm.com/fip: "true"
    # Operator-managed (read-only)
    vpc.roks.ibm.com/vni-id: "02h7-vni-1234-abcd"
    vpc.roks.ibm.com/mac-address: "fa:16:3e:aa:bb:cc"
    vpc.roks.ibm.com/reserved-ip: "10.240.64.10"
    vpc.roks.ibm.com/reserved-ip-id: "02h7-rip-5678-efgh"
    vpc.roks.ibm.com/fip-id: "r006-fip-9012-ijkl"
    vpc.roks.ibm.com/fip-address: "169.48.100.50"
  finalizers:
    - vpc.roks.ibm.com/vm-cleanup
spec:
  running: true
  template:
    spec:
      domain:
        devices:
          interfaces:
            - name: vpc-net
              bridge: {}
      networks:
        - name: vpc-net
          multus:
            networkName: tenant-ns/tenant-network
```

---

## See Also

- [Glossary](../glossary.md) -- definitions of VPC networking terms
- [Creating VMs with VPC Networking](creating-vms.md) -- step-by-step VM creation guide
- [Network Setup](network-setup.md) -- initial CUDN and VPC configuration
