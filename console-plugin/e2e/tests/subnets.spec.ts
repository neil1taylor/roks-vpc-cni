import { test, expect } from '@playwright/test';
import { setupDefaultApiMocks } from '../helpers/api-mocks';

test.describe('Subnets List', () => {
  test.beforeEach(async ({ page }) => {
    await setupDefaultApiMocks(page);
  });

  test('renders subnets table with mock data', async ({ page }) => {
    await page.goto('/vpc-networking/subnets');

    await expect(page.getByRole('heading', { name: 'VPC Subnets', level: 1 })).toBeVisible();

    // Table should have rows for our 3 mock subnets
    const rows = page.locator('tbody tr');
    await expect(rows).toHaveCount(3);

    // Verify first subnet data
    await expect(rows.nth(0)).toContainText('prod-subnet-1');
    await expect(rows.nth(0)).toContainText('prod-vpc');
    await expect(rows.nth(0)).toContainText('us-south-1');
    await expect(rows.nth(0)).toContainText('10.240.0.0/24');
  });

  test('clicking a subnet name navigates to detail page', async ({ page }) => {
    await page.goto('/vpc-networking/subnets');

    // Wait for table to render
    await expect(page.locator('tbody tr')).toHaveCount(3);

    // Click the first subnet link — it's an <a> tag, so we click it
    const link = page.locator('a', { hasText: 'prod-subnet-1' });
    await link.click();

    // Should navigate to the detail page
    await expect(page).toHaveURL(/\/vpc-networking\/subnets\/prod-subnet-1/);
    await expect(page.getByRole('heading', { name: /Subnet: prod-subnet-1/, level: 1 })).toBeVisible();
  });

  test('shows empty state when no subnets', async ({ page }) => {
    // Override subnets endpoint to return empty array
    await page.route('**/api/proxy/plugin/vpc-network-management/bff/api/v1/subnets', (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([]),
      }),
    );

    await page.goto('/vpc-networking/subnets');
    await expect(page.getByText('No subnets found')).toBeVisible();
  });

  test('Create Subnet button is visible', async ({ page }) => {
    await page.goto('/vpc-networking/subnets');
    await expect(page.getByRole('button', { name: /Create Subnet/ })).toBeVisible();
  });
});

test.describe('Subnet Detail', () => {
  test.beforeEach(async ({ page }) => {
    await setupDefaultApiMocks(page);
  });

  test('displays subnet details', async ({ page }) => {
    await page.goto('/vpc-networking/subnets/prod-subnet-1');

    await expect(page.getByRole('heading', { name: /Subnet: prod-subnet-1/, level: 1 })).toBeVisible();

    // Breadcrumb should be visible
    await expect(page.getByRole('navigation', { name: 'Breadcrumb' })).toBeVisible();
    await expect(page.getByRole('navigation', { name: 'Breadcrumb' }).getByText('Subnets')).toBeVisible();
  });
});
