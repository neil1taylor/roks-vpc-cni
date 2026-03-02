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
import { Table, Thead, Tr, Th, Tbody, Td } from '@patternfly/react-table';
import { CubesIcon } from '@patternfly/react-icons';
import { Link, useNavigate } from 'react-router-dom-v5-compat';
import { useVPNGateway } from '../api/hooks';
import { apiClient } from '../api/client';
import { StatusBadge } from '../components/StatusBadge';
import { DeleteConfirmModal } from '../components/DeleteConfirmModal';
import { formatTimestamp, formatBytes } from '../utils/formatters';
import VPCNetworkingShell from '../components/VPCNetworkingShell';

const protocolColors: Record<string, 'blue' | 'purple'> = {
  wireguard: 'blue',
  ipsec: 'purple',
};

const tunnelStatusColors: Record<string, 'green' | 'red' | 'orange'> = {
  Up: 'green',
  Down: 'red',
  Connecting: 'orange',
};

const VPNGatewayDetailPage: React.FC = () => {
  const { name } = useParams<{ name: string }>();
  const [searchParams] = useSearchParams();
  const ns = searchParams.get('ns') || undefined;
  const { vpnGateway, loading } = useVPNGateway(name || '', ns);
  const navigate = useNavigate();

  const [isDeleteModalOpen, setIsDeleteModalOpen] = useState(false);
  const [actionLoading, setActionLoading] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);

  const handleDelete = async () => {
    if (!name) return;
    setActionLoading(true);
    setActionError(null);
    const resp = await apiClient.deleteVPNGateway(name, ns);
    if (resp.error) {
      setActionError(resp.error.message);
      setActionLoading(false);
    } else {
      setIsDeleteModalOpen(false);
      navigate('/vpc-networking/vpn-gateways');
    }
  };

  return (
    <VPCNetworkingShell>
      <PageSection variant={PageSectionVariants.light}>
        <Breadcrumb>
          <BreadcrumbItem><Link to="/vpc-networking/vpn-gateways">VPN Gateways</Link></BreadcrumbItem>
          <BreadcrumbItem isActive>{vpnGateway?.name || name}</BreadcrumbItem>
        </Breadcrumb>
      </PageSection>

      <PageSection>
        {loading ? (
          <Spinner size="lg" />
        ) : vpnGateway ? (
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
                        <DescriptionListDescription>{vpnGateway.name || '-'}</DescriptionListDescription>
                      </DescriptionListGroup>
                      <DescriptionListGroup>
                        <DescriptionListTerm>Namespace</DescriptionListTerm>
                        <DescriptionListDescription>{vpnGateway.namespace || '-'}</DescriptionListDescription>
                      </DescriptionListGroup>
                      <DescriptionListGroup>
                        <DescriptionListTerm>Protocol</DescriptionListTerm>
                        <DescriptionListDescription>
                          <Label color={protocolColors[vpnGateway.protocol] || 'grey'}>{vpnGateway.protocol}</Label>
                        </DescriptionListDescription>
                      </DescriptionListGroup>
                      <DescriptionListGroup>
                        <DescriptionListTerm>Gateway</DescriptionListTerm>
                        <DescriptionListDescription>
                          {vpnGateway.gatewayRef ? (
                            <Link to={`/vpc-networking/gateways/${vpnGateway.gatewayRef}${ns ? `?ns=${encodeURIComponent(ns)}` : ''}`}>
                              {vpnGateway.gatewayRef}
                            </Link>
                          ) : '-'}
                        </DescriptionListDescription>
                      </DescriptionListGroup>
                      <DescriptionListGroup>
                        <DescriptionListTerm>Phase</DescriptionListTerm>
                        <DescriptionListDescription>
                          <StatusBadge status={vpnGateway.phase} />
                        </DescriptionListDescription>
                      </DescriptionListGroup>
                      <DescriptionListGroup>
                        <DescriptionListTerm>Sync Status</DescriptionListTerm>
                        <DescriptionListDescription>
                          <StatusBadge status={vpnGateway.syncStatus} />
                        </DescriptionListDescription>
                      </DescriptionListGroup>
                      <DescriptionListGroup>
                        <DescriptionListTerm>Tunnel Endpoint</DescriptionListTerm>
                        <DescriptionListDescription>{vpnGateway.tunnelEndpoint || '-'}</DescriptionListDescription>
                      </DescriptionListGroup>
                      <DescriptionListGroup>
                        <DescriptionListTerm>Pod</DescriptionListTerm>
                        <DescriptionListDescription>{vpnGateway.podName || '-'}</DescriptionListDescription>
                      </DescriptionListGroup>
                      <DescriptionListGroup>
                        <DescriptionListTerm>Created</DescriptionListTerm>
                        <DescriptionListDescription>
                          {formatTimestamp(vpnGateway.createdAt)}
                        </DescriptionListDescription>
                      </DescriptionListGroup>
                      {vpnGateway.message && (
                        <DescriptionListGroup>
                          <DescriptionListTerm>Message</DescriptionListTerm>
                          <DescriptionListDescription>{vpnGateway.message}</DescriptionListDescription>
                        </DescriptionListGroup>
                      )}
                    </DescriptionList>
                  </CardBody>
                </Card>
              </GridItem>

              {/* Card 2: Tunnels */}
              <GridItem>
                <Card isFullHeight>
                  <CardTitle>
                    Tunnels ({vpnGateway.activeTunnels}/{vpnGateway.totalTunnels} active)
                  </CardTitle>
                  <CardBody>
                    {vpnGateway.tunnels && vpnGateway.tunnels.length > 0 ? (
                      <Table aria-label="Tunnels" variant="compact">
                        <Thead>
                          <Tr>
                            <Th>Name</Th>
                            <Th>Status</Th>
                            <Th>Last Handshake</Th>
                            <Th>In</Th>
                            <Th>Out</Th>
                          </Tr>
                        </Thead>
                        <Tbody>
                          {vpnGateway.tunnels.map((t) => (
                            <Tr key={t.name}>
                              <Td>{t.name}</Td>
                              <Td>
                                <Label color={tunnelStatusColors[t.status] || 'grey'}>{t.status}</Label>
                              </Td>
                              <Td>{t.lastHandshake ? formatTimestamp(t.lastHandshake) : '-'}</Td>
                              <Td>{formatBytes(t.bytesIn)}</Td>
                              <Td>{formatBytes(t.bytesOut)}</Td>
                            </Tr>
                          ))}
                        </Tbody>
                      </Table>
                    ) : (
                      <span style={{ color: 'var(--pf-v5-global--Color--200)' }}>No tunnel status available yet</span>
                    )}
                  </CardBody>
                </Card>
              </GridItem>

              {/* Card 3: Advertised Routes */}
              <GridItem>
                <Card isFullHeight>
                  <CardTitle>Advertised Routes</CardTitle>
                  <CardBody>
                    {vpnGateway.advertisedRoutes && vpnGateway.advertisedRoutes.length > 0 ? (
                      <DescriptionList>
                        {vpnGateway.advertisedRoutes.map((route) => (
                          <DescriptionListGroup key={route}>
                            <DescriptionListDescription>
                              <code>{route}</code>
                            </DescriptionListDescription>
                          </DescriptionListGroup>
                        ))}
                      </DescriptionList>
                    ) : (
                      <span style={{ color: 'var(--pf-v5-global--Color--200)' }}>No routes advertised</span>
                    )}
                  </CardBody>
                </Card>
              </GridItem>

              {/* Card 4: MTU & Stats */}
              <GridItem>
                <Card isFullHeight>
                  <CardTitle>Configuration</CardTitle>
                  <CardBody>
                    <DescriptionList>
                      <DescriptionListGroup>
                        <DescriptionListTerm>Tunnel MTU</DescriptionListTerm>
                        <DescriptionListDescription>{vpnGateway.tunnelMTU || 1420}</DescriptionListDescription>
                      </DescriptionListGroup>
                      <DescriptionListGroup>
                        <DescriptionListTerm>MSS Clamping</DescriptionListTerm>
                        <DescriptionListDescription>
                          {vpnGateway.mssClamp === false ? 'Disabled' : 'Enabled'}
                        </DescriptionListDescription>
                      </DescriptionListGroup>
                      <DescriptionListGroup>
                        <DescriptionListTerm>Connected Clients</DescriptionListTerm>
                        <DescriptionListDescription>{vpnGateway.connectedClients}</DescriptionListDescription>
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
              titleText="VPN Gateway not found"
              icon={<EmptyStateIcon icon={CubesIcon} />}
            />
            <EmptyStateBody>
              The VPN gateway &quot;{name}&quot; could not be found. It may have been deleted or the name is incorrect.
            </EmptyStateBody>
          </EmptyState>
        )}
      </PageSection>

      <DeleteConfirmModal
        isOpen={isDeleteModalOpen}
        title="Delete VPN Gateway"
        message={`Deleting VPN gateway "${vpnGateway?.name}" will tear down all VPN tunnels and disconnect remote sites. This action cannot be undone.`}
        resourceName={vpnGateway?.name}
        onConfirm={handleDelete}
        onCancel={() => { setIsDeleteModalOpen(false); setActionError(null); }}
        isLoading={actionLoading}
      />
    </VPCNetworkingShell>
  );
};

VPNGatewayDetailPage.displayName = 'VPNGatewayDetailPage';
export default VPNGatewayDetailPage;
