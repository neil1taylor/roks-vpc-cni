package handler

import (
	"log/slog"
	"net/http"
	"sort"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// AlertTimelineEntry represents a single alert or event in the timeline.
type AlertTimelineEntry struct {
	Timestamp   time.Time    `json:"timestamp"`
	Severity    string       `json:"severity"`    // info, warning, critical
	Source      string       `json:"source"`      // k8s-event, prometheus-alert
	Message     string       `json:"message"`
	ResourceRef *ResourceRef `json:"resourceRef,omitempty"`
}

// ResourceRef identifies the Kubernetes resource related to an alert.
type ResourceRef struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

// AlertsHandler handles alert timeline API operations.
type AlertsHandler struct {
	k8sClient kubernetes.Interface
}

// NewAlertsHandler creates a new alerts handler.
func NewAlertsHandler(k8sClient kubernetes.Interface) *AlertsHandler {
	return &AlertsHandler{k8sClient: k8sClient}
}

// vpcCRDKinds is the set of VPC CRD resource kinds we care about for alert filtering.
var vpcCRDKinds = map[string]bool{
	"VPCSubnet":              true,
	"VirtualNetworkInterface": true,
	"VLANAttachment":         true,
	"FloatingIP":             true,
	"VPCGateway":             true,
	"VPCRouter":              true,
	"VPCL2Bridge":            true,
	"VPCVPNGateway":          true,
	"VPCDNSPolicy":           true,
}

// GetTimeline handles GET /api/v1/alerts/timeline
// Returns a merged, sorted list of K8s Warning events for VPC CRD resources.
// Optional query param: ?range=1h|6h|24h|7d (default 24h)
func (h *AlertsHandler) GetTimeline(w http.ResponseWriter, r *http.Request) {
	if h.k8sClient == nil {
		WriteError(w, http.StatusServiceUnavailable, "kubernetes client not configured", "CLIENT_NOT_CONFIGURED")
		return
	}

	// Parse time range (default 24h)
	rangeParam := GetQueryParam(r, "range")
	if rangeParam == "" {
		rangeParam = "24h"
	}
	duration := parseDuration(rangeParam)
	cutoff := time.Now().Add(-duration)

	// Fetch Warning events across all namespaces
	eventList, err := h.k8sClient.CoreV1().Events("").List(r.Context(), metav1.ListOptions{
		FieldSelector: "type=Warning",
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to list events", "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to list events", "LIST_FAILED")
		return
	}

	entries := make([]AlertTimelineEntry, 0, len(eventList.Items))
	for _, event := range eventList.Items {
		// Filter to VPC CRD resource kinds
		if !vpcCRDKinds[event.InvolvedObject.Kind] {
			continue
		}

		// Determine event timestamp (prefer LastTimestamp, then EventTime, then FirstTimestamp)
		var ts time.Time
		if !event.LastTimestamp.IsZero() {
			ts = event.LastTimestamp.Time
		} else if !event.EventTime.IsZero() {
			ts = event.EventTime.Time
		} else if !event.FirstTimestamp.IsZero() {
			ts = event.FirstTimestamp.Time
		} else {
			ts = event.CreationTimestamp.Time
		}

		// Filter by time range
		if ts.Before(cutoff) {
			continue
		}

		// Map event reason to severity
		severity := mapSeverity(event.Reason)

		entries = append(entries, AlertTimelineEntry{
			Timestamp: ts,
			Severity:  severity,
			Source:    "k8s-event",
			Message:  event.Message,
			ResourceRef: &ResourceRef{
				Kind:      event.InvolvedObject.Kind,
				Name:      event.InvolvedObject.Name,
				Namespace: event.InvolvedObject.Namespace,
			},
		})
	}

	// Sort by timestamp descending (newest first)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp.After(entries[j].Timestamp)
	})

	// Cap at 100 entries
	if len(entries) > 100 {
		entries = entries[:100]
	}

	WriteJSON(w, http.StatusOK, entries)
}

// parseDuration is defined in router_metrics.go — reuse it here.

// mapSeverity maps a Kubernetes event reason to an alert severity level.
func mapSeverity(reason string) string {
	switch reason {
	case "CreateFailed", "DeleteFailed", "ReconcileError", "SyncFailed", "Error":
		return "critical"
	case "DriftDetected", "OrphanDetected", "Degraded", "Unhealthy":
		return "warning"
	default:
		return "warning"
	}
}
