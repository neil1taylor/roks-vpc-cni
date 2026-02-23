import type { SecurityGroup } from '../../src/api/types';

export const mockSecurityGroups: SecurityGroup[] = [
  {
    id: 'sg-001',
    name: 'web-sg',
    createdAt: '2025-01-15T11:00:00Z',
    displayName: 'Web Security Group',
    description: 'Security group for web tier',
    vpc: { id: 'vpc-001', name: 'prod-vpc' },
    status: 'available',
    rules: [
      {
        id: 'rule-001',
        direction: 'inbound',
        protocol: 'tcp',
        portMin: 443,
        portMax: 443,
        remote: '0.0.0.0/0',
        remoteType: 'cidr',
        createdAt: '2025-01-15T11:00:00Z',
      },
      {
        id: 'rule-002',
        direction: 'outbound',
        protocol: 'all',
        remote: '0.0.0.0/0',
        remoteType: 'cidr',
        createdAt: '2025-01-15T11:00:00Z',
      },
    ],
    targets: [],
  },
  {
    id: 'sg-002',
    name: 'db-sg',
    createdAt: '2025-01-15T11:30:00Z',
    displayName: 'Database Security Group',
    description: 'Security group for database tier',
    vpc: { id: 'vpc-001', name: 'prod-vpc' },
    status: 'available',
    rules: [
      {
        id: 'rule-003',
        direction: 'inbound',
        protocol: 'tcp',
        portMin: 5432,
        portMax: 5432,
        remote: 'sg-001',
        remoteType: 'security_group',
        createdAt: '2025-01-15T11:30:00Z',
      },
    ],
    targets: [],
  },
];
