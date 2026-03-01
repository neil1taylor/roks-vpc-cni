package handler

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/IBM/roks-vpc-network-operator/cmd/bff/internal/auth"
	"github.com/IBM/roks-vpc-network-operator/cmd/bff/internal/model"
	"github.com/IBM/roks-vpc-network-operator/pkg/vpc"
)

// FloatingIPHandler handles floating IP operations.
type FloatingIPHandler struct {
	vpcClient    vpc.ExtendedClient
	rbac         *auth.RBACChecker
	defaultVPCID string
}

// NewFloatingIPHandler creates a new floating IP handler.
func NewFloatingIPHandler(vpcClient vpc.ExtendedClient, rbac *auth.RBACChecker, defaultVPCID string) *FloatingIPHandler {
	return &FloatingIPHandler{
		vpcClient:    vpcClient,
		rbac:         rbac,
		defaultVPCID: defaultVPCID,
	}
}

// ListFloatingIPs handles GET /api/v1/floating-ips
func (h *FloatingIPHandler) ListFloatingIPs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	slog.DebugContext(r.Context(), "listing floating IPs")

	fips, err := h.vpcClient.ListFloatingIPs(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to list floating IPs", "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to list floating IPs", "LIST_FLOATING_IPS_FAILED")
		return
	}

	// Filter FIPs to those targeting VNIs in the cluster's VPC (+ unbound FIPs)
	fips = h.filterFIPsByVPC(r.Context(), fips)

	responses := make([]model.FloatingIPResponse, 0, len(fips))
	for _, f := range fips {
		responses = append(responses, fipToResponse(f))
	}

	WriteJSON(w, http.StatusOK, responses)
}

// GetFloatingIP handles GET /api/v1/floating-ips/{id}
func (h *FloatingIPHandler) GetFloatingIP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/v1/floating-ips/")
	id = strings.Split(id, "/")[0]

	slog.DebugContext(r.Context(), "getting floating IP", "id", id)

	fip, err := h.vpcClient.GetFloatingIP(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to get floating IP", "id", id, "error", err)
		WriteError(w, http.StatusNotFound, "floating IP not found", "FLOATING_IP_NOT_FOUND")
		return
	}

	WriteJSON(w, http.StatusOK, fipToResponse(*fip))
}

// CreateFloatingIP handles POST /api/v1/floating-ips
func (h *FloatingIPHandler) CreateFloatingIP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	userInfo := auth.GetUserFromContext(r.Context())
	if userInfo == nil || userInfo.Name == "" {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	allowed, err := h.rbac.CheckAccess(r.Context(), userInfo.Name, userInfo.Groups, "create", "floatingips", "default")
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "authorization check failed", "AUTHZ_CHECK_FAILED")
		return
	}
	if !allowed {
		WriteError(w, http.StatusForbidden, "forbidden", "FORBIDDEN")
		return
	}

	var req model.FloatingIPRequest
	if err := ReadJSON(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body", "INVALID_REQUEST")
		return
	}

	slog.DebugContext(r.Context(), "creating floating IP", "name", req.Name, "zone", req.Zone)

	fip, err := h.vpcClient.CreateFloatingIP(r.Context(), vpc.CreateFloatingIPOptions{
		Name: req.Name,
		Zone: req.Zone,
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to create floating IP", "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to create floating IP", "CREATE_FLOATING_IP_FAILED")
		return
	}

	WriteJSON(w, http.StatusCreated, fipToResponse(*fip))
}

// UpdateFloatingIP handles PATCH /api/v1/floating-ips/{id}
func (h *FloatingIPHandler) UpdateFloatingIP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	userInfo := auth.GetUserFromContext(r.Context())
	if userInfo == nil || userInfo.Name == "" {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	allowed, err := h.rbac.CheckAccess(r.Context(), userInfo.Name, userInfo.Groups, "update", "floatingips", "default")
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "authorization check failed", "AUTHZ_CHECK_FAILED")
		return
	}
	if !allowed {
		WriteError(w, http.StatusForbidden, "forbidden", "FORBIDDEN")
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/v1/floating-ips/")
	id = strings.Split(id, "/")[0]

	var req model.FloatingIPUpdateRequest
	if err := ReadJSON(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body", "INVALID_REQUEST")
		return
	}

	slog.DebugContext(r.Context(), "updating floating IP", "id", id, "target_id", req.TargetID)

	fip, err := h.vpcClient.UpdateFloatingIP(r.Context(), id, vpc.UpdateFloatingIPOptions{
		TargetID: req.TargetID,
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to update floating IP", "id", id, "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to update floating IP", "UPDATE_FLOATING_IP_FAILED")
		return
	}

	WriteJSON(w, http.StatusOK, fipToResponse(*fip))
}

// filterFIPsByVPC keeps only floating IPs that target a VNI on a subnet in the
// cluster's VPC. Unbound FIPs (no target) are always included since they are
// account-level resources the user may want to bind.
// When no defaultVPCID is set, all FIPs are returned unfiltered.
func (h *FloatingIPHandler) filterFIPsByVPC(ctx context.Context, fips []vpc.FloatingIP) []vpc.FloatingIP {
	if h.defaultVPCID == "" {
		return fips
	}

	// Build set of subnet IDs in this VPC
	subnets, err := h.vpcClient.ListSubnets(ctx, h.defaultVPCID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to list subnets for FIP filtering, returning all FIPs", "error", err)
		return fips
	}
	subnetSet := make(map[string]bool, len(subnets))
	for _, s := range subnets {
		subnetSet[s.ID] = true
	}

	// Build set of VNI IDs on those subnets
	allVNIs, err := h.vpcClient.ListVNIs(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to list VNIs for FIP filtering, returning all FIPs", "error", err)
		return fips
	}
	vniSet := make(map[string]bool, len(allVNIs))
	for _, v := range allVNIs {
		if subnetSet[v.SubnetID] {
			vniSet[v.ID] = true
		}
	}

	// Keep FIPs that are unbound or target a VNI in this VPC
	filtered := make([]vpc.FloatingIP, 0, len(fips))
	for _, f := range fips {
		if f.Target == "" || vniSet[f.Target] {
			filtered = append(filtered, f)
		}
	}
	return filtered
}

func fipToResponse(f vpc.FloatingIP) model.FloatingIPResponse {
	resp := model.FloatingIPResponse{
		ID:        f.ID,
		Name:      f.Name,
		Address:   f.Address,
		Status:    f.Status,
		Zone:      model.RefResponse{ID: f.Zone, Name: f.Zone},
		CreatedAt: f.CreatedAt,
	}
	if f.Target != "" {
		resp.Target = &model.RefResponse{ID: f.Target, Name: f.TargetName}
	}
	return resp
}
