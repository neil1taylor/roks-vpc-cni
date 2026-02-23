package annotations

const (
	// Prefix for all operator-managed annotations
	Prefix = "vpc.roks.ibm.com/"

	// ── CUDN Annotations (admin-provided) ──

	// Zone is the VPC zone for the subnet (e.g., "us-south-1")
	Zone = Prefix + "zone"

	// CIDR is the IPv4 CIDR block for the VPC subnet (e.g., "10.240.64.0/24")
	CIDR = Prefix + "cidr"

	// VPCID is the VPC ID in which to create the subnet
	VPCID = Prefix + "vpc-id"

	// VLANID is the VLAN ID for OVN LocalNet and bare metal VLAN attachments
	VLANID = Prefix + "vlan-id"

	// SecurityGroupIDs is a comma-separated list of pre-existing security group IDs
	SecurityGroupIDs = Prefix + "security-group-ids"

	// ACLID is the pre-existing network ACL ID for the VPC subnet
	ACLID = Prefix + "acl-id"

	// ── CUDN Annotations (operator-managed status) ──

	// SubnetID is the VPC subnet ID created by the operator
	SubnetID = Prefix + "subnet-id"

	// SubnetStatus is the current status of the VPC subnet ("active", "pending", "error")
	SubnetStatus = Prefix + "subnet-status"

	// VLANAttachments maps node names to VLAN attachment IDs ("node1:att-id-1,node2:att-id-2")
	VLANAttachments = Prefix + "vlan-attachments"

	// ── VM Annotations (admin-provided) ──

	// FIPRequested indicates the admin wants a floating IP for this VM ("true")
	FIPRequested = Prefix + "fip"

	// ── VM Annotations (operator-managed) ──

	// VNIID is the VPC Virtual Network Interface ID bound to this VM
	VNIID = Prefix + "vni-id"

	// MACAddress is the VPC-generated MAC address (also set in VM interface spec)
	MACAddress = Prefix + "mac-address"

	// ReservedIP is the private IP address reserved on the VPC subnet
	ReservedIP = Prefix + "reserved-ip"

	// ReservedIPID is the VPC reserved IP resource ID (for cleanup)
	ReservedIPID = Prefix + "reserved-ip-id"

	// FIPID is the floating IP resource ID (if requested)
	FIPID = Prefix + "fip-id"

	// FIPAddress is the public floating IP address (if requested)
	FIPAddress = Prefix + "fip-address"
)

// RequiredCUDNAnnotations lists all annotations that must be present on a CUDN
// for the operator to process it.
var RequiredCUDNAnnotations = []string{
	Zone,
	CIDR,
	VPCID,
	VLANID,
	SecurityGroupIDs,
	ACLID,
}
