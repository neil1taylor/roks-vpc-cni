package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/IBM/roks-vpc-network-operator/pkg/vpc"
)

func TestListSecurityGroups(t *testing.T) {
	mock := vpc.NewMockClient()
	mock.ListSecurityGroupsFn = func(ctx context.Context, vpcID string) ([]vpc.SecurityGroup, error) {
		return []vpc.SecurityGroup{
			{ID: "sg-1", Name: "test-sg", VPCID: "vpc-1", Description: "Test SG"},
			{ID: "sg-2", Name: "test-sg-2", VPCID: "vpc-1"},
		}, nil
	}

	rbac := newAllowAllRBACChecker()
	h := NewSecurityGroupHandler(mock, rbac, "")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/security-groups", nil)
	h.ListSecurityGroups(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
	}
	var resp []map[string]interface{}
	decodeJSON(t, rec, &resp)
	if len(resp) != 2 {
		t.Fatalf("expected 2 security groups, got %d", len(resp))
	}
	if resp[0]["id"] != "sg-1" {
		t.Errorf("expected id 'sg-1', got %v", resp[0]["id"])
	}
}

func TestListSecurityGroups_WithVPCFilter(t *testing.T) {
	mock := vpc.NewMockClient()
	var capturedVPCID string
	mock.ListSecurityGroupsFn = func(ctx context.Context, vpcID string) ([]vpc.SecurityGroup, error) {
		capturedVPCID = vpcID
		return nil, nil
	}

	rbac := newAllowAllRBACChecker()
	h := NewSecurityGroupHandler(mock, rbac, "")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/security-groups?vpc_id=vpc-123", nil)
	h.ListSecurityGroups(rec, req)

	if capturedVPCID != "vpc-123" {
		t.Errorf("expected vpc_id filter 'vpc-123', got %q", capturedVPCID)
	}
}

func TestListSecurityGroups_Error(t *testing.T) {
	mock := vpc.NewMockClient()
	// Not configured — will return error

	rbac := newAllowAllRBACChecker()
	h := NewSecurityGroupHandler(mock, rbac, "")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/security-groups", nil)
	h.ListSecurityGroups(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected %d, got %d", http.StatusInternalServerError, rec.Code)
	}
}

func TestGetSecurityGroup(t *testing.T) {
	mock := vpc.NewMockClient()
	mock.GetSecurityGroupFn = func(ctx context.Context, sgID string) (*vpc.SecurityGroup, error) {
		return &vpc.SecurityGroup{
			ID:    sgID,
			Name:  "my-sg",
			VPCID: "vpc-1",
			Rules: []vpc.SecurityGroupRule{
				{
					ID:        "rule-1",
					Direction: "inbound",
					Protocol:  "tcp",
					PortMin:   int64Ptr(80),
					PortMax:   int64Ptr(80),
					Remote:    vpc.SecurityGroupRuleRemote{CIDRBlock: "0.0.0.0/0"},
				},
			},
		}, nil
	}

	rbac := newAllowAllRBACChecker()
	h := NewSecurityGroupHandler(mock, rbac, "")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/security-groups/sg-1", nil)
	h.GetSecurityGroup(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
	}
	var resp map[string]interface{}
	decodeJSON(t, rec, &resp)
	if resp["id"] != "sg-1" {
		t.Errorf("expected id 'sg-1', got %v", resp["id"])
	}
	rules := resp["rules"].([]interface{})
	if len(rules) != 1 {
		t.Errorf("expected 1 rule, got %d", len(rules))
	}
}

func TestCreateSecurityGroup(t *testing.T) {
	mock := vpc.NewMockClient()
	mock.CreateSecurityGroupFn = func(ctx context.Context, opts vpc.CreateSecurityGroupOptions) (*vpc.SecurityGroup, error) {
		return &vpc.SecurityGroup{
			ID:          "sg-new",
			Name:        opts.Name,
			VPCID:       opts.VPCID,
			Description: opts.Description,
		}, nil
	}

	rbac := newAllowAllRBACChecker()
	h := NewSecurityGroupHandler(mock, rbac, "")
	body := `{"name":"new-sg","vpc_id":"vpc-1","description":"My new SG"}`
	rec := httptest.NewRecorder()
	req := authRequest(http.MethodPost, "/api/v1/security-groups", body)
	h.CreateSecurityGroup(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected %d, got %d: %s", http.StatusCreated, rec.Code, rec.Body.String())
	}
	var resp map[string]interface{}
	decodeJSON(t, rec, &resp)
	if resp["name"] != "new-sg" {
		t.Errorf("expected name 'new-sg', got %v", resp["name"])
	}
}

func TestCreateSecurityGroup_Unauthorized(t *testing.T) {
	mock := vpc.NewMockClient()
	rbac := newAllowAllRBACChecker()
	h := NewSecurityGroupHandler(mock, rbac, "")
	body := `{"name":"new-sg","vpc_id":"vpc-1"}`
	rec := httptest.NewRecorder()
	// No auth headers
	req := httptest.NewRequest(http.MethodPost, "/api/v1/security-groups", strings.NewReader(body))
	h.CreateSecurityGroup(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestCreateSecurityGroup_Forbidden(t *testing.T) {
	mock := vpc.NewMockClient()
	rbac := newDenyAllRBACChecker()
	h := NewSecurityGroupHandler(mock, rbac, "")
	body := `{"name":"new-sg","vpc_id":"vpc-1"}`
	rec := httptest.NewRecorder()
	req := authRequest(http.MethodPost, "/api/v1/security-groups", body)
	h.CreateSecurityGroup(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected %d, got %d", http.StatusForbidden, rec.Code)
	}
}

func TestDeleteSecurityGroup(t *testing.T) {
	mock := vpc.NewMockClient()
	mock.DeleteSecurityGroupFn = func(ctx context.Context, sgID string) error {
		return nil
	}

	rbac := newAllowAllRBACChecker()
	h := NewSecurityGroupHandler(mock, rbac, "")
	rec := httptest.NewRecorder()
	req := authRequest(http.MethodDelete, "/api/v1/security-groups/sg-1", "")
	h.DeleteSecurityGroup(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected %d, got %d: %s", http.StatusNoContent, rec.Code, rec.Body.String())
	}
}

func TestAddSecurityGroupRule(t *testing.T) {
	mock := vpc.NewMockClient()
	mock.AddSecurityGroupRuleFn = func(ctx context.Context, sgID string, opts vpc.CreateSGRuleOptions) (*vpc.SecurityGroupRule, error) {
		return &vpc.SecurityGroupRule{
			ID:        "rule-new",
			Direction: opts.Direction,
			Protocol:  opts.Protocol,
			PortMin:   opts.PortMin,
			PortMax:   opts.PortMax,
		}, nil
	}

	rbac := newAllowAllRBACChecker()
	h := NewSecurityGroupHandler(mock, rbac, "")
	body := `{"direction":"inbound","protocol":"tcp","portMin":443,"portMax":443}`
	rec := httptest.NewRecorder()
	req := authRequest(http.MethodPost, "/api/v1/security-groups/sg-1/rules", body)
	h.AddSecurityGroupRule(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected %d, got %d: %s", http.StatusCreated, rec.Code, rec.Body.String())
	}
	var resp map[string]interface{}
	decodeJSON(t, rec, &resp)
	if resp["id"] != "rule-new" {
		t.Errorf("expected id 'rule-new', got %v", resp["id"])
	}
}

func TestDeleteSecurityGroupRule(t *testing.T) {
	mock := vpc.NewMockClient()
	mock.DeleteSecurityGroupRuleFn = func(ctx context.Context, sgID, ruleID string) error {
		return nil
	}

	rbac := newAllowAllRBACChecker()
	h := NewSecurityGroupHandler(mock, rbac, "")
	rec := httptest.NewRecorder()
	req := authRequest(http.MethodDelete, "/api/v1/security-groups/sg-1/rules/rule-1", "")
	h.DeleteSecurityGroupRule(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected %d, got %d", http.StatusNoContent, rec.Code)
	}
}

func TestUpdateSecurityGroupRule(t *testing.T) {
	mock := vpc.NewMockClient()
	mock.UpdateSecurityGroupRuleFn = func(ctx context.Context, sgID, ruleID string, opts vpc.UpdateSGRuleOptions) (*vpc.SecurityGroupRule, error) {
		return &vpc.SecurityGroupRule{
			ID:        ruleID,
			Direction: "inbound",
			Protocol:  "tcp",
		}, nil
	}

	rbac := newAllowAllRBACChecker()
	h := NewSecurityGroupHandler(mock, rbac, "")
	body := `{"direction":"inbound","portMin":8080,"portMax":8080}`
	rec := httptest.NewRecorder()
	req := authRequest(http.MethodPatch, "/api/v1/security-groups/sg-1/rules/rule-1", body)
	h.UpdateSecurityGroupRule(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}
}

func int64Ptr(v int64) *int64 {
	return &v
}
