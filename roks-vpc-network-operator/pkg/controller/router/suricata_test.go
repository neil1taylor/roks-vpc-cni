package router

import (
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1alpha1 "github.com/IBM/roks-vpc-network-operator/api/v1alpha1"
)

func TestGenerateSuricataConfig_IDSMode(t *testing.T) {
	ids := &v1alpha1.RouterIDS{
		Enabled:    true,
		Mode:       "ids",
		Interfaces: "all",
	}
	networks := []v1alpha1.RouterNetwork{
		{Name: "l2-app", Address: "10.100.0.1/24"},
	}

	config := generateSuricataConfig(ids, networks)

	if !strings.Contains(config, "af-packet:") {
		t.Error("expected AF_PACKET section in IDS mode config")
	}
	if strings.Contains(config, "nfq:") {
		t.Error("expected no NFQ section in IDS mode config")
	}
	if !strings.Contains(config, "interface: uplink") {
		t.Error("expected uplink interface in AF_PACKET config")
	}
	if !strings.Contains(config, "interface: net0") {
		t.Error("expected net0 interface in AF_PACKET config")
	}
	if !strings.Contains(config, "eve-log:") {
		t.Error("expected EVE JSON output in config")
	}
}

func TestGenerateSuricataConfig_IPSMode(t *testing.T) {
	queueNum := int32(5)
	ids := &v1alpha1.RouterIDS{
		Enabled:    true,
		Mode:       "ips",
		NFQueueNum: &queueNum,
	}
	networks := []v1alpha1.RouterNetwork{
		{Name: "l2-app", Address: "10.100.0.1/24"},
	}

	config := generateSuricataConfig(ids, networks)

	if !strings.Contains(config, "nfq:") {
		t.Error("expected NFQ section in IPS mode config")
	}
	if strings.Contains(config, "af-packet:") {
		t.Error("expected no AF_PACKET section in IPS mode config")
	}
	if !strings.Contains(config, "queue-num: 5") {
		t.Error("expected queue-num 5 in IPS mode config")
	}
	if !strings.Contains(config, "fail-open: yes") {
		t.Error("expected fail-open in IPS mode config")
	}
}

func TestGenerateSuricataConfig_WithSyslog(t *testing.T) {
	ids := &v1alpha1.RouterIDS{
		Enabled:      true,
		Mode:         "ids",
		SyslogTarget: "syslog.example.com:514",
	}
	networks := []v1alpha1.RouterNetwork{
		{Name: "l2-app", Address: "10.100.0.1/24"},
	}

	config := generateSuricataConfig(ids, networks)

	if !strings.Contains(config, "filetype: syslog") {
		t.Error("expected syslog output when SyslogTarget is set")
	}
	if !strings.Contains(config, "identity: suricata-ids") {
		t.Error("expected syslog identity with mode")
	}
}

func TestGenerateSuricataConfig_Disabled(t *testing.T) {
	ids := &v1alpha1.RouterIDS{
		Enabled: false,
		Mode:    "ids",
	}

	config := generateSuricataConfig(ids, nil)
	if config != "" {
		t.Errorf("expected empty config when IDS is disabled, got %q", config)
	}
}

func TestGenerateSuricataConfig_Nil(t *testing.T) {
	config := generateSuricataConfig(nil, nil)
	if config != "" {
		t.Errorf("expected empty config when IDS is nil, got %q", config)
	}
}

func TestGenerateNFQueueRules_IPSMode(t *testing.T) {
	queueNum := int32(3)
	ids := &v1alpha1.RouterIDS{
		Enabled:    true,
		Mode:       "ips",
		NFQueueNum: &queueNum,
	}
	networks := []v1alpha1.RouterNetwork{
		{Name: "l2-app", Address: "10.100.0.1/24"},
	}

	rules := generateNFQueueRules(ids, networks)

	if !strings.Contains(rules, "table ip suricata") {
		t.Error("expected nftables table 'suricata'")
	}
	if !strings.Contains(rules, "priority -10") {
		t.Error("expected priority -10 for IPS chain")
	}
	if !strings.Contains(rules, "queue num 3 bypass") {
		t.Error("expected NFQUEUE with num 3 and bypass flag")
	}
	if !strings.Contains(rules, "ct state established,related counter accept") {
		t.Error("expected conntrack bypass for established flows with counter")
	}
}

func TestGenerateNFQueueRules_DefaultQueueNum(t *testing.T) {
	ids := &v1alpha1.RouterIDS{
		Enabled: true,
		Mode:    "ips",
		// NFQueueNum is nil — should default to 0
	}

	rules := generateNFQueueRules(ids, nil)

	if !strings.Contains(rules, "queue num 0 bypass") {
		t.Error("expected default queue num 0")
	}
}

func TestGenerateNFQueueRules_IDSMode(t *testing.T) {
	ids := &v1alpha1.RouterIDS{
		Enabled: true,
		Mode:    "ids",
	}

	rules := generateNFQueueRules(ids, nil)
	if rules != "" {
		t.Errorf("expected empty NFQUEUE rules in IDS mode, got %q", rules)
	}
}

func TestGenerateNFQueueRules_Nil(t *testing.T) {
	rules := generateNFQueueRules(nil, nil)
	if rules != "" {
		t.Errorf("expected empty NFQUEUE rules for nil IDS, got %q", rules)
	}
}

func TestSuricataInterfaces_All(t *testing.T) {
	ids := &v1alpha1.RouterIDS{Interfaces: "all"}
	networks := []v1alpha1.RouterNetwork{
		{Name: "l2-app"},
		{Name: "l2-db"},
	}

	ifaces := suricataInterfaces(ids, networks)

	if len(ifaces) != 3 {
		t.Fatalf("expected 3 interfaces, got %d", len(ifaces))
	}
	if ifaces[0] != "uplink" {
		t.Errorf("expected ifaces[0] = 'uplink', got %q", ifaces[0])
	}
	if ifaces[1] != "net0" {
		t.Errorf("expected ifaces[1] = 'net0', got %q", ifaces[1])
	}
	if ifaces[2] != "net1" {
		t.Errorf("expected ifaces[2] = 'net1', got %q", ifaces[2])
	}
}

func TestSuricataInterfaces_Uplink(t *testing.T) {
	ids := &v1alpha1.RouterIDS{Interfaces: "uplink"}
	networks := []v1alpha1.RouterNetwork{
		{Name: "l2-app"},
	}

	ifaces := suricataInterfaces(ids, networks)

	if len(ifaces) != 1 || ifaces[0] != "uplink" {
		t.Errorf("expected only uplink, got %v", ifaces)
	}
}

func TestSuricataInterfaces_Workload(t *testing.T) {
	ids := &v1alpha1.RouterIDS{Interfaces: "workload"}
	networks := []v1alpha1.RouterNetwork{
		{Name: "l2-app"},
		{Name: "l2-db"},
	}

	ifaces := suricataInterfaces(ids, networks)

	if len(ifaces) != 2 {
		t.Fatalf("expected 2 workload interfaces, got %d", len(ifaces))
	}
	for _, iface := range ifaces {
		if iface == "uplink" {
			t.Error("expected no uplink in workload mode")
		}
	}
}

func TestSuricataInterfaces_EmptyDefault(t *testing.T) {
	ids := &v1alpha1.RouterIDS{Interfaces: ""}
	networks := []v1alpha1.RouterNetwork{
		{Name: "l2-app"},
	}

	ifaces := suricataInterfaces(ids, networks)

	// Empty defaults to "all"
	if len(ifaces) != 2 {
		t.Fatalf("expected 2 interfaces (all), got %d", len(ifaces))
	}
	if ifaces[0] != "uplink" {
		t.Errorf("expected uplink first, got %q", ifaces[0])
	}
}

func TestResolveSuricataImage_Default(t *testing.T) {
	router := &v1alpha1.VPCRouter{
		ObjectMeta: metav1.ObjectMeta{Name: "rt-test"},
	}

	image := resolveSuricataImage(router)
	if image != defaultSuricataImage {
		t.Errorf("expected default image %q, got %q", defaultSuricataImage, image)
	}
}

func TestResolveSuricataImage_Override(t *testing.T) {
	router := &v1alpha1.VPCRouter{
		ObjectMeta: metav1.ObjectMeta{Name: "rt-test"},
		Spec: v1alpha1.VPCRouterSpec{
			IDS: &v1alpha1.RouterIDS{
				Image: "custom-suricata:latest",
			},
		},
	}

	image := resolveSuricataImage(router)
	if image != "custom-suricata:latest" {
		t.Errorf("expected custom image, got %q", image)
	}
}

func TestResolveSuricataImage_EnvVar(t *testing.T) {
	t.Setenv("SURICATA_IMAGE", "env-suricata:7.1")

	router := &v1alpha1.VPCRouter{
		ObjectMeta: metav1.ObjectMeta{Name: "rt-test"},
	}

	image := resolveSuricataImage(router)
	if image != "env-suricata:7.1" {
		t.Errorf("expected env var image, got %q", image)
	}
}

func TestGenerateSuricataStartScript_IDS(t *testing.T) {
	ids := &v1alpha1.RouterIDS{
		Enabled: true,
		Mode:    "ids",
	}

	script := generateSuricataStartScript(ids)

	if !strings.Contains(script, "suricata-update") {
		t.Error("expected suricata-update in start script")
	}
	if !strings.Contains(script, "--af-packet") {
		t.Error("expected --af-packet flag in IDS mode start script")
	}
	if !strings.Contains(script, "tail -F") {
		t.Error("expected tail -F for EVE JSON streaming")
	}
}

func TestGenerateSuricataStartScript_IPS(t *testing.T) {
	queueNum := int32(2)
	ids := &v1alpha1.RouterIDS{
		Enabled:    true,
		Mode:       "ips",
		NFQueueNum: &queueNum,
	}

	script := generateSuricataStartScript(ids)

	if !strings.Contains(script, "-q 2") {
		t.Error("expected -q 2 flag in IPS mode start script")
	}
	if strings.Contains(script, "--af-packet") {
		t.Error("expected no --af-packet in IPS mode")
	}
}

func TestGenerateSuricataStartScript_CustomRules(t *testing.T) {
	ids := &v1alpha1.RouterIDS{
		Enabled:     true,
		Mode:        "ids",
		CustomRules: "alert tcp any any -> any 80 (msg:\"test\"; sid:1000001; rev:1;)",
	}

	script := generateSuricataStartScript(ids)

	if !strings.Contains(script, "CUSTOM_RULES") {
		t.Error("expected CUSTOM_RULES handling in start script")
	}
}

func TestGenerateSuricataStartScript_Disabled(t *testing.T) {
	ids := &v1alpha1.RouterIDS{Enabled: false}

	script := generateSuricataStartScript(ids)
	if script != "" {
		t.Errorf("expected empty start script when disabled, got %q", script)
	}
}
