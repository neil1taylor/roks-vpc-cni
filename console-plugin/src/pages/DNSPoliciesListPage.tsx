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
import { useDNSPolicies } from '../api/hooks';
import { DNSPolicy } from '../api/types';
import { apiClient } from '../api/client';
import StatusBadge from '../components/StatusBadge';
import { DeleteConfirmModal } from '../components/DeleteConfirmModal';
import VPCNetworkingShell from '../components/VPCNetworkingShell';
import { formatRelativeTime } from '../utils/formatters';

const DNSPoliciesListPage: React.FC = () => {
  const { dnsPolicies, loading, error } = useDNSPolicies();
  const [deleteTarget, setDeleteTarget] = useState<DNSPolicy | null>(null);
  const [deleteError, setDeleteError] = useState('');
  const [searchFilter, setSearchFilter] = useState('');
  const navigate = useNavigate();

  const filteredPolicies = dnsPolicies?.filter((dp) => {
    const lower = searchFilter.toLowerCase();
    return (
      dp.name.toLowerCase().includes(lower) ||
      dp.routerRef.toLowerCase().includes(lower) ||
      (dp.phase || '').toLowerCase().includes(lower)
    );
  });

  const handleDelete = async () => {
    if (!deleteTarget) return;
    setDeleteError('');
    try {
      const resp = await apiClient.deleteDNSPolicy(deleteTarget.name, deleteTarget.namespace);
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
          DNS Policies configure upstream DNS servers, ad/tracker filtering, and local DNS resolution for VPCRouter pods via AdGuard Home sidecars.
        </Text>

        {deleteError && (
          <Alert variant="danger" title={deleteError} isInline isPlain style={{ marginBottom: '16px' }} />
        )}

        <Toolbar>
          <ToolbarContent>
            <ToolbarItem>
              <SearchInput
                placeholder="Filter by name, router, phase"
                value={searchFilter}
                onChange={(_e, value) => setSearchFilter(value)}
                onClear={() => setSearchFilter('')}
              />
            </ToolbarItem>
            <ToolbarItem>
              <Button variant="primary" onClick={() => navigate('/vpc-networking/dns-policies/create')}>Create DNS Policy</Button>
            </ToolbarItem>
          </ToolbarContent>
        </Toolbar>

        {error && (
          <div style={{ color: 'var(--pf-v5-global--danger-color--100)', marginBottom: '16px' }}>
            Error loading DNS policies: {error.message}
          </div>
        )}

        {(!filteredPolicies || filteredPolicies.length === 0) ? (
          <EmptyState>
            <EmptyStateHeader
              titleText={searchFilter ? 'No matching DNS policies' : 'No DNS policies configured'}
              icon={<EmptyStateIcon icon={CubesIcon} />}
            />
            <EmptyStateBody>
              {searchFilter
                ? `No DNS policies match "${searchFilter}". Try a different search or clear the filter.`
                : 'Create a VPCDNSPolicy to configure DNS resolution and filtering for a VPCRouter.'}
            </EmptyStateBody>
          </EmptyState>
        ) : (
          <Table aria-label="DNS Policies list" variant="compact">
            <Thead>
              <Tr>
                <Th>Name</Th>
                <Th>Router</Th>
                <Th>Phase</Th>
                <Th>Rules</Th>
                <Th>Upstream</Th>
                <Th>Age</Th>
                <Th />
              </Tr>
            </Thead>
            <Tbody>
              {filteredPolicies.map((dp: DNSPolicy) => (
                <Tr key={`${dp.namespace}/${dp.name}`}>
                  <Td>
                    <Button variant="link" isInline onClick={() => navigate(`/vpc-networking/dns-policies/${dp.name}?ns=${encodeURIComponent(dp.namespace)}`)}>
                      {dp.name}
                    </Button>
                  </Td>
                  <Td>{dp.routerRef || '-'}</Td>
                  <Td><StatusBadge status={dp.phase} /></Td>
                  <Td>
                    {dp.filteringEnabled ? (
                      <Label color="blue">{dp.filterRulesLoaded} rules</Label>
                    ) : (
                      <Label color="grey">Off</Label>
                    )}
                  </Td>
                  <Td>{dp.upstreamServers && dp.upstreamServers.length > 0 ? dp.upstreamServers.length : '-'}</Td>
                  <Td>{formatRelativeTime(dp.createdAt)}</Td>
                  <Td isActionCell>
                    <Button variant={ButtonVariant.link} isDanger onClick={() => { setDeleteTarget(dp); setDeleteError(''); }}>
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
        title={`Delete ${deleteTarget?.name || 'DNS policy'}?`}
        message="Deleting this DNS policy will remove the AdGuard Home sidecar from the associated router pod. DNS resolution will revert to default dnsmasq behavior."
        resourceName={deleteTarget?.name}
        onConfirm={handleDelete}
        onCancel={() => { setDeleteTarget(null); setDeleteError(''); }}
      />
    </VPCNetworkingShell>
  );
};

DNSPoliciesListPage.displayName = 'DNSPoliciesListPage';
export default DNSPoliciesListPage;
