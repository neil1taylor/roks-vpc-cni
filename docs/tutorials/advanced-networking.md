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
- [Part 13: Multi-tenant architecture — per-BU gateway isolation](#part-13-multi-tenant-architecture--per-bu-gateway-isolation)
- [Part 14: Per-tenant egress and ingress](#part-14-per-tenant-egress-and-ingress)
- [Part 15: Tenant isolation verification and RBAC](#part-15-tenant-isolation-verification-and-rbac)
- [Part 16: Multi-tenant cleanup](#part-16-multi-tenant-cleanup)

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

### Enterprise multi-tenant extension

Parts 1-12 build a **single-team** topology: one shared gateway and router serving all networks. Parts 13-16 extend this into a **multi-tenant** architecture where each business unit gets its own isolated gateway, router, egress IP, and RBAC boundary.

| Concern | Shared (Parts 1-12) | Multi-Tenant (Parts 13-16) |
|---------|---------------------|---------------------------|
| Gateway | One `adv-gw` | Per-BU: `gw-bu-a`, `gw-bu-b` |
| Router | One `adv-router` | Per-BU: `router-bu-a`, `router-bu-b` |
| Egress IP | Shared FIP | Per-BU FIPs |
| Blast radius | All networks | Isolated per BU |
| RBAC | Cluster-wide | Namespace-scoped |
| VPN | Shared `adv-vpn` | Per-BU VPN gateways |

You can complete Parts 1-12 independently. Parts 13-16 build on that foundation by replacing the shared infrastructure with per-tenant pairs.

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

The VPN gateway pod gets a Multus attachment to the localnet-ext network with the gateway VNI's MAC address pinned — the same pattern used by the router pod. It configures a static reserved IP and policy routing so return traffic exits via net0. The VPCGateway automatically creates a VPC route for `192.168.0.0/24` when the VPN gateway advertises it.

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
3. The pod gets a Multus attachment to `localnet-ext` as its `net0` interface with the gateway VNI MAC pinned, and configures a static reserved IP with policy routing for return traffic
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

> **Note:** To continue to Part 13 (Multi-Tenant Architecture), **skip this cleanup**. Parts 13-16 replace the shared gateway and router with per-BU pairs, reusing the existing CUDNs and VMs.

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

---

## Part 13: Multi-tenant architecture — per-BU gateway isolation

Parts 1-12 built a single-team topology with one shared gateway and router. In this part, you replace them with **per-business-unit pairs** — each BU gets its own VPCGateway, VPCRouter, Floating IP, and VPC route table entries. The existing CUDNs and VMs remain untouched.

### 13.1 Multi-tenant topology

```
                    Internet
                   /        \
              FIP-A          FIP-B
                |              |
          +-----------+  +-----------+
          | gw-bu-a   |  | gw-bu-b   |
          | NAT: 10.100|  | NAT: 10.200|
          +-----+-----+  +-----+-----+
                |              |
           localnet-ext   localnet-ext   (shared VPC subnet, separate VNIs)
                |              |
          +-----+-----+  +-----+-----+
          |router-bu-a |  |router-bu-b |
          | app-l2     |  | web-l2     |
          | db-l2      |  | svc-l2     |
          +--+----+----+  +--+----+----+
             |    |          |    |
          app-l2 db-l2    web-l2 svc-l2
           VM-1  VM-2     VM-3  VM-4
          (ns-a) (ns-a)  (ns-b) (ns-b)
```

Each gateway creates its own VNI on the shared `localnet-ext` VPC subnet. The VPC fabric sees two distinct identities with separate FIPs and route entries. Routers inherit their gateway's VNI MAC, so BU-A traffic never touches BU-B's VPC identity.

### 13.2 Multi-tenant address plan

| Resource | Owner | CIDR / Address | Purpose |
|----------|-------|---------------|---------|
| `gw-bu-a` transit | BU-A | `10.99.0.1` | Gateway-A logical address |
| `gw-bu-b` transit | BU-B | `10.99.0.5` | Gateway-B logical address |
| `router-bu-a` app-l2 | BU-A | `10.100.0.1/24` | Router-A on app network |
| `router-bu-a` db-l2 | BU-A | `10.100.1.1/24` | Router-A on db network |
| `router-bu-b` web-l2 | BU-B | `10.200.0.1/24` | Router-B on web network |
| `router-bu-b` svc-l2 | BU-B | `10.200.1.1/24` | Router-B on svc network |
| BU-A NAT scope | BU-A | `10.100.0.0/16` | Only BU-A traffic gets SNAT via FIP-A |
| BU-B NAT scope | BU-B | `10.200.0.0/16` | Only BU-B traffic gets SNAT via FIP-B |

### 13.3 Delete shared infrastructure

Remove the shared VPN gateway, router, and gateway from Parts 1-12. The CUDNs and VMs stay.

```bash
# Delete VPN gateway first (depends on gateway)
oc delete vpcvpngateway adv-vpn -n roks-vpc-network-operator --wait

# Delete shared router
oc delete vpcrouter adv-router -n roks-vpc-network-operator --wait

# Wait for router pod to terminate
oc wait --for=delete pod/adv-router-pod -n roks-vpc-network-operator --timeout=120s

# Delete shared gateway (finalizer cleans up FIP, PAR, VPC routes)
oc delete vpcgateway adv-gw -n roks-vpc-network-operator --wait

# Delete WireGuard secret (will recreate per-BU if needed)
oc delete secret adv-wg-key -n roks-vpc-network-operator --ignore-not-found
```

Verify the shared infrastructure is gone:

```bash
oc get vgw,vrt,vvg -n roks-vpc-network-operator
# No resources found

# CUDNs and VMs should still exist
oc get cudn
# localnet-ext, app-l2, db-l2, web-l2, svc-l2

oc get vm -A
# ns-a: vm-app, vm-db
# ns-b: vm-web, vm-svc
```

### 13.4 Create BU-A gateway

```yaml
apiVersion: vpc.roks.ibm.com/v1alpha1
kind: VPCGateway
metadata:
  name: gw-bu-a
  namespace: roks-vpc-network-operator
spec:
  zone: eu-de-1                     # adjust for your zone
  uplink:
    network: localnet-ext
  transit:
    address: 10.99.0.1
    cidr: 10.100.0.0/16             # NAT scope: only BU-A subnets
  floatingIP:
    enabled: true
  nat:
    snat:
      - source: 10.100.0.0/16       # BU-A workload subnets
  firewall:
    enabled: true
    rules: []                        # open by default, tighten in Part 14
```

```bash
oc apply -f - <<'EOF'
apiVersion: vpc.roks.ibm.com/v1alpha1
kind: VPCGateway
metadata:
  name: gw-bu-a
  namespace: roks-vpc-network-operator
spec:
  zone: eu-de-1
  uplink:
    network: localnet-ext
  transit:
    address: 10.99.0.1
    cidr: 10.100.0.0/16
  floatingIP:
    enabled: true
  nat:
    snat:
      - source: 10.100.0.0/16
  firewall:
    enabled: true
    rules: []
EOF
```

### 13.5 Create BU-B gateway

```bash
oc apply -f - <<'EOF'
apiVersion: vpc.roks.ibm.com/v1alpha1
kind: VPCGateway
metadata:
  name: gw-bu-b
  namespace: roks-vpc-network-operator
spec:
  zone: eu-de-1
  uplink:
    network: localnet-ext
  transit:
    address: 10.99.0.5
    cidr: 10.200.0.0/16
  floatingIP:
    enabled: true
  nat:
    snat:
      - source: 10.200.0.0/16
  firewall:
    enabled: true
    rules: []
EOF
```

Wait for both gateways to become Ready:

```bash
oc get vgw -n roks-vpc-network-operator -w
# NAME       ZONE      PHASE   VNI IP         FIP            SYNC     AGE
# gw-bu-a    eu-de-1   Ready   10.240.64.x    158.177.x.x    Synced   30s
# gw-bu-b    eu-de-1   Ready   10.240.64.y    161.156.y.y    Synced   20s
```

Note the two **separate FIPs** — this is the key to per-tenant egress identity.

### 13.6 Create BU-A router

BU-A's router connects only to `app-l2` and `db-l2`. It references `gw-bu-a` and advertises only BU-A subnets.

```bash
oc apply -f - <<'EOF'
apiVersion: vpc.roks.ibm.com/v1alpha1
kind: VPCRouter
metadata:
  name: router-bu-a
  namespace: roks-vpc-network-operator
spec:
  gateway: gw-bu-a
  networks:
    - name: app-l2
      address: 10.100.0.1/24
      dhcp:
        enabled: true
        range: 10.100.0.100-10.100.0.200
        dns:
          - 8.8.8.8
    - name: db-l2
      address: 10.100.1.1/24
      dhcp:
        enabled: true
        range: 10.100.1.100-10.100.1.200
        dns:
          - 8.8.8.8
  dhcp:
    enabled: true
  routeAdvertisement:
    connectedSegments: true
EOF
```

### 13.7 Create BU-B router

BU-B's router connects only to `web-l2` and `svc-l2`, referencing `gw-bu-b`.

```bash
oc apply -f - <<'EOF'
apiVersion: vpc.roks.ibm.com/v1alpha1
kind: VPCRouter
metadata:
  name: router-bu-b
  namespace: roks-vpc-network-operator
spec:
  gateway: gw-bu-b
  networks:
    - name: web-l2
      address: 10.200.0.1/24
      dhcp:
        enabled: true
        range: 10.200.0.100-10.200.0.200
        dns:
          - 8.8.8.8
    - name: svc-l2
      address: 10.200.1.1/24
      dhcp:
        enabled: true
        range: 10.200.1.100-10.200.1.200
        dns:
          - 8.8.8.8
  dhcp:
    enabled: true
  routeAdvertisement:
    connectedSegments: true
EOF
```

### 13.8 Verify multi-tenant infrastructure

```bash
# Both gateways Ready with separate FIPs
oc get vgw -n roks-vpc-network-operator
# NAME       ZONE      PHASE   VNI IP         FIP            SYNC     AGE
# gw-bu-a    eu-de-1   Ready   10.240.64.x    158.177.x.x    Synced   2m
# gw-bu-b    eu-de-1   Ready   10.240.64.y    161.156.y.y    Synced   2m

# Both routers Ready, each referencing its own gateway
oc get vrt -n roks-vpc-network-operator
# NAME           GATEWAY    PHASE   SYNC     AGE
# router-bu-a    gw-bu-a    Ready   Synced   1m
# router-bu-b    gw-bu-b    Ready   Synced   1m

# Router pods are running
oc get pods -n roks-vpc-network-operator -l 'vpc.roks.ibm.com/router'
# router-bu-a-pod   1/1   Running
# router-bu-b-pod   1/1   Running

# VMs should pick up DHCP from their new routers
# (VMs may need a DHCP renewal — reboot if addresses don't appear within 60s)
```

Verify VMs got DHCP addresses from their respective routers:

```bash
# BU-A VMs (should have 10.100.x addresses)
sshpass -p fedora virtctl ssh --username=fedora \
  -t '-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o PreferredAuthentications=password' \
  -c 'ip -4 addr show eth0 | grep inet' vm/vm-app -n ns-a

# BU-B VMs (should have 10.200.x addresses)
sshpass -p fedora virtctl ssh --username=fedora \
  -t '-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o PreferredAuthentications=password' \
  -c 'ip -4 addr show eth0 | grep inet' vm/vm-web -n ns-b
```

---

## Part 14: Per-tenant egress and ingress

With separate gateways and FIPs, each business unit now has its own egress identity and can be configured independently for ingress, firewall, IPS, and VPN.

### 14.1 Verify per-tenant egress

Each BU's outbound traffic exits through its own FIP. This is critical for compliance audit trails — you can attribute every outbound connection to a specific business unit.

```bash
# Record each gateway's FIP
FIP_A=$(oc get vgw gw-bu-a -n roks-vpc-network-operator -o jsonpath='{.status.floatingIP}')
FIP_B=$(oc get vgw gw-bu-b -n roks-vpc-network-operator -o jsonpath='{.status.floatingIP}')
echo "BU-A FIP: $FIP_A"
echo "BU-B FIP: $FIP_B"

# BU-A VM egress — should show FIP_A
sshpass -p fedora virtctl ssh --username=fedora \
  -t '-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o PreferredAuthentications=password' \
  -c 'curl -s ifconfig.me' vm/vm-app -n ns-a
# Expected: 158.177.x.x (FIP_A)

# BU-B VM egress — should show FIP_B
sshpass -p fedora virtctl ssh --username=fedora \
  -t '-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o PreferredAuthentications=password' \
  -c 'curl -s ifconfig.me' vm/vm-web -n ns-b
# Expected: 161.156.y.y (FIP_B)
```

The two FIPs are different — BU-A and BU-B have **completely separate egress identities**.

### 14.2 Per-tenant DNAT ingress

Add DNAT to BU-B's gateway so VM-3 (web tier) is reachable from the internet. BU-A's gateway is unaffected.

```bash
oc patch vpcgateway gw-bu-b -n roks-vpc-network-operator --type merge -p '
spec:
  publicAddressRange:
    enabled: true
  nat:
    snat:
      - source: 10.200.0.0/16
    dnat:
      - externalPort: 80
        internalAddress: 10.200.0.100
        internalPort: 80
        protocol: tcp
'
```

Start a web server on VM-3 (if not already running from Part 7):

```bash
sshpass -p fedora virtctl ssh --username=fedora \
  -t '-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o PreferredAuthentications=password' \
  -c 'echo "BU-B Web Server" | sudo tee /var/www/html/index.html && sudo python3 -m http.server 80 &' \
  vm/vm-web -n ns-b
```

Test DNAT from outside the cluster:

```bash
# Get BU-B's FIP
FIP_B=$(oc get vgw gw-bu-b -n roks-vpc-network-operator -o jsonpath='{.status.floatingIP}')
curl -s http://$FIP_B:80
# Expected: BU-B Web Server

# BU-A's FIP should NOT have DNAT — this should timeout or refuse
FIP_A=$(oc get vgw gw-bu-a -n roks-vpc-network-operator -o jsonpath='{.status.floatingIP}')
curl -s --max-time 5 http://$FIP_A:80
# Expected: connection timeout (no DNAT configured on gw-bu-a)
```

### 14.3 Per-tenant firewall

Block BU-A's database VMs from reaching the internet (compliance requirement) while leaving BU-B completely open.

```bash
oc patch vpcrouter router-bu-a -n roks-vpc-network-operator --type merge -p '
spec:
  firewall:
    enabled: true
    rules:
      - name: allow-dns
        source: 10.100.1.0/24
        destination: 0.0.0.0/0
        destinationPort: "53"
        protocol: udp
        action: accept
      - name: block-db-internet
        source: 10.100.1.0/24
        destination: 0.0.0.0/0
        protocol: tcp
        action: drop
'
```

Verify:

```bash
# BU-A db VM (vm-db) — internet should be blocked
sshpass -p fedora virtctl ssh --username=fedora \
  -t '-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o PreferredAuthentications=password' \
  -c 'curl -s --max-time 5 ifconfig.me; echo "exit: $?"' vm/vm-db -n ns-a
# Expected: timeout, exit: 28

# BU-A app VM (vm-app) — internet should still work
sshpass -p fedora virtctl ssh --username=fedora \
  -t '-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o PreferredAuthentications=password' \
  -c 'curl -s ifconfig.me' vm/vm-app -n ns-a
# Expected: 158.177.x.x (FIP_A)

# BU-B VMs — completely unaffected by BU-A's firewall
sshpass -p fedora virtctl ssh --username=fedora \
  -t '-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o PreferredAuthentications=password' \
  -c 'curl -s ifconfig.me' vm/vm-svc -n ns-b
# Expected: 161.156.y.y (FIP_B)
```

### 14.4 Per-tenant IPS

Enable Suricata IPS on BU-A's router only. BU-B runs without IPS overhead.

```bash
oc patch vpcrouter router-bu-a -n roks-vpc-network-operator --type merge -p '
spec:
  ids:
    enabled: true
    mode: ips
    interfaces: all
'
```

Verify the IPS sidecar is running on BU-A's router but not BU-B's:

```bash
# BU-A router should have 2 containers (router + suricata)
oc get pod router-bu-a-pod -n roks-vpc-network-operator -o jsonpath='{.spec.containers[*].name}'
# Expected: router suricata

# BU-B router should have 1 container (router only)
oc get pod router-bu-b-pod -n roks-vpc-network-operator -o jsonpath='{.spec.containers[*].name}'
# Expected: router
```

### 14.5 Per-tenant VPN

Create a WireGuard VPN for BU-A only. BU-B has no VPN connectivity.

Generate a WireGuard key pair for BU-A:

```bash
wg genkey | tee /dev/stderr | wg pubkey
# Save the private key and public key

# Create the secret
oc create secret generic wg-bu-a-key \
  --from-literal=private-key='<BU_A_PRIVATE_KEY>' \
  -n roks-vpc-network-operator
```

Create BU-A's VPN gateway:

```bash
oc apply -f - <<'EOF'
apiVersion: vpc.roks.ibm.com/v1alpha1
kind: VPCVPNGateway
metadata:
  name: vpn-bu-a
  namespace: roks-vpc-network-operator
spec:
  protocol: wireguard
  gatewayRef: gw-bu-a
  wireGuard:
    privateKey:
      name: wg-bu-a-key
      key: private-key
    listenPort: 51820
  tunnels:
    - name: bu-a-remote-office
      remoteEndpoint: 203.0.113.10
      remoteNetworks:
        - 192.168.100.0/24
      peerPublicKey: <REMOTE_PUBLIC_KEY>
      tunnelAddressLocal: 10.98.0.1/30
      tunnelAddressRemote: 10.98.0.2/30
  localNetworks:
    - cidr: 10.100.0.0/16
EOF
```

Verify:

```bash
# BU-A VPN gateway should be Ready
oc get vvg -n roks-vpc-network-operator
# NAME        PROTOCOL    GATEWAY    TUNNELS   PHASE   SYNC     AGE
# vpn-bu-a    wireguard   gw-bu-a    1         Ready   Synced   30s

# BU-A gateway should now have VPN routes
oc get vgw gw-bu-a -n roks-vpc-network-operator -o jsonpath='{.status.vpcRoutes}' | jq .
# Should include route to 192.168.100.0/24 via gw-bu-a's VNI

# BU-B gateway should have NO VPN routes
oc get vgw gw-bu-b -n roks-vpc-network-operator -o jsonpath='{.status.vpcRoutes}' | jq .
# Should only have workload subnet routes (10.200.x)
```

---

## Part 15: Tenant isolation verification and RBAC

### 15.1 Network isolation

The most important property of multi-tenant networking: **BU-A VMs cannot reach BU-B VMs**, and vice versa. This isolation comes from each router only having interfaces on its own BU's networks.

```bash
# Get VM IPs
APP_IP=$(sshpass -p fedora virtctl ssh --username=fedora \
  -t '-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o PreferredAuthentications=password' \
  -c 'hostname -I | awk "{print \$1}"' vm/vm-app -n ns-a 2>/dev/null | tail -1)

WEB_IP=$(sshpass -p fedora virtctl ssh --username=fedora \
  -t '-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o PreferredAuthentications=password' \
  -c 'hostname -I | awk "{print \$1}"' vm/vm-web -n ns-b 2>/dev/null | tail -1)

echo "VM-App (BU-A): $APP_IP"
echo "VM-Web (BU-B): $WEB_IP"

# BU-A → BU-B: MUST FAIL
sshpass -p fedora virtctl ssh --username=fedora \
  -t '-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o PreferredAuthentications=password' \
  -c "ping -c 2 -W 3 $WEB_IP; echo exit:\$?" vm/vm-app -n ns-a
# Expected: 100% packet loss, exit:1

# BU-B → BU-A: MUST FAIL
sshpass -p fedora virtctl ssh --username=fedora \
  -t '-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o PreferredAuthentications=password' \
  -c "ping -c 2 -W 3 $APP_IP; echo exit:\$?" vm/vm-web -n ns-b
# Expected: 100% packet loss, exit:1
```

Why this works: `router-bu-a` has no interface on `web-l2` or `svc-l2`, so it has no route to `10.200.x.x`. The packet is dropped at the routing layer — there is no path between BU-A and BU-B at L3.

### 15.2 VPC route isolation

Each gateway creates VPC routes only for its own tenant's subnets. Verify the VPC route tables are separate:

```bash
# BU-A gateway routes — should only contain 10.100.x subnets
oc get vgw gw-bu-a -n roks-vpc-network-operator -o jsonpath='{.status.vpcRoutes[*].destination}'
# Expected: 10.100.0.0/24 10.100.1.0/24

# BU-B gateway routes — should only contain 10.200.x subnets
oc get vgw gw-bu-b -n roks-vpc-network-operator -o jsonpath='{.status.vpcRoutes[*].destination}'
# Expected: 10.200.0.0/24 10.200.1.0/24
```

This means even at the VPC fabric level, traffic for BU-A's subnets is routed to BU-A's VNI, and traffic for BU-B's subnets is routed to BU-B's VNI. There is no cross-contamination.

### 15.3 RBAC — namespace-scoped access

Create Kubernetes RBAC so BU-A operators can only manage their own gateway and router, and BU-B operators can only manage theirs.

```bash
# BU-A Role — scoped to gw-bu-a and router-bu-a by resourceNames
oc apply -f - <<'EOF'
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: bu-a-network-admin
  namespace: roks-vpc-network-operator
rules:
  - apiGroups: ["vpc.roks.ibm.com"]
    resources: ["vpcgateways"]
    resourceNames: ["gw-bu-a"]
    verbs: ["get", "list", "watch", "update", "patch"]
  - apiGroups: ["vpc.roks.ibm.com"]
    resources: ["vpcrouters"]
    resourceNames: ["router-bu-a"]
    verbs: ["get", "list", "watch", "update", "patch"]
  - apiGroups: ["vpc.roks.ibm.com"]
    resources: ["vpcvpngateways"]
    resourceNames: ["vpn-bu-a"]
    verbs: ["get", "list", "watch", "update", "patch"]
  - apiGroups: ["vpc.roks.ibm.com"]
    resources: ["vpcgateways/status", "vpcrouters/status", "vpcvpngateways/status"]
    verbs: ["get"]
EOF

# BU-B Role — scoped to gw-bu-b and router-bu-b
oc apply -f - <<'EOF'
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: bu-b-network-admin
  namespace: roks-vpc-network-operator
rules:
  - apiGroups: ["vpc.roks.ibm.com"]
    resources: ["vpcgateways"]
    resourceNames: ["gw-bu-b"]
    verbs: ["get", "list", "watch", "update", "patch"]
  - apiGroups: ["vpc.roks.ibm.com"]
    resources: ["vpcrouters"]
    resourceNames: ["router-bu-b"]
    verbs: ["get", "list", "watch", "update", "patch"]
  - apiGroups: ["vpc.roks.ibm.com"]
    resources: ["vpcgateways/status", "vpcrouters/status"]
    verbs: ["get"]
EOF

# Bind to groups (adjust group names to match your identity provider)
oc create rolebinding bu-a-network-admin \
  --role=bu-a-network-admin \
  --group=bu-a-admins \
  -n roks-vpc-network-operator

oc create rolebinding bu-b-network-admin \
  --role=bu-b-network-admin \
  --group=bu-b-admins \
  -n roks-vpc-network-operator
```

Additionally, scope VM management to each BU's namespace:

```bash
# BU-A can manage VMs in ns-a only
oc create rolebinding bu-a-vm-admin \
  --clusterrole=kubevirt.io:admin \
  --group=bu-a-admins \
  -n ns-a

# BU-B can manage VMs in ns-b only
oc create rolebinding bu-b-vm-admin \
  --clusterrole=kubevirt.io:admin \
  --group=bu-b-admins \
  -n ns-b
```

### 15.4 Cost attribution

Each BU's VPC resources (VNIs, FIPs, VPC routes) are tagged with the gateway name, making cost attribution straightforward:

```bash
# List FIPs — each is tagged with its gateway name
ibmcloud is floating-ips --output json | jq '.[] | select(.name | startswith("roks-")) | {name, address, tags: [.tags[]]}'

# List VPC routes by gateway tag
ibmcloud is vpc-routing-table-routes <VPC_ID> <ROUTING_TABLE_ID> --output json | \
  jq '.[] | select(.name | startswith("roks-")) | {name, destination, next_hop}'
```

Map resources to BUs:
- Resources tagged with `gw-bu-a` → charge to BU-A cost center
- Resources tagged with `gw-bu-b` → charge to BU-B cost center

### 15.5 Full isolation matrix

| # | Source | Destination | Expected | Reason |
|---|--------|-------------|----------|--------|
| 1 | VM-1 (BU-A app) | VM-2 (BU-A db) | Allow | Same router (`router-bu-a`) |
| 2 | VM-1 (BU-A app) | Internet | Allow (SNAT via FIP-A) | `gw-bu-a` NAT |
| 3 | VM-2 (BU-A db) | Internet | Deny | Firewall rule on `router-bu-a` |
| 4 | VM-3 (BU-B web) | VM-4 (BU-B svc) | Allow | Same router (`router-bu-b`) |
| 5 | VM-3 (BU-B web) | Internet | Allow (SNAT via FIP-B) | `gw-bu-b` NAT |
| 6 | Internet | VM-3:80 | Allow (DNAT) | PAR + DNAT on `gw-bu-b` |
| 7 | VM-1 (BU-A) | VM-3 (BU-B) | Deny | No route — routers are isolated |
| 8 | VM-3 (BU-B) | VM-1 (BU-A) | Deny | No route — routers are isolated |
| 9 | VPN remote | BU-A 10.100.x | Allow | `vpn-bu-a` tunnel |

Run the full matrix:

```bash
echo "=== Test 1: BU-A app → BU-A db (expect: PASS) ==="
sshpass -p fedora virtctl ssh --username=fedora \
  -t '-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o PreferredAuthentications=password' \
  -c 'ping -c 1 -W 3 10.100.1.100 && echo PASS || echo FAIL' vm/vm-app -n ns-a

echo "=== Test 2: BU-A app → Internet (expect: PASS with FIP-A) ==="
sshpass -p fedora virtctl ssh --username=fedora \
  -t '-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o PreferredAuthentications=password' \
  -c 'curl -s ifconfig.me' vm/vm-app -n ns-a

echo "=== Test 3: BU-A db → Internet (expect: FAIL — firewall) ==="
sshpass -p fedora virtctl ssh --username=fedora \
  -t '-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o PreferredAuthentications=password' \
  -c 'curl -s --max-time 5 ifconfig.me || echo BLOCKED' vm/vm-db -n ns-a

echo "=== Test 4: BU-B web → BU-B svc (expect: PASS) ==="
sshpass -p fedora virtctl ssh --username=fedora \
  -t '-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o PreferredAuthentications=password' \
  -c 'ping -c 1 -W 3 10.200.1.100 && echo PASS || echo FAIL' vm/vm-web -n ns-b

echo "=== Test 5: BU-B web → Internet (expect: PASS with FIP-B) ==="
sshpass -p fedora virtctl ssh --username=fedora \
  -t '-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o PreferredAuthentications=password' \
  -c 'curl -s ifconfig.me' vm/vm-web -n ns-b

echo "=== Test 6: Internet → VM-3:80 DNAT (expect: PASS) ==="
FIP_B=$(oc get vgw gw-bu-b -n roks-vpc-network-operator -o jsonpath='{.status.floatingIP}')
curl -s --max-time 5 http://$FIP_B:80

echo "=== Test 7: BU-A → BU-B (expect: FAIL — isolated) ==="
sshpass -p fedora virtctl ssh --username=fedora \
  -t '-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o PreferredAuthentications=password' \
  -c 'ping -c 1 -W 3 10.200.0.100; echo exit:$?' vm/vm-app -n ns-a

echo "=== Test 8: BU-B → BU-A (expect: FAIL — isolated) ==="
sshpass -p fedora virtctl ssh --username=fedora \
  -t '-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o PreferredAuthentications=password' \
  -c 'ping -c 1 -W 3 10.100.0.100; echo exit:$?' vm/vm-web -n ns-b

echo "=== Test 9: VPN routes exist for BU-A only ==="
oc get vgw gw-bu-a -n roks-vpc-network-operator -o jsonpath='{.status.vpcRoutes}' | jq '.[].destination' | grep 192.168 && echo "PASS: VPN route exists on gw-bu-a"
oc get vgw gw-bu-b -n roks-vpc-network-operator -o jsonpath='{.status.vpcRoutes}' | jq '.[].destination' | grep 192.168 || echo "PASS: No VPN route on gw-bu-b"
```

---

## Part 16: Multi-tenant cleanup

Delete all multi-tenant resources in reverse dependency order.

### 16.1 Delete VPN gateway

```bash
oc delete vpcvpngateway vpn-bu-a -n roks-vpc-network-operator --wait
oc delete secret wg-bu-a-key -n roks-vpc-network-operator --ignore-not-found
```

### 16.2 Delete routers

```bash
oc delete vpcrouter router-bu-a router-bu-b -n roks-vpc-network-operator --wait

# Wait for router pods to terminate
oc wait --for=delete pod/router-bu-a-pod pod/router-bu-b-pod \
  -n roks-vpc-network-operator --timeout=120s
```

### 16.3 Delete gateways

```bash
oc delete vpcgateway gw-bu-a gw-bu-b -n roks-vpc-network-operator --wait
```

### 16.4 Delete VMs

```bash
oc delete vm vm-app vm-db -n ns-a
oc delete vm vm-web vm-svc -n ns-b

oc wait --for=delete vmi/vm-app vmi/vm-db -n ns-a --timeout=120s
oc wait --for=delete vmi/vm-web vmi/vm-svc -n ns-b --timeout=120s
```

### 16.5 Delete CUDNs

```bash
oc delete cudn localnet-ext app-l2 db-l2 web-l2 svc-l2
```

### 16.6 Delete RBAC and namespaces

```bash
# RBAC (roles, bindings)
oc delete role bu-a-network-admin bu-b-network-admin -n roks-vpc-network-operator --ignore-not-found
oc delete rolebinding bu-a-network-admin bu-b-network-admin -n roks-vpc-network-operator --ignore-not-found
oc delete rolebinding bu-a-vm-admin -n ns-a --ignore-not-found
oc delete rolebinding bu-b-vm-admin -n ns-b --ignore-not-found

# Namespaces
oc delete namespace ns-a ns-b
```

### 16.7 Verify cleanup

```bash
# No operator resources
oc get vgw,vrt,vvg,vsn,vla -n roks-vpc-network-operator
# No resources found

# No CUDNs
oc get cudn | grep -E 'app-l2|db-l2|web-l2|svc-l2|localnet-ext'
# (no output)

# No tutorial namespaces
oc get ns | grep -E 'ns-a|ns-b'
# (no output)

# VPC resources cleaned up
ibmcloud is subnets | grep roks-
ibmcloud is floating-ips | grep roks-
```
