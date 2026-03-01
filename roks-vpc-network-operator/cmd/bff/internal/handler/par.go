package handler

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/IBM/roks-vpc-network-operator/cmd/bff/internal/auth"
	"github.com/IBM/roks-vpc-network-operator/cmd/bff/internal/model"
	"github.com/IBM/roks-vpc-network-operator/pkg/vpc"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
)

// PARHandler handles VPC public address range operations.
type PARHandler struct {
	vpcClient vpc.ExtendedClient
	rbac      *auth.RBACChecker
	dynClient dynamic.Interface
	vpcID     string
}

// NewPARHandler creates a new PAR handler.
func NewPARHandler(vpcClient vpc.ExtendedClient, rbac *auth.RBACChecker, dynClient dynamic.Interface, vpcID string) *PARHandler {
	return &PARHandler{
		vpcClient: vpcClient,
		rbac:      rbac,
		dynClient: dynClient,
		vpcID:     vpcID,
	}
}

// ListPARs handles GET /api/v1/pars
func (h *PARHandler) ListPARs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	slog.DebugContext(r.Context(), "listing PARs")

	pars, err := h.vpcClient.ListPublicAddressRanges(r.Context(), h.vpcID)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to list PARs", "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to list public address ranges", "LIST_PARS_FAILED")
		return
	}

	// Build gateway cross-reference map: PAR ID → (name, namespace)
	gwMap := h.buildGatewayPARMap(r)

	responses := make([]model.PARResponse, 0, len(pars))
	for _, p := range pars {
		resp := parToResponse(p)
		if gw, ok := gwMap[p.ID]; ok {
			resp.GatewayName = gw.name
			resp.GatewayNS = gw.namespace
		}
		responses = append(responses, resp)
	}

	WriteJSON(w, http.StatusOK, responses)
}

// GetPAR handles GET /api/v1/pars/{id}
func (h *PARHandler) GetPAR(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/v1/pars/")
	id = strings.Split(id, "/")[0]

	slog.DebugContext(r.Context(), "getting PAR", "id", id)

	par, err := h.vpcClient.GetPublicAddressRange(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to get PAR", "id", id, "error", err)
		WriteError(w, http.StatusNotFound, "public address range not found", "PAR_NOT_FOUND")
		return
	}

	resp := parToResponse(*par)

	// Cross-reference with gateways
	gwMap := h.buildGatewayPARMap(r)
	if gw, ok := gwMap[par.ID]; ok {
		resp.GatewayName = gw.name
		resp.GatewayNS = gw.namespace
	}

	WriteJSON(w, http.StatusOK, resp)
}

// CreatePAR handles POST /api/v1/pars
func (h *PARHandler) CreatePAR(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	userInfo := auth.GetUserFromContext(r.Context())
	if userInfo == nil || userInfo.Name == "" {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	allowed, err := h.rbac.CheckAccess(r.Context(), userInfo.Name, userInfo.Groups, "create", "vpcgateways", "default")
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "authorization check failed", "AUTHZ_CHECK_FAILED")
		return
	}
	if !allowed {
		WriteError(w, http.StatusForbidden, "forbidden", "FORBIDDEN")
		return
	}

	var req model.CreatePARRequest
	if err := ReadJSON(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body", "INVALID_REQUEST")
		return
	}

	if req.Zone == "" || req.PrefixLength == 0 {
		WriteError(w, http.StatusBadRequest, "zone and prefixLength are required", "INVALID_REQUEST")
		return
	}

	slog.DebugContext(r.Context(), "creating PAR", "name", req.Name, "zone", req.Zone, "prefixLength", req.PrefixLength)

	par, err := h.vpcClient.CreatePublicAddressRange(r.Context(), vpc.CreatePublicAddressRangeOptions{
		Name:         req.Name,
		VPCID:        h.vpcID,
		Zone:         req.Zone,
		PrefixLength: req.PrefixLength,
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to create PAR", "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to create public address range", "CREATE_PAR_FAILED")
		return
	}

	WriteJSON(w, http.StatusCreated, parToResponse(*par))
}

// DeletePAR handles DELETE /api/v1/pars/{id}
func (h *PARHandler) DeletePAR(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	userInfo := auth.GetUserFromContext(r.Context())
	if userInfo == nil || userInfo.Name == "" {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	allowed, err := h.rbac.CheckAccess(r.Context(), userInfo.Name, userInfo.Groups, "delete", "vpcgateways", "default")
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "authorization check failed", "AUTHZ_CHECK_FAILED")
		return
	}
	if !allowed {
		WriteError(w, http.StatusForbidden, "forbidden", "FORBIDDEN")
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/v1/pars/")
	id = strings.Split(id, "/")[0]

	slog.DebugContext(r.Context(), "deleting PAR", "id", id)

	if err := h.vpcClient.DeletePublicAddressRange(r.Context(), id); err != nil {
		slog.ErrorContext(r.Context(), "failed to delete PAR", "id", id, "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to delete public address range", "DELETE_PAR_FAILED")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

type gatewayRef struct {
	name      string
	namespace string
}

// buildGatewayPARMap lists all VPCGateways and returns a map from PAR ID → gateway ref.
func (h *PARHandler) buildGatewayPARMap(r *http.Request) map[string]gatewayRef {
	gwMap := make(map[string]gatewayRef)
	if h.dynClient == nil {
		return gwMap
	}

	list, err := h.dynClient.Resource(vpcGatewayGVR).Namespace("").List(r.Context(), metav1.ListOptions{})
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to list VPCGateways for PAR cross-reference", "error", err)
		return gwMap
	}

	for _, item := range list.Items {
		parID, found, _ := unstructured.NestedString(item.Object, "status", "publicAddressRangeID")
		if found && parID != "" {
			gwMap[parID] = gatewayRef{
				name:      item.GetName(),
				namespace: item.GetNamespace(),
			}
		}
	}

	return gwMap
}

func parToResponse(p vpc.PublicAddressRange) model.PARResponse {
	return model.PARResponse{
		ID:             p.ID,
		Name:           p.Name,
		CIDR:           p.CIDR,
		Zone:           p.Zone,
		LifecycleState: p.LifecycleState,
		CreatedAt:      p.CreatedAt,
	}
}
