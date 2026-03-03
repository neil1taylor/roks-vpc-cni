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
import { useVPNGateways } from '../api/hooks';
import { VPNGateway } from '../api/types';
import { apiClient } from '../api/client';
import StatusBadge from '../components/StatusBadge';
import { DeleteConfirmModal } from '../components/DeleteConfirmModal';
import VPCNetworkingShell from '../components/VPCNetworkingShell';
import { formatRelativeTime } from '../utils/formatters';

const protocolColors: Record<string, 'blue' | 'purple' | 'cyan'> = {
  wireguard: 'blue',
  ipsec: 'purple',
  openvpn: 'cyan',
};

const VPNGatewaysListPage: React.FC = () => {
  const { vpnGateways, loading, error } = useVPNGateways();
  const [deleteTarget, setDeleteTarget] = useState<VPNGateway | null>(null);
  const [deleteError, setDeleteError] = useState('');
  const [searchFilter, setSearchFilter] = useState('');
  const navigate = useNavigate();

  const filteredGateways = vpnGateways?.filter((gw) => {
    const lower = searchFilter.toLowerCase();
    return (
      gw.name.toLowerCase().includes(lower) ||
      gw.protocol.toLowerCase().includes(lower) ||
      gw.gatewayRef.toLowerCase().includes(lower)
    );
  });

  const handleDelete = async () => {
    if (!deleteTarget) return;
    setDeleteError('');
    try {
      const resp = await apiClient.deleteVPNGateway(deleteTarget.name, deleteTarget.namespace);
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
          VPN Gateways provide site-to-site and client-to-site VPN connectivity for VM workload networks using WireGuard, IPsec/StrongSwan, or OpenVPN.
        </Text>

        {deleteError && (
          <Alert variant="danger" title={deleteError} isInline isPlain style={{ marginBottom: '16px' }} />
        )}

        <Toolbar>
          <ToolbarContent>
            <ToolbarItem>
              <SearchInput
                placeholder="Filter by name, protocol, gateway"
                value={searchFilter}
                onChange={(_e, value) => setSearchFilter(value)}
                onClear={() => setSearchFilter('')}
              />
            </ToolbarItem>
            <ToolbarItem>
              <Button variant="primary" onClick={() => navigate('/vpc-networking/vpn-gateways/create')}>Create VPN Gateway</Button>
            </ToolbarItem>
          </ToolbarContent>
        </Toolbar>

        {error && (
          <div style={{ color: 'var(--pf-v5-global--danger-color--100)', marginBottom: '16px' }}>
            Error loading VPN gateways: {error.message}
          </div>
        )}

        {(!filteredGateways || filteredGateways.length === 0) ? (
          <EmptyState>
            <EmptyStateHeader
              titleText={searchFilter ? 'No matching VPN gateways' : 'No VPN gateways configured'}
              icon={<EmptyStateIcon icon={CubesIcon} />}
            />
            <EmptyStateBody>
              {searchFilter
                ? `No VPN gateways match "${searchFilter}". Try a different search or clear the filter.`
                : 'Create a VPCVPNGateway to establish encrypted VPN tunnels to remote sites.'}
            </EmptyStateBody>
          </EmptyState>
        ) : (
          <Table aria-label="VPN Gateways list" variant="compact">
            <Thead>
              <Tr>
                <Th>Name</Th>
                <Th>Protocol</Th>
                <Th>Gateway</Th>
                <Th>Tunnels</Th>
                <Th>Phase</Th>
                <Th>Age</Th>
                <Th />
              </Tr>
            </Thead>
            <Tbody>
              {filteredGateways.map((gw: VPNGateway) => (
                <Tr key={`${gw.namespace}/${gw.name}`}>
                  <Td>
                    <Button variant="link" isInline onClick={() => navigate(`/vpc-networking/vpn-gateways/${gw.name}?ns=${encodeURIComponent(gw.namespace)}`)}>
                      {gw.name}
                    </Button>
                  </Td>
                  <Td>
                    <Label color={protocolColors[gw.protocol] || 'grey'}>{gw.protocol}</Label>
                  </Td>
                  <Td>{gw.gatewayRef || '-'}</Td>
                  <Td>{gw.activeTunnels}/{gw.totalTunnels}</Td>
                  <Td><StatusBadge status={gw.phase} /></Td>
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
        title={`Delete ${deleteTarget?.name || 'VPN gateway'}?`}
        message="Deleting this VPN gateway will tear down all VPN tunnels and disconnect remote sites. Traffic will stop flowing through the VPN."
        resourceName={deleteTarget?.name}
        onConfirm={handleDelete}
        onCancel={() => { setDeleteTarget(null); setDeleteError(''); }}
      />
    </VPCNetworkingShell>
  );
};

VPNGatewaysListPage.displayName = 'VPNGatewaysListPage';
export default VPNGatewaysListPage;
