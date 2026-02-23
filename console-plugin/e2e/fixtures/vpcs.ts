import type { VPC } from '../../src/api/types';

export const mockVPCs: VPC[] = [
  {
    id: 'vpc-001',
    name: 'prod-vpc',
    crn: 'crn:v1:bluemix:public:is:us-south:a/abc123::vpc:vpc-001',
    href: '/vpcs/vpc-001',
    createdAt: '2025-01-15T10:00:00Z',
    displayName: 'Production VPC',
    isDefault: false,
    status: 'available',
    classicAccess: false,
    addressPrefixes: [
      { cidr: '10.240.0.0/18', isDefault: true, name: 'prefix-1', zone: 'us-south-1' },
      { cidr: '10.240.64.0/18', isDefault: true, name: 'prefix-2', zone: 'us-south-2' },
    ],
  },
  {
    id: 'vpc-002',
    name: 'dev-vpc',
    crn: 'crn:v1:bluemix:public:is:us-south:a/abc123::vpc:vpc-002',
    href: '/vpcs/vpc-002',
    createdAt: '2025-02-01T12:00:00Z',
    displayName: 'Development VPC',
    isDefault: true,
    status: 'available',
    classicAccess: false,
    addressPrefixes: [
      { cidr: '10.241.0.0/18', isDefault: true, name: 'prefix-1', zone: 'us-south-1' },
    ],
  },
];
