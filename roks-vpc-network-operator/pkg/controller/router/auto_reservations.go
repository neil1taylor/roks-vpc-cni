package router

import (
	"context"
	"encoding/json"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	v1alpha1 "github.com/IBM/roks-vpc-network-operator/api/v1alpha1"
	"github.com/IBM/roks-vpc-network-operator/pkg/annotations"
)

var vmGVR = schema.GroupVersionResource{
	Group: "kubevirt.io", Version: "v1", Resource: "virtualmachines",
}

// vmNetworkInterface mirrors the JSON structure in vpc.roks.ibm.com/network-interfaces.
type vmNetworkInterface struct {
	NetworkName string `json:"networkName"`
	MACAddress  string `json:"macAddress"`
	ReservedIP  string `json:"reservedIP"`
}

// discoverAutoReservations lists all VMs and extracts MAC->IP pairs matching the router's networks.
func discoverAutoReservations(ctx context.Context, dynClient dynamic.Interface, networks []v1alpha1.RouterNetwork) (map[string][]v1alpha1.DHCPStaticReservation, error) {
	networkSet := make(map[string]bool, len(networks))
	for _, n := range networks {
		networkSet[n.Name] = true
	}

	result := make(map[string][]v1alpha1.DHCPStaticReservation)

	vmList, err := dynClient.Resource(vmGVR).Namespace("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list VMs: %w", err)
	}

	for _, vm := range vmList.Items {
		annots := vm.GetAnnotations()
		if annots == nil {
			continue
		}

		netIfacesJSON, ok := annots[annotations.NetworkInterfaces]
		if !ok {
			continue
		}

		var ifaces []vmNetworkInterface
		if err := json.Unmarshal([]byte(netIfacesJSON), &ifaces); err != nil {
			continue
		}

		vmName := vm.GetName()
		for _, iface := range ifaces {
			if !networkSet[iface.NetworkName] {
				continue
			}
			if iface.MACAddress == "" || iface.ReservedIP == "" {
				continue
			}
			result[iface.NetworkName] = append(result[iface.NetworkName], v1alpha1.DHCPStaticReservation{
				MAC:      iface.MACAddress,
				IP:       iface.ReservedIP,
				Hostname: vmName,
			})
		}
	}

	return result, nil
}

// mergeReservations combines manual and auto reservations. Manual wins on MAC collision.
func mergeReservations(manual, auto []v1alpha1.DHCPStaticReservation) []v1alpha1.DHCPStaticReservation {
	seen := make(map[string]bool, len(manual))
	merged := make([]v1alpha1.DHCPStaticReservation, 0, len(manual)+len(auto))

	for _, r := range manual {
		seen[r.MAC] = true
		merged = append(merged, r)
	}

	for _, r := range auto {
		if !seen[r.MAC] {
			merged = append(merged, r)
		}
	}

	return merged
}
