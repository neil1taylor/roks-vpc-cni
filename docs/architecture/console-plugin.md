# Console Plugin Architecture

The VPC Network Operator includes an OpenShift Console dynamic plugin that provides a web-based UI for viewing and managing VPC networking resources.

---

## Technology Stack

| Technology | Purpose |
|------------|---------|
| TypeScript | Type-safe application code |
| React | UI component framework |
| PatternFly 5 | Red Hat's enterprise design system |
| Webpack | Module bundling |
| Module Federation | Runtime code sharing with OpenShift Console |
| `@openshift-console/dynamic-plugin-sdk` | Console extension APIs |

---

## Module Federation

The console plugin uses Webpack Module Federation to integrate with the OpenShift Console at runtime. This means:

- The plugin is built and deployed separately from the console
- It shares React and PatternFly dependencies with the host console
- Pages and navigation entries are registered dynamically
- No console rebuild is required to add the plugin

The plugin is registered with the console via:

```bash
oc patch consoles.operator.openshift.io cluster \
  --type=merge \
  --patch '{"spec":{"plugins":["vpc-network-console-plugin"]}}'
```

---

## Navigation Structure

The plugin adds 8 navigation entries under the **Networking** section:

| Navigation Entry | Path | Component | Icon |
|-----------------|------|-----------|------|
| VPC Dashboard | `/vpc-networking` | `VPCDashboard` | CloudIcon |
| VPC Subnets | `/vpc-networking/subnets` | `SubnetsList` | NetworkIcon |
| Virtual Network Interfaces | `/vpc-networking/vnis` | `VNIsList` | NetworkIcon |
| VLAN Attachments | `/vpc-networking/vlan-attachments` | `VLANAttachments` | NetworkIcon |
| Floating IPs | `/vpc-networking/floating-ips` | `FloatingIPs` | IpIcon |
| Security Groups | `/vpc-networking/security-groups` | `SecurityGroupsList` | ShieldIcon |
| Network ACLs | `/vpc-networking/network-acls` | `NetworkACLsList` | ShieldIcon |
| Network Topology | `/vpc-networking/topology` | `Topology` | TopologyIcon |

---

## Page Routes

The plugin registers 12 page routes, including detail pages for resources that support drill-down:

| Route | Component | Description |
|-------|-----------|-------------|
| `/vpc-networking` | `VPCDashboard` | Overview dashboard |
| `/vpc-networking/subnets` | `SubnetsList` | Subnet list |
| `/vpc-networking/subnets/:name` | `SubnetDetail` | Subnet detail |
| `/vpc-networking/vnis` | `VNIsList` | VNI list |
| `/vpc-networking/vnis/:name` | `VNIDetail` | VNI detail |
| `/vpc-networking/vlan-attachments` | `VLANAttachments` | VLAN attachment list |
| `/vpc-networking/floating-ips` | `FloatingIPs` | Floating IP list |
| `/vpc-networking/security-groups` | `SecurityGroupsList` | Security group list |
| `/vpc-networking/security-groups/:name` | `SecurityGroupDetail` | SG detail with rules |
| `/vpc-networking/network-acls` | `NetworkACLsList` | ACL list |
| `/vpc-networking/network-acls/:name` | `NetworkACLDetail` | ACL detail with rules |
| `/vpc-networking/topology` | `Topology` | Network topology graph |

---

## Data Flow

The console plugin fetches data from the BFF service:

```
┌─────────────────┐     ┌──────────────────┐     ┌────────────┐
│ Console Plugin   │────►│  BFF Service      │────►│ VPC API    │
│ (browser)        │     │  (cluster pod)    │     │            │
│                  │◄────│                   │◄────│            │
│ React components │     │ REST API + AuthZ  │     │            │
└─────────────────┘     └──────────┬───────┘     └────────────┘
                                   │
                                   ▼
                          ┌────────────────┐
                          │ Kubernetes API  │
                          │ (CRDs, SARs)   │
                          └────────────────┘
```

1. The console plugin makes HTTP requests to the BFF service's REST API
2. The OpenShift OAuth proxy adds `X-Remote-User` and `X-Remote-Group` headers
3. The BFF authenticates the request and calls the VPC API or Kubernetes API
4. Results are returned as JSON to the console plugin

---

## Cluster Mode Awareness

On initialization, the console plugin calls `GET /api/v1/cluster-info` to determine the cluster mode and available features. In ROKS mode, the VNI and VLAN Attachment management pages may be hidden or shown as read-only, since the ROKS platform manages those resources.

```typescript
// Pseudocode
const { clusterMode, features } = await fetch('/api/v1/cluster-info');
if (!features.vniManagement) {
  // Hide VNI create/delete buttons
}
```

---

## Deployment

- **Image:** `icr.io/roks/vpc-network-console-plugin`
- **Replicas:** 2 (configurable via `consolePlugin.replicas`)
- **Port:** 9443 (configurable via `consolePlugin.port`)
- **Enabled by:** `consolePlugin.enabled: true` in Helm values

The console plugin is served as a static bundle by an nginx container. The OpenShift Console loads the plugin's module at runtime via the registered `ConsolePlugin` resource.

---

## Development

```bash
cd console-plugin
npm install         # Install dependencies
npm run build       # Production build
npm run build:dev   # Development build
npm start           # Dev server on port 9001
npm run ts-check    # TypeScript type checking
```

The dev server at port 9001 can be used with the OpenShift Console's plugin proxy for local development.

---

## Next Steps

- [Console Guide: Overview](../console-guide/overview.md) — User guide for the console UI
- [BFF Service Architecture](bff-service.md) — The API consumed by the plugin
- [BFF API Reference](../reference/api/bff-api.md) — Complete endpoint documentation
