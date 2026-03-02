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
  Tabs,
  Tab,
  TabTitleText,
  Grid,
  GridItem,
  EmptyState,
  EmptyStateBody,
} from '@patternfly/react-core';
import { Table, Thead, Tr, Th, Tbody, Td } from '@patternfly/react-table';
import { Link, useNavigate } from 'react-router-dom-v5-compat';
import { useRouter, useGateway, useRouterHealth, useRouterInterfaces, useRouterConntrack, useRouterDHCP, useRouterNFT, useRouterLeases } from '../api/hooks';
import { apiClient } from '../api/client';
import { StatusBadge } from '../components/StatusBadge';
import { DeleteConfirmModal } from '../components/DeleteConfirmModal';
import { EditReservationsModal } from '../components/EditReservationsModal';
import { EditIDSModal } from '../components/EditIDSModal';
import { formatTimestamp } from '../utils/formatters';
import VPCNetworkingShell from '../components/VPCNetworkingShell';
import { RouterNetworkDHCP, DHCPReservation, RouterIDS } from '../api/types';
import RouterHealthCard from '../components/charts/RouterHealthCard';
import ThroughputChart from '../components/charts/ThroughputChart';
import ConntrackGauge from '../components/charts/ConntrackGauge';
import DHCPPoolGauge from '../components/charts/DHCPPoolGauge';
import NFTCountersTable from '../components/charts/NFTCountersTable';
import TimeRangeSelector from '../components/charts/TimeRangeSelector';

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
  const [activeTab, setActiveTab] = useState<string>('overview');
  const [range, setRange] = useState('1h');
  const [step, setStep] = useState('1m');

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
  const hasMetrics = router?.metricsEnabled;

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

            <Split hasGutter style={{ marginBottom: '16px' }}>
              <SplitItem isFilled>
                <Tabs activeKey={activeTab} onSelect={(_e, key) => setActiveTab(key as string)}>
                  <Tab eventKey="overview" title={<TabTitleText>Overview</TabTitleText>} />
                  {hasMetrics && <Tab eventKey="monitoring" title={<TabTitleText>Monitoring</TabTitleText>} />}
                  <Tab eventKey="networks" title={<TabTitleText>Networks</TabTitleText>} />
                  {hasMetrics && <Tab eventKey="nft" title={<TabTitleText>NFT Rules</TabTitleText>} />}
                </Tabs>
              </SplitItem>
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

            {activeTab === 'overview' && (
              <OverviewTab
                router={router}
                gatewayDetail={gatewayDetail}
                hasDHCP={!!hasDHCP}
                hasMetrics={!!hasMetrics}
                dhcpEnabledCount={dhcpEnabledCount}
                totalNetworks={totalNetworks}
                networksWithOverrides={networksWithOverrides}
                routerName={name || ''}
                routerNamespace={router.namespace}
              />
            )}

            {activeTab === 'monitoring' && hasMetrics && (
              <MonitoringTab
                name={router.name}
                namespace={router.namespace}
                range={range}
                step={step}
                onRangeChange={(r, s) => { setRange(r); setStep(s); }}
              />
            )}

            {activeTab === 'networks' && (
              <NetworksTab router={router} hasDHCP={!!hasDHCP} routerName={name || ''} routerNamespace={router.namespace} />
            )}

            {activeTab === 'nft' && hasMetrics && (
              <NFTTab name={router.name} namespace={router.namespace} />
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

// ── Overview Tab ──

interface OverviewTabProps {
  router: { name: string; namespace: string; gateway: string; phase: string; transitIP?: string; advertisedRoutes?: string[]; idsMode?: string; ids?: RouterIDS; metricsEnabled?: boolean; syncStatus: string; createdAt?: string; dhcp?: { enabled: boolean; leaseTime?: string; dns?: { nameservers?: string[]; searchDomains?: string[]; localDomain?: string }; options?: { mtu?: number; ntpServers?: string[] } } };
  gatewayDetail: { zone: string; phase: string } | null;
  hasDHCP: boolean;
  hasMetrics: boolean;
  dhcpEnabledCount: number;
  totalNetworks: number;
  networksWithOverrides: { name: string; dhcp?: RouterNetworkDHCP }[];
  routerName: string;
  routerNamespace: string;
}

const OverviewTab: React.FC<OverviewTabProps> = ({ router, gatewayDetail, hasDHCP, hasMetrics, dhcpEnabledCount, totalNetworks, networksWithOverrides, routerName, routerNamespace }) => {
  const [editIDSOpen, setEditIDSOpen] = useState(false);

  const { data: health, loading: healthLoading } = useRouterHealth(
    hasMetrics ? router.name : '',
    hasMetrics ? router.namespace : undefined,
  );

  return (
    <>
      {hasMetrics && (
        <div style={{ marginBottom: '24px' }}>
          <RouterHealthCard health={health} loading={healthLoading} />
        </div>
      )}

      <Card style={{ marginBottom: '24px' }}>
        <CardTitle>Details</CardTitle>
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
              <DescriptionListDescription><StatusBadge status={router.phase} /></DescriptionListDescription>
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
                  <>
                    <Label isCompact color="grey">Disabled</Label>
                    {' '}
                    <Button variant="link" isInline size="sm" onClick={() => setEditIDSOpen(true)}>Enable</Button>
                  </>
                )}
              </DescriptionListDescription>
            </DescriptionListGroup>
            <DescriptionListGroup>
              <DescriptionListTerm>Metrics</DescriptionListTerm>
              <DescriptionListDescription>
                <Label isCompact color={hasMetrics ? 'green' : 'grey'}>
                  {hasMetrics ? 'Enabled' : 'Disabled'}
                </Label>
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
              <DescriptionListDescription><StatusBadge status={router.syncStatus} /></DescriptionListDescription>
            </DescriptionListGroup>
            <DescriptionListGroup>
              <DescriptionListTerm>Created</DescriptionListTerm>
              <DescriptionListDescription>{formatTimestamp(router.createdAt)}</DescriptionListDescription>
            </DescriptionListGroup>
          </DescriptionList>
        </CardBody>
      </Card>

      {router.ids?.enabled && (
        <Card style={{ marginBottom: '24px' }}>
          <CardTitle>
            <Split>
              <SplitItem isFilled>IDS/IPS Configuration</SplitItem>
              <SplitItem>
                <Button variant="secondary" size="sm" onClick={() => setEditIDSOpen(true)}>
                  Edit IDS/IPS
                </Button>
              </SplitItem>
            </Split>
          </CardTitle>
          <CardBody>
            <DescriptionList isHorizontal>
              <DescriptionListGroup>
                <DescriptionListTerm>Mode</DescriptionListTerm>
                <DescriptionListDescription>
                  <Label isCompact color={router.ids.mode === 'ips' ? 'orange' : 'blue'}>
                    {router.ids.mode === 'ips' ? 'IPS — Inline Blocking' : 'IDS — Passive Monitoring'}
                  </Label>
                </DescriptionListDescription>
              </DescriptionListGroup>
              <DescriptionListGroup>
                <DescriptionListTerm>Interfaces</DescriptionListTerm>
                <DescriptionListDescription>{router.ids.interfaces || 'all'}</DescriptionListDescription>
              </DescriptionListGroup>
              {router.ids.syslogTarget && (
                <DescriptionListGroup>
                  <DescriptionListTerm>Syslog Target</DescriptionListTerm>
                  <DescriptionListDescription><code>{router.ids.syslogTarget}</code></DescriptionListDescription>
                </DescriptionListGroup>
              )}
              {router.ids.customRules && (
                <DescriptionListGroup>
                  <DescriptionListTerm>Custom Rules</DescriptionListTerm>
                  <DescriptionListDescription>
                    {router.ids.customRules.split('\n').filter(Boolean).length} rule(s)
                  </DescriptionListDescription>
                </DescriptionListGroup>
              )}
              {router.ids.image && (
                <DescriptionListGroup>
                  <DescriptionListTerm>Image</DescriptionListTerm>
                  <DescriptionListDescription><code>{router.ids.image}</code></DescriptionListDescription>
                </DescriptionListGroup>
              )}
            </DescriptionList>
          </CardBody>
        </Card>
      )}

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
              {router.dhcp?.dns?.nameservers && router.dhcp.dns.nameservers.length > 0 && (
                <DescriptionListGroup>
                  <DescriptionListTerm>DNS Nameservers</DescriptionListTerm>
                  <DescriptionListDescription>{router.dhcp.dns.nameservers.join(', ')}</DescriptionListDescription>
                </DescriptionListGroup>
              )}
              {router.dhcp?.dns?.searchDomains && router.dhcp.dns.searchDomains.length > 0 && (
                <DescriptionListGroup>
                  <DescriptionListTerm>Search Domains</DescriptionListTerm>
                  <DescriptionListDescription>{router.dhcp.dns.searchDomains.join(', ')}</DescriptionListDescription>
                </DescriptionListGroup>
              )}
              {router.dhcp?.dns?.localDomain && (
                <DescriptionListGroup>
                  <DescriptionListTerm>Local Domain</DescriptionListTerm>
                  <DescriptionListDescription>{router.dhcp.dns.localDomain}</DescriptionListDescription>
                </DescriptionListGroup>
              )}
              {router.dhcp?.options?.mtu != null && (
                <DescriptionListGroup>
                  <DescriptionListTerm>MTU</DescriptionListTerm>
                  <DescriptionListDescription>{router.dhcp.options.mtu}</DescriptionListDescription>
                </DescriptionListGroup>
              )}
              {router.dhcp?.options?.ntpServers && router.dhcp.options.ntpServers.length > 0 && (
                <DescriptionListGroup>
                  <DescriptionListTerm>NTP Servers</DescriptionListTerm>
                  <DescriptionListDescription>{router.dhcp.options.ntpServers.join(', ')}</DescriptionListDescription>
                </DescriptionListGroup>
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

      <EditIDSModal
        isOpen={editIDSOpen}
        onClose={(updated) => {
          setEditIDSOpen(false);
          if (updated) window.location.reload();
        }}
        routerName={routerName}
        routerNamespace={routerNamespace}
        currentIDS={router.ids}
      />
    </>
  );
};

// ── Monitoring Tab ──

interface MonitoringTabProps {
  name: string;
  namespace: string;
  range: string;
  step: string;
  onRangeChange: (range: string, step: string) => void;
}

const MonitoringTab: React.FC<MonitoringTabProps> = ({ name, namespace, range, step, onRangeChange }) => {
  const { data: health, loading: healthLoading } = useRouterHealth(name, namespace);
  const { data: interfaces } = useRouterInterfaces(name, namespace, range, step);
  const { data: conntrack } = useRouterConntrack(name, namespace, range, step);
  const { data: dhcpPools } = useRouterDHCP(name, namespace);

  return (
    <>
      <Split hasGutter style={{ marginBottom: '16px' }}>
        <SplitItem isFilled />
        <SplitItem>
          <TimeRangeSelector selected={range} onSelect={onRangeChange} />
        </SplitItem>
      </Split>

      <Grid hasGutter style={{ marginBottom: '24px' }}>
        <GridItem span={12}>
          <RouterHealthCard health={health} loading={healthLoading} />
        </GridItem>

        {conntrack && (
          <GridItem span={4}>
            <ConntrackGauge
              entries={conntrack.entries?.[conntrack.entries.length - 1]?.v || 0}
              max={conntrack.max}
              percentage={conntrack.percentage}
            />
          </GridItem>
        )}

        {dhcpPools && dhcpPools.map((pool) => (
          <GridItem span={4} key={pool.name}>
            <DHCPPoolGauge pool={pool} />
          </GridItem>
        ))}
      </Grid>

      {interfaces && interfaces.length > 0 ? (
        interfaces.map((iface) => (
          <div key={iface.name} style={{ marginBottom: '16px' }}>
            <ThroughputChart data={iface} />
          </div>
        ))
      ) : (
        <Card isCompact>
          <CardBody>No interface metrics available yet</CardBody>
        </Card>
      )}
    </>
  );
};

// ── Networks Tab ──

interface NetworksTabProps {
  router: { networks: { name: string; address: string; connected: boolean; dhcp?: RouterNetworkDHCP }[]; dhcp?: { enabled: boolean } };
  hasDHCP: boolean;
  routerName: string;
  routerNamespace: string;
}

const NetworksTab: React.FC<NetworksTabProps> = ({ router, hasDHCP, routerName, routerNamespace }) => {
  const { data: leases, loading: leasesLoading } = useRouterLeases(hasDHCP ? routerName : '', routerNamespace);
  const [editNetwork, setEditNetwork] = useState<{ name: string; reservations: DHCPReservation[] } | null>(null);

  const handleEditClose = () => {
    setEditNetwork(null);
  };

  return (
    <>
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
                  {hasDHCP && <Th />}
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
                          <>{' '}<Label isCompact color="blue">Override</Label></>
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
                    {hasDHCP && (
                      <Td>
                        {net.dhcp?.enabled && (
                          <Button
                            variant="link"
                            size="sm"
                            onClick={() => setEditNetwork({
                              name: net.name,
                              reservations: net.dhcp?.reservations || [],
                            })}
                          >
                            Edit Reservations
                          </Button>
                        )}
                      </Td>
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
          <CardTitle>Active DHCP Leases</CardTitle>
          <CardBody>
            {leasesLoading ? (
              <Spinner size="md" />
            ) : leases && leases.length > 0 ? (
              <Table aria-label="Active DHCP leases" variant="compact">
                <Thead>
                  <Tr>
                    <Th>IP</Th>
                    <Th>MAC</Th>
                    <Th>Hostname</Th>
                    <Th>Expires</Th>
                  </Tr>
                </Thead>
                <Tbody>
                  {leases.map((lease, i) => (
                    <Tr key={i}>
                      <Td><code>{lease.ip}</code></Td>
                      <Td><code>{lease.mac}</code></Td>
                      <Td>{lease.hostname || '–'}</Td>
                      <Td>{formatLeaseExpiry(lease.expiresAt)}</Td>
                    </Tr>
                  ))}
                </Tbody>
              </Table>
            ) : (
              <EmptyState>
                <EmptyStateBody>No active DHCP leases</EmptyStateBody>
              </EmptyState>
            )}
          </CardBody>
        </Card>
      )}

      {editNetwork && (
        <EditReservationsModal
          isOpen={!!editNetwork}
          onClose={handleEditClose}
          routerName={routerName}
          routerNamespace={routerNamespace}
          networkName={editNetwork.name}
          currentReservations={editNetwork.reservations}
        />
      )}
    </>
  );
};

function formatLeaseExpiry(epochSeconds: number): string {
  const now = Date.now() / 1000;
  const remaining = epochSeconds - now;
  if (remaining <= 0) return 'Expired';
  const hours = Math.floor(remaining / 3600);
  const minutes = Math.floor((remaining % 3600) / 60);
  if (hours > 0) return `in ${hours}h ${minutes}m`;
  return `in ${minutes}m`;
}

// ── NFT Tab ──

interface NFTTabProps {
  name: string;
  namespace: string;
}

const NFTTab: React.FC<NFTTabProps> = ({ name, namespace }) => {
  const { data: nftRules, loading } = useRouterNFT(name, namespace);

  if (loading) return <Spinner size="lg" />;
  if (!nftRules || nftRules.length === 0) {
    return (
      <EmptyState>
        <Title headingLevel="h3" size="md">No NFT Counters</Title>
        <EmptyStateBody>No nftables rule counters available for this router.</EmptyStateBody>
      </EmptyState>
    );
  }

  return <NFTCountersTable rules={nftRules} />;
};

// ── Network DHCP Section (reused from before) ──

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
