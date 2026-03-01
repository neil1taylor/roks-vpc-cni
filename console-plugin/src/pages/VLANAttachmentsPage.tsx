import React from 'react';
import {
  PageSection,
  Card,
  CardBody,
  EmptyState,
  EmptyStateIcon,
  EmptyStateBody,
  EmptyStateHeader,
  Spinner,
  Text,
  TextVariants,
} from '@patternfly/react-core';
import { CubesIcon } from '@patternfly/react-icons';
import { Table, Thead, Tbody, Tr, Th, Td } from '@patternfly/react-table';
import { useK8sVLANAttachments, useClusterInfo } from '../api/hooks';
import { VLANAttachmentResource } from '../api/types';
import { formatRelativeTime } from '../utils/formatters';
import VPCNetworkingShell from '../components/VPCNetworkingShell';

/**
 * VLAN Attachments Page
 *
 * On ROKS clusters, VLAN attachments on bare metal servers are managed by the
 * ROKS platform. Shows a "Coming Soon" placeholder until the ROKS API is available.
 */
const VLANAttachmentsPage: React.FC = () => {
  const { clusterInfo, loading: clusterInfoLoading, isROKS } = useClusterInfo();
  const roksAPIAvailable = clusterInfo?.features?.roksAPIAvailable === true;

  const { attachments, loading: attachmentsLoading } = useK8sVLANAttachments();
  const loading = clusterInfoLoading || attachmentsLoading;

  // ROKS cluster without ROKS API — show Coming Soon (VLAN attachments are platform-managed)
  if (!clusterInfoLoading && isROKS && !roksAPIAvailable) {
    return (
      <VPCNetworkingShell>
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
      </VPCNetworkingShell>
    );
  }

  return (
    <VPCNetworkingShell>
      <PageSection>
        <Text component={TextVariants.p} style={{ marginBottom: '16px', color: 'var(--pf-v5-global--Color--200)' }}>
          VLAN attachments on bare metal worker nodes that connect VPC subnets to OVN LocalNet networks.
        </Text>
        <Card>
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
                    <Th>Node</Th>
                    <Th>VLAN ID</Th>
                    <Th>Network</Th>
                    <Th>Status</Th>
                    <Th>Sync</Th>
                    <Th>Age</Th>
                  </Tr>
                </Thead>
                <Tbody>
                  {attachments.map((item: VLANAttachmentResource) => (
                    <Tr key={item.metadata?.name || item.id || item.name}>
                      <Td>{item.metadata?.name || item.name || '-'}</Td>
                      <Td>{item.spec?.nodeName || '-'}</Td>
                      <Td>{item.spec?.vlanID ?? '-'}</Td>
                      <Td>{item.metadata?.labels?.['vpc.roks.ibm.com/network'] || '-'}</Td>
                      <Td>{item.status?.attachmentStatus || '-'}</Td>
                      <Td>{item.status?.syncStatus || '-'}</Td>
                      <Td>{formatRelativeTime(item.metadata?.creationTimestamp || item.createdAt)}</Td>
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

export default VLANAttachmentsPage;
