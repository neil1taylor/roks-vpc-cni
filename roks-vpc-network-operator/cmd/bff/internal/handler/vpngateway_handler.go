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
	"k8s.io/client-go/kubernetes"
)

var vpcVPNGatewayGVR = schema.GroupVersionResource{
	Group:    "vpc.roks.ibm.com",
	Version:  "v1alpha1",
	Resource: "vpcvpngateways",
}

// VPNGatewayHandler handles VPCVPNGateway API operations.
type VPNGatewayHandler struct {
	dynClient dynamic.Interface
	rbac      *auth.RBACChecker
	k8sClient kubernetes.Interface
}

// NewVPNGatewayHandler creates a new VPN gateway handler.
func NewVPNGatewayHandler(dynClient dynamic.Interface, rbac *auth.RBACChecker, k8sClient kubernetes.Interface) *VPNGatewayHandler {
	return &VPNGatewayHandler{
		dynClient: dynClient,
		rbac:      rbac,
		k8sClient: k8sClient,
	}
}

// ListVPNGateways handles GET /api/v1/vpn-gateways
func (h *VPNGatewayHandler) ListVPNGateways(w http.ResponseWriter, r *http.Request) {
	if h.dynClient == nil {
		WriteError(w, http.StatusServiceUnavailable, "dynamic client not configured", "CLIENT_NOT_CONFIGURED")
		return
	}

	list, err := h.dynClient.Resource(vpcVPNGatewayGVR).Namespace("").List(r.Context(), metav1.ListOptions{})
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to list VPCVPNGateways", "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to list vpn gateways", "LIST_FAILED")
		return
	}

	gateways := make([]model.VPNGatewayResponse, 0, len(list.Items))
	for _, item := range list.Items {
		gateways = append(gateways, unstructuredToVPNGateway(&item))
	}

	WriteJSON(w, http.StatusOK, gateways)
}

// GetVPNGateway handles GET /api/v1/vpn-gateways/:name
func (h *VPNGatewayHandler) GetVPNGateway(w http.ResponseWriter, r *http.Request) {
	name := extractLastPathSegment(r.URL.Path, "/api/v1/vpn-gateways/")
	if name == "" {
		WriteError(w, http.StatusBadRequest, "missing vpn gateway name", "MISSING_NAME")
		return
	}

	if h.dynClient == nil {
		WriteError(w, http.StatusServiceUnavailable, "dynamic client not configured", "CLIENT_NOT_CONFIGURED")
		return
	}

	ns := r.URL.Query().Get("namespace")
	if ns != "" {
		item, err := h.dynClient.Resource(vpcVPNGatewayGVR).Namespace(ns).Get(r.Context(), name, metav1.GetOptions{})
		if err != nil {
			slog.ErrorContext(r.Context(), "failed to get VPCVPNGateway", "name", name, "namespace", ns, "error", err)
			WriteError(w, http.StatusNotFound, "vpn gateway not found", "NOT_FOUND")
			return
		}
		WriteJSON(w, http.StatusOK, unstructuredToVPNGateway(item))
		return
	}

	// No namespace — cross-namespace List + filter by name
	list, err := h.dynClient.Resource(vpcVPNGatewayGVR).Namespace("").List(r.Context(), metav1.ListOptions{})
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to list VPCVPNGateways for get", "name", name, "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to list vpn gateways", "LIST_FAILED")
		return
	}
	for _, item := range list.Items {
		if item.GetName() == name {
			WriteJSON(w, http.StatusOK, unstructuredToVPNGateway(&item))
			return
		}
	}
	WriteError(w, http.StatusNotFound, "vpn gateway not found", "NOT_FOUND")
}

// CreateVPNGateway handles POST /api/v1/vpn-gateways
func (h *VPNGatewayHandler) CreateVPNGateway(w http.ResponseWriter, r *http.Request) {
	userInfo := auth.GetUserFromContext(r.Context())
	if userInfo == nil || userInfo.Name == "" {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	allowed, err := h.rbac.CheckAccess(r.Context(), userInfo.Name, userInfo.Groups, "create", "vpcvpngateways", "")
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "authorization check failed", "AUTHZ_CHECK_FAILED")
		return
	}
	if !allowed {
		WriteError(w, http.StatusForbidden, "forbidden", "FORBIDDEN")
		return
	}

	var req model.VPNGatewayRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body", "INVALID_REQUEST")
		return
	}

	if h.dynClient == nil {
		WriteError(w, http.StatusServiceUnavailable, "dynamic client not configured", "CLIENT_NOT_CONFIGURED")
		return
	}

	obj := buildVPNGatewayUnstructured(req)
	ns := req.Namespace
	if ns == "" {
		ns = "default"
	}
	created, err := h.dynClient.Resource(vpcVPNGatewayGVR).Namespace(ns).Create(r.Context(), obj, metav1.CreateOptions{})
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to create VPCVPNGateway", "error", err)
		WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create vpn gateway: %v", err), "CREATE_FAILED")
		return
	}

	gw := unstructuredToVPNGateway(created)
	WriteJSON(w, http.StatusCreated, gw)
}

// DeleteVPNGateway handles DELETE /api/v1/vpn-gateways/:name
func (h *VPNGatewayHandler) DeleteVPNGateway(w http.ResponseWriter, r *http.Request) {
	name := extractLastPathSegment(r.URL.Path, "/api/v1/vpn-gateways/")
	if name == "" {
		WriteError(w, http.StatusBadRequest, "missing vpn gateway name", "MISSING_NAME")
		return
	}

	userInfo := auth.GetUserFromContext(r.Context())
	if userInfo == nil || userInfo.Name == "" {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	allowed, err := h.rbac.CheckAccess(r.Context(), userInfo.Name, userInfo.Groups, "delete", "vpcvpngateways", "")
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
		list, err := h.dynClient.Resource(vpcVPNGatewayGVR).Namespace("").List(r.Context(), metav1.ListOptions{})
		if err != nil {
			slog.ErrorContext(r.Context(), "failed to list VPCVPNGateways for delete", "name", name, "error", err)
			WriteError(w, http.StatusInternalServerError, "failed to find vpn gateway", "LIST_FAILED")
			return
		}
		for _, item := range list.Items {
			if item.GetName() == name {
				ns = item.GetNamespace()
				break
			}
		}
		if ns == "" {
			WriteError(w, http.StatusNotFound, "vpn gateway not found", "NOT_FOUND")
			return
		}
	}

	if err := h.dynClient.Resource(vpcVPNGatewayGVR).Namespace(ns).Delete(r.Context(), name, metav1.DeleteOptions{}); err != nil {
		slog.ErrorContext(r.Context(), "failed to delete VPCVPNGateway", "name", name, "namespace", ns, "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to delete vpn gateway", "DELETE_FAILED")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// unstructuredToVPNGateway maps an unstructured VPCVPNGateway to the response model.
func unstructuredToVPNGateway(obj *unstructured.Unstructured) model.VPNGatewayResponse {
	resp := model.VPNGatewayResponse{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}

	// Spec fields
	resp.Protocol, _, _ = unstructured.NestedString(obj.Object, "spec", "protocol")
	resp.GatewayRef, _, _ = unstructured.NestedString(obj.Object, "spec", "gatewayRef")

	// Tunnel count from spec
	tunnels, found, _ := unstructured.NestedSlice(obj.Object, "spec", "tunnels")
	if found {
		resp.TotalTunnels = int32(len(tunnels))
	}

	// MTU spec
	tunnelMTU, found, _ := unstructured.NestedInt64(obj.Object, "spec", "mtu", "tunnelMTU")
	if found {
		resp.TunnelMTU = int32(tunnelMTU)
	}
	mssClamp, found, _ := unstructured.NestedBool(obj.Object, "spec", "mtu", "mssClamp")
	if found {
		resp.MSSClamp = &mssClamp
	}

	// Status fields
	resp.Phase, _, _ = unstructured.NestedString(obj.Object, "status", "phase")
	resp.TunnelEndpoint, _, _ = unstructured.NestedString(obj.Object, "status", "tunnelEndpoint")
	resp.PodName, _, _ = unstructured.NestedString(obj.Object, "status", "podName")
	resp.SyncStatus, _, _ = unstructured.NestedString(obj.Object, "status", "syncStatus")
	resp.Message, _, _ = unstructured.NestedString(obj.Object, "status", "message")

	activeTunnels, found, _ := unstructured.NestedInt64(obj.Object, "status", "activeTunnels")
	if found {
		resp.ActiveTunnels = int32(activeTunnels)
	}
	connectedClients, found, _ := unstructured.NestedInt64(obj.Object, "status", "connectedClients")
	if found {
		resp.ConnectedClients = int32(connectedClients)
	}

	// Advertised routes
	routeSlice, found, _ := unstructured.NestedStringSlice(obj.Object, "status", "advertisedRoutes")
	if found {
		resp.AdvertisedRoutes = routeSlice
	}

	// Per-tunnel status
	tunnelStatuses, found, _ := unstructured.NestedSlice(obj.Object, "status", "tunnels")
	if found {
		for _, ts := range tunnelStatuses {
			tsMap, ok := ts.(map[string]interface{})
			if !ok {
				continue
			}
			tunnelResp := model.VPNTunnelStatusResp{}
			if v, ok := tsMap["name"].(string); ok {
				tunnelResp.Name = v
			}
			if v, ok := tsMap["status"].(string); ok {
				tunnelResp.Status = v
			}
			if v, ok := tsMap["lastHandshake"].(string); ok {
				tunnelResp.LastHandshake = v
			}
			if v, ok := tsMap["bytesIn"].(int64); ok {
				tunnelResp.BytesIn = v
			}
			if v, ok := tsMap["bytesOut"].(int64); ok {
				tunnelResp.BytesOut = v
			}
			resp.Tunnels = append(resp.Tunnels, tunnelResp)
		}
	}

	if ct := obj.GetCreationTimestamp(); !ct.IsZero() {
		resp.CreatedAt = ct.UTC().Format("2006-01-02T15:04:05Z")
	}

	return resp
}

// buildVPNGatewayUnstructured creates an unstructured VPCVPNGateway from a request.
func buildVPNGatewayUnstructured(req model.VPNGatewayRequest) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "vpc.roks.ibm.com",
		Version: "v1alpha1",
		Kind:    "VPCVPNGateway",
	})
	obj.SetName(req.Name)
	if req.Namespace != "" {
		obj.SetNamespace(req.Namespace)
	}

	spec := map[string]interface{}{
		"protocol":   req.Protocol,
		"gatewayRef": req.GatewayRef,
	}

	// WireGuard config
	if req.WireGuard != nil {
		wg := map[string]interface{}{
			"privateKey": map[string]interface{}{
				"name": req.WireGuard.PrivateKeySecret,
				"key":  req.WireGuard.PrivateKeySecretKey,
			},
		}
		if req.WireGuard.ListenPort != nil {
			wg["listenPort"] = int64(*req.WireGuard.ListenPort)
		}
		spec["wireGuard"] = wg
	}

	// IPsec config
	if req.IPsec != nil {
		ipsec := map[string]interface{}{}
		if req.IPsec.Image != "" {
			ipsec["image"] = req.IPsec.Image
		}
		spec["ipsec"] = ipsec
	}

	// OpenVPN config
	if req.OpenVPN != nil {
		ovpn := map[string]interface{}{
			"ca":   map[string]interface{}{"name": req.OpenVPN.CASecret, "key": req.OpenVPN.CASecretKey},
			"cert": map[string]interface{}{"name": req.OpenVPN.CertSecret, "key": req.OpenVPN.CertSecretKey},
			"key":  map[string]interface{}{"name": req.OpenVPN.KeySecret, "key": req.OpenVPN.KeySecretKey},
		}
		if req.OpenVPN.DHSecret != "" {
			ovpn["dh"] = map[string]interface{}{"name": req.OpenVPN.DHSecret, "key": req.OpenVPN.DHSecretKey}
		}
		if req.OpenVPN.TLSAuthSecret != "" {
			ovpn["tlsAuth"] = map[string]interface{}{"name": req.OpenVPN.TLSAuthSecret, "key": req.OpenVPN.TLSAuthSecretKey}
		}
		if req.OpenVPN.ListenPort != nil {
			ovpn["listenPort"] = int64(*req.OpenVPN.ListenPort)
		}
		if req.OpenVPN.Proto != "" {
			ovpn["proto"] = req.OpenVPN.Proto
		}
		if req.OpenVPN.Cipher != "" {
			ovpn["cipher"] = req.OpenVPN.Cipher
		}
		if req.OpenVPN.ClientSubnet != "" {
			ovpn["clientSubnet"] = req.OpenVPN.ClientSubnet
		}
		if req.OpenVPN.Image != "" {
			ovpn["image"] = req.OpenVPN.Image
		}
		spec["openVPN"] = ovpn
	}

	// Remote access
	if req.RemoteAccess != nil {
		ra := map[string]interface{}{
			"enabled": req.RemoteAccess.Enabled,
		}
		if req.RemoteAccess.AddressPool != "" {
			ra["addressPool"] = req.RemoteAccess.AddressPool
		}
		if len(req.RemoteAccess.DNSServers) > 0 {
			dns := make([]interface{}, len(req.RemoteAccess.DNSServers))
			for i, d := range req.RemoteAccess.DNSServers {
				dns[i] = d
			}
			ra["dnsServers"] = dns
		}
		if req.RemoteAccess.MaxClients != nil {
			ra["maxClients"] = int64(*req.RemoteAccess.MaxClients)
		}
		spec["remoteAccess"] = ra
	}

	// Local networks
	if len(req.LocalNetworks) > 0 {
		nets := make([]interface{}, 0, len(req.LocalNetworks))
		for _, ln := range req.LocalNetworks {
			entry := map[string]interface{}{}
			if ln.CIDR != "" {
				entry["cidr"] = ln.CIDR
			}
			nets = append(nets, entry)
		}
		spec["localNetworks"] = nets
	}

	// Tunnels
	tunnels := make([]interface{}, 0, len(req.Tunnels))
	for _, t := range req.Tunnels {
		tunnel := map[string]interface{}{
			"name":           t.Name,
			"remoteEndpoint": t.RemoteEndpoint,
		}
		if len(t.RemoteNetworks) > 0 {
			nets := make([]interface{}, len(t.RemoteNetworks))
			for i, n := range t.RemoteNetworks {
				nets[i] = n
			}
			tunnel["remoteNetworks"] = nets
		}
		if t.PeerPublicKey != "" {
			tunnel["peerPublicKey"] = t.PeerPublicKey
		}
		if t.TunnelAddressLocal != "" {
			tunnel["tunnelAddressLocal"] = t.TunnelAddressLocal
		}
		if t.TunnelAddressRemote != "" {
			tunnel["tunnelAddressRemote"] = t.TunnelAddressRemote
		}
		if t.PresharedKeySecret != "" {
			tunnel["presharedKey"] = map[string]interface{}{
				"name": t.PresharedKeySecret,
				"key":  t.PresharedKeySecretKey,
			}
		}
		tunnels = append(tunnels, tunnel)
	}
	spec["tunnels"] = tunnels

	// MTU settings
	if req.MTU != nil {
		mtu := map[string]interface{}{}
		if req.MTU.TunnelMTU != nil {
			mtu["tunnelMTU"] = int64(*req.MTU.TunnelMTU)
		}
		if req.MTU.MSSClamp != nil {
			mtu["mssClamp"] = *req.MTU.MSSClamp
		}
		if len(mtu) > 0 {
			spec["mtu"] = mtu
		}
	}

	obj.Object["spec"] = spec
	return obj
}
