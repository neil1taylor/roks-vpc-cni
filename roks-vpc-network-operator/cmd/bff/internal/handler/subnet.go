package handler

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/IBM/roks-vpc-network-operator/cmd/bff/internal/auth"
	"github.com/IBM/roks-vpc-network-operator/cmd/bff/internal/model"
	"github.com/IBM/roks-vpc-network-operator/pkg/vpc"
)

// SubnetHandler handles VPC subnet operations.
type SubnetHandler struct {
	vpcClient    vpc.ExtendedClient
	rbac         *auth.RBACChecker
	defaultVPCID string
}

// NewSubnetHandler creates a new subnet handler.
func NewSubnetHandler(vpcClient vpc.ExtendedClient, rbac *auth.RBACChecker, defaultVPCID string) *SubnetHandler {
	return &SubnetHandler{
		vpcClient:    vpcClient,
		rbac:         rbac,
		defaultVPCID: defaultVPCID,
	}
}

// ListSubnets handles GET /api/v1/subnets
func (h *SubnetHandler) ListSubnets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	vpcID := GetQueryParam(r, "vpc_id")
	if vpcID == "" {
		vpcID = h.defaultVPCID
	}

	slog.DebugContext(r.Context(), "listing subnets", "vpc_id", vpcID)

	subnets, err := h.vpcClient.ListSubnets(r.Context(), vpcID)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to list subnets", "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to list subnets", "LIST_SUBNETS_FAILED")
		return
	}

	responses := make([]model.SubnetResponse, 0, len(subnets))
	for _, s := range subnets {
		responses = append(responses, subnetToResponse(s))
	}

	WriteJSON(w, http.StatusOK, responses)
}

// GetSubnet handles GET /api/v1/subnets/{id}
func (h *SubnetHandler) GetSubnet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/v1/subnets/")
	id = strings.Split(id, "/")[0]

	slog.DebugContext(r.Context(), "getting subnet", "id", id)

	subnet, err := h.vpcClient.GetSubnet(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to get subnet", "id", id, "error", err)
		WriteError(w, http.StatusNotFound, "subnet not found", "SUBNET_NOT_FOUND")
		return
	}

	WriteJSON(w, http.StatusOK, subnetToResponse(*subnet))
}

// CreateSubnet handles POST /api/v1/subnets
func (h *SubnetHandler) CreateSubnet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	userInfo := auth.GetUserFromContext(r.Context())
	if userInfo == nil || userInfo.Name == "" {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	allowed, err := h.rbac.CheckAccess(r.Context(), userInfo.Name, userInfo.Groups, "create", "subnets", "default")
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "authorization check failed", "AUTHZ_CHECK_FAILED")
		return
	}
	if !allowed {
		WriteError(w, http.StatusForbidden, "forbidden", "FORBIDDEN")
		return
	}

	var req model.SubnetRequest
	if err := ReadJSON(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body", "INVALID_REQUEST")
		return
	}

	slog.DebugContext(r.Context(), "creating subnet", "name", req.Name, "vpc_id", req.VPC.ID, "zone", req.Zone.Name)

	subnet, err := h.vpcClient.CreateSubnet(r.Context(), vpc.CreateSubnetOptions{
		Name:  req.Name,
		VPCID: req.VPC.ID,
		Zone:  req.Zone.Name,
		CIDR:  req.IPV4CIDRBlock,
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to create subnet", "error", err)
		WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create subnet: %v", err), "CREATE_SUBNET_FAILED")
		return
	}

	WriteJSON(w, http.StatusCreated, subnetToResponse(*subnet))
}

func subnetToResponse(s vpc.Subnet) model.SubnetResponse {
	resp := model.SubnetResponse{
		ID:                        s.ID,
		Name:                      s.Name,
		IPV4CIDRBlock:             s.CIDR,
		Status:                    s.Status,
		AvailableIPv4AddressCount: s.AvailableIPv4AddressCount,
		TotalIPv4AddressCount:     s.TotalIPv4AddressCount,
		VPC:                       model.RefResponse{ID: s.VPCID, Name: s.VPCName},
		Zone:                      model.RefResponse{ID: s.Zone, Name: s.Zone},
		CreatedAt:                 s.CreatedAt,
	}
	if s.NetworkACLID != "" {
		resp.NetworkACL = &model.RefResponse{ID: s.NetworkACLID, Name: s.NetworkACLName}
	}
	return resp
}
