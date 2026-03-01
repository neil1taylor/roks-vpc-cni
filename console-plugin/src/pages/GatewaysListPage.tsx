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
} from '@patternfly/react-core';
import { CubesIcon } from '@patternfly/react-icons';
import { Table, Thead, Tr, Th, Tbody, Td } from '@patternfly/react-table';
import { useNavigate } from 'react-router-dom-v5-compat';
import { useGateways } from '../api/hooks';
import { Gateway } from '../api/types';
import { apiClient } from '../api/client';
import StatusBadge from '../components/StatusBadge';
import { DeleteConfirmModal } from '../components/DeleteConfirmModal';
import VPCNetworkingShell from '../components/VPCNetworkingShell';
import { formatRelativeTime } from '../utils/formatters';

const GatewaysListPage: React.FC = () => {
  const { gateways, loading, error } = useGateways();
  const [deleteTarget, setDeleteTarget] = useState<Gateway | null>(null);
  const [deleteError, setDeleteError] = useState('');
  const [searchFilter, setSearchFilter] = useState('');
  const navigate = useNavigate();

  const filteredGateways = gateways?.filter(
    (gw) => gw.name.toLowerCase().includes(searchFilter.toLowerCase()),
  );

  const handleDelete = async () => {
    if (!deleteTarget) return;
    setDeleteError('');
    try {
      const resp = await apiClient.deleteGateway(deleteTarget.name, deleteTarget.namespace);
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
          Gateways route traffic between Layer2 overlay networks and the VPC fabric.
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
              <Button variant="primary" onClick={() => navigate('/vpc-networking/gateways/create')}>Create Gateway</Button>
            </ToolbarItem>
          </ToolbarContent>
        </Toolbar>

        {error && (
          <div style={{ color: 'var(--pf-v5-global--danger-color--100)', marginBottom: '16px' }}>
            Error loading gateways: {error.message}
          </div>
        )}

        {(!filteredGateways || filteredGateways.length === 0) ? (
          <EmptyState>
            <EmptyStateHeader
              titleText={searchFilter ? 'No matching gateways' : 'No gateways configured'}
              icon={<EmptyStateIcon icon={CubesIcon} />}
            />
            <EmptyStateBody>
              {searchFilter
                ? `No gateways match "${searchFilter}". Try a different search or clear the filter.`
                : 'Create a VPCGateway to route traffic between Layer2 overlay networks and the VPC fabric.'}
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
              {filteredGateways.map((gw: Gateway) => (
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

      <DeleteConfirmModal
        isOpen={!!deleteTarget}
        title={`Delete ${deleteTarget?.name || 'gateway'}?`}
        message="Deleting this gateway will remove its uplink VNI, floating IP, all VPC routes, NAT rules, and any associated Public Address Range. Connected routers will lose their uplink."
        resourceName={deleteTarget?.name}
        onConfirm={handleDelete}
        onCancel={() => { setDeleteTarget(null); setDeleteError(''); }}
      />
    </VPCNetworkingShell>
  );
};

GatewaysListPage.displayName = 'GatewaysListPage';
export default GatewaysListPage;
