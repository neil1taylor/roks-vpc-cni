import React from 'react';
import {
  PageSection,
  Card,
  CardBody,
  CardTitle,
  Spinner,
  Text,
  TextVariants,
  Label,
} from '@patternfly/react-core';
import { Table, Thead, Tbody, Tr, Th, Td } from '@patternfly/react-table';
import { Link } from 'react-router-dom-v5-compat';
import { useSubnets, useAddressPrefixes } from '../api/hooks';
import StatusBadge from '../components/StatusBadge';
import { formatRelativeTime } from '../utils/formatters';
import VPCNetworkingShell from '../components/VPCNetworkingShell';

/**
 * Subnets List Page
 * Displays list of VPC subnets with CRUD operations
 */
const SubnetsListPage: React.FC = () => {
  const { subnets, loading } = useSubnets();
  const { addressPrefixes, loading: prefixesLoading } = useAddressPrefixes();

  return (
    <VPCNetworkingShell>
      <PageSection>
        <Text component={TextVariants.p} style={{ marginBottom: '16px', color: 'var(--pf-v5-global--Color--200)' }}>
          VPC subnets provisioned by the operator for LocalNet networks. Each subnet maps to a CUDN in a specific availability zone.
        </Text>

        {/* Address Prefixes Card — always visible */}
        <Card style={{ marginBottom: '16px' }}>
          <CardTitle>VPC Address Prefixes ({addressPrefixes?.length || 0})</CardTitle>
          <CardBody>
            {prefixesLoading ? (
              <div style={{ textAlign: 'center', padding: '16px' }}><Spinner size="md" /></div>
            ) : !addressPrefixes?.length ? (
              <Text component={TextVariants.small} style={{ padding: '8px 0' }}>
                No address prefixes found. Subnets must be created within a VPC address prefix range.
              </Text>
            ) : (
              <Table aria-label="Address prefixes table" variant="compact" borders>
                <Thead>
                  <Tr>
                    <Th>Name</Th>
                    <Th>CIDR</Th>
                    <Th>Zone</Th>
                    <Th>Default</Th>
                  </Tr>
                </Thead>
                <Tbody>
                  {addressPrefixes.map((prefix) => (
                    <Tr key={prefix.name + prefix.cidr}>
                      <Td>{prefix.name || '-'}</Td>
                      <Td><code>{prefix.cidr}</code></Td>
                      <Td>{prefix.zone || '-'}</Td>
                      <Td>{prefix.isDefault ? <Label isCompact color="blue">Default</Label> : '-'}</Td>
                    </Tr>
                  ))}
                </Tbody>
              </Table>
            )}
          </CardBody>
        </Card>

        {/* Subnets Table */}
        <Card>
          <CardTitle>Subnets</CardTitle>
          <CardBody>
            {loading ? (
              <div style={{ textAlign: 'center', padding: '40px' }}><Spinner size="lg" /></div>
            ) : !subnets?.length ? (
              <div style={{ textAlign: 'center', padding: '40px' }}>No subnets found</div>
            ) : (
              <Table aria-label="Subnets table" borders>
                <Thead>
                  <Tr>
                    <Th>Name</Th>
                    <Th>VPC</Th>
                    <Th>Zone</Th>
                    <Th>CIDR</Th>
                    <Th>Status</Th>
                    <Th>Flow Logs</Th>
                    <Th>Age</Th>
                  </Tr>
                </Thead>
                <Tbody>
                  {subnets.map((subnet) => (
                    <Tr key={subnet.id || subnet.name}>
                      <Td><Link to={`/vpc-networking/subnets/${subnet.id}`}>{subnet.name || '-'}</Link></Td>
                      <Td>{subnet.vpc?.name || subnet.vpc?.id || '-'}</Td>
                      <Td>{subnet.zone?.name || '-'}</Td>
                      <Td>{subnet.ipv4CidrBlock || '-'}</Td>
                      <Td><StatusBadge status={subnet.status} /></Td>
                      <Td>
                        <Label color={subnet.flowLogActive ? 'green' : 'grey'} isCompact>
                          {subnet.flowLogActive ? 'Active' : '\u2014'}
                        </Label>
                      </Td>
                      <Td>{formatRelativeTime(subnet.createdAt)}</Td>
                    </Tr>
                  ))}
                </Tbody>
              </Table>
            )}
          </CardBody>
        </Card>
      </PageSection>
    </VPCNetworkingShell>
  );
};

SubnetsListPage.displayName = 'SubnetsListPage';

export default SubnetsListPage;
