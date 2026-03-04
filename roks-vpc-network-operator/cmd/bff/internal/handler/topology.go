package handler

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/IBM/roks-vpc-network-operator/cmd/bff/internal/model"
	"github.com/IBM/roks-vpc-network-operator/cmd/bff/internal/thanos"
	"github.com/IBM/roks-vpc-network-operator/pkg/vpc"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

// CRD GVRs for topology fetching
var (
	vpcSubnetGVR = schema.GroupVersionResource{
		Group: "vpc.roks.ibm.com", Version: "v1alpha1", Resource: "vpcsubnets",
	}
	vniCRDGVR = schema.GroupVersionResource{
		Group: "vpc.roks.ibm.com", Version: "v1alpha1", Resource: "virtualnetworkinterfaces",
	}
	vlanAttachmentGVR = schema.GroupVersionResource{
		Group: "vpc.roks.ibm.com", Version: "v1alpha1", Resource: "vlanattachments",
	}
	floatingIPGVR = schema.GroupVersionResource{
		Group: "vpc.roks.ibm.com", Version: "v1alpha1", Resource: "floatingips",
	}
)

// TopologyHandler handles topology aggregation
type TopologyHandler struct {
	vpcClient     vpc.ExtendedClient
	k8sClient     kubernetes.Interface
	dynamicClient dynamic.Interface
	defaultVPCID  string
	thanos        *thanos.Client
}

// NewTopologyHandler creates a new topology handler
func NewTopologyHandler(vpcClient vpc.ExtendedClient, k8sClient kubernetes.Interface, dynamicClient dynamic.Interface, defaultVPCID string) *TopologyHandler {
	return &TopologyHandler{
		vpcClient:     vpcClient,
		k8sClient:     k8sClient,
		dynamicClient: dynamicClient,
		defaultVPCID:  defaultVPCID,
	}
}

// SetThanosClient configures the Thanos/Prometheus client for health enrichment.
func (h *TopologyHandler) SetThanosClient(c *thanos.Client) {
	h.thanos = c
}

// GetTopology handles GET /api/v1/topology
func (h *TopologyHandler) GetTopology(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	includeHealth := r.URL.Query().Get("includeHealth") == "true"
	slog.DebugContext(r.Context(), "getting topology", "includeHealth", includeHealth)

	resp, err := h.buildTopology(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to build topology", "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to build topology", "TOPOLOGY_FAILED")
		return
	}

	if includeHealth && h.thanos != nil {
		h.enrichHealthData(r.Context(), resp)
	}

	WriteJSON(w, http.StatusOK, resp)
}

// topoEdgeID generates a deterministic edge ID.
func topoEdgeID(source, target, edgeType string) string {
	return fmt.Sprintf("%s-%s-%s", source, target, edgeType)
}

// buildTopology constructs the aggregated topology from VPC API and K8s CRDs
func (h *TopologyHandler) buildTopology(ctx context.Context) (*model.TopologyResponse, error) {
	nodes := []model.TopologyNode{}
	edges := []model.TopologyEdge{}

	// Fetch VPCs from VPC API — scope to cluster VPC when configured
	if h.defaultVPCID != "" {
		v, err := h.vpcClient.GetVPC(ctx, h.defaultVPCID)
		if err != nil {
			slog.ErrorContext(ctx, "failed to get cluster VPC for topology", "vpcID", h.defaultVPCID, "error", err)
		} else {
			nodes = append(nodes, model.TopologyNode{
				ID:     v.ID,
				Label:  v.Name,
				Type:   "vpc",
				Status: v.Status,
				Metadata: map[string]interface{}{
					"region": v.Region,
				},
			})
		}
	} else {
		vpcs, err := h.vpcClient.ListVPCs(ctx)
		if err != nil {
			slog.ErrorContext(ctx, "failed to list VPCs for topology", "error", err)
		} else {
			for _, v := range vpcs {
				nodes = append(nodes, model.TopologyNode{
					ID:     v.ID,
					Label:  v.Name,
					Type:   "vpc",
					Status: v.Status,
					Metadata: map[string]interface{}{
						"region": v.Region,
					},
				})
			}
		}
	}

	// Fetch security groups from VPC API
	sgs, err := h.vpcClient.ListSecurityGroups(ctx, h.defaultVPCID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to list security groups for topology", "error", err)
	} else {
		for _, sg := range sgs {
			nodes = append(nodes, model.TopologyNode{
				ID:    sg.ID,
				Label: sg.Name,
				Type:  "security-group",
				Metadata: map[string]interface{}{
					"vpc_id":      sg.VPCID,
					"description": sg.Description,
					"rule_count":  len(sg.Rules),
				},
			})
			edges = append(edges, model.TopologyEdge{
				ID:     topoEdgeID(sg.VPCID, sg.ID, "contains"),
				Source: sg.VPCID,
				Target: sg.ID,
				Type:   "contains",
			})
		}
	}

	// Fetch network ACLs from VPC API
	acls, err := h.vpcClient.ListNetworkACLs(ctx, h.defaultVPCID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to list network ACLs for topology", "error", err)
	} else {
		for _, acl := range acls {
			nodes = append(nodes, model.TopologyNode{
				ID:    acl.ID,
				Label: acl.Name,
				Type:  "network-acl",
				Metadata: map[string]interface{}{
					"vpc_id":     acl.VPCID,
					"rule_count": len(acl.Rules),
				},
			})
			edges = append(edges, model.TopologyEdge{
				ID:     topoEdgeID(acl.VPCID, acl.ID, "contains"),
				Source: acl.VPCID,
				Target: acl.ID,
				Type:   "contains",
			})
		}
	}

	// Fetch CRD data from K8s API
	if h.dynamicClient != nil {
		h.addCRDNodes(ctx, &nodes, &edges)
	}

	return &model.TopologyResponse{
		Nodes: nodes,
		Edges: edges,
	}, nil
}

// addCRDNodes fetches Kubernetes CRDs and adds them as topology nodes/edges.
func (h *TopologyHandler) addCRDNodes(ctx context.Context, nodes *[]model.TopologyNode, edges *[]model.TopologyEdge) {
	// VPCSubnets
	subnets, err := h.dynamicClient.Resource(vpcSubnetGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		slog.ErrorContext(ctx, "failed to list VPCSubnets for topology", "error", err)
	} else {
		for _, item := range subnets.Items {
			nodeID := fmt.Sprintf("vsn-%s", item.GetName())
			vpcID := topoNestedStr(item, "spec", "vpcID")
			zone := topoNestedStr(item, "spec", "zone")
			cidr := topoNestedStr(item, "spec", "ipv4CIDRBlock")
			subnetID := topoNestedStr(item, "status", "subnetID")
			syncStatus := topoNestedStr(item, "status", "syncStatus")

			*nodes = append(*nodes, model.TopologyNode{
				ID:     nodeID,
				Label:  item.GetName(),
				Type:   "subnet",
				Status: syncToStatus(syncStatus),
				Metadata: map[string]interface{}{
					"subnet_id": subnetID,
					"vpc_id":    vpcID,
					"zone":      zone,
					"cidr":      cidr,
					"namespace": item.GetNamespace(),
				},
			})

			if vpcID != "" {
				*edges = append(*edges, model.TopologyEdge{
					ID:     topoEdgeID(vpcID, nodeID, "contains"),
					Source: vpcID,
					Target: nodeID,
					Type:   "contains",
				})
			}
		}
	}

	// VirtualNetworkInterfaces
	vnis, err := h.dynamicClient.Resource(vniCRDGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		slog.ErrorContext(ctx, "failed to list VNIs for topology", "error", err)
	} else {
		for _, item := range vnis.Items {
			nodeID := fmt.Sprintf("vni-%s-%s", item.GetNamespace(), item.GetName())
			vniID := topoNestedStr(item, "status", "vniID")
			subnetID := topoNestedStr(item, "spec", "subnetID")
			syncStatus := topoNestedStr(item, "status", "syncStatus")

			*nodes = append(*nodes, model.TopologyNode{
				ID:     nodeID,
				Label:  item.GetName(),
				Type:   "vni",
				Status: syncToStatus(syncStatus),
				Metadata: map[string]interface{}{
					"vni_id":    vniID,
					"subnet_id": subnetID,
					"namespace": item.GetNamespace(),
				},
			})

			// Edge to matching VPCSubnet node by subnet_id
			if subnetID != "" {
				for _, s := range *nodes {
					if s.Type == "subnet" && s.Metadata != nil && s.Metadata["subnet_id"] == subnetID {
						*edges = append(*edges, model.TopologyEdge{
							ID:     topoEdgeID(s.ID, nodeID, "connected"),
							Source: s.ID,
							Target: nodeID,
							Type:   "connected",
						})
						break
					}
				}
			}
		}
	}

	// FloatingIPs
	fips, err := h.dynamicClient.Resource(floatingIPGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		slog.ErrorContext(ctx, "failed to list FloatingIPs for topology", "error", err)
	} else {
		for _, item := range fips.Items {
			nodeID := fmt.Sprintf("fip-%s-%s", item.GetNamespace(), item.GetName())
			fipID := topoNestedStr(item, "status", "floatingIPID")
			targetVNIID := topoNestedStr(item, "spec", "vniID")
			syncStatus := topoNestedStr(item, "status", "syncStatus")

			*nodes = append(*nodes, model.TopologyNode{
				ID:     nodeID,
				Label:  item.GetName(),
				Type:   "floating-ip",
				Status: syncToStatus(syncStatus),
				Metadata: map[string]interface{}{
					"fip_id":    fipID,
					"target":    targetVNIID,
					"namespace": item.GetNamespace(),
				},
			})

			// Edge to VNI target
			if targetVNIID != "" {
				for _, n := range *nodes {
					if n.Type == "vni" && n.Metadata != nil && n.Metadata["vni_id"] == targetVNIID {
						*edges = append(*edges, model.TopologyEdge{
							ID:     topoEdgeID(nodeID, n.ID, "targets"),
							Source: nodeID,
							Target: n.ID,
							Type:   "targets",
						})
						break
					}
				}
			}
		}
	}

	// CUDNs — cluster-wide network definitions
	cudns, err := h.dynamicClient.Resource(cudnGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		slog.ErrorContext(ctx, "failed to list CUDNs for topology", "error", err)
	} else {
		for _, item := range cudns.Items {
			nodeID := fmt.Sprintf("cudn-%s", item.GetName())
			topology, _, _ := unstructured.NestedString(item.Object, "spec", "topology")
			annots := item.GetAnnotations()
			subnetID := ""
			if annots != nil {
				subnetID = annots["vpc.roks.ibm.com/subnet-id"]
			}

			*nodes = append(*nodes, model.TopologyNode{
				ID:    nodeID,
				Label: item.GetName(),
				Type:  "subnet",
				Metadata: map[string]interface{}{
					"resource_type": "cudn",
					"topology":      topology,
					"subnet_id":     subnetID,
				},
			})

			if subnetID != "" {
				for _, s := range *nodes {
					if s.Type == "subnet" && s.Metadata != nil && s.Metadata["subnet_id"] == subnetID && s.ID != nodeID {
						*edges = append(*edges, model.TopologyEdge{
							ID:     topoEdgeID(nodeID, s.ID, "associates"),
							Source: nodeID,
							Target: s.ID,
							Type:   "associates",
						})
						break
					}
				}
			}
		}
	}

	// UDNs — namespace-scoped network definitions
	udns, err := h.dynamicClient.Resource(udnGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		slog.ErrorContext(ctx, "failed to list UDNs for topology", "error", err)
	} else {
		for _, item := range udns.Items {
			nodeID := fmt.Sprintf("udn-%s-%s", item.GetNamespace(), item.GetName())
			topology, _, _ := unstructured.NestedString(item.Object, "spec", "topology")
			annots := item.GetAnnotations()
			subnetID := ""
			if annots != nil {
				subnetID = annots["vpc.roks.ibm.com/subnet-id"]
			}

			*nodes = append(*nodes, model.TopologyNode{
				ID:    nodeID,
				Label: item.GetName(),
				Type:  "subnet",
				Metadata: map[string]interface{}{
					"resource_type": "udn",
					"topology":      topology,
					"subnet_id":     subnetID,
					"namespace":     item.GetNamespace(),
				},
			})

			if subnetID != "" {
				for _, s := range *nodes {
					if s.Type == "subnet" && s.Metadata != nil && s.Metadata["subnet_id"] == subnetID && s.ID != nodeID {
						*edges = append(*edges, model.TopologyEdge{
							ID:     topoEdgeID(nodeID, s.ID, "associates"),
							Source: nodeID,
							Target: s.ID,
							Type:   "associates",
						})
						break
					}
				}
			}
		}
	}

	// VPCGateways
	gateways, err := h.dynamicClient.Resource(vpcGatewayGVR).Namespace("").List(ctx, metav1.ListOptions{})
	if err != nil {
		slog.ErrorContext(ctx, "failed to list VPCGateways for topology", "error", err)
	} else {
		for _, item := range gateways.Items {
			nodeID := fmt.Sprintf("gw-%s-%s", item.GetNamespace(), item.GetName())
			phase := topoNestedStr(item, "status", "phase")
			floatingIP := topoNestedStr(item, "status", "floatingIP")
			uplinkNetwork := topoNestedStr(item, "spec", "uplinkNetworkRef", "name")
			transitNetwork := topoNestedStr(item, "spec", "transitNetworkRef", "name")

			*nodes = append(*nodes, model.TopologyNode{
				ID:     nodeID,
				Label:  item.GetName(),
				Type:   "gateway",
				Status: phaseToStatus(phase),
				Metadata: map[string]interface{}{
					"namespace":      item.GetNamespace(),
					"phase":          phase,
					"floatingIP":     floatingIP,
					"uplinkNetwork":  uplinkNetwork,
					"transitNetwork": transitNetwork,
				},
			})
		}
	}

	// VPCRouters
	routers, err := h.dynamicClient.Resource(vpcRouterGVR).Namespace("").List(ctx, metav1.ListOptions{})
	if err != nil {
		slog.ErrorContext(ctx, "failed to list VPCRouters for topology", "error", err)
	} else {
		for _, item := range routers.Items {
			nodeID := fmt.Sprintf("rtr-%s-%s", item.GetNamespace(), item.GetName())
			phase := topoNestedStr(item, "status", "phase")
			gateway := topoNestedStr(item, "spec", "gatewayRef", "name")
			mode := topoNestedStr(item, "spec", "mode")
			podIP := topoNestedStr(item, "status", "podIP")

			*nodes = append(*nodes, model.TopologyNode{
				ID:     nodeID,
				Label:  item.GetName(),
				Type:   "router",
				Status: phaseToStatus(phase),
				Metadata: map[string]interface{}{
					"namespace": item.GetNamespace(),
					"phase":     phase,
					"gateway":   gateway,
					"mode":      mode,
					"podIP":     podIP,
				},
			})

			// Edge from router to its gateway
			if gateway != "" {
				gwNodeID := fmt.Sprintf("gw-%s-%s", item.GetNamespace(), gateway)
				*edges = append(*edges, model.TopologyEdge{
					ID:     topoEdgeID(nodeID, gwNodeID, "connected"),
					Source: nodeID,
					Target: gwNodeID,
					Type:   "connected",
				})
			}

			// Edges from router to its connected subnets/networks
			networks, _, _ := unstructured.NestedSlice(item.Object, "spec", "networks")
			for _, netRaw := range networks {
				netMap, ok := netRaw.(map[string]interface{})
				if !ok {
					continue
				}
				netName, _ := netMap["name"].(string)
				if netName == "" {
					continue
				}
				// Find matching subnet node by label
				for _, n := range *nodes {
					if n.Type == "subnet" && n.Label == netName {
						*edges = append(*edges, model.TopologyEdge{
							ID:     topoEdgeID(nodeID, n.ID, "connected"),
							Source: nodeID,
							Target: n.ID,
							Type:   "connected",
						})
						break
					}
				}
			}
		}
	}
}

// phaseToStatus converts a CRD phase to a topology status string.
func phaseToStatus(phase string) string {
	switch phase {
	case "Ready", "Running":
		return "available"
	case "Failed", "Error":
		return "error"
	default:
		return "pending"
	}
}

// enrichHealthData queries Thanos for health metrics and attaches them to topology nodes and edges.
func (h *TopologyHandler) enrichHealthData(ctx context.Context, resp *model.TopologyResponse) {
	// Build a map of node index by ID for quick lookups
	nodeIndex := make(map[string]int, len(resp.Nodes))
	for i := range resp.Nodes {
		nodeIndex[resp.Nodes[i].ID] = i
	}

	// Enrich router nodes with health metrics
	for i := range resp.Nodes {
		node := &resp.Nodes[i]
		switch node.Type {
		case "router":
			h.enrichRouterHealth(ctx, node)
		case "subnet":
			h.enrichSubnetHealth(ctx, node)
		}
	}

	// Enrich edges between routers and subnets with throughput data
	for i := range resp.Edges {
		edge := &resp.Edges[i]
		if edge.Type != "connected" {
			continue
		}
		// Check if source is a router node
		srcIdx, srcOK := nodeIndex[edge.Source]
		tgtIdx, tgtOK := nodeIndex[edge.Target]
		if !srcOK || !tgtOK {
			continue
		}
		srcNode := resp.Nodes[srcIdx]
		tgtNode := resp.Nodes[tgtIdx]
		if srcNode.Type == "router" && tgtNode.Type == "subnet" {
			h.enrichEdgeThroughput(ctx, edge, srcNode, tgtNode)
		}
	}
}

// enrichRouterHealth queries Thanos for router-specific health metrics.
func (h *TopologyHandler) enrichRouterHealth(ctx context.Context, node *model.TopologyNode) {
	routerName := node.Label
	podSelector := fmt.Sprintf(`{pod=~"%s-pod"}`, routerName)

	metrics := make(map[string]float64)
	status := "healthy"

	// Conntrack utilization
	ctCountResult, err := h.thanos.QueryInstant(ctx, fmt.Sprintf(`router_conntrack_entries%s`, podSelector))
	if err != nil {
		slog.DebugContext(ctx, "failed to query conntrack entries for topology health", "router", routerName, "error", err)
	}
	ctMaxResult, err := h.thanos.QueryInstant(ctx, fmt.Sprintf(`router_conntrack_max%s`, podSelector))
	if err != nil {
		slog.DebugContext(ctx, "failed to query conntrack max for topology health", "router", routerName, "error", err)
	}
	ctCount := extractFirstValue(ctCountResult)
	ctMax := extractFirstValue(ctMaxResult)
	if ctMax > 0 {
		ctPct := (ctCount / ctMax) * 100
		metrics["conntrackPct"] = ctPct
		if ctPct > 95 {
			status = "critical"
		} else if ctPct > 80 {
			status = "warning"
		}
	}

	// Error rate (rx errors across all interfaces)
	errResult, err := h.thanos.QueryInstant(ctx,
		fmt.Sprintf(`sum(rate(router_interface_rx_errors_total%s[5m]))`, podSelector))
	if err != nil {
		slog.DebugContext(ctx, "failed to query error rate for topology health", "router", routerName, "error", err)
	}
	errorRate := extractFirstValue(errResult)
	metrics["errorRate"] = errorRate
	if errorRate > 0 && status == "healthy" {
		status = "warning"
	}

	// Throughput (sum of all interface rx+tx bytes/sec)
	throughputResult, err := h.thanos.QueryInstant(ctx,
		fmt.Sprintf(`sum(rate(router_interface_rx_bytes_total%s[5m])) + sum(rate(router_interface_tx_bytes_total%s[5m]))`, podSelector, podSelector))
	if err != nil {
		slog.DebugContext(ctx, "failed to query throughput for topology health", "router", routerName, "error", err)
	}
	throughput := extractFirstValue(throughputResult)
	metrics["throughputBps"] = throughput

	// Process health — check if critical processes are down
	processResult, err := h.thanos.QueryInstant(ctx, fmt.Sprintf(`router_process_running%s`, podSelector))
	if err != nil {
		slog.DebugContext(ctx, "failed to query process status for topology health", "router", routerName, "error", err)
	}
	if processResult != nil {
		for _, ds := range processResult.Data.Result {
			val := parseValueString(ds.Value[1])
			if val == 0 {
				// A process is down — critical
				status = "critical"
				break
			}
		}
	}

	node.Health = &model.NodeHealth{
		Status:  status,
		Metrics: metrics,
	}
}

// enrichSubnetHealth computes IP utilization for subnet nodes from DHCP metrics if available.
func (h *TopologyHandler) enrichSubnetHealth(ctx context.Context, node *model.TopologyNode) {
	if node.Metadata == nil {
		return
	}

	// Only enrich subnets that have a name (used as interface label in DHCP metrics)
	subnetName := node.Label
	if subnetName == "" {
		return
	}

	// Query DHCP pool utilization for this subnet
	leaseResult, err := h.thanos.QueryInstant(ctx,
		fmt.Sprintf(`router_dhcp_active_leases{interface=%q}`, subnetName))
	if err != nil {
		slog.DebugContext(ctx, "failed to query DHCP leases for subnet health", "subnet", subnetName, "error", err)
		return
	}
	poolResult, err := h.thanos.QueryInstant(ctx,
		fmt.Sprintf(`router_dhcp_pool_size{interface=%q}`, subnetName))
	if err != nil {
		slog.DebugContext(ctx, "failed to query DHCP pool size for subnet health", "subnet", subnetName, "error", err)
		return
	}

	activeLeases := extractFirstValue(leaseResult)
	poolSize := extractFirstValue(poolResult)

	// Only attach health if we got actual DHCP data
	if poolSize <= 0 {
		return
	}

	utilization := (activeLeases / poolSize) * 100
	status := "healthy"
	if utilization > 90 {
		status = "critical"
	} else if utilization > 75 {
		status = "warning"
	}

	node.Health = &model.NodeHealth{
		Status: status,
		Metrics: map[string]float64{
			"ipUtilization": utilization,
			"activeLeases":  activeLeases,
			"poolSize":      poolSize,
		},
	}
}

// enrichEdgeThroughput adds throughput metadata to edges between routers and subnets.
func (h *TopologyHandler) enrichEdgeThroughput(ctx context.Context, edge *model.TopologyEdge, routerNode, subnetNode model.TopologyNode) {
	routerName := routerNode.Label
	subnetName := subnetNode.Label
	podSelector := fmt.Sprintf(`{pod=~"%s-pod",interface=%q}`, routerName, subnetName)

	rxResult, err := h.thanos.QueryInstant(ctx,
		fmt.Sprintf(`rate(router_interface_rx_bytes_total%s[5m])`, podSelector))
	if err != nil {
		return
	}
	txResult, err := h.thanos.QueryInstant(ctx,
		fmt.Sprintf(`rate(router_interface_tx_bytes_total%s[5m])`, podSelector))
	if err != nil {
		return
	}

	rxBps := extractFirstValue(rxResult)
	txBps := extractFirstValue(txResult)

	if rxBps > 0 || txBps > 0 {
		edge.Metadata = map[string]interface{}{
			"throughputBps": rxBps + txBps,
			"rxBps":         rxBps,
			"txBps":         txBps,
		}
	}
}

// topoNestedStr safely extracts a nested string field from an unstructured object.
func topoNestedStr(item unstructured.Unstructured, fields ...string) string {
	val, _, _ := unstructured.NestedString(item.Object, fields...)
	return val
}

// syncToStatus converts a CRD syncStatus to a topology status.
func syncToStatus(syncStatus string) string {
	switch syncStatus {
	case "Synced":
		return "available"
	case "Failed":
		return "error"
	default:
		return "pending"
	}
}
