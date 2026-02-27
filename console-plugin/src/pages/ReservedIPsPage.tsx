import React from 'react';
import { useParams } from 'react-router-dom-v5-compat';
import {
  PageSection,
  PageSectionVariants,
  Card,
  CardBody,
  Breadcrumb,
  BreadcrumbItem,
  Spinner,
  Alert,
  Label,
} from '@patternfly/react-core';
import { Table, Thead, Tbody, Tr, Th, Td } from '@patternfly/react-table';
import { Link } from 'react-router-dom-v5-compat';
import { useSubnetReservedIPs, useSubnet } from '../api/hooks';
import { formatRelativeTime } from '../utils/formatters';
import VPCNetworkingShell from '../components/VPCNetworkingShell';

const ReservedIPsPage: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const { subnet } = useSubnet(id || '');
  const { reservedIPs, loading, error } = useSubnetReservedIPs(id || '');

  return (
    <VPCNetworkingShell>
      <PageSection variant={PageSectionVariants.light}>
        <Breadcrumb>
          <BreadcrumbItem><Link to="/vpc-networking/subnets">Subnets</Link></BreadcrumbItem>
          <BreadcrumbItem><Link to={`/vpc-networking/subnets/${id}`}>{subnet?.name || id}</Link></BreadcrumbItem>
          <BreadcrumbItem isActive>Reserved IPs</BreadcrumbItem>
        </Breadcrumb>
      </PageSection>

      <PageSection>
        <Alert variant="info" isInline title="Reserved IPs are managed automatically" style={{ marginBottom: '16px' }}>
          When the operator creates a Virtual Network Interface (VNI) for a VM, the VPC API
          automatically assigns a reserved IP from the subnet's address pool. Reserved IPs
          are released when the VNI is deleted.
        </Alert>

        <Card>
          <CardBody>
            {loading ? (
              <div style={{ textAlign: 'center', padding: '40px' }}><Spinner size="lg" /></div>
            ) : error ? (
              <Alert variant="danger" isInline title="Failed to load reserved IPs">
                {error.message}
              </Alert>
            ) : !reservedIPs?.length ? (
              <div style={{ textAlign: 'center', padding: '40px' }}>No reserved IPs found for this subnet</div>
            ) : (
              <Table aria-label="Reserved IPs table" borders>
                <Thead>
                  <Tr>
                    <Th>Name</Th>
                    <Th>Address</Th>
                    <Th>Owner</Th>
                    <Th>Target (VNI)</Th>
                    <Th>Auto Delete</Th>
                    <Th>Age</Th>
                  </Tr>
                </Thead>
                <Tbody>
                  {reservedIPs.map((ip) => (
                    <Tr key={ip.id}>
                      <Td>{ip.name || '-'}</Td>
                      <Td><code>{ip.address}</code></Td>
                      <Td>{ip.owner || '-'}</Td>
                      <Td>
                        {ip.target ? (
                          <Link to={`/vpc-networking/vnis/${ip.target.id}`}>
                            {ip.target.name || ip.target.id}
                          </Link>
                        ) : '-'}
                      </Td>
                      <Td>
                        <Label color={ip.autoDelete ? 'green' : 'grey'} isCompact>
                          {ip.autoDelete ? 'Yes' : 'No'}
                        </Label>
                      </Td>
                      <Td>{formatRelativeTime(ip.createdAt)}</Td>
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

export default ReservedIPsPage;
