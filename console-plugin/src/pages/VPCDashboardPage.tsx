import React from 'react';
import {
  PageSection,
  Title,
  Card,
  CardBody,
  CardTitle,
  Grid,
  GridItem,
  Spinner,
  Text,
  TextVariants,
  Label,
} from '@patternfly/react-core';
import { CheckCircleIcon, ExclamationCircleIcon } from '@patternfly/react-icons';
import { Link } from 'react-router-dom-v5-compat';
import {
  useVPCs,
  useSubnets,
  useSecurityGroups,
  useNetworkACLs,
  useFloatingIPs,
  useK8sVPCSubnets,
  useK8sVNIs,
  useK8sVLANAttachments,
  useK8sFloatingIPs,
  useNetworkDefinitions,
  useClusterInfo,
  useGateways,
  useRouters,
  usePARs,
  useL2Bridges,
  useVPNGateways,
  useDNSPolicies,
} from '../api/hooks';
import VPCNetworkingShell from '../components/VPCNetworkingShell';
import AlertTimelineCard from '../components/AlertTimelineCard';
import { Router } from '../api/types';

/**
 * VPC Dashboard Page
 * Displays overview of VPC networking resources
 */
const VPCDashboardPage: React.FC = () => {
  const { clusterInfo, isROKS } = useClusterInfo();
  const roksAPIAvailable = clusterInfo?.features?.roksAPIAvailable === true;
  const isROKSManaged = isROKS && !roksAPIAvailable;

  const { vpcs, loading: vpcLoading } = useVPCs();
  const { subnets, loading: subnetLoading } = useSubnets();
  const { securityGroups, loading: sgLoading } = useSecurityGroups();
  const { networkAcls, loading: aclLoading } = useNetworkACLs();
  const { floatingIps, loading: fipLoading } = useFloatingIPs();
  const { pars, loading: parLoading } = usePARs();

  // K8s CR counts — skip VNI/VLAN watches when ROKS-managed
  const { subnets: k8sSubnets, loading: k8sSubnetLoading } = useK8sVPCSubnets();
  const { vnis: k8sVNIs, loading: k8sVNILoading } = useK8sVNIs();
  const { attachments: k8sAttachments, loading: k8sAttLoading } = useK8sVLANAttachments();
  const { floatingIps: k8sFIPs, loading: k8sFIPLoading } = useK8sFloatingIPs();
  const { gateways, loading: gwLoading } = useGateways();
  const { routers, loading: rtLoading } = useRouters();
  const { l2bridges, loading: l2bLoading } = useL2Bridges();
  const { vpnGateways, loading: vpnLoading } = useVPNGateways();
  const { dnsPolicies, loading: dnsLoading } = useDNSPolicies();
  const { networks, loading: networksLoading } = useNetworkDefinitions();

  const renderCount = (count: number | undefined, loading: boolean) => {
    if (loading) return <Spinner size="md" />;
    return <span style={{ fontSize: '2rem', fontWeight: 600 }}>{count ?? 0}</span>;
  };

  return (
    <VPCNetworkingShell>
      <PageSection>
        <Text component={TextVariants.p} style={{ marginBottom: '16px', color: 'var(--pf-v5-global--Color--200)' }}>
          Overview of VPC API resources and Kubernetes CRD sync status for this cluster.
        </Text>
        <Title headingLevel="h2" size="lg" style={{ marginBottom: '16px' }}>
          VPC API Resources
        </Title>
        <Grid hasGutter>
          <GridItem span={2}>
            <Card isCompact>
              <CardTitle>VPCs</CardTitle>
              <CardBody>{renderCount(vpcs?.length, vpcLoading)}</CardBody>
            </Card>
          </GridItem>
          <GridItem span={2}>
            <Card isCompact>
              <CardTitle>Subnets</CardTitle>
              <CardBody>{renderCount(subnets?.length, subnetLoading)}</CardBody>
            </Card>
          </GridItem>
          <GridItem span={2}>
            <Card isCompact>
              <CardTitle>Security Groups</CardTitle>
              <CardBody>{renderCount(securityGroups?.length, sgLoading)}</CardBody>
            </Card>
          </GridItem>
          <GridItem span={2}>
            <Card isCompact>
              <CardTitle>Network ACLs</CardTitle>
              <CardBody>{renderCount(networkAcls?.length, aclLoading)}</CardBody>
            </Card>
          </GridItem>
          <GridItem span={2}>
            <Card isCompact>
              <CardTitle>Floating IPs</CardTitle>
              <CardBody>{renderCount(floatingIps?.length, fipLoading)}</CardBody>
            </Card>
          </GridItem>
          <GridItem span={2}>
            <Card isCompact>
              <CardTitle>PARs</CardTitle>
              <CardBody>{renderCount(pars?.length, parLoading)}</CardBody>
            </Card>
          </GridItem>
        </Grid>
      </PageSection>

      <PageSection>
        <Title headingLevel="h2" size="lg" style={{ marginBottom: '16px' }}>
          Kubernetes CRD Resources
        </Title>
        <Grid hasGutter>
          <GridItem span={3}>
            <Card isCompact>
              <CardTitle>VPCSubnets</CardTitle>
              <CardBody>{renderCount(k8sSubnets?.length, k8sSubnetLoading)}</CardBody>
            </Card>
          </GridItem>
          <GridItem span={3}>
            <Card isCompact>
              <CardTitle>VNIs</CardTitle>
              <CardBody>
                {isROKSManaged ? (
                  <span style={{ fontSize: '0.875rem', color: '#6a6e73' }}>ROKS-managed</span>
                ) : (
                  renderCount(k8sVNIs?.length, k8sVNILoading)
                )}
              </CardBody>
            </Card>
          </GridItem>
          <GridItem span={3}>
            <Card isCompact>
              <CardTitle>VLAN Attachments</CardTitle>
              <CardBody>
                {isROKSManaged ? (
                  <span style={{ fontSize: '0.875rem', color: '#6a6e73' }}>ROKS-managed</span>
                ) : (
                  renderCount(k8sAttachments?.length, k8sAttLoading)
                )}
              </CardBody>
            </Card>
          </GridItem>
          <GridItem span={3}>
            <Card isCompact>
              <CardTitle>FloatingIPs</CardTitle>
              <CardBody>{renderCount(k8sFIPs?.length, k8sFIPLoading)}</CardBody>
            </Card>
          </GridItem>
          <GridItem span={3}>
            <Card isCompact>
              <CardTitle>Gateways</CardTitle>
              <CardBody>
                {renderCount(gateways?.length, gwLoading)}
                <div style={{ fontSize: 'var(--pf-v5-global--FontSize--sm)', color: 'var(--pf-v5-global--Color--200)', marginTop: 'var(--pf-v5-global--spacer--xs)' }}>
                  Connect routers to the VPC fabric
                </div>
              </CardBody>
            </Card>
          </GridItem>
          <GridItem span={3}>
            <Card isCompact>
              <CardTitle>Routers</CardTitle>
              <CardBody>
                {renderCount(routers?.length, rtLoading)}
                <div style={{ fontSize: 'var(--pf-v5-global--FontSize--sm)', color: 'var(--pf-v5-global--Color--200)', marginTop: 'var(--pf-v5-global--spacer--xs)' }}>
                  Connect Layer2 segments to other Layer2 segments, and optionally via an uplink to a gateway which connects to the VPC fabric
                </div>
              </CardBody>
            </Card>
          </GridItem>
          <GridItem span={3}>
            <Card isCompact>
              <CardTitle>VPN Gateways</CardTitle>
              <CardBody>
                {renderCount(vpnGateways?.length, vpnLoading)}
                {!vpnLoading && vpnGateways && vpnGateways.length > 0 && (
                  <>
                    <div style={{ fontSize: 'var(--pf-v5-global--FontSize--sm)', color: 'var(--pf-v5-global--Color--200)', marginTop: 'var(--pf-v5-global--spacer--xs)' }}>
                      <strong>Active tunnels:</strong>{' '}
                      {vpnGateways.reduce((sum, gw) => sum + gw.activeTunnels, 0)}/{vpnGateways.reduce((sum, gw) => sum + gw.totalTunnels, 0)}
                    </div>
                    <div style={{ fontSize: 'var(--pf-v5-global--FontSize--sm)', color: 'var(--pf-v5-global--Color--200)', marginTop: 'var(--pf-v5-global--spacer--xs)' }}>
                      <strong>By protocol:</strong>{' '}
                      {['wireguard', 'ipsec'].map((proto) => {
                        const count = vpnGateways.filter((gw) => gw.protocol === proto).length;
                        return count > 0 ? `${proto}: ${count}` : null;
                      }).filter(Boolean).join(' · ')}
                    </div>
                  </>
                )}
                <div style={{ marginTop: 'var(--pf-v5-global--spacer--sm)' }}>
                  <a href="/vpc-networking/vpn-gateways">View all VPN Gateways →</a>
                </div>
              </CardBody>
            </Card>
          </GridItem>
          <GridItem span={3}>
            <Card isCompact>
              <CardTitle>L2 Bridges</CardTitle>
              <CardBody>
                {renderCount(l2bridges?.length, l2bLoading)}
                {!l2bLoading && l2bridges && l2bridges.length > 0 && (
                  <>
                    <div style={{ fontSize: 'var(--pf-v5-global--FontSize--sm)', color: 'var(--pf-v5-global--Color--200)', marginTop: 'var(--pf-v5-global--spacer--xs)' }}>
                      <strong>By phase:</strong>{' '}
                      {['Established', 'Provisioning', 'Pending', 'Error'].map((phase) => {
                        const count = l2bridges.filter((b) => b.phase === phase).length;
                        return count > 0 ? `${phase}: ${count}` : null;
                      }).filter(Boolean).join(' · ')}
                    </div>
                    <div style={{ fontSize: 'var(--pf-v5-global--FontSize--sm)', color: 'var(--pf-v5-global--Color--200)', marginTop: 'var(--pf-v5-global--spacer--xs)' }}>
                      <strong>By type:</strong>{' '}
                      {['gretap-wireguard', 'l2vpn', 'evpn-vxlan'].map((type) => {
                        const count = l2bridges.filter((b) => b.type === type).length;
                        return count > 0 ? `${type}: ${count}` : null;
                      }).filter(Boolean).join(' · ')}
                    </div>
                  </>
                )}
                <div style={{ marginTop: 'var(--pf-v5-global--spacer--sm)' }}>
                  <a href="/vpc-networking/l2-bridges">View all L2 Bridges →</a>
                </div>
              </CardBody>
            </Card>
          </GridItem>
          <GridItem span={3}>
            <Card isCompact>
              <CardTitle>DNS Policies</CardTitle>
              <CardBody>
                {renderCount(dnsPolicies?.length, dnsLoading)}
                {!dnsLoading && dnsPolicies && dnsPolicies.length > 0 && (
                  <>
                    <div style={{ fontSize: 'var(--pf-v5-global--FontSize--sm)', color: 'var(--pf-v5-global--Color--200)', marginTop: 'var(--pf-v5-global--spacer--xs)' }}>
                      <strong>Filtering:</strong>{' '}
                      {dnsPolicies.filter((dp) => dp.filteringEnabled).length} enabled
                    </div>
                    <div style={{ fontSize: 'var(--pf-v5-global--FontSize--sm)', color: 'var(--pf-v5-global--Color--200)', marginTop: 'var(--pf-v5-global--spacer--xs)' }}>
                      <strong>Total rules:</strong>{' '}
                      {dnsPolicies.reduce((sum, dp) => sum + dp.filterRulesLoaded, 0).toLocaleString()}
                    </div>
                  </>
                )}
                <div style={{ marginTop: 'var(--pf-v5-global--spacer--sm)' }}>
                  <a href="/vpc-networking/dns-policies">View all DNS Policies →</a>
                </div>
              </CardBody>
            </Card>
          </GridItem>
        </Grid>
      </PageSection>

      <PageSection>
        <Title headingLevel="h2" size="lg" style={{ marginBottom: '16px' }}>
          Network Definitions
        </Title>
        <Grid hasGutter>
          <GridItem span={3}>
            <Card isCompact>
              <CardTitle>CUDNs</CardTitle>
              <CardBody>
                {renderCount(
                  networks?.filter((n) => n.kind === 'ClusterUserDefinedNetwork').length,
                  networksLoading,
                )}
              </CardBody>
            </Card>
          </GridItem>
          <GridItem span={3}>
            <Card isCompact>
              <CardTitle>UDNs</CardTitle>
              <CardBody>
                {renderCount(
                  networks?.filter((n) => n.kind === 'UserDefinedNetwork').length,
                  networksLoading,
                )}
              </CardBody>
            </Card>
          </GridItem>
          <GridItem span={3}>
            <Card isCompact>
              <CardTitle>LocalNet</CardTitle>
              <CardBody>
                {renderCount(
                  networks?.filter((n) => n.topology === 'LocalNet').length,
                  networksLoading,
                )}
              </CardBody>
            </Card>
          </GridItem>
          <GridItem span={3}>
            <Card isCompact>
              <CardTitle>Layer2</CardTitle>
              <CardBody>
                {renderCount(
                  networks?.filter((n) => n.topology === 'Layer2').length,
                  networksLoading,
                )}
              </CardBody>
            </Card>
          </GridItem>
        </Grid>
      </PageSection>

      {!rtLoading && routers && routers.some((r: Router) => r.metricsEnabled) && (
        <PageSection>
          <Title headingLevel="h2" size="lg" style={{ marginBottom: '16px' }}>
            Router Health
          </Title>
          <Grid hasGutter>
            {routers.filter((r: Router) => r.metricsEnabled).map((r: Router) => (
              <GridItem span={4} key={`${r.namespace}/${r.name}`}>
                <Card isCompact isClickable>
                  <CardTitle>
                    <Link to={`/vpc-networking/routers/${r.name}?ns=${encodeURIComponent(r.namespace)}`}>
                      {r.name}
                    </Link>
                  </CardTitle>
                  <CardBody>
                    {r.phase === 'Running' ? (
                      <CheckCircleIcon color="var(--pf-v5-global--success-color--100)" />
                    ) : (
                      <ExclamationCircleIcon color="var(--pf-v5-global--danger-color--100)" />
                    )}
                    {' '}
                    <Label isCompact color={r.phase === 'Running' ? 'green' : 'red'}>{r.phase}</Label>
                    {' '}
                    <Label isCompact color="blue">{r.namespace}</Label>
                    {r.idsMode && (
                      <>{' '}<Label isCompact color="purple">{r.idsMode.toUpperCase()}</Label></>
                    )}
                  </CardBody>
                </Card>
              </GridItem>
            ))}
          </Grid>
        </PageSection>
      )}

      <PageSection>
        <Title headingLevel="h2" size="lg" style={{ marginBottom: '16px' }}>
          Recent Alerts
        </Title>
        <Grid hasGutter>
          <GridItem span={12}>
            <AlertTimelineCard />
          </GridItem>
        </Grid>
      </PageSection>
    </VPCNetworkingShell>
  );
};

VPCDashboardPage.displayName = 'VPCDashboardPage';

export default VPCDashboardPage;
