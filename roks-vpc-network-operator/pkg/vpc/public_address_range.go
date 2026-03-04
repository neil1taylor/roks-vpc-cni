package vpc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/IBM/go-sdk-core/v5/core"
)

// PAR API types — the VPC Go SDK does not yet have PAR support,
// so we use raw REST calls via the SDK's underlying HTTP client.

type parCreateRequest struct {
	Name         string     `json:"name,omitempty"`
	VPC          idRef      `json:"vpc"`
	Zone         nameRef    `json:"zone"`
	PrefixLength int        `json:"prefix_length"`
}

type idRef struct {
	ID string `json:"id"`
}

type nameRef struct {
	Name string `json:"name"`
}

type parResponse struct {
	ID             string  `json:"id"`
	CRN            string  `json:"crn"`
	Name           string  `json:"name"`
	CIDR           string  `json:"cidr"`
	LifecycleState string  `json:"lifecycle_state"`
	CreatedAt      string  `json:"created_at"`
	Zone           *struct {
		Name string `json:"name"`
	} `json:"zone"`
	VPC *struct {
		ID string `json:"id"`
	} `json:"vpc"`
}

type parListResponse struct {
	PublicAddressRanges []parResponse `json:"public_address_ranges"`
	Next                *struct {
		Href string `json:"href"`
	} `json:"next"`
}

func parFromResponse(r *parResponse) PublicAddressRange {
	par := PublicAddressRange{
		ID:             r.ID,
		CRN:            r.CRN,
		Name:           r.Name,
		CIDR:           r.CIDR,
		LifecycleState: r.LifecycleState,
		CreatedAt:      r.CreatedAt,
	}
	if r.Zone != nil {
		par.Zone = r.Zone.Name
	}
	if r.VPC != nil {
		par.VPCID = r.VPC.ID
	}
	return par
}

// CreatePublicAddressRange creates a public address range bound to a VPC + zone.
func (c *vpcClient) CreatePublicAddressRange(ctx context.Context, opts CreatePublicAddressRangeOptions) (*PublicAddressRange, error) {
	if err := c.limiter.Acquire(ctx); err != nil {
		return nil, err
	}
	defer c.limiter.Release()

	body := parCreateRequest{
		Name:         opts.Name,
		VPC:          idRef{ID: opts.VPCID},
		Zone:         nameRef{Name: opts.Zone},
		PrefixLength: opts.PrefixLength,
	}

	resp, err := c.doREST(ctx, http.MethodPost, "/v1/public_address_ranges", body)
	if err != nil {
		return nil, fmt.Errorf("VPC API CreatePublicAddressRange: %w", err)
	}

	var result parResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("VPC API CreatePublicAddressRange: unmarshal: %w", err)
	}

	par := parFromResponse(&result)

	// Tag for traceability and orphan GC
	if par.CRN != "" && (opts.ClusterID != "" || opts.OwnerKind != "") {
		c.tagResource(ctx, par.CRN, BuildTags(opts.ClusterID, "par", opts.OwnerKind, opts.OwnerName))
	}

	return &par, nil
}

// GetPublicAddressRange retrieves a public address range by ID.
func (c *vpcClient) GetPublicAddressRange(ctx context.Context, parID string) (*PublicAddressRange, error) {
	if err := c.limiter.Acquire(ctx); err != nil {
		return nil, err
	}
	defer c.limiter.Release()

	resp, err := c.doREST(ctx, http.MethodGet, fmt.Sprintf("/v1/public_address_ranges/%s", parID), nil)
	if err != nil {
		return nil, fmt.Errorf("VPC API GetPublicAddressRange(%s): %w", parID, err)
	}

	var result parResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("VPC API GetPublicAddressRange(%s): unmarshal: %w", parID, err)
	}

	par := parFromResponse(&result)
	return &par, nil
}

// ListPublicAddressRanges lists all PARs for a VPC.
func (c *vpcClient) ListPublicAddressRanges(ctx context.Context, vpcID string) ([]PublicAddressRange, error) {
	if err := c.limiter.Acquire(ctx); err != nil {
		return nil, err
	}
	defer c.limiter.Release()

	path := fmt.Sprintf("/v1/public_address_ranges?vpc.id=%s", vpcID)
	resp, err := c.doREST(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("VPC API ListPublicAddressRanges(%s): %w", vpcID, err)
	}

	var result parListResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("VPC API ListPublicAddressRanges(%s): unmarshal: %w", vpcID, err)
	}

	pars := make([]PublicAddressRange, 0, len(result.PublicAddressRanges))
	for i := range result.PublicAddressRanges {
		pars = append(pars, parFromResponse(&result.PublicAddressRanges[i]))
	}
	return pars, nil
}

// DeletePublicAddressRange deletes a public address range by ID.
func (c *vpcClient) DeletePublicAddressRange(ctx context.Context, parID string) error {
	if err := c.limiter.Acquire(ctx); err != nil {
		return err
	}
	defer c.limiter.Release()

	_, err := c.doREST(ctx, http.MethodDelete, fmt.Sprintf("/v1/public_address_ranges/%s", parID), nil)
	if err != nil {
		return fmt.Errorf("VPC API DeletePublicAddressRange(%s): %w", parID, err)
	}
	return nil
}

// doREST performs a raw REST call via the VPC SDK's underlying HTTP client.
// This is used for API endpoints not yet supported by the Go SDK (e.g., PAR).
func (c *vpcClient) doREST(ctx context.Context, method, path string, body interface{}) ([]byte, error) {
	builder := core.NewRequestBuilder(method)
	builder.WithContext(ctx)

	baseURL := c.service.Service.GetServiceURL()
	url := baseURL + path
	// Append API version query parameter
	separator := "?"
	if contains(path, '?') {
		separator = "&"
	}
	url += separator + "version=2024-11-19&generation=2"

	_, err := builder.ResolveRequestURL(url, "", nil)
	if err != nil {
		return nil, fmt.Errorf("resolve URL: %w", err)
	}

	if body != nil {
		_, err = builder.SetBodyContentJSON(body)
		if err != nil {
			return nil, fmt.Errorf("set body: %w", err)
		}
	}

	builder.AddHeader("Accept", "application/json")
	builder.AddHeader("Content-Type", "application/json")

	request, err := builder.Build()
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	var rawResponse *http.Response
	_, err = c.service.Service.Request(request, &rawResponse)
	if err != nil {
		return nil, err
	}
	if rawResponse == nil {
		// DELETE with 204 No Content
		return nil, nil
	}
	defer rawResponse.Body.Close()

	respBody, err := io.ReadAll(rawResponse.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	return respBody, nil
}

func contains(s string, c byte) bool {
	for i := range s {
		if s[i] == c {
			return true
		}
	}
	return false
}
