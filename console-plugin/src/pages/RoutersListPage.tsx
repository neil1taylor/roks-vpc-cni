import React, { useState } from 'react';
import {
  PageSection,
  Button,
  ButtonVariant,
  Toolbar,
  ToolbarContent,
  ToolbarItem,
  Spinner,
  Alert,
  EmptyState,
  EmptyStateBody,
  EmptyStateHeader,
  EmptyStateIcon,
  Text,
  TextVariants,
  SearchInput,
  Label,
} from '@patternfly/react-core';
import { CubesIcon } from '@patternfly/react-icons';
import { Table, Thead, Tr, Th, Tbody, Td } from '@patternfly/react-table';
import { useNavigate } from 'react-router-dom-v5-compat';
import { useRouters } from '../api/hooks';
import { Router } from '../api/types';
import { apiClient } from '../api/client';
import StatusBadge from '../components/StatusBadge';
import { DeleteConfirmModal } from '../components/DeleteConfirmModal';
import VPCNetworkingShell from '../components/VPCNetworkingShell';
import { formatRelativeTime } from '../utils/formatters';

const RoutersListPage: React.FC = () => {
  const { routers, loading, error } = useRouters();
  const [deleteTarget, setDeleteTarget] = useState<Router | null>(null);
  const [deleteError, setDeleteError] = useState('');
  const [searchFilter, setSearchFilter] = useState('');
  const navigate = useNavigate();

  const filteredRouters = routers?.filter(
    (r) => r.name.toLowerCase().includes(searchFilter.toLowerCase()),
  );

  const handleDelete = async () => {
    if (!deleteTarget) return;
    setDeleteError('');
    try {
      const resp = await apiClient.deleteRouter(deleteTarget.name, deleteTarget.namespace);
      if (resp.error) {
        const msg = resp.error.message || 'Delete failed';
        setDeleteError(typeof msg === 'string' ? msg : JSON.stringify(msg));
        return;
      }
      setDeleteTarget(null);
      window.location.reload();
    } catch (e) {
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

        {deleteError && (
          <Alert variant="danger" title={deleteError} isInline isPlain style={{ marginBottom: '16px' }} />
        )}

        <Toolbar>
          <ToolbarContent>
            <ToolbarItem>
              <SearchInput
                placeholder="Filter by name"
                value={searchFilter}
                onChange={(_e, value) => setSearchFilter(value)}
                onClear={() => setSearchFilter('')}
              />
            </ToolbarItem>
            <ToolbarItem>
              <Button variant="primary" onClick={() => navigate('/vpc-networking/routers/create')}>Create Router</Button>
            </ToolbarItem>
          </ToolbarContent>
        </Toolbar>

        {error && (
          <div style={{ color: 'var(--pf-v5-global--danger-color--100)', marginBottom: '16px' }}>
            Error loading routers: {error.message}
          </div>
        )}

        {(!filteredRouters || filteredRouters.length === 0) ? (
          <EmptyState>
            <EmptyStateHeader
              titleText={searchFilter ? 'No matching routers' : 'No routers configured'}
              icon={<EmptyStateIcon icon={CubesIcon} />}
            />
            <EmptyStateBody>
              {searchFilter
                ? `No routers match "${searchFilter}". Try a different search or clear the filter.`
                : 'Create a VPCRouter to connect Layer2 segments and uplink to a gateway.'}
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
                <Th>Mode</Th>
                <Th>Functions</Th>
                <Th>Status</Th>
                <Th>Age</Th>
                <Th />
              </Tr>
            </Thead>
            <Tbody>
              {filteredRouters.map((router: Router) => (
                <Tr key={router.name}>
                  <Td>
                    <Button variant="link" isInline onClick={() => navigate(`/vpc-networking/routers/${router.name}?ns=${encodeURIComponent(router.namespace)}`)}>
                      {router.name}
                    </Button>
                  </Td>
                  <Td>
                    <Button variant="link" isInline onClick={() => navigate(`/vpc-networking/gateways/${router.gateway}?ns=${encodeURIComponent(router.namespace)}`)}>
                      {router.gateway}
                    </Button>
                  </Td>
                  <Td>{router.networks?.length ?? 0}</Td>
                  <Td>{router.transitIP || '-'}</Td>
                  <Td>
                    {router.mode === 'fast-path' ? (
                      <>
                        <Label color="purple" isCompact>Fast-path</Label>
                        {router.xdpEnabled && <Label color="green" isCompact style={{ marginLeft: 4 }}>XDP</Label>}
                      </>
                    ) : (
                      <Label color="blue" isCompact>Standard</Label>
                    )}
                  </Td>
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

      <DeleteConfirmModal
        isOpen={!!deleteTarget}
        title={`Delete ${deleteTarget?.name || 'router'}?`}
        message="Deleting this router will remove the router pod and disconnect all attached networks from the gateway. VMs on those networks will lose external connectivity."
        resourceName={deleteTarget?.name}
        onConfirm={handleDelete}
        onCancel={() => { setDeleteTarget(null); setDeleteError(''); }}
      />
    </VPCNetworkingShell>
  );
};

RoutersListPage.displayName = 'RoutersListPage';
export default RoutersListPage;
