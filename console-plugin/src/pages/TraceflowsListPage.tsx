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
import { useTraceflows } from '../api/hooks';
import { Traceflow } from '../api/types';
import { apiClient } from '../api/client';
import { DeleteConfirmModal } from '../components/DeleteConfirmModal';
import VPCNetworkingShell from '../components/VPCNetworkingShell';
import { formatRelativeTime } from '../utils/formatters';

type PhaseBadgeColor = 'grey' | 'blue' | 'green' | 'red';
type ResultBadgeColor = 'green' | 'red' | 'orange' | 'grey';

const phaseColors: Record<string, PhaseBadgeColor> = {
  pending: 'grey',
  running: 'blue',
  completed: 'green',
  failed: 'red',
};

const resultColors: Record<string, ResultBadgeColor> = {
  reachable: 'green',
  unreachable: 'red',
  filtered: 'orange',
  timeout: 'grey',
};

const TraceflowsListPage: React.FC = () => {
  const { traceflows, loading, error } = useTraceflows();
  const [deleteTarget, setDeleteTarget] = useState<Traceflow | null>(null);
  const [deleteError, setDeleteError] = useState('');
  const [searchFilter, setSearchFilter] = useState('');
  const navigate = useNavigate();

  const filteredTraceflows = traceflows?.filter((tf) => {
    const lower = searchFilter.toLowerCase();
    return (
      tf.name.toLowerCase().includes(lower) ||
      tf.sourceIP.toLowerCase().includes(lower) ||
      tf.destinationIP.toLowerCase().includes(lower) ||
      tf.router.toLowerCase().includes(lower) ||
      (tf.phase || '').toLowerCase().includes(lower) ||
      (tf.result || '').toLowerCase().includes(lower)
    );
  });

  const handleDelete = async () => {
    if (!deleteTarget) return;
    setDeleteError('');
    try {
      const resp = await apiClient.deleteTraceflow(deleteTarget.name, deleteTarget.namespace);
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
          Traceflows diagnose network path reachability through VPCRouter pods by injecting probe packets and recording hop-by-hop results.
        </Text>

        {deleteError && (
          <Alert variant="danger" title={deleteError} isInline isPlain style={{ marginBottom: '16px' }} />
        )}

        <Toolbar>
          <ToolbarContent>
            <ToolbarItem>
              <SearchInput
                placeholder="Filter by name, IP, router, phase, result"
                value={searchFilter}
                onChange={(_e, value) => setSearchFilter(value)}
                onClear={() => setSearchFilter('')}
              />
            </ToolbarItem>
            <ToolbarItem>
              <Button variant="primary" onClick={() => navigate('/vpc-networking/traceflows/create')}>Create Traceflow</Button>
            </ToolbarItem>
          </ToolbarContent>
        </Toolbar>

        {error && (
          <div style={{ color: 'var(--pf-v5-global--danger-color--100)', marginBottom: '16px' }}>
            Error loading traceflows: {error.message}
          </div>
        )}

        {(!filteredTraceflows || filteredTraceflows.length === 0) ? (
          <EmptyState>
            <EmptyStateHeader
              titleText={searchFilter ? 'No matching traceflows' : 'No traceflows'}
              icon={<EmptyStateIcon icon={CubesIcon} />}
            />
            <EmptyStateBody>
              {searchFilter
                ? `No traceflows match "${searchFilter}". Try a different search or clear the filter.`
                : 'Create a VPCTraceflow to diagnose network path reachability through a VPCRouter.'}
            </EmptyStateBody>
          </EmptyState>
        ) : (
          <Table aria-label="Traceflows list" variant="compact">
            <Thead>
              <Tr>
                <Th>Name</Th>
                <Th>Phase</Th>
                <Th>Result</Th>
                <Th>Source</Th>
                <Th>Destination</Th>
                <Th>Router</Th>
                <Th>Latency</Th>
                <Th>Age</Th>
                <Th />
              </Tr>
            </Thead>
            <Tbody>
              {filteredTraceflows.map((tf: Traceflow) => (
                <Tr key={`${tf.namespace}/${tf.name}`}>
                  <Td>
                    <Button variant="link" isInline onClick={() => navigate(`/vpc-networking/traceflows/${tf.name}?ns=${encodeURIComponent(tf.namespace)}`)}>
                      {tf.name}
                    </Button>
                  </Td>
                  <Td>
                    <Label color={phaseColors[tf.phase?.toLowerCase()] || 'grey'} variant="outline">
                      {tf.phase || 'Unknown'}
                    </Label>
                  </Td>
                  <Td>
                    {tf.result ? (
                      <Label color={resultColors[tf.result?.toLowerCase()] || 'grey'} variant="outline">
                        {tf.result}
                      </Label>
                    ) : '-'}
                  </Td>
                  <Td>{tf.sourceIP || '-'}</Td>
                  <Td>
                    {tf.destinationIP}
                    {tf.destinationPort ? `:${tf.destinationPort}` : ''}
                  </Td>
                  <Td>{tf.router || '-'}</Td>
                  <Td>
                    {tf.totalLatencyMs !== undefined && tf.totalLatencyMs !== null
                      ? `${tf.totalLatencyMs} ms`
                      : '-'}
                  </Td>
                  <Td>{formatRelativeTime(tf.createdAt)}</Td>
                  <Td isActionCell>
                    <Button variant={ButtonVariant.link} isDanger onClick={() => { setDeleteTarget(tf); setDeleteError(''); }}>
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
        title={`Delete ${deleteTarget?.name || 'traceflow'}?`}
        message="Deleting this traceflow will remove the diagnostic result. This action cannot be undone."
        resourceName={deleteTarget?.name}
        onConfirm={handleDelete}
        onCancel={() => { setDeleteTarget(null); setDeleteError(''); }}
      />
    </VPCNetworkingShell>
  );
};

TraceflowsListPage.displayName = 'TraceflowsListPage';
export default TraceflowsListPage;
