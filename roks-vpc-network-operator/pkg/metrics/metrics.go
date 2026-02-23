package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	// VPCAPICallsTotal counts VPC API calls by method and status.
	VPCAPICallsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "vpc_api_calls_total",
			Help: "Total number of VPC API calls",
		},
		[]string{"method", "status"},
	)

	// VPCAPILatency tracks VPC API call latency in seconds.
	VPCAPILatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "vpc_api_latency_seconds",
			Help:    "Latency of VPC API calls in seconds",
			Buckets: prometheus.ExponentialBuckets(0.1, 2, 10), // 0.1s to ~51s
		},
		[]string{"method"},
	)

	// ReconcileOpsTotal counts reconcile operations by controller and result.
	ReconcileOpsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "reconcile_operations_total",
			Help: "Total reconcile operations by controller and result",
		},
		[]string{"controller", "operation", "result"},
	)

	// OrphanGCDeletionsTotal counts orphan GC deletions by resource type.
	OrphanGCDeletionsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "orphan_gc_deletions_total",
			Help: "Total orphan GC deletions by resource type",
		},
		[]string{"resource_type"},
	)

	// ManagedResourcesGauge tracks the count of managed VPC resources.
	ManagedResourcesGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "managed_resources",
			Help: "Current count of managed VPC resources",
		},
		[]string{"resource_type"},
	)
)

func init() {
	metrics.Registry.MustRegister(
		VPCAPICallsTotal,
		VPCAPILatency,
		ReconcileOpsTotal,
		OrphanGCDeletionsTotal,
		ManagedResourcesGauge,
	)
}
