// IBM VPC and Networking Resource Types

// Common base types
export interface ResourceMetadata {
  id: string;
  name?: string;
  crn?: string;
  href?: string;
  createdAt?: string;
  createdBy?: string;
  modified?: string;
  modifiedBy?: string;
  region?: string;
  tags?: string[];
}

// VPC Types
export interface VPC extends ResourceMetadata {
  displayName?: string;
  isDefault?: boolean;
  status?: 'available' | 'pending' | 'deleting' | 'failed';
  addressPrefixes?: AddressPrefix[];
  classicAccess?: boolean;
}

export interface AddressPrefix {
  cidr: string;
  isDefault: boolean;
  name: string;
  zone: string;
}

export interface Zone extends ResourceMetadata {
  displayName?: string;
  status?: 'available' | 'unavailable';
  zone?: string;
  region?: string;
}

// Subnet Types
export interface Subnet extends ResourceMetadata {
  displayName?: string;
  ipv4CidrBlock: string;
  availableIpv4AddressCount: number;
  totalIpv4AddressCount: number;
  status?: 'available' | 'pending' | 'deleting' | 'failed';
  vpc: {
    id: string;
    name?: string;
  };
  zone: {
    id: string;
    name?: string;
  };
  networkAcl?: {
    id: string;
    name?: string;
  };
  publicGateway?: {
    id: string;
    name?: string;
  };
}

// Virtual Network Interface Types
export interface VirtualNetworkInterface extends ResourceMetadata {
  allowIpSpoofing: boolean;
  enableInfrastructureNat: boolean;
  subnet?: Subnet;
  primaryIp?: NetworkInterfaceIp;
  secondaryIps?: NetworkInterfaceIp[];
  securityGroups?: SecurityGroup[];
  status?: 'available' | 'pending' | 'deleting' | 'failed';
  type?: 'primary' | 'secondary';
}

export interface NetworkInterfaceIp {
  address: string;
  autoDelete: boolean;
  href?: string;
  name?: string;
  primary: boolean;
  reservationHref?: string;
}

// VLAN Attachment Types
export interface VLANAttachment extends ResourceMetadata {
  vlan: number;
  virtualNetworkInterface?: VirtualNetworkInterface;
  subnet?: Subnet;
  status?: 'available' | 'pending' | 'deleting' | 'failed';
}

// Floating IP Types
export interface FloatingIP extends ResourceMetadata {
  address: string;
  status?: 'available' | 'reserved' | 'associating' | 'associated' | 'disassociating' | 'unassociated' | 'failed';
  zone: Zone;
  target?: {
    id: string;
    href?: string;
    name?: string;
    resourceType?: string;
  };
  vpc?: VPC;
}

// Security Group Types
export interface SecurityGroup extends ResourceMetadata {
  displayName?: string;
  description?: string;
  vpc: {
    id: string;
    name?: string;
  };
  rules?: SecurityGroupRule[];
  status?: 'available' | 'pending' | 'deleting' | 'failed';
  targets?: SecurityGroupTarget[];
}

export interface SecurityGroupRule {
  id?: string;
  direction: 'inbound' | 'outbound';
  protocol: 'tcp' | 'udp' | 'icmp' | 'all';
  portMin?: number;
  portMax?: number;
  icmpType?: number;
  icmpCode?: number;
  remote?: string;
  remoteType?: string;
  createdAt?: string;
}

export interface SecurityGroupTarget {
  id: string;
  name?: string;
  resourceType: 'instance_interface' | 'network_interface' | 'load_balancer' | 'endpoint_gateway';
}

// Network ACL Types
export interface NetworkACL extends ResourceMetadata {
  displayName?: string;
  vpc: {
    id: string;
    name?: string;
  };
  rules?: NetworkACLRule[];
  subnets?: Subnet[];
  status?: 'available' | 'pending' | 'deleting' | 'failed';
}

export interface NetworkACLRule {
  id?: string;
  name?: string;
  action: 'allow' | 'deny';
  direction: 'inbound' | 'outbound';
  ipVersion: 'ipv4' | 'ipv6';
  protocol: 'tcp' | 'udp' | 'icmp' | 'all';
  source?: string;
  destination?: string;
  portMin?: number;
  portMax?: number;
  icmpType?: number;
  icmpCode?: number;
  priority?: number;
  createdAt?: string;
}

// Kubernetes CR Types (matching the Go structs)

export interface VPCSubnet extends ResourceMetadata {
  apiVersion?: string;
  kind?: string;
  spec?: {
    vpcId: string;
    subnetId: string;
    zone: string;
    cidrBlock: string;
  };
  status?: {
    synced: boolean;
    lastSyncTime?: string;
    syncError?: string;
    availableIps: number;
  };
}

export interface VirtualNetworkInterfaceResource extends ResourceMetadata {
  apiVersion?: string;
  kind?: string;
  spec?: {
    vpcId: string;
    vniId: string;
    subnetId: string;
    securityGroupIds?: string[];
    isPrimary: boolean;
  };
  status?: {
    synced: boolean;
    lastSyncTime?: string;
    syncError?: string;
    primaryIp?: string;
    secondaryIps?: string[];
  };
}

export interface VLANAttachmentResource extends ResourceMetadata {
  apiVersion?: string;
  kind?: string;
  spec?: {
    vniId: string;
    vlan: number;
    subnetId: string;
  };
  status?: {
    synced: boolean;
    lastSyncTime?: string;
    syncError?: string;
  };
}

export interface FloatingIPResource extends ResourceMetadata {
  apiVersion?: string;
  kind?: string;
  spec?: {
    vpcId: string;
    floatingIpId: string;
    address: string;
    zone: string;
  };
  status?: {
    synced: boolean;
    lastSyncTime?: string;
    syncError?: string;
    targetId?: string;
    targetName?: string;
  };
}

// Topology Types
export interface TopologyData {
  nodes: TopologyNode[];
  edges: TopologyEdge[];
}

export interface TopologyNode {
  id: string;
  label: string;
  type: 'vpc' | 'subnet' | 'instance' | 'vni' | 'security-group' | 'network-acl' | 'floating-ip';
  status?: 'available' | 'pending' | 'error';
  metadata?: Record<string, unknown>;
}

export interface TopologyEdge {
  id: string;
  source: string;
  target: string;
  type?: 'contains' | 'connected' | 'protected-by' | 'associates' | 'targets';
}

// API Response Wrappers
export interface ApiResponse<T> {
  data?: T;
  error?: ApiError;
  metadata?: {
    totalCount?: number;
    offset?: number;
    limit?: number;
  };
}

export interface ApiError {
  code: string;
  message: string;
  details?: Record<string, unknown>;
}

// Cluster Info Types
export interface ClusterInfo {
  clusterMode: 'roks' | 'unmanaged';
  features: FeatureFlags;
}

export interface FeatureFlags {
  vniManagement: boolean;
  vlanAttachmentManagement: boolean;
  subnetManagement: boolean;
  securityGroupManagement: boolean;
  networkACLManagement: boolean;
  floatingIPManagement: boolean;
  roksAPIAvailable: boolean;
}

// Permissions
export interface UserPermissions {
  isAdmin: boolean;
  canWrite: boolean;
  canDelete: boolean;
  resources?: string[];
}
