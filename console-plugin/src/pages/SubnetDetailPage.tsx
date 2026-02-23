import React from 'react';
import { useParams } from 'react-router-dom';
import {
  Page,
  PageSection,
  PageSectionVariants,
  Title,
  Card,
  CardBody,
  Breadcrumb,
  BreadcrumbItem,
  Spinner,
  DescriptionList,
  DescriptionListGroup,
  DescriptionListTerm,
  DescriptionListDescription,
} from '@patternfly/react-core';
import { useSubnet } from '../api/hooks';
import { StatusBadge } from '../components/StatusBadge';
import { formatTimestamp } from '../utils/formatters';

/**
 * Subnet Detail Page
 * Displays detailed information about a specific subnet
 */
const SubnetDetailPage: React.FC = () => {
  const { name } = useParams<{ name: string }>();
  const { subnet, loading } = useSubnet(name || '');

  return (
    <Page>
      <PageSection variant={PageSectionVariants.light}>
        <Breadcrumb>
          <BreadcrumbItem href="/vpc-networking/subnets">Subnets</BreadcrumbItem>
          <BreadcrumbItem isActive>{name}</BreadcrumbItem>
        </Breadcrumb>
        <Title headingLevel="h1">Subnet: {name}</Title>
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
      </PageSection>
    </Page>
  );
};

SubnetDetailPage.displayName = 'SubnetDetailPage';

export default SubnetDetailPage;
