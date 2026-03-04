package router

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	v1alpha1 "github.com/IBM/roks-vpc-network-operator/api/v1alpha1"
)

// leasePVCName returns the deterministic PVC name for a router's DHCP leases.
func leasePVCName(routerName string) string {
	return routerName + "-dhcp-leases"
}

// ensureLeasePVC creates the PVC if it doesn't exist and returns whether it is Bound.
func (r *Reconciler) ensureLeasePVC(ctx context.Context, router *v1alpha1.VPCRouter) (bool, error) {
	name := leasePVCName(router.Name)
	pvc := &corev1.PersistentVolumeClaim{}
	err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: router.Namespace}, pvc)
	if err == nil {
		return pvc.Status.Phase == corev1.ClaimBound, nil
	}
	if !errors.IsNotFound(err) {
		return false, fmt.Errorf("failed to get lease PVC: %w", err)
	}

	// Determine storage size
	storageSize := "100Mi"
	if router.Spec.DHCP != nil && router.Spec.DHCP.LeasePersistence != nil && router.Spec.DHCP.LeasePersistence.StorageSize != "" {
		storageSize = router.Spec.DHCP.LeasePersistence.StorageSize
	}

	pvc = &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: router.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "roks-vpc-network-operator",
				"app.kubernetes.io/component":  "dhcp-leases",
				"vpc.roks.ibm.com/router":      router.Name,
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(storageSize),
				},
			},
		},
	}

	// Set StorageClassName if specified
	if router.Spec.DHCP != nil && router.Spec.DHCP.LeasePersistence != nil && router.Spec.DHCP.LeasePersistence.StorageClassName != "" {
		sc := router.Spec.DHCP.LeasePersistence.StorageClassName
		pvc.Spec.StorageClassName = &sc
	}

	// Set owner reference for automatic cleanup
	if err := controllerutil.SetControllerReference(router, pvc, r.Scheme); err != nil {
		return false, fmt.Errorf("failed to set owner reference on PVC: %w", err)
	}

	if err := r.Create(ctx, pvc); err != nil {
		return false, fmt.Errorf("failed to create lease PVC: %w", err)
	}

	return false, nil // newly created, not yet Bound
}
