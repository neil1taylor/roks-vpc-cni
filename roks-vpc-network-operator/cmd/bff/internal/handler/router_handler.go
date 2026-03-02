package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/IBM/roks-vpc-network-operator/cmd/bff/internal/auth"
	"github.com/IBM/roks-vpc-network-operator/cmd/bff/internal/model"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

var vpcRouterGVR = schema.GroupVersionResource{
	Group:    "vpc.roks.ibm.com",
	Version:  "v1alpha1",
	Resource: "vpcrouters",
}

// RouterHandler handles VPCRouter API operations.
type RouterHandler struct {
	dynClient dynamic.Interface
	rbac      *auth.RBACChecker
}

// NewRouterHandler creates a new router handler.
func NewRouterHandler(dynClient dynamic.Interface, rbac *auth.RBACChecker) *RouterHandler {
	return &RouterHandler{
		dynClient: dynClient,
		rbac:      rbac,
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
