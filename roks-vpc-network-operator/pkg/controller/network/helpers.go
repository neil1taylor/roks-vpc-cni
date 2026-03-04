package network

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	apiv1alpha1 "github.com/IBM/roks-vpc-network-operator/api/v1alpha1"
	"github.com/IBM/roks-vpc-network-operator/pkg/annotations"
	operatormetrics "github.com/IBM/roks-vpc-network-operator/pkg/metrics"
	"github.com/IBM/roks-vpc-network-operator/pkg/vpc"
)

// EnsureVPCSubnet creates a VPC subnet for the network if one doesn't already exist.
// Returns true if a new subnet was created (annotations were updated).
func EnsureVPCSubnet(ctx context.Context, k8sClient client.Client, vpcClient vpc.Client, obj client.Object, clusterID, metricsLabel string) (bool, error) {
	logger := log.FromContext(ctx)
	annots := obj.GetAnnotations()
	if annots == nil {
		annots = map[string]string{}
	}

	if annots[annotations.SubnetID] != "" {
		// Backfill subnet-name for CUDNs created before this annotation existed
		if annots[annotations.SubnetName] == "" {
			subnetName := TruncateVPCName(fmt.Sprintf("roks-%s-%s", clusterID, obj.GetName()))
			if obj.GetNamespace() != "" {
				subnetName = TruncateVPCName(fmt.Sprintf("roks-%s-%s-%s", clusterID, obj.GetNamespace(), obj.GetName()))
			}
			annots[annotations.SubnetName] = subnetName
			obj.SetAnnotations(annots)
			if err := k8sClient.Update(ctx, obj); err != nil {
				logger.Error(err, "Failed to backfill subnet-name annotation")
			} else {
				logger.Info("Backfilled subnet-name annotation", "subnetName", subnetName)
			}
		}
		return false, nil
	}

	// Clear any previous error annotation on retry
	delete(annots, annotations.SubnetError)

	// Validate CIDR against VPC address prefixes before calling CreateSubnet.
	// If no matching prefix exists, auto-create one.
	vpcID := annots[annotations.VPCID]
	cidr := strings.TrimSpace(annots[annotations.CIDR])
	zone := annots[annotations.Zone]
	if vpcID != "" && cidr != "" {
		prefixes, err := vpcClient.ListVPCAddressPrefixes(ctx, vpcID)
		if err != nil {
			logger.Error(err, "Failed to list VPC address prefixes for CIDR validation")
			// Non-fatal: proceed to CreateSubnet and let VPC API validate
		} else if !cidrFitsPrefix(cidr, prefixes) {
			logger.Info("CIDR does not fit any existing VPC address prefix, auto-creating prefix", "cidr", cidr, "zone", zone)
			prefixName := TruncateVPCName(fmt.Sprintf("roks-%s-%s", clusterID, obj.GetName()))
			_, prefixErr := vpcClient.CreateVPCAddressPrefix(ctx, vpc.CreateAddressPrefixOptions{
				VPCID: vpcID,
				CIDR:  cidr,
				Zone:  zone,
				Name:  prefixName,
			})
			if prefixErr != nil {
				errMsg := fmt.Sprintf("CIDR %s does not fit any VPC address prefix and auto-creation failed: %v. Available prefixes: %s",
					cidr, prefixErr, formatPrefixes(prefixes))
				logger.Error(prefixErr, "Failed to auto-create VPC address prefix", "cidr", cidr)
				annots[annotations.SubnetStatus] = "error"
				annots[annotations.SubnetError] = errMsg
				obj.SetAnnotations(annots)
				if updateErr := k8sClient.Update(ctx, obj); updateErr != nil {
					logger.Error(updateErr, "Failed to update error annotation")
				}
				operatormetrics.ReconcileOpsTotal.WithLabelValues(metricsLabel, "create_subnet", "error").Inc()
				return false, fmt.Errorf("%s", errMsg)
			}
			logger.Info("Auto-created VPC address prefix", "cidr", cidr, "name", prefixName)
		}
	}

	subnetName := TruncateVPCName(fmt.Sprintf("roks-%s-%s", clusterID, obj.GetName()))
	if obj.GetNamespace() != "" {
		subnetName = TruncateVPCName(fmt.Sprintf("roks-%s-%s-%s", clusterID, obj.GetNamespace(), obj.GetName()))
	}

	// Idempotency: check if a subnet with this name already exists (e.g. from a
	// previous CUDN that was deleted without annotation cleanup, or a re-create).
	existing, listErr := vpcClient.ListSubnets(ctx, vpcID)
	if listErr == nil {
		for i := range existing {
			if existing[i].Name == subnetName {
				if existing[i].CIDR != cidr {
					errMsg := fmt.Sprintf("existing VPC subnet %s (%s) has CIDR %s but annotation specifies %s",
						subnetName, existing[i].ID, existing[i].CIDR, cidr)
					logger.Error(nil, "CIDR mismatch on subnet adoption", "subnetID", existing[i].ID, "existingCIDR", existing[i].CIDR, "expectedCIDR", cidr)
					annots[annotations.SubnetStatus] = "error"
					annots[annotations.SubnetError] = errMsg
					obj.SetAnnotations(annots)
					if updateErr := k8sClient.Update(ctx, obj); updateErr != nil {
						logger.Error(updateErr, "Failed to update error annotation")
					}
					operatormetrics.ReconcileOpsTotal.WithLabelValues(metricsLabel, "adopt_subnet", "error").Inc()
					return false, fmt.Errorf("%s", errMsg)
				}
				logger.Info("Found existing VPC subnet by name, adopting", "subnetID", existing[i].ID, "name", subnetName)
				annots[annotations.SubnetID] = existing[i].ID
				annots[annotations.SubnetName] = subnetName
				annots[annotations.SubnetStatus] = existing[i].Status
				obj.SetAnnotations(annots)
				if updateErr := k8sClient.Update(ctx, obj); updateErr != nil {
					return false, updateErr
				}
				operatormetrics.ReconcileOpsTotal.WithLabelValues(metricsLabel, "adopt_subnet", "success").Inc()
				return true, nil
			}
		}
	} else {
		logger.Error(listErr, "Failed to list subnets for idempotency check, proceeding with create")
	}

	subnet, err := vpcClient.CreateSubnet(ctx, vpc.CreateSubnetOptions{
		Name:            subnetName,
		VPCID:           annots[annotations.VPCID],
		Zone:            annots[annotations.Zone],
		CIDR:            cidr,
		ACLID:           annots[annotations.ACLID],
		PublicGatewayID: annots[annotations.PublicGatewayID],
		ClusterID:       clusterID,
		CUDNName:        obj.GetName(),
	})
	if err != nil {
		logger.Error(err, "Failed to create VPC subnet")
		operatormetrics.ReconcileOpsTotal.WithLabelValues(metricsLabel, "create_subnet", "error").Inc()
		return false, err
	}
	operatormetrics.ReconcileOpsTotal.WithLabelValues(metricsLabel, "create_subnet", "success").Inc()

	annots[annotations.SubnetID] = subnet.ID
	annots[annotations.SubnetName] = subnetName
	annots[annotations.SubnetStatus] = subnet.Status
	obj.SetAnnotations(annots)
	if err := k8sClient.Update(ctx, obj); err != nil {
		return false, err
	}
	logger.Info("Created VPC subnet", "subnetID", subnet.ID)
	return true, nil
}

// EnsureVLANAttachments creates VLAN attachments on all bare metal nodes for a LocalNet network.
// It also creates corresponding VLANAttachment CRs in operatorNamespace so the console UI can display them.
func EnsureVLANAttachments(ctx context.Context, k8sClient client.Client, vpcClient vpc.Client, obj client.Object, clusterID, metricsLabel string) error {
	return ensureVLANAttachments(ctx, k8sClient, vpcClient, obj, clusterID, metricsLabel, "roks-vpc-network-operator")
}

func ensureVLANAttachments(ctx context.Context, k8sClient client.Client, vpcClient vpc.Client, obj client.Object, clusterID, metricsLabel, operatorNamespace string) error {
	logger := log.FromContext(ctx)
	annots := obj.GetAnnotations()
	if annots == nil {
		annots = map[string]string{}
	}

	subnetID := annots[annotations.SubnetID]
	if subnetID == "" {
		return nil
	}

	nodeList := &corev1.NodeList{}
	if err := k8sClient.List(ctx, nodeList); err != nil {
		return fmt.Errorf("failed to list nodes: %w", err)
	}

	// Build a BM server name→ID lookup map from VPC API so we can resolve
	// nodes that lack a providerID (e.g. unmanaged bare metal clusters).
	bmServerMap := buildBMServerMap(ctx, vpcClient, annots[annotations.VPCID])

	existingAttachments := ParseAttachments(annots[annotations.VLANAttachments])
	vlanID, _ := strconv.ParseInt(annots[annotations.VLANID], 10, 64)

	for i := range nodeList.Items {
		node := &nodeList.Items[i]
		if _, exists := existingAttachments[node.Name]; exists {
			continue
		}

		// Try providerID first (ROKS clusters), then VPC API hostname lookup
		bmServerID := ExtractBMServerID(node.Spec.ProviderID)
		if bmServerID == "" {
			bmServerID = resolveBMServerIDByHostname(node.Name, bmServerMap)
		}
		if bmServerID == "" {
			// Not a bare metal node or not found in VPC — skip silently unless instance-type says metal
			if IsBareMetalNode(node) {
				logger.Info("Bare metal node has no providerID and not found in VPC BM servers", "node", node.Name)
			}
			continue
		}

		// Ensure the VLAN ID is in the PCI interface's allowed VLANs list
		if err := vpcClient.EnsurePCIAllowedVLAN(ctx, bmServerID, vlanID); err != nil {
			logger.Error(err, "Failed to ensure VLAN in PCI allowed list", "node", node.Name, "vlanID", vlanID)
			operatormetrics.ReconcileOpsTotal.WithLabelValues(metricsLabel, "ensure_pci_vlan", "error").Inc()
			continue
		}

		att, err := vpcClient.CreateVLANAttachment(ctx, vpc.CreateVLANAttachmentOptions{
			BMServerID: bmServerID,
			Name:       fmt.Sprintf("roks-%s-vlan%d", obj.GetName(), vlanID),
			VLANID:     vlanID,
			SubnetID:   subnetID,
		})
		if err != nil {
			logger.Error(err, "Failed to create VLAN attachment", "node", node.Name)
			operatormetrics.ReconcileOpsTotal.WithLabelValues(metricsLabel, "create_vlan_attachment", "error").Inc()
			continue
		}
		operatormetrics.ReconcileOpsTotal.WithLabelValues(metricsLabel, "create_vlan_attachment", "success").Inc()
		existingAttachments[node.Name] = att.ID
		logger.Info("Created VLAN attachment", "node", node.Name, "attachmentID", att.ID)

		// Create a VLANAttachment CR so the console UI can display it
		vlaCR := &apiv1alpha1.VLANAttachment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-%s-vlan%d", obj.GetName(), sanitizeName(node.Name), vlanID),
				Namespace: operatorNamespace,
				Labels: map[string]string{
					"vpc.roks.ibm.com/network": obj.GetName(),
					"vpc.roks.ibm.com/node":    sanitizeName(node.Name),
				},
			},
			Spec: apiv1alpha1.VLANAttachmentSpec{
				BMServerID:   bmServerID,
				VLANID:       vlanID,
				SubnetRef:    obj.GetName(),
				SubnetID:     subnetID,
				AllowToFloat: true,
				NodeName:     node.Name,
			},
		}
		if createErr := k8sClient.Create(ctx, vlaCR); createErr != nil {
			logger.Error(createErr, "Failed to create VLANAttachment CR (non-fatal)", "name", vlaCR.Name)
		} else {
			// Update status
			now := metav1.NewTime(time.Now())
			vlaCR.Status = apiv1alpha1.VLANAttachmentStatus{
				SyncStatus:       "Synced",
				AttachmentID:     att.ID,
				AttachmentStatus: "attached",
				LastSyncTime:     &now,
			}
			if statusErr := k8sClient.Status().Update(ctx, vlaCR); statusErr != nil {
				logger.Error(statusErr, "Failed to update VLANAttachment CR status (non-fatal)", "name", vlaCR.Name)
			}
		}
	}

	annots[annotations.VLANAttachments] = SerializeAttachments(existingAttachments)
	obj.SetAnnotations(annots)
	if err := k8sClient.Update(ctx, obj); err != nil {
		return err
	}

	// Ensure VLANAttachment CRs exist for all tracked attachments (idempotent)
	for nodeName, attID := range existingAttachments {
		crName := fmt.Sprintf("%s-%s-vlan%d", obj.GetName(), sanitizeName(nodeName), vlanID)
		existing := &apiv1alpha1.VLANAttachment{}
		if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: operatorNamespace, Name: crName}, existing); err == nil {
			continue // CR already exists
		}
		// Resolve BM server ID for this node
		bmID := ExtractBMServerID(resolveNodeProviderID(ctx, k8sClient, nodeName))
		if bmID == "" {
			bmID = resolveBMServerIDByHostname(nodeName, bmServerMap)
		}
		vlaCR := &apiv1alpha1.VLANAttachment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      crName,
				Namespace: operatorNamespace,
				Labels: map[string]string{
					"vpc.roks.ibm.com/network": obj.GetName(),
					"vpc.roks.ibm.com/node":    sanitizeName(nodeName),
				},
			},
			Spec: apiv1alpha1.VLANAttachmentSpec{
				BMServerID:   bmID,
				VLANID:       vlanID,
				SubnetRef:    obj.GetName(),
				SubnetID:     subnetID,
				AllowToFloat: true,
				NodeName:     nodeName,
			},
		}
		if createErr := k8sClient.Create(ctx, vlaCR); createErr != nil {
			logger.Error(createErr, "Failed to backfill VLANAttachment CR (non-fatal)", "name", crName)
			continue
		}
		now := metav1.NewTime(time.Now())
		vlaCR.Status = apiv1alpha1.VLANAttachmentStatus{
			SyncStatus:       "Synced",
			AttachmentID:     attID,
			AttachmentStatus: "attached",
			LastSyncTime:     &now,
		}
		if statusErr := k8sClient.Status().Update(ctx, vlaCR); statusErr != nil {
			logger.Error(statusErr, "Failed to update backfilled VLANAttachment CR status (non-fatal)", "name", crName)
		} else {
			logger.Info("Backfilled VLANAttachment CR", "name", crName, "attachmentID", attID)
		}
	}

	return nil
}

// SubnetHasActiveVNIs checks whether a VPC subnet has non-system reserved IPs,
// which indicates VNIs (from VMs) still exist on the subnet. The VPC API will
// reject subnet deletion if reserved IPs are present, so callers should check
// this before attempting to delete a network's VLAN attachments and subnet.
// Returns true and the count of active reserved IPs if any exist.
func SubnetHasActiveVNIs(ctx context.Context, vpcClient vpc.Client, subnetID string) (bool, int, error) {
	logger := log.FromContext(ctx)
	reservedIPs, err := vpcClient.ListSubnetReservedIPs(ctx, subnetID)
	if err != nil {
		return false, 0, fmt.Errorf("failed to list reserved IPs for subnet %s: %w", subnetID, err)
	}

	// Count non-system reserved IPs. Provider-owned IPs (gateway, broadcast)
	// are managed by VPC infrastructure and don't block subnet deletion.
	activeCount := 0
	for _, rip := range reservedIPs {
		if rip.Owner != "provider" {
			activeCount++
			logger.Info("Active reserved IP on subnet", "subnetID", subnetID, "ripID", rip.ID, "address", rip.Address, "target", rip.Target)
		}
	}
	return activeCount > 0, activeCount, nil
}

// DeleteVLANAttachments removes all VLAN attachments tracked in the object's annotations.
// Already-deleted attachments (404) are tolerated — this handles out-of-band deletion via IBM Cloud console.
func DeleteVLANAttachments(ctx context.Context, k8sClient client.Client, vpcClient vpc.Client, obj client.Object) error {
	logger := log.FromContext(ctx)
	annots := obj.GetAnnotations()
	if annots == nil {
		return nil
	}

	attachmentsStr := annots[annotations.VLANAttachments]
	if attachmentsStr == "" {
		return nil
	}

	// Build BM server map for hostname fallback resolution
	bmServerMap := buildBMServerMap(ctx, vpcClient, annots[annotations.VPCID])

	attachments := ParseAttachments(attachmentsStr)
	for nodeName, attachmentID := range attachments {
		bmServerID := resolveNodeBMServerID(ctx, k8sClient, nodeName)
		if bmServerID == "" {
			bmServerID = resolveBMServerIDByHostname(nodeName, bmServerMap)
		}
		if bmServerID == "" {
			logger.Info("Could not resolve BM server ID for node, skipping VLAN delete", "node", nodeName)
			continue
		}
		if err := vpcClient.DeleteVLANAttachment(ctx, bmServerID, attachmentID); err != nil {
			if isNotFoundError(err) {
				logger.Info("VLAN attachment already deleted (out-of-band)", "attachmentID", attachmentID, "node", nodeName)
				continue
			}
			logger.Error(err, "Failed to delete VLAN attachment", "attachmentID", attachmentID)
			return err
		}
		logger.Info("Deleted VLAN attachment", "node", nodeName, "attachmentID", attachmentID)
	}
	return nil
}

// DeleteVPCSubnet removes the VPC subnet tracked in the object's annotations.
// Already-deleted subnets (404) are tolerated — this handles out-of-band deletion via IBM Cloud console.
func DeleteVPCSubnet(ctx context.Context, vpcClient vpc.Client, obj client.Object) error {
	logger := log.FromContext(ctx)
	annots := obj.GetAnnotations()
	if annots == nil {
		return nil
	}

	subnetID := annots[annotations.SubnetID]
	if subnetID == "" {
		return nil
	}

	if err := vpcClient.DeleteSubnet(ctx, subnetID); err != nil {
		if isNotFoundError(err) {
			logger.Info("VPC subnet already deleted (out-of-band)", "subnetID", subnetID)
			return nil
		}
		logger.Error(err, "Failed to delete VPC subnet", "subnetID", subnetID)
		return err
	}
	logger.Info("Deleted VPC subnet", "subnetID", subnetID)
	return nil
}

// isNotFoundError checks if a VPC API error indicates the resource was not found.
func isNotFoundError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "not found")
}

// resolveNodeBMServerID looks up a node by name and extracts its BM server ID.
func resolveNodeBMServerID(ctx context.Context, k8sClient client.Client, nodeName string) string {
	node := &corev1.Node{}
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: nodeName}, node); err != nil {
		return ""
	}
	return ExtractBMServerID(node.Spec.ProviderID)
}

// resolveNodeProviderID looks up a node by name and returns its provider ID.
func resolveNodeProviderID(ctx context.Context, k8sClient client.Client, nodeName string) string {
	node := &corev1.Node{}
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: nodeName}, node); err != nil {
		return ""
	}
	return node.Spec.ProviderID
}

// IsBareMetalNode checks if a node is a bare metal worker.
func IsBareMetalNode(node *corev1.Node) bool {
	instanceType := node.Labels["node.kubernetes.io/instance-type"]
	return strings.Contains(instanceType, "metal")
}

// ExtractBMServerID extracts the bare metal server ID from a node's provider ID.
// Expected format: "ibm://<account>/<region>/<zone>/<server-id>"
func ExtractBMServerID(providerID string) string {
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

// ParseAttachments parses "node1:att-id-1,node2:att-id-2" into a map.
func ParseAttachments(s string) map[string]string {
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

// SerializeAttachments converts a map to "node1:att-id-1,node2:att-id-2".
func SerializeAttachments(m map[string]string) string {
	var entries []string
	for node, attID := range m {
		entries = append(entries, fmt.Sprintf("%s:%s", node, attID))
	}
	return strings.Join(entries, ",")
}

// cidrFitsPrefix checks if the given CIDR fits within any of the VPC address prefixes.
// A CIDR "fits" a prefix if the prefix network fully contains the CIDR network.
func cidrFitsPrefix(cidr string, prefixes []vpc.AddressPrefix) bool {
	_, subnetNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return false
	}
	for _, p := range prefixes {
		_, prefixNet, err := net.ParseCIDR(p.CIDR)
		if err != nil {
			continue
		}
		// The prefix must contain both the subnet's network address and
		// the prefix size must be equal or shorter (wider range).
		pOnes, _ := prefixNet.Mask.Size()
		sOnes, _ := subnetNet.Mask.Size()
		if prefixNet.Contains(subnetNet.IP) && pOnes <= sOnes {
			return true
		}
	}
	return false
}

// formatPrefixes formats address prefixes for display in error messages.
func formatPrefixes(prefixes []vpc.AddressPrefix) string {
	if len(prefixes) == 0 {
		return "(none)"
	}
	var parts []string
	for _, p := range prefixes {
		parts = append(parts, fmt.Sprintf("%s (%s)", p.CIDR, p.Zone))
	}
	return strings.Join(parts, ", ")
}

// sanitizeName converts a string to a valid K8s resource name component
// by replacing dots with dashes and truncating to 63 chars.
func sanitizeName(s string) string {
	s = strings.ReplaceAll(s, ".", "-")
	if len(s) > 63 {
		s = s[:63]
	}
	return strings.TrimRight(s, "-")
}

// buildBMServerMap calls the VPC API to list bare metal servers and returns a
// map of server name → server ID. This allows resolving nodes that lack a
// providerID (unmanaged bare metal clusters).
func buildBMServerMap(ctx context.Context, vpcClient vpc.Client, vpcID string) map[string]string {
	logger := log.FromContext(ctx)
	if vpcID == "" {
		return nil
	}
	servers, err := vpcClient.ListBareMetalServers(ctx, vpcID)
	if err != nil {
		logger.Error(err, "Failed to list VPC bare metal servers for hostname resolution")
		return nil
	}
	m := make(map[string]string, len(servers))
	for _, s := range servers {
		m[s.Name] = s.ID
	}
	logger.Info("Built BM server map from VPC API", "count", len(m))
	return m
}

// TruncateVPCName truncates a VPC resource name to the 63-character API limit.
// If the name fits, it is returned as-is. Otherwise, the first 55 characters are
// kept followed by a dash and a 7-character SHA-256 hash of the full name to
// ensure uniqueness.
func TruncateVPCName(name string) string {
	const maxLen = 63
	if len(name) <= maxLen {
		return name
	}
	hash := sha256.Sum256([]byte(name))
	suffix := hex.EncodeToString(hash[:])[:7]
	return name[:55] + "-" + suffix
}

// PickBMServer returns the BM server ID of any bare metal node in the cluster.
// Since allow_to_float: true is set on VLAN attachments, it doesn't matter which
// BM server is chosen. Tries providerID first, then falls back to VPC API
// hostname resolution for unmanaged clusters.
func PickBMServer(ctx context.Context, k8sClient client.Client, vpcClient vpc.Client, vpcID string) (string, error) {
	nodeList := &corev1.NodeList{}
	if err := k8sClient.List(ctx, nodeList); err != nil {
		return "", fmt.Errorf("failed to list nodes: %w", err)
	}

	// Try providerID first (ROKS clusters)
	for i := range nodeList.Items {
		bmServerID := ExtractBMServerID(nodeList.Items[i].Spec.ProviderID)
		if bmServerID != "" {
			return bmServerID, nil
		}
	}

	// Fallback: resolve via VPC API hostname lookup (unmanaged BM clusters)
	bmServerMap := buildBMServerMap(ctx, vpcClient, vpcID)
	for i := range nodeList.Items {
		bmServerID := resolveBMServerIDByHostname(nodeList.Items[i].Name, bmServerMap)
		if bmServerID != "" {
			return bmServerID, nil
		}
	}

	// Last resort: check existing VPCGateway statuses for a known BM server ID.
	// On ROKS clusters, the VPC ListBareMetalServers API may not return managed
	// BM servers. If another gateway already discovered the BM server, reuse it.
	bmServerID := pickBMServerFromGatewayStatus(ctx, k8sClient)
	if bmServerID != "" {
		logger := log.FromContext(ctx)
		logger.Info("Resolved BM server ID from existing VPCGateway status", "bmServerID", bmServerID)
		return bmServerID, nil
	}

	return "", fmt.Errorf("no bare metal server found in cluster")
}

// pickBMServerFromGatewayStatus scans existing VPCGateway objects for a non-empty
// BMServerID in their status. This handles the case where ROKS-managed BM servers
// aren't visible via the VPC ListBareMetalServers API but a previous gateway
// already discovered the server ID.
func pickBMServerFromGatewayStatus(ctx context.Context, k8sClient client.Client) string {
	gwList := &apiv1alpha1.VPCGatewayList{}
	if err := k8sClient.List(ctx, gwList); err != nil {
		return ""
	}
	for i := range gwList.Items {
		if gwList.Items[i].Status.BMServerID != "" {
			return gwList.Items[i].Status.BMServerID
		}
	}
	return ""
}

// resolveBMServerIDByHostname matches a K8s node name against the VPC BM server
// name map. It tries exact match first, then matches the hostname prefix (the
// part before the first dot) against the BM server name.
func resolveBMServerIDByHostname(nodeName string, bmServerMap map[string]string) string {
	if bmServerMap == nil {
		return ""
	}
	// Exact match
	if id, ok := bmServerMap[nodeName]; ok {
		return id
	}
	// Hostname prefix match: "sno1-host1.sno1.demo1.cloud" → "sno1-host1"
	hostname := nodeName
	if idx := strings.IndexByte(nodeName, '.'); idx > 0 {
		hostname = nodeName[:idx]
	}
	if id, ok := bmServerMap[hostname]; ok {
		return id
	}
	return ""
}
