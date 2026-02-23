import type { VirtualNetworkInterface } from '../../src/api/types';

export const mockVNIs: VirtualNetworkInterface[] = [
  {
    id: 'vni-001',
    name: 'vm-web-01-vni',
    createdAt: '2025-01-20T08:00:00Z',
    allowIpSpoofing: true,
    enableInfrastructureNat: false,
    primaryIp: { address: '10.240.0.10', autoDelete: false, primary: true },
    status: 'available',
    type: 'primary',
  },
  {
    id: 'vni-002',
    name: 'vm-db-01-vni',
    createdAt: '2025-01-20T09:00:00Z',
    allowIpSpoofing: true,
    enableInfrastructureNat: false,
    primaryIp: { address: '10.240.0.20', autoDelete: false, primary: true },
    status: 'available',
    type: 'primary',
  },
];
