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
  Alert,
  Text,
  TextVariants,
} from '@patternfly/react-core';
import { CubesIcon } from '@patternfly/react-icons';
import { Table, Thead, Tbody, Tr, Th, Td } from '@patternfly/react-table';
import { Link } from 'react-router-dom-v5-compat';
import { useVNIs, useClusterInfo } from '../api/hooks';
import StatusBadge from '../components/StatusBadge';
import { formatRelativeTime } from '../utils/formatters';
import VPCNetworkingShell from '../components/VPCNetworkingShell';

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
      </VPCNetworkingShell>
    );
  }

  return (
    <VPCNetworkingShell>
      <PageSection>
        <Text component={TextVariants.p} style={{ marginBottom: '16px', color: 'var(--pf-v5-global--Color--200)' }}>
          Virtual Network Interfaces created for VMs on LocalNet networks. VNIs are provisioned automatically when a VM is created.
        </Text>
        <Alert variant="info" isInline title="VNIs are created automatically" style={{ marginBottom: '16px' }}>
          When a VirtualMachine is created, the operator's mutating webhook provisions a VNI
          on the appropriate VPC subnet, assigns a MAC address and reserved IP, and injects
          them into the VM spec. VNIs are deleted when the VM is removed.
        </Alert>

        <Card>
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
                    <Th>Primary IP</Th>
                    <Th>Status</Th>
                    <Th>Age</Th>
                  </Tr>
                </Thead>
                <Tbody>
                  {vnis.map((vni) => (
                    <Tr key={vni.id || vni.name}>
                      <Td><Link to={`/vpc-networking/vnis/${vni.id}`}>{vni.name || '-'}</Link></Td>
                      <Td>{vni.subnet?.name || '-'}</Td>
                      <Td>{vni.primaryIp?.address ? <code>{vni.primaryIp.address}</code> : '-'}</Td>
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
    </VPCNetworkingShell>
  );
};

export default VNIsListPage;
