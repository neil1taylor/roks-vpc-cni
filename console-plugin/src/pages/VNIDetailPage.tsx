import React from 'react';
import { useParams } from 'react-router-dom-v5-compat';
import {
  PageSection,
  PageSectionVariants,
  Card,
  CardBody,
  Breadcrumb,
  BreadcrumbItem,
  EmptyState,
  EmptyStateIcon,
  EmptyStateBody,
  EmptyStateHeader,
  Spinner,
  Alert,
  DescriptionList,
  DescriptionListGroup,
  DescriptionListTerm,
  DescriptionListDescription,
} from '@patternfly/react-core';
import { CubesIcon } from '@patternfly/react-icons';
import { Link } from 'react-router-dom-v5-compat';
import { useVNI, useClusterInfo } from '../api/hooks';
import { StatusBadge } from '../components/StatusBadge';
import { formatTimestamp } from '../utils/formatters';
import VPCNetworkingShell from '../components/VPCNetworkingShell';

/**
 * Virtual Network Interface Detail Page
 *
 * On ROKS clusters, shows a "Coming Soon" placeholder until the ROKS API
 * is available for VNI management.
 */
const VNIDetailPage: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const { clusterInfo, loading: clusterInfoLoading } = useClusterInfo();
  const vniManagementEnabled = clusterInfo?.features?.vniManagement !== false;
  const roksAPIAvailable = clusterInfo?.features?.roksAPIAvailable === true;

  const { vni, loading: vniLoading } = useVNI(id || '');
  const loading = clusterInfoLoading || vniLoading;

  // ROKS cluster without ROKS API — show Coming Soon
  if (!clusterInfoLoading && !vniManagementEnabled && !roksAPIAvailable) {
    return (
      <VPCNetworkingShell>
        <PageSection variant={PageSectionVariants.light}>
          <Breadcrumb>
            <BreadcrumbItem><Link to="/vpc-networking/vnis">VNIs</Link></BreadcrumbItem>
            <BreadcrumbItem isActive>{vni?.name || id}</BreadcrumbItem>
          </Breadcrumb>
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
      </VPCNetworkingShell>
    );
  }

  return (
    <VPCNetworkingShell>
      <PageSection variant={PageSectionVariants.light}>
        <Breadcrumb>
          <BreadcrumbItem><Link to="/vpc-networking/vnis">VNIs</Link></BreadcrumbItem>
          <BreadcrumbItem isActive>{vni?.name || id}</BreadcrumbItem>
        </Breadcrumb>
      </PageSection>

      <PageSection>
        {loading ? (
          <Spinner size="lg" />
        ) : vni ? (
          <>
          <Alert variant="info" isInline title="VNIs are created automatically" style={{ marginBottom: '16px' }}>
            When a VirtualMachine is created, the operator's mutating webhook provisions a VNI
            on the appropriate VPC subnet, assigns a MAC address and reserved IP, and injects
            them into the VM spec. VNIs are deleted when the VM is removed.
          </Alert>
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
          </>
        ) : (
          <Card>
            <CardBody>VNI not found</CardBody>
          </Card>
        )}
      </PageSection>
    </VPCNetworkingShell>
  );
};

export default VNIDetailPage;
