# VPC Prerequisites

Before installing the VPC Network Operator, several IBM Cloud VPC resources must be in place. This guide walks through each prerequisite with `ibmcloud` CLI commands.

## Overview

The operator does not create your VPC, security groups, or network ACLs. These foundational resources must exist before the operator can provision subnets, VNIs, VLAN attachments, and floating IPs within them.

You will need:

- An existing IBM Cloud VPC
- One or more security groups with appropriate rules
- A network ACL (optional but recommended)
- A CIDR plan for your VM subnets
- A Service ID with an API key that has the required IAM permissions

## Step 1: Verify Your VPC

Ensure you have a VPC in the target region. If you already have one, note its ID.

```bash
# Log in to IBM Cloud
ibmcloud login --sso

# Target the correct region
ibmcloud target -r us-south

# List all VPCs in the region
ibmcloud is vpcs

# Get details of a specific VPC
ibmcloud is vpc <vpc-id>
```

Record the VPC ID -- you will reference it in CUDN annotations.

If you do not have a VPC, create one:

```bash
ibmcloud is vpc-create my-vpc \
  --resource-group-id <resource-group-id>
```

## Step 2: Create Security Groups

Security groups control traffic to and from VNIs attached to your KubeVirt VMs. Create at least one security group with rules appropriate for your workloads.

### Create a security group

```bash
ibmcloud is security-group-create vm-default-sg <vpc-id> \
  --resource-group-id <resource-group-id>
```

Record the security group ID from the output.

### Add common inbound rules

Allow SSH access from a specific CIDR:

```bash
ibmcloud is security-group-rule-add <sg-id> inbound tcp \
  --port-min 22 --port-max 22 \
  --remote 10.0.0.0/8
```

Allow HTTP and HTTPS:

```bash
ibmcloud is security-group-rule-add <sg-id> inbound tcp \
  --port-min 80 --port-max 80 \
  --remote 0.0.0.0/0

ibmcloud is security-group-rule-add <sg-id> inbound tcp \
  --port-min 443 --port-max 443 \
  --remote 0.0.0.0/0
```

Allow ICMP (ping):

```bash
ibmcloud is security-group-rule-add <sg-id> inbound icmp \
  --icmp-type 8 --icmp-code 0 \
  --remote 0.0.0.0/0
```

Allow all traffic within the security group (VM-to-VM):

```bash
ibmcloud is security-group-rule-add <sg-id> inbound all \
  --remote <sg-id>
```

### Add outbound rules

Allow all outbound traffic (default for most use cases):

```bash
ibmcloud is security-group-rule-add <sg-id> outbound all \
  --remote 0.0.0.0/0
```

### Verify the security group rules

```bash
ibmcloud is security-group-rules <sg-id>
```

You can create multiple security groups for different workload tiers (e.g., `web-tier-sg`, `db-tier-sg`) and reference them in different CUDN annotations.

## Step 3: Create a Network ACL

Network ACLs provide subnet-level traffic filtering. While optional, they add a second layer of defense.

### Create a network ACL

```bash
ibmcloud is network-acl-create vm-subnet-acl <vpc-id> \
  --resource-group-id <resource-group-id>
```

Record the ACL ID from the output.

### Add ACL rules

Allow all inbound traffic from the VPC CIDR:

```bash
ibmcloud is network-acl-rule-add vm-subnet-acl allow inbound all \
  --source 10.0.0.0/8 --destination 0.0.0.0/0 \
  --name allow-internal-inbound
```

Allow all outbound traffic:

```bash
ibmcloud is network-acl-rule-add vm-subnet-acl allow outbound all \
  --source 0.0.0.0/0 --destination 0.0.0.0/0 \
  --name allow-all-outbound
```

Deny all other inbound traffic (default deny):

```bash
ibmcloud is network-acl-rule-add vm-subnet-acl deny inbound all \
  --source 0.0.0.0/0 --destination 0.0.0.0/0 \
  --name deny-all-inbound
```

ACL rules are evaluated in order. Place more specific allow rules before the deny-all rule.

### Verify the ACL

```bash
ibmcloud is network-acl vm-subnet-acl
ibmcloud is network-acl-rules vm-subnet-acl
```

## Step 4: Plan Your Subnets

Each CUDN creates a VPC subnet in a specific zone. Plan your CIDR ranges to avoid conflicts with existing VPC subnets, the cluster pod network, and the cluster service network.

### Guidelines

- Each VPC subnet CIDR must fall within the VPC's address prefixes.
- CIDRs must not overlap with other VPC subnets in the same VPC.
- Use at least a `/24` for small deployments (254 usable IPs) or `/20` for larger deployments (4094 usable IPs).
- Each VNI consumes one IP address from the subnet.

### Check existing subnets and address prefixes

```bash
# List VPC address prefixes
ibmcloud is vpc-address-prefixes <vpc-id>

# List existing subnets in the VPC
ibmcloud is subnets --vpc <vpc-id>
```

### Example CIDR plan

| Zone | CUDN Name | CIDR | Purpose |
|------|-----------|------|---------|
| us-south-1 | production-net | 10.240.10.0/24 | Production VM network |
| us-south-2 | production-net-2 | 10.240.20.0/24 | Production VM network (zone 2) |
| us-south-1 | dev-net | 10.240.30.0/24 | Development VM network |

If the CIDR you plan to use is outside the existing VPC address prefixes, add a new prefix:

```bash
ibmcloud is vpc-address-prefix-create <vpc-id> us-south-1 10.240.10.0/20 \
  --name vm-prefix-zone1
```

## Step 5: Create the Service ID and API Key

The operator authenticates to the IBM Cloud VPC API using an API key. Best practice is to use a Service ID with only the permissions the operator needs.

### Create the Service ID

```bash
ibmcloud iam service-id-create vpc-network-operator \
  --description "Service ID for VPC Network Operator"
```

### Assign IAM policies

The operator requires two roles on VPC Infrastructure Services:

**Editor** -- to create, update, and delete VPC subnets, VNIs, VLAN attachments, and floating IPs:

```bash
ibmcloud iam service-policy-create vpc-network-operator \
  --roles Editor \
  --service-name is \
  --resource-group-id <resource-group-id>
```

**IP Spoofing Operator** -- to create VNIs with `allow_ip_spoofing` enabled:

```bash
ibmcloud iam service-policy-create vpc-network-operator \
  --roles "IP Spoofing Operator" \
  --service-name is \
  --resource-group-id <resource-group-id>
```

### Create the API key

```bash
ibmcloud iam service-api-key-create vpc-operator-key vpc-network-operator \
  --description "API key for VPC Network Operator" \
  --output json
```

Save the `apikey` value from the output securely. You will not be able to retrieve it again.

### Create the Kubernetes secret

```bash
kubectl create namespace roks-vpc-network-operator

kubectl create secret generic roks-vpc-network-operator-credentials \
  --namespace roks-vpc-network-operator \
  --from-literal=IBMCLOUD_API_KEY=<your-api-key>
```

## Checklist

Before proceeding to installation, confirm the following:

- [ ] IBM Cloud VPC exists and you have recorded the VPC ID
- [ ] VPC address prefixes cover the CIDRs you plan to use for VM subnets
- [ ] At least one security group exists with appropriate rules; security group ID recorded
- [ ] (Optional) Network ACL exists with rules; ACL ID recorded
- [ ] CIDR ranges planned for each zone, with no overlaps
- [ ] Service ID created with Editor and IP Spoofing Operator roles on VPC Infrastructure Services
- [ ] API key created and stored securely
- [ ] Kubernetes namespace created
- [ ] Kubernetes Secret created with the API key
- [ ] Resource group ID recorded
- [ ] ROKS cluster ID recorded

---

**See also:**

- [Configuration](configuration.md) -- Helm values and environment variables
- [Network Setup](network-setup.md) -- creating CUDNs after installation
- [RBAC and Access Control](rbac.md) -- Kubernetes permissions
