import { useCallback, useEffect, useState } from 'react';
import { useK8sWatchResource } from '@openshift-console/dynamic-plugin-sdk';
import { apiClient } from './client';
import {
  VPC,
  Zone,
  SecurityGroup,
  NetworkACL,
  Subnet,
  VirtualNetworkInterface,
  FloatingIP,
  TopologyData,
  ClusterInfo,
  VPCSubnet,
  VirtualNetworkInterfaceResource,
  VLANAttachmentResource,
  FloatingIPResource,
  NetworkDefinition,
  NetworkTypesInfo,
  AddressPrefix,
  RoutingTable,
  Route,
  ReservedIP,
  PublicGateway,
  ApiResponse,
  ApiError,
} from './types';
import {
  VPCSubnetModel,
  VirtualNetworkInterfaceModel,
  VLANAttachmentModel,
  FloatingIPModel,
} from './k8s';

// Generic hook for BFF API calls
function useBFFData<T>(
  fetchFn: () => Promise<ApiResponse<T>>,
  dependencies: unknown[] = [],
): {
  data: T | null;
  loading: boolean;
  error: ApiError | null;
} {
  const [data, setData] = useState<T | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<ApiError | null>(null);

  useEffect(() => {
    let mounted = true;

    const fetchData = async () => {
      setLoading(true);
      const response = await fetchFn();
      if (mounted) {
        if (response.error) {
          setError(response.error);
          setData(null);
        } else {
          setData(response.data || null);
          setError(null);
        }
        setLoading(false);
      }
    };

    fetchData();

    return () => {
      mounted = false;
    };
  }, dependencies);

  return { data, loading, error };
}

// Cluster Info Hook
export function useClusterInfo(): {
  clusterInfo: ClusterInfo | null;
  loading: boolean;
  error: ApiError | null;
  isROKS: boolean;
} {
  const { data: clusterInfo, loading, error } = useBFFData(
    () => apiClient.getClusterInfo(),
    [],
  );
  return {
    clusterInfo,
    loading,
    error,
    isROKS: clusterInfo?.clusterMode === 'roks',
  };
}

// VPC Hooks
export function useVPCs(region?: string): {
  vpcs: VPC[] | null;
  loading: boolean;
  error: ApiError | null;
} {
  const { data: vpcs, loading, error } = useBFFData(
    () => apiClient.listVPCs(region),
    [region],
  );
  return { vpcs, loading, error };
}

export function useVPC(vpcId: string): {
  vpc: VPC | null;
  loading: boolean;
  error: ApiError | null;
} {
  const { data: vpc, loading, error } = useBFFData(
    () => apiClient.getVPC(vpcId),
    [vpcId],
  );
  return { vpc, loading, error };
}

// Zone Hooks
export function useZones(region?: string): {
  zones: Zone[] | null;
  loading: boolean;
  error: ApiError | null;
} {
  const { data: zones, loading, error } = useBFFData(
    () => apiClient.listZones(region),
    [region],
  );
  return { zones, loading, error };
}

export function useZone(zoneId: string): {
  zone: Zone | null;
  loading: boolean;
  error: ApiError | null;
} {
  const { data: zone, loading, error } = useBFFData(
    () => apiClient.getZone(zoneId),
    [zoneId],
  );
  return { zone, loading, error };
}

// Subnet Hooks
export function useSubnets(vpcId?: string): {
  subnets: Subnet[] | null;
  loading: boolean;
  error: ApiError | null;
} {
  const { data: subnets, loading, error } = useBFFData(
    () => apiClient.listSubnets(vpcId),
    [vpcId],
  );
  return { subnets, loading, error };
}

export function useSubnet(subnetId: string): {
  subnet: Subnet | null;
  loading: boolean;
  error: ApiError | null;
} {
  const { data: subnet, loading, error } = useBFFData(
    () => apiClient.getSubnet(subnetId),
    [subnetId],
  );
  return { subnet, loading, error };
}

// Reserved IP Hooks
export function useSubnetReservedIPs(subnetId: string): {
  reservedIPs: ReservedIP[] | null;
  loading: boolean;
  error: ApiError | null;
} {
  const { data: reservedIPs, loading, error } = useBFFData(
    () => apiClient.listSubnetReservedIPs(subnetId),
    [subnetId],
  );
  return { reservedIPs, loading, error };
}

// Virtual Network Interface Hooks
export function useVNIs(subnetId?: string): {
  vnis: VirtualNetworkInterface[] | null;
  loading: boolean;
  error: ApiError | null;
} {
  const { data: vnis, loading, error } = useBFFData(
    () => apiClient.listVNIs(subnetId),
    [subnetId],
  );
  return { vnis, loading, error };
}

export function useVNI(vniId: string): {
  vni: VirtualNetworkInterface | null;
  loading: boolean;
  error: ApiError | null;
} {
  const { data: vni, loading, error } = useBFFData(
    () => apiClient.getVNI(vniId),
    [vniId],
  );
  return { vni, loading, error };
}

// Floating IP Hooks
export function useFloatingIPs(vpcId?: string, zone?: string): {
  floatingIps: FloatingIP[] | null;
  loading: boolean;
  error: ApiError | null;
} {
  const { data: floatingIps, loading, error } = useBFFData(
    () => apiClient.listFloatingIPs(vpcId, zone),
    [vpcId, zone],
  );
  return { floatingIps, loading, error };
}

export function useFloatingIP(floatingIpId: string): {
  floatingIp: FloatingIP | null;
  loading: boolean;
  error: ApiError | null;
} {
  const { data: floatingIp, loading, error } = useBFFData(
    () => apiClient.getFloatingIP(floatingIpId),
    [floatingIpId],
  );
  return { floatingIp, loading, error };
}

// Public Gateway Hooks
export function usePublicGateways(vpcId?: string): {
  publicGateways: PublicGateway[] | null;
  loading: boolean;
  error: ApiError | null;
} {
  const { data: publicGateways, loading, error } = useBFFData(
    () => apiClient.listPublicGateways(vpcId),
    [vpcId],
  );
  return { publicGateways, loading, error };
}

// Security Group Hooks
export function useSecurityGroups(vpcId?: string): {
  securityGroups: SecurityGroup[] | null;
  loading: boolean;
  error: ApiError | null;
} {
  const { data: securityGroups, loading, error } = useBFFData(
    () => apiClient.listSecurityGroups(vpcId),
    [vpcId],
  );
  return { securityGroups, loading, error };
}

export function useSecurityGroup(sgId: string): {
  securityGroup: SecurityGroup | null;
  loading: boolean;
  error: ApiError | null;
} {
  const { data: securityGroup, loading, error } = useBFFData(
    () => apiClient.getSecurityGroup(sgId),
    [sgId],
  );
  return { securityGroup, loading, error };
}

// Network ACL Hooks
export function useNetworkACLs(vpcId?: string): {
  networkAcls: NetworkACL[] | null;
  loading: boolean;
  error: ApiError | null;
} {
  const { data: networkAcls, loading, error } = useBFFData(
    () => apiClient.listNetworkACLs(vpcId),
    [vpcId],
  );
  return { networkAcls, loading, error };
}

export function useNetworkACL(aclId: string): {
  networkAcl: NetworkACL | null;
  loading: boolean;
  error: ApiError | null;
} {
  const { data: networkAcl, loading, error } = useBFFData(
    () => apiClient.getNetworkACL(aclId),
    [aclId],
  );
  return { networkAcl, loading, error };
}

// Topology Hooks
export function useTopology(vpcId?: string): {
  topology: TopologyData | null;
  loading: boolean;
  error: ApiError | null;
} {
  const { data: topology, loading, error } = useBFFData(
    () => apiClient.getTopology(vpcId),
    [vpcId],
  );
  return { topology, loading, error };
}

// Network Definition Hooks
export function useNetworkDefinitions(paused?: boolean): {
  networks: NetworkDefinition[] | null;
  loading: boolean;
  error: ApiError | null;
  refetch: () => void;
} {
  const [networks, setNetworks] = useState<NetworkDefinition[] | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<ApiError | null>(null);
  const [fetchCount, setFetchCount] = useState(0);

  const refetch = useCallback(() => setFetchCount((c) => c + 1), []);

  useEffect(() => {
    if (paused) return;
    let mounted = true;

    const fetchData = async () => {
      if (fetchCount === 0) setLoading(true);
      const [cudnResp, udnResp] = await Promise.all([
        apiClient.listCUDNs(),
        apiClient.listUDNs(),
      ]);
      if (mounted) {
        const all: NetworkDefinition[] = [];
        if (cudnResp.data) all.push(...cudnResp.data);
        if (udnResp.data) all.push(...udnResp.data);
        setNetworks(all);
        setError(cudnResp.error || udnResp.error || null);
        setLoading(false);
      }
    };

    fetchData();
    const interval = setInterval(fetchData, 10000);
    return () => { mounted = false; clearInterval(interval); };
  }, [fetchCount, paused]);

  return { networks, loading, error, refetch };
}

// Network Types Hook (combinations, tiers, IP modes)
export function useNetworkTypes(): {
  networkTypes: NetworkTypesInfo | null;
  loading: boolean;
  error: ApiError | null;
} {
  const { data: networkTypes, loading, error } = useBFFData(
    () => apiClient.getNetworkTypes(),
    [],
  );
  return { networkTypes, loading, error };
}

// Single Network Definition Hook (fetches CUDN or UDN by name)
export function useNetworkDefinition(
  name: string,
  kind?: string,
  namespace?: string,
): {
  network: NetworkDefinition | null;
  loading: boolean;
  error: ApiError | null;
} {
  const { data: network, loading, error } = useBFFData(
    () => {
      if (kind === 'UserDefinedNetwork' && namespace) {
        return apiClient.getUDN(namespace, name);
      }
      return apiClient.getCUDN(name);
    },
    [name, kind, namespace],
  );
  return { network, loading, error };
}

// Routing Table Hooks
export function useRoutingTables(): {
  routingTables: RoutingTable[] | null;
  loading: boolean;
  error: ApiError | null;
} {
  const { data: routingTables, loading, error } = useBFFData(
    () => apiClient.listRoutingTables(),
    [],
  );
  return { routingTables, loading, error };
}

// Address Prefix Hooks
export function useAddressPrefixes(vpcId?: string): {
  addressPrefixes: AddressPrefix[] | null;
  loading: boolean;
  error: ApiError | null;
} {
  const { data: addressPrefixes, loading, error } = useBFFData(
    () => apiClient.listAddressPrefixes(vpcId),
    [vpcId],
  );
  return { addressPrefixes, loading, error };
}

export function useRoutes(routingTableId: string): {
  routes: Route[] | null;
  loading: boolean;
  error: ApiError | null;
} {
  const { data: routes, loading, error } = useBFFData(
    () => apiClient.listRoutes(routingTableId),
    [routingTableId],
  );
  return { routes, loading, error };
}

// Gateway Hooks
export function useGateways() {
  const { data: gateways, loading, error } = useBFFData(
    () => apiClient.listGateways(),
    [],
  );
  return { gateways, loading, error };
}

export function useGateway(name: string) {
  const { data: gateway, loading, error } = useBFFData(
    () => apiClient.getGateway(name),
    [name],
  );
  return { gateway, loading, error };
}

// Router Hooks
export function useRouters() {
  const { data: routers, loading, error } = useBFFData(
    () => apiClient.listRouters(),
    [],
  );
  return { routers, loading, error };
}

export function useRouter(name: string) {
  const { data: router, loading, error } = useBFFData(
    () => apiClient.getRouter(name),
    [name],
  );
  return { router, loading, error };
}

// Kubernetes CR Hooks

export function useK8sVPCSubnets(namespace?: string): {
  subnets: VPCSubnet[] | null;
  loading: boolean;
  error?: Error;
} {
  const [subnets, loaded, error] = useK8sWatchResource<VPCSubnet[]>({
    groupVersionKind: {
      group: VPCSubnetModel.apiGroup,
      version: 'v1alpha1',
      kind: VPCSubnetModel.kind,
    },
    namespace,
    isList: true,
  });

  return {
    subnets: subnets as VPCSubnet[] | null,
    loading: !loaded,
    error: error as Error | undefined,
  };
}

export function useK8sVNIs(namespace?: string): {
  vnis: VirtualNetworkInterfaceResource[] | null;
  loading: boolean;
  error?: Error;
} {
  const [vnis, loaded, error] = useK8sWatchResource<VirtualNetworkInterfaceResource[]>({
    groupVersionKind: {
      group: VirtualNetworkInterfaceModel.apiGroup,
      version: 'v1alpha1',
      kind: VirtualNetworkInterfaceModel.kind,
    },
    namespace,
    isList: true,
  });

  return {
    vnis: vnis as VirtualNetworkInterfaceResource[] | null,
    loading: !loaded,
    error: error as Error | undefined,
  };
}

export function useK8sVLANAttachments(namespace?: string): {
  attachments: VLANAttachmentResource[] | null;
  loading: boolean;
  error?: Error;
} {
  const [attachments, loaded, error] = useK8sWatchResource<VLANAttachmentResource[]>({
    groupVersionKind: {
      group: VLANAttachmentModel.apiGroup,
      version: 'v1alpha1',
      kind: VLANAttachmentModel.kind,
    },
    namespace,
    isList: true,
  });

  return {
    attachments: attachments as VLANAttachmentResource[] | null,
    loading: !loaded,
    error: error as Error | undefined,
  };
}

export function useK8sFloatingIPs(namespace?: string): {
  floatingIps: FloatingIPResource[] | null;
  loading: boolean;
  error?: Error;
} {
  const [floatingIps, loaded, error] = useK8sWatchResource<FloatingIPResource[]>({
    groupVersionKind: {
      group: FloatingIPModel.apiGroup,
      version: 'v1alpha1',
      kind: FloatingIPModel.kind,
    },
    namespace,
    isList: true,
  });

  return {
    floatingIps: floatingIps as FloatingIPResource[] | null,
    loading: !loaded,
    error: error as Error | undefined,
  };
}
