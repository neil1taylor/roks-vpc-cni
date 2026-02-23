# Network Setup

This guide covers creating and managing VPC-backed networks for KubeVirt VMs using ClusterUserDefinedNetworks (CUDNs). Each CUDN provisions a VPC subnet and associated VLAN attachments on all bare metal nodes in the specified zone.

## Creating a CUDN

A ClusterUserDefinedNetwork (CUDN) is an OVN-Kubernetes resource that defines a LocalNet network. The VPC Network Operator watches CUDNs annotated with `vpc.roks.ibm.com/*` annotations and provisions the corresponding VPC resources automatically.

### Full annotated example

```yaml
apiVersion: k8s.ovn.org/v1
kind: ClusterUserDefinedNetwork
metadata:
  name: production-net
  annotations:
    # Required: The VPC availability zone for the subnet.
    vpc.roks.ibm.com/zone: "us-south-1"

    # Required: The CIDR block for the VPC subnet.
    # Must fall within an existing VPC address prefix and not overlap
    # with other subnets in the VPC.
    vpc.roks.ibm.com/cidr: "10.240.10.0/24"

    # Required: The ID of the VPC where the subnet will be created.
    vpc.roks.ibm.com/vpc-id: "r006-xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"

    # Required: The VLAN ID for the LocalNet network.
    # Must be unique across all CUDNs on the same bare metal nodes.
    # Valid range: 1-4094.
    vpc.roks.ibm.com/vlan-id: "100"

    # Required: Comma-separated list of security group IDs to attach
    # to VNIs created on this network.
    vpc.roks.ibm.com/security-group-ids: "r006-aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa,r006-bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"

    # Optional: Network ACL ID to associate with the VPC subnet.
    # If omitted, the VPC default ACL is used.
    vpc.roks.ibm.com/acl-id: "r006-cccccccc-cccc-cccc-cccc-cccccccccccc"
spec:
  topology: LocalNet
  network:
    localNet:
      role: Secondary
      physicalNetworkName: production-net
      subnets:
        cidrs:
          - "10.240.10.0/24"
```

### Annotation reference

| Annotation | Required | Description |
|-----------|----------|-------------|
| `vpc.roks.ibm.com/zone` | Yes | VPC availability zone (e.g., `us-south-1`, `us-south-2`). |
| `vpc.roks.ibm.com/cidr` | Yes | CIDR block for the VPC subnet. Must match the `spec.network.localNet.subnets.cidrs` entry. |
| `vpc.roks.ibm.com/vpc-id` | Yes | VPC ID where the subnet is created. |
| `vpc.roks.ibm.com/vlan-id` | Yes | VLAN ID for the LocalNet bridge mapping. Must be unique per bare metal node. |
| `vpc.roks.ibm.com/security-group-ids` | Yes | Comma-separated security group IDs applied to all VNIs on this network. |
| `vpc.roks.ibm.com/acl-id` | No | Network ACL ID for the VPC subnet. Defaults to the VPC default ACL if omitted. |

### What happens after creation

When you apply the CUDN, the operator performs the following in order:

1. **CUDN Reconciler** creates a VPC subnet in the specified zone with the given CIDR.
2. **CUDN Reconciler** creates a VLAN attachment on every bare metal node in that zone, using the specified VLAN ID.
3. **Node Reconciler** watches for new bare metal nodes joining the cluster and creates VLAN attachments for all existing CUDNs on those nodes.
4. The CUDN status is updated with the VPC subnet ID once provisioning completes.

## Verifying Network Provisioning

After creating a CUDN, verify that the VPC resources were provisioned correctly.

### Check the CUDN status

```bash
kubectl get clusteruserdefinednetwork production-net -o yaml
```

Look for status conditions indicating the network is ready.

### Check the VPC subnet CRD

```bash
kubectl get vpcsubnets -o wide
```

This lists all VPC subnets managed by the operator, including their VPC subnet IDs and status.

### Check VLAN attachments

```bash
kubectl get vlanattachments -o wide
```

There should be one VLAN attachment per bare metal node in the CUDN's zone.

### Verify in IBM Cloud

```bash
# List subnets in the VPC
ibmcloud is subnets --vpc <vpc-id>

# Check a specific subnet
ibmcloud is subnet <subnet-id>
```

### Check operator logs

```bash
kubectl logs -n roks-vpc-network-operator deployment/roks-vpc-network-operator \
  --tail=100 | grep production-net
```

### Check events

```bash
kubectl get events --field-selector reason=VPCSubnetCreated
kubectl get events --field-selector reason=VLANAttachmentCreated
```

## Multiple Networks

You can create multiple CUDNs to support different zones, tenants, or network tiers.

### Multiple zones

For high availability, create one CUDN per zone with different subnets:

```yaml
# Zone 1
apiVersion: k8s.ovn.org/v1
kind: ClusterUserDefinedNetwork
metadata:
  name: production-net-zone1
  annotations:
    vpc.roks.ibm.com/zone: "us-south-1"
    vpc.roks.ibm.com/cidr: "10.240.10.0/24"
    vpc.roks.ibm.com/vpc-id: "r006-xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
    vpc.roks.ibm.com/vlan-id: "100"
    vpc.roks.ibm.com/security-group-ids: "r006-aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
spec:
  topology: LocalNet
  network:
    localNet:
      role: Secondary
      physicalNetworkName: production-net-zone1
      subnets:
        cidrs:
          - "10.240.10.0/24"
---
# Zone 2
apiVersion: k8s.ovn.org/v1
kind: ClusterUserDefinedNetwork
metadata:
  name: production-net-zone2
  annotations:
    vpc.roks.ibm.com/zone: "us-south-2"
    vpc.roks.ibm.com/cidr: "10.240.20.0/24"
    vpc.roks.ibm.com/vpc-id: "r006-xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
    vpc.roks.ibm.com/vlan-id: "100"
    vpc.roks.ibm.com/security-group-ids: "r006-aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
spec:
  topology: LocalNet
  network:
    localNet:
      role: Secondary
      physicalNetworkName: production-net-zone2
      subnets:
        cidrs:
          - "10.240.20.0/24"
```

Note that the VLAN ID can be the same across zones because VLAN IDs only need to be unique per bare metal node.

### Tenant isolation

Create separate CUDNs with different security groups for multi-tenant isolation:

```yaml
apiVersion: k8s.ovn.org/v1
kind: ClusterUserDefinedNetwork
metadata:
  name: tenant-a-net
  annotations:
    vpc.roks.ibm.com/zone: "us-south-1"
    vpc.roks.ibm.com/cidr: "10.240.30.0/24"
    vpc.roks.ibm.com/vpc-id: "r006-xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
    vpc.roks.ibm.com/vlan-id: "200"
    vpc.roks.ibm.com/security-group-ids: "r006-tenant-a-sg-id"
spec:
  topology: LocalNet
  network:
    localNet:
      role: Secondary
      physicalNetworkName: tenant-a-net
      subnets:
        cidrs:
          - "10.240.30.0/24"
```

### Key constraints for multiple networks

- Each CUDN must have a unique CIDR that does not overlap with other subnets in the VPC.
- VLAN IDs must be unique per bare metal node. If two CUDNs target the same zone (and thus the same nodes), they must use different VLAN IDs.
- The `physicalNetworkName` in the spec should match the CUDN `metadata.name`.

## Modifying a Network

Some properties of a CUDN can be changed after creation, while others cannot.

### Properties that can be modified

- `vpc.roks.ibm.com/security-group-ids` -- Changing security group IDs updates the security groups on new VNIs. Existing VNIs are updated on the next reconciliation cycle.
- `vpc.roks.ibm.com/acl-id` -- The ACL on the VPC subnet can be changed.

### Properties that cannot be modified

- `vpc.roks.ibm.com/zone` -- The zone is fixed at subnet creation time.
- `vpc.roks.ibm.com/cidr` -- VPC subnets cannot be resized. To change the CIDR, delete the CUDN and create a new one.
- `vpc.roks.ibm.com/vpc-id` -- The VPC is fixed at subnet creation time.
- `vpc.roks.ibm.com/vlan-id` -- Changing the VLAN ID would disconnect all existing VMs. Delete all VMs first, then delete and recreate the CUDN.

If you need to change an immutable property, follow the deletion procedure below, then create a new CUDN with the desired configuration.

## Deleting a Network

Deleting a CUDN triggers cleanup of all associated VPC resources. You must delete VMs using the network before deleting the CUDN.

### Step 1: List VMs on the network

```bash
kubectl get virtualmachines --all-namespaces -o json | \
  jq -r '.items[] | select(.spec.template.spec.domain.devices.interfaces[]?.bridge?.networkName == "production-net") | .metadata.namespace + "/" + .metadata.name'
```

### Step 2: Delete VMs

```bash
kubectl delete virtualmachine <vm-name> -n <namespace>
```

Wait for all VMs on the network to be fully deleted. The VM reconciler handles cleanup of each VM's VNI and floating IP.

### Step 3: Verify VNI cleanup

```bash
kubectl get virtualnetworkinterfaces --all-namespaces
```

Ensure no VNIs remain that reference the network being deleted.

### Step 4: Delete the CUDN

```bash
kubectl delete clusteruserdefinednetwork production-net
```

The CUDN reconciler's finalizer (`vpc.roks.ibm.com/cudn-cleanup`) triggers deletion of:

1. All VLAN attachments on bare metal nodes for this CUDN.
2. The VPC subnet.

### Step 5: Verify cleanup

```bash
# Check that VLAN attachments are gone
kubectl get vlanattachments -l vpc.roks.ibm.com/cudn=production-net

# Check that the VPC subnet is gone
kubectl get vpcsubnets

# Verify in IBM Cloud
ibmcloud is subnets --vpc <vpc-id>
```

If any resources remain after 15 minutes, the orphan garbage collector will clean them up automatically.

---

**See also:**

- [VPC Prerequisites](vpc-prerequisites.md) -- preparing VPC resources before creating CUDNs
- [Configuration](configuration.md) -- Helm values and environment variables
- [Uninstalling](uninstalling.md) -- full cleanup when removing the operator
