import type { FloatingIP } from '../../src/api/types';

export const mockFloatingIPs: FloatingIP[] = [
  {
    id: 'fip-001',
    name: 'web-public-ip',
    createdAt: '2025-01-20T10:00:00Z',
    address: '169.48.100.1',
    status: 'available',
    zone: { id: 'us-south-1', name: 'us-south-1' },
    target: { id: 'vni-001', name: 'vm-web-01-vni', resourceType: 'virtual_network_interface' },
  },
  {
    id: 'fip-002',
    name: 'api-public-ip',
    createdAt: '2025-01-22T14:00:00Z',
    address: '169.48.100.2',
    status: 'available',
    zone: { id: 'us-south-2', name: 'us-south-2' },
  },
];
