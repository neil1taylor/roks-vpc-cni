package router

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1alpha1 "github.com/IBM/roks-vpc-network-operator/api/v1alpha1"
)

func TestEnsureLeasePVC_Creates(t *testing.T) {
	scheme := newTestScheme()
	router := &v1alpha1.VPCRouter{
		ObjectMeta: metav1.ObjectMeta{Name: "r1", Namespace: "default", UID: "uid-123"},
		Spec: v1alpha1.VPCRouterSpec{
			Gateway:  "gw1",
			Networks: []v1alpha1.RouterNetwork{{Name: "net-a", Address: "10.0.0.1/24"}},
			DHCP: &v1alpha1.RouterDHCP{
				Enabled: true,
				LeasePersistence: &v1alpha1.DHCPLeasePersistence{
					Enabled:     true,
					StorageSize: "200Mi",
				},
			},
		},
	}

	fc := fake.NewClientBuilder().WithScheme(scheme).WithObjects(router).Build()
	r := &Reconciler{Client: fc, Scheme: scheme}

	bound, err := r.ensureLeasePVC(context.Background(), router)
	if err != nil {
		t.Fatalf("ensureLeasePVC() error = %v", err)
	}
	if bound {
		t.Error("expected bound=false for newly created PVC (Pending)")
	}

	// Verify PVC was created
	pvc := &corev1.PersistentVolumeClaim{}
	if err := fc.Get(context.Background(), types.NamespacedName{Name: "r1-dhcp-leases", Namespace: "default"}, pvc); err != nil {
		t.Fatalf("PVC not created: %v", err)
	}
	expectedSize := resource.MustParse("200Mi")
	if pvc.Spec.Resources.Requests.Storage().Cmp(expectedSize) != 0 {
		t.Errorf("expected storage 200Mi, got %s", pvc.Spec.Resources.Requests.Storage())
	}
	if len(pvc.OwnerReferences) != 1 || pvc.OwnerReferences[0].Name != "r1" {
		t.Error("expected ownerReference to router")
	}
}

func TestEnsureLeasePVC_AlreadyExists(t *testing.T) {
	scheme := newTestScheme()
	router := &v1alpha1.VPCRouter{
		ObjectMeta: metav1.ObjectMeta{Name: "r1", Namespace: "default", UID: "uid-123"},
		Spec: v1alpha1.VPCRouterSpec{
			Gateway:  "gw1",
			Networks: []v1alpha1.RouterNetwork{{Name: "net-a", Address: "10.0.0.1/24"}},
			DHCP: &v1alpha1.RouterDHCP{
				Enabled:          true,
				LeasePersistence: &v1alpha1.DHCPLeasePersistence{Enabled: true},
			},
		},
	}

	existingPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "r1-dhcp-leases", Namespace: "default"},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("100Mi")},
			},
		},
		Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound},
	}

	fc := fake.NewClientBuilder().WithScheme(scheme).WithObjects(router, existingPVC).Build()
	r := &Reconciler{Client: fc, Scheme: scheme}

	bound, err := r.ensureLeasePVC(context.Background(), router)
	if err != nil {
		t.Fatalf("ensureLeasePVC() error = %v", err)
	}
	if !bound {
		t.Error("expected bound=true for existing Bound PVC")
	}
}

func TestLeasePVCName(t *testing.T) {
	if got := leasePVCName("my-router"); got != "my-router-dhcp-leases" {
		t.Errorf("leasePVCName() = %q, want %q", got, "my-router-dhcp-leases")
	}
}
