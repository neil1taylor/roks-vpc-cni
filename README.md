# VPC Network Operator

A Kubernetes operator that automates IBM Cloud VPC resource lifecycle for KubeVirt virtual machines on bare metal OpenShift (ROKS) clusters with OVN LocalNet networking.

## What It Does

When you create a network (CUDN), the operator provisions a VPC subnet and VLAN attachments. When you create a VM, a webhook transparently creates a VPC virtual network interface, injects the MAC address and IP, and optionally assigns a floating IP. On deletion, all VPC resources are automatically cleaned up.

## Quick Install

```bash
# 1. Create namespace
oc create namespace roks-vpc-network-operator

# 2. Store your IBM Cloud API key
oc create secret generic roks-vpc-network-operator-credentials \
  --namespace roks-vpc-network-operator \
  --from-literal=IBMCLOUD_API_KEY=<your-api-key>

# 3. Install via Helm
helm upgrade --install vpc-network-operator \
  deploy/helm/roks-vpc-network-operator \
  --namespace roks-vpc-network-operator \
  --set vpc.region=us-south \
  --set vpc.resourceGroupID=<your-resource-group-id> \
  --set cluster.id=<your-cluster-id>

# 4. Enable the console plugin
oc patch consoles.operator.openshift.io cluster \
  --type=merge \
  --patch '{"spec":{"plugins":["vpc-network-console-plugin"]}}'
```

## Components

| Component | Description |
|-----------|-------------|
| **Operator** (`roks-vpc-network-operator/`) | Go operator with 7 reconcilers, mutating webhook, and orphan GC |
| **BFF Service** (`roks-vpc-network-operator/cmd/bff/`) | REST API aggregating VPC + K8s data for the console |
| **Console Plugin** (`console-plugin/`) | OpenShift Console dynamic plugin (TypeScript/React/PatternFly 5) |

## CRDs

| CRD | Short Name | Description |
|-----|-----------|-------------|
| `VPCSubnet` | `vsn` | VPC subnet lifecycle |
| `VirtualNetworkInterface` | `vni` | Virtual network interface lifecycle |
| `VLANAttachment` | `vla` | Bare metal VLAN attachment lifecycle |
| `FloatingIP` | `fip` | Floating IP lifecycle |

## Documentation

Full documentation is in the [`docs/`](docs/index.md) directory:

| Section | Description |
|---------|-------------|
| [Overview](docs/overview/what-is-vpc-network-operator.md) | What the operator does and why |
| [Key Concepts](docs/overview/key-concepts.md) | VPC, Kubernetes, and KubeVirt fundamentals |
| [Getting Started](docs/getting-started/prerequisites.md) | Prerequisites, installation, quick start |
| [Admin Guide](docs/admin-guide/configuration.md) | Configuration, VPC setup, RBAC, upgrades |
| [User Guide](docs/user-guide/creating-vms.md) | Creating VMs, floating IPs, live migration |
| [Tutorial](docs/tutorials/end-to-end-vm-deployment.md) | End-to-end VM deployment walkthrough |
| [Architecture](docs/architecture/overview.md) | System design, data path, internals |
| [CRD Reference](docs/reference/crds/vpcsubnet.md) | Complete CRD field documentation |
| [API Reference](docs/reference/api/bff-api.md) | BFF REST API documentation |
| [Console Guide](docs/console-guide/overview.md) | Using the OpenShift Console plugin |
| [Helm Values](docs/reference/helm-values.md) | All configurable Helm values |
| [Metrics](docs/reference/metrics.md) | Prometheus metrics reference |
| [Troubleshooting](docs/troubleshooting.md) | Common issues and solutions |
| [FAQ](docs/faq.md) | Frequently asked questions |
| [Glossary](docs/glossary.md) | Technical terms explained |

## Development

### Operator

```bash
cd roks-vpc-network-operator
make build          # Build binary
make test           # Run tests
make lint           # Run linter
make docker-build   # Build container image
```

### Console Plugin

```bash
cd console-plugin
npm install         # Install dependencies
npm run build       # Production build
npm start           # Dev server on port 9001
```

## Architecture

```
kubectl apply CUDN ──► CUDN Reconciler ──► VPC Subnet + VLAN Attachments
kubectl apply VM   ──► Mutating Webhook ──► VNI + Reserved IP + Floating IP
                                            (MAC + IP injected into VM spec)
Node joins         ──► Node Reconciler  ──► VLAN Attachments for all CUDNs
Periodic (10min)   ──► Orphan GC        ──► Cleanup leaked VPC resources
```

## License

Licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE) for details.
