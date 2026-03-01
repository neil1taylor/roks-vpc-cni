package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/IBM/roks-vpc-network-operator/pkg/annotations"
	"github.com/IBM/roks-vpc-network-operator/pkg/controller/network"
	"github.com/IBM/roks-vpc-network-operator/pkg/finalizers"
	"github.com/IBM/roks-vpc-network-operator/pkg/vpc"
)

var cudnGVK = schema.GroupVersionKind{
	Group:   "k8s.ovn.org",
	Version: "v1",
	Kind:    "ClusterUserDefinedNetwork",
}

var udnGVK = schema.GroupVersionKind{
	Group:   "k8s.ovn.org",
	Version: "v1",
	Kind:    "UserDefinedNetwork",
}

// VMMutatingWebhook intercepts VirtualMachine CREATE requests and injects
// VPC networking configuration (MAC address, cloud-init IP) transparently.
type VMMutatingWebhook struct {
	VPC       vpc.Client
	K8s       client.Client
	ClusterID string
	decoder   admission.Decoder
}

// Handle processes the admission request for VirtualMachine creation.
func (w *VMMutatingWebhook) Handle(ctx context.Context, req admission.Request) admission.Response {
	logger := log.FromContext(ctx)

	// Only handle CREATE operations
	if req.Operation != "CREATE" {
		return admission.Allowed("not a CREATE operation")
	}

	logger.Info("Processing VM admission request",
		"namespace", req.Namespace, "name", req.Name)

	// Decode the VirtualMachine from the request
	var vmObj map[string]interface{}
	if err := json.Unmarshal(req.Object.Raw, &vmObj); err != nil {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("failed to decode VM: %w", err))
	}

	// Find all Multus interfaces
	multusNets, interfacePaths := findAllMultusNetworks(vmObj)
	if len(multusNets) == 0 {
		return admission.Allowed("no multus interfaces")
	}

	// Resolve network info for each Multus interface
	vmAnnots := getNestedStringMap(vmObj, "metadata", "annotations")
	if vmAnnots == nil {
		vmAnnots = map[string]string{}
	}

	var vmNetInterfaces []network.VMNetworkInterface
	var localNetIPs []localNetIPEntry
	isMultiNetwork := len(multusNets) > 1

	// Parse FIP networks annotation
	fipNetworks := parseFIPNetworks(vmAnnots)

	for i, mn := range multusNets {
		netInfo, netAnnots, err := w.resolveNetworkInfo(ctx, mn.networkRef, req.Namespace)
		if err != nil {
			logger.Error(err, "Failed to look up network", "network", mn.networkRef)
			return admission.Errored(http.StatusInternalServerError,
				fmt.Errorf("failed to look up network %q: %w", mn.networkRef, err))
		}

		entry := network.VMNetworkInterface{
			NetworkName:   netInfo.Name,
			NetworkKind:   netInfo.Kind,
			Topology:      netInfo.Topology,
			Role:          netInfo.Role,
			InterfaceName: mn.ifaceName,
		}

		if netInfo.IsLocalNet() {
			subnetID := netAnnots[annotations.SubnetID]
			if subnetID == "" {
				return admission.Errored(http.StatusServiceUnavailable,
					fmt.Errorf("network %q does not have a subnet provisioned yet", mn.networkRef))
			}

			// Create per-VM VLAN attachment with inline VNI
			vlanIDStr := netAnnots[annotations.VLANID]
			vlanID, _ := strconv.ParseInt(vlanIDStr, 10, 64)
			vpcID := netAnnots[annotations.VPCID]

			attResult, err := w.ensureVNIAttachment(ctx, req.Namespace, req.Name, netInfo.Name, isMultiNetwork, subnetID, vlanID, vpcID, netAnnots)
			if err != nil {
				logger.Error(err, "Failed to ensure VNI attachment", "network", netInfo.Name)
				return admission.Errored(http.StatusInternalServerError,
					fmt.Errorf("failed to create VPC network attachment for %q: %w", netInfo.Name, err))
			}

			entry.VNIID = attResult.VNI.ID
			entry.MACAddress = attResult.VNI.MACAddress
			entry.ReservedIP = attResult.VNI.PrimaryIP.Address
			entry.ReservedIPID = attResult.VNI.PrimaryIP.ID
			entry.AttachmentID = attResult.AttachmentID
			entry.BMServerID = attResult.BMServerID

			// Set MAC on the interface spec
			setNestedField(vmObj, attResult.VNI.MACAddress, interfacePaths[i], "macAddress")

			// Track LocalNet IPs for cloud-init
			localNetIPs = append(localNetIPs, localNetIPEntry{
				ip:   attResult.VNI.PrimaryIP.Address,
				mac:  attResult.VNI.MACAddress,
				name: mn.ifaceName,
			})

			// Create FIP if requested
			wantFIP := false
			if !isMultiNetwork {
				// Legacy: single-network VMs use vpc.roks.ibm.com/fip: "true"
				wantFIP = vmAnnots[annotations.FIPRequested] == "true"
			}
			// Multi-network: check fip-networks annotation
			if fipNetworks[netInfo.Name] || fipNetworks[mn.ifaceName] {
				wantFIP = true
			}

			if wantFIP {
				zone := netAnnots[annotations.Zone]
				fip, err := w.VPC.CreateFloatingIP(ctx, vpc.CreateFloatingIPOptions{
					Name:  network.TruncateVPCName(fmt.Sprintf("roks-%s-%s-%s-fip", w.ClusterID, req.Namespace, req.Name)),
					Zone:  zone,
					VNIID: attResult.VNI.ID,
				})
				if err != nil {
					logger.Error(err, "Failed to create floating IP")
				} else {
					entry.FIPID = fip.ID
					entry.FIPAddress = fip.Address
					logger.Info("Created floating IP", "fipID", fip.ID, "address", fip.Address)
				}
			}
		}
		// Layer2 interfaces: no VPC resources, just record in the annotation

		vmNetInterfaces = append(vmNetInterfaces, entry)
	}

	// Write multi-network JSON annotation
	netInterfacesJSON, err := json.Marshal(vmNetInterfaces)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError,
			fmt.Errorf("failed to marshal network interfaces: %w", err))
	}
	vmAnnots[annotations.NetworkInterfaces] = string(netInterfacesJSON)

	// Backwards compatibility: if single LocalNet interface, also write legacy flat annotations
	for _, entry := range vmNetInterfaces {
		if entry.Topology == "LocalNet" && entry.VNIID != "" {
			vmAnnots[annotations.VNIID] = entry.VNIID
			vmAnnots[annotations.MACAddress] = entry.MACAddress
			vmAnnots[annotations.ReservedIP] = entry.ReservedIP
			vmAnnots[annotations.ReservedIPID] = entry.ReservedIPID
			if entry.AttachmentID != "" {
				vmAnnots[annotations.AttachmentID] = entry.AttachmentID
			}
			if entry.BMServerID != "" {
				vmAnnots[annotations.BMServerID] = entry.BMServerID
			}
			if entry.FIPID != "" {
				vmAnnots[annotations.FIPID] = entry.FIPID
				vmAnnots[annotations.FIPAddress] = entry.FIPAddress
			}
			break // only first LocalNet for legacy annotations
		}
	}

	setNestedStringMap(vmObj, vmAnnots, "metadata", "annotations")

	// Inject cloud-init network config for all LocalNet interfaces
	injectCloudInitNetworkConfig(vmObj, localNetIPs)

	// Add finalizer
	existingFinalizers := getNestedStringSlice(vmObj, "metadata", "finalizers")
	if !containsString(existingFinalizers, finalizers.VMCleanup) {
		existingFinalizers = append(existingFinalizers, finalizers.VMCleanup)
		setNestedStringSlice(vmObj, existingFinalizers, "metadata", "finalizers")
	}

	// Marshal and return
	marshaledVM, err := json.Marshal(vmObj)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError,
			fmt.Errorf("failed to marshal mutated VM: %w", err))
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledVM)
}

// InjectDecoder injects the admission decoder.
func (w *VMMutatingWebhook) InjectDecoder(d admission.Decoder) error {
	w.decoder = d
	return nil
}

// ensureVNIAttachment creates a per-VM VLAN attachment with inline VNI on any
// bare metal server. The VNI is created as part of the attachment (allow_to_float: true
// handles VNI migration to the correct node). Returns the attachment result
// including VNI details (MAC, IP).
func (w *VMMutatingWebhook) ensureVNIAttachment(ctx context.Context, namespace, vmName, networkName string, isMultiNetwork bool, subnetID string, vlanID int64, vpcID string, netAnnots map[string]string) (*vpc.VMAttachmentResult, error) {
	logger := log.FromContext(ctx)

	// Pick any bare metal server for the attachment
	bmServerID, err := w.pickBMServer(ctx, vpcID)
	if err != nil {
		return nil, fmt.Errorf("failed to find bare metal server: %w", err)
	}

	sgIDs := splitTrimmed(netAnnots[annotations.SecurityGroupIDs], ",")

	// VNI naming: single-network keeps legacy name, multi-network appends network name.
	vniName := network.TruncateVPCName(fmt.Sprintf("roks-%s-%s-%s", w.ClusterID, namespace, vmName))
	if isMultiNetwork {
		vniName = network.TruncateVPCName(fmt.Sprintf("roks-%s-%s-%s-%s", w.ClusterID, namespace, vmName, networkName))
	}

	// Attachment name includes VM name and VLAN for uniqueness
	attName := network.TruncateVPCName(fmt.Sprintf("roks-%s-%s-%s-vlan%d", w.ClusterID, namespace, vmName, vlanID))
	if isMultiNetwork {
		attName = network.TruncateVPCName(fmt.Sprintf("roks-%s-%s-%s-%s-vlan%d", w.ClusterID, namespace, vmName, networkName, vlanID))
	}

	result, err := w.VPC.CreateVMAttachment(ctx, vpc.CreateVMAttachmentOptions{
		BMServerID:       bmServerID,
		Name:             attName,
		VLANID:           vlanID,
		SubnetID:         subnetID,
		VNIName:          vniName,
		SecurityGroupIDs: sgIDs,
	})
	if err != nil {
		return nil, err
	}

	logger.Info("Created VM attachment with inline VNI",
		"attachmentID", result.AttachmentID,
		"bmServerID", result.BMServerID,
		"vniID", result.VNI.ID,
		"mac", result.VNI.MACAddress,
		"ip", result.VNI.PrimaryIP.Address,
		"network", networkName)

	return result, nil
}

// pickBMServer returns the BM server ID of any bare metal node in the cluster.
// Since allow_to_float: true is set on the attachment, it doesn't matter which
// BM server is chosen. Tries providerID first, then falls back to VPC API
// hostname resolution for unmanaged clusters without providerID.
func (w *VMMutatingWebhook) pickBMServer(ctx context.Context, vpcID string) (string, error) {
	logger := log.FromContext(ctx)

	nodeList := &corev1.NodeList{}
	if err := w.K8s.List(ctx, nodeList); err != nil {
		return "", fmt.Errorf("failed to list nodes: %w", err)
	}

	// Try providerID first (ROKS clusters)
	for i := range nodeList.Items {
		node := &nodeList.Items[i]
		bmServerID := network.ExtractBMServerID(node.Spec.ProviderID)
		if bmServerID != "" {
			return bmServerID, nil
		}
	}

	// Fallback: resolve via VPC API hostname lookup (unmanaged BM clusters)
	if vpcID != "" {
		servers, err := w.VPC.ListBareMetalServers(ctx, vpcID)
		if err != nil {
			logger.Error(err, "Failed to list BM servers from VPC API for hostname resolution")
		} else {
			bmMap := make(map[string]string, len(servers))
			for _, s := range servers {
				bmMap[s.Name] = s.ID
			}
			for i := range nodeList.Items {
				node := &nodeList.Items[i]
				// Try exact match then hostname prefix match
				if id, ok := bmMap[node.Name]; ok {
					return id, nil
				}
				hostname := node.Name
				if idx := strings.IndexByte(node.Name, '.'); idx > 0 {
					hostname = node.Name[:idx]
				}
				if id, ok := bmMap[hostname]; ok {
					return id, nil
				}
			}
		}
	}

	return "", fmt.Errorf("no bare metal server found among %d nodes (no providerID, VPC lookup failed)", len(nodeList.Items))
}

// resolveNetworkInfo looks up a network by name, trying CUDN first, then UDN.
func (w *VMMutatingWebhook) resolveNetworkInfo(ctx context.Context, networkRef, vmNamespace string) (*network.NetworkInfo, map[string]string, error) {
	if w.K8s == nil {
		return nil, nil, fmt.Errorf("K8s client not configured on webhook")
	}

	name := extractNetworkName(networkRef)

	// Try CUDN first (cluster-scoped)
	// OVN CUDNs store topology at spec.network.topology and role at spec.network.<topology>.role
	cudn := &unstructured.Unstructured{}
	cudn.SetGroupVersionKind(cudnGVK)
	if err := w.K8s.Get(ctx, client.ObjectKey{Name: name}, cudn); err == nil {
		topology, role := extractCUDNTopologyAndRole(cudn)
		annots := cudn.GetAnnotations()
		if annots == nil {
			annots = map[string]string{}
		}
		return &network.NetworkInfo{
			Name:     name,
			Topology: topology,
			Role:     role,
			Kind:     "ClusterUserDefinedNetwork",
		}, annots, nil
	}

	// Try UDN (namespace-scoped, in VM's namespace)
	// OVN UDNs store topology at spec.topology and role at spec.<topology>.role
	udn := &unstructured.Unstructured{}
	udn.SetGroupVersionKind(udnGVK)
	if err := w.K8s.Get(ctx, client.ObjectKey{Namespace: vmNamespace, Name: name}, udn); err == nil {
		topology, role := extractUDNTopologyAndRole(udn)
		annots := udn.GetAnnotations()
		if annots == nil {
			annots = map[string]string{}
		}
		return &network.NetworkInfo{
			Name:      name,
			Namespace: vmNamespace,
			Topology:  topology,
			Role:      role,
			Kind:      "UserDefinedNetwork",
		}, annots, nil
	}

	return nil, nil, fmt.Errorf("network %q not found as CUDN or UDN in namespace %q", name, vmNamespace)
}

type multusNetEntry struct {
	networkRef string // the Multus networkName (may be "namespace/name" or just "name")
	ifaceName  string // the VM interface name (e.g., "net1")
}

// findAllMultusNetworks looks for all Multus network attachments in the VM spec
// and returns network references and their interface paths.
func findAllMultusNetworks(vmObj map[string]interface{}) ([]multusNetEntry, [][]string) {
	var entries []multusNetEntry
	var paths [][]string

	networks, _ := getNestedSlice(vmObj, "spec", "template", "spec", "networks")
	interfaces, _ := getNestedSlice(vmObj, "spec", "template", "spec", "domain", "devices", "interfaces")

	for _, net := range networks {
		netMap, ok := net.(map[string]interface{})
		if !ok {
			continue
		}
		multus, ok := netMap["multus"].(map[string]interface{})
		if !ok {
			continue
		}
		networkName, ok := multus["networkName"].(string)
		if !ok {
			continue
		}

		netName, _ := netMap["name"].(string)
		for j, iface := range interfaces {
			ifaceMap, ok := iface.(map[string]interface{})
			if !ok {
				continue
			}
			ifaceName, _ := ifaceMap["name"].(string)
			if ifaceName == netName {
				entries = append(entries, multusNetEntry{
					networkRef: networkName,
					ifaceName:  netName,
				})
				paths = append(paths, []string{
					"spec", "template", "spec", "domain", "devices", "interfaces",
					fmt.Sprintf("%d", j),
				})
				break
			}
		}
	}

	return entries, paths
}

// extractNetworkName extracts the network name from a reference.
// Format may be "namespace/name" or just "name".
func extractNetworkName(ref string) string {
	parts := strings.SplitN(ref, "/", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return parts[0]
}

// extractCUDNName is an alias for backwards compatibility with tests.
func extractCUDNName(ref string) string {
	return extractNetworkName(ref)
}

// parseFIPNetworks parses the fip-networks annotation into a set.
func parseFIPNetworks(annots map[string]string) map[string]bool {
	result := map[string]bool{}
	raw := annots[annotations.FIPNetworks]
	if raw == "" {
		return result
	}
	for _, name := range strings.Split(raw, ",") {
		name = strings.TrimSpace(name)
		if name != "" {
			result[name] = true
		}
	}
	return result
}

type localNetIPEntry struct {
	ip   string
	mac  string
	name string
}

// injectCloudInitNetworkConfig injects IP configuration into cloud-init for all LocalNet interfaces.
func injectCloudInitNetworkConfig(vmObj map[string]interface{}, entries []localNetIPEntry) {
	if len(entries) == 0 {
		return
	}

	// Build cloud-init v2 network config for all LocalNet interfaces.
	// Use MAC-based matching so the config works regardless of guest interface
	// naming (eth0 vs enp1s0 etc).
	var ethernets strings.Builder
	for i, entry := range entries {
		if entry.ip == "" || entry.mac == "" {
			continue
		}
		ipParts := strings.Split(entry.ip, ".")
		if len(ipParts) != 4 {
			continue
		}
		gatewayIP := fmt.Sprintf("%s.%s.%s.1", ipParts[0], ipParts[1], ipParts[2])
		// Use network name as the config ID (or fallback to index-based name)
		ifID := entry.name
		if ifID == "" {
			ifID = fmt.Sprintf("localnet%d", i)
		}

		ethernets.WriteString(fmt.Sprintf("  %s:\n", ifID))
		ethernets.WriteString(fmt.Sprintf("    match:\n"))
		ethernets.WriteString(fmt.Sprintf("      macaddress: \"%s\"\n", strings.ToLower(entry.mac)))
		ethernets.WriteString(fmt.Sprintf("    addresses:\n"))
		ethernets.WriteString(fmt.Sprintf("      - %s/24\n", entry.ip))
		if i == 0 {
			// Only set default route on first interface
			ethernets.WriteString(fmt.Sprintf("    routes:\n"))
			ethernets.WriteString(fmt.Sprintf("      - to: 0.0.0.0/0\n"))
			ethernets.WriteString(fmt.Sprintf("        via: %s\n", gatewayIP))
		}
	}

	if ethernets.Len() == 0 {
		return
	}

	networkConfig := "version: 2\nethernets:\n" + ethernets.String()

	// Find cloudInitNoCloud volume
	volumes, _ := getNestedSlice(vmObj, "spec", "template", "spec", "volumes")
	for i, vol := range volumes {
		volMap, ok := vol.(map[string]interface{})
		if !ok {
			continue
		}
		if _, ok := volMap["cloudInitNoCloud"]; ok {
			cloudInit, _ := volMap["cloudInitNoCloud"].(map[string]interface{})
			if cloudInit == nil {
				cloudInit = map[string]interface{}{}
			}
			cloudInit["networkData"] = networkConfig
			volMap["cloudInitNoCloud"] = cloudInit
			volumes[i] = volMap
			setNestedSlice(vmObj, volumes, "spec", "template", "spec", "volumes")
			return
		}
	}
}

// findLocalNetNetworks is kept for backwards compatibility with existing tests.
// It returns the same data as findAllMultusNetworks in the old format.
func findLocalNetNetworks(vmObj map[string]interface{}) ([]string, [][]string) {
	entries, paths := findAllMultusNetworks(vmObj)
	var names []string
	for _, e := range entries {
		names = append(names, extractNetworkName(e.networkRef))
	}
	return names, paths
}

// Helper functions for unstructured JSON navigation

func getNestedSlice(obj map[string]interface{}, fields ...string) ([]interface{}, bool) {
	val, found, err := unstructured.NestedSlice(obj, fields...)
	if err != nil || !found {
		return nil, false
	}
	return val, true
}

func setNestedField(obj map[string]interface{}, value interface{}, path []string, field string) {
	// Navigate to the parent element, handling both map keys and array indices.
	// unstructured.SetNestedField only works with maps, so we must handle
	// []interface{} elements (e.g. spec.template.spec.domain.devices.interfaces[0])
	// manually.
	var current interface{} = obj
	for _, key := range path {
		switch c := current.(type) {
		case map[string]interface{}:
			current = c[key]
		case []interface{}:
			idx, err := strconv.Atoi(key)
			if err != nil || idx < 0 || idx >= len(c) {
				return
			}
			current = c[idx]
		default:
			return
		}
	}
	if m, ok := current.(map[string]interface{}); ok {
		m[field] = value
	}
}

func setNestedSlice(obj map[string]interface{}, val []interface{}, fields ...string) {
	_ = unstructured.SetNestedSlice(obj, val, fields...)
}

func getNestedStringMap(obj map[string]interface{}, fields ...string) map[string]string {
	val, _, _ := unstructured.NestedStringMap(obj, fields...)
	return val
}

func setNestedStringMap(obj map[string]interface{}, val map[string]string, fields ...string) {
	m := make(map[string]interface{}, len(val))
	for k, v := range val {
		m[k] = v
	}
	_ = unstructured.SetNestedField(obj, m, fields...)
}

func getNestedStringSlice(obj map[string]interface{}, fields ...string) []string {
	val, _, _ := unstructured.NestedStringSlice(obj, fields...)
	return val
}

func setNestedStringSlice(obj map[string]interface{}, val []string, fields ...string) {
	s := make([]interface{}, len(val))
	for i, v := range val {
		s[i] = v
	}
	_ = unstructured.SetNestedSlice(obj, s, fields...)
}

func splitTrimmed(s, sep string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, sep)
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// normalizeTopology converts OVN topology values ("Localnet", "Layer2", "Layer3")
// to the operator's canonical form ("LocalNet", "Layer2", "Layer3").
func normalizeTopology(topology string) string {
	switch strings.ToLower(topology) {
	case "localnet":
		return "LocalNet"
	case "layer2":
		return "Layer2"
	case "layer3":
		return "Layer3"
	default:
		return topology
	}
}

// extractCUDNTopologyAndRole reads topology and role from a CUDN's spec.
// OVN CUDNs: spec.network.topology, spec.network.localnet.role (or spec.network.layer2.role).
func extractCUDNTopologyAndRole(cudn *unstructured.Unstructured) (string, string) {
	topology, _, _ := unstructured.NestedString(cudn.Object, "spec", "network", "topology")
	topology = normalizeTopology(topology)

	// Role is nested under the topology-specific key
	var role string
	switch strings.ToLower(topology) {
	case "localnet":
		role, _, _ = unstructured.NestedString(cudn.Object, "spec", "network", "localnet", "role")
	case "layer2":
		role, _, _ = unstructured.NestedString(cudn.Object, "spec", "network", "layer2", "role")
	}
	if role == "" {
		role = "Secondary"
	}
	return topology, role
}

// extractUDNTopologyAndRole reads topology and role from a UDN's spec.
// OVN UDNs: spec.topology, spec.localnet.role (or spec.layer2.role).
func extractUDNTopologyAndRole(udn *unstructured.Unstructured) (string, string) {
	topology, _, _ := unstructured.NestedString(udn.Object, "spec", "topology")
	topology = normalizeTopology(topology)

	var role string
	switch strings.ToLower(topology) {
	case "localnet":
		role, _, _ = unstructured.NestedString(udn.Object, "spec", "localnet", "role")
	case "layer2":
		role, _, _ = unstructured.NestedString(udn.Object, "spec", "layer2", "role")
	}
	if role == "" {
		role = "Secondary"
	}
	return topology, role
}
