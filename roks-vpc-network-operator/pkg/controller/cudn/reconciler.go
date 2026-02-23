package cudn

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/IBM/roks-vpc-network-operator/pkg/annotations"
	"github.com/IBM/roks-vpc-network-operator/pkg/finalizers"
	operatormetrics "github.com/IBM/roks-vpc-network-operator/pkg/metrics"
	"github.com/IBM/roks-vpc-network-operator/pkg/vpc"
)

var cudnGVK = schema.GroupVersionKind{
	Group:   "k8s.ovn.org",
	Version: "v1",
	Kind:    "ClusterUserDefinedNetwork",
}

// Reconciler reconciles ClusterUserDefinedNetwork objects with LocalNet topology.
// See DESIGN.md §6.1 for the full specification.
type Reconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	VPC       vpc.Client
	ClusterID string
}

// Reconcile handles CUDN create/update/delete events.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling CUDN", "name", req.Name)

	// Fetch the ClusterUserDefinedNetwork via unstructured (avoids OVN type import)
	cudn := &unstructured.Unstructured{}
	cudn.SetGroupVersionKind(cudnGVK)
	if err := r.Get(ctx, req.NamespacedName, cudn); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Check topology — only handle LocalNet
	topology, _, _ := unstructured.NestedString(cudn.Object, "spec", "topology")
	if topology != "LocalNet" {
		return ctrl.Result{}, nil
	}

	// Handle deletion
	if !cudn.GetDeletionTimestamp().IsZero() {
		return r.reconcileDelete(ctx, cudn)
	}

	return r.reconcileNormal(ctx, cudn)
}

// reconcileNormal handles CUDN creation and updates.
func (r *Reconciler) reconcileNormal(ctx context.Context, cudn client.Object) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	annots := cudn.GetAnnotations()
	if annots == nil {
		annots = map[string]string{}
	}

	// Step 1: Validate required annotations
	for _, key := range annotations.RequiredCUDNAnnotations {
		if _, ok := annots[key]; !ok {
			logger.Error(nil, "Missing required annotation", "annotation", key)
			return ctrl.Result{RequeueAfter: 30 * time.Second}, fmt.Errorf("missing required annotation: %s", key)
		}
	}

	// Step 2: Add finalizer
	if err := finalizers.EnsureAdded(ctx, r.Client, cudn, finalizers.CUDNCleanup); err != nil {
		return ctrl.Result{}, err
	}

	// Step 3: Create VPC subnet (if not already created)
	if annots[annotations.SubnetID] == "" {
		subnetName := fmt.Sprintf("roks-%s-%s", r.ClusterID, cudn.GetName())
		subnet, err := r.VPC.CreateSubnet(ctx, vpc.CreateSubnetOptions{
			Name:            subnetName,
			VPCID:           annots[annotations.VPCID],
			Zone:            annots[annotations.Zone],
			CIDR:            annots[annotations.CIDR],
			ACLID:           annots[annotations.ACLID],
			ClusterID:       r.ClusterID,
			CUDNName:        cudn.GetName(),
		})
		if err != nil {
			logger.Error(err, "Failed to create VPC subnet")
			operatormetrics.ReconcileOpsTotal.WithLabelValues("cudn", "create_subnet", "error").Inc()
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
		operatormetrics.ReconcileOpsTotal.WithLabelValues("cudn", "create_subnet", "success").Inc()

		annots[annotations.SubnetID] = subnet.ID
		annots[annotations.SubnetStatus] = subnet.Status
		cudn.SetAnnotations(annots)
		if err := r.Update(ctx, cudn); err != nil {
			return ctrl.Result{}, err
		}
		logger.Info("Created VPC subnet", "subnetID", subnet.ID)
	}

	// Step 4: Create VLAN attachments on all bare metal nodes
	nodeList := &corev1.NodeList{}
	if err := r.List(ctx, nodeList); err != nil {
		logger.Error(err, "Failed to list nodes")
		return ctrl.Result{}, err
	}

	existingAttachments := parseAttachments(annots[annotations.VLANAttachments])
	vlanID, _ := strconv.ParseInt(annots[annotations.VLANID], 10, 64)

	for i := range nodeList.Items {
		node := &nodeList.Items[i]
		if !isBareMetalNode(node) {
			continue
		}
		if _, exists := existingAttachments[node.Name]; exists {
			continue
		}

		bmServerID := extractBMServerID(node.Spec.ProviderID)
		if bmServerID == "" {
			logger.Info("Skipping node without BM server ID", "node", node.Name)
			continue
		}

		att, err := r.VPC.CreateVLANAttachment(ctx, vpc.CreateVLANAttachmentOptions{
			BMServerID: bmServerID,
			Name:       fmt.Sprintf("roks-%s-vlan%d", cudn.GetName(), vlanID),
			VLANID:     vlanID,
			SubnetID:   annots[annotations.SubnetID],
		})
		if err != nil {
			logger.Error(err, "Failed to create VLAN attachment", "node", node.Name)
			operatormetrics.ReconcileOpsTotal.WithLabelValues("cudn", "create_vlan_attachment", "error").Inc()
			continue
		}
		operatormetrics.ReconcileOpsTotal.WithLabelValues("cudn", "create_vlan_attachment", "success").Inc()
		existingAttachments[node.Name] = att.ID
		logger.Info("Created VLAN attachment", "node", node.Name, "attachmentID", att.ID)
	}

	// Update annotations with current attachment mapping
	annots[annotations.VLANAttachments] = serializeAttachments(existingAttachments)
	cudn.SetAnnotations(annots)
	if err := r.Update(ctx, cudn); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// reconcileDelete handles CUDN deletion.
func (r *Reconciler) reconcileDelete(ctx context.Context, cudn client.Object) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	annots := cudn.GetAnnotations()
	if annots == nil {
		annots = map[string]string{}
	}

	// Step 1: Delete VLAN attachments
	if attachmentsStr, ok := annots[annotations.VLANAttachments]; ok && attachmentsStr != "" {
		attachments := parseAttachments(attachmentsStr)
		for nodeName, attachmentID := range attachments {
			bmServerID := r.resolveNodeBMServerID(ctx, nodeName)
			if bmServerID == "" {
				logger.Info("Could not resolve BM server ID for node, skipping VLAN delete", "node", nodeName)
				continue
			}
			if err := r.VPC.DeleteVLANAttachment(ctx, bmServerID, attachmentID); err != nil {
				logger.Error(err, "Failed to delete VLAN attachment", "attachmentID", attachmentID)
				return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
			}
			logger.Info("Deleted VLAN attachment", "node", nodeName, "attachmentID", attachmentID)
		}
	}

	// Step 2: Delete VPC subnet
	if subnetID, ok := annots[annotations.SubnetID]; ok && subnetID != "" {
		if err := r.VPC.DeleteSubnet(ctx, subnetID); err != nil {
			logger.Error(err, "Failed to delete VPC subnet", "subnetID", subnetID)
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
		logger.Info("Deleted VPC subnet", "subnetID", subnetID)
	}

	// Step 3: Remove finalizer
	if err := finalizers.EnsureRemoved(ctx, r.Client, cudn, finalizers.CUDNCleanup); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// resolveNodeBMServerID looks up a node by name and extracts its BM server ID.
func (r *Reconciler) resolveNodeBMServerID(ctx context.Context, nodeName string) string {
	node := &corev1.Node{}
	if err := r.Get(ctx, client.ObjectKey{Name: nodeName}, node); err != nil {
		return ""
	}
	return extractBMServerID(node.Spec.ProviderID)
}

// SetupWithManager registers the CUDN reconciler with the controller manager.
// Uses unstructured watch to avoid importing OVN-Kubernetes types.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(cudnGVK)
	return ctrl.NewControllerManagedBy(mgr).
		For(u).
		Complete(r)
}

// isBareMetalNode checks if a node is a bare metal worker.
func isBareMetalNode(node *corev1.Node) bool {
	instanceType := node.Labels["node.kubernetes.io/instance-type"]
	return strings.Contains(instanceType, "metal")
}

// extractBMServerID extracts the bare metal server ID from a node's provider ID.
// Expected format: "ibm://<account>/<region>/<zone>/<server-id>"
func extractBMServerID(providerID string) string {
	if providerID == "" {
		return ""
	}
	// Strip ibm:// prefix
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
