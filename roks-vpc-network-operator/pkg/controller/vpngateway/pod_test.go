package vpngateway

import (
	"encoding/json"
	"strings"
	"testing"

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
