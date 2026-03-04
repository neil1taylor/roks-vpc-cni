package traceflow

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	v1alpha1 "github.com/IBM/roks-vpc-network-operator/api/v1alpha1"
)

// ExecFn is the function signature for executing a command in a pod container.
// Returns stdout, stderr, and any error.
type ExecFn func(namespace, pod, container string, cmd []string) (stdout, stderr string, err error)

// Reconciler reconciles a VPCTraceflow object.
type Reconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder

	// ExecFn executes a command in a pod container. Injected for testability.
	ExecFn ExecFn
}

// Reconcile handles the reconciliation loop for VPCTraceflow resources.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling VPCTraceflow", "name", req.Name, "namespace", req.Namespace)

	// Fetch the VPCTraceflow CR
	tf := &v1alpha1.VPCTraceflow{}
	if err := r.Get(ctx, req.NamespacedName, tf); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// TTL cleanup: if completed or failed, check if TTL has expired
	if tf.Status.Phase == "Completed" || tf.Status.Phase == "Failed" {
		ttl, err := parseDurationWithDefault(tf.Spec.TTL, time.Hour)
		if err != nil {
			logger.Error(err, "Invalid TTL, using default 1h")
			ttl = time.Hour
		}
		expiry := tf.CreationTimestamp.Time.Add(ttl)
		if time.Now().After(expiry) {
			logger.Info("TTL expired, deleting VPCTraceflow", "name", tf.Name)
			if err := r.Delete(ctx, tf); err != nil && !errors.IsNotFound(err) {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		// Requeue at expiry time for cleanup
		requeueAfter := time.Until(expiry)
		if requeueAfter < 0 {
			requeueAfter = time.Second
		}
		return ctrl.Result{RequeueAfter: requeueAfter}, nil
	}

	// If already running, skip (avoid duplicate exec)
	if tf.Status.Phase == "Running" {
		return ctrl.Result{}, nil
	}

	// Set phase to Running and record start time
	now := metav1.Now()
	tf.Status.Phase = "Running"
	tf.Status.StartTime = &now
	meta.SetStatusCondition(&tf.Status.Conditions, metav1.Condition{
		Type:               "Running",
		Status:             metav1.ConditionTrue,
		Reason:             "ProbeStarted",
		Message:            "Traceflow probe started",
		LastTransitionTime: now,
	})
	if err := r.Status().Update(ctx, tf); err != nil {
		return ctrl.Result{}, err
	}

	// Execute the traceflow probe
	if err := r.executeProbe(ctx, tf); err != nil {
		return r.setFailed(ctx, tf, err)
	}

	return ctrl.Result{}, nil
}

// executeProbe performs the actual traceflow probe by exec-ing into the router pod.
func (r *Reconciler) executeProbe(ctx context.Context, tf *v1alpha1.VPCTraceflow) error {
	logger := log.FromContext(ctx)

	// Look up the VPCRouter CR
	router := &v1alpha1.VPCRouter{}
	if err := r.Get(ctx, types.NamespacedName{Name: tf.Spec.RouterRef, Namespace: tf.Namespace}, router); err != nil {
		if errors.IsNotFound(err) {
			return fmt.Errorf("router %q not found", tf.Spec.RouterRef)
		}
		return fmt.Errorf("failed to get router %q: %w", tf.Spec.RouterRef, err)
	}

	// Find the router pod (named {routerName}-router)
	podName := tf.Spec.RouterRef + "-router"
	pod := &corev1.Pod{}
	if err := r.Get(ctx, types.NamespacedName{Name: podName, Namespace: tf.Namespace}, pod); err != nil {
		if errors.IsNotFound(err) {
			return fmt.Errorf("router pod %q not found", podName)
		}
		return fmt.Errorf("failed to get router pod %q: %w", podName, err)
	}

	if pod.Status.Phase != corev1.PodRunning {
		return fmt.Errorf("router pod %q is not running (phase: %s)", podName, pod.Status.Phase)
	}

	container := "router"

	// Parse timeout
	timeoutSec := 30
	if tf.Spec.Timeout != "" {
		d, err := time.ParseDuration(tf.Spec.Timeout)
		if err == nil && d > 0 {
			timeoutSec = int(d.Seconds())
		}
	}

	dest := tf.Spec.Destination.IP
	protocol := tf.Spec.Destination.Protocol
	if protocol == "" {
		protocol = "ICMP"
	}
	var port int32
	if tf.Spec.Destination.Port != nil {
		port = *tf.Spec.Destination.Port
	}

	// Step 1: Capture nft counters before
	nftBeforeOutput, _, err := r.ExecFn(tf.Namespace, podName, container, []string{"nft", "list", "ruleset"})
	if err != nil {
		logger.Info("nft list ruleset failed (before), continuing without nft data", "error", err)
		nftBeforeOutput = ""
	}
	nftBefore := parseNftCounters(nftBeforeOutput)

	// Step 2: Run traceroute
	tracerouteCmd := buildTracerouteCommand(dest, timeoutSec)
	tracerouteOutput, _, err := r.ExecFn(tf.Namespace, podName, container, tracerouteCmd)
	if err != nil {
		logger.Info("Traceroute failed, continuing with probe", "error", err)
	}

	// Step 3: Run probe
	probeCmd := buildProbeCommand(dest, port, protocol, timeoutSec)
	probeStdout, probeStderr, probeErr := r.ExecFn(tf.Namespace, podName, container, probeCmd)

	// Step 4: Capture nft counters after
	nftAfterOutput, _, err := r.ExecFn(tf.Namespace, podName, container, []string{"nft", "list", "ruleset"})
	if err != nil {
		logger.Info("nft list ruleset failed (after), continuing without nft data", "error", err)
		nftAfterOutput = ""
	}
	nftAfter := parseNftCounters(nftAfterOutput)

	// Build hops from traceroute output
	parsedHops := parseTracerouteOutput(tracerouteOutput)
	nftHits := nftCountersDiff(nftBefore, nftAfter)

	var hops []v1alpha1.TraceflowHop
	for _, h := range parsedHops {
		hop := v1alpha1.TraceflowHop{
			Order:     h.HopNum,
			Node:      h.IP,
			Component: "router",
			Action:    "forward",
			Latency:   h.Latency,
		}
		hops = append(hops, hop)
	}

	// Add nft hits to the last hop (or create a synthetic hop)
	if len(nftHits) > 0 {
		if len(hops) > 0 {
			hops[len(hops)-1].NftablesHits = nftHits
		} else {
			hops = append(hops, v1alpha1.TraceflowHop{
				Order:        1,
				Node:         podName,
				Component:    "nftables",
				Action:       "filter",
				NftablesHits: nftHits,
			})
		}
	}

	// Determine result
	result := determineResult(probeErr, probeStdout, probeStderr, nftHits)

	// Calculate total latency from last hop
	var totalLatency string
	if len(parsedHops) > 0 {
		totalLatency = parsedHops[len(parsedHops)-1].Latency
	}

	// Update status to Completed
	completionTime := metav1.Now()
	tf.Status.Phase = "Completed"
	tf.Status.Hops = hops
	tf.Status.Result = result
	tf.Status.TotalLatency = totalLatency
	tf.Status.CompletionTime = &completionTime
	tf.Status.Message = fmt.Sprintf("Traceflow to %s completed: %s", dest, result)
	meta.SetStatusCondition(&tf.Status.Conditions, metav1.Condition{
		Type:               "Complete",
		Status:             metav1.ConditionTrue,
		Reason:             "ProbeCompleted",
		Message:            tf.Status.Message,
		LastTransitionTime: completionTime,
	})
	if err := r.Status().Update(ctx, tf); err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	r.Recorder.Event(tf, corev1.EventTypeNormal, "TraceflowCompleted",
		fmt.Sprintf("Traceflow to %s: %s", dest, result))

	return nil
}

// determineResult determines the traceflow result based on probe output and nft hits.
func determineResult(probeErr error, stdout, stderr string, nftHits []v1alpha1.NFTablesRuleHit) string {
	if probeErr == nil {
		return "Reachable"
	}

	// Check for timeout indicators
	combined := stdout + stderr
	if strings.Contains(combined, "timed out") ||
		strings.Contains(combined, "100% packet loss") ||
		strings.Contains(combined, "Unreachable") {
		// Check if any drop rules were hit
		for _, hit := range nftHits {
			if strings.Contains(hit.Rule, "drop") || strings.Contains(hit.Rule, "reject") {
				return "Filtered"
			}
		}
		if strings.Contains(combined, "timed out") || strings.Contains(combined, "100% packet loss") {
			return "Timeout"
		}
		return "Unreachable"
	}

	// Check for filtering
	for _, hit := range nftHits {
		if strings.Contains(hit.Rule, "drop") || strings.Contains(hit.Rule, "reject") {
			return "Filtered"
		}
	}

	return "Unreachable"
}

// setFailed sets the traceflow phase to Failed and records the error.
func (r *Reconciler) setFailed(ctx context.Context, tf *v1alpha1.VPCTraceflow, probeErr error) (ctrl.Result, error) {
	now := metav1.Now()
	tf.Status.Phase = "Failed"
	tf.Status.Message = probeErr.Error()
	tf.Status.CompletionTime = &now
	meta.SetStatusCondition(&tf.Status.Conditions, metav1.Condition{
		Type:               "Complete",
		Status:             metav1.ConditionFalse,
		Reason:             "ProbeFailed",
		Message:            probeErr.Error(),
		LastTransitionTime: now,
	})
	if err := r.Status().Update(ctx, tf); err != nil {
		return ctrl.Result{}, err
	}
	r.Recorder.Event(tf, corev1.EventTypeWarning, "TraceflowFailed", probeErr.Error())
	return ctrl.Result{}, nil
}

// parseDurationWithDefault parses a duration string, returning the default if empty or invalid.
func parseDurationWithDefault(s string, defaultDur time.Duration) (time.Duration, error) {
	if s == "" {
		return defaultDur, nil
	}
	return time.ParseDuration(s)
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Recorder = mgr.GetEventRecorderFor("vpctraceflow-controller")
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.VPCTraceflow{}).
		Complete(r)
}
