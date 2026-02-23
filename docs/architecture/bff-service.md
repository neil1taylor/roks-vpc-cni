# BFF Service Architecture

The Backend for Frontend (BFF) service is a Go HTTP server that sits between the OpenShift Console plugin and the IBM Cloud VPC API. It aggregates data from multiple sources, handles authentication, and enforces authorization.

---

## Purpose

The console plugin runs in the user's browser and cannot call the VPC API directly (it requires a Service ID API key). The BFF service:

1. Authenticates requests using OpenShift-provided headers
2. Authorizes write operations via Kubernetes SubjectAccessReview
3. Calls the VPC API using the operator's Service ID credentials
4. Aggregates data from VPC API and Kubernetes API for the topology view
5. Returns cluster mode information so the console can show/hide features

---

## Deployment

- **Image:** `icr.io/roks/vpc-network-bff`
- **Replicas:** 2 (configurable via `bff.replicas`)
- **Port:** 8443 (configurable via `bff.port`)
- **Namespace:** Same as the operator (`roks-vpc-network-operator`)

---

## Authentication

The BFF uses request headers for authentication, following the OpenShift OAuth proxy pattern:

| Header | Content |
|--------|---------|
| `X-Remote-User` | Authenticated username |
| `X-Remote-Group` | Comma-separated group memberships |

The `AuthMiddleware` extracts these headers and stores them in the request context as a `UserInfo` struct. If no user header is present, the request proceeds without user context (read endpoints are accessible; write endpoints will fail authorization).

---

## Authorization

Write operations (create, delete) are authorized via Kubernetes SubjectAccessReview (SAR):

```
Console Plugin ──► BFF ──► SubjectAccessReview ──► K8s API
                    │                                  │
                    │           Is user "alice"         │
                    │           allowed to "create"     │
                    │           "securitygroups"?       │
                    │                                  │
                    │◄──────── allowed: true ──────────┘
                    │
                    ▼
              VPC API call
```

The SAR checks against the `vpc.ibm.com/v1alpha1` API group. RBAC policies (ClusterRoles and RoleBindings) control which users can perform which operations.

---

## REST API Routes

All routes are prefixed with `/api/v1/`. Authentication middleware wraps all data endpoints.

### Health Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/healthz` | No | Liveness probe |
| GET | `/readyz` | No | Readiness probe |

### Security Group Endpoints

| Method | Path | Auth | RBAC | Description |
|--------|------|------|------|-------------|
| GET | `/api/v1/security-groups` | Yes | — | List all security groups (optional `?vpc_id=` filter) |
| POST | `/api/v1/security-groups` | Yes | create securitygroups | Create a security group |
| GET | `/api/v1/security-groups/{id}` | Yes | — | Get security group with rules |
| DELETE | `/api/v1/security-groups/{id}` | Yes | delete securitygroups | Delete a security group |
| POST | `/api/v1/security-groups/{id}/rules` | Yes | create securitygroups | Add a rule |
| PATCH | `/api/v1/security-groups/{id}/rules/{ruleId}` | Yes | update securitygroups | Update a rule |
| DELETE | `/api/v1/security-groups/{id}/rules/{ruleId}` | Yes | delete securitygroups | Delete a rule |

### Network ACL Endpoints

| Method | Path | Auth | RBAC | Description |
|--------|------|------|------|-------------|
| GET | `/api/v1/network-acls` | Yes | — | List all ACLs (optional `?vpc_id=` filter) |
| POST | `/api/v1/network-acls` | Yes | create networkacls | Create an ACL |
| GET | `/api/v1/network-acls/{id}` | Yes | — | Get ACL with rules |
| DELETE | `/api/v1/network-acls/{id}` | Yes | delete networkacls | Delete an ACL |
| POST | `/api/v1/network-acls/{id}/rules` | Yes | create networkacls | Add a rule |
| PATCH | `/api/v1/network-acls/{id}/rules/{ruleId}` | Yes | update networkacls | Update a rule |
| DELETE | `/api/v1/network-acls/{id}/rules/{ruleId}` | Yes | delete networkacls | Delete a rule |

### VPC and Zone Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/v1/vpcs` | Yes | List all VPCs in the account |
| GET | `/api/v1/vpcs/{id}` | Yes | Get a specific VPC |
| GET | `/api/v1/zones` | Yes | List zones (optional `?region=` filter) |

### Topology Endpoint

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/v1/topology` | Yes | Aggregated network topology (VPCs, SGs, ACLs, nodes, edges) |

### Cluster Info Endpoint

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/v1/cluster-info` | No | Returns cluster mode and feature flags |

The cluster info response tells the console which features are available:

```json
{
  "clusterMode": "unmanaged",
  "features": {
    "vniManagement": true,
    "vlanAttachmentManagement": true,
    "subnetManagement": true,
    "securityGroupManagement": true,
    "networkACLManagement": true,
    "floatingIPManagement": true,
    "roksAPIAvailable": false
  }
}
```

In ROKS mode, `vniManagement` and `vlanAttachmentManagement` are `false` because the ROKS platform manages these resources.

---

## Error Response Format

All error responses follow a consistent JSON format:

```json
{
  "error": {
    "code": "ERROR_CODE",
    "message": "Human-readable error message"
  }
}
```

Common error codes:

| HTTP Status | Code | Description |
|-------------|------|-------------|
| 400 | `INVALID_REQUEST` | Malformed request body |
| 400 | `INVALID_PATH` | Missing resource ID in URL |
| 401 | `UNAUTHORIZED` | Missing user identity |
| 403 | `FORBIDDEN` | User not authorized for this action |
| 404 | `*_NOT_FOUND` | Resource not found (e.g., `SG_NOT_FOUND`) |
| 405 | `METHOD_NOT_ALLOWED` | HTTP method not supported for this route |
| 500 | `*_FAILED` | VPC API call failed (e.g., `LIST_SG_FAILED`) |

---

## Topology Aggregation

The `/api/v1/topology` endpoint constructs a graph of network resources:

- **Nodes** represent VPCs, security groups, and ACLs (each with type-specific data)
- **Edges** represent relationships (e.g., "VPC contains SG")

The topology handler fetches data from:
1. VPC API — Lists VPCs, security groups, and network ACLs
2. Kubernetes API — (planned) Lists CRD instances (subnets, VNIs, VMs)

Response format:

```json
{
  "nodes": [
    { "id": "vpc-123", "type": "vpc", "data": { "name": "my-vpc", "region": "us-south" } },
    { "id": "sg-456", "type": "sg", "data": { "name": "web-sg", "vpcID": "vpc-123" } }
  ],
  "edges": [
    { "source": "vpc-123", "target": "sg-456", "type": "contains" }
  ]
}
```

---

## Configuration

The BFF reads configuration from environment variables and Helm values:

| Value | Env Var | Default | Description |
|-------|---------|---------|-------------|
| `bff.clusterMode` | `CLUSTER_MODE` | `roks` | `roks` or `unmanaged` |
| `bff.port` | — | `8443` | Listen port |
| `bff.logLevel` | `LOG_LEVEL` | `info` | Log verbosity |
| `bff.apiKeySecretName` | — | `ibm-vpc-api-key` | Secret containing VPC API key |

---

## Next Steps

- [Console Plugin Architecture](console-plugin.md) — How the UI consumes the BFF API
- [BFF API Reference](../reference/api/bff-api.md) — Complete endpoint documentation
- [RBAC](../admin-guide/rbac.md) — Configuring access control
