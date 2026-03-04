import React, { useState, useEffect, useCallback } from 'react';
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
  Modal,
  TextInput,
  FormGroup,
  FormHelperText,
  HelperText,
  HelperTextItem,
} from '@patternfly/react-core';
import { Table, Thead, Tr, Th, Tbody, Td } from '@patternfly/react-table';
import { CubesIcon, DownloadIcon } from '@patternfly/react-icons';
import { Link, useNavigate } from 'react-router-dom-v5-compat';
import { useVPNGateway } from '../api/hooks';
import { apiClient } from '../api/client';
import { IssuedClient } from '../api/types';
import { StatusBadge } from '../components/StatusBadge';
import { DeleteConfirmModal } from '../components/DeleteConfirmModal';
import { formatTimestamp, formatBytes } from '../utils/formatters';
import VPCNetworkingShell from '../components/VPCNetworkingShell';

const protocolColors: Record<string, 'blue' | 'purple' | 'cyan'> = {
  wireguard: 'blue',
  ipsec: 'purple',
  openvpn: 'cyan',
};

const tunnelStatusColors: Record<string, 'green' | 'red' | 'orange'> = {
  Up: 'green',
  Down: 'red',
  Connecting: 'orange',
};

const clientNamePattern = /^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?$/;

const VPNGatewayDetailPage: React.FC = () => {
  const { name } = useParams<{ name: string }>();
  const [searchParams] = useSearchParams();
  const ns = searchParams.get('ns') || undefined;
  const { vpnGateway, loading } = useVPNGateway(name || '', ns);
  const navigate = useNavigate();

  const [isDeleteModalOpen, setIsDeleteModalOpen] = useState(false);
  const [actionLoading, setActionLoading] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);

  // Client config generation state
  const [isClientConfigModalOpen, setIsClientConfigModalOpen] = useState(false);
  const [clientName, setClientName] = useState('');
  const [clientConfigLoading, setClientConfigLoading] = useState(false);
  const [clientConfigError, setClientConfigError] = useState<string | null>(null);

  // Issued clients state
  const [issuedClients, setIssuedClients] = useState<IssuedClient[]>([]);
  const [clientsLoading, setClientsLoading] = useState(false);
  const [refreshKey, setRefreshKey] = useState(0);

  // Revoke confirmation state
  const [isRevokeModalOpen, setIsRevokeModalOpen] = useState(false);
  const [revokeTarget, setRevokeTarget] = useState<string | null>(null);
  const [revokeLoading, setRevokeLoading] = useState(false);

  const fetchClients = useCallback(async () => {
    if (!name || !vpnGateway || vpnGateway.protocol !== 'openvpn') return;
    setClientsLoading(true);
    const resp = await apiClient.listVPNClients(name, ns);
    if (resp.data) {
      setIssuedClients(resp.data);
    }
    setClientsLoading(false);
  }, [name, ns, vpnGateway]);

  useEffect(() => {
    fetchClients();
  }, [fetchClients, refreshKey]);

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

  const handleGenerateClientConfig = async () => {
    if (!name || !clientName.trim()) return;
    setClientConfigLoading(true);
    setClientConfigError(null);

    const resp = await apiClient.generateClientConfig(name, clientName.trim(), ns);
    if (resp.error) {
      setClientConfigError(resp.error.message);
      setClientConfigLoading(false);
    } else if (resp.data) {
      // Trigger browser download
      const blob = new Blob([resp.data.ovpnConfig], { type: 'application/x-openvpn-profile' });
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `${clientName.trim()}.ovpn`;
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      URL.revokeObjectURL(url);

      setClientConfigLoading(false);
      setClientName('');
      setIsClientConfigModalOpen(false);
      setRefreshKey((k) => k + 1);
    }
  };

  const handleRevoke = async () => {
    if (!name || !revokeTarget) return;
    setRevokeLoading(true);
    const resp = await apiClient.revokeVPNClient(name, revokeTarget, ns);
    setRevokeLoading(false);
    if (resp.error) {
      setActionError(resp.error.message);
    }
    setIsRevokeModalOpen(false);
    setRevokeTarget(null);
    setRefreshKey((k) => k + 1);
  };

  const isClientNameValid = clientName.trim() !== '' && clientNamePattern.test(clientName.trim());

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
                        {vpnGateway.protocol === 'openvpn' && (
                          <Button
                            variant="secondary"
                            icon={<DownloadIcon />}
                            onClick={() => {
                              setClientConfigError(null);
                              setClientName('');
                              setIsClientConfigModalOpen(true);
                            }}
                            isDisabled={actionLoading}
                            style={{ marginRight: '8px' }}
                          >
                            Generate .ovpn
                          </Button>
                        )}
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

              {/* Card 5: Issued Clients (OpenVPN only) */}
              {vpnGateway.protocol === 'openvpn' && (
                <GridItem span={12}>
                  <Card>
                    <CardTitle>
                      <Split hasGutter>
                        <SplitItem isFilled>Issued Clients ({issuedClients.length})</SplitItem>
                        <SplitItem>
                          <Button
                            variant="primary"
                            icon={<DownloadIcon />}
                            onClick={() => {
                              setClientConfigError(null);
                              setClientName('');
                              setIsClientConfigModalOpen(true);
                            }}
                            isDisabled={actionLoading}
                          >
                            Generate .ovpn
                          </Button>
                        </SplitItem>
                      </Split>
                    </CardTitle>
                    <CardBody>
                      {clientsLoading ? (
                        <Spinner size="md" />
                      ) : issuedClients.length > 0 ? (
                        <Table aria-label="Issued clients" variant="compact">
                          <Thead>
                            <Tr>
                              <Th>Client Name</Th>
                              <Th>Serial</Th>
                              <Th>Issued</Th>
                              <Th>Expires</Th>
                              <Th>Status</Th>
                              <Th>Actions</Th>
                            </Tr>
                          </Thead>
                          <Tbody>
                            {issuedClients.map((client) => (
                              <Tr key={client.secretName}>
                                <Td>{client.clientName}</Td>
                                <Td><code>{client.serialHex.substring(0, 16)}...</code></Td>
                                <Td>{formatTimestamp(client.issuedAt)}</Td>
                                <Td>{formatTimestamp(client.expiresAt)}</Td>
                                <Td>
                                  <Label color={client.revoked ? 'red' : 'green'}>
                                    {client.revoked ? 'Revoked' : 'Active'}
                                  </Label>
                                </Td>
                                <Td>
                                  {!client.revoked && (
                                    <Button
                                      variant="danger"
                                      size="sm"
                                      onClick={() => {
                                        setRevokeTarget(client.clientName);
                                        setIsRevokeModalOpen(true);
                                      }}
                                    >
                                      Revoke
                                    </Button>
                                  )}
                                </Td>
                              </Tr>
                            ))}
                          </Tbody>
                        </Table>
                      ) : (
                        <span style={{ color: 'var(--pf-v5-global--Color--200)' }}>
                          No client certificates issued yet. Click &quot;Generate .ovpn&quot; to create one.
                        </span>
                      )}
                    </CardBody>
                  </Card>
                </GridItem>
              )}
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

      {/* Revoke Client Confirmation Modal */}
      <DeleteConfirmModal
        isOpen={isRevokeModalOpen}
        title="Revoke Client Certificate"
        message={`Revoking the certificate for "${revokeTarget}" will add it to the CRL and prevent this client from connecting. The client secret will be deleted. This action cannot be undone.`}
        resourceName={revokeTarget || undefined}
        onConfirm={handleRevoke}
        onCancel={() => { setIsRevokeModalOpen(false); setRevokeTarget(null); }}
        isLoading={revokeLoading}
      />

      {/* Client Config Generation Modal */}
      <Modal
        title="Generate Client Configuration"
        isOpen={isClientConfigModalOpen}
        onClose={() => setIsClientConfigModalOpen(false)}
        actions={[
          <Button
            key="generate"
            variant="primary"
            onClick={handleGenerateClientConfig}
            isLoading={clientConfigLoading}
            isDisabled={!isClientNameValid || clientConfigLoading}
          >
            Generate &amp; Download
          </Button>,
          <Button
            key="cancel"
            variant="link"
            onClick={() => setIsClientConfigModalOpen(false)}
            isDisabled={clientConfigLoading}
          >
            Cancel
          </Button>,
        ]}
      >
        {clientConfigError && (
          <Alert variant="danger" title={clientConfigError} isInline style={{ marginBottom: '1rem' }} />
        )}
        <FormGroup label="Client Name" isRequired fieldId="client-name">
          <TextInput
            id="client-name"
            value={clientName}
            onChange={(_e, v) => setClientName(v)}
            isRequired
            placeholder="e.g. alice"
            validated={clientName && !isClientNameValid ? 'error' : 'default'}
          />
          <FormHelperText>
            <HelperText>
              <HelperTextItem variant={clientName && !isClientNameValid ? 'error' : 'default'}>
                {clientName && !isClientNameValid
                  ? 'Must be alphanumeric with optional hyphens (e.g. "alice", "laptop-1")'
                  : 'A unique name for this client. Used as the certificate CN and secret name suffix.'}
              </HelperTextItem>
            </HelperText>
          </FormHelperText>
        </FormGroup>
      </Modal>
    </VPCNetworkingShell>
  );
};

VPNGatewayDetailPage.displayName = 'VPNGatewayDetailPage';
export default VPNGatewayDetailPage;
