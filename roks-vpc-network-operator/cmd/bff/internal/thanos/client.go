package thanos

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

// Client queries the Thanos/Prometheus Query API.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a Thanos client. It reads the service account token
// from the mounted path for authentication against the Thanos querier.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true, // Thanos uses cluster-internal CA
				},
			},
		},
	}
}

// NewClientFromEnv creates a client from the THANOS_URL env var. Returns nil if not set.
func NewClientFromEnv() *Client {
	url := os.Getenv("THANOS_URL")
	if url == "" {
		return nil
	}
	return NewClient(url)
}

// QueryResult represents the response from Prometheus/Thanos query API.
type QueryResult struct {
	Status string     `json:"status"`
	Data   ResultData `json:"data"`
}

// ResultData holds the data portion of a query result.
type ResultData struct {
	ResultType string       `json:"resultType"`
	Result     []DataSeries `json:"result"`
}

// DataSeries represents a single time series result.
type DataSeries struct {
	Metric map[string]string `json:"metric"`
	Value  []interface{}     `json:"value,omitempty"`  // for instant queries: [timestamp, value]
	Values [][]interface{}   `json:"values,omitempty"` // for range queries: [[t,v], [t,v], ...]
}

// QueryInstant executes an instant query against the Thanos API.
func (c *Client) QueryInstant(ctx context.Context, query string) (*QueryResult, error) {
	params := url.Values{
		"query": {query},
	}
	return c.doQuery(ctx, "/api/v1/query", params)
}

// QueryRange executes a range query against the Thanos API.
func (c *Client) QueryRange(ctx context.Context, query, start, end, step string) (*QueryResult, error) {
	params := url.Values{
		"query": {query},
		"start": {start},
		"end":   {end},
		"step":  {step},
	}
	return c.doQuery(ctx, "/api/v1/query_range", params)
}

func (c *Client) doQuery(ctx context.Context, path string, params url.Values) (*QueryResult, error) {
	reqURL := fmt.Sprintf("%s%s?%s", c.baseURL, path, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Add bearer token from service account
	token, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
	if err == nil && len(token) > 0 {
		req.Header.Set("Authorization", "Bearer "+string(token))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("thanos query: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("thanos returned %d: %s", resp.StatusCode, string(body))
	}

	var result QueryResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &result, nil
}
