# VPN Gateway Tutorial

This tutorial shows how to set up a VPCVPNGateway to create encrypted site-to-site VPN tunnels between your OpenShift cluster and remote sites using WireGuard, IPsec, or OpenVPN.

## Prerequisites

- A running OpenShift cluster with the ROKS VPC Network Operator installed
- A VPCGateway resource in the `Ready` state (provides the floating IP for the VPN tunnel endpoint)
- WireGuard key pair generated (for WireGuard protocol), pre-shared keys (for IPsec), or PKI certificates (for OpenVPN — CA cert, server cert/key)

## Part 1: WireGuard Site-to-Site VPN

### Step 1: Generate WireGuard Keys

On your local machine:

```bash
# Generate key pair for the cluster side
wg genkey | tee cluster-private.key | wg pubkey > cluster-public.key

# Generate key pair for the remote side
wg genkey | tee remote-private.key | wg pubkey > remote-public.key
```

### Step 2: Create the WireGuard Private Key Secret

```bash
kubectl create secret generic wg-vpn-key \
  --from-file=privatekey=cluster-private.key \
  -n roks-vpc-network-operator
```

### Step 3: Create the VPCVPNGateway

```yaml
apiVersion: vpc.roks.ibm.com/v1alpha1
kind: VPCVPNGateway
metadata:
  name: site-to-onprem
  namespace: roks-vpc-network-operator
spec:
  protocol: wireguard
  gatewayRef: my-gateway  # must be a Ready VPCGateway in the same namespace

  wireGuard:
    privateKey:
      name: wg-vpn-key
      key: privatekey
    listenPort: 51820

  tunnels:
    - name: onprem-dc1
      remoteEndpoint: 203.0.113.10        # public IP of the remote WireGuard peer
      remoteNetworks:
        - 10.0.0.0/24                     # CIDRs reachable via this tunnel
        - 192.168.1.0/24
      peerPublicKey: "aB3d...base64..."   # contents of remote-public.key
      tunnelAddressLocal: "10.99.0.1/30"  # local inner tunnel address
      tunnelAddressRemote: "10.99.0.2/30" # remote inner tunnel address

  mtu:
    tunnelMTU: 1420
    mssClamp: true
```

```bash
kubectl apply -f vpn-gateway.yaml
```

### Step 4: Configure the Remote Side

On the remote WireGuard peer (e.g., a Linux server or WireGuard appliance):

```ini
# /etc/wireguard/wg0.conf
[Interface]
PrivateKey = <contents of remote-private.key>
Address = 10.99.0.2/30
ListenPort = 51820

[Peer]
PublicKey = <contents of cluster-public.key>
Endpoint = <VPCGateway floating IP>:51820
AllowedIPs = 10.100.0.0/24, 10.200.0.0/24  # cluster-side networks
PersistentKeepalive = 25
```

```bash
sudo wg-quick up wg0
```

### Step 5: Verify the Tunnel

```bash
# Check VPN gateway status
kubectl get vpcvpngateway site-to-onprem -n roks-vpc-network-operator

# Check detailed status
kubectl get vpcvpngateway site-to-onprem -n roks-vpc-network-operator -o yaml

# Expected: phase=Ready, activeTunnels=1
```

The VPCGateway will automatically pick up the advertised routes (`10.0.0.0/24`, `192.168.1.0/24`) and create VPC routes pointing traffic for those CIDRs through the VPN gateway pod.

## Part 2: IPsec/StrongSwan Site-to-Site VPN

### Step 1: Create Pre-Shared Key Secrets

```bash
# Generate a random PSK
openssl rand -base64 32 > tunnel-psk.key

# Create the secret
kubectl create secret generic ipsec-psk-dc1 \
  --from-file=psk=tunnel-psk.key \
  -n roks-vpc-network-operator
```

### Step 2: Create the IPsec VPCVPNGateway

```yaml
apiVersion: vpc.roks.ibm.com/v1alpha1
kind: VPCVPNGateway
metadata:
  name: ipsec-to-onprem
  namespace: roks-vpc-network-operator
spec:
  protocol: ipsec
  gatewayRef: my-gateway

  tunnels:
    - name: dc1-tunnel
      remoteEndpoint: 198.51.100.1
      remoteNetworks:
        - 10.0.0.0/8
      presharedKey:
        name: ipsec-psk-dc1
        key: psk
      tunnelAddressLocal: "10.99.1.1/30"
      tunnelAddressRemote: "10.99.1.2/30"

  mtu:
    tunnelMTU: 1400
    mssClamp: true
```

### Step 3: Configure the Remote Side

On the remote side (e.g., a Cisco router, Fortinet firewall, or Linux StrongSwan):

**StrongSwan example** (`/etc/swanctl/swanctl.conf`):

```
connections {
  dc1-tunnel {
    remote_addrs = <VPCGateway floating IP>
    local {
      auth = psk
    }
    remote {
      auth = psk
    }
    children {
      dc1-tunnel {
        local_ts = 10.0.0.0/8
        remote_ts = 10.100.0.0/24,10.200.0.0/24
        start_action = trap
        dpd_action = restart
      }
    }
  }
}

secrets {
  ike-dc1 {
    secret = "<contents of tunnel-psk.key>"
  }
}
```

## Part 3: OpenVPN Site-to-Site + Remote Access VPN

OpenVPN provides TLS-based VPN tunnels with certificate authentication. It supports both site-to-site tunneling and remote-access client pools via the `clientSubnet` option.

### Step 1: Generate PKI

Use `easy-rsa` to create the certificate authority, server certificate, and server key:

```bash
# Install easy-rsa
sudo apt install easy-rsa   # Debian/Ubuntu
# or: brew install easy-rsa  # macOS

# Initialize PKI
make-cadir ~/ovpn-pki && cd ~/ovpn-pki
./easyrsa init-pki
./easyrsa build-ca nopass          # creates pki/ca.crt
./easyrsa gen-req server nopass    # creates pki/private/server.key, pki/reqs/server.req
./easyrsa sign-req server server   # creates pki/issued/server.crt

# Optional: generate DH parameters (omit to use ECDH instead)
./easyrsa gen-dh                   # creates pki/dh.pem

# Optional: generate TLS-Auth HMAC key
openvpn --genkey secret ta.key
```

For remote-access clients, also generate client certificates:

```bash
./easyrsa gen-req client1 nopass
./easyrsa sign-req client client1
```

### Step 2: Create Kubernetes Secrets

```bash
# Required: CA, server cert, and server key
kubectl create secret generic ovpn-ca \
  --from-file=ca.crt=pki/ca.crt \
  -n roks-vpc-network-operator

kubectl create secret generic ovpn-cert \
  --from-file=server.crt=pki/issued/server.crt \
  -n roks-vpc-network-operator

kubectl create secret generic ovpn-key \
  --from-file=server.key=pki/private/server.key \
  -n roks-vpc-network-operator

# Optional: DH parameters
kubectl create secret generic ovpn-dh \
  --from-file=dh.pem=pki/dh.pem \
  -n roks-vpc-network-operator

# Optional: TLS-Auth HMAC key
kubectl create secret generic ovpn-tls-auth \
  --from-file=ta.key=ta.key \
  -n roks-vpc-network-operator
```

### Step 3: Create the VPCVPNGateway

```yaml
apiVersion: vpc.roks.ibm.com/v1alpha1
kind: VPCVPNGateway
metadata:
  name: ovpn-to-onprem
  namespace: roks-vpc-network-operator
spec:
  protocol: openvpn
  gatewayRef: my-gateway  # must be a Ready VPCGateway in the same namespace

  openVPN:
    ca:
      name: ovpn-ca
      key: ca.crt
    cert:
      name: ovpn-cert
      key: server.crt
    key:
      name: ovpn-key
      key: server.key
    listenPort: 1194
    proto: udp              # "udp" (default) or "tcp"
    cipher: AES-256-GCM
    # Optional: DH parameters (omit to use ECDH)
    # dh:
    #   name: ovpn-dh
    #   key: dh.pem
    # Optional: TLS-Auth HMAC key
    # tlsAuth:
    #   name: ovpn-tls-auth
    #   key: ta.key
    # Optional: remote-access client IP pool
    # clientSubnet: "10.8.0.0/24"

  tunnels:
    - name: onprem-dc1
      remoteEndpoint: 203.0.113.10
      remoteNetworks:
        - 10.0.0.0/24
        - 192.168.1.0/24
      tunnelAddressLocal: "10.99.0.1/30"
      tunnelAddressRemote: "10.99.0.2/30"

  mtu:
    tunnelMTU: 1400
    mssClamp: true
```

```bash
kubectl apply -f ovpn-gateway.yaml
```

### Step 4: Configure the Remote Side

On the remote OpenVPN peer, create a client configuration file:

```ini
# client.ovpn
client
dev tun
proto udp
remote <VPCGateway floating IP> 1194
resolv-retry infinite
nobind

ca   ca.crt       # copy from pki/ca.crt
cert client1.crt  # copy from pki/issued/client1.crt (or server cert for site-to-site)
key  client1.key  # copy from pki/private/client1.key

cipher AES-256-GCM
verb 3

# If TLS-Auth is enabled on the server:
# tls-auth ta.key 1
```

```bash
sudo openvpn --config client.ovpn
```

For site-to-site mode, the remote side should also push routes to networks behind it, matching the `remoteNetworks` configured in the tunnel spec.

### Step 5: Verify the Tunnel

```bash
# Check VPN gateway status
kubectl get vpcvpngateway ovpn-to-onprem -n roks-vpc-network-operator

# Check detailed status
kubectl get vpcvpngateway ovpn-to-onprem -n roks-vpc-network-operator -o yaml

# Expected: phase=Ready, activeTunnels=1

# Check OpenVPN status log inside the pod
kubectl exec -it vpngw-ovpn-to-onprem -n roks-vpc-network-operator -- cat /run/openvpn/status.log
```

The VPCGateway will automatically pick up the advertised routes (`10.0.0.0/24`, `192.168.1.0/24`) and create VPC routes pointing traffic for those CIDRs through the VPN gateway pod.

## Multi-Tunnel Configuration

A VPN gateway can have multiple tunnels to different remote peers:

```yaml
spec:
  protocol: wireguard
  gatewayRef: my-gateway
  wireGuard:
    privateKey:
      name: wg-vpn-key
      key: privatekey

  tunnels:
    - name: onprem-dc1
      remoteEndpoint: 203.0.113.10
      remoteNetworks: ["10.0.0.0/24"]
      peerPublicKey: "key1..."
      tunnelAddressLocal: "10.99.0.1/30"
      tunnelAddressRemote: "10.99.0.2/30"

    - name: aws-vpc
      remoteEndpoint: 52.1.2.3
      remoteNetworks: ["172.31.0.0/16"]
      peerPublicKey: "key2..."
      tunnelAddressLocal: "10.99.0.5/30"
      tunnelAddressRemote: "10.99.0.6/30"

    - name: azure-vnet
      remoteEndpoint: 40.10.20.30
      remoteNetworks: ["10.200.0.0/16"]
      peerPublicKey: "key3..."
      tunnelAddressLocal: "10.99.0.9/30"
      tunnelAddressRemote: "10.99.0.10/30"
```

## How It Works

1. The VPCVPNGateway reconciler looks up the referenced VPCGateway to obtain the floating IP (tunnel endpoint)
2. It builds a privileged pod with WireGuard, StrongSwan, or OpenVPN configured using the tunnel spec
3. The pod obtains an uplink interface (net0) via Multus with the gateway VNI's MAC address pinned. It configures a static reserved IP from the gateway and policy routing (source-based rules) to ensure return traffic exits via net0, then sets up VPN tunnels to all configured remote peers
4. Remote networks from all tunnels are collected as `advertisedRoutes` in the VPN gateway status
5. The VPCGateway reconciler watches VPN gateway status and creates VPC routes for the advertised routes
6. Traffic from VMs destined for remote networks flows: VM -> OVN -> VPC route -> VPN gateway pod -> encrypted tunnel -> remote site

## Troubleshooting

```bash
# Check VPN gateway pod logs
kubectl logs -l app=vpngateway -n roks-vpc-network-operator

# Check WireGuard status inside the pod
kubectl exec -it vpngw-site-to-onprem -n roks-vpc-network-operator -- wg show

# Check IPsec tunnel status
kubectl exec -it vpngw-ipsec-to-onprem -n roks-vpc-network-operator -- swanctl --list-sas

# Check OpenVPN status log
kubectl exec -it vpngw-ovpn-to-onprem -n roks-vpc-network-operator -- cat /run/openvpn/status.log

# Check OpenVPN server config (useful for debugging)
kubectl exec -it vpngw-ovpn-to-onprem -n roks-vpc-network-operator -- cat /etc/openvpn/server-final.conf

# Verify OpenVPN process is running
kubectl exec -it vpngw-ovpn-to-onprem -n roks-vpc-network-operator -- pgrep -a openvpn

# Check VPC routes created by the gateway
kubectl get vpcgateway my-gateway -n roks-vpc-network-operator -o yaml
```
