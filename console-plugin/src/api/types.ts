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

// Public Gateway Types
export interface PublicGateway {
  id: string;
  name: string;
  status: string;
  zone: Zone;
  floatingIp?: { address: string };
  createdAt?: string;
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

// Reserved IP Types
export interface ReservedIP {
  id: string;
  name: string;
  address: string;
  autoDelete: boolean;
  owner: string;
  target?: { id: string; name: string };
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
  metadata?: {
    name?: string;
    namespace?: string;
    creationTimestamp?: string;
    labels?: Record<string, string>;
  };
  spec?: {
    bmServerID: string;
    vlanID: number;
    subnetRef: string;
    subnetID?: string;
    allowToFloat?: boolean;
    nodeName?: string;
  };
  status?: {
    syncStatus?: string;
    attachmentID?: string;
    attachmentStatus?: string;
    lastSyncTime?: string;
    message?: string;
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

// Network Tier and IP Assignment Mode Types
export type IPAssignmentMode = 'static_reserved' | 'dhcp' | 'none';
export type NetworkTier = 'recommended' | 'advanced' | 'expert';

export interface NetworkCombination {
  id: string;
  topology: 'LocalNet' | 'Layer2';
  scope: 'ClusterUserDefinedNetwork' | 'UserDefinedNetwork';
  role: 'Primary' | 'Secondary';
  tier: NetworkTier;
  ip_mode: IPAssignmentMode;
  label: string;
  description: string;
  ip_mode_description: string;
  requires_vpc: boolean;
}

// Network Definition Types (CUDN/UDN)
export interface NetworkDefinition {
  name: string;
  namespace?: string;
  kind: 'ClusterUserDefinedNetwork' | 'UserDefinedNetwork';
  topology: 'LocalNet' | 'Layer2';
  role?: 'Primary' | 'Secondary';
  tier?: NetworkTier;
  ip_mode?: IPAssignmentMode;
  subnet_id?: string;
  subnet_name?: string;
  subnet_status?: string;
  vpc_id?: string;
  zone?: string;
  cidr?: string;
  vlan_id?: string;
  vlan_attachments?: string;
}

export interface CreateNetworkRequest {
  name: string;
  namespace?: string;
  topology: 'LocalNet' | 'Layer2';
  role?: 'Primary' | 'Secondary';
  vpc_id?: string;
  zone?: string;
  cidr?: string;
  vlan_id?: string;
  security_group_ids?: string;
  acl_id?: string;
  public_gateway_id?: string;
  target_namespaces?: string[];
}

export interface NetworkTypesInfo {
  topologies: string[];
  scopes: string[];
  roles: string[];
  combinations: NetworkCombination[];
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

// Routing Table and Route Types
export interface RoutingTable {
  id: string;
  name: string;
  isDefault: boolean;
  lifecycleState: string;
  routeCount: number;
  createdAt?: string;
}

export interface Route {
  id: string;
  name: string;
  destination: string;
  action: 'delegate' | 'delegate_vpc' | 'deliver' | 'drop';
  nextHop?: string;
  zone: string;
  priority: number;
  origin: 'service' | 'user';
  lifecycleState: string;
  createdAt?: string;
}

export interface CreateRouteRequest {
  name?: string;
  destination: string;
  action: string;
  nextHopIp?: string;
  zone: string;
  priority?: number;
}

// Request types for BFF create endpoints (flat payloads, not nested response shapes)
export interface CreateSecurityGroupRequest {
  name: string;
  vpc_id: string;
  description?: string;
}

export interface CreateNetworkACLRequest {
  name: string;
  vpc_id: string;
}

export interface CreateFloatingIPRequest {
  name: string;
  zone: string;
}

export interface UpdateFloatingIPRequest {
  target_id: string;
}

// Namespace Types
export interface NamespaceInfo {
  name: string;
  hasPrimaryLabel: boolean;
}

export interface CreateNamespaceRequest {
  name: string;
  labels?: Record<string, string>;
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
  vpcId?: string;
  features: FeatureFlags;
}

export interface FeatureFlags {
  vniManagement: boolean;
  vlanAttachmentManagement: boolean;
  subnetManagement: boolean;
  securityGroupManagement: boolean;
  networkACLManagement: boolean;
  floatingIPManagement: boolean;
  routeManagement: boolean;
  parManagement: boolean;
  roksAPIAvailable: boolean;
  cudnManagement: boolean;
  udnManagement: boolean;
  layer2Support: boolean;
  multiNetworkVMs: boolean;
}

// Permissions
export interface UserPermissions {
  isAdmin: boolean;
  canWrite: boolean;
  canDelete: boolean;
  resources?: string[];
}

// ── VPCGateway ──

export interface Gateway {
  name: string;
  namespace: string;
  zone: string;
  phase: string;
  uplinkNetwork: string;
  transitNetwork: string;
  vniID?: string;
  reservedIP?: string;
  floatingIP?: string;
  vpcRouteCount: number;
  natRuleCount: number;
  syncStatus: string;
  createdAt?: string;
  // PAR fields
  parEnabled: boolean;
  parPrefixLength?: number;
  publicAddressRangeID?: string;
  publicAddressRangeCIDR?: string;
  ingressRoutingTableID?: string;
}

export interface CreateGatewayRequest {
  name: string;
  namespace?: string;
  zone: string;
  uplinkNetwork: string;
  transitAddress: string;
  transitCIDR?: string;
  transitNetwork?: string;
  // PAR fields
  parEnabled?: boolean;
  parPrefixLength?: number;
  parID?: string;
}

// ── VPCRouter ──

export interface Router {
  name: string;
  namespace: string;
  gateway: string;
  phase: string;
  transitIP?: string;
  networks: RouterNetwork[];
  advertisedRoutes?: string[];
  functions?: string[];
  syncStatus: string;
  createdAt?: string;
}

export interface RouterNetwork {
  name: string;
  address: string;
  connected: boolean;
}

export interface CreateRouterRequest {
  name: string;
  namespace?: string;
  gateway: string;
  networks?: { name: string; address: string }[];
}

// ── Public Address Range (PAR) ──

export interface PublicAddressRange {
  id: string;
  name: string;
  cidr: string;
  zone: string;
  lifecycleState: string;
  createdAt?: string;
  gatewayName?: string;
  gatewayNamespace?: string;
}

export interface CreatePARRequest {
  name: string;
  zone: string;
  prefixLength: number;
}
