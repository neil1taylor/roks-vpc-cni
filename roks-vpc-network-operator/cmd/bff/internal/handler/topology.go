package handler

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/IBM/roks-vpc-network-operator/cmd/bff/internal/model"
	"github.com/IBM/roks-vpc-network-operator/pkg/vpc"

	"k8s.io/client-go/kubernetes"
)

// TopologyHandler handles topology aggregation
type TopologyHandler struct {
	vpcClient vpc.ExtendedClient
	k8sClient kubernetes.Interface
}

// NewTopologyHandler creates a new topology handler
func NewTopologyHandler(vpcClient vpc.ExtendedClient, k8sClient kubernetes.Interface) *TopologyHandler {
	return &TopologyHandler{
		vpcClient: vpcClient,
		k8sClient: k8sClient,
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

// buildTopology constructs the aggregated topology from VPC API and K8s CRDs
func (h *TopologyHandler) buildTopology(ctx context.Context) (*model.TopologyResponse, error) {
	nodes := []model.TopologyNode{}
	edges := []model.TopologyEdge{}

	// Fetch VPCs from VPC API
	vpcs, err := h.vpcClient.ListVPCs(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to list VPCs for topology", "error", err)
		// Continue with partial data
	} else {
		for _, v := range vpcs {
			nodes = append(nodes, model.TopologyNode{
				ID:   v.ID,
				Type: "vpc",
				Data: model.VPCNodeData{
					Name:   v.Name,
					Region: v.Region,
					Status: v.Status,
				},
			})
		}
	}

	// Fetch security groups from VPC API
	sgs, err := h.vpcClient.ListSecurityGroups(ctx, "")
	if err != nil {
		slog.ErrorContext(ctx, "failed to list security groups for topology", "error", err)
	} else {
		for _, sg := range sgs {
			nodes = append(nodes, model.TopologyNode{
				ID:   sg.ID,
				Type: "sg",
				Data: model.SecurityGroupNodeData{
					Name:        sg.Name,
					VPCID:       sg.VPCID,
					Description: sg.Description,
					RuleCount:   len(sg.Rules),
				},
			})

			// Add edge: VPC contains SG
			edges = append(edges, model.TopologyEdge{
				Source: sg.VPCID,
				Target: sg.ID,
				Type:   "contains",
			})
		}
	}

	// Fetch network ACLs from VPC API
	acls, err := h.vpcClient.ListNetworkACLs(ctx, "")
	if err != nil {
		slog.ErrorContext(ctx, "failed to list network ACLs for topology", "error", err)
	} else {
		for _, acl := range acls {
			nodes = append(nodes, model.TopologyNode{
				ID:   acl.ID,
				Type: "acl",
				Data: model.ACLNodeData{
					Name:      acl.Name,
					VPCID:     acl.VPCID,
					RuleCount: len(acl.Rules),
				},
			})

			// Add edge: VPC contains ACL
			edges = append(edges, model.TopologyEdge{
				Source: acl.VPCID,
				Target: acl.ID,
				Type:   "contains",
			})
		}
	}

	// TODO: Fetch CRD data (Subnets, VNIs, VMs) from K8s API
	// This would involve querying custom resources using the K8s client

	return &model.TopologyResponse{
		Nodes: nodes,
		Edges: edges,
	}, nil
}
