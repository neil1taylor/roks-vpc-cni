import { test, expect } from '@playwright/test';

const NAV_ITEMS = [
  'VPC Dashboard',
  'Networks',
  'VPC Subnets',
  'Virtual Network Interfaces',
  'VLAN Attachments',
  'Floating IPs',
  'Security Groups',
  'Network ACLs',
  'Network Topology',
];

test.describe('Sidebar Navigation', () => {
  test('all 9 VPC nav items are visible under Networking', async ({ page }) => {
    await page.goto('/');

    // Expand the Networking nav section if collapsed
    const networkingSection = page.locator('nav').getByRole('button', { name: 'Networking' });
    if (await networkingSection.isVisible()) {
      const expanded = await networkingSection.getAttribute('aria-expanded');
      if (expanded !== 'true') {
        await networkingSection.click();
      }
    }

    // Verify each nav item is visible in the sidebar
    const sidebar = page.locator('#page-sidebar, nav[aria-label="Nav"]');
    for (const item of NAV_ITEMS) {
      await expect(
        sidebar.getByRole('link', { name: item }),
      ).toBeVisible({ timeout: 15_000 });
    }
  });

  test('clicking a nav item navigates to the correct page', async ({ page }) => {
    await page.goto('/');

    // Expand Networking section
    const networkingSection = page.locator('nav').getByRole('button', { name: 'Networking' });
    if (await networkingSection.isVisible()) {
      const expanded = await networkingSection.getAttribute('aria-expanded');
      if (expanded !== 'true') {
        await networkingSection.click();
      }
    }

    // Click VPC Dashboard and verify navigation
    const sidebar = page.locator('#page-sidebar, nav[aria-label="Nav"]');
    await sidebar.getByRole('link', { name: 'VPC Dashboard' }).click();

    await expect(page).toHaveURL(/\/vpc-networking$/);
    await expect(
      page.getByRole('heading', { name: /VPC Networking Dashboard/, level: 1 }),
    ).toBeVisible({ timeout: 30_000 });
  });
});
