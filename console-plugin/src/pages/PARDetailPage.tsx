import React, { useState } from 'react';
import { useParams } from 'react-router-dom-v5-compat';
import {
  PageSection,
  PageSectionVariants,
  Card,
  CardBody,
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
} from '@patternfly/react-core';
import { Link, useNavigate } from 'react-router-dom-v5-compat';
import { usePAR } from '../api/hooks';
import { apiClient } from '../api/client';
import { StatusBadge } from '../components/StatusBadge';
import { formatTimestamp } from '../utils/formatters';
import VPCNetworkingShell from '../components/VPCNetworkingShell';

const PARDetailPage: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const { par, loading } = usePAR(id || '');
  const navigate = useNavigate();

  const [isDeleteModalOpen, setIsDeleteModalOpen] = useState(false);
  const [actionLoading, setActionLoading] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);

  const handleDelete = async () => {
    if (!id) return;
    setActionLoading(true);
    setActionError(null);
    const resp = await apiClient.deletePAR(id);
    if (resp.error) {
      setActionError(resp.error.message);
      setActionLoading(false);
    } else {
      setIsDeleteModalOpen(false);
      navigate('/vpc-networking/pars');
    }
  };

  return (
    <VPCNetworkingShell>
      <PageSection variant={PageSectionVariants.light}>
        <Breadcrumb>
          <BreadcrumbItem><Link to="/vpc-networking/pars">Public Address Ranges</Link></BreadcrumbItem>
          <BreadcrumbItem isActive>{par?.name || id}</BreadcrumbItem>
        </Breadcrumb>
      </PageSection>

      <PageSection>
        {loading ? (
          <Spinner size="lg" />
        ) : par ? (
          <Card>
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
                    isDisabled={actionLoading || !!par.gatewayName}
                  >
                    Delete PAR
                  </Button>
                </SplitItem>
              </Split>
              <DescriptionList>
                <DescriptionListGroup>
                  <DescriptionListTerm>ID</DescriptionListTerm>
                  <DescriptionListDescription>{par.id || '-'}</DescriptionListDescription>
                </DescriptionListGroup>
                <DescriptionListGroup>
                  <DescriptionListTerm>Name</DescriptionListTerm>
                  <DescriptionListDescription>{par.name || '-'}</DescriptionListDescription>
                </DescriptionListGroup>
                <DescriptionListGroup>
                  <DescriptionListTerm>CIDR</DescriptionListTerm>
                  <DescriptionListDescription>{par.cidr || '-'}</DescriptionListDescription>
                </DescriptionListGroup>
                <DescriptionListGroup>
                  <DescriptionListTerm>Zone</DescriptionListTerm>
                  <DescriptionListDescription>{par.zone || '-'}</DescriptionListDescription>
                </DescriptionListGroup>
                <DescriptionListGroup>
                  <DescriptionListTerm>State</DescriptionListTerm>
                  <DescriptionListDescription>
                    <StatusBadge status={par.lifecycleState} />
                  </DescriptionListDescription>
                </DescriptionListGroup>
                <DescriptionListGroup>
                  <DescriptionListTerm>Gateway</DescriptionListTerm>
                  <DescriptionListDescription>
                    {par.gatewayName ? (
                      <Link to={`/vpc-networking/gateways/${par.gatewayName}?ns=${par.gatewayNamespace}`}>
                        {par.gatewayName}
                      </Link>
                    ) : (
                      'Unattached'
                    )}
                  </DescriptionListDescription>
                </DescriptionListGroup>
                <DescriptionListGroup>
                  <DescriptionListTerm>Created</DescriptionListTerm>
                  <DescriptionListDescription>
                    {formatTimestamp(par.createdAt)}
                  </DescriptionListDescription>
                </DescriptionListGroup>
              </DescriptionList>
            </CardBody>
          </Card>
        ) : (
          <Card>
            <CardBody>Public address range not found</CardBody>
          </Card>
        )}
      </PageSection>

      <Modal
        title="Delete Public Address Range"
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
        Are you sure you want to delete PAR <strong>{par?.name || id}</strong>? This action cannot be undone.
      </Modal>
    </VPCNetworkingShell>
  );
};

export default PARDetailPage;
