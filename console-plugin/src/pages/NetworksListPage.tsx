import React, { useState } from 'react';
import {
  PageSection,
  Button,
  ButtonVariant,
  Toolbar,
  ToolbarContent,
  ToolbarItem,
  Spinner,
  Label,
  Modal,
  ModalVariant,
  Alert,
  EmptyState,
  EmptyStateBody,
  EmptyStateHeader,
  EmptyStateIcon,
  FormSelect,
  FormSelectOption,
  Text,
  TextVariants,
} from '@patternfly/react-core';
import { CubesIcon } from '@patternfly/react-icons';
import { Table, Thead, Tr, Th, Tbody, Td } from '@patternfly/react-table';
import { useNavigate } from 'react-router-dom-v5-compat';
import { useNetworkDefinitions, useClusterInfo } from '../api/hooks';
import { NetworkDefinition } from '../api/types';
import { apiClient } from '../api/client';
import TierBadge from '../components/TierBadge';
import NetworkCreationWizard from '../components/NetworkCreationWizard';
import NetworkTypesInfoPanel from '../components/NetworkTypesInfoPanel';
import VPCNetworkingShell from '../components/VPCNetworkingShell';

const ipModeLabels: Record<string, string> = {
  static_reserved: 'Static Reserved IP',
  dhcp: 'DHCP',
  none: 'Manual',
};

const NetworksListPage: React.FC = () => {
  const [removedKeys, setRemovedKeys] = useState<Set<string>>(new Set());
  const [isWizardOpen, setIsWizardOpen] = useState(false);

  const { networks: fetchedNetworks, loading, error, refetch } = useNetworkDefinitions(isWizardOpen);
  const { isROKS } = useClusterInfo();
  const [nsFilter, setNsFilter] = useState('');
  const [deleteTarget, setDeleteTarget] = useState<NetworkDefinition | null>(null);
  const [isDeleting, setIsDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState('');
  const navigate = useNavigate();

  // Filter out optimistically removed networks (pending finalizer deletion)
  const networksAfterRemoval = fetchedNetworks?.filter(
    (n) => !removedKeys.has(`${n.kind}-${n.namespace || ''}-${n.name}`),
  ) ?? null;

  // Apply namespace filter (CUDNs always show since they're cluster-scoped)
  const networks = nsFilter
    ? networksAfterRemoval?.filter((n) => n.kind === 'ClusterUserDefinedNetwork' || n.namespace === nsFilter) ?? null
    : networksAfterRemoval;

  // Distinct namespaces for the filter dropdown
  const distinctNamespaces = React.useMemo(() => {
    if (!networksAfterRemoval) return [];
    const nsSet = new Set<string>();
    networksAfterRemoval.forEach((n) => { if (n.namespace) nsSet.add(n.namespace); });
    return Array.from(nsSet).sort();
  }, [networksAfterRemoval]);

  // Clear removed keys when a fresh fetch no longer includes them
  React.useEffect(() => {
    if (!fetchedNetworks || removedKeys.size === 0) return;
    const fetchedKeys = new Set(fetchedNetworks.map((n) => `${n.kind}-${n.namespace || ''}-${n.name}`));
    const stillPending = new Set<string>();
    removedKeys.forEach((k) => { if (fetchedKeys.has(k)) stillPending.add(k); });
    if (stillPending.size !== removedKeys.size) setRemovedKeys(stillPending);
  }, [fetchedNetworks, removedKeys]);

  const handleDelete = async () => {
    if (!deleteTarget) return;
    setIsDeleting(true);
    setDeleteError('');
    try {
      const resp = deleteTarget.kind === 'UserDefinedNetwork'
        ? await apiClient.deleteUDN(deleteTarget.namespace || '', deleteTarget.name)
        : await apiClient.deleteCUDN(deleteTarget.name);
      if (resp.error) {
        const msg = resp.error.message || 'Delete failed';
        setDeleteError(typeof msg === 'string' ? msg : JSON.stringify(msg));
        setIsDeleting(false);
        return;
      }
      // Optimistically remove from UI (K8s object may linger due to finalizers)
      setRemovedKeys((prev) => new Set(prev).add(`${deleteTarget.kind}-${deleteTarget.namespace || ''}-${deleteTarget.name}`));
      setDeleteTarget(null);
      setIsDeleting(false);
      refetch();
    } catch (e) {
      setIsDeleting(false);
      setDeleteError(e instanceof Error ? e.message : JSON.stringify(e));
    }
  };

  const topologyLabel = (topology: string) => {
    if (topology === 'LocalNet') return <Label color="blue">LocalNet</Label>;
    if (topology === 'Layer2') return <Label color="green">Layer2</Label>;
    return <Label>{topology}</Label>;
  };

  const scopeLabel = (kind: string) => {
    if (kind === 'ClusterUserDefinedNetwork') return <Label color="purple">Cluster</Label>;
    if (kind === 'UserDefinedNetwork') return <Label color="cyan">Namespace</Label>;
    return <Label>{kind}</Label>;
  };

  const handleNameClick = (net: NetworkDefinition) => {
    const params = new URLSearchParams();
    params.set('kind', net.kind);
    if (net.namespace) params.set('ns', net.namespace);
    navigate(`/vpc-networking/networks/${net.name}?${params.toString()}`);
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
          OVN user-defined networks (CUDNs and UDNs) that provide VPC-backed or cluster-internal connectivity for VMs.
        </Text>
        <Toolbar>
          <ToolbarContent>
            <ToolbarItem>
              <Button variant="primary" onClick={() => setIsWizardOpen(true)}>Create Network</Button>
            </ToolbarItem>
            <ToolbarItem>
              <FormSelect
                value={nsFilter}
                onChange={(_e, val) => setNsFilter(val)}
                aria-label="Filter by namespace"
                style={{ minWidth: '200px' }}
              >
                <FormSelectOption value="" label="All namespaces" />
                {distinctNamespaces.map((ns) => (
                  <FormSelectOption key={ns} value={ns} label={ns} />
                ))}
              </FormSelect>
            </ToolbarItem>
          </ToolbarContent>
        </Toolbar>

        <NetworkTypesInfoPanel isROKS={isROKS} />

        {error && (
          <div style={{ color: 'var(--pf-v5-global--danger-color--100)', marginBottom: '16px' }}>
            Error loading networks: {error.message}
          </div>
        )}

        {(!networks || networks.length === 0) ? (
          <EmptyState>
            <EmptyStateHeader titleText="No networks found" icon={<EmptyStateIcon icon={CubesIcon} />} />
            <EmptyStateBody>
              Create a ClusterUserDefinedNetwork or UserDefinedNetwork to get started with VPC networking.
            </EmptyStateBody>
          </EmptyState>
        ) : (
          <Table aria-label="Networks list" variant="compact">
            <Thead>
              <Tr>
                <Th>Name</Th>
                <Th>Topology</Th>
                <Th>Scope</Th>
                <Th>Role</Th>
                <Th>Namespace</Th>
                <Th>IP Mode</Th>
                <Th>Tier</Th>
                <Th>VPC Subnet</Th>
                <Th>Status</Th>
                <Th />
              </Tr>
            </Thead>
            <Tbody>
              {networks.map((net: NetworkDefinition) => (
                <Tr key={`${net.kind}-${net.namespace || ''}-${net.name}`}>
                  <Td>
                    <Button variant="link" isInline onClick={() => handleNameClick(net)}>
                      {net.name}
                    </Button>
                  </Td>
                  <Td>{topologyLabel(net.topology)}</Td>
                  <Td>{scopeLabel(net.kind)}</Td>
                  <Td>{net.role || 'Secondary'}</Td>
                  <Td>{net.namespace || '-'}</Td>
                  <Td>{net.ip_mode ? ipModeLabels[net.ip_mode] || net.ip_mode : '-'}</Td>
                  <Td>{net.tier ? <TierBadge tier={net.tier} /> : '-'}</Td>
                  <Td>{net.topology === 'LocalNet' ? (net.subnet_name || net.subnet_id || 'Pending') : 'N/A'}</Td>
                  <Td>
                    {net.topology === 'LocalNet'
                      ? <Label color={net.subnet_status === 'active' ? 'green' : 'orange'}>{net.subnet_status || 'pending'}</Label>
                      : <Label color="green">Active</Label>
                    }
                  </Td>
                  <Td isActionCell>
                    <Button variant={ButtonVariant.link} isDanger onClick={() => { setDeleteTarget(net); setDeleteError(''); }}>
                      Delete
                    </Button>
                  </Td>
                </Tr>
              ))}
            </Tbody>
          </Table>
        )}
      </PageSection>

      <NetworkCreationWizard
        isOpen={isWizardOpen}
        onClose={() => setIsWizardOpen(false)}
        onCreated={() => {
          setIsWizardOpen(false);
          refetch();
        }}
      />

      <Modal
        variant={ModalVariant.small}
        title={`Delete ${deleteTarget?.name || 'network'}?`}
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
        Are you sure you want to delete <strong>{deleteTarget?.name}</strong>?
        {deleteTarget?.kind === 'ClusterUserDefinedNetwork' && (
          <> This will remove the NetworkAttachmentDefinition from all targeted namespaces.</>
        )}
      </Modal>
    </VPCNetworkingShell>
  );
};

NetworksListPage.displayName = 'NetworksListPage';
export default NetworksListPage;
