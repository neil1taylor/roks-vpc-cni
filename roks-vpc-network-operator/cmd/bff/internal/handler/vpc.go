package handler

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/IBM/roks-vpc-network-operator/cmd/bff/internal/model"
	"github.com/IBM/roks-vpc-network-operator/pkg/vpc"
)

// VPCHandler handles VPC operations
type VPCHandler struct {
	vpcClient    vpc.ExtendedClient
	defaultVPCID string
}

// NewVPCHandler creates a new VPC handler. When defaultVPCID is set, ListVPCs
// returns only that VPC instead of listing all VPCs in the account.
func NewVPCHandler(vpcClient vpc.ExtendedClient, defaultVPCID string) *VPCHandler {
	return &VPCHandler{
		vpcClient:    vpcClient,
		defaultVPCID: defaultVPCID,
	}
}

// ListVPCs handles GET /api/v1/vpcs
func (h *VPCHandler) ListVPCs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	slog.DebugContext(r.Context(), "listing VPCs", "defaultVPCID", h.defaultVPCID)

	// When scoped to a cluster VPC, fetch only that VPC instead of listing all.
	// If GetVPC fails (e.g. wrong ID), gracefully fall through to ListVPCs.
	if h.defaultVPCID != "" {
		vpcObj, err := h.vpcClient.GetVPC(r.Context(), h.defaultVPCID)
		if err != nil {
			slog.WarnContext(r.Context(), "GetVPC failed for configured VPC ID, falling back to ListVPCs",
				"vpcID", h.defaultVPCID, "error", err)
			// fall through to ListVPCs below
		} else {
			WriteJSON(w, http.StatusOK, []model.VPCResponse{{
				ID:        vpcObj.ID,
				Name:      vpcObj.Name,
				Region:    vpcObj.Region,
				CreatedAt: vpcObj.CreatedAt,
				Status:    vpcObj.Status,
			}})
			return
		}
	}

	vpcs, err := h.vpcClient.ListVPCs(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to list VPCs", "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to list VPCs", "LIST_VPCS_FAILED")
		return
	}

	responses := make([]model.VPCResponse, 0, len(vpcs))
	for _, v := range vpcs {
		responses = append(responses, model.VPCResponse{
			ID:        v.ID,
			Name:      v.Name,
			Region:    v.Region,
			CreatedAt: v.CreatedAt,
			Status:    v.Status,
		})
	}

	WriteJSON(w, http.StatusOK, responses)
}

// GetVPC handles GET /api/v1/vpcs/{id}
func (h *VPCHandler) GetVPC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/v1/vpcs/")
	id = strings.Split(id, "/")[0]

	slog.DebugContext(r.Context(), "getting VPC", "id", id)

	vpcObj, err := h.vpcClient.GetVPC(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to get VPC", "id", id, "error", err)
		WriteError(w, http.StatusNotFound, "VPC not found", "VPC_NOT_FOUND")
		return
	}

	resp := model.VPCResponse{
		ID:        vpcObj.ID,
		Name:      vpcObj.Name,
		Region:    vpcObj.Region,
		CreatedAt: vpcObj.CreatedAt,
		Status:    vpcObj.Status,
	}

	WriteJSON(w, http.StatusOK, resp)
}
