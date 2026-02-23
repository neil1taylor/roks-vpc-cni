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
import { useK8sVLANAttachments, useClusterInfo } from '../api/hooks';
import { formatRelativeTime } from '../utils/formatters';

/**
 * VLAN Attachments Page
 *
 * On ROKS clusters, VLAN attachments on bare metal servers are managed by the
 * ROKS platform. Shows a "Coming Soon" placeholder until the ROKS API is available.
 */
const VLANAttachmentsPage: React.FC = () => {
  const { clusterInfo, loading: clusterInfoLoading } = useClusterInfo();
  const vlanManagementEnabled = clusterInfo?.features?.vlanAttachmentManagement !== false;
  const roksAPIAvailable = clusterInfo?.features?.roksAPIAvailable === true;

  const { attachments, loading: attachmentsLoading } = useK8sVLANAttachments();
  const loading = clusterInfoLoading || attachmentsLoading;

  // ROKS cluster without ROKS API — show Coming Soon
  if (!clusterInfoLoading && !vlanManagementEnabled && !roksAPIAvailable) {
    return (
      <Page>
        <PageSection variant={PageSectionVariants.light}>
          <Title headingLevel="h1">VLAN Attachments</Title>
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
                  On ROKS-managed clusters, VLAN attachments on bare metal servers are
                  managed by the ROKS platform. Direct VLAN attachment management will be
                  available once the ROKS API integration is complete.
                  <br />
                  <br />
                  VLAN attachments are automatically configured when bare metal workers
                  join the cluster and can be viewed through the IBM Cloud console.
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
        <Title headingLevel="h1">VLAN Attachments</Title>
      </PageSection>

      <PageSection>
        <Card>
          <Toolbar>
            <ToolbarContent>
              <ToolbarItem>
                <Button variant={ButtonVariant.primary} icon={<PlusCircleIcon />}>
                  Create Attachment
                </Button>
              </ToolbarItem>
            </ToolbarContent>
          </Toolbar>
          <CardBody>
            {loading ? (
              <div style={{ textAlign: 'center', padding: '40px' }}><Spinner size="lg" /></div>
            ) : !attachments?.length ? (
              <div style={{ textAlign: 'center', padding: '40px' }}>No VLAN attachments found</div>
            ) : (
              <Table aria-label="VLAN Attachments table" borders>
                <Thead>
                  <Tr>
                    <Th>Name</Th>
                    <Th>VLAN ID</Th>
                    <Th>Status</Th>
                    <Th>Age</Th>
                  </Tr>
                </Thead>
                <Tbody>
                  {attachments.map((item) => (
                    <Tr key={item.id || item.name}>
                      <Td>{item.name || '-'}</Td>
                      <Td>{item.spec?.vlan ?? '-'}</Td>
                      <Td>{item.status?.synced ? 'Synced' : 'Pending'}</Td>
                      <Td>{formatRelativeTime(item.createdAt)}</Td>
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

export default VLANAttachmentsPage;
