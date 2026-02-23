import { test, expect } from '@playwright/test';
import { setupDefaultApiMocks } from '../helpers/api-mocks';

test.describe('VPC Dashboard', () => {
  test.beforeEach(async ({ page }) => {
    await setupDefaultApiMocks(page);
  });

  test('renders the dashboard title', async ({ page }) => {
    await page.goto('/vpc-networking');
    await expect(page.getByRole('heading', { name: 'VPC Networking Dashboard', level: 1 })).toBeVisible();
  });

  test('renders VPC API resource count cards', async ({ page }) => {
    await page.goto('/vpc-networking');

    await expect(page.getByText('VPC API Resources')).toBeVisible();

    // Use CardTitle text matching — each card title is rendered in a <div class="pf-v5-c-card__title">
    const cardTitles = page.locator('.pf-v5-c-card__title-text');
    await expect(cardTitles.filter({ hasText: /^VPCs$/ })).toBeVisible();
    await expect(cardTitles.filter({ hasText: /^Subnets$/ })).toBeVisible();
    await expect(cardTitles.filter({ hasText: /^Security Groups$/ })).toBeVisible();
    await expect(cardTitles.filter({ hasText: /^Network ACLs$/ })).toBeVisible();
    await expect(cardTitles.filter({ hasText: /^Floating IPs$/ })).toBeVisible();

    // Verify VPC count renders (mock has 2 VPCs)
    // The count is in a <span> inside the card body, next to the card titled "VPCs"
    const vpcsCard = page.locator('.pf-v5-c-card').filter({ has: cardTitles.filter({ hasText: /^VPCs$/ }) });
    await expect(vpcsCard.locator('.pf-v5-c-card__body span')).toContainText('2');
  });

  test('renders Kubernetes CRD resource section', async ({ page }) => {
    await page.goto('/vpc-networking');

    await expect(page.getByText('Kubernetes CRD Resources')).toBeVisible();

    const cardTitles = page.locator('.pf-v5-c-card__title-text');
    await expect(cardTitles.filter({ hasText: /^VPCSubnets$/ })).toBeVisible();
    await expect(cardTitles.filter({ hasText: /^VNIs$/ })).toBeVisible();
    await expect(cardTitles.filter({ hasText: /^VLAN Attachments$/ })).toBeVisible();
    await expect(cardTitles.filter({ hasText: /^FloatingIPs$/ })).toBeVisible();
  });
});
