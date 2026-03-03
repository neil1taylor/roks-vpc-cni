# OpenVPN Protocol Support — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add `protocol: openvpn` as a third protocol option on VPCVPNGateway, with full pod construction, validation, console UI, and tests.

**Architecture:** Extend the existing VPCVPNGateway CRD with a new `VPNOpenVPNConfig` type and `buildOpenVPNPod()` function following the identical pattern used by WireGuard and StrongSwan. Console plugin gets conditional OpenVPN fields on create/detail pages.

**Tech Stack:** Go (controller-runtime), TypeScript/React (PatternFly 5), Helm CRD YAML

---

### Task 1: Add VPNOpenVPNConfig type and extend CRD spec

**Files:**
- Modify: `roks-vpc-network-operator/api/v1alpha1/vpcvpngateway_types.go:148-192`

**Step 1: Add VPNOpenVPNConfig type**

Insert after `VPNGatewayMTU` (after line 148), before `VPCVPNGatewaySpec`:

```go
// VPNOpenVPNConfig defines global OpenVPN configuration for the VPN gateway.
type VPNOpenVPNConfig struct {
	// CA is a reference to a Secret containing the CA certificate.
	// +kubebuilder:validation:Required
	CA SecretKeyRef `json:"ca"`

	// Cert is a reference to a Secret containing the server certificate.
	// +kubebuilder:validation:Required
	Cert SecretKeyRef `json:"cert"`

	// Key is a reference to a Secret containing the server private key.
	// +kubebuilder:validation:Required
	Key SecretKeyRef `json:"key"`

	// DH is a reference to a Secret containing Diffie-Hellman parameters.
	// Optional — omit to use ECDH.
	// +optional
	DH *SecretKeyRef `json:"dh,omitempty"`

	// TLSAuth is a reference to a Secret containing the TLS-Auth HMAC key.
	// +optional
	TLSAuth *SecretKeyRef `json:"tlsAuth,omitempty"`

	// ListenPort is the OpenVPN listen port.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:default=1194
	// +optional
	ListenPort *int32 `json:"listenPort,omitempty"`

	// Proto is the transport protocol: "udp" (default) or "tcp".
	// +kubebuilder:validation:Enum=udp;tcp
	// +kubebuilder:default=udp
	// +optional
	Proto string `json:"proto,omitempty"`

	// Cipher is the data channel cipher.
	// +kubebuilder:default="AES-256-GCM"
	// +optional
	Cipher string `json:"cipher,omitempty"`

	// ClientSubnet is the CIDR for the remote-access client IP pool (e.g., "10.8.0.0/24").
	// Required when remoteAccess is enabled.
	// +optional
	ClientSubnet string `json:"clientSubnet,omitempty"`
}
```

**Step 2: Extend protocol enum and add OpenVPN field**

In `VPCVPNGatewaySpec`, change line 154 from:
```go
// +kubebuilder:validation:Enum=wireguard;ipsec
```
to:
```go
// +kubebuilder:validation:Enum=wireguard;ipsec;openvpn
```

Add after `IPsec` field (after line 170):
```go
	// OpenVPN contains global OpenVPN configuration.
	// Required when protocol is "openvpn".
	// +optional
	OpenVPN *VPNOpenVPNConfig `json:"openVPN,omitempty"`
```

**Step 3: Verify compilation**

Run: `cd roks-vpc-network-operator && go build ./...`
Expected: SUCCESS

**Step 4: Commit**

```bash
git add roks-vpc-network-operator/api/v1alpha1/vpcvpngateway_types.go
git commit -m "feat(vpngateway): add VPNOpenVPNConfig type and extend protocol enum"
```

---

### Task 2: Add OpenVPN pod construction

**Files:**
- Modify: `roks-vpc-network-operator/pkg/controller/vpngateway/pod.go`

**Step 1: Add default OpenVPN listen port constant**

At line 18, add:
```go
	defaultOpenVPNPort    = int32(1194)
```

**Step 2: Add `resolveOpenVPNImage()` function**

After `resolveStrongSwanImage()` (after line 482):

```go
// resolveOpenVPNImage determines the container image for the OpenVPN pod.
// Falls back to the UBI9 default (same as WireGuard).
func resolveOpenVPNImage(vpn *v1alpha1.VPCVPNGateway) string {
	return resolveVPNImage(vpn)
}

// resolveOpenVPNPort returns the listen port from the OpenVPN spec or the default.
func resolveOpenVPNPort(vpn *v1alpha1.VPCVPNGateway) int32 {
	if vpn.Spec.OpenVPN != nil && vpn.Spec.OpenVPN.ListenPort != nil {
		return *vpn.Spec.OpenVPN.ListenPort
	}
	return defaultOpenVPNPort
}

// resolveOpenVPNProto returns the transport protocol from the OpenVPN spec or "udp".
func resolveOpenVPNProto(vpn *v1alpha1.VPCVPNGateway) string {
	if vpn.Spec.OpenVPN != nil && vpn.Spec.OpenVPN.Proto != "" {
		return vpn.Spec.OpenVPN.Proto
	}
	return "udp"
}

// resolveOpenVPNCipher returns the cipher from the OpenVPN spec or "AES-256-GCM".
func resolveOpenVPNCipher(vpn *v1alpha1.VPCVPNGateway) string {
	if vpn.Spec.OpenVPN != nil && vpn.Spec.OpenVPN.Cipher != "" {
		return vpn.Spec.OpenVPN.Cipher
	}
	return "AES-256-GCM"
}
```

**Step 3: Add `buildOpenVPNEnvVars()` function**

```go
// buildOpenVPNEnvVars constructs environment variables for the OpenVPN container.
func buildOpenVPNEnvVars(vpn *v1alpha1.VPCVPNGateway) []corev1.EnvVar {
	tunnelsJSON := buildTunnelsJSON(vpn)
	tunnelMTU := resolveTunnelMTU(vpn)

	mssClamp := "true"
	if vpn.Spec.MTU != nil && vpn.Spec.MTU.MSSClamp != nil && !*vpn.Spec.MTU.MSSClamp {
		mssClamp = "false"
	}

	clientSubnet := ""
	if vpn.Spec.OpenVPN != nil {
		clientSubnet = vpn.Spec.OpenVPN.ClientSubnet
	}

	return []corev1.EnvVar{
		{Name: "OVPN_LISTEN_PORT", Value: fmt.Sprintf("%d", resolveOpenVPNPort(vpn))},
		{Name: "OVPN_PROTO", Value: resolveOpenVPNProto(vpn)},
		{Name: "OVPN_CIPHER", Value: resolveOpenVPNCipher(vpn)},
		{Name: "OVPN_CLIENT_SUBNET", Value: clientSubnet},
		{Name: "VPN_TUNNELS", Value: tunnelsJSON},
		{Name: "TUNNEL_MTU", Value: fmt.Sprintf("%d", tunnelMTU)},
		{Name: "MSS_CLAMP", Value: mssClamp},
	}
}
```

**Step 4: Add `buildOpenVPNInitScript()` function**

```go
// buildOpenVPNInitScript generates the bash init script for the OpenVPN pod.
func buildOpenVPNInitScript(vpn *v1alpha1.VPCVPNGateway) string {
	var sb strings.Builder
	sb.WriteString("set -e\n\n")

	// Install OpenVPN
	sb.WriteString("# Install OpenVPN\n")
	sb.WriteString("dnf install -y epel-release 2>/dev/null || true\n")
	sb.WriteString("dnf install -y openvpn iptables jq 2>/dev/null || ")
	sb.WriteString("yum install -y epel-release && yum install -y openvpn iptables jq 2>/dev/null || ")
	sb.WriteString("apt-get update && apt-get install -y openvpn iptables jq 2>/dev/null || true\n\n")

	// Uplink via DHCP
	sb.WriteString("# Uplink via DHCP\n")
	sb.WriteString("dhclient net0 2>/dev/null || true\n\n")

	// Enable IP forwarding
	sb.WriteString("# Enable IP forwarding\n")
	sb.WriteString("sysctl -w net.ipv4.ip_forward=1\n\n")

	// Create CCD directory for site-to-site tunnels
	sb.WriteString("# Create client config directory\n")
	sb.WriteString("mkdir -p /etc/openvpn/ccd\n\n")

	// Generate CCD entries from VPN_TUNNELS
	sb.WriteString("# Generate CCD entries for site-to-site tunnels\n")
	sb.WriteString("echo \"${VPN_TUNNELS}\" | jq -c '.[]' | while read -r tunnel; do\n")
	sb.WriteString("  TNAME=$(echo \"$tunnel\" | jq -r '.name')\n")
	sb.WriteString("  ROUTES=$(echo \"$tunnel\" | jq -r '.remoteNetworks[]')\n")
	sb.WriteString("  for cidr in $ROUTES; do\n")
	sb.WriteString("    IFS='/' read -r net prefix <<< \"$cidr\"\n")
	sb.WriteString("    MASK=$(python3 -c \"import ipaddress; print(ipaddress.IPv4Network('$cidr', strict=False).netmask)\" 2>/dev/null || echo '255.255.255.0')\n")
	sb.WriteString("    echo \"iroute $net $MASK\" >> /etc/openvpn/ccd/$TNAME\n")
	sb.WriteString("  done\n")
	sb.WriteString("done\n\n")

	// Generate server.conf
	sb.WriteString("# Generate server.conf\n")
	sb.WriteString("cat > /etc/openvpn/server.conf << 'SERVEREOF'\n")
	sb.WriteString("port ${OVPN_LISTEN_PORT}\n")
	sb.WriteString("proto ${OVPN_PROTO}\n")
	sb.WriteString("dev tun\n")
	sb.WriteString("ca /run/secrets/openvpn/ca/ca.crt\n")
	sb.WriteString("cert /run/secrets/openvpn/cert/server.crt\n")
	sb.WriteString("key /run/secrets/openvpn/key/server.key\n")
	sb.WriteString("cipher ${OVPN_CIPHER}\n")
	sb.WriteString("keepalive 10 120\n")
	sb.WriteString("persist-key\n")
	sb.WriteString("persist-tun\n")
	sb.WriteString("status /run/openvpn/status.log 30\n")
	sb.WriteString("verb 3\n")
	sb.WriteString("client-config-dir /etc/openvpn/ccd\n")
	sb.WriteString("SERVEREOF\n\n")

	// DH params or ECDH
	if vpn.Spec.OpenVPN != nil && vpn.Spec.OpenVPN.DH != nil {
		sb.WriteString("echo 'dh /run/secrets/openvpn/dh/dh.pem' >> /etc/openvpn/server.conf\n")
	} else {
		sb.WriteString("echo 'dh none' >> /etc/openvpn/server.conf\n")
	}

	// TLS-Auth
	if vpn.Spec.OpenVPN != nil && vpn.Spec.OpenVPN.TLSAuth != nil {
		sb.WriteString("echo 'tls-auth /run/secrets/openvpn/tls-auth/ta.key 0' >> /etc/openvpn/server.conf\n")
	}

	// Client subnet (remote access)
	sb.WriteString("\n# Client subnet for remote access\n")
	sb.WriteString("if [ -n \"${OVPN_CLIENT_SUBNET}\" ]; then\n")
	sb.WriteString("  IFS='/' read -r NET PREFIX <<< \"${OVPN_CLIENT_SUBNET}\"\n")
	sb.WriteString("  MASK=$(python3 -c \"import ipaddress; print(ipaddress.IPv4Network('${OVPN_CLIENT_SUBNET}', strict=False).netmask)\" 2>/dev/null || echo '255.255.255.0')\n")
	sb.WriteString("  echo \"server $NET $MASK\" >> /etc/openvpn/server.conf\n")
	sb.WriteString("fi\n\n")

	// Push routes for site-to-site tunnel remote networks
	sb.WriteString("# Push routes for remote networks\n")
	sb.WriteString("echo \"${VPN_TUNNELS}\" | jq -r '.[].remoteNetworks[]' | sort -u | while read -r cidr; do\n")
	sb.WriteString("  IFS='/' read -r net prefix <<< \"$cidr\"\n")
	sb.WriteString("  MASK=$(python3 -c \"import ipaddress; print(ipaddress.IPv4Network('$cidr', strict=False).netmask)\" 2>/dev/null || echo '255.255.255.0')\n")
	sb.WriteString("  echo \"route $net $MASK\" >> /etc/openvpn/server.conf\n")
	sb.WriteString("done\n\n")

	// Expand env vars in server.conf
	sb.WriteString("# Expand env vars in server.conf\n")
	sb.WriteString("envsubst < /etc/openvpn/server.conf > /etc/openvpn/server-final.conf\n\n")

	// MSS clamping
	if isMSSClampEnabled(vpn) {
		mtu := resolveTunnelMTU(vpn)
		mss := computeMSS(mtu)
		sb.WriteString("# MSS clamping\n")
		sb.WriteString("iptables -t mangle -A FORWARD -p tcp --tcp-flags SYN,RST SYN -j TCPMSS --clamp-mss-to-pmtu 2>/dev/null || true\n")
		sb.WriteString(fmt.Sprintf("iptables -t mangle -A FORWARD -o tun0 -p tcp --tcp-flags SYN,RST SYN -j TCPMSS --set-mss %d 2>/dev/null || true\n\n", mss))
	}

	// Create status directory and start OpenVPN
	sb.WriteString("# Start OpenVPN\n")
	sb.WriteString("mkdir -p /run/openvpn\n")
	sb.WriteString("exec openvpn --config /etc/openvpn/server-final.conf\n")

	return sb.String()
}
```

**Step 5: Add `buildOpenVPNPod()` function**

```go
// buildOpenVPNPod constructs the Pod spec for an OpenVPN gateway.
func buildOpenVPNPod(vpn *v1alpha1.VPCVPNGateway, gw *v1alpha1.VPCGateway) *corev1.Pod {
	podName := vpnPodName(vpn.Name)
	image := resolveOpenVPNImage(vpn)
	script := buildOpenVPNInitScript(vpn)
	envVars := buildOpenVPNEnvVars(vpn)
	multusJSON := buildVPNMultusAnnotation(vpn, gw)

	isTrue := true

	// Build volumes: CA, cert, key (required) + optional DH, TLS-Auth
	volumes := []corev1.Volume{
		{
			Name: "openvpn-ca",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{SecretName: vpn.Spec.OpenVPN.CA.Name},
			},
		},
		{
			Name: "openvpn-cert",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{SecretName: vpn.Spec.OpenVPN.Cert.Name},
			},
		},
		{
			Name: "openvpn-key",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{SecretName: vpn.Spec.OpenVPN.Key.Name},
			},
		},
	}
	volumeMounts := []corev1.VolumeMount{
		{Name: "openvpn-ca", MountPath: "/run/secrets/openvpn/ca", ReadOnly: true},
		{Name: "openvpn-cert", MountPath: "/run/secrets/openvpn/cert", ReadOnly: true},
		{Name: "openvpn-key", MountPath: "/run/secrets/openvpn/key", ReadOnly: true},
	}

	if vpn.Spec.OpenVPN.DH != nil {
		volumes = append(volumes, corev1.Volume{
			Name: "openvpn-dh",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{SecretName: vpn.Spec.OpenVPN.DH.Name},
			},
		})
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name: "openvpn-dh", MountPath: "/run/secrets/openvpn/dh", ReadOnly: true,
		})
	}

	if vpn.Spec.OpenVPN.TLSAuth != nil {
		volumes = append(volumes, corev1.Volume{
			Name: "openvpn-tls-auth",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{SecretName: vpn.Spec.OpenVPN.TLSAuth.Name},
			},
		})
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name: "openvpn-tls-auth", MountPath: "/run/secrets/openvpn/tls-auth", ReadOnly: true,
		})
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: vpn.Namespace,
			Labels: map[string]string{
				"app":                         "vpngateway",
				"vpc.roks.ibm.com/vpngateway": vpn.Name,
			},
			Annotations: map[string]string{
				"k8s.v1.cni.cncf.io/networks": multusJSON,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "vpc.roks.ibm.com/v1alpha1",
					Kind:               "VPCVPNGateway",
					Name:               vpn.Name,
					UID:                vpn.UID,
					Controller:         &isTrue,
					BlockOwnerDeletion: &isTrue,
				},
			},
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyAlways,
			Volumes:       volumes,
			Containers: []corev1.Container{
				{
					Name:         "vpn",
					Image:        image,
					Command:      []string{"/bin/bash", "-c", script},
					Env:          envVars,
					VolumeMounts: volumeMounts,
					SecurityContext: &corev1.SecurityContext{
						Privileged: &isTrue,
						Capabilities: &corev1.Capabilities{
							Add: []corev1.Capability{"NET_ADMIN", "NET_RAW"},
						},
					},
					LivenessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							Exec: &corev1.ExecAction{
								Command: []string{"pgrep", "openvpn"},
							},
						},
						InitialDelaySeconds: 30,
						PeriodSeconds:       30,
					},
					ReadinessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							Exec: &corev1.ExecAction{
								Command: []string{"test", "-f", "/run/openvpn/status.log"},
							},
						},
						InitialDelaySeconds: 10,
						PeriodSeconds:       10,
					},
				},
			},
		},
	}

	return pod
}
```

**Step 6: Verify compilation**

Run: `cd roks-vpc-network-operator && go build ./...`
Expected: SUCCESS

**Step 7: Commit**

```bash
git add roks-vpc-network-operator/pkg/controller/vpngateway/pod.go
git commit -m "feat(vpngateway): add buildOpenVPNPod and OpenVPN init script"
```

---

### Task 3: Wire OpenVPN into the reconciler

**Files:**
- Modify: `roks-vpc-network-operator/pkg/controller/vpngateway/reconciler.go:104-111,217-236`

**Step 1: Add OpenVPN case to protocol switch**

At line 108, change:
```go
	case "ipsec":
		desiredPod = buildStrongSwanPod(vpn, gw)
	default:
```
to:
```go
	case "ipsec":
		desiredPod = buildStrongSwanPod(vpn, gw)
	case "openvpn":
		desiredPod = buildOpenVPNPod(vpn, gw)
	default:
```

**Step 2: Add OpenVPN validation**

In `validateConfig()` (line 233), before the closing `}`, add:
```go
	case "openvpn":
		if vpn.Spec.OpenVPN == nil {
			return fmt.Errorf("protocol %q requires spec.openVPN configuration", vpn.Spec.Protocol)
		}
```

**Step 3: Verify compilation**

Run: `cd roks-vpc-network-operator && go build ./...`
Expected: SUCCESS

**Step 4: Commit**

```bash
git add roks-vpc-network-operator/pkg/controller/vpngateway/reconciler.go
git commit -m "feat(vpngateway): wire OpenVPN protocol into reconciler"
```

---

### Task 4: Write OpenVPN pod unit tests

**Files:**
- Modify: `roks-vpc-network-operator/pkg/controller/vpngateway/pod_test.go`

**Step 1: Add test helper for OpenVPN**

Add after the existing helpers (e.g., `newTestIPsecVPN`):

```go
func newTestOpenVPNVPN() *v1alpha1.VPCVPNGateway {
	port := int32(1194)
	return &v1alpha1.VPCVPNGateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ovpn",
			Namespace: "default",
			UID:       "uid-ovpn-123",
		},
		Spec: v1alpha1.VPCVPNGatewaySpec{
			Protocol:   "openvpn",
			GatewayRef: "my-gw",
			OpenVPN: &v1alpha1.VPNOpenVPNConfig{
				CA:   v1alpha1.SecretKeyRef{Name: "ovpn-ca", Key: "ca.crt"},
				Cert: v1alpha1.SecretKeyRef{Name: "ovpn-cert", Key: "server.crt"},
				Key:  v1alpha1.SecretKeyRef{Name: "ovpn-key", Key: "server.key"},
				ListenPort: &port,
				Proto:      "udp",
				Cipher:     "AES-256-GCM",
				ClientSubnet: "10.8.0.0/24",
			},
			Tunnels: []v1alpha1.VPNTunnel{
				{
					Name:           "to-dc",
					RemoteEndpoint: "203.0.113.1",
					RemoteNetworks: []string{"10.0.0.0/8"},
				},
			},
		},
	}
}
```

**Step 2: Add core pod construction test**

```go
func TestBuildOpenVPNPod(t *testing.T) {
	vpn := newTestOpenVPNVPN()
	gw := newTestVPNGateway()
	pod := buildOpenVPNPod(vpn, gw)

	// Pod name
	assert.Equal(t, "vpngw-test-ovpn", pod.Name)
	assert.Equal(t, "default", pod.Namespace)

	// Labels
	assert.Equal(t, "vpngateway", pod.Labels["app"])
	assert.Equal(t, "test-ovpn", pod.Labels["vpc.roks.ibm.com/vpngateway"])

	// Owner reference
	assert.Len(t, pod.OwnerReferences, 1)
	assert.Equal(t, "VPCVPNGateway", pod.OwnerReferences[0].Kind)

	// Container
	assert.Len(t, pod.Spec.Containers, 1)
	c := pod.Spec.Containers[0]
	assert.Equal(t, "vpn", c.Name)
	assert.Equal(t, defaultVPNImage, c.Image)

	// Security context
	assert.True(t, *c.SecurityContext.Privileged)
	assert.Contains(t, c.SecurityContext.Capabilities.Add, corev1.Capability("NET_ADMIN"))

	// Volumes: 3 required (ca, cert, key)
	assert.GreaterOrEqual(t, len(pod.Spec.Volumes), 3)
	volNames := make([]string, 0, len(pod.Spec.Volumes))
	for _, v := range pod.Spec.Volumes {
		volNames = append(volNames, v.Name)
	}
	assert.Contains(t, volNames, "openvpn-ca")
	assert.Contains(t, volNames, "openvpn-cert")
	assert.Contains(t, volNames, "openvpn-key")

	// Env vars
	envMap := envMapFromContainer(c)
	assert.Equal(t, "1194", envMap["OVPN_LISTEN_PORT"])
	assert.Equal(t, "udp", envMap["OVPN_PROTO"])
	assert.Equal(t, "AES-256-GCM", envMap["OVPN_CIPHER"])
	assert.Equal(t, "10.8.0.0/24", envMap["OVPN_CLIENT_SUBNET"])
	assert.NotEmpty(t, envMap["VPN_TUNNELS"])
}
```

**Step 3: Add optional volumes test**

```go
func TestBuildOpenVPNPod_OptionalVolumes(t *testing.T) {
	vpn := newTestOpenVPNVPN()
	vpn.Spec.OpenVPN.DH = &v1alpha1.SecretKeyRef{Name: "ovpn-dh", Key: "dh.pem"}
	vpn.Spec.OpenVPN.TLSAuth = &v1alpha1.SecretKeyRef{Name: "ovpn-ta", Key: "ta.key"}
	gw := newTestVPNGateway()
	pod := buildOpenVPNPod(vpn, gw)

	// Should have 5 volumes: ca, cert, key, dh, tls-auth
	assert.Len(t, pod.Spec.Volumes, 5)
	volNames := make([]string, 0, len(pod.Spec.Volumes))
	for _, v := range pod.Spec.Volumes {
		volNames = append(volNames, v.Name)
	}
	assert.Contains(t, volNames, "openvpn-dh")
	assert.Contains(t, volNames, "openvpn-tls-auth")
}
```

**Step 4: Add init script test**

```go
func TestBuildOpenVPNInitScript(t *testing.T) {
	vpn := newTestOpenVPNVPN()
	script := buildOpenVPNInitScript(vpn)

	assert.Contains(t, script, "openvpn")
	assert.Contains(t, script, "dhclient net0")
	assert.Contains(t, script, "net.ipv4.ip_forward=1")
	assert.Contains(t, script, "server.conf")
	assert.Contains(t, script, "client-config-dir")
	assert.Contains(t, script, "exec openvpn")
	// MSS clamping enabled by default
	assert.Contains(t, script, "iptables")
}

func TestBuildOpenVPNInitScript_NoMSSClamp(t *testing.T) {
	vpn := newTestOpenVPNVPN()
	noClamp := false
	vpn.Spec.MTU = &v1alpha1.VPNGatewayMTU{MSSClamp: &noClamp}
	script := buildOpenVPNInitScript(vpn)

	assert.NotContains(t, script, "iptables")
}
```

**Step 5: Add health probe tests**

```go
func TestBuildOpenVPNPod_LivenessProbe(t *testing.T) {
	vpn := newTestOpenVPNVPN()
	gw := newTestVPNGateway()
	pod := buildOpenVPNPod(vpn, gw)

	probe := pod.Spec.Containers[0].LivenessProbe
	assert.NotNil(t, probe)
	assert.Equal(t, []string{"pgrep", "openvpn"}, probe.Exec.Command)
}

func TestBuildOpenVPNPod_ReadinessProbe(t *testing.T) {
	vpn := newTestOpenVPNVPN()
	gw := newTestVPNGateway()
	pod := buildOpenVPNPod(vpn, gw)

	probe := pod.Spec.Containers[0].ReadinessProbe
	assert.NotNil(t, probe)
	assert.Equal(t, []string{"test", "-f", "/run/openvpn/status.log"}, probe.Exec.Command)
}
```

**Step 6: Run tests**

Run: `cd roks-vpc-network-operator && go test ./pkg/controller/vpngateway/ -v`
Expected: All tests PASS (existing + 6 new)

**Step 7: Commit**

```bash
git add roks-vpc-network-operator/pkg/controller/vpngateway/pod_test.go
git commit -m "test(vpngateway): add OpenVPN pod construction tests"
```

---

### Task 5: Write OpenVPN reconciler tests

**Files:**
- Modify: `roks-vpc-network-operator/pkg/controller/vpngateway/reconciler_test.go`

**Step 1: Add OpenVPN reconciler create test**

```go
func TestReconcileNormal_CreateOpenVPNGateway(t *testing.T) {
	vpn := newTestOpenVPNVPN()  // from pod_test.go helper
	gw := &v1alpha1.VPCGateway{
		ObjectMeta: metav1.ObjectMeta{Name: "my-gw", Namespace: "default"},
		Spec:       v1alpha1.VPCGatewaySpec{Uplink: v1alpha1.GatewayUplink{Network: "uplink-net", Namespace: "default"}},
		Status:     v1alpha1.VPCGatewayStatus{Phase: "Ready", FloatingIP: "169.1.2.3"},
	}

	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(vpn, gw).
		WithStatusSubresource(vpn).
		Build()

	r := &Reconciler{Client: fakeClient, Scheme: scheme}
	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "test-ovpn", Namespace: "default"},
	})
	assert.NoError(t, err)

	// Verify pod was created
	pod := &corev1.Pod{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{Name: "vpngw-test-ovpn", Namespace: "default"}, pod)
	assert.NoError(t, err)
	assert.Equal(t, defaultVPNImage, pod.Spec.Containers[0].Image)

	// Verify status
	updated := &v1alpha1.VPCVPNGateway{}
	_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: "test-ovpn", Namespace: "default"}, updated)
	assert.Equal(t, "Provisioning", updated.Status.Phase)
	assert.Equal(t, "vpngw-test-ovpn", updated.Status.PodName)
	assert.Equal(t, "169.1.2.3", updated.Status.TunnelEndpoint)
	assert.Equal(t, int32(1), updated.Status.TotalTunnels)
	assert.Contains(t, updated.Status.AdvertisedRoutes, "10.0.0.0/8")
	assert.NotEmpty(t, result.RequeueAfter)
}
```

**Step 2: Add missing config validation test**

```go
func TestReconcileNormal_MissingOpenVPNConfig(t *testing.T) {
	vpn := newTestOpenVPNVPN()
	vpn.Spec.OpenVPN = nil  // Remove required config
	gw := &v1alpha1.VPCGateway{
		ObjectMeta: metav1.ObjectMeta{Name: "my-gw", Namespace: "default"},
		Spec:       v1alpha1.VPCGatewaySpec{Uplink: v1alpha1.GatewayUplink{Network: "uplink-net", Namespace: "default"}},
		Status:     v1alpha1.VPCGatewayStatus{Phase: "Ready", FloatingIP: "169.1.2.3"},
	}

	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(vpn, gw).
		WithStatusSubresource(vpn).
		Build()

	r := &Reconciler{Client: fakeClient, Scheme: scheme}
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "test-ovpn", Namespace: "default"},
	})
	assert.NoError(t, err)

	// Status should be Error
	updated := &v1alpha1.VPCVPNGateway{}
	_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: "test-ovpn", Namespace: "default"}, updated)
	assert.Equal(t, "Error", updated.Status.Phase)
}
```

**Step 3: Run tests**

Run: `cd roks-vpc-network-operator && go test ./pkg/controller/vpngateway/ -v`
Expected: All tests PASS (existing 8 + new 2 = 10)

**Step 4: Commit**

```bash
git add roks-vpc-network-operator/pkg/controller/vpngateway/reconciler_test.go
git commit -m "test(vpngateway): add OpenVPN reconciler tests"
```

---

### Task 6: Update CRD YAML in Helm chart

**Files:**
- Modify: `roks-vpc-network-operator/deploy/helm/roks-vpc-network-operator/templates/crds/vpcvpngateway-crd.yaml`

**Step 1: Extend protocol enum**

At line 74-76, change:
```yaml
                enum:
                - wireguard
                - ipsec
```
to:
```yaml
                enum:
                - wireguard
                - ipsec
                - openvpn
```

**Step 2: Add openVPN spec block**

After the `ipsec` block (after line 155), add:
```yaml
              openVPN:
                description: >-
                  OpenVPN contains global OpenVPN configuration.
                  Required when protocol is "openvpn".
                type: object
                required:
                - ca
                - cert
                - key
                properties:
                  ca:
                    description: CA is a reference to a Secret containing the CA certificate.
                    type: object
                    required:
                    - name
                    - key
                    properties:
                      name:
                        type: string
                        minLength: 1
                      key:
                        type: string
                        minLength: 1
                  cert:
                    description: Cert is a reference to a Secret containing the server certificate.
                    type: object
                    required:
                    - name
                    - key
                    properties:
                      name:
                        type: string
                        minLength: 1
                      key:
                        type: string
                        minLength: 1
                  key:
                    description: Key is a reference to a Secret containing the server private key.
                    type: object
                    required:
                    - name
                    - key
                    properties:
                      name:
                        type: string
                        minLength: 1
                      key:
                        type: string
                        minLength: 1
                  dh:
                    description: DH is a reference to a Secret containing DH parameters. Optional.
                    type: object
                    required:
                    - name
                    - key
                    properties:
                      name:
                        type: string
                        minLength: 1
                      key:
                        type: string
                        minLength: 1
                  tlsAuth:
                    description: TLSAuth is a reference to a Secret containing the TLS-Auth HMAC key.
                    type: object
                    required:
                    - name
                    - key
                    properties:
                      name:
                        type: string
                        minLength: 1
                      key:
                        type: string
                        minLength: 1
                  listenPort:
                    description: ListenPort is the OpenVPN listen port.
                    type: integer
                    format: int32
                    minimum: 1
                    maximum: 65535
                    default: 1194
                  proto:
                    description: Transport protocol.
                    type: string
                    enum: [udp, tcp]
                    default: udp
                  cipher:
                    description: Data channel cipher.
                    type: string
                    default: AES-256-GCM
                  clientSubnet:
                    description: CIDR for the remote-access client IP pool.
                    type: string
```

**Step 3: Lint Helm chart**

Run: `helm lint roks-vpc-network-operator/deploy/helm/roks-vpc-network-operator/`
Expected: 0 errors

**Step 4: Commit**

```bash
git add roks-vpc-network-operator/deploy/helm/roks-vpc-network-operator/templates/crds/vpcvpngateway-crd.yaml
git commit -m "feat(helm): add OpenVPN config to VPCVPNGateway CRD"
```

---

### Task 7: Update console plugin TypeScript types

**Files:**
- Modify: `console-plugin/src/api/types.ts:775,792-819`

**Step 1: Extend VPNGateway protocol union**

At line 775, change:
```typescript
  protocol: 'wireguard' | 'ipsec';
```
to:
```typescript
  protocol: 'wireguard' | 'ipsec' | 'openvpn';
```

**Step 2: Add OpenVPN fields to CreateVPNGatewayRequest**

After the `ipsec` block (after line 804), add:
```typescript
  openVPN?: {
    caSecret: string;
    caSecretKey: string;
    certSecret: string;
    certSecretKey: string;
    keySecret: string;
    keySecretKey: string;
    dhSecret?: string;
    dhSecretKey?: string;
    tlsAuthSecret?: string;
    tlsAuthSecretKey?: string;
    listenPort?: number;
    proto?: string;
    cipher?: string;
    clientSubnet?: string;
  };
```

**Step 3: Type check**

Run: `cd console-plugin && npm run ts-check`
Expected: SUCCESS

**Step 4: Commit**

```bash
git add console-plugin/src/api/types.ts
git commit -m "feat(console): add OpenVPN types to VPN gateway API"
```

---

### Task 8: Update console plugin create page

**Files:**
- Modify: `console-plugin/src/pages/VPNGatewayCreatePage.tsx`

**Step 1: Add OpenVPN state variables**

After the WireGuard state block (after line 65), add:
```typescript
  // OpenVPN global config
  const [ovpnCaSecret, setOvpnCaSecret] = useState('');
  const [ovpnCaKey, setOvpnCaKey] = useState('ca.crt');
  const [ovpnCertSecret, setOvpnCertSecret] = useState('');
  const [ovpnCertKey, setOvpnCertKey] = useState('server.crt');
  const [ovpnKeySecret, setOvpnKeySecret] = useState('');
  const [ovpnKeyKey, setOvpnKeyKey] = useState('server.key');
  const [ovpnPort, setOvpnPort] = useState('1194');
  const [ovpnProto, setOvpnProto] = useState('udp');
  const [ovpnCipher, setOvpnCipher] = useState('AES-256-GCM');
  const [ovpnClientSubnet, setOvpnClientSubnet] = useState('');
```

**Step 2: Add openvpn option to protocol dropdown**

At line 179, add after the ipsec option:
```tsx
                  <FormSelectOption value="openvpn" label="OpenVPN" />
```

**Step 3: Update description text**

At line 156, change:
```tsx
          A VPCVPNGateway establishes encrypted VPN tunnels to remote sites using WireGuard or IPsec/StrongSwan.
```
to:
```tsx
          A VPCVPNGateway establishes encrypted VPN tunnels to remote sites using WireGuard, IPsec/StrongSwan, or OpenVPN.
```

**Step 4: Update validation**

At line 99-103, change the `isValid` computation to:
```typescript
  const isValid =
    name.trim() !== '' &&
    gateway !== '' &&
    tunnels.every(isTunnelValid) &&
    (protocol !== 'wireguard' || (wgSecretName.trim() !== '' && wgSecretKey.trim() !== '')) &&
    (protocol !== 'openvpn' || (ovpnCaSecret.trim() !== '' && ovpnCertSecret.trim() !== '' && ovpnKeySecret.trim() !== ''));
```

Also update `isTunnelValid` — OpenVPN tunnels only need the common fields, so add a condition:
```typescript
  const isTunnelValid = (t: TunnelEntry): boolean => {
    if (!t.name.trim() || !t.remoteEndpoint.trim() || !t.remoteNetworks.trim()) return false;
    if (protocol === 'wireguard' && !t.peerPublicKey.trim()) return false;
    if (protocol === 'ipsec' && (!t.presharedKeySecret.trim() || !t.presharedKeySecretKey.trim())) return false;
    // OpenVPN tunnels only require common fields (name, endpoint, networks)
    return true;
  };
```
(This is the same — OpenVPN falls through to `return true`, which is correct.)

**Step 5: Add OpenVPN config section**

After the WireGuard config conditional block (after line 246), add:
```tsx
              {protocol === 'openvpn' && (
                <>
                  <Title headingLevel="h3" style={{ marginTop: '16px', marginBottom: '8px' }}>OpenVPN Configuration</Title>

                  <FormGroup label="CA Secret" isRequired fieldId="vpn-ovpn-ca">
                    <TextInput id="vpn-ovpn-ca" value={ovpnCaSecret} onChange={(_e, v) => setOvpnCaSecret(v)} isRequired placeholder="e.g. ovpn-ca" />
                    <FormHelperText><HelperText><HelperTextItem>Kubernetes Secret containing the CA certificate</HelperTextItem></HelperText></FormHelperText>
                  </FormGroup>
                  <FormGroup label="CA Secret Key" fieldId="vpn-ovpn-ca-key">
                    <TextInput id="vpn-ovpn-ca-key" value={ovpnCaKey} onChange={(_e, v) => setOvpnCaKey(v)} />
                    <FormHelperText><HelperText><HelperTextItem>Key within the Secret (default: ca.crt)</HelperTextItem></HelperText></FormHelperText>
                  </FormGroup>

                  <FormGroup label="Server Cert Secret" isRequired fieldId="vpn-ovpn-cert">
                    <TextInput id="vpn-ovpn-cert" value={ovpnCertSecret} onChange={(_e, v) => setOvpnCertSecret(v)} isRequired placeholder="e.g. ovpn-cert" />
                    <FormHelperText><HelperText><HelperTextItem>Kubernetes Secret containing the server certificate</HelperTextItem></HelperText></FormHelperText>
                  </FormGroup>
                  <FormGroup label="Cert Secret Key" fieldId="vpn-ovpn-cert-key">
                    <TextInput id="vpn-ovpn-cert-key" value={ovpnCertKey} onChange={(_e, v) => setOvpnCertKey(v)} />
                    <FormHelperText><HelperText><HelperTextItem>Key within the Secret (default: server.crt)</HelperTextItem></HelperText></FormHelperText>
                  </FormGroup>

                  <FormGroup label="Server Key Secret" isRequired fieldId="vpn-ovpn-key">
                    <TextInput id="vpn-ovpn-key" value={ovpnKeySecret} onChange={(_e, v) => setOvpnKeySecret(v)} isRequired placeholder="e.g. ovpn-key" />
                    <FormHelperText><HelperText><HelperTextItem>Kubernetes Secret containing the server private key</HelperTextItem></HelperText></FormHelperText>
                  </FormGroup>
                  <FormGroup label="Key Secret Key" fieldId="vpn-ovpn-key-key">
                    <TextInput id="vpn-ovpn-key-key" value={ovpnKeyKey} onChange={(_e, v) => setOvpnKeyKey(v)} />
                    <FormHelperText><HelperText><HelperTextItem>Key within the Secret (default: server.key)</HelperTextItem></HelperText></FormHelperText>
                  </FormGroup>

                  <FormGroup label="Listen Port" fieldId="vpn-ovpn-port">
                    <TextInput id="vpn-ovpn-port" type="number" value={ovpnPort} onChange={(_e, v) => setOvpnPort(v)} />
                    <FormHelperText><HelperText><HelperTextItem>OpenVPN listen port (default: 1194)</HelperTextItem></HelperText></FormHelperText>
                  </FormGroup>

                  <FormGroup label="Protocol" fieldId="vpn-ovpn-proto">
                    <FormSelect id="vpn-ovpn-proto" value={ovpnProto} onChange={(_e, v) => setOvpnProto(v)}>
                      <FormSelectOption value="udp" label="UDP (Recommended)" />
                      <FormSelectOption value="tcp" label="TCP" />
                    </FormSelect>
                    <FormHelperText><HelperText><HelperTextItem>Transport protocol. UDP is faster; TCP can traverse restrictive firewalls.</HelperTextItem></HelperText></FormHelperText>
                  </FormGroup>

                  <FormGroup label="Cipher" fieldId="vpn-ovpn-cipher">
                    <TextInput id="vpn-ovpn-cipher" value={ovpnCipher} onChange={(_e, v) => setOvpnCipher(v)} />
                    <FormHelperText><HelperText><HelperTextItem>Data channel cipher (default: AES-256-GCM)</HelperTextItem></HelperText></FormHelperText>
                  </FormGroup>

                  <FormGroup label="Client Subnet" fieldId="vpn-ovpn-client-subnet">
                    <TextInput id="vpn-ovpn-client-subnet" value={ovpnClientSubnet} onChange={(_e, v) => setOvpnClientSubnet(v)} placeholder="e.g. 10.8.0.0/24" />
                    <FormHelperText><HelperText><HelperTextItem>IP pool for remote-access clients. Leave empty for site-to-site only.</HelperTextItem></HelperText></FormHelperText>
                  </FormGroup>
                </>
              )}
```

**Step 6: Add OpenVPN config to submit handler**

After the WireGuard block in `handleSubmit` (after line 136), add:
```typescript
    if (protocol === 'openvpn') {
      req.openVPN = {
        caSecret: ovpnCaSecret.trim(),
        caSecretKey: ovpnCaKey.trim(),
        certSecret: ovpnCertSecret.trim(),
        certSecretKey: ovpnCertKey.trim(),
        keySecret: ovpnKeySecret.trim(),
        keySecretKey: ovpnKeyKey.trim(),
        listenPort: parseInt(ovpnPort, 10) || 1194,
        proto: ovpnProto,
        cipher: ovpnCipher.trim(),
        clientSubnet: ovpnClientSubnet.trim() || undefined,
      };
    }
```

**Step 7: Type check and build**

Run: `cd console-plugin && npm run ts-check && npm run build`
Expected: SUCCESS

**Step 8: Commit**

```bash
git add console-plugin/src/pages/VPNGatewayCreatePage.tsx
git commit -m "feat(console): add OpenVPN config to VPN gateway create page"
```

---

### Task 9: Update console plugin detail and list pages

**Files:**
- Modify: `console-plugin/src/pages/VPNGatewayDetailPage.tsx:38-41`
- Modify: `console-plugin/src/pages/VPNGatewaysListPage.tsx:31-34`

**Step 1: Add openvpn to protocolColors in detail page**

At line 38-41 of `VPNGatewayDetailPage.tsx`, change:
```typescript
const protocolColors: Record<string, 'blue' | 'purple'> = {
  wireguard: 'blue',
  ipsec: 'purple',
};
```
to:
```typescript
const protocolColors: Record<string, 'blue' | 'purple' | 'cyan'> = {
  wireguard: 'blue',
  ipsec: 'purple',
  openvpn: 'cyan',
};
```

**Step 2: Add openvpn to protocolColors in list page**

At line 31-34 of `VPNGatewaysListPage.tsx`, apply the same change:
```typescript
const protocolColors: Record<string, 'blue' | 'purple' | 'cyan'> = {
  wireguard: 'blue',
  ipsec: 'purple',
  openvpn: 'cyan',
};
```

**Step 3: Update list page description**

At line 83, change:
```tsx
          VPN Gateways provide site-to-site and client-to-site VPN connectivity for VM workload networks using WireGuard or IPsec/StrongSwan.
```
to:
```tsx
          VPN Gateways provide site-to-site and client-to-site VPN connectivity for VM workload networks using WireGuard, IPsec/StrongSwan, or OpenVPN.
```

**Step 4: Type check and build**

Run: `cd console-plugin && npm run ts-check && npm run build`
Expected: SUCCESS

**Step 5: Commit**

```bash
git add console-plugin/src/pages/VPNGatewayDetailPage.tsx console-plugin/src/pages/VPNGatewaysListPage.tsx
git commit -m "feat(console): add OpenVPN protocol color to detail and list pages"
```

---

### Task 10: Full verification

**Step 1: Run all Go tests**

Run: `cd roks-vpc-network-operator && go test ./... -count=1`
Expected: All PASS

**Step 2: Run Go vet**

Run: `cd roks-vpc-network-operator && go vet ./...`
Expected: No issues

**Step 3: Console plugin type check + build**

Run: `cd console-plugin && npm run ts-check && npm run build`
Expected: SUCCESS

**Step 4: Helm lint**

Run: `helm lint roks-vpc-network-operator/deploy/helm/roks-vpc-network-operator/`
Expected: 0 errors

**Step 5: Commit all remaining changes (if any)**

Verify clean working tree:
```bash
git status
```
