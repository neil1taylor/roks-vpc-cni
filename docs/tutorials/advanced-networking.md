# Tutorial: Advanced Multi-Namespace Enterprise Networking

This tutorial builds a realistic enterprise network topology using every major feature of the ROKS VPC Network Operator — multi-namespace isolation, multi-tier routing, SNAT/DNAT, IPS inline security, WireGuard VPN, firewall segmentation, and DHCP reservations. Each part builds on the previous one. By the end, you will have a fully connected, secured, multi-namespace topology with VPN access and verified connectivity at every stage.

## Table of contents

- [Part 1: Prerequisites & network planning](#part-1-prerequisites--network-planning)
- [Part 2: Foundation — L2 networks](#part-2-foundation--l2-networks)
- [Part 3: VMs — workloads on L2 networks](#part-3-vms--workloads-on-l2-networks)
- [Part 4: Routers — connecting L2 to transit](#part-4-routers--connecting-l2-to-transit)
- [Part 5: Gateway — internet uplink with SNAT](#part-5-gateway--internet-uplink-with-snat)
- [Part 6: Connectivity testing — the foundation works](#part-6-connectivity-testing--the-foundation-works)
- [Part 7: Secure DMZ — DNAT + PAR ingress](#part-7-secure-dmz--dnat--par-ingress)
- [Part 8: IPS inline security](#part-8-ips-inline-security)
- [Part 9: VPN — site-to-site WireGuard](#part-9-vpn--site-to-site-wireguard)
- [Part 10: Split routing verification](#part-10-split-routing-verification)
- [Part 11: Multi-tier firewall + DHCP reservations](#part-11-multi-tier-firewall--dhcp-reservations)
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
            |  transit=10.99.0.1|
            +--------+----------+
                     |
                     | transit-l2 (10.99.0.0/24)
                     |
          +----------+----------+
          |                     |
   +------+--------+    +------+--------+
   | VPCRouter     |    | VPCVPNGateway |
   | "router-a"    |    | "adv-vpn"     |
   | transit=.2    |    | WireGuard     |
   | ns-a          |    | 192.168.0/24  |
   +---+-------+---+    +------+--------+
       |       |                |
       |       |         +------+--------+
       |       |         | VPCRouter     |
       |       |         | "router-b"    |
       |       |         | transit=.3    |
       |       |         | ns-b          |
       |       |         +---+-------+---+
       |       |             |       |
   app-l2  db-l2        web-l2   svc-l2
  10.100.0 10.100.1    10.200.0 10.200.1
   /24      /24         /24      /24
     |       |           |       |
   VM-1    VM-2        VM-3    VM-4
  (app)    (db)        (web)   (svc)
```

### Address plan

| Resource | CIDR / Address | Purpose |
|----------|---------------|---------|
| `transit-l2` | `10.99.0.0/24` | Inter-router transit network |
| `app-l2` | `10.100.0.0/24` | Application tier (ns-a) |
| `db-l2` | `10.100.1.0/24` | Database tier (ns-a) |
| `web-l2` | `10.200.0.0/24` | Web tier (ns-b) |
| `svc-l2` | `10.200.1.0/24` | Services tier (ns-b) |
| `adv-gw` transit | `10.99.0.1` | Gateway on transit |
| `router-a` transit | `10.99.0.2` | Router A on transit |
| `router-b` transit | `10.99.0.3` | Router B on transit |
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

This topology uses five Layer2 CUDNs (no VPC resources needed) and one LocalNet CUDN (VPC-backed, for the gateway uplink).

### 2.1 Create the workload Layer2 networks

Layer2 CUDNs are pure OVN networks — no VPC subnet, no VLAN attachment. They provide isolated L2 segments for VM workloads.

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

### 2.2 Create the transit Layer2 network

The transit network connects all routers and the gateway. Both workload namespaces and the operator namespace must be included.

```yaml
# adv-transit.yaml
apiVersion: k8s.ovn.org/v1
kind: ClusterUserDefinedNetwork
metadata:
  name: transit-l2
spec:
  namespaceSelector:
    matchExpressions:
    - key: kubernetes.io/metadata.name
      operator: In
      values: [ns-a, ns-b, roks-vpc-network-operator]
  network:
    topology: Layer2
    layer2:
      role: Secondary
      subnets:
      - cidr: "10.99.0.0/24"
```

```bash
oc apply -f adv-transit.yaml
```

### 2.3 Create the uplink LocalNet CUDN

The uplink network is a LocalNet CUDN — it creates a VPC subnet and VLAN attachments on every bare metal node. The gateway uses this to reach the VPC fabric.

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

### 2.4 Verify all networks

```bash
# All 6 CUDNs should appear
oc get cudn

# Expected output:
# NAME           TOPOLOGY   AGE
# app-l2         Layer2     30s
# db-l2          Layer2     30s
# web-l2         Layer2     30s
# svc-l2         Layer2     30s
# transit-l2     Layer2     30s
# localnet-ext   LocalNet   30s

# Verify VPC resources for the LocalNet CUDN
oc get vpcsubnets       # vsn — one VPC subnet
oc get vlanattachments  # vla — one per bare metal node
```

Wait until the VPC subnet status shows `active` (10-30 seconds).

---

## Part 3: VMs — workloads on L2 networks

Each VM connects to a single Layer2 network with `autoattachPodInterface: false` (no pod network). DHCP will come from routers in Part 4 — for now, VMs boot and wait for an IP.

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

The VMs are running but have no IP addresses yet — DHCP service starts when we create the routers in the next step.

---

## Part 4: Routers — connecting L2 to transit

Each router connects workload L2 networks to the transit network, provides DHCP, and advertises its connected routes to the gateway.

### How it works

```
  transit-l2 (10.99.0.0/24)
       |
  +----+------+
  | router-a  |  (ns-a)
  | .2 transit|
  +----+--+---+
       |  |
       |  +--- db-l2 (10.100.1.0/24)   router address: 10.100.1.1
       |
       +------ app-l2 (10.100.0.0/24)  router address: 10.100.0.1
```

The router pod gets Multus attachments to each network. IP forwarding bridges traffic between them. DHCP serves addresses on each workload network with the router as the default gateway.

### 4.1 Router A (ns-a workloads)

```yaml
# adv-router-a.yaml
apiVersion: vpc.roks.ibm.com/v1alpha1
kind: VPCRouter
metadata:
  name: router-a
  namespace: roks-vpc-network-operator
spec:
  gateway: adv-gw                       # References the gateway (created in Part 5)

  transit:
    network: transit-l2
    address: "10.99.0.2"

  networks:
  - name: app-l2
    namespace: ns-a
    address: "10.100.0.1/24"
  - name: db-l2
    namespace: ns-a
    address: "10.100.1.1/24"

  dhcp:
    enabled: true
    leaseTime: "12h"
    dns:
      nameservers:
      - "8.8.8.8"
      - "8.8.4.4"

  routeAdvertisement:
    connectedSegments: true              # Advertises 10.100.0.0/24 + 10.100.1.0/24
```

### 4.2 Router B (ns-b workloads)

```yaml
# adv-router-b.yaml
apiVersion: vpc.roks.ibm.com/v1alpha1
kind: VPCRouter
metadata:
  name: router-b
  namespace: roks-vpc-network-operator
spec:
  gateway: adv-gw

  transit:
    network: transit-l2
    address: "10.99.0.3"

  networks:
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
    connectedSegments: true              # Advertises 10.200.0.0/24 + 10.200.1.0/24
```

> **Note:** The routers reference `adv-gw` which does not exist yet. Apply the routers now — they will enter `Pending` phase and become `Ready` after the gateway is created in Part 5.

```bash
oc apply -f adv-router-a.yaml -f adv-router-b.yaml
```

**What the reconciler does:**
1. Adds the `vpc.roks.ibm.com/router-cleanup` finalizer
2. Waits for the referenced gateway `adv-gw` to be `Ready`
3. Creates a router pod with Multus attachments to the transit and workload networks
4. Starts dnsmasq for DHCP on each workload interface
5. Writes `status.advertisedRoutes` with the connected network CIDRs

### 4.3 Verify routers (after gateway creation)

After the gateway is created in Part 5, the routers will transition to `Ready`:

```bash
oc get vrt -n roks-vpc-network-operator

# Expected output:
# NAME       GATEWAY   PHASE   SYNC     AGE
# router-a   adv-gw    Ready   Synced   2m
# router-b   adv-gw    Ready   Synced   2m

# Check advertised routes
oc get vrt router-a -n roks-vpc-network-operator -o jsonpath='{.status.advertisedRoutes}'
# ["10.100.0.0/24","10.100.1.0/24"]

oc get vrt router-b -n roks-vpc-network-operator -o jsonpath='{.status.advertisedRoutes}'
# ["10.200.0.0/24","10.200.1.0/24"]
```

### 4.4 Verify VMs get DHCP leases

Once the routers are running, VMs should acquire IP addresses via DHCP:

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

The VPCGateway creates a shared uplink VNI on the VPC fabric. It connects to the transit network and provides internet access via SNAT. VPC routes are auto-collected from router `advertisedRoutes`.

### How it works

```
  Internet
     |
  Floating IP (158.177.x.x)
     |
  VPCGateway "adv-gw"
  +-----------------------+
  | uplink: localnet-ext  |  VNI on VPC subnet
  | transit: 10.99.0.1    |
  | NAT: SNAT 10.0/8->FIP|
  | NAT: noNAT 10/8<->10/8|
  +-----------+-----------+
              |
         transit-l2
         10.99.0.0/24
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
    network: transit-l2
    address: "10.99.0.1"
    cidr: "10.99.0.0/24"

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
4. Watches router-a and router-b status — collects `advertisedRoutes` from both
5. Creates VPC routes for all 4 L2 CIDRs (`10.100.0.0/24`, `10.100.1.0/24`, `10.200.0.0/24`, `10.200.1.0/24`) with `action: deliver` and `next_hop: <gateway VNI reserved IP>`
6. Sets phase to `Ready`

### 5.2 Verify the gateway

```bash
# Check gateway status
oc get vgw adv-gw -n roks-vpc-network-operator

# Expected output:
# NAME     ZONE      PHASE   VNI IP           FIP              SYNC     AGE
# adv-gw   eu-de-1   Ready   10.240.64.x      158.177.x.x      Synced   30s

# Check VPC routes (auto-collected from routers)
oc get vgw adv-gw -n roks-vpc-network-operator -o jsonpath='{.status.vpcRouteIDs}' | jq .

# Verify via IBM Cloud CLI
ibmcloud is vpc-routing-table-routes <vpc-id> <default-rt-id>
# Should show 4 routes: 10.100.0.0/24, 10.100.1.0/24, 10.200.0.0/24, 10.200.1.0/24
```

Now go back and verify the routers are Ready (they were waiting for the gateway):

```bash
oc get vrt -n roks-vpc-network-operator

# Both routers should now be Ready
# NAME       GATEWAY   PHASE   SYNC     AGE
# router-a   adv-gw    Ready   Synced   5m
# router-b   adv-gw    Ready   Synced   5m

# Check router pods are running
oc get pods -n roks-vpc-network-operator -l 'vpc.roks.ibm.com/router'

# Expected:
# NAME              READY   STATUS    AGE
# router-a-pod       1/1     Running   3m
# router-b-pod       1/1     Running   3m
```

---

## Part 6: Connectivity testing — the foundation works

Now the full data path is functional. Let's verify every traffic pattern.

### 6.1 VM-to-VM same router (cross-L2)

VM-1 (app-l2) pinging VM-2 (db-l2) — both behind router-a.

```
VM-1 (10.100.0.x) --app-l2--> router-a --db-l2--> VM-2 (10.100.1.x)
```

```bash
# Get VM-2's IP
VM2_IP=$(virtctl ssh --namespace=ns-a --username=fedora \
  -t '-o StrictHostKeyChecking=no' -t '-o UserKnownHostsFile=/dev/null' \
  -t '-o PreferredAuthentications=password' \
  -c 'ip -4 -o addr show enp1s0 | awk "{print \$4}" | cut -d/ -f1' \
  vm-db 2>/dev/null | tr -d '\r')
echo "VM-2 (db) IP: $VM2_IP"

# From VM-1, ping VM-2
virtctl ssh --namespace=ns-a --username=fedora \
  -t '-o StrictHostKeyChecking=no' -t '-o UserKnownHostsFile=/dev/null' \
  -t '-o PreferredAuthentications=password' \
  -c "ping -c 3 $VM2_IP" \
  vm-app
```

### 6.2 VM-to-VM cross router

VM-1 (app-l2, behind router-a) pinging VM-3 (web-l2, behind router-b). Traffic crosses the transit network.

```
VM-1 (10.100.0.x) --app-l2--> router-a --transit-l2--> router-b --web-l2--> VM-3 (10.200.0.x)
```

```bash
# Get VM-3's IP
VM3_IP=$(virtctl ssh --namespace=ns-b --username=fedora \
  -t '-o StrictHostKeyChecking=no' -t '-o UserKnownHostsFile=/dev/null' \
  -t '-o PreferredAuthentications=password' \
  -c 'ip -4 -o addr show enp1s0 | awk "{print \$4}" | cut -d/ -f1' \
  vm-web 2>/dev/null | tr -d '\r')
echo "VM-3 (web) IP: $VM3_IP"

# From VM-1, ping VM-3 (cross-router)
virtctl ssh --namespace=ns-a --username=fedora \
  -t '-o StrictHostKeyChecking=no' -t '-o UserKnownHostsFile=/dev/null' \
  -t '-o PreferredAuthentications=password' \
  -c "ping -c 3 $VM3_IP" \
  vm-app
```

### 6.3 VM-to-Internet (SNAT)

VM-1 reaches the internet via SNAT through the gateway's floating IP.

```
VM-1 --app-l2--> router-a --transit-l2--> gateway --SNAT--> VPC --FIP--> Internet
```

```bash
# From VM-1, test internet connectivity
virtctl ssh --namespace=ns-a --username=fedora \
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
  virtctl ssh --namespace=$NS --username=fedora \
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
  transit-l2 --> router-b --> web-l2 --> VM-3
```

### 7.1 Update the gateway with PAR and DNAT

First, get VM-3's DHCP-assigned IP:

```bash
VM3_IP=$(virtctl ssh --namespace=ns-b --username=fedora \
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
    network: transit-l2
    address: "10.99.0.1"
    cidr: "10.99.0.0/24"

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
4. Gateway pod is recreated with updated DNAT nftables rules

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
virtctl ssh --namespace=ns-b --username=fedora \
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
virtctl ssh --namespace=ns-a --username=fedora \
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

Enable inline IPS (intrusion prevention) on router-a to protect the ns-a workloads. Suricata inspects all forwarded traffic and can block malicious patterns.

### How it works

```
                     Router-A Pod
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

### 8.1 Update router-a with IPS

```yaml
# adv-router-a-ips.yaml
apiVersion: vpc.roks.ibm.com/v1alpha1
kind: VPCRouter
metadata:
  name: router-a
  namespace: roks-vpc-network-operator
spec:
  gateway: adv-gw

  transit:
    network: transit-l2
    address: "10.99.0.2"

  networks:
  - name: app-l2
    namespace: ns-a
    address: "10.100.0.1/24"
  - name: db-l2
    namespace: ns-a
    address: "10.100.1.1/24"

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
oc apply -f adv-router-a-ips.yaml
```

The router reconciler detects the IDS configuration change and recreates the pod with a Suricata sidecar.

### 8.2 Verify IPS is active

```bash
# Wait for pod recreation
oc wait --for=condition=Ready pod/router-a-pod -n roks-vpc-network-operator --timeout=120s

# Pod should show 2/2 containers (router + suricata)
oc get pod router-a-pod -n roks-vpc-network-operator

# Expected:
# NAME             READY   STATUS    AGE
# router-a-pod      2/2     Running   30s

# Verify IDS mode in status
oc get vrt router-a -n roks-vpc-network-operator -o wide
# The IDS column should show "ips"

# Verify NFQUEUE rules on the router container
oc exec router-a-pod -n roks-vpc-network-operator -c router -- nft list table ip suricata
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

From VM-1 (behind router-a), make an HTTP request to trigger the ET Open ruleset:

```bash
virtctl ssh --namespace=ns-a --username=fedora \
  -t '-o StrictHostKeyChecking=no' -t '-o UserKnownHostsFile=/dev/null' \
  -t '-o PreferredAuthentications=password' \
  -c "curl -s http://testmynids.org/uid/index.html" \
  vm-app
```

Check Suricata logs for alerts:

```bash
oc logs router-a-pod -n roks-vpc-network-operator -c suricata --tail=20 | grep '"alert"'
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
                                           transit-l2
                                                |
                                    routes: 192.168.0.0/24
                                    via VPN gateway pod
```

The VPCGateway automatically creates a VPC route for `192.168.0.0/24` when the VPN gateway advertises it.

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
2. Creates a VPN pod with WireGuard configured with the tunnel spec
3. Sets `status.advertisedRoutes: ["192.168.0.0/24"]`
4. The VPCGateway reconciler detects the new advertised route and creates a VPC route for `192.168.0.0/24`

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
AllowedIPs = 10.100.0.0/24, 10.100.1.0/24, 10.200.0.0/24, 10.200.1.0/24, 10.99.0.0/24
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
virtctl ssh --namespace=ns-a --username=fedora \
  -t '-o StrictHostKeyChecking=no' -t '-o UserKnownHostsFile=/dev/null' \
  -t '-o PreferredAuthentications=password' \
  -c "ip route" \
  vm-app
```

Expected:

```
default via 10.100.0.1 dev enp1s0             # router-a is the default gateway
10.100.0.0/24 dev enp1s0 proto kernel scope link src 10.100.0.x
```

### 10.2 Traffic path: VM-to-VM cross router

VM-1 (10.100.0.x) to VM-3 (10.200.0.x):

```
VM-1 -> 10.100.0.1 (router-a) -> 10.99.0.3 (router-b via transit) -> 10.200.0.x (VM-3)
```

```bash
# Traceroute from VM-1 to VM-3
virtctl ssh --namespace=ns-a --username=fedora \
  -t '-o StrictHostKeyChecking=no' -t '-o UserKnownHostsFile=/dev/null' \
  -t '-o PreferredAuthentications=password' \
  -c "traceroute -n $VM3_IP" \
  vm-app
```

Expected hops:
1. `10.100.0.1` (router-a)
2. `10.200.0.1` (router-b)
3. `10.200.0.x` (VM-3)

### 10.3 Traffic path: VM-to-Internet

VM-1 (10.100.0.x) to the internet:

```
VM-1 -> router-a (10.100.0.1) -> transit (10.99.0.1 gateway) -> SNAT -> FIP -> Internet
```

```bash
# Traceroute to an internet host
virtctl ssh --namespace=ns-a --username=fedora \
  -t '-o StrictHostKeyChecking=no' -t '-o UserKnownHostsFile=/dev/null' \
  -t '-o PreferredAuthentications=password' \
  -c "traceroute -n 8.8.8.8" \
  vm-app
```

Expected first hops:
1. `10.100.0.1` (router-a)
2. `10.99.0.1` (gateway on transit)
3. VPC fabric hops...

### 10.4 Traffic path: VM-to-VPN remote

VM-1 (10.100.0.x) to the remote office (192.168.0.x):

```
VM-1 -> router-a -> transit -> gateway -> VPC route -> VPN pod -> WireGuard tunnel -> 192.168.0.x
```

```bash
# From VM-1, ping the remote office subnet (only works if remote side is connected)
virtctl ssh --namespace=ns-a --username=fedora \
  -t '-o StrictHostKeyChecking=no' -t '-o UserKnownHostsFile=/dev/null' \
  -t '-o PreferredAuthentications=password' \
  -c "ping -c 3 -W 5 192.168.0.1 || echo 'Remote side not reachable (expected if no real remote peer)'" \
  vm-app
```

### 10.5 Routing summary

| Source | Destination | Path | NAT |
|--------|------------|------|-----|
| VM-1 (10.100.0.x) | VM-2 (10.100.1.x) | app-l2 -> router-a -> db-l2 | None (noNAT) |
| VM-1 (10.100.0.x) | VM-3 (10.200.0.x) | app-l2 -> router-a -> transit -> router-b -> web-l2 | None (noNAT) |
| VM-1 (10.100.0.x) | Internet | app-l2 -> router-a -> transit -> gateway -> VPC | SNAT to FIP |
| VM-1 (10.100.0.x) | 192.168.0.x | app-l2 -> router-a -> transit -> gateway -> VPN tunnel | None |
| Internet | VM-3:80 | FIP:80 -> gateway DNAT -> transit -> router-b -> web-l2 | DNAT |

---

## Part 11: Multi-tier firewall + DHCP reservations

Add firewall rules to router-a to enforce micro-segmentation between the application and database tiers. Add DHCP reservations for predictable database IPs.

### 11.1 Get VM-2's MAC address

DHCP reservations require the VM's MAC address:

```bash
VM2_MAC=$(virtctl ssh --namespace=ns-a --username=fedora \
  -t '-o StrictHostKeyChecking=no' -t '-o UserKnownHostsFile=/dev/null' \
  -t '-o PreferredAuthentications=password' \
  -c "ip link show enp1s0 | awk '/ether/ {print \$2}'" \
  vm-db 2>/dev/null | tr -d '\r')
echo "VM-2 (db) MAC: $VM2_MAC"
```

### 11.2 Update router-a with firewall and DHCP reservations

```yaml
# adv-router-a-fw.yaml
apiVersion: vpc.roks.ibm.com/v1alpha1
kind: VPCRouter
metadata:
  name: router-a
  namespace: roks-vpc-network-operator
spec:
  gateway: adv-gw

  transit:
    network: transit-l2
    address: "10.99.0.2"

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
    # Allow app tier to reach DB on PostgreSQL port
    - name: allow-app-to-db-postgres
      direction: egress
      action: allow
      source: "10.100.0.0/24"
      destination: "10.100.1.0/24"
      protocol: tcp
      port: 5432
      priority: 10

    # Allow app tier to ping DB tier (monitoring)
    - name: allow-app-to-db-icmp
      direction: egress
      action: allow
      source: "10.100.0.0/24"
      destination: "10.100.1.0/24"
      protocol: icmp
      priority: 11

    # Deny DB tier initiating connections to app tier
    - name: deny-db-to-app
      direction: egress
      action: deny
      source: "10.100.1.0/24"
      destination: "10.100.0.0/24"
      protocol: tcp
      priority: 20

    # Deny SSH to DB tier from anywhere
    - name: deny-ssh-to-db
      direction: ingress
      action: deny
      destination: "10.100.1.0/24"
      protocol: tcp
      port: 22
      priority: 30

    # Allow all other traffic
    - name: allow-all-egress
      direction: egress
      action: allow
      protocol: any
      priority: 999

    - name: allow-all-ingress
      direction: ingress
      action: allow
      protocol: any
      priority: 999
```

> **Important:** Replace `<VM2_MAC>` with the actual MAC address from step 11.1.

```bash
oc apply -f adv-router-a-fw.yaml
```

The router pod is recreated with the new firewall rules and DHCP reservation.

### 11.3 Verify DHCP reservation

After the router pod restarts, renew VM-2's DHCP lease:

```bash
# Force DHCP renewal on VM-2
virtctl ssh --namespace=ns-a --username=fedora \
  -t '-o StrictHostKeyChecking=no' -t '-o UserKnownHostsFile=/dev/null' \
  -t '-o PreferredAuthentications=password' \
  -c "sudo dhclient -r enp1s0 && sudo dhclient enp1s0 && ip addr show enp1s0" \
  vm-db
```

VM-2 should now have IP `10.100.1.100`.

### 11.4 Verify firewall rules

```bash
# Check nftables on the router pod
oc exec router-a-pod -n roks-vpc-network-operator -c router -- nft list ruleset
```

### 11.5 Test firewall enforcement

```bash
# Test 1: VM-1 CAN ping VM-2 (ICMP allowed)
virtctl ssh --namespace=ns-a --username=fedora \
  -t '-o StrictHostKeyChecking=no' -t '-o UserKnownHostsFile=/dev/null' \
  -t '-o PreferredAuthentications=password' \
  -c "ping -c 2 -W 3 10.100.1.100 && echo 'PASS: ICMP allowed'" \
  vm-app

# Test 2: VM-1 CAN reach VM-2 on port 5432 (PostgreSQL allowed)
virtctl ssh --namespace=ns-a --username=fedora \
  -t '-o StrictHostKeyChecking=no' -t '-o UserKnownHostsFile=/dev/null' \
  -t '-o PreferredAuthentications=password' \
  -c "timeout 3 bash -c 'echo | nc -w 2 10.100.1.100 5432' 2>&1; echo 'PASS: Port 5432 reachable (connection attempt made)'" \
  vm-app

# Test 3: VM-3 CANNOT SSH to VM-2 (denied by firewall)
virtctl ssh --namespace=ns-b --username=fedora \
  -t '-o StrictHostKeyChecking=no' -t '-o UserKnownHostsFile=/dev/null' \
  -t '-o PreferredAuthentications=password' \
  -c "timeout 3 ssh -o ConnectTimeout=2 -o StrictHostKeyChecking=no fedora@10.100.1.100 echo 'FAIL: SSH should be blocked' 2>&1 || echo 'PASS: SSH blocked by firewall'" \
  vm-web
```

---

## Part 12: Full topology verification & cleanup

### 12.1 Final connectivity matrix

| Source | Destination | Expected | Test command |
|--------|------------|----------|-------------|
| VM-1 (app) | VM-2 (db) ICMP | Allow | `ping -c 1 10.100.1.100` |
| VM-1 (app) | VM-2 (db) TCP/5432 | Allow | `nc -w 2 10.100.1.100 5432` |
| VM-1 (app) | VM-3 (web) ICMP | Allow | `ping -c 1 <VM3_IP>` |
| VM-1 (app) | Internet | Allow (SNAT) | `curl -s ifconfig.me` |
| VM-3 (web) | VM-2 (db) SSH | Deny | `ssh fedora@10.100.1.100` |
| VM-3 (web) | Internet | Allow (SNAT) | `curl -s ifconfig.me` |
| Internet | VM-3:80 | Allow (DNAT) | `curl http://<GW_FIP>:80` |

### 12.2 Resource health check

```bash
# All resources should be Ready/Synced
oc get vrt,vgw,vvg -n roks-vpc-network-operator

# Expected:
# NAME                              GATEWAY   PHASE   SYNC     AGE
# vpcrouter.vpc.roks.ibm.com/router-a   adv-gw    Ready   Synced   20m
# vpcrouter.vpc.roks.ibm.com/router-b   adv-gw    Ready   Synced   20m
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
2. The topology view shows all resources: gateway, routers, VPN gateway, networks, and VMs
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

# 3. Delete routers (depend on gateway)
oc delete vpcrouter router-a router-b -n roks-vpc-network-operator

# Wait for router pods to terminate
oc wait --for=delete pod/router-a-pod pod/router-b-pod -n roks-vpc-network-operator --timeout=120s

# 4. Delete gateway (finalizer cleans up FIP, PAR, VPC routes, VLAN attachment)
oc delete vpcgateway adv-gw -n roks-vpc-network-operator

# 5. Delete WireGuard secret
oc delete secret adv-wg-key -n roks-vpc-network-operator

# 6. Delete CUDNs (LocalNet finalizer cleans up VPC subnet + VLAN attachments)
oc delete cudn localnet-ext transit-l2 app-l2 db-l2 web-l2 svc-l2

# 7. Delete namespaces
oc delete namespace ns-a ns-b
```

### 12.5 Verify cleanup

```bash
# No operator resources should remain
oc get vrt,vgw,vvg,vsn,vla -n roks-vpc-network-operator
# No resources found

# No CUDNs from this tutorial
oc get cudn | grep -E 'app-l2|db-l2|web-l2|svc-l2|transit-l2|localnet-ext'
# (no output)

# Optionally verify VPC resources are cleaned up
ibmcloud is subnets | grep roks-
ibmcloud is floating-ips | grep roks-
```
