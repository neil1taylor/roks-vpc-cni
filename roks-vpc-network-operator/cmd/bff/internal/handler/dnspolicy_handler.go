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

var vpcDNSPolicyGVR = schema.GroupVersionResource{
	Group:    "vpc.roks.ibm.com",
	Version:  "v1alpha1",
	Resource: "vpcdnspolicies",
}

// DNSPolicyHandler handles VPCDNSPolicy API operations.
type DNSPolicyHandler struct {
	dynClient dynamic.Interface
	rbac      *auth.RBACChecker
}

// NewDNSPolicyHandler creates a new DNS policy handler.
func NewDNSPolicyHandler(dynClient dynamic.Interface, rbac *auth.RBACChecker) *DNSPolicyHandler {
	return &DNSPolicyHandler{
		dynClient: dynClient,
		rbac:      rbac,
	}
}

// ListDNSPolicies handles GET /api/v1/dns-policies
func (h *DNSPolicyHandler) ListDNSPolicies(w http.ResponseWriter, r *http.Request) {
	if h.dynClient == nil {
		WriteError(w, http.StatusServiceUnavailable, "dynamic client not configured", "CLIENT_NOT_CONFIGURED")
		return
	}

	list, err := h.dynClient.Resource(vpcDNSPolicyGVR).Namespace("").List(r.Context(), metav1.ListOptions{})
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to list VPCDNSPolicies", "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to list dns policies", "LIST_FAILED")
		return
	}

	policies := make([]model.DNSPolicyResponse, 0, len(list.Items))
	for _, item := range list.Items {
		policies = append(policies, unstructuredToDNSPolicy(&item))
	}

	WriteJSON(w, http.StatusOK, policies)
}

// GetDNSPolicy handles GET /api/v1/dns-policies/:name
func (h *DNSPolicyHandler) GetDNSPolicy(w http.ResponseWriter, r *http.Request) {
	name := extractLastPathSegment(r.URL.Path, "/api/v1/dns-policies/")
	if name == "" {
		WriteError(w, http.StatusBadRequest, "missing dns policy name", "MISSING_NAME")
		return
	}

	if h.dynClient == nil {
		WriteError(w, http.StatusServiceUnavailable, "dynamic client not configured", "CLIENT_NOT_CONFIGURED")
		return
	}

	ns := r.URL.Query().Get("namespace")
	if ns != "" {
		item, err := h.dynClient.Resource(vpcDNSPolicyGVR).Namespace(ns).Get(r.Context(), name, metav1.GetOptions{})
		if err != nil {
			slog.ErrorContext(r.Context(), "failed to get VPCDNSPolicy", "name", name, "namespace", ns, "error", err)
			WriteError(w, http.StatusNotFound, "dns policy not found", "NOT_FOUND")
			return
		}
		WriteJSON(w, http.StatusOK, unstructuredToDNSPolicy(item))
		return
	}

	// No namespace — cross-namespace List + filter by name
	list, err := h.dynClient.Resource(vpcDNSPolicyGVR).Namespace("").List(r.Context(), metav1.ListOptions{})
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to list VPCDNSPolicies for get", "name", name, "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to list dns policies", "LIST_FAILED")
		return
	}
	for _, item := range list.Items {
		if item.GetName() == name {
			WriteJSON(w, http.StatusOK, unstructuredToDNSPolicy(&item))
			return
		}
	}
	WriteError(w, http.StatusNotFound, "dns policy not found", "NOT_FOUND")
}

// CreateDNSPolicy handles POST /api/v1/dns-policies
func (h *DNSPolicyHandler) CreateDNSPolicy(w http.ResponseWriter, r *http.Request) {
	userInfo := auth.GetUserFromContext(r.Context())
	if userInfo == nil || userInfo.Name == "" {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	allowed, err := h.rbac.CheckAccess(r.Context(), userInfo.Name, userInfo.Groups, "create", "vpcdnspolicies", "")
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "authorization check failed", "AUTHZ_CHECK_FAILED")
		return
	}
	if !allowed {
		WriteError(w, http.StatusForbidden, "forbidden", "FORBIDDEN")
		return
	}

	var req model.DNSPolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body", "INVALID_REQUEST")
		return
	}

	if h.dynClient == nil {
		WriteError(w, http.StatusServiceUnavailable, "dynamic client not configured", "CLIENT_NOT_CONFIGURED")
		return
	}

	obj := buildDNSPolicyUnstructured(req)
	ns := req.Namespace
	if ns == "" {
		ns = "default"
	}
	created, err := h.dynClient.Resource(vpcDNSPolicyGVR).Namespace(ns).Create(r.Context(), obj, metav1.CreateOptions{})
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to create VPCDNSPolicy", "error", err)
		WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create dns policy: %v", err), "CREATE_FAILED")
		return
	}

	policy := unstructuredToDNSPolicy(created)
	WriteJSON(w, http.StatusCreated, policy)
}

// DeleteDNSPolicy handles DELETE /api/v1/dns-policies/:name
func (h *DNSPolicyHandler) DeleteDNSPolicy(w http.ResponseWriter, r *http.Request) {
	name := extractLastPathSegment(r.URL.Path, "/api/v1/dns-policies/")
	if name == "" {
		WriteError(w, http.StatusBadRequest, "missing dns policy name", "MISSING_NAME")
		return
	}

	userInfo := auth.GetUserFromContext(r.Context())
	if userInfo == nil || userInfo.Name == "" {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	allowed, err := h.rbac.CheckAccess(r.Context(), userInfo.Name, userInfo.Groups, "delete", "vpcdnspolicies", "")
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
		list, err := h.dynClient.Resource(vpcDNSPolicyGVR).Namespace("").List(r.Context(), metav1.ListOptions{})
		if err != nil {
			slog.ErrorContext(r.Context(), "failed to list VPCDNSPolicies for delete", "name", name, "error", err)
			WriteError(w, http.StatusInternalServerError, "failed to find dns policy", "LIST_FAILED")
			return
		}
		for _, item := range list.Items {
			if item.GetName() == name {
				ns = item.GetNamespace()
				break
			}
		}
		if ns == "" {
			WriteError(w, http.StatusNotFound, "dns policy not found", "NOT_FOUND")
			return
		}
	}

	if err := h.dynClient.Resource(vpcDNSPolicyGVR).Namespace(ns).Delete(r.Context(), name, metav1.DeleteOptions{}); err != nil {
		slog.ErrorContext(r.Context(), "failed to delete VPCDNSPolicy", "name", name, "namespace", ns, "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to delete dns policy", "DELETE_FAILED")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// unstructuredToDNSPolicy maps an unstructured VPCDNSPolicy to the response model.
func unstructuredToDNSPolicy(obj *unstructured.Unstructured) model.DNSPolicyResponse {
	resp := model.DNSPolicyResponse{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}

	// Spec fields
	resp.RouterRef, _, _ = unstructured.NestedString(obj.Object, "spec", "routerRef")

	// Upstream servers
	servers, found, _ := unstructured.NestedSlice(obj.Object, "spec", "upstream", "servers")
	if found {
		for _, s := range servers {
			sMap, ok := s.(map[string]interface{})
			if !ok {
				continue
			}
			if url, ok := sMap["url"].(string); ok {
				resp.UpstreamServers = append(resp.UpstreamServers, url)
			}
		}
	}

	// Filtering
	filterEnabled, found, _ := unstructured.NestedBool(obj.Object, "spec", "filtering", "enabled")
	if found {
		resp.FilteringEnabled = filterEnabled
	}

	// Local DNS
	localEnabled, found, _ := unstructured.NestedBool(obj.Object, "spec", "localDNS", "enabled")
	if found {
		resp.LocalDNSEnabled = localEnabled
	}
	resp.LocalDNSDomain, _, _ = unstructured.NestedString(obj.Object, "spec", "localDNS", "domain")

	// Status fields
	resp.Phase, _, _ = unstructured.NestedString(obj.Object, "status", "phase")
	resp.SyncStatus, _, _ = unstructured.NestedString(obj.Object, "status", "syncStatus")
	resp.ConfigMapName, _, _ = unstructured.NestedString(obj.Object, "status", "configMapName")
	resp.Message, _, _ = unstructured.NestedString(obj.Object, "status", "message")

	filterRules, found, _ := unstructured.NestedInt64(obj.Object, "status", "filterRulesLoaded")
	if found {
		resp.FilterRulesLoaded = filterRules
	}

	if ct := obj.GetCreationTimestamp(); !ct.IsZero() {
		resp.CreatedAt = ct.UTC().Format("2006-01-02T15:04:05Z")
	}

	return resp
}

// buildDNSPolicyUnstructured creates an unstructured VPCDNSPolicy from a request.
func buildDNSPolicyUnstructured(req model.DNSPolicyRequest) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "vpc.roks.ibm.com",
		Version: "v1alpha1",
		Kind:    "VPCDNSPolicy",
	})
	obj.SetName(req.Name)
	if req.Namespace != "" {
		obj.SetNamespace(req.Namespace)
	}

	spec := map[string]interface{}{
		"routerRef": req.RouterRef,
	}

	// Upstream servers
	if req.Upstream != nil && len(req.Upstream.Servers) > 0 {
		servers := make([]interface{}, 0, len(req.Upstream.Servers))
		for _, url := range req.Upstream.Servers {
			servers = append(servers, map[string]interface{}{
				"url": url,
			})
		}
		spec["upstream"] = map[string]interface{}{
			"servers": servers,
		}
	}

	// Filtering
	if req.Filtering != nil {
		filtering := map[string]interface{}{
			"enabled": req.Filtering.Enabled,
		}
		if len(req.Filtering.Blocklists) > 0 {
			bl := make([]interface{}, len(req.Filtering.Blocklists))
			for i, b := range req.Filtering.Blocklists {
				bl[i] = b
			}
			filtering["blocklists"] = bl
		}
		if len(req.Filtering.Allowlist) > 0 {
			al := make([]interface{}, len(req.Filtering.Allowlist))
			for i, a := range req.Filtering.Allowlist {
				al[i] = a
			}
			filtering["allowlist"] = al
		}
		if len(req.Filtering.Denylist) > 0 {
			dl := make([]interface{}, len(req.Filtering.Denylist))
			for i, d := range req.Filtering.Denylist {
				dl[i] = d
			}
			filtering["denylist"] = dl
		}
		spec["filtering"] = filtering
	}

	// Local DNS
	if req.LocalDNS != nil {
		localDNS := map[string]interface{}{
			"enabled": req.LocalDNS.Enabled,
		}
		if req.LocalDNS.Domain != "" {
			localDNS["domain"] = req.LocalDNS.Domain
		}
		spec["localDNS"] = localDNS
	}

	// Image override
	if req.Image != "" {
		spec["image"] = req.Image
	}

	obj.Object["spec"] = spec
	return obj
}
