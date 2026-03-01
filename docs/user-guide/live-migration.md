# Live Migration

Live migration moves a running VirtualMachine from one bare metal node to another with zero downtime. The VM's memory, CPU state, and storage are transferred while the guest continues to run. For live migration to work seamlessly with VPC networking, the VM's network identity -- MAC address, private IP, and floating IP -- must be preserved throughout the process. The VPC Network Operator is designed to make this happen automatically.

---

## How the Operator Enables Live Migration

Three design decisions in the operator ensure that VPC networking survives live migration:

### Floatable VLAN Attachments

Every VLAN attachment created by the operator has `AllowToFloat: true` set. This tells the VPC infrastructure that virtual network interfaces attached via this VLAN are allowed to move between bare metal hosts. Without this flag, the VPC would block traffic to a VNI that appears on a different host than where it was originally created.

### Non-Auto-Delete VNIs

Virtual network interfaces are created with `AutoDelete: false`. This prevents the VPC from automatically deleting the VNI when the hosting bare metal server's network attachment changes. The VNI persists independently of any specific host, which is essential because during migration the VNI effectively detaches from the source host and reattaches on the destination host.

### Node Reconciler Pre-Provisioning

The Node Reconciler watches for bare metal `Node` objects in the cluster. Whenever a new node joins (or an existing node is reconciled), the operator ensures that VLAN attachments exist on that node for every active CUDN. This means that when KubeVirt selects a destination node for live migration, the necessary VLAN attachment is already in place -- there is no delay waiting for VPC infrastructure to be provisioned.

---

## What Happens During Migration

When a live migration is initiated, the following sequence occurs:

1. **KubeVirt initiates the migration.** A `VirtualMachineInstanceMigration` resource is created (either manually or by the cluster scheduler). KubeVirt selects a destination node and starts the migration process.

2. **Memory and state transfer begins.** KubeVirt copies the VM's memory pages from the source node to the destination node over the cluster's internal network. The VM continues running on the source during this phase.

3. **Final switchover.** Once the bulk of memory has been transferred, KubeVirt performs a brief pause (typically milliseconds) to copy the last dirty pages and CPU state, then resumes the VM on the destination node.

4. **VNI floats to the destination.** The VNI, which was associated with the source node's VLAN attachment, is now associated with the destination node's VLAN attachment. Because both VLAN attachments have `AllowToFloat: true`, the VPC infrastructure updates its forwarding tables to direct traffic to the new host.

5. **Traffic continues.** The VM retains its original MAC address, private IP, and floating IP (if any). External clients and other VMs on the VPC subnet see no change in addressing. After a brief forwarding-table convergence period, traffic flows to the VM on its new host.

---

## Network Continuity

The following network properties are fully preserved across a live migration:

| Property | Preserved? | Mechanism |
|---|---|---|
| MAC address | Yes | The VNI retains its MAC address. It is a property of the VNI, not the host. |
| Private reserved IP | Yes | The reserved IP is bound to the VNI, not to a host or VLAN attachment. |
| Floating IP | Yes | The floating IP is bound to the VNI. It follows the VNI to the new host. |
| Security group rules | Yes | Security groups are attached to the VNI. They remain in effect regardless of host. |
| TCP connections | Yes* | Existing TCP connections survive because the IP and MAC do not change. Brief packet loss during switchover may cause minor retransmissions. |

*Extremely latency-sensitive applications (e.g., real-time audio/video) may notice a brief disruption during the switchover phase, but TCP connections will not be dropped.

---

## Prerequisites

For live migration to work correctly with VPC networking:

1. **Multiple bare metal nodes.** The cluster must have at least two bare metal worker nodes in the same VPC zone.

2. **VLAN attachments on all nodes.** Each bare metal node must have a VLAN attachment for every CUDN. The Node Reconciler handles this automatically. You can verify:
   ```bash
   kubectl get cudn tenant-network \
     -o jsonpath='{.metadata.annotations.vpc\.roks\.ibm\.com/vlan-attachments}'
   ```
   The output should list all bare metal nodes in the cluster.

3. **KubeVirt live migration support.** The KubeVirt installation must be configured to support live migration (this is the default on OpenShift Virtualization).

4. **Sufficient resources on the destination node.** The destination node must have enough CPU and memory to accommodate the migrating VM.

---

## Triggering a Migration

You can trigger a live migration using the `virtctl` CLI or `oc`/`kubectl`:

### Using virtctl

```bash
# Migrate a running VM
virtctl migrate my-vm -n tenant-ns
```

### Using kubectl

```bash
# Create a VirtualMachineInstanceMigration resource
cat <<EOF | kubectl apply -f -
apiVersion: kubevirt.io/v1
kind: VirtualMachineInstanceMigration
metadata:
  name: my-vm-migration
  namespace: tenant-ns
spec:
  vmiName: my-vm
EOF
```

### Using oc (OpenShift)

```bash
# OpenShift Virtualization provides integrated migration support
oc get vmi my-vm -n tenant-ns -o jsonpath='{.status.nodeName}'
# Note the current node, then trigger migration:
virtctl migrate my-vm -n tenant-ns
```

---

## Verifying After Migration

After the migration completes, verify that VPC networking is intact:

```bash
# Confirm the VM is running on a new node
kubectl get vmi my-vm -n tenant-ns -o jsonpath='{.status.nodeName}'

# Verify the VNI annotations are unchanged
kubectl get vm my-vm -n tenant-ns -o jsonpath='{.metadata.annotations.vpc\.roks\.ibm\.com/vni-id}'
kubectl get vm my-vm -n tenant-ns -o jsonpath='{.metadata.annotations.vpc\.roks\.ibm\.com/reserved-ip}'

# Verify the floating IP (if applicable) is unchanged
kubectl get vm my-vm -n tenant-ns -o jsonpath='{.metadata.annotations.vpc\.roks\.ibm\.com/fip-address}'

# Check the migration status
kubectl get vmim -n tenant-ns
```

You can also verify connectivity by pinging the VM's private or floating IP from another VM or from an external host:

```bash
# From another VM on the same VPC subnet
ping 10.240.64.10

# From the internet (if floating IP is assigned)
ping 169.48.100.50
```

If connectivity does not recover within a few seconds after migration, check:

- The VLAN attachment exists on the destination node (see the CUDN's `vlan-attachments` annotation).
- The VNI still exists in the VPC (check via `ibmcloud is virtual-network-interfaces`).
- Security group rules allow the traffic you are testing.

---

## See Also

- [Annotations Reference](annotations-reference.md) -- annotations that persist across migrations
- [Creating VMs with VPC Networking](creating-vms.md) -- initial VM setup
- [Floating IPs](floating-ips.md) -- floating IPs and migration behavior
- [Network Setup](../admin-guide/network-setup.md) -- CUDN configuration and VLAN attachments
- [Glossary](../glossary.md) -- definitions of VPC networking terms
