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
import { useRouters } from '../api/hooks';
import { Router } from '../api/types';
import { apiClient } from '../api/client';
import StatusBadge from '../components/StatusBadge';
import VPCNetworkingShell from '../components/VPCNetworkingShell';
import { formatRelativeTime } from '../utils/formatters';

const RoutersListPage: React.FC = () => {
  const { routers, loading, error } = useRouters();
  const [deleteTarget, setDeleteTarget] = useState<Router | null>(null);
  const [isDeleting, setIsDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState('');
  const navigate = useNavigate();

  const handleDelete = async () => {
    if (!deleteTarget) return;
    setIsDeleting(true);
    setDeleteError('');
    try {
      const resp = await apiClient.deleteRouter(deleteTarget.name);
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
          Routers connect workload networks to a gateway for external access and inter-network routing.
        </Text>
        <Toolbar>
          <ToolbarContent>
            <ToolbarItem>
              <Button variant="primary">Create Router</Button>
            </ToolbarItem>
          </ToolbarContent>
        </Toolbar>

        {error && (
          <div style={{ color: 'var(--pf-v5-global--danger-color--100)', marginBottom: '16px' }}>
            Error loading routers: {error.message}
          </div>
        )}

        {(!routers || routers.length === 0) ? (
          <EmptyState>
            <EmptyStateHeader titleText="No routers configured" icon={<EmptyStateIcon icon={CubesIcon} />} />
            <EmptyStateBody>
              Create a VPCRouter to connect Layer2 segments and uplink to a gateway.
            </EmptyStateBody>
          </EmptyState>
        ) : (
          <Table aria-label="Routers list" variant="compact">
            <Thead>
              <Tr>
                <Th>Name</Th>
                <Th>Gateway</Th>
                <Th>Networks</Th>
                <Th>Transit IP</Th>
                <Th>Functions</Th>
                <Th>Status</Th>
                <Th>Age</Th>
                <Th />
              </Tr>
            </Thead>
            <Tbody>
              {routers.map((router: Router) => (
                <Tr key={router.name}>
                  <Td>
                    <Button variant="link" isInline onClick={() => navigate(`/vpc-networking/routers/${router.name}`)}>
                      {router.name}
                    </Button>
                  </Td>
                  <Td>
                    <Button variant="link" isInline onClick={() => navigate(`/vpc-networking/gateways/${router.gateway}`)}>
                      {router.gateway}
                    </Button>
                  </Td>
                  <Td>{router.networks?.length ?? 0}</Td>
                  <Td>{router.transitIP || '-'}</Td>
                  <Td>{router.functions && router.functions.length > 0 ? router.functions.join(', ') : '-'}</Td>
                  <Td><StatusBadge status={router.syncStatus} /></Td>
                  <Td>{formatRelativeTime(router.createdAt)}</Td>
                  <Td isActionCell>
                    <Button variant={ButtonVariant.link} isDanger onClick={() => { setDeleteTarget(router); setDeleteError(''); }}>
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
        title={`Delete ${deleteTarget?.name || 'router'}?`}
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
        Are you sure you want to delete router <strong>{deleteTarget?.name}</strong>?
        This will disconnect all attached networks and remove the router.
      </Modal>
    </VPCNetworkingShell>
  );
};

RoutersListPage.displayName = 'RoutersListPage';
export default RoutersListPage;
