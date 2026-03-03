package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/IBM/roks-vpc-network-operator/cmd/bff/internal/auth"
	"github.com/IBM/roks-vpc-network-operator/cmd/bff/internal/model"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

var vpcRouterGVR = schema.GroupVersionResource{
	Group:    "vpc.roks.ibm.com",
	Version:  "v1alpha1",
	Resource: "vpcrouters",
}

// RouterHandler handles VPCRouter API operations.
type RouterHandler struct {
	dynClient  dynamic.Interface
	k8sClient  kubernetes.Interface
	restConfig *rest.Config
	rbac       *auth.RBACChecker
}

// NewRouterHandler creates a new router handler.
func NewRouterHandler(dynClient dynamic.Interface, k8sClient kubernetes.Interface, restConfig *rest.Config, rbac *auth.RBACChecker) *RouterHandler {
	return &RouterHandler{
		dynClient:  dynClient,
		k8sClient:  k8sClient,
		restConfig: restConfig,
		rbac:       rbac,
	}
}

// ListRouters handles GET /api/v1/routers
func (h *RouterHandler) ListRouters(w http.ResponseWriter, r *http.Request) {
	if h.dynClient == nil {
		WriteError(w, http.StatusServiceUnavailable, "dynamic client not configured", "CLIENT_NOT_CONFIGURED")
		return
	}

	list, err := h.dynClient.Resource(vpcRouterGVR).Namespace("").List(r.Context(), metav1.ListOptions{})
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to list VPCRouters", "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to list routers", "LIST_FAILED")
		return
	}

	routers := make([]model.RouterResponse, 0, len(list.Items))
	for _, item := range list.Items {
		routers = append(routers, unstructuredToRouter(&item))
	}

	WriteJSON(w, http.StatusOK, routers)
}

// GetRouter handles GET /api/v1/routers/:name
func (h *RouterHandler) GetRouter(w http.ResponseWriter, r *http.Request) {
	name := extractLastPathSegment(r.URL.Path, "/api/v1/routers/")
	if name == "" {
		WriteError(w, http.StatusBadRequest, "missing router name", "MISSING_NAME")
		return
	}

	if h.dynClient == nil {
		WriteError(w, http.StatusServiceUnavailable, "dynamic client not configured", "CLIENT_NOT_CONFIGURED")
		return
	}

	ns := r.URL.Query().Get("namespace")
	if ns != "" {
		item, err := h.dynClient.Resource(vpcRouterGVR).Namespace(ns).Get(r.Context(), name, metav1.GetOptions{})
		if err != nil {
			slog.ErrorContext(r.Context(), "failed to get VPCRouter", "name", name, "namespace", ns, "error", err)
			WriteError(w, http.StatusNotFound, "router not found", "NOT_FOUND")
			return
		}
		WriteJSON(w, http.StatusOK, unstructuredToRouter(item))
		return
	}

	// No namespace — cross-namespace List + filter by name
	list, err := h.dynClient.Resource(vpcRouterGVR).Namespace("").List(r.Context(), metav1.ListOptions{})
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to list VPCRouters for get", "name", name, "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to list routers", "LIST_FAILED")
		return
	}
	for _, item := range list.Items {
		if item.GetName() == name {
			WriteJSON(w, http.StatusOK, unstructuredToRouter(&item))
			return
		}
	}
	WriteError(w, http.StatusNotFound, "router not found", "NOT_FOUND")
}

// CreateRouter handles POST /api/v1/routers
func (h *RouterHandler) CreateRouter(w http.ResponseWriter, r *http.Request) {
	userInfo := auth.GetUserFromContext(r.Context())
	if userInfo == nil || userInfo.Name == "" {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	allowed, err := h.rbac.CheckAccess(r.Context(), userInfo.Name, userInfo.Groups, "create", "vpcrouters", "")
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "authorization check failed", "AUTHZ_CHECK_FAILED")
		return
	}
	if !allowed {
		WriteError(w, http.StatusForbidden, "forbidden", "FORBIDDEN")
		return
	}

	var req model.RouterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body", "INVALID_REQUEST")
		return
	}

	if h.dynClient == nil {
		WriteError(w, http.StatusServiceUnavailable, "dynamic client not configured", "CLIENT_NOT_CONFIGURED")
		return
	}

	obj := buildRouterUnstructured(req)
	ns := req.Namespace
	if ns == "" {
		ns = "default"
	}
	created, err := h.dynClient.Resource(vpcRouterGVR).Namespace(ns).Create(r.Context(), obj, metav1.CreateOptions{})
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to create VPCRouter", "error", err)
		WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create router: %v", err), "CREATE_FAILED")
		return
	}

	router := unstructuredToRouter(created)
	WriteJSON(w, http.StatusCreated, router)
}

// DeleteRouter handles DELETE /api/v1/routers/:name
func (h *RouterHandler) DeleteRouter(w http.ResponseWriter, r *http.Request) {
	name := extractLastPathSegment(r.URL.Path, "/api/v1/routers/")
	if name == "" {
		WriteError(w, http.StatusBadRequest, "missing router name", "MISSING_NAME")
		return
	}

	userInfo := auth.GetUserFromContext(r.Context())
	if userInfo == nil || userInfo.Name == "" {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	allowed, err := h.rbac.CheckAccess(r.Context(), userInfo.Name, userInfo.Groups, "delete", "vpcrouters", "")
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "authorization check failed", "AUTHZ_CHECK_FAILED")
		return
	}
	if !allowed {
		WriteError(w, http.StatusForbidden, "forbidden", "FORBIDDEN")
		return
	}

	if h.dynClient == nil {
		WriteError(w, http.StatusServiceUnavailable, "dynamic client not configured", "CLIENT_NOT_CONFIGURED")
		return
	}

	ns := r.URL.Query().Get("namespace")
	if ns == "" {
		list, err := h.dynClient.Resource(vpcRouterGVR).Namespace("").List(r.Context(), metav1.ListOptions{})
		if err != nil {
			slog.ErrorContext(r.Context(), "failed to list VPCRouters for delete", "name", name, "error", err)
			WriteError(w, http.StatusInternalServerError, "failed to find router", "LIST_FAILED")
			return
		}
		for _, item := range list.Items {
			if item.GetName() == name {
				ns = item.GetNamespace()
				break
			}
		}
		if ns == "" {
			WriteError(w, http.StatusNotFound, "router not found", "NOT_FOUND")
			return
		}
	}

	if err := h.dynClient.Resource(vpcRouterGVR).Namespace(ns).Delete(r.Context(), name, metav1.DeleteOptions{}); err != nil {
		slog.ErrorContext(r.Context(), "failed to delete VPCRouter", "name", name, "namespace", ns, "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to delete router", "DELETE_FAILED")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// GetLeases handles GET /api/v1/routers/:name/leases
func (h *RouterHandler) GetLeases(w http.ResponseWriter, r *http.Request) {
	// Extract router name from path: /api/v1/routers/<name>/leases
	path := r.URL.Path
	trimmed := strings.TrimPrefix(path, "/api/v1/routers/")
	parts := strings.SplitN(trimmed, "/", 2)
	name := parts[0]
	if name == "" {
		WriteError(w, http.StatusBadRequest, "missing router name", "MISSING_NAME")
		return
	}

	if h.k8sClient == nil || h.restConfig == nil {
		WriteError(w, http.StatusServiceUnavailable, "kubernetes client not configured", "CLIENT_NOT_CONFIGURED")
		return
	}

	ns := r.URL.Query().Get("namespace")
	if ns == "" {
		// Find namespace by listing routers
		ns = h.findRouterNamespace(r, name)
		if ns == "" {
			WriteError(w, http.StatusNotFound, "router not found", "NOT_FOUND")
			return
		}
	}

	// Find router pod by label
	podList, err := h.k8sClient.CoreV1().Pods(ns).List(r.Context(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("vpc.roks.ibm.com/router=%s", name),
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to list router pods", "name", name, "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to find router pod", "POD_LIST_FAILED")
		return
	}
	if len(podList.Items) == 0 {
		WriteJSON(w, http.StatusOK, []model.DHCPLeaseResp{})
		return
	}

	pod := podList.Items[0]
	if pod.Status.Phase != corev1.PodRunning {
		WriteJSON(w, http.StatusOK, []model.DHCPLeaseResp{})
		return
	}

	// Exec into pod to read lease file
	req := h.k8sClient.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod.Name).
		Namespace(pod.Namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: "router",
			Command:   []string{"cat", "/var/lib/misc/dnsmasq.leases"},
			Stdout:    true,
			Stderr:    true,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(h.restConfig, "POST", req.URL())
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to create exec", "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to exec into router pod", "EXEC_FAILED")
		return
	}

	var stdout, stderr bytes.Buffer
	err = exec.StreamWithContext(r.Context(), remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		// File may not exist yet (no leases)
		slog.DebugContext(r.Context(), "exec failed (may be no leases)", "error", err, "stderr", stderr.String())
		WriteJSON(w, http.StatusOK, []model.DHCPLeaseResp{})
		return
	}

	leases := parseDnsmasqLeases(stdout.String())
	WriteJSON(w, http.StatusOK, leases)
}

// UpdateReservations handles PATCH /api/v1/routers/:name/reservations
func (h *RouterHandler) UpdateReservations(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	trimmed := strings.TrimPrefix(path, "/api/v1/routers/")
	parts := strings.SplitN(trimmed, "/", 2)
	name := parts[0]
	if name == "" {
		WriteError(w, http.StatusBadRequest, "missing router name", "MISSING_NAME")
		return
	}

	userInfo := auth.GetUserFromContext(r.Context())
	if userInfo == nil || userInfo.Name == "" {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	allowed, err := h.rbac.CheckAccess(r.Context(), userInfo.Name, userInfo.Groups, "update", "vpcrouters", "")
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "authorization check failed", "AUTHZ_CHECK_FAILED")
		return
	}
	if !allowed {
		WriteError(w, http.StatusForbidden, "forbidden", "FORBIDDEN")
		return
	}

	var req model.UpdateReservationsReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body", "INVALID_REQUEST")
		return
	}
	if req.Network == "" {
		WriteError(w, http.StatusBadRequest, "network name is required", "MISSING_NETWORK")
		return
	}

	if h.dynClient == nil {
		WriteError(w, http.StatusServiceUnavailable, "dynamic client not configured", "CLIENT_NOT_CONFIGURED")
		return
	}

	ns := r.URL.Query().Get("namespace")
	if ns == "" {
		ns = h.findRouterNamespace(r, name)
		if ns == "" {
			WriteError(w, http.StatusNotFound, "router not found", "NOT_FOUND")
			return
		}
	}

	// Get the current router
	obj, err := h.dynClient.Resource(vpcRouterGVR).Namespace(ns).Get(r.Context(), name, metav1.GetOptions{})
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to get VPCRouter", "name", name, "error", err)
		WriteError(w, http.StatusNotFound, "router not found", "NOT_FOUND")
		return
	}

	// Find or create the network entry in spec.networks
	specNets, _, _ := unstructured.NestedSlice(obj.Object, "spec", "networks")

	networkFound := false
	for i, item := range specNets {
		if m, ok := item.(map[string]interface{}); ok {
			if netName, _ := m["name"].(string); netName == req.Network {
				networkFound = true
				// Get or create dhcp map
				dhcpMap, _ := m["dhcp"].(map[string]interface{})
				if dhcpMap == nil {
					dhcpMap = map[string]interface{}{}
				}
				// Build reservations array
				dhcpMap["reservations"] = buildReservationsList(req.Reservations)
				m["dhcp"] = dhcpMap
				specNets[i] = m
				break
			}
		}
	}

	if !networkFound {
		WriteError(w, http.StatusBadRequest, fmt.Sprintf("network %q not found in router spec", req.Network), "NETWORK_NOT_FOUND")
		return
	}

	if err := unstructured.SetNestedSlice(obj.Object, specNets, "spec", "networks"); err != nil {
		slog.ErrorContext(r.Context(), "failed to set spec.networks", "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to update router", "UPDATE_FAILED")
		return
	}

	updated, err := h.dynClient.Resource(vpcRouterGVR).Namespace(ns).Update(r.Context(), obj, metav1.UpdateOptions{})
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to update VPCRouter", "name", name, "error", err)
		WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to update router: %v", err), "UPDATE_FAILED")
		return
	}

	WriteJSON(w, http.StatusOK, unstructuredToRouter(updated))
}

// findRouterNamespace locates the namespace for a named router via cross-namespace list.
func (h *RouterHandler) findRouterNamespace(r *http.Request, name string) string {
	if h.dynClient == nil {
		return ""
	}
	list, err := h.dynClient.Resource(vpcRouterGVR).Namespace("").List(r.Context(), metav1.ListOptions{})
	if err != nil {
		return ""
	}
	for _, item := range list.Items {
		if item.GetName() == name {
			return item.GetNamespace()
		}
	}
	return ""
}

// parseDnsmasqLeases parses dnsmasq lease file content into DHCPLeaseResp slice.
// Format: <expiry_epoch> <mac> <ip> <hostname> <client_id>
func parseDnsmasqLeases(content string) []model.DHCPLeaseResp {
	var leases []model.DHCPLeaseResp
	for _, line := range strings.Split(strings.TrimSpace(content), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		expiry, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			continue
		}

		lease := model.DHCPLeaseResp{
			ExpiresAt: expiry,
			MAC:       fields[1],
			IP:        fields[2],
			Hostname:  fields[3],
		}
		if lease.Hostname == "*" {
			lease.Hostname = ""
		}
		if len(fields) >= 5 && fields[4] != "*" {
			lease.ClientID = fields[4]
		}
		leases = append(leases, lease)
	}
	if leases == nil {
		leases = []model.DHCPLeaseResp{}
	}
	return leases
}

// buildReservationsList converts a slice of DHCPReservationResp to unstructured format.
func buildReservationsList(reservations []model.DHCPReservationResp) []interface{} {
	result := make([]interface{}, 0, len(reservations))
	for _, r := range reservations {
		rm := map[string]interface{}{
			"mac": r.MAC,
			"ip":  r.IP,
		}
		if r.Hostname != "" {
			rm["hostname"] = r.Hostname
		}
		result = append(result, rm)
	}
	return result
}

// unstructuredToRouter maps an unstructured VPCRouter to the response model.
func unstructuredToRouter(obj *unstructured.Unstructured) model.RouterResponse {
	rt := model.RouterResponse{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
		Networks:  []model.RouterNetworkResp{},
	}

	// Spec fields
	rt.Gateway, _, _ = unstructured.NestedString(obj.Object, "spec", "gateway")

	// Status fields
	rt.Phase, _, _ = unstructured.NestedString(obj.Object, "status", "phase")
	rt.TransitIP, _, _ = unstructured.NestedString(obj.Object, "status", "transitIP")
	rt.SyncStatus, _, _ = unstructured.NestedString(obj.Object, "status", "syncStatus")
	rt.IDSMode, _, _ = unstructured.NestedString(obj.Object, "status", "idsMode")
	rt.MetricsEnabled, _, _ = unstructured.NestedBool(obj.Object, "status", "metricsEnabled")
	rt.Mode, _, _ = unstructured.NestedString(obj.Object, "status", "mode")
	rt.XDPEnabled, _, _ = unstructured.NestedBool(obj.Object, "status", "xdpEnabled")

	// Extract IDS config from spec
	rt.IDS = extractIDS(obj)

	// Extract global DHCP from spec
	rt.DHCP = extractGlobalDHCP(obj)

	// Build spec.networks lookup keyed by name for per-network DHCP overrides
	specNetworks := buildSpecNetworkMap(obj)

	// Networks from status
	networkSlice, found, _ := unstructured.NestedSlice(obj.Object, "status", "networks")
	if found {
		for _, item := range networkSlice {
			if m, ok := item.(map[string]interface{}); ok {
				nr := model.RouterNetworkResp{}
				if v, ok := m["name"].(string); ok {
					nr.Name = v
				}
				if v, ok := m["address"].(string); ok {
					nr.Address = v
				}
				if v, ok := m["connected"].(bool); ok {
					nr.Connected = v
				}
				nr.DHCP = extractNetworkDHCP(m, specNetworks[nr.Name])
				rt.Networks = append(rt.Networks, nr)
			}
		}
	}

	// Advertised routes from status
	routeSlice, found, _ := unstructured.NestedStringSlice(obj.Object, "status", "advertisedRoutes")
	if found {
		rt.AdvertisedRoutes = routeSlice
	}

	// PodIP from status
	podIP, found, _ := unstructured.NestedString(obj.Object, "status", "podIP")
	if found {
		rt.PodIP = podIP
	}

	if ct := obj.GetCreationTimestamp(); !ct.IsZero() {
		rt.CreatedAt = ct.UTC().Format("2006-01-02T15:04:05Z")
	}

	return rt
}

// extractGlobalDHCP reads spec.dhcp and returns a RouterDHCPResp.
func extractGlobalDHCP(obj *unstructured.Unstructured) *model.RouterDHCPResp {
	dhcpMap, found, _ := unstructured.NestedMap(obj.Object, "spec", "dhcp")
	if !found || dhcpMap == nil {
		return nil
	}

	resp := &model.RouterDHCPResp{}
	if v, ok := dhcpMap["enabled"].(bool); ok {
		resp.Enabled = v
	}
	if v, ok := dhcpMap["leaseTime"].(string); ok {
		resp.LeaseTime = v
	}

	if dnsMap, ok := dhcpMap["dns"].(map[string]interface{}); ok {
		resp.DNS = extractDNSResp(dnsMap)
	}
	if optMap, ok := dhcpMap["options"].(map[string]interface{}); ok {
		resp.Options = extractOptionsResp(optMap)
	}

	return resp
}

// buildSpecNetworkMap returns a map of network name → spec network map for DHCP lookup.
func buildSpecNetworkMap(obj *unstructured.Unstructured) map[string]map[string]interface{} {
	result := make(map[string]map[string]interface{})
	specNets, found, _ := unstructured.NestedSlice(obj.Object, "spec", "networks")
	if !found {
		return result
	}
	for _, item := range specNets {
		if m, ok := item.(map[string]interface{}); ok {
			if name, ok := m["name"].(string); ok {
				result[name] = m
			}
		}
	}
	return result
}

// extractNetworkDHCP merges status DHCP and spec DHCP into a single response.
func extractNetworkDHCP(statusNet map[string]interface{}, specNet map[string]interface{}) *model.RouterNetworkDHCPResp {
	// Extract status DHCP
	var statusDHCP map[string]interface{}
	if d, ok := statusNet["dhcp"].(map[string]interface{}); ok {
		statusDHCP = d
	}

	// Extract spec DHCP (per-network overrides)
	var specDHCP map[string]interface{}
	if specNet != nil {
		if d, ok := specNet["dhcp"].(map[string]interface{}); ok {
			specDHCP = d
		}
	}

	// If neither has DHCP, return nil
	if statusDHCP == nil && specDHCP == nil {
		return nil
	}

	resp := &model.RouterNetworkDHCPResp{}

	// Status fields
	if statusDHCP != nil {
		if v, ok := statusDHCP["enabled"].(bool); ok {
			resp.Enabled = v
		}
		if v, ok := statusDHCP["poolStart"].(string); ok {
			resp.PoolStart = v
		}
		if v, ok := statusDHCP["poolEnd"].(string); ok {
			resp.PoolEnd = v
		}
		if v, ok := statusDHCP["reservationCount"].(int64); ok {
			resp.ReservationCount = int(v)
		} else if v, ok := statusDHCP["reservationCount"].(float64); ok {
			resp.ReservationCount = int(v)
		}
	}

	// Spec fields (per-network overrides)
	if specDHCP != nil {
		resp.HasOverride = true

		if v, ok := specDHCP["leaseTime"].(string); ok {
			resp.LeaseTime = v
		}
		if rng, ok := specDHCP["range"].(map[string]interface{}); ok {
			resp.RangeOverride = &model.DHCPRangeResp{}
			if v, ok := rng["start"].(string); ok {
				resp.RangeOverride.Start = v
			}
			if v, ok := rng["end"].(string); ok {
				resp.RangeOverride.End = v
			}
		}
		if resList, ok := specDHCP["reservations"].([]interface{}); ok {
			for _, r := range resList {
				if rm, ok := r.(map[string]interface{}); ok {
					res := model.DHCPReservationResp{}
					if v, ok := rm["mac"].(string); ok {
						res.MAC = v
					}
					if v, ok := rm["ip"].(string); ok {
						res.IP = v
					}
					if v, ok := rm["hostname"].(string); ok {
						res.Hostname = v
					}
					resp.Reservations = append(resp.Reservations, res)
				}
			}
		}
		if dnsMap, ok := specDHCP["dns"].(map[string]interface{}); ok {
			resp.DNS = extractDNSResp(dnsMap)
		}
		if optMap, ok := specDHCP["options"].(map[string]interface{}); ok {
			resp.Options = extractOptionsResp(optMap)
		}
	}

	return resp
}

// extractDNSResp extracts DNS settings from an unstructured map.
func extractDNSResp(m map[string]interface{}) *model.DHCPDNSResp {
	dns := &model.DHCPDNSResp{}
	if ns, ok := m["nameservers"].([]interface{}); ok {
		for _, v := range ns {
			if s, ok := v.(string); ok {
				dns.Nameservers = append(dns.Nameservers, s)
			}
		}
	}
	if sd, ok := m["searchDomains"].([]interface{}); ok {
		for _, v := range sd {
			if s, ok := v.(string); ok {
				dns.SearchDomains = append(dns.SearchDomains, s)
			}
		}
	}
	if v, ok := m["localDomain"].(string); ok {
		dns.LocalDomain = v
	}
	return dns
}

// extractOptionsResp extracts DHCP options from an unstructured map.
func extractOptionsResp(m map[string]interface{}) *model.DHCPOptionsResp {
	opts := &model.DHCPOptionsResp{}
	if v, ok := m["router"].(string); ok {
		opts.Router = v
	}
	if v, ok := m["mtu"].(int64); ok {
		i := int32(v)
		opts.MTU = &i
	} else if v, ok := m["mtu"].(float64); ok {
		i := int32(v)
		opts.MTU = &i
	}
	if ns, ok := m["ntpServers"].([]interface{}); ok {
		for _, v := range ns {
			if s, ok := v.(string); ok {
				opts.NTPServers = append(opts.NTPServers, s)
			}
		}
	}
	if cs, ok := m["custom"].([]interface{}); ok {
		for _, v := range cs {
			if s, ok := v.(string); ok {
				opts.Custom = append(opts.Custom, s)
			}
		}
	}
	return opts
}

// buildRouterUnstructured creates an unstructured VPCRouter from a request.
func buildRouterUnstructured(req model.RouterRequest) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "vpc.roks.ibm.com",
		Version: "v1alpha1",
		Kind:    "VPCRouter",
	})
	obj.SetName(req.Name)
	if req.Namespace != "" {
		obj.SetNamespace(req.Namespace)
	}

	networks := make([]interface{}, 0, len(req.Networks))
	for _, n := range req.Networks {
		netMap := map[string]interface{}{
			"name":    n.Name,
			"address": n.Address,
		}
		if n.DHCP != nil {
			netMap["dhcp"] = buildNetworkDHCPMap(n.DHCP)
		}
		networks = append(networks, netMap)
	}

	spec := map[string]interface{}{
		"gateway":  req.Gateway,
		"networks": networks,
	}

	if req.DHCP != nil {
		spec["dhcp"] = buildGlobalDHCPMap(req.DHCP)
	}

	if req.IDS != nil {
		spec["ids"] = buildIDSMap(req.IDS)
	}

	if req.Mode != "" {
		spec["mode"] = req.Mode
	}

	obj.Object["spec"] = spec

	return obj
}

// buildGlobalDHCPMap builds the spec.dhcp map from a request.
func buildGlobalDHCPMap(d *model.RouterDHCPReq) map[string]interface{} {
	m := map[string]interface{}{
		"enabled": d.Enabled,
	}
	if d.LeaseTime != "" {
		m["leaseTime"] = d.LeaseTime
	}
	if d.DNS != nil {
		m["dns"] = buildDNSMap(d.DNS)
	}
	if d.Options != nil {
		m["options"] = buildOptionsMap(d.Options)
	}
	return m
}

// buildNetworkDHCPMap builds a per-network dhcp map from a request.
func buildNetworkDHCPMap(d *model.RouterNetworkDHCPReq) map[string]interface{} {
	m := map[string]interface{}{}

	switch d.Override {
	case "enabled":
		enabled := true
		m["enabled"] = enabled
	case "disabled":
		enabled := false
		m["enabled"] = enabled
	}

	if d.RangeStart != "" && d.RangeEnd != "" {
		m["range"] = map[string]interface{}{
			"start": d.RangeStart,
			"end":   d.RangeEnd,
		}
	}
	if d.LeaseTime != "" {
		m["leaseTime"] = d.LeaseTime
	}
	if len(d.Reservations) > 0 {
		resList := make([]interface{}, 0, len(d.Reservations))
		for _, r := range d.Reservations {
			rm := map[string]interface{}{
				"mac": r.MAC,
				"ip":  r.IP,
			}
			if r.Hostname != "" {
				rm["hostname"] = r.Hostname
			}
			resList = append(resList, rm)
		}
		m["reservations"] = resList
	}
	if d.DNS != nil {
		m["dns"] = buildDNSMap(d.DNS)
	}
	if d.Options != nil {
		m["options"] = buildOptionsMap(d.Options)
	}
	return m
}

// buildDNSMap builds a dns map for DHCP configuration.
func buildDNSMap(dns *model.DHCPDNSResp) map[string]interface{} {
	m := map[string]interface{}{}
	if len(dns.Nameservers) > 0 {
		ns := make([]interface{}, len(dns.Nameservers))
		for i, v := range dns.Nameservers {
			ns[i] = v
		}
		m["nameservers"] = ns
	}
	if len(dns.SearchDomains) > 0 {
		sd := make([]interface{}, len(dns.SearchDomains))
		for i, v := range dns.SearchDomains {
			sd[i] = v
		}
		m["searchDomains"] = sd
	}
	if dns.LocalDomain != "" {
		m["localDomain"] = dns.LocalDomain
	}
	return m
}

// buildOptionsMap builds a DHCP options map.
func buildOptionsMap(opts *model.DHCPOptionsResp) map[string]interface{} {
	m := map[string]interface{}{}
	if opts.Router != "" {
		m["router"] = opts.Router
	}
	if opts.MTU != nil {
		m["mtu"] = int64(*opts.MTU)
	}
	if len(opts.NTPServers) > 0 {
		ns := make([]interface{}, len(opts.NTPServers))
		for i, v := range opts.NTPServers {
			ns[i] = v
		}
		m["ntpServers"] = ns
	}
	if len(opts.Custom) > 0 {
		cs := make([]interface{}, len(opts.Custom))
		for i, v := range opts.Custom {
			cs[i] = v
		}
		m["custom"] = cs
	}
	return m
}

// ── IDS/IPS Functions ──

// extractIDS reads spec.ids and returns a RouterIDSResp.
func extractIDS(obj *unstructured.Unstructured) *model.RouterIDSResp {
	idsMap, found, _ := unstructured.NestedMap(obj.Object, "spec", "ids")
	if !found || idsMap == nil {
		return nil
	}

	resp := &model.RouterIDSResp{}
	if v, ok := idsMap["enabled"].(bool); ok {
		resp.Enabled = v
	}
	if v, ok := idsMap["mode"].(string); ok {
		resp.Mode = v
	}
	if v, ok := idsMap["interfaces"].(string); ok {
		resp.Interfaces = v
	}
	if v, ok := idsMap["customRules"].(string); ok {
		resp.CustomRules = v
	}
	if v, ok := idsMap["syslogTarget"].(string); ok {
		resp.SyslogTarget = v
	}
	if v, ok := idsMap["image"].(string); ok {
		resp.Image = v
	}
	if v, ok := idsMap["nfqueueNum"].(int64); ok {
		i := int32(v)
		resp.NFQueueNum = &i
	} else if v, ok := idsMap["nfqueueNum"].(float64); ok {
		i := int32(v)
		resp.NFQueueNum = &i
	}

	return resp
}

// buildIDSMap builds the spec.ids map from a create request.
func buildIDSMap(req *model.RouterIDSReq) map[string]interface{} {
	m := map[string]interface{}{
		"enabled": req.Enabled,
		"mode":    req.Mode,
	}
	if req.Interfaces != "" {
		m["interfaces"] = req.Interfaces
	}
	if req.CustomRules != "" {
		m["customRules"] = req.CustomRules
	}
	if req.SyslogTarget != "" {
		m["syslogTarget"] = req.SyslogTarget
	}
	return m
}

// buildIDSMapFromUpdate builds the spec.ids map from an update request.
func buildIDSMapFromUpdate(req model.UpdateIDSReq) map[string]interface{} {
	if !req.Enabled {
		return map[string]interface{}{
			"enabled": false,
		}
	}
	m := map[string]interface{}{
		"enabled": true,
		"mode":    req.Mode,
	}
	if req.Interfaces != "" {
		m["interfaces"] = req.Interfaces
	}
	if req.CustomRules != "" {
		m["customRules"] = req.CustomRules
	}
	if req.SyslogTarget != "" {
		m["syslogTarget"] = req.SyslogTarget
	}
	return m
}

// UpdateIDS handles PATCH /api/v1/routers/:name/ids
func (h *RouterHandler) UpdateIDS(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	trimmed := strings.TrimPrefix(path, "/api/v1/routers/")
	parts := strings.SplitN(trimmed, "/", 2)
	name := parts[0]
	if name == "" {
		WriteError(w, http.StatusBadRequest, "missing router name", "MISSING_NAME")
		return
	}

	userInfo := auth.GetUserFromContext(r.Context())
	if userInfo == nil || userInfo.Name == "" {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	allowed, err := h.rbac.CheckAccess(r.Context(), userInfo.Name, userInfo.Groups, "update", "vpcrouters", "")
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "authorization check failed", "AUTHZ_CHECK_FAILED")
		return
	}
	if !allowed {
		WriteError(w, http.StatusForbidden, "forbidden", "FORBIDDEN")
		return
	}

	var req model.UpdateIDSReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body", "INVALID_REQUEST")
		return
	}

	if h.dynClient == nil {
		WriteError(w, http.StatusServiceUnavailable, "dynamic client not configured", "CLIENT_NOT_CONFIGURED")
		return
	}

	ns := r.URL.Query().Get("namespace")
	if ns == "" {
		ns = h.findRouterNamespace(r, name)
		if ns == "" {
			WriteError(w, http.StatusNotFound, "router not found", "NOT_FOUND")
			return
		}
	}

	obj, err := h.dynClient.Resource(vpcRouterGVR).Namespace(ns).Get(r.Context(), name, metav1.GetOptions{})
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to get VPCRouter", "name", name, "error", err)
		WriteError(w, http.StatusNotFound, "router not found", "NOT_FOUND")
		return
	}

	idsMap := buildIDSMapFromUpdate(req)
	if err := unstructured.SetNestedField(obj.Object, idsMap, "spec", "ids"); err != nil {
		slog.ErrorContext(r.Context(), "failed to set spec.ids", "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to update router", "UPDATE_FAILED")
		return
	}

	updated, err := h.dynClient.Resource(vpcRouterGVR).Namespace(ns).Update(r.Context(), obj, metav1.UpdateOptions{})
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to update VPCRouter", "name", name, "error", err)
		WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to update router: %v", err), "UPDATE_FAILED")
		return
	}

	WriteJSON(w, http.StatusOK, unstructuredToRouter(updated))
}
