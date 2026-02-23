package node

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/IBM/roks-vpc-network-operator/pkg/annotations"
	operatormetrics "github.com/IBM/roks-vpc-network-operator/pkg/metrics"
	"github.com/IBM/roks-vpc-network-operator/pkg/vpc"
)

var cudnGVK = schema.GroupVersionKind{
	Group:   "k8s.ovn.org",
	Version: "v1",
	Kind:    "ClusterUserDefinedNetwork",
}

// Reconciler reconciles Node objects to ensure VLAN attachments exist
// for all LocalNet CUDNs on every bare metal node.
// See DESIGN.md §6.2 for the full specification.
type Reconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	VPC       vpc.Client
	ClusterID string
}

// Reconcile handles Node join and removal events.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling Node", "name", req.Name)

	// Fetch the Node
	node := &corev1.Node{}
	if err := r.Get(ctx, req.NamespacedName, node); err != nil {
		if client.IgnoreNotFound(err) == nil {
			// Node was deleted — clean up VLAN attachments
			return r.reconcileDelete(ctx, req.Name)
		}
		return ctrl.Result{}, err
	}

	// Filter for bare metal nodes only
	if !isBareMetalNode(node) {
		return ctrl.Result{}, nil
	}

	// Check if node is Ready
	if !isNodeReady(node) {
		logger.Info("Node is not Ready, will retry", "node", node.Name)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	return r.reconcileNormal(ctx, node)
}

// reconcileNormal ensures the node has VLAN attachments for all LocalNet CUDNs.
func (r *Reconciler) reconcileNormal(ctx context.Context, node *corev1.Node) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	bmServerID := extractBMServerID(node.Spec.ProviderID)
	if bmServerID == "" {
		logger.Info("Could not extract BM server ID from provider ID", "providerID", node.Spec.ProviderID)
		return ctrl.Result{}, nil
	}

	// List all CUDNs
	cudnList := &unstructured.UnstructuredList{}
	cudnList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   cudnGVK.Group,
		Version: cudnGVK.Version,
		Kind:    "ClusterUserDefinedNetworkList",
	})
	if err := r.List(ctx, cudnList); err != nil {
		logger.Error(err, "Failed to list CUDNs")
		return ctrl.Result{}, err
	}

	for i := range cudnList.Items {
		cudn := &cudnList.Items[i]

		// Only handle LocalNet CUDNs
		topology, _, _ := unstructured.NestedString(cudn.Object, "spec", "topology")
		if topology != "LocalNet" {
			continue
		}

		cudnAnnots := cudn.GetAnnotations()
		if cudnAnnots == nil {
			continue
		}

		// Check if subnet is ready
		subnetID := cudnAnnots[annotations.SubnetID]
		if subnetID == "" {
			continue
		}

		// Check if this node already has a VLAN attachment
		existingAttachments := parseAttachments(cudnAnnots[annotations.VLANAttachments])
		if _, exists := existingAttachments[node.Name]; exists {
			continue
		}

		// Create VLAN attachment
		vlanID, _ := strconv.ParseInt(cudnAnnots[annotations.VLANID], 10, 64)
		att, err := r.VPC.CreateVLANAttachment(ctx, vpc.CreateVLANAttachmentOptions{
			BMServerID: bmServerID,
			Name:       fmt.Sprintf("roks-%s-vlan%d", cudn.GetName(), vlanID),
			VLANID:     vlanID,
			SubnetID:   subnetID,
		})
		if err != nil {
			logger.Error(err, "Failed to create VLAN attachment", "node", node.Name, "cudn", cudn.GetName())
			operatormetrics.ReconcileOpsTotal.WithLabelValues("node", "create_vlan_attachment", "error").Inc()
			continue
		}
		operatormetrics.ReconcileOpsTotal.WithLabelValues("node", "create_vlan_attachment", "success").Inc()

		// Update CUDN annotation
		existingAttachments[node.Name] = att.ID
		cudnAnnots[annotations.VLANAttachments] = serializeAttachments(existingAttachments)
		cudn.SetAnnotations(cudnAnnots)
		if err := r.Update(ctx, cudn); err != nil {
			logger.Error(err, "Failed to update CUDN annotations", "cudn", cudn.GetName())
			continue
		}

		logger.Info("Created VLAN attachment for node", "node", node.Name, "cudn", cudn.GetName(), "attachmentID", att.ID)
	}

	return ctrl.Result{}, nil
}

// reconcileDelete cleans up VLAN attachments when a node is removed.
func (r *Reconciler) reconcileDelete(ctx context.Context, nodeName string) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Cleaning up VLAN attachments for removed node", "node", nodeName)

	// List all CUDNs
	cudnList := &unstructured.UnstructuredList{}
	cudnList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   cudnGVK.Group,
		Version: cudnGVK.Version,
		Kind:    "ClusterUserDefinedNetworkList",
	})
	if err := r.List(ctx, cudnList); err != nil {
		logger.Error(err, "Failed to list CUDNs for node cleanup")
		return ctrl.Result{}, err
	}

	for i := range cudnList.Items {
		cudn := &cudnList.Items[i]
		cudnAnnots := cudn.GetAnnotations()
		if cudnAnnots == nil {
			continue
		}

		existingAttachments := parseAttachments(cudnAnnots[annotations.VLANAttachments])
		attachmentID, exists := existingAttachments[nodeName]
		if !exists {
			continue
		}

		// We can't resolve the BM server ID since the node is deleted.
		// The VPC API may still allow deletion by attachment ID if we can
		// find the BM server ID from the attachment annotation or VPC API.
		// For now, we'll try to look up existing attachments.
		logger.Info("Node removed, VLAN attachment orphaned — will be handled by GC",
			"node", nodeName, "attachmentID", attachmentID)

		// Remove from CUDN annotation
		delete(existingAttachments, nodeName)
		cudnAnnots[annotations.VLANAttachments] = serializeAttachments(existingAttachments)
		cudn.SetAnnotations(cudnAnnots)
		if err := r.Update(ctx, cudn); err != nil {
			logger.Error(err, "Failed to update CUDN annotations during node cleanup", "cudn", cudn.GetName())
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager registers the Node reconciler with the controller manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Node{}).
		Complete(r)
}

// isBareMetalNode checks if a node is a bare metal worker.
func isBareMetalNode(node *corev1.Node) bool {
	instanceType := node.Labels["node.kubernetes.io/instance-type"]
	return strings.Contains(instanceType, "metal")
}

// isNodeReady checks if a node has the Ready condition.
func isNodeReady(node *corev1.Node) bool {
	for _, cond := range node.Status.Conditions {
		if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

// extractBMServerID extracts the bare metal server ID from a node's provider ID.
// Expected format: "ibm://<account>/<region>/<zone>/<server-id>"
func extractBMServerID(providerID string) string {
	if providerID == "" {
		return ""
	}
	trimmed := strings.TrimPrefix(providerID, "ibm://")
	parts := strings.Split(trimmed, "/")
	if len(parts) < 4 {
		return ""
	}
	return parts[3]
}

// parseAttachments parses "node1:att-id-1,node2:att-id-2" into a map.
func parseAttachments(s string) map[string]string {
	result := map[string]string{}
	if s == "" {
		return result
	}
	for _, entry := range strings.Split(s, ",") {
		parts := strings.SplitN(entry, ":", 2)
		if len(parts) == 2 {
			result[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return result
}

// serializeAttachments converts a map to "node1:att-id-1,node2:att-id-2".
func serializeAttachments(m map[string]string) string {
	var entries []string
	for node, attID := range m {
		entries = append(entries, fmt.Sprintf("%s:%s", node, attID))
	}
	return strings.Join(entries, ",")
}
