import React, { useState } from 'react';
import { useParams } from 'react-router-dom-v5-compat';
import {
  PageSection,
  PageSectionVariants,
  Card,
  CardBody,
  CardTitle,
  Breadcrumb,
  BreadcrumbItem,
  Spinner,
  DescriptionList,
  DescriptionListGroup,
  DescriptionListTerm,
  DescriptionListDescription,
  Button,
  Modal,
  ModalVariant,
  Alert,
  Split,
  SplitItem,
  Label,
} from '@patternfly/react-core';
import { Table, Thead, Tr, Th, Tbody, Td } from '@patternfly/react-table';
import { Link, useNavigate } from 'react-router-dom-v5-compat';
import { useRouter } from '../api/hooks';
import { apiClient } from '../api/client';
import { StatusBadge } from '../components/StatusBadge';
import { formatTimestamp } from '../utils/formatters';
import VPCNetworkingShell from '../components/VPCNetworkingShell';

const RouterDetailPage: React.FC = () => {
  const { name } = useParams<{ name: string }>();
  const { router, loading } = useRouter(name || '');
  const navigate = useNavigate();

  const [isDeleteModalOpen, setIsDeleteModalOpen] = useState(false);
  const [actionLoading, setActionLoading] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);

  const handleDelete = async () => {
    if (!name) return;
    setActionLoading(true);
    setActionError(null);
    const resp = await apiClient.deleteRouter(name);
    if (resp.error) {
      setActionError(resp.error.message);
      setActionLoading(false);
    } else {
      setIsDeleteModalOpen(false);
      navigate('/vpc-networking/routers');
    }
  };

  return (
    <VPCNetworkingShell>
      <PageSection variant={PageSectionVariants.light}>
        <Breadcrumb>
          <BreadcrumbItem><Link to="/vpc-networking/routers">Routers</Link></BreadcrumbItem>
          <BreadcrumbItem isActive>{router?.name || name}</BreadcrumbItem>
        </Breadcrumb>
      </PageSection>

      <PageSection>
        {loading ? (
          <Spinner size="lg" />
        ) : router ? (
          <>
            <Card style={{ marginBottom: '24px' }}>
              <CardBody>
                {actionError && (
                  <Alert variant="danger" title={actionError} isInline style={{ marginBottom: '1rem' }} />
                )}
                <Split hasGutter style={{ marginBottom: '1rem' }}>
                  <SplitItem isFilled />
                  <SplitItem>
                    <Button
                      variant="danger"
                      onClick={() => { setActionError(null); setIsDeleteModalOpen(true); }}
                      isDisabled={actionLoading}
                    >
                      Delete Router
                    </Button>
                  </SplitItem>
                </Split>
                <DescriptionList>
                  <DescriptionListGroup>
                    <DescriptionListTerm>Name</DescriptionListTerm>
                    <DescriptionListDescription>{router.name || '-'}</DescriptionListDescription>
                  </DescriptionListGroup>
                  <DescriptionListGroup>
                    <DescriptionListTerm>Namespace</DescriptionListTerm>
                    <DescriptionListDescription>{router.namespace || '-'}</DescriptionListDescription>
                  </DescriptionListGroup>
                  <DescriptionListGroup>
                    <DescriptionListTerm>Gateway</DescriptionListTerm>
                    <DescriptionListDescription>
                      <Link to={`/vpc-networking/gateways/${router.gateway}`}>{router.gateway}</Link>
                    </DescriptionListDescription>
                  </DescriptionListGroup>
                  <DescriptionListGroup>
                    <DescriptionListTerm>Phase</DescriptionListTerm>
                    <DescriptionListDescription>
                      <StatusBadge status={router.phase} />
                    </DescriptionListDescription>
                  </DescriptionListGroup>
                  <DescriptionListGroup>
                    <DescriptionListTerm>Transit IP</DescriptionListTerm>
                    <DescriptionListDescription>{router.transitIP || '-'}</DescriptionListDescription>
                  </DescriptionListGroup>
                  <DescriptionListGroup>
                    <DescriptionListTerm>Advertised Routes</DescriptionListTerm>
                    <DescriptionListDescription>
                      {router.advertisedRoutes && router.advertisedRoutes.length > 0
                        ? router.advertisedRoutes.join(', ')
                        : '-'}
                    </DescriptionListDescription>
                  </DescriptionListGroup>
                  <DescriptionListGroup>
                    <DescriptionListTerm>Functions</DescriptionListTerm>
                    <DescriptionListDescription>
                      {router.functions && router.functions.length > 0
                        ? router.functions.join(', ')
                        : '-'}
                    </DescriptionListDescription>
                  </DescriptionListGroup>
                  <DescriptionListGroup>
                    <DescriptionListTerm>Sync Status</DescriptionListTerm>
                    <DescriptionListDescription>
                      <StatusBadge status={router.syncStatus} />
                    </DescriptionListDescription>
                  </DescriptionListGroup>
                  <DescriptionListGroup>
                    <DescriptionListTerm>Created</DescriptionListTerm>
                    <DescriptionListDescription>
                      {formatTimestamp(router.createdAt)}
                    </DescriptionListDescription>
                  </DescriptionListGroup>
                </DescriptionList>
              </CardBody>
            </Card>

            <Card>
              <CardTitle>Connected Networks</CardTitle>
              <CardBody>
                {router.networks && router.networks.length > 0 ? (
                  <Table aria-label="Connected networks" variant="compact">
                    <Thead>
                      <Tr>
                        <Th>Name</Th>
                        <Th>Address</Th>
                        <Th>Connected</Th>
                      </Tr>
                    </Thead>
                    <Tbody>
                      {router.networks.map((net) => (
                        <Tr key={net.name}>
                          <Td>{net.name}</Td>
                          <Td>{net.address}</Td>
                          <Td>
                            <Label color={net.connected ? 'green' : 'red'}>
                              {net.connected ? 'Connected' : 'Disconnected'}
                            </Label>
                          </Td>
                        </Tr>
                      ))}
                    </Tbody>
                  </Table>
                ) : (
                  <span>No networks connected</span>
                )}
              </CardBody>
            </Card>
          </>
        ) : (
          <Card>
            <CardBody>Router not found</CardBody>
          </Card>
        )}
      </PageSection>

      <Modal
        title="Delete Router"
        variant={ModalVariant.small}
        isOpen={isDeleteModalOpen}
        onClose={() => { setIsDeleteModalOpen(false); setActionError(null); }}
        actions={[
          <Button
            key="delete"
            variant="danger"
            onClick={handleDelete}
            isLoading={actionLoading}
            isDisabled={actionLoading}
          >
            Delete
          </Button>,
          <Button
            key="cancel"
            variant="link"
            onClick={() => { setIsDeleteModalOpen(false); setActionError(null); }}
          >
            Cancel
          </Button>,
        ]}
      >
        {actionError && (
          <Alert variant="danger" title={actionError} isInline style={{ marginBottom: '1rem' }} />
        )}
        Are you sure you want to delete router <strong>{router?.name}</strong>? This action cannot be undone.
      </Modal>
    </VPCNetworkingShell>
  );
};

export default RouterDetailPage;
