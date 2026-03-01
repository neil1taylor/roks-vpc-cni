import React, { useState } from 'react';
import { useParams, useSearchParams } from 'react-router-dom-v5-compat';
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
  Alert,
  Split,
  SplitItem,
  Label,
} from '@patternfly/react-core';
import { Table, Thead, Tr, Th, Tbody, Td } from '@patternfly/react-table';
import { Link, useNavigate } from 'react-router-dom-v5-compat';
import { useRouter, useGateway } from '../api/hooks';
import { apiClient } from '../api/client';
import { StatusBadge } from '../components/StatusBadge';
import { DeleteConfirmModal } from '../components/DeleteConfirmModal';
import { formatTimestamp } from '../utils/formatters';
import VPCNetworkingShell from '../components/VPCNetworkingShell';

const RouterDetailPage: React.FC = () => {
  const { name } = useParams<{ name: string }>();
  const [searchParams] = useSearchParams();
  const ns = searchParams.get('ns') || undefined;
  const { router, loading } = useRouter(name || '', ns);
  const { gateway: gatewayDetail } = useGateway(router?.gateway || '', router?.namespace);
  const navigate = useNavigate();

  const [isDeleteModalOpen, setIsDeleteModalOpen] = useState(false);
  const [actionLoading, setActionLoading] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);

  const handleDelete = async () => {
    if (!name) return;
    setActionLoading(true);
    setActionError(null);
    const resp = await apiClient.deleteRouter(name, ns);
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
            {actionError && (
              <Alert variant="danger" title={actionError} isInline style={{ marginBottom: '1rem' }} />
            )}

            <Card style={{ marginBottom: '24px' }}>
              <CardTitle>
                <Split hasGutter>
                  <SplitItem isFilled>Overview</SplitItem>
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
              </CardTitle>
              <CardBody>
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
                      <Link to={`/vpc-networking/gateways/${router.gateway}?ns=${encodeURIComponent(router.namespace)}`}>{router.gateway}</Link>
                      {gatewayDetail && (
                        <>
                          {' '}
                          <Label isCompact color="blue">{gatewayDetail.zone}</Label>
                          {' '}
                          <StatusBadge status={gatewayDetail.phase} />
                        </>
                      )}
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

      <DeleteConfirmModal
        isOpen={isDeleteModalOpen}
        title="Delete Router"
        message={`Deleting router "${router?.name}" will remove the router pod and disconnect all attached networks from the gateway. VMs on those networks will lose external connectivity. This action cannot be undone.`}
        resourceName={router?.name}
        onConfirm={handleDelete}
        onCancel={() => { setIsDeleteModalOpen(false); setActionError(null); }}
        isLoading={actionLoading}
      />
    </VPCNetworkingShell>
  );
};

export default RouterDetailPage;
