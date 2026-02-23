import type {
  VPCSubnet,
  VirtualNetworkInterfaceResource,
  VLANAttachmentResource,
  FloatingIPResource,
} from '../../src/api/types';

export const mockK8sVPCSubnets: VPCSubnet[] = [
  {
    id: 'k8s-vsn-001',
    name: 'prod-subnet-1',
    apiVersion: 'vpc.roks.ibm.com/v1alpha1',
    kind: 'VPCSubnet',
    spec: {
      vpcId: 'vpc-001',
      subnetId: 'subnet-001',
      zone: 'us-south-1',
      cidrBlock: '10.240.0.0/24',
    },
    status: {
      synced: true,
      lastSyncTime: '2025-01-20T12:00:00Z',
      availableIps: 248,
    },
  },
];

export const mockK8sVNIs: VirtualNetworkInterfaceResource[] = [
  {
    id: 'k8s-vni-001',
    name: 'vm-web-01-vni',
    apiVersion: 'vpc.roks.ibm.com/v1alpha1',
    kind: 'VirtualNetworkInterface',
    spec: {
      vpcId: 'vpc-001',
      vniId: 'vni-001',
      subnetId: 'subnet-001',
      isPrimary: true,
    },
    status: {
      synced: true,
      lastSyncTime: '2025-01-20T12:00:00Z',
      primaryIp: '10.240.0.10',
    },
  },
];

export const mockK8sVLANAttachments: VLANAttachmentResource[] = [
  {
    id: 'k8s-vla-001',
    name: 'node-1-vlan-100',
    apiVersion: 'vpc.roks.ibm.com/v1alpha1',
    kind: 'VLANAttachment',
    spec: {
      vniId: 'vni-001',
      vlan: 100,
      subnetId: 'subnet-001',
    },
    status: {
      synced: true,
      lastSyncTime: '2025-01-20T12:00:00Z',
    },
  },
];

export const mockK8sFloatingIPs: FloatingIPResource[] = [
  {
    id: 'k8s-fip-001',
    name: 'web-public-ip',
    apiVersion: 'vpc.roks.ibm.com/v1alpha1',
    kind: 'FloatingIP',
    spec: {
      vpcId: 'vpc-001',
      floatingIpId: 'fip-001',
      address: '169.48.100.1',
      zone: 'us-south-1',
    },
    status: {
      synced: true,
      lastSyncTime: '2025-01-20T12:00:00Z',
      targetId: 'vni-001',
      targetName: 'vm-web-01-vni',
    },
  },
];
