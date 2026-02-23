# VPC Network Management Console Plugin

This is an OpenShift Console dynamic plugin for managing IBM VPC networking resources for KubeVirt workloads. The plugin provides comprehensive management capabilities for VPCs, subnets, security groups, network ACLs, floating IPs, and network topology visualization.

## Overview

The VPC Network Management plugin is built as a dynamic console plugin that integrates seamlessly with the OpenShift Console under the Networking section. It enables users to:

- View and manage VPC resources
- Configure subnets and virtual network interfaces
- Manage security groups and network access control lists
- Monitor floating IPs and VLAN attachments
- Visualize network topology

## Project Structure

```
console-plugin/
├── src/
│   ├── api/
│   │   ├── client.ts         # BFF API client with consoleFetch
│   │   ├── k8s.ts            # Kubernetes CR helpers
│   │   ├── types.ts          # TypeScript interfaces
│   │   └── hooks.ts          # React data fetching hooks
│   ├── components/
│   │   ├── StatusBadge.tsx   # Status indicator component
│   │   └── DeleteConfirmModal.tsx  # Reusable delete modal
│   ├── hooks/
│   │   └── usePermissions.ts # User authorization hook
│   ├── utils/
│   │   ├── formatters.ts     # Data formatting utilities
│   │   └── validators.ts     # Input validation utilities
│   ├── pages/
│   │   ├── VPCDashboardPage.tsx
│   │   ├── SubnetsListPage.tsx
│   │   ├── SubnetDetailPage.tsx
│   │   ├── VNIsListPage.tsx
│   │   ├── VNIDetailPage.tsx
│   │   ├── VLANAttachmentsPage.tsx
│   │   ├── FloatingIPsPage.tsx
│   │   ├── SecurityGroupsListPage.tsx
│   │   ├── SecurityGroupDetailPage.tsx
│   │   ├── NetworkACLsListPage.tsx
│   │   ├── NetworkACLDetailPage.tsx
│   │   └── TopologyPage.tsx
│   └── plugin.ts             # Plugin entry point
├── package.json              # Dependencies and scripts
├── tsconfig.json             # TypeScript configuration
├── webpack.config.ts         # Webpack build configuration
├── console-extensions.json   # Console extension definitions
├── Dockerfile               # Multi-stage Docker build
├── nginx.conf              # Nginx web server config
└── .gitignore

```

## Features

### Pages

1. **VPC Dashboard**: Overview of VPC resources with summary cards
2. **VPC Subnets**: List and manage VPC subnets with CRUD operations
3. **Virtual Network Interfaces**: Manage network interface attachments
4. **VLAN Attachments**: Configure VLAN attachments to VNIs
5. **Floating IPs**: Reserve and manage floating IP addresses
6. **Security Groups**: Create and manage security group rules
7. **Network ACLs**: Configure subnet-level network access control
8. **Network Topology**: Visualize VPC network structure

### Components

- **StatusBadge**: Color-coded status indicators (green/blue/red/orange)
- **DeleteConfirmModal**: Reusable confirmation dialog for destructive operations

### API Integration

- **BFF Client**: REST API client for backend services
- **Kubernetes Integration**: Native K8s resource management via SDK
- **Data Hooks**: React hooks for data fetching and caching

## Development

### Prerequisites

- Node.js 18+
- npm or yarn
- TypeScript 5.3+

### Installation

```bash
npm install
# or
yarn install
```

### Build

```bash
# Development build
npm run build:dev

# Production build
npm run build

# Type checking
npm run ts-check
```

### Development Server

```bash
npm start
```

Starts webpack-dev-server on `http://localhost:9001`

## API Client

The plugin uses a BFF (Backend for Frontend) API client for VPC resource management.

### Base URL

```
/api/proxy/plugin/vpc-network-management/bff/api/v1
```

### Endpoints

#### VPCs
- `GET /vpcs` - List VPCs
- `GET /vpcs/{vpcId}` - Get VPC details
- `POST /vpcs` - Create VPC
- `PATCH /vpcs/{vpcId}` - Update VPC
- `DELETE /vpcs/{vpcId}` - Delete VPC

#### Subnets
- `GET /subnets` - List subnets
- `GET /subnets/{subnetId}` - Get subnet details
- `POST /subnets` - Create subnet
- `PATCH /subnets/{subnetId}` - Update subnet
- `DELETE /subnets/{subnetId}` - Delete subnet

#### Virtual Network Interfaces
- `GET /vnis` - List VNIs
- `GET /vnis/{vniId}` - Get VNI details
- `POST /vnis` - Create VNI
- `PATCH /vnis/{vniId}` - Update VNI
- `DELETE /vnis/{vniId}` - Delete VNI

#### Floating IPs
- `GET /floating-ips` - List floating IPs
- `GET /floating-ips/{floatingIpId}` - Get floating IP details
- `POST /floating-ips` - Reserve floating IP
- `PATCH /floating-ips/{floatingIpId}` - Update floating IP
- `DELETE /floating-ips/{floatingIpId}` - Release floating IP

#### Security Groups
- `GET /security-groups` - List security groups
- `GET /security-groups/{sgId}` - Get security group details
- `POST /security-groups` - Create security group
- `PATCH /security-groups/{sgId}` - Update security group
- `DELETE /security-groups/{sgId}` - Delete security group
- `POST /security-groups/{sgId}/rules` - Add rule
- `PATCH /security-groups/{sgId}/rules/{ruleId}` - Update rule
- `DELETE /security-groups/{sgId}/rules/{ruleId}` - Delete rule

#### Network ACLs
- `GET /network-acls` - List Network ACLs
- `GET /network-acls/{aclId}` - Get Network ACL details
- `POST /network-acls` - Create Network ACL
- `PATCH /network-acls/{aclId}` - Update Network ACL
- `DELETE /network-acls/{aclId}` - Delete Network ACL
- `POST /network-acls/{aclId}/rules` - Add rule
- `PATCH /network-acls/{aclId}/rules/{ruleId}` - Update rule
- `DELETE /network-acls/{aclId}/rules/{ruleId}` - Delete rule

#### Topology
- `GET /topology` - Get network topology (nodes and edges)

## Kubernetes Resources

The plugin supports management of custom Kubernetes resources:

### Models

- **VPCSubnet** (`vpc.roks.ibm.com/v1alpha1`)
- **VirtualNetworkInterface** (`vpc.roks.ibm.com/v1alpha1`)
- **VLANAttachment** (`vpc.roks.ibm.com/v1alpha1`)
- **FloatingIP** (`vpc.roks.ibm.com/v1alpha1`)

### Usage

```typescript
import { useK8sVPCSubnets, useK8sVNIs } from '@/api/hooks';

function MyComponent() {
  const { subnets, loading } = useK8sVPCSubnets('default');
  const { vnis, loading: vniLoading } = useK8sVNIs('default');

  // Use data...
}
```

## Type Safety

All resources are fully typed using TypeScript interfaces:

```typescript
import { VPC, SecurityGroup, NetworkACL, Subnet } from '@/api/types';

const vpc: VPC = {
  id: 'vpc-123',
  name: 'My VPC',
  status: 'available',
  // ...
};
```

## Utilities

### Formatters

```typescript
import { formatTimestamp, formatIPAddress, formatCIDRBlock } from '@/utils/formatters';

formatTimestamp(new Date()); // "Feb 22, 2026, 10:30:45 AM EST"
formatIPAddress('192.168.1.1'); // "192.168.1.1"
formatCIDRBlock('10.0.0.0/16'); // "10.0.0.0/16"
```

### Validators

```typescript
import { isValidCIDRBlock, isValidPort, validateSubnetConfiguration } from '@/utils/validators';

isValidCIDRBlock('10.0.0.0/16'); // true
isValidPort(8080); // true
validateSubnetConfiguration('10.0.0.0/24'); // { valid: true, errors: [] }
```

## Container Build

The plugin includes a multi-stage Dockerfile for containerization:

```bash
podman build -t vpc-network-management-plugin:latest .
podman run -p 9443:9443 vpc-network-management-plugin:latest
```

The container serves the plugin on port 9443 with TLS support.

### SSL/TLS Configuration

Mount certificates at runtime:

```bash
podman run -p 9443:9443 \
  -v /path/to/tls.crt:/etc/nginx/ssl/tls.crt:ro \
  -v /path/to/tls.key:/etc/nginx/ssl/tls.key:ro \
  vpc-network-management-plugin:latest
```

## Navigation

The plugin registers the following navigation items under the Networking section:

- VPC Dashboard → `/vpc-networking`
- VPC Subnets → `/vpc-networking/subnets`
- Virtual Network Interfaces → `/vpc-networking/vnis`
- VLAN Attachments → `/vpc-networking/vlan-attachments`
- Floating IPs → `/vpc-networking/floating-ips`
- Security Groups → `/vpc-networking/security-groups`
- Network ACLs → `/vpc-networking/network-acls`
- Network Topology → `/vpc-networking/topology`

## Permissions

The plugin checks user permissions via the auth API:

```typescript
import { usePermissions } from '@/hooks/usePermissions';

function AdminPanel() {
  const { isAdmin, canWrite, canDelete } = usePermissions();

  if (!isAdmin) return <AccessDenied />;

  return <AdminContent />;
}
```

## Configuration

### Environment Variables

The plugin respects the following environment variables:

- `NODE_ENV`: Build environment (development/production)
- `BASE_URL`: BFF API base URL (default: `/api/proxy/plugin/vpc-network-management/bff/api/v1`)

### Console Extensions

Modify `console-extensions.json` to customize:

- Navigation items
- Page routes
- Extension properties

## Testing

The project includes TypeScript strict mode. Run type checking:

```bash
npm run ts-check
```

## Performance

- Lazy-loaded page components for faster initial load
- Module federation for efficient chunk loading
- Shared dependencies with OpenShift Console
- Request caching via React hooks

## Security

- No hardcoded credentials
- RBAC-aware (checks user permissions)
- CSP-compliant headers via nginx
- TLS/SSL support
- Input validation for all user data

## Support

For issues or feature requests, please refer to the parent project documentation.
