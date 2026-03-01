# Floating IPs

A floating IP (FIP) is a public IPv4 address that you can attach to a VirtualMachine's virtual network interface (VNI) to make it reachable from the internet. Floating IPs are not tied to a specific physical host -- they follow the VNI, which means they are preserved across live migrations.

Use a floating IP when:

- Your VM needs to accept inbound connections from the public internet.
- You need a stable public IP that persists across VM restarts and migrations.
- You want to expose a service running inside a VM without configuring a load balancer.

---

## Requesting a Floating IP

The simplest way to request a floating IP is to add the `vpc.roks.ibm.com/fip` annotation to your VirtualMachine before creating it:

```yaml
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: my-vm
  namespace: tenant-ns
  annotations:
    vpc.roks.ibm.com/fip: "true"
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
      # ... remaining spec
```

The mutating webhook detects this annotation during VM creation and allocates a floating IP from the VPC API as part of the admission flow.

---

## How It Works

When the operator processes a VM with `vpc.roks.ibm.com/fip: "true"`, the following sequence occurs:

1. **VNI creation.** The webhook first creates the virtual network interface on the VPC subnet (this happens regardless of whether a floating IP is requested).
2. **Floating IP allocation.** The operator calls the VPC API to create a new floating IP in the same zone as the VPC subnet.
3. **Binding.** The floating IP is bound to the VM's VNI. All inbound traffic to the floating IP is forwarded to the VNI's private reserved IP.
4. **Annotation update.** The operator writes two annotations onto the VM:
   - `vpc.roks.ibm.com/fip-id` -- the VPC resource ID of the floating IP
   - `vpc.roks.ibm.com/fip-address` -- the public IPv4 address (e.g., `169.48.100.50`)
5. **FloatingIP CRD creation.** A `FloatingIP` custom resource is created in the VM's namespace to track the lifecycle of the VPC floating IP resource.

Because the floating IP is bound to the VNI (not to a bare metal host), it automatically follows the VM during live migrations.

---

## Viewing Floating IP Details

After the VM has been created, inspect its floating IP:

```bash
# Get the public floating IP address
kubectl get vm my-vm -n tenant-ns \
  -o jsonpath='{.metadata.annotations.vpc\.roks\.ibm\.com/fip-address}'

# Get the floating IP resource ID
kubectl get vm my-vm -n tenant-ns \
  -o jsonpath='{.metadata.annotations.vpc\.roks\.ibm\.com/fip-id}'

# List all FloatingIP CRDs in the namespace
kubectl get fip -n tenant-ns

# Describe a specific FloatingIP CRD for full details
kubectl describe fip my-vm-fip -n tenant-ns
```

You can also verify the floating IP exists in IBM Cloud:

```bash
ibmcloud is floating-ips --output json | jq '.[] | select(.name | contains("my-vm"))'
```

---

## FloatingIP CRD

The operator creates a `FloatingIP` custom resource (API group `vpc.roks.ibm.com/v1alpha1`, short name `fip`) for each floating IP it manages. This CRD provides a Kubernetes-native view of the VPC floating IP and is used by the FloatingIP reconciler to track lifecycle and handle cleanup.

```yaml
apiVersion: vpc.roks.ibm.com/v1alpha1
kind: FloatingIP
metadata:
  name: my-vm-fip
  namespace: tenant-ns
  ownerReferences:
    - apiVersion: kubevirt.io/v1
      kind: VirtualMachine
      name: my-vm
      uid: abc12345-...
spec:
  zone: us-south-1
  vniRef:
    name: my-vm-vni
    namespace: tenant-ns
status:
  id: "r006-fip-9012-ijkl"
  address: "169.48.100.50"
  state: active
  vpcResourceID: "r006-fip-9012-ijkl"
```

The `ownerReferences` field ties the FloatingIP to its parent VirtualMachine. When the VM is deleted, the FloatingIP CRD is garbage-collected, which triggers the FloatingIP reconciler to release the VPC floating IP.

---

## Removing a Floating IP

There are two ways to remove a floating IP:

### Delete the VM

When a VM with a floating IP is deleted, the `vpc.roks.ibm.com/vm-cleanup` finalizer ensures the operator deletes both the VNI and the floating IP from the VPC before allowing the VM to be fully removed.

```bash
kubectl delete vm my-vm -n tenant-ns
```

### Delete the FloatingIP CRD

If you want to release the floating IP while keeping the VM running, delete the FloatingIP CRD directly:

```bash
kubectl delete fip my-vm-fip -n tenant-ns
```

The FloatingIP reconciler will unbind and release the VPC floating IP. The VM will continue running with only its private reserved IP. Note that the `vpc.roks.ibm.com/fip-id` and `vpc.roks.ibm.com/fip-address` annotations on the VM will be cleared by the VM reconciler's drift detection.

---

## Security Considerations

A floating IP makes your VM reachable from the public internet. Inbound traffic is controlled by the **VPC security groups** attached to the VM's VNI. Before assigning a floating IP, ensure your security groups are properly configured:

- **Default deny.** Start with security groups that deny all inbound traffic, then add specific rules for the ports and protocols you need.
- **Restrict source IPs.** Where possible, limit inbound rules to specific source CIDR ranges rather than allowing `0.0.0.0/0`.
- **Use network ACLs as a second layer.** The network ACL on the VPC subnet provides stateless packet filtering in addition to stateful security group rules.
- **Audit regularly.** Use the console plugin's Security Groups page or the IBM Cloud console to review security group rules attached to VNIs with floating IPs.

The security groups attached to a VNI are determined by the `vpc.roks.ibm.com/security-group-ids` annotation on the CUDN. All VMs on the same CUDN share the same set of security groups. If you need per-VM security group control, consider placing VMs on separate CUDNs or managing security groups directly via the VPC API.

---

## See Also

- [Annotations Reference](annotations-reference.md) -- full list of annotations including floating IP annotations
- [Creating VMs with VPC Networking](creating-vms.md) -- end-to-end VM creation workflow
- [Live Migration](live-migration.md) -- floating IPs are preserved during migration
- [Network Setup](../admin-guide/network-setup.md) -- configuring CUDNs, security groups, and ACLs
- [Glossary](../glossary.md) -- definitions of VPC networking terms
