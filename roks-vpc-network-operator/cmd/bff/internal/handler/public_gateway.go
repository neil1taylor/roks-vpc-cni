package handler

import (
	"log/slog"
	"net/http"

	"github.com/IBM/roks-vpc-network-operator/cmd/bff/internal/model"
	"github.com/IBM/roks-vpc-network-operator/pkg/vpc"
)

// PublicGatewayHandler handles public gateway operations.
type PublicGatewayHandler struct {
	vpcClient    vpc.ExtendedClient
	defaultVPCID string
}

// NewPublicGatewayHandler creates a new public gateway handler.
func NewPublicGatewayHandler(vpcClient vpc.ExtendedClient, defaultVPCID string) *PublicGatewayHandler {
	return &PublicGatewayHandler{
		vpcClient:    vpcClient,
		defaultVPCID: defaultVPCID,
	}
}

// ListPublicGateways handles GET /api/v1/public-gateways
func (h *PublicGatewayHandler) ListPublicGateways(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	vpcID := r.URL.Query().Get("vpcId")
	if vpcID == "" {
		vpcID = h.defaultVPCID
	}

	slog.DebugContext(r.Context(), "listing public gateways", "vpcId", vpcID)

	pgws, err := h.vpcClient.ListPublicGateways(r.Context(), vpcID)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to list public gateways", "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to list public gateways", "LIST_PUBLIC_GATEWAYS_FAILED")
		return
	}

	responses := make([]model.PublicGatewayResponse, 0, len(pgws))
	for _, p := range pgws {
		resp := model.PublicGatewayResponse{
			ID:        p.ID,
			Name:      p.Name,
			Status:    p.Status,
			Zone:      model.RefResponse{ID: p.Zone, Name: p.Zone},
			CreatedAt: p.CreatedAt,
		}
		if p.FloatingIPAddress != "" {
			resp.FloatingIP = &model.IPResponse{Address: p.FloatingIPAddress}
		}
		responses = append(responses, resp)
	}

	WriteJSON(w, http.StatusOK, responses)
}
