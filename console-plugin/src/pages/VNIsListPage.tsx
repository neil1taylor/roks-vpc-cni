import React from 'react';
import {
  Page,
  PageSection,
  PageSectionVariants,
  Title,
  Card,
  CardBody,
  Button,
  ButtonVariant,
  Toolbar,
  ToolbarContent,
  ToolbarItem,
  EmptyState,
  EmptyStateIcon,
  EmptyStateBody,
  EmptyStateHeader,
  Spinner,
} from '@patternfly/react-core';
import { PlusCircleIcon, CubesIcon } from '@patternfly/react-icons';
import { Table, Thead, Tbody, Tr, Th, Td } from '@patternfly/react-table';
import { useVNIs, useClusterInfo } from '../api/hooks';
import StatusBadge from '../components/StatusBadge';
import { formatRelativeTime } from '../utils/formatters';

/**
 * Virtual Network Interfaces List Page
 *
 * On ROKS clusters, VNIs are managed by the ROKS platform and cannot be
 * accessed directly via the VPC API. Shows a "Coming Soon" placeholder
 * until the ROKS API is available.
 */
const VNIsListPage: React.FC = () => {
  const { clusterInfo, loading: clusterInfoLoading } = useClusterInfo();
  const vniManagementEnabled = clusterInfo?.features?.vniManagement !== false;
  const roksAPIAvailable = clusterInfo?.features?.roksAPIAvailable === true;

  // Only fetch VNIs if management is enabled (unmanaged cluster) or ROKS API is available
  const shouldFetchVNIs = vniManagementEnabled || roksAPIAvailable;
  const { vnis, loading: vnisLoading } = useVNIs();
  const loading = clusterInfoLoading || (shouldFetchVNIs && vnisLoading);

  // ROKS cluster without ROKS API — show Coming Soon
  if (!clusterInfoLoading && !vniManagementEnabled && !roksAPIAvailable) {
    return (
      <Page>
        <PageSection variant={PageSectionVariants.light}>
          <Title headingLevel="h1">Virtual Network Interfaces</Title>
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
                  On ROKS-managed clusters, Virtual Network Interfaces are managed by the
                  ROKS platform. Direct VNI management will be available once the ROKS API
                  integration is complete.
                  <br />
                  <br />
                  In the meantime, VNIs are automatically provisioned when VirtualMachines
                  are created and can be viewed through the IBM Cloud console.
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
        <Title headingLevel="h1">Virtual Network Interfaces</Title>
      </PageSection>

      <PageSection>
        <Card>
          <Toolbar>
            <ToolbarContent>
              <ToolbarItem>
                <Button variant={ButtonVariant.primary} icon={<PlusCircleIcon />}>
                  Create VNI
                </Button>
              </ToolbarItem>
            </ToolbarContent>
          </Toolbar>
          <CardBody>
            {loading ? (
              <div style={{ textAlign: 'center', padding: '40px' }}><Spinner size="lg" /></div>
            ) : !vnis?.length ? (
              <div style={{ textAlign: 'center', padding: '40px' }}>No VNIs found</div>
            ) : (
              <Table aria-label="Virtual Network Interfaces table" borders>
                <Thead>
                  <Tr>
                    <Th>Name</Th>
                    <Th>Subnet</Th>
                    <Th>Status</Th>
                    <Th>Age</Th>
                  </Tr>
                </Thead>
                <Tbody>
                  {vnis.map((vni) => (
                    <Tr key={vni.id || vni.name}>
                      <Td>{vni.name || '-'}</Td>
                      <Td>{vni.subnet?.name || '-'}</Td>
                      <Td><StatusBadge status={vni.status} /></Td>
                      <Td>{formatRelativeTime(vni.createdAt)}</Td>
                    </Tr>
                  ))}
                </Tbody>
              </Table>
            )}
          </CardBody>
        </Card>
      </PageSection>
    </Page>
  );
};

export default VNIsListPage;
