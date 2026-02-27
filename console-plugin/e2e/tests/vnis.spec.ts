import { test, expect } from '@playwright/test';
import { setupDefaultApiMocks } from '../helpers/api-mocks';

test.describe('VNIs List', () => {
  test.beforeEach(async ({ page }) => {
    await setupDefaultApiMocks(page);
  });

  test('renders VNIs table with mock data', async ({ page }) => {
    await page.goto('/vpc-networking/vnis');

    await expect(page.getByRole('heading', { name: 'Virtual Network Interfaces', level: 1 })).toBeVisible();

    // Table should have rows for our 2 mock VNIs
    const rows = page.locator('tbody tr');
    await expect(rows).toHaveCount(2);

    // Verify VNI names
    await expect(rows.nth(0)).toContainText('vm-web-01-vni');
    await expect(rows.nth(1)).toContainText('vm-db-01-vni');
  });

  test('shows status for each VNI', async ({ page }) => {
    await page.goto('/vpc-networking/vnis');

    const rows = page.locator('tbody tr');
    await expect(rows).toHaveCount(2);

    // Both VNIs have status 'available'
    await expect(rows.nth(0)).toContainText('available');
    await expect(rows.nth(1)).toContainText('available');
  });

  test('shows empty state when no VNIs', async ({ page }) => {
    await page.route('**/api/proxy/plugin/vpc-network-management/bff/api/v1/vnis', (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([]),
      }),
    );

    await page.goto('/vpc-networking/vnis');
    await expect(page.getByText('No VNIs found')).toBeVisible();
  });

  test('Create VNI button is visible', async ({ page }) => {
    await page.goto('/vpc-networking/vnis');
    await expect(page.getByRole('button', { name: /Create VNI/ })).toBeVisible();
  });
});

test.describe('VNI Detail', () => {
  test.beforeEach(async ({ page }) => {
    await setupDefaultApiMocks(page);
  });

  test('displays VNI details', async ({ page }) => {
    await page.goto('/vpc-networking/vnis/vm-web-01-vni');

    await expect(page.getByRole('heading', { name: /VNI: vm-web-01-vni/, level: 1 })).toBeVisible();

    // Breadcrumb should be visible
    await expect(page.getByRole('navigation', { name: 'Breadcrumb' })).toBeVisible();
    await expect(page.getByRole('navigation', { name: 'Breadcrumb' }).getByText('VNIs')).toBeVisible();
  });

  test('shows VNI properties', async ({ page }) => {
    await page.goto('/vpc-networking/vnis/vm-web-01-vni');

    await expect(page.getByRole('heading', { name: /VNI: vm-web-01-vni/, level: 1 })).toBeVisible();

    // Primary IP should be displayed
    await expect(page.getByText('10.240.0.10')).toBeVisible();

    // Allow IP Spoofing should show Yes
    await expect(page.getByText('Allow IP Spoofing')).toBeVisible();
    await expect(page.getByText('Yes').first()).toBeVisible();

    // Infrastructure NAT should show No
    await expect(page.getByText('Infrastructure NAT')).toBeVisible();
  });

  test('shows status on detail page', async ({ page }) => {
    await page.goto('/vpc-networking/vnis/vm-web-01-vni');

    await expect(page.getByRole('heading', { name: /VNI: vm-web-01-vni/, level: 1 })).toBeVisible();

    // Status should be visible
    await expect(page.getByText('available')).toBeVisible();
  });
});
