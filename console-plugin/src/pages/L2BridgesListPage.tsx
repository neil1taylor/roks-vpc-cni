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
import { useL2Bridges } from '../api/hooks';
import { L2Bridge } from '../api/types';
import { apiClient } from '../api/client';
import StatusBadge from '../components/StatusBadge';
import { DeleteConfirmModal } from '../components/DeleteConfirmModal';
import VPCNetworkingShell from '../components/VPCNetworkingShell';
import { formatRelativeTime } from '../utils/formatters';

const typeColors: Record<string, 'blue' | 'purple' | 'cyan'> = {
  'gretap-wireguard': 'blue',
  'l2vpn': 'purple',
  'evpn-vxlan': 'cyan',
};

const L2BridgesListPage: React.FC = () => {
  const { l2bridges, loading, error } = useL2Bridges();
  const [deleteTarget, setDeleteTarget] = useState<L2Bridge | null>(null);
  const [deleteError, setDeleteError] = useState('');
  const [searchFilter, setSearchFilter] = useState('');
  const navigate = useNavigate();

  const filteredBridges = l2bridges?.filter((bridge) => {
    const lower = searchFilter.toLowerCase();
    return (
      bridge.name.toLowerCase().includes(lower) ||
      bridge.type.toLowerCase().includes(lower) ||
      bridge.remoteEndpoint.toLowerCase().includes(lower)
    );
  });

  const handleDelete = async () => {
    if (!deleteTarget) return;
    setDeleteError('');
    try {
      const resp = await apiClient.deleteL2Bridge(deleteTarget.name, deleteTarget.namespace);
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
          L2 Bridges extend Layer 2 networks across sites — connecting NSX-T segments, on-prem networks, or multi-cloud VNETs to OVN-Kubernetes workload networks via encrypted tunnels.
        </Text>

        {deleteError && (
          <Alert variant="danger" title={deleteError} isInline isPlain style={{ marginBottom: '16px' }} />
        )}

        <Toolbar>
          <ToolbarContent>
            <ToolbarItem>
              <SearchInput
                placeholder="Filter by name, type, remote endpoint"
                value={searchFilter}
                onChange={(_e, value) => setSearchFilter(value)}
                onClear={() => setSearchFilter('')}
              />
            </ToolbarItem>
            <ToolbarItem>
              <Button variant="primary" onClick={() => navigate('/vpc-networking/l2-bridges/create')}>Create L2 Bridge</Button>
            </ToolbarItem>
          </ToolbarContent>
        </Toolbar>

        {error && (
          <div style={{ color: 'var(--pf-v5-global--danger-color--100)', marginBottom: '16px' }}>
            Error loading L2 bridges: {error.message}
          </div>
        )}

        {(!filteredBridges || filteredBridges.length === 0) ? (
          <EmptyState>
            <EmptyStateHeader
              titleText={searchFilter ? 'No matching L2 bridges' : 'No L2 bridges configured'}
              icon={<EmptyStateIcon icon={CubesIcon} />}
            />
            <EmptyStateBody>
              {searchFilter
                ? `No L2 bridges match "${searchFilter}". Try a different search or clear the filter.`
                : 'Create a VPCL2Bridge to extend Layer 2 networks across sites via encrypted tunnels.'}
            </EmptyStateBody>
          </EmptyState>
        ) : (
          <Table aria-label="L2 Bridges list" variant="compact">
            <Thead>
              <Tr>
                <Th>Name</Th>
                <Th>Type</Th>
                <Th>Network</Th>
                <Th>Remote</Th>
                <Th>Phase</Th>
                <Th>Age</Th>
                <Th />
              </Tr>
            </Thead>
            <Tbody>
              {filteredBridges.map((bridge: L2Bridge) => (
                <Tr key={`${bridge.namespace}/${bridge.name}`}>
                  <Td>
                    <Button variant="link" isInline onClick={() => navigate(`/vpc-networking/l2-bridges/${bridge.name}?ns=${encodeURIComponent(bridge.namespace)}`)}>
                      {bridge.name}
                    </Button>
                  </Td>
                  <Td>
                    <Label color={typeColors[bridge.type] || 'grey'}>{bridge.type}</Label>
                  </Td>
                  <Td>{bridge.networkRef?.name || '-'}</Td>
                  <Td>{bridge.remoteEndpoint || '-'}</Td>
                  <Td><StatusBadge status={bridge.phase} /></Td>
                  <Td>{formatRelativeTime(bridge.createdAt)}</Td>
                  <Td isActionCell>
                    <Button variant={ButtonVariant.link} isDanger onClick={() => { setDeleteTarget(bridge); setDeleteError(''); }}>
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
        title={`Delete ${deleteTarget?.name || 'L2 bridge'}?`}
        message="Deleting this L2 bridge will tear down the encrypted tunnel and disconnect the remote network segment. Traffic will stop flowing between sites."
        resourceName={deleteTarget?.name}
        onConfirm={handleDelete}
        onCancel={() => { setDeleteTarget(null); setDeleteError(''); }}
      />
    </VPCNetworkingShell>
  );
};

L2BridgesListPage.displayName = 'L2BridgesListPage';
export default L2BridgesListPage;
