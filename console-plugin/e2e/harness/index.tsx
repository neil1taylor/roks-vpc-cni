import React from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter, Routes, Route } from 'react-router-dom';

import '@patternfly/react-core/dist/styles/base.css';

// Page components
import VPCDashboardPage from '../../src/pages/VPCDashboardPage';
import SubnetsListPage from '../../src/pages/SubnetsListPage';
import SubnetDetailPage from '../../src/pages/SubnetDetailPage';
import VNIsListPage from '../../src/pages/VNIsListPage';
import VNIDetailPage from '../../src/pages/VNIDetailPage';
import VLANAttachmentsPage from '../../src/pages/VLANAttachmentsPage';
import FloatingIPsPage from '../../src/pages/FloatingIPsPage';
import SecurityGroupsListPage from '../../src/pages/SecurityGroupsListPage';
import SecurityGroupDetailPage from '../../src/pages/SecurityGroupDetailPage';
import NetworkACLsListPage from '../../src/pages/NetworkACLsListPage';
import NetworkACLDetailPage from '../../src/pages/NetworkACLDetailPage';
import TopologyPage from '../../src/pages/TopologyPage';

// Initialise K8s watch data store
window.__K8S_WATCH_DATA__ = window.__K8S_WATCH_DATA__ || {};

const App: React.FC = () => (
  <BrowserRouter>
    <Routes>
      <Route path="/vpc-networking" element={<VPCDashboardPage />} />
      <Route path="/vpc-networking/subnets" element={<SubnetsListPage />} />
      <Route path="/vpc-networking/subnets/:name" element={<SubnetDetailPage />} />
      <Route path="/vpc-networking/vnis" element={<VNIsListPage />} />
      <Route path="/vpc-networking/vnis/:name" element={<VNIDetailPage />} />
      <Route path="/vpc-networking/vlan-attachments" element={<VLANAttachmentsPage />} />
      <Route path="/vpc-networking/floating-ips" element={<FloatingIPsPage />} />
      <Route path="/vpc-networking/security-groups" element={<SecurityGroupsListPage />} />
      <Route path="/vpc-networking/security-groups/:name" element={<SecurityGroupDetailPage />} />
      <Route path="/vpc-networking/network-acls" element={<NetworkACLsListPage />} />
      <Route path="/vpc-networking/network-acls/:name" element={<NetworkACLDetailPage />} />
      <Route path="/vpc-networking/topology" element={<TopologyPage />} />
    </Routes>
  </BrowserRouter>
);

const container = document.getElementById('root');
if (container) {
  const root = createRoot(container);
  root.render(<App />);
}
