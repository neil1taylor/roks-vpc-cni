import type { NetworkDefinition } from '../../src/api/types';

export const mockCUDNs: NetworkDefinition[] = [
  {
    name: 'localnet-cudn',
    kind: 'ClusterUserDefinedNetwork',
    topology: 'LocalNet',
    role: 'Secondary',
    subnet_id: 'subnet-001',
    subnet_status: 'active',
    vpc_id: 'vpc-001',
    zone: 'us-south-1',
    cidr: '10.240.0.0/24',
    vlan_id: '100',
  },
  {
    name: 'layer2-cudn',
    kind: 'ClusterUserDefinedNetwork',
    topology: 'Layer2',
    role: 'Secondary',
  },
];

export const mockUDNs: NetworkDefinition[] = [
  {
    name: 'layer2-udn',
    namespace: 'prod',
    kind: 'UserDefinedNetwork',
    topology: 'Layer2',
    role: 'Secondary',
  },
];
