# BFF API Reference

Complete reference for the VPC Network Operator's Backend for Frontend (BFF) REST API.

**Base URL:** `https://<bff-service>:8443`
**Authentication:** All `/api/v1/*` endpoints require `X-Remote-User` header (provided by OpenShift OAuth proxy).
**Content-Type:** `application/json`

---

## Health Endpoints

### GET /healthz

Liveness probe. Returns `200 OK` when the service is running.

**Response:**
```json
{"status": "ok"}
```

### GET /readyz

Readiness probe. Returns `200 OK` when the service is ready to accept requests.

**Response:**
```json
{"status": "ready"}
```

---

## Security Groups

### GET /api/v1/security-groups

List all security groups in the VPC account.

**Query Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `vpc_id` | string | No | Filter by VPC ID |

**Response:** `200 OK`
```json
[
  {
    "id": "r006-sg-abc123",
    "name": "web-tier-sg",
    "vpcID": "r006-vpc-def456",
    "description": "Security group for web tier",
    "createdAt": "2026-02-20T12:00:00Z"
  }
]
```

### POST /api/v1/security-groups

Create a new security group. **Requires RBAC: `create securitygroups`.**

**Request Body:**
```json
{
  "name": "web-tier-sg",
  "vpcID": "r006-vpc-def456",
  "description": "Security group for web tier"
}
```

**Response:** `201 Created`
```json
{
  "id": "r006-sg-abc123",
  "name": "web-tier-sg",
  "vpcID": "r006-vpc-def456",
  "description": "Security group for web tier",
  "createdAt": "2026-02-20T12:00:00Z"
}
```

### GET /api/v1/security-groups/{id}

Get a security group with its rules.

**Response:** `200 OK`
```json
{
  "id": "r006-sg-abc123",
  "name": "web-tier-sg",
  "vpcID": "r006-vpc-def456",
  "description": "Security group for web tier",
  "createdAt": "2026-02-20T12:00:00Z",
  "rules": [
    {
      "id": "r006-rule-xyz",
      "direction": "inbound",
      "protocol": "tcp",
      "portMin": 443,
      "portMax": 443,
      "remoteCIDR": "0.0.0.0/0",
      "remoteSGID": ""
    }
  ]
}
```

### DELETE /api/v1/security-groups/{id}

Delete a security group. **Requires RBAC: `delete securitygroups`.**

**Response:** `204 No Content`

### POST /api/v1/security-groups/{id}/rules

Add a rule to a security group. **Requires RBAC: `create securitygroups`.**

**Request Body:**
```json
{
  "direction": "inbound",
  "protocol": "tcp",
  "portMin": 22,
  "portMax": 22,
  "remoteCIDR": "10.0.0.0/8"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `direction` | string | Yes | `inbound` or `outbound` |
| `protocol` | string | Yes | `tcp`, `udp`, `icmp`, or `all` |
| `portMin` | integer | No | Minimum port (for TCP/UDP) |
| `portMax` | integer | No | Maximum port (for TCP/UDP) |
| `remoteCIDR` | string | No | Source/destination CIDR |
| `remoteSGID` | string | No | Source/destination security group ID |

**Response:** `201 Created`

### PATCH /api/v1/security-groups/{id}/rules/{ruleId}

Update a security group rule. **Requires RBAC: `update securitygroups`.** Only include fields to change.

**Request Body:**
```json
{
  "portMin": 8080,
  "portMax": 8080
}
```

**Response:** `200 OK`

### DELETE /api/v1/security-groups/{id}/rules/{ruleId}

Delete a security group rule. **Requires RBAC: `delete securitygroups`.**

**Response:** `204 No Content`

---

## Network ACLs

### GET /api/v1/network-acls

List all network ACLs.

**Query Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `vpc_id` | string | No | Filter by VPC ID |

**Response:** `200 OK`
```json
[
  {
    "id": "r006-acl-abc123",
    "name": "production-acl",
    "vpcID": "r006-vpc-def456",
    "createdAt": "2026-02-20T12:00:00Z"
  }
]
```

### POST /api/v1/network-acls

Create a network ACL. **Requires RBAC: `create networkacls`.**

**Request Body:**
```json
{
  "name": "production-acl",
  "vpcID": "r006-vpc-def456"
}
```

**Response:** `201 Created`

### GET /api/v1/network-acls/{id}

Get a network ACL with its rules.

**Response:** `200 OK`
```json
{
  "id": "r006-acl-abc123",
  "name": "production-acl",
  "vpcID": "r006-vpc-def456",
  "createdAt": "2026-02-20T12:00:00Z",
  "rules": [
    {
      "id": "r006-rule-xyz",
      "name": "allow-ssh-inbound",
      "action": "allow",
      "direction": "inbound",
      "protocol": "tcp",
      "source": "10.0.0.0/8",
      "destination": "10.240.64.0/24",
      "portMin": 22,
      "portMax": 22
    }
  ]
}
```

### DELETE /api/v1/network-acls/{id}

Delete a network ACL. **Requires RBAC: `delete networkacls`.**

**Response:** `204 No Content`

### POST /api/v1/network-acls/{id}/rules

Add a rule to a network ACL. **Requires RBAC: `create networkacls`.**

**Request Body:**
```json
{
  "name": "allow-ssh-inbound",
  "action": "allow",
  "direction": "inbound",
  "protocol": "tcp",
  "source": "10.0.0.0/8",
  "destination": "10.240.64.0/24",
  "portMin": 22,
  "portMax": 22
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Rule name |
| `action` | string | Yes | `allow` or `deny` |
| `direction` | string | Yes | `inbound` or `outbound` |
| `protocol` | string | Yes | `tcp`, `udp`, `icmp`, or `all` |
| `source` | string | Yes | Source CIDR |
| `destination` | string | Yes | Destination CIDR |
| `portMin` | integer | No | Minimum port (for TCP/UDP) |
| `portMax` | integer | No | Maximum port (for TCP/UDP) |

**Response:** `201 Created`

### PATCH /api/v1/network-acls/{id}/rules/{ruleId}

Update a network ACL rule. **Requires RBAC: `update networkacls`.**

**Response:** `200 OK`

### DELETE /api/v1/network-acls/{id}/rules/{ruleId}

Delete a network ACL rule. **Requires RBAC: `delete networkacls`.**

**Response:** `204 No Content`

---

## VPCs

### GET /api/v1/vpcs

List all VPCs in the account.

**Response:** `200 OK`
```json
[
  {
    "id": "r006-vpc-abc123",
    "name": "production-vpc",
    "region": "us-south",
    "createdAt": "2026-01-15T10:00:00Z",
    "status": "available"
  }
]
```

### GET /api/v1/vpcs/{id}

Get a specific VPC.

**Response:** `200 OK`

---

## Zones

### GET /api/v1/zones

List availability zones.

**Query Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `region` | string | No | Filter by region (e.g., `us-south`) |

**Response:** `200 OK`
```json
[
  {
    "name": "us-south-1",
    "region": "us-south",
    "status": "available"
  },
  {
    "name": "us-south-2",
    "region": "us-south",
    "status": "available"
  }
]
```

---

## Topology

### GET /api/v1/topology

Get the aggregated network topology graph.

**Response:** `200 OK`
```json
{
  "nodes": [
    {
      "id": "r006-vpc-abc123",
      "type": "vpc",
      "data": {
        "name": "production-vpc",
        "region": "us-south",
        "status": "available"
      }
    },
    {
      "id": "r006-sg-def456",
      "type": "sg",
      "data": {
        "name": "web-tier-sg",
        "vpcID": "r006-vpc-abc123",
        "description": "Web tier security group",
        "ruleCount": 5
      }
    },
    {
      "id": "r006-acl-ghi789",
      "type": "acl",
      "data": {
        "name": "production-acl",
        "vpcID": "r006-vpc-abc123",
        "ruleCount": 10
      }
    }
  ],
  "edges": [
    {
      "source": "r006-vpc-abc123",
      "target": "r006-sg-def456",
      "type": "contains"
    },
    {
      "source": "r006-vpc-abc123",
      "target": "r006-acl-ghi789",
      "type": "contains"
    }
  ]
}
```

**Node types:** `vpc`, `sg`, `acl`, `subnet`, `vni`, `vm`
**Edge types:** `contains`, `attached-to`, `bound-to`

---

## Network Types

### GET /api/v1/network-types

Returns the valid network topology+scope+role combinations. Use this to populate network creation wizards and validate user selections.

**Response:** `200 OK`
```json
{
  "topologies": ["LocalNet", "Layer2"],
  "scopes": ["ClusterUserDefinedNetwork", "UserDefinedNetwork"],
  "roles": ["Primary", "Secondary"],
  "combinations": [
    {
      "id": "localnet-cudn-secondary",
      "topology": "LocalNet",
      "scope": "ClusterUserDefinedNetwork",
      "role": "Secondary",
      "tier": "recommended",
      "ip_mode": "static_reserved",
      "requires_vpc": true,
      "label": "LocalNet Cluster Secondary",
      "description": "Cluster-wide secondary network backed by a VPC subnet.",
      "ip_mode_description": "VPC API reserves a static IP from the subnet when the VNI is created."
    }
  ]
}
```

**Valid Combinations:**

| ID | Topology | Scope | Role | VPC? | Tier |
|----|----------|-------|------|------|------|
| `localnet-cudn-secondary` | LocalNet | CUDN | Secondary | Yes | Recommended |
| `layer2-cudn-secondary` | Layer2 | CUDN | Secondary | No | Recommended |
| `layer2-udn-secondary` | Layer2 | UDN | Secondary | No | Advanced |
| `layer2-cudn-primary` | Layer2 | CUDN | Primary | No | Expert |

---

## CUDNs (ClusterUserDefinedNetworks)

### GET /api/v1/cudns

List all ClusterUserDefinedNetworks in the cluster.

**Response:** `200 OK`
```json
[
  {
    "name": "production-net",
    "kind": "ClusterUserDefinedNetwork",
    "topology": "LocalNet",
    "role": "Secondary",
    "tier": "recommended",
    "ip_mode": "static_reserved",
    "subnet_id": "0717-abc123",
    "vpc_id": "r010-def456",
    "zone": "eu-de-1",
    "cidr": "10.240.10.0/24",
    "vlan_id": "100"
  }
]
```

### GET /api/v1/cudns/{name}

Get a specific CUDN by name.

**Response:** `200 OK` (same schema as list item)

### POST /api/v1/cudns

Create a ClusterUserDefinedNetwork.

**Request Body:**
```json
{
  "name": "production-net",
  "topology": "LocalNet",
  "role": "Secondary",
  "vpc_id": "r010-def456",
  "zone": "eu-de-1",
  "cidr": "10.240.10.0/24",
  "vlan_id": "100",
  "security_group_ids": "r010-sg1,r010-sg2",
  "acl_id": "r010-acl1",
  "target_namespaces": ["tenant-a", "tenant-b"]
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Network name (DNS-compatible) |
| `topology` | string | Yes | `LocalNet` or `Layer2` |
| `role` | string | No | `Primary` or `Secondary` (default: `Secondary`) |
| `vpc_id` | string | LocalNet only | VPC ID for subnet creation |
| `zone` | string | LocalNet only | Availability zone |
| `cidr` | string | LocalNet / Layer2 Primary | Subnet CIDR block |
| `vlan_id` | string | LocalNet only | VLAN ID (1-4094) |
| `security_group_ids` | string | No | Comma-separated security group IDs |
| `acl_id` | string | No | Network ACL ID |
| `target_namespaces` | string[] | No | Restrict to specific namespaces (empty = all) |

**Response:** `201 Created`

### DELETE /api/v1/cudns/{name}

Delete a CUDN by name.

**Response:** `200 OK`
```json
{"status": "deleted"}
```

---

## UDNs (UserDefinedNetworks)

### GET /api/v1/udns

List all UserDefinedNetworks across namespaces.

**Query Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `namespace` | string | No | Filter by namespace |

**Response:** `200 OK`
```json
[
  {
    "name": "app-net",
    "namespace": "my-app",
    "kind": "UserDefinedNetwork",
    "topology": "Layer2",
    "role": "Secondary",
    "tier": "advanced",
    "ip_mode": "dhcp"
  }
]
```

### GET /api/v1/udns/{namespace}/{name}

Get a specific UDN.

**Response:** `200 OK` (same schema as list item)

### POST /api/v1/udns

Create a UserDefinedNetwork. Only `Layer2` topology with `Secondary` role is supported for UDNs.

**Request Body:**
```json
{
  "name": "app-net",
  "namespace": "my-app",
  "topology": "Layer2",
  "role": "Secondary",
  "cidr": "10.100.0.0/24"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Network name |
| `namespace` | string | Yes | Target namespace |
| `topology` | string | Yes | Must be `Layer2` (UDN does not support LocalNet) |
| `role` | string | No | Must be `Secondary` (UDN Layer2 does not support Primary) |
| `cidr` | string | No | Subnet CIDR for OVN IPAM (omit for disabled IPAM) |

**Response:** `201 Created`

**Validation Errors:**
- `UDN does not support LocalNet topology; use ClusterUserDefinedNetwork instead`
- `UDN Layer2 only supports Secondary role; use ClusterUserDefinedNetwork for Primary`

### DELETE /api/v1/udns/{namespace}/{name}

Delete a UDN.

**Response:** `200 OK`
```json
{"status": "deleted"}
```

---

## Namespaces

### GET /api/v1/namespaces

List namespaces with primary network label information.

**Response:** `200 OK`
```json
[
  {
    "name": "default",
    "hasPrimaryLabel": false
  },
  {
    "name": "primary-net-ns",
    "hasPrimaryLabel": true
  }
]
```

### POST /api/v1/namespaces

Create a namespace with optional labels.

**Request Body:**
```json
{
  "name": "my-namespace",
  "labels": {
    "k8s.ovn.org/primary-user-defined-network": ""
  }
}
```

**Response:** `201 Created`

---

## Cluster Info

### GET /api/v1/cluster-info

Get cluster mode and feature flags. No authentication required.

**Response:** `200 OK`
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

---

## Error Responses

All errors follow this format:

```json
{
  "error": {
    "code": "ERROR_CODE",
    "message": "Human-readable description"
  }
}
```

| HTTP Status | Code | When |
|-------------|------|------|
| 400 | `INVALID_REQUEST` | Malformed request body |
| 400 | `INVALID_PATH` | Missing resource ID in URL path |
| 401 | `UNAUTHORIZED` | Missing `X-Remote-User` header |
| 403 | `FORBIDDEN` | User lacks required RBAC permission |
| 404 | `SG_NOT_FOUND` / `ACL_NOT_FOUND` / `VPC_NOT_FOUND` | Resource not found |
| 405 | `METHOD_NOT_ALLOWED` | HTTP method not supported |
| 500 | `LIST_SG_FAILED` / `CREATE_SG_FAILED` / etc. | VPC API call failed |
| 500 | `AUTHZ_CHECK_FAILED` | SubjectAccessReview failed |
| 500 | `TOPOLOGY_FAILED` | Failed to build topology |
