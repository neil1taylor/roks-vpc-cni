import { test, expect } from '@playwright/test';

/**
 * Verify each plugin route renders the expected h1 heading on the live console.
 * Uses 30s timeout per heading to account for Module Federation async loading.
 */

const routes: Array<{ path: string; heading: RegExp }> = [
  { path: '/vpc-networking', heading: /VPC Networking Dashboard/ },
  { path: '/vpc-networking/networks', heading: /Networks/ },
  { path: '/vpc-networking/subnets', heading: /VPC Subnets/ },
  { path: '/vpc-networking/vnis', heading: /Virtual Network Interfaces/ },
  { path: '/vpc-networking/vlan-attachments', heading: /VLAN Attachments/ },
  { path: '/vpc-networking/floating-ips', heading: /Floating IPs/ },
  { path: '/vpc-networking/security-groups', heading: /Security Groups/ },
  { path: '/vpc-networking/network-acls', heading: /Network ACLs/ },
  { path: '/vpc-networking/topology', heading: /Network Topology/ },
];

for (const { path, heading } of routes) {
  test(`renders heading at ${path}`, async ({ page }) => {
    await page.goto(path);
    await expect(
      page.getByRole('heading', { name: heading, level: 1 }),
    ).toBeVisible({ timeout: 30_000 });
  });
}
