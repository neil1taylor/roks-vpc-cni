// Plugin extensions are defined in console-extensions.json
// This file provides the route configuration for the dynamic plugin

// Lazy load page components
const VPCDashboard = () => import('./pages/VPCDashboardPage').then(m => ({ default: m.default }));
const SubnetsList = () => import('./pages/SubnetsListPage').then(m => ({ default: m.default }));
const SubnetDetail = () => import('./pages/SubnetDetailPage').then(m => ({ default: m.default }));
const VNIsList = () => import('./pages/VNIsListPage').then(m => ({ default: m.default }));
const VNIDetail = () => import('./pages/VNIDetailPage').then(m => ({ default: m.default }));
const VLANAttachments = () => import('./pages/VLANAttachmentsPage').then(m => ({ default: m.default }));
const FloatingIPs = () => import('./pages/FloatingIPsPage').then(m => ({ default: m.default }));
const SecurityGroupsList = () => import('./pages/SecurityGroupsListPage').then(m => ({ default: m.default }));
const SecurityGroupDetail = () => import('./pages/SecurityGroupDetailPage').then(m => ({ default: m.default }));
const NetworkACLsList = () => import('./pages/NetworkACLsListPage').then(m => ({ default: m.default }));
const NetworkACLDetail = () => import('./pages/NetworkACLDetailPage').then(m => ({ default: m.default }));
const Topology = () => import('./pages/TopologyPage').then(m => ({ default: m.default }));

const plugin: any[] = [
  {
    type: 'console.page/route',
    properties: {
      exact: true,
      path: '/vpc-networking',
      component: VPCDashboard,
    },
  },
  {
    type: 'console.page/route',
    properties: {
      exact: true,
      path: '/vpc-networking/subnets',
      component: SubnetsList,
    },
  },
  {
    type: 'console.page/route',
    properties: {
      exact: true,
      path: '/vpc-networking/subnets/:name',
      component: SubnetDetail,
    },
  },
  {
    type: 'console.page/route',
    properties: {
      exact: true,
      path: '/vpc-networking/vnis',
      component: VNIsList,
    },
  },
  {
    type: 'console.page/route',
    properties: {
      exact: true,
      path: '/vpc-networking/vnis/:name',
      component: VNIDetail,
    },
  },
  {
    type: 'console.page/route',
    properties: {
      exact: true,
      path: '/vpc-networking/vlan-attachments',
      component: VLANAttachments,
    },
  },
  {
    type: 'console.page/route',
    properties: {
      exact: true,
      path: '/vpc-networking/floating-ips',
      component: FloatingIPs,
    },
  },
  {
    type: 'console.page/route',
    properties: {
      exact: true,
      path: '/vpc-networking/security-groups',
      component: SecurityGroupsList,
    },
  },
  {
    type: 'console.page/route',
    properties: {
      exact: true,
      path: '/vpc-networking/security-groups/:name',
      component: SecurityGroupDetail,
    },
  },
  {
    type: 'console.page/route',
    properties: {
      exact: true,
      path: '/vpc-networking/network-acls',
      component: NetworkACLsList,
    },
  },
  {
    type: 'console.page/route',
    properties: {
      exact: true,
      path: '/vpc-networking/network-acls/:name',
      component: NetworkACLDetail,
    },
  },
  {
    type: 'console.page/route',
    properties: {
      exact: true,
      path: '/vpc-networking/topology',
      component: Topology,
    },
  },
];

export default plugin;
