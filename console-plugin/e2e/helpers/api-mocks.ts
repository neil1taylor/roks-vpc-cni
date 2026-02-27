import { Page } from '@playwright/test';
import { mockVPCs } from '../fixtures/vpcs';
import { mockSubnets } from '../fixtures/subnets';
import { mockVNIs } from '../fixtures/vnis';
import { mockFloatingIPs } from '../fixtures/floating-ips';
import { mockSecurityGroups } from '../fixtures/security-groups';
import { mockNetworkACLs } from '../fixtures/network-acls';
import { mockTopology } from '../fixtures/topology';
import { mockCUDNs, mockUDNs } from '../fixtures/networks';
import { mockNetworkTypes } from '../fixtures/network-types';

const BFF_PREFIX = '**/api/proxy/plugin/vpc-network-management/bff/api/v1';

// The BFF client's request<T> method does:
//   const data = (await response.json()) as T;
//   return { data };
// So mock endpoints must return the raw payload (not wrapped in { data }).

function json(body: unknown, status = 200) {
  return {
    status,
    contentType: 'application/json',
    body: JSON.stringify(body),
  };
}

/**
 * Register page.route() handlers for all BFF endpoints.
 * Call this in beforeEach to ensure every page has API data available.
 */
export async function setupDefaultApiMocks(page: Page): Promise<void> {
  // Cluster info
  await page.route(`${BFF_PREFIX}/cluster-info`, (route) =>
    route.fulfill(json({
      clusterMode: 'unmanaged',
      features: {
        vniManagement: true,
        vlanAttachmentManagement: true,
        subnetManagement: true,
        securityGroupManagement: true,
        networkACLManagement: true,
        floatingIPManagement: true,
        roksAPIAvailable: false,
      },
    })),
  );

  // Auth permissions (BFF path)
  await page.route(`${BFF_PREFIX}/auth/permissions`, (route) =>
    route.fulfill(json({ isAdmin: true, canWrite: true, canDelete: true })),
  );

  // Auth permissions (usePermissions.ts uses /api/v1/auth/permissions directly)
  await page.route('**/api/v1/auth/permissions', (route) =>
    route.fulfill(json({ isAdmin: true, canWrite: true, canDelete: true })),
  );

  // VPCs
  await page.route(`${BFF_PREFIX}/vpcs`, (route) =>
    route.fulfill(json(mockVPCs)),
  );

  // Subnets — detail routes must be registered BEFORE the list route
  // because Playwright matches routes in registration order
  await page.route(`${BFF_PREFIX}/subnets/*`, (route) => {
    const url = route.request().url();
    const id = url.split('/').pop()!.split('?')[0];
    const subnet = mockSubnets.find((s) => s.id === id || s.name === id);
    return route.fulfill(subnet
      ? json(subnet)
      : json({ code: 'NOT_FOUND', message: 'Subnet not found' }, 404));
  });

  await page.route(`${BFF_PREFIX}/subnets`, (route) =>
    route.fulfill(json(mockSubnets)),
  );

  // VNIs
  await page.route(`${BFF_PREFIX}/vnis/*`, (route) => {
    const url = route.request().url();
    const id = url.split('/').pop()!.split('?')[0];
    const vni = mockVNIs.find((v) => v.id === id || v.name === id);
    return route.fulfill(vni
      ? json(vni)
      : json({ code: 'NOT_FOUND', message: 'VNI not found' }, 404));
  });

  await page.route(`${BFF_PREFIX}/vnis`, (route) =>
    route.fulfill(json(mockVNIs)),
  );

  // Floating IPs
  await page.route(`${BFF_PREFIX}/floating-ips/*`, (route) => {
    const url = route.request().url();
    const id = url.split('/').pop()!.split('?')[0];
    const fip = mockFloatingIPs.find((f) => f.id === id || f.name === id);
    return route.fulfill(fip
      ? json(fip)
      : json({ code: 'NOT_FOUND', message: 'Floating IP not found' }, 404));
  });

  await page.route(`${BFF_PREFIX}/floating-ips`, (route) =>
    route.fulfill(json(mockFloatingIPs)),
  );

  // Security Groups
  await page.route(`${BFF_PREFIX}/security-groups/*`, (route) => {
    const url = route.request().url();
    const id = url.split('/').pop()!.split('?')[0];
    const sg = mockSecurityGroups.find((s) => s.id === id || s.name === id);
    return route.fulfill(sg
      ? json(sg)
      : json({ code: 'NOT_FOUND', message: 'Security group not found' }, 404));
  });

  await page.route(`${BFF_PREFIX}/security-groups`, (route) =>
    route.fulfill(json(mockSecurityGroups)),
  );

  // Network ACLs
  await page.route(`${BFF_PREFIX}/network-acls/*`, (route) => {
    const url = route.request().url();
    const id = url.split('/').pop()!.split('?')[0];
    const acl = mockNetworkACLs.find((a) => a.id === id || a.name === id);
    return route.fulfill(acl
      ? json(acl)
      : json({ code: 'NOT_FOUND', message: 'Network ACL not found' }, 404));
  });

  await page.route(`${BFF_PREFIX}/network-acls`, (route) =>
    route.fulfill(json(mockNetworkACLs)),
  );

  // Topology
  await page.route(`${BFF_PREFIX}/topology`, (route) =>
    route.fulfill(json(mockTopology)),
  );

  // CUDNs
  await page.route(`${BFF_PREFIX}/cudns/*`, (route) => {
    const url = route.request().url();
    const name = url.split('/').pop()!.split('?')[0];
    const cudn = mockCUDNs.find((c) => c.name === name);
    return route.fulfill(cudn
      ? json(cudn)
      : json({ code: 'NOT_FOUND', message: 'CUDN not found' }, 404));
  });

  await page.route(`${BFF_PREFIX}/cudns`, (route) =>
    route.fulfill(json(mockCUDNs)),
  );

  // UDNs
  await page.route(`${BFF_PREFIX}/udns/**`, (route) => {
    const url = route.request().url();
    const segments = url.split('/');
    const name = segments.pop()!.split('?')[0];
    const udn = mockUDNs.find((u) => u.name === name);
    return route.fulfill(udn
      ? json(udn)
      : json({ code: 'NOT_FOUND', message: 'UDN not found' }, 404));
  });

  await page.route(`${BFF_PREFIX}/udns`, (route) =>
    route.fulfill(json(mockUDNs)),
  );

  // Network types
  await page.route(`${BFF_PREFIX}/network-types`, (route) =>
    route.fulfill(json(mockNetworkTypes)),
  );

  // Zones
  await page.route(`${BFF_PREFIX}/zones`, (route) =>
    route.fulfill(json([
      { id: 'us-south-1', name: 'us-south-1', status: 'available' },
      { id: 'us-south-2', name: 'us-south-2', status: 'available' },
      { id: 'us-south-3', name: 'us-south-3', status: 'available' },
    ])),
  );
}

/**
 * Override a specific API endpoint to return an error.
 */
export async function mockApiError(
  page: Page,
  urlPattern: string,
  statusCode: number,
  message = 'Mock error',
): Promise<void> {
  await page.route(urlPattern, (route) =>
    route.fulfill(json({ code: `ERROR_${statusCode}`, message }, statusCode)),
  );
}
