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
		networks = append(networks, map[string]interface{}{
			"name":    n.Name,
			"address": n.Address,
		})
	}

	spec := map[string]interface{}{
		"gateway":  req.Gateway,
		"networks": networks,
	}
	obj.Object["spec"] = spec

	return obj
}
