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

var vpcL2BridgeGVR = schema.GroupVersionResource{
	Group:    "vpc.roks.ibm.com",
	Version:  "v1alpha1",
	Resource: "vpcl2bridges",
}

// L2BridgeHandler handles VPCL2Bridge API operations.
type L2BridgeHandler struct {
	dynClient dynamic.Interface
	rbac      *auth.RBACChecker
}

// NewL2BridgeHandler creates a new L2 bridge handler.
func NewL2BridgeHandler(dynClient dynamic.Interface, rbac *auth.RBACChecker) *L2BridgeHandler {
	return &L2BridgeHandler{
		dynClient: dynClient,
		rbac:      rbac,
	}
}

// ListL2Bridges handles GET /api/v1/l2bridges
func (h *L2BridgeHandler) ListL2Bridges(w http.ResponseWriter, r *http.Request) {
	if h.dynClient == nil {
		WriteError(w, http.StatusServiceUnavailable, "dynamic client not configured", "CLIENT_NOT_CONFIGURED")
		return
	}

	list, err := h.dynClient.Resource(vpcL2BridgeGVR).Namespace("").List(r.Context(), metav1.ListOptions{})
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to list VPCL2Bridges", "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to list l2 bridges", "LIST_FAILED")
		return
	}

	bridges := make([]model.L2BridgeResponse, 0, len(list.Items))
	for _, item := range list.Items {
		bridges = append(bridges, unstructuredToL2Bridge(&item))
	}

	WriteJSON(w, http.StatusOK, bridges)
}

// GetL2Bridge handles GET /api/v1/l2bridges/:name
func (h *L2BridgeHandler) GetL2Bridge(w http.ResponseWriter, r *http.Request) {
	name := extractLastPathSegment(r.URL.Path, "/api/v1/l2bridges/")
	if name == "" {
		WriteError(w, http.StatusBadRequest, "missing l2 bridge name", "MISSING_NAME")
		return
	}

	if h.dynClient == nil {
		WriteError(w, http.StatusServiceUnavailable, "dynamic client not configured", "CLIENT_NOT_CONFIGURED")
		return
	}

	ns := r.URL.Query().Get("namespace")
	if ns != "" {
		item, err := h.dynClient.Resource(vpcL2BridgeGVR).Namespace(ns).Get(r.Context(), name, metav1.GetOptions{})
		if err != nil {
			slog.ErrorContext(r.Context(), "failed to get VPCL2Bridge", "name", name, "namespace", ns, "error", err)
			WriteError(w, http.StatusNotFound, "l2 bridge not found", "NOT_FOUND")
			return
		}
		WriteJSON(w, http.StatusOK, unstructuredToL2Bridge(item))
		return
	}

	// No namespace — cross-namespace List + filter by name
	list, err := h.dynClient.Resource(vpcL2BridgeGVR).Namespace("").List(r.Context(), metav1.ListOptions{})
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to list VPCL2Bridges for get", "name", name, "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to list l2 bridges", "LIST_FAILED")
		return
	}
	for _, item := range list.Items {
		if item.GetName() == name {
			WriteJSON(w, http.StatusOK, unstructuredToL2Bridge(&item))
			return
		}
	}
	WriteError(w, http.StatusNotFound, "l2 bridge not found", "NOT_FOUND")
}

// CreateL2Bridge handles POST /api/v1/l2bridges
func (h *L2BridgeHandler) CreateL2Bridge(w http.ResponseWriter, r *http.Request) {
	userInfo := auth.GetUserFromContext(r.Context())
	if userInfo == nil || userInfo.Name == "" {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	allowed, err := h.rbac.CheckAccess(r.Context(), userInfo.Name, userInfo.Groups, "create", "vpcl2bridges", "")
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "authorization check failed", "AUTHZ_CHECK_FAILED")
		return
	}
	if !allowed {
		WriteError(w, http.StatusForbidden, "forbidden", "FORBIDDEN")
		return
	}

	var req model.L2BridgeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body", "INVALID_REQUEST")
		return
	}

	if h.dynClient == nil {
		WriteError(w, http.StatusServiceUnavailable, "dynamic client not configured", "CLIENT_NOT_CONFIGURED")
		return
	}

	obj := buildL2BridgeUnstructured(req)
	ns := req.Namespace
	if ns == "" {
		ns = "default"
	}
	created, err := h.dynClient.Resource(vpcL2BridgeGVR).Namespace(ns).Create(r.Context(), obj, metav1.CreateOptions{})
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to create VPCL2Bridge", "error", err)
		WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create l2 bridge: %v", err), "CREATE_FAILED")
		return
	}

	bridge := unstructuredToL2Bridge(created)
	WriteJSON(w, http.StatusCreated, bridge)
}

// DeleteL2Bridge handles DELETE /api/v1/l2bridges/:name
func (h *L2BridgeHandler) DeleteL2Bridge(w http.ResponseWriter, r *http.Request) {
	name := extractLastPathSegment(r.URL.Path, "/api/v1/l2bridges/")
	if name == "" {
		WriteError(w, http.StatusBadRequest, "missing l2 bridge name", "MISSING_NAME")
		return
	}

	userInfo := auth.GetUserFromContext(r.Context())
	if userInfo == nil || userInfo.Name == "" {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	allowed, err := h.rbac.CheckAccess(r.Context(), userInfo.Name, userInfo.Groups, "delete", "vpcl2bridges", "")
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
		// Fall back to cross-namespace list to find the namespace
		list, err := h.dynClient.Resource(vpcL2BridgeGVR).Namespace("").List(r.Context(), metav1.ListOptions{})
		if err != nil {
			slog.ErrorContext(r.Context(), "failed to list VPCL2Bridges for delete", "name", name, "error", err)
			WriteError(w, http.StatusInternalServerError, "failed to find l2 bridge", "LIST_FAILED")
			return
		}
		for _, item := range list.Items {
			if item.GetName() == name {
				ns = item.GetNamespace()
				break
			}
		}
		if ns == "" {
			WriteError(w, http.StatusNotFound, "l2 bridge not found", "NOT_FOUND")
			return
		}
	}

	if err := h.dynClient.Resource(vpcL2BridgeGVR).Namespace(ns).Delete(r.Context(), name, metav1.DeleteOptions{}); err != nil {
		slog.ErrorContext(r.Context(), "failed to delete VPCL2Bridge", "name", name, "namespace", ns, "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to delete l2 bridge", "DELETE_FAILED")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// unstructuredToL2Bridge maps an unstructured VPCL2Bridge to the response model.
func unstructuredToL2Bridge(obj *unstructured.Unstructured) model.L2BridgeResponse {
	br := model.L2BridgeResponse{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}

	// Spec fields
	br.Type, _, _ = unstructured.NestedString(obj.Object, "spec", "type")
	br.GatewayRef, _, _ = unstructured.NestedString(obj.Object, "spec", "gatewayRef")
	br.RemoteEndpoint, _, _ = unstructured.NestedString(obj.Object, "spec", "remote", "endpoint")

	// NetworkRef
	nrName, _, _ := unstructured.NestedString(obj.Object, "spec", "networkRef", "name")
	nrKind, _, _ := unstructured.NestedString(obj.Object, "spec", "networkRef", "kind")
	nrNS, _, _ := unstructured.NestedString(obj.Object, "spec", "networkRef", "namespace")
	br.NetworkRef = model.L2BridgeNetworkRefResp{
		Name:      nrName,
		Kind:      nrKind,
		Namespace: nrNS,
	}

	// MTU spec
	tunnelMTU, found, _ := unstructured.NestedInt64(obj.Object, "spec", "mtu", "tunnelMTU")
	if found {
		br.TunnelMTU = int32(tunnelMTU)
	}
	mssClamp, found, _ := unstructured.NestedBool(obj.Object, "spec", "mtu", "mssClamp")
	if found {
		br.MSSClamp = &mssClamp
	}

	// Status fields
	br.Phase, _, _ = unstructured.NestedString(obj.Object, "status", "phase")
	br.TunnelEndpoint, _, _ = unstructured.NestedString(obj.Object, "status", "tunnelEndpoint")
	br.PodName, _, _ = unstructured.NestedString(obj.Object, "status", "podName")
	br.SyncStatus, _, _ = unstructured.NestedString(obj.Object, "status", "syncStatus")

	remoteMACsLearned, found, _ := unstructured.NestedInt64(obj.Object, "status", "remoteMACsLearned")
	if found {
		br.RemoteMACsLearned = int32(remoteMACsLearned)
	}
	localMACsAdvertised, found, _ := unstructured.NestedInt64(obj.Object, "status", "localMACsAdvertised")
	if found {
		br.LocalMACsAdvertised = int32(localMACsAdvertised)
	}
	bytesIn, found, _ := unstructured.NestedInt64(obj.Object, "status", "bytesIn")
	if found {
		br.BytesIn = bytesIn
	}
	bytesOut, found, _ := unstructured.NestedInt64(obj.Object, "status", "bytesOut")
	if found {
		br.BytesOut = bytesOut
	}

	lastHandshake, _, _ := unstructured.NestedString(obj.Object, "status", "lastHandshake")
	if lastHandshake != "" {
		br.LastHandshake = lastHandshake
	}

	if ct := obj.GetCreationTimestamp(); !ct.IsZero() {
		br.CreatedAt = ct.UTC().Format("2006-01-02T15:04:05Z")
	}

	return br
}

// buildL2BridgeUnstructured creates an unstructured VPCL2Bridge from a request.
func buildL2BridgeUnstructured(req model.L2BridgeRequest) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "vpc.roks.ibm.com",
		Version: "v1alpha1",
		Kind:    "VPCL2Bridge",
	})
	obj.SetName(req.Name)
	if req.Namespace != "" {
		obj.SetNamespace(req.Namespace)
	}

	// Build networkRef
	networkRef := map[string]interface{}{
		"name": req.NetworkRef.Name,
	}
	if req.NetworkRef.Kind != "" {
		networkRef["kind"] = req.NetworkRef.Kind
	}
	if req.NetworkRef.Namespace != "" {
		networkRef["namespace"] = req.NetworkRef.Namespace
	}

	// Build remote
	remote := map[string]interface{}{
		"endpoint": req.Remote.Endpoint,
	}
	if req.Remote.WireGuard != nil {
		wg := map[string]interface{}{
			"peerPublicKey":       req.Remote.WireGuard.PeerPublicKey,
			"tunnelAddressLocal":  req.Remote.WireGuard.TunnelAddressLocal,
			"tunnelAddressRemote": req.Remote.WireGuard.TunnelAddressRemote,
			"privateKey": map[string]interface{}{
				"name": req.Remote.WireGuard.PrivateKeySecret,
				"key":  req.Remote.WireGuard.PrivateKeySecretKey,
			},
		}
		if req.Remote.WireGuard.ListenPort != nil {
			wg["listenPort"] = int64(*req.Remote.WireGuard.ListenPort)
		}
		remote["wireGuard"] = wg
	}

	spec := map[string]interface{}{
		"type":       req.Type,
		"gatewayRef": req.GatewayRef,
		"networkRef": networkRef,
		"remote":     remote,
	}

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
