package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/IBM/roks-vpc-network-operator/pkg/annotations"
	"github.com/IBM/roks-vpc-network-operator/pkg/finalizers"
	"github.com/IBM/roks-vpc-network-operator/pkg/vpc"
)

var cudnGVK = schema.GroupVersionKind{
	Group:   "k8s.ovn.org",
	Version: "v1",
	Kind:    "ClusterUserDefinedNetwork",
}

// VMMutatingWebhook intercepts VirtualMachine CREATE requests and injects
// VPC networking configuration (MAC address, cloud-init IP) transparently.
// See DESIGN.md §7 for the full specification.
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

	// Step 1: Decode the VirtualMachine from the request
	var vmObj map[string]interface{}
	if err := json.Unmarshal(req.Object.Raw, &vmObj); err != nil {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("failed to decode VM: %w", err))
	}

	// Step 2: Find LocalNet interfaces
	networks, networkPaths := findLocalNetNetworks(vmObj)
	if len(networks) == 0 {
		return admission.Allowed("no localnet interfaces")
	}

	// Process the first LocalNet interface
	networkName := networks[0]
	interfacePath := networkPaths[0]

	// Step 3: Look up the CUDN to get VPC configuration
	cudnAnnots, err := w.getCUDNAnnotations(ctx, networkName)
	if err != nil {
		logger.Error(err, "Failed to look up CUDN", "network", networkName)
		return admission.Errored(http.StatusInternalServerError,
			fmt.Errorf("failed to look up CUDN %q: %w", networkName, err))
	}

	subnetID := cudnAnnots[annotations.SubnetID]
	if subnetID == "" {
		return admission.Errored(http.StatusServiceUnavailable,
			fmt.Errorf("CUDN %q does not have a subnet provisioned yet", networkName))
	}

	// Step 4: Check for idempotency — reuse existing VNI if present
	var vni *vpc.VNI
	existingVNIs, err := w.VPC.ListVNIsByTag(ctx, w.ClusterID, req.Namespace, req.Name)
	if err == nil && len(existingVNIs) > 0 {
		vni = &existingVNIs[0]
		logger.Info("Reusing existing VNI", "vniID", vni.ID)
	} else {
		// Step 5: Create floating VNI
		sgIDs := splitTrimmed(cudnAnnots[annotations.SecurityGroupIDs], ",")
		vniName := fmt.Sprintf("roks-%s-%s-%s", w.ClusterID, req.Namespace, req.Name)

		vni, err = w.VPC.CreateVNI(ctx, vpc.CreateVNIOptions{
			Name:             vniName,
			SubnetID:         subnetID,
			SecurityGroupIDs: sgIDs,
			ClusterID:        w.ClusterID,
			Namespace:        req.Namespace,
			VMName:           req.Name,
		})
		if err != nil {
			logger.Error(err, "Failed to create VNI")
			return admission.Errored(http.StatusInternalServerError,
				fmt.Errorf("failed to create VPC network interface: %w", err))
		}
		logger.Info("Created VNI", "vniID", vni.ID)
	}

	// Step 6: Read MAC and reserved IP from VNI response
	macAddress := vni.MACAddress
	reservedIP := vni.PrimaryIP.Address
	reservedIPID := vni.PrimaryIP.ID

	// Step 7: Create FIP if requested
	vmAnnots := getNestedStringMap(vmObj, "metadata", "annotations")
	var fipID, fipAddress string
	if vmAnnots[annotations.FIPRequested] == "true" {
		zone := cudnAnnots[annotations.Zone]
		fip, err := w.VPC.CreateFloatingIP(ctx, vpc.CreateFloatingIPOptions{
			Name:  fmt.Sprintf("roks-%s-%s-%s-fip", w.ClusterID, req.Namespace, req.Name),
			Zone:  zone,
			VNIID: vni.ID,
		})
		if err != nil {
			logger.Error(err, "Failed to create floating IP")
			// Don't block VM creation for FIP failure
		} else {
			fipID = fip.ID
			fipAddress = fip.Address
			logger.Info("Created floating IP", "fipID", fipID, "address", fipAddress)
		}
	}

	// Step 8: Mutate the VM spec

	// 8a: Set macAddress on the matching interface
	setNestedField(vmObj, macAddress, interfacePath, "macAddress")

	// 8b: Inject reserved IP into cloud-init network-config
	injectCloudInitNetworkConfig(vmObj, reservedIP)

	// Step 9: Set annotations and finalizer
	if vmAnnots == nil {
		vmAnnots = map[string]string{}
	}
	vmAnnots[annotations.VNIID] = vni.ID
	vmAnnots[annotations.MACAddress] = macAddress
	vmAnnots[annotations.ReservedIP] = reservedIP
	vmAnnots[annotations.ReservedIPID] = reservedIPID
	if fipID != "" {
		vmAnnots[annotations.FIPID] = fipID
		vmAnnots[annotations.FIPAddress] = fipAddress
	}
	setNestedStringMap(vmObj, vmAnnots, "metadata", "annotations")

	// Add finalizer
	existingFinalizers := getNestedStringSlice(vmObj, "metadata", "finalizers")
	if !containsString(existingFinalizers, finalizers.VMCleanup) {
		existingFinalizers = append(existingFinalizers, finalizers.VMCleanup)
		setNestedStringSlice(vmObj, existingFinalizers, "metadata", "finalizers")
	}

	// Step 10: Marshal and return
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

// getCUDNAnnotations fetches the CUDN and returns its annotations.
func (w *VMMutatingWebhook) getCUDNAnnotations(ctx context.Context, cudnName string) (map[string]string, error) {
	if w.K8s == nil {
		return nil, fmt.Errorf("K8s client not configured on webhook")
	}
	cudn := &unstructured.Unstructured{}
	cudn.SetGroupVersionKind(cudnGVK)
	if err := w.K8s.Get(ctx, client.ObjectKey{Name: cudnName}, cudn); err != nil {
		return nil, err
	}
	annots := cudn.GetAnnotations()
	if annots == nil {
		return map[string]string{}, nil
	}
	return annots, nil
}

// findLocalNetNetworks looks for Multus network attachments in the VM spec
// and returns network names and their interface paths.
func findLocalNetNetworks(vmObj map[string]interface{}) ([]string, [][]string) {
	var names []string
	var paths [][]string

	// Navigate spec.template.spec.networks
	networks, _ := getNestedSlice(vmObj, "spec", "template", "spec", "networks")
	interfaces, _ := getNestedSlice(vmObj, "spec", "template", "spec", "domain", "devices", "interfaces")

	for i, net := range networks {
		netMap, ok := net.(map[string]interface{})
		if !ok {
			continue
		}
		// Check for multus network reference
		multus, ok := netMap["multus"].(map[string]interface{})
		if !ok {
			continue
		}
		networkName, ok := multus["networkName"].(string)
		if !ok {
			continue
		}

		// Find matching interface by name
		netName, _ := netMap["name"].(string)
		for j, iface := range interfaces {
			ifaceMap, ok := iface.(map[string]interface{})
			if !ok {
				continue
			}
			ifaceName, _ := ifaceMap["name"].(string)
			if ifaceName == netName {
				names = append(names, extractCUDNName(networkName))
				paths = append(paths, []string{
					"spec", "template", "spec", "domain", "devices", "interfaces",
					fmt.Sprintf("%d", j),
				})
				break
			}
		}
		_ = i
	}

	return names, paths
}

// extractCUDNName extracts the CUDN name from a network attachment reference.
// Format may be "namespace/name" or just "name".
func extractCUDNName(ref string) string {
	parts := strings.SplitN(ref, "/", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return parts[0]
}

// injectCloudInitNetworkConfig injects IP configuration into cloud-init.
func injectCloudInitNetworkConfig(vmObj map[string]interface{}, reservedIP string) {
	if reservedIP == "" {
		return
	}

	// Calculate gateway (assume .1 in the subnet)
	ipParts := strings.Split(reservedIP, ".")
	if len(ipParts) != 4 {
		return
	}
	gatewayIP := fmt.Sprintf("%s.%s.%s.1", ipParts[0], ipParts[1], ipParts[2])

	networkConfig := fmt.Sprintf(`version: 2
ethernets:
  enp1s0:
    addresses:
      - %s/24
    routes:
      - to: 0.0.0.0/0
        via: %s`, reservedIP, gatewayIP)

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

// Helper functions for unstructured JSON navigation

func getNestedSlice(obj map[string]interface{}, fields ...string) ([]interface{}, bool) {
	val, found, err := unstructured.NestedSlice(obj, fields...)
	if err != nil || !found {
		return nil, false
	}
	return val, true
}

func setNestedField(obj map[string]interface{}, value interface{}, path []string, field string) {
	fullPath := append(path, field)
	_ = unstructured.SetNestedField(obj, value, fullPath...)
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
