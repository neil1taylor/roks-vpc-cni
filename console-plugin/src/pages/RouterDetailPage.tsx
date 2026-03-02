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
  ExpandableSection,
  Title,
} from '@patternfly/react-core';
import { Table, Thead, Tr, Th, Tbody, Td } from '@patternfly/react-table';
import { Link, useNavigate } from 'react-router-dom-v5-compat';
import { useRouter, useGateway } from '../api/hooks';
import { apiClient } from '../api/client';
import { StatusBadge } from '../components/StatusBadge';
import { DeleteConfirmModal } from '../components/DeleteConfirmModal';
import { formatTimestamp } from '../utils/formatters';
import VPCNetworkingShell from '../components/VPCNetworkingShell';
import { RouterNetworkDHCP } from '../api/types';

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

  const dhcpEnabledCount = router?.networks.filter((n) => n.dhcp?.enabled).length || 0;
  const totalNetworks = router?.networks.length || 0;
  const hasDHCP = router?.dhcp?.enabled;

  const networksWithOverrides = router?.networks.filter(
    (n) => n.dhcp?.hasOverride && (n.dhcp.reservations?.length || n.dhcp.leaseTime || n.dhcp.dns || n.dhcp.options || n.dhcp.rangeOverride),
  ) || [];

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
                    <DescriptionListTerm>IDS/IPS</DescriptionListTerm>
                    <DescriptionListDescription>
                      {router.idsMode ? (
                        <Label isCompact color={router.idsMode === 'ips' ? 'orange' : 'blue'}>
                          {router.idsMode.toUpperCase()}
                        </Label>
                      ) : (
                        '-'
                      )}
                    </DescriptionListDescription>
                  </DescriptionListGroup>
                  <DescriptionListGroup>
                    <DescriptionListTerm>DHCP</DescriptionListTerm>
                    <DescriptionListDescription>
                      {hasDHCP ? (
                        <Label isCompact color="green">
                          Enabled ({dhcpEnabledCount} of {totalNetworks} networks)
                        </Label>
                      ) : (
                        <Label isCompact color="grey">Disabled</Label>
                      )}
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

            <Card style={{ marginBottom: '24px' }}>
              <CardTitle>Connected Networks</CardTitle>
              <CardBody>
                {router.networks && router.networks.length > 0 ? (
                  <Table aria-label="Connected networks" variant="compact">
                    <Thead>
                      <Tr>
                        <Th>Name</Th>
                        <Th>Address</Th>
                        <Th>Connected</Th>
                        {hasDHCP && <Th>DHCP</Th>}
                        {hasDHCP && <Th>Pool Range</Th>}
                        {hasDHCP && <Th>Reservations</Th>}
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
                          {hasDHCP && (
                            <Td>
                              <Label isCompact color={net.dhcp?.enabled ? 'green' : 'grey'}>
                                {net.dhcp?.enabled ? 'Enabled' : 'Disabled'}
                              </Label>
                              {net.dhcp?.hasOverride && (
                                <>
                                  {' '}
                                  <Label isCompact color="blue">Override</Label>
                                </>
                              )}
                            </Td>
                          )}
                          {hasDHCP && (
                            <Td>
                              {net.dhcp?.poolStart && net.dhcp?.poolEnd
                                ? `${net.dhcp.poolStart} – ${net.dhcp.poolEnd}`
                                : '–'}
                            </Td>
                          )}
                          {hasDHCP && (
                            <Td>{net.dhcp?.reservationCount ?? 0}</Td>
                          )}
                        </Tr>
                      ))}
                    </Tbody>
                  </Table>
                ) : (
                  <span>No networks connected</span>
                )}
              </CardBody>
            </Card>

            {hasDHCP && (
              <Card>
                <CardTitle>DHCP Configuration</CardTitle>
                <CardBody>
                  <Title headingLevel="h4" style={{ marginBottom: '12px' }}>Global Defaults</Title>
                  <DescriptionList isHorizontal>
                    <DescriptionListGroup>
                      <DescriptionListTerm>Lease Time</DescriptionListTerm>
                      <DescriptionListDescription>{router.dhcp?.leaseTime || '12h (default)'}</DescriptionListDescription>
                    </DescriptionListGroup>
                    {router.dhcp?.dns && (
                      <>
                        {router.dhcp.dns.nameservers && router.dhcp.dns.nameservers.length > 0 && (
                          <DescriptionListGroup>
                            <DescriptionListTerm>DNS Nameservers</DescriptionListTerm>
                            <DescriptionListDescription>{router.dhcp.dns.nameservers.join(', ')}</DescriptionListDescription>
                          </DescriptionListGroup>
                        )}
                        {router.dhcp.dns.searchDomains && router.dhcp.dns.searchDomains.length > 0 && (
                          <DescriptionListGroup>
                            <DescriptionListTerm>Search Domains</DescriptionListTerm>
                            <DescriptionListDescription>{router.dhcp.dns.searchDomains.join(', ')}</DescriptionListDescription>
                          </DescriptionListGroup>
                        )}
                        {router.dhcp.dns.localDomain && (
                          <DescriptionListGroup>
                            <DescriptionListTerm>Local Domain</DescriptionListTerm>
                            <DescriptionListDescription>{router.dhcp.dns.localDomain}</DescriptionListDescription>
                          </DescriptionListGroup>
                        )}
                      </>
                    )}
                    {router.dhcp?.options && (
                      <>
                        {router.dhcp.options.mtu != null && (
                          <DescriptionListGroup>
                            <DescriptionListTerm>MTU</DescriptionListTerm>
                            <DescriptionListDescription>{router.dhcp.options.mtu}</DescriptionListDescription>
                          </DescriptionListGroup>
                        )}
                        {router.dhcp.options.ntpServers && router.dhcp.options.ntpServers.length > 0 && (
                          <DescriptionListGroup>
                            <DescriptionListTerm>NTP Servers</DescriptionListTerm>
                            <DescriptionListDescription>{router.dhcp.options.ntpServers.join(', ')}</DescriptionListDescription>
                          </DescriptionListGroup>
                        )}
                      </>
                    )}
                  </DescriptionList>

                  {networksWithOverrides.length > 0 && (
                    <>
                      <Title headingLevel="h4" style={{ marginTop: '24px', marginBottom: '12px' }}>Per-Network Overrides</Title>
                      {networksWithOverrides.map((net) => (
                        <NetworkDHCPSection key={net.name} name={net.name} dhcp={net.dhcp!} />
                      ))}
                    </>
                  )}
                </CardBody>
              </Card>
            )}
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

const NetworkDHCPSection: React.FC<{ name: string; dhcp: RouterNetworkDHCP }> = ({ name, dhcp }) => {
  const [expanded, setExpanded] = useState(false);
  return (
    <ExpandableSection
      toggleText={name}
      isExpanded={expanded}
      onToggle={(_e, val) => setExpanded(val)}
      style={{ marginBottom: '8px' }}
    >
      <DescriptionList isHorizontal isCompact style={{ marginBottom: '8px' }}>
        {dhcp.leaseTime && (
          <DescriptionListGroup>
            <DescriptionListTerm>Lease Time</DescriptionListTerm>
            <DescriptionListDescription>{dhcp.leaseTime}</DescriptionListDescription>
          </DescriptionListGroup>
        )}
        {dhcp.rangeOverride && (
          <DescriptionListGroup>
            <DescriptionListTerm>Custom Range</DescriptionListTerm>
            <DescriptionListDescription>{dhcp.rangeOverride.start} – {dhcp.rangeOverride.end}</DescriptionListDescription>
          </DescriptionListGroup>
        )}
        {dhcp.dns?.nameservers && dhcp.dns.nameservers.length > 0 && (
          <DescriptionListGroup>
            <DescriptionListTerm>DNS Nameservers</DescriptionListTerm>
            <DescriptionListDescription>{dhcp.dns.nameservers.join(', ')}</DescriptionListDescription>
          </DescriptionListGroup>
        )}
        {dhcp.dns?.searchDomains && dhcp.dns.searchDomains.length > 0 && (
          <DescriptionListGroup>
            <DescriptionListTerm>Search Domains</DescriptionListTerm>
            <DescriptionListDescription>{dhcp.dns.searchDomains.join(', ')}</DescriptionListDescription>
          </DescriptionListGroup>
        )}
      </DescriptionList>
      {dhcp.reservations && dhcp.reservations.length > 0 && (
        <Table aria-label={`Reservations for ${name}`} variant="compact">
          <Thead>
            <Tr>
              <Th>MAC</Th>
              <Th>IP</Th>
              <Th>Hostname</Th>
            </Tr>
          </Thead>
          <Tbody>
            {dhcp.reservations.map((r, i) => (
              <Tr key={i}>
                <Td><code>{r.mac}</code></Td>
                <Td><code>{r.ip}</code></Td>
                <Td>{r.hostname || '–'}</Td>
              </Tr>
            ))}
          </Tbody>
        </Table>
      )}
    </ExpandableSection>
  );
};

export default RouterDetailPage;
