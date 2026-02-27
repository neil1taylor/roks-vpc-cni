import React from 'react';
import { useParams } from 'react-router-dom-v5-compat';
import {
  PageSection,
  PageSectionVariants,
  Card,
  CardBody,
  CardTitle,
  Breadcrumb,
  BreadcrumbItem,
  Spinner,
  Button,
  DescriptionList,
  DescriptionListGroup,
  DescriptionListTerm,
  DescriptionListDescription,
  Label,
} from '@patternfly/react-core';
import { Table, Thead, Tbody, Tr, Th, Td } from '@patternfly/react-table';
import { Link } from 'react-router-dom-v5-compat';
import { useSubnet, useSubnetReservedIPs } from '../api/hooks';
import { StatusBadge } from '../components/StatusBadge';
import { formatTimestamp, formatRelativeTime } from '../utils/formatters';
import VPCNetworkingShell from '../components/VPCNetworkingShell';

/**
 * Subnet Detail Page
 * Displays detailed information about a specific subnet
 */
const SubnetDetailPage: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const { subnet, loading } = useSubnet(id || '');
  const { reservedIPs, loading: ripsLoading } = useSubnetReservedIPs(id || '');

  return (
    <VPCNetworkingShell>
      <PageSection variant={PageSectionVariants.light}>
        <Breadcrumb>
          <BreadcrumbItem><Link to="/vpc-networking/subnets">Subnets</Link></BreadcrumbItem>
          <BreadcrumbItem isActive>{subnet?.name || id}</BreadcrumbItem>
        </Breadcrumb>
      </PageSection>

      <PageSection>
        {loading ? (
          <Spinner size="lg" />
        ) : subnet ? (
          <Card>
            <CardBody>
              <DescriptionList>
                <DescriptionListGroup>
                  <DescriptionListTerm>ID</DescriptionListTerm>
                  <DescriptionListDescription>{subnet.id}</DescriptionListDescription>
                </DescriptionListGroup>
                <DescriptionListGroup>
                  <DescriptionListTerm>Name</DescriptionListTerm>
                  <DescriptionListDescription>{subnet.name || '-'}</DescriptionListDescription>
                </DescriptionListGroup>
                <DescriptionListGroup>
                  <DescriptionListTerm>CIDR Block</DescriptionListTerm>
                  <DescriptionListDescription>{subnet.ipv4CidrBlock}</DescriptionListDescription>
                </DescriptionListGroup>
                <DescriptionListGroup>
                  <DescriptionListTerm>Available IPs</DescriptionListTerm>
                  <DescriptionListDescription>
                    {subnet.availableIpv4AddressCount} / {subnet.totalIpv4AddressCount}
                  </DescriptionListDescription>
                </DescriptionListGroup>
                <DescriptionListGroup>
                  <DescriptionListTerm>Zone</DescriptionListTerm>
                  <DescriptionListDescription>
                    {subnet.zone.name || subnet.zone.id}
                  </DescriptionListDescription>
                </DescriptionListGroup>
                <DescriptionListGroup>
                  <DescriptionListTerm>VPC</DescriptionListTerm>
                  <DescriptionListDescription>
                    {subnet.vpc.name || subnet.vpc.id}
                  </DescriptionListDescription>
                </DescriptionListGroup>
                <DescriptionListGroup>
                  <DescriptionListTerm>Network ACL</DescriptionListTerm>
                  <DescriptionListDescription>
                    {subnet.networkAcl?.name || subnet.networkAcl?.id || 'None'}
                  </DescriptionListDescription>
                </DescriptionListGroup>
                <DescriptionListGroup>
                  <DescriptionListTerm>Status</DescriptionListTerm>
                  <DescriptionListDescription>
                    <StatusBadge status={subnet.status} />
                  </DescriptionListDescription>
                </DescriptionListGroup>
                <DescriptionListGroup>
                  <DescriptionListTerm>Created</DescriptionListTerm>
                  <DescriptionListDescription>
                    {formatTimestamp(subnet.createdAt)}
                  </DescriptionListDescription>
                </DescriptionListGroup>
              </DescriptionList>
            </CardBody>
          </Card>
        ) : (
          <Card>
            <CardBody>Subnet not found</CardBody>
          </Card>
        )}

        {subnet && (
          <Card style={{ marginTop: '16px' }}>
            <CardTitle>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <span>Reserved IPs</span>
                <Link to={`/vpc-networking/subnets/${id}/reserved-ips`}>
                  <Button variant="link" size="sm">View all</Button>
                </Link>
              </div>
            </CardTitle>
            <CardBody>
              {ripsLoading ? (
                <div style={{ textAlign: 'center', padding: '20px' }}><Spinner size="md" /></div>
              ) : !reservedIPs?.length ? (
                <div style={{ padding: '16px', textAlign: 'center' }}>No reserved IPs</div>
              ) : (
                <Table aria-label="Reserved IPs" borders>
                  <Thead>
                    <Tr>
                      <Th>Name</Th>
                      <Th>Address</Th>
                      <Th>Owner</Th>
                      <Th>Target</Th>
                      <Th>Auto Delete</Th>
                      <Th>Age</Th>
                    </Tr>
                  </Thead>
                  <Tbody>
                    {reservedIPs.slice(0, 10).map((ip) => (
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
        )}
      </PageSection>
    </VPCNetworkingShell>
  );
};

SubnetDetailPage.displayName = 'SubnetDetailPage';

export default SubnetDetailPage;
