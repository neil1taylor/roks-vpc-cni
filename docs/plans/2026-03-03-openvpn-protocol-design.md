# OpenVPN Protocol Support for VPCVPNGateway

**Date:** 2026-03-03
**Status:** Approved
**Approach:** Extend existing VPCVPNGateway CRD with `protocol: openvpn`

## Summary

Add OpenVPN as a third protocol option on the VPCVPNGateway CRD, alongside WireGuard and IPsec. Supports both site-to-site tunnels and remote-access (client-to-site) VPN. Uses UBI9 base image with openvpn installed via init script. Network reach is configurable per tunnel via existing `remoteNetworks` and `localNetworks` fields.

## CRD Changes

### Protocol Enum

Extend validation from `wireguard;ipsec` to `wireguard;ipsec;openvpn`.

### New Config Block: `VPNOpenVPNConfig`

Added to `VPCVPNGatewaySpec` alongside existing `WireGuard` and `IPsec` blocks:

```go
type VPNOpenVPNConfig struct {
    // CA is the CA certificate used to verify client and server certs.
    CA SecretKeyRef `json:"ca"`
    // Cert is the server certificate.
    Cert SecretKeyRef `json:"cert"`
    // Key is the server private key.
    Key SecretKeyRef `json:"key"`
    // DH is the Diffie-Hellman parameters file. Optional — omit to use ECDH.
    DH *SecretKeyRef `json:"dh,omitempty"`
    // TLSAuth is an HMAC key for TLS-Auth (DoS protection). Optional.
    TLSAuth *SecretKeyRef `json:"tlsAuth,omitempty"`
    // ListenPort is the OpenVPN listen port. Default: 1194.
    ListenPort *int32 `json:"listenPort,omitempty"`
    // Proto is the transport protocol: "udp" (default) or "tcp".
    Proto string `json:"proto,omitempty"`
    // Cipher is the data channel cipher. Default: "AES-256-GCM".
    Cipher string `json:"cipher,omitempty"`
    // ClientSubnet is the CIDR for the remote-access client IP pool (e.g., "10.8.0.0/24").
    ClientSubnet string `json:"clientSubnet,omitempty"`
}
```

### Tunnel Mapping

- **Site-to-site:** Each `VPNTunnel` entry becomes a CCD (client config directory) entry with `iroute` directives for the tunnel's `remoteNetworks`. The remote side authenticates via a cert signed by the same CA.
- **Remote access:** When `clientSubnet` is set, the server allocates IPs from this pool and pushes routes to clients. Individual `.ovpn` profiles reference the server's FIP + CA cert.
- **WireGuard-only fields** (`peerPublicKey`, `tunnelAddressLocal`, `tunnelAddressRemote`) are ignored for OpenVPN tunnels.
- **IPsec-only fields** (`presharedKey`) are ignored for OpenVPN tunnels.

### Validation

```go
case "openvpn":
    if vpn.Spec.OpenVPN == nil {
        return fmt.Errorf("spec.openVPN is required when protocol is openvpn")
    }
```

No per-tunnel validation beyond the common required fields (name, remoteEndpoint, remoteNetworks).

## Pod Construction

### `buildOpenVPNPod()`

Follows the established pattern (same as `buildWireGuardPod` / `buildStrongSwanPod`).

**Image:** UBI9 (same default as WireGuard via `resolveVPNImage()`).

**Volumes:**
- Required: CA cert, server cert, server key (3 Secret volumes)
- Optional: DH params, TLS-Auth key (up to 2 additional Secret volumes)

**Init script:**
1. `dnf install -y openvpn iptables`
2. `dhclient` on uplink interface (Multus-attached)
3. `sysctl -w net.ipv4.ip_forward=1`
4. Generate `/etc/openvpn/server.conf`:
   - `port`, `proto udp|tcp`, `dev tun`, `cipher`
   - `ca`, `cert`, `key` paths from mounted secrets
   - `server <clientSubnet>` when remote-access enabled
   - `push "route ..."` for each local network
   - CCD entries with `iroute` for site-to-site tunnels
5. Optional: MSS clamping via iptables (`-t mangle -A FORWARD -p tcp --tcp-flags SYN,RST SYN -j TCPMSS --clamp-mss-to-pmtu`)
6. `exec openvpn --config /etc/openvpn/server.conf` (foreground)

**Security context:** Privileged + NET_ADMIN, NET_RAW (same as WireGuard/IPsec).

**Health probes:**
- Liveness: `pgrep openvpn`
- Readiness: `test -f /run/openvpn/status.log`

**Env vars:** `VPN_TUNNELS` (JSON), `OVPN_CLIENT_SUBNET`, `OVPN_PROTO`, `OVPN_CIPHER`, `OVPN_LISTEN_PORT`

**Drift detection:** Image, Multus annotation, env vars (existing `vpnPodNeedsRecreation` logic).

## Reconciler Changes

Minimal — add third case to protocol switch:

```go
case "openvpn":
    desiredPod = buildOpenVPNPod(vpn, gw)
```

Add validation for OpenVPN config in `validateConfig()`.

All other reconciler logic (finalizer, gateway lookup, pod management, status, route advertisement) is protocol-agnostic and works unchanged.

## Console Plugin Changes

- **Create page:** Add `openvpn` to protocol dropdown. Conditionally show OpenVPN fields (CA/cert/key refs, listen port, proto, cipher, client subnet) when selected.
- **Detail page:** Add OpenVPN config card. Tunnel table shows common fields only.
- **List page:** Protocol badge — new color for `openvpn`.
- **Dashboard:** Stats aggregate automatically.

## BFF Changes

None. BFF is a dynamic client pass-through for VPCVPNGateway CRUD.

## Helm Changes

- Update CRD YAML with new OpenVPN fields and extended protocol enum.
- No new RBAC needed (same resource type).

## Out of Scope

- **PKI management / cert generation** — users provide their own CA, certs, and keys via K8s Secrets.
- **`.ovpn` profile generation** — future enhancement. Users create profiles manually.
- **Multi-protocol per CRD instance** — each VPCVPNGateway is one protocol.

## Testing

- Unit tests for `buildOpenVPNPod()` (init script, volumes, env vars, security context)
- Unit tests for OpenVPN validation in `validateConfig()`
- Reconciler tests with mock client (create/update/delete/drift)
- Console plugin: `npm run ts-check` + `npm run build`
