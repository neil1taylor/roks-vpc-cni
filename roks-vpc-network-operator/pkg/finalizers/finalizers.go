package finalizers

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	// CUDNCleanup is added to CUDNs to ensure VPC subnet and VLAN attachments
	// are deleted before the CUDN object is removed.
	CUDNCleanup = "vpc.roks.ibm.com/cudn-cleanup"

	// VMCleanup is added to VirtualMachines to ensure VNI, reserved IP, and
	// floating IP are deleted before the VM object is removed.
	VMCleanup = "vpc.roks.ibm.com/vm-cleanup"

	// UDNCleanup is added to UserDefinedNetworks to ensure VPC subnet and
	// VLAN attachments are deleted before the UDN object is removed.
	UDNCleanup = "vpc.roks.ibm.com/udn-cleanup"

	// GatewayCleanup is added to VPCGateways to ensure VNI, VPC routes,
	// floating IP, transit CUDN, and router pod are cleaned up on deletion.
	GatewayCleanup = "vpc.roks.ibm.com/gateway-cleanup"

	// RouterCleanup is added to VPCRouters to ensure router pod and
	// ConfigMaps are cleaned up on deletion.
	RouterCleanup = "vpc.roks.ibm.com/router-cleanup"

	// L2BridgeCleanup is added to VPCL2Bridges to ensure the bridge pod
	// is cleaned up on deletion.
	L2BridgeCleanup = "vpc.roks.ibm.com/l2bridge-cleanup"
)

// Add adds the given finalizer to the object if not already present.
// Returns true if the finalizer was added (object needs update).
func Add(obj metav1.Object, finalizer string) bool {
	return controllerutil.AddFinalizer(obj.(client.Object), finalizer)
}

// Remove removes the given finalizer from the object.
// Returns true if the finalizer was removed (object needs update).
func Remove(obj metav1.Object, finalizer string) bool {
	return controllerutil.RemoveFinalizer(obj.(client.Object), finalizer)
}

// Has checks if the object has the given finalizer.
func Has(obj metav1.Object, finalizer string) bool {
	return controllerutil.ContainsFinalizer(obj.(client.Object), finalizer)
}

// EnsureAdded adds the finalizer and persists the update if it was added.
func EnsureAdded(ctx context.Context, c client.Client, obj client.Object, finalizer string) error {
	if controllerutil.AddFinalizer(obj, finalizer) {
		return c.Update(ctx, obj)
	}
	return nil
}

// EnsureRemoved removes the finalizer and persists the update if it was removed.
func EnsureRemoved(ctx context.Context, c client.Client, obj client.Object, finalizer string) error {
	if controllerutil.RemoveFinalizer(obj, finalizer) {
		return c.Update(ctx, obj)
	}
	return nil
}
