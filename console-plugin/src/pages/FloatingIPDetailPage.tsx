import React, { useState, useCallback } from 'react';
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
  FormGroup,
  FormSelect,
  FormSelectOption,
  Alert,
  Split,
  SplitItem,
} from '@patternfly/react-core';
import { Link } from 'react-router-dom-v5-compat';
import { useFloatingIP, useVNIs } from '../api/hooks';
import { apiClient } from '../api/client';
import { StatusBadge } from '../components/StatusBadge';
import { formatTimestamp } from '../utils/formatters';
import VPCNetworkingShell from '../components/VPCNetworkingShell';
import { FloatingIP } from '../api/types';

const FloatingIPDetailPage: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const [fipData, setFipData] = useState<FloatingIP | null>(null);
  const { floatingIp: initialFip, loading } = useFloatingIP(id || '');
  const floatingIp = fipData || initialFip;

  const [isAttachModalOpen, setIsAttachModalOpen] = useState(false);
  const [selectedVNI, setSelectedVNI] = useState('');
  const [actionLoading, setActionLoading] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);

  const { vnis, loading: vnisLoading } = useVNIs();

  const refetch = useCallback(async () => {
    if (!id) return;
    const resp = await apiClient.getFloatingIP(id);
    if (resp.data) {
      setFipData(resp.data);
    }
  }, [id]);

  const handleAttach = async () => {
    if (!id || !selectedVNI) return;
    setActionLoading(true);
    setActionError(null);
    const resp = await apiClient.updateFloatingIP(id, { target_id: selectedVNI });
    if (resp.error) {
      setActionError(resp.error.message);
    } else {
      setIsAttachModalOpen(false);
      setSelectedVNI('');
      await refetch();
    }
    setActionLoading(false);
  };

  const handleDetach = async () => {
    if (!id) return;
    setActionLoading(true);
    setActionError(null);
    const resp = await apiClient.updateFloatingIP(id, { target_id: '' });
    if (resp.error) {
      setActionError(resp.error.message);
    } else {
      await refetch();
    }
    setActionLoading(false);
  };

  const isBound = !!floatingIp?.target?.id;

  return (
    <VPCNetworkingShell>
      <PageSection variant={PageSectionVariants.light}>
        <Breadcrumb>
          <BreadcrumbItem><Link to="/vpc-networking/floating-ips">Floating IPs</Link></BreadcrumbItem>
          <BreadcrumbItem isActive>{floatingIp?.name || id}</BreadcrumbItem>
        </Breadcrumb>
      </PageSection>

      <PageSection>
        {loading ? (
          <Spinner size="lg" />
        ) : floatingIp ? (
          <Card>
            <CardBody>
              {actionError && (
                <Alert variant="danger" title={actionError} isInline style={{ marginBottom: '1rem' }} />
              )}
              <Split hasGutter style={{ marginBottom: '1rem' }}>
                <SplitItem isFilled />
                <SplitItem>
                  {isBound ? (
                    <Button
                      variant="secondary"
                      onClick={handleDetach}
                      isLoading={actionLoading}
                      isDisabled={actionLoading}
                    >
                      Detach from VNI
                    </Button>
                  ) : (
                    <Button
                      variant="primary"
                      onClick={() => { setActionError(null); setIsAttachModalOpen(true); }}
                      isDisabled={actionLoading}
                    >
                      Attach to VNI
                    </Button>
                  )}
                </SplitItem>
              </Split>
              <DescriptionList>
                <DescriptionListGroup>
                  <DescriptionListTerm>ID</DescriptionListTerm>
                  <DescriptionListDescription>{floatingIp.id}</DescriptionListDescription>
                </DescriptionListGroup>
                <DescriptionListGroup>
                  <DescriptionListTerm>Name</DescriptionListTerm>
                  <DescriptionListDescription>{floatingIp.name || '-'}</DescriptionListDescription>
                </DescriptionListGroup>
                <DescriptionListGroup>
                  <DescriptionListTerm>Address</DescriptionListTerm>
                  <DescriptionListDescription>{floatingIp.address}</DescriptionListDescription>
                </DescriptionListGroup>
                <DescriptionListGroup>
                  <DescriptionListTerm>Zone</DescriptionListTerm>
                  <DescriptionListDescription>
                    {floatingIp.zone?.name || floatingIp.zone?.id || '-'}
                  </DescriptionListDescription>
                </DescriptionListGroup>
                <DescriptionListGroup>
                  <DescriptionListTerm>Target</DescriptionListTerm>
                  <DescriptionListDescription>
                    {floatingIp.target ? (
                      <Link to={`/vpc-networking/vnis/${floatingIp.target.id}`}>
                        {floatingIp.target.name || floatingIp.target.id}
                      </Link>
                    ) : (
                      'Unbound'
                    )}
                  </DescriptionListDescription>
                </DescriptionListGroup>
                <DescriptionListGroup>
                  <DescriptionListTerm>Status</DescriptionListTerm>
                  <DescriptionListDescription>
                    <StatusBadge status={floatingIp.status} />
                  </DescriptionListDescription>
                </DescriptionListGroup>
                <DescriptionListGroup>
                  <DescriptionListTerm>Created</DescriptionListTerm>
                  <DescriptionListDescription>
                    {formatTimestamp(floatingIp.createdAt)}
                  </DescriptionListDescription>
                </DescriptionListGroup>
              </DescriptionList>
            </CardBody>
          </Card>
        ) : (
          <Card>
            <CardBody>Floating IP not found</CardBody>
          </Card>
        )}
      </PageSection>

      <Modal
        title="Attach Floating IP to VNI"
        variant={ModalVariant.small}
        isOpen={isAttachModalOpen}
        onClose={() => { setIsAttachModalOpen(false); setSelectedVNI(''); setActionError(null); }}
        actions={[
          <Button
            key="attach"
            variant="primary"
            onClick={handleAttach}
            isLoading={actionLoading}
            isDisabled={actionLoading || !selectedVNI}
          >
            Attach
          </Button>,
          <Button
            key="cancel"
            variant="link"
            onClick={() => { setIsAttachModalOpen(false); setSelectedVNI(''); setActionError(null); }}
          >
            Cancel
          </Button>,
        ]}
      >
        {actionError && (
          <Alert variant="danger" title={actionError} isInline style={{ marginBottom: '1rem' }} />
        )}
        <FormGroup label="Virtual Network Interface" fieldId="vni-select">
          {vnisLoading ? (
            <Spinner size="md" />
          ) : (
            <FormSelect
              id="vni-select"
              value={selectedVNI}
              onChange={(_event, value) => setSelectedVNI(value)}
            >
              <FormSelectOption key="" value="" label="Select a VNI..." isPlaceholder />
              {(vnis || []).map((vni) => (
                <FormSelectOption
                  key={vni.id}
                  value={vni.id}
                  label={`${vni.name || vni.id}${vni.primaryIp?.address ? ` (${vni.primaryIp.address})` : ''}`}
                />
              ))}
            </FormSelect>
          )}
        </FormGroup>
      </Modal>
    </VPCNetworkingShell>
  );
};

export default FloatingIPDetailPage;
