package network

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/IBM/roks-vpc-network-operator/pkg/annotations"
)

var nncpGVK = schema.GroupVersionKind{
	Group:   "nmstate.io",
	Version: "v1",
	Kind:    "NodeNetworkConfigurationPolicy",
}

// NNCPConfig holds configuration for auto-creating OVN bridge-mapping NNCPs.
type NNCPConfig struct {
	Enabled      bool
	BridgeName   string            // default "br-localnet"
	SecondaryNIC string            // default "bond1"
	NodeSelector map[string]string // default: node-role.kubernetes.io/worker=""
}

// EnsureNNCP creates a NodeNetworkConfigurationPolicy for the given LocalNet network
// so OVN-Kubernetes can map the logical network to a physical OVS bridge.
// The physicalNetworkName comes from the CUDN/UDN spec (spec.network.localnet.physicalNetworkName)
// or falls back to the object name.
func EnsureNNCP(ctx context.Context, k8sClient client.Client, obj client.Object, physicalNetworkName string, cfg NNCPConfig) error {
	if !cfg.Enabled {
		return nil
	}

	logger := log.FromContext(ctx)

	if physicalNetworkName == "" {
		physicalNetworkName = obj.GetName()
	}

	nncpName := fmt.Sprintf("localnet-%s", physicalNetworkName)

	// Idempotent: if annotation is already set and NNCP exists, skip
	annots := obj.GetAnnotations()
	if annots != nil && annots[annotations.NNCPName] != "" {
		existing := &unstructured.Unstructured{}
		existing.SetGroupVersionKind(nncpGVK)
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: annots[annotations.NNCPName]}, existing); err == nil {
			return nil
		}
		// NNCP was deleted externally — recreate it
	}

	bridgeName := cfg.BridgeName
	if bridgeName == "" {
		bridgeName = "br-localnet"
	}
	secondaryNIC := cfg.SecondaryNIC
	if secondaryNIC == "" {
		secondaryNIC = "bond1"
	}
	nodeSelector := cfg.NodeSelector
	if nodeSelector == nil {
		nodeSelector = map[string]string{"node-role.kubernetes.io/worker": ""}
	}

	nncp := &unstructured.Unstructured{}
	nncp.SetGroupVersionKind(nncpGVK)
	nncp.SetName(nncpName)
	nncp.SetLabels(map[string]string{
		"vpc.roks.ibm.com/managed-by": "roks-vpc-network-operator",
		"vpc.roks.ibm.com/network":    obj.GetName(),
	})

	// Build the desiredState for OVS bridge + OVN bridge-mapping
	desiredState := map[string]interface{}{
		"interfaces": []interface{}{
			map[string]interface{}{
				"name":  bridgeName,
				"type":  "ovs-bridge",
				"state": "up",
				"bridge": map[string]interface{}{
					"allow-extra-patch-ports": true,
					"options": map[string]interface{}{
						"stp": false,
					},
					"port": []interface{}{
						map[string]interface{}{
							"name": secondaryNIC,
						},
					},
				},
			},
		},
		"ovn": map[string]interface{}{
			"bridge-mappings": []interface{}{
				map[string]interface{}{
					"localnet": physicalNetworkName,
					"bridge":   bridgeName,
					"state":    "present",
				},
			},
		},
	}

	// Convert nodeSelector to map[string]interface{} for unstructured
	nodeSelectorIface := make(map[string]interface{}, len(nodeSelector))
	for k, v := range nodeSelector {
		nodeSelectorIface[k] = v
	}

	if err := unstructured.SetNestedField(nncp.Object, nodeSelectorIface, "spec", "nodeSelector"); err != nil {
		return fmt.Errorf("failed to set nodeSelector: %w", err)
	}
	if err := unstructured.SetNestedField(nncp.Object, desiredState, "spec", "desiredState"); err != nil {
		return fmt.Errorf("failed to set desiredState: %w", err)
	}

	if err := k8sClient.Create(ctx, nncp); err != nil {
		if errors.IsAlreadyExists(err) {
			logger.Info("NNCP already exists", "name", nncpName)
		} else {
			return fmt.Errorf("failed to create NNCP %s: %w", nncpName, err)
		}
	} else {
		logger.Info("Created NNCP for bridge-mapping", "name", nncpName, "physicalNetwork", physicalNetworkName, "bridge", bridgeName)
	}

	// Store the NNCP name in annotations
	if annots == nil {
		annots = map[string]string{}
	}
	annots[annotations.NNCPName] = nncpName
	obj.SetAnnotations(annots)
	if err := k8sClient.Update(ctx, obj); err != nil {
		return fmt.Errorf("failed to update NNCP annotation: %w", err)
	}

	return nil
}

// DeleteNNCP removes the NNCP tracked in the object's annotations. Tolerates NotFound.
func DeleteNNCP(ctx context.Context, k8sClient client.Client, obj client.Object) {
	logger := log.FromContext(ctx)
	annots := obj.GetAnnotations()
	if annots == nil {
		return
	}

	nncpName := annots[annotations.NNCPName]
	if nncpName == "" {
		return
	}

	nncp := &unstructured.Unstructured{}
	nncp.SetGroupVersionKind(nncpGVK)
	nncp.SetName(nncpName)

	if err := k8sClient.Delete(ctx, nncp); err != nil {
		if !errors.IsNotFound(err) {
			logger.Error(err, "Failed to delete NNCP (non-fatal)", "name", nncpName)
		}
	} else {
		logger.Info("Deleted NNCP", "name", nncpName)
	}
}

// ExtractPhysicalNetworkName reads the physicalNetworkName from an unstructured CUDN/UDN spec.
// For CUDNs: spec.network.localnet.physicalNetworkName
// Falls back to the object name if not found.
func ExtractPhysicalNetworkName(obj *unstructured.Unstructured) string {
	// CUDN path: spec.network.localnet.physicalNetworkName
	if name, found, _ := unstructured.NestedString(obj.Object, "spec", "network", "localnet", "physicalNetworkName"); found && name != "" {
		return name
	}
	// UDN path: spec.localnet.physicalNetworkName
	if name, found, _ := unstructured.NestedString(obj.Object, "spec", "localnet", "physicalNetworkName"); found && name != "" {
		return name
	}
	return obj.GetName()
}
