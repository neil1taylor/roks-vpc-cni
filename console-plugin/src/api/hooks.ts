import { useEffect, useState } from 'react';
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
