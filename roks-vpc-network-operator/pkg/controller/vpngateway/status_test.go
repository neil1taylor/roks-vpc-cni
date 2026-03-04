package vpngateway

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

const twoClientStatusLog = `OpenVPN CLIENT LIST
Updated,Thu Mar 04 10:00:00 2026
Common Name,Real Address,Bytes Received,Bytes Sent,Connected Since
client1,203.0.113.5:51234,12345678,87654321,Thu Mar 04 09:00:00 2026
client2,198.51.100.10:44210,9876543,11223344,Thu Mar 04 09:30:00 2026
ROUTING TABLE
Virtual Address,Common Name,Real Address,Last Ref
10.8.0.6,client1,203.0.113.5:51234,Thu Mar 04 09:59:00 2026
10.8.0.10,client2,198.51.100.10:44210,Thu Mar 04 09:58:00 2026
GLOBAL STATS
Max bcast/mcast queue length,0
END
`

func TestParseOpenVPNStatusLog_TwoClients(t *testing.T) {
	clients := parseOpenVPNStatusLog(twoClientStatusLog)

	if len(clients) != 2 {
		t.Fatalf("expected 2 clients, got %d", len(clients))
	}

	// Build map for order-independent verification
	byName := map[string]OpenVPNClientStatus{}
	for _, c := range clients {
		byName[c.CommonName] = c
	}

	c1, ok := byName["client1"]
	if !ok {
		t.Fatal("missing client1")
	}
	if c1.RealAddress != "203.0.113.5:51234" {
		t.Errorf("client1 RealAddress = %q, want %q", c1.RealAddress, "203.0.113.5:51234")
	}
	if c1.BytesReceived != 12345678 {
		t.Errorf("client1 BytesReceived = %d, want %d", c1.BytesReceived, 12345678)
	}
	if c1.BytesSent != 87654321 {
		t.Errorf("client1 BytesSent = %d, want %d", c1.BytesSent, 87654321)
	}
	if c1.ConnectedSince != "Thu Mar 04 09:00:00 2026" {
		t.Errorf("client1 ConnectedSince = %q", c1.ConnectedSince)
	}
	if c1.VirtualAddress != "10.8.0.6" {
		t.Errorf("client1 VirtualAddress = %q, want %q", c1.VirtualAddress, "10.8.0.6")
	}

	c2, ok := byName["client2"]
	if !ok {
		t.Fatal("missing client2")
	}
	if c2.VirtualAddress != "10.8.0.10" {
		t.Errorf("client2 VirtualAddress = %q, want %q", c2.VirtualAddress, "10.8.0.10")
	}
	if c2.BytesReceived != 9876543 {
		t.Errorf("client2 BytesReceived = %d, want %d", c2.BytesReceived, 9876543)
	}
}

func TestParseOpenVPNStatusLog_EmptyFile(t *testing.T) {
	clients := parseOpenVPNStatusLog("")
	if len(clients) != 0 {
		t.Errorf("expected 0 clients from empty file, got %d", len(clients))
	}
}

func TestParseOpenVPNStatusLog_HeadersOnly(t *testing.T) {
	raw := `OpenVPN CLIENT LIST
Updated,Thu Mar 04 10:00:00 2026
Common Name,Real Address,Bytes Received,Bytes Sent,Connected Since
ROUTING TABLE
Virtual Address,Common Name,Real Address,Last Ref
GLOBAL STATS
Max bcast/mcast queue length,0
END
`
	clients := parseOpenVPNStatusLog(raw)
	if len(clients) != 0 {
		t.Errorf("expected 0 clients from headers-only file, got %d", len(clients))
	}
}

func TestParseOpenVPNStatusLog_MalformedLines(t *testing.T) {
	raw := `OpenVPN CLIENT LIST
Updated,Thu Mar 04 10:00:00 2026
Common Name,Real Address,Bytes Received,Bytes Sent,Connected Since
good-client,1.2.3.4:1234,100,200,Thu Mar 04 09:00:00 2026
bad-line-only-two-fields,1.2.3.4
another-bad
ROUTING TABLE
Virtual Address,Common Name,Real Address,Last Ref
GLOBAL STATS
Max bcast/mcast queue length,0
END
`
	clients := parseOpenVPNStatusLog(raw)
	if len(clients) != 1 {
		t.Fatalf("expected 1 client (malformed skipped), got %d", len(clients))
	}
	if clients[0].CommonName != "good-client" {
		t.Errorf("expected good-client, got %q", clients[0].CommonName)
	}
}

func TestParseOpenVPNStatusLog_NoRoutingTable(t *testing.T) {
	raw := `OpenVPN CLIENT LIST
Updated,Thu Mar 04 10:00:00 2026
Common Name,Real Address,Bytes Received,Bytes Sent,Connected Since
client1,1.2.3.4:5678,1000,2000,Thu Mar 04 09:00:00 2026
GLOBAL STATS
Max bcast/mcast queue length,0
END
`
	clients := parseOpenVPNStatusLog(raw)
	if len(clients) != 1 {
		t.Fatalf("expected 1 client, got %d", len(clients))
	}
	if clients[0].VirtualAddress != "" {
		t.Errorf("expected empty VirtualAddress without routing table, got %q", clients[0].VirtualAddress)
	}
}

// testExtractHostPort splits "host:port" from a test server address.
func testExtractHostPort(t *testing.T, addr string) (string, int) {
	t.Helper()
	idx := strings.LastIndex(addr, ":")
	if idx < 0 {
		t.Fatalf("invalid address %q", addr)
	}
	port, err := strconv.Atoi(addr[idx+1:])
	if err != nil {
		t.Fatalf("invalid port in %q: %v", addr, err)
	}
	return addr[:idx], port
}

func TestFetchOpenVPNStatus_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/status" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(twoClientStatusLog))
	}))
	defer server.Close()

	host, port := testExtractHostPort(t, server.Listener.Addr().String())
	clients, err := fetchOpenVPNStatus(host, port)
	if err != nil {
		t.Fatalf("fetchOpenVPNStatus() error = %v", err)
	}
	if len(clients) != 2 {
		t.Errorf("expected 2 clients, got %d", len(clients))
	}
}

func TestFetchOpenVPNStatus_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	host, port := testExtractHostPort(t, server.Listener.Addr().String())
	_, err := fetchOpenVPNStatus(host, port)
	if err == nil {
		t.Fatal("expected error on 500 response")
	}
}

func TestFetchOpenVPNStatus_ConnectionRefused(t *testing.T) {
	_, err := fetchOpenVPNStatus("127.0.0.1", 19999)
	if err == nil {
		t.Fatal("expected error on connection refused")
	}
}
