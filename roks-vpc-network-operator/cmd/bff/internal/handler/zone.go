package handler

import (
	"log/slog"
	"net/http"

	"github.com/IBM/roks-vpc-network-operator/cmd/bff/internal/model"
	"github.com/IBM/roks-vpc-network-operator/pkg/vpc"
)

// ZoneHandler handles zone operations
type ZoneHandler struct {
	vpcClient vpc.ExtendedClient
}

// NewZoneHandler creates a new zone handler
func NewZoneHandler(vpcClient vpc.ExtendedClient) *ZoneHandler {
	return &ZoneHandler{
		vpcClient: vpcClient,
	}
}

// ListZones handles GET /api/v1/zones
func (h *ZoneHandler) ListZones(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	region := GetQueryParam(r, "region")
	slog.DebugContext(r.Context(), "listing zones", "region", region)

	zones, err := h.vpcClient.ListZones(r.Context(), region)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to list zones", "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to list zones", "LIST_ZONES_FAILED")
		return
	}

	responses := make([]model.ZoneResponse, 0, len(zones))
	for _, zone := range zones {
		responses = append(responses, model.ZoneResponse{
			Name:   zone.Name,
			Region: zone.Region,
			Status: zone.Status,
		})
	}

	WriteJSON(w, http.StatusOK, responses)
}
