import { test, expect } from '@playwright/test';
import { setupDefaultApiMocks } from '../helpers/api-mocks';

test.describe('Network ACLs List', () => {
  test.beforeEach(async ({ page }) => {
    await setupDefaultApiMocks(page);
  });

  test('renders network ACLs table with mock data', async ({ page }) => {
    await page.goto('/vpc-networking/network-acls');

    await expect(page.getByRole('heading', { name: 'Network ACLs', level: 1 })).toBeVisible();

    // Table should have rows for our 2 mock ACLs
    const rows = page.locator('tbody tr');
    await expect(rows).toHaveCount(2);

    // Verify ACL names
    await expect(rows.nth(0)).toContainText('prod-acl');
    await expect(rows.nth(1)).toContainText('dev-acl');
  });

  test('shows VPC name and rules count', async ({ page }) => {
    await page.goto('/vpc-networking/network-acls');

    const rows = page.locator('tbody tr');
    await expect(rows).toHaveCount(2);

    // prod-acl has 2 rules, in prod-vpc
    await expect(rows.nth(0)).toContainText('prod-vpc');
    await expect(rows.nth(0)).toContainText('2');

    // dev-acl has 0 rules, in dev-vpc
    await expect(rows.nth(1)).toContainText('dev-vpc');
    await expect(rows.nth(1)).toContainText('0');
  });

  test('clicking an ACL name navigates to detail page', async ({ page }) => {
    await page.goto('/vpc-networking/network-acls');

    // Wait for table to render
    await expect(page.locator('tbody tr')).toHaveCount(2);

    // Click the first ACL link
    const link = page.locator('a', { hasText: 'prod-acl' });
    await link.click();

    // Should navigate to the detail page
    await expect(page).toHaveURL(/\/vpc-networking\/network-acls\/prod-acl/);
    await expect(page.getByRole('heading', { name: /Network ACL: prod-acl/, level: 1 })).toBeVisible();
  });

  test('shows empty state when no ACLs', async ({ page }) => {
    await page.route('**/api/proxy/plugin/vpc-network-management/bff/api/v1/network-acls', (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([]),
      }),
    );

    await page.goto('/vpc-networking/network-acls');
    await expect(page.getByText('No network ACLs found')).toBeVisible();
  });
});

test.describe('Network ACL Detail', () => {
  test.beforeEach(async ({ page }) => {
    await setupDefaultApiMocks(page);
  });

  test('displays network ACL details', async ({ page }) => {
    await page.goto('/vpc-networking/network-acls/prod-acl');

    await expect(page.getByRole('heading', { name: /Network ACL: prod-acl/, level: 1 })).toBeVisible();

    // Breadcrumb should be visible
    await expect(page.getByRole('navigation', { name: 'Breadcrumb' })).toBeVisible();
    await expect(page.getByRole('navigation', { name: 'Breadcrumb' }).getByText('Network ACLs')).toBeVisible();
  });

  test('displays rules section', async ({ page }) => {
    await page.goto('/vpc-networking/network-acls/prod-acl');

    await expect(page.getByRole('heading', { name: /Network ACL: prod-acl/, level: 1 })).toBeVisible();

    // Rules card should be visible
    await expect(page.getByText('Rules')).toBeVisible();
  });

  test('shows VPC info', async ({ page }) => {
    await page.goto('/vpc-networking/network-acls/prod-acl');

    await expect(page.getByRole('heading', { name: /Network ACL: prod-acl/, level: 1 })).toBeVisible();

    // VPC name should be shown
    await expect(page.getByText('prod-vpc')).toBeVisible();
  });
});
