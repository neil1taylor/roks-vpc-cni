# VPC Network Management BFF (Backend-for-Frontend)

Backend-for-Frontend service that provides REST API endpoints for the VPC Network Management console plugin. The BFF aggregates data from both the IBM VPC API (for resources like Security Groups, ACLs, VPCs, and Zones) and Kubernetes custom resources (for cluster-native networking objects).

## Features

- REST API endpoints for VPC resources not backed by CRDs
- Security Groups management with rule operations
- Network ACLs management with rule operations
- VPC listing and retrieval
- Availability zones listing
- Topology aggregation endpoint combining VPC and K8s resources
- Role-Based Access Control (RBAC) using Kubernetes SubjectAccessReview
- Structured JSON logging
- Kubernetes-native authentication and authorization

## Architecture

### Directory Structure

```
cmd/bff/
├── main.go                          # Server entry point
├── Dockerfile                       # Multi-stage container build
├── go.mod                          # Module definition
├── internal/
│   ├── auth/
│   │   ├── middleware.go           # Request authentication middleware
│   │   └── rbac.go                 # RBAC authorization checks
│   ├── credentials/
│   │   └── loader.go               # VPC API credential loading
│   ├── handler/
│   │   ├── router.go               # Route registration
│   │   ├── health.go               # Health check endpoints
│   │   ├── security_group.go       # Security group operations
│   │   ├── network_acl.go          # Network ACL operations
│   │   ├── vpc.go                  # VPC operations
│   │   ├── zone.go                 # Zone operations
│   │   ├── topology.go             # Topology aggregation
│   │   └── util.go                 # Handler utilities
│   ├── model/
│   │   └── types.go                # Request/response DTOs
└── README.md                       # This file
```

## Configuration

### Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `BFF_LISTEN_ADDR` | No | `:8443` | Server listen address |
| `BFF_CLUSTER_MODE` | No | - | Set to "roks" for ROKS deployment |
| `BFF_CSI_MOUNT_PATH` | No | `/etc/vpc-credentials` | CSI mount path for API key (ROKS mode) |
| `BFF_SECRET_NAME` | No | `vpc-api-credentials` | K8s Secret name (non-ROKS mode) |
| `BFF_SECRET_NAMESPACE` | No | `default` | K8s Secret namespace (non-ROKS mode) |
| `KUBECONFIG` | No | `~/.kube/config` | Path to kubeconfig (dev mode only) |

### Credential Loading

The BFF supports two credential loading modes:

1. **ROKS Mode** (`BFF_CLUSTER_MODE=roks`)
   - Reads VPC API key from file at `BFF_CSI_MOUNT_PATH/apikey`
   - Suitable for ROKS clusters with CSI volume mounting

2. **Non-ROKS Mode** (default)
   - Reads VPC API key from Kubernetes Secret
   - Secret name: `BFF_SECRET_NAME`
   - Secret namespace: `BFF_SECRET_NAMESPACE`
   - Key: `apikey`

## API Endpoints

### Health Checks

```
GET /healthz                    # Liveness probe
GET /readyz                     # Readiness probe
```

### Security Groups

```
GET    /api/v1/security-groups                     # List SGs (query: vpc_id)
POST   /api/v1/security-groups                     # Create SG
GET    /api/v1/security-groups/{id}                # Get SG with rules
DELETE /api/v1/security-groups/{id}                # Delete SG

POST   /api/v1/security-groups/{id}/rules          # Add rule to SG
PATCH  /api/v1/security-groups/{id}/rules/{ruleId} # Update rule
DELETE /api/v1/security-groups/{id}/rules/{ruleId} # Delete rule
```

### Network ACLs

```
GET    /api/v1/network-acls                     # List ACLs (query: vpc_id)
POST   /api/v1/network-acls                     # Create ACL
GET    /api/v1/network-acls/{id}                # Get ACL with rules
DELETE /api/v1/network-acls/{id}                # Delete ACL

POST   /api/v1/network-acls/{id}/rules          # Add rule to ACL
PATCH  /api/v1/network-acls/{id}/rules/{ruleId} # Update rule
DELETE /api/v1/network-acls/{id}/rules/{ruleId} # Delete rule
```

### VPCs

```
GET /api/v1/vpcs           # List VPCs (query: region)
GET /api/v1/vpcs/{id}      # Get VPC details
```

### Zones

```
GET /api/v1/zones          # List zones (query: region)
```

### Topology

```
GET /api/v1/topology       # Get aggregated topology graph
```

## Request/Response Examples

### Create Security Group

**Request:**
```bash
curl -X POST https://localhost:8443/api/v1/security-groups \
  -H "Content-Type: application/json" \
  -H "X-Remote-User: user@example.com" \
  -d '{
    "name": "web-sg",
    "vpc_id": "vpc-12345",
    "description": "Web server security group"
  }'
```

**Response:**
```json
{
  "id": "sg-abc123",
  "name": "web-sg",
  "vpc_id": "vpc-12345",
  "description": "Web server security group",
  "created_at": "2024-01-15T10:30:00Z",
  "rules": []
}
```

### Add Security Group Rule

**Request:**
```bash
curl -X POST https://localhost:8443/api/v1/security-groups/sg-abc123/rules \
  -H "Content-Type: application/json" \
  -H "X-Remote-User: user@example.com" \
  -d '{
    "direction": "inbound",
    "protocol": "tcp",
    "port_min": 80,
    "port_max": 80,
    "cidr": "0.0.0.0/0"
  }'
```

**Response:**
```json
{
  "id": "rule-123",
  "direction": "inbound",
  "protocol": "tcp",
  "port_min": 80,
  "port_max": 80,
  "cidr": "0.0.0.0/0",
  "created_at": "2024-01-15T10:31:00Z"
}
```

## Authentication & Authorization

### Authentication

The BFF uses HTTP headers for user identity:
- `X-Remote-User`: Username or user identifier
- `X-Remote-Group`: Comma-separated list of group memberships

These headers are typically set by an API gateway or ingress controller (e.g., OAuth2 Proxy, Keycloak).

### Authorization

Write operations (create, update, delete) are protected by Kubernetes RBAC:

1. User credentials from headers are extracted
2. A Kubernetes SubjectAccessReview (SAR) is submitted
3. The K8s API server evaluates RBAC policies
4. Request proceeds only if authorized

Required RBAC permissions:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: vpc-admin
rules:
- apiGroups: ["vpc.ibm.com"]
  resources: ["securitygroups"]
  verbs: ["get", "list", "create", "update", "delete"]
- apiGroups: ["vpc.ibm.com"]
  resources: ["networkacls"]
  verbs: ["get", "list", "create", "update", "delete"]
```

## Building & Deployment

### Build Docker Image

```bash
docker build -t vpc-bff:latest -f cmd/bff/Dockerfile .
```

### Run Locally (Development)

```bash
# Set credentials via Secret
kubectl create secret generic vpc-api-credentials \
  --from-literal=apikey=$VPC_API_KEY

# Run BFF
go run cmd/bff/main.go
```

### Deploy to Kubernetes

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: vpc-bff
  namespace: vpc-system

---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: vpc-bff
  namespace: vpc-system
spec:
  replicas: 2
  selector:
    matchLabels:
      app: vpc-bff
  template:
    metadata:
      labels:
        app: vpc-bff
    spec:
      serviceAccountName: vpc-bff
      containers:
      - name: bff
        image: vpc-bff:latest
        imagePullPolicy: IfNotPresent
        ports:
        - containerPort: 8443
          name: https
        env:
        - name: BFF_LISTEN_ADDR
          value: ":8443"
        - name: BFF_SECRET_NAME
          value: vpc-api-credentials
        - name: BFF_SECRET_NAMESPACE
          value: vpc-system
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8443
            scheme: HTTPS
          initialDelaySeconds: 10
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /readyz
            port: 8443
            scheme: HTTPS
          initialDelaySeconds: 5
          periodSeconds: 5
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 500m
            memory: 512Mi

---
apiVersion: v1
kind: Service
metadata:
  name: vpc-bff
  namespace: vpc-system
spec:
  selector:
    app: vpc-bff
  ports:
  - port: 8443
    targetPort: 8443
    protocol: TCP
```

## Error Handling

All errors are returned as JSON with standardized error responses:

```json
{
  "error": "Invalid request",
  "code": "INVALID_REQUEST",
  "message": "Invalid request body"
}
```

Common error codes:
- `INVALID_REQUEST` - Malformed request body
- `UNAUTHORIZED` - Missing authentication
- `FORBIDDEN` - Authorization denied
- `NOT_FOUND` - Resource not found
- `METHOD_NOT_ALLOWED` - HTTP method not supported
- `INTERNAL_ERROR` - Server-side error
- `LIST_SG_FAILED` - Failed to list security groups
- `CREATE_SG_FAILED` - Failed to create security group
- `DELETE_SG_FAILED` - Failed to delete security group

## Logging

The BFF uses structured JSON logging with slog. Log levels:
- DEBUG: Detailed operation information
- INFO: General informational messages
- WARN: Warning conditions
- ERROR: Error conditions

Example log output:
```json
{
  "time": "2024-01-15T10:30:00.123Z",
  "level": "INFO",
  "msg": "user authenticated",
  "user": "user@example.com",
  "groups": ["developers", "vpc-admins"]
}
```

## Testing

### Unit Tests

```bash
go test ./...
```

### Integration Tests

```bash
go test -tags integration ./...
```

### Manual API Testing

```bash
# Get security groups
curl -H "X-Remote-User: test-user" \
  http://localhost:8443/api/v1/security-groups

# Create security group
curl -X POST http://localhost:8443/api/v1/security-groups \
  -H "Content-Type: application/json" \
  -H "X-Remote-User: test-user" \
  -d '{"name":"test-sg","vpc_id":"vpc-123"}'
```

## Troubleshooting

### Cannot load credentials

Ensure the credential source is properly configured:
- Check environment variables are set correctly
- Verify Secret exists in K8s cluster
- Check file permissions for CSI mount

### Authorization denied

Check RBAC role bindings:
```bash
kubectl get rolebindings -n vpc-system
kubectl get clusterrolebindings | grep vpc
```

### Service won't start

Check logs:
```bash
kubectl logs -n vpc-system deployment/vpc-bff
```

## Contributing

See the main project README for contribution guidelines.

## License

This project is licensed under the Apache 2.0 License.
