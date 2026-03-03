# Tutorial: Advanced Multi-Namespace Enterprise Networking

This tutorial builds a realistic enterprise network topology using every major feature of the ROKS VPC Network Operator — multi-namespace isolation, multi-network routing, SNAT/DNAT, IPS inline security, WireGuard VPN, firewall rules, and DHCP reservations. Each part builds on the previous one. By the end, you will have a fully connected, secured, multi-namespace topology with VPN access and verified connectivity at every stage.

## Table of contents

- [Part 1: Prerequisites & network planning](#part-1-prerequisites--network-planning)
- [Part 2: Foundation — L2 networks](#part-2-foundation--l2-networks)
- [Part 3: VMs — workloads on L2 networks](#part-3-vms--workloads-on-l2-networks)
- [Part 4: Router — connecting all L2 networks](#part-4-router--connecting-all-l2-networks)
- [Part 5: Gateway — internet uplink with SNAT](#part-5-gateway--internet-uplink-with-snat)
- [Part 6: Connectivity testing — the foundation works](#part-6-connectivity-testing--the-foundation-works)
- [Part 7: Secure DMZ — DNAT + PAR ingress](#part-7-secure-dmz--dnat--par-ingress)
- [Part 8: IPS inline security](#part-8-ips-inline-security)
- [Part 9: VPN — site-to-site WireGuard](#part-9-vpn--site-to-site-wireguard)
- [Part 10: Split routing verification](#part-10-split-routing-verification)
- [Part 11: Firewall + DHCP reservations](#part-11-firewall--dhcp-reservations)
- [Part 12: Full topology verification & cleanup](#part-12-full-topology-verification--cleanup)

---

## Part 1: Prerequisites & network planning

### Cluster requirements

- A ROKS cluster with **bare metal workers** (VLAN attachments require bare metal network interfaces)
- **OVN-Kubernetes** with the UserDefinedNetwork feature gate enabled
- The **VPC Network Operator** installed via Helm (see [Installation](../getting-started/installation.md))
- A VPC API key stored in a Kubernetes Secret (configured during installation)

### Tools

- `oc` (OpenShift CLI), logged in to the cluster
- `virtctl` (KubeVirt CLI) for VM console/SSH access
- `sshpass` for scripted SSH commands (install via `brew install hudochenkov/sshpass/sshpass` on macOS or `apt install sshpass` on Linux)
- `wg` (WireGuard tools) for key generation
- `ibmcloud` CLI (optional, for verifying VPC resources)

### Target topology

```
                    Internet
                       |
                  FIP + PAR
                       |
            +-------------------+
            |  VPCGateway       |
            |  "adv-gw"        |
            |  uplink=localnet  |
            +--------+----------+
                     |
                     | localnet-ext (VPC subnet)
                     |
            +--------+----------+           +------------------+
            |  VPCRouter        |           |  VPCVPNGateway   |
            |  "adv-router"     |           |  "adv-vpn"       |
            |  (all 4 networks) |           |  WireGuard       |
            +--+-+-+-+----------+           |  192.168.0/24    |
               | | | |                      +------------------+
               | | | |
      +--------+ | | +--------+
      |          | |          |
   app-l2     db-l2     web-l2     svc-l2
  10.100.0   10.100.1  10.200.0   10.200.1
    /24        /24       /24        /24
     |          |         |          |
   VM-1       VM-2      VM-3      VM-4
  (ns-a)     (ns-a)    (ns-b)    (ns-b)
```

A single VPCRouter connects all four workload L2 networks. The router pod gets the gateway's VNI MAC on its uplink interface for VPC fabric connectivity, plus Multus attachments to each workload network. A separate VPCVPNGateway pod handles WireGuard tunnels with its own identity on the uplink.

### Address plan

| Resource | CIDR / Address | Purpose |
|----------|---------------|---------|
| `app-l2` | `10.100.0.0/24` | Application tier (ns-a) |
| `db-l2` | `10.100.1.0/24` | Database tier (ns-a) |
| `web-l2` | `10.200.0.0/24` | Web tier (ns-b) |
| `svc-l2` | `10.200.1.0/24` | Services tier (ns-b) |
| `adv-gw` transit | `10.99.0.1` | Gateway logical address |
| `adv-router` app-l2 | `10.100.0.1/24` | Router on app network |
| `adv-router` db-l2 | `10.100.1.1/24` | Router on db network |
| `adv-router` web-l2 | `10.200.0.1/24` | Router on web network |
| `adv-router` svc-l2 | `10.200.1.1/24` | Router on svc network |
| VPN remote | `192.168.0.0/24` | Simulated remote office |

### VPC resource IDs

Collect the following from your IBM Cloud account:

| Resource | How to find it |
|----------|---------------|
| VPC ID | `ibmcloud is vpcs` |
| Zone | `ibmcloud is zones` (e.g., `eu-de-1`) |
| Security Group ID | `ibmcloud is security-groups` |
| Network ACL ID | `ibmcloud is network-acls` |

### Create namespaces

```bash
oc create namespace ns-a
oc create namespace ns-b
```

---

## Part 2: Foundation — L2 networks

This topology uses four Layer2 CUDNs (pure OVN, no VPC resources) and one LocalNet CUDN (VPC-backed, for the gateway uplink).

### 2.1 Create the workload Layer2 networks

Layer2 CUDNs are pure OVN networks — no VPC subnet, no VLAN attachment. They provide isolated L2 segments for VM workloads. Each CUDN includes both the workload namespace and the operator namespace so that router pods can attach.

```yaml
# adv-l2-networks.yaml
apiVersion: k8s.ovn.org/v1
kind: ClusterUserDefinedNetwork
metadata:
  name: app-l2
spec:
  namespaceSelector:
    matchExpressions:
    - key: kubernetes.io/metadata.name
      operator: In
      values: [ns-a, roks-vpc-network-operator]
  network:
    topology: Layer2
    layer2:
      role: Secondary
      subnets:
      - cidr: "10.100.0.0/24"
---
apiVersion: k8s.ovn.org/v1
kind: ClusterUserDefinedNetwork
metadata:
  name: db-l2
spec:
  namespaceSelector:
    matchExpressions:
    - key: kubernetes.io/metadata.name
      operator: In
      values: [ns-a, roks-vpc-network-operator]
  network:
    topology: Layer2
    layer2:
      role: Secondary
      subnets:
      - cidr: "10.100.1.0/24"
---
apiVersion: k8s.ovn.org/v1
kind: ClusterUserDefinedNetwork
metadata:
  name: web-l2
spec:
  namespaceSelector:
    matchExpressions:
    - key: kubernetes.io/metadata.name
      operator: In
      values: [ns-b, roks-vpc-network-operator]
  network:
    topology: Layer2
    layer2:
      role: Secondary
      subnets:
      - cidr: "10.200.0.0/24"
---
apiVersion: k8s.ovn.org/v1
kind: ClusterUserDefinedNetwork
metadata:
  name: svc-l2
spec:
  namespaceSelector:
    matchExpressions:
    - key: kubernetes.io/metadata.name
      operator: In
      values: [ns-b, roks-vpc-network-operator]
  network:
    topology: Layer2
    layer2:
      role: Secondary
      subnets:
      - cidr: "10.200.1.0/24"
```

```bash
oc apply -f adv-l2-networks.yaml
```

### 2.2 Create the uplink LocalNet CUDN

The uplink network is a LocalNet CUDN — it creates a VPC subnet and VLAN attachments on every bare metal node. The gateway uses this to reach the VPC fabric, and the router pod gets a Multus attachment to it.

```yaml
# adv-localnet.yaml
apiVersion: k8s.ovn.org/v1
kind: ClusterUserDefinedNetwork
metadata:
  name: localnet-ext
  annotations:
    vpc.roks.ibm.com/zone: "eu-de-1"              # VPC zone
    vpc.roks.ibm.com/cidr: "10.240.64.0/24"       # VPC subnet CIDR
    vpc.roks.ibm.com/vpc-id: "<your-vpc-id>"       # VPC ID
    vpc.roks.ibm.com/vlan-id: "300"                # VLAN tag (unused value 1-4094)
    vpc.roks.ibm.com/security-group-ids: "<sg-id>" # Security group(s)
    vpc.roks.ibm.com/acl-id: "<acl-id>"            # Network ACL
spec:
  namespaceSelector:
    matchExpressions:
    - key: kubernetes.io/metadata.name
      operator: In
      values: [roks-vpc-network-operator]
  network:
    topology: LocalNet
    localNet:
      role: Secondary
      subnets:
      - cidr: "10.240.64.0/24"
```

```bash
oc apply -f adv-localnet.yaml
```

**What happens:** The CUDN reconciler creates a VPC subnet (`roks-<cluster>-localnet-ext`) and a VLAN attachment on every bare metal node with `vlan_id: 300`, `allow_to_float: true`.

### 2.3 Verify all networks

```bash
# All 5 CUDNs should appear
oc get cudn

# Expected output:
# NAME           TOPOLOGY   AGE
# app-l2         Layer2     30s
# db-l2          Layer2     30s
# web-l2         Layer2     30s
# svc-l2         Layer2     30s
# localnet-ext   LocalNet   30s

# Verify VPC resources for the LocalNet CUDN
oc get vpcsubnets       # vsn — one VPC subnet
oc get vlanattachments  # vla — one per bare metal node
```

Wait until the VPC subnet status shows `active` (10-30 seconds).

---

## Part 3: VMs — workloads on L2 networks

Each VM connects to a single Layer2 network with `autoattachPodInterface: false` (no pod network). DHCP will come from the router in Part 4 — for now, VMs boot and wait for an IP.

### 3.1 Application VM (ns-a, app-l2)

```yaml
# adv-vm-app.yaml
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: vm-app
  namespace: ns-a
spec:
  running: true
  template:
    spec:
      domain:
        resources:
          requests:
            memory: 2Gi
            cpu: "1"
        devices:
          autoattachPodInterface: false
          interfaces:
          - name: app
            bridge: {}
          disks:
          - name: rootdisk
            disk:
              bus: virtio
      networks:
      - name: app
        multus:
          networkName: app-l2
      volumes:
      - name: rootdisk
        containerDisk:
          image: quay.io/containerdisks/fedora:40
      - name: cloudinit
        cloudInitNoCloud:
          userData: |
            #cloud-config
            password: fedora
            chpasswd: { expire: false }
            ssh_pwauth: true
          networkData: |
            network:
              version: 2
              ethernets:
                enp1s0:
                  dhcp4: true
```

### 3.2 Database VM (ns-a, db-l2)

```yaml
# adv-vm-db.yaml
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: vm-db
  namespace: ns-a
spec:
  running: true
  template:
    spec:
      domain:
        resources:
          requests:
            memory: 2Gi
            cpu: "1"
        devices:
          autoattachPodInterface: false
          interfaces:
          - name: db
            bridge: {}
          disks:
          - name: rootdisk
            disk:
              bus: virtio
      networks:
      - name: db
        multus:
          networkName: db-l2
      volumes:
      - name: rootdisk
        containerDisk:
          image: quay.io/containerdisks/fedora:40
      - name: cloudinit
        cloudInitNoCloud:
          userData: |
            #cloud-config
            password: fedora
            chpasswd: { expire: false }
            ssh_pwauth: true
          networkData: |
            network:
              version: 2
              ethernets:
                enp1s0:
                  dhcp4: true
```

### 3.3 Web VM (ns-b, web-l2)

```yaml
# adv-vm-web.yaml
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: vm-web
  namespace: ns-b
spec:
  running: true
  template:
    spec:
      domain:
        resources:
          requests:
            memory: 2Gi
            cpu: "1"
        devices:
          autoattachPodInterface: false
          interfaces:
          - name: web
            bridge: {}
          disks:
          - name: rootdisk
            disk:
              bus: virtio
      networks:
      - name: web
        multus:
          networkName: web-l2
      volumes:
      - name: rootdisk
        containerDisk:
          image: quay.io/containerdisks/fedora:40
      - name: cloudinit
        cloudInitNoCloud:
          userData: |
            #cloud-config
            password: fedora
            chpasswd: { expire: false }
            ssh_pwauth: true
          networkData: |
            network:
              version: 2
              ethernets:
                enp1s0:
                  dhcp4: true
```

### 3.4 Service VM (ns-b, svc-l2)

```yaml
# adv-vm-svc.yaml
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: vm-svc
  namespace: ns-b
spec:
  running: true
  template:
    spec:
      domain:
        resources:
          requests:
            memory: 2Gi
            cpu: "1"
        devices:
          autoattachPodInterface: false
          interfaces:
          - name: svc
            bridge: {}
          disks:
          - name: rootdisk
            disk:
              bus: virtio
      networks:
      - name: svc
        multus:
          networkName: svc-l2
      volumes:
      - name: rootdisk
        containerDisk:
          image: quay.io/containerdisks/fedora:40
      - name: cloudinit
        cloudInitNoCloud:
          userData: |
            #cloud-config
            password: fedora
            chpasswd: { expire: false }
            ssh_pwauth: true
          networkData: |
            network:
              version: 2
              ethernets:
                enp1s0:
                  dhcp4: true
```

### 3.5 Apply all VMs

```bash
oc apply -f adv-vm-app.yaml -f adv-vm-db.yaml -f adv-vm-web.yaml -f adv-vm-svc.yaml
```

### 3.6 Verify VMs are running

```bash
# Wait for all VMIs to boot
oc get vmi -n ns-a -w
oc get vmi -n ns-b -w

# Expected: all 4 VMs in Running phase
oc get vmi -A -l 'kubevirt.io/domain'
```

The VMs are running but have no IP addresses yet — DHCP service starts when we create the router in the next step.

---

## Part 4: Router — connecting all L2 networks

A single VPCRouter connects all four workload L2 networks and provides DHCP on each.

### How it works

```
        localnet-ext (VPC fabric)
             |
        +----+----------+
        | adv-router     |
        | uplink (VNI MAC from gateway)
        +--+--+--+--+---+
           |  |  |  |
           |  |  |  +--- svc-l2 (10.200.1.0/24)  router: 10.200.1.1
           |  |  +------ web-l2 (10.200.0.0/24)  router: 10.200.0.1
           |  +--------- db-l2  (10.100.1.0/24)  router: 10.100.1.1
           +------------ app-l2 (10.100.0.0/24)  router: 10.100.0.1
```

The router pod gets a Multus attachment to each network. The `uplink` interface uses the gateway's VNI MAC for VPC identity. IP forwarding bridges traffic between workload networks. DHCP serves addresses on each workload network with the router as the default gateway.

### 4.1 Create the router

```yaml
# adv-router.yaml
apiVersion: vpc.roks.ibm.com/v1alpha1
kind: VPCRouter
metadata:
  name: adv-router
  namespace: roks-vpc-network-operator
spec:
  gateway: adv-gw                       # References the gateway (created in Part 5)

  networks:
  - name: app-l2
    namespace: ns-a
    address: "10.100.0.1/24"
  - name: db-l2
    namespace: ns-a
    address: "10.100.1.1/24"
  - name: web-l2
    namespace: ns-b
    address: "10.200.0.1/24"
  - name: svc-l2
    namespace: ns-b
    address: "10.200.1.1/24"

  dhcp:
    enabled: true
    leaseTime: "12h"
    dns:
      nameservers:
      - "8.8.8.8"
      - "8.8.4.4"

  routeAdvertisement:
    connectedSegments: true              # Advertises all 4 network CIDRs
```

> **Note:** The router references `adv-gw` which does not exist yet. Apply the router now — it will enter `Pending` phase and become `Ready` after the gateway is created in Part 5.

```bash
oc apply -f adv-router.yaml
```

**What the reconciler does:**
1. Adds the `vpc.roks.ibm.com/router-cleanup` finalizer
2. Waits for the referenced gateway `adv-gw` to be `Ready`
3. Creates a router pod with Multus attachments: `uplink` (gateway's LocalNet with VNI MAC) + `net0` (app-l2) + `net1` (db-l2) + `net2` (web-l2) + `net3` (svc-l2)
4. Enables IP forwarding between all interfaces
5. Starts dnsmasq for DHCP on each workload interface (net0–net3)
6. Writes `status.advertisedRoutes` with the connected network CIDRs

### 4.2 Verify the router (after gateway creation)

After the gateway is created in Part 5, the router will transition to `Ready`:

```bash
oc get vrt -n roks-vpc-network-operator

# Expected output:
# NAME         GATEWAY   PHASE   SYNC     AGE
# adv-router   adv-gw    Ready   Synced   2m

# Check advertised routes
oc get vrt adv-router -n roks-vpc-network-operator -o jsonpath='{.status.advertisedRoutes}'
# ["10.100.0.0/24","10.100.1.0/24","10.200.0.0/24","10.200.1.0/24"]
```

### 4.3 Verify VMs get DHCP leases

Once the router is running, VMs should acquire IP addresses via DHCP:

```bash
# SSH into vm-app and check IP
virtctl ssh --namespace=ns-a --username=fedora vm-app
# password: fedora

# Inside the VM
ip addr show enp1s0
# Should show an IP in 10.100.0.10-254

# Check the default route points to the router
ip route
# default via 10.100.0.1 dev enp1s0
```

---

## Part 5: Gateway — internet uplink with SNAT

The VPCGateway provisions a VNI on the VPC fabric, allocates a floating IP, and manages VPC routes. It does **not** create a pod — the router pod uses the gateway's VNI MAC on its uplink interface for VPC connectivity. VPC routes are auto-collected from the router's `advertisedRoutes`.

### How it works

```
  Internet
     |
  Floating IP (158.177.x.x)
     |
  VPCGateway "adv-gw"          (VPC resources only, no pod)
  +-----------------------+
  | uplink: localnet-ext  |     VNI + VLAN attachment on VPC subnet
  | transit: 10.99.0.1    |     Logical address for routing
  | NAT: SNAT 10.0/8->FIP|     NAT rules pushed to router pod
  | NAT: noNAT 10/8<->10/8|
  +-----------+-----------+
              |
         localnet-ext
              |
         adv-router pod        (uses gateway's VNI MAC on uplink)
```

### 5.1 Create the gateway

```yaml
# adv-gateway.yaml
apiVersion: vpc.roks.ibm.com/v1alpha1
kind: VPCGateway
metadata:
  name: adv-gw
  namespace: roks-vpc-network-operator
spec:
  zone: "eu-de-1"

  uplink:
    network: localnet-ext
    securityGroupIDs:
    - "<sg-id>"

  transit:
    address: "10.99.0.1"

  floatingIP:
    enabled: true

  nat:
    # NoNAT for all internal traffic — evaluated first (lowest priority number)
    noNAT:
    - source: "10.0.0.0/8"
      destination: "10.0.0.0/8"
      priority: 10

    # SNAT everything else to the floating IP
    snat:
    - source: "10.0.0.0/8"
      priority: 100
```

```bash
oc apply -f adv-gateway.yaml
```

**What the reconciler does:**
1. Adds the `vpc.roks.ibm.com/gateway-cleanup` finalizer
2. Picks a bare metal server and creates a VLAN attachment with an inline VNI
3. Allocates a floating IP and binds it to the VNI
4. Watches the router's status — collects `advertisedRoutes`
5. Creates VPC routes for all 4 L2 CIDRs (`10.100.0.0/24`, `10.100.1.0/24`, `10.200.0.0/24`, `10.200.1.0/24`) with `action: deliver` and `next_hop: <gateway VNI reserved IP>`
6. Sets phase to `Ready`

### 5.2 Verify the gateway

```bash
# Check gateway status
oc get vgw adv-gw -n roks-vpc-network-operator

# Expected output:
# NAME     ZONE      PHASE   VNI IP           FIP              SYNC     AGE
# adv-gw   eu-de-1   Ready   10.240.64.x      158.177.x.x      Synced   30s

# Check VPC routes (auto-collected from router)
oc get vgw adv-gw -n roks-vpc-network-operator -o jsonpath='{.status.vpcRouteIDs}' | jq .

# Verify via IBM Cloud CLI
ibmcloud is vpc-routing-table-routes <vpc-id> <default-rt-id>
# Should show 4 routes: 10.100.0.0/24, 10.100.1.0/24, 10.200.0.0/24, 10.200.1.0/24
```

Now go back and verify the router is Ready (it was waiting for the gateway):

```bash
oc get vrt -n roks-vpc-network-operator

# Router should now be Ready
# NAME         GATEWAY   PHASE   SYNC     AGE
# adv-router   adv-gw    Ready   Synced   5m

# Check router pod is running
oc get pods -n roks-vpc-network-operator -l 'vpc.roks.ibm.com/router'

# Expected:
# NAME               READY   STATUS    AGE
# adv-router-pod      1/1     Running   3m
```

---

## Part 6: Connectivity testing — the foundation works

Now the full data path is functional. Let's verify every traffic pattern.

### 6.1 VM-to-VM same namespace (cross-L2)

VM-1 (app-l2) pinging VM-2 (db-l2) — both on different L2 networks, forwarded by the router.

```
VM-1 (10.100.0.x) --app-l2--> adv-router --db-l2--> VM-2 (10.100.1.x)
```

```bash
# Get VM-2's IP
VM2_IP=$(sshpass -p fedora virtctl ssh --namespace=ns-a --username=fedora \
  -t '-o StrictHostKeyChecking=no' -t '-o UserKnownHostsFile=/dev/null' \
  -t '-o PreferredAuthentications=password' \
  -c 'ip -4 -o addr show enp1s0 | awk "{print \$4}" | cut -d/ -f1' \
  vm-db 2>/dev/null | tr -d '\r')
echo "VM-2 (db) IP: $VM2_IP"

# From VM-1, ping VM-2
sshpass -p fedora virtctl ssh --namespace=ns-a --username=fedora \
  -t '-o StrictHostKeyChecking=no' -t '-o UserKnownHostsFile=/dev/null' \
  -t '-o PreferredAuthentications=password' \
  -c "ping -c 3 $VM2_IP" \
  vm-app
```

### 6.2 VM-to-VM cross namespace

VM-1 (app-l2, ns-a) pinging VM-3 (web-l2, ns-b). Traffic crosses L2 networks through the single router.

```
VM-1 (10.100.0.x) --app-l2--> adv-router --web-l2--> VM-3 (10.200.0.x)
```

```bash
# Get VM-3's IP
VM3_IP=$(sshpass -p fedora virtctl ssh --namespace=ns-b --username=fedora \
  -t '-o StrictHostKeyChecking=no' -t '-o UserKnownHostsFile=/dev/null' \
  -t '-o PreferredAuthentications=password' \
  -c 'ip -4 -o addr show enp1s0 | awk "{print \$4}" | cut -d/ -f1' \
  vm-web 2>/dev/null | tr -d '\r')
echo "VM-3 (web) IP: $VM3_IP"

# From VM-1, ping VM-3 (cross-namespace via router)
sshpass -p fedora virtctl ssh --namespace=ns-a --username=fedora \
  -t '-o StrictHostKeyChecking=no' -t '-o UserKnownHostsFile=/dev/null' \
  -t '-o PreferredAuthentications=password' \
  -c "ping -c 3 $VM3_IP" \
  vm-app
```

### 6.3 VM-to-Internet (SNAT)

VM-1 reaches the internet via SNAT through the gateway's floating IP.

```
VM-1 --app-l2--> adv-router --uplink--> VPC fabric --SNAT--> FIP --> Internet
```

```bash
# From VM-1, test internet connectivity
sshpass -p fedora virtctl ssh --namespace=ns-a --username=fedora \
  -t '-o StrictHostKeyChecking=no' -t '-o UserKnownHostsFile=/dev/null' \
  -t '-o PreferredAuthentications=password' \
  -c "curl -s --max-time 10 ifconfig.me" \
  vm-app

# Output should show the gateway's floating IP
```

### 6.4 Verify from all VMs

Repeat the internet test from each VM to confirm all paths work:

```bash
for NS_VM in "ns-a vm-app" "ns-a vm-db" "ns-b vm-web" "ns-b vm-svc"; do
  NS=$(echo $NS_VM | awk '{print $1}')
  VM=$(echo $NS_VM | awk '{print $2}')
  echo -n "$VM ($NS): "
  sshpass -p fedora virtctl ssh --namespace=$NS --username=fedora \
    -t '-o StrictHostKeyChecking=no' -t '-o UserKnownHostsFile=/dev/null' \
    -t '-o PreferredAuthentications=password' \
    -c "curl -s --max-time 10 ifconfig.me || echo TIMEOUT" \
    $VM 2>/dev/null
done
```

All four VMs should report the same floating IP address.

---

## Part 7: Secure DMZ — DNAT + PAR ingress

Add a Public Address Range (PAR) to the gateway for inbound traffic. DNAT rules forward specific ports to the web-tier VM.

### How it works

```
  Internet
     |
  PAR CIDR (150.240.x.0/29 — 8 IPs)
     |
  Ingress Routing Table
     |
  VPCGateway "adv-gw"
     | DNAT: port 443 -> VM-3:8443
     | DNAT: port 80  -> VM-3:80
     |
  localnet-ext --> adv-router --> web-l2 --> VM-3
```

### 7.1 Update the gateway with PAR and DNAT

First, get VM-3's DHCP-assigned IP:

```bash
VM3_IP=$(sshpass -p fedora virtctl ssh --namespace=ns-b --username=fedora \
  -t '-o StrictHostKeyChecking=no' -t '-o UserKnownHostsFile=/dev/null' \
  -t '-o PreferredAuthentications=password' \
  -c 'ip -4 -o addr show enp1s0 | awk "{print \$4}" | cut -d/ -f1' \
  vm-web 2>/dev/null | tr -d '\r')
echo "VM-3 (web) IP: $VM3_IP"
```

Update the gateway — add `publicAddressRange` and `dnat` rules:

```yaml
# adv-gateway-par.yaml
apiVersion: vpc.roks.ibm.com/v1alpha1
kind: VPCGateway
metadata:
  name: adv-gw
  namespace: roks-vpc-network-operator
spec:
  zone: "eu-de-1"

  uplink:
    network: localnet-ext
    securityGroupIDs:
    - "<sg-id>"

  transit:
    address: "10.99.0.1"

  floatingIP:
    enabled: true

  publicAddressRange:
    enabled: true
    prefixLength: 29                     # 8 public IPs

  nat:
    noNAT:
    - source: "10.0.0.0/8"
      destination: "10.0.0.0/8"
      priority: 10

    snat:
    - source: "10.0.0.0/8"
      priority: 100

    dnat:
    - externalPort: 443                  # HTTPS -> VM-3
      internalAddress: "<VM3_IP>"        # Replace with VM-3's actual IP
      internalPort: 8443
      protocol: tcp
      priority: 50
    - externalPort: 80                   # HTTP -> VM-3
      internalAddress: "<VM3_IP>"        # Replace with VM-3's actual IP
      internalPort: 80
      protocol: tcp
      priority: 51
```

> **Important:** Replace `<VM3_IP>` with the actual IP address from the command above.

```bash
oc apply -f adv-gateway-par.yaml
```

**What the reconciler does:**
1. Creates a PAR in the VPC zone with prefix length /29
2. Creates an ingress routing table with `route_internet_ingress: true`
3. Creates an ingress route: `destination=<PAR CIDR>`, `next_hop=<gateway VNI IP>`
4. The router pod is recreated with updated DNAT nftables rules (the gateway pushes NAT config to the router)

### 7.2 Verify the PAR

```bash
oc get vgw adv-gw -n roks-vpc-network-operator -o wide

# Expected — PAR CIDR column shows the allocated public range:
# NAME     ZONE      PHASE   VNI IP         FIP            PAR CIDR            SYNC     AGE
# adv-gw   eu-de-1   Ready   10.240.64.x    158.177.x.x    150.240.68.0/29     Synced   5m
```

### 7.3 Start a web server on VM-3

```bash
# SSH into VM-3 and start a simple HTTP server
sshpass -p fedora virtctl ssh --namespace=ns-b --username=fedora \
  -t '-o StrictHostKeyChecking=no' -t '-o UserKnownHostsFile=/dev/null' \
  -t '-o PreferredAuthentications=password' \
  -c "nohup python3 -m http.server 80 &>/dev/null &" \
  vm-web
```

### 7.4 Test inbound DNAT

From a VM on a different network (e.g., vm-app), test reaching vm-web through the gateway's FIP:

```bash
# Get the gateway's floating IP
GW_FIP=$(oc get vgw adv-gw -n roks-vpc-network-operator -o jsonpath='{.status.floatingIP}')

# From VM-1, curl VM-3's HTTP server via the gateway's public IP
sshpass -p fedora virtctl ssh --namespace=ns-a --username=fedora \
  -t '-o StrictHostKeyChecking=no' -t '-o UserKnownHostsFile=/dev/null' \
  -t '-o PreferredAuthentications=password' \
  -c "curl -s --max-time 10 http://$GW_FIP:80 || echo TIMEOUT" \
  vm-app
```

From outside the cluster, you can also test directly:

```bash
curl -s --max-time 10 http://$GW_FIP:80
```

---

## Part 8: IPS inline security

Enable inline IPS (intrusion prevention) on the router to protect all workloads. Suricata inspects all forwarded traffic and can block malicious patterns.

### How it works

```
                     adv-router Pod
  +----------------------------------------------------+
  |                                                    |
  |   +------------+         +--------------------+    |
  |   |  router    | NFQUEUE |    suricata         |    |
  |   | container  | ------->|    sidecar (IPS)    |    |
  |   +------------+         +--------------------+    |
  |        |                                           |
  |   traffic flows through NFQUEUE for inspection     |
  |   matched "drop" rules block packets inline        |
  +----------------------------------------------------+
```

### 8.1 Update the router with IPS

```yaml
# adv-router-ips.yaml
apiVersion: vpc.roks.ibm.com/v1alpha1
kind: VPCRouter
metadata:
  name: adv-router
  namespace: roks-vpc-network-operator
spec:
  gateway: adv-gw

  networks:
  - name: app-l2
    namespace: ns-a
    address: "10.100.0.1/24"
  - name: db-l2
    namespace: ns-a
    address: "10.100.1.1/24"
  - name: web-l2
    namespace: ns-b
    address: "10.200.0.1/24"
  - name: svc-l2
    namespace: ns-b
    address: "10.200.1.1/24"

  dhcp:
    enabled: true
    leaseTime: "12h"
    dns:
      nameservers:
      - "8.8.8.8"
      - "8.8.4.4"

  routeAdvertisement:
    connectedSegments: true

  ids:
    enabled: true
    mode: ips                            # Inline blocking (NFQUEUE)
    interfaces: all                      # Monitor all interfaces
    customRules: |
      # Alert on any HTTP traffic to the database subnet
      alert http any any -> 10.100.1.0/24 any (msg:"HTTP to DB subnet"; sid:1000001; rev:1;)
```

```bash
oc apply -f adv-router-ips.yaml
```

The router reconciler detects the IDS configuration change and recreates the pod with a Suricata sidecar.

### 8.2 Verify IPS is active

```bash
# Wait for pod recreation
oc wait --for=condition=Ready pod/adv-router-pod -n roks-vpc-network-operator --timeout=120s

# Pod should show 2/2 containers (router + suricata)
oc get pod adv-router-pod -n roks-vpc-network-operator

# Expected:
# NAME               READY   STATUS    AGE
# adv-router-pod      2/2     Running   30s

# Verify IDS mode in status
oc get vrt adv-router -n roks-vpc-network-operator -o wide
# The IDS column should show "ips"

# Verify NFQUEUE rules on the router container
oc exec adv-router-pod -n roks-vpc-network-operator -c router -- nft list table ip suricata
```

Expected nftables output:

```
table ip suricata {
  chain forward_ips {
    type filter hook forward priority -10; policy accept;
    ct state established,related accept
    queue num 0 bypass
  }
}
```

### 8.3 Trigger an alert

From VM-1, make an HTTP request to trigger the ET Open ruleset:

```bash
sshpass -p fedora virtctl ssh --namespace=ns-a --username=fedora \
  -t '-o StrictHostKeyChecking=no' -t '-o UserKnownHostsFile=/dev/null' \
  -t '-o PreferredAuthentications=password' \
  -c "curl -s http://testmynids.org/uid/index.html" \
  vm-app
```

Check Suricata logs for alerts:

```bash
oc logs adv-router-pod -n roks-vpc-network-operator -c suricata --tail=20 | grep '"alert"'
```

---

## Part 9: VPN — site-to-site WireGuard

Add a WireGuard VPN gateway to provide encrypted connectivity to a simulated remote office network (192.168.0.0/24).

### How it works

```
  Remote Office                    Cluster
  192.168.0.0/24                   10.100.x.x, 10.200.x.x

  WireGuard Peer  <-- encrypted tunnel -->  VPCVPNGateway "adv-vpn"
  203.0.113.10:51820                         <GW FIP>:51820
                                                |
                                           localnet-ext
                                           (own MAC, separate pod)
                                                |
                                    routes: 192.168.0.0/24
                                    via VPN gateway pod
```

The VPN gateway pod gets its own Multus attachment to the localnet-ext network (without MAC pinning — it uses its own VPC identity). The VPCGateway automatically creates a VPC route for `192.168.0.0/24` when the VPN gateway advertises it.

### 9.1 Generate WireGuard keys

```bash
# Generate key pair for the cluster side
wg genkey | tee cluster-private.key | wg pubkey > cluster-public.key

# Generate key pair for the remote side
wg genkey | tee remote-private.key | wg pubkey > remote-public.key

echo "Cluster public key: $(cat cluster-public.key)"
echo "Remote public key:  $(cat remote-public.key)"
```

### 9.2 Create the WireGuard secret

```bash
oc create secret generic adv-wg-key \
  --from-file=privatekey=cluster-private.key \
  -n roks-vpc-network-operator
```

### 9.3 Create the VPCVPNGateway

```yaml
# adv-vpn.yaml
apiVersion: vpc.roks.ibm.com/v1alpha1
kind: VPCVPNGateway
metadata:
  name: adv-vpn
  namespace: roks-vpc-network-operator
spec:
  protocol: wireguard
  gatewayRef: adv-gw

  wireGuard:
    privateKey:
      name: adv-wg-key
      key: privatekey
    listenPort: 51820

  tunnels:
  - name: remote-office
    remoteEndpoint: "203.0.113.10"       # Remote WireGuard peer public IP
    remoteNetworks:
    - "192.168.0.0/24"                   # CIDRs reachable via this tunnel
    peerPublicKey: "<contents of remote-public.key>"
    tunnelAddressLocal: "10.99.1.1/30"   # Inner tunnel address (local)
    tunnelAddressRemote: "10.99.1.2/30"  # Inner tunnel address (remote)

  mtu:
    tunnelMTU: 1420
    mssClamp: true
```

> **Note:** Replace `<contents of remote-public.key>` with the actual base64 public key from step 9.1.

```bash
oc apply -f adv-vpn.yaml
```

**What the reconciler does:**
1. Looks up `adv-gw` to obtain the floating IP as the tunnel endpoint
2. Creates a VPN pod (`vpngw-adv-vpn`) with WireGuard configured using the tunnel spec
3. The pod gets a Multus attachment to `localnet-ext` as its `net0` interface (no MAC pinning — it gets its own identity from VPC DHCP)
4. Sets `status.advertisedRoutes: ["192.168.0.0/24"]`
5. The VPCGateway reconciler detects the new advertised route and creates a VPC route for `192.168.0.0/24`

### 9.4 Configure the remote side

On the remote WireGuard peer (Linux server or appliance):

```ini
# /etc/wireguard/wg0.conf
[Interface]
PrivateKey = <contents of remote-private.key>
Address = 10.99.1.2/30
ListenPort = 51820

[Peer]
PublicKey = <contents of cluster-public.key>
Endpoint = <VPCGateway floating IP>:51820
AllowedIPs = 10.100.0.0/24, 10.100.1.0/24, 10.200.0.0/24, 10.200.1.0/24
PersistentKeepalive = 25
```

```bash
sudo wg-quick up wg0
```

### 9.5 Verify the VPN

```bash
# Check VPN gateway status
oc get vvg adv-vpn -n roks-vpc-network-operator

# Expected:
# NAME      PROTOCOL    GATEWAY   TUNNELS   PHASE   SYNC     AGE
# adv-vpn   wireguard   adv-gw    1         Ready   Synced   30s

# Check the gateway now has a VPC route for 192.168.0.0/24
oc get vgw adv-gw -n roks-vpc-network-operator -o jsonpath='{.status.vpcRouteIDs}' | jq .

# Check WireGuard status inside the VPN pod
VPN_POD=$(oc get pods -n roks-vpc-network-operator -l vpc.roks.ibm.com/vpngateway=adv-vpn -o name)
oc exec $VPN_POD -n roks-vpc-network-operator -- wg show
```

---

## Part 10: Split routing verification

With the full topology in place, traffic from any VM takes a different path depending on the destination. Let's trace each path.

### 10.1 Routing table from VM-1

```bash
# SSH into VM-1 and check routes
sshpass -p fedora virtctl ssh --namespace=ns-a --username=fedora \
  -t '-o StrictHostKeyChecking=no' -t '-o UserKnownHostsFile=/dev/null' \
  -t '-o PreferredAuthentications=password' \
  -c "ip route" \
  vm-app
```

Expected:

```
default via 10.100.0.1 dev enp1s0             # adv-router is the default gateway
10.100.0.0/24 dev enp1s0 proto kernel scope link src 10.100.0.x
```

### 10.2 Traffic path: VM-to-VM cross-network

VM-1 (10.100.0.x) to VM-3 (10.200.0.x) — traffic is forwarded within the router pod between its net0 and net2 interfaces:

```
VM-1 -> 10.100.0.1 (adv-router net0) -> kernel forwarding -> (adv-router net2) -> 10.200.0.x (VM-3)
```

```bash
# Traceroute from VM-1 to VM-3
sshpass -p fedora virtctl ssh --namespace=ns-a --username=fedora \
  -t '-o StrictHostKeyChecking=no' -t '-o UserKnownHostsFile=/dev/null' \
  -t '-o PreferredAuthentications=password' \
  -c "traceroute -n $VM3_IP" \
  vm-app
```

Expected hops:
1. `10.100.0.1` (adv-router — single hop since all networks are on the same router)
2. `10.200.0.x` (VM-3)

### 10.3 Traffic path: VM-to-Internet

VM-1 (10.100.0.x) to the internet — traffic goes to the router, out the uplink to the VPC fabric, SNAT to FIP:

```
VM-1 -> adv-router (net0 -> uplink) -> VPC fabric -> SNAT -> FIP -> Internet
```

```bash
# Traceroute to an internet host
sshpass -p fedora virtctl ssh --namespace=ns-a --username=fedora \
  -t '-o StrictHostKeyChecking=no' -t '-o UserKnownHostsFile=/dev/null' \
  -t '-o PreferredAuthentications=password' \
  -c "traceroute -n 8.8.8.8" \
  vm-app
```

Expected first hops:
1. `10.100.0.1` (adv-router)
2. VPC fabric gateway hops...

### 10.4 Traffic path: VM-to-VPN remote

VM-1 (10.100.0.x) to the remote office (192.168.0.x) — traffic goes through the VPC fabric to the VPN pod:

```
VM-1 -> adv-router -> uplink -> VPC route (192.168.0.0/24) -> VPN pod -> WireGuard tunnel -> 192.168.0.x
```

```bash
# From VM-1, ping the remote office subnet (only works if remote side is connected)
sshpass -p fedora virtctl ssh --namespace=ns-a --username=fedora \
  -t '-o StrictHostKeyChecking=no' -t '-o UserKnownHostsFile=/dev/null' \
  -t '-o PreferredAuthentications=password' \
  -c "ping -c 3 -W 5 192.168.0.1 || echo 'Remote side not reachable (expected if no real remote peer)'" \
  vm-app
```

### 10.5 Routing summary

| Source | Destination | Path | NAT |
|--------|------------|------|-----|
| VM-1 (10.100.0.x) | VM-2 (10.100.1.x) | app-l2 -> router -> db-l2 | None |
| VM-1 (10.100.0.x) | VM-3 (10.200.0.x) | app-l2 -> router -> web-l2 | None |
| VM-1 (10.100.0.x) | Internet | app-l2 -> router -> uplink -> VPC -> SNAT | SNAT to FIP |
| VM-1 (10.100.0.x) | 192.168.0.x | app-l2 -> router -> uplink -> VPC route -> VPN tunnel | None |
| Internet | VM-3:80 | FIP:80 -> router DNAT -> web-l2 | DNAT |

---

## Part 11: Firewall + DHCP reservations

Add firewall rules to control internet access and DHCP reservations for predictable IPs.

### How the firewall works

The router's firewall rules generate nftables chains in the `forward` hook. Rules use `direction: ingress` (maps to `iifname "uplink"` — traffic arriving from the VPC fabric) and `direction: egress` (maps to `oifname "uplink"` — traffic leaving to the VPC fabric). This means the firewall controls traffic between VMs and the internet. Traffic forwarded between workload networks (net0 ↔ net1 ↔ net2 ↔ net3) does not traverse the uplink and is handled by the kernel's forwarding table directly.

> **Note:** For network-level microsegmentation between L2 networks (e.g., preventing db-l2 from reaching web-l2), use OVN Network Policies or Kubernetes NetworkPolicies at the CNI layer.

### 11.1 Get VM-2's MAC address

DHCP reservations require the VM's MAC address:

```bash
VM2_MAC=$(sshpass -p fedora virtctl ssh --namespace=ns-a --username=fedora \
  -t '-o StrictHostKeyChecking=no' -t '-o UserKnownHostsFile=/dev/null' \
  -t '-o PreferredAuthentications=password' \
  -c "ip link show enp1s0 | awk '/ether/ {print \$2}'" \
  vm-db 2>/dev/null | tr -d '\r')
echo "VM-2 (db) MAC: $VM2_MAC"
```

### 11.2 Update the router with firewall and DHCP reservations

```yaml
# adv-router-fw.yaml
apiVersion: vpc.roks.ibm.com/v1alpha1
kind: VPCRouter
metadata:
  name: adv-router
  namespace: roks-vpc-network-operator
spec:
  gateway: adv-gw

  networks:
  - name: app-l2
    namespace: ns-a
    address: "10.100.0.1/24"
  - name: db-l2
    namespace: ns-a
    address: "10.100.1.1/24"
    dhcp:
      reservations:
      - mac: "<VM2_MAC>"                 # Replace with VM-2's actual MAC
        ip: "10.100.1.100"
        hostname: "db-server"
  - name: web-l2
    namespace: ns-b
    address: "10.200.0.1/24"
  - name: svc-l2
    namespace: ns-b
    address: "10.200.1.1/24"

  dhcp:
    enabled: true
    leaseTime: "12h"
    dns:
      nameservers:
      - "8.8.8.8"
      - "8.8.4.4"

  routeAdvertisement:
    connectedSegments: true

  ids:
    enabled: true
    mode: ips
    interfaces: all
    customRules: |
      alert http any any -> 10.100.1.0/24 any (msg:"HTTP to DB subnet"; sid:1000001; rev:1;)

  firewall:
    enabled: true
    rules:
    # Deny database tier from reaching the internet (block outbound)
    - name: deny-db-to-internet
      direction: egress
      action: deny
      source: "10.100.1.0/24"
      protocol: any
      priority: 10

    # Deny inbound SSH from internet to database tier
    - name: deny-internet-ssh-to-db
      direction: ingress
      action: deny
      destination: "10.100.1.0/24"
      protocol: tcp
      port: 22
      priority: 20

    # Deny inbound SSH from internet to services tier
    - name: deny-internet-ssh-to-svc
      direction: ingress
      action: deny
      destination: "10.200.1.0/24"
      protocol: tcp
      port: 22
      priority: 21

    # Allow all other outbound traffic to internet
    - name: allow-all-egress
      direction: egress
      action: allow
      protocol: any
      priority: 999

    # Allow all other inbound traffic from internet
    - name: allow-all-ingress
      direction: ingress
      action: allow
      protocol: any
      priority: 999
```

> **Important:** Replace `<VM2_MAC>` with the actual MAC address from step 11.1.

```bash
oc apply -f adv-router-fw.yaml
```

The router pod is recreated with the new firewall rules and DHCP reservation.

### 11.3 Verify DHCP reservation

After the router pod restarts, renew VM-2's DHCP lease:

```bash
# Force DHCP renewal on VM-2
sshpass -p fedora virtctl ssh --namespace=ns-a --username=fedora \
  -t '-o StrictHostKeyChecking=no' -t '-o UserKnownHostsFile=/dev/null' \
  -t '-o PreferredAuthentications=password' \
  -c "sudo dhclient -r enp1s0 && sudo dhclient enp1s0 && ip addr show enp1s0" \
  vm-db
```

VM-2 should now have IP `10.100.1.100`.

### 11.4 Verify firewall rules

```bash
# Check nftables on the router pod
oc exec adv-router-pod -n roks-vpc-network-operator -c router -- nft list ruleset
```

Expected nftables output (firewall section):

```
table ip filter {
  chain forward {
    type filter hook forward priority 0; policy drop;
    ct state established,related accept
    oifname "uplink" ip saddr 10.100.1.0/24 counter deny
    iifname "uplink" ip daddr 10.100.1.0/24 meta l4proto tcp th dport 22 counter deny
    iifname "uplink" ip daddr 10.200.1.0/24 meta l4proto tcp th dport 22 counter deny
    oifname "uplink" counter allow
    iifname "uplink" counter allow
  }
}
```

### 11.5 Test firewall enforcement

```bash
# Test 1: VM-1 CAN reach the internet (app tier allowed)
sshpass -p fedora virtctl ssh --namespace=ns-a --username=fedora \
  -t '-o StrictHostKeyChecking=no' -t '-o UserKnownHostsFile=/dev/null' \
  -t '-o PreferredAuthentications=password' \
  -c "curl -s --max-time 10 ifconfig.me && echo ' PASS: internet reachable'" \
  vm-app

# Test 2: VM-2 CANNOT reach the internet (db tier blocked)
sshpass -p fedora virtctl ssh --namespace=ns-a --username=fedora \
  -t '-o StrictHostKeyChecking=no' -t '-o UserKnownHostsFile=/dev/null' \
  -t '-o PreferredAuthentications=password' \
  -c "curl -s --max-time 5 ifconfig.me || echo 'PASS: internet blocked for db tier'" \
  vm-db

# Test 3: VM-1 CAN still ping VM-2 (inter-L2 traffic bypasses the firewall)
sshpass -p fedora virtctl ssh --namespace=ns-a --username=fedora \
  -t '-o StrictHostKeyChecking=no' -t '-o UserKnownHostsFile=/dev/null' \
  -t '-o PreferredAuthentications=password' \
  -c "ping -c 2 -W 3 10.100.1.100 && echo 'PASS: inter-L2 connectivity preserved'" \
  vm-app

# Test 4: VM-3 CAN reach the internet (web tier allowed)
sshpass -p fedora virtctl ssh --namespace=ns-b --username=fedora \
  -t '-o StrictHostKeyChecking=no' -t '-o UserKnownHostsFile=/dev/null' \
  -t '-o PreferredAuthentications=password' \
  -c "curl -s --max-time 10 ifconfig.me && echo ' PASS: internet reachable'" \
  vm-web
```

---

## Part 12: Full topology verification & cleanup

### 12.1 Final connectivity matrix

| Source | Destination | Expected | Test command |
|--------|------------|----------|-------------|
| VM-1 (app) | VM-2 (db) ICMP | Allow | `ping -c 1 10.100.1.100` |
| VM-1 (app) | VM-3 (web) ICMP | Allow | `ping -c 1 <VM3_IP>` |
| VM-1 (app) | Internet | Allow (SNAT) | `curl -s ifconfig.me` |
| VM-2 (db) | Internet | Deny (firewall) | `curl -s --max-time 5 ifconfig.me` |
| VM-3 (web) | Internet | Allow (SNAT) | `curl -s ifconfig.me` |
| Internet | VM-3:80 | Allow (DNAT) | `curl http://<GW_FIP>:80` |

### 12.2 Resource health check

```bash
# All resources should be Ready/Synced
oc get vrt,vgw,vvg -n roks-vpc-network-operator

# Expected:
# NAME                                    GATEWAY   PHASE   SYNC     AGE
# vpcrouter.vpc.roks.ibm.com/adv-router   adv-gw    Ready   Synced   20m
#
# NAME                              ZONE      PHASE   VNI IP         FIP            SYNC     AGE
# vpcgateway.vpc.roks.ibm.com/adv-gw   eu-de-1   Ready   10.240.64.x    158.177.x.x    Synced   18m
#
# NAME                                   PROTOCOL    GATEWAY   TUNNELS   PHASE   SYNC     AGE
# vpcvpngateway.vpc.roks.ibm.com/adv-vpn   wireguard   adv-gw    1         Ready   Synced   10m

# Check all pods are healthy
oc get pods -n roks-vpc-network-operator -l 'vpc.roks.ibm.com/router'
oc get pods -n roks-vpc-network-operator -l 'vpc.roks.ibm.com/vpngateway'

# Check all VMs are running
oc get vmi -A
```

### 12.3 Topology overview

View the full topology in the OpenShift Console plugin:

1. Navigate to **Networking > VPC Networking > Topology**
2. The topology view shows all resources: gateway, router, VPN gateway, networks, and VMs
3. Connections between resources show the data path
4. Click any resource to see its status and configuration

### 12.4 Cleanup

Delete resources in reverse dependency order to ensure finalizers clean up properly:

```bash
# 1. Delete VMs (no dependencies)
oc delete vm vm-app vm-db -n ns-a
oc delete vm vm-web vm-svc -n ns-b

# Wait for VMs to terminate
oc wait --for=delete vmi/vm-app vmi/vm-db -n ns-a --timeout=120s
oc wait --for=delete vmi/vm-web vmi/vm-svc -n ns-b --timeout=120s

# 2. Delete VPN gateway (depends on gateway)
oc delete vpcvpngateway adv-vpn -n roks-vpc-network-operator

# 3. Delete router (depends on gateway)
oc delete vpcrouter adv-router -n roks-vpc-network-operator

# Wait for router pod to terminate
oc wait --for=delete pod/adv-router-pod -n roks-vpc-network-operator --timeout=120s

# 4. Delete gateway (finalizer cleans up FIP, PAR, VPC routes, VLAN attachment)
oc delete vpcgateway adv-gw -n roks-vpc-network-operator

# 5. Delete WireGuard secret
oc delete secret adv-wg-key -n roks-vpc-network-operator

# 6. Delete CUDNs (LocalNet finalizer cleans up VPC subnet + VLAN attachments)
oc delete cudn localnet-ext app-l2 db-l2 web-l2 svc-l2

# 7. Delete namespaces
oc delete namespace ns-a ns-b
```

### 12.5 Verify cleanup

```bash
# No operator resources should remain
oc get vrt,vgw,vvg,vsn,vla -n roks-vpc-network-operator
# No resources found

# No CUDNs from this tutorial
oc get cudn | grep -E 'app-l2|db-l2|web-l2|svc-l2|localnet-ext'
# (no output)

# Optionally verify VPC resources are cleaned up
ibmcloud is subnets | grep roks-
ibmcloud is floating-ips | grep roks-
```
