import React, { useState } from 'react';
import {
  Page,
  PageSection,
  PageSectionVariants,
  Title,
  Tabs,
  Tab,
  TabTitleText,
  Alert,
  AlertVariant,
  AlertActionCloseButton,
} from '@patternfly/react-core';
import { useLocation, useNavigate } from 'react-router-dom-v5-compat';
import { useClusterInfo } from '../api/hooks';

const tabs = [
  { key: 'dashboard', label: 'Dashboard', path: '/vpc-networking' },
  { key: 'networks', label: 'Networks', path: '/vpc-networking/networks' },
  { key: 'subnets', label: 'Subnets', path: '/vpc-networking/subnets' },
  { key: 'vnis', label: 'VNIs', path: '/vpc-networking/vnis' },
  { key: 'vlan-attachments', label: 'VLAN Attachments', path: '/vpc-networking/vlan-attachments' },
  { key: 'floating-ips', label: 'Floating IPs', path: '/vpc-networking/floating-ips' },
  { key: 'pars', label: 'PARs', path: '/vpc-networking/pars' },
  { key: 'security-groups', label: 'Security Groups', path: '/vpc-networking/security-groups' },
  { key: 'network-acls', label: 'Network ACLs', path: '/vpc-networking/network-acls' },
  { key: 'routes', label: 'Routes', path: '/vpc-networking/routes' },
  { key: 'gateways', label: 'Gateways', path: '/vpc-networking/gateways' },
  { key: 'routers', label: 'Routers', path: '/vpc-networking/routers' },
  { key: 'l2-bridges', label: 'L2 Bridges', path: '/vpc-networking/l2-bridges' },
  { key: 'vpn-gateways', label: 'VPN Gateways', path: '/vpc-networking/vpn-gateways' },
  { key: 'topology', label: 'Topology', path: '/vpc-networking/topology' },
];

const VPCNetworkingShell: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const location = useLocation();
  const navigate = useNavigate();
  const { clusterInfo } = useClusterInfo();
  const [bannerDismissed, setBannerDismissed] = useState(false);

  const showROKSBanner =
    !bannerDismissed &&
    clusterInfo?.clusterMode === 'roks' &&
    !clusterInfo.features.roksAPIAvailable;

  // Longest prefix match to find active tab
  const activeTab = [...tabs]
    .sort((a, b) => b.path.length - a.path.length)
    .find((tab) => location.pathname.startsWith(tab.path))?.key || 'dashboard';

  const handleTabSelect = (_event: React.MouseEvent<HTMLElement>, tabKey: string | number) => {
    const tab = tabs.find((t) => t.key === tabKey);
    if (tab) {
      navigate(tab.path);
    }
  };

  return (
    <Page>
      <PageSection variant={PageSectionVariants.light}>
        <Title headingLevel="h1" style={{ marginBottom: '16px' }}>IBM VPC Networking</Title>
        <Tabs activeKey={activeTab} onSelect={handleTabSelect}>
          {tabs.map((tab) => (
            <Tab
              key={tab.key}
              eventKey={tab.key}
              title={<TabTitleText>{tab.label}</TabTitleText>}
            />
          ))}
        </Tabs>
      </PageSection>
      {showROKSBanner && (
        <PageSection variant={PageSectionVariants.light} padding={{ default: 'noPadding' }} style={{ paddingLeft: '24px', paddingRight: '24px', paddingTop: '16px' }}>
          <Alert
            variant={AlertVariant.info}
            isInline
            title="ROKS-Managed Cluster"
            actionClose={<AlertActionCloseButton onClose={() => setBannerDismissed(true)} />}
          >
            This cluster is managed by the ROKS platform. VNI and VLAN attachment management is not
            yet available through this console and will be enabled in a future release.
          </Alert>
        </PageSection>
      )}
      {children}
    </Page>
  );
};

export default VPCNetworkingShell;
