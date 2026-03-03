# VPC Network Operator Documentation

Welcome to the VPC Network Operator documentation. This operator automates IBM Cloud VPC resource lifecycle for KubeVirt virtual machines on bare metal OpenShift clusters.

---

## Choose Your Path

### I'm an Executive or Decision Maker

Want to understand what this does and why it matters?

1. [What Is the VPC Network Operator?](overview/what-is-vpc-network-operator.md) — Non-technical introduction
2. [Use Cases](overview/use-cases.md) — Real-world scenarios
3. [Glossary](glossary.md) — Technical terms explained
4. [FAQ](faq.md) — Common questions answered

### I'm a Platform Administrator

Need to install, configure, and manage the operator?

1. [Key Concepts](overview/key-concepts.md) — VPC, Kubernetes, and KubeVirt fundamentals
2. [Prerequisites](getting-started/prerequisites.md) — What you need before installing
3. [VPC Prerequisites](admin-guide/vpc-prerequisites.md) — Setting up VPC resources
4. [Installation](getting-started/installation.md) — Step-by-step Helm install
5. [Configuration](admin-guide/configuration.md) — All Helm values and env vars
6. [Network Setup](admin-guide/network-setup.md) — Creating CUDNs and subnets
7. [RBAC](admin-guide/rbac.md) — Access control configuration
8. [Quick Start](getting-started/quick-start.md) — Deploy a VM in 10 minutes
9. [Troubleshooting](troubleshooting.md) — Common issues and solutions

### I'm a Developer or VM User

Need to deploy VMs with VPC networking?

1. [Key Concepts](overview/key-concepts.md) — Understanding the building blocks
2. [Quick Start](getting-started/quick-start.md) — Your first VM in 10 minutes
3. [Creating VMs](user-guide/creating-vms.md) — Detailed VM deployment guide
4. [Floating IPs](user-guide/floating-ips.md) — Public IP management
5. [End-to-End Tutorial](tutorials/end-to-end-vm-deployment.md) — Full walkthrough
6. [Advanced Networking Tutorial](tutorials/advanced-networking.md) — Multi-namespace enterprise topology
7. [Annotations Reference](user-guide/annotations-reference.md) — All operator annotations
8. [Console Plugin Overview](console-guide/overview.md) — Using the web UI

### I'm a Contributor or Architect

Want to understand the internals?

1. [Architecture Overview](architecture/overview.md) — Components, CRDs, repo layout
2. [Data Path](architecture/data-path.md) — VM-to-VPC packet flow
3. [Operator Internals](architecture/operator-internals.md) — Reconcilers, webhook, GC
4. [BFF Service](architecture/bff-service.md) — REST API architecture
5. [Console Plugin](architecture/console-plugin.md) — UI architecture
6. [Dual Cluster Mode](architecture/dual-cluster-mode.md) — ROKS vs. unmanaged
7. [CRD References](reference/crds/vpcsubnet.md) — Detailed field documentation
8. [BFF API Reference](reference/api/bff-api.md) — Complete endpoint docs
9. [Network Types Reference](reference/network-types.md) — Valid OVN network combinations
10. [Metrics Reference](reference/metrics.md) — Prometheus metrics

### I'm a Console User

Want to use the OpenShift Console plugin?

1. [Console Plugin Overview](console-guide/overview.md) — Navigation and access
2. [VPC Dashboard](console-guide/dashboard.md) — At-a-glance status
3. [Managing Resources](console-guide/managing-resources.md) — Subnets, VNIs, FIPs
4. [Security Groups & ACLs](console-guide/security.md) — Rule management
5. [Network Topology](console-guide/topology.md) — Visual resource map

---

## Quick Links

| Topic | Link |
|-------|------|
| Install the operator | [Installation](getting-started/installation.md) |
| Deploy your first VM | [Quick Start](getting-started/quick-start.md) |
| All annotations | [Annotations Reference](user-guide/annotations-reference.md) |
| All Helm values | [Helm Values Reference](reference/helm-values.md) |
| All CRDs | [VPCSubnet](reference/crds/vpcsubnet.md) / [VNI](reference/crds/virtualnetworkinterface.md) / [VLANAttachment](reference/crds/vlanattachment.md) / [FloatingIP](reference/crds/floatingip.md) |
| BFF API | [BFF API Reference](reference/api/bff-api.md) |
| Network types | [Network Types Reference](reference/network-types.md) |
| Prometheus metrics | [Metrics Reference](reference/metrics.md) |
| Troubleshooting | [Troubleshooting](troubleshooting.md) |
| FAQ | [FAQ](faq.md) |
| Glossary | [Glossary](glossary.md) |
