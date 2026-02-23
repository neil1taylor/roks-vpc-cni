# Security Groups and Network ACLs

The console plugin provides pages for viewing and managing VPC security groups and network ACLs, including creating, deleting, and editing individual rules.

---

## Security Groups

**Path:** `/vpc-networking/security-groups`

### List View

Displays all security groups in the VPC:

| Column | Description |
|--------|-------------|
| Name | Security group name |
| ID | VPC security group ID |
| VPC | Associated VPC ID |
| Description | Human-readable description |
| Rule Count | Number of rules |
| Created | Creation timestamp |

### Actions

- **Create Security Group** — Opens a form to create a new security group (requires `create securitygroups` RBAC permission)
- **Delete** — Removes the security group (requires `delete securitygroups` permission). The security group must not be in use by any VNIs.

### Detail View

**Path:** `/vpc-networking/security-groups/:name`

The detail page shows:

#### Security Group Info
- Name, ID, VPC, description, creation date

#### Rules Table

| Column | Description |
|--------|-------------|
| Direction | `inbound` or `outbound` |
| Protocol | `tcp`, `udp`, `icmp`, or `all` |
| Port Range | Min-max port (for TCP/UDP) |
| Remote | Source/destination CIDR or security group reference |

#### Rule Actions

- **Add Rule** — Opens a form to add a new inbound or outbound rule
- **Edit Rule** — Modify an existing rule's port range, protocol, or remote
- **Delete Rule** — Remove a rule

All rule modifications require appropriate RBAC permissions.

### Creating a Security Group

1. Click **Create Security Group**
2. Fill in:
   - **Name** — A descriptive name (e.g., `web-tier-sg`)
   - **VPC** — Select from the dropdown of available VPCs
   - **Description** — (Optional) Describe the purpose
3. Click **Create**
4. After creation, navigate to the detail page to add rules

### Common Rule Examples

| Purpose | Direction | Protocol | Port Range | Remote |
|---------|-----------|----------|------------|--------|
| Allow SSH | Inbound | TCP | 22-22 | `0.0.0.0/0` or specific CIDR |
| Allow HTTP | Inbound | TCP | 80-80 | `0.0.0.0/0` |
| Allow HTTPS | Inbound | TCP | 443-443 | `0.0.0.0/0` |
| Allow ICMP (ping) | Inbound | ICMP | — | `0.0.0.0/0` |
| Allow all internal | Inbound | All | — | `10.0.0.0/8` |
| Allow all outbound | Outbound | All | — | `0.0.0.0/0` |

---

## Network ACLs

**Path:** `/vpc-networking/network-acls`

### List View

| Column | Description |
|--------|-------------|
| Name | ACL name |
| ID | VPC ACL ID |
| VPC | Associated VPC |
| Rule Count | Number of rules |
| Created | Creation timestamp |

### Actions

- **Create Network ACL** — Create a new ACL (requires `create networkacls` permission)
- **Delete** — Remove the ACL (requires `delete networkacls` permission). The ACL must not be associated with any subnets.

### Detail View

**Path:** `/vpc-networking/network-acls/:name`

#### ACL Info
- Name, ID, VPC, associated subnets, creation date

#### Rules Table

| Column | Description |
|--------|-------------|
| Name | Rule name |
| Action | `allow` or `deny` |
| Direction | `inbound` or `outbound` |
| Protocol | `tcp`, `udp`, `icmp`, or `all` |
| Source | Source CIDR |
| Destination | Destination CIDR |
| Port Range | Min-max port (for TCP/UDP) |

#### Rule Actions

- **Add Rule** — Add a new ACL rule
- **Edit Rule** — Modify an existing rule
- **Delete Rule** — Remove a rule

### Key Differences from Security Groups

| Aspect | Security Group | Network ACL |
|--------|---------------|-------------|
| Scope | Per network interface (VNI) | Per subnet |
| Statefulness | Stateful | Stateless |
| Rule evaluation | All rules evaluated | Ordered evaluation |
| Default behavior | Deny all inbound | Allow all |

### Creating an ACL Rule

1. Navigate to the ACL detail page
2. Click **Add Rule**
3. Fill in:
   - **Name** — Descriptive rule name
   - **Action** — Allow or Deny
   - **Direction** — Inbound or Outbound
   - **Protocol** — TCP, UDP, ICMP, or All
   - **Source CIDR** — Source address range
   - **Destination CIDR** — Destination address range
   - **Port Range** — (Optional, for TCP/UDP) Min and max port
4. Click **Create**

---

## Authorization

Write operations require RBAC permissions. If a button is grayed out, you may not have the required permissions. Contact your cluster administrator to request access.

See [RBAC](../admin-guide/rbac.md) for configuring permissions.

---

## Next Steps

- [Topology](topology.md) — Visualize security group and ACL relationships
- [VPC Prerequisites](../admin-guide/vpc-prerequisites.md) — Creating SGs and ACLs via CLI
- [Managing Resources](managing-resources.md) — Subnets, VNIs, and more
