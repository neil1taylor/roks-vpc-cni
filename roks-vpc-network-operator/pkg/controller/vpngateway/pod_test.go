package vpngateway

import (
	"encoding/json"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1alpha1 "github.com/IBM/roks-vpc-network-operator/api/v1alpha1"
)

// newTestWireGuardVPN creates a VPCVPNGateway for testing with WireGuard config.
func newTestWireGuardVPN() *v1alpha1.VPCVPNGateway {
	listenPort := int32(51820)
	tunnelMTU := int32(1420)
	mssClamp := true
	return &v1alpha1.VPCVPNGateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-vpn",
			Namespace: "default",
			UID:       "test-uid-vpn",
		},
		Spec: v1alpha1.VPCVPNGatewaySpec{
			Protocol:   "wireguard",
			GatewayRef: "gw-prod",
			WireGuard: &v1alpha1.VPNWireGuardConfig{
				PrivateKey: v1alpha1.SecretKeyRef{
					Name: "wg-vpn-secret",
					Key:  "privateKey",
				},
				ListenPort: &listenPort,
			},
			Tunnels: []v1alpha1.VPNTunnel{
				{
					Name:                "dc1",
					RemoteEndpoint:      "198.51.100.1",
					RemoteNetworks:      []string{"10.0.0.0/8", "172.16.0.0/12"},
					PeerPublicKey:       "aB3dEfGhIjKlMnOpQrStUvWxYz1234567890ABCDE=",
					TunnelAddressLocal:  "10.99.0.1/30",
					TunnelAddressRemote: "10.99.0.2/30",
				},
			},
			MTU: &v1alpha1.VPNGatewayMTU{
				TunnelMTU: &tunnelMTU,
				MSSClamp:  &mssClamp,
			},
		},
	}
}

// newTestIPsecVPN creates a VPCVPNGateway for testing with IPsec config.
func newTestIPsecVPN() *v1alpha1.VPCVPNGateway {
	return &v1alpha1.VPCVPNGateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ipsec",
			Namespace: "default",
			UID:       "test-uid-ipsec",
		},
		Spec: v1alpha1.VPCVPNGatewaySpec{
			Protocol:   "ipsec",
			GatewayRef: "gw-prod",
			IPsec:      &v1alpha1.VPNIPsecConfig{},
			Tunnels: []v1alpha1.VPNTunnel{
				{
					Name:           "site-a",
					RemoteEndpoint: "203.0.113.10",
					RemoteNetworks: []string{"192.168.1.0/24"},
					PresharedKey: &v1alpha1.SecretKeyRef{
						Name: "ipsec-psk-site-a",
						Key:  "psk",
					},
				},
			},
		},
	}
}

// newTestVPNGateway creates a VPCGateway for testing.
func newTestVPNGateway() *v1alpha1.VPCGateway {
	return &v1alpha1.VPCGateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gw-prod",
			Namespace: "default",
		},
		Spec: v1alpha1.VPCGatewaySpec{
			Zone: "eu-de-1",
			Uplink: v1alpha1.GatewayUplink{
				Network:   "uplink-net",
				Namespace: "default",
			},
		},
		Status: v1alpha1.VPCGatewayStatus{
			Phase:      "Ready",
			FloatingIP: "169.48.1.1",
			MACAddress: "fa:16:3e:aa:bb:cc",
			ReservedIP: "10.240.1.5",
		},
	}
}

func TestVPNPodName(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"my-vpn", "vpngw-my-vpn"},
		{"dc1-tunnel", "vpngw-dc1-tunnel"},
		{"a", "vpngw-a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := vpnPodName(tt.name)
			if got != tt.want {
				t.Errorf("vpnPodName(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestComputeMSS(t *testing.T) {
	tests := []struct {
		mtu  int32
		want int32
	}{
		{1420, 1380},
		{1400, 1360},
		{9000, 8960},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := computeMSS(tt.mtu)
			if got != tt.want {
				t.Errorf("computeMSS(%d) = %d, want %d", tt.mtu, got, tt.want)
			}
		})
	}
}

func TestBuildWireGuardPod(t *testing.T) {
	vpn := newTestWireGuardVPN()
	gw := newTestVPNGateway()

	pod := buildWireGuardPod(vpn, gw)

	// Pod metadata
	if pod.Name != "vpngw-test-vpn" {
		t.Errorf("pod name = %q, want %q", pod.Name, "vpngw-test-vpn")
	}

	// Labels
	if pod.Labels["app"] != "vpngateway" {
		t.Errorf("label app = %q, want %q", pod.Labels["app"], "vpngateway")
	}
	if pod.Labels["vpc.roks.ibm.com/vpngateway"] != "test-vpn" {
		t.Errorf("label vpngateway = %q, want %q", pod.Labels["vpc.roks.ibm.com/vpngateway"], "test-vpn")
	}

	// Owner reference
	if len(pod.OwnerReferences) != 1 {
		t.Fatalf("expected 1 owner reference, got %d", len(pod.OwnerReferences))
	}
	if pod.OwnerReferences[0].Kind != "VPCVPNGateway" {
		t.Errorf("owner ref kind = %q, want %q", pod.OwnerReferences[0].Kind, "VPCVPNGateway")
	}

	// Container
	if len(pod.Spec.Containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(pod.Spec.Containers))
	}
	container := pod.Spec.Containers[0]
	if container.Name != "vpn" {
		t.Errorf("container name = %q, want %q", container.Name, "vpn")
	}

	// Security context: privileged + NET_ADMIN + NET_RAW
	if container.SecurityContext == nil || container.SecurityContext.Privileged == nil || !*container.SecurityContext.Privileged {
		t.Error("container should be privileged")
	}
	caps := container.SecurityContext.Capabilities.Add
	foundNetAdmin, foundNetRaw := false, false
	for _, cap := range caps {
		if cap == "NET_ADMIN" {
			foundNetAdmin = true
		}
		if cap == "NET_RAW" {
			foundNetRaw = true
		}
	}
	if !foundNetAdmin || !foundNetRaw {
		t.Errorf("expected NET_ADMIN and NET_RAW capabilities, got %v", caps)
	}

	// Env vars
	envMap := make(map[string]string)
	for _, env := range container.Env {
		envMap[env.Name] = env.Value
	}
	if envMap["WG_LISTEN_PORT"] != "51820" {
		t.Errorf("WG_LISTEN_PORT = %q, want %q", envMap["WG_LISTEN_PORT"], "51820")
	}
	if envMap["TUNNEL_MTU"] != "1420" {
		t.Errorf("TUNNEL_MTU = %q, want %q", envMap["TUNNEL_MTU"], "1420")
	}
	if envMap["MSS_CLAMP"] != "true" {
		t.Errorf("MSS_CLAMP = %q, want %q", envMap["MSS_CLAMP"], "true")
	}

	// Volume for WireGuard private key
	foundVolume := false
	for _, vol := range pod.Spec.Volumes {
		if vol.Name == "wireguard-key" {
			foundVolume = true
			if vol.Secret == nil || vol.Secret.SecretName != "wg-vpn-secret" {
				t.Error("wireguard-key volume should reference secret 'wg-vpn-secret'")
			}
		}
	}
	if !foundVolume {
		t.Error("missing wireguard-key volume")
	}

	// Volume mount
	foundMount := false
	for _, mount := range container.VolumeMounts {
		if mount.Name == "wireguard-key" && mount.MountPath == "/run/secrets/wireguard" {
			foundMount = true
			if !mount.ReadOnly {
				t.Error("wireguard-key mount should be read-only")
			}
		}
	}
	if !foundMount {
		t.Error("missing wireguard-key volume mount")
	}
}

func TestBuildWireGuardPod_GWIdentity(t *testing.T) {
	vpn := newTestWireGuardVPN()
	gw := newTestVPNGateway()

	pod := buildWireGuardPod(vpn, gw)
	envMap := make(map[string]string)
	for _, env := range pod.Spec.Containers[0].Env {
		envMap[env.Name] = env.Value
	}
	if envMap["GW_RESERVED_IP"] != gw.Status.ReservedIP {
		t.Errorf("GW_RESERVED_IP = %q, want %q", envMap["GW_RESERVED_IP"], gw.Status.ReservedIP)
	}

	// Verify Multus annotation includes MAC
	multus := pod.Annotations["k8s.v1.cni.cncf.io/networks"]
	if !strings.Contains(multus, strings.ToLower(gw.Status.MACAddress)) {
		t.Errorf("Multus annotation should contain gateway MAC %q, got %q", gw.Status.MACAddress, multus)
	}
}

func TestBuildWireGuardPod_DefaultImage(t *testing.T) {
	vpn := newTestWireGuardVPN()
	vpn.Spec.Pod = nil
	gw := newTestVPNGateway()

	pod := buildWireGuardPod(vpn, gw)
	if pod.Spec.Containers[0].Image != defaultVPNImage {
		t.Errorf("image = %q, want default %q", pod.Spec.Containers[0].Image, defaultVPNImage)
	}
}

func TestBuildWireGuardPod_CustomImage(t *testing.T) {
	vpn := newTestWireGuardVPN()
	vpn.Spec.Pod = &v1alpha1.RouterPodSpec{Image: "custom/wg:v2"}
	gw := newTestVPNGateway()

	pod := buildWireGuardPod(vpn, gw)
	if pod.Spec.Containers[0].Image != "custom/wg:v2" {
		t.Errorf("image = %q, want %q", pod.Spec.Containers[0].Image, "custom/wg:v2")
	}
}

func TestBuildWireGuardInitScript(t *testing.T) {
	vpn := newTestWireGuardVPN()
	script := buildWireGuardInitScript(vpn)

	mustContain := []string{
		"ip addr add ${GW_RESERVED_IP}/24 dev net0",
		"ip link add dev wg0 type wireguard",
		"wg set wg0 listen-port ${WG_LISTEN_PORT}",
		"/run/secrets/wireguard/${WG_PRIVATE_KEY_FILE}",
		"VPN_TUNNELS",
		"jq",
		"peerPublicKey",
		"remoteEndpoint",
		"ip link set wg0 up",
		"ip route add",
		"ip_forward",
		"nft add table inet mangle",
		"tcp option maxseg size set",
		"exec sleep infinity",
	}

	for _, s := range mustContain {
		if !strings.Contains(script, s) {
			t.Errorf("init script missing expected content: %q", s)
		}
	}
}

func TestBuildWireGuardInitScript_NoMSSClamp(t *testing.T) {
	vpn := newTestWireGuardVPN()
	mssClamp := false
	vpn.Spec.MTU.MSSClamp = &mssClamp

	script := buildWireGuardInitScript(vpn)

	if strings.Contains(script, "nft add table inet mangle") {
		t.Error("init script should NOT contain MSS clamp rules when disabled")
	}
	if !strings.Contains(script, "ip link add dev wg0 type wireguard") {
		t.Error("init script missing WireGuard setup")
	}
}

func TestBuildTunnelsJSON(t *testing.T) {
	vpn := newTestWireGuardVPN()
	// Add a second tunnel
	vpn.Spec.Tunnels = append(vpn.Spec.Tunnels, v1alpha1.VPNTunnel{
		Name:                "dc2",
		RemoteEndpoint:      "198.51.100.2",
		RemoteNetworks:      []string{"192.168.0.0/16"},
		PeerPublicKey:       "SecondPeerKey123456789012345678901234567890=",
		TunnelAddressLocal:  "10.99.0.5/30",
		TunnelAddressRemote: "10.99.0.6/30",
	})

	tunnelsJSON := buildTunnelsJSON(vpn)

	var tunnels []map[string]interface{}
	if err := json.Unmarshal([]byte(tunnelsJSON), &tunnels); err != nil {
		t.Fatalf("VPN_TUNNELS is not valid JSON: %v", err)
	}

	if len(tunnels) != 2 {
		t.Fatalf("expected 2 tunnels, got %d", len(tunnels))
	}

	if tunnels[0]["name"] != "dc1" {
		t.Errorf("tunnel[0].name = %v, want %q", tunnels[0]["name"], "dc1")
	}
	if tunnels[1]["name"] != "dc2" {
		t.Errorf("tunnel[1].name = %v, want %q", tunnels[1]["name"], "dc2")
	}
}

func TestBuildVPNMultusAnnotation(t *testing.T) {
	vpn := newTestWireGuardVPN()
	gw := newTestVPNGateway()

	annotation := buildVPNMultusAnnotation(vpn, gw)

	var attachments []multusNetworkAttachment
	if err := json.Unmarshal([]byte(annotation), &attachments); err != nil {
		t.Fatalf("Multus annotation is not valid JSON: %v", err)
	}

	if len(attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(attachments))
	}
	if attachments[0].Name != "uplink-net" {
		t.Errorf("attachment name = %q, want %q", attachments[0].Name, "uplink-net")
	}
	if attachments[0].Interface != "net0" {
		t.Errorf("attachment interface = %q, want %q", attachments[0].Interface, "net0")
	}
}

func TestBuildStrongSwanPod(t *testing.T) {
	vpn := newTestIPsecVPN()
	gw := newTestVPNGateway()

	pod := buildStrongSwanPod(vpn, gw)

	if pod.Name != "vpngw-test-ipsec" {
		t.Errorf("pod name = %q, want %q", pod.Name, "vpngw-test-ipsec")
	}

	if pod.Labels["app"] != "vpngateway" {
		t.Errorf("label app = %q, want %q", pod.Labels["app"], "vpngateway")
	}

	// Should have IPsec PSK volume
	if len(pod.Spec.Volumes) != 1 {
		t.Fatalf("expected 1 volume (ipsec-psk-site-a), got %d", len(pod.Spec.Volumes))
	}
	if pod.Spec.Volumes[0].Name != "ipsec-psk-site-a" {
		t.Errorf("volume name = %q, want %q", pod.Spec.Volumes[0].Name, "ipsec-psk-site-a")
	}
	if pod.Spec.Volumes[0].Secret.SecretName != "ipsec-psk-site-a" {
		t.Errorf("secret name = %q, want %q", pod.Spec.Volumes[0].Secret.SecretName, "ipsec-psk-site-a")
	}

	// Should have volume mount
	container := pod.Spec.Containers[0]
	if len(container.VolumeMounts) != 1 {
		t.Fatalf("expected 1 volume mount, got %d", len(container.VolumeMounts))
	}
	if container.VolumeMounts[0].MountPath != "/run/secrets/ipsec/site-a" {
		t.Errorf("mount path = %q, want %q", container.VolumeMounts[0].MountPath, "/run/secrets/ipsec/site-a")
	}

	// Default StrongSwan image
	if container.Image != defaultStrongSwanImage {
		t.Errorf("image = %q, want default %q", container.Image, defaultStrongSwanImage)
	}
}

func TestBuildStrongSwanPod_GWIdentity(t *testing.T) {
	vpn := newTestIPsecVPN()
	gw := newTestVPNGateway()

	pod := buildStrongSwanPod(vpn, gw)
	envMap := make(map[string]string)
	for _, env := range pod.Spec.Containers[0].Env {
		envMap[env.Name] = env.Value
	}
	if envMap["GW_RESERVED_IP"] != gw.Status.ReservedIP {
		t.Errorf("GW_RESERVED_IP = %q, want %q", envMap["GW_RESERVED_IP"], gw.Status.ReservedIP)
	}

	// Verify Multus annotation includes MAC
	multus := pod.Annotations["k8s.v1.cni.cncf.io/networks"]
	if !strings.Contains(multus, strings.ToLower(gw.Status.MACAddress)) {
		t.Errorf("Multus annotation should contain gateway MAC %q, got %q", gw.Status.MACAddress, multus)
	}
}

func TestGenerateSwanctlConf(t *testing.T) {
	vpn := newTestIPsecVPN()
	conf := generateSwanctlConf(vpn)

	mustContain := []string{
		"connections {",
		"site-a {",
		"remote_addrs = 203.0.113.10",
		"auth = psk",
		"remote_ts = 192.168.1.0/24",
		"start_action = start",
		"dpd_action = restart",
		"secrets {",
		"ike-site-a {",
		"/run/secrets/ipsec/site-a/psk",
	}

	for _, s := range mustContain {
		if !strings.Contains(conf, s) {
			t.Errorf("swanctl.conf missing expected content: %q\nFull conf:\n%s", s, conf)
		}
	}
}

func TestBuildWireGuardPod_LivenessProbe(t *testing.T) {
	vpn := newTestWireGuardVPN()
	gw := newTestVPNGateway()

	pod := buildWireGuardPod(vpn, gw)
	container := pod.Spec.Containers[0]

	if container.LivenessProbe == nil {
		t.Fatal("expected liveness probe")
	}
	if container.LivenessProbe.Exec == nil {
		t.Fatal("expected exec-based liveness probe")
	}
	cmd := strings.Join(container.LivenessProbe.Exec.Command, " ")
	if !strings.Contains(cmd, "wg show wg0") {
		t.Errorf("liveness command = %q, expected 'wg show wg0'", cmd)
	}
}

func TestBuildWireGuardPod_ReadinessProbe(t *testing.T) {
	vpn := newTestWireGuardVPN()
	gw := newTestVPNGateway()

	pod := buildWireGuardPod(vpn, gw)
	container := pod.Spec.Containers[0]

	if container.ReadinessProbe == nil {
		t.Fatal("expected readiness probe")
	}
	if container.ReadinessProbe.Exec == nil {
		t.Fatal("expected exec-based readiness probe")
	}
}

func TestBuildStrongSwanPod_LivenessProbe(t *testing.T) {
	vpn := newTestIPsecVPN()
	gw := newTestVPNGateway()

	pod := buildStrongSwanPod(vpn, gw)
	container := pod.Spec.Containers[0]

	if container.LivenessProbe == nil {
		t.Fatal("expected liveness probe")
	}
	cmd := strings.Join(container.LivenessProbe.Exec.Command, " ")
	if !strings.Contains(cmd, "swanctl") {
		t.Errorf("liveness command = %q, expected swanctl", cmd)
	}
}

func TestVPNPodNeedsRecreation(t *testing.T) {
	vpn := newTestWireGuardVPN()
	gw := newTestVPNGateway()

	pod := buildWireGuardPod(vpn, gw)

	// Same pod should not need recreation
	if vpnPodNeedsRecreation(pod, pod) {
		t.Error("identical pods should not need recreation")
	}

	// Changed image should trigger recreation
	vpn2 := newTestWireGuardVPN()
	vpn2.Spec.Pod = &v1alpha1.RouterPodSpec{Image: "new-image:v2"}
	pod2 := buildWireGuardPod(vpn2, gw)
	if !vpnPodNeedsRecreation(pod, pod2) {
		t.Error("different image should trigger recreation")
	}
}

func TestBuildWireGuardPod_DefaultMTU(t *testing.T) {
	vpn := newTestWireGuardVPN()
	vpn.Spec.MTU = nil
	gw := newTestVPNGateway()

	pod := buildWireGuardPod(vpn, gw)

	envMap := make(map[string]string)
	for _, env := range pod.Spec.Containers[0].Env {
		envMap[env.Name] = env.Value
	}

	if envMap["TUNNEL_MTU"] != "1420" {
		t.Errorf("TUNNEL_MTU = %q, want %q (default)", envMap["TUNNEL_MTU"], "1420")
	}
	if envMap["MSS_CLAMP"] != "true" {
		t.Errorf("MSS_CLAMP = %q, want %q (default)", envMap["MSS_CLAMP"], "true")
	}
}

// newTestOpenVPNVPN creates a VPCVPNGateway for testing with OpenVPN config.
func newTestOpenVPNVPN() *v1alpha1.VPCVPNGateway {
	listenPort := int32(1194)
	mssClamp := true
	return &v1alpha1.VPCVPNGateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ovpn",
			Namespace: "default",
			UID:       "test-uid-ovpn",
		},
		Spec: v1alpha1.VPCVPNGatewaySpec{
			Protocol:   "openvpn",
			GatewayRef: "gw-prod",
			OpenVPN: &v1alpha1.VPNOpenVPNConfig{
				CA:           v1alpha1.SecretKeyRef{Name: "ovpn-ca-secret", Key: "ca.crt"},
				Cert:         v1alpha1.SecretKeyRef{Name: "ovpn-cert-secret", Key: "server.crt"},
				Key:          v1alpha1.SecretKeyRef{Name: "ovpn-key-secret", Key: "server.key"},
				ListenPort:   &listenPort,
				Proto:        "udp",
				Cipher:       "AES-256-GCM",
				ClientSubnet: "10.8.0.0/24",
			},
			Tunnels: []v1alpha1.VPNTunnel{
				{
					Name:           "to-dc",
					RemoteEndpoint: "203.0.113.1",
					RemoteNetworks: []string{"10.0.0.0/8"},
				},
			},
			MTU: &v1alpha1.VPNGatewayMTU{
				MSSClamp: &mssClamp,
			},
		},
	}
}

func TestBuildOpenVPNPod(t *testing.T) {
	vpn := newTestOpenVPNVPN()
	gw := newTestVPNGateway()

	pod := buildOpenVPNPod(vpn, gw)

	// Pod metadata
	if pod.Name != "vpngw-test-ovpn" {
		t.Errorf("pod name = %q, want %q", pod.Name, "vpngw-test-ovpn")
	}
	if pod.Namespace != "default" {
		t.Errorf("pod namespace = %q, want %q", pod.Namespace, "default")
	}

	// Labels
	if pod.Labels["app"] != "vpngateway" {
		t.Errorf("label app = %q, want %q", pod.Labels["app"], "vpngateway")
	}
	if pod.Labels["vpc.roks.ibm.com/vpngateway"] != "test-ovpn" {
		t.Errorf("label vpngateway = %q, want %q", pod.Labels["vpc.roks.ibm.com/vpngateway"], "test-ovpn")
	}

	// Owner reference
	if len(pod.OwnerReferences) != 1 {
		t.Fatalf("expected 1 owner reference, got %d", len(pod.OwnerReferences))
	}
	if pod.OwnerReferences[0].Kind != "VPCVPNGateway" {
		t.Errorf("owner ref kind = %q, want %q", pod.OwnerReferences[0].Kind, "VPCVPNGateway")
	}

	// Containers: vpn + status-exporter
	if len(pod.Spec.Containers) != 2 {
		t.Fatalf("expected 2 containers (vpn + status-exporter), got %d", len(pod.Spec.Containers))
	}
	container := pod.Spec.Containers[0]
	if container.Name != "vpn" {
		t.Errorf("container name = %q, want %q", container.Name, "vpn")
	}
	sidecar := pod.Spec.Containers[1]
	if sidecar.Name != "status-exporter" {
		t.Errorf("sidecar name = %q, want %q", sidecar.Name, "status-exporter")
	}

	// Default image (Fedora for OpenVPN)
	if container.Image != defaultOpenVPNImage {
		t.Errorf("image = %q, want default %q", container.Image, defaultOpenVPNImage)
	}

	// Security context: privileged + NET_ADMIN + NET_RAW
	if container.SecurityContext == nil || container.SecurityContext.Privileged == nil || !*container.SecurityContext.Privileged {
		t.Error("container should be privileged")
	}
	caps := container.SecurityContext.Capabilities.Add
	foundNetAdmin, foundNetRaw := false, false
	for _, cap := range caps {
		if cap == "NET_ADMIN" {
			foundNetAdmin = true
		}
		if cap == "NET_RAW" {
			foundNetRaw = true
		}
	}
	if !foundNetAdmin || !foundNetRaw {
		t.Errorf("expected NET_ADMIN and NET_RAW capabilities, got %v", caps)
	}

	// Volumes: 3 required (ca, cert, key) + 1 CRL + 1 shared run dir
	if len(pod.Spec.Volumes) != 5 {
		t.Fatalf("expected 5 volumes (ca, cert, key, crl, openvpn-run), got %d", len(pod.Spec.Volumes))
	}
	expectedSecretVolumes := map[string]string{
		"openvpn-ca":   "ovpn-ca-secret",
		"openvpn-cert": "ovpn-cert-secret",
		"openvpn-key":  "ovpn-key-secret",
		"openvpn-crl":  "test-ovpn-crl",
	}
	for _, vol := range pod.Spec.Volumes {
		if vol.Name == "openvpn-run" {
			if vol.EmptyDir == nil {
				t.Error("openvpn-run volume should be emptyDir")
			}
			continue
		}
		expectedSecret, ok := expectedSecretVolumes[vol.Name]
		if !ok {
			t.Errorf("unexpected volume %q", vol.Name)
			continue
		}
		if vol.Secret == nil || vol.Secret.SecretName != expectedSecret {
			t.Errorf("volume %q should reference secret %q, got %v", vol.Name, expectedSecret, vol.Secret)
		}
	}

	// Env vars
	envMap := make(map[string]string)
	for _, env := range container.Env {
		envMap[env.Name] = env.Value
	}
	if envMap["OVPN_LISTEN_PORT"] != "1194" {
		t.Errorf("OVPN_LISTEN_PORT = %q, want %q", envMap["OVPN_LISTEN_PORT"], "1194")
	}
	if envMap["OVPN_PROTO"] != "udp" {
		t.Errorf("OVPN_PROTO = %q, want %q", envMap["OVPN_PROTO"], "udp")
	}
	if envMap["OVPN_CIPHER"] != "AES-256-GCM" {
		t.Errorf("OVPN_CIPHER = %q, want %q", envMap["OVPN_CIPHER"], "AES-256-GCM")
	}
	if envMap["OVPN_CLIENT_SUBNET"] != "10.8.0.0/24" {
		t.Errorf("OVPN_CLIENT_SUBNET = %q, want %q", envMap["OVPN_CLIENT_SUBNET"], "10.8.0.0/24")
	}
	if envMap["VPN_TUNNELS"] == "" {
		t.Error("VPN_TUNNELS env var should not be empty")
	}
}

func TestBuildOpenVPNPod_GWIdentity(t *testing.T) {
	vpn := newTestOpenVPNVPN()
	gw := newTestVPNGateway()

	pod := buildOpenVPNPod(vpn, gw)
	envMap := make(map[string]string)
	for _, env := range pod.Spec.Containers[0].Env {
		envMap[env.Name] = env.Value
	}
	if envMap["GW_RESERVED_IP"] != gw.Status.ReservedIP {
		t.Errorf("GW_RESERVED_IP = %q, want %q", envMap["GW_RESERVED_IP"], gw.Status.ReservedIP)
	}

	// Verify Multus annotation includes MAC
	multus := pod.Annotations["k8s.v1.cni.cncf.io/networks"]
	if !strings.Contains(multus, strings.ToLower(gw.Status.MACAddress)) {
		t.Errorf("Multus annotation should contain gateway MAC %q, got %q", gw.Status.MACAddress, multus)
	}
}

func TestBuildOpenVPNPod_OptionalVolumes(t *testing.T) {
	vpn := newTestOpenVPNVPN()
	// Add optional DH and TLSAuth
	vpn.Spec.OpenVPN.DH = &v1alpha1.SecretKeyRef{Name: "ovpn-dh-secret", Key: "dh.pem"}
	vpn.Spec.OpenVPN.TLSAuth = &v1alpha1.SecretKeyRef{Name: "ovpn-ta-secret", Key: "ta.key"}
	gw := newTestVPNGateway()

	pod := buildOpenVPNPod(vpn, gw)

	// Should have 7 volumes total: ca, cert, key, crl, openvpn-run, dh, tls-auth
	if len(pod.Spec.Volumes) != 7 {
		t.Fatalf("expected 7 volumes (ca, cert, key, crl, openvpn-run, dh, tls-auth), got %d", len(pod.Spec.Volumes))
	}

	// Verify the optional volume names exist
	volNames := make(map[string]bool)
	for _, vol := range pod.Spec.Volumes {
		volNames[vol.Name] = true
	}
	if !volNames["openvpn-dh"] {
		t.Error("missing openvpn-dh volume")
	}
	if !volNames["openvpn-tls-auth"] {
		t.Error("missing openvpn-tls-auth volume")
	}

	// Verify volume mount count matches (7 = ca, cert, key, crl, openvpn-run, dh, tls-auth)
	container := pod.Spec.Containers[0]
	if len(container.VolumeMounts) != 7 {
		t.Fatalf("expected 7 volume mounts, got %d", len(container.VolumeMounts))
	}
}

func TestBuildOpenVPNInitScript(t *testing.T) {
	vpn := newTestOpenVPNVPN()
	script := buildOpenVPNInitScript(vpn)

	mustContain := []string{
		"ip addr add ${GW_RESERVED_IP}/24 dev net0",
		"openvpn",
		"net.ipv4.ip_forward=1",
		"server.conf",
		"client-config-dir",
		"exec openvpn",
		"iptables -t mangle",
	}

	for _, s := range mustContain {
		if !strings.Contains(script, s) {
			t.Errorf("init script missing expected content: %q", s)
		}
	}
}

func TestBuildOpenVPNInitScript_NoMSSClamp(t *testing.T) {
	vpn := newTestOpenVPNVPN()
	mssClamp := false
	vpn.Spec.MTU.MSSClamp = &mssClamp

	script := buildOpenVPNInitScript(vpn)

	if strings.Contains(script, "iptables -t mangle") {
		t.Error("init script should NOT contain iptables MSS clamp rules when disabled")
	}
	if !strings.Contains(script, "openvpn") {
		t.Error("init script missing OpenVPN setup")
	}
}

func TestBuildOpenVPNPod_LivenessProbe(t *testing.T) {
	vpn := newTestOpenVPNVPN()
	gw := newTestVPNGateway()

	pod := buildOpenVPNPod(vpn, gw)
	container := pod.Spec.Containers[0]

	if container.LivenessProbe == nil {
		t.Fatal("expected liveness probe")
	}
	if container.LivenessProbe.Exec == nil {
		t.Fatal("expected exec-based liveness probe")
	}
	cmd := container.LivenessProbe.Exec.Command
	if len(cmd) != 2 || cmd[0] != "pgrep" || cmd[1] != "openvpn" {
		t.Errorf("liveness command = %v, expected [pgrep openvpn]", cmd)
	}
}

func TestBuildOpenVPNPod_ReadinessProbe(t *testing.T) {
	vpn := newTestOpenVPNVPN()
	gw := newTestVPNGateway()

	pod := buildOpenVPNPod(vpn, gw)
	container := pod.Spec.Containers[0]

	if container.ReadinessProbe == nil {
		t.Fatal("expected readiness probe")
	}
	if container.ReadinessProbe.Exec == nil {
		t.Fatal("expected exec-based readiness probe")
	}
	cmd := container.ReadinessProbe.Exec.Command
	if len(cmd) != 3 || cmd[0] != "test" || cmd[1] != "-f" || cmd[2] != "/run/openvpn/status.log" {
		t.Errorf("readiness command = %v, expected [test -f /run/openvpn/status.log]", cmd)
	}
}

func TestBuildOpenVPNPod_CRLVolume(t *testing.T) {
	vpn := newTestOpenVPNVPN()
	gw := newTestVPNGateway()

	pod := buildOpenVPNPod(vpn, gw)

	// CRL volume should be present
	foundCRLVol := false
	for _, vol := range pod.Spec.Volumes {
		if vol.Name == "openvpn-crl" {
			foundCRLVol = true
			if vol.Secret == nil {
				t.Error("openvpn-crl volume should reference a secret")
			} else {
				expectedSecret := vpn.Name + "-crl"
				if vol.Secret.SecretName != expectedSecret {
					t.Errorf("CRL secret name = %q, want %q", vol.Secret.SecretName, expectedSecret)
				}
				if vol.Secret.Optional == nil || !*vol.Secret.Optional {
					t.Error("CRL secret should be optional")
				}
			}
			break
		}
	}
	if !foundCRLVol {
		t.Error("missing openvpn-crl volume")
	}

	// CRL volume mount should be present
	container := pod.Spec.Containers[0]
	foundCRLMount := false
	for _, mount := range container.VolumeMounts {
		if mount.Name == "openvpn-crl" {
			foundCRLMount = true
			if mount.MountPath != "/run/secrets/openvpn/crl" {
				t.Errorf("CRL mount path = %q, want %q", mount.MountPath, "/run/secrets/openvpn/crl")
			}
			if !mount.ReadOnly {
				t.Error("CRL mount should be read-only")
			}
			break
		}
	}
	if !foundCRLMount {
		t.Error("missing openvpn-crl volume mount")
	}
}

func TestBuildOpenVPNInitScript_CRLVerify(t *testing.T) {
	vpn := newTestOpenVPNVPN()
	script := buildOpenVPNInitScript(vpn)

	// Should contain conditional CRL verification
	if !strings.Contains(script, "crl-verify /run/secrets/openvpn/crl/crl.pem") {
		t.Error("init script missing crl-verify directive")
	}
	// Should be conditional on file existence
	if !strings.Contains(script, "if [ -s /run/secrets/openvpn/crl/crl.pem ]") {
		t.Error("init script missing CRL file existence check")
	}
}

func TestVPNPodNeedsRecreation_MissingVolume(t *testing.T) {
	vpn := newTestOpenVPNVPN()
	gw := newTestVPNGateway()

	desiredPod := buildOpenVPNPod(vpn, gw)

	// Simulate a pre-feature pod without CRL volume
	oldPod := buildOpenVPNPod(vpn, gw)
	filteredVolumes := []corev1.Volume{}
	for _, v := range oldPod.Spec.Volumes {
		if v.Name != "openvpn-crl" {
			filteredVolumes = append(filteredVolumes, v)
		}
	}
	oldPod.Spec.Volumes = filteredVolumes

	if !vpnPodNeedsRecreation(oldPod, desiredPod) {
		t.Error("pod with missing CRL volume should need recreation")
	}
}

func TestBuildOpenVPNPod_StatusSidecar(t *testing.T) {
	vpn := newTestOpenVPNVPN()
	gw := newTestVPNGateway()

	pod := buildOpenVPNPod(vpn, gw)

	// Should have 2 containers
	if len(pod.Spec.Containers) != 2 {
		t.Fatalf("expected 2 containers, got %d", len(pod.Spec.Containers))
	}

	sidecar := pod.Spec.Containers[1]
	if sidecar.Name != "status-exporter" {
		t.Errorf("sidecar name = %q, want %q", sidecar.Name, "status-exporter")
	}

	// Sidecar should use the same image as the main container
	if sidecar.Image != pod.Spec.Containers[0].Image {
		t.Errorf("sidecar image = %q, should match main container %q", sidecar.Image, pod.Spec.Containers[0].Image)
	}

	// Sidecar should have port 9190
	if len(sidecar.Ports) != 1 || sidecar.Ports[0].ContainerPort != 9190 {
		t.Errorf("sidecar port = %v, want 9190", sidecar.Ports)
	}

	// Sidecar should mount openvpn-run read-only
	foundMount := false
	for _, m := range sidecar.VolumeMounts {
		if m.Name == "openvpn-run" {
			foundMount = true
			if !m.ReadOnly {
				t.Error("sidecar openvpn-run mount should be read-only")
			}
			if m.MountPath != "/run/openvpn" {
				t.Errorf("sidecar mount path = %q, want %q", m.MountPath, "/run/openvpn")
			}
		}
	}
	if !foundMount {
		t.Error("sidecar missing openvpn-run volume mount")
	}

	// Sidecar should have liveness probe
	if sidecar.LivenessProbe == nil || sidecar.LivenessProbe.TCPSocket == nil {
		t.Error("sidecar should have TCP liveness probe")
	}

	// Sidecar should have resource limits
	if sidecar.Resources.Requests == nil || sidecar.Resources.Limits == nil {
		t.Error("sidecar should have resource requests and limits")
	}
}

func TestBuildOpenVPNPod_StatusSidecar_NotForWireGuard(t *testing.T) {
	vpn := newTestWireGuardVPN()
	gw := newTestVPNGateway()

	pod := buildWireGuardPod(vpn, gw)

	// WireGuard pod should still have 1 container (no sidecar)
	if len(pod.Spec.Containers) != 1 {
		t.Errorf("WireGuard pod should have 1 container, got %d", len(pod.Spec.Containers))
	}
}

func TestBuildOpenVPNPod_SharedRunVolume(t *testing.T) {
	vpn := newTestOpenVPNVPN()
	gw := newTestVPNGateway()

	pod := buildOpenVPNPod(vpn, gw)

	// Main container should mount openvpn-run read-write
	mainMounts := pod.Spec.Containers[0].VolumeMounts
	foundMain := false
	for _, m := range mainMounts {
		if m.Name == "openvpn-run" {
			foundMain = true
			if m.ReadOnly {
				t.Error("main container openvpn-run mount should be read-write")
			}
		}
	}
	if !foundMain {
		t.Error("main container missing openvpn-run volume mount")
	}
}

func TestVPNPodNeedsRecreation_ContainerCountChanged(t *testing.T) {
	vpn := newTestOpenVPNVPN()
	gw := newTestVPNGateway()

	desired := buildOpenVPNPod(vpn, gw)

	// Simulate a pre-sidecar pod with only 1 container
	old := buildOpenVPNPod(vpn, gw)
	old.Spec.Containers = old.Spec.Containers[:1]

	if !vpnPodNeedsRecreation(old, desired) {
		t.Error("pod with different container count should need recreation")
	}
}

func TestVolumeNameSet(t *testing.T) {
	volumes := []corev1.Volume{
		{Name: "vol-a"},
		{Name: "vol-b"},
		{Name: "vol-c"},
	}
	set := volumeNameSet(volumes)
	if len(set) != 3 {
		t.Errorf("expected 3 entries, got %d", len(set))
	}
	if !set["vol-a"] || !set["vol-b"] || !set["vol-c"] {
		t.Error("missing expected volume names in set")
	}
	if set["vol-d"] {
		t.Error("unexpected volume name in set")
	}
}
