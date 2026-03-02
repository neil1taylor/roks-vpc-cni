package thanos

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestQueryInstant(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/query" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		query := r.URL.Query().Get("query")
		if query == "" {
			t.Error("expected query parameter")
		}

		resp := QueryResult{
			Status: "success",
			Data: ResultData{
				ResultType: "vector",
				Result: []DataSeries{
					{
						Metric: map[string]string{"interface": "uplink"},
						Value:  []interface{}{1709332800.0, "12345"},
					},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	result, err := client.QueryInstant(context.Background(), "router_interface_rx_bytes_total")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != "success" {
		t.Errorf("status = %q, want 'success'", result.Status)
	}
	if len(result.Data.Result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result.Data.Result))
	}
	if result.Data.Result[0].Metric["interface"] != "uplink" {
		t.Errorf("metric interface = %q, want 'uplink'", result.Data.Result[0].Metric["interface"])
	}
}

func TestQueryRange(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/query_range" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("start") == "" {
			t.Error("expected start parameter")
		}
		if r.URL.Query().Get("end") == "" {
			t.Error("expected end parameter")
		}
		if r.URL.Query().Get("step") == "" {
			t.Error("expected step parameter")
		}

		resp := QueryResult{
			Status: "success",
			Data: ResultData{
				ResultType: "matrix",
				Result: []DataSeries{
					{
						Metric: map[string]string{"interface": "net0"},
						Values: [][]interface{}{
							{1709332800.0, "100"},
							{1709332860.0, "200"},
						},
					},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	result, err := client.QueryRange(context.Background(),
		"rate(router_interface_rx_bytes_total[1m])", "1709332800", "1709336400", "60s")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Data.ResultType != "matrix" {
		t.Errorf("resultType = %q, want 'matrix'", result.Data.ResultType)
	}
	if len(result.Data.Result[0].Values) != 2 {
		t.Errorf("expected 2 values, got %d", len(result.Data.Result[0].Values))
	}
}

func TestQueryInstant_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.QueryInstant(context.Background(), "up")
	if err == nil {
		t.Error("expected error for 500 response")
	}
}
