# Prerequisites

Before installing the VPC Network Operator, ensure you have the following in place.

---

## Cluster Requirements

| Requirement | Details |
|-------------|---------|
| **Platform** | Red Hat OpenShift 4.20 or later |
| **Worker type** | At least one bare metal worker node |
| **Network plugin** | OVN-Kubernetes (default on OpenShift) |
| **KubeVirt** | OpenShift Virtualization operator installed |
| **Cluster type** | ROKS (managed) or self-managed OpenShift on IBM Cloud VPC |

### ROKS Clusters

If running on ROKS with bare metal workers, the infrastructure is pre-configured. The ROKS platform manages the underlying VPC networking and server provisioning.

### Unmanaged (Self-Managed) Clusters

If running a self-managed OpenShift cluster on IBM Cloud VPC infrastructure, you are responsible for VPC setup and IAM configuration (see below).

---

## IBM Cloud Requirements

### VPC Resources

You need an existing VPC in the same region as your cluster. The operator creates subnets, VNIs, VLAN attachments, and floating IPs within this VPC.

| Resource | Who Creates It | Notes |
|----------|----------------|-------|
| VPC | Administrator (pre-existing) | Must exist before operator installation |
| Security Groups | Administrator (pre-existing) | At least one SG for VM traffic. Referenced in CUDN annotations |
| Network ACL | Administrator (pre-existing) | One ACL per CUDN. Referenced in CUDN annotations |
| VPC Subnets | Operator (automated) | Created when a CUDN is applied |
| VNIs | Operator (automated) | Created when a VM is applied |
| VLAN Attachments | Operator (automated) | Created when a CUDN is applied or a node joins |
| Floating IPs | Operator (automated) | Created when a VM requests one |

See [VPC Prerequisites](../admin-guide/vpc-prerequisites.md) for step-by-step VPC setup instructions.

### IAM Permissions

The operator authenticates to the VPC API using a Service ID API key. The Service ID needs the following IAM roles, scoped to the cluster's resource group:

| IAM Role | Purpose |
|----------|---------|
| **VPC Infrastructure Services: Editor** | Create, read, update, and delete subnets, VNIs, VLAN attachments, floating IPs, and reserved IPs |
| **VPC Infrastructure Services: IP Spoofing Operator** | Enable `allow_ip_spoofing` on VNIs (required for KubeVirt VMs) |

To create the Service ID and API key:

```bash
# Create a Service ID
ibmcloud iam service-id-create vpc-network-operator \
  --description "VPC Network Operator for ROKS cluster"

# Assign Editor role
ibmcloud iam service-policy-create vpc-network-operator \
  --roles Editor \
  --service-name is \
  --resource-group-name <your-resource-group>

# Assign IP Spoofing Operator role
ibmcloud iam service-policy-create vpc-network-operator \
  --roles "IP Spoofing Operator" \
  --service-name is \
  --resource-group-name <your-resource-group>

# Create an API key
ibmcloud iam service-api-key-create vpc-network-operator-key vpc-network-operator \
  --description "API key for VPC Network Operator"
```

Save the API key securely. You will need it during [installation](installation.md).

---

## Tools

Ensure the following CLI tools are installed on your workstation:

| Tool | Version | Purpose |
|------|---------|---------|
| `kubectl` or `oc` | Latest | Interact with the Kubernetes cluster |
| `helm` | v3.x | Install the operator via Helm chart |
| `ibmcloud` CLI | Latest | (Optional) Manage IBM Cloud resources and create API keys |

---

## Network Planning

Before creating your first network (CUDN), plan the following:

1. **CIDR block** — Choose a non-overlapping IP range for each subnet (e.g., `10.240.64.0/24`). Ensure it does not overlap with existing VPC subnets or the cluster pod/service CIDRs.

2. **Zone** — Each VPC subnet exists in one availability zone. Choose the zone where your bare metal workers are located (e.g., `us-south-1`).

3. **VLAN ID** — Each CUDN needs a unique VLAN ID (1–4094) for OVN LocalNet tagging. This ID must not be used by any other CUDN in the cluster.

4. **Security groups** — Pre-create security groups in your VPC for the types of traffic you want to allow (e.g., SSH, HTTP, internal cluster traffic).

5. **Network ACL** — Pre-create a network ACL with rules appropriate for the subnet.

---

## Next Steps

- [Installation](installation.md) — Install the operator via Helm
- [VPC Prerequisites](../admin-guide/vpc-prerequisites.md) — Detailed VPC resource setup
- [Quick Start](quick-start.md) — Deploy your first VM in 10 minutes
