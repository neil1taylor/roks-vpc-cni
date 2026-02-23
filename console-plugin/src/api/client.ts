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
        const error = (await response.json()) as ApiError;
        return {
          error: error || {
            code: 'HTTP_ERROR',
            message: `HTTP ${response.status}: ${response.statusText}`,
          },
        };
      }

      const data = (await response.json()) as T;
      return { data };
    } catch (err) {
      const error = err as Error;
      return {
        error: {
          code: 'NETWORK_ERROR',
          message: error.message || 'Unknown network error',
        },
      };
    }
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

  async createFloatingIP(floatingIp: Partial<FloatingIP>): Promise<ApiResponse<FloatingIP>> {
    return this.request<FloatingIP>('POST', '/floating-ips', floatingIp as Record<string, unknown>);
  }

  async updateFloatingIP(floatingIpId: string, floatingIp: Partial<FloatingIP>): Promise<ApiResponse<FloatingIP>> {
    return this.request<FloatingIP>('PATCH', `/floating-ips/${floatingIpId}`, floatingIp as Record<string, unknown>);
  }

  async deleteFloatingIP(floatingIpId: string): Promise<ApiResponse<void>> {
    return this.request<void>('DELETE', `/floating-ips/${floatingIpId}`);
  }

  // Security Group Operations
  async listSecurityGroups(vpcId?: string): Promise<ApiResponse<SecurityGroup[]>> {
    const endpoint = vpcId ? `/security-groups?vpcId=${vpcId}` : '/security-groups';
    return this.request<SecurityGroup[]>('GET', endpoint);
  }

  async getSecurityGroup(sgId: string): Promise<ApiResponse<SecurityGroup>> {
    return this.request<SecurityGroup>('GET', `/security-groups/${sgId}`);
  }

  async createSecurityGroup(sg: Partial<SecurityGroup>): Promise<ApiResponse<SecurityGroup>> {
    return this.request<SecurityGroup>('POST', '/security-groups', sg as Record<string, unknown>);
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

  async createNetworkACL(acl: Partial<NetworkACL>): Promise<ApiResponse<NetworkACL>> {
    return this.request<NetworkACL>('POST', '/network-acls', acl as Record<string, unknown>);
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
  async getTopology(vpcId?: string): Promise<ApiResponse<TopologyData>> {
    const endpoint = vpcId ? `/topology?vpcId=${vpcId}` : '/topology';
    return this.request<TopologyData>('GET', endpoint);
  }
}

// Export singleton instance
export const apiClient = new VPCNetworkClient();
export default apiClient;
