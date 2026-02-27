import { test, expect } from '@playwright/test';
import { setupDefaultApiMocks } from '../helpers/api-mocks';

/**
 * Verify that direct URL navigation to each route renders the expected page heading.
 * This validates that the harness router is configured correctly for all 12 pages.
 */

const routes: Array<{ path: string; heading: RegExp }> = [
  { path: '/vpc-networking', heading: /VPC Networking Dashboard/ },
  { path: '/vpc-networking/subnets', heading: /VPC Subnets/ },
  { path: '/vpc-networking/subnets/prod-subnet-1', heading: /Subnet: prod-subnet-1/ },
  { path: '/vpc-networking/vnis', heading: /Virtual Network Interfaces/ },
  { path: '/vpc-networking/vnis/vm-web-01-vni', heading: /VNI: vm-web-01-vni/ },
  { path: '/vpc-networking/vlan-attachments', heading: /VLAN Attachments/ },
  { path: '/vpc-networking/floating-ips', heading: /Floating IPs/ },
  { path: '/vpc-networking/security-groups', heading: /Security Groups/ },
  { path: '/vpc-networking/security-groups/web-sg', heading: /Security Group: web-sg/ },
  { path: '/vpc-networking/network-acls', heading: /Network ACLs/ },
  { path: '/vpc-networking/network-acls/prod-acl', heading: /Network ACL: prod-acl/ },
  { path: '/vpc-networking/networks', heading: /Networks/ },
  { path: '/vpc-networking/networks/localnet-cudn', heading: /Network:.*localnet-cudn/ },
  { path: '/vpc-networking/topology', heading: /Network Topology/ },
];

for (const { path, heading } of routes) {
  test(`navigates to ${path}`, async ({ page }) => {
    await setupDefaultApiMocks(page);
    await page.goto(path);
    await expect(page.getByRole('heading', { name: heading, level: 1 })).toBeVisible();
  });
}
