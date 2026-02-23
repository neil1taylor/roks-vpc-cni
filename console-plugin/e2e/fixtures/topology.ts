import type { TopologyData } from '../../src/api/types';

export const mockTopology: TopologyData = {
  nodes: [
    { id: 'vpc-001', label: 'prod-vpc', type: 'vpc', status: 'available' },
    { id: 'subnet-001', label: 'prod-subnet-1', type: 'subnet', status: 'available' },
    { id: 'vni-001', label: 'vm-web-01-vni', type: 'vni', status: 'available' },
    { id: 'sg-001', label: 'web-sg', type: 'security-group', status: 'available' },
    { id: 'fip-001', label: 'web-public-ip', type: 'floating-ip', status: 'available' },
  ],
  edges: [
    { id: 'e-1', source: 'vpc-001', target: 'subnet-001', type: 'contains' },
    { id: 'e-2', source: 'subnet-001', target: 'vni-001', type: 'contains' },
    { id: 'e-3', source: 'sg-001', target: 'vni-001', type: 'protected-by' },
    { id: 'e-4', source: 'fip-001', target: 'vni-001', type: 'targets' },
  ],
};
