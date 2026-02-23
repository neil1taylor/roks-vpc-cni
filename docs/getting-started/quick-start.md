# Quick Start

Deploy your first VM with VPC networking in 10 minutes. This guide assumes you have already completed [Prerequisites](prerequisites.md) and [Installation](installation.md).

---

## Step 1: Create a Network (CUDN)

Create a `ClusterUserDefinedNetwork` with VPC annotations. Replace the placeholder values with your VPC resource IDs.

```yaml
# cudn-quickstart.yaml
apiVersion: k8s.ovn.org/v1
kind: ClusterUserDefinedNetwork
metadata:
  name: quickstart-network
  annotations:
    vpc.roks.ibm.com/zone: "us-south-1"
    vpc.roks.ibm.com/cidr: "10.240.64.0/24"
    vpc.roks.ibm.com/vpc-id: "<your-vpc-id>"
    vpc.roks.ibm.com/vlan-id: "100"
    vpc.roks.ibm.com/security-group-ids: "<your-security-group-id>"
    vpc.roks.ibm.com/acl-id: "<your-acl-id>"
spec:
  topology: LocalNet
```

Apply it:

```bash
oc apply -f cudn-quickstart.yaml
```

The operator will:
1. Validate the annotations
2. Create a VPC subnet (`10.240.64.0/24`) in the specified zone
3. Create VLAN attachments on every bare metal node

Check the status:

```bash
oc get cudn quickstart-network -o yaml | grep 'vpc.roks.ibm.com'
```

Wait until you see `vpc.roks.ibm.com/subnet-status: active`.

## Step 2: Deploy a VM

Create a VirtualMachine that references the CUDN network. This example deploys an Ubuntu VM with a floating IP for SSH access.

```yaml
# vm-quickstart.yaml
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: quickstart-vm
  namespace: default
  annotations:
    vpc.roks.ibm.com/fip: "true"
spec:
  running: true
  template:
    spec:
      networks:
      - name: vpc-net
        multus:
          networkName: quickstart-network
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
            password: changeme
            chpasswd: { expire: false }
            ssh_pwauth: true
```

Apply it:

```bash
oc apply -f vm-quickstart.yaml
```

The mutating webhook will automatically:
1. Create a VNI on the VPC subnet
2. Read the VPC-generated MAC address and reserved IP
3. Inject the MAC into the VM interface spec
4. Inject the IP into cloud-init network config
5. Create and attach a floating IP (because `vpc.roks.ibm.com/fip: "true"`)
6. Add operator annotations and a cleanup finalizer

## Step 3: Verify VPC Resources

Check the annotations on the VM to see the provisioned VPC resources:

```bash
oc get vm quickstart-vm -o jsonpath='{.metadata.annotations}' | jq .
```

You should see:

```json
{
  "vpc.roks.ibm.com/fip": "true",
  "vpc.roks.ibm.com/vni-id": "r006-...",
  "vpc.roks.ibm.com/mac-address": "02:00:04:00:ab:cd",
  "vpc.roks.ibm.com/reserved-ip": "10.240.64.12",
  "vpc.roks.ibm.com/reserved-ip-id": "r006-...",
  "vpc.roks.ibm.com/fip-id": "r006-...",
  "vpc.roks.ibm.com/fip-address": "169.48.x.x"
}
```

## Step 4: Connect to the VM

Wait for the VM to be running:

```bash
oc get vmi quickstart-vm
```

Once `PHASE` shows `Running`, SSH in using the floating IP:

```bash
ssh ubuntu@<fip-address>
# Password: changeme
```

You can also use `virtctl` to access the console:

```bash
virtctl console quickstart-vm
```

## Step 5: Clean Up

Delete the VM (finalizers will clean up VNI, reserved IP, and floating IP):

```bash
oc delete vm quickstart-vm
```

Delete the network (ensure no VMs are using it first):

```bash
oc delete cudn quickstart-network
```

The operator will delete the VLAN attachments and VPC subnet automatically.

---

## What Just Happened?

Here is what the operator did behind the scenes:

```
You applied CUDN                  Operator created:
  quickstart-network         ──►    VPC Subnet (10.240.64.0/24)
                                    VLAN Attachments (one per BM node)

You applied VM                    Webhook created:
  quickstart-vm              ──►    VNI (MAC + reserved IP)
  (with fip: "true")                Floating IP (public address)
                                    Injected MAC + IP into VM spec

You deleted VM                    Finalizer deleted:
  quickstart-vm              ──►    Floating IP
                                    VNI (+ reserved IP)

You deleted CUDN                  Finalizer deleted:
  quickstart-network         ──►    VLAN Attachments
                                    VPC Subnet
```

## Next Steps

- [Creating VMs](../user-guide/creating-vms.md) — Detailed VM deployment guide
- [Floating IPs](../user-guide/floating-ips.md) — Managing public IP addresses
- [Network Setup](../admin-guide/network-setup.md) — Advanced network configuration
- [End-to-End Tutorial](../tutorials/end-to-end-vm-deployment.md) — Comprehensive walkthrough
