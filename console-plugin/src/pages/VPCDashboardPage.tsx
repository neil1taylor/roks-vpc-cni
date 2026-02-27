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
} from '@patternfly/react-core';
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
} from '../api/hooks';
import VPCNetworkingShell from '../components/VPCNetworkingShell';

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

  // K8s CR counts — skip VNI/VLAN watches when ROKS-managed
  const { subnets: k8sSubnets, loading: k8sSubnetLoading } = useK8sVPCSubnets();
  const { vnis: k8sVNIs, loading: k8sVNILoading } = useK8sVNIs();
  const { attachments: k8sAttachments, loading: k8sAttLoading } = useK8sVLANAttachments();
  const { floatingIps: k8sFIPs, loading: k8sFIPLoading } = useK8sFloatingIPs();
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
    </VPCNetworkingShell>
  );
};

VPCDashboardPage.displayName = 'VPCDashboardPage';

export default VPCDashboardPage;
