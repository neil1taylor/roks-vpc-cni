import { test, expect } from '@playwright/test';
import { setupDefaultApiMocks } from '../helpers/api-mocks';

test.describe('Security Groups List', () => {
  test.beforeEach(async ({ page }) => {
    await setupDefaultApiMocks(page);
  });

  test('renders security groups table with mock data', async ({ page }) => {
    await page.goto('/vpc-networking/security-groups');

    await expect(page.getByRole('heading', { name: 'Security Groups', level: 1 })).toBeVisible();

    // Table should have rows for our 2 mock security groups
    const rows = page.locator('tbody tr');
    await expect(rows).toHaveCount(2);

    // Verify security group names
    await expect(rows.nth(0)).toContainText('web-sg');
    await expect(rows.nth(1)).toContainText('db-sg');
  });

  test('shows VPC name and rules count', async ({ page }) => {
    await page.goto('/vpc-networking/security-groups');

    const rows = page.locator('tbody tr');
    await expect(rows).toHaveCount(2);

    // web-sg has 2 rules, in prod-vpc
    await expect(rows.nth(0)).toContainText('prod-vpc');
    await expect(rows.nth(0)).toContainText('2');

    // db-sg has 1 rule, in prod-vpc
    await expect(rows.nth(1)).toContainText('prod-vpc');
    await expect(rows.nth(1)).toContainText('1');
  });

  test('clicking a security group name navigates to detail page', async ({ page }) => {
    await page.goto('/vpc-networking/security-groups');

    // Wait for table to render
    await expect(page.locator('tbody tr')).toHaveCount(2);

    // Click the first security group link
    const link = page.locator('a', { hasText: 'web-sg' });
    await link.click();

    // Should navigate to the detail page
    await expect(page).toHaveURL(/\/vpc-networking\/security-groups\/web-sg/);
    await expect(page.getByRole('heading', { name: /Security Group: web-sg/, level: 1 })).toBeVisible();
  });

  test('shows empty state when no security groups', async ({ page }) => {
    await page.route('**/api/proxy/plugin/vpc-network-management/bff/api/v1/security-groups', (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([]),
      }),
    );

    await page.goto('/vpc-networking/security-groups');
    await expect(page.getByText('No security groups found')).toBeVisible();
  });
});

test.describe('Security Group Detail', () => {
  test.beforeEach(async ({ page }) => {
    await setupDefaultApiMocks(page);
  });

  test('displays security group details', async ({ page }) => {
    await page.goto('/vpc-networking/security-groups/web-sg');

    await expect(page.getByRole('heading', { name: /Security Group: web-sg/, level: 1 })).toBeVisible();

    // Breadcrumb should be visible
    await expect(page.getByRole('navigation', { name: 'Breadcrumb' })).toBeVisible();
    await expect(page.getByRole('navigation', { name: 'Breadcrumb' }).getByText('Security Groups')).toBeVisible();
  });

  test('displays rules section', async ({ page }) => {
    await page.goto('/vpc-networking/security-groups/web-sg');

    await expect(page.getByRole('heading', { name: /Security Group: web-sg/, level: 1 })).toBeVisible();

    // Rules card should be visible
    await expect(page.getByText('Rules')).toBeVisible();
  });

  test('shows description and VPC info', async ({ page }) => {
    await page.goto('/vpc-networking/security-groups/web-sg');

    await expect(page.getByRole('heading', { name: /Security Group: web-sg/, level: 1 })).toBeVisible();

    // VPC name should be shown
    await expect(page.getByText('prod-vpc')).toBeVisible();

    // Description should be visible
    await expect(page.getByText('Security group for web tier')).toBeVisible();
  });
});
