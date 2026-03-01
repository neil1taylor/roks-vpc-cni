package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/IBM/roks-vpc-network-operator/cmd/bff/internal/auth"
	"github.com/IBM/roks-vpc-network-operator/cmd/bff/internal/model"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

var cudnGVR = schema.GroupVersionResource{
	Group:    "k8s.ovn.org",
	Version:  "v1",
	Resource: "clusteruserdefinednetworks",
}

var udnGVR = schema.GroupVersionResource{
	Group:    "k8s.ovn.org",
	Version:  "v1",
	Resource: "userdefinednetworks",
}

// NetworkHandler handles CUDN and UDN API operations
type NetworkHandler struct {
	dynamicClient dynamic.Interface
	rbacChecker   *auth.RBACChecker
	clusterInfo   ClusterInfo
}

// NewNetworkHandler creates a new network handler
func NewNetworkHandler(_ kubernetes.Interface, dynamicClient dynamic.Interface, rbacChecker *auth.RBACChecker, clusterInfo ClusterInfo) *NetworkHandler {
	return &NetworkHandler{
		dynamicClient: dynamicClient,
		rbacChecker:   rbacChecker,
		clusterInfo:   clusterInfo,
	}
}

// ListCUDNs handles GET /api/v1/cudns
func (h *NetworkHandler) ListCUDNs(w http.ResponseWriter, r *http.Request) {
	if h.dynamicClient == nil {
		WriteError(w, http.StatusServiceUnavailable, "dynamic client not configured", "CLIENT_NOT_CONFIGURED")
		return
	}

	list, err := h.dynamicClient.Resource(cudnGVR).List(r.Context(), metav1.ListOptions{})
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to list CUDNs", "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to list CUDNs", "LIST_FAILED")
		return
	}

	networks := make([]model.NetworkDefinition, 0, len(list.Items))
	for _, item := range list.Items {
		networks = append(networks, unstructuredToNetworkDef(&item, "ClusterUserDefinedNetwork"))
	}

	WriteJSON(w, http.StatusOK, networks)
}

// GetCUDN handles GET /api/v1/cudns/:name
func (h *NetworkHandler) GetCUDN(w http.ResponseWriter, r *http.Request) {
	name := extractLastPathSegment(r.URL.Path, "/api/v1/cudns/")
	if name == "" {
		WriteError(w, http.StatusBadRequest, "missing CUDN name", "MISSING_NAME")
		return
	}

	if h.dynamicClient == nil {
		WriteError(w, http.StatusServiceUnavailable, "dynamic client not configured", "CLIENT_NOT_CONFIGURED")
		return
	}

	item, err := h.dynamicClient.Resource(cudnGVR).Get(r.Context(), name, metav1.GetOptions{})
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to get CUDN", "name", name, "error", err)
		WriteError(w, http.StatusNotFound, "CUDN not found", "NOT_FOUND")
		return
	}

	network := unstructuredToNetworkDef(item, "ClusterUserDefinedNetwork")
	WriteJSON(w, http.StatusOK, network)
}

// CreateCUDN handles POST /api/v1/cudns
func (h *NetworkHandler) CreateCUDN(w http.ResponseWriter, r *http.Request) {
	var req model.CreateNetworkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body", "INVALID_REQUEST")
		return
	}

	if h.dynamicClient == nil {
		WriteError(w, http.StatusServiceUnavailable, "dynamic client not configured", "CLIENT_NOT_CONFIGURED")
		return
	}

	obj, err := buildCUDNUnstructured(req)
	if err != nil {
		WriteError(w, http.StatusBadRequest, err.Error(), "INVALID_REQUEST")
		return
	}
	created, err := h.dynamicClient.Resource(cudnGVR).Create(r.Context(), obj, metav1.CreateOptions{})
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to create CUDN", "error", err)
		WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create CUDN: %v", err), "CREATE_FAILED")
		return
	}

	network := unstructuredToNetworkDef(created, "ClusterUserDefinedNetwork")
	WriteJSON(w, http.StatusCreated, network)
}

// DeleteCUDN handles DELETE /api/v1/cudns/:name
func (h *NetworkHandler) DeleteCUDN(w http.ResponseWriter, r *http.Request) {
	name := extractLastPathSegment(r.URL.Path, "/api/v1/cudns/")
	if name == "" {
		WriteError(w, http.StatusBadRequest, "missing CUDN name", "MISSING_NAME")
		return
	}

	if h.dynamicClient == nil {
		WriteError(w, http.StatusServiceUnavailable, "dynamic client not configured", "CLIENT_NOT_CONFIGURED")
		return
	}

	if err := h.dynamicClient.Resource(cudnGVR).Delete(r.Context(), name, metav1.DeleteOptions{}); err != nil {
		slog.ErrorContext(r.Context(), "failed to delete CUDN", "name", name, "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to delete CUDN", "DELETE_FAILED")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ListUDNs handles GET /api/v1/udns
func (h *NetworkHandler) ListUDNs(w http.ResponseWriter, r *http.Request) {
	if h.dynamicClient == nil {
		WriteError(w, http.StatusServiceUnavailable, "dynamic client not configured", "CLIENT_NOT_CONFIGURED")
		return
	}

	namespace := r.URL.Query().Get("namespace")
	var list *unstructured.UnstructuredList
	var err error
	if namespace != "" {
		list, err = h.dynamicClient.Resource(udnGVR).Namespace(namespace).List(r.Context(), metav1.ListOptions{})
	} else {
		list, err = h.dynamicClient.Resource(udnGVR).List(r.Context(), metav1.ListOptions{})
	}
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to list UDNs", "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to list UDNs", "LIST_FAILED")
		return
	}

	networks := make([]model.NetworkDefinition, 0, len(list.Items))
	for _, item := range list.Items {
		networks = append(networks, unstructuredToNetworkDef(&item, "UserDefinedNetwork"))
	}

	WriteJSON(w, http.StatusOK, networks)
}

// CreateUDN handles POST /api/v1/udns
func (h *NetworkHandler) CreateUDN(w http.ResponseWriter, r *http.Request) {
	var req model.CreateNetworkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body", "INVALID_REQUEST")
		return
	}

	if h.dynamicClient == nil {
		WriteError(w, http.StatusServiceUnavailable, "dynamic client not configured", "CLIENT_NOT_CONFIGURED")
		return
	}

	obj, err := buildUDNUnstructured(req)
	if err != nil {
		WriteError(w, http.StatusBadRequest, err.Error(), "INVALID_REQUEST")
		return
	}
	created, err := h.dynamicClient.Resource(udnGVR).Namespace(req.Namespace).Create(r.Context(), obj, metav1.CreateOptions{})
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to create UDN", "error", err)
		WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create UDN: %v", err), "CREATE_FAILED")
		return
	}

	network := unstructuredToNetworkDef(created, "UserDefinedNetwork")
	WriteJSON(w, http.StatusCreated, network)
}

// DeleteUDN handles DELETE /api/v1/udns/:namespace/:name
func (h *NetworkHandler) DeleteUDN(w http.ResponseWriter, r *http.Request) {
	// Path: /api/v1/udns/namespace/name
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/udns/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		WriteError(w, http.StatusBadRequest, "path must be /api/v1/udns/{namespace}/{name}", "INVALID_PATH")
		return
	}

	if h.dynamicClient == nil {
		WriteError(w, http.StatusServiceUnavailable, "dynamic client not configured", "CLIENT_NOT_CONFIGURED")
		return
	}

	if err := h.dynamicClient.Resource(udnGVR).Namespace(parts[0]).Delete(r.Context(), parts[1], metav1.DeleteOptions{}); err != nil {
		slog.ErrorContext(r.Context(), "failed to delete UDN", "namespace", parts[0], "name", parts[1], "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to delete UDN", "DELETE_FAILED")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// GetUDN handles GET /api/v1/udns/:namespace/:name
func (h *NetworkHandler) GetUDN(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/udns/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		WriteError(w, http.StatusBadRequest, "path must be /api/v1/udns/{namespace}/{name}", "INVALID_PATH")
		return
	}

	if h.dynamicClient == nil {
		WriteError(w, http.StatusServiceUnavailable, "dynamic client not configured", "CLIENT_NOT_CONFIGURED")
		return
	}

	item, err := h.dynamicClient.Resource(udnGVR).Namespace(parts[0]).Get(r.Context(), parts[1], metav1.GetOptions{})
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to get UDN", "namespace", parts[0], "name", parts[1], "error", err)
		WriteError(w, http.StatusNotFound, "UDN not found", "NOT_FOUND")
		return
	}

	network := unstructuredToNetworkDef(item, "UserDefinedNetwork")
	WriteJSON(w, http.StatusOK, network)
}

// GetNetworkTypes handles GET /api/v1/network-types
func (h *NetworkHandler) GetNetworkTypes(w http.ResponseWriter, _ *http.Request) {
	allCombinations := []model.NetworkCombination{
		// Recommended tier — cluster-wide secondary networks
		{
			ID: "localnet-cudn-secondary", Topology: "LocalNet", Scope: "ClusterUserDefinedNetwork", Role: "Secondary",
			Tier: model.TierRecommended, IPMode: model.IPModeStaticReserved, RequiresVPC: true,
			Label:       "LocalNet Cluster Secondary",
			Description: "Cluster-wide secondary network backed by a VPC subnet. Best for VMs that need VPC-routable IPs.",
			IPModeDesc:  "VPC API reserves a static IP from the subnet when the VNI is created. IP and MAC are injected into the VM via cloud-init.",
		},
		{
			ID: "layer2-cudn-secondary", Topology: "Layer2", Scope: "ClusterUserDefinedNetwork", Role: "Secondary",
			Tier: model.TierRecommended, IPMode: model.IPModeDHCP, RequiresVPC: false,
			Label:       "Layer2 Cluster Secondary",
			Description: "Cluster-wide secondary L2 network with OVN DHCP. Best for isolated VM-to-VM traffic without VPC integration.",
			IPModeDesc:  "OVN's built-in DHCP server assigns IPs from the configured subnet range.",
		},
		// Advanced tier — namespace-scoped secondary networks
		{
			ID: "layer2-udn-secondary", Topology: "Layer2", Scope: "UserDefinedNetwork", Role: "Secondary",
			Tier: model.TierAdvanced, IPMode: model.IPModeDHCP, RequiresVPC: false,
			Label:       "Layer2 Namespace Secondary",
			Description: "Namespace-scoped secondary L2 network with OVN DHCP. Use for namespace-isolated VM-to-VM traffic.",
			IPModeDesc:  "OVN's built-in DHCP server assigns IPs from the configured subnet range.",
		},
		// Expert tier — primary network replacements
		{
			ID: "layer2-cudn-primary", Topology: "Layer2", Scope: "ClusterUserDefinedNetwork", Role: "Primary",
			Tier: model.TierExpert, IPMode: model.IPModeDHCP, RequiresVPC: false,
			Label:       "Layer2 Cluster Primary",
			Description: "Replaces the default pod network cluster-wide with an L2 network. Requires a subnet CIDR for persistent IPAM.",
			IPModeDesc:  "OVN assigns IPs from the configured subnet with persistent IPAM so VM addresses survive restarts.",
		},
	}

	// On ROKS clusters, LocalNet requires the ROKS platform API for VNI/VLAN
	// attachment management. Exclude it until that API is available.
	roksAPIAvailable := false // TODO(roks-api): set true when ROKS platform API ships
	combinations := allCombinations
	if h.clusterInfo.Mode == "roks" && !roksAPIAvailable {
		combinations = make([]model.NetworkCombination, 0, len(allCombinations))
		for _, c := range allCombinations {
			if c.Topology != "LocalNet" {
				combinations = append(combinations, c)
			}
		}
	}

	topologies := []string{"LocalNet", "Layer2"}
	if h.clusterInfo.Mode == "roks" && !roksAPIAvailable {
		topologies = []string{"Layer2"}
	}

	WriteJSON(w, http.StatusOK, model.NetworkTypesResponse{
		Topologies:   topologies,
		Scopes:       []string{"ClusterUserDefinedNetwork", "UserDefinedNetwork"},
		Roles:        []string{"Primary", "Secondary"},
		Combinations: combinations,
	})
}

// topologyToCRD maps our display topology name to the OVN CRD value.
// OVN CRD uses "Localnet" (lowercase 'n'), we use "LocalNet" everywhere else.
func topologyToCRD(t string) string {
	if t == "LocalNet" {
		return "Localnet"
	}
	return t
}

// topologyFromCRD maps the OVN CRD topology value back to our display name.
func topologyFromCRD(t string) string {
	if strings.EqualFold(t, "localnet") {
		return "LocalNet"
	}
	return t
}

// Helper functions

func unstructuredToNetworkDef(obj *unstructured.Unstructured, kind string) model.NetworkDefinition {
	// UDN: spec.topology, CUDN: spec.network.topology
	rawTopology, _, _ := unstructured.NestedString(obj.Object, "spec", "topology")
	if rawTopology == "" {
		rawTopology, _, _ = unstructured.NestedString(obj.Object, "spec", "network", "topology")
	}
	topology := topologyFromCRD(rawTopology)
	// UDN: role is in spec.<topology>.role, CUDN: spec.network.<topology>.role
	role, _, _ := unstructured.NestedString(obj.Object, "spec", "role")
	if role == "" {
		lc := strings.ToLower(rawTopology)
		role, _, _ = unstructured.NestedString(obj.Object, "spec", lc, "role")
		if role == "" {
			role, _, _ = unstructured.NestedString(obj.Object, "spec", "network", lc, "role")
		}
	}
	annots := obj.GetAnnotations()

	nd := model.NetworkDefinition{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
		Kind:      kind,
		Topology:  topology,
		Role:      role,
	}

	if annots != nil {
		nd.SubnetID = annots["vpc.roks.ibm.com/subnet-id"]
		nd.SubnetName = annots["vpc.roks.ibm.com/subnet-name"]
		nd.SubnetStatus = annots["vpc.roks.ibm.com/subnet-status"]
		nd.VPCID = annots["vpc.roks.ibm.com/vpc-id"]
		nd.Zone = annots["vpc.roks.ibm.com/zone"]
		nd.CIDR = annots["vpc.roks.ibm.com/cidr"]
		nd.VLANID = annots["vpc.roks.ibm.com/vlan-id"]
		nd.VLANAttachments = annots["vpc.roks.ibm.com/vlan-attachments"]
	}

	nd.Tier = computeTier(kind, nd.Role)
	nd.IPMode = computeIPMode(topology, nd.Role)

	return nd
}

func computeTier(kind, role string) model.NetworkTier {
	if role == "Primary" {
		return model.TierExpert
	}
	if kind == "UserDefinedNetwork" {
		return model.TierAdvanced
	}
	return model.TierRecommended
}

func computeIPMode(topology, role string) model.IPAssignmentMode {
	if topology == "LocalNet" {
		return model.IPModeStaticReserved
	}
	// Layer2: secondary gets DHCP, primary gets none
	if role == "Primary" {
		return model.IPModeNone
	}
	return model.IPModeDHCP
}

func buildCUDNUnstructured(req model.CreateNetworkRequest) (*unstructured.Unstructured, error) {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "k8s.ovn.org",
		Version: "v1",
		Kind:    "ClusterUserDefinedNetwork",
	})
	obj.SetName(req.Name)

	// Build the topology-specific network config
	networkSpec, err := buildNetworkSpec(req)
	if err != nil {
		return nil, err
	}

	// Build namespace selector: if specific namespaces provided, use matchExpressions;
	// otherwise empty selector = all namespaces.
	nsSelector := map[string]interface{}{}
	if len(req.TargetNamespaces) > 0 {
		values := make([]interface{}, len(req.TargetNamespaces))
		for i, ns := range req.TargetNamespaces {
			values[i] = ns
		}
		nsSelector["matchExpressions"] = []interface{}{
			map[string]interface{}{
				"key":      "kubernetes.io/metadata.name",
				"operator": "In",
				"values":   values,
			},
		}
	}

	obj.Object["spec"] = map[string]interface{}{
		"namespaceSelector": nsSelector,
		"network":           networkSpec,
	}

	// Set VPC annotations for the operator to act on
	if req.Topology == "LocalNet" {
		setVPCAnnotations(obj, req)
	}

	return obj, nil
}

func buildUDNUnstructured(req model.CreateNetworkRequest) (*unstructured.Unstructured, error) {
	// UDN does not support LocalNet topology — only CUDNs have the localnet field
	if req.Topology == "LocalNet" {
		return nil, fmt.Errorf("UDN does not support LocalNet topology; use ClusterUserDefinedNetwork instead")
	}
	// UDN Layer2 only supports Secondary role
	if req.Topology == "Layer2" && req.Role == "Primary" {
		return nil, fmt.Errorf("UDN Layer2 only supports Secondary role; use ClusterUserDefinedNetwork for Primary")
	}

	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "k8s.ovn.org",
		Version: "v1",
		Kind:    "UserDefinedNetwork",
	})
	obj.SetName(req.Name)
	obj.SetNamespace(req.Namespace)

	// UDN has topology + config directly under spec (no "network" wrapper)
	spec, err := buildTopologySpec(req)
	if err != nil {
		return nil, err
	}
	obj.Object["spec"] = spec

	return obj, nil
}

// buildNetworkSpec builds the spec.network object for CUDNs
func buildNetworkSpec(req model.CreateNetworkRequest) (map[string]interface{}, error) {
	topologySpec, err := buildTopologySpec(req)
	if err != nil {
		return nil, err
	}
	crdTopology := topologyToCRD(req.Topology)
	// CUDN wraps topology config inside spec.network
	result := map[string]interface{}{
		"topology": crdTopology,
	}
	switch req.Topology {
	case "LocalNet":
		result["localnet"] = topologySpec["localnet"]
	case "Layer2":
		result["layer2"] = topologySpec["layer2"]
	}
	return result, nil
}

// buildTopologySpec builds the topology-specific config (shared by CUDN and UDN)
func buildTopologySpec(req model.CreateNetworkRequest) (map[string]interface{}, error) {
	crdTopology := topologyToCRD(req.Topology)
	spec := map[string]interface{}{
		"topology": crdTopology,
	}

	role := req.Role
	if role == "" {
		role = "Secondary"
	}

	switch req.Topology {
	case "LocalNet":
		localnet := map[string]interface{}{
			"role": role,
			// physicalNetworkName is required — use the network name as default
			"physicalNetworkName": req.Name,
			// VPC API manages IPs, not OVN — always disable IPAM in the CRD.
			// The CIDR is only stored in VPC annotations (via setVPCAnnotations).
			"ipam": map[string]interface{}{
				"mode": "Disabled",
			},
		}
		if req.VLANID != "" {
			vlanID, err := strconv.Atoi(req.VLANID)
			if err != nil {
				return nil, fmt.Errorf("invalid VLAN ID %q: %w", req.VLANID, err)
			}
			localnet["vlan"] = map[string]interface{}{
				"mode": "Access",
				"access": map[string]interface{}{
					"id": int64(vlanID),
				},
			}
		}
		spec["localnet"] = localnet

	case "Layer2":
		layer2 := map[string]interface{}{
			"role": role,
		}
		if role == "Primary" {
			// Primary Layer2 requires a CIDR for OVN to assign IPs
			if req.CIDR == "" {
				return nil, fmt.Errorf("Layer2 Primary requires a subnet CIDR")
			}
			layer2["subnets"] = []interface{}{req.CIDR}
			layer2["ipam"] = map[string]interface{}{
				"lifecycle": "Persistent",
			}
		} else if req.CIDR != "" {
			// Secondary with user-provided CIDR — persistent IPAM
			layer2["subnets"] = []interface{}{req.CIDR}
			layer2["ipam"] = map[string]interface{}{
				"lifecycle": "Persistent",
			}
		} else {
			// Secondary without CIDR — disable IPAM
			layer2["ipam"] = map[string]interface{}{
				"mode": "Disabled",
			}
		}
		spec["layer2"] = layer2

	default:
		return nil, fmt.Errorf("unsupported topology %q", req.Topology)
	}

	return spec, nil
}

// setVPCAnnotations adds vpc.roks.ibm.com/* annotations for the operator
func setVPCAnnotations(obj *unstructured.Unstructured, req model.CreateNetworkRequest) {
	annots := map[string]string{}
	if req.VPCID != "" {
		annots["vpc.roks.ibm.com/vpc-id"] = req.VPCID
	}
	if req.Zone != "" {
		annots["vpc.roks.ibm.com/zone"] = req.Zone
	}
	if req.CIDR != "" {
		annots["vpc.roks.ibm.com/cidr"] = strings.TrimSpace(req.CIDR)
	}
	if req.VLANID != "" {
		annots["vpc.roks.ibm.com/vlan-id"] = req.VLANID
	}
	if req.SecurityGroupIDs != "" {
		annots["vpc.roks.ibm.com/security-group-ids"] = req.SecurityGroupIDs
	}
	if req.ACLID != "" {
		annots["vpc.roks.ibm.com/acl-id"] = req.ACLID
	}
	if req.PublicGatewayID != "" {
		annots["vpc.roks.ibm.com/public-gateway-id"] = req.PublicGatewayID
	}
	if len(annots) > 0 {
		obj.SetAnnotations(annots)
	}
}

func extractLastPathSegment(path, prefix string) string {
	rest := strings.TrimPrefix(path, prefix)
	// Remove trailing slash
	rest = strings.TrimSuffix(rest, "/")
	// Return the first segment after the prefix
	if idx := strings.Index(rest, "/"); idx >= 0 {
		return rest[:idx]
	}
	return rest
}
