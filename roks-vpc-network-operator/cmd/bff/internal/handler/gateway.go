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

var vpcGatewayGVR = schema.GroupVersionResource{
	Group:    "vpc.roks.ibm.com",
	Version:  "v1alpha1",
	Resource: "vpcgateways",
}

// GatewayHandler handles VPCGateway API operations.
type GatewayHandler struct {
	dynClient dynamic.Interface
	rbac      *auth.RBACChecker
}

// NewGatewayHandler creates a new gateway handler.
func NewGatewayHandler(dynClient dynamic.Interface, rbac *auth.RBACChecker) *GatewayHandler {
	return &GatewayHandler{
		dynClient: dynClient,
		rbac:      rbac,
	}
}

// ListGateways handles GET /api/v1/gateways
func (h *GatewayHandler) ListGateways(w http.ResponseWriter, r *http.Request) {
	if h.dynClient == nil {
		WriteError(w, http.StatusServiceUnavailable, "dynamic client not configured", "CLIENT_NOT_CONFIGURED")
		return
	}

	list, err := h.dynClient.Resource(vpcGatewayGVR).Namespace("").List(r.Context(), metav1.ListOptions{})
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to list VPCGateways", "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to list gateways", "LIST_FAILED")
		return
	}

	gateways := make([]model.GatewayResponse, 0, len(list.Items))
	for _, item := range list.Items {
		gateways = append(gateways, unstructuredToGateway(&item))
	}

	WriteJSON(w, http.StatusOK, gateways)
}

// GetGateway handles GET /api/v1/gateways/:name
func (h *GatewayHandler) GetGateway(w http.ResponseWriter, r *http.Request) {
	name := extractLastPathSegment(r.URL.Path, "/api/v1/gateways/")
	if name == "" {
		WriteError(w, http.StatusBadRequest, "missing gateway name", "MISSING_NAME")
		return
	}

	if h.dynClient == nil {
		WriteError(w, http.StatusServiceUnavailable, "dynamic client not configured", "CLIENT_NOT_CONFIGURED")
		return
	}

	// VPCGateway is cluster-scoped, so no namespace needed
	item, err := h.dynClient.Resource(vpcGatewayGVR).Namespace("").Get(r.Context(), name, metav1.GetOptions{})
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to get VPCGateway", "name", name, "error", err)
		WriteError(w, http.StatusNotFound, "gateway not found", "NOT_FOUND")
		return
	}

	gateway := unstructuredToGateway(item)
	WriteJSON(w, http.StatusOK, gateway)
}

// CreateGateway handles POST /api/v1/gateways
func (h *GatewayHandler) CreateGateway(w http.ResponseWriter, r *http.Request) {
	userInfo := auth.GetUserFromContext(r.Context())
	if userInfo == nil || userInfo.Name == "" {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	allowed, err := h.rbac.CheckAccess(r.Context(), userInfo.Name, userInfo.Groups, "create", "vpcgateways", "")
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "authorization check failed", "AUTHZ_CHECK_FAILED")
		return
	}
	if !allowed {
		WriteError(w, http.StatusForbidden, "forbidden", "FORBIDDEN")
		return
	}

	var req model.GatewayRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body", "INVALID_REQUEST")
		return
	}

	if h.dynClient == nil {
		WriteError(w, http.StatusServiceUnavailable, "dynamic client not configured", "CLIENT_NOT_CONFIGURED")
		return
	}

	obj := buildGatewayUnstructured(req)
	created, err := h.dynClient.Resource(vpcGatewayGVR).Namespace("").Create(r.Context(), obj, metav1.CreateOptions{})
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to create VPCGateway", "error", err)
		WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create gateway: %v", err), "CREATE_FAILED")
		return
	}

	gateway := unstructuredToGateway(created)
	WriteJSON(w, http.StatusCreated, gateway)
}

// DeleteGateway handles DELETE /api/v1/gateways/:name
func (h *GatewayHandler) DeleteGateway(w http.ResponseWriter, r *http.Request) {
	name := extractLastPathSegment(r.URL.Path, "/api/v1/gateways/")
	if name == "" {
		WriteError(w, http.StatusBadRequest, "missing gateway name", "MISSING_NAME")
		return
	}

	userInfo := auth.GetUserFromContext(r.Context())
	if userInfo == nil || userInfo.Name == "" {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	allowed, err := h.rbac.CheckAccess(r.Context(), userInfo.Name, userInfo.Groups, "delete", "vpcgateways", "")
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

	if err := h.dynClient.Resource(vpcGatewayGVR).Namespace("").Delete(r.Context(), name, metav1.DeleteOptions{}); err != nil {
		slog.ErrorContext(r.Context(), "failed to delete VPCGateway", "name", name, "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to delete gateway", "DELETE_FAILED")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// unstructuredToGateway maps an unstructured VPCGateway to the response model.
func unstructuredToGateway(obj *unstructured.Unstructured) model.GatewayResponse {
	spec, _, _ := unstructured.NestedMap(obj.Object, "spec")
	status, _, _ := unstructured.NestedMap(obj.Object, "status")

	gw := model.GatewayResponse{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}

	if spec != nil {
		gw.Zone, _, _ = unstructured.NestedString(obj.Object, "spec", "zone")
		gw.UplinkNetwork, _, _ = unstructured.NestedString(obj.Object, "spec", "uplinkNetworkRef")
		gw.TransitNetwork, _, _ = unstructured.NestedString(obj.Object, "spec", "transitNetworkRef")
	}

	if status != nil {
		gw.Phase, _, _ = unstructured.NestedString(obj.Object, "status", "phase")
		gw.VNIID, _, _ = unstructured.NestedString(obj.Object, "status", "vniID")
		gw.ReservedIP, _, _ = unstructured.NestedString(obj.Object, "status", "reservedIP")
		gw.FloatingIP, _, _ = unstructured.NestedString(obj.Object, "status", "floatingIP")
		gw.SyncStatus, _, _ = unstructured.NestedString(obj.Object, "status", "syncStatus")

		vpcRouteCount, found, _ := unstructured.NestedInt64(obj.Object, "status", "vpcRouteCount")
		if found {
			gw.VPCRouteCount = int(vpcRouteCount)
		}
		natRuleCount, found, _ := unstructured.NestedInt64(obj.Object, "status", "natRuleCount")
		if found {
			gw.NATRuleCount = int(natRuleCount)
		}
	}

	if ct := obj.GetCreationTimestamp(); !ct.IsZero() {
		gw.CreatedAt = ct.UTC().Format("2006-01-02T15:04:05Z")
	}

	return gw
}

// buildGatewayUnstructured creates an unstructured VPCGateway from a request.
func buildGatewayUnstructured(req model.GatewayRequest) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "vpc.roks.ibm.com",
		Version: "v1alpha1",
		Kind:    "VPCGateway",
	})
	obj.SetName(req.Name)

	spec := map[string]interface{}{
		"zone":              req.Zone,
		"uplinkNetworkRef":  req.Uplink,
	}
	if req.Transit != "" {
		spec["transitNetworkRef"] = req.Transit
	}
	obj.Object["spec"] = spec

	return obj
}
