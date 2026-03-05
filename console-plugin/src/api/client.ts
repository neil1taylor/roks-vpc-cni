import { consoleFetch } from '@openshift-console/dynamic-plugin-sdk';
import {
  VPC,
  Zone,
  SecurityGroup,
  SecurityGroupRule,
  NetworkACL,
  NetworkACLRule,
  Subnet,
  VirtualNetworkInterface,
  FloatingIP,
  TopologyData,
  ClusterInfo,
  NetworkDefinition,
  CreateNetworkRequest,
  NetworkTypesInfo,
  AddressPrefix,
  RoutingTable,
  Route,
  CreateRouteRequest,
  CreateSecurityGroupRequest,
  CreateNetworkACLRequest,
  CreateFloatingIPRequest,
  UpdateFloatingIPRequest,
  ReservedIP,
  NamespaceInfo,
  CreateNamespaceRequest,
  PublicGateway,
  Gateway,
  CreateGatewayRequest,
  Router,
  CreateRouterRequest,
  PublicAddressRange,
  CreatePARRequest,
  RouterHealthSummary,
  InterfaceTimeSeries,
  ConntrackTimeSeries,
  DHCPPoolMetrics,
  NFTRuleMetrics,
  DHCPLease,
  DHCPReservation,
  RouterIDS,
  L2Bridge,
  CreateL2BridgeRequest,
  VPNGateway,
  CreateVPNGatewayRequest,
  DNSPolicy,
  CreateDNSPolicyRequest,
  Traceflow,
  CreateTraceflowRequest,
  FlowLogCollector,
  IssuedClient,
  AlertTimelineEntry,
  SubnetMetrics,
  ApiResponse,
  ApiError,
} from './types';

const BASE_URL = '/api/proxy/plugin/vpc-network-management/bff/api/v1';

/**
 * BFF API client for VPC networking resources
 */
class VPCNetworkClient {
  private baseUrl: string;

  constructor(baseUrl: string = BASE_URL) {
    this.baseUrl = baseUrl;
  }

  /**
   * Helper method to make API calls
   */
  private async request<T>(
    method: string,
    endpoint: string,
    body?: Record<string, unknown>,
  ): Promise<ApiResponse<T>> {
    try {
      const url = `${this.baseUrl}${endpoint}`;
      const options: RequestInit = {
        method,
        headers: {
          'Content-Type': 'application/json',
        },
      };

      if (body) {
        options.body = JSON.stringify(body);
      }

      const response = await consoleFetch(url, options);

      if (!response.ok) {
        try {
          const body = await response.json();
          // BFF wraps errors as { error: { code, message } }
          const apiError: ApiError = body.error || body;
          return {
            error: {
              code: apiError.code || 'HTTP_ERROR',
              message: apiError.message || `HTTP ${response.status}: ${response.statusText}`,
            },
          };
        } catch {
          return {
            error: {
              code: 'HTTP_ERROR',
              message: `HTTP ${response.status}: ${response.statusText}`,
            },
          };
        }
      }

      const data = (await response.json()) as T;
      return { data };
    } catch (err) {
      return {
        error: {
          code: 'NETWORK_ERROR',
          message: this.extractErrorMessage(err),
        },
      };
    }
  }

  /**
   * Safely extract an error message from any thrown value.
   * consoleFetch may throw Error, Response-like objects, HttpError with .json,
   * plain objects, or strings — this handles all shapes.
   */
  private extractErrorMessage(err: unknown): string {
    if (typeof err === 'string') {
      return err;
    }
    if (err && typeof err === 'object') {
      const obj = err as Record<string, unknown>;
      // HttpError with json body (OpenShift Console SDK throws this on non-2xx)
      if (obj.json && typeof obj.json === 'object') {
        const json = obj.json as Record<string, unknown>;
        if (json.error && typeof json.error === 'object') {
          const inner = json.error as Record<string, unknown>;
          if (typeof inner.message === 'string') return inner.message;
        }
        if (typeof json.message === 'string') return json.message;
      }
      // Plain object with message (Error instances, HttpError, etc.)
      if (typeof obj.message === 'string' && obj.message) return obj.message;
      // Response-like with status/statusText
      if (typeof obj.status === 'number' && typeof obj.statusText === 'string') {
        return `HTTP ${obj.status}: ${obj.statusText}`;
      }
      // HttpError with only a numeric status and response
      if (typeof obj.status === 'number') {
        return `HTTP error ${obj.status}`;
      }
      // Last resort: try JSON.stringify to avoid [object Object]
      try {
        const s = JSON.stringify(err);
        if (s && s !== '{}') return s;
      } catch {
        // ignore
      }
    }
    return 'An unexpected error occurred';
  }

  // VPC Operations
  async listVPCs(region?: string): Promise<ApiResponse<VPC[]>> {
    const endpoint = region ? `/vpcs?region=${region}` : '/vpcs';
    return this.request<VPC[]>('GET', endpoint);
  }

  async getVPC(vpcId: string): Promise<ApiResponse<VPC>> {
    return this.request<VPC>('GET', `/vpcs/${vpcId}`);
  }

  async createVPC(vpc: Partial<VPC>): Promise<ApiResponse<VPC>> {
    return this.request<VPC>('POST', '/vpcs', vpc as Record<string, unknown>);
  }

  async updateVPC(vpcId: string, vpc: Partial<VPC>): Promise<ApiResponse<VPC>> {
    return this.request<VPC>('PATCH', `/vpcs/${vpcId}`, vpc as Record<string, unknown>);
  }

  async deleteVPC(vpcId: string): Promise<ApiResponse<void>> {
    return this.request<void>('DELETE', `/vpcs/${vpcId}`);
  }

  // Zone Operations
  async listZones(region?: string): Promise<ApiResponse<Zone[]>> {
    const endpoint = region ? `/zones?region=${region}` : '/zones';
    return this.request<Zone[]>('GET', endpoint);
  }

  async getZone(zoneId: string): Promise<ApiResponse<Zone>> {
    return this.request<Zone>('GET', `/zones/${zoneId}`);
  }

  // Subnet Operations
  async listSubnets(vpcId?: string): Promise<ApiResponse<Subnet[]>> {
    const endpoint = vpcId ? `/subnets?vpcId=${vpcId}` : '/subnets';
    return this.request<Subnet[]>('GET', endpoint);
  }

  async getSubnet(subnetId: string): Promise<ApiResponse<Subnet>> {
    return this.request<Subnet>('GET', `/subnets/${subnetId}`);
  }

  async createSubnet(subnet: Partial<Subnet>): Promise<ApiResponse<Subnet>> {
    return this.request<Subnet>('POST', '/subnets', subnet as Record<string, unknown>);
  }

  async updateSubnet(subnetId: string, subnet: Partial<Subnet>): Promise<ApiResponse<Subnet>> {
    return this.request<Subnet>('PATCH', `/subnets/${subnetId}`, subnet as Record<string, unknown>);
  }

  async deleteSubnet(subnetId: string): Promise<ApiResponse<void>> {
    return this.request<void>('DELETE', `/subnets/${subnetId}`);
  }

  async listSubnetReservedIPs(subnetId: string): Promise<ApiResponse<ReservedIP[]>> {
    return this.request<ReservedIP[]>('GET', `/subnets/${subnetId}/reserved-ips`);
  }

  async getSubnetMetrics(
    name: string,
    namespace?: string,
    range?: string,
  ): Promise<ApiResponse<SubnetMetrics>> {
    const params = new URLSearchParams();
    if (namespace) params.set('namespace', namespace);
    if (range) params.set('range', range);
    const qs = params.toString();
    return this.request<SubnetMetrics>('GET', `/subnets/${name}/metrics${qs ? `?${qs}` : ''}`);
  }

  // Virtual Network Interface Operations
  async listVNIs(subnetId?: string): Promise<ApiResponse<VirtualNetworkInterface[]>> {
    const endpoint = subnetId ? `/vnis?subnetId=${subnetId}` : '/vnis';
    return this.request<VirtualNetworkInterface[]>('GET', endpoint);
  }

  async getVNI(vniId: string): Promise<ApiResponse<VirtualNetworkInterface>> {
    return this.request<VirtualNetworkInterface>('GET', `/vnis/${vniId}`);
  }

  async createVNI(vni: Partial<VirtualNetworkInterface>): Promise<ApiResponse<VirtualNetworkInterface>> {
    return this.request<VirtualNetworkInterface>('POST', '/vnis', vni as Record<string, unknown>);
  }

  async updateVNI(vniId: string, vni: Partial<VirtualNetworkInterface>): Promise<ApiResponse<VirtualNetworkInterface>> {
    return this.request<VirtualNetworkInterface>('PATCH', `/vnis/${vniId}`, vni as Record<string, unknown>);
  }

  async deleteVNI(vniId: string): Promise<ApiResponse<void>> {
    return this.request<void>('DELETE', `/vnis/${vniId}`);
  }

  // Floating IP Operations
  async listFloatingIPs(vpcId?: string, zone?: string): Promise<ApiResponse<FloatingIP[]>> {
    let endpoint = '/floating-ips';
    const params: string[] = [];
    if (vpcId) params.push(`vpcId=${vpcId}`);
    if (zone) params.push(`zone=${zone}`);
    if (params.length > 0) endpoint += `?${params.join('&')}`;
    return this.request<FloatingIP[]>('GET', endpoint);
  }

  async getFloatingIP(floatingIpId: string): Promise<ApiResponse<FloatingIP>> {
    return this.request<FloatingIP>('GET', `/floating-ips/${floatingIpId}`);
  }

  async createFloatingIP(floatingIp: CreateFloatingIPRequest): Promise<ApiResponse<FloatingIP>> {
    return this.request<FloatingIP>('POST', '/floating-ips', floatingIp as unknown as Record<string, unknown>);
  }

  async updateFloatingIP(floatingIpId: string, req: UpdateFloatingIPRequest): Promise<ApiResponse<FloatingIP>> {
    return this.request<FloatingIP>('PATCH', `/floating-ips/${floatingIpId}`, req as unknown as Record<string, unknown>);
  }

  async deleteFloatingIP(floatingIpId: string): Promise<ApiResponse<void>> {
    return this.request<void>('DELETE', `/floating-ips/${floatingIpId}`);
  }

  // Public Gateway Operations
  async listPublicGateways(vpcId?: string): Promise<ApiResponse<PublicGateway[]>> {
    const endpoint = vpcId ? `/public-gateways?vpcId=${vpcId}` : '/public-gateways';
    return this.request<PublicGateway[]>('GET', endpoint);
  }

  // Security Group Operations
  async listSecurityGroups(vpcId?: string): Promise<ApiResponse<SecurityGroup[]>> {
    const endpoint = vpcId ? `/security-groups?vpcId=${vpcId}` : '/security-groups';
    return this.request<SecurityGroup[]>('GET', endpoint);
  }

  async getSecurityGroup(sgId: string): Promise<ApiResponse<SecurityGroup>> {
    return this.request<SecurityGroup>('GET', `/security-groups/${sgId}`);
  }

  async createSecurityGroup(sg: CreateSecurityGroupRequest): Promise<ApiResponse<SecurityGroup>> {
    return this.request<SecurityGroup>('POST', '/security-groups', sg as unknown as Record<string, unknown>);
  }

  async updateSecurityGroup(sgId: string, sg: Partial<SecurityGroup>): Promise<ApiResponse<SecurityGroup>> {
    return this.request<SecurityGroup>('PATCH', `/security-groups/${sgId}`, sg as Record<string, unknown>);
  }

  async deleteSecurityGroup(sgId: string): Promise<ApiResponse<void>> {
    return this.request<void>('DELETE', `/security-groups/${sgId}`);
  }

  async addSecurityGroupRule(sgId: string, rule: SecurityGroupRule): Promise<ApiResponse<SecurityGroupRule>> {
    return this.request<SecurityGroupRule>('POST', `/security-groups/${sgId}/rules`, rule as unknown as Record<string, unknown>);
  }

  async updateSecurityGroupRule(
    sgId: string,
    ruleId: string,
    rule: Partial<SecurityGroupRule>,
  ): Promise<ApiResponse<SecurityGroupRule>> {
    return this.request<SecurityGroupRule>('PATCH', `/security-groups/${sgId}/rules/${ruleId}`, rule as Record<string, unknown>);
  }

  async deleteSecurityGroupRule(sgId: string, ruleId: string): Promise<ApiResponse<void>> {
    return this.request<void>('DELETE', `/security-groups/${sgId}/rules/${ruleId}`);
  }

  // Network ACL Operations
  async listNetworkACLs(vpcId?: string): Promise<ApiResponse<NetworkACL[]>> {
    const endpoint = vpcId ? `/network-acls?vpcId=${vpcId}` : '/network-acls';
    return this.request<NetworkACL[]>('GET', endpoint);
  }

  async getNetworkACL(aclId: string): Promise<ApiResponse<NetworkACL>> {
    return this.request<NetworkACL>('GET', `/network-acls/${aclId}`);
  }

  async createNetworkACL(acl: CreateNetworkACLRequest): Promise<ApiResponse<NetworkACL>> {
    return this.request<NetworkACL>('POST', '/network-acls', acl as unknown as Record<string, unknown>);
  }

  async updateNetworkACL(aclId: string, acl: Partial<NetworkACL>): Promise<ApiResponse<NetworkACL>> {
    return this.request<NetworkACL>('PATCH', `/network-acls/${aclId}`, acl as Record<string, unknown>);
  }

  async deleteNetworkACL(aclId: string): Promise<ApiResponse<void>> {
    return this.request<void>('DELETE', `/network-acls/${aclId}`);
  }

  async addNetworkACLRule(aclId: string, rule: NetworkACLRule): Promise<ApiResponse<NetworkACLRule>> {
    return this.request<NetworkACLRule>('POST', `/network-acls/${aclId}/rules`, rule as unknown as Record<string, unknown>);
  }

  async updateNetworkACLRule(
    aclId: string,
    ruleId: string,
    rule: Partial<NetworkACLRule>,
  ): Promise<ApiResponse<NetworkACLRule>> {
    return this.request<NetworkACLRule>('PATCH', `/network-acls/${aclId}/rules/${ruleId}`, rule as Record<string, unknown>);
  }

  async deleteNetworkACLRule(aclId: string, ruleId: string): Promise<ApiResponse<void>> {
    return this.request<void>('DELETE', `/network-acls/${aclId}/rules/${ruleId}`);
  }

  // Cluster Info
  async getClusterInfo(): Promise<ApiResponse<ClusterInfo>> {
    return this.request<ClusterInfo>('GET', '/cluster-info');
  }

  // Topology Operations
  async getTopology(vpcId?: string, includeHealth?: boolean): Promise<ApiResponse<TopologyData>> {
    const params = new URLSearchParams();
    if (vpcId) params.set('vpcId', vpcId);
    if (includeHealth) params.set('includeHealth', 'true');
    const qs = params.toString();
    return this.request<TopologyData>('GET', `/topology${qs ? `?${qs}` : ''}`);
  }

  // Network (CUDN/UDN) Operations
  async listCUDNs(): Promise<ApiResponse<NetworkDefinition[]>> {
    return this.request<NetworkDefinition[]>('GET', '/cudns');
  }

  async getCUDN(name: string): Promise<ApiResponse<NetworkDefinition>> {
    return this.request<NetworkDefinition>('GET', `/cudns/${name}`);
  }

  async createCUDN(network: CreateNetworkRequest): Promise<ApiResponse<NetworkDefinition>> {
    return this.request<NetworkDefinition>('POST', '/cudns', network as unknown as Record<string, unknown>);
  }

  async deleteCUDN(name: string): Promise<ApiResponse<void>> {
    return this.request<void>('DELETE', `/cudns/${name}`);
  }

  async getUDN(namespace: string, name: string): Promise<ApiResponse<NetworkDefinition>> {
    return this.request<NetworkDefinition>('GET', `/udns/${namespace}/${name}`);
  }

  async listUDNs(namespace?: string): Promise<ApiResponse<NetworkDefinition[]>> {
    const endpoint = namespace ? `/udns?namespace=${namespace}` : '/udns';
    return this.request<NetworkDefinition[]>('GET', endpoint);
  }

  async createUDN(network: CreateNetworkRequest): Promise<ApiResponse<NetworkDefinition>> {
    return this.request<NetworkDefinition>('POST', '/udns', network as unknown as Record<string, unknown>);
  }

  async deleteUDN(namespace: string, name: string): Promise<ApiResponse<void>> {
    return this.request<void>('DELETE', `/udns/${namespace}/${name}`);
  }

  async getNetworkTypes(): Promise<ApiResponse<NetworkTypesInfo>> {
    return this.request<NetworkTypesInfo>('GET', '/network-types');
  }

  // Namespace Operations
  async listNamespaces(label?: string): Promise<ApiResponse<NamespaceInfo[]>> {
    const endpoint = label ? `/namespaces?label=${encodeURIComponent(label)}` : '/namespaces';
    return this.request<NamespaceInfo[]>('GET', endpoint);
  }

  async createNamespace(req: CreateNamespaceRequest): Promise<ApiResponse<NamespaceInfo>> {
    return this.request<NamespaceInfo>('POST', '/namespaces', req as unknown as Record<string, unknown>);
  }

  // Address Prefix Operations
  async listAddressPrefixes(vpcId?: string): Promise<ApiResponse<AddressPrefix[]>> {
    const endpoint = vpcId ? `/address-prefixes?vpc_id=${vpcId}` : '/address-prefixes';
    return this.request<AddressPrefix[]>('GET', endpoint);
  }

  async createAddressPrefix(opts: {
    vpcId?: string;
    cidr: string;
    zone: string;
    name?: string;
  }): Promise<ApiResponse<AddressPrefix>> {
    return this.request<AddressPrefix>('POST', '/address-prefixes', opts as unknown as Record<string, unknown>);
  }

  // Routing Table and Route Operations
  async listRoutingTables(): Promise<ApiResponse<RoutingTable[]>> {
    return this.request<RoutingTable[]>('GET', '/routing-tables');
  }

  async getRoutingTable(rtId: string): Promise<ApiResponse<RoutingTable>> {
    return this.request<RoutingTable>('GET', `/routing-tables/${rtId}`);
  }

  async listRoutes(rtId: string): Promise<ApiResponse<Route[]>> {
    return this.request<Route[]>('GET', `/routing-tables/${rtId}/routes`);
  }

  async createRoute(rtId: string, route: CreateRouteRequest): Promise<ApiResponse<Route>> {
    return this.request<Route>('POST', `/routing-tables/${rtId}/routes`, route as unknown as Record<string, unknown>);
  }

  async deleteRoute(rtId: string, routeId: string): Promise<ApiResponse<void>> {
    return this.request<void>('DELETE', `/routing-tables/${rtId}/routes/${routeId}`);
  }

  // Gateway Operations
  async listGateways(): Promise<ApiResponse<Gateway[]>> {
    return this.request<Gateway[]>('GET', '/gateways');
  }

  async getGateway(name: string, namespace?: string): Promise<ApiResponse<Gateway>> {
    const params = namespace ? `?namespace=${encodeURIComponent(namespace)}` : '';
    return this.request<Gateway>('GET', `/gateways/${name}${params}`);
  }

  async createGateway(req: CreateGatewayRequest): Promise<ApiResponse<Gateway>> {
    return this.request<Gateway>('POST', '/gateways', req as unknown as Record<string, unknown>);
  }

  async deleteGateway(name: string, namespace?: string): Promise<ApiResponse<void>> {
    const params = namespace ? `?namespace=${encodeURIComponent(namespace)}` : '';
    return this.request<void>('DELETE', `/gateways/${name}${params}`);
  }

  // Router Operations
  async listRouters(): Promise<ApiResponse<Router[]>> {
    return this.request<Router[]>('GET', '/routers');
  }

  async getRouter(name: string, namespace?: string): Promise<ApiResponse<Router>> {
    const params = namespace ? `?namespace=${encodeURIComponent(namespace)}` : '';
    return this.request<Router>('GET', `/routers/${name}${params}`);
  }

  async createRouter(req: CreateRouterRequest): Promise<ApiResponse<Router>> {
    return this.request<Router>('POST', '/routers', req as unknown as Record<string, unknown>);
  }

  async deleteRouter(name: string, namespace?: string): Promise<ApiResponse<void>> {
    const params = namespace ? `?namespace=${encodeURIComponent(namespace)}` : '';
    return this.request<void>('DELETE', `/routers/${name}${params}`);
  }

  async getRouterLeases(name: string, namespace?: string): Promise<ApiResponse<DHCPLease[]>> {
    const params = namespace ? `?namespace=${encodeURIComponent(namespace)}` : '';
    return this.request<DHCPLease[]>('GET', `/routers/${name}/leases${params}`);
  }

  async updateRouterReservations(
    name: string,
    namespace: string,
    network: string,
    reservations: DHCPReservation[],
  ): Promise<ApiResponse<Router>> {
    const params = `?namespace=${encodeURIComponent(namespace)}`;
    return this.request<Router>('PATCH', `/routers/${name}/reservations${params}`, {
      network,
      reservations,
    } as unknown as Record<string, unknown>);
  }

  async updateRouterIDS(
    name: string,
    namespace: string,
    ids: Omit<RouterIDS, 'image' | 'nfqueueNum'>,
  ): Promise<ApiResponse<Router>> {
    const params = `?namespace=${encodeURIComponent(namespace)}`;
    return this.request<Router>('PATCH', `/routers/${name}/ids${params}`, ids as unknown as Record<string, unknown>);
  }

  // PAR (Public Address Range) Operations
  async listPARs(): Promise<ApiResponse<PublicAddressRange[]>> {
    return this.request<PublicAddressRange[]>('GET', '/pars');
  }

  async getPAR(id: string): Promise<ApiResponse<PublicAddressRange>> {
    return this.request<PublicAddressRange>('GET', `/pars/${id}`);
  }

  async createPAR(req: CreatePARRequest): Promise<ApiResponse<PublicAddressRange>> {
    return this.request<PublicAddressRange>('POST', '/pars', req as unknown as Record<string, unknown>);
  }

  async deletePAR(id: string): Promise<ApiResponse<void>> {
    return this.request<void>('DELETE', `/pars/${id}`);
  }

  // Router Metrics Operations
  async getRouterHealthSummary(
    name: string,
    namespace?: string,
  ): Promise<ApiResponse<RouterHealthSummary>> {
    const params = namespace ? `?namespace=${encodeURIComponent(namespace)}` : '';
    return this.request<RouterHealthSummary>('GET', `/routers/${name}/metrics/summary${params}`);
  }

  async getRouterInterfaceMetrics(
    name: string,
    namespace?: string,
    range?: string,
    step?: string,
  ): Promise<ApiResponse<InterfaceTimeSeries[]>> {
    const params = new URLSearchParams();
    if (namespace) params.set('namespace', namespace);
    if (range) params.set('range', range);
    if (step) params.set('step', step);
    const qs = params.toString();
    return this.request<InterfaceTimeSeries[]>('GET', `/routers/${name}/metrics/interfaces${qs ? `?${qs}` : ''}`);
  }

  async getRouterConntrackMetrics(
    name: string,
    namespace?: string,
    range?: string,
    step?: string,
  ): Promise<ApiResponse<ConntrackTimeSeries>> {
    const params = new URLSearchParams();
    if (namespace) params.set('namespace', namespace);
    if (range) params.set('range', range);
    if (step) params.set('step', step);
    const qs = params.toString();
    return this.request<ConntrackTimeSeries>('GET', `/routers/${name}/metrics/conntrack${qs ? `?${qs}` : ''}`);
  }

  async getRouterDHCPMetrics(
    name: string,
    namespace?: string,
  ): Promise<ApiResponse<DHCPPoolMetrics[]>> {
    const params = namespace ? `?namespace=${encodeURIComponent(namespace)}` : '';
    return this.request<DHCPPoolMetrics[]>('GET', `/routers/${name}/metrics/dhcp${params}`);
  }

  async getRouterNFTMetrics(
    name: string,
    namespace?: string,
  ): Promise<ApiResponse<NFTRuleMetrics[]>> {
    const params = namespace ? `?namespace=${encodeURIComponent(namespace)}` : '';
    return this.request<NFTRuleMetrics[]>('GET', `/routers/${name}/metrics/nft${params}`);
  }

  // L2 Bridge Operations
  async listL2Bridges(): Promise<ApiResponse<L2Bridge[]>> {
    return this.request<L2Bridge[]>('GET', '/l2bridges');
  }

  async getL2Bridge(name: string, namespace?: string): Promise<ApiResponse<L2Bridge>> {
    const params = namespace ? `?ns=${encodeURIComponent(namespace)}` : '';
    return this.request<L2Bridge>('GET', `/l2bridges/${encodeURIComponent(name)}${params}`);
  }

  async createL2Bridge(req: CreateL2BridgeRequest): Promise<ApiResponse<L2Bridge>> {
    return this.request<L2Bridge>('POST', '/l2bridges', req as unknown as Record<string, unknown>);
  }

  async deleteL2Bridge(name: string, namespace?: string): Promise<ApiResponse<void>> {
    const params = namespace ? `?ns=${encodeURIComponent(namespace)}` : '';
    return this.request<void>('DELETE', `/l2bridges/${encodeURIComponent(name)}${params}`);
  }

  // VPN Gateway Operations
  async listVPNGateways(): Promise<ApiResponse<VPNGateway[]>> {
    return this.request<VPNGateway[]>('GET', '/vpn-gateways');
  }

  async getVPNGateway(name: string, namespace?: string): Promise<ApiResponse<VPNGateway>> {
    const params = namespace ? `?namespace=${encodeURIComponent(namespace)}` : '';
    return this.request<VPNGateway>('GET', `/vpn-gateways/${encodeURIComponent(name)}${params}`);
  }

  async createVPNGateway(req: CreateVPNGatewayRequest): Promise<ApiResponse<VPNGateway>> {
    return this.request<VPNGateway>('POST', '/vpn-gateways', req as unknown as Record<string, unknown>);
  }

  async deleteVPNGateway(name: string, namespace?: string): Promise<ApiResponse<void>> {
    const params = namespace ? `?namespace=${encodeURIComponent(namespace)}` : '';
    return this.request<void>('DELETE', `/vpn-gateways/${encodeURIComponent(name)}${params}`);
  }

  async generateClientConfig(
    name: string,
    clientName: string,
    namespace?: string,
  ): Promise<ApiResponse<{ clientName: string; secretName: string; ovpnConfig: string }>> {
    const params = namespace ? `?namespace=${encodeURIComponent(namespace)}` : '';
    return this.request('POST', `/vpn-gateways/${encodeURIComponent(name)}/client-config${params}`, {
      clientName,
    } as unknown as Record<string, unknown>);
  }

  async listVPNClients(name: string, namespace?: string): Promise<ApiResponse<IssuedClient[]>> {
    const params = namespace ? `?namespace=${encodeURIComponent(namespace)}` : '';
    return this.request<IssuedClient[]>('GET', `/vpn-gateways/${encodeURIComponent(name)}/clients${params}`);
  }

  async revokeVPNClient(
    name: string,
    clientName: string,
    namespace?: string,
  ): Promise<ApiResponse<void>> {
    const params = namespace ? `?namespace=${encodeURIComponent(namespace)}` : '';
    return this.request<void>(
      'DELETE',
      `/vpn-gateways/${encodeURIComponent(name)}/clients/${encodeURIComponent(clientName)}${params}`,
    );
  }

  // Alert Timeline Operations
  async getAlertTimeline(range?: string): Promise<ApiResponse<AlertTimelineEntry[]>> {
    const params = range ? `?range=${range}` : '';
    return this.request<AlertTimelineEntry[]>('GET', `/alerts/timeline${params}`);
  }

  // DNS Policy Operations
  async listDNSPolicies(): Promise<ApiResponse<DNSPolicy[]>> {
    return this.request<DNSPolicy[]>('GET', '/dns-policies');
  }

  async getDNSPolicy(name: string, namespace?: string): Promise<ApiResponse<DNSPolicy>> {
    const params = namespace ? `?namespace=${encodeURIComponent(namespace)}` : '';
    return this.request<DNSPolicy>('GET', `/dns-policies/${encodeURIComponent(name)}${params}`);
  }

  async createDNSPolicy(req: CreateDNSPolicyRequest): Promise<ApiResponse<DNSPolicy>> {
    return this.request<DNSPolicy>('POST', '/dns-policies', req as unknown as Record<string, unknown>);
  }

  async deleteDNSPolicy(name: string, namespace?: string): Promise<ApiResponse<void>> {
    const params = namespace ? `?namespace=${encodeURIComponent(namespace)}` : '';
    return this.request<void>('DELETE', `/dns-policies/${encodeURIComponent(name)}${params}`);
  }

  // Flow Log Operations
  async listFlowLogs(): Promise<ApiResponse<FlowLogCollector[]>> {
    return this.request<FlowLogCollector[]>('GET', '/flow-logs');
  }

  async getFlowLog(id: string): Promise<ApiResponse<FlowLogCollector>> {
    return this.request<FlowLogCollector>('GET', `/flow-logs/${id}`);
  }

  async deleteFlowLog(id: string): Promise<ApiResponse<void>> {
    return this.request<void>('DELETE', `/flow-logs/${id}`);
  }

  // Traceflow Operations
  async listTraceflows(): Promise<ApiResponse<Traceflow[]>> {
    return this.request<Traceflow[]>('GET', '/traceflows');
  }

  async getTraceflow(name: string, namespace?: string): Promise<ApiResponse<Traceflow>> {
    const params = namespace ? `?namespace=${encodeURIComponent(namespace)}` : '';
    return this.request<Traceflow>('GET', `/traceflows/${encodeURIComponent(name)}${params}`);
  }

  async createTraceflow(req: CreateTraceflowRequest): Promise<ApiResponse<Traceflow>> {
    return this.request<Traceflow>('POST', '/traceflows', req as unknown as Record<string, unknown>);
  }

  async deleteTraceflow(name: string, namespace?: string): Promise<ApiResponse<void>> {
    const params = namespace ? `?namespace=${encodeURIComponent(namespace)}` : '';
    return this.request<void>('DELETE', `/traceflows/${encodeURIComponent(name)}${params}`);
  }
}

// Export singleton instance
export const apiClient = new VPCNetworkClient();
export default apiClient;
