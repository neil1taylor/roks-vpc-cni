import React from 'react';
import {
  PageSection,
  PageSectionVariants,
  Card,
  CardBody,
  CardTitle,
  Breadcrumb,
  BreadcrumbItem,
  Spinner,
  Label,
  DescriptionList,
  DescriptionListGroup,
  DescriptionListTerm,
  DescriptionListDescription,
  Button,
} from '@patternfly/react-core';
import { useParams, useSearchParams } from 'react-router-dom-v5-compat';
import { useNetworkDefinition } from '../api/hooks';
import TierBadge from '../components/TierBadge';
import IPModeInfoAlert from '../components/IPModeInfoAlert';
import VPCNetworkingShell from '../components/VPCNetworkingShell';

const ipModeLabels: Record<string, string> = {
  static_reserved: 'Static Reserved IP',
  dhcp: 'DHCP',
  none: 'Manual',
};

const NetworkDetailPage: React.FC = () => {
  const { name } = useParams<{ name: string }>();
  const [searchParams] = useSearchParams();
  const kind = searchParams.get('kind') || 'ClusterUserDefinedNetwork';
  const ns = searchParams.get('ns') || undefined;

  const { network, loading } = useNetworkDefinition(name || '', kind, ns);

  return (
    <VPCNetworkingShell>
      <PageSection variant={PageSectionVariants.light}>
        <Breadcrumb>
          <BreadcrumbItem href="/vpc-networking/networks">Networks</BreadcrumbItem>
          <BreadcrumbItem isActive>{name}</BreadcrumbItem>
        </Breadcrumb>
      </PageSection>

      <PageSection>
        {loading ? (
          <Spinner size="lg" />
        ) : network ? (
          <>
            {/* Overview Section */}
            <Card style={{ marginBottom: '16px' }}>
              <CardTitle>Overview</CardTitle>
              <CardBody>
                <DescriptionList isHorizontal>
                  <DescriptionListGroup>
                    <DescriptionListTerm>Name</DescriptionListTerm>
                    <DescriptionListDescription>{network.name}</DescriptionListDescription>
                  </DescriptionListGroup>
                  <DescriptionListGroup>
                    <DescriptionListTerm>Kind</DescriptionListTerm>
                    <DescriptionListDescription>
                      {network.kind === 'ClusterUserDefinedNetwork' ? (
                        <Label color="purple">ClusterUserDefinedNetwork</Label>
                      ) : (
                        <Label color="cyan">UserDefinedNetwork</Label>
                      )}
                    </DescriptionListDescription>
                  </DescriptionListGroup>
                  <DescriptionListGroup>
                    <DescriptionListTerm>Topology</DescriptionListTerm>
                    <DescriptionListDescription>
                      {network.topology === 'LocalNet' ? (
                        <Label color="blue">LocalNet</Label>
                      ) : (
                        <Label color="green">Layer2</Label>
                      )}
                    </DescriptionListDescription>
                  </DescriptionListGroup>
                  <DescriptionListGroup>
                    <DescriptionListTerm>Role</DescriptionListTerm>
                    <DescriptionListDescription>{network.role || 'Secondary'}</DescriptionListDescription>
                  </DescriptionListGroup>
                  {network.tier && (
                    <DescriptionListGroup>
                      <DescriptionListTerm>Tier</DescriptionListTerm>
                      <DescriptionListDescription>
                        <TierBadge tier={network.tier} />
                      </DescriptionListDescription>
                    </DescriptionListGroup>
                  )}
                  {network.namespace && (
                    <DescriptionListGroup>
                      <DescriptionListTerm>Namespace</DescriptionListTerm>
                      <DescriptionListDescription>{network.namespace}</DescriptionListDescription>
                    </DescriptionListGroup>
                  )}
                  <DescriptionListGroup>
                    <DescriptionListTerm>Status</DescriptionListTerm>
                    <DescriptionListDescription>
                      {network.topology === 'LocalNet' ? (
                        <Label color={network.subnet_status === 'active' ? 'green' : 'orange'}>
                          {network.subnet_status || 'pending'}
                        </Label>
                      ) : (
                        <Label color="green">Active</Label>
                      )}
                    </DescriptionListDescription>
                  </DescriptionListGroup>
                </DescriptionList>
              </CardBody>
            </Card>

            {/* IP Assignment Section */}
            {network.ip_mode && (
              <Card style={{ marginBottom: '16px' }}>
                <CardTitle>IP Assignment</CardTitle>
                <CardBody>
                  <DescriptionList isHorizontal>
                    <DescriptionListGroup>
                      <DescriptionListTerm>Mode</DescriptionListTerm>
                      <DescriptionListDescription>
                        {ipModeLabels[network.ip_mode] || network.ip_mode}
                      </DescriptionListDescription>
                    </DescriptionListGroup>
                  </DescriptionList>
                  <div style={{ marginTop: '16px' }}>
                    <IPModeInfoAlert mode={network.ip_mode} />
                  </div>
                </CardBody>
              </Card>
            )}

            {/* VPC Resources Section (LocalNet only) */}
            {network.topology === 'LocalNet' && (
              <Card>
                <CardTitle>VPC Resources</CardTitle>
                <CardBody>
                  <DescriptionList isHorizontal>
                    {network.subnet_id && (
                      <DescriptionListGroup>
                        <DescriptionListTerm>VPC Subnet</DescriptionListTerm>
                        <DescriptionListDescription>
                          <Button
                            variant="link"
                            isInline
                            component="a"
                            href={`/vpc-networking/subnets/${network.subnet_id}`}
                          >
                            {network.subnet_id}
                          </Button>
                        </DescriptionListDescription>
                      </DescriptionListGroup>
                    )}
                    {network.vpc_id && (
                      <DescriptionListGroup>
                        <DescriptionListTerm>VPC</DescriptionListTerm>
                        <DescriptionListDescription>{network.vpc_id}</DescriptionListDescription>
                      </DescriptionListGroup>
                    )}
                    {network.zone && (
                      <DescriptionListGroup>
                        <DescriptionListTerm>Zone</DescriptionListTerm>
                        <DescriptionListDescription>{network.zone}</DescriptionListDescription>
                      </DescriptionListGroup>
                    )}
                    {network.cidr && (
                      <DescriptionListGroup>
                        <DescriptionListTerm>CIDR</DescriptionListTerm>
                        <DescriptionListDescription>{network.cidr}</DescriptionListDescription>
                      </DescriptionListGroup>
                    )}
                    {network.vlan_id && (
                      <DescriptionListGroup>
                        <DescriptionListTerm>VLAN ID</DescriptionListTerm>
                        <DescriptionListDescription>{network.vlan_id}</DescriptionListDescription>
                      </DescriptionListGroup>
                    )}
                    {network.vlan_attachments && (
                      <DescriptionListGroup>
                        <DescriptionListTerm>VLAN Attachments</DescriptionListTerm>
                        <DescriptionListDescription>{network.vlan_attachments}</DescriptionListDescription>
                      </DescriptionListGroup>
                    )}
                  </DescriptionList>
                </CardBody>
              </Card>
            )}
          </>
        ) : (
          <Card>
            <CardBody>Network not found</CardBody>
          </Card>
        )}
      </PageSection>
    </VPCNetworkingShell>
  );
};

NetworkDetailPage.displayName = 'NetworkDetailPage';
export default NetworkDetailPage;
