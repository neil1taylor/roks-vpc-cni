package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/IBM/roks-vpc-network-operator/cmd/bff/internal/thanos"
)

// RouterMetricsHandler serves per-router metrics from Thanos/Prometheus.
type RouterMetricsHandler struct {
	thanos *thanos.Client
}

// NewRouterMetricsHandler creates a handler. Returns nil if thanos is nil.
func NewRouterMetricsHandler(thanos *thanos.Client) *RouterMetricsHandler {
	if thanos == nil {
		return nil
	}
	return &RouterMetricsHandler{thanos: thanos}
}

// --- Response types ---

type RouterHealthSummary struct {
	Status         string             `json:"status"`
	UptimeSeconds  float64            `json:"uptimeSeconds"`
	Interfaces     []InterfaceSummary `json:"interfaces"`
	ConntrackPct   float64            `json:"conntrackPct"`
	Processes      map[string]bool    `json:"processes"`
	MetricsEnabled bool               `json:"metricsEnabled"`
}

type InterfaceSummary struct {
	Name    string  `json:"name"`
	RxBps   float64 `json:"rxBps"`
	TxBps   float64 `json:"txBps"`
	RxErrs  float64 `json:"rxErrors"`
	TxErrs  float64 `json:"txErrors"`
}

type InterfaceTimeSeries struct {
	Interface string      `json:"interface"`
	RxBps     []DataPoint `json:"rxBps"`
	TxBps     []DataPoint `json:"txBps"`
	RxPps     []DataPoint `json:"rxPps"`
	TxPps     []DataPoint `json:"txPps"`
}

type DataPoint struct {
	T int64   `json:"t"`
	V float64 `json:"v"`
}

type ConntrackTimeSeries struct {
	Entries    []DataPoint `json:"entries"`
	Max        float64     `json:"max"`
	Percentage float64     `json:"percentage"`
}

type DHCPPoolMetrics struct {
	Interface    string  `json:"interface"`
	ActiveLeases float64 `json:"activeLeases"`
	PoolSize     float64 `json:"poolSize"`
	Utilization  float64 `json:"utilization"`
}

type NFTRuleMetrics struct {
	Table   string  `json:"table"`
	Chain   string  `json:"chain"`
	Comment string  `json:"comment"`
	Packets float64 `json:"packets"`
	Bytes   float64 `json:"bytes"`
}

// --- Handlers ---

// GetSummary returns aggregated health for a router.
// GET /api/v1/routers/{name}/metrics/summary?namespace=...
func (h *RouterMetricsHandler) GetSummary(w http.ResponseWriter, r *http.Request) {
	router, ns := extractRouterNameNS(r)
	if router == "" || ns == "" {
		WriteError(w, http.StatusBadRequest, "router name and namespace required", "MISSING_PARAMS")
		return
	}

	ctx := r.Context()
	podSelector := fmt.Sprintf(`{pod=~"%s-pod"}`, router)

	// Uptime
	uptimeResult, _ := h.thanos.QueryInstant(ctx, fmt.Sprintf(`router_uptime_seconds%s`, podSelector))
	uptime := extractFirstValue(uptimeResult)

	// Interface rates
	rxRateResult, _ := h.thanos.QueryInstant(ctx,
		fmt.Sprintf(`rate(router_interface_rx_bytes_total%s[1m])`, podSelector))
	txRateResult, _ := h.thanos.QueryInstant(ctx,
		fmt.Sprintf(`rate(router_interface_tx_bytes_total%s[1m])`, podSelector))

	interfaces := buildInterfaceSummaries(rxRateResult, txRateResult)

	// Conntrack
	ctCountResult, _ := h.thanos.QueryInstant(ctx, fmt.Sprintf(`router_conntrack_entries%s`, podSelector))
	ctMaxResult, _ := h.thanos.QueryInstant(ctx, fmt.Sprintf(`router_conntrack_max%s`, podSelector))
	ctCount := extractFirstValue(ctCountResult)
	ctMax := extractFirstValue(ctMaxResult)
	ctPct := 0.0
	if ctMax > 0 {
		ctPct = (ctCount / ctMax) * 100
	}

	// Processes
	processResult, _ := h.thanos.QueryInstant(ctx, fmt.Sprintf(`router_process_running%s`, podSelector))
	processes := extractProcessStatus(processResult)

	summary := RouterHealthSummary{
		Status:         "ok",
		UptimeSeconds:  uptime,
		Interfaces:     interfaces,
		ConntrackPct:   ctPct,
		Processes:      processes,
		MetricsEnabled: true,
	}

	WriteJSON(w, http.StatusOK, summary)
}

// GetInterfaces returns per-interface time series.
// GET /api/v1/routers/{name}/metrics/interfaces?namespace=...&range=1h&step=1m
func (h *RouterMetricsHandler) GetInterfaces(w http.ResponseWriter, r *http.Request) {
	router, ns := extractRouterNameNS(r)
	if router == "" || ns == "" {
		WriteError(w, http.StatusBadRequest, "router name and namespace required", "MISSING_PARAMS")
		return
	}

	rangeStr := r.URL.Query().Get("range")
	if rangeStr == "" {
		rangeStr = "1h"
	}
	step := r.URL.Query().Get("step")
	if step == "" {
		step = "1m"
	}

	ctx := r.Context()
	end := time.Now()
	start := end.Add(-parseDuration(rangeStr))
	startStr := fmt.Sprintf("%d", start.Unix())
	endStr := fmt.Sprintf("%d", end.Unix())
	podSelector := fmt.Sprintf(`{pod=~"%s-pod"}`, router)

	rxResult, _ := h.thanos.QueryRange(ctx,
		fmt.Sprintf(`rate(router_interface_rx_bytes_total%s[1m])`, podSelector),
		startStr, endStr, step)
	txResult, _ := h.thanos.QueryRange(ctx,
		fmt.Sprintf(`rate(router_interface_tx_bytes_total%s[1m])`, podSelector),
		startStr, endStr, step)
	rxPpsResult, _ := h.thanos.QueryRange(ctx,
		fmt.Sprintf(`rate(router_interface_rx_packets_total%s[1m])`, podSelector),
		startStr, endStr, step)
	txPpsResult, _ := h.thanos.QueryRange(ctx,
		fmt.Sprintf(`rate(router_interface_tx_packets_total%s[1m])`, podSelector),
		startStr, endStr, step)

	series := buildInterfaceTimeSeries(rxResult, txResult, rxPpsResult, txPpsResult)

	WriteJSON(w, http.StatusOK, series)
}

// GetConntrack returns conntrack time series.
// GET /api/v1/routers/{name}/metrics/conntrack?namespace=...&range=1h&step=1m
func (h *RouterMetricsHandler) GetConntrack(w http.ResponseWriter, r *http.Request) {
	router, ns := extractRouterNameNS(r)
	if router == "" || ns == "" {
		WriteError(w, http.StatusBadRequest, "router name and namespace required", "MISSING_PARAMS")
		return
	}

	rangeStr := r.URL.Query().Get("range")
	if rangeStr == "" {
		rangeStr = "1h"
	}
	step := r.URL.Query().Get("step")
	if step == "" {
		step = "1m"
	}

	ctx := r.Context()
	end := time.Now()
	start := end.Add(-parseDuration(rangeStr))
	startStr := fmt.Sprintf("%d", start.Unix())
	endStr := fmt.Sprintf("%d", end.Unix())
	podSelector := fmt.Sprintf(`{pod=~"%s-pod"}`, router)

	entriesResult, _ := h.thanos.QueryRange(ctx,
		fmt.Sprintf(`router_conntrack_entries%s`, podSelector),
		startStr, endStr, step)
	maxResult, _ := h.thanos.QueryInstant(ctx,
		fmt.Sprintf(`router_conntrack_max%s`, podSelector))

	maxVal := extractFirstValue(maxResult)
	entries := extractTimeSeries(entriesResult)
	pct := 0.0
	if maxVal > 0 && len(entries) > 0 {
		lastVal := entries[len(entries)-1].V
		pct = (lastVal / maxVal) * 100
	}

	WriteJSON(w, http.StatusOK, ConntrackTimeSeries{
		Entries:    entries,
		Max:        maxVal,
		Percentage: pct,
	})
}

// GetDHCP returns per-interface DHCP pool utilization.
// GET /api/v1/routers/{name}/metrics/dhcp?namespace=...
func (h *RouterMetricsHandler) GetDHCP(w http.ResponseWriter, r *http.Request) {
	router, ns := extractRouterNameNS(r)
	if router == "" || ns == "" {
		WriteError(w, http.StatusBadRequest, "router name and namespace required", "MISSING_PARAMS")
		return
	}

	ctx := r.Context()
	podSelector := fmt.Sprintf(`{pod=~"%s-pod"}`, router)

	leasesResult, _ := h.thanos.QueryInstant(ctx,
		fmt.Sprintf(`router_dhcp_active_leases%s`, podSelector))
	poolResult, _ := h.thanos.QueryInstant(ctx,
		fmt.Sprintf(`router_dhcp_pool_size%s`, podSelector))

	pools := buildDHCPPools(leasesResult, poolResult)

	WriteJSON(w, http.StatusOK, pools)
}

// GetNFT returns nftables rule counters.
// GET /api/v1/routers/{name}/metrics/nft?namespace=...
func (h *RouterMetricsHandler) GetNFT(w http.ResponseWriter, r *http.Request) {
	router, ns := extractRouterNameNS(r)
	if router == "" || ns == "" {
		WriteError(w, http.StatusBadRequest, "router name and namespace required", "MISSING_PARAMS")
		return
	}

	ctx := r.Context()
	podSelector := fmt.Sprintf(`{pod=~"%s-pod"}`, router)

	packetsResult, _ := h.thanos.QueryInstant(ctx,
		fmt.Sprintf(`router_nft_rule_packets_total%s`, podSelector))
	bytesResult, _ := h.thanos.QueryInstant(ctx,
		fmt.Sprintf(`router_nft_rule_bytes_total%s`, podSelector))

	rules := buildNFTRules(packetsResult, bytesResult)

	WriteJSON(w, http.StatusOK, rules)
}

// --- Helpers ---

func extractRouterNameNS(r *http.Request) (string, string) {
	// URL pattern: /api/v1/routers/{name}/metrics/{endpoint}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/routers/")
	parts := strings.SplitN(path, "/", 3) // name, "metrics", endpoint
	name := ""
	if len(parts) >= 1 {
		name = parts[0]
	}
	ns := r.URL.Query().Get("namespace")
	return name, ns
}

func extractFirstValue(result *thanos.QueryResult) float64 {
	if result == nil || len(result.Data.Result) == 0 {
		return 0
	}
	ds := result.Data.Result[0]
	if len(ds.Value) >= 2 {
		return parseValueString(ds.Value[1])
	}
	return 0
}

func parseValueString(v interface{}) float64 {
	switch val := v.(type) {
	case string:
		f, _ := strconv.ParseFloat(val, 64)
		return f
	case float64:
		return val
	default:
		return 0
	}
}

func extractTimeSeries(result *thanos.QueryResult) []DataPoint {
	if result == nil || len(result.Data.Result) == 0 {
		return nil
	}
	var points []DataPoint
	for _, vals := range result.Data.Result[0].Values {
		if len(vals) >= 2 {
			t := int64(0)
			if ts, ok := vals[0].(float64); ok {
				t = int64(ts)
			}
			v := parseValueString(vals[1])
			points = append(points, DataPoint{T: t, V: v})
		}
	}
	return points
}

func buildInterfaceSummaries(rxResult, txResult *thanos.QueryResult) []InterfaceSummary {
	rxMap := make(map[string]float64)
	txMap := make(map[string]float64)

	if rxResult != nil {
		for _, ds := range rxResult.Data.Result {
			iface := ds.Metric["interface"]
			rxMap[iface] = parseValueString(ds.Value[1])
		}
	}
	if txResult != nil {
		for _, ds := range txResult.Data.Result {
			iface := ds.Metric["interface"]
			txMap[iface] = parseValueString(ds.Value[1])
		}
	}

	// Merge
	allIfaces := make(map[string]bool)
	for k := range rxMap {
		allIfaces[k] = true
	}
	for k := range txMap {
		allIfaces[k] = true
	}

	var summaries []InterfaceSummary
	for iface := range allIfaces {
		summaries = append(summaries, InterfaceSummary{
			Name:  iface,
			RxBps: rxMap[iface],
			TxBps: txMap[iface],
		})
	}
	return summaries
}

func buildInterfaceTimeSeries(rxResult, txResult, rxPpsResult, txPpsResult *thanos.QueryResult) []InterfaceTimeSeries {
	seriesMap := make(map[string]*InterfaceTimeSeries)

	extractSeries := func(result *thanos.QueryResult, field string) {
		if result == nil {
			return
		}
		for _, ds := range result.Data.Result {
			iface := ds.Metric["interface"]
			ts, ok := seriesMap[iface]
			if !ok {
				ts = &InterfaceTimeSeries{Interface: iface}
				seriesMap[iface] = ts
			}
			var points []DataPoint
			for _, vals := range ds.Values {
				if len(vals) >= 2 {
					t := int64(0)
					if tsVal, ok := vals[0].(float64); ok {
						t = int64(tsVal)
					}
					points = append(points, DataPoint{T: t, V: parseValueString(vals[1])})
				}
			}
			switch field {
			case "rxBps":
				ts.RxBps = points
			case "txBps":
				ts.TxBps = points
			case "rxPps":
				ts.RxPps = points
			case "txPps":
				ts.TxPps = points
			}
		}
	}

	extractSeries(rxResult, "rxBps")
	extractSeries(txResult, "txBps")
	extractSeries(rxPpsResult, "rxPps")
	extractSeries(txPpsResult, "txPps")

	var result []InterfaceTimeSeries
	for _, ts := range seriesMap {
		result = append(result, *ts)
	}
	return result
}

func extractProcessStatus(result *thanos.QueryResult) map[string]bool {
	processes := make(map[string]bool)
	if result == nil {
		return processes
	}
	for _, ds := range result.Data.Result {
		name := ds.Metric["process"]
		val := parseValueString(ds.Value[1])
		processes[name] = val == 1
	}
	return processes
}

func buildDHCPPools(leasesResult, poolResult *thanos.QueryResult) []DHCPPoolMetrics {
	leaseMap := make(map[string]float64)
	poolMap := make(map[string]float64)

	if leasesResult != nil {
		for _, ds := range leasesResult.Data.Result {
			iface := ds.Metric["interface"]
			leaseMap[iface] = parseValueString(ds.Value[1])
		}
	}
	if poolResult != nil {
		for _, ds := range poolResult.Data.Result {
			iface := ds.Metric["interface"]
			poolMap[iface] = parseValueString(ds.Value[1])
		}
	}

	allIfaces := make(map[string]bool)
	for k := range leaseMap {
		allIfaces[k] = true
	}
	for k := range poolMap {
		allIfaces[k] = true
	}

	var pools []DHCPPoolMetrics
	for iface := range allIfaces {
		leases := leaseMap[iface]
		size := poolMap[iface]
		util := 0.0
		if size > 0 {
			util = (leases / size) * 100
		}
		pools = append(pools, DHCPPoolMetrics{
			Interface:    iface,
			ActiveLeases: leases,
			PoolSize:     size,
			Utilization:  util,
		})
	}
	return pools
}

func buildNFTRules(packetsResult, bytesResult *thanos.QueryResult) []NFTRuleMetrics {
	type ruleKey struct {
		table, chain, comment string
	}
	pktMap := make(map[ruleKey]float64)
	byteMap := make(map[ruleKey]float64)

	if packetsResult != nil {
		for _, ds := range packetsResult.Data.Result {
			key := ruleKey{
				table:   ds.Metric["table"],
				chain:   ds.Metric["chain"],
				comment: ds.Metric["comment"],
			}
			pktMap[key] = parseValueString(ds.Value[1])
		}
	}
	if bytesResult != nil {
		for _, ds := range bytesResult.Data.Result {
			key := ruleKey{
				table:   ds.Metric["table"],
				chain:   ds.Metric["chain"],
				comment: ds.Metric["comment"],
			}
			byteMap[key] = parseValueString(ds.Value[1])
		}
	}

	allKeys := make(map[ruleKey]bool)
	for k := range pktMap {
		allKeys[k] = true
	}
	for k := range byteMap {
		allKeys[k] = true
	}

	var rules []NFTRuleMetrics
	for key := range allKeys {
		rules = append(rules, NFTRuleMetrics{
			Table:   key.table,
			Chain:   key.chain,
			Comment: key.comment,
			Packets: pktMap[key],
			Bytes:   byteMap[key],
		})
	}
	return rules
}

func parseDuration(s string) time.Duration {
	// Simple parser for Prometheus-style durations: 5m, 15m, 1h, 6h, 24h
	if len(s) < 2 {
		return time.Hour
	}
	unit := s[len(s)-1]
	val, err := strconv.Atoi(s[:len(s)-1])
	if err != nil {
		return time.Hour
	}
	switch unit {
	case 's':
		return time.Duration(val) * time.Second
	case 'm':
		return time.Duration(val) * time.Minute
	case 'h':
		return time.Duration(val) * time.Hour
	case 'd':
		return time.Duration(val) * 24 * time.Hour
	default:
		return time.Hour
	}
}
