# BFF (Backend-for-Frontend) - Files Created

This document lists all files created for the VPC Network Management console plugin BFF service.

## Project Structure

```
cmd/bff/
├── main.go                                  # HTTP server entry point
├── Dockerfile                              # Multi-stage container build
├── go.mod                                  # Module definition
├── README.md                               # Comprehensive documentation
├── FILES_CREATED.md                        # This file
└── internal/
    ├── auth/
    │   ├── middleware.go                   # Request authentication middleware
    │   └── rbac.go                         # RBAC authorization checks
    ├── credentials/
    │   └── loader.go                       # VPC API credential loading
    ├── handler/
    │   ├── router.go                       # Route registration & dispatcher
    │   ├── health.go                       # Health check endpoints
    │   ├── security_group.go               # Security group CRUD + rules
    │   ├── network_acl.go                  # Network ACL CRUD + rules
    │   ├── vpc.go                          # VPC listing & retrieval
    │   ├── zone.go                         # Availability zone listing
    │   ├── topology.go                     # Topology graph aggregation
    │   └── util.go                         # Handler utilities
    └── model/
        └── types.go                        # Request/response DTOs
```

## Files Overview

### Core Server

**main.go**
- HTTP server using net/http with ServeMux router
- Loads VPC credentials (ROKS or K8s Secret)
- Creates VPC client using vpc.NewExtendedClient()
- Creates K8s client for CRD operations and SubjectAccessReview
- Registers routes under /api/v1/
- Serves on BFF_LISTEN_ADDR (default ":8443")
- Health endpoints: /healthz, /readyz
- Structured JSON logging with slog
- Graceful shutdown handling

**Dockerfile**
- Multi-stage build: golang:1.22 → gcr.io/distroless/static-debian12
- Builds cmd/bff/main.go binary
- Exposes port 8443
- Includes health check probe

### Authentication & Authorization

**internal/auth/middleware.go**
- AuthMiddleware: Extracts X-Remote-User and X-Remote-Group headers
- Stores user info in request context
- RequireUserMiddleware: Ensures authentication for protected endpoints
- RBACMiddleware: Wraps handlers to check RBAC for write operations
- Structured logging for auth events

**internal/auth/rbac.go**
- RBACChecker: Provides RBAC authorization checks
- CheckAccess: Creates Kubernetes SubjectAccessReview
- Takes user, groups, verb, resource, namespace
- Returns allowed boolean and error
- UserInfo: Holds user identity information
- Context management functions for user info

### Credential Loading

**internal/credentials/loader.go**
- LoadCredentials: Main entry point for credential loading
- ROKS mode: Reads API key from CSI-mounted file
- Non-ROKS mode: Reads from K8s Secret
- Supports KUBECONFIG fallback for development
- Returns API key string or error
- Comprehensive error handling and logging

### Data Models

**internal/model/types.go**
- ErrorResponse: Standardized error format
- SecurityGroupRequest/Response: SG DTOs
- RuleRequest/Response: Security group rule DTOs
- NetworkACLRequest/Response: ACL DTOs
- ACLRuleRequest/Response: ACL rule DTOs
- VPCRequest/Response: VPC DTOs
- ZoneResponse: Zone DTOs
- TopologyResponse: Aggregated graph response
- TopologyNode/Edge: Graph components
- Specialized NodeData types: VPCNodeData, SubnetNodeData, VNINodeData, VMNodeData, SecurityGroupNodeData, ACLNodeData

### Request Handlers

**internal/handler/health.go**
- HealthHandler: GET /healthz
- ReadyHandler: GET /readyz
- Returns JSON status responses

**internal/handler/security_group.go**
- SecurityGroupHandler: Handles all SG operations
- ListSecurityGroups: GET /api/v1/security-groups
- CreateSecurityGroup: POST /api/v1/security-groups (with RBAC)
- GetSecurityGroup: GET /api/v1/security-groups/{id}
- DeleteSecurityGroup: DELETE /api/v1/security-groups/{id} (with RBAC)
- AddSecurityGroupRule: POST /api/v1/security-groups/{id}/rules (with RBAC)
- UpdateSecurityGroupRule: PATCH /api/v1/security-groups/{id}/rules/{ruleId} (with RBAC)
- DeleteSecurityGroupRule: DELETE /api/v1/security-groups/{id}/rules/{ruleId} (with RBAC)
- JSON request/response with proper error handling

**internal/handler/network_acl.go**
- NetworkACLHandler: Handles all ACL operations
- ListNetworkACLs: GET /api/v1/network-acls
- CreateNetworkACL: POST /api/v1/network-acls (with RBAC)
- GetNetworkACL: GET /api/v1/network-acls/{id}
- DeleteNetworkACL: DELETE /api/v1/network-acls/{id} (with RBAC)
- AddNetworkACLRule: POST /api/v1/network-acls/{id}/rules (with RBAC)
- UpdateNetworkACLRule: PATCH /api/v1/network-acls/{id}/rules/{ruleId} (with RBAC)
- DeleteNetworkACLRule: DELETE /api/v1/network-acls/{id}/rules/{ruleId} (with RBAC)

**internal/handler/vpc.go**
- VPCHandler: Handles VPC operations
- ListVPCs: GET /api/v1/vpcs (optional region filter)
- GetVPC: GET /api/v1/vpcs/{id}

**internal/handler/zone.go**
- ZoneHandler: Handles zone operations
- ListZones: GET /api/v1/zones (optional region filter)

**internal/handler/topology.go**
- TopologyHandler: Handles topology aggregation
- GetTopology: GET /api/v1/topology
- buildTopology: Aggregates VPC API and K8s CRD data
- Returns nodes (VPCs, subnets, VNIs, VMs, SGs, ACLs) and edges
- Response format: {"nodes": [...], "edges": [...]}

**internal/handler/util.go**
- WriteJSON: Writes JSON response with status code
- WriteError: Writes standardized error response
- ReadJSON: Reads and decodes JSON from request body
- GetQueryParam: Extracts query parameters

**internal/handler/router.go**
- SetupRoutes: Registers all handlers on mux
- SetupRoutesWithK8s: Extended setup with K8s client
- Route registration for all endpoints
- Standard net/http patterns (no external router framework)
- Middleware wrapping for auth
- Path-based routing for nested resources
- Detail and rule operation dispatchers

### Configuration

**go.mod**
- Module: github.com/IBM/roks-vpc-network-operator/cmd/bff
- Go 1.22
- Dependencies: k8s.io/api, k8s.io/apimachinery, k8s.io/client-go
- Local replace for parent module

## API Endpoints Summary

### Health
- GET /healthz
- GET /readyz

### Security Groups
- GET /api/v1/security-groups
- POST /api/v1/security-groups
- GET /api/v1/security-groups/{id}
- DELETE /api/v1/security-groups/{id}
- POST /api/v1/security-groups/{id}/rules
- PATCH /api/v1/security-groups/{id}/rules/{ruleId}
- DELETE /api/v1/security-groups/{id}/rules/{ruleId}

### Network ACLs
- GET /api/v1/network-acls
- POST /api/v1/network-acls
- GET /api/v1/network-acls/{id}
- DELETE /api/v1/network-acls/{id}
- POST /api/v1/network-acls/{id}/rules
- PATCH /api/v1/network-acls/{id}/rules/{ruleId}
- DELETE /api/v1/network-acls/{id}/rules/{ruleId}

### VPCs
- GET /api/v1/vpcs
- GET /api/v1/vpcs/{id}

### Zones
- GET /api/v1/zones

### Topology
- GET /api/v1/topology

## Key Features

✓ Clean, idiomatic Go code
✓ Standard net/http (no heavy frameworks)
✓ Role-Based Access Control (RBAC) via K8s SubjectAccessReview
✓ Kubernetes-native authentication (headers)
✓ Structured JSON logging
✓ Dual credential loading modes (ROKS and K8s Secret)
✓ Graceful shutdown handling
✓ Health check endpoints
✓ Comprehensive error handling
✓ Multi-stage Docker build
✓ Request/response type safety
✓ Context-aware logging
✓ Proper HTTP status codes
✓ Resource aggregation (topology)
✓ Query parameter support

## Dependencies

- k8s.io/api: Kubernetes API types
- k8s.io/apimachinery: Kubernetes utility libraries
- k8s.io/client-go: Kubernetes client library
- github.com/IBM/roks-vpc-network-operator/pkg/vpc: VPC client library
- Standard library only otherwise

## Development Notes

- All handlers implement net/http.HandlerFunc pattern
- Middleware uses http.Handler wrapping
- Path parsing uses strings package (no routing framework)
- Error responses follow RFC 7807 conventions
- Request body limits should be set in production
- TLS termination handled by ingress controller
- Namespace defaults to "default" - adjust for multi-tenancy if needed
