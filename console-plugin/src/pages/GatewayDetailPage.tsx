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
import { useGateway } from '../api/hooks';
import { apiClient } from '../api/client';
import { StatusBadge } from '../components/StatusBadge';
import { formatTimestamp } from '../utils/formatters';
import VPCNetworkingShell from '../components/VPCNetworkingShell';

const GatewayDetailPage: React.FC = () => {
  const { name } = useParams<{ name: string }>();
  const { gateway, loading } = useGateway(name || '');
  const navigate = useNavigate();

  const [isDeleteModalOpen, setIsDeleteModalOpen] = useState(false);
  const [actionLoading, setActionLoading] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);

  const handleDelete = async () => {
    if (!name) return;
    setActionLoading(true);
    setActionError(null);
    const resp = await apiClient.deleteGateway(name);
    if (resp.error) {
      setActionError(resp.error.message);
      setActionLoading(false);
    } else {
      setIsDeleteModalOpen(false);
      navigate('/vpc-networking/gateways');
    }
  };

  return (
    <VPCNetworkingShell>
      <PageSection variant={PageSectionVariants.light}>
        <Breadcrumb>
          <BreadcrumbItem><Link to="/vpc-networking/gateways">Gateways</Link></BreadcrumbItem>
          <BreadcrumbItem isActive>{gateway?.name || name}</BreadcrumbItem>
        </Breadcrumb>
      </PageSection>

      <PageSection>
        {loading ? (
          <Spinner size="lg" />
        ) : gateway ? (
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
                    isDisabled={actionLoading}
                  >
                    Delete Gateway
                  </Button>
                </SplitItem>
              </Split>
              <DescriptionList>
                <DescriptionListGroup>
                  <DescriptionListTerm>Name</DescriptionListTerm>
                  <DescriptionListDescription>{gateway.name || '-'}</DescriptionListDescription>
                </DescriptionListGroup>
                <DescriptionListGroup>
                  <DescriptionListTerm>Namespace</DescriptionListTerm>
                  <DescriptionListDescription>{gateway.namespace || '-'}</DescriptionListDescription>
                </DescriptionListGroup>
                <DescriptionListGroup>
                  <DescriptionListTerm>Zone</DescriptionListTerm>
                  <DescriptionListDescription>{gateway.zone || '-'}</DescriptionListDescription>
                </DescriptionListGroup>
                <DescriptionListGroup>
                  <DescriptionListTerm>Phase</DescriptionListTerm>
                  <DescriptionListDescription>
                    <StatusBadge status={gateway.phase} />
                  </DescriptionListDescription>
                </DescriptionListGroup>
                <DescriptionListGroup>
                  <DescriptionListTerm>Uplink Network</DescriptionListTerm>
                  <DescriptionListDescription>{gateway.uplinkNetwork || '-'}</DescriptionListDescription>
                </DescriptionListGroup>
                <DescriptionListGroup>
                  <DescriptionListTerm>Transit Network</DescriptionListTerm>
                  <DescriptionListDescription>{gateway.transitNetwork || '-'}</DescriptionListDescription>
                </DescriptionListGroup>
                <DescriptionListGroup>
                  <DescriptionListTerm>VNI ID</DescriptionListTerm>
                  <DescriptionListDescription>{gateway.vniID || '-'}</DescriptionListDescription>
                </DescriptionListGroup>
                <DescriptionListGroup>
                  <DescriptionListTerm>VNI IP</DescriptionListTerm>
                  <DescriptionListDescription>{gateway.reservedIP || '-'}</DescriptionListDescription>
                </DescriptionListGroup>
                <DescriptionListGroup>
                  <DescriptionListTerm>Floating IP</DescriptionListTerm>
                  <DescriptionListDescription>{gateway.floatingIP || '-'}</DescriptionListDescription>
                </DescriptionListGroup>
                <DescriptionListGroup>
                  <DescriptionListTerm>VPC Routes</DescriptionListTerm>
                  <DescriptionListDescription>{gateway.vpcRouteCount}</DescriptionListDescription>
                </DescriptionListGroup>
                <DescriptionListGroup>
                  <DescriptionListTerm>NAT Rules</DescriptionListTerm>
                  <DescriptionListDescription>{gateway.natRuleCount}</DescriptionListDescription>
                </DescriptionListGroup>
                <DescriptionListGroup>
                  <DescriptionListTerm>Sync Status</DescriptionListTerm>
                  <DescriptionListDescription>
                    <StatusBadge status={gateway.syncStatus} />
                  </DescriptionListDescription>
                </DescriptionListGroup>
                <DescriptionListGroup>
                  <DescriptionListTerm>Created</DescriptionListTerm>
                  <DescriptionListDescription>
                    {formatTimestamp(gateway.createdAt)}
                  </DescriptionListDescription>
                </DescriptionListGroup>
              </DescriptionList>
            </CardBody>
          </Card>
        ) : (
          <Card>
            <CardBody>Gateway not found</CardBody>
          </Card>
        )}
      </PageSection>

      <Modal
        title="Delete Gateway"
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
        Are you sure you want to delete gateway <strong>{gateway?.name}</strong>? This action cannot be undone.
      </Modal>
    </VPCNetworkingShell>
  );
};

export default GatewayDetailPage;
