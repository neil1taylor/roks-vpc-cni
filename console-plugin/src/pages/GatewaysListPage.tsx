import React, { useState } from 'react';
import {
  PageSection,
  Button,
  ButtonVariant,
  Toolbar,
  ToolbarContent,
  ToolbarItem,
  Spinner,
  Modal,
  ModalVariant,
  Alert,
  EmptyState,
  EmptyStateBody,
  EmptyStateHeader,
  EmptyStateIcon,
  Text,
  TextVariants,
} from '@patternfly/react-core';
import { CubesIcon } from '@patternfly/react-icons';
import { Table, Thead, Tr, Th, Tbody, Td } from '@patternfly/react-table';
import { useNavigate } from 'react-router-dom-v5-compat';
import { useGateways } from '../api/hooks';
import { Gateway } from '../api/types';
import { apiClient } from '../api/client';
import StatusBadge from '../components/StatusBadge';
import VPCNetworkingShell from '../components/VPCNetworkingShell';
import { formatRelativeTime } from '../utils/formatters';

const GatewaysListPage: React.FC = () => {
  const { gateways, loading, error } = useGateways();
  const [deleteTarget, setDeleteTarget] = useState<Gateway | null>(null);
  const [isDeleting, setIsDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState('');
  const navigate = useNavigate();

  const handleDelete = async () => {
    if (!deleteTarget) return;
    setIsDeleting(true);
    setDeleteError('');
    try {
      const resp = await apiClient.deleteGateway(deleteTarget.name, deleteTarget.namespace);
      if (resp.error) {
        const msg = resp.error.message || 'Delete failed';
        setDeleteError(typeof msg === 'string' ? msg : JSON.stringify(msg));
        setIsDeleting(false);
        return;
      }
      setDeleteTarget(null);
      setIsDeleting(false);
      window.location.reload();
    } catch (e) {
      setIsDeleting(false);
      setDeleteError(e instanceof Error ? e.message : JSON.stringify(e));
    }
  };

  if (loading) {
    return (
      <VPCNetworkingShell>
        <PageSection>
          <Spinner size="xl" />
        </PageSection>
      </VPCNetworkingShell>
    );
  }

  return (
    <VPCNetworkingShell>
      <PageSection>
        <Text component={TextVariants.p} style={{ marginBottom: '16px', color: 'var(--pf-v5-global--Color--200)' }}>
          Gateways route traffic between Layer2 overlay networks and the VPC fabric.
        </Text>
        <Toolbar>
          <ToolbarContent>
            <ToolbarItem>
              <Button variant="primary" onClick={() => navigate('/vpc-networking/gateways/create')}>Create Gateway</Button>
            </ToolbarItem>
          </ToolbarContent>
        </Toolbar>

        {error && (
          <div style={{ color: 'var(--pf-v5-global--danger-color--100)', marginBottom: '16px' }}>
            Error loading gateways: {error.message}
          </div>
        )}

        {(!gateways || gateways.length === 0) ? (
          <EmptyState>
            <EmptyStateHeader titleText="No gateways configured" icon={<EmptyStateIcon icon={CubesIcon} />} />
            <EmptyStateBody>
              Create a VPCGateway to route traffic between Layer2 overlay networks and the VPC fabric.
            </EmptyStateBody>
          </EmptyState>
        ) : (
          <Table aria-label="Gateways list" variant="compact">
            <Thead>
              <Tr>
                <Th>Name</Th>
                <Th>Zone</Th>
                <Th>Uplink Network</Th>
                <Th>Floating IP</Th>
                <Th>PAR CIDR</Th>
                <Th>VNI IP</Th>
                <Th>Routes</Th>
                <Th>Status</Th>
                <Th>Age</Th>
                <Th />
              </Tr>
            </Thead>
            <Tbody>
              {gateways.map((gw: Gateway) => (
                <Tr key={gw.name}>
                  <Td>
                    <Button variant="link" isInline onClick={() => navigate(`/vpc-networking/gateways/${gw.name}?ns=${encodeURIComponent(gw.namespace)}`)}>
                      {gw.name}
                    </Button>
                  </Td>
                  <Td>{gw.zone || '-'}</Td>
                  <Td>{gw.uplinkNetwork || '-'}</Td>
                  <Td>{gw.floatingIP || '-'}</Td>
                  <Td>{gw.publicAddressRangeCIDR || '-'}</Td>
                  <Td>{gw.reservedIP || '-'}</Td>
                  <Td>{gw.vpcRouteCount}</Td>
                  <Td><StatusBadge status={gw.syncStatus} /></Td>
                  <Td>{formatRelativeTime(gw.createdAt)}</Td>
                  <Td isActionCell>
                    <Button variant={ButtonVariant.link} isDanger onClick={() => { setDeleteTarget(gw); setDeleteError(''); }}>
                      Delete
                    </Button>
                  </Td>
                </Tr>
              ))}
            </Tbody>
          </Table>
        )}
      </PageSection>

      <Modal
        variant={ModalVariant.small}
        title={`Delete ${deleteTarget?.name || 'gateway'}?`}
        isOpen={!!deleteTarget}
        onClose={() => setDeleteTarget(null)}
        actions={[
          <Button key="cancel" variant={ButtonVariant.link} onClick={() => setDeleteTarget(null)} isDisabled={isDeleting}>
            Cancel
          </Button>,
          <Button key="delete" variant={ButtonVariant.danger} onClick={handleDelete} isLoading={isDeleting} isDisabled={isDeleting}>
            Delete
          </Button>,
        ]}
      >
        {deleteError && (
          <Alert variant="danger" isInline title={deleteError} style={{ marginBottom: '16px' }} />
        )}
        Are you sure you want to delete gateway <strong>{deleteTarget?.name}</strong>?
        This will remove all associated VPC routes, NAT rules, and the gateway VNI.
      </Modal>
    </VPCNetworkingShell>
  );
};

GatewaysListPage.displayName = 'GatewaysListPage';
export default GatewaysListPage;
