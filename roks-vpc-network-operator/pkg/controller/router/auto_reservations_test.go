package router

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"

	v1alpha1 "github.com/IBM/roks-vpc-network-operator/api/v1alpha1"
)

func TestDiscoverAutoReservations(t *testing.T) {
	vm1 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kubevirt.io/v1",
			"kind":       "VirtualMachine",
			"metadata": map[string]interface{}{
				"name":      "vm-web",
				"namespace": "default",
				"annotations": map[string]interface{}{
					"vpc.roks.ibm.com/network-interfaces": `[{"networkName":"net-a","macAddress":"fa:16:3e:aa:bb:01","reservedIP":"10.100.0.11"},{"networkName":"net-b","macAddress":"fa:16:3e:aa:bb:02","reservedIP":"10.200.0.11"}]`,
				},
			},
		},
	}
	vm2 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kubevirt.io/v1",
			"kind":       "VirtualMachine",
			"metadata": map[string]interface{}{
				"name":      "vm-db",
				"namespace": "default",
				"annotations": map[string]interface{}{
					"vpc.roks.ibm.com/network-interfaces": `[{"networkName":"net-a","macAddress":"fa:16:3e:cc:dd:01","reservedIP":"10.100.0.12"}]`,
				},
			},
		},
	}

	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: "kubevirt.io", Version: "v1", Kind: "VirtualMachineList"}, &unstructured.UnstructuredList{})

	dynClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{
			vmGVR: "VirtualMachineList",
		},
		vm1, vm2,
	)

	networks := []v1alpha1.RouterNetwork{
		{Name: "net-a", Address: "10.100.0.1/24"},
	}

	reservations, err := discoverAutoReservations(context.Background(), dynClient, networks)
	if err != nil {
		t.Fatalf("discoverAutoReservations() error = %v", err)
	}

	if len(reservations["net-a"]) != 2 {
		t.Fatalf("expected 2 reservations for net-a, got %d", len(reservations["net-a"]))
	}

	if _, ok := reservations["net-b"]; ok {
		t.Error("net-b should not be in results")
	}
}

func TestDiscoverAutoReservations_SkipsEmptyMAC(t *testing.T) {
	vm := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kubevirt.io/v1",
			"kind":       "VirtualMachine",
			"metadata": map[string]interface{}{
				"name":      "vm-no-mac",
				"namespace": "default",
				"annotations": map[string]interface{}{
					"vpc.roks.ibm.com/network-interfaces": `[{"networkName":"net-a","macAddress":"","reservedIP":"10.100.0.13"}]`,
				},
			},
		},
	}

	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: "kubevirt.io", Version: "v1", Kind: "VirtualMachineList"}, &unstructured.UnstructuredList{})
	dynClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{vmGVR: "VirtualMachineList"},
		vm,
	)

	networks := []v1alpha1.RouterNetwork{{Name: "net-a", Address: "10.100.0.1/24"}}
	reservations, err := discoverAutoReservations(context.Background(), dynClient, networks)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if len(reservations["net-a"]) != 0 {
		t.Errorf("expected 0 reservations (empty MAC), got %d", len(reservations["net-a"]))
	}
}

func TestMergeReservations(t *testing.T) {
	manual := []v1alpha1.DHCPStaticReservation{
		{MAC: "fa:16:3e:aa:bb:01", IP: "10.100.0.50", Hostname: "manual-host"},
	}
	auto := []v1alpha1.DHCPStaticReservation{
		{MAC: "fa:16:3e:aa:bb:01", IP: "10.100.0.11", Hostname: "vm-web"},
		{MAC: "fa:16:3e:cc:dd:01", IP: "10.100.0.12", Hostname: "vm-db"},
	}

	merged := mergeReservations(manual, auto)

	if len(merged) != 2 {
		t.Fatalf("expected 2 merged reservations, got %d", len(merged))
	}

	for _, r := range merged {
		if r.MAC == "fa:16:3e:aa:bb:01" && r.IP != "10.100.0.50" {
			t.Errorf("manual reservation should win, got IP=%s", r.IP)
		}
	}
}
