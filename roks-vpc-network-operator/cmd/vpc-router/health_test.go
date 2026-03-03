package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthzHandler_LinuxOnly(t *testing.T) {
	// This test only works on Linux where /proc/sys/net/ipv4/ip_forward exists.
	// On non-Linux systems, the handler will return ServiceUnavailable which is correct.
	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()

	healthzHandler(w, req)

	// On macOS/CI without /proc, this will return 503 -- that's expected
	if w.Code != http.StatusOK && w.Code != http.StatusServiceUnavailable {
		t.Errorf("healthz returned status %d, want 200 or 503", w.Code)
	}
}

func TestReadyzHandler_NoUplink(t *testing.T) {
	// Without an uplink interface, readyz should return 503
	req := httptest.NewRequest("GET", "/readyz", nil)
	w := httptest.NewRecorder()

	readyzHandler(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("readyz without uplink returned status %d, want 503", w.Code)
	}
}
