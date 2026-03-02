package l2bridge

import (
	"encoding/json"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1alpha1 "github.com/IBM/roks-vpc-network-operator/api/v1alpha1"
)

// TestBridgePodName is a table-driven test for the bridgePodName helper.
func TestBridgePodName(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"my-bridge", "l2bridge-my-bridge"},
		{"nsx-migration", "l2bridge-nsx-migration"},
		{"a", "l2bridge-a"},
		{"prod-dc1-to-vpc", "l2bridge-prod-dc1-to-vpc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := bridgePodName(tt.name)
			if got != tt.want {
				t.Errorf("bridgePodName(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

// TestComputeMSS is a table-driven test for the MSS computation.
func TestComputeMSS(t *testing.T) {
	tests := []struct {
		mtu  int32
		want int32
	}{
		{1400, 1360},
		{1300, 1260},
		{9000, 8960},
		{1500, 1460},
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

// newTestBridge creates a VPCL2Bridge for testing with all WireGuard fields populated.
func newTestBridge() *v1alpha1.VPCL2Bridge {
	listenPort := int32(51820)
	tunnelMTU := int32(1400)
	mssClamp := true
	return &v1alpha1.VPCL2Bridge{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-bridge",
			Namespace: "default",
			UID:       "test-uid-123",
		},
		Spec: v1alpha1.VPCL2BridgeSpec{
			Type:       "gretap-wireguard",
			GatewayRef: "gw-prod",
			NetworkRef: v1alpha1.BridgeNetworkRef{
				Name:      "localnet-1",
				Kind:      "ClusterUserDefinedNetwork",
				Namespace: "default",
			},
			Remote: v1alpha1.BridgeRemote{
				Endpoint: "198.51.100.1",
				WireGuard: &v1alpha1.BridgeWireGuard{
					PrivateKey: v1alpha1.SecretKeyRef{
						Name: "wg-secret",
						Key:  "privateKey",
					},
					PeerPublicKey:     "aB3dEfGhIjKlMnOpQrStUvWxYz1234567890ABCDE=",
					ListenPort:        &listenPort,
					TunnelAddressLocal:  "10.99.0.1/30",
					TunnelAddressRemote: "10.99.0.2/30",
				},
			},
			MTU: &v1alpha1.BridgeMTU{
				TunnelMTU: &tunnelMTU,
				MSSClamp:  &mssClamp,
			},
		},
	}
}

// newTestGateway creates a VPCGateway for testing.
func newTestGateway() *v1alpha1.VPCGateway {
	return &v1alpha1.VPCGateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gw-prod",
			Namespace: "default",
		},
		Spec: v1alpha1.VPCGatewaySpec{
			Zone: "eu-de-1",
			Uplink: v1alpha1.GatewayUplink{
				Network: "uplink-net",
			},
		},
		Status: v1alpha1.VPCGatewayStatus{
			Phase:      "Ready",
			MACAddress: "fa:16:3e:aa:bb:cc",
			ReservedIP: "10.240.1.5",
		},
	}
}

// TestBuildGRETAPInitScript verifies the init script contains all expected
// WireGuard, GRETAP, bridge, and MSS clamping sections.
func TestBuildGRETAPInitScript(t *testing.T) {
	bridge := newTestBridge()
	script := buildGRETAPInitScript(bridge)

	mustContain := []string{
		// WireGuard
		"ip link add dev wg0 type wireguard",
		"${WG_LOCAL_ADDR}",
		"${WG_PEER_PUBLIC_KEY}",
		"${WG_REMOTE_ENDPOINT}",
		"${WG_LISTEN_PORT}",
		"wg set wg0 listen-port ${WG_LISTEN_PORT} private-key /run/secrets/wireguard/privateKey",
		"ip link set wg0 up",

		// GRETAP
		"ip link add dev gretap0 type gretap",
		"${GRETAP_LOCAL}",
		"${GRETAP_REMOTE}",
		"${TUNNEL_MTU}",
		"ip link set gretap0 up",

		// Bridge
		"ip link add name br-l2 type bridge",
		"ip link set gretap0 master br-l2",
		"ip link set net0 master br-l2",
		"ip link set br-l2 up",

		// MSS clamping
		"tcp option maxseg size set",
		"nft add table inet mangle",
		"MSS_CLAMP",

		// Lifecycle
		"exec sleep infinity",
	}

	for _, s := range mustContain {
		if !strings.Contains(script, s) {
			t.Errorf("init script missing expected content: %q", s)
		}
	}
}

// TestBuildGRETAPInitScript_NoMSSClamp verifies the init script does NOT
// contain MSS clamping rules when MSSClamp is disabled.
func TestBuildGRETAPInitScript_NoMSSClamp(t *testing.T) {
	bridge := newTestBridge()
	mssClamp := false
	bridge.Spec.MTU.MSSClamp = &mssClamp

	script := buildGRETAPInitScript(bridge)

	mustNotContain := []string{
		"nft add table inet mangle",
		"nft add chain inet mangle",
		"tcp option maxseg size set",
	}

	for _, s := range mustNotContain {
		if strings.Contains(script, s) {
			t.Errorf("init script should NOT contain MSS clamp rules when disabled, but found: %q", s)
		}
	}

	// Should still have basic tunnel setup
	if !strings.Contains(script, "ip link add dev wg0 type wireguard") {
		t.Error("init script missing WireGuard setup even with MSS clamping disabled")
	}
	if !strings.Contains(script, "ip link add name br-l2 type bridge") {
		t.Error("init script missing bridge setup even with MSS clamping disabled")
	}
}

// TestBuildMultusAnnotation verifies the JSON format of the Multus annotation.
func TestBuildMultusAnnotation(t *testing.T) {
	bridge := newTestBridge()
	annotation := buildMultusAnnotation(bridge)

	// Verify it's valid JSON
	var attachments []multusNetworkAttachment
	if err := json.Unmarshal([]byte(annotation), &attachments); err != nil {
		t.Fatalf("Multus annotation is not valid JSON: %v\nannotation: %s", err, annotation)
	}

	if len(attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(attachments))
	}

	att := attachments[0]
	if att.Name != "localnet-1" {
		t.Errorf("attachment name = %q, want %q", att.Name, "localnet-1")
	}
	if att.Namespace != "default" {
		t.Errorf("attachment namespace = %q, want %q", att.Namespace, "default")
	}
	if att.Interface != "net0" {
		t.Errorf("attachment interface = %q, want %q", att.Interface, "net0")
	}
}

// TestBuildGRETAPPod verifies the full pod construction: labels, owner refs,
// security context, env vars, volumes, and Multus annotations.
func TestBuildGRETAPPod(t *testing.T) {
	bridge := newTestBridge()
	gw := newTestGateway()

	pod := buildGRETAPPod(bridge, gw)

	// Pod metadata
	if pod.Name != "l2bridge-test-bridge" {
		t.Errorf("pod name = %q, want %q", pod.Name, "l2bridge-test-bridge")
	}
	if pod.Namespace != "default" {
		t.Errorf("pod namespace = %q, want %q", pod.Namespace, "default")
	}

	// Labels
	if pod.Labels["app"] != "l2bridge" {
		t.Errorf("label app = %q, want %q", pod.Labels["app"], "l2bridge")
	}
	if pod.Labels["vpc.roks.ibm.com/l2bridge"] != "test-bridge" {
		t.Errorf("label vpc.roks.ibm.com/l2bridge = %q, want %q", pod.Labels["vpc.roks.ibm.com/l2bridge"], "test-bridge")
	}

	// Owner reference
	if len(pod.OwnerReferences) != 1 {
		t.Fatalf("expected 1 owner reference, got %d", len(pod.OwnerReferences))
	}
	ownerRef := pod.OwnerReferences[0]
	if ownerRef.Kind != "VPCL2Bridge" {
		t.Errorf("owner ref kind = %q, want %q", ownerRef.Kind, "VPCL2Bridge")
	}
	if ownerRef.Name != "test-bridge" {
		t.Errorf("owner ref name = %q, want %q", ownerRef.Name, "test-bridge")
	}

	// Multus annotation
	multusJSON := pod.Annotations["k8s.v1.cni.cncf.io/networks"]
	if multusJSON == "" {
		t.Fatal("missing Multus network annotation")
	}
	var attachments []multusNetworkAttachment
	if err := json.Unmarshal([]byte(multusJSON), &attachments); err != nil {
		t.Fatalf("Multus annotation is not valid JSON: %v", err)
	}
	if len(attachments) != 1 {
		t.Fatalf("expected 1 Multus attachment, got %d", len(attachments))
	}

	// Container
	if len(pod.Spec.Containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(pod.Spec.Containers))
	}
	container := pod.Spec.Containers[0]
	if container.Name != "bridge" {
		t.Errorf("container name = %q, want %q", container.Name, "bridge")
	}

	// Security context: privileged + NET_ADMIN
	if container.SecurityContext == nil || container.SecurityContext.Privileged == nil || !*container.SecurityContext.Privileged {
		t.Error("container should be privileged")
	}
	foundNetAdmin := false
	for _, cap := range container.SecurityContext.Capabilities.Add {
		if cap == "NET_ADMIN" {
			foundNetAdmin = true
		}
	}
	if !foundNetAdmin {
		t.Error("container should have NET_ADMIN capability")
	}

	// Command
	if len(container.Command) != 3 || container.Command[0] != "/bin/bash" || container.Command[1] != "-c" {
		t.Errorf("container command = %v, want [/bin/bash -c <script>]", container.Command)
	}

	// Environment variables
	envMap := make(map[string]string)
	for _, env := range container.Env {
		envMap[env.Name] = env.Value
	}

	expectedEnvs := map[string]string{
		"WG_LOCAL_ADDR":       "10.99.0.1/30",
		"WG_REMOTE_ENDPOINT":  "198.51.100.1",
		"WG_PEER_PUBLIC_KEY":  "aB3dEfGhIjKlMnOpQrStUvWxYz1234567890ABCDE=",
		"WG_LISTEN_PORT":      "51820",
		"GRETAP_LOCAL":        "10.99.0.1",
		"GRETAP_REMOTE":       "10.99.0.2",
		"TUNNEL_MTU":          "1400",
		"MSS_CLAMP":           "true",
	}

	for k, want := range expectedEnvs {
		got, ok := envMap[k]
		if !ok {
			t.Errorf("missing env var %q", k)
			continue
		}
		if got != want {
			t.Errorf("env %q = %q, want %q", k, got, want)
		}
	}

	// Volume for WireGuard private key
	foundVolume := false
	for _, vol := range pod.Spec.Volumes {
		if vol.Name == "wireguard-key" {
			foundVolume = true
			if vol.Secret == nil || vol.Secret.SecretName != "wg-secret" {
				t.Error("wireguard-key volume should reference secret 'wg-secret'")
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
		t.Error("missing wireguard-key volume mount at /run/secrets/wireguard")
	}
}

// TestBuildGRETAPPod_DefaultImage verifies the default container image is used
// when none is specified in the bridge spec.
func TestBuildGRETAPPod_DefaultImage(t *testing.T) {
	bridge := newTestBridge()
	bridge.Spec.Pod = nil
	gw := newTestGateway()

	pod := buildGRETAPPod(bridge, gw)

	container := pod.Spec.Containers[0]
	if container.Image != defaultBridgeImage {
		t.Errorf("container image = %q, want default %q", container.Image, defaultBridgeImage)
	}
}

// TestBuildGRETAPPod_CustomImage verifies the custom container image is used
// when specified in the bridge spec.
func TestBuildGRETAPPod_CustomImage(t *testing.T) {
	bridge := newTestBridge()
	bridge.Spec.Pod = &v1alpha1.RouterPodSpec{
		Image: "custom-registry.io/my-bridge:v1",
	}
	gw := newTestGateway()

	pod := buildGRETAPPod(bridge, gw)

	container := pod.Spec.Containers[0]
	if container.Image != "custom-registry.io/my-bridge:v1" {
		t.Errorf("container image = %q, want %q", container.Image, "custom-registry.io/my-bridge:v1")
	}
}

// TestBuildGRETAPPod_MSSClampDisabled verifies that MSS_CLAMP env is "false"
// when MSSClamp is disabled.
func TestBuildGRETAPPod_MSSClampDisabled(t *testing.T) {
	bridge := newTestBridge()
	mssClamp := false
	bridge.Spec.MTU.MSSClamp = &mssClamp
	gw := newTestGateway()

	pod := buildGRETAPPod(bridge, gw)

	envMap := make(map[string]string)
	for _, env := range pod.Spec.Containers[0].Env {
		envMap[env.Name] = env.Value
	}

	if envMap["MSS_CLAMP"] != "false" {
		t.Errorf("MSS_CLAMP = %q, want %q", envMap["MSS_CLAMP"], "false")
	}
}

// TestBuildGRETAPPod_DefaultMTU verifies that default MTU (1400) is used when
// not specified.
func TestBuildGRETAPPod_DefaultMTU(t *testing.T) {
	bridge := newTestBridge()
	bridge.Spec.MTU = nil
	gw := newTestGateway()

	pod := buildGRETAPPod(bridge, gw)

	envMap := make(map[string]string)
	for _, env := range pod.Spec.Containers[0].Env {
		envMap[env.Name] = env.Value
	}

	if envMap["TUNNEL_MTU"] != "1400" {
		t.Errorf("TUNNEL_MTU = %q, want %q (default)", envMap["TUNNEL_MTU"], "1400")
	}
	if envMap["MSS_CLAMP"] != "true" {
		t.Errorf("MSS_CLAMP = %q, want %q (default)", envMap["MSS_CLAMP"], "true")
	}
}

// TestBuildL2VPNPod_Stub verifies the L2VPN stub returns a non-nil pod.
func TestBuildL2VPNPod_Stub(t *testing.T) {
	bridge := newTestBridge()
	bridge.Spec.Type = "l2vpn"
	gw := newTestGateway()

	pod := buildL2VPNPod(bridge, gw)
	if pod == nil {
		t.Fatal("buildL2VPNPod returned nil")
	}
	if pod.Name != "l2bridge-test-bridge" {
		t.Errorf("pod name = %q, want %q", pod.Name, "l2bridge-test-bridge")
	}
}

// TestBuildEVPNPod_Stub verifies the EVPN stub returns a non-nil pod.
func TestBuildEVPNPod_Stub(t *testing.T) {
	bridge := newTestBridge()
	bridge.Spec.Type = "evpn-vxlan"
	gw := newTestGateway()

	pod := buildEVPNPod(bridge, gw)
	if pod == nil {
		t.Fatal("buildEVPNPod returned nil")
	}
	if pod.Name != "l2bridge-test-bridge" {
		t.Errorf("pod name = %q, want %q", pod.Name, "l2bridge-test-bridge")
	}
}

// TestBuildGRETAPPod_LivenessProbe verifies the pod has a liveness probe.
func TestBuildGRETAPPod_LivenessProbe(t *testing.T) {
	bridge := newTestBridge()
	gw := newTestGateway()

	pod := buildGRETAPPod(bridge, gw)
	container := pod.Spec.Containers[0]

	if container.LivenessProbe == nil {
		t.Fatal("expected liveness probe on bridge container")
	}
	if container.LivenessProbe.Exec == nil {
		t.Fatal("expected exec-based liveness probe")
	}
}

// TestBuildGRETAPPod_ReadinessProbe verifies the pod has a readiness probe.
func TestBuildGRETAPPod_ReadinessProbe(t *testing.T) {
	bridge := newTestBridge()
	gw := newTestGateway()

	pod := buildGRETAPPod(bridge, gw)
	container := pod.Spec.Containers[0]

	if container.ReadinessProbe == nil {
		t.Fatal("expected readiness probe on bridge container")
	}
	if container.ReadinessProbe.Exec == nil {
		t.Fatal("expected exec-based readiness probe")
	}
}

// Ensure corev1 import is used.
var _ = corev1.Pod{}
