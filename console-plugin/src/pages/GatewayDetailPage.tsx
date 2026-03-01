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
} from '@patternfly/react-core';
import { Link, useNavigate } from 'react-router-dom-v5-compat';
import { useGateway } from '../api/hooks';
import { apiClient } from '../api/client';
import { StatusBadge } from '../components/StatusBadge';
import { DeleteConfirmModal } from '../components/DeleteConfirmModal';
import { formatTimestamp } from '../utils/formatters';
import VPCNetworkingShell from '../components/VPCNetworkingShell';

const GatewayDetailPage: React.FC = () => {
  const { name } = useParams<{ name: string }>();
  const [searchParams] = useSearchParams();
  const ns = searchParams.get('ns') || undefined;
  const { gateway, loading } = useGateway(name || '', ns);
  const navigate = useNavigate();

  const [isDeleteModalOpen, setIsDeleteModalOpen] = useState(false);
  const [actionLoading, setActionLoading] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);

  const handleDelete = async () => {
    if (!name) return;
    setActionLoading(true);
    setActionError(null);
    const resp = await apiClient.deleteGateway(name, ns);
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
          <>
            {actionError && (
              <Alert variant="danger" title={actionError} isInline style={{ marginBottom: '1rem' }} />
            )}

            {/* Overview */}
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
                      Delete Gateway
                    </Button>
                  </SplitItem>
                </Split>
              </CardTitle>
              <CardBody>
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

            {/* Networking */}
            <Card style={{ marginBottom: '24px' }}>
              <CardTitle>Networking</CardTitle>
              <CardBody>
                <DescriptionList>
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
                </DescriptionList>
              </CardBody>
            </Card>

            {/* Routing */}
            <Card style={{ marginBottom: '24px' }}>
              <CardTitle>Routing</CardTitle>
              <CardBody>
                <DescriptionList>
                  <DescriptionListGroup>
                    <DescriptionListTerm>VPC Routes</DescriptionListTerm>
                    <DescriptionListDescription>{gateway.vpcRouteCount}</DescriptionListDescription>
                  </DescriptionListGroup>
                  <DescriptionListGroup>
                    <DescriptionListTerm>NAT Rules</DescriptionListTerm>
                    <DescriptionListDescription>{gateway.natRuleCount}</DescriptionListDescription>
                  </DescriptionListGroup>
                </DescriptionList>
              </CardBody>
            </Card>

            {/* Public Address Range */}
            <Card>
              <CardTitle>Public Address Range</CardTitle>
              <CardBody>
                <DescriptionList>
                  <DescriptionListGroup>
                    <DescriptionListTerm>Status</DescriptionListTerm>
                    <DescriptionListDescription>
                      {gateway.parEnabled ? (gateway.publicAddressRangeCIDR || 'Provisioning...') : 'Disabled'}
                    </DescriptionListDescription>
                  </DescriptionListGroup>
                  {gateway.publicAddressRangeID && (
                    <DescriptionListGroup>
                      <DescriptionListTerm>PAR ID</DescriptionListTerm>
                      <DescriptionListDescription>{gateway.publicAddressRangeID}</DescriptionListDescription>
                    </DescriptionListGroup>
                  )}
                  {gateway.publicAddressRangeCIDR && (
                    <DescriptionListGroup>
                      <DescriptionListTerm>PAR CIDR</DescriptionListTerm>
                      <DescriptionListDescription>{gateway.publicAddressRangeCIDR}</DescriptionListDescription>
                    </DescriptionListGroup>
                  )}
                  {gateway.parPrefixLength && (
                    <DescriptionListGroup>
                      <DescriptionListTerm>Prefix Length</DescriptionListTerm>
                      <DescriptionListDescription>/{gateway.parPrefixLength}</DescriptionListDescription>
                    </DescriptionListGroup>
                  )}
                  {gateway.ingressRoutingTableID && (
                    <DescriptionListGroup>
                      <DescriptionListTerm>Ingress Routing Table</DescriptionListTerm>
                      <DescriptionListDescription>{gateway.ingressRoutingTableID}</DescriptionListDescription>
                    </DescriptionListGroup>
                  )}
                </DescriptionList>
              </CardBody>
            </Card>
          </>
        ) : (
          <Card>
            <CardBody>Gateway not found</CardBody>
          </Card>
        )}
      </PageSection>

      <DeleteConfirmModal
        isOpen={isDeleteModalOpen}
        title="Delete Gateway"
        message={`Deleting gateway "${gateway?.name}" will remove its uplink VNI, floating IP, all VPC routes, NAT rules, and any associated Public Address Range. Connected routers will lose their uplink. This action cannot be undone.`}
        resourceName={gateway?.name}
        onConfirm={handleDelete}
        onCancel={() => { setIsDeleteModalOpen(false); setActionError(null); }}
        isLoading={actionLoading}
      />
    </VPCNetworkingShell>
  );
};

export default GatewayDetailPage;
