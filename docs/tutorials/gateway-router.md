# Tutorial: VPC Network Operator

This tutorial walks through every feature of the ROKS VPC Network Operator. Each section builds on the previous one. By the end, you will have deployed VMs on both LocalNet and Layer2 networks, configured gateways with floating IPs and public address ranges, set up routers with DHCP and firewall rules, enabled IDS/IPS with Suricata, and verified end-to-end internet connectivity.

## Table of contents

- [Part 1: Prerequisites](#part-1-prerequisites)
- [Part 2: LocalNet VM with floating IP](#part-2-localnet-vm-with-floating-ip)
- [Part 3: VPCGateway](#part-3-vpcgateway)
- [Part 4: VPCRouter](#part-4-vpcrouter)
- [Part 5: Layer2 VM with DHCP](#part-5-layer2-vm-with-dhcp)
- [Part 6: NAT configurations](#part-6-nat-configurations)
- [Part 7: Public Address Range (PAR)](#part-7-public-address-range-par)
- [Part 8: Firewall rules](#part-8-firewall-rules)
- [Part 9: Multi-network routing](#part-9-multi-network-routing)
- [Part 10: IDS/IPS with Suricata](#part-10-idsips-with-suricata)
- [Part 11: Cleanup](#part-11-cleanup)
- [Reference](#reference)

---

## Part 1: Prerequisites

### Cluster requirements

- A ROKS cluster with **bare metal workers** (VLAN attachments require bare metal network interfaces)
- **OVN-Kubernetes** with the UserDefinedNetwork feature gate enabled
- The **VPC Network Operator** installed via Helm (see [Installation](../getting-started/installation.md))
- A VPC API key stored in a Kubernetes Secret (configured during installation)

### Tools

- `oc` (OpenShift CLI), logged in to the cluster
- `virtctl` (KubeVirt CLI) for VM console/SSH access
- `ibmcloud` CLI (optional, for verifying VPC resources)

### VPC resource IDs

Collect the following from your IBM Cloud account:

| Resource | How to find it |
|----------|---------------|
| VPC ID | `ibmcloud is vpcs` |
| Zone | `ibmcloud is zones` (e.g., `eu-de-1`, `us-south-1`) |
| Security Group ID | `ibmcloud is security-groups` |
| Network ACL ID | `ibmcloud is network-acls` |

### Create a namespace

```bash
oc create namespace vm-demo
```

---

## Part 2: LocalNet VM with floating IP

This is the simplest use case: a VM with a VPC-native IP and direct internet access via a floating IP.

### How it works

```
                     +--------------------------------+
                     |        IBM Cloud VPC            |
                     |                                 |
  Internet ---- Floating IP ---- VNI ---- VPC Subnet   |
                     |             |          |        |
                     +-------------|----------|--------+
                                   |          |
                     +-------------|----------|--------+
                     |  Bare Metal | Node     |        |
                     |     VLAN Attachment ----+        |
                     |             |                    |
                     |     OVN LocalNet bridge          |
                     |             |                    |
                     |         +-------+                |
                     |         |  VM   |                |
                     |         +-------+                |
                     +-------------------------------------+
```

**LocalNet** maps a Kubernetes secondary network onto a physical VLAN, giving VMs first-class VPC identities (MAC, IP, security groups). The operator automates VPC subnet creation, VLAN attachments on every bare metal node, and VNI provisioning per VM.

### 2.1 Create a LocalNet CUDN

A `ClusterUserDefinedNetwork` (CUDN) with LocalNet topology tells OVN-Kubernetes to create a localnet bridge. The operator's annotations trigger VPC resource provisioning.

```yaml
# demo-localnet.yaml
apiVersion: k8s.ovn.org/v1
kind: ClusterUserDefinedNetwork
metadata:
  name: demo-localnet
  annotations:
    vpc.roks.ibm.com/zone: "eu-de-1"              # VPC zone
    vpc.roks.ibm.com/cidr: "10.240.64.0/24"       # VPC subnet CIDR
    vpc.roks.ibm.com/vpc-id: "<your-vpc-id>"       # VPC ID
    vpc.roks.ibm.com/vlan-id: "200"                # VLAN tag (1-4094, unused)
    vpc.roks.ibm.com/security-group-ids: "<sg-id>" # Security group(s)
    vpc.roks.ibm.com/acl-id: "<acl-id>"            # Network ACL
spec:
  namespaceSelector:
    matchExpressions:
    - key: kubernetes.io/metadata.name
      operator: In
      values: [vm-demo, default]    # Include 'default' for Multus namespace isolation
  network:
    topology: LocalNet
    localNet:
      role: Secondary
      subnets:
      - cidr: "10.240.64.0/24"
```

```bash
oc apply -f demo-localnet.yaml
```

**What happens:** The CUDN reconciler creates a VPC subnet (`roks-<cluster>-demo-localnet`) and a VLAN attachment on every bare metal node with `vlan_id: 200`, `allow_to_float: true`.

### 2.2 Verify the network

```bash
# Check operator-written annotations
oc get cudn demo-localnet -o yaml | grep 'vpc.roks.ibm.com'

# Check CRDs
oc get vpcsubnets     # vsn — VPC subnet
oc get vlanattachments  # vla — one per BM node
```

Wait until `subnet-status` shows `active` (10-30 seconds).

### 2.3 Create a VM

The mutating webhook intercepts the VM CREATE request, provisions a VNI, reads the VPC-assigned MAC and IP, and injects them into the VM spec.

```yaml
# demo-vm-localnet.yaml
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: demo-vm-localnet
  namespace: vm-demo
  annotations:
    vpc.roks.ibm.com/fip: "true"   # Request a floating IP
spec:
  running: true
  template:
    spec:
      networks:
      - name: vpc
        multus:
          networkName: demo-localnet
      domain:
        resources:
          requests:
            memory: 2Gi
            cpu: "1"
        devices:
          interfaces:
          - name: vpc
            bridge: {}
          disks:
          - name: rootdisk
            disk:
              bus: virtio
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
```

```bash
oc apply -f demo-vm-localnet.yaml
```

**What the webhook does:**
1. Looks up `demo-localnet` CUDN for VPC subnet ID and security groups
2. Creates a per-VM VLAN attachment with inline VNI (`allow_ip_spoofing: true`, `enable_infrastructure_nat: true`, `auto_delete: true`)
3. Creates a floating IP and binds it to the VNI
4. Injects the MAC address into `interfaces[0].macAddress`
5. Injects cloud-init network-config with the reserved IP
6. Writes annotations: `vni-id`, `mac-address`, `reserved-ip`, `fip-id`, `fip-address`
7. Adds the `vpc.roks.ibm.com/vm-cleanup` finalizer

### 2.4 Verify and connect

```bash
# Wait for VM to boot
oc get vmi demo-vm-localnet -n vm-demo -w

# Check webhook-written annotations
oc get vm demo-vm-localnet -n vm-demo -o yaml | grep 'vpc.roks.ibm.com'

# Check CRDs
oc get vni -n vm-demo     # VNI created for the VM
oc get fip -n vm-demo     # Floating IP bound to the VNI

# SSH via floating IP
FIP=$(oc get vm demo-vm-localnet -n vm-demo -o jsonpath='{.metadata.annotations.vpc\.roks\.ibm\.com/fip-address}')
ssh fedora@$FIP   # password: fedora

# Inside the VM
ip addr show          # VPC-assigned IP on enp1s0
ping -c 3 1.1.1.1    # Internet via floating IP
curl -s ifconfig.me   # Shows the floating IP
```

---

## Part 3: VPCGateway

A `VPCGateway` provides a shared uplink to the VPC fabric. Unlike per-VM VNIs, a gateway creates a single VNI with a dedicated VLAN attachment that multiple routers can share. This is the foundation for routing Layer2 network traffic to the internet.

### How it works

```
                     +-------------------------------------+
                     |           IBM Cloud VPC              |
  Internet ---- FIP -+-- VNI (uplink) -- VPC Subnet        |
                     |       |                              |
                     |   VPC Route: 10.100.0.0/24           |
                     |     -> deliver to VNI IP             |
                     +--------|----------------------------+
                              |
                     +--------|----------------------------+
                     |   Bare Metal Node                    |
                     |   VLAN Attachment (LocalNet bridge)   |
                     |        |                              |
                     |   +----+------+                       |
                     |   | Router Pod |--- L2 Network        |
                     |   +-----------+        |              |
                     |                    +-------+          |
                     |                    |  VM   |          |
                     |                    +-------+          |
                     +--------------------------------------+
```

> **Progressive examples:** Sections 3.1–3.7 show gateway features incrementally. You do not need to create every example gateway. **For the rest of this tutorial (Parts 4–10), we use `demo-gw` from Section 3.1.** Create only `demo-gw` and skip the other variants unless you want to experiment.

### 3.1 Basic gateway

A minimal gateway needs a zone, an uplink network (a LocalNet CUDN), and a transit address.

> **Multus namespace isolation:** If your cluster has `namespaceIsolation: true` in the Multus config (common on ROKS), the router pod in `roks-vpc-network-operator` can only reference NADs from its own namespace or Multus global namespaces. Use `namespace: default` (a global namespace) for the uplink and network references. Ensure your CUDNs include `default` in their `namespaceSelector`.

This gateway includes a floating IP, a VPC route for the L2 network, and SNAT — everything needed for Parts 4–5.

```yaml
# demo-gateway.yaml
apiVersion: vpc.roks.ibm.com/v1alpha1
kind: VPCGateway
metadata:
  name: demo-gw
  namespace: roks-vpc-network-operator
spec:
  zone: "eu-de-1"

  uplink:
    network: demo-localnet        # LocalNet CUDN for VPC connectivity
    namespace: default            # Use a Multus global namespace (not the CUDN's target namespace)
    securityGroupIDs:             # Optional: security groups for the uplink VNI
    - "<sg-id>"

  transit:
    address: "172.16.100.1/24"    # IP on the transit network (inter-router link)

  floatingIP:
    enabled: true                 # Public IP for outbound SNAT

  vpcRoutes:
  - destination: "10.100.0.0/24"  # Route L2 traffic back to this gateway

  nat:
    snat:
    - source: "10.100.0.0/24"    # SNAT L2 traffic for outbound internet
      priority: 100
```

```bash
oc apply -f demo-gateway.yaml
```

**What the reconciler does:**
1. Adds the `vpc.roks.ibm.com/gateway-cleanup` finalizer
2. Picks a bare metal server and creates a VLAN attachment with an inline VNI
3. Allocates a floating IP and binds it to the VNI
4. Creates VPC routes (`action: deliver` to the VNI's reserved IP)
5. Stores the VNI ID, MAC address, reserved IP, FIP, attachment ID in status
6. Sets phase to `Ready`

```bash
# Check gateway status
oc get vpcgateway demo-gw -n roks-vpc-network-operator

# Example output:
# NAME      ZONE      PHASE   VNI IP           FIP              SYNC     AGE
# demo-gw   eu-de-1   Ready   10.240.64.8      158.177.12.94    Synced   30s

# Detailed status
oc get vpcgateway demo-gw -n roks-vpc-network-operator -o yaml
```

### 3.2 Gateway with floating IP

Enable a floating IP to give the gateway a public address for outbound SNAT or inbound DNAT.

```yaml
# demo-gateway-fip.yaml
apiVersion: vpc.roks.ibm.com/v1alpha1
kind: VPCGateway
metadata:
  name: demo-gw-fip
  namespace: roks-vpc-network-operator
spec:
  zone: "eu-de-1"
  uplink:
    network: demo-localnet
    namespace: default
  transit:
    address: "172.16.100.2/24"
  floatingIP:
    enabled: true                 # Allocate a new floating IP
```

```bash
oc apply -f demo-gateway-fip.yaml

# Check — FIP column shows the public IP
oc get vpcgateway demo-gw-fip -n roks-vpc-network-operator
# NAME           ZONE      PHASE   VNI IP           FIP              SYNC     AGE
# demo-gw-fip   eu-de-1   Ready   172.16.100.35    158.177.12.94    Synced   30s
```

To adopt an existing floating IP instead of creating a new one:

```yaml
  floatingIP:
    enabled: true
    id: "r006-existing-fip-id"    # Bind existing FIP to this gateway's VNI
```

### 3.3 Gateway with VPC routes

VPC routes tell the VPC fabric to deliver traffic for specific CIDRs to the gateway's VNI. This is how return traffic reaches VMs on Layer2 networks (which have no VPC subnet).

```yaml
# demo-gateway-routes.yaml
apiVersion: vpc.roks.ibm.com/v1alpha1
kind: VPCGateway
metadata:
  name: demo-gw-routes
  namespace: roks-vpc-network-operator
spec:
  zone: "eu-de-1"
  uplink:
    network: demo-localnet
    namespace: default
  transit:
    address: "172.16.100.3/24"
  floatingIP:
    enabled: true
  vpcRoutes:
  - destination: "10.100.0.0/24"   # Route L2 network traffic to this gateway
  - destination: "10.200.0.0/24"   # Multiple L2 networks
```

The operator creates VPC routes with `action: deliver` and `next_hop: <gateway VNI reserved IP>` in the VPC's default routing table.

```bash
# Verify routes were created
oc get vpcgateway demo-gw-routes -n roks-vpc-network-operator -o jsonpath='{.status.vpcRouteIDs}'

# Check via IBM Cloud CLI
ibmcloud is vpc-routing-table-routes <vpc-id> <default-rt-id>
```

### 3.4 Gateway with NAT

NAT rules are rendered as nftables configuration and applied on the router pod. They are defined on the gateway spec because the gateway owns the public IP addresses.

```yaml
spec:
  nat:
    # Source NAT: translate outbound traffic from L2 networks
    snat:
    - source: "10.100.0.0/24"        # Match traffic from this L2 network
      priority: 100                    # Lower = evaluated first
      # translatedAddress omitted → uses the gateway's floating IP

    - source: "10.200.0.0/24"
      translatedAddress: "10.240.64.50"  # Translate to a specific IP
      priority: 200

    # Destination NAT: port forwarding from public IP to internal servers
    dnat:
    - externalPort: 443               # Incoming port on the FIP
      internalAddress: "10.100.0.50"  # Forward to this internal address
      internalPort: 8443              # On this internal port
      protocol: tcp                    # tcp (default) or udp
      priority: 50

    - externalPort: 8080
      internalAddress: "10.100.0.60"
      internalPort: 80
      protocol: tcp
      priority: 51

    # No-NAT exceptions: bypass NAT for specific source/destination pairs
    noNAT:
    - source: "10.100.0.0/24"
      destination: "10.200.0.0/24"    # Inter-L2 traffic skips NAT
      priority: 10                     # Evaluated before SNAT rules
```

**Generated nftables:**
```
table ip nat {
  chain prerouting {
    type nat hook prerouting priority -100; policy accept;
    tcp dport 443 dnat to 10.100.0.50:8443
    tcp dport 8080 dnat to 10.100.0.60:80
  }
  chain postrouting {
    type nat hook postrouting priority 100; policy accept;
    ip saddr 10.100.0.0/24 ip daddr 10.200.0.0/24 accept
    ip saddr 10.100.0.0/24 snat to 158.177.12.94
    ip saddr 10.200.0.0/24 snat to 10.240.64.50
  }
}
```

### 3.5 Gateway with firewall

Firewall rules create an nftables filter table on the router pod.

```yaml
spec:
  firewall:
    enabled: true
    rules:
    - name: allow-ssh
      direction: ingress
      action: allow
      protocol: tcp
      port: 22
      priority: 10

    - name: allow-https
      direction: ingress
      action: allow
      protocol: tcp
      port: 443
      priority: 20

    - name: allow-icmp
      direction: ingress
      action: allow
      protocol: icmp
      priority: 30

    - name: deny-all-ingress
      direction: ingress
      action: deny
      protocol: any
      priority: 999

    - name: allow-all-egress
      direction: egress
      action: allow
      protocol: any
      priority: 100
```

### 3.6 Gateway with pod overrides

Override the default container image for the router pod.

```yaml
spec:
  pod:
    image: "de.icr.io/roks/vpc-router:v1.0.0"   # Custom router image
```

### 3.7 Pod resource limits and scheduling

Configure CPU/memory guarantees, node placement, and runtime class for production router pods.

```yaml
spec:
  pod:
    resources:
      requests:
        cpu: "2"
        memory: "1Gi"
      limits:
        cpu: "4"
        memory: "2Gi"
    runtimeClassName: performance          # CPU Manager static policy
    priorityClassName: system-node-critical
    nodeSelector:
      node-role.kubernetes.io/worker: ""
      feature.node.kubernetes.io/network-sriov.capable: "true"
    tolerations:
      - key: dedicated
        operator: Equal
        value: network
        effect: NoSchedule
```

All fields are optional and apply to both VPCGateway and VPCRouter `spec.pod`. The router pod is recreated automatically when any field changes.

---

## Part 4: VPCRouter

A `VPCRouter` connects one or more Layer2 networks to a VPCGateway's uplink. The reconciler creates a privileged pod with Multus network attachments, IP forwarding, and optional NAT/DHCP/firewall.

### 4.1 Basic router

A minimal router needs a gateway reference and at least one workload network.

First, create a Layer2 CUDN (no VPC resources needed):

```yaml
# demo-l2.yaml
apiVersion: k8s.ovn.org/v1
kind: ClusterUserDefinedNetwork
metadata:
  name: demo-l2
spec:
  namespaceSelector:
    matchExpressions:
    - key: kubernetes.io/metadata.name
      operator: In
      values: [vm-demo, default]      # Include 'default' for Multus namespace isolation
  network:
    topology: Layer2
    layer2:
      role: Secondary
      subnets:
      - cidr: "10.100.0.0/24"
```

```bash
oc apply -f demo-l2.yaml
```

Now create the router:

```yaml
# demo-router.yaml
apiVersion: vpc.roks.ibm.com/v1alpha1
kind: VPCRouter
metadata:
  name: demo-router
  namespace: roks-vpc-network-operator
spec:
  gateway: demo-gw-routes         # Must reference a Ready VPCGateway
  networks:
  - name: demo-l2                 # Layer2 CUDN name
    namespace: default            # Must be a Multus global namespace
    address: "10.100.0.1/24"      # Router's IP on this network
```

```bash
oc apply -f demo-router.yaml
```

**What the reconciler does:**
1. Adds the `vpc.roks.ibm.com/router-cleanup` finalizer
2. Validates the referenced gateway exists and is `Ready`
3. Creates a router pod (`demo-router-pod`) with:
   - Multus annotation attaching to the gateway's uplink network (with the gateway's VNI MAC)
   - Multus annotation attaching to each workload network
   - Privileged container with `NET_ADMIN` and `NET_RAW`
   - Init script: installs tools, configures interfaces, enables IP forwarding
4. Sets conditions: `TransitConnected`, `RoutesConfigured`, `PodReady`

```bash
# Check router status
oc get vpcrouter demo-router -n roks-vpc-network-operator
# NAME           GATEWAY          PHASE   SYNC     AGE
# demo-router   demo-gw-routes   Ready   Synced   30s

# Check the router pod
oc get pods -n roks-vpc-network-operator -l vpc.roks.ibm.com/router=demo-router
# NAME                READY   STATUS    AGE
# demo-router-pod     1/1     Running   25s

# Inspect the pod's Multus annotation
oc get pod demo-router-pod -n roks-vpc-network-operator -o jsonpath='{.metadata.annotations.k8s\.v1\.cni\.cncf\.io/networks}' | jq .
```

### 4.2 Verify the router pod

```bash
# Check interfaces inside the router pod
oc exec demo-router-pod -n roks-vpc-network-operator -- ip addr

# Verify IP forwarding is enabled
oc exec demo-router-pod -n roks-vpc-network-operator -- sysctl net.ipv4.ip_forward

# Check routes
oc exec demo-router-pod -n roks-vpc-network-operator -- ip route

# Check nftables (if NAT or firewall configured)
oc exec demo-router-pod -n roks-vpc-network-operator -- nft list ruleset
```

### 4.3 Route advertisement

Route advertisement controls which routes the router declares in its status. This is informational and used by the VPCGateway to know which VPC routes to create.

```yaml
spec:
  routeAdvertisement:
    connectedSegments: true    # Advertise CIDRs of attached networks (default)
    staticRoutes: false        # Advertise manually configured routes
    natIPs: false              # Advertise NAT-translated IPs
```

With `connectedSegments: true` and a network at `10.100.0.1/24`, the router advertises `10.100.0.0/24` in `status.advertisedRoutes`.

The VPCGateway reconciler watches all VPCRouter status changes. When a router's `advertisedRoutes` change, the gateway automatically collects routes from all associated routers and creates or deletes VPC routes accordingly. You do not need to manually add `vpcRoutes` to the gateway spec when using route advertisement.

### 4.4 Router pod health probes

The router pod includes liveness and readiness probes to ensure reliable operation:

- **Liveness probe**: Checks that IP forwarding is enabled via `sysctl -n net.ipv4.ip_forward`. If forwarding is disabled (e.g., due to a kernel issue), the pod is restarted.
- **Readiness probe**: Verifies the uplink interface is configured and operational by checking `ip route show default | grep -q uplink && ip link show uplink | grep -q UP`. The pod is marked unready if the uplink is down.

### 4.5 Router pod IP in status

The router's pod IP is exposed in `status.podIP`. This is used by the gateway to know the next-hop address for VPC routes:

```bash
oc get vpcrouter demo-router -n roks-vpc-network-operator -o jsonpath='{.status.podIP}'
```

### 4.6 Gateway-triggered router pod recreation

The VPCRouter reconciler watches the referenced VPCGateway for changes. If the gateway's NAT rules, firewall rules, container image, or uplink MAC address change, the router pod is automatically recreated to pick up the new configuration. This ensures router pods always reflect the latest gateway settings without manual intervention.

---

## Part 5: Layer2 VM with DHCP

This scenario creates a VM on a Layer2 network that gets its IP via DHCP from the router. No VPC resources are needed for the VM itself — the router provides all connectivity.

### 5.1 Enable DHCP on the router

```yaml
# demo-router-dhcp.yaml
apiVersion: vpc.roks.ibm.com/v1alpha1
kind: VPCRouter
metadata:
  name: demo-router
  namespace: roks-vpc-network-operator
spec:
  gateway: demo-gw-routes
  networks:
  - name: demo-l2
    namespace: default
    address: "10.100.0.1/24"
  dhcp:
    enabled: true                  # Start dnsmasq on workload interfaces
```

```bash
oc apply -f demo-router-dhcp.yaml
```

The router pod restarts with dnsmasq serving DHCP on the `net0` interface:
- Range: `10.100.0.10` - `10.100.0.254`
- Netmask: `255.255.255.0`
- Gateway: `10.100.0.1` (the router)
- Lease time: 12 hours

```bash
# Verify dnsmasq is running
oc exec demo-router-pod -n roks-vpc-network-operator -- ps aux | grep dnsmasq
```

### 5.2 Create a VM with DHCP

This VM uses `dhcp4: true` in cloud-init to request an IP from the router.

```yaml
# demo-vm-l2.yaml
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: demo-vm-l2
  namespace: vm-demo
spec:
  running: true
  template:
    spec:
      networks:
      - name: l2net
        multus:
          networkName: demo-l2
      domain:
        resources:
          requests:
            memory: 2Gi
            cpu: "1"
        devices:
          interfaces:
          - name: l2net
            bridge: {}
          disks:
          - name: rootdisk
            disk:
              bus: virtio
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

```bash
oc apply -f demo-vm-l2.yaml
```

### 5.3 Verify connectivity

```bash
# Wait for VM to boot
oc get vmi demo-vm-l2 -n vm-demo -w

# SSH into the VM via virtctl
virtctl ssh --namespace=vm-demo --username=fedora demo-vm-l2
# password: fedora

# Inside the VM — check DHCP-assigned IP
ip addr show enp1s0
# Should show an IP in the 10.100.0.10-254 range

# Ping the router
ping -c 3 10.100.0.1

# Ping the internet (via router -> gateway -> VPC -> internet)
ping -c 3 1.1.1.1
ping -c 3 8.8.8.8

# Trace the path
traceroute 1.1.1.1
```

The traffic path: VM (`10.100.0.x`) -> L2 network -> router pod (`10.100.0.1`) -> uplink interface -> VPC fabric -> gateway VNI -> internet.

Return traffic: internet -> VPC route (`10.100.0.0/24 deliver <gateway VNI IP>`) -> gateway VNI -> router pod -> L2 network -> VM.

### 5.4 Static IP alternative

For VMs that need a fixed IP, use cloud-init static configuration instead of DHCP:

```yaml
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
                  addresses:
                  - 10.100.0.50/24
                  routes:
                  - to: 0.0.0.0/0
                    via: 10.100.0.1
                    metric: 100
```

---

## Part 6: NAT configurations

### 6.1 No SNAT (pure routing with VPC routes)

This is the simplest configuration and the default. The router forwards traffic without modifying source addresses. VPC routes handle return traffic.

**Requirements:**
- VPCGateway `vpcRoutes` must include each L2 network CIDR
- The uplink VNI has `allow_ip_spoofing: true`, so the VPC fabric accepts packets with non-VNI source IPs

```yaml
# Gateway — routes traffic for L2 networks
apiVersion: vpc.roks.ibm.com/v1alpha1
kind: VPCGateway
metadata:
  name: gw-nonat
  namespace: roks-vpc-network-operator
spec:
  zone: "eu-de-1"
  uplink:
    network: demo-localnet
    namespace: default
  transit:
    address: "172.16.100.10/24"
  floatingIP:
    enabled: true
  vpcRoutes:
  - destination: "10.100.0.0/24"    # L2 network CIDR
---
# Router — pure forwarding, no NAT
apiVersion: vpc.roks.ibm.com/v1alpha1
kind: VPCRouter
metadata:
  name: router-nonat
  namespace: roks-vpc-network-operator
spec:
  gateway: gw-nonat
  networks:
  - name: demo-l2
    namespace: default
    address: "10.100.0.1/24"
  dhcp:
    enabled: true
  # No nat section = pure IP forwarding
```

**Verify no NAT is active:**

```bash
oc exec router-nonat-pod -n roks-vpc-network-operator -- nft list ruleset
# Should show no nat table or masquerade rules
```

### 6.2 SNAT (source NAT to floating IP)

SNAT translates the source address of outbound packets to the gateway's floating IP. This hides internal IPs and is useful when VPC routes are not available or when you want all traffic to appear from a single public IP.

```yaml
apiVersion: vpc.roks.ibm.com/v1alpha1
kind: VPCGateway
metadata:
  name: gw-snat
  namespace: roks-vpc-network-operator
spec:
  zone: "eu-de-1"
  uplink:
    network: demo-localnet
    namespace: default
  transit:
    address: "172.16.100.11/24"
  floatingIP:
    enabled: true
  nat:
    snat:
    - source: "10.100.0.0/24"      # Translate all traffic from L2 network
      priority: 100
      # translatedAddress omitted → uses the gateway's reserved IP
```

**Verify SNAT:**

```bash
# From a VM on the L2 network
curl -s ifconfig.me
# Should show the gateway's floating IP, not the VM's L2 IP
```

### 6.3 DNAT (port forwarding)

DNAT forwards incoming traffic on the gateway's public IP to internal servers.

```yaml
spec:
  nat:
    dnat:
    - externalPort: 443
      internalAddress: "10.100.0.50"   # Web server VM
      internalPort: 443
      protocol: tcp
      priority: 50

    - externalPort: 8080
      internalAddress: "10.100.0.60"   # App server VM
      internalPort: 80
      protocol: tcp
      priority: 51

    - externalPort: 53
      internalAddress: "10.100.0.70"   # DNS server VM
      internalPort: 53
      protocol: udp
      priority: 52
```

**Test DNAT:**

```bash
# From outside the cluster, connect to the gateway's floating IP
curl https://<gateway-fip>:443
# Reaches 10.100.0.50:443

curl http://<gateway-fip>:8080
# Reaches 10.100.0.60:80
```

### 6.4 NoNAT exceptions

Exempt specific source/destination pairs from NAT processing. Use this to allow inter-network traffic to pass through without translation.

```yaml
spec:
  nat:
    noNAT:
    - source: "10.100.0.0/24"
      destination: "10.200.0.0/24"     # L2-to-L2 traffic: no NAT
      priority: 10
    - source: "10.100.0.0/24"
      destination: "10.240.64.0/24"    # L2-to-LocalNet traffic: no NAT
      priority: 11
    snat:
    - source: "10.100.0.0/24"          # Everything else: SNAT
      priority: 100
```

NoNAT rules are evaluated before SNAT rules (they have lower priority numbers). Matching packets are accepted in the postrouting chain, bypassing all SNAT rules.

---

## Part 7: Public Address Range (PAR)

A Public Address Range provides a contiguous block of IBM-assigned public IPs routed to the gateway via an ingress routing table. Unlike floating IPs (one per VNI), a PAR gives you multiple public IPs that can be assigned to different internal servers via DNAT.

### 7.1 Gateway with PAR

```yaml
apiVersion: vpc.roks.ibm.com/v1alpha1
kind: VPCGateway
metadata:
  name: gw-par
  namespace: roks-vpc-network-operator
spec:
  zone: "eu-de-1"
  uplink:
    network: demo-localnet
    namespace: default
  transit:
    address: "172.16.100.12/24"
  publicAddressRange:
    enabled: true
    prefixLength: 30              # /30 = 4 public IPs
  vpcRoutes:
  - destination: "10.100.0.0/24"
```

Available prefix lengths:

| Prefix | IPs |
|--------|-----|
| `/28`  | 16  |
| `/29`  | 8   |
| `/30`  | 4   |
| `/31`  | 2   |
| `/32`  | 1   |

**What the reconciler does:**
1. Creates a PAR in the VPC zone with the specified prefix length
2. Creates an ingress routing table with `route_internet_ingress: true`
3. Creates an ingress route: `destination=<PAR CIDR>`, `next_hop=<gateway VNI IP>`

```bash
# Check the allocated PAR CIDR
oc get vpcgateway gw-par -n roks-vpc-network-operator
# NAME     ZONE      PHASE   VNI IP           FIP   PAR CIDR            SYNC     AGE
# gw-par   eu-de-1   Ready   172.16.100.36          150.240.68.0/30     Synced   30s
```

### 7.2 PAR with DNAT for multiple public IPs

Use the PAR addresses as external addresses in DNAT rules:

```yaml
spec:
  publicAddressRange:
    enabled: true
    prefixLength: 30              # Allocates e.g., 150.240.68.0/30
  nat:
    dnat:
    - externalAddress: "150.240.68.0"    # First PAR IP
      externalPort: 443
      internalAddress: "10.100.0.50"
      internalPort: 443
      protocol: tcp
      priority: 50

    - externalAddress: "150.240.68.1"    # Second PAR IP
      externalPort: 443
      internalAddress: "10.100.0.60"
      internalPort: 443
      protocol: tcp
      priority: 51

    snat:
    - source: "10.100.0.0/24"
      translatedAddress: "150.240.68.0"  # SNAT to first PAR IP
      priority: 100
```

### 7.3 Adopt an existing PAR

If you have a pre-provisioned PAR, the operator will only manage the ingress routing table and routes:

```yaml
spec:
  publicAddressRange:
    enabled: true
    id: "r006-existing-par-id"     # Operator adopts, doesn't create or delete
```

### 7.4 Combined: FIP + PAR

You can use both a floating IP and a PAR on the same gateway. The FIP is bound to the VNI for direct connectivity, while the PAR provides additional public IPs.

```yaml
spec:
  floatingIP:
    enabled: true                   # FIP for outbound SNAT default
  publicAddressRange:
    enabled: true
    prefixLength: 29                # 8 PAR IPs for DNAT targets
  nat:
    dnat:
    - externalAddress: "150.240.68.0"
      externalPort: 443
      internalAddress: "10.100.0.50"
      internalPort: 443
      protocol: tcp
      priority: 50
    snat:
    - source: "10.100.0.0/24"
      priority: 100
      # translatedAddress omitted → uses gateway's FIP
```

---

## Part 8: Firewall rules

Firewall rules can be configured on both the VPCGateway (applied globally) and the VPCRouter (applied per-router). They generate nftables filter rules on the router pod.

### 8.1 Router-level firewall

```yaml
apiVersion: vpc.roks.ibm.com/v1alpha1
kind: VPCRouter
metadata:
  name: router-fw
  namespace: roks-vpc-network-operator
spec:
  gateway: gw-nonat
  networks:
  - name: demo-l2
    namespace: default
    address: "10.100.0.1/24"
  firewall:
    enabled: true
    rules:
    # Allow established connections (stateful firewall)
    # Note: this is automatic — ct state established,related accept is always first

    # Allow SSH from anywhere
    - name: allow-ssh
      direction: ingress
      action: allow
      protocol: tcp
      port: 22
      priority: 10

    # Allow HTTP/HTTPS from anywhere
    - name: allow-http
      direction: ingress
      action: allow
      protocol: tcp
      port: 80
      priority: 20

    - name: allow-https
      direction: ingress
      action: allow
      protocol: tcp
      port: 443
      priority: 21

    # Allow ICMP (ping)
    - name: allow-icmp
      direction: ingress
      action: allow
      protocol: icmp
      priority: 30

    # Allow traffic from specific subnet only
    - name: allow-management
      direction: ingress
      action: allow
      source: "10.240.0.0/16"
      protocol: any
      priority: 40

    # Deny all other ingress
    - name: deny-all-ingress
      direction: ingress
      action: deny
      protocol: any
      priority: 999

    # Allow all egress
    - name: allow-all-egress
      direction: egress
      action: allow
      protocol: any
      priority: 100
```

**Generated nftables:**
```
table ip filter {
  chain forward {
    type filter hook forward priority 0; policy drop;
    ct state established,related accept
    iifname "uplink" meta l4proto tcp th dport 22 accept
    iifname "uplink" meta l4proto tcp th dport 80 accept
    iifname "uplink" meta l4proto tcp th dport 443 accept
    iifname "uplink" meta l4proto icmp accept
    iifname "uplink" ip saddr 10.240.0.0/16 accept
    iifname "uplink" drop
    oifname "uplink" accept
  }
}
```

**Verify firewall:**

```bash
oc exec router-fw-pod -n roks-vpc-network-operator -- nft list ruleset
```

### 8.2 Direction mapping

| `direction` | nftables match | Meaning |
|------------|---------------|---------|
| `ingress` | `iifname "uplink"` | Traffic coming from the VPC/internet into the L2 network |
| `egress` | `oifname "uplink"` | Traffic going from the L2 network out to the VPC/internet |

### 8.3 Gateway-level firewall

Firewall rules on the gateway apply to all routers that reference it:

```yaml
apiVersion: vpc.roks.ibm.com/v1alpha1
kind: VPCGateway
metadata:
  name: gw-with-fw
  namespace: roks-vpc-network-operator
spec:
  zone: "eu-de-1"
  uplink:
    network: demo-localnet
    namespace: default
  transit:
    address: "172.16.100.13/24"
  firewall:
    enabled: true
    rules:
    - name: block-ssh-from-internet
      direction: ingress
      action: deny
      protocol: tcp
      port: 22
      priority: 1
    - name: allow-all-else
      direction: ingress
      action: allow
      protocol: any
      priority: 999
```

---

## Part 9: Multi-network routing

A single router can connect multiple Layer2 networks, providing inter-network routing, DHCP, and shared internet access through one gateway.

### 9.1 Create multiple Layer2 networks

```yaml
# l2-web.yaml
apiVersion: k8s.ovn.org/v1
kind: ClusterUserDefinedNetwork
metadata:
  name: l2-web
spec:
  namespaceSelector:
    matchExpressions:
    - key: kubernetes.io/metadata.name
      operator: In
      values: [vm-demo, default]      # Include 'default' for Multus namespace isolation
  network:
    topology: Layer2
    layer2:
      role: Secondary
      subnets:
      - cidr: "10.100.0.0/24"
---
# l2-db.yaml
apiVersion: k8s.ovn.org/v1
kind: ClusterUserDefinedNetwork
metadata:
  name: l2-db
spec:
  namespaceSelector:
    matchExpressions:
    - key: kubernetes.io/metadata.name
      operator: In
      values: [vm-demo, default]      # Include 'default' for Multus namespace isolation
  network:
    topology: Layer2
    layer2:
      role: Secondary
      subnets:
      - cidr: "10.200.0.0/24"
```

```bash
oc apply -f l2-web.yaml -f l2-db.yaml
```

### 9.2 Multi-network router

```yaml
# multi-router.yaml
apiVersion: vpc.roks.ibm.com/v1alpha1
kind: VPCGateway
metadata:
  name: gw-multi
  namespace: roks-vpc-network-operator
spec:
  zone: "eu-de-1"
  uplink:
    network: demo-localnet
    namespace: default
  transit:
    address: "172.16.100.14/24"
  floatingIP:
    enabled: true
  vpcRoutes:
  - destination: "10.100.0.0/24"     # Web network
  - destination: "10.200.0.0/24"     # DB network
  nat:
    noNAT:
    - source: "10.100.0.0/24"
      destination: "10.200.0.0/24"   # Web-to-DB: no NAT
      priority: 10
    - source: "10.200.0.0/24"
      destination: "10.100.0.0/24"   # DB-to-Web: no NAT
      priority: 11
    snat:
    - source: "10.100.0.0/24"        # Web outbound: SNAT
      priority: 100
    - source: "10.200.0.0/24"        # DB outbound: SNAT
      priority: 101
---
apiVersion: vpc.roks.ibm.com/v1alpha1
kind: VPCRouter
metadata:
  name: router-multi
  namespace: roks-vpc-network-operator
spec:
  gateway: gw-multi
  networks:
  - name: l2-web
    namespace: default
    address: "10.100.0.1/24"         # Router is .1 on web network
  - name: l2-db
    namespace: default
    address: "10.200.0.1/24"         # Router is .1 on DB network
  dhcp:
    enabled: true                     # DHCP on both networks
  routeAdvertisement:
    connectedSegments: true           # Advertises 10.100.0.0/24 + 10.200.0.0/24
```

```bash
oc apply -f multi-router.yaml
```

The router pod gets three interfaces:
- `uplink` — connected to the LocalNet CUDN (VPC fabric)
- `net0` — connected to `l2-web` (10.100.0.1/24)
- `net1` — connected to `l2-db` (10.200.0.1/24)

### 9.3 VMs on each network

```yaml
# vm-web.yaml
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: web-server
  namespace: vm-demo
spec:
  running: true
  template:
    spec:
      networks:
      - name: web
        multus:
          networkName: l2-web
      domain:
        resources:
          requests: { memory: 2Gi, cpu: "1" }
        devices:
          interfaces:
          - name: web
            bridge: {}
          disks:
          - name: rootdisk
            disk: { bus: virtio }
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
---
# vm-db.yaml
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: db-server
  namespace: vm-demo
spec:
  running: true
  template:
    spec:
      networks:
      - name: db
        multus:
          networkName: l2-db
      domain:
        resources:
          requests: { memory: 2Gi, cpu: "1" }
        devices:
          interfaces:
          - name: db
            bridge: {}
          disks:
          - name: rootdisk
            disk: { bus: virtio }
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

```bash
oc apply -f vm-web.yaml -f vm-db.yaml
```

### 9.4 Verify inter-network connectivity

```bash
# SSH into web-server
virtctl ssh --namespace=vm-demo --username=fedora web-server

# From web-server, ping the DB server (cross-network, via router)
ping -c 3 10.200.0.x    # DB server's DHCP-assigned IP

# Ping the internet (outbound via SNAT)
ping -c 3 1.1.1.1

# Check the route
ip route
# default via 10.100.0.1 dev enp1s0   (DHCP-assigned default route)
```

### 9.5 Network segmentation with firewall

Add a firewall to restrict DB network access:

```yaml
spec:
  firewall:
    enabled: true
    rules:
    # Allow web servers to reach DB port
    - name: allow-db-from-web
      direction: ingress
      action: allow
      source: "10.100.0.0/24"
      destination: "10.200.0.0/24"
      protocol: tcp
      port: 5432
      priority: 10

    # Block all other access to DB network
    - name: deny-db-access
      direction: ingress
      action: deny
      destination: "10.200.0.0/24"
      protocol: any
      priority: 50

    # Allow all egress
    - name: allow-egress
      direction: egress
      action: allow
      protocol: any
      priority: 100
```

---

## Part 10: IDS/IPS with Suricata

> For a comprehensive standalone guide, see [IDS/IPS with Suricata](ids-ips-suricata.md).

The VPCRouter supports an optional Suricata sidecar container for intrusion detection (IDS) and inline intrusion prevention (IPS). IDS/IPS config lives on the VPCRouter — this is consistent with the firewall pattern where per-router security policies are configured on the router, not the gateway.

### How it works

```
                          Router Pod
  ┌────────────────────────────────────────────────┐
  │                                                │
  │   ┌──────────┐         ┌──────────────────┐    │
  │   │  router   │ ──────▶│    suricata       │    │
  │   │ container │         │    sidecar        │    │
  │   └──────────┘         └──────────────────┘    │
  │        │                     │                 │
  │   IDS: AF_PACKET (passive mirror)              │
  │   IPS: NFQUEUE (inline, fail-open)             │
  │                                                │
  └────────────────────────────────────────────────┘
```

- **IDS mode** (`mode: ids`): Suricata runs in AF_PACKET mode, passively mirroring traffic for analysis. No packets are dropped — alerts only.
- **IPS mode** (`mode: ips`): Suricata runs with NFQUEUE, inspecting forwarded packets inline. Matching `drop` rules block traffic. Uses `fail-open` so traffic flows if Suricata is unavailable.

### 10.1 Enable IDS (passive monitoring)

Add `ids` configuration to your router. This example builds on the router from Part 4.

```yaml
# demo-router-ids.yaml
apiVersion: vpc.roks.ibm.com/v1alpha1
kind: VPCRouter
metadata:
  name: demo-router-ids
  namespace: roks-vpc-network-operator
spec:
  gateway: demo-gw-routes
  networks:
  - name: demo-l2
    namespace: default
    address: "10.100.0.1/24"
  dhcp:
    enabled: true
  ids:
    enabled: true
    mode: ids
```

Apply and wait for the router to be Ready:

```bash
oc apply -f demo-router-ids.yaml
oc wait --for=jsonpath='{.status.phase}'=Ready vpcrouter/demo-router-ids -n roks-vpc-network-operator --timeout=120s
```

Verify the IDS column shows `ids` (use `-o wide` since IDS is a priority-1 column):

```bash
oc get vrt -n roks-vpc-network-operator -o wide
```

Expected output:

```
NAME              GATEWAY          PHASE   SYNC     AGE   IDS
demo-router-ids   demo-gw-routes   Ready   Synced   30s   ids
```

Verify the pod has 2 containers (router + suricata):

```bash
oc get pod demo-router-ids-pod -n roks-vpc-network-operator
```

Expected output:

```
NAME                    READY   STATUS    RESTARTS   AGE
demo-router-ids-pod     2/2     Running   0          30s
```

Check Suricata is running and streaming EVE JSON logs:

```bash
oc logs demo-router-ids-pod -n roks-vpc-network-operator -c suricata --tail=10
```

You should see JSON lines with `event_type` fields like `stats`, `flow`, etc.

**Trigger an ET Open alert** — from a VM behind this router:

```bash
# From the VM (via virtctl ssh)
curl http://testmynids.org/uid/index.html
```

Then check Suricata logs for the alert:

```bash
oc logs demo-router-ids-pod -n roks-vpc-network-operator -c suricata | grep '"alert"'
```

You should see an alert with `signature: "ET POLICY curl User-Agent Outbound"` or similar.

### 10.2 Switch to IPS (inline blocking)

Patch the router to switch from passive IDS to inline IPS:

```bash
# Record current pod UID
OLD_UID=$(oc get pod demo-router-ids-pod -n roks-vpc-network-operator -o jsonpath='{.metadata.uid}')

# Switch to IPS mode
oc patch vpcrouter demo-router-ids -n roks-vpc-network-operator --type=merge \
  -p '{"spec":{"ids":{"mode":"ips"}}}'
```

The router reconciler detects the mode change and recreates the pod:

```bash
sleep 10
oc wait --for=condition=Ready pod/demo-router-ids-pod -n roks-vpc-network-operator --timeout=120s

# Verify pod was recreated
NEW_UID=$(oc get pod demo-router-ids-pod -n roks-vpc-network-operator -o jsonpath='{.metadata.uid}')
if [ "$OLD_UID" != "$NEW_UID" ]; then echo "PASS: Pod recreated for IPS"; else echo "FAIL: Pod not recreated"; fi
```

Verify `status.idsMode` is updated:

```bash
oc get vpcrouter demo-router-ids -n roks-vpc-network-operator -o jsonpath='{.status.idsMode}'
# Output: ips
```

Verify NFQUEUE nftables rules are configured on the router container:

```bash
oc exec demo-router-ids-pod -n roks-vpc-network-operator -c router -- nft list table ip suricata
```

Expected output:

```
table ip suricata {
  chain forward_ips {
    type filter hook forward priority -10; policy accept;
    ct state established,related accept
    queue num 0 bypass
  }
}
```

The `bypass` flag ensures fail-open behavior — if Suricata is unavailable, packets are accepted rather than dropped.

### 10.3 Custom rules

Add custom Suricata rules to block specific traffic. This example drops connections to port 4444 (a common reverse shell port) and alerts on the Nikto scanner user agent:

```bash
oc patch vpcrouter demo-router-ids -n roks-vpc-network-operator --type=merge -p '
{
  "spec": {
    "ids": {
      "customRules": "drop tcp any any -> any 4444 (msg:\"Block reverse shell port\"; sid:1000001; rev:1;)\nalert http any any -> any any (msg:\"Suspicious user agent\"; content:\"nikto\"; http_user_agent; sid:1000002; rev:1;)"
    }
  }
}'
```

Wait for the pod to recreate (custom rules change triggers recreation):

```bash
sleep 10
oc wait --for=condition=Ready pod/demo-router-ids-pod -n roks-vpc-network-operator --timeout=120s
```

Verify the custom rules were written to the container:

```bash
oc exec demo-router-ids-pod -n roks-vpc-network-operator -c suricata -- cat /var/lib/suricata/rules/custom.rules
```

### 10.4 Interface selection

By default, Suricata monitors all interfaces (`all`). You can restrict monitoring to specific interfaces:

| Value | Interfaces monitored |
|-------|---------------------|
| `all` (default) | Uplink + all workload networks |
| `uplink` | Only the transit/uplink interface |
| `workload` | Only the workload network interfaces (net0, net1, ...) |

Example — monitor only workload traffic:

```bash
oc patch vpcrouter demo-router-ids -n roks-vpc-network-operator --type=merge \
  -p '{"spec":{"ids":{"interfaces":"workload"}}}'
```

### 10.5 Syslog output

Forward Suricata alerts to an external syslog server for centralized security monitoring:

```bash
oc patch vpcrouter demo-router-ids -n roks-vpc-network-operator --type=merge \
  -p '{"spec":{"ids":{"syslogTarget":"syslog.example.com:514"}}}'
```

This adds a second EVE output that sends alerts via syslog in addition to the default file-based logging streamed to stdout.

### Full IPS example

This is a complete VPCRouter spec with IPS mode, custom rules, and syslog — suitable for production use:

```yaml
# demo-router-ips.yaml
apiVersion: vpc.roks.ibm.com/v1alpha1
kind: VPCRouter
metadata:
  name: demo-router-ips
  namespace: roks-vpc-network-operator
spec:
  gateway: demo-gw-routes
  networks:
  - name: demo-l2
    namespace: default
    address: "10.100.0.1/24"
  dhcp:
    enabled: true
  ids:
    enabled: true
    mode: ips
    interfaces: all
    syslogTarget: "syslog.example.com:514"
    customRules: |
      drop tcp any any -> any 4444 (msg:"Block reverse shell port"; sid:1000001; rev:1;)
      alert http any any -> any any (msg:"Suspicious user agent"; content:"nikto"; http_user_agent; sid:1000002; rev:1;)
```

---

## Part 11: Cleanup

Delete resources in reverse order: VMs first, then routers, gateways, and finally networks.

### Delete VMs

```bash
oc delete vm demo-vm-localnet demo-vm-l2 web-server db-server -n vm-demo
```

For LocalNet VMs, the `vpc.roks.ibm.com/vm-cleanup` finalizer triggers automatic VNI and FIP deletion.

### Delete routers

```bash
oc delete vpcrouter --all -n roks-vpc-network-operator
```

The `vpc.roks.ibm.com/router-cleanup` finalizer deletes the router pod.

### Delete gateways

```bash
oc delete vpcgateway --all -n roks-vpc-network-operator
```

The `vpc.roks.ibm.com/gateway-cleanup` finalizer triggers cleanup in order:
1. Deletes PAR resources (ingress routes, routing table, PAR)
2. Deletes or unbinds floating IP
3. Deletes VPC routes
4. Deletes VLAN attachment (auto-deletes inline VNI)

### Delete networks

```bash
# Layer2 CUDNs (no VPC resources to clean up)
oc delete cudn demo-l2 l2-web l2-db

# LocalNet CUDN (operator cleans up VPC subnet and VLAN attachments)
oc delete cudn demo-localnet
```

### Verify cleanup

```bash
# All operator CRDs should be empty
oc get vpcsubnets
oc get vlanattachments
oc get vni -A
oc get fip -A
oc get vpcgateway -A
oc get vpcrouter -A

# Verify VPC resources are gone
ibmcloud is subnets --output json | jq '.[] | select(.name | startswith("roks-"))'
ibmcloud is floating-ips --output json | jq '.[] | select(.name | startswith("roks-"))'
```

### Delete namespace

```bash
oc delete namespace vm-demo
```

---

## Reference

### CRD quick reference

| CRD | Short | Scope | API Group | Description |
|-----|-------|-------|-----------|-------------|
| `VPCSubnet` | `vsn` | Cluster | `vpc.roks.ibm.com/v1alpha1` | VPC subnet |
| `VirtualNetworkInterface` | `vni` | Namespaced | `vpc.roks.ibm.com/v1alpha1` | VPC VNI for a VM |
| `VLANAttachment` | `vla` | Cluster | `vpc.roks.ibm.com/v1alpha1` | VLAN attachment on BM node |
| `FloatingIP` | `fip` | Namespaced | `vpc.roks.ibm.com/v1alpha1` | Public floating IP |
| `VPCGateway` | `vgw` | Namespaced | `vpc.roks.ibm.com/v1alpha1` | Shared VPC uplink |
| `VPCRouter` | `vrt` | Namespaced | `vpc.roks.ibm.com/v1alpha1` | L2 network router |

### VPCGateway spec fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `zone` | string | Yes | - | VPC availability zone |
| `uplink.network` | string | Yes | - | LocalNet CUDN name |
| `uplink.namespace` | string | No | - | NAD namespace |
| `uplink.securityGroupIDs` | []string | No | - | VPC security groups |
| `transit.address` | string | Yes | - | IP on transit network |
| `transit.network` | string | No | - | Transit L2 network name |
| `transit.cidr` | string | No | - | Transit CIDR |
| `vpcRoutes[].destination` | string | Yes | - | Route destination CIDR |
| `floatingIP.enabled` | bool | No | `false` | Allocate a floating IP |
| `floatingIP.id` | string | No | - | Adopt existing FIP |
| `publicAddressRange.enabled` | bool | No | `false` | Provision a PAR |
| `publicAddressRange.prefixLength` | int | No | `32` | PAR size (28-32) |
| `publicAddressRange.id` | string | No | - | Adopt existing PAR |
| `nat.snat[]` | | No | - | Source NAT rules |
| `nat.dnat[]` | | No | - | Destination NAT rules |
| `nat.noNAT[]` | | No | - | NAT exceptions |
| `firewall.enabled` | bool | No | `false` | Enable firewall |
| `firewall.rules[]` | | No | - | Firewall rules |
| `pod.image` | string | No | - | Router pod image |

### VPCRouter spec fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `gateway` | string | Yes | - | VPCGateway name |
| `transit.network` | string | No | - | Override transit network |
| `transit.address` | string | No | - | Override transit address |
| `networks[].name` | string | Yes | - | L2 network name |
| `networks[].namespace` | string | No | - | NAD namespace |
| `networks[].address` | string | Yes | - | Router's IP on network |
| `dhcp.enabled` | bool | No | `false` | Run DHCP server |
| `firewall.enabled` | bool | No | `false` | Enable firewall |
| `firewall.rules[]` | | No | - | Firewall rules |
| `routeAdvertisement.connectedSegments` | bool | No | `true` | Advertise connected CIDRs |
| `routeAdvertisement.staticRoutes` | bool | No | `false` | Advertise static routes |
| `routeAdvertisement.natIPs` | bool | No | `false` | Advertise NAT IPs |
| `ids.enabled` | bool | No | `false` | Deploy Suricata sidecar |
| `ids.mode` | string | No | `ids` | `ids` (passive) or `ips` (inline) |
| `ids.interfaces` | string | No | `all` | `all`, `uplink`, or `workload` |
| `ids.customRules` | string | No | - | Additional Suricata rules (one per line) |
| `ids.syslogTarget` | string | No | - | Syslog destination (`host:port`) |
| `ids.image` | string | No | - | Override Suricata image |
| `ids.nfqueueNum` | int | No | `0` | NFQUEUE number (IPS mode) |
| `pod.image` | string | No | - | Container image |

### Finalizers

| Finalizer | Applied to | Cleans up |
|-----------|-----------|-----------|
| `vpc.roks.ibm.com/cudn-cleanup` | CUDN | VPC subnet + VLAN attachments |
| `vpc.roks.ibm.com/vm-cleanup` | VirtualMachine | VNI + reserved IP + floating IP |
| `vpc.roks.ibm.com/gateway-cleanup` | VPCGateway | FIP + PAR + VPC routes + VLAN attachment + VNI |
| `vpc.roks.ibm.com/router-cleanup` | VPCRouter | Router pod |

### Annotation quick reference

#### CUDN annotations (user-provided)

| Annotation | Example | Description |
|-----------|---------|-------------|
| `vpc.roks.ibm.com/zone` | `eu-de-1` | VPC zone |
| `vpc.roks.ibm.com/cidr` | `10.240.64.0/24` | VPC subnet CIDR |
| `vpc.roks.ibm.com/vpc-id` | `r006-xxxx-...` | VPC ID |
| `vpc.roks.ibm.com/vlan-id` | `200` | VLAN tag |
| `vpc.roks.ibm.com/security-group-ids` | `r006-xxxx-...` | Security groups |
| `vpc.roks.ibm.com/acl-id` | `r006-xxxx-...` | Network ACL |

#### CUDN annotations (operator-written)

| Annotation | Description |
|-----------|-------------|
| `vpc.roks.ibm.com/subnet-id` | VPC subnet ID |
| `vpc.roks.ibm.com/subnet-name` | VPC subnet name |
| `vpc.roks.ibm.com/subnet-status` | `active`, `pending`, or `error` |
| `vpc.roks.ibm.com/vlan-attachments` | Node-to-attachment-ID map |

#### VM annotations (user-provided)

| Annotation | Value | Description |
|-----------|-------|-------------|
| `vpc.roks.ibm.com/fip` | `"true"` | Request a floating IP |

#### VM annotations (webhook-written)

| Annotation | Description |
|-----------|-------------|
| `vpc.roks.ibm.com/vni-id` | VNI ID |
| `vpc.roks.ibm.com/mac-address` | VPC-assigned MAC |
| `vpc.roks.ibm.com/reserved-ip` | Private IP on VPC subnet |
| `vpc.roks.ibm.com/reserved-ip-id` | Reserved IP resource ID |
| `vpc.roks.ibm.com/fip-id` | Floating IP resource ID |
| `vpc.roks.ibm.com/fip-address` | Public floating IP address |

### Status conditions

#### VPCGateway conditions

| Type | Description |
|------|-------------|
| `Ready` | Gateway fully operational |
| `VNIReady` | Uplink VNI created |
| `RoutesConfigured` | VPC routes created |
| `FloatingIPReady` | FIP bound to VNI |
| `PARReady` | PAR with ingress routing configured |

#### VPCRouter status fields

| Field | Description |
|------|-------------|
| `podIP` | IP address of the router pod (used as VPC route next-hop) |
| `advertisedRoutes` | CIDRs advertised to the gateway (auto-collected for VPC routes) |
| `idsMode` | Active IDS/IPS mode (`ids`, `ips`, or empty if disabled) |

#### VPCRouter conditions

| Type | Description |
|------|-------------|
| `TransitConnected` | Connected to gateway |
| `RoutesConfigured` | Advertising routes |
| `PodReady` | Router pod is Running |
| `IDSReady` | Suricata sidecar is running (only present when IDS is enabled) |

### Troubleshooting

```bash
# Operator logs
oc logs -l app.kubernetes.io/name=roks-vpc-network-operator -n roks-vpc-network-operator -f

# Gateway events
oc describe vpcgateway <name> -n roks-vpc-network-operator

# Router events
oc describe vpcrouter <name> -n roks-vpc-network-operator

# Router pod logs (init script output)
oc logs <router>-pod -n roks-vpc-network-operator

# Check nftables on router
oc exec <router>-pod -n roks-vpc-network-operator -- nft list ruleset

# Check interfaces on router
oc exec <router>-pod -n roks-vpc-network-operator -- ip addr

# Check DHCP leases
oc exec <router>-pod -n roks-vpc-network-operator -- cat /var/lib/misc/dnsmasq.leases

# Suricata IDS/IPS logs (EVE JSON alerts)
oc logs <router>-pod -n roks-vpc-network-operator -c suricata --tail=50

# Suricata alert summary
oc logs <router>-pod -n roks-vpc-network-operator -c suricata | grep '"event_type":"alert"'

# Suricata config inside the container
oc exec <router>-pod -n roks-vpc-network-operator -c suricata -- cat /etc/suricata/suricata.yaml

# Custom rules loaded by Suricata
oc exec <router>-pod -n roks-vpc-network-operator -c suricata -- cat /var/lib/suricata/rules/custom.rules

# NFQUEUE nftables rules (IPS mode only)
oc exec <router>-pod -n roks-vpc-network-operator -c router -- nft list table ip suricata

# Verify VPC resources via IBM Cloud CLI
ibmcloud is subnets --output json | jq '.[] | select(.name | startswith("roks-")) | {name, id, status}'
ibmcloud is floating-ips --output json | jq '.[] | select(.name | startswith("roks-")) | {name, address, target}'
ibmcloud is vpc-routing-table-routes <vpc-id> <rt-id>
```
