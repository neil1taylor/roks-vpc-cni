package handler

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/IBM/roks-vpc-network-operator/cmd/bff/internal/auth"
	"github.com/IBM/roks-vpc-network-operator/cmd/bff/internal/model"
	"github.com/IBM/roks-vpc-network-operator/pkg/vpc"
)

// SecurityGroupHandler handles security group operations
type SecurityGroupHandler struct {
	vpcClient    vpc.ExtendedClient
	rbac         *auth.RBACChecker
	defaultVPCID string
}

// NewSecurityGroupHandler creates a new security group handler
func NewSecurityGroupHandler(vpcClient vpc.ExtendedClient, rbac *auth.RBACChecker, defaultVPCID string) *SecurityGroupHandler {
	return &SecurityGroupHandler{
		vpcClient:    vpcClient,
		rbac:         rbac,
		defaultVPCID: defaultVPCID,
	}
}

// ListSecurityGroups handles GET /api/v1/security-groups
func (h *SecurityGroupHandler) ListSecurityGroups(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	vpcID := GetQueryParam(r, "vpc_id")
	if vpcID == "" {
		vpcID = h.defaultVPCID
	}
	slog.DebugContext(r.Context(), "listing security groups", "vpc_id", vpcID)

	// Call VPC client to list security groups
	sgs, err := h.vpcClient.ListSecurityGroups(r.Context(), vpcID)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to list security groups", "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to list security groups", "LIST_SG_FAILED")
		return
	}

	// Convert to response format
	responses := make([]model.SecurityGroupResponse, 0, len(sgs))
	for _, sg := range sgs {
		responses = append(responses, model.SecurityGroupResponse{
			ID:          sg.ID,
			Name:        sg.Name,
			VPC:         model.RefResponse{ID: sg.VPCID},
			Description: sg.Description,
			Status:      "available",
			CreatedAt:   sg.CreatedAt,
		})
	}

	WriteJSON(w, http.StatusOK, responses)
}

// CreateSecurityGroup handles POST /api/v1/security-groups
func (h *SecurityGroupHandler) CreateSecurityGroup(w http.ResponseWriter, r *http.Request) {
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

	allowed, err := h.rbac.CheckAccess(r.Context(), userInfo.Name, userInfo.Groups, "create", "securitygroups", "default")
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "authorization check failed", "AUTHZ_CHECK_FAILED")
		return
	}
	if !allowed {
		WriteError(w, http.StatusForbidden, "forbidden", "FORBIDDEN")
		return
	}

	var req model.SecurityGroupRequest
	if err := ReadJSON(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body", "INVALID_REQUEST")
		return
	}

	slog.DebugContext(r.Context(), "creating security group", "name", req.Name, "vpc_id", req.VPCID)

	// Call VPC client to create security group
	sg, err := h.vpcClient.CreateSecurityGroup(r.Context(), vpc.CreateSecurityGroupOptions{
		Name:        req.Name,
		VPCID:       req.VPCID,
		Description: req.Description,
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to create security group", "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to create security group", "CREATE_SG_FAILED")
		return
	}

	resp := model.SecurityGroupResponse{
		ID:          sg.ID,
		Name:        sg.Name,
		VPC:         model.RefResponse{ID: sg.VPCID},
		Description: sg.Description,
		Status:      "available",
		CreatedAt:   sg.CreatedAt,
	}

	WriteJSON(w, http.StatusCreated, resp)
}

// GetSecurityGroup handles GET /api/v1/security-groups/{id}
func (h *SecurityGroupHandler) GetSecurityGroup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/v1/security-groups/")
	id = strings.Split(id, "/")[0]

	slog.DebugContext(r.Context(), "getting security group", "id", id)

	sg, err := h.vpcClient.GetSecurityGroup(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to get security group", "id", id, "error", err)
		WriteError(w, http.StatusNotFound, "security group not found", "SG_NOT_FOUND")
		return
	}

	// Convert rules from the SG (GetSecurityGroup returns rules inline)
	ruleResponses := make([]model.RuleResponse, 0, len(sg.Rules))
	for _, rule := range sg.Rules {
		ruleResponses = append(ruleResponses, ruleToResponse(&rule))
	}

	resp := model.SecurityGroupResponse{
		ID:          sg.ID,
		Name:        sg.Name,
		VPC:         model.RefResponse{ID: sg.VPCID},
		Description: sg.Description,
		Status:      "available",
		CreatedAt:   sg.CreatedAt,
		Rules:       ruleResponses,
	}

	WriteJSON(w, http.StatusOK, resp)
}

// DeleteSecurityGroup handles DELETE /api/v1/security-groups/{id}
func (h *SecurityGroupHandler) DeleteSecurityGroup(w http.ResponseWriter, r *http.Request) {
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

	allowed, err := h.rbac.CheckAccess(r.Context(), userInfo.Name, userInfo.Groups, "delete", "securitygroups", "default")
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "authorization check failed", "AUTHZ_CHECK_FAILED")
		return
	}
	if !allowed {
		WriteError(w, http.StatusForbidden, "forbidden", "FORBIDDEN")
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/v1/security-groups/")
	id = strings.Split(id, "/")[0]

	slog.DebugContext(r.Context(), "deleting security group", "id", id)

	err = h.vpcClient.DeleteSecurityGroup(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to delete security group", "id", id, "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to delete security group", "DELETE_SG_FAILED")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// AddSecurityGroupRule handles POST /api/v1/security-groups/{id}/rules
func (h *SecurityGroupHandler) AddSecurityGroupRule(w http.ResponseWriter, r *http.Request) {
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

	allowed, err := h.rbac.CheckAccess(r.Context(), userInfo.Name, userInfo.Groups, "create", "securitygroups", "default")
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "authorization check failed", "AUTHZ_CHECK_FAILED")
		return
	}
	if !allowed {
		WriteError(w, http.StatusForbidden, "forbidden", "FORBIDDEN")
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/v1/security-groups/")
	id = strings.Split(id, "/")[0]

	var req model.RuleRequest
	if err := ReadJSON(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body", "INVALID_REQUEST")
		return
	}

	slog.DebugContext(r.Context(), "adding security group rule", "sg_id", id)

	rule, err := h.vpcClient.AddSecurityGroupRule(r.Context(), id, vpc.CreateSGRuleOptions{
		Direction:  req.Direction,
		Protocol:   req.Protocol,
		PortMin:    req.PortMin,
		PortMax:    req.PortMax,
		RemoteCIDR: req.RemoteCIDR,
		RemoteSGID: req.RemoteSGID,
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to add rule", "id", id, "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to add rule", "ADD_RULE_FAILED")
		return
	}

	WriteJSON(w, http.StatusCreated, ruleToResponse(rule))
}

// UpdateSecurityGroupRule handles PATCH /api/v1/security-groups/{id}/rules/{ruleId}
func (h *SecurityGroupHandler) UpdateSecurityGroupRule(w http.ResponseWriter, r *http.Request) {
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

	allowed, err := h.rbac.CheckAccess(r.Context(), userInfo.Name, userInfo.Groups, "update", "securitygroups", "default")
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "authorization check failed", "AUTHZ_CHECK_FAILED")
		return
	}
	if !allowed {
		WriteError(w, http.StatusForbidden, "forbidden", "FORBIDDEN")
		return
	}

	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/security-groups/"), "/")
	if len(parts) < 3 {
		WriteError(w, http.StatusBadRequest, "invalid path", "INVALID_PATH")
		return
	}
	sgID := parts[0]
	ruleID := parts[2]

	var req model.RuleRequest
	if err := ReadJSON(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body", "INVALID_REQUEST")
		return
	}

	slog.DebugContext(r.Context(), "updating security group rule", "sg_id", sgID, "rule_id", ruleID)

	// Build update options with pointer fields for partial update
	updateOpts := vpc.UpdateSGRuleOptions{}
	if req.Direction != "" {
		updateOpts.Direction = &req.Direction
	}
	if req.PortMin != nil {
		updateOpts.PortMin = req.PortMin
	}
	if req.PortMax != nil {
		updateOpts.PortMax = req.PortMax
	}
	if req.RemoteCIDR != "" {
		updateOpts.RemoteCIDR = &req.RemoteCIDR
	}
	if req.RemoteSGID != "" {
		updateOpts.RemoteSGID = &req.RemoteSGID
	}

	rule, err := h.vpcClient.UpdateSecurityGroupRule(r.Context(), sgID, ruleID, updateOpts)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to update rule", "sg_id", sgID, "rule_id", ruleID, "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to update rule", "UPDATE_RULE_FAILED")
		return
	}

	WriteJSON(w, http.StatusOK, ruleToResponse(rule))
}

// DeleteSecurityGroupRule handles DELETE /api/v1/security-groups/{id}/rules/{ruleId}
func (h *SecurityGroupHandler) DeleteSecurityGroupRule(w http.ResponseWriter, r *http.Request) {
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

	allowed, err := h.rbac.CheckAccess(r.Context(), userInfo.Name, userInfo.Groups, "delete", "securitygroups", "default")
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "authorization check failed", "AUTHZ_CHECK_FAILED")
		return
	}
	if !allowed {
		WriteError(w, http.StatusForbidden, "forbidden", "FORBIDDEN")
		return
	}

	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/security-groups/"), "/")
	if len(parts) < 3 {
		WriteError(w, http.StatusBadRequest, "invalid path", "INVALID_PATH")
		return
	}
	sgID := parts[0]
	ruleID := parts[2]

	slog.DebugContext(r.Context(), "deleting security group rule", "sg_id", sgID, "rule_id", ruleID)

	err = h.vpcClient.DeleteSecurityGroupRule(r.Context(), sgID, ruleID)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to delete rule", "sg_id", sgID, "rule_id", ruleID, "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to delete rule", "DELETE_RULE_FAILED")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ruleToResponse converts a VPC SecurityGroupRule to a RuleResponse with
// unified Remote/RemoteType fields matching the frontend SecurityGroupRule interface.
func ruleToResponse(rule *vpc.SecurityGroupRule) model.RuleResponse {
	resp := model.RuleResponse{
		ID:        rule.ID,
		Direction: rule.Direction,
		Protocol:  rule.Protocol,
		PortMin:   rule.PortMin,
		PortMax:   rule.PortMax,
	}
	if rule.Remote.CIDRBlock != "" {
		resp.Remote = rule.Remote.CIDRBlock
		resp.RemoteType = "cidr"
	} else if rule.Remote.SecurityGroupID != "" {
		resp.Remote = rule.Remote.SecurityGroupID
		resp.RemoteType = "sg"
	}
	return resp
}
