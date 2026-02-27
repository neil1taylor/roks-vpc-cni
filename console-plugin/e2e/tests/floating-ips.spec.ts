import { test, expect } from '@playwright/test';
import { setupDefaultApiMocks } from '../helpers/api-mocks';

test.describe('Floating IPs List', () => {
  test.beforeEach(async ({ page }) => {
    await setupDefaultApiMocks(page);
  });

  test('renders floating IPs table with mock data', async ({ page }) => {
    await page.goto('/vpc-networking/floating-ips');

    await expect(page.getByRole('heading', { name: 'Floating IPs', level: 1 })).toBeVisible();

    // Table should have rows for our 2 mock floating IPs
    const rows = page.locator('tbody tr');
    await expect(rows).toHaveCount(2);

    // Verify floating IP names
    await expect(rows.nth(0)).toContainText('web-public-ip');
    await expect(rows.nth(1)).toContainText('api-public-ip');
  });

  test('shows address and zone for each floating IP', async ({ page }) => {
    await page.goto('/vpc-networking/floating-ips');

    const rows = page.locator('tbody tr');
    await expect(rows).toHaveCount(2);

    // First FIP: address and zone
    await expect(rows.nth(0)).toContainText('169.48.100.1');
    await expect(rows.nth(0)).toContainText('us-south-1');

    // Second FIP: address and zone
    await expect(rows.nth(1)).toContainText('169.48.100.2');
    await expect(rows.nth(1)).toContainText('us-south-2');
  });

  test('shows target info when available', async ({ page }) => {
    await page.goto('/vpc-networking/floating-ips');

    const rows = page.locator('tbody tr');
    await expect(rows).toHaveCount(2);

    // First FIP has a target VNI
    await expect(rows.nth(0)).toContainText('vm-web-01-vni');

    // Second FIP has no target
    await expect(rows.nth(1)).toContainText('-');
  });

  test('shows empty state when no floating IPs', async ({ page }) => {
    await page.route('**/api/proxy/plugin/vpc-network-management/bff/api/v1/floating-ips', (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([]),
      }),
    );

    await page.goto('/vpc-networking/floating-ips');
    await expect(page.getByText('No floating IPs found')).toBeVisible();
  });

  test('Reserve Floating IP button is visible', async ({ page }) => {
    await page.goto('/vpc-networking/floating-ips');
    await expect(page.getByRole('button', { name: /Reserve Floating IP/ })).toBeVisible();
  });
});
