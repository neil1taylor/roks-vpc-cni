# Tutorial: L2 Bridge — NSX-T to OVN (GRETAP over WireGuard)

This tutorial walks through bridging an **NSX-T segment** to an **OVN LocalNet network** on a ROKS cluster using a VPCL2Bridge. The primary use case is VM migration: you have VMs on an NSX-T segment in your data center and VMs on an OVN LocalNet in your ROKS cluster, and you need them to communicate at Layer 2 — same broadcast domain, same subnet, ARP and DHCP working transparently. By the end, traffic flows bidirectionally between NSX VMs and KubeVirt VMs as if they were on the same physical switch.

> **Other environments:** While this tutorial focuses on NSX-T as the remote side, the same GRETAP-over-WireGuard approach works with any site that can run WireGuard and a Linux bridge — on-premises bare metal, another cloud, or a standalone hypervisor. Adapt the remote-side steps to your environment.
>
> **Tunnel types:** This tutorial covers `gretap-wireguard`, the only fully implemented tunnel type. Two additional types — `l2vpn` (NSX-T L2VPN appliance) and `evpn-vxlan` (FRR-based EVPN) — are planned for future releases.

## Table of contents

- [Part 1: Prerequisites](#part-1-prerequisites)
- [Part 2: How L2 bridging works](#part-2-how-l2-bridging-works)
- [Part 3: Configure the NSX-T remote peer](#part-3-configure-the-nsx-t-remote-peer)
- [Part 4: Generate WireGuard keys](#part-4-generate-wireguard-keys)
- [Part 5: Create the L2 Bridge](#part-5-create-the-l2-bridge)
- [Part 6: Verify tunnel interfaces](#part-6-verify-tunnel-interfaces)
- [Part 7: MTU tuning](#part-7-mtu-tuning)
- [Part 8: Cleanup](#part-8-cleanup)
- [Reference](#reference)

---

## Part 1: Prerequisites

### Prior setup

This tutorial assumes you have completed **Parts 1–3** of the [Gateway-Router tutorial](./gateway-router.md):

- A ROKS cluster with bare metal workers
- A namespace (`vm-demo`) with a LocalNet CUDN
- A **VPCGateway** with a floating IP (the gateway's FIP becomes the tunnel endpoint)

If you have not completed those parts, go back and do so now. The L2 bridge references a VPCGateway to obtain its public tunnel endpoint IP.

### Additional requirements

| Requirement | Why |
|-------------|-----|
| WireGuard kernel module | The bridge pod creates a `wg0` interface. Run `modprobe wireguard` on each bare metal node (or verify it loads automatically). |
| Remote peer endpoint | You need the public IP and WireGuard public key of the remote site. |
| WireGuard tools | `wg genkey` and `wg pubkey` on your local workstation (for key generation). |

### Tools

- `oc` (OpenShift CLI), logged in to the cluster
- `wg` (WireGuard tools) — `brew install wireguard-tools` on macOS

---

## Part 2: How L2 bridging works

### The encapsulation stack

```
 NSX-T Data Center                                     ROKS Cluster
 +-----------+                                         +-----------+
 |  NSX VM   |                                         |    VM     |
 | (workload)|                                         | (KubeVirt)|
 +-----+-----+                                         +-----+-----+
       |                                                      |
  NSX-T Segment                                       OVN LocalNet
  (e.g. 172.16.100.0/24)                             (same subnet)
       |                                                      |
 +-----+-----+                                         +-----+-----+
 | Bridge VM |    Encrypted tunnel (UDP 51820)          | Bridge Pod|
 | (Linux)   |<---------------------------------------->| (l2bridge)|
 | wg0+gretap|   over public internet via FIP           | wg0+gretap|
 +-----+-----+                                         +-----+-----+
       |                                                      |
  Linux Bridge (br-l2)                                 Linux Bridge (br-l2)
  joins gretap0 + ens192                               joins gretap0 + net0
  (NSX segment NIC)                                    (Multus/OVN)
```

The diagram shows the full path: **NSX VM → NSX Segment → Bridge VM (WireGuard + GRETAP) → Internet/FIP → Bridge Pod (WireGuard + GRETAP) → OVN LocalNet → KubeVirt VM**. Both sides mirror each other — a WireGuard tunnel carries GRETAP-encapsulated Ethernet frames, and a Linux bridge splices the tunnel onto the local L2 network.

The L2 bridge works in three layers:

1. **WireGuard** — creates an encrypted point-to-point tunnel between the gateway's floating IP and the remote peer. All traffic is encrypted with Curve25519 keys and transported over UDP.

2. **GRETAP** — encapsulates full Ethernet frames (including MAC headers) inside IP packets that travel through the WireGuard tunnel. This preserves L2 semantics — ARP, DHCP, and broadcast all work transparently.

3. **Linux bridge** — joins the GRETAP interface (`gretap0`) with the Multus-attached OVN LocalNet interface (`net0`). This makes remote workloads appear on the same L2 segment as cluster VMs.

### What the reconciler automates

When you create a `VPCL2Bridge`, the reconciler:

1. Adds the `vpc.roks.ibm.com/l2bridge-cleanup` finalizer
2. Validates the referenced VPCGateway exists and is Ready
3. Validates the tunnel-type-specific configuration
4. Builds a privileged pod with Multus annotation for the target network
5. The pod's init script creates the WireGuard tunnel, GRETAP interface, and Linux bridge
6. Sets status conditions (`PodReady`, `GatewayConnected`) and phase transitions

---

## Part 3: Configure the NSX-T remote peer

This part walks through setting up the WireGuard peer on the NSX-T side. You will deploy a lightweight Linux VM on the NSX segment, install WireGuard, create a GRETAP tunnel, and bridge it onto the NSX segment. Once complete, this VM becomes the tunnel endpoint that carries L2 frames between the NSX segment and the ROKS cluster.

> **Skip this part** if your remote peer is not NSX-T, or if it is already configured. The only requirement for the ROKS side is the peer's public IP and WireGuard public key (used in Part 5).

### 3.1 Deploy a Linux bridge VM on the NSX segment

In NSX Manager, identify or create the segment you want to bridge (Networking → Segments). Then deploy a lightweight Linux VM connected to that segment:

- **OS:** RHEL 9, Ubuntu 22.04, or any distro with WireGuard support
- **Networking:** The VM needs two paths:
  - **L2 side:** A NIC on the NSX segment (this NIC will be enslaved to the Linux bridge)
  - **WireGuard side:** Internet reachability to the VPCGateway FIP — either a second NIC with a routable IP, or the same NIC with outbound routing via an NSX Tier-1 gateway
- **Resources:** 1 vCPU, 1 GB RAM is sufficient — the VM only forwards packets

> **Naming convention:** We refer to the NSX segment NIC as `ens192` throughout this tutorial. Your interface name may differ (`eth0`, `ens224`, etc.) — adjust accordingly.

### 3.2 Install WireGuard on the bridge VM

SSH into the bridge VM and install WireGuard:

**RHEL 9 / CentOS Stream 9:**

```bash
sudo dnf install -y epel-release
sudo dnf install -y wireguard-tools
sudo modprobe wireguard
```

**Ubuntu 22.04:**

```bash
sudo apt update && sudo apt install -y wireguard
sudo modprobe wireguard
```

Generate a key pair on the bridge VM:

```bash
wg genkey | sudo tee /etc/wireguard/private.key | wg pubkey | sudo tee /etc/wireguard/public.key
sudo chmod 600 /etc/wireguard/private.key
```

Note the public key — you will provide it to the ROKS side in Part 5:

```bash
cat /etc/wireguard/public.key
# e.g.: aBcDeFgHiJkLmNoPqRsTuVwXyZ0123456789abcd=
```

### 3.3 Configure WireGuard on the bridge VM

Create `/etc/wireguard/wg0.conf`:

```ini
[Interface]
Address = 10.99.0.2/30
ListenPort = 51820
PrivateKey = <contents of /etc/wireguard/private.key>

[Peer]
PublicKey = <ROKS-side public key from Part 4>
Endpoint = <VPCGateway FIP>:51820
AllowedIPs = 0.0.0.0/0
PersistentKeepalive = 25
```

Key values:
- `Address = 10.99.0.2/30` — must match `tunnelAddressRemote` in the VPCL2Bridge spec
- `Endpoint` — the VPCGateway's floating IP (from your gateway-router tutorial)
- `PublicKey` — the ROKS-side public key you generate in Part 4
- `PersistentKeepalive = 25` — keeps the tunnel alive through NAT/firewalls

Bring the tunnel up and verify:

```bash
sudo wg-quick up wg0
sudo wg show
```

You should see the interface with `listening port: 51820`. The handshake will not complete until the ROKS-side bridge pod is created (Part 5).

### 3.4 Configure GRETAP and Linux bridge on the bridge VM

Create the GRETAP tunnel over the WireGuard interface, then bridge it with the NSX segment NIC:

```bash
# Create GRETAP tunnel (L2 over the WireGuard L3 tunnel)
sudo ip link add gretap0 type gretap local 10.99.0.2 remote 10.99.0.1 ttl 255
sudo ip link set gretap0 mtu 1400
sudo ip link set gretap0 up

# Create Linux bridge joining GRETAP and the NSX segment NIC
sudo ip link add br-l2 type bridge
sudo ip link set gretap0 master br-l2
sudo ip link set ens192 master br-l2    # ens192 = your NSX segment NIC
sudo ip link set br-l2 up
```

Key values:
- `local 10.99.0.2` — the bridge VM's WireGuard tunnel IP
- `remote 10.99.0.1` — the ROKS-side WireGuard tunnel IP (matches `tunnelAddressLocal`)
- `mtu 1400` — must match `spec.mtu.tunnelMTU` in the VPCL2Bridge spec
- `ens192` — replace with your actual NSX segment NIC name

Verify the bridge members:

```bash
bridge link show
# Should show both gretap0 and ens192 as members of br-l2
```

> **Do not assign an IP to `br-l2`** unless the bridge VM itself needs to participate in the bridged subnet. The bridge is purely a forwarding device.

### 3.5 NSX-T segment configuration

The bridge VM forwards frames with foreign MAC addresses (KubeVirt VM MACs). NSX-T's default security policies will drop these unless you allow them:

**Segment port security (NSX Manager → Networking → Segments → your segment → Ports):**

| Setting | Value | Why |
|---------|-------|-----|
| Promiscuous mode | Allow | Bridge VM must receive frames for all MACs on the segment |
| Forged transmits | Allow | Bridge VM sends frames with KubeVirt VM source MACs |
| MAC changes | Allow | Bridge VM's port learns remote MACs dynamically |

**Firewall rules:**

- **NSX DFW (Distributed Firewall):** Add a rule allowing all traffic between the bridge VM and VMs on the segment, or place the bridge VM in an exclusion list
- **Outbound:** Allow UDP 51820 from the bridge VM to the VPCGateway FIP
- **Inbound:** Allow UDP 51820 from the VPCGateway FIP to the bridge VM (if the bridge VM has a public IP; otherwise NAT/firewall rules on your edge)

### 3.6 Make persistent (optional)

The WireGuard tunnel persists across reboots via systemd:

```bash
sudo systemctl enable wg-quick@wg0
```

For the GRETAP and bridge setup, create a systemd service at `/etc/systemd/system/l2bridge.service`:

```ini
[Unit]
Description=GRETAP L2 Bridge
After=wg-quick@wg0.service
Requires=wg-quick@wg0.service

[Service]
Type=oneshot
RemainAfterExit=yes
ExecStart=/usr/sbin/ip link add gretap0 type gretap local 10.99.0.2 remote 10.99.0.1 ttl 255
ExecStart=/usr/sbin/ip link set gretap0 mtu 1400
ExecStart=/usr/sbin/ip link set gretap0 up
ExecStart=/usr/sbin/ip link add br-l2 type bridge
ExecStart=/usr/sbin/ip link set gretap0 master br-l2
ExecStart=/usr/sbin/ip link set ens192 master br-l2
ExecStart=/usr/sbin/ip link set br-l2 up
ExecStop=/usr/sbin/ip link del br-l2
ExecStop=/usr/sbin/ip link del gretap0

[Install]
WantedBy=multi-user.target
```

Enable it:

```bash
sudo systemctl daemon-reload
sudo systemctl enable l2bridge.service
```

---

## Part 4: Generate WireGuard keys

WireGuard uses Curve25519 key pairs. You need a key pair for the ROKS side, and the NSX bridge VM needs its own key pair (generated in Part 3). The two sides exchange public keys.

### 4.1 Generate the ROKS-side key pair

```bash
# Generate private key
wg genkey | tee roks-private.key | wg pubkey > roks-public.key

# View the public key (this goes into the NSX bridge VM's wg0.conf)
cat roks-public.key
```

### 4.2 Create a Kubernetes Secret

The bridge pod mounts the private key from a Secret. The key must be stored under the key name `privateKey`:

```bash
oc create secret generic wg-bridge-key \
  --from-file=privateKey=./roks-private.key \
  -n vm-demo
```

Verify:

```bash
oc get secret wg-bridge-key -n vm-demo
```

### 4.3 Exchange keys with the NSX bridge VM

The NSX bridge VM provides (from Part 3):
- Its **WireGuard public key** (`/etc/wireguard/public.key`) → goes into `spec.remote.wireGuard.peerPublicKey`
- Its **public IP** (or NAT IP reachable from the internet) → goes into `spec.remote.endpoint`

You provide to the NSX bridge VM's `/etc/wireguard/wg0.conf` (Part 3, step 3.3):
- The **ROKS-side public key** (from `roks-public.key`) → goes into `[Peer] PublicKey`
- The **VPCGateway floating IP** → goes into `[Peer] Endpoint`

> **Security:** Delete the private key file from your workstation after creating the Secret. The key only needs to exist in the Kubernetes Secret.
>
> ```bash
> rm roks-private.key
> ```

---

## Part 5: Create the L2 Bridge

### 5.1 Write the VPCL2Bridge manifest

```yaml
# demo-l2bridge.yaml
apiVersion: vpc.roks.ibm.com/v1alpha1
kind: VPCL2Bridge
metadata:
  name: demo-bridge
  namespace: vm-demo
spec:
  # Tunnel technology — only "gretap-wireguard" is fully implemented
  type: gretap-wireguard

  # Reference to the VPCGateway (must be in the same namespace, must be Ready)
  gatewayRef: demo-gateway

  # The network this bridge extends to the remote site
  networkRef:
    name: demo-localnet                   # CUDN name
    kind: ClusterUserDefinedNetwork       # default; can also be UserDefinedNetwork

  # Remote peer configuration
  remote:
    endpoint: "203.0.113.50"              # NSX bridge VM's public IP
    wireGuard:
      privateKey:
        name: wg-bridge-key              # Secret created in Part 4
        key: privateKey                   # Key within the Secret
      peerPublicKey: "REMOTE_PEER_PUBLIC_KEY_BASE64="  # NSX bridge VM's WG public key
      tunnelAddressLocal: "10.99.0.1/30"  # Local WireGuard tunnel IP
      tunnelAddressRemote: "10.99.0.2/30" # Remote WireGuard tunnel IP
      listenPort: 51820                   # UDP port (default: 51820)

  # MTU settings (optional — defaults are usually fine)
  mtu:
    tunnelMTU: 1400                       # Tunnel interface MTU (default: 1400)
    mssClamp: true                        # TCP MSS clamping to prevent fragmentation (default: true)
```

Replace:
- `demo-gateway` with your VPCGateway name from the gateway-router tutorial
- `demo-localnet` with your CUDN name
- `203.0.113.50` with the NSX bridge VM's public IP
- `REMOTE_PEER_PUBLIC_KEY_BASE64=` with the NSX bridge VM's WireGuard public key (from Part 3, step 3.2)
- `10.99.0.1/30` and `10.99.0.2/30` with the tunnel addressing chosen in Part 3 (must match the bridge VM's `wg0.conf`)

### 5.2 Apply the manifest

```bash
oc apply -f demo-l2bridge.yaml
```

### What the reconciler does:

1. Adds the `vpc.roks.ibm.com/l2bridge-cleanup` finalizer to the VPCL2Bridge
2. Looks up the VPCGateway `demo-gateway` — verifies it exists and has `status.phase: Ready`
3. Validates that `spec.remote.wireGuard` is present (required for `gretap-wireguard` type)
4. Builds a privileged pod (`l2bridge-demo-bridge`) with:
   - Multus annotation attaching it to the `demo-localnet` network as interface `net0`
   - The WireGuard private key mounted from the Secret at `/run/secrets/wireguard/privateKey`
   - An init script that creates `wg0`, `gretap0`, `br-l2`, and optional nftables MSS rules
   - Liveness probe checking `br-l2` exists, readiness probe checking `wg0` and `br-l2` are UP
5. Creates the pod in the `vm-demo` namespace
6. Sets `status.phase` to `Provisioning`, then `Established` once the pod is Running

### 5.3 Watch the phase transition

```bash
oc get vlb demo-bridge -n vm-demo -w
```

Expected output:

```
NAME          TYPE                NETWORK        REMOTE          PHASE          SYNC     AGE
demo-bridge   gretap-wireguard   demo-localnet  203.0.113.50    Pending        Pending  0s
demo-bridge   gretap-wireguard   demo-localnet  203.0.113.50    Provisioning   Synced   2s
demo-bridge   gretap-wireguard   demo-localnet  203.0.113.50    Established    Synced   30s
```

### 5.4 Check the bridge pod

```bash
oc get pods -l app=l2bridge -n vm-demo
```

Expected output:

```
NAME                      READY   STATUS    RESTARTS   AGE
l2bridge-demo-bridge      1/1     Running   0          45s
```

### 5.5 Check events

```bash
oc describe vlb demo-bridge -n vm-demo | tail -10
```

You should see events:
```
Events:
  Type    Reason      Age   From                       Message
  ----    ------      ----  ----                       -------
  Normal  PodCreated  1m    vpcl2bridge-controller     Created bridge pod l2bridge-demo-bridge
  Normal  Synced      1m    vpcl2bridge-controller     Bridge demo-bridge is Established
```

---

## Part 6: Verify tunnel interfaces

Once the pod is Running, inspect the tunnel interfaces to confirm everything is wired correctly.

### 6.1 Identify the pod

```bash
BRIDGE_POD=$(oc get vlb demo-bridge -n vm-demo -o jsonpath='{.status.podName}')
echo $BRIDGE_POD
# l2bridge-demo-bridge
```

### 6.2 Inspect WireGuard

```bash
oc exec $BRIDGE_POD -n vm-demo -- wg show
```

Expected output:

```
interface: wg0
  public key: <your-roks-public-key>
  private key: (hidden)
  listening port: 51820

peer: REMOTE_PEER_PUBLIC_KEY_BASE64=
  endpoint: 203.0.113.50:51820
  allowed ips: 0.0.0.0/0
  latest handshake: 23 seconds ago
  transfer: 1.24 KiB received, 0.86 KiB sent
```

Key things to verify:
- `listening port` matches `spec.remote.wireGuard.listenPort`
- `peer` shows the correct remote public key
- `endpoint` shows the remote peer's IP and port
- `latest handshake` has a recent timestamp (confirms the tunnel is active)

> **No handshake?** If `latest handshake` is missing, the NSX bridge VM may not be configured yet (Part 3), or a firewall is blocking UDP 51820. Check VPC security groups, NSX DFW rules, and network ACLs on both sides.

### 6.3 Inspect GRETAP

```bash
oc exec $BRIDGE_POD -n vm-demo -- ip link show gretap0
```

Expected output:

```
4: gretap0@NONE: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1400 qdisc noqueue master br-l2 state UNKNOWN
    link/ether <mac> brd ff:ff:ff:ff:ff:ff
```

Verify:
- State is `UP`
- MTU matches `spec.mtu.tunnelMTU` (default 1400)
- `master br-l2` — the interface is enslaved to the Linux bridge

### 6.4 Inspect the Linux bridge

```bash
oc exec $BRIDGE_POD -n vm-demo -- ip link show br-l2
```

Expected output:

```
5: br-l2: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1400 qdisc noqueue state UP
    link/ether <mac> brd ff:ff:ff:ff:ff:ff
```

Check which interfaces are enslaved to the bridge:

```bash
oc exec $BRIDGE_POD -n vm-demo -- bridge link show
```

You should see both `gretap0` and `net0` as members of `br-l2`:

```
4: gretap0: <BROADCAST,MULTICAST,UP,LOWER_UP> ... master br-l2
3: net0@if...: <BROADCAST,MULTICAST,UP,LOWER_UP> ... master br-l2
```

### 6.5 Check the bridge forwarding table

```bash
oc exec $BRIDGE_POD -n vm-demo -- bridge fdb show dev gretap0
```

This shows MAC addresses learned from the NSX side via the GRETAP tunnel. If the NSX segment has active VMs, you should see their MAC addresses here.

### 6.6 Verify MSS clamping

If `spec.mtu.mssClamp` is `true` (the default), the pod installs nftables rules to clamp TCP MSS:

```bash
oc exec $BRIDGE_POD -n vm-demo -- nft list ruleset
```

Expected output:

```
table inet mangle {
    chain forward {
        type filter hook forward priority mangle; policy accept;
        tcp flags syn / syn,rst tcp option maxseg size set 1360
    }
}
```

The MSS value is `tunnelMTU - 40` (20 bytes for IP header + 20 bytes for TCP header). With the default 1400 MTU, MSS is clamped to 1360.

### 6.7 Test L2 connectivity

With both sides configured, test end-to-end L2 reachability:

```bash
# From the ROKS bridge pod, arping an NSX VM on the bridged segment
oc exec $BRIDGE_POD -n vm-demo -- arping -c 3 -I br-l2 <nsx-vm-ip>
```

If arping succeeds, Ethernet frames are traversing the tunnel. Now test from the VMs themselves:

```bash
# From a KubeVirt VM, ping an NSX VM
virtctl ssh --username=fedora vm/demo-vm -n vm-demo -c 'ping -c 3 <nsx-vm-ip>'

# From an NSX VM, ping a KubeVirt VM
ssh user@<nsx-vm-ip> 'ping -c 3 <kubevirt-vm-ip>'
```

> **No response?** Check: (1) NSX segment port security allows promiscuous mode (Part 3, step 3.5), (2) both bridge forwarding tables show learned MACs (`bridge fdb show`), (3) WireGuard handshake is recent on both sides (`wg show`).

---

## Part 7: MTU tuning

The default tunnel MTU of 1400 accounts for encapsulation overhead: **WireGuard adds ~60 bytes** (40 bytes UDP/IP + 16 bytes WireGuard header + padding) and **GRETAP adds ~38 bytes** (4 bytes GRE + 14 bytes inner Ethernet + 20 bytes outer IP). The total overhead is ~98 bytes from a standard 1500-byte MTU.

### 7.1 Change the tunnel MTU

If your VPC fabric supports jumbo frames (MTU 9000), you can increase the tunnel MTU:

```bash
oc patch vlb demo-bridge -n vm-demo --type merge -p '{"spec":{"mtu":{"tunnelMTU":1402}}}'
```

The reconciler will detect the spec change and recreate the pod with the new MTU. Watch the phase transition:

```bash
oc get vlb demo-bridge -n vm-demo -w
```

### 7.2 Disable MSS clamping

If you handle fragmentation at a different layer (e.g., the remote peer does path MTU discovery), you can disable MSS clamping:

```bash
oc patch vlb demo-bridge -n vm-demo --type merge -p '{"spec":{"mtu":{"mssClamp":false}}}'
```

Verify no nftables rules are installed:

```bash
BRIDGE_POD=$(oc get vlb demo-bridge -n vm-demo -o jsonpath='{.status.podName}')
oc exec $BRIDGE_POD -n vm-demo -- nft list ruleset
```

The output should be empty (no `table inet mangle`).

### 7.3 MTU calculation guide

| VPC MTU | Overhead | Recommended tunnelMTU | Resulting MSS |
|---------|----------|-----------------------|---------------|
| 1500    | ~98 B    | 1400 (default)        | 1360          |
| 9000    | ~98 B    | 8900                  | 8860          |

> **Rule of thumb:** Set `tunnelMTU` to your VPC fabric MTU minus 100. The MSS clamp automatically adjusts.

---

## Part 8: Cleanup

### 8.1 Delete the L2 Bridge

```bash
oc delete vlb demo-bridge -n vm-demo
```

**What happens:**

1. Kubernetes sets `deletionTimestamp` on the VPCL2Bridge
2. The reconciler detects the deletion and runs `reconcileDelete`:
   - Deletes the bridge pod `l2bridge-demo-bridge`
   - Removes the `vpc.roks.ibm.com/l2bridge-cleanup` finalizer
3. Kubernetes garbage-collects the VPCL2Bridge object

### 8.2 Verify cleanup

```bash
# Pod should be gone
oc get pods -l app=l2bridge -n vm-demo
# No resources found

# VLB should be gone
oc get vlb -n vm-demo
# No resources found
```

### 8.3 Delete the WireGuard Secret

```bash
oc delete secret wg-bridge-key -n vm-demo
```

### 8.4 Clean up key files

```bash
rm -f roks-private.key roks-public.key
```

---

## Reference

### VPCL2Bridge spec fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `spec.type` | `string` | Yes | — | Tunnel technology: `gretap-wireguard`, `l2vpn`, or `evpn-vxlan` |
| `spec.gatewayRef` | `string` | Yes | — | Name of the VPCGateway in the same namespace |
| `spec.networkRef.name` | `string` | Yes | — | Name of the CUDN or UDN to bridge |
| `spec.networkRef.kind` | `string` | No | `ClusterUserDefinedNetwork` | `ClusterUserDefinedNetwork` or `UserDefinedNetwork` |
| `spec.networkRef.namespace` | `string` | No | — | Required when kind is `UserDefinedNetwork` |
| `spec.remote.endpoint` | `string` | Yes | — | Remote peer's public IP or hostname |
| `spec.remote.wireGuard.privateKey.name` | `string` | Yes | — | Name of the Secret containing the WireGuard private key |
| `spec.remote.wireGuard.privateKey.key` | `string` | Yes | — | Key within the Secret (e.g., `privateKey`) |
| `spec.remote.wireGuard.peerPublicKey` | `string` | Yes | — | Remote peer's WireGuard public key (base64) |
| `spec.remote.wireGuard.tunnelAddressLocal` | `string` | Yes | — | Local WireGuard IP with prefix (e.g., `10.99.0.1/30`) |
| `spec.remote.wireGuard.tunnelAddressRemote` | `string` | Yes | — | Remote WireGuard IP with prefix (e.g., `10.99.0.2/30`) |
| `spec.remote.wireGuard.listenPort` | `int32` | No | `51820` | WireGuard UDP listen port |
| `spec.mtu.tunnelMTU` | `int32` | No | `1400` | MTU for the GRETAP tunnel interface (range: 1200–9000) |
| `spec.mtu.mssClamp` | `bool` | No | `true` | Enable TCP MSS clamping via nftables |
| `spec.pod.image` | `string` | No | `registry.access.redhat.com/ubi9/ubi:latest` | Container image for the bridge pod |

### Status fields

| Field | Type | Description |
|-------|------|-------------|
| `status.phase` | `string` | Lifecycle phase: `Pending`, `Provisioning`, `Established`, `Degraded`, `Error` |
| `status.podName` | `string` | Name of the bridge pod (e.g., `l2bridge-demo-bridge`) |
| `status.tunnelEndpoint` | `string` | Local tunnel endpoint IP |
| `status.remoteMACsLearned` | `int32` | Number of remote MAC addresses learned via the tunnel |
| `status.localMACsAdvertised` | `int32` | Number of local MAC addresses advertised to the remote peer |
| `status.bytesIn` | `int64` | Total bytes received through the tunnel |
| `status.bytesOut` | `int64` | Total bytes sent through the tunnel |
| `status.lastHandshake` | `Time` | Timestamp of the last successful WireGuard handshake |
| `status.syncStatus` | `string` | Sync state: `Synced`, `Pending`, `Failed` |
| `status.lastSyncTime` | `Time` | Timestamp of the last successful reconciliation |
| `status.message` | `string` | Human-readable status message |

### Conditions

| Condition | Meaning |
|-----------|---------|
| `PodReady` | `True` when the bridge pod is Running; `False` when the pod is not yet ready (reason: `PodNotReady`) |
| `GatewayConnected` | `True` when the referenced VPCGateway is Ready (reason: `GatewayReady`) |

### Environment variables (bridge pod)

These are set on the bridge container by `buildBridgeEnvVars()`:

| Variable | Source | Example |
|----------|--------|---------|
| `WG_LOCAL_ADDR` | `spec.remote.wireGuard.tunnelAddressLocal` | `10.99.0.1/30` |
| `WG_REMOTE_ENDPOINT` | `spec.remote.endpoint` | `203.0.113.50` |
| `WG_PEER_PUBLIC_KEY` | `spec.remote.wireGuard.peerPublicKey` | `abc123...=` |
| `WG_LISTEN_PORT` | `spec.remote.wireGuard.listenPort` | `51820` |
| `GRETAP_LOCAL` | tunnelAddressLocal (without prefix) | `10.99.0.1` |
| `GRETAP_REMOTE` | tunnelAddressRemote (without prefix) | `10.99.0.2` |
| `TUNNEL_MTU` | `spec.mtu.tunnelMTU` | `1400` |
| `MSS_CLAMP` | `spec.mtu.mssClamp` | `true` |

### Troubleshooting

**Pod not starting:**

```bash
# Check pod events
oc describe pod l2bridge-demo-bridge -n vm-demo

# Check pod logs
oc logs l2bridge-demo-bridge -n vm-demo

# Check bridge status
oc describe vlb demo-bridge -n vm-demo
```

**WireGuard tunnel not establishing:**

```bash
# Check if WireGuard module is loaded on the node
oc debug node/<node-name> -- chroot /host modprobe wireguard

# Check WireGuard status
oc exec l2bridge-demo-bridge -n vm-demo -- wg show

# Verify UDP 51820 is allowed in VPC security groups
ibmcloud is security-group-rules <sg-id>
```

**No traffic flowing:**

```bash
# Check bridge members
oc exec l2bridge-demo-bridge -n vm-demo -- bridge link show

# Check interface states
oc exec l2bridge-demo-bridge -n vm-demo -- ip link

# Check for packet counters on WireGuard
oc exec l2bridge-demo-bridge -n vm-demo -- wg show wg0

# Check forwarding table
oc exec l2bridge-demo-bridge -n vm-demo -- bridge fdb show
```

**MTU issues (packet fragmentation or drops):**

```bash
# Check current MTU on all interfaces
oc exec l2bridge-demo-bridge -n vm-demo -- ip link | grep mtu

# Check nftables MSS rules
oc exec l2bridge-demo-bridge -n vm-demo -- nft list ruleset

# Test with reduced packet size from a VM
ping -M do -s 1360 <remote-workload-ip>
```
