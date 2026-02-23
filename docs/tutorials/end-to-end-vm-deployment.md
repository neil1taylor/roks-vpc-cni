# Tutorial: End-to-End VM Deployment

This tutorial walks you through the complete lifecycle of deploying a VM with VPC networking: planning the network, creating the CUDN, deploying the VM with a floating IP, connecting via SSH, and cleaning up.

**Time:** ~20 minutes
**Prerequisites:** VPC Network Operator [installed](../getting-started/installation.md), IBM Cloud CLI tools, an existing VPC with security groups and ACLs.

---

## Step 1: Plan Your Network

Before creating any resources, decide on:

| Decision | Value for This Tutorial |
|----------|------------------------|
| VPC | Use your existing VPC |
| Zone | `us-south-1` |
| CIDR | `10.240.100.0/24` (256 addresses) |
| VLAN ID | `200` |
| Security Group | A group that allows SSH (port 22) and ICMP |
| Network ACL | An ACL that allows all internal traffic |

### Verify VPC resources

```bash
# List VPCs
ibmcloud is vpcs

# List security groups (note the IDs you want to use)
ibmcloud is security-groups

# List network ACLs
ibmcloud is network-acls
```

If you need to create a security group for this tutorial:

```bash
# Create a security group
ibmcloud is security-group-create tutorial-sg <your-vpc-id> \
  --output json

# Allow inbound SSH
ibmcloud is security-group-rule-add <sg-id> inbound tcp \
  --port-min 22 --port-max 22 --remote 0.0.0.0/0

# Allow inbound ICMP (ping)
ibmcloud is security-group-rule-add <sg-id> inbound icmp \
  --remote 0.0.0.0/0

# Allow all outbound
ibmcloud is security-group-rule-add <sg-id> outbound all
```

---

## Step 2: Create the Network (CUDN)

Create a file `tutorial-cudn.yaml`:

```yaml
apiVersion: k8s.ovn.org/v1
kind: ClusterUserDefinedNetwork
metadata:
  name: tutorial-network
  annotations:
    vpc.roks.ibm.com/zone: "us-south-1"
    vpc.roks.ibm.com/cidr: "10.240.100.0/24"
    vpc.roks.ibm.com/vpc-id: "<your-vpc-id>"
    vpc.roks.ibm.com/vlan-id: "200"
    vpc.roks.ibm.com/security-group-ids: "<your-sg-id>"
    vpc.roks.ibm.com/acl-id: "<your-acl-id>"
spec:
  topology: LocalNet
```

Apply it:

```bash
oc apply -f tutorial-cudn.yaml
```

### What happens behind the scenes

1. The CUDN Reconciler validates all six required annotations
2. Adds the `vpc.roks.ibm.com/cudn-cleanup` finalizer
3. Creates a VPC subnet `roks-<cluster-id>-tutorial-network` with CIDR `10.240.100.0/24`
4. Creates a VLAN attachment on every bare metal node (VLAN ID 200, `floatable: true`)
5. Writes status annotations: `subnet-id`, `subnet-status`, `vlan-attachments`

### Verify

```bash
# Check annotations
oc get cudn tutorial-network -o yaml | grep 'vpc.roks.ibm.com'
```

Wait for `vpc.roks.ibm.com/subnet-status: active`. This typically takes 10-30 seconds.

```bash
# Verify the VPC subnet was created
ibmcloud is subnets | grep tutorial-network
```

---

## Step 3: Create the Namespace

```bash
oc create namespace tutorial
```

---

## Step 4: Deploy the VM

Create a file `tutorial-vm.yaml`:

```yaml
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: tutorial-vm
  namespace: tutorial
  annotations:
    # Request a floating IP for SSH access
    vpc.roks.ibm.com/fip: "true"
spec:
  running: true
  template:
    spec:
      networks:
      - name: vpc-net
        multus:
          networkName: tutorial-network
      domain:
        resources:
          requests:
            memory: 2Gi
            cpu: "1"
        devices:
          interfaces:
          - name: vpc-net
            bridge: {}
          disks:
          - name: rootdisk
            disk:
              bus: virtio
      volumes:
      - name: rootdisk
        containerDisk:
          image: quay.io/containerdisks/ubuntu:22.04
      - name: cloudinit
        cloudInitNoCloud:
          userData: |
            #cloud-config
            password: tutorial123
            chpasswd: { expire: false }
            ssh_pwauth: true
            packages:
              - nginx
            runcmd:
              - systemctl enable nginx
              - systemctl start nginx
```

Apply it:

```bash
oc apply -f tutorial-vm.yaml
```

### What happens behind the scenes

1. The `kubectl apply` sends a CREATE request to the Kubernetes API
2. The mutating webhook intercepts the request
3. The webhook looks up CUDN `tutorial-network` to get the VPC subnet ID and security group IDs
4. Creates a floating VNI on the subnet (with `allow_ip_spoofing: true`, `enable_infrastructure_nat: false`, `auto_delete: false`)
5. Reads the VPC-generated MAC address and reserved IP from the API response
6. Creates a floating IP (because `vpc.roks.ibm.com/fip: "true"`) and attaches it to the VNI
7. Injects the MAC address into the VM's `interfaces[0].macAddress`
8. Injects the reserved IP into cloud-init network configuration
9. Sets operator annotations (`vni-id`, `mac-address`, `reserved-ip`, `fip-id`, `fip-address`, etc.)
10. Adds the `vpc.roks.ibm.com/vm-cleanup` finalizer
11. Returns the mutated VM to the API server

---

## Step 5: Verify VPC Resources

Check the VM annotations to see all provisioned resources:

```bash
oc get vm tutorial-vm -n tutorial \
  -o jsonpath='{.metadata.annotations}' | jq .
```

Expected output:

```json
{
  "vpc.roks.ibm.com/fip": "true",
  "vpc.roks.ibm.com/vni-id": "r006-xxxx-xxxx-xxxx",
  "vpc.roks.ibm.com/mac-address": "02:00:04:00:ab:cd",
  "vpc.roks.ibm.com/reserved-ip": "10.240.100.5",
  "vpc.roks.ibm.com/reserved-ip-id": "r006-xxxx-xxxx-xxxx",
  "vpc.roks.ibm.com/fip-id": "r006-xxxx-xxxx-xxxx",
  "vpc.roks.ibm.com/fip-address": "169.48.x.x"
}
```

Note the `fip-address` — this is the public IP you will use to connect.

Wait for the VM to be running:

```bash
oc get vmi tutorial-vm -n tutorial
```

Wait until `PHASE` shows `Running` (this may take 1-2 minutes for the container disk to pull and the VM to boot).

---

## Step 6: Connect to the VM

### SSH via Floating IP

```bash
ssh ubuntu@<fip-address>
# Password: tutorial123
```

### Verify networking inside the VM

Once logged in:

```bash
# Check the network interface
ip addr show

# You should see the VPC-assigned IP (10.240.100.x)

# Verify internet connectivity
curl -s ifconfig.me
# Should show the floating IP address

# Check that nginx is running
curl localhost
```

### Alternative: Console access via virtctl

```bash
virtctl console tutorial-vm -n tutorial
```

---

## Step 7: Explore the Console Plugin

If the console plugin is enabled, open the OpenShift Console and navigate to:

1. **Networking > VPC Dashboard** — See the overview of all VPC resources
2. **Networking > VPC Subnets** — Find the `tutorial-network` subnet
3. **Networking > Virtual Network Interfaces** — Find the VNI for `tutorial-vm`
4. **Networking > Floating IPs** — See the floating IP assigned to the VM
5. **Networking > Network Topology** — Visualize the VPC, subnet, and VM relationships

---

## Step 8: Clean Up

### Delete the VM

```bash
oc delete vm tutorial-vm -n tutorial
```

The `vpc.roks.ibm.com/vm-cleanup` finalizer triggers cleanup:
1. Deletes the floating IP
2. Deletes the VNI (which auto-deletes the reserved IP)
3. Removes the finalizer
4. The VM object is deleted from Kubernetes

Verify:

```bash
# VNI should be gone
ibmcloud is virtual-network-interfaces | grep tutorial

# Floating IP should be gone
ibmcloud is floating-ips | grep tutorial
```

### Delete the namespace

```bash
oc delete namespace tutorial
```

### Delete the CUDN

```bash
oc delete cudn tutorial-network
```

The `vpc.roks.ibm.com/cudn-cleanup` finalizer triggers cleanup:
1. Deletes all VLAN attachments on all bare metal nodes
2. Deletes the VPC subnet
3. Removes the finalizer

Verify:

```bash
# Subnet should be gone
ibmcloud is subnets | grep tutorial
```

### (Optional) Delete the security group

```bash
ibmcloud is security-group-delete <sg-id>
```

---

## Summary

In this tutorial you:

1. Planned a network (CIDR, zone, VLAN ID, security groups, ACL)
2. Created a CUDN — the operator provisioned a VPC subnet and VLAN attachments
3. Deployed a VM — the webhook provisioned a VNI, reserved IP, and floating IP
4. Connected via SSH using the floating IP
5. Explored the VPC resources via the console plugin
6. Cleaned up — finalizers deleted all VPC resources automatically

## Next Steps

- [Creating VMs](../user-guide/creating-vms.md) — More VM deployment options
- [Floating IPs](../user-guide/floating-ips.md) — Managing public IP addresses
- [Live Migration](../user-guide/live-migration.md) — VM migration behavior
- [Network Setup](../admin-guide/network-setup.md) — Advanced network configuration
- [Troubleshooting](../troubleshooting.md) — Common issues and solutions
