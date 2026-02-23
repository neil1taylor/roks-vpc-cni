# RBAC and Access Control

This guide covers the Kubernetes RBAC configuration used by the VPC Network Operator, the console plugin, and the BFF service. It also provides examples for creating custom roles for administrators and developers.

## Operator Service Account

The operator runs under a dedicated service account and requires cluster-scoped permissions to watch and manage resources across all namespaces.

### Required ClusterRole permissions

| Resource | API Group | Verbs |
|----------|-----------|-------|
| `clusteruserdefinednetworks` | `k8s.ovn.org` | get, list, watch, update, patch |
| `virtualmachines` | `kubevirt.io` | get, list, watch, update, patch |
| `nodes` | (core) | get, list, watch |
| `secrets` | (core) | get |
| `events` | (core) | create, patch |
| `vpcsubnets` | `vpc.roks.ibm.com` | get, list, watch, create, update, patch, delete |
| `virtualnetworkinterfaces` | `vpc.roks.ibm.com` | get, list, watch, create, update, patch, delete |
| `vlanattachments` | `vpc.roks.ibm.com` | get, list, watch, create, update, patch, delete |
| `floatingips` | `vpc.roks.ibm.com` | get, list, watch, create, update, patch, delete |

The Helm chart creates the ClusterRole and ClusterRoleBinding automatically. The operator service account also needs permissions for leader election (create/get/update on `leases` in the `coordination.k8s.io` API group within its own namespace).

### Webhook permissions

The mutating admission webhook requires permissions to read `MutatingWebhookConfiguration` resources and manage TLS certificates. These are also provisioned by the Helm chart.

## Console Plugin RBAC

The Helm chart includes configurable RBAC for the console plugin, controlled by the `pluginRbac` section in values.yaml.

### Admin binding

```yaml
pluginRbac:
  createAdminBinding: true
```

When `createAdminBinding` is `true` (the default), the chart creates a ClusterRoleBinding that grants users with `cluster-admin` privileges access to all VPC networking resources through the console plugin. This allows administrators to view and manage subnets, VNIs, VLAN attachments, and floating IPs from the OpenShift Console.

### Developer binding

```yaml
pluginRbac:
  createDeveloperBinding: true
  developerNamespaces:
    - team-alpha
    - team-beta
```

When `createDeveloperBinding` is `true`, the chart creates namespace-scoped RoleBindings in each listed namespace. This grants developers read access to VPC networking resources in their namespaces, enabling them to see network details for their VMs in the console plugin.

## Admin Role

Cluster administrators (users with `cluster-admin` or equivalent privileges) can perform all VPC networking operations:

- Create, modify, and delete CUDNs
- View all VPC CRDs (VPCSubnets, VNIs, VLANAttachments, FloatingIPs) across all namespaces
- Access the full console plugin dashboard and topology views
- Configure operator settings via Helm values
- View operator logs and events

No additional RBAC configuration is needed for cluster administrators -- they inherit full access through the `cluster-admin` ClusterRole.

## Developer Role

Developers typically need permissions to:

- Create and manage VirtualMachines in their namespaces
- View VPC networking resources (VNIs, FloatingIPs) associated with their VMs
- View network topology for their namespaces in the console plugin

Developers should not be able to:

- Create or delete CUDNs (cluster-scoped resource)
- Modify VPC subnets or VLAN attachments directly
- Access VPC networking resources in other namespaces

The `pluginRbac.createDeveloperBinding` Helm value creates appropriate namespace-scoped bindings. For more fine-grained control, use the custom RBAC policies described below.

## Custom RBAC Policies

### VPC Network Viewer

A read-only role for users who need visibility into VPC networking resources but should not modify them.

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: vpc-network-viewer
rules:
  - apiGroups: ["vpc.roks.ibm.com"]
    resources:
      - vpcsubnets
      - virtualnetworkinterfaces
      - vlanattachments
      - floatingips
    verbs: ["get", "list", "watch"]
  - apiGroups: ["k8s.ovn.org"]
    resources:
      - clusteruserdefinednetworks
    verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: vpc-network-viewers
subjects:
  - kind: Group
    name: vpc-network-viewers
    apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: vpc-network-viewer
  apiGroup: rbac.authorization.k8s.io
```

### VPC Network Admin

A role for network administrators who can manage CUDNs and VPC resources but do not need full cluster-admin privileges.

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: vpc-network-admin
rules:
  - apiGroups: ["vpc.roks.ibm.com"]
    resources:
      - vpcsubnets
      - virtualnetworkinterfaces
      - vlanattachments
      - floatingips
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: ["vpc.roks.ibm.com"]
    resources:
      - vpcsubnets/status
      - virtualnetworkinterfaces/status
      - vlanattachments/status
      - floatingips/status
    verbs: ["get", "list", "watch"]
  - apiGroups: ["k8s.ovn.org"]
    resources:
      - clusteruserdefinednetworks
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: ["kubevirt.io"]
    resources:
      - virtualmachines
    verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: vpc-network-admins
subjects:
  - kind: Group
    name: vpc-network-admins
    apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: vpc-network-admin
  apiGroup: rbac.authorization.k8s.io
```

### Namespace-scoped developer role

For developers who only need access to VPC resources in their own namespace:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: vpc-network-developer
  namespace: team-alpha
rules:
  - apiGroups: ["vpc.roks.ibm.com"]
    resources:
      - virtualnetworkinterfaces
      - floatingips
    verbs: ["get", "list", "watch"]
  - apiGroups: ["kubevirt.io"]
    resources:
      - virtualmachines
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: vpc-network-developers
  namespace: team-alpha
subjects:
  - kind: Group
    name: team-alpha-developers
    apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: Role
  name: vpc-network-developer
  apiGroup: rbac.authorization.k8s.io
```

## BFF Authorization

The BFF (Backend-for-Frontend) service enforces authorization for every API request using Kubernetes SubjectAccessReview (SAR).

### How it works

1. The console plugin sends requests to the BFF with the user's authentication token in the request headers.
2. The BFF extracts the user identity from the token.
3. Before returning any data, the BFF creates a SubjectAccessReview to verify the user has permission to access the requested resource type and namespace.
4. If the SAR check fails, the BFF returns a `403 Forbidden` response.

### What is checked

| BFF Endpoint | SAR Resource | SAR Verb |
|-------------|-------------|----------|
| GET /api/subnets | vpcsubnets | list |
| GET /api/subnets/:id | vpcsubnets | get |
| GET /api/vnis | virtualnetworkinterfaces | list |
| GET /api/vnis/:id | virtualnetworkinterfaces | get |
| GET /api/vlan-attachments | vlanattachments | list |
| GET /api/floating-ips | floatingips | list |
| GET /api/topology | vpcsubnets | list |

This means users can only see VPC networking data in the console plugin if they have the corresponding Kubernetes RBAC permissions. The custom roles described above control what data is visible.

### Implications

- Users with the `vpc-network-viewer` ClusterRole can access all read-only endpoints.
- Users with namespace-scoped roles (e.g., `vpc-network-developer`) can only see resources in their namespaces.
- Users with no VPC networking RBAC permissions will see empty views in the console plugin.

---

**See also:**

- [Configuration](configuration.md) -- `pluginRbac` Helm values
- [VPC Prerequisites](vpc-prerequisites.md) -- IBM Cloud IAM roles (distinct from Kubernetes RBAC)
- [Network Setup](network-setup.md) -- creating CUDNs (requires admin role)
