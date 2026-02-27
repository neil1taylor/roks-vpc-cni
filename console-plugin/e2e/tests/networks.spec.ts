import { test, expect } from '@playwright/test';
import { setupDefaultApiMocks } from '../helpers/api-mocks';

test.describe('Networks List', () => {
  test.beforeEach(async ({ page }) => {
    await setupDefaultApiMocks(page);
  });

  test('renders networks table with mock data', async ({ page }) => {
    await page.goto('/vpc-networking/networks');

    await expect(page.getByRole('heading', { name: 'Networks', level: 1 })).toBeVisible();

    // Table should have rows for 3 networks (2 CUDNs + 1 UDN)
    const rows = page.locator('tbody tr');
    await expect(rows).toHaveCount(3);

    // Verify network names are present
    await expect(page.getByText('localnet-cudn')).toBeVisible();
    await expect(page.getByText('layer2-cudn')).toBeVisible();
    await expect(page.getByText('layer2-udn')).toBeVisible();
  });

  test('shows topology labels', async ({ page }) => {
    await page.goto('/vpc-networking/networks');

    const rows = page.locator('tbody tr');
    await expect(rows).toHaveCount(3);

    // LocalNet and Layer2 labels should be visible
    await expect(page.getByText('LocalNet').first()).toBeVisible();
    await expect(page.getByText('Layer2')).toBeVisible();
  });

  test('shows scope labels for Cluster and Namespace', async ({ page }) => {
    await page.goto('/vpc-networking/networks');

    const rows = page.locator('tbody tr');
    await expect(rows).toHaveCount(3);

    // Cluster scope for CUDNs and Namespace scope for UDN
    await expect(page.getByText('Cluster').first()).toBeVisible();
    await expect(page.getByText('Namespace')).toBeVisible();
  });

  test('clicking a network name navigates to detail page', async ({ page }) => {
    await page.goto('/vpc-networking/networks');

    // Wait for table to render
    const rows = page.locator('tbody tr');
    await expect(rows).toHaveCount(3);

    // Click the first network name link
    const link = page.getByRole('button', { name: 'localnet-cudn' });
    await link.click();

    // Should navigate to the detail page
    await expect(page).toHaveURL(/\/vpc-networking\/networks\/localnet-cudn/);
    await expect(page.getByRole('heading', { name: /Network:.*localnet-cudn/, level: 1 })).toBeVisible();
  });

  test('shows empty state when no networks', async ({ page }) => {
    // Override both cudns and udns endpoints to return empty arrays
    await page.route('**/api/proxy/plugin/vpc-network-management/bff/api/v1/cudns', (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([]),
      }),
    );
    await page.route('**/api/proxy/plugin/vpc-network-management/bff/api/v1/udns', (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([]),
      }),
    );

    await page.goto('/vpc-networking/networks');
    await expect(page.getByText('No networks found')).toBeVisible();
  });

  test('Create Network button is visible', async ({ page }) => {
    await page.goto('/vpc-networking/networks');
    await expect(page.getByRole('button', { name: /Create Network/ })).toBeVisible();
  });
});

test.describe('Network Detail', () => {
  test.beforeEach(async ({ page }) => {
    await setupDefaultApiMocks(page);
  });

  test('displays network details for a CUDN', async ({ page }) => {
    await page.goto('/vpc-networking/networks/localnet-cudn?kind=ClusterUserDefinedNetwork');

    await expect(page.getByRole('heading', { name: /Network:.*localnet-cudn/, level: 1 })).toBeVisible();

    // Breadcrumb should be visible
    await expect(page.getByRole('navigation', { name: 'Breadcrumb' })).toBeVisible();
    await expect(page.getByRole('navigation', { name: 'Breadcrumb' }).getByText('Networks')).toBeVisible();

    // Overview card should show topology and kind
    await expect(page.getByText('LocalNet')).toBeVisible();
    await expect(page.getByText('ClusterUserDefinedNetwork')).toBeVisible();
  });

  test('shows VPC resources section for LocalNet networks', async ({ page }) => {
    await page.goto('/vpc-networking/networks/localnet-cudn?kind=ClusterUserDefinedNetwork');

    await expect(page.getByRole('heading', { name: /Network:.*localnet-cudn/, level: 1 })).toBeVisible();

    // VPC Resources card should be visible for LocalNet
    await expect(page.getByText('VPC Resources')).toBeVisible();
  });
});

test.describe('Create Network Wizard', () => {
  test.beforeEach(async ({ page }) => {
    await setupDefaultApiMocks(page);
  });

  test('guided flow: progressive reveal and summary', async ({ page }) => {
    await page.goto('/vpc-networking/networks');

    // Open wizard
    await page.getByRole('button', { name: /Create Network/ }).click();

    // Q1 should be visible, Q2/Q3 should not exist in DOM
    await expect(page.getByTestId('guided-q1')).toBeVisible();
    await expect(page.getByTestId('guided-q2')).toHaveCount(0);
    await expect(page.getByTestId('guided-q3')).toHaveCount(0);
    await expect(page.getByTestId('guided-summary')).toHaveCount(0);

    // Pick "Cluster-Internal Only" (Layer2)
    await page.getByTestId('option-cluster-internal').click();

    // Q2 should appear, Q3 still absent
    await expect(page.getByTestId('guided-q2')).toBeVisible();
    await expect(page.getByTestId('guided-q3')).toHaveCount(0);

    // Pick "Cluster-wide"
    await page.getByTestId('option-cluster-wide').click();

    // Q3 should appear
    await expect(page.getByTestId('guided-q3')).toBeVisible();

    // Pick "Secondary"
    await page.getByTestId('option-secondary').click();

    // Summary should show with the resolved combination
    await expect(page.getByTestId('guided-summary')).toBeVisible();
    await expect(page.getByTestId('guided-summary')).toContainText('Layer2 Cluster Secondary');
  });

  test('advanced card grid selects combination and syncs guided flow', async ({ page }) => {
    await page.goto('/vpc-networking/networks');
    await page.getByRole('button', { name: /Create Network/ }).click();

    // Expand the advanced section
    await page.getByText('I know what I need').click();

    // Click a card in the advanced grid
    await page.getByTestId('card-layer2-cudn-primary').click();

    // Guided flow should update — all 3 questions should be visible and answered
    await expect(page.getByTestId('guided-q1')).toBeVisible();
    await expect(page.getByTestId('guided-q2')).toBeVisible();
    await expect(page.getByTestId('guided-q3')).toBeVisible();

    // Summary should show with the matching combination
    await expect(page.getByTestId('guided-summary')).toBeVisible();
    await expect(page.getByTestId('guided-summary')).toContainText('Layer2 Cluster Primary');

    // Primary warning should be visible
    await expect(page.getByText('Primary networks replace the default pod network')).toBeVisible();
  });

  test('cascading reset: changing Q1 resets Q2 and Q3', async ({ page }) => {
    await page.goto('/vpc-networking/networks');
    await page.getByRole('button', { name: /Create Network/ }).click();

    // Complete the guided flow
    await page.getByTestId('option-cluster-internal').click();
    await page.getByTestId('option-cluster-wide').click();
    await page.getByTestId('option-secondary').click();
    await expect(page.getByTestId('guided-summary')).toBeVisible();

    // Change Q1 to VPC-Routable (LocalNet)
    await page.getByTestId('option-vpc-routable').click();

    // Q2 should be visible; for LocalNet, scope auto-selects to CUDN (only valid scope),
    // then role auto-selects to Secondary (only valid role), producing a summary
    await expect(page.getByTestId('guided-q2')).toBeVisible();
    await expect(page.getByTestId('guided-summary')).toBeVisible();
    await expect(page.getByTestId('guided-summary')).toContainText('LocalNet Cluster Secondary');
  });

  test('Primary role shows caution warning', async ({ page }) => {
    await page.goto('/vpc-networking/networks');
    await page.getByRole('button', { name: /Create Network/ }).click();

    // Navigate to Q3 — use Layer2 + CUDN (only combo that supports Primary)
    await page.getByTestId('option-cluster-internal').click();
    await page.getByTestId('option-cluster-wide').click();

    // Select Primary
    await page.getByTestId('option-primary').click();

    // Warning alert should appear
    await expect(page.getByText('Primary networks replace the default pod network')).toBeVisible();
  });
});
