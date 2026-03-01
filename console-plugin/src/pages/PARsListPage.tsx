import React, { useState, useCallback } from 'react';
import {
  PageSection,
  Card,
  CardBody,
  Button,
  ButtonVariant,
  Toolbar,
  ToolbarContent,
  ToolbarItem,
  Spinner,
  Text,
  TextVariants,
  Modal,
  ModalVariant,
  Form,
  FormGroup,
  TextInput,
  FormSelect,
  FormSelectOption,
  Alert,
} from '@patternfly/react-core';
import { PlusCircleIcon, TrashIcon } from '@patternfly/react-icons';
import { Table, Thead, Tbody, Tr, Th, Td } from '@patternfly/react-table';
import { Link } from 'react-router-dom-v5-compat';
import { usePARs, useZones } from '../api/hooks';
import { apiClient } from '../api/client';
import StatusBadge from '../components/StatusBadge';
import VPCNetworkingShell from '../components/VPCNetworkingShell';
import { formatTimestamp } from '../utils/formatters';

const PARsListPage: React.FC = () => {
  const { pars, loading } = usePARs();
  const { zones } = useZones();
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [isDeleteOpen, setIsDeleteOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<{ id: string; name: string } | null>(null);

  // Create form state
  const [createName, setCreateName] = useState('');
  const [createZone, setCreateZone] = useState('');
  const [createPrefixLength, setCreatePrefixLength] = useState('32');
  const [createLoading, setCreateLoading] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);

  // Delete state
  const [deleteLoading, setDeleteLoading] = useState(false);
  const [deleteError, setDeleteError] = useState<string | null>(null);

  const handleCreate = useCallback(async () => {
    if (!createZone) return;
    setCreateLoading(true);
    setCreateError(null);

    const resp = await apiClient.createPAR({
      name: createName.trim() || undefined as unknown as string,
      zone: createZone,
      prefixLength: parseInt(createPrefixLength, 10),
    });

    if (resp.error) {
      setCreateError(resp.error.message);
      setCreateLoading(false);
    } else {
      setIsCreateOpen(false);
      setCreateName('');
      setCreateZone('');
      setCreatePrefixLength('32');
      setCreateLoading(false);
      window.location.reload();
    }
  }, [createName, createZone, createPrefixLength]);

  const handleDelete = useCallback(async () => {
    if (!deleteTarget) return;
    setDeleteLoading(true);
    setDeleteError(null);

    const resp = await apiClient.deletePAR(deleteTarget.id);
    if (resp.error) {
      setDeleteError(resp.error.message);
      setDeleteLoading(false);
    } else {
      setIsDeleteOpen(false);
      setDeleteTarget(null);
      setDeleteLoading(false);
      window.location.reload();
    }
  }, [deleteTarget]);

  const openDelete = (id: string, name: string) => {
    setDeleteTarget({ id, name });
    setDeleteError(null);
    setIsDeleteOpen(true);
  };

  return (
    <VPCNetworkingShell>
      <PageSection>
        <Text component={TextVariants.p} style={{ marginBottom: '16px', color: 'var(--pf-v5-global--Color--200)' }}>
          Public address ranges provide blocks of public IPs routed to a gateway&apos;s uplink VNI.
        </Text>
        <Card>
          <Toolbar>
            <ToolbarContent>
              <ToolbarItem>
                <Button
                  variant={ButtonVariant.primary}
                  icon={<PlusCircleIcon />}
                  onClick={() => { setCreateError(null); setIsCreateOpen(true); }}
                >
                  Create PAR
                </Button>
              </ToolbarItem>
            </ToolbarContent>
          </Toolbar>
          <CardBody>
            {loading ? (
              <div style={{ textAlign: 'center', padding: '40px' }}><Spinner size="lg" /></div>
            ) : !pars?.length ? (
              <div style={{ textAlign: 'center', padding: '40px' }}>No public address ranges found</div>
            ) : (
              <Table aria-label="Public Address Ranges table" borders>
                <Thead>
                  <Tr>
                    <Th>Name</Th>
                    <Th>CIDR</Th>
                    <Th>Zone</Th>
                    <Th>Gateway</Th>
                    <Th>State</Th>
                    <Th>Created</Th>
                    <Th />
                  </Tr>
                </Thead>
                <Tbody>
                  {pars.map((par) => (
                    <Tr key={par.id}>
                      <Td>
                        <Link to={`/vpc-networking/pars/${par.id}`}>{par.name || par.id}</Link>
                      </Td>
                      <Td>{par.cidr || '-'}</Td>
                      <Td>{par.zone || '-'}</Td>
                      <Td>
                        {par.gatewayName ? (
                          <Link to={`/vpc-networking/gateways/${par.gatewayName}?ns=${par.gatewayNamespace}`}>
                            {par.gatewayName}
                          </Link>
                        ) : (
                          'Unattached'
                        )}
                      </Td>
                      <Td><StatusBadge status={par.lifecycleState} /></Td>
                      <Td>{formatTimestamp(par.createdAt)}</Td>
                      <Td>
                        <Button
                          variant="plain"
                          aria-label="Delete"
                          onClick={() => openDelete(par.id, par.name || par.id)}
                          isDisabled={!!par.gatewayName}
                        >
                          <TrashIcon />
                        </Button>
                      </Td>
                    </Tr>
                  ))}
                </Tbody>
              </Table>
            )}
          </CardBody>
        </Card>
      </PageSection>

      {/* Create PAR Modal */}
      <Modal
        title="Create Public Address Range"
        variant={ModalVariant.small}
        isOpen={isCreateOpen}
        onClose={() => { setIsCreateOpen(false); setCreateError(null); }}
        actions={[
          <Button
            key="create"
            variant="primary"
            onClick={handleCreate}
            isLoading={createLoading}
            isDisabled={createLoading || !createZone}
          >
            Create
          </Button>,
          <Button
            key="cancel"
            variant="link"
            onClick={() => { setIsCreateOpen(false); setCreateError(null); }}
          >
            Cancel
          </Button>,
        ]}
      >
        {createError && (
          <Alert variant="danger" title={createError} isInline style={{ marginBottom: '1rem' }} />
        )}
        <Form>
          <FormGroup label="Name" fieldId="par-name">
            <TextInput
              id="par-name"
              value={createName}
              onChange={(_e, v) => setCreateName(v)}
              placeholder="Optional name"
            />
          </FormGroup>
          <FormGroup label="Zone" isRequired fieldId="par-zone">
            <FormSelect
              id="par-zone"
              value={createZone}
              onChange={(_e, v) => setCreateZone(v)}
            >
              <FormSelectOption value="" label="Select a zone" isPlaceholder />
              {zones?.map((z) => (
                <FormSelectOption key={z.name} value={z.name} label={z.name || ''} />
              ))}
            </FormSelect>
          </FormGroup>
          <FormGroup label="Prefix Length" isRequired fieldId="par-prefix">
            <FormSelect
              id="par-prefix"
              value={createPrefixLength}
              onChange={(_e, v) => setCreatePrefixLength(v)}
            >
              <FormSelectOption value="28" label="/28 (16 IPs)" />
              <FormSelectOption value="29" label="/29 (8 IPs)" />
              <FormSelectOption value="30" label="/30 (4 IPs)" />
              <FormSelectOption value="31" label="/31 (2 IPs)" />
              <FormSelectOption value="32" label="/32 (1 IP)" />
            </FormSelect>
          </FormGroup>
        </Form>
      </Modal>

      {/* Delete PAR Modal */}
      <Modal
        title="Delete Public Address Range"
        variant={ModalVariant.small}
        isOpen={isDeleteOpen}
        onClose={() => { setIsDeleteOpen(false); setDeleteError(null); }}
        actions={[
          <Button
            key="delete"
            variant="danger"
            onClick={handleDelete}
            isLoading={deleteLoading}
            isDisabled={deleteLoading}
          >
            Delete
          </Button>,
          <Button
            key="cancel"
            variant="link"
            onClick={() => { setIsDeleteOpen(false); setDeleteError(null); }}
          >
            Cancel
          </Button>,
        ]}
      >
        {deleteError && (
          <Alert variant="danger" title={deleteError} isInline style={{ marginBottom: '1rem' }} />
        )}
        Are you sure you want to delete PAR <strong>{deleteTarget?.name}</strong>? This action cannot be undone.
      </Modal>
    </VPCNetworkingShell>
  );
};

export default PARsListPage;
