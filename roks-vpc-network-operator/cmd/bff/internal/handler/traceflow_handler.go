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

var vpcTraceflowGVR = schema.GroupVersionResource{
	Group:    "vpc.roks.ibm.com",
	Version:  "v1alpha1",
	Resource: "vpctraceflows",
}

// TraceflowHandler handles VPCTraceflow API operations.
type TraceflowHandler struct {
	dynClient dynamic.Interface
	rbac      *auth.RBACChecker
}

// NewTraceflowHandler creates a new traceflow handler.
func NewTraceflowHandler(dynClient dynamic.Interface, rbac *auth.RBACChecker) *TraceflowHandler {
	return &TraceflowHandler{
		dynClient: dynClient,
		rbac:      rbac,
	}
}

// ListTraceflows handles GET /api/v1/traceflows
func (h *TraceflowHandler) ListTraceflows(w http.ResponseWriter, r *http.Request) {
	if h.dynClient == nil {
		WriteError(w, http.StatusServiceUnavailable, "dynamic client not configured", "CLIENT_NOT_CONFIGURED")
		return
	}

	list, err := h.dynClient.Resource(vpcTraceflowGVR).Namespace("").List(r.Context(), metav1.ListOptions{})
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to list VPCTraceflows", "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to list traceflows", "LIST_FAILED")
		return
	}

	traceflows := make([]model.TraceflowResponse, 0, len(list.Items))
	for _, item := range list.Items {
		traceflows = append(traceflows, unstructuredToTraceflow(&item))
	}

	WriteJSON(w, http.StatusOK, traceflows)
}

// GetTraceflow handles GET /api/v1/traceflows/:name
func (h *TraceflowHandler) GetTraceflow(w http.ResponseWriter, r *http.Request) {
	name := extractLastPathSegment(r.URL.Path, "/api/v1/traceflows/")
	if name == "" {
		WriteError(w, http.StatusBadRequest, "missing traceflow name", "MISSING_NAME")
		return
	}

	if h.dynClient == nil {
		WriteError(w, http.StatusServiceUnavailable, "dynamic client not configured", "CLIENT_NOT_CONFIGURED")
		return
	}

	ns := r.URL.Query().Get("namespace")
	if ns != "" {
		item, err := h.dynClient.Resource(vpcTraceflowGVR).Namespace(ns).Get(r.Context(), name, metav1.GetOptions{})
		if err != nil {
			slog.ErrorContext(r.Context(), "failed to get VPCTraceflow", "name", name, "namespace", ns, "error", err)
			WriteError(w, http.StatusNotFound, "traceflow not found", "NOT_FOUND")
			return
		}
		WriteJSON(w, http.StatusOK, unstructuredToTraceflow(item))
		return
	}

	// No namespace — cross-namespace List + filter by name
	list, err := h.dynClient.Resource(vpcTraceflowGVR).Namespace("").List(r.Context(), metav1.ListOptions{})
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to list VPCTraceflows for get", "name", name, "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to list traceflows", "LIST_FAILED")
		return
	}
	for _, item := range list.Items {
		if item.GetName() == name {
			WriteJSON(w, http.StatusOK, unstructuredToTraceflow(&item))
			return
		}
	}
	WriteError(w, http.StatusNotFound, "traceflow not found", "NOT_FOUND")
}

// CreateTraceflow handles POST /api/v1/traceflows
func (h *TraceflowHandler) CreateTraceflow(w http.ResponseWriter, r *http.Request) {
	userInfo := auth.GetUserFromContext(r.Context())
	if userInfo == nil || userInfo.Name == "" {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	ns := "default"
	// Peek at namespace from body for RBAC check — we'll decode fully below
	var req model.TraceflowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body", "INVALID_REQUEST")
		return
	}
	if req.Namespace != "" {
		ns = req.Namespace
	}

	allowed, err := h.rbac.CheckAccess(r.Context(), userInfo.Name, userInfo.Groups, "create", "vpctraceflows", ns)
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

	obj := buildTraceflowUnstructured(req)
	created, err := h.dynClient.Resource(vpcTraceflowGVR).Namespace(ns).Create(r.Context(), obj, metav1.CreateOptions{})
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to create VPCTraceflow", "error", err)
		WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create traceflow: %v", err), "CREATE_FAILED")
		return
	}

	tf := unstructuredToTraceflow(created)
	WriteJSON(w, http.StatusCreated, tf)
}

// DeleteTraceflow handles DELETE /api/v1/traceflows/:name
func (h *TraceflowHandler) DeleteTraceflow(w http.ResponseWriter, r *http.Request) {
	name := extractLastPathSegment(r.URL.Path, "/api/v1/traceflows/")
	if name == "" {
		WriteError(w, http.StatusBadRequest, "missing traceflow name", "MISSING_NAME")
		return
	}

	userInfo := auth.GetUserFromContext(r.Context())
	if userInfo == nil || userInfo.Name == "" {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	allowed, err := h.rbac.CheckAccess(r.Context(), userInfo.Name, userInfo.Groups, "delete", "vpctraceflows", "")
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
		list, err := h.dynClient.Resource(vpcTraceflowGVR).Namespace("").List(r.Context(), metav1.ListOptions{})
		if err != nil {
			slog.ErrorContext(r.Context(), "failed to list VPCTraceflows for delete", "name", name, "error", err)
			WriteError(w, http.StatusInternalServerError, "failed to find traceflow", "LIST_FAILED")
			return
		}
		for _, item := range list.Items {
			if item.GetName() == name {
				ns = item.GetNamespace()
				break
			}
		}
		if ns == "" {
			WriteError(w, http.StatusNotFound, "traceflow not found", "NOT_FOUND")
			return
		}
	}

	if err := h.dynClient.Resource(vpcTraceflowGVR).Namespace(ns).Delete(r.Context(), name, metav1.DeleteOptions{}); err != nil {
		slog.ErrorContext(r.Context(), "failed to delete VPCTraceflow", "name", name, "namespace", ns, "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to delete traceflow", "DELETE_FAILED")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// unstructuredToTraceflow maps an unstructured VPCTraceflow to the response model.
func unstructuredToTraceflow(obj *unstructured.Unstructured) model.TraceflowResponse {
	resp := model.TraceflowResponse{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}

	// Spec fields
	resp.RouterRef, _, _ = unstructured.NestedString(obj.Object, "spec", "routerRef")

	// Source
	sourceIP, _, _ := unstructured.NestedString(obj.Object, "spec", "source", "ip")
	resp.Source.IP = sourceIP

	vmRefName, found, _ := unstructured.NestedString(obj.Object, "spec", "source", "vmRef", "name")
	if found {
		resp.Source.VMRef = vmRefName
	}

	// Destination
	resp.Destination.IP, _, _ = unstructured.NestedString(obj.Object, "spec", "destination", "ip")
	resp.Destination.Protocol, _, _ = unstructured.NestedString(obj.Object, "spec", "destination", "protocol")

	destPort, found, _ := unstructured.NestedInt64(obj.Object, "spec", "destination", "port")
	if found {
		p := int32(destPort)
		resp.Destination.Port = &p
	}

	// Status fields
	resp.Phase, _, _ = unstructured.NestedString(obj.Object, "status", "phase")
	resp.Result, _, _ = unstructured.NestedString(obj.Object, "status", "result")
	resp.TotalLatency, _, _ = unstructured.NestedString(obj.Object, "status", "totalLatency")
	resp.Message, _, _ = unstructured.NestedString(obj.Object, "status", "message")

	// Hops
	hops, found, _ := unstructured.NestedSlice(obj.Object, "status", "hops")
	if found {
		for _, hopRaw := range hops {
			hopMap, ok := hopRaw.(map[string]interface{})
			if !ok {
				continue
			}
			hop := model.TraceflowHopResponse{}
			if order, ok := hopMap["order"].(int64); ok {
				hop.Order = int(order)
			}
			if node, ok := hopMap["node"].(string); ok {
				hop.Node = node
			}
			if component, ok := hopMap["component"].(string); ok {
				hop.Component = component
			}
			if action, ok := hopMap["action"].(string); ok {
				hop.Action = action
			}
			if latency, ok := hopMap["latency"].(string); ok {
				hop.Latency = latency
			}
			// NFTables hits
			if nftHits, ok := hopMap["nftablesHits"].([]interface{}); ok {
				for _, hitRaw := range nftHits {
					hitMap, ok := hitRaw.(map[string]interface{})
					if !ok {
						continue
					}
					hit := model.TraceflowNFTablesHitResp{}
					if rule, ok := hitMap["rule"].(string); ok {
						hit.Rule = rule
					}
					if chain, ok := hitMap["chain"].(string); ok {
						hit.Chain = chain
					}
					if packets, ok := hitMap["packets"].(int64); ok {
						hit.Packets = packets
					}
					hop.NftablesHits = append(hop.NftablesHits, hit)
				}
			}
			resp.Hops = append(resp.Hops, hop)
		}
	}

	if ct := obj.GetCreationTimestamp(); !ct.IsZero() {
		resp.CreatedAt = ct.UTC().Format("2006-01-02T15:04:05Z")
	}

	return resp
}

// buildTraceflowUnstructured creates an unstructured VPCTraceflow from a request.
func buildTraceflowUnstructured(req model.TraceflowRequest) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "vpc.roks.ibm.com",
		Version: "v1alpha1",
		Kind:    "VPCTraceflow",
	})
	obj.SetName(req.Name)
	if req.Namespace != "" {
		obj.SetNamespace(req.Namespace)
	}

	// Source
	source := map[string]interface{}{}
	if req.Source.IP != "" {
		source["ip"] = req.Source.IP
	}
	if req.Source.VMRef != "" {
		source["vmRef"] = map[string]interface{}{
			"name": req.Source.VMRef,
		}
	}

	// Destination
	dest := map[string]interface{}{
		"ip": req.Destination.IP,
	}
	if req.Destination.Port != nil {
		dest["port"] = int64(*req.Destination.Port)
	}
	if req.Destination.Protocol != "" {
		dest["protocol"] = req.Destination.Protocol
	}

	spec := map[string]interface{}{
		"source":      source,
		"destination": dest,
		"routerRef":   req.RouterRef,
	}

	if req.Timeout != "" {
		spec["timeout"] = req.Timeout
	}
	if req.TTL != "" {
		spec["ttl"] = req.TTL
	}

	obj.Object["spec"] = spec
	return obj
}
