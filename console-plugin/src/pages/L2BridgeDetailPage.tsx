import React, { useState } from 'react';
import { useParams, useSearchParams } from 'react-router-dom-v5-compat';
import {
  PageSection,
  PageSectionVariants,
  Card,
  CardBody,
  CardTitle,
  Grid,
  GridItem,
  Breadcrumb,
  BreadcrumbItem,
  DescriptionList,
  DescriptionListGroup,
  DescriptionListTerm,
  DescriptionListDescription,
  Spinner,
  Button,
  Alert,
  Split,
  SplitItem,
  EmptyState,
  EmptyStateBody,
  EmptyStateHeader,
  EmptyStateIcon,
  Label,
} from '@patternfly/react-core';
import { CubesIcon } from '@patternfly/react-icons';
import { Link, useNavigate } from 'react-router-dom-v5-compat';
import { useL2Bridge } from '../api/hooks';
import { apiClient } from '../api/client';
import { StatusBadge } from '../components/StatusBadge';
import { DeleteConfirmModal } from '../components/DeleteConfirmModal';
import { formatTimestamp, formatBytes } from '../utils/formatters';
import VPCNetworkingShell from '../components/VPCNetworkingShell';

const typeColors: Record<string, 'blue' | 'purple' | 'cyan'> = {
  'gretap-wireguard': 'blue',
  'l2vpn': 'purple',
  'evpn-vxlan': 'cyan',
};

const typeNotes: Record<string, string> = {
  'gretap-wireguard': 'WireGuard encrypted tunnel',
  'l2vpn': 'NSX-T L2VPN tunnel',
  'evpn-vxlan': 'EVPN-VXLAN with FRRouting',
};

const L2BridgeDetailPage: React.FC = () => {
  const { name } = useParams<{ name: string }>();
  const [searchParams] = useSearchParams();
  const ns = searchParams.get('ns') || undefined;
  const { l2bridge, loading } = useL2Bridge(name || '', ns);
  const navigate = useNavigate();

  const [isDeleteModalOpen, setIsDeleteModalOpen] = useState(false);
  const [actionLoading, setActionLoading] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);

  const handleDelete = async () => {
    if (!name) return;
    setActionLoading(true);
    setActionError(null);
    const resp = await apiClient.deleteL2Bridge(name, ns);
    if (resp.error) {
      setActionError(resp.error.message);
      setActionLoading(false);
    } else {
      setIsDeleteModalOpen(false);
      navigate('/vpc-networking/l2-bridges');
    }
  };

  return (
    <VPCNetworkingShell>
      <PageSection variant={PageSectionVariants.light}>
        <Breadcrumb>
          <BreadcrumbItem><Link to="/vpc-networking/l2-bridges">L2 Bridges</Link></BreadcrumbItem>
          <BreadcrumbItem isActive>{l2bridge?.name || name}</BreadcrumbItem>
        </Breadcrumb>
      </PageSection>

      <PageSection>
        {loading ? (
          <Spinner size="lg" />
        ) : l2bridge ? (
          <>
            {actionError && (
              <Alert variant="danger" title={actionError} isInline style={{ marginBottom: '1rem' }} />
            )}

            <Grid hasGutter lg={6}>
              {/* Card 1: Overview */}
              <GridItem>
                <Card isFullHeight>
                  <CardTitle>
                    <Split hasGutter>
                      <SplitItem isFilled>Overview</SplitItem>
                      <SplitItem>
                        <Button
                          variant="danger"
                          onClick={() => { setActionError(null); setIsDeleteModalOpen(true); }}
                          isDisabled={actionLoading}
                        >
                          Delete
                        </Button>
                      </SplitItem>
                    </Split>
                  </CardTitle>
                  <CardBody>
                    <DescriptionList>
                      <DescriptionListGroup>
                        <DescriptionListTerm>Name</DescriptionListTerm>
                        <DescriptionListDescription>{l2bridge.name || '-'}</DescriptionListDescription>
                      </DescriptionListGroup>
                      <DescriptionListGroup>
                        <DescriptionListTerm>Namespace</DescriptionListTerm>
                        <DescriptionListDescription>{l2bridge.namespace || '-'}</DescriptionListDescription>
                      </DescriptionListGroup>
                      <DescriptionListGroup>
                        <DescriptionListTerm>Type</DescriptionListTerm>
                        <DescriptionListDescription>
                          <Label color={typeColors[l2bridge.type] || 'grey'}>{l2bridge.type}</Label>
                        </DescriptionListDescription>
                      </DescriptionListGroup>
                      <DescriptionListGroup>
                        <DescriptionListTerm>Phase</DescriptionListTerm>
                        <DescriptionListDescription>
                          <StatusBadge status={l2bridge.phase} />
                        </DescriptionListDescription>
                      </DescriptionListGroup>
                      <DescriptionListGroup>
                        <DescriptionListTerm>Sync Status</DescriptionListTerm>
                        <DescriptionListDescription>
                          <StatusBadge status={l2bridge.syncStatus} />
                        </DescriptionListDescription>
                      </DescriptionListGroup>
                      <DescriptionListGroup>
                        <DescriptionListTerm>Created</DescriptionListTerm>
                        <DescriptionListDescription>
                          {formatTimestamp(l2bridge.createdAt)}
                        </DescriptionListDescription>
                      </DescriptionListGroup>
                    </DescriptionList>
                  </CardBody>
                </Card>
              </GridItem>

              {/* Card 2: Tunnel */}
              <GridItem>
                <Card isFullHeight>
                  <CardTitle>Tunnel</CardTitle>
                  <CardBody>
                    <DescriptionList>
                      <DescriptionListGroup>
                        <DescriptionListTerm>Remote Endpoint</DescriptionListTerm>
                        <DescriptionListDescription>{l2bridge.remoteEndpoint || '-'}</DescriptionListDescription>
                      </DescriptionListGroup>
                      <DescriptionListGroup>
                        <DescriptionListTerm>Tunnel Endpoint</DescriptionListTerm>
                        <DescriptionListDescription>{l2bridge.tunnelEndpoint || '-'}</DescriptionListDescription>
                      </DescriptionListGroup>
                      <DescriptionListGroup>
                        <DescriptionListTerm>Last Handshake</DescriptionListTerm>
                        <DescriptionListDescription>
                          {l2bridge.lastHandshake ? formatTimestamp(l2bridge.lastHandshake) : '-'}
                        </DescriptionListDescription>
                      </DescriptionListGroup>
                      <DescriptionListGroup>
                        <DescriptionListTerm>Tunnel MTU</DescriptionListTerm>
                        <DescriptionListDescription>{l2bridge.tunnelMTU}</DescriptionListDescription>
                      </DescriptionListGroup>
                      <DescriptionListGroup>
                        <DescriptionListTerm>MSS Clamping</DescriptionListTerm>
                        <DescriptionListDescription>{l2bridge.mssClamp ? 'Yes' : 'No'}</DescriptionListDescription>
                      </DescriptionListGroup>
                      <DescriptionListGroup>
                        <DescriptionListTerm>Tunnel Technology</DescriptionListTerm>
                        <DescriptionListDescription>
                          {typeNotes[l2bridge.type] || l2bridge.type}
                        </DescriptionListDescription>
                      </DescriptionListGroup>
                    </DescriptionList>
                  </CardBody>
                </Card>
              </GridItem>

              {/* Card 3: Network */}
              <GridItem>
                <Card isFullHeight>
                  <CardTitle>Network</CardTitle>
                  <CardBody>
                    <DescriptionList>
                      <DescriptionListGroup>
                        <DescriptionListTerm>Bridged Network</DescriptionListTerm>
                        <DescriptionListDescription>
                          {l2bridge.networkRef?.name ? (
                            <Link to={`/vpc-networking/networks/${l2bridge.networkRef.name}`}>
                              {l2bridge.networkRef.name}
                            </Link>
                          ) : '-'}
                        </DescriptionListDescription>
                      </DescriptionListGroup>
                      <DescriptionListGroup>
                        <DescriptionListTerm>Gateway</DescriptionListTerm>
                        <DescriptionListDescription>
                          {l2bridge.gatewayRef ? (
                            <Link to={`/vpc-networking/gateways/${l2bridge.gatewayRef}${ns ? `?ns=${encodeURIComponent(ns)}` : ''}`}>
                              {l2bridge.gatewayRef}
                            </Link>
                          ) : '-'}
                        </DescriptionListDescription>
                      </DescriptionListGroup>
                      <DescriptionListGroup>
                        <DescriptionListTerm>Remote MACs Learned</DescriptionListTerm>
                        <DescriptionListDescription>{l2bridge.remoteMACsLearned}</DescriptionListDescription>
                      </DescriptionListGroup>
                      <DescriptionListGroup>
                        <DescriptionListTerm>Local MACs Advertised</DescriptionListTerm>
                        <DescriptionListDescription>{l2bridge.localMACsAdvertised}</DescriptionListDescription>
                      </DescriptionListGroup>
                    </DescriptionList>
                  </CardBody>
                </Card>
              </GridItem>

              {/* Card 4: Throughput */}
              <GridItem>
                <Card isFullHeight>
                  <CardTitle>Throughput</CardTitle>
                  <CardBody>
                    <DescriptionList>
                      <DescriptionListGroup>
                        <DescriptionListTerm>Bytes In</DescriptionListTerm>
                        <DescriptionListDescription>{formatBytes(l2bridge.bytesIn)}</DescriptionListDescription>
                      </DescriptionListGroup>
                      <DescriptionListGroup>
                        <DescriptionListTerm>Bytes Out</DescriptionListTerm>
                        <DescriptionListDescription>{formatBytes(l2bridge.bytesOut)}</DescriptionListDescription>
                      </DescriptionListGroup>
                      <DescriptionListGroup>
                        <DescriptionListTerm>Pod Name</DescriptionListTerm>
                        <DescriptionListDescription>{l2bridge.podName || '-'}</DescriptionListDescription>
                      </DescriptionListGroup>
                    </DescriptionList>
                  </CardBody>
                </Card>
              </GridItem>
            </Grid>
          </>
        ) : (
          <EmptyState>
            <EmptyStateHeader
              titleText="L2 Bridge not found"
              icon={<EmptyStateIcon icon={CubesIcon} />}
            />
            <EmptyStateBody>
              The L2 bridge &quot;{name}&quot; could not be found. It may have been deleted or the name is incorrect.
            </EmptyStateBody>
          </EmptyState>
        )}
      </PageSection>

      <DeleteConfirmModal
        isOpen={isDeleteModalOpen}
        title="Delete L2 Bridge"
        message={`Deleting L2 bridge "${l2bridge?.name}" will tear down the encrypted tunnel and disconnect the remote network segment. Traffic will stop flowing between sites. This action cannot be undone.`}
        resourceName={l2bridge?.name}
        onConfirm={handleDelete}
        onCancel={() => { setIsDeleteModalOpen(false); setActionError(null); }}
        isLoading={actionLoading}
      />
    </VPCNetworkingShell>
  );
};

L2BridgeDetailPage.displayName = 'L2BridgeDetailPage';
export default L2BridgeDetailPage;
