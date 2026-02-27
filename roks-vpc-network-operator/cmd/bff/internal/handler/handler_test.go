package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/IBM/roks-vpc-network-operator/cmd/bff/internal/auth"
	"github.com/IBM/roks-vpc-network-operator/cmd/bff/internal/model"
	"github.com/IBM/roks-vpc-network-operator/pkg/vpc"

	authzv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// newAllowAllRBACChecker creates an RBACChecker with a fake K8s client that allows all SAR requests.
func newAllowAllRBACChecker() *auth.RBACChecker {
	fakeClient := fake.NewSimpleClientset()
	fakeClient.PrependReactor("create", "subjectaccessreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, &authzv1.SubjectAccessReview{
			Status: authzv1.SubjectAccessReviewStatus{Allowed: true},
		}, nil
	})
	return auth.NewRBACChecker(fakeClient)
}

// newDenyAllRBACChecker creates an RBACChecker that denies all SAR requests.
func newDenyAllRBACChecker() *auth.RBACChecker {
	fakeClient := fake.NewSimpleClientset()
	fakeClient.PrependReactor("create", "subjectaccessreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, &authzv1.SubjectAccessReview{
			Status: authzv1.SubjectAccessReviewStatus{Allowed: false},
		}, nil
	})
	return auth.NewRBACChecker(fakeClient)
}

// authRequest creates an HTTP request with user auth headers.
func authRequest(method, target string, body string) *http.Request {
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, target, strings.NewReader(body))
	} else {
		req = httptest.NewRequest(method, target, nil)
	}
	req.Header.Set("X-Remote-User", "test-user")
	req.Header.Set("X-Remote-Group", "system:authenticated")
	// Add user info to context as the handler expects
	ctx := auth.WithUserInfo(req.Context(), &auth.UserInfo{
		Name:   "test-user",
		Groups: []string{"system:authenticated"},
	})
	return req.WithContext(ctx)
}

// decodeJSON decodes a JSON response body.
func decodeJSON(t *testing.T, rec *httptest.ResponseRecorder, v interface{}) {
	t.Helper()
	if err := json.NewDecoder(rec.Body).Decode(v); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
}

// --- Health handler tests ---

func TestHealthHandler(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	HealthHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	var resp map[string]string
	decodeJSON(t, rec, &resp)
	if resp["status"] != "ok" {
		t.Errorf("expected status 'ok', got %q", resp["status"])
	}
}

func TestReadyHandler(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	ReadyHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	var resp map[string]string
	decodeJSON(t, rec, &resp)
	if resp["status"] != "ready" {
		t.Errorf("expected status 'ready', got %q", resp["status"])
	}
}

// --- VPC handler tests ---

func TestListVPCs(t *testing.T) {
	mock := vpc.NewMockClient()
	mock.ListVPCsFn = func(ctx context.Context) ([]vpc.VPC, error) {
		return []vpc.VPC{
			{ID: "vpc-1", Name: "test-vpc", Region: "us-south", Status: "available"},
		}, nil
	}

	h := NewVPCHandler(mock, "") // no defaultVPCID — lists all
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/vpcs", nil)
	h.ListVPCs(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	var resp []map[string]interface{}
	decodeJSON(t, rec, &resp)
	if len(resp) != 1 {
		t.Fatalf("expected 1 VPC, got %d", len(resp))
	}
	if resp[0]["id"] != "vpc-1" {
		t.Errorf("expected VPC id 'vpc-1', got %v", resp[0]["id"])
	}
}

func TestListVPCs_ScopedToClusterVPC(t *testing.T) {
	mock := vpc.NewMockClient()
	mock.GetVPCFn = func(ctx context.Context, vpcID string) (*vpc.VPC, error) {
		if vpcID != "vpc-cluster" {
			return nil, fmt.Errorf("unexpected VPC ID %s", vpcID)
		}
		return &vpc.VPC{ID: "vpc-cluster", Name: "cluster-vpc", Region: "eu-de", Status: "available"}, nil
	}
	mock.ListVPCsFn = func(ctx context.Context) ([]vpc.VPC, error) {
		t.Error("ListVPCs should not be called when defaultVPCID is set")
		return nil, nil
	}

	h := NewVPCHandler(mock, "vpc-cluster")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/vpcs", nil)
	h.ListVPCs(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	var resp []map[string]interface{}
	decodeJSON(t, rec, &resp)
	if len(resp) != 1 {
		t.Fatalf("expected 1 VPC, got %d", len(resp))
	}
	if resp[0]["id"] != "vpc-cluster" {
		t.Errorf("expected VPC id 'vpc-cluster', got %v", resp[0]["id"])
	}
	if resp[0]["name"] != "cluster-vpc" {
		t.Errorf("expected VPC name 'cluster-vpc', got %v", resp[0]["name"])
	}
	if mock.CallCount("GetVPC") != 1 {
		t.Errorf("expected GetVPC to be called once, got %d", mock.CallCount("GetVPC"))
	}
}

func TestListVPCs_ScopedGetVPCFails_FallsBackToList(t *testing.T) {
	mock := vpc.NewMockClient()
	// GetVPC fails (e.g. wrong VPC ID)
	mock.GetVPCFn = func(ctx context.Context, vpcID string) (*vpc.VPC, error) {
		return nil, fmt.Errorf("VPC not found")
	}
	// ListVPCs returns all VPCs as fallback
	mock.ListVPCsFn = func(ctx context.Context) ([]vpc.VPC, error) {
		return []vpc.VPC{
			{ID: "vpc-a", Name: "fallback-vpc", Region: "eu-de", Status: "available"},
			{ID: "vpc-b", Name: "other-vpc", Region: "eu-de", Status: "available"},
		}, nil
	}

	h := NewVPCHandler(mock, "vpc-invalid") // invalid defaultVPCID
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/vpcs", nil)
	h.ListVPCs(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	var resp []map[string]interface{}
	decodeJSON(t, rec, &resp)
	if len(resp) != 2 {
		t.Fatalf("expected 2 VPCs (fallback to list), got %d", len(resp))
	}
	if mock.CallCount("GetVPC") != 1 {
		t.Errorf("expected GetVPC to be called once, got %d", mock.CallCount("GetVPC"))
	}
	if mock.CallCount("ListVPCs") != 1 {
		t.Errorf("expected ListVPCs to be called once as fallback, got %d", mock.CallCount("ListVPCs"))
	}
}

func TestGetVPC(t *testing.T) {
	mock := vpc.NewMockClient()
	mock.GetVPCFn = func(ctx context.Context, vpcID string) (*vpc.VPC, error) {
		return &vpc.VPC{ID: vpcID, Name: "my-vpc", Region: "us-south", Status: "available"}, nil
	}

	h := NewVPCHandler(mock, "")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/vpcs/vpc-1", nil)
	h.GetVPC(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	var resp map[string]interface{}
	decodeJSON(t, rec, &resp)
	if resp["id"] != "vpc-1" {
		t.Errorf("expected id 'vpc-1', got %v", resp["id"])
	}
}

func TestGetVPC_NotFound(t *testing.T) {
	mock := vpc.NewMockClient()
	// GetVPCFn not set — will return error

	h := NewVPCHandler(mock, "")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/vpcs/not-exist", nil)
	h.GetVPC(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

// --- Zone handler tests ---

func TestListZones(t *testing.T) {
	mock := vpc.NewMockClient()
	mock.ListZonesFn = func(ctx context.Context, region string) ([]vpc.Zone, error) {
		return []vpc.Zone{
			{Name: "us-south-1", Region: "us-south", Status: "available"},
			{Name: "us-south-2", Region: "us-south", Status: "available"},
		}, nil
	}

	h := NewZoneHandler(mock, "")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/zones?region=us-south", nil)
	h.ListZones(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	var resp []map[string]interface{}
	decodeJSON(t, rec, &resp)
	if len(resp) != 2 {
		t.Fatalf("expected 2 zones, got %d", len(resp))
	}
}

func TestListZones_Error(t *testing.T) {
	mock := vpc.NewMockClient()
	// ListZonesFn not set — returns error

	h := NewZoneHandler(mock, "")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/zones", nil)
	h.ListZones(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}
}

// --- Topology handler tests ---

func TestGetTopology(t *testing.T) {
	mock := vpc.NewMockClient()
	mock.ListVPCsFn = func(ctx context.Context) ([]vpc.VPC, error) {
		return []vpc.VPC{{ID: "vpc-1", Name: "test-vpc", Region: "us-south", Status: "available"}}, nil
	}
	mock.ListSecurityGroupsFn = func(ctx context.Context, vpcID string) ([]vpc.SecurityGroup, error) {
		return []vpc.SecurityGroup{{ID: "sg-1", Name: "test-sg", VPCID: "vpc-1"}}, nil
	}
	mock.ListNetworkACLsFn = func(ctx context.Context, vpcID string) ([]vpc.NetworkACL, error) {
		return []vpc.NetworkACL{{ID: "acl-1", Name: "test-acl", VPCID: "vpc-1"}}, nil
	}

	h := NewTopologyHandler(mock, nil, nil, "") // no dynamic client for this test
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/topology", nil)
	h.GetTopology(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp map[string]interface{}
	decodeJSON(t, rec, &resp)
	nodes := resp["nodes"].([]interface{})
	edges := resp["edges"].([]interface{})

	if len(nodes) != 3 {
		t.Errorf("expected 3 nodes (vpc + sg + acl), got %d", len(nodes))
	}
	if len(edges) != 2 {
		t.Errorf("expected 2 edges, got %d", len(edges))
	}

	// Verify node labels are populated
	vpcNode := nodes[0].(map[string]interface{})
	if vpcNode["label"] != "test-vpc" {
		t.Errorf("expected vpc label 'test-vpc', got %v", vpcNode["label"])
	}
}

func TestGetTopology_PartialFailure(t *testing.T) {
	mock := vpc.NewMockClient()
	mock.ListVPCsFn = func(ctx context.Context) ([]vpc.VPC, error) {
		return []vpc.VPC{{ID: "vpc-1", Name: "test-vpc"}}, nil
	}
	// SG and ACL not configured — will error but topology should still return

	h := NewTopologyHandler(mock, nil, nil, "")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/topology", nil)
	h.GetTopology(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	var resp map[string]interface{}
	decodeJSON(t, rec, &resp)
	nodes := resp["nodes"].([]interface{})
	if len(nodes) != 1 {
		t.Errorf("expected 1 node (vpc only), got %d", len(nodes))
	}
}

func TestGetTopology_MethodNotAllowed(t *testing.T) {
	mock := vpc.NewMockClient()
	h := NewTopologyHandler(mock, nil, nil, "")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/topology", nil)
	h.GetTopology(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

// --- SetupRoutes smoke test ---

func TestSetupRoutes(t *testing.T) {
	mock := vpc.NewMockClient()
	mock.ListVPCsFn = func(ctx context.Context) ([]vpc.VPC, error) { return nil, nil }
	mock.ListZonesFn = func(ctx context.Context, region string) ([]vpc.Zone, error) { return nil, nil }
	mock.ListSecurityGroupsFn = func(ctx context.Context, vpcID string) ([]vpc.SecurityGroup, error) {
		return nil, nil
	}
	mock.ListNetworkACLsFn = func(ctx context.Context, vpcID string) ([]vpc.NetworkACL, error) { return nil, nil }

	rbac := newAllowAllRBACChecker()
	mux := http.NewServeMux()
	SetupRoutesWithClusterInfo(mux, mock, rbac, nil, nil, ClusterInfo{Mode: "unmanaged"})

	// Verify health endpoints
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("/healthz: expected %d, got %d", http.StatusOK, rec.Code)
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/readyz", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("/readyz: expected %d, got %d", http.StatusOK, rec.Code)
	}

	// Verify cluster-info
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/api/v1/cluster-info", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("/cluster-info: expected %d, got %d", http.StatusOK, rec.Code)
	}
	var info map[string]interface{}
	decodeJSON(t, rec, &info)
	if info["clusterMode"] != "unmanaged" {
		t.Errorf("expected clusterMode 'unmanaged', got %v", info["clusterMode"])
	}
}

// --- Helper function tests ---

func TestExtractPathParam(t *testing.T) {
	tests := []struct {
		path   string
		prefix string
		want   string
	}{
		{"/api/v1/security-groups/sg-123", "/api/v1/security-groups/", "sg-123"},
		{"/api/v1/security-groups/sg-123/rules", "/api/v1/security-groups/", "sg-123"},
		{"/api/v1/security-groups/", "/api/v1/security-groups/", ""},
	}

	for _, tt := range tests {
		got := ExtractPathParam(tt.path, tt.prefix)
		if got != tt.want {
			t.Errorf("ExtractPathParam(%q, %q) = %q, want %q", tt.path, tt.prefix, got, tt.want)
		}
	}
}

func TestExtractSubPathParam(t *testing.T) {
	tests := []struct {
		path   string
		prefix string
		sub    string
		want   string
	}{
		{"/api/v1/security-groups/sg-123/rules/rule-456", "/api/v1/security-groups/", "rules/", "rule-456"},
		{"/api/v1/security-groups/sg-123/rules/", "/api/v1/security-groups/", "rules/", ""},
	}

	for _, tt := range tests {
		got := ExtractSubPathParam(tt.path, tt.prefix, tt.sub)
		if got != tt.want {
			t.Errorf("ExtractSubPathParam(%q, %q, %q) = %q, want %q", tt.path, tt.prefix, tt.sub, got, tt.want)
		}
	}
}

// --- Network handler tests ---

func TestGetNetworkTypes(t *testing.T) {
	h := NewNetworkHandler(nil, nil, nil, ClusterInfo{Mode: "unmanaged"})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/network-types", nil)
	h.GetNetworkTypes(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp map[string]interface{}
	decodeJSON(t, rec, &resp)

	combos := resp["combinations"].([]interface{})
	if len(combos) != 4 {
		t.Errorf("expected 4 network type combinations, got %d", len(combos))
	}

	// Verify topology values
	topos := resp["topologies"].([]interface{})
	if len(topos) != 2 {
		t.Errorf("expected 2 topologies, got %d", len(topos))
	}
}

func TestListCUDNs_NoDynamicClient(t *testing.T) {
	h := NewNetworkHandler(nil, nil, nil, ClusterInfo{Mode: "unmanaged"})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/cudns", nil)
	h.ListCUDNs(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected %d, got %d", http.StatusServiceUnavailable, rec.Code)
	}
}

func TestListCUDNs_WithDynamicClient(t *testing.T) {
	dynClient := newFakeDynClient()
	h := NewNetworkHandler(nil, dynClient, nil, ClusterInfo{Mode: "unmanaged"})

	// Create a test CUDN
	ctx := context.Background()
	createTestCUDN(t, ctx, dynClient, "test-cudn", "LocalNet")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/cudns", nil)
	h.ListCUDNs(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp []map[string]interface{}
	decodeJSON(t, rec, &resp)
	if len(resp) != 1 {
		t.Fatalf("expected 1 CUDN, got %d", len(resp))
	}
	if resp[0]["name"] != "test-cudn" {
		t.Errorf("expected name 'test-cudn', got %v", resp[0]["name"])
	}
}

func TestGetCUDN(t *testing.T) {
	dynClient := newFakeDynClient()
	h := NewNetworkHandler(nil, dynClient, nil, ClusterInfo{Mode: "unmanaged"})

	ctx := context.Background()
	createTestCUDN(t, ctx, dynClient, "my-cudn", "LocalNet")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/cudns/my-cudn", nil)
	h.GetCUDN(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}
}

func TestGetCUDN_NotFound(t *testing.T) {
	dynClient := newFakeDynClient()
	h := NewNetworkHandler(nil, dynClient, nil, ClusterInfo{Mode: "unmanaged"})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/cudns/not-exist", nil)
	h.GetCUDN(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestDeleteCUDN(t *testing.T) {
	dynClient := newFakeDynClient()
	h := NewNetworkHandler(nil, dynClient, nil, ClusterInfo{Mode: "unmanaged"})

	ctx := context.Background()
	createTestCUDN(t, ctx, dynClient, "del-cudn", "LocalNet")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/cudns/del-cudn", nil)
	h.DeleteCUDN(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}
}

func TestCreateCUDN(t *testing.T) {
	dynClient := newFakeDynClient()
	h := NewNetworkHandler(nil, dynClient, nil, ClusterInfo{Mode: "unmanaged"})

	body := `{"name":"new-cudn","topology":"Layer2","role":"Secondary"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cudns", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	h.CreateCUDN(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, rec.Code, rec.Body.String())
	}
	var resp map[string]interface{}
	decodeJSON(t, rec, &resp)
	if resp["name"] != "new-cudn" {
		t.Errorf("expected name 'new-cudn', got %v", resp["name"])
	}
}

func TestListUDNs_WithDynamicClient(t *testing.T) {
	dynClient := newFakeDynClient()
	h := NewNetworkHandler(nil, dynClient, nil, ClusterInfo{Mode: "unmanaged"})

	ctx := context.Background()
	createTestUDN(t, ctx, dynClient, "test-ns", "test-udn", "Layer2")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/udns", nil)
	h.ListUDNs(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp []map[string]interface{}
	decodeJSON(t, rec, &resp)
	if len(resp) != 1 {
		t.Fatalf("expected 1 UDN, got %d", len(resp))
	}
}

func TestGetUDN(t *testing.T) {
	dynClient := newFakeDynClient()
	h := NewNetworkHandler(nil, dynClient, nil, ClusterInfo{Mode: "unmanaged"})

	ctx := context.Background()
	createTestUDN(t, ctx, dynClient, "ns1", "my-udn", "LocalNet")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/udns/ns1/my-udn", nil)
	h.GetUDN(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}
}

func TestDeleteUDN(t *testing.T) {
	dynClient := newFakeDynClient()
	h := NewNetworkHandler(nil, dynClient, nil, ClusterInfo{Mode: "unmanaged"})

	ctx := context.Background()
	createTestUDN(t, ctx, dynClient, "ns1", "del-udn", "Layer2")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/udns/ns1/del-udn", nil)
	h.DeleteUDN(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}
}

func TestGetUDN_InvalidPath(t *testing.T) {
	dynClient := newFakeDynClient()
	h := NewNetworkHandler(nil, dynClient, nil, ClusterInfo{Mode: "unmanaged"})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/udns/only-one-segment", nil)
	h.GetUDN(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

// --- Topology: single VPC test ---

func TestGetTopology_SingleVPC(t *testing.T) {
	mock := vpc.NewMockClient()
	mock.GetVPCFn = func(ctx context.Context, vpcID string) (*vpc.VPC, error) {
		if vpcID != "vpc-cluster" {
			return nil, fmt.Errorf("unexpected VPC ID %s", vpcID)
		}
		return &vpc.VPC{ID: "vpc-cluster", Name: "cluster-vpc", Region: "eu-de", Status: "available"}, nil
	}
	mock.ListSecurityGroupsFn = func(ctx context.Context, vpcID string) ([]vpc.SecurityGroup, error) {
		return nil, nil
	}
	mock.ListNetworkACLsFn = func(ctx context.Context, vpcID string) ([]vpc.NetworkACL, error) {
		return nil, nil
	}

	h := NewTopologyHandler(mock, nil, nil, "vpc-cluster")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/topology", nil)
	h.GetTopology(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp map[string]interface{}
	decodeJSON(t, rec, &resp)
	nodes := resp["nodes"].([]interface{})

	if len(nodes) != 1 {
		t.Fatalf("expected 1 node (single VPC), got %d", len(nodes))
	}
	vpcNode := nodes[0].(map[string]interface{})
	if vpcNode["id"] != "vpc-cluster" {
		t.Errorf("expected VPC id 'vpc-cluster', got %v", vpcNode["id"])
	}
	if vpcNode["label"] != "cluster-vpc" {
		t.Errorf("expected label 'cluster-vpc', got %v", vpcNode["label"])
	}

	// Verify ListVPCs was NOT called (should use GetVPC instead)
	if mock.CallCount("ListVPCs") != 0 {
		t.Errorf("expected ListVPCs not to be called, but it was called %d times", mock.CallCount("ListVPCs"))
	}
	if mock.CallCount("GetVPC") != 1 {
		t.Errorf("expected GetVPC to be called once, got %d", mock.CallCount("GetVPC"))
	}
}

// --- VNI handler: VPC filtering test ---

func TestListVNIs_FilteredByVPC(t *testing.T) {
	mock := vpc.NewMockClient()

	// Two subnets: one in cluster VPC, one in another VPC
	mock.ListSubnetsFn = func(ctx context.Context, vpcID string) ([]vpc.Subnet, error) {
		return []vpc.Subnet{
			{ID: "subnet-in-vpc", Name: "my-subnet", VPCID: "vpc-cluster"},
		}, nil
	}

	// Three VNIs: two on the VPC subnet, one on a different subnet
	mock.ListVNIsFn = func(ctx context.Context) ([]vpc.VNI, error) {
		return []vpc.VNI{
			{ID: "vni-1", Name: "vni-in-vpc-1", SubnetID: "subnet-in-vpc", Status: "stable"},
			{ID: "vni-2", Name: "vni-in-vpc-2", SubnetID: "subnet-in-vpc", Status: "stable"},
			{ID: "vni-3", Name: "vni-other", SubnetID: "subnet-other-vpc", Status: "stable"},
		}, nil
	}

	h := NewVNIHandler(mock, "vpc-cluster")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/vnis", nil)
	h.ListVNIs(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp []model.VNIResponse
	decodeJSON(t, rec, &resp)
	if len(resp) != 2 {
		t.Fatalf("expected 2 VNIs (filtered), got %d", len(resp))
	}
	for _, v := range resp {
		if v.ID == "vni-3" {
			t.Errorf("VNI from other VPC should have been filtered out")
		}
	}
}

func TestListVNIs_NoVPCID_NoFiltering(t *testing.T) {
	mock := vpc.NewMockClient()
	mock.ListVNIsFn = func(ctx context.Context) ([]vpc.VNI, error) {
		return []vpc.VNI{
			{ID: "vni-1", Name: "vni-a", SubnetID: "sub-1", Status: "stable"},
			{ID: "vni-2", Name: "vni-b", SubnetID: "sub-2", Status: "stable"},
		}, nil
	}

	h := NewVNIHandler(mock, "") // no VPCID — no filtering
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/vnis", nil)
	h.ListVNIs(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp []model.VNIResponse
	decodeJSON(t, rec, &resp)
	if len(resp) != 2 {
		t.Fatalf("expected 2 VNIs (unfiltered), got %d", len(resp))
	}
}

// --- Floating IP handler: VPC filtering test ---

func TestListFloatingIPs_FilteredByVPC(t *testing.T) {
	mock := vpc.NewMockClient()

	mock.ListSubnetsFn = func(ctx context.Context, vpcID string) ([]vpc.Subnet, error) {
		return []vpc.Subnet{
			{ID: "subnet-in-vpc", Name: "my-subnet", VPCID: "vpc-cluster"},
		}, nil
	}

	mock.ListVNIsFn = func(ctx context.Context) ([]vpc.VNI, error) {
		return []vpc.VNI{
			{ID: "vni-in-vpc", Name: "vni-1", SubnetID: "subnet-in-vpc"},
			{ID: "vni-other", Name: "vni-2", SubnetID: "subnet-other-vpc"},
		}, nil
	}

	mock.ListFloatingIPsFn = func(ctx context.Context) ([]vpc.FloatingIP, error) {
		return []vpc.FloatingIP{
			{ID: "fip-1", Name: "fip-in-vpc", Target: "vni-in-vpc", Status: "available"},
			{ID: "fip-2", Name: "fip-other", Target: "vni-other", Status: "available"},
			{ID: "fip-3", Name: "fip-unbound", Target: "", Status: "available"}, // unbound — should be included
		}, nil
	}

	rbac := newAllowAllRBACChecker()
	h := NewFloatingIPHandler(mock, rbac, "vpc-cluster")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/floating-ips", nil)
	h.ListFloatingIPs(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp []model.FloatingIPResponse
	decodeJSON(t, rec, &resp)
	if len(resp) != 2 {
		t.Fatalf("expected 2 FIPs (in-vpc + unbound), got %d", len(resp))
	}
	for _, f := range resp {
		if f.ID == "fip-2" {
			t.Errorf("FIP targeting VNI in other VPC should have been filtered out")
		}
	}
}

func TestListFloatingIPs_NoVPCID_NoFiltering(t *testing.T) {
	mock := vpc.NewMockClient()
	mock.ListFloatingIPsFn = func(ctx context.Context) ([]vpc.FloatingIP, error) {
		return []vpc.FloatingIP{
			{ID: "fip-1", Name: "fip-a", Status: "available"},
			{ID: "fip-2", Name: "fip-b", Status: "available"},
		}, nil
	}

	rbac := newAllowAllRBACChecker()
	h := NewFloatingIPHandler(mock, rbac, "") // no VPCID — no filtering
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/floating-ips", nil)
	h.ListFloatingIPs(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp []model.FloatingIPResponse
	decodeJSON(t, rec, &resp)
	if len(resp) != 2 {
		t.Fatalf("expected 2 FIPs (unfiltered), got %d", len(resp))
	}
}

// --- Dynamic client helpers ---

func newFakeDynClient() *fakeDynamic {
	return newFakeDynamicClient()
}
