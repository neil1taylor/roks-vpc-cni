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
  EmptyState,
  EmptyStateIcon,
  EmptyStateBody,
  EmptyStateHeader,
  Spinner,
  DescriptionList,
  DescriptionListGroup,
  DescriptionListTerm,
  DescriptionListDescription,
} from '@patternfly/react-core';
import { CubesIcon } from '@patternfly/react-icons';
import { useVNI, useClusterInfo } from '../api/hooks';
import { StatusBadge } from '../components/StatusBadge';
import { formatTimestamp } from '../utils/formatters';

/**
 * Virtual Network Interface Detail Page
 *
 * On ROKS clusters, shows a "Coming Soon" placeholder until the ROKS API
 * is available for VNI management.
 */
const VNIDetailPage: React.FC = () => {
  const { name } = useParams<{ name: string }>();
  const { clusterInfo, loading: clusterInfoLoading } = useClusterInfo();
  const vniManagementEnabled = clusterInfo?.features?.vniManagement !== false;
  const roksAPIAvailable = clusterInfo?.features?.roksAPIAvailable === true;

  const { vni, loading: vniLoading } = useVNI(name || '');
  const loading = clusterInfoLoading || vniLoading;

  // ROKS cluster without ROKS API — show Coming Soon
  if (!clusterInfoLoading && !vniManagementEnabled && !roksAPIAvailable) {
    return (
      <Page>
        <PageSection variant={PageSectionVariants.light}>
          <Breadcrumb>
            <BreadcrumbItem href="/vpc-networking/vnis">VNIs</BreadcrumbItem>
            <BreadcrumbItem isActive>{name}</BreadcrumbItem>
          </Breadcrumb>
          <Title headingLevel="h1">VNI: {name}</Title>
        </PageSection>

        <PageSection>
          <Card>
            <CardBody>
              <EmptyState>
                <EmptyStateHeader
                  titleText="Coming Soon"
                  headingLevel="h4"
                  icon={<EmptyStateIcon icon={CubesIcon} />}
                />
                <EmptyStateBody>
                  VNI details are not available on ROKS-managed clusters through this
                  console. Direct VNI management will be available once the ROKS API
                  integration is complete.
                </EmptyStateBody>
              </EmptyState>
            </CardBody>
          </Card>
        </PageSection>
      </Page>
    );
  }

  return (
    <Page>
      <PageSection variant={PageSectionVariants.light}>
        <Breadcrumb>
          <BreadcrumbItem href="/vpc-networking/vnis">VNIs</BreadcrumbItem>
          <BreadcrumbItem isActive>{name}</BreadcrumbItem>
        </Breadcrumb>
        <Title headingLevel="h1">VNI: {name}</Title>
      </PageSection>

      <PageSection>
        {loading ? (
          <Spinner size="lg" />
        ) : vni ? (
          <Card>
            <CardBody>
              <DescriptionList>
                <DescriptionListGroup>
                  <DescriptionListTerm>ID</DescriptionListTerm>
                  <DescriptionListDescription>{vni.id}</DescriptionListDescription>
                </DescriptionListGroup>
                <DescriptionListGroup>
                  <DescriptionListTerm>Name</DescriptionListTerm>
                  <DescriptionListDescription>{vni.name || '-'}</DescriptionListDescription>
                </DescriptionListGroup>
                <DescriptionListGroup>
                  <DescriptionListTerm>Primary IP</DescriptionListTerm>
                  <DescriptionListDescription>
                    {vni.primaryIp?.address || 'Not assigned'}
                  </DescriptionListDescription>
                </DescriptionListGroup>
                <DescriptionListGroup>
                  <DescriptionListTerm>Allow IP Spoofing</DescriptionListTerm>
                  <DescriptionListDescription>
                    {vni.allowIpSpoofing ? 'Yes' : 'No'}
                  </DescriptionListDescription>
                </DescriptionListGroup>
                <DescriptionListGroup>
                  <DescriptionListTerm>Infrastructure NAT</DescriptionListTerm>
                  <DescriptionListDescription>
                    {vni.enableInfrastructureNat ? 'Yes' : 'No'}
                  </DescriptionListDescription>
                </DescriptionListGroup>
                <DescriptionListGroup>
                  <DescriptionListTerm>Subnet</DescriptionListTerm>
                  <DescriptionListDescription>
                    {vni.subnet?.name || vni.subnet?.id || 'None'}
                  </DescriptionListDescription>
                </DescriptionListGroup>
                <DescriptionListGroup>
                  <DescriptionListTerm>Status</DescriptionListTerm>
                  <DescriptionListDescription>
                    <StatusBadge status={vni.status} />
                  </DescriptionListDescription>
                </DescriptionListGroup>
                <DescriptionListGroup>
                  <DescriptionListTerm>Created</DescriptionListTerm>
                  <DescriptionListDescription>
                    {formatTimestamp(vni.createdAt)}
                  </DescriptionListDescription>
                </DescriptionListGroup>
              </DescriptionList>
            </CardBody>
          </Card>
        ) : (
          <Card>
            <CardBody>VNI not found</CardBody>
          </Card>
        )}
      </PageSection>
    </Page>
  );
};

export default VNIDetailPage;
