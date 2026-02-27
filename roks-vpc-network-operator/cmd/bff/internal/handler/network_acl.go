package handler

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/IBM/roks-vpc-network-operator/cmd/bff/internal/auth"
	"github.com/IBM/roks-vpc-network-operator/cmd/bff/internal/model"
	"github.com/IBM/roks-vpc-network-operator/pkg/vpc"
)

// NetworkACLHandler handles network ACL operations
type NetworkACLHandler struct {
	vpcClient    vpc.ExtendedClient
	rbac         *auth.RBACChecker
	defaultVPCID string
}

// NewNetworkACLHandler creates a new network ACL handler
func NewNetworkACLHandler(vpcClient vpc.ExtendedClient, rbac *auth.RBACChecker, defaultVPCID string) *NetworkACLHandler {
	return &NetworkACLHandler{
		vpcClient:    vpcClient,
		rbac:         rbac,
		defaultVPCID: defaultVPCID,
	}
}

// ListNetworkACLs handles GET /api/v1/network-acls
func (h *NetworkACLHandler) ListNetworkACLs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	vpcID := GetQueryParam(r, "vpc_id")
	if vpcID == "" {
		vpcID = h.defaultVPCID
	}
	slog.DebugContext(r.Context(), "listing network ACLs", "vpc_id", vpcID)

	acls, err := h.vpcClient.ListNetworkACLs(r.Context(), vpcID)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to list network ACLs", "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to list network ACLs", "LIST_ACLS_FAILED")
		return
	}

	responses := make([]model.NetworkACLResponse, 0, len(acls))
	for _, acl := range acls {
		subnets := make([]model.RefResponse, 0, len(acl.Subnets))
		for _, sid := range acl.Subnets {
			subnets = append(subnets, model.RefResponse{ID: sid})
		}
		responses = append(responses, model.NetworkACLResponse{
			ID:        acl.ID,
			Name:      acl.Name,
			VPC:       model.RefResponse{ID: acl.VPCID},
			Status:    "available",
			Subnets:   subnets,
			CreatedAt: acl.CreatedAt,
		})
	}

	WriteJSON(w, http.StatusOK, responses)
}

// CreateNetworkACL handles POST /api/v1/network-acls
func (h *NetworkACLHandler) CreateNetworkACL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	// Check authorization
	userInfo := auth.GetUserFromContext(r.Context())
	if userInfo == nil || userInfo.Name == "" {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	allowed, err := h.rbac.CheckAccess(r.Context(), userInfo.Name, userInfo.Groups, "create", "networkacls", "default")
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "authorization check failed", "AUTHZ_CHECK_FAILED")
		return
	}
	if !allowed {
		WriteError(w, http.StatusForbidden, "forbidden", "FORBIDDEN")
		return
	}

	var req model.NetworkACLRequest
	if err := ReadJSON(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body", "INVALID_REQUEST")
		return
	}

	slog.DebugContext(r.Context(), "creating network ACL", "name", req.Name, "vpc_id", req.VPCID)

	acl, err := h.vpcClient.CreateNetworkACL(r.Context(), vpc.CreateNetworkACLOptions{
		Name:  req.Name,
		VPCID: req.VPCID,
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to create network ACL", "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to create network ACL", "CREATE_ACL_FAILED")
		return
	}

	resp := model.NetworkACLResponse{
		ID:        acl.ID,
		Name:      acl.Name,
		VPC:       model.RefResponse{ID: acl.VPCID},
		Status:    "available",
		CreatedAt: acl.CreatedAt,
	}

	WriteJSON(w, http.StatusCreated, resp)
}

// GetNetworkACL handles GET /api/v1/network-acls/{id}
func (h *NetworkACLHandler) GetNetworkACL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/v1/network-acls/")
	id = strings.Split(id, "/")[0]

	slog.DebugContext(r.Context(), "getting network ACL", "id", id)

	acl, err := h.vpcClient.GetNetworkACL(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to get network ACL", "id", id, "error", err)
		WriteError(w, http.StatusNotFound, "network ACL not found", "ACL_NOT_FOUND")
		return
	}

	// Convert rules from the ACL (GetNetworkACL returns rules inline)
	ruleResponses := make([]model.ACLRuleResponse, 0, len(acl.Rules))
	for _, rule := range acl.Rules {
		ruleResponses = append(ruleResponses, model.ACLRuleResponse{
			ID:          rule.ID,
			Name:        rule.Name,
			Action:      rule.Action,
			Direction:   rule.Direction,
			Protocol:    rule.Protocol,
			Source:      rule.Source,
			Destination: rule.Destination,
			PortMin:     rule.PortMin,
			PortMax:     rule.PortMax,
		})
	}

	subnets := make([]model.RefResponse, 0, len(acl.Subnets))
	for _, sid := range acl.Subnets {
		subnets = append(subnets, model.RefResponse{ID: sid})
	}

	resp := model.NetworkACLResponse{
		ID:        acl.ID,
		Name:      acl.Name,
		VPC:       model.RefResponse{ID: acl.VPCID},
		Status:    "available",
		Subnets:   subnets,
		CreatedAt: acl.CreatedAt,
		Rules:     ruleResponses,
	}

	WriteJSON(w, http.StatusOK, resp)
}

// DeleteNetworkACL handles DELETE /api/v1/network-acls/{id}
func (h *NetworkACLHandler) DeleteNetworkACL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	// Check authorization
	userInfo := auth.GetUserFromContext(r.Context())
	if userInfo == nil || userInfo.Name == "" {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	allowed, err := h.rbac.CheckAccess(r.Context(), userInfo.Name, userInfo.Groups, "delete", "networkacls", "default")
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "authorization check failed", "AUTHZ_CHECK_FAILED")
		return
	}
	if !allowed {
		WriteError(w, http.StatusForbidden, "forbidden", "FORBIDDEN")
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/v1/network-acls/")
	id = strings.Split(id, "/")[0]

	slog.DebugContext(r.Context(), "deleting network ACL", "id", id)

	err = h.vpcClient.DeleteNetworkACL(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to delete network ACL", "id", id, "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to delete network ACL", "DELETE_ACL_FAILED")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// AddNetworkACLRule handles POST /api/v1/network-acls/{id}/rules
func (h *NetworkACLHandler) AddNetworkACLRule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	// Check authorization
	userInfo := auth.GetUserFromContext(r.Context())
	if userInfo == nil || userInfo.Name == "" {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	allowed, err := h.rbac.CheckAccess(r.Context(), userInfo.Name, userInfo.Groups, "create", "networkacls", "default")
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "authorization check failed", "AUTHZ_CHECK_FAILED")
		return
	}
	if !allowed {
		WriteError(w, http.StatusForbidden, "forbidden", "FORBIDDEN")
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/v1/network-acls/")
	id = strings.Split(id, "/")[0]

	var req model.ACLRuleRequest
	if err := ReadJSON(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body", "INVALID_REQUEST")
		return
	}

	slog.DebugContext(r.Context(), "adding network ACL rule", "acl_id", id)

	rule, err := h.vpcClient.AddNetworkACLRule(r.Context(), id, vpc.CreateACLRuleOptions{
		Name:        req.Name,
		Action:      req.Action,
		Direction:   req.Direction,
		Protocol:    req.Protocol,
		Source:      req.Source,
		Destination: req.Destination,
		PortMin:     req.PortMin,
		PortMax:     req.PortMax,
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to add rule", "id", id, "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to add rule", "ADD_RULE_FAILED")
		return
	}

	resp := model.ACLRuleResponse{
		ID:          rule.ID,
		Name:        rule.Name,
		Action:      rule.Action,
		Direction:   rule.Direction,
		Protocol:    rule.Protocol,
		Source:      rule.Source,
		Destination: rule.Destination,
		PortMin:     rule.PortMin,
		PortMax:     rule.PortMax,
	}

	WriteJSON(w, http.StatusCreated, resp)
}

// UpdateNetworkACLRule handles PATCH /api/v1/network-acls/{id}/rules/{ruleId}
func (h *NetworkACLHandler) UpdateNetworkACLRule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	// Check authorization
	userInfo := auth.GetUserFromContext(r.Context())
	if userInfo == nil || userInfo.Name == "" {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	allowed, err := h.rbac.CheckAccess(r.Context(), userInfo.Name, userInfo.Groups, "update", "networkacls", "default")
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "authorization check failed", "AUTHZ_CHECK_FAILED")
		return
	}
	if !allowed {
		WriteError(w, http.StatusForbidden, "forbidden", "FORBIDDEN")
		return
	}

	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/network-acls/"), "/")
	if len(parts) < 3 {
		WriteError(w, http.StatusBadRequest, "invalid path", "INVALID_PATH")
		return
	}
	aclID := parts[0]
	ruleID := parts[2]

	var req model.ACLRuleRequest
	if err := ReadJSON(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body", "INVALID_REQUEST")
		return
	}

	slog.DebugContext(r.Context(), "updating network ACL rule", "acl_id", aclID, "rule_id", ruleID)

	// Build update options with pointer fields for partial update
	updateOpts := vpc.UpdateACLRuleOptions{}
	if req.Name != "" {
		updateOpts.Name = &req.Name
	}
	if req.Direction != "" {
		updateOpts.Direction = &req.Direction
	}
	if req.Action != "" {
		updateOpts.Action = &req.Action
	}
	if req.PortMin != nil {
		updateOpts.PortMin = req.PortMin
	}
	if req.PortMax != nil {
		updateOpts.PortMax = req.PortMax
	}
	if req.Source != "" {
		updateOpts.Source = &req.Source
	}
	if req.Destination != "" {
		updateOpts.Destination = &req.Destination
	}

	rule, err := h.vpcClient.UpdateNetworkACLRule(r.Context(), aclID, ruleID, updateOpts)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to update rule", "acl_id", aclID, "rule_id", ruleID, "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to update rule", "UPDATE_RULE_FAILED")
		return
	}

	resp := model.ACLRuleResponse{
		ID:          rule.ID,
		Name:        rule.Name,
		Action:      rule.Action,
		Direction:   rule.Direction,
		Protocol:    rule.Protocol,
		Source:      rule.Source,
		Destination: rule.Destination,
		PortMin:     rule.PortMin,
		PortMax:     rule.PortMax,
	}

	WriteJSON(w, http.StatusOK, resp)
}

// DeleteNetworkACLRule handles DELETE /api/v1/network-acls/{id}/rules/{ruleId}
func (h *NetworkACLHandler) DeleteNetworkACLRule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	// Check authorization
	userInfo := auth.GetUserFromContext(r.Context())
	if userInfo == nil || userInfo.Name == "" {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	allowed, err := h.rbac.CheckAccess(r.Context(), userInfo.Name, userInfo.Groups, "delete", "networkacls", "default")
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "authorization check failed", "AUTHZ_CHECK_FAILED")
		return
	}
	if !allowed {
		WriteError(w, http.StatusForbidden, "forbidden", "FORBIDDEN")
		return
	}

	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/network-acls/"), "/")
	if len(parts) < 3 {
		WriteError(w, http.StatusBadRequest, "invalid path", "INVALID_PATH")
		return
	}
	aclID := parts[0]
	ruleID := parts[2]

	slog.DebugContext(r.Context(), "deleting network ACL rule", "acl_id", aclID, "rule_id", ruleID)

	err = h.vpcClient.DeleteNetworkACLRule(r.Context(), aclID, ruleID)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to delete rule", "acl_id", aclID, "rule_id", ruleID, "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to delete rule", "DELETE_RULE_FAILED")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
