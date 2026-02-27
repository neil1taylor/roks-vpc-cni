package handler

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/IBM/roks-vpc-network-operator/cmd/bff/internal/model"
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

// GetTopology handles GET /api/v1/topology
func (h *TopologyHandler) GetTopology(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	slog.DebugContext(r.Context(), "getting topology")

	resp, err := h.buildTopology(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to build topology", "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to build topology", "TOPOLOGY_FAILED")
		return
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
