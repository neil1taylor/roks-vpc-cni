import { test, expect } from '@playwright/test';

test.describe('Dashboard Data', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/vpc-networking');
    // Wait for the page heading to confirm the plugin loaded
    await expect(
      page.getByRole('heading', { name: /VPC Networking Dashboard/, level: 1 }),
    ).toBeVisible({ timeout: 30_000 });
  });

  test('VPC API Resources section shows 5 cards', async ({ page }) => {
    await expect(page.getByText('VPC API Resources')).toBeVisible();

    const cardTitles = page.locator('.pf-v5-c-card__title-text');
    await expect(cardTitles.filter({ hasText: /^VPCs$/ })).toBeVisible();
    await expect(cardTitles.filter({ hasText: /^Subnets$/ })).toBeVisible();
    await expect(cardTitles.filter({ hasText: /^Security Groups$/ })).toBeVisible();
    await expect(cardTitles.filter({ hasText: /^Network ACLs$/ })).toBeVisible();
    await expect(cardTitles.filter({ hasText: /^Floating IPs$/ })).toBeVisible();
  });

  test('Kubernetes CRD Resources section shows 4 cards', async ({ page }) => {
    await expect(page.getByText('Kubernetes CRD Resources')).toBeVisible();

    const cardTitles = page.locator('.pf-v5-c-card__title-text');
    await expect(cardTitles.filter({ hasText: /^VPCSubnets$/ })).toBeVisible();
    await expect(cardTitles.filter({ hasText: /^VNIs$/ })).toBeVisible();
    await expect(cardTitles.filter({ hasText: /^VLAN Attachments$/ })).toBeVisible();
    await expect(cardTitles.filter({ hasText: /^FloatingIPs$/ })).toBeVisible();
  });

  test('Network Definitions section shows 4 cards', async ({ page }) => {
    await expect(page.getByText('Network Definitions')).toBeVisible();

    const cardTitles = page.locator('.pf-v5-c-card__title-text');
    await expect(cardTitles.filter({ hasText: /^CUDNs$/ })).toBeVisible();
    await expect(cardTitles.filter({ hasText: /^UDNs$/ })).toBeVisible();
    await expect(cardTitles.filter({ hasText: /^LocalNet$/ })).toBeVisible();
    await expect(cardTitles.filter({ hasText: /^Layer2$/ })).toBeVisible();
  });

  test('VPCs card transitions from spinner to numeric count', async ({ page }) => {
    const cardTitles = page.locator('.pf-v5-c-card__title-text');
    const vpcsCard = page.locator('.pf-v5-c-card').filter({
      has: cardTitles.filter({ hasText: /^VPCs$/ }),
    });
    const cardBody = vpcsCard.locator('.pf-v5-c-card__body');

    // Wait for the count to appear (spinner replaced by <span> with a number)
    const countSpan = cardBody.locator('span').filter({ hasText: /^\d+$/ });
    await expect(countSpan).toBeVisible({ timeout: 30_000 });

    // The count should be a non-negative integer
    const text = await countSpan.textContent();
    const count = parseInt(text!, 10);
    expect(count).toBeGreaterThanOrEqual(0);
  });
});
