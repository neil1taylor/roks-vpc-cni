package traceflow

import (
	"context"
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1alpha1 "github.com/IBM/roks-vpc-network-operator/api/v1alpha1"
)

func newTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(s)
	_ = v1alpha1.AddToScheme(s)
	return s
}

// fakeExecFn returns a mock ExecFn that returns canned output based on the command.
func fakeExecFn(nftOutput, tracerouteOutput, probeOutput string, probeErr error) ExecFn {
	callCount := 0
	return func(namespace, pod, container string, cmd []string) (string, string, error) {
		if len(cmd) == 0 {
			return "", "", fmt.Errorf("empty command")
		}
		switch cmd[0] {
		case "nft":
			callCount++
			return nftOutput, "", nil
		case "traceroute":
			return tracerouteOutput, "", nil
		case "ping", "nping":
			return probeOutput, "", probeErr
		default:
			return "", "", fmt.Errorf("unknown command: %s", cmd[0])
		}
	}
}

// fakeExecFnWithNftDiff returns a mock ExecFn that returns different nft output
// for the before and after snapshots to simulate counter changes.
func fakeExecFnWithNftDiff(nftBefore, nftAfter, tracerouteOutput, probeOutput string, probeErr error) ExecFn {
	nftCallCount := 0
	return func(namespace, pod, container string, cmd []string) (string, string, error) {
		if len(cmd) == 0 {
			return "", "", fmt.Errorf("empty command")
		}
		switch cmd[0] {
		case "nft":
			nftCallCount++
			if nftCallCount == 1 {
				return nftBefore, "", nil
			}
			return nftAfter, "", nil
		case "traceroute":
			return tracerouteOutput, "", nil
		case "ping", "nping":
			return probeOutput, "", probeErr
		default:
			return "", "", fmt.Errorf("unknown command: %s", cmd[0])
		}
	}
}

func newRouter(name, namespace string) *v1alpha1.VPCRouter {
	return &v1alpha1.VPCRouter{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: v1alpha1.VPCRouterSpec{
			Gateway:  "gw1",
			Networks: []v1alpha1.RouterNetwork{{Name: "net-a", Address: "10.0.0.1/24"}},
		},
	}
}

func newRouterPod(routerName, namespace string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      routerName + "-router",
			Namespace: namespace,
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}
}

func newTraceflow(name, namespace, routerRef, destIP string) *v1alpha1.VPCTraceflow {
	return &v1alpha1.VPCTraceflow{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: v1alpha1.VPCTraceflowSpec{
			Source:      v1alpha1.TraceflowSource{IP: "10.0.0.2"},
			Destination: v1alpha1.TraceflowDestination{IP: destIP, Protocol: "ICMP"},
			RouterRef:   routerRef,
			Timeout:     "30s",
			TTL:         "1h",
		},
	}
}

const sampleTraceroute = `traceroute to 8.8.8.8 (8.8.8.8), 30 hops max, 60 byte packets
 1  172.16.100.1 (172.16.100.1)  0.456 ms  0.312 ms  0.298 ms
 2  10.0.0.1 (10.0.0.1)  1.234 ms  1.100 ms  1.050 ms
 3  8.8.8.8 (8.8.8.8)  4.567 ms  4.321 ms  4.100 ms`

const sampleNftOutput = `table ip nat {
	chain postrouting {
		counter packets 100 bytes 5000 masquerade
	}
	chain prerouting {
		counter packets 50 bytes 2500 accept
	}
}
table ip filter {
	chain forward {
		counter packets 200 bytes 10000 accept
		counter packets 0 bytes 0 drop
	}
}`

const sampleNftAfterOutput = `table ip nat {
	chain postrouting {
		counter packets 103 bytes 5150 masquerade
	}
	chain prerouting {
		counter packets 53 bytes 2650 accept
	}
}
table ip filter {
	chain forward {
		counter packets 203 bytes 10150 accept
		counter packets 0 bytes 0 drop
	}
}`

func TestReconcile_SuccessfulProbe(t *testing.T) {
	scheme := newTestScheme()

	router := newRouter("my-router", "default")
	pod := newRouterPod("my-router", "default")
	tf := newTraceflow("test-trace", "default", "my-router", "8.8.8.8")

	fc := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(router, pod, tf).
		WithStatusSubresource(tf).
		Build()

	rec := &Reconciler{
		Client:   fc,
		Scheme:   scheme,
		Recorder: record.NewFakeRecorder(10),
		ExecFn:   fakeExecFn(sampleNftOutput, sampleTraceroute, "3 packets transmitted, 3 received, 0% packet loss", nil),
	}

	// First reconcile: sets Running then completes
	_, err := rec.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "test-trace", Namespace: "default"},
	})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	// Verify status
	updated := &v1alpha1.VPCTraceflow{}
	if err := fc.Get(context.Background(), types.NamespacedName{Name: "test-trace", Namespace: "default"}, updated); err != nil {
		t.Fatalf("Failed to get traceflow: %v", err)
	}

	// After the first reconcile, we set Running and then execute probe which sets Completed
	// The status update for Running happens, then executeProbe updates to Completed
	if updated.Status.Phase != "Completed" {
		t.Errorf("expected phase Completed, got %q", updated.Status.Phase)
	}
	if updated.Status.Result != "Reachable" {
		t.Errorf("expected result Reachable, got %q", updated.Status.Result)
	}
	if len(updated.Status.Hops) != 3 {
		t.Errorf("expected 3 hops, got %d", len(updated.Status.Hops))
	}
	if updated.Status.StartTime == nil {
		t.Error("expected startTime to be set")
	}
	if updated.Status.CompletionTime == nil {
		t.Error("expected completionTime to be set")
	}
	if updated.Status.TotalLatency != "4.567ms" {
		t.Errorf("expected totalLatency 4.567ms, got %q", updated.Status.TotalLatency)
	}
}

func TestReconcile_MissingRouter(t *testing.T) {
	scheme := newTestScheme()

	tf := newTraceflow("bad-ref", "default", "nonexistent", "8.8.8.8")

	fc := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(tf).
		WithStatusSubresource(tf).
		Build()

	rec := &Reconciler{
		Client:   fc,
		Scheme:   scheme,
		Recorder: record.NewFakeRecorder(10),
		ExecFn:   fakeExecFn("", "", "", nil),
	}

	_, err := rec.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "bad-ref", Namespace: "default"},
	})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	updated := &v1alpha1.VPCTraceflow{}
	_ = fc.Get(context.Background(), types.NamespacedName{Name: "bad-ref", Namespace: "default"}, updated)
	if updated.Status.Phase != "Failed" {
		t.Errorf("expected phase Failed, got %q", updated.Status.Phase)
	}
	if updated.Status.Message == "" {
		t.Error("expected error message to be set")
	}
}

func TestReconcile_MissingPod(t *testing.T) {
	scheme := newTestScheme()

	router := newRouter("my-router", "default")
	tf := newTraceflow("no-pod", "default", "my-router", "8.8.8.8")

	fc := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(router, tf).
		WithStatusSubresource(tf).
		Build()

	rec := &Reconciler{
		Client:   fc,
		Scheme:   scheme,
		Recorder: record.NewFakeRecorder(10),
		ExecFn:   fakeExecFn("", "", "", nil),
	}

	_, err := rec.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "no-pod", Namespace: "default"},
	})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	updated := &v1alpha1.VPCTraceflow{}
	_ = fc.Get(context.Background(), types.NamespacedName{Name: "no-pod", Namespace: "default"}, updated)
	if updated.Status.Phase != "Failed" {
		t.Errorf("expected phase Failed, got %q", updated.Status.Phase)
	}
}

func TestReconcile_ProbeTimeout(t *testing.T) {
	scheme := newTestScheme()

	router := newRouter("my-router", "default")
	pod := newRouterPod("my-router", "default")
	tf := newTraceflow("timeout-trace", "default", "my-router", "192.168.1.1")

	fc := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(router, pod, tf).
		WithStatusSubresource(tf).
		Build()

	rec := &Reconciler{
		Client:   fc,
		Scheme:   scheme,
		Recorder: record.NewFakeRecorder(10),
		ExecFn: fakeExecFn(sampleNftOutput, "", "3 packets transmitted, 0 received, 100% packet loss",
			fmt.Errorf("exit status 1")),
	}

	_, err := rec.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "timeout-trace", Namespace: "default"},
	})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	updated := &v1alpha1.VPCTraceflow{}
	_ = fc.Get(context.Background(), types.NamespacedName{Name: "timeout-trace", Namespace: "default"}, updated)
	if updated.Status.Phase != "Completed" {
		t.Errorf("expected phase Completed, got %q", updated.Status.Phase)
	}
	if updated.Status.Result != "Timeout" {
		t.Errorf("expected result Timeout, got %q", updated.Status.Result)
	}
}

func TestReconcile_Filtered(t *testing.T) {
	scheme := newTestScheme()

	router := newRouter("my-router", "default")
	pod := newRouterPod("my-router", "default")
	tf := newTraceflow("filtered-trace", "default", "my-router", "10.0.0.99")

	fc := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(router, pod, tf).
		WithStatusSubresource(tf).
		Build()

	nftBefore := `table ip filter {
	chain forward {
		counter packets 100 bytes 5000 accept
		counter packets 0 bytes 0 drop
	}
}`
	nftAfter := `table ip filter {
	chain forward {
		counter packets 100 bytes 5000 accept
		counter packets 3 bytes 150 drop
	}
}`

	rec := &Reconciler{
		Client:   fc,
		Scheme:   scheme,
		Recorder: record.NewFakeRecorder(10),
		ExecFn: fakeExecFnWithNftDiff(nftBefore, nftAfter, "", "100% packet loss",
			fmt.Errorf("exit status 1")),
	}

	_, err := rec.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "filtered-trace", Namespace: "default"},
	})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	updated := &v1alpha1.VPCTraceflow{}
	_ = fc.Get(context.Background(), types.NamespacedName{Name: "filtered-trace", Namespace: "default"}, updated)
	if updated.Status.Phase != "Completed" {
		t.Errorf("expected phase Completed, got %q", updated.Status.Phase)
	}
	if updated.Status.Result != "Filtered" {
		t.Errorf("expected result Filtered, got %q", updated.Status.Result)
	}
	// Should have a synthetic nftables hop
	if len(updated.Status.Hops) == 0 {
		t.Error("expected at least one hop with nftables hits")
	}
}

func TestReconcile_TTLCleanup(t *testing.T) {
	scheme := newTestScheme()

	// Create a completed traceflow with creation time in the past
	pastTime := metav1.NewTime(time.Now().Add(-2 * time.Hour))
	completionTime := metav1.NewTime(time.Now().Add(-2*time.Hour + time.Minute))

	tf := &v1alpha1.VPCTraceflow{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "old-trace",
			Namespace:         "default",
			CreationTimestamp: pastTime,
		},
		Spec: v1alpha1.VPCTraceflowSpec{
			Source:      v1alpha1.TraceflowSource{IP: "10.0.0.2"},
			Destination: v1alpha1.TraceflowDestination{IP: "8.8.8.8", Protocol: "ICMP"},
			RouterRef:   "my-router",
			TTL:         "1h",
		},
		Status: v1alpha1.VPCTraceflowStatus{
			Phase:          "Completed",
			Result:         "Reachable",
			CompletionTime: &completionTime,
		},
	}

	fc := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(tf).
		WithStatusSubresource(tf).
		Build()

	rec := &Reconciler{
		Client:   fc,
		Scheme:   scheme,
		Recorder: record.NewFakeRecorder(10),
		ExecFn:   fakeExecFn("", "", "", nil),
	}

	_, err := rec.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "old-trace", Namespace: "default"},
	})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	// Verify the object was deleted
	deleted := &v1alpha1.VPCTraceflow{}
	err = fc.Get(context.Background(), types.NamespacedName{Name: "old-trace", Namespace: "default"}, deleted)
	if err == nil {
		t.Error("expected traceflow to be deleted after TTL expiry")
	}
}

func TestReconcile_AlreadyCompleted_NotExpired(t *testing.T) {
	scheme := newTestScheme()

	completionTime := metav1.Now()
	tf := &v1alpha1.VPCTraceflow{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "recent-trace",
			Namespace:         "default",
			CreationTimestamp: metav1.Now(),
		},
		Spec: v1alpha1.VPCTraceflowSpec{
			Source:      v1alpha1.TraceflowSource{IP: "10.0.0.2"},
			Destination: v1alpha1.TraceflowDestination{IP: "8.8.8.8", Protocol: "ICMP"},
			RouterRef:   "my-router",
			TTL:         "1h",
		},
		Status: v1alpha1.VPCTraceflowStatus{
			Phase:          "Completed",
			Result:         "Reachable",
			CompletionTime: &completionTime,
		},
	}

	fc := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(tf).
		WithStatusSubresource(tf).
		Build()

	rec := &Reconciler{
		Client:   fc,
		Scheme:   scheme,
		Recorder: record.NewFakeRecorder(10),
		ExecFn:   fakeExecFn("", "", "", nil),
	}

	result, err := rec.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "recent-trace", Namespace: "default"},
	})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	// Should requeue for future TTL cleanup
	if result.RequeueAfter <= 0 {
		t.Error("expected RequeueAfter > 0 for not-yet-expired completed trace")
	}

	// Object should still exist
	existing := &v1alpha1.VPCTraceflow{}
	if err := fc.Get(context.Background(), types.NamespacedName{Name: "recent-trace", Namespace: "default"}, existing); err != nil {
		t.Errorf("expected traceflow to still exist: %v", err)
	}
}

func TestReconcile_NotFound(t *testing.T) {
	scheme := newTestScheme()

	fc := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	rec := &Reconciler{
		Client:   fc,
		Scheme:   scheme,
		Recorder: record.NewFakeRecorder(10),
		ExecFn:   fakeExecFn("", "", "", nil),
	}

	_, err := rec.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "gone", Namespace: "default"},
	})
	if err != nil {
		t.Fatalf("Reconcile() error = %v, expected nil for not found", err)
	}
}

func TestReconcile_NftCounterDiff(t *testing.T) {
	scheme := newTestScheme()

	router := newRouter("my-router", "default")
	pod := newRouterPod("my-router", "default")
	tf := newTraceflow("nft-trace", "default", "my-router", "8.8.8.8")

	fc := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(router, pod, tf).
		WithStatusSubresource(tf).
		Build()

	rec := &Reconciler{
		Client:   fc,
		Scheme:   scheme,
		Recorder: record.NewFakeRecorder(10),
		ExecFn: fakeExecFnWithNftDiff(sampleNftOutput, sampleNftAfterOutput, sampleTraceroute,
			"3 packets transmitted, 3 received, 0% packet loss", nil),
	}

	_, err := rec.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "nft-trace", Namespace: "default"},
	})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	updated := &v1alpha1.VPCTraceflow{}
	_ = fc.Get(context.Background(), types.NamespacedName{Name: "nft-trace", Namespace: "default"}, updated)

	if updated.Status.Result != "Reachable" {
		t.Errorf("expected result Reachable, got %q", updated.Status.Result)
	}

	// The last hop should have nftables hits
	if len(updated.Status.Hops) == 0 {
		t.Fatal("expected hops to be populated")
	}
	lastHop := updated.Status.Hops[len(updated.Status.Hops)-1]
	if len(lastHop.NftablesHits) == 0 {
		t.Error("expected nftables hits on last hop")
	}
}

func TestReconcile_TCPProbe(t *testing.T) {
	scheme := newTestScheme()

	router := newRouter("my-router", "default")
	pod := newRouterPod("my-router", "default")
	port := int32(443)
	tf := &v1alpha1.VPCTraceflow{
		ObjectMeta: metav1.ObjectMeta{Name: "tcp-trace", Namespace: "default"},
		Spec: v1alpha1.VPCTraceflowSpec{
			Source: v1alpha1.TraceflowSource{IP: "10.0.0.2"},
			Destination: v1alpha1.TraceflowDestination{
				IP:       "8.8.8.8",
				Port:     &port,
				Protocol: "TCP",
			},
			RouterRef: "my-router",
			Timeout:   "30s",
			TTL:       "1h",
		},
	}

	var capturedCmd []string
	execFn := func(namespace, pod, container string, cmd []string) (string, string, error) {
		if len(cmd) > 0 && cmd[0] == "nping" {
			capturedCmd = cmd
			return "SENT (0.0010s) TCP > 8.8.8.8:443", "", nil
		}
		if len(cmd) > 0 && cmd[0] == "nft" {
			return sampleNftOutput, "", nil
		}
		if len(cmd) > 0 && cmd[0] == "traceroute" {
			return sampleTraceroute, "", nil
		}
		return "", "", nil
	}

	fc := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(router, pod, tf).
		WithStatusSubresource(tf).
		Build()

	rec := &Reconciler{
		Client:   fc,
		Scheme:   scheme,
		Recorder: record.NewFakeRecorder(10),
		ExecFn:   execFn,
	}

	_, err := rec.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "tcp-trace", Namespace: "default"},
	})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	// Verify nping was called with --tcp
	if len(capturedCmd) == 0 {
		t.Fatal("expected nping command to be captured")
	}
	if capturedCmd[0] != "nping" || capturedCmd[1] != "--tcp" {
		t.Errorf("expected nping --tcp, got %v", capturedCmd)
	}

	updated := &v1alpha1.VPCTraceflow{}
	_ = fc.Get(context.Background(), types.NamespacedName{Name: "tcp-trace", Namespace: "default"}, updated)
	if updated.Status.Phase != "Completed" {
		t.Errorf("expected phase Completed, got %q", updated.Status.Phase)
	}
	if updated.Status.Result != "Reachable" {
		t.Errorf("expected result Reachable, got %q", updated.Status.Result)
	}
}

func TestDetermineResult(t *testing.T) {
	tests := []struct {
		name     string
		probeErr error
		stdout   string
		stderr   string
		nftHits  []v1alpha1.NFTablesRuleHit
		expected string
	}{
		{
			name:     "success",
			probeErr: nil,
			expected: "Reachable",
		},
		{
			name:     "timeout",
			probeErr: fmt.Errorf("exit 1"),
			stdout:   "100% packet loss",
			expected: "Timeout",
		},
		{
			name:     "filtered by drop rule",
			probeErr: fmt.Errorf("exit 1"),
			stdout:   "100% packet loss",
			nftHits:  []v1alpha1.NFTablesRuleHit{{Chain: "filter/forward", Rule: "drop", Packets: 3}},
			expected: "Filtered",
		},
		{
			name:     "unreachable",
			probeErr: fmt.Errorf("exit 1"),
			stdout:   "some other error",
			expected: "Unreachable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := determineResult(tt.probeErr, tt.stdout, tt.stderr, tt.nftHits)
			if result != tt.expected {
				t.Errorf("determineResult() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestParseDurationWithDefault(t *testing.T) {
	d, err := parseDurationWithDefault("", time.Hour)
	if err != nil || d != time.Hour {
		t.Errorf("expected 1h default, got %v (err: %v)", d, err)
	}

	d, err = parseDurationWithDefault("30m", time.Hour)
	if err != nil || d != 30*time.Minute {
		t.Errorf("expected 30m, got %v (err: %v)", d, err)
	}

	_, err = parseDurationWithDefault("invalid", time.Hour)
	if err == nil {
		t.Error("expected error for invalid duration")
	}
}

func TestReconcile_PodNotRunning(t *testing.T) {
	scheme := newTestScheme()

	router := newRouter("my-router", "default")
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-router-router",
			Namespace: "default",
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
		},
	}
	tf := newTraceflow("pending-pod", "default", "my-router", "8.8.8.8")

	fc := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(router, pod, tf).
		WithStatusSubresource(tf).
		Build()

	rec := &Reconciler{
		Client:   fc,
		Scheme:   scheme,
		Recorder: record.NewFakeRecorder(10),
		ExecFn:   fakeExecFn("", "", "", nil),
	}

	_, err := rec.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "pending-pod", Namespace: "default"},
	})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	updated := &v1alpha1.VPCTraceflow{}
	_ = fc.Get(context.Background(), types.NamespacedName{Name: "pending-pod", Namespace: "default"}, updated)
	if updated.Status.Phase != "Failed" {
		t.Errorf("expected phase Failed, got %q", updated.Status.Phase)
	}
}
