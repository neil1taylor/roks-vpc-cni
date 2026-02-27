package handler

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	fakedynamic "k8s.io/client-go/dynamic/fake"
)

// fakeDynamic wraps the k8s fake dynamic client.
type fakeDynamic = fakedynamic.FakeDynamicClient

// newFakeDynamicClient creates a fake dynamic client with the required CRD schemes registered.
func newFakeDynamicClient() *fakeDynamic {
	scheme := runtime.NewScheme()
	// Register GVRs so the fake client can track them
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{
		Group: "k8s.ovn.org", Version: "v1", Kind: "ClusterUserDefinedNetworkList",
	}, &unstructured.UnstructuredList{})
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{
		Group: "k8s.ovn.org", Version: "v1", Kind: "ClusterUserDefinedNetwork",
	}, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{
		Group: "k8s.ovn.org", Version: "v1", Kind: "UserDefinedNetworkList",
	}, &unstructured.UnstructuredList{})
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{
		Group: "k8s.ovn.org", Version: "v1", Kind: "UserDefinedNetwork",
	}, &unstructured.Unstructured{})

	return fakedynamic.NewSimpleDynamicClient(scheme)
}

// createTestCUDN creates a test ClusterUserDefinedNetwork in the fake dynamic client.
func createTestCUDN(t *testing.T, ctx context.Context, client *fakeDynamic, name, topology string) {
	t.Helper()
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "k8s.ovn.org/v1",
			"kind":       "ClusterUserDefinedNetwork",
			"metadata": map[string]interface{}{
				"name": name,
			},
			"spec": map[string]interface{}{
				"topology": topology,
			},
		},
	}
	_, err := client.Resource(cudnGVR).Create(ctx, obj, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create test CUDN: %v", err)
	}
}

// createTestUDN creates a test UserDefinedNetwork in the fake dynamic client.
func createTestUDN(t *testing.T, ctx context.Context, client *fakeDynamic, namespace, name, topology string) {
	t.Helper()
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "k8s.ovn.org/v1",
			"kind":       "UserDefinedNetwork",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
			"spec": map[string]interface{}{
				"topology": topology,
			},
		},
	}
	_, err := client.Resource(udnGVR).Namespace(namespace).Create(ctx, obj, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create test UDN: %v", err)
	}
}
