package cudn

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1alpha1 "github.com/IBM/roks-vpc-network-operator/api/v1alpha1"
	"github.com/IBM/roks-vpc-network-operator/pkg/vpc"
)

func newTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(s)
	_ = v1alpha1.AddToScheme(s)
	return s
}

func makeTestCUDN(name, topology string, annots map[string]string) *unstructured.Unstructured {
	cudn := &unstructured.Unstructured{}
	cudn.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "k8s.ovn.org",
		Version: "v1",
		Kind:    "ClusterUserDefinedNetwork",
	})
	cudn.SetName(name)
	if annots != nil {
		cudn.SetAnnotations(annots)
	}
	_ = unstructured.SetNestedField(cudn.Object, topology, "spec", "network", "topology")
	return cudn
}

func TestReconcile_Layer2CUDN_NoVPCCalls(t *testing.T) {
	scheme := newTestScheme()

	cudn := makeTestCUDN("layer2-net", "Layer2", nil)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cudn).
		Build()

	mockVPC := vpc.NewMockClient()

	r := &Reconciler{
		Client:    fakeClient,
		Scheme:    scheme,
		VPC:       mockVPC,
		ClusterID: "cluster-abc",
	}

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "layer2-net"},
	})

	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	// No VPC API calls should be made for Layer2 CUDNs
	if mockVPC.CallCount("CreateSubnet") != 0 {
		t.Error("CreateSubnet should not be called for Layer2 CUDN")
	}
	if mockVPC.CallCount("CreateVLANAttachment") != 0 {
		t.Error("CreateVLANAttachment should not be called for Layer2 CUDN")
	}
}

func TestReconcile_Layer2CUDN_AddsFinalizer(t *testing.T) {
	scheme := newTestScheme()

	cudn := makeTestCUDN("layer2-net", "Layer2", nil)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cudn).
		Build()

	r := &Reconciler{
		Client:    fakeClient,
		Scheme:    scheme,
		VPC:       vpc.NewMockClient(),
		ClusterID: "cluster-abc",
	}

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "layer2-net"},
	})

	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	// Verify finalizer was added
	updated := &unstructured.Unstructured{}
	updated.SetGroupVersionKind(cudnGVK)
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "layer2-net"}, updated); err != nil {
		t.Fatalf("Failed to get updated CUDN: %v", err)
	}

	finalizers := updated.GetFinalizers()
	found := false
	for _, f := range finalizers {
		if f == "vpc.roks.ibm.com/cudn-cleanup" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected cudn-cleanup finalizer to be added to Layer2 CUDN")
	}
}

func TestReconcile_UnknownTopology_Skipped(t *testing.T) {
	scheme := newTestScheme()

	cudn := makeTestCUDN("unknown-net", "Routed", nil)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cudn).
		Build()

	mockVPC := vpc.NewMockClient()

	r := &Reconciler{
		Client:    fakeClient,
		Scheme:    scheme,
		VPC:       mockVPC,
		ClusterID: "cluster-abc",
	}

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "unknown-net"},
	})

	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if result.Requeue {
		t.Error("Should not requeue for unknown topology")
	}

	if mockVPC.CallCount("CreateSubnet") != 0 {
		t.Error("No VPC calls should be made for unknown topology")
	}
}
