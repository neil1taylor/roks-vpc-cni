import React from 'react';
import {
  Page,
  PageSection,
  PageSectionVariants,
  Title,
  Card,
  CardBody,
  CardTitle,
  Grid,
  GridItem,
  Spinner,
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
} from '../api/hooks';

/**
 * VPC Dashboard Page
 * Displays overview of VPC networking resources
 */
const VPCDashboardPage: React.FC = () => {
  const { vpcs, loading: vpcLoading } = useVPCs();
  const { subnets, loading: subnetLoading } = useSubnets();
  const { securityGroups, loading: sgLoading } = useSecurityGroups();
  const { networkAcls, loading: aclLoading } = useNetworkACLs();
  const { floatingIps, loading: fipLoading } = useFloatingIPs();

  // K8s CR counts
  const { subnets: k8sSubnets, loading: k8sSubnetLoading } = useK8sVPCSubnets();
  const { vnis: k8sVNIs, loading: k8sVNILoading } = useK8sVNIs();
  const { attachments: k8sAttachments, loading: k8sAttLoading } = useK8sVLANAttachments();
  const { floatingIps: k8sFIPs, loading: k8sFIPLoading } = useK8sFloatingIPs();

  const renderCount = (count: number | undefined, loading: boolean) => {
    if (loading) return <Spinner size="md" />;
    return <span style={{ fontSize: '2rem', fontWeight: 600 }}>{count ?? 0}</span>;
  };

  return (
    <Page>
      <PageSection variant={PageSectionVariants.light}>
        <Title headingLevel="h1">VPC Networking Dashboard</Title>
      </PageSection>

      <PageSection>
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
              <CardBody>{renderCount(k8sVNIs?.length, k8sVNILoading)}</CardBody>
            </Card>
          </GridItem>
          <GridItem span={3}>
            <Card isCompact>
              <CardTitle>VLAN Attachments</CardTitle>
              <CardBody>{renderCount(k8sAttachments?.length, k8sAttLoading)}</CardBody>
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
    </Page>
  );
};

VPCDashboardPage.displayName = 'VPCDashboardPage';

export default VPCDashboardPage;
