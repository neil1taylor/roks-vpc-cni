package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/IBM/roks-vpc-network-operator/pkg/vpc"
)

func TestListNetworkACLs(t *testing.T) {
	mock := vpc.NewMockClient()
	mock.ListNetworkACLsFn = func(ctx context.Context, vpcID string) ([]vpc.NetworkACL, error) {
		return []vpc.NetworkACL{
			{ID: "acl-1", Name: "test-acl", VPCID: "vpc-1"},
			{ID: "acl-2", Name: "test-acl-2", VPCID: "vpc-1"},
		}, nil
	}

	rbac := newAllowAllRBACChecker()
	h := NewNetworkACLHandler(mock, rbac, "")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/network-acls", nil)
	h.ListNetworkACLs(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
	}
	var resp []map[string]interface{}
	decodeJSON(t, rec, &resp)
	if len(resp) != 2 {
		t.Fatalf("expected 2 ACLs, got %d", len(resp))
	}
}

func TestListNetworkACLs_WithVPCFilter(t *testing.T) {
	mock := vpc.NewMockClient()
	var capturedVPCID string
	mock.ListNetworkACLsFn = func(ctx context.Context, vpcID string) ([]vpc.NetworkACL, error) {
		capturedVPCID = vpcID
		return nil, nil
	}

	rbac := newAllowAllRBACChecker()
	h := NewNetworkACLHandler(mock, rbac, "")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/network-acls?vpc_id=vpc-456", nil)
	h.ListNetworkACLs(rec, req)

	if capturedVPCID != "vpc-456" {
		t.Errorf("expected vpc_id filter 'vpc-456', got %q", capturedVPCID)
	}
}

func TestListNetworkACLs_Error(t *testing.T) {
	mock := vpc.NewMockClient()
	rbac := newAllowAllRBACChecker()
	h := NewNetworkACLHandler(mock, rbac, "")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/network-acls", nil)
	h.ListNetworkACLs(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected %d, got %d", http.StatusInternalServerError, rec.Code)
	}
}

func TestGetNetworkACL(t *testing.T) {
	mock := vpc.NewMockClient()
	mock.GetNetworkACLFn = func(ctx context.Context, aclID string) (*vpc.NetworkACL, error) {
		return &vpc.NetworkACL{
			ID:    aclID,
			Name:  "my-acl",
			VPCID: "vpc-1",
			Rules: []vpc.NetworkACLRule{
				{
					ID:          "rule-1",
					Name:        "allow-inbound",
					Direction:   "inbound",
					Action:      "allow",
					Protocol:    "tcp",
					Source:      "0.0.0.0/0",
					Destination: "10.0.0.0/8",
					PortMin:     int64Ptr(80),
					PortMax:     int64Ptr(80),
				},
			},
		}, nil
	}

	rbac := newAllowAllRBACChecker()
	h := NewNetworkACLHandler(mock, rbac, "")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/network-acls/acl-1", nil)
	h.GetNetworkACL(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
	}
	var resp map[string]interface{}
	decodeJSON(t, rec, &resp)
	if resp["id"] != "acl-1" {
		t.Errorf("expected id 'acl-1', got %v", resp["id"])
	}
	rules := resp["rules"].([]interface{})
	if len(rules) != 1 {
		t.Errorf("expected 1 rule, got %d", len(rules))
	}
}

func TestGetNetworkACL_NotFound(t *testing.T) {
	mock := vpc.NewMockClient()
	rbac := newAllowAllRBACChecker()
	h := NewNetworkACLHandler(mock, rbac, "")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/network-acls/not-exist", nil)
	h.GetNetworkACL(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestCreateNetworkACL(t *testing.T) {
	mock := vpc.NewMockClient()
	mock.CreateNetworkACLFn = func(ctx context.Context, opts vpc.CreateNetworkACLOptions) (*vpc.NetworkACL, error) {
		return &vpc.NetworkACL{
			ID:    "acl-new",
			Name:  opts.Name,
			VPCID: opts.VPCID,
		}, nil
	}

	rbac := newAllowAllRBACChecker()
	h := NewNetworkACLHandler(mock, rbac, "")
	body := `{"name":"new-acl","vpc_id":"vpc-1"}`
	rec := httptest.NewRecorder()
	req := authRequest(http.MethodPost, "/api/v1/network-acls", body)
	h.CreateNetworkACL(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected %d, got %d: %s", http.StatusCreated, rec.Code, rec.Body.String())
	}
	var resp map[string]interface{}
	decodeJSON(t, rec, &resp)
	if resp["name"] != "new-acl" {
		t.Errorf("expected name 'new-acl', got %v", resp["name"])
	}
}

func TestCreateNetworkACL_Unauthorized(t *testing.T) {
	mock := vpc.NewMockClient()
	rbac := newAllowAllRBACChecker()
	h := NewNetworkACLHandler(mock, rbac, "")
	body := `{"name":"new-acl","vpc_id":"vpc-1"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/network-acls", strings.NewReader(body))
	h.CreateNetworkACL(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestCreateNetworkACL_Forbidden(t *testing.T) {
	mock := vpc.NewMockClient()
	rbac := newDenyAllRBACChecker()
	h := NewNetworkACLHandler(mock, rbac, "")
	body := `{"name":"new-acl","vpc_id":"vpc-1"}`
	rec := httptest.NewRecorder()
	req := authRequest(http.MethodPost, "/api/v1/network-acls", body)
	h.CreateNetworkACL(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected %d, got %d", http.StatusForbidden, rec.Code)
	}
}

func TestDeleteNetworkACL(t *testing.T) {
	mock := vpc.NewMockClient()
	mock.DeleteNetworkACLFn = func(ctx context.Context, aclID string) error {
		return nil
	}

	rbac := newAllowAllRBACChecker()
	h := NewNetworkACLHandler(mock, rbac, "")
	rec := httptest.NewRecorder()
	req := authRequest(http.MethodDelete, "/api/v1/network-acls/acl-1", "")
	h.DeleteNetworkACL(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected %d, got %d: %s", http.StatusNoContent, rec.Code, rec.Body.String())
	}
}

func TestAddNetworkACLRule(t *testing.T) {
	mock := vpc.NewMockClient()
	mock.AddNetworkACLRuleFn = func(ctx context.Context, aclID string, opts vpc.CreateACLRuleOptions) (*vpc.NetworkACLRule, error) {
		return &vpc.NetworkACLRule{
			ID:          "rule-new",
			Name:        opts.Name,
			Direction:   opts.Direction,
			Action:      opts.Action,
			Protocol:    opts.Protocol,
			Source:      opts.Source,
			Destination: opts.Destination,
		}, nil
	}

	rbac := newAllowAllRBACChecker()
	h := NewNetworkACLHandler(mock, rbac, "")
	body := `{"name":"allow-http","direction":"inbound","action":"allow","protocol":"tcp","source":"0.0.0.0/0","destination":"10.0.0.0/8"}`
	rec := httptest.NewRecorder()
	req := authRequest(http.MethodPost, "/api/v1/network-acls/acl-1/rules", body)
	h.AddNetworkACLRule(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected %d, got %d: %s", http.StatusCreated, rec.Code, rec.Body.String())
	}
	var resp map[string]interface{}
	decodeJSON(t, rec, &resp)
	if resp["name"] != "allow-http" {
		t.Errorf("expected name 'allow-http', got %v", resp["name"])
	}
}

func TestUpdateNetworkACLRule(t *testing.T) {
	mock := vpc.NewMockClient()
	mock.UpdateNetworkACLRuleFn = func(ctx context.Context, aclID, ruleID string, opts vpc.UpdateACLRuleOptions) (*vpc.NetworkACLRule, error) {
		return &vpc.NetworkACLRule{
			ID:        ruleID,
			Name:      "updated-rule",
			Direction: "inbound",
			Action:    "deny",
			Protocol:  "tcp",
		}, nil
	}

	rbac := newAllowAllRBACChecker()
	h := NewNetworkACLHandler(mock, rbac, "")
	body := `{"action":"deny"}`
	rec := httptest.NewRecorder()
	req := authRequest(http.MethodPatch, "/api/v1/network-acls/acl-1/rules/rule-1", body)
	h.UpdateNetworkACLRule(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}
}

func TestDeleteNetworkACLRule(t *testing.T) {
	mock := vpc.NewMockClient()
	mock.DeleteNetworkACLRuleFn = func(ctx context.Context, aclID, ruleID string) error {
		return nil
	}

	rbac := newAllowAllRBACChecker()
	h := NewNetworkACLHandler(mock, rbac, "")
	rec := httptest.NewRecorder()
	req := authRequest(http.MethodDelete, "/api/v1/network-acls/acl-1/rules/rule-1", "")
	h.DeleteNetworkACLRule(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected %d, got %d", http.StatusNoContent, rec.Code)
	}
}
