package handler

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/IBM/roks-vpc-network-operator/cmd/bff/internal/model"
	"github.com/IBM/roks-vpc-network-operator/pkg/vpc"
)

// VNIHandler handles Virtual Network Interface operations.
type VNIHandler struct {
	vpcClient    vpc.ExtendedClient
	defaultVPCID string
}

// NewVNIHandler creates a new VNI handler.
func NewVNIHandler(vpcClient vpc.ExtendedClient, defaultVPCID string) *VNIHandler {
	return &VNIHandler{
		vpcClient:    vpcClient,
		defaultVPCID: defaultVPCID,
	}
}

// ListVNIs handles GET /api/v1/vnis
func (h *VNIHandler) ListVNIs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	slog.DebugContext(r.Context(), "listing VNIs")

	vnis, err := h.vpcClient.ListVNIs(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to list VNIs", "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to list VNIs", "LIST_VNIS_FAILED")
		return
	}

	// Filter VNIs to only those on subnets in the cluster's VPC
	vnis = h.filterVNIsByVPC(r.Context(), vnis)

	responses := make([]model.VNIResponse, 0, len(vnis))
	for _, v := range vnis {
		responses = append(responses, vniToResponse(v))
	}

	WriteJSON(w, http.StatusOK, responses)
}

// GetVNI handles GET /api/v1/vnis/{id}
func (h *VNIHandler) GetVNI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/v1/vnis/")
	id = strings.Split(id, "/")[0]

	slog.DebugContext(r.Context(), "getting VNI", "id", id)

	vni, err := h.vpcClient.GetVNI(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to get VNI", "id", id, "error", err)
		WriteError(w, http.StatusNotFound, "VNI not found", "VNI_NOT_FOUND")
		return
	}

	WriteJSON(w, http.StatusOK, vniToResponse(*vni))
}

// filterVNIsByVPC keeps only VNIs whose subnet belongs to the cluster's VPC.
// When no defaultVPCID is set, all VNIs are returned unfiltered.
func (h *VNIHandler) filterVNIsByVPC(ctx context.Context, vnis []vpc.VNI) []vpc.VNI {
	if h.defaultVPCID == "" {
		return vnis
	}

	// Build the set of subnet IDs in this VPC
	subnets, err := h.vpcClient.ListSubnets(ctx, h.defaultVPCID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to list subnets for VNI filtering, returning all VNIs", "error", err)
		return vnis
	}
	subnetSet := make(map[string]bool, len(subnets))
	for _, s := range subnets {
		subnetSet[s.ID] = true
	}

	filtered := make([]vpc.VNI, 0, len(vnis))
	for _, v := range vnis {
		if subnetSet[v.SubnetID] {
			filtered = append(filtered, v)
		}
	}
	return filtered
}

func vniToResponse(v vpc.VNI) model.VNIResponse {
	resp := model.VNIResponse{
		ID:                      v.ID,
		Name:                    v.Name,
		AllowIPSpoofing:         v.AllowIPSpoofing,
		EnableInfrastructureNat: v.EnableInfrastructureNat,
		Status:                  v.Status,
		CreatedAt:               v.CreatedAt,
	}
	if v.SubnetID != "" {
		resp.Subnet = &model.RefResponse{ID: v.SubnetID, Name: v.SubnetName}
	}
	if v.PrimaryIP.Address != "" {
		resp.PrimaryIP = &model.IPResponse{Address: v.PrimaryIP.Address}
	}
	return resp
}
