package vpngateway

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	statusExporterPort = 9190
	statusFetchTimeout = 5 * time.Second
)

// OpenVPNClientStatus represents a single connected client parsed from the
// OpenVPN status.log file.
type OpenVPNClientStatus struct {
	CommonName     string
	RealAddress    string
	BytesReceived  int64
	BytesSent      int64
	ConnectedSince string
	VirtualAddress string
}

// parseOpenVPNStatusLog parses the CSV-like OpenVPN status.log format and
// returns a list of connected clients. Malformed lines are skipped.
func parseOpenVPNStatusLog(raw string) []OpenVPNClientStatus {
	lines := strings.Split(raw, "\n")

	// First pass: parse CLIENT LIST
	clients := map[string]*OpenVPNClientStatus{}
	section := ""

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || line == "END" {
			continue
		}

		// Detect section transitions
		if strings.HasPrefix(line, "OpenVPN CLIENT LIST") {
			section = "clients"
			continue
		}
		if strings.HasPrefix(line, "ROUTING TABLE") {
			section = "routing"
			continue
		}
		if strings.HasPrefix(line, "GLOBAL STATS") {
			section = ""
			continue
		}

		// Skip headers
		if strings.HasPrefix(line, "Updated,") ||
			strings.HasPrefix(line, "Common Name,") ||
			strings.HasPrefix(line, "Virtual Address,") ||
			strings.HasPrefix(line, "Max bcast/mcast") {
			continue
		}

		switch section {
		case "clients":
			// Common Name,Real Address,Bytes Received,Bytes Sent,Connected Since
			parts := strings.SplitN(line, ",", 5)
			if len(parts) < 5 {
				continue
			}
			bytesRecv, _ := strconv.ParseInt(parts[2], 10, 64)
			bytesSent, _ := strconv.ParseInt(parts[3], 10, 64)
			clients[parts[0]] = &OpenVPNClientStatus{
				CommonName:     parts[0],
				RealAddress:    parts[1],
				BytesReceived:  bytesRecv,
				BytesSent:      bytesSent,
				ConnectedSince: parts[4],
			}

		case "routing":
			// Virtual Address,Common Name,Real Address,Last Ref
			parts := strings.SplitN(line, ",", 4)
			if len(parts) < 2 {
				continue
			}
			if c, ok := clients[parts[1]]; ok {
				c.VirtualAddress = parts[0]
			}
		}
	}

	result := make([]OpenVPNClientStatus, 0, len(clients))
	for _, c := range clients {
		result = append(result, *c)
	}
	return result
}

// fetchOpenVPNStatus fetches the OpenVPN status.log from the sidecar HTTP
// server running in the VPN pod and parses it into client statuses.
func fetchOpenVPNStatus(podIP string, port int) ([]OpenVPNClientStatus, error) {
	client := &http.Client{Timeout: statusFetchTimeout}
	url := fmt.Sprintf("http://%s:%d/status", podIP, port)

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch OpenVPN status: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenVPN status endpoint returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read OpenVPN status response: %w", err)
	}

	return parseOpenVPNStatusLog(string(body)), nil
}
