package handler

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/IBM/roks-vpc-network-operator/cmd/bff/internal/auth"
	"github.com/IBM/roks-vpc-network-operator/pkg/vpc"
)

// FlowLogResponse represents a VPC flow log collector for JSON responses.
type FlowLogResponse struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	TargetSubnetID string `json:"targetSubnetId,omitempty"`
	COSBucketCRN   string `json:"cosBucketCrn,omitempty"`
	IsActive       bool   `json:"isActive"`
	LifecycleState string `json:"lifecycleState"`
}

// FlowLogHandler handles VPC flow log collector operations.
type FlowLogHandler struct {
	vpcClient vpc.ExtendedClient
	rbac      *auth.RBACChecker
}

// NewFlowLogHandler creates a new flow log handler.
func NewFlowLogHandler(vpcClient vpc.ExtendedClient, rbac *auth.RBACChecker) *FlowLogHandler {
	return &FlowLogHandler{
		vpcClient: vpcClient,
		rbac:      rbac,
	}
}

// ListFlowLogs handles GET /api/v1/flow-logs
func (h *FlowLogHandler) ListFlowLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	slog.DebugContext(r.Context(), "listing flow log collectors")

	collectors, err := h.vpcClient.ListFlowLogCollectors(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to list flow log collectors", "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to list flow log collectors", "LIST_FLOW_LOGS_FAILED")
		return
	}

	responses := make([]FlowLogResponse, 0, len(collectors))
	for _, c := range collectors {
		responses = append(responses, flowLogToResponse(c))
	}

	WriteJSON(w, http.StatusOK, responses)
}

// GetFlowLog handles GET /api/v1/flow-logs/{id}
func (h *FlowLogHandler) GetFlowLog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/v1/flow-logs/")
	id = strings.Split(id, "/")[0]

	slog.DebugContext(r.Context(), "getting flow log collector", "id", id)

	collector, err := h.vpcClient.GetFlowLogCollector(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to get flow log collector", "id", id, "error", err)
		WriteError(w, http.StatusNotFound, "flow log collector not found", "FLOW_LOG_NOT_FOUND")
		return
	}

	WriteJSON(w, http.StatusOK, flowLogToResponse(*collector))
}

// DeleteFlowLog handles DELETE /api/v1/flow-logs/{id}
func (h *FlowLogHandler) DeleteFlowLog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	userInfo := auth.GetUserFromContext(r.Context())
	if userInfo == nil || userInfo.Name == "" {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	allowed, err := h.rbac.CheckAccess(r.Context(), userInfo.Name, userInfo.Groups, "delete", "flow-logs", "default")
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "authorization check failed", "AUTHZ_CHECK_FAILED")
		return
	}
	if !allowed {
		WriteError(w, http.StatusForbidden, "forbidden", "FORBIDDEN")
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/v1/flow-logs/")
	id = strings.Split(id, "/")[0]

	slog.DebugContext(r.Context(), "deleting flow log collector", "id", id)

	if err := h.vpcClient.DeleteFlowLogCollector(r.Context(), id); err != nil {
		slog.ErrorContext(r.Context(), "failed to delete flow log collector", "id", id, "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to delete flow log collector", "DELETE_FLOW_LOG_FAILED")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func flowLogToResponse(c vpc.FlowLogCollector) FlowLogResponse {
	return FlowLogResponse{
		ID:             c.ID,
		Name:           c.Name,
		TargetSubnetID: c.TargetSubnetID,
		COSBucketCRN:   c.COSBucketCRN,
		IsActive:       c.IsActive,
		LifecycleState: c.LifecycleState,
	}
}
