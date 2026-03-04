package handler

import (
	"net/http"
	"strings"

	"github.com/IBM/roks-vpc-network-operator/cmd/bff/internal/auth"
	"github.com/IBM/roks-vpc-network-operator/cmd/bff/internal/thanos"
	"github.com/IBM/roks-vpc-network-operator/pkg/vpc"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// SetupRoutes registers all HTTP handlers with the mux
func SetupRoutes(mux *http.ServeMux, vpcClient vpc.ExtendedClient, rbacChecker *auth.RBACChecker) {
	SetupRoutesWithK8s(mux, vpcClient, rbacChecker, nil)
}

// ClusterInfo holds cluster mode information passed to handlers.
type ClusterInfo struct {
	// Mode is "roks" or "unmanaged"
	Mode string
	// Region is the VPC region (e.g. "eu-de")
	Region string
	// VPCID is the cluster's VPC ID for scoping API calls
	VPCID string
}

// SetupRoutesWithK8s registers all HTTP handlers with the mux and K8s client
func SetupRoutesWithK8s(mux *http.ServeMux, vpcClient vpc.ExtendedClient, rbacChecker *auth.RBACChecker, k8sClient kubernetes.Interface) {
	SetupRoutesWithClusterInfo(mux, vpcClient, rbacChecker, k8sClient, nil, ClusterInfo{Mode: "unmanaged"})
}

// SetupRoutesWithClusterInfo registers all HTTP handlers with cluster mode awareness
func SetupRoutesWithClusterInfo(mux *http.ServeMux, vpcClient vpc.ExtendedClient, rbacChecker *auth.RBACChecker, k8sClient kubernetes.Interface, dynClient dynamic.Interface, clusterInfo ClusterInfo, restConfigs ...*rest.Config) {
	var restConfig *rest.Config
	if len(restConfigs) > 0 {
		restConfig = restConfigs[0]
	}

	// Initialize Thanos client from environment (nil if THANOS_URL not set)
	thanosClient := thanos.NewClientFromEnv()

	// Health check endpoints
	mux.HandleFunc("/healthz", HealthHandler)
	mux.HandleFunc("/readyz", ReadyHandler)

	// Create handlers
	sgHandler := NewSecurityGroupHandler(vpcClient, rbacChecker, clusterInfo.VPCID)
	aclHandler := NewNetworkACLHandler(vpcClient, rbacChecker, clusterInfo.VPCID)
	vpcHandler := NewVPCHandler(vpcClient, clusterInfo.VPCID)
	zoneHandler := NewZoneHandler(vpcClient, clusterInfo.Region)
	topologyHandler := NewTopologyHandler(vpcClient, k8sClient, dynClient, clusterInfo.VPCID)

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

	// Subnet routes
	subnetHandler := NewSubnetHandler(vpcClient, rbacChecker, clusterInfo.VPCID)
	reservedIPHandler := NewReservedIPHandler(vpcClient, clusterInfo.VPCID)
	mux.HandleFunc("/api/v1/subnets", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			authMiddleware(subnetHandler.ListSubnets).ServeHTTP(w, r)
		case http.MethodPost:
			authMiddleware(subnetHandler.CreateSubnet).ServeHTTP(w, r)
		default:
			WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		}
	})
	mux.HandleFunc("/api/v1/subnets/", func(w http.ResponseWriter, r *http.Request) {
		if contains(r.URL.Path, "/reserved-ips") {
			authMiddleware(reservedIPHandler.ListSubnetReservedIPs).ServeHTTP(w, r)
			return
		}
		authMiddleware(subnetHandler.GetSubnet).ServeHTTP(w, r)
	})

	// VNI routes
	vniHandler := NewVNIHandler(vpcClient, clusterInfo.VPCID)
	mux.HandleFunc("/api/v1/vnis", wrapHandler(authMiddleware(vniHandler.ListVNIs)))
	mux.HandleFunc("/api/v1/vnis/", wrapHandler(authMiddleware(vniHandler.GetVNI)))

	// Floating IP routes
	fipHandler := NewFloatingIPHandler(vpcClient, rbacChecker, clusterInfo.VPCID)
	mux.HandleFunc("/api/v1/floating-ips", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			authMiddleware(fipHandler.ListFloatingIPs).ServeHTTP(w, r)
		case http.MethodPost:
			authMiddleware(fipHandler.CreateFloatingIP).ServeHTTP(w, r)
		default:
			WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		}
	})
	mux.HandleFunc("/api/v1/floating-ips/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			authMiddleware(fipHandler.GetFloatingIP).ServeHTTP(w, r)
		case http.MethodPatch:
			authMiddleware(fipHandler.UpdateFloatingIP).ServeHTTP(w, r)
		default:
			WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		}
	})

	// Route management routes
	routeHandler := NewRouteHandler(vpcClient, rbacChecker, clusterInfo.VPCID)
	mux.HandleFunc("/api/v1/routing-tables", wrapHandler(authMiddleware(routeHandler.ListRoutingTables)))
	mux.HandleFunc("/api/v1/routing-tables/", handleRoutingTableDetail(routeHandler))

	// Address Prefix routes
	apHandler := NewAddressPrefixHandler(vpcClient, clusterInfo.VPCID)
	mux.HandleFunc("/api/v1/address-prefixes", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			authMiddleware(apHandler.ListAddressPrefixes).ServeHTTP(w, r)
		case http.MethodPost:
			authMiddleware(apHandler.CreateAddressPrefix).ServeHTTP(w, r)
		default:
			WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		}
	})

	// Public Gateway routes (read-only, for network creation dropdown)
	pgwHandler := NewPublicGatewayHandler(vpcClient, clusterInfo.VPCID)
	mux.HandleFunc("/api/v1/public-gateways", wrapHandler(authMiddleware(pgwHandler.ListPublicGateways)))

	// Topology routes
	mux.HandleFunc("/api/v1/topology", wrapHandler(authMiddleware(topologyHandler.GetTopology)))

	// Namespace routes
	nsHandler := NewNamespaceHandler(k8sClient)
	mux.HandleFunc("/api/v1/namespaces", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			authMiddleware(nsHandler.ListNamespaces).ServeHTTP(w, r)
		case http.MethodPost:
			authMiddleware(nsHandler.CreateNamespace).ServeHTTP(w, r)
		default:
			WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		}
	})

	// Network (CUDN/UDN) routes
	networkHandler := NewNetworkHandler(k8sClient, dynClient, rbacChecker, clusterInfo)

	mux.HandleFunc("/api/v1/cudns", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			authMiddleware(networkHandler.ListCUDNs).ServeHTTP(w, r)
		case http.MethodPost:
			authMiddleware(networkHandler.CreateCUDN).ServeHTTP(w, r)
		default:
			WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		}
	})

	mux.HandleFunc("/api/v1/cudns/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			authMiddleware(networkHandler.GetCUDN).ServeHTTP(w, r)
		case http.MethodDelete:
			authMiddleware(networkHandler.DeleteCUDN).ServeHTTP(w, r)
		default:
			WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		}
	})

	mux.HandleFunc("/api/v1/udns", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			authMiddleware(networkHandler.ListUDNs).ServeHTTP(w, r)
		case http.MethodPost:
			authMiddleware(networkHandler.CreateUDN).ServeHTTP(w, r)
		default:
			WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		}
	})

	mux.HandleFunc("/api/v1/udns/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			authMiddleware(networkHandler.GetUDN).ServeHTTP(w, r)
		case http.MethodDelete:
			authMiddleware(networkHandler.DeleteUDN).ServeHTTP(w, r)
		default:
			WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		}
	})

	mux.HandleFunc("/api/v1/network-types", wrapHandler(authMiddleware(networkHandler.GetNetworkTypes)))

	// VPCGateway routes
	gwHandler := NewGatewayHandler(dynClient, rbacChecker)
	mux.HandleFunc("/api/v1/gateways", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			authMiddleware(gwHandler.ListGateways).ServeHTTP(w, r)
		case http.MethodPost:
			authMiddleware(gwHandler.CreateGateway).ServeHTTP(w, r)
		default:
			WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		}
	})
	mux.HandleFunc("/api/v1/gateways/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			authMiddleware(gwHandler.GetGateway).ServeHTTP(w, r)
		case http.MethodDelete:
			authMiddleware(gwHandler.DeleteGateway).ServeHTTP(w, r)
		default:
			WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		}
	})

	// Public Address Range (PAR) routes
	parHandler := NewPARHandler(vpcClient, rbacChecker, dynClient, clusterInfo.VPCID)
	mux.HandleFunc("/api/v1/pars", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			authMiddleware(parHandler.ListPARs).ServeHTTP(w, r)
		case http.MethodPost:
			authMiddleware(parHandler.CreatePAR).ServeHTTP(w, r)
		default:
			WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		}
	})
	mux.HandleFunc("/api/v1/pars/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			authMiddleware(parHandler.GetPAR).ServeHTTP(w, r)
		case http.MethodDelete:
			authMiddleware(parHandler.DeletePAR).ServeHTTP(w, r)
		default:
			WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		}
	})

	// VPCRouter routes
	rtHandler := NewRouterHandler(dynClient, k8sClient, restConfig, rbacChecker)
	mux.HandleFunc("/api/v1/routers", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			authMiddleware(rtHandler.ListRouters).ServeHTTP(w, r)
		case http.MethodPost:
			authMiddleware(rtHandler.CreateRouter).ServeHTTP(w, r)
		default:
			WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		}
	})
	metricsHandler := NewRouterMetricsHandler(thanosClient)
	mux.HandleFunc("/api/v1/routers/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		// Check if this is an IDS sub-route
		if strings.HasSuffix(path, "/ids") || strings.Contains(path, "/ids?") {
			if r.Method == http.MethodPatch {
				authMiddleware(rtHandler.UpdateIDS).ServeHTTP(w, r)
			} else {
				WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
			}
			return
		}
		// Check if this is a leases sub-route
		if strings.HasSuffix(path, "/leases") || strings.Contains(path, "/leases?") {
			authMiddleware(rtHandler.GetLeases).ServeHTTP(w, r)
			return
		}
		// Check if this is a reservations sub-route
		if strings.HasSuffix(path, "/reservations") || strings.Contains(path, "/reservations?") {
			if r.Method == http.MethodPatch {
				authMiddleware(rtHandler.UpdateReservations).ServeHTTP(w, r)
			} else {
				WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
			}
			return
		}
		// Check if this is a metrics sub-route
		if metricsHandler != nil && strings.Contains(path, "/metrics/") {
			parts := strings.SplitN(path, "/metrics/", 2)
			if len(parts) >= 2 {
				endpoint := strings.TrimSuffix(parts[1], "/")
				switch endpoint {
				case "summary":
					authMiddleware(metricsHandler.GetSummary).ServeHTTP(w, r)
				case "interfaces":
					authMiddleware(metricsHandler.GetInterfaces).ServeHTTP(w, r)
				case "conntrack":
					authMiddleware(metricsHandler.GetConntrack).ServeHTTP(w, r)
				case "dhcp":
					authMiddleware(metricsHandler.GetDHCP).ServeHTTP(w, r)
				case "nft":
					authMiddleware(metricsHandler.GetNFT).ServeHTTP(w, r)
				default:
					WriteError(w, http.StatusNotFound, "unknown metrics endpoint", "NOT_FOUND")
				}
				return
			}
		}
		// Standard router CRUD
		switch r.Method {
		case http.MethodGet:
			authMiddleware(rtHandler.GetRouter).ServeHTTP(w, r)
		case http.MethodDelete:
			authMiddleware(rtHandler.DeleteRouter).ServeHTTP(w, r)
		default:
			WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		}
	})

	// VPCVPNGateway routes
	vpnHandler := NewVPNGatewayHandler(dynClient, rbacChecker, k8sClient)
	mux.HandleFunc("/api/v1/vpn-gateways", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			authMiddleware(vpnHandler.ListVPNGateways).ServeHTTP(w, r)
		case http.MethodPost:
			authMiddleware(vpnHandler.CreateVPNGateway).ServeHTTP(w, r)
		default:
			WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		}
	})
	mux.HandleFunc("/api/v1/vpn-gateways/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		// Check for /client-config suffix
		if strings.HasSuffix(path, "/client-config") && r.Method == http.MethodPost {
			authMiddleware(vpnHandler.GenerateClientConfig).ServeHTTP(w, r)
			return
		}
		// Check for /clients sub-path
		if strings.Contains(path, "/clients") {
			// Extract suffix after /clients
			clientsIdx := strings.Index(path, "/clients")
			suffix := path[clientsIdx+len("/clients"):]
			if suffix == "" || suffix == "/" {
				// /clients or /clients/ — list
				if r.Method == http.MethodGet {
					authMiddleware(vpnHandler.ListClients).ServeHTTP(w, r)
				} else {
					WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
				}
			} else {
				// /clients/<clientName> — revoke
				if r.Method == http.MethodDelete {
					authMiddleware(vpnHandler.RevokeClient).ServeHTTP(w, r)
				} else {
					WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
				}
			}
			return
		}
		switch r.Method {
		case http.MethodGet:
			authMiddleware(vpnHandler.GetVPNGateway).ServeHTTP(w, r)
		case http.MethodDelete:
			authMiddleware(vpnHandler.DeleteVPNGateway).ServeHTTP(w, r)
		default:
			WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		}
	})

	// VPCL2Bridge routes
	l2bHandler := NewL2BridgeHandler(dynClient, rbacChecker)
	mux.HandleFunc("/api/v1/l2bridges", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			authMiddleware(l2bHandler.ListL2Bridges).ServeHTTP(w, r)
		case http.MethodPost:
			authMiddleware(l2bHandler.CreateL2Bridge).ServeHTTP(w, r)
		default:
			WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		}
	})
	mux.HandleFunc("/api/v1/l2bridges/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			authMiddleware(l2bHandler.GetL2Bridge).ServeHTTP(w, r)
		case http.MethodDelete:
			authMiddleware(l2bHandler.DeleteL2Bridge).ServeHTTP(w, r)
		default:
			WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		}
	})

	// Cluster info endpoint — tells the console plugin what mode the cluster is in
	// This allows the frontend to show/hide features based on ROKS vs unmanaged
	mux.HandleFunc("/api/v1/cluster-info", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
			return
		}
		// VNIs and VLAN attachments are always viewable — the webhook creates them via VPC API
		// regardless of cluster mode. These flags gate listing/viewing, not create/delete.
		roksAPIAvailable := false // TODO(roks-api): set true when ROKS platform API ships
		WriteJSON(w, http.StatusOK, map[string]interface{}{
			"clusterMode": clusterInfo.Mode,
			"vpcId":       clusterInfo.VPCID,
			"features": map[string]interface{}{
				"vniManagement":            true,
				"vlanAttachmentManagement": true,
				"subnetManagement":         true,
				"securityGroupManagement":  true,
				"networkACLManagement":     true,
				"floatingIPManagement":     true,
				"cudnManagement":           true,
				"udnManagement":            true,
				"layer2Support":            true,
				"multiNetworkVMs":          true,
				"routeManagement":          true,
				"parManagement":            true,
				"roksAPIAvailable":         roksAPIAvailable,
				"routerMetrics":            thanosClient != nil,
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

// handleRoutingTableDetail routes routing table detail and route operations
func handleRoutingTableDetail(h *RouteHandler) http.HandlerFunc {
	authMiddleware := func(handler http.HandlerFunc) http.Handler {
		return auth.AuthMiddleware(http.HandlerFunc(handler))
	}
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		if path == "/api/v1/routing-tables/" {
			WriteError(w, http.StatusBadRequest, "invalid path", "INVALID_PATH")
			return
		}

		// Check if it's a route operation (contains /routes)
		if contains(path, "/routes") {
			switch r.Method {
			case http.MethodGet:
				authMiddleware(h.ListRoutes).ServeHTTP(w, r)
			case http.MethodPost:
				authMiddleware(h.CreateRoute).ServeHTTP(w, r)
			case http.MethodDelete:
				authMiddleware(h.DeleteRoute).ServeHTTP(w, r)
			default:
				WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
			}
			return
		}

		// Single routing table operations
		switch r.Method {
		case http.MethodGet:
			h.GetRoutingTable(w, r)
		default:
			WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		}
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
