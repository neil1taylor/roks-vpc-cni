package handler

import (
	"net/http"

	"github.com/IBM/roks-vpc-network-operator/cmd/bff/internal/auth"
	"github.com/IBM/roks-vpc-network-operator/pkg/vpc"

	"k8s.io/client-go/kubernetes"
)

// SetupRoutes registers all HTTP handlers with the mux
func SetupRoutes(mux *http.ServeMux, vpcClient vpc.ExtendedClient, rbacChecker *auth.RBACChecker) {
	SetupRoutesWithK8s(mux, vpcClient, rbacChecker, nil)
}

// ClusterInfo holds cluster mode information passed to handlers.
type ClusterInfo struct {
	// Mode is "roks" or "unmanaged"
	Mode string
}

// SetupRoutesWithK8s registers all HTTP handlers with the mux and K8s client
func SetupRoutesWithK8s(mux *http.ServeMux, vpcClient vpc.ExtendedClient, rbacChecker *auth.RBACChecker, k8sClient kubernetes.Interface) {
	SetupRoutesWithClusterInfo(mux, vpcClient, rbacChecker, k8sClient, ClusterInfo{Mode: "unmanaged"})
}

// SetupRoutesWithClusterInfo registers all HTTP handlers with cluster mode awareness
func SetupRoutesWithClusterInfo(mux *http.ServeMux, vpcClient vpc.ExtendedClient, rbacChecker *auth.RBACChecker, k8sClient kubernetes.Interface, clusterInfo ClusterInfo) {

	// Health check endpoints
	mux.HandleFunc("/healthz", HealthHandler)
	mux.HandleFunc("/readyz", ReadyHandler)

	// Create handlers
	sgHandler := NewSecurityGroupHandler(vpcClient, rbacChecker)
	aclHandler := NewNetworkACLHandler(vpcClient, rbacChecker)
	vpcHandler := NewVPCHandler(vpcClient)
	zoneHandler := NewZoneHandler(vpcClient)
	topologyHandler := NewTopologyHandler(vpcClient, k8sClient)

	// Wrap all handlers with authentication middleware
	authMiddleware := func(handler http.HandlerFunc) http.Handler {
		return auth.AuthMiddleware(http.HandlerFunc(handler))
	}

	// Security Group routes — use method-based dispatch to avoid duplicate registrations
	mux.HandleFunc("/api/v1/security-groups", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			authMiddleware(sgHandler.ListSecurityGroups).ServeHTTP(w, r)
		case http.MethodPost:
			authMiddleware(sgHandler.CreateSecurityGroup).ServeHTTP(w, r)
		default:
			WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		}
	})

	// Security Group detail and operations
	mux.HandleFunc("/api/v1/security-groups/", handleSecurityGroupDetail(sgHandler))

	// Network ACL routes — use method-based dispatch to avoid duplicate registrations
	mux.HandleFunc("/api/v1/network-acls", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			authMiddleware(aclHandler.ListNetworkACLs).ServeHTTP(w, r)
		case http.MethodPost:
			authMiddleware(aclHandler.CreateNetworkACL).ServeHTTP(w, r)
		default:
			WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		}
	})

	// Network ACL detail and operations
	mux.HandleFunc("/api/v1/network-acls/", handleNetworkACLDetail(aclHandler))

	// VPC routes
	mux.HandleFunc("/api/v1/vpcs", wrapHandler(authMiddleware(vpcHandler.ListVPCs)))
	mux.HandleFunc("/api/v1/vpcs/", wrapHandler(authMiddleware(vpcHandler.GetVPC)))

	// Zone routes
	mux.HandleFunc("/api/v1/zones", wrapHandler(authMiddleware(zoneHandler.ListZones)))

	// Topology routes
	mux.HandleFunc("/api/v1/topology", wrapHandler(authMiddleware(topologyHandler.GetTopology)))

	// Cluster info endpoint — tells the console plugin what mode the cluster is in
	// This allows the frontend to show/hide features based on ROKS vs unmanaged
	mux.HandleFunc("/api/v1/cluster-info", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
			return
		}
		WriteJSON(w, http.StatusOK, map[string]interface{}{
			"clusterMode": clusterInfo.Mode,
			"features": map[string]interface{}{
				"vniManagement":            clusterInfo.Mode == "unmanaged",
				"vlanAttachmentManagement": clusterInfo.Mode == "unmanaged",
				"subnetManagement":         true, // Available on both ROKS and unmanaged
				"securityGroupManagement":  true, // Available on both
				"networkACLManagement":     true, // Available on both
				"floatingIPManagement":     true, // Available on both
				"roksAPIAvailable":         false, // TODO(roks-api): Set to true when ROKS API is ready
			},
		})
	})
}

// wrapHandler wraps an http.Handler to work with HandleFunc
func wrapHandler(handler http.Handler) http.HandlerFunc {
	return handler.ServeHTTP
}

// handleSecurityGroupDetail routes security group detail and rule operations
func handleSecurityGroupDetail(h *SecurityGroupHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		if path == "/api/v1/security-groups/" {
			WriteError(w, http.StatusBadRequest, "invalid path", "INVALID_PATH")
			return
		}

		// Check if it's a rule operation
		if contains(path, "/rules") {
			handleSecurityGroupRules(h, w, r, path)
			return
		}

		// Single SG operations
		switch r.Method {
		case http.MethodGet:
			h.GetSecurityGroup(w, r)
		case http.MethodDelete:
			wrapped := auth.AuthMiddleware(http.HandlerFunc(h.DeleteSecurityGroup))
			wrapped.ServeHTTP(w, r)
		default:
			WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		}
	}
}

// handleSecurityGroupRules routes security group rule operations
func handleSecurityGroupRules(h *SecurityGroupHandler, w http.ResponseWriter, r *http.Request, path string) {
	switch r.Method {
	case http.MethodPost:
		wrapped := auth.AuthMiddleware(http.HandlerFunc(h.AddSecurityGroupRule))
		wrapped.ServeHTTP(w, r)
	case http.MethodPatch:
		wrapped := auth.AuthMiddleware(http.HandlerFunc(h.UpdateSecurityGroupRule))
		wrapped.ServeHTTP(w, r)
	case http.MethodDelete:
		wrapped := auth.AuthMiddleware(http.HandlerFunc(h.DeleteSecurityGroupRule))
		wrapped.ServeHTTP(w, r)
	default:
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
	}
}

// handleNetworkACLDetail routes network ACL detail and rule operations
func handleNetworkACLDetail(h *NetworkACLHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		if path == "/api/v1/network-acls/" {
			WriteError(w, http.StatusBadRequest, "invalid path", "INVALID_PATH")
			return
		}

		// Check if it's a rule operation
		if contains(path, "/rules") {
			handleNetworkACLRules(h, w, r, path)
			return
		}

		// Single ACL operations
		switch r.Method {
		case http.MethodGet:
			h.GetNetworkACL(w, r)
		case http.MethodDelete:
			wrapped := auth.AuthMiddleware(http.HandlerFunc(h.DeleteNetworkACL))
			wrapped.ServeHTTP(w, r)
		default:
			WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		}
	}
}

// handleNetworkACLRules routes network ACL rule operations
func handleNetworkACLRules(h *NetworkACLHandler, w http.ResponseWriter, r *http.Request, path string) {
	switch r.Method {
	case http.MethodPost:
		wrapped := auth.AuthMiddleware(http.HandlerFunc(h.AddNetworkACLRule))
		wrapped.ServeHTTP(w, r)
	case http.MethodPatch:
		wrapped := auth.AuthMiddleware(http.HandlerFunc(h.UpdateNetworkACLRule))
		wrapped.ServeHTTP(w, r)
	case http.MethodDelete:
		wrapped := auth.AuthMiddleware(http.HandlerFunc(h.DeleteNetworkACLRule))
		wrapped.ServeHTTP(w, r)
	default:
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
	}
}

// Helper function
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
