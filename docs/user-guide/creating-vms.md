# Creating VMs with VPC Networking

This guide walks through creating KubeVirt VirtualMachines that are connected to IBM Cloud VPC networks via the VPC Network Operator. When you create a VM, a mutating webhook intercepts the request, provisions VPC networking resources (virtual network interface, reserved IP), and injects the MAC address and IP into the VM spec so that it comes up fully connected to your VPC subnet.

---

## Prerequisites

Before creating a VM, ensure the following:

1. **The VPC Network Operator is installed and running.** The operator deployment, webhook configuration, and CRDs must all be present in the cluster.

2. **A CUDN exists and is active.** The ClusterUserDefinedNetwork that the VM will attach to must have `vpc.roks.ibm.com/subnet-status: "active"` in its annotations. This means the operator has successfully created the VPC subnet and VLAN attachments.

3. **The target namespace exists** and is configured to use the CUDN's LocalNet network.

4. **VPC quota is sufficient.** Each VM consumes one Virtual Network Interface, one reserved IP, and optionally one floating IP. Check your IBM Cloud VPC quotas before provisioning at scale.

---

## Basic VM

The simplest case: a VM attached to a VPC subnet with no floating IP.

```yaml
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: my-vm
  namespace: tenant-ns
spec:
  running: true
  template:
    spec:
      domain:
        resources:
          requests:
            memory: 4Gi
            cpu: "2"
        devices:
          interfaces:
            - name: vpc-net
              bridge: {}
          disks:
            - name: rootdisk
              disk:
                bus: virtio
      networks:
        - name: vpc-net
          multus:
            networkName: tenant-ns/tenant-network
      volumes:
        - name: rootdisk
          containerDisk:
            image: quay.io/containerdisks/ubuntu:22.04
```

After creation, the webhook will add operator-managed annotations (`vni-id`, `mac-address`, `reserved-ip`, `reserved-ip-id`) and the finalizer `vpc.roks.ibm.com/vm-cleanup`.

---

## VM with Floating IP

To assign a public floating IP, add the `vpc.roks.ibm.com/fip` annotation set to `"true"`:

```yaml
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: my-public-vm
  namespace: tenant-ns
  annotations:
    vpc.roks.ibm.com/fip: "true"
spec:
  running: true
  template:
    spec:
      domain:
        resources:
          requests:
            memory: 4Gi
            cpu: "2"
        devices:
          interfaces:
            - name: vpc-net
              bridge: {}
          disks:
            - name: rootdisk
              disk:
                bus: virtio
      networks:
        - name: vpc-net
          multus:
            networkName: tenant-ns/tenant-network
      volumes:
        - name: rootdisk
          containerDisk:
            image: quay.io/containerdisks/ubuntu:22.04
```

After creation, the VM will have additional annotations for `fip-id` and `fip-address` containing the allocated floating IP details.

---

## What the Webhook Does

When you submit a VirtualMachine CREATE request, the mutating admission webhook performs the following steps:

1. **Validates the request.** Confirms the VM references a valid CUDN with an active VPC subnet.
2. **Looks up the CUDN.** Reads the CUDN annotations to determine the VPC subnet ID, security group IDs, and VLAN configuration.
3. **Creates a Virtual Network Interface (VNI).** Calls the VPC API to create a VNI on the target subnet with the following settings:
   - `AllowIPSpoofing: true` -- required for OVN LocalNet traffic forwarding
   - `EnableInfrastructureNat: false` -- the VM gets direct VPC connectivity
   - `AutoDelete: false` -- the VNI persists across live migrations
4. **Attaches security groups.** Binds the security groups specified in the CUDN annotations to the new VNI.
5. **Records the reserved IP.** The VPC API allocates a private IP from the subnet CIDR and returns it along with the VNI's MAC address.
6. **Creates a floating IP (if requested).** If `vpc.roks.ibm.com/fip: "true"` is present, allocates a floating IP and binds it to the VNI.
7. **Writes operator-managed annotations.** Sets `vni-id`, `mac-address`, `reserved-ip`, `reserved-ip-id`, and optionally `fip-id` and `fip-address`.
8. **Injects MAC and IP into the VM spec.** Modifies the VM's network interface configuration so the guest receives the VPC-assigned MAC and IP.
9. **Adds the finalizer.** Adds `vpc.roks.ibm.com/vm-cleanup` to ensure VPC resources are cleaned up when the VM is deleted.
10. **Returns the mutated VM.** The Kubernetes API server stores the modified VM and proceeds with scheduling.

If any VPC API call fails during this process, the webhook **rejects the admission request** and the VM is not created. Check the operator logs for details.

---

## Verifying VPC Resources

After creating a VM, verify that the operator provisioned the expected VPC resources:

```bash
# Check VM annotations for VPC resource IDs
kubectl get vm my-vm -n tenant-ns -o jsonpath='{.metadata.annotations}' | jq .

# Confirm the VNI was created
kubectl get vm my-vm -n tenant-ns -o jsonpath='{.metadata.annotations.vpc\.roks\.ibm\.com/vni-id}'

# Check the assigned private IP
kubectl get vm my-vm -n tenant-ns -o jsonpath='{.metadata.annotations.vpc\.roks\.ibm\.com/reserved-ip}'

# Check the floating IP (if requested)
kubectl get vm my-vm -n tenant-ns -o jsonpath='{.metadata.annotations.vpc\.roks\.ibm\.com/fip-address}'

# List VNI CRDs in the namespace
kubectl get vni -n tenant-ns

# List FloatingIP CRDs in the namespace
kubectl get fip -n tenant-ns
```

---

## Multiple Network Interfaces

A VM can be attached to multiple CUDNs by specifying multiple network interfaces in the VM spec. Each interface references a different CUDN (and therefore a different VPC subnet). The webhook will create a separate VNI for each interface.

```yaml
spec:
  template:
    spec:
      domain:
        devices:
          interfaces:
            - name: frontend
              bridge: {}
            - name: backend
              bridge: {}
      networks:
        - name: frontend
          multus:
            networkName: tenant-ns/frontend-network
        - name: backend
          multus:
            networkName: tenant-ns/backend-network
```

Each CUDN must have its own set of admin-provided annotations (zone, CIDR, VPC ID, VLAN ID, security groups, ACL). The operator manages each VPC subnet and set of VLAN attachments independently.

---

## Deleting a VM

When you delete a VirtualMachine, the finalizer `vpc.roks.ibm.com/vm-cleanup` triggers the VM reconciler to perform cleanup:

1. **Deletes the floating IP** (if one was allocated).
2. **Deletes the VNI** and its associated reserved IP from the VPC.
3. **Removes the finalizer**, allowing Kubernetes to complete the deletion.

```bash
# Delete the VM
kubectl delete vm my-vm -n tenant-ns

# The operator will clean up VPC resources automatically.
# Verify no orphaned VNI or FIP CRDs remain:
kubectl get vni -n tenant-ns
kubectl get fip -n tenant-ns
```

If the operator is down or the VPC API is unreachable when the VM is deleted, the finalizer will block deletion. The VM will remain in a `Terminating` state until the operator can successfully clean up the VPC resources. The orphan GC (which runs every 10 minutes) also acts as a safety net for resources that may have been leaked.

---

## Troubleshooting

### Webhook Timeout

**Symptom:** VM creation fails with a timeout error.

**Cause:** The VPC API took too long to create the VNI or floating IP.

**Resolution:** Check IBM Cloud VPC API status. Retry the VM creation. Review operator logs for specific API errors:
```bash
kubectl logs -n vpc-network-operator deploy/vpc-network-operator -c manager | grep "webhook"
```

### Missing or Inactive CUDN

**Symptom:** VM creation is rejected with a message about a missing or inactive CUDN.

**Cause:** The CUDN referenced by the VM's network does not exist, or its `subnet-status` annotation is not `"active"`.

**Resolution:** Verify the CUDN exists and is active:
```bash
kubectl get cudn tenant-network -o jsonpath='{.metadata.annotations.vpc\.roks\.ibm\.com/subnet-status}'
```
If the status is `"pending"` or `"error"`, check the CUDN reconciler logs for details.

### VPC Quota Exceeded

**Symptom:** VM creation fails with a VPC API quota error.

**Cause:** Your IBM Cloud account has reached the limit for VNIs, reserved IPs, or floating IPs in the target region.

**Resolution:** Check your current VPC resource usage in the IBM Cloud console or via the CLI:
```bash
ibmcloud is virtual-network-interfaces --output json | jq length
```
Request a quota increase through IBM Cloud support if needed.

### VM Stuck in Terminating

**Symptom:** A deleted VM remains in `Terminating` state indefinitely.

**Cause:** The `vpc.roks.ibm.com/vm-cleanup` finalizer cannot complete because the operator is unable to delete VPC resources.

**Resolution:** Check operator logs for errors. If the VPC resources have already been manually deleted, you can remove the finalizer as a last resort:
```bash
kubectl patch vm my-vm -n tenant-ns --type merge -p '{"metadata":{"finalizers":null}}'
```

---

## See Also

- [Annotations Reference](annotations-reference.md) -- full list of all annotations
- [Floating IPs](floating-ips.md) -- detailed guide on floating IP management
- [Live Migration](live-migration.md) -- how VPC networking supports live migration
- [Network Setup](../admin-guide/network-setup.md) -- initial CUDN and VPC configuration
- [Glossary](../glossary.md) -- definitions of VPC networking terms
