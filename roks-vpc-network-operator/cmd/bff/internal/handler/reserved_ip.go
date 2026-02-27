package handler

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/IBM/roks-vpc-network-operator/cmd/bff/internal/model"
	"github.com/IBM/roks-vpc-network-operator/pkg/vpc"
)

// ReservedIPHandler handles VPC subnet reserved IP operations.
type ReservedIPHandler struct {
	vpcClient    vpc.ExtendedClient
	defaultVPCID string
}

// NewReservedIPHandler creates a new reserved IP handler.
func NewReservedIPHandler(vpcClient vpc.ExtendedClient, defaultVPCID string) *ReservedIPHandler {
	return &ReservedIPHandler{
		vpcClient:    vpcClient,
		defaultVPCID: defaultVPCID,
	}
}

// ListSubnetReservedIPs handles GET /api/v1/subnets/{id}/reserved-ips
func (h *ReservedIPHandler) ListSubnetReservedIPs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	// Extract subnet ID from path: /api/v1/subnets/{id}/reserved-ips
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/subnets/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) == 0 || parts[0] == "" {
		WriteError(w, http.StatusBadRequest, "missing subnet ID", "MISSING_ID")
		return
	}
	subnetID := parts[0]

	slog.DebugContext(r.Context(), "listing reserved IPs for subnet", "subnet_id", subnetID)

	ips, err := h.vpcClient.ListSubnetReservedIPs(r.Context(), subnetID)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to list reserved IPs", "subnet_id", subnetID, "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to list reserved IPs", "LIST_RESERVED_IPS_FAILED")
		return
	}

	responses := make([]model.ReservedIPResponse, 0, len(ips))
	for _, ip := range ips {
		resp := model.ReservedIPResponse{
			ID:         ip.ID,
			Name:       ip.Name,
			Address:    ip.Address,
			AutoDelete: ip.AutoDelete,
			Owner:      ip.Owner,
			CreatedAt:  ip.CreatedAt,
		}
		if ip.TargetID != "" {
			resp.Target = &model.RefResponse{
				ID:   ip.TargetID,
				Name: ip.Target,
			}
		}
		responses = append(responses, resp)
	}

	WriteJSON(w, http.StatusOK, responses)
}
