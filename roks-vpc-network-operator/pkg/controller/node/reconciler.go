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

var udnGVK = schema.GroupVersionKind{
	Group:   "k8s.ovn.org",
	Version: "v1",
	Kind:    "UserDefinedNetwork",
}

// Reconciler reconciles Node objects to ensure VLAN attachments exist
// for all LocalNet CUDNs and UDNs on every bare metal node.
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

// reconcileNormal ensures the node has VLAN attachments for all LocalNet CUDNs and UDNs.
func (r *Reconciler) reconcileNormal(ctx context.Context, node *corev1.Node) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	bmServerID := extractBMServerID(node.Spec.ProviderID)
	if bmServerID == "" {
		logger.Info("Could not extract BM server ID from provider ID", "providerID", node.Spec.ProviderID)
		return ctrl.Result{}, nil
	}

	// Collect all LocalNet network definitions (CUDNs + UDNs)
	localNetworks, err := r.listLocalNetNetworks(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	for _, netObj := range localNetworks {
		cudnAnnots := netObj.GetAnnotations()
		if cudnAnnots == nil {
			continue
		}

		subnetID := cudnAnnots[annotations.SubnetID]
		if subnetID == "" {
			continue
		}

		existingAttachments := parseAttachments(cudnAnnots[annotations.VLANAttachments])
		if _, exists := existingAttachments[node.Name]; exists {
			continue
		}

		vlanID, _ := strconv.ParseInt(cudnAnnots[annotations.VLANID], 10, 64)
		att, err := r.VPC.CreateVLANAttachment(ctx, vpc.CreateVLANAttachmentOptions{
			BMServerID: bmServerID,
			Name:       fmt.Sprintf("roks-%s-vlan%d", netObj.GetName(), vlanID),
			VLANID:     vlanID,
			SubnetID:   subnetID,
		})
		if err != nil {
			logger.Error(err, "Failed to create VLAN attachment", "node", node.Name, "network", netObj.GetName())
			operatormetrics.ReconcileOpsTotal.WithLabelValues("node", "create_vlan_attachment", "error").Inc()
			continue
		}
		operatormetrics.ReconcileOpsTotal.WithLabelValues("node", "create_vlan_attachment", "success").Inc()

		existingAttachments[node.Name] = att.ID
		cudnAnnots[annotations.VLANAttachments] = serializeAttachments(existingAttachments)
		netObj.SetAnnotations(cudnAnnots)
		if err := r.Update(ctx, netObj); err != nil {
			logger.Error(err, "Failed to update network annotations", "network", netObj.GetName())
			continue
		}

		logger.Info("Created VLAN attachment for node", "node", node.Name, "network", netObj.GetName(), "attachmentID", att.ID)
	}

	return ctrl.Result{}, nil
}

// listLocalNetNetworks returns all LocalNet CUDNs and UDNs as unstructured objects.
func (r *Reconciler) listLocalNetNetworks(ctx context.Context) ([]*unstructured.Unstructured, error) {
	logger := log.FromContext(ctx)
	var result []*unstructured.Unstructured

	// List CUDNs
	cudnList := &unstructured.UnstructuredList{}
	cudnList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   cudnGVK.Group,
		Version: cudnGVK.Version,
		Kind:    "ClusterUserDefinedNetworkList",
	})
	if err := r.List(ctx, cudnList); err != nil {
		logger.Error(err, "Failed to list CUDNs")
		return nil, err
	}

	for i := range cudnList.Items {
		cudn := &cudnList.Items[i]
		// Only include LocalNet CUDNs — Layer2 CUDNs don't need VLAN attachments
		topology, _, _ := unstructured.NestedString(cudn.Object, "spec", "topology")
		if topology == "LocalNet" {
			result = append(result, cudn)
		}
	}

	// List UDNs across all namespaces
	udnList := &unstructured.UnstructuredList{}
	udnList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   udnGVK.Group,
		Version: udnGVK.Version,
		Kind:    "UserDefinedNetworkList",
	})
	if err := r.List(ctx, udnList); err != nil {
		// UDN CRD may not be installed — log and continue with CUDNs only
		logger.V(1).Info("Failed to list UDNs (CRD may not be installed)", "error", err)
	} else {
		for i := range udnList.Items {
			udn := &udnList.Items[i]
			topology, _, _ := unstructured.NestedString(udn.Object, "spec", "topology")
			if topology == "LocalNet" {
				result = append(result, udn)
			}
		}
	}

	return result, nil
}

// reconcileDelete cleans up VLAN attachments when a node is removed.
func (r *Reconciler) reconcileDelete(ctx context.Context, nodeName string) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Cleaning up VLAN attachments for removed node", "node", nodeName)

	// Clean up from all network definitions (CUDNs + UDNs)
	allNetworks, err := r.listAllNetworks(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	for _, netObj := range allNetworks {
		cudnAnnots := netObj.GetAnnotations()
		if cudnAnnots == nil {
			continue
		}

		existingAttachments := parseAttachments(cudnAnnots[annotations.VLANAttachments])
		attachmentID, exists := existingAttachments[nodeName]
		if !exists {
			continue
		}

		logger.Info("Node removed, VLAN attachment orphaned — will be handled by GC",
			"node", nodeName, "attachmentID", attachmentID)

		delete(existingAttachments, nodeName)
		cudnAnnots[annotations.VLANAttachments] = serializeAttachments(existingAttachments)
		netObj.SetAnnotations(cudnAnnots)
		if err := r.Update(ctx, netObj); err != nil {
			logger.Error(err, "Failed to update network annotations during node cleanup", "network", netObj.GetName())
		}
	}

	return ctrl.Result{}, nil
}

// listAllNetworks returns all CUDNs and UDNs regardless of topology.
func (r *Reconciler) listAllNetworks(ctx context.Context) ([]*unstructured.Unstructured, error) {
	logger := log.FromContext(ctx)
	var result []*unstructured.Unstructured

	cudnList := &unstructured.UnstructuredList{}
	cudnList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   cudnGVK.Group,
		Version: cudnGVK.Version,
		Kind:    "ClusterUserDefinedNetworkList",
	})
	if err := r.List(ctx, cudnList); err != nil {
		logger.Error(err, "Failed to list CUDNs for node cleanup")
		return nil, err
	}
	for i := range cudnList.Items {
		result = append(result, &cudnList.Items[i])
	}

	udnList := &unstructured.UnstructuredList{}
	udnList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   udnGVK.Group,
		Version: udnGVK.Version,
		Kind:    "UserDefinedNetworkList",
	})
	if err := r.List(ctx, udnList); err != nil {
		logger.V(1).Info("Failed to list UDNs for node cleanup", "error", err)
	} else {
		for i := range udnList.Items {
			result = append(result, &udnList.Items[i])
		}
	}

	return result, nil
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
