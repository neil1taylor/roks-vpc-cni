# Data Path

This page describes how network traffic flows between a KubeVirt VM inside the cluster and the IBM Cloud VPC fabric.

---

## Outbound Path (VM to VPC)

When a VM sends a packet to another VPC resource (or the internet via a floating IP):

```
┌──────────────────────────────────────────────────────────────┐
│                     Bare Metal Worker Node                    │
│                                                               │
│  ┌─────────┐    ┌──────────┐    ┌──────────┐    ┌────────┐ │
│  │ KubeVirt │    │  OVN     │    │ Physical │    │  PCI   │ │
│  │   VM     │───►│ br-int   │───►│ OVS      │───►│ Uplink │─┼──►
│  │ (VPC MAC)│    │ localnet │    │ Bridge   │    │        │ │
│  └─────────┘    │ port     │    │ (VLAN    │    └────────┘ │
│                  │          │    │  tag)    │               │
│                  └──────────┘    └──────────┘               │
└──────────────────────────────────────────────────────────────┘
                                                        │
                                                        ▼
                                               ┌──────────────┐
                                               │   VPC Fabric  │
                                               │               │
                                               │ MAC → VNI     │
                                               │ SG rules      │
                                               │ Route to dest │
                                               └──────────────┘
```

### Step-by-Step

1. **VM sends traffic** — The KubeVirt VM sends a frame using the VPC-assigned MAC address on its localnet network interface.

2. **OVN bridge forwarding** — OVN-Kubernetes `br-int` receives the frame. The localnet port on the bridge routes it through patch ports to the physical OVS bridge.

3. **VLAN tagging** — The physical OVS bridge tags the frame with the CUDN's VLAN ID. This VLAN ID was specified in the CUDN annotation (`vpc.roks.ibm.com/vlan-id`) and matches the VLAN attachment on the bare metal node.

4. **PCI uplink transmission** — The bare metal server's PCI network interface (uplink) forwards the VLAN-tagged frame onto the VPC fabric.

5. **VPC fabric processing** — The VPC fabric receives the frame and:
   - Matches the source MAC address to the corresponding floating VNI
   - Applies the VNI's security group rules
   - Routes the packet to its destination (another VPC resource, VPN gateway, or internet via floating IP)

---

## Inbound Path (VPC to VM)

When traffic arrives at the VM from the VPC:

```
                                               ┌──────────────┐
                                               │   VPC Fabric  │
                                               │               │
                                               │ Dest IP →     │
                                               │ Reserved IP → │
                                               │ VNI → VLAN att│
                                               └──────┬───────┘
                                                       │
                                                       ▼
┌──────────────────────────────────────────────────────────────┐
│                     Bare Metal Worker Node                    │
│                                                               │
│  ┌─────────┐    ┌──────────┐    ┌──────────┐    ┌────────┐ │
│  │ KubeVirt │    │  OVN     │    │ Physical │    │  PCI   │ │
│  │   VM     │◄───│ br-int   │◄───│ OVS      │◄───│ Uplink │◄┼──
│  │ (VPC MAC)│    │ localnet │    │ Bridge   │    │        │ │
│  └─────────┘    │ port     │    │ (untag)  │    └────────┘ │
│                  └──────────┘    └──────────┘               │
└──────────────────────────────────────────────────────────────┘
```

### Step-by-Step

1. **VPC routes to reserved IP** — The VPC fabric routes the packet to the VM's reserved IP address, which is associated with the VNI.

2. **VLAN attachment delivery** — The VPC fabric delivers the frame to the bare metal node's VLAN attachment for the matching VLAN ID and subnet.

3. **VLAN untagging** — The physical OVS bridge receives the VLAN-tagged frame, removes the VLAN tag, and forwards it to OVN's bridge.

4. **OVN delivery** — OVN-Kubernetes `br-int` receives the frame via the localnet port and delivers it to the VM's virtual network interface, matching on the destination MAC.

5. **VM receives traffic** — The KubeVirt VM receives the frame on its network interface.

---

## Floating IP Path

When a VM has a floating IP, internet traffic follows this additional path:

### Inbound (Internet to VM)

1. External client sends packet to the floating IP address (e.g., `169.48.x.x`)
2. VPC's internet gateway performs 1:1 NAT: floating IP → reserved IP
3. Packet follows the normal inbound path to the VM

### Outbound (VM to Internet)

1. VM sends packet to an internet destination
2. VPC fabric routes through the VNI's floating IP
3. 1:1 NAT: reserved IP → floating IP as the source
4. Packet exits via the VPC internet gateway

---

## MAC Address as Identity Anchor

The MAC address is the critical link between the VM (inside the cluster) and the VNI (in the VPC):

```
VPC API                          Kubernetes Cluster
────────                         ──────────────────
VNI created ──► MAC generated    Webhook reads MAC
                (02:00:04:...)   from VPC API response
                     │                    │
                     │                    ▼
                     │           VM spec.interfaces[].macAddress
                     │                    │
                     ▼                    ▼
VPC fabric matches   ◄──── Frame ────── VM sends traffic
MAC to VNI                               with this MAC
```

The VPC auto-generates a unique MAC when a VNI is created. The webhook reads this MAC and injects it into the VM's interface spec. When the VM boots, it uses this MAC on its localnet interface. The VPC fabric sees traffic with this MAC on the VLAN attachment and associates it with the correct VNI, applying security groups and routing.

---

## IP Address Assignment

The VM's private IP is assigned through one of two mechanisms:

1. **Cloud-init injection** — The webhook injects the reserved IP into the VM's cloud-init `networkData`, configuring a static IP on first boot.

2. **DHCP** — The VPC's MAC-to-reserved-IP binding enables DHCP-based assignment if cloud-init is not used.

---

## Live Migration Data Path

During KubeVirt live migration, the data path changes minimally:

1. Before migration: VM traffic flows through VLAN attachment on **Node A**
2. During migration: Memory pages are copied to **Node B** (the destination already has a VLAN attachment for this CUDN, created by the Node Reconciler)
3. After migration: The VNI "floats" from Node A's VLAN attachment to Node B's VLAN attachment
4. Traffic now flows through **Node B** — same MAC, same IP, same floating IP

The `floatable: true` setting on VLAN attachments and `auto_delete: false` on VNIs make this seamless.

---

## Key Settings That Enable This Architecture

| Setting | Where | Why |
|---------|-------|-----|
| `allow_ip_spoofing: true` | VNI | VM MAC differs from VLAN interface MAC |
| `enable_infrastructure_nat: false` | VNI | VM needs its own routable IP, not host NAT |
| `auto_delete: false` | VNI | VNI must survive live migration |
| `allow_to_float: true` | VLAN Attachment | Enables VNI floating between nodes |
| `interface_type: "vlan"` | VLAN Attachment | Software-defined, dynamic |
| Matching VLAN ID | CUDN annotation + VLAN Attachment | Connects OVN LocalNet to VPC subnet |

---

## Next Steps

- [Operator Internals](operator-internals.md) — How reconcilers manage these resources
- [Live Migration](../user-guide/live-migration.md) — User guide for live migration
- [Key Concepts](../overview/key-concepts.md) — Background on VPC networking
