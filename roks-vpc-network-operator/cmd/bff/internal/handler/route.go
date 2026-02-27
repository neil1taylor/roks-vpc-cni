package handler

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/IBM/roks-vpc-network-operator/cmd/bff/internal/auth"
	"github.com/IBM/roks-vpc-network-operator/cmd/bff/internal/model"
	"github.com/IBM/roks-vpc-network-operator/pkg/vpc"
)

// RouteHandler handles VPC routing table and route operations.
type RouteHandler struct {
	vpcClient    vpc.ExtendedClient
	rbac         *auth.RBACChecker
	defaultVPCID string
}

// NewRouteHandler creates a new route handler.
func NewRouteHandler(vpcClient vpc.ExtendedClient, rbac *auth.RBACChecker, defaultVPCID string) *RouteHandler {
	return &RouteHandler{
		vpcClient:    vpcClient,
		rbac:         rbac,
		defaultVPCID: defaultVPCID,
	}
}

func (h *RouteHandler) resolveVPCID(r *http.Request) string {
	vpcID := GetQueryParam(r, "vpc_id")
	if vpcID == "" {
		vpcID = h.defaultVPCID
	}
	return vpcID
}

// ListRoutingTables handles GET /api/v1/routing-tables
func (h *RouteHandler) ListRoutingTables(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	vpcID := h.resolveVPCID(r)
	if vpcID == "" {
		WriteError(w, http.StatusBadRequest, "vpc_id is required", "MISSING_VPC_ID")
		return
	}

	slog.DebugContext(r.Context(), "listing routing tables", "vpc_id", vpcID)

	tables, err := h.vpcClient.ListRoutingTables(r.Context(), vpcID)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to list routing tables", "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to list routing tables", "LIST_RT_FAILED")
		return
	}

	responses := make([]model.RoutingTableResponse, 0, len(tables))
	for _, t := range tables {
		responses = append(responses, model.RoutingTableResponse{
			ID:             t.ID,
			Name:           t.Name,
			IsDefault:      t.IsDefault,
			LifecycleState: t.LifecycleState,
			RouteCount:     t.RouteCount,
			CreatedAt:      t.CreatedAt,
		})
	}

	WriteJSON(w, http.StatusOK, responses)
}

// GetRoutingTable handles GET /api/v1/routing-tables/{rtId}
func (h *RouteHandler) GetRoutingTable(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	vpcID := h.resolveVPCID(r)
	if vpcID == "" {
		WriteError(w, http.StatusBadRequest, "vpc_id is required", "MISSING_VPC_ID")
		return
	}

	rtID := strings.TrimPrefix(r.URL.Path, "/api/v1/routing-tables/")
	rtID = strings.Split(rtID, "/")[0]

	slog.DebugContext(r.Context(), "getting routing table", "rt_id", rtID)

	rt, err := h.vpcClient.GetRoutingTable(r.Context(), vpcID, rtID)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to get routing table", "rt_id", rtID, "error", err)
		WriteError(w, http.StatusNotFound, "routing table not found", "RT_NOT_FOUND")
		return
	}

	WriteJSON(w, http.StatusOK, model.RoutingTableResponse{
		ID:             rt.ID,
		Name:           rt.Name,
		IsDefault:      rt.IsDefault,
		LifecycleState: rt.LifecycleState,
		RouteCount:     rt.RouteCount,
		CreatedAt:      rt.CreatedAt,
	})
}

// ListRoutes handles GET /api/v1/routing-tables/{rtId}/routes
func (h *RouteHandler) ListRoutes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	vpcID := h.resolveVPCID(r)
	if vpcID == "" {
		WriteError(w, http.StatusBadRequest, "vpc_id is required", "MISSING_VPC_ID")
		return
	}

	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/routing-tables/"), "/")
	if len(parts) < 2 {
		WriteError(w, http.StatusBadRequest, "invalid path", "INVALID_PATH")
		return
	}
	rtID := parts[0]

	slog.DebugContext(r.Context(), "listing routes", "rt_id", rtID)

	routes, err := h.vpcClient.ListRoutes(r.Context(), vpcID, rtID)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to list routes", "rt_id", rtID, "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to list routes", "LIST_ROUTES_FAILED")
		return
	}

	responses := make([]model.RouteResponse, 0, len(routes))
	for _, route := range routes {
		responses = append(responses, routeToResponse(route))
	}

	WriteJSON(w, http.StatusOK, responses)
}

// CreateRoute handles POST /api/v1/routing-tables/{rtId}/routes
func (h *RouteHandler) CreateRoute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	userInfo := auth.GetUserFromContext(r.Context())
	if userInfo == nil || userInfo.Name == "" {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	allowed, err := h.rbac.CheckAccess(r.Context(), userInfo.Name, userInfo.Groups, "create", "routes", "default")
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "authorization check failed", "AUTHZ_CHECK_FAILED")
		return
	}
	if !allowed {
		WriteError(w, http.StatusForbidden, "forbidden", "FORBIDDEN")
		return
	}

	vpcID := h.resolveVPCID(r)
	if vpcID == "" {
		WriteError(w, http.StatusBadRequest, "vpc_id is required", "MISSING_VPC_ID")
		return
	}

	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/routing-tables/"), "/")
	if len(parts) < 2 {
		WriteError(w, http.StatusBadRequest, "invalid path", "INVALID_PATH")
		return
	}
	rtID := parts[0]

	var req model.CreateRouteRequest
	if err := ReadJSON(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body", "INVALID_REQUEST")
		return
	}

	if req.Destination == "" || req.Action == "" || req.Zone == "" {
		WriteError(w, http.StatusBadRequest, "destination, action, and zone are required", "INVALID_REQUEST")
		return
	}

	slog.DebugContext(r.Context(), "creating route", "rt_id", rtID, "destination", req.Destination, "action", req.Action)

	route, err := h.vpcClient.CreateRoute(r.Context(), vpcID, rtID, vpc.CreateRouteOptions{
		Name:        req.Name,
		Destination: req.Destination,
		Action:      req.Action,
		NextHopIP:   req.NextHopIP,
		Zone:        req.Zone,
		Priority:    req.Priority,
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to create route", "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to create route", "CREATE_ROUTE_FAILED")
		return
	}

	WriteJSON(w, http.StatusCreated, routeToResponse(*route))
}

// DeleteRoute handles DELETE /api/v1/routing-tables/{rtId}/routes/{routeId}
func (h *RouteHandler) DeleteRoute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	userInfo := auth.GetUserFromContext(r.Context())
	if userInfo == nil || userInfo.Name == "" {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	allowed, err := h.rbac.CheckAccess(r.Context(), userInfo.Name, userInfo.Groups, "delete", "routes", "default")
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "authorization check failed", "AUTHZ_CHECK_FAILED")
		return
	}
	if !allowed {
		WriteError(w, http.StatusForbidden, "forbidden", "FORBIDDEN")
		return
	}

	vpcID := h.resolveVPCID(r)
	if vpcID == "" {
		WriteError(w, http.StatusBadRequest, "vpc_id is required", "MISSING_VPC_ID")
		return
	}

	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/routing-tables/"), "/")
	if len(parts) < 3 {
		WriteError(w, http.StatusBadRequest, "invalid path", "INVALID_PATH")
		return
	}
	rtID := parts[0]
	routeID := parts[2]

	slog.DebugContext(r.Context(), "deleting route", "rt_id", rtID, "route_id", routeID)

	err = h.vpcClient.DeleteRoute(r.Context(), vpcID, rtID, routeID)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to delete route", "rt_id", rtID, "route_id", routeID, "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to delete route", "DELETE_ROUTE_FAILED")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func routeToResponse(r vpc.Route) model.RouteResponse {
	return model.RouteResponse{
		ID:             r.ID,
		Name:           r.Name,
		Destination:    r.Destination,
		Action:         r.Action,
		NextHop:        r.NextHop,
		Zone:           r.Zone,
		Priority:       r.Priority,
		Origin:         r.Origin,
		LifecycleState: r.LifecycleState,
		CreatedAt:      r.CreatedAt,
	}
}
