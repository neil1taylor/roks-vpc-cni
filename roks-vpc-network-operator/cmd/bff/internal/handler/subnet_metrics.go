package handler

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/IBM/roks-vpc-network-operator/cmd/bff/internal/thanos"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// SubnetMetricsResponse holds combined metrics for a subnet.
type SubnetMetricsResponse struct {
	ThroughputRx []DataPoint `json:"throughputRx"`
	ThroughputTx []DataPoint `json:"throughputTx"`
	DHCPPoolSize int         `json:"dhcpPoolSize"`
	DHCPActive   int         `json:"dhcpActiveLeases"`
	DHCPUtilPct  float64     `json:"dhcpUtilizationPct"`
}

// SubnetMetricsHandler serves per-subnet metrics from Thanos/Prometheus
// by cross-referencing VPCRouter network attachments with the queried subnet.
type SubnetMetricsHandler struct {
	thanos    *thanos.Client
	dynClient dynamic.Interface
}

// NewSubnetMetricsHandler creates a handler. Returns nil if thanos is nil.
func NewSubnetMetricsHandler(thanosClient *thanos.Client, dynClient dynamic.Interface) *SubnetMetricsHandler {
	if thanosClient == nil {
		return nil
	}
	return &SubnetMetricsHandler{
		thanos:    thanosClient,
		dynClient: dynClient,
	}
}

// GetSubnetMetrics handles GET /api/v1/subnets/{name}/metrics
// Query params: namespace, range (default 1h), step (default 1m)
func (h *SubnetMetricsHandler) GetSubnetMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	// Extract subnet name from path: /api/v1/subnets/{name}/metrics
	subnetName := extractSubnetNameFromMetricsPath(r.URL.Path)
	if subnetName == "" {
		WriteError(w, http.StatusBadRequest, "subnet name required", "MISSING_PARAMS")
		return
	}

	namespace := r.URL.Query().Get("namespace")
	rangeStr := r.URL.Query().Get("range")
	if rangeStr == "" {
		rangeStr = "1h"
	}
	step := r.URL.Query().Get("step")
	if step == "" {
		step = "1m"
	}

	ctx := r.Context()
	slog.DebugContext(ctx, "fetching subnet metrics", "subnet", subnetName, "namespace", namespace, "range", rangeStr)

	// Cross-reference subnet → VPCRouter network → interface name
	routerName, ifaceName := h.resolveSubnetToInterface(ctx, subnetName, namespace)

	resp := SubnetMetricsResponse{}

	if routerName == "" || ifaceName == "" {
		slog.DebugContext(ctx, "no router/interface found for subnet, returning empty metrics", "subnet", subnetName)
		WriteJSON(w, http.StatusOK, resp)
		return
	}

	podSelector := fmt.Sprintf(`pod=~"%s-pod"`, routerName)

	// Time range
	end := time.Now()
	start := end.Add(-parseDuration(rangeStr))
	startStr := fmt.Sprintf("%d", start.Unix())
	endStr := fmt.Sprintf("%d", end.Unix())

	// Throughput RX
	rxResult, _ := h.thanos.QueryRange(ctx,
		fmt.Sprintf(`rate(router_interface_rx_bytes_total{%s,interface="%s"}[1m])`, podSelector, ifaceName),
		startStr, endStr, step)

	// Throughput TX
	txResult, _ := h.thanos.QueryRange(ctx,
		fmt.Sprintf(`rate(router_interface_tx_bytes_total{%s,interface="%s"}[1m])`, podSelector, ifaceName),
		startStr, endStr, step)

	resp.ThroughputRx = extractTimeSeries(rxResult)
	resp.ThroughputTx = extractTimeSeries(txResult)

	// DHCP metrics for this interface
	leasesResult, _ := h.thanos.QueryInstant(ctx,
		fmt.Sprintf(`router_dhcp_active_leases{%s,interface="%s"}`, podSelector, ifaceName))
	poolResult, _ := h.thanos.QueryInstant(ctx,
		fmt.Sprintf(`router_dhcp_pool_size{%s,interface="%s"}`, podSelector, ifaceName))

	activeLeases := extractFirstValue(leasesResult)
	poolSize := extractFirstValue(poolResult)

	resp.DHCPActive = int(activeLeases)
	resp.DHCPPoolSize = int(poolSize)
	if poolSize > 0 {
		resp.DHCPUtilPct = (activeLeases / poolSize) * 100
	}

	WriteJSON(w, http.StatusOK, resp)
}

// resolveSubnetToInterface finds the VPCRouter and interface name associated
// with the given subnet name by scanning VPCRouter spec.networks[].
func (h *SubnetMetricsHandler) resolveSubnetToInterface(ctx context.Context, subnetName, namespace string) (string, string) {
	if h.dynClient == nil {
		return "", ""
	}

	vpcRouterGVR := schema.GroupVersionResource{
		Group:    "vpc.roks.ibm.com",
		Version:  "v1alpha1",
		Resource: "vpcrouters",
	}

	var routers *unstructured.UnstructuredList
	var err error

	if namespace != "" {
		routers, err = h.dynClient.Resource(vpcRouterGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	} else {
		routers, err = h.dynClient.Resource(vpcRouterGVR).Namespace("").List(ctx, metav1.ListOptions{})
	}
	if err != nil {
		slog.DebugContext(ctx, "failed to list VPCRouters", "error", err)
		return "", ""
	}

	for _, router := range routers.Items {
		routerName := router.GetName()
		networks, found, _ := unstructured.NestedSlice(router.Object, "spec", "networks")
		if !found {
			continue
		}
		for i, n := range networks {
			netMap, ok := n.(map[string]interface{})
			if !ok {
				continue
			}
			netName, _ := netMap["name"].(string)
			if netName == subnetName {
				// Interface naming convention: net0, net1, net2, etc.
				// Maps to the order in spec.networks array.
				ifaceName := fmt.Sprintf("net%d", i)
				slog.DebugContext(ctx, "resolved subnet to interface",
					"subnet", subnetName, "router", routerName, "interface", ifaceName)
				return routerName, ifaceName
			}
		}
	}

	return "", ""
}

// extractSubnetNameFromMetricsPath extracts the subnet name from path
// /api/v1/subnets/{name}/metrics -> {name}
func extractSubnetNameFromMetricsPath(path string) string {
	trimmed := strings.TrimPrefix(path, "/api/v1/subnets/")
	parts := strings.SplitN(trimmed, "/", 2)
	if len(parts) >= 1 && parts[0] != "" {
		return parts[0]
	}
	return ""
}
